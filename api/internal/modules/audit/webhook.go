package audit

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrWebhookTenantIDRequired = errors.New("tenant_id is required")
var ErrWebhookEndpointRequired = errors.New("webhook endpoint is required when enabled")
var ErrInvalidWebhookEndpoint = errors.New("webhook endpoint must use http:// or https://")
var ErrWebhookConfigNotFound = errors.New("audit webhook config not found")
var ErrWebhookDisabled = errors.New("audit webhook is disabled")
var ErrWebhookActionFiltered = errors.New("audit webhook action is filtered")
var ErrAuditLogNotFound = errors.New("audit log not found")

type WebhookConfig struct {
	TenantID  string    `json:"tenant_id"`
	Enabled   bool      `json:"enabled"`
	Endpoint  string    `json:"endpoint"`
	Actions   []string  `json:"actions,omitempty"`
	UpdatedBy string    `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WebhookDelivery struct {
	TenantID     string    `json:"tenant_id"`
	ID           string    `json:"id"`
	AuditLogID   string    `json:"audit_log_id"`
	Action       string    `json:"action"`
	Endpoint     string    `json:"endpoint"`
	Status       string    `json:"status"`
	HTTPStatus   int       `json:"http_status,omitempty"`
	Error        string    `json:"error,omitempty"`
	ResponseBody string    `json:"response_body,omitempty"`
	DispatchedAt time.Time `json:"dispatched_at"`
}

func (s *Service) GetWebhookConfig(tenantID string) (WebhookConfig, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return WebhookConfig{}, ErrWebhookTenantIDRequired
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	record, exists := s.webhookConfigs[nextTenantID]
	if !exists {
		return WebhookConfig{}, ErrWebhookConfigNotFound
	}
	record.Actions = append([]string(nil), record.Actions...)
	return record, nil
}

func (s *Service) UpsertWebhookConfig(
	tenantID string,
	enabled bool,
	endpoint string,
	actions []string,
	updatedBy string,
) (WebhookConfig, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return WebhookConfig{}, ErrWebhookTenantIDRequired
	}

	nextEndpoint := strings.TrimSpace(endpoint)
	nextActions := normalizeWebhookActions(actions)
	nextUpdatedBy := strings.TrimSpace(updatedBy)
	if nextUpdatedBy == "" {
		nextUpdatedBy = "system"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.webhookConfigs == nil {
		s.webhookConfigs = make(map[string]WebhookConfig)
	}

	current := s.webhookConfigs[nextTenantID]
	if nextEndpoint == "" {
		nextEndpoint = strings.TrimSpace(current.Endpoint)
	}
	if enabled {
		if nextEndpoint == "" {
			return WebhookConfig{}, ErrWebhookEndpointRequired
		}
		if !isHTTPWebhookEndpoint(nextEndpoint) {
			return WebhookConfig{}, ErrInvalidWebhookEndpoint
		}
	}

	record := WebhookConfig{
		TenantID:  nextTenantID,
		Enabled:   enabled,
		Endpoint:  nextEndpoint,
		Actions:   nextActions,
		UpdatedBy: nextUpdatedBy,
		UpdatedAt: time.Now().UTC(),
	}
	s.webhookConfigs[nextTenantID] = record
	if err := s.persistLocked(); err != nil {
		return WebhookConfig{}, err
	}
	record.Actions = append([]string(nil), record.Actions...)
	return record, nil
}

func (s *Service) ListWebhookDeliveries(tenantID string, limit int) []WebhookDelivery {
	filterTenantID := strings.TrimSpace(tenantID)
	if limit < 0 {
		limit = 0
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]WebhookDelivery, 0, len(s.webhookDeliveries))
	for i := range s.webhookDeliveries {
		if filterTenantID != "" && strings.TrimSpace(s.webhookDeliveries[i].TenantID) != filterTenantID {
			continue
		}
		items = append(items, s.webhookDeliveries[i])
	}
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) FindLogByID(tenantID string, logID string) (Log, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return Log{}, ErrWebhookTenantIDRequired
	}
	nextLogID := strings.TrimSpace(logID)
	if nextLogID == "" {
		return Log{}, ErrAuditLogNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.logs {
		if strings.TrimSpace(s.logs[i].TenantID) != nextTenantID {
			continue
		}
		if strings.TrimSpace(s.logs[i].ID) == nextLogID {
			return s.logs[i], nil
		}
	}
	return Log{}, ErrAuditLogNotFound
}

func (s *Service) DispatchWebhookForLog(
	ctx context.Context,
	tenantID string,
	logRecord Log,
	client *http.Client,
) (WebhookDelivery, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return WebhookDelivery{}, ErrWebhookTenantIDRequired
	}

	s.mu.RLock()
	config, exists := s.webhookConfigs[nextTenantID]
	s.mu.RUnlock()
	if !exists {
		return WebhookDelivery{}, ErrWebhookConfigNotFound
	}
	if !config.Enabled {
		return WebhookDelivery{}, ErrWebhookDisabled
	}
	if !webhookActionAllowed(config.Actions, logRecord.Action) {
		return WebhookDelivery{}, ErrWebhookActionFiltered
	}

	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	payload := map[string]any{
		"tenant_id": nextTenantID,
		"event":     logRecord,
		"sent_at":   time.Now().UTC(),
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return WebhookDelivery{}, err
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		config.Endpoint,
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return WebhookDelivery{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-MistyPass-Event-ID", strings.TrimSpace(logRecord.ID))
	request.Header.Set("X-MistyPass-Event-Action", strings.TrimSpace(logRecord.Action))

	delivery := WebhookDelivery{
		TenantID:     nextTenantID,
		AuditLogID:   strings.TrimSpace(logRecord.ID),
		Action:       strings.TrimSpace(logRecord.Action),
		Endpoint:     strings.TrimSpace(config.Endpoint),
		Status:       "failed",
		DispatchedAt: time.Now().UTC(),
	}
	deliveryID, idErr := webhookDeliveryID()
	if idErr != nil {
		return WebhookDelivery{}, idErr
	}
	delivery.ID = deliveryID

	resp, reqErr := client.Do(request)
	if reqErr != nil {
		delivery.Error = strings.TrimSpace(reqErr.Error())
		s.recordWebhookDelivery(delivery)
		return delivery, fmt.Errorf("webhook request failed: %w", reqErr)
	}
	defer resp.Body.Close()

	responseBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	if readErr != nil {
		delivery.Error = strings.TrimSpace(readErr.Error())
		delivery.HTTPStatus = resp.StatusCode
		s.recordWebhookDelivery(delivery)
		return delivery, fmt.Errorf("failed reading webhook response: %w", readErr)
	}
	delivery.HTTPStatus = resp.StatusCode
	delivery.ResponseBody = strings.TrimSpace(string(responseBytes))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		delivery.Status = "success"
		s.recordWebhookDelivery(delivery)
		return delivery, nil
	}

	delivery.Error = fmt.Sprintf("webhook status %d", resp.StatusCode)
	s.recordWebhookDelivery(delivery)
	return delivery, fmt.Errorf("webhook returned status %d", resp.StatusCode)
}

func (s *Service) recordWebhookDelivery(delivery WebhookDelivery) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.webhookDeliveries = append([]WebhookDelivery{delivery}, s.webhookDeliveries...)
	_ = s.persistLocked()
}

func normalizeWebhookActions(actions []string) []string {
	output := make([]string, 0, len(actions))
	seen := make(map[string]struct{}, len(actions))
	for i := range actions {
		action := strings.ToLower(strings.TrimSpace(actions[i]))
		if action == "" {
			continue
		}
		if _, exists := seen[action]; exists {
			continue
		}
		seen[action] = struct{}{}
		output = append(output, action)
	}
	return output
}

func webhookActionAllowed(actions []string, action string) bool {
	if len(actions) == 0 {
		return true
	}
	target := strings.ToLower(strings.TrimSpace(action))
	for i := range actions {
		if actions[i] == target {
			return true
		}
	}
	return false
}

func isHTTPWebhookEndpoint(input string) bool {
	next := strings.ToLower(strings.TrimSpace(input))
	return strings.HasPrefix(next, "https://") || strings.HasPrefix(next, "http://")
}

func webhookDeliveryID() (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "awd_" + hex.EncodeToString(raw), nil
}
