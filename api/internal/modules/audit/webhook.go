package audit

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
	TenantID      string    `json:"tenant_id"`
	Enabled       bool      `json:"enabled"`
	Endpoint      string    `json:"endpoint"`
	Actions       []string  `json:"actions,omitempty"`
	SigningSecret string    `json:"signing_secret,omitempty"`
	UpdatedBy     string    `json:"updated_by"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type WebhookDelivery struct {
	TenantID     string    `json:"tenant_id"`
	ID           string    `json:"id"`
	AuditLogID   string    `json:"audit_log_id"`
	Action       string    `json:"action"`
	Endpoint     string    `json:"endpoint"`
	Status       string    `json:"status"`
	AttemptCount int       `json:"attempt_count,omitempty"`
	HTTPStatus   int       `json:"http_status,omitempty"`
	Error        string    `json:"error,omitempty"`
	ResponseBody string    `json:"response_body,omitempty"`
	DispatchedAt time.Time `json:"dispatched_at"`
}

const (
	webhookSignatureHeader          = "X-MistyPass-Signature"
	webhookSignatureTimestampHeader = "X-MistyPass-Signature-Timestamp"
	webhookSignatureAlgorithm       = "sha256"
	webhookDispatchMaxAttempts      = 3
	webhookDispatchRetryBaseDelay   = 200 * time.Millisecond
	webhookDeliveryMaxRecords       = 1000
)

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
	signingSecret string,
	updatedBy string,
) (WebhookConfig, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return WebhookConfig{}, ErrWebhookTenantIDRequired
	}

	nextEndpoint := strings.TrimSpace(endpoint)
	nextActions := normalizeWebhookActions(actions)
	nextSigningSecret := strings.TrimSpace(signingSecret)
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
	if nextSigningSecret == "" {
		nextSigningSecret = strings.TrimSpace(current.SigningSecret)
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
		TenantID:      nextTenantID,
		Enabled:       enabled,
		Endpoint:      nextEndpoint,
		Actions:       nextActions,
		SigningSecret: nextSigningSecret,
		UpdatedBy:     nextUpdatedBy,
		UpdatedAt:     time.Now().UTC(),
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

	dispatchedAt := time.Now().UTC()
	payload := map[string]any{
		"tenant_id": nextTenantID,
		"event":     logRecord,
		"sent_at":   dispatchedAt,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return WebhookDelivery{}, err
	}

	delivery := WebhookDelivery{
		TenantID:     nextTenantID,
		AuditLogID:   strings.TrimSpace(logRecord.ID),
		Action:       strings.TrimSpace(logRecord.Action),
		Endpoint:     strings.TrimSpace(config.Endpoint),
		Status:       "failed",
		DispatchedAt: dispatchedAt,
	}
	deliveryID, idErr := webhookDeliveryID()
	if idErr != nil {
		return WebhookDelivery{}, idErr
	}
	delivery.ID = deliveryID

	signingSecret := strings.TrimSpace(config.SigningSecret)
	var lastErr error
	for attempt := 1; attempt <= webhookDispatchMaxAttempts; attempt++ {
		delivery.AttemptCount = attempt
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
		if signingSecret != "" {
			timestamp := strconv.FormatInt(dispatchedAt.Unix(), 10)
			request.Header.Set(webhookSignatureTimestampHeader, timestamp)
			request.Header.Set(webhookSignatureHeader, webhookPayloadSignature(signingSecret, timestamp, bodyBytes))
		}

		resp, reqErr := client.Do(request)
		if reqErr != nil {
			delivery.Error = strings.TrimSpace(reqErr.Error())
			delivery.HTTPStatus = 0
			delivery.ResponseBody = ""
			lastErr = fmt.Errorf("webhook request failed: %w", reqErr)
			if !shouldRetryWebhookDispatch(attempt, 0) || !waitWebhookRetryDelay(ctx, attempt) {
				break
			}
			continue
		}

		responseBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		_ = resp.Body.Close()
		if readErr != nil {
			delivery.Error = strings.TrimSpace(readErr.Error())
			delivery.HTTPStatus = resp.StatusCode
			delivery.ResponseBody = ""
			lastErr = fmt.Errorf("failed reading webhook response: %w", readErr)
			if !shouldRetryWebhookDispatch(attempt, resp.StatusCode) || !waitWebhookRetryDelay(ctx, attempt) {
				break
			}
			continue
		}

		delivery.HTTPStatus = resp.StatusCode
		delivery.ResponseBody = strings.TrimSpace(string(responseBytes))
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			delivery.Status = "success"
			delivery.Error = ""
			s.recordWebhookDelivery(delivery)
			return delivery, nil
		}

		delivery.Error = fmt.Sprintf("webhook status %d", resp.StatusCode)
		lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
		if !shouldRetryWebhookDispatch(attempt, resp.StatusCode) || !waitWebhookRetryDelay(ctx, attempt) {
			break
		}
	}

	if lastErr == nil {
		lastErr = errors.New("webhook dispatch failed")
	}
	s.recordWebhookDelivery(delivery)
	return delivery, lastErr
}

func (s *Service) recordWebhookDelivery(delivery WebhookDelivery) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.webhookDeliveries = append([]WebhookDelivery{delivery}, s.webhookDeliveries...)
	if len(s.webhookDeliveries) > webhookDeliveryMaxRecords {
		s.webhookDeliveries = append([]WebhookDelivery(nil), s.webhookDeliveries[:webhookDeliveryMaxRecords]...)
	}
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

func webhookPayloadSignature(secret, timestamp string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	mac.Write([]byte(strings.TrimSpace(timestamp)))
	mac.Write([]byte("."))
	mac.Write(payload)
	return webhookSignatureAlgorithm + "=" + hex.EncodeToString(mac.Sum(nil))
}

func shouldRetryWebhookDispatch(attempt int, statusCode int) bool {
	if attempt >= webhookDispatchMaxAttempts {
		return false
	}
	if statusCode == 0 {
		return true
	}
	if statusCode == http.StatusRequestTimeout || statusCode == http.StatusTooManyRequests {
		return true
	}
	return statusCode >= http.StatusInternalServerError
}

func waitWebhookRetryDelay(ctx context.Context, attempt int) bool {
	if attempt >= webhookDispatchMaxAttempts {
		return false
	}
	delay := webhookDispatchRetryBaseDelay * time.Duration(1<<(attempt-1))
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
