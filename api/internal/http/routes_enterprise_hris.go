package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/hris"
	"github.com/mistypass/cloud/api/internal/modules/hris/talenta"
	"github.com/mistypass/cloud/api/internal/redistore"
	"github.com/mistypass/cloud/api/internal/retrybackoff"
)


const enterpriseHRISWebhookMaxPayloadBytes = 1 << 20
const enterpriseHRISWebhookQueueListMaxLimit = 500

type hrisWebhookRuntimeCounts struct {
	All          int `json:"all"`
	Ready        int `json:"ready"`
	Cooldown     int `json:"cooldown"`
	InFlight     int `json:"in_flight"`
	AttemptLimit int `json:"attempt_limit"`
	Terminal     int `json:"terminal"`
}

type hrisWebhookReceiptListItem struct {
	enterprise.HRISWebhookReceipt
	QueueState           string     `json:"queue_state"`
	NextRetryAt          *time.Time `json:"next_retry_at,omitempty"`
	ProcessingDeadlineAt *time.Time `json:"processing_deadline_at,omitempty"`
	RemainingAttempts    int        `json:"remaining_attempts"`
	CooldownRemainingSec int64      `json:"cooldown_remaining_seconds"`
	StaleInFlight        bool       `json:"stale_in_flight"`
}

type hrisWebhookReceiptListResult struct {
	Items       []hrisWebhookReceiptListItem `json:"items"`
	Total       int                          `json:"total"`
	Offset      int                          `json:"offset"`
	Limit       int                          `json:"limit"`
	NextOffset  int                          `json:"next_offset,omitempty"`
	HasMore     bool                         `json:"has_more"`
	QueueCounts hrisWebhookRuntimeCounts     `json:"queue_counts"`
}

type hrisWebhookReceiptBatchProcessItem struct {
	ReceiptID   string                         `json:"receipt_id"`
	Status      string                         `json:"status"`
	Reason      string                         `json:"reason,omitempty"`
	Error       string                         `json:"error,omitempty"`
	ExecutionID string                         `json:"execution_id,omitempty"`
	Item        *enterprise.HRISWebhookReceipt `json:"item,omitempty"`
}

type hrisWebhookReceiptBatchProcessResult struct {
	TenantID      string                               `json:"tenant_id"`
	TotalReceipts int                                  `json:"total_receipts"`
	Queued        int                                  `json:"queued,omitempty"`
	Processed     int                                  `json:"processed"`
	Skipped       int                                  `json:"skipped"`
	Failed        int                                  `json:"failed"`
	DLQ           int                                  `json:"dlq"`
	ExecutionMode string                               `json:"execution_mode,omitempty"`
	DispatchMode  string                               `json:"dispatch_mode,omitempty"`
	Items         []hrisWebhookReceiptBatchProcessItem `json:"items,omitempty"`
	UpdatedAt     time.Time                            `json:"updated_at"`
}

type hrisWebhookDLQListItem struct {
	hris.DeadLetterEntry
	ReplayState          string     `json:"replay_state"`
	NextRetryAt          *time.Time `json:"next_retry_at,omitempty"`
	ProcessingDeadlineAt *time.Time `json:"processing_deadline_at,omitempty"`
	RemainingAttempts    int        `json:"remaining_attempts"`
	CooldownRemainingSec int64      `json:"cooldown_remaining_seconds"`
	StaleInFlight        bool       `json:"stale_in_flight"`
}

type hrisWebhookDLQListResult struct {
	Items        []hrisWebhookDLQListItem `json:"items"`
	Total        int                      `json:"total"`
	Offset       int                      `json:"offset"`
	Limit        int                      `json:"limit"`
	NextOffset   int                      `json:"next_offset,omitempty"`
	HasMore      bool                     `json:"has_more"`
	ReplayCounts hrisWebhookRuntimeCounts `json:"replay_counts"`
}

type hrisWebhookDLQBatchReplayItem struct {
	EntryID     string                `json:"entry_id"`
	Status      string                `json:"status"`
	Reason      string                `json:"reason,omitempty"`
	Error       string                `json:"error,omitempty"`
	ExecutionID string                `json:"execution_id,omitempty"`
	Item        *hris.DeadLetterEntry `json:"item,omitempty"`
}

type hrisWebhookDLQBatchReplayResult struct {
	TenantID      string                          `json:"tenant_id"`
	TotalEntries  int                             `json:"total_entries"`
	Queued        int                             `json:"queued,omitempty"`
	Replayed      int                             `json:"replayed"`
	Skipped       int                             `json:"skipped"`
	Failed        int                             `json:"failed"`
	ExecutionMode string                          `json:"execution_mode,omitempty"`
	DispatchMode  string                          `json:"dispatch_mode,omitempty"`
	Items         []hrisWebhookDLQBatchReplayItem `json:"items,omitempty"`
	UpdatedAt     time.Time                       `json:"updated_at"`
}

type hrisWebhookExecutionStatusCounts struct {
	All       int `json:"all"`
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

type hrisWebhookExecutionListItem struct {
	enterprise.HRISWebhookExecution
	QueueState                        string     `json:"queue_state,omitempty"`
	NextRetryAt                       *time.Time `json:"next_retry_at,omitempty"`
	ProcessingDeadlineAt              *time.Time `json:"processing_deadline_at,omitempty"`
	CooldownRemainingSec              int64      `json:"cooldown_remaining_seconds,omitempty"`
	StaleInFlight                     bool       `json:"stale_in_flight,omitempty"`
	ExternalQueueName                 string     `json:"external_queue_name,omitempty"`
	ExternalQueueState                string     `json:"external_queue_state,omitempty"`
	ExternalQueueVisibilityDeadlineAt *time.Time `json:"external_queue_visibility_deadline_at,omitempty"`
}

type hrisWebhookExecutionExternalQueueSummary struct {
	Kind         string `json:"kind"`
	QueueName    string `json:"queue_name"`
	PendingCount int    `json:"pending_count"`
	ClaimedCount int    `json:"claimed_count"`
}

type hrisWebhookExecutionListResult struct {
	Items          []hrisWebhookExecutionListItem             `json:"items"`
	Total          int                                        `json:"total"`
	Offset         int                                        `json:"offset"`
	Limit          int                                        `json:"limit"`
	NextOffset     int                                        `json:"next_offset,omitempty"`
	HasMore        bool                                       `json:"has_more"`
	QueueCounts    hrisWebhookRuntimeCounts                   `json:"queue_counts"`
	StatusCounts   hrisWebhookExecutionStatusCounts           `json:"status_counts"`
	ExternalQueues []hrisWebhookExecutionExternalQueueSummary `json:"external_queues,omitempty"`
}

type hrisWebhookExecutionReplayResponse struct {
	SourceExecutionID string                           `json:"source_execution_id"`
	ExecutionMode     string                           `json:"execution_mode"`
	DispatchMode      string                           `json:"dispatch_mode,omitempty"`
	ExecutionID       string                           `json:"execution_id,omitempty"`
	Execution         *enterprise.HRISWebhookExecution `json:"execution,omitempty"`
	ReceiptItem       *enterprise.HRISWebhookReceipt   `json:"receipt_item,omitempty"`
	DLQItem           *hris.DeadLetterEntry            `json:"dlq_item,omitempty"`
}

const (
	enterpriseExecutionModeInline = "inline"
	enterpriseExecutionModeQueued = "queued"
)

const enterpriseHRISWebhookExecutionReplayAuditSource = "enterprise_sync_execution_replay"

const (
	enterpriseHRISWebhookExecutionDispatchModeWorkerTick        = enterprise.HRISWebhookExecutionDispatchModeWorkerTick
	enterpriseHRISWebhookExecutionDispatchModeWorkerTaskChannel = enterprise.HRISWebhookExecutionDispatchModeWorkerTaskChannel
	enterpriseHRISWebhookExecutionDispatchModeGoroutineFallback = enterprise.HRISWebhookExecutionDispatchModeGoroutineFallback
)

var (
	errEnterpriseHRISWebhookQueuedReceiptWorkerRequired = errors.New(
		"queued hris webhook receipt processing with require_worker=true requires enabled receipt worker",
	)
	errEnterpriseHRISWebhookQueuedDLQWorkerRequired = errors.New(
		"queued hris webhook dlq replay with require_worker=true requires enabled dlq worker",
	)
)

type hrisWebhookReceiptProcessConflictError struct {
	reason string
}

type hrisQueueRuntime struct {
	State                string
	NextRetryAt          *time.Time
	ProcessingDeadlineAt *time.Time
	RemainingAttempts    int
	CooldownRemainingSec int64
	StaleInFlight        bool
}

type hrisWebhookReceiptListOptions struct {
	ConnectorID string
	Status      string
	QueueState  string
	Query       string
	Offset      int
	Limit       int
}

type hrisWebhookDLQListOptions struct {
	ConnectorID string
	Status      string
	ReplayState string
	Query       string
	Offset      int
	Limit       int
}

type hrisWebhookExecutionListOptions struct {
	ConnectorID   string
	Kind          string
	Status        string
	QueueState    string
	ReplayScope   string
	ExecutionMode string
	DispatchMode  string
	TargetStatus  string
	TargetID      string
	Query         string
	Offset        int
	Limit         int
}

func (e *hrisWebhookReceiptProcessConflictError) Error() string {
	return fmt.Sprintf("hris webhook receipt is not ready for processing: %s", e.reason)
}

func normalizeEnterpriseExecutionMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", enterpriseExecutionModeInline:
		return enterpriseExecutionModeInline
	case enterpriseExecutionModeQueued:
		return enterpriseExecutionModeQueued
	default:
		return ""
	}
}

func parseOptionalEnterpriseBool(raw string, field string) (bool, error) {
	nextRaw := strings.TrimSpace(raw)
	if nextRaw == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(nextRaw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", field)
	}
	return value, nil
}

func (s *server) listEnterpriseHRISConnectors(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.enterpriseSvc.ListHRISConnectors(tenantID),
	})
}

func (s *server) listEnterpriseHRISSecrets(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if s.hrisVaultSvc == nil {
		writeError(w, http.StatusInternalServerError, "hris vault service is not configured")
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref != "" {
		item, err := s.hrisVaultSvc.GetSecretMetadataByRef(tenantID, ref)
		if err != nil {
			switch {
			case errors.Is(err, hris.ErrTenantIDRequired),
				errors.Is(err, hris.ErrInvalidSecretRef):
				writeError(w, http.StatusBadRequest, err.Error())
			case errors.Is(err, hris.ErrSecretNotFound):
				writeError(w, http.StatusNotFound, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"item": item,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.hrisVaultSvc.ListSecrets(tenantID),
	})
}

func (s *server) upsertEnterpriseHRISSecret(w http.ResponseWriter, r *http.Request) {
	if s.hrisVaultSvc == nil {
		writeError(w, http.StatusInternalServerError, "hris vault service is not configured")
		return
	}

	var request struct {
		TenantID  string `json:"tenant_id"`
		Name      string `json:"name"`
		Kind      string `json:"kind"`
		Value     string `json:"value"`
		UpdatedBy string `json:"updated_by"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	updatedBy := strings.TrimSpace(request.UpdatedBy)
	if updatedBy == "" {
		if user, exists := authenticatedUser(r); exists {
			updatedBy = strings.TrimSpace(user.Email)
		}
	}
	item, err := s.hrisVaultSvc.UpsertSecret(tenantID, request.Name, request.Kind, request.Value, updatedBy)
	if err != nil {
		switch {
		case errors.Is(err, hris.ErrTenantIDRequired),
			errors.Is(err, hris.ErrSecretNameRequired),
			errors.Is(err, hris.ErrSecretValueRequired),
			errors.Is(err, hris.ErrInvalidSecretName),
			errors.Is(err, hris.ErrInvalidSecretKind):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	s.appendAuditLog(
		r,
		tenantID,
		"enterprise_hris_secret_upserted",
		fmt.Sprintf("ref=%s,kind=%s", item.Ref, item.Kind),
		"enterprise_sync",
	)
	writeJSON(w, http.StatusOK, map[string]any{
		"item": item,
	})
}

func (s *server) createEnterpriseHRISConnector(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID           string `json:"tenant_id"`
		Vendor             string `json:"vendor"`
		Status             string `json:"status"`
		SyncStrategy       string `json:"sync_strategy"`
		CredentialRef      string `json:"credential_ref"`
		CredentialValue    string `json:"credential_value"`
		WebhookSecretRef   string `json:"webhook_secret_ref"`
		WebhookSecretValue string `json:"webhook_secret_value"`
		UpdatedBy          string `json:"updated_by"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	updatedBy := strings.TrimSpace(request.UpdatedBy)
	if updatedBy == "" {
		if user, exists := authenticatedUser(r); exists {
			updatedBy = strings.TrimSpace(user.Email)
		}
	}
	if strings.TrimSpace(request.CredentialRef) != "" && strings.TrimSpace(request.CredentialValue) != "" {
		writeError(w, http.StatusBadRequest, "credential_ref and credential_value cannot both be set")
		return
	}
	if strings.TrimSpace(request.WebhookSecretRef) != "" && strings.TrimSpace(request.WebhookSecretValue) != "" {
		writeError(w, http.StatusBadRequest, "webhook_secret_ref and webhook_secret_value cannot both be set")
		return
	}
	credentialRef, webhookSecretRef, err := s.materializeEnterpriseHRISConnectorSecrets(
		tenantID,
		request.Vendor,
		request.CredentialRef,
		request.CredentialValue,
		request.WebhookSecretRef,
		request.WebhookSecretValue,
		updatedBy,
	)
	if err != nil {
		switch {
		case errors.Is(err, hris.ErrTenantIDRequired),
			errors.Is(err, hris.ErrSecretNameRequired),
			errors.Is(err, hris.ErrSecretValueRequired),
			errors.Is(err, hris.ErrInvalidSecretName),
			errors.Is(err, hris.ErrInvalidSecretKind):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	created, err := s.enterpriseSvc.CreateHRISConnector(
		tenantID,
		request.Vendor,
		request.Status,
		request.SyncStrategy,
		credentialRef,
		webhookSecretRef,
		updatedBy,
	)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrInvalidHRISConnectorVendor),
			errors.Is(err, enterprise.ErrInvalidHRISConnectorStatus),
			errors.Is(err, enterprise.ErrInvalidHRISConnectorSyncStrategy):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrHRISConnectorAlreadyExists):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	s.appendAuditLog(
		r,
		tenantID,
		"enterprise_hris_connector_created",
		fmt.Sprintf(
			"connector_id=%s,vendor=%s,status=%s,sync_strategy=%s",
			created.ID,
			created.Vendor,
			created.Status,
			created.SyncStrategy,
		),
		"enterprise_sync",
	)
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) updateEnterpriseHRISConnector(w http.ResponseWriter, r *http.Request) {
	connectorID := chi.URLParam(r, "connectorID")
	var request struct {
		TenantID           string `json:"tenant_id"`
		Status             string `json:"status"`
		SyncStrategy       string `json:"sync_strategy"`
		CredentialRef      string `json:"credential_ref"`
		CredentialValue    string `json:"credential_value"`
		WebhookSecretRef   string `json:"webhook_secret_ref"`
		WebhookSecretValue string `json:"webhook_secret_value"`
		UpdatedBy          string `json:"updated_by"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	updatedBy := strings.TrimSpace(request.UpdatedBy)
	if updatedBy == "" {
		if user, exists := authenticatedUser(r); exists {
			updatedBy = strings.TrimSpace(user.Email)
		}
	}
	if strings.TrimSpace(request.CredentialRef) != "" && strings.TrimSpace(request.CredentialValue) != "" {
		writeError(w, http.StatusBadRequest, "credential_ref and credential_value cannot both be set")
		return
	}
	if strings.TrimSpace(request.WebhookSecretRef) != "" && strings.TrimSpace(request.WebhookSecretValue) != "" {
		writeError(w, http.StatusBadRequest, "webhook_secret_ref and webhook_secret_value cannot both be set")
		return
	}
	currentConnector, err := s.enterpriseSvc.GetHRISConnector(tenantID, connectorID)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrHRISConnectorNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	credentialRef, webhookSecretRef, err := s.materializeEnterpriseHRISConnectorSecrets(
		tenantID,
		currentConnector.Vendor,
		request.CredentialRef,
		request.CredentialValue,
		request.WebhookSecretRef,
		request.WebhookSecretValue,
		updatedBy,
	)
	if err != nil {
		switch {
		case errors.Is(err, hris.ErrTenantIDRequired),
			errors.Is(err, hris.ErrSecretNameRequired),
			errors.Is(err, hris.ErrSecretValueRequired),
			errors.Is(err, hris.ErrInvalidSecretName),
			errors.Is(err, hris.ErrInvalidSecretKind):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	updated, err := s.enterpriseSvc.UpdateHRISConnector(
		tenantID,
		connectorID,
		request.Status,
		request.SyncStrategy,
		credentialRef,
		webhookSecretRef,
		updatedBy,
	)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrInvalidHRISConnectorStatus),
			errors.Is(err, enterprise.ErrInvalidHRISConnectorSyncStrategy):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrHRISConnectorNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	s.appendAuditLog(
		r,
		tenantID,
		"enterprise_hris_connector_updated",
		fmt.Sprintf(
			"connector_id=%s,vendor=%s,status=%s,sync_strategy=%s",
			updated.ID,
			updated.Vendor,
			updated.Status,
			updated.SyncStrategy,
		),
		"enterprise_sync",
	)
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) receiveEnterpriseHRISWebhook(w http.ResponseWriter, r *http.Request) {
	connectorID := chi.URLParam(r, "connectorID")
	bodyReader := http.MaxBytesReader(w, r.Body, enterpriseHRISWebhookMaxPayloadBytes)
	payload, err := io.ReadAll(bodyReader)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(
				w,
				http.StatusRequestEntityTooLarge,
				fmt.Sprintf("webhook payload exceeds %d bytes", enterpriseHRISWebhookMaxPayloadBytes),
			)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	connector, err := s.resolveEnterpriseHRISWebhookConnector(connectorID)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrHRISConnectorNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, enterprise.ErrHRISConnectorInactive):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	if err := s.validateEnterpriseHRISWebhookRequest(r, connector, payload); err != nil {
		s.appendAuditLog(
			r,
			connector.TenantID,
			"enterprise_hris_webhook_rejected",
			fmt.Sprintf(
				"connector_id=%s,vendor=%s,error=%s",
				connector.ID,
				connector.Vendor,
				err.Error(),
			),
			"enterprise_webhook",
		)
		switch {
		case errors.Is(err, hris.ErrSecretNotFound),
			errors.Is(err, hris.ErrInvalidSecretRef),
			errors.Is(err, talenta.ErrWebhookSecretRequired):
			writeError(w, http.StatusInternalServerError, "internal server error")
		case errors.Is(err, talenta.ErrWebhookAuthorizationHeaderRequired),
			errors.Is(err, talenta.ErrWebhookDateHeaderRequired),
			errors.Is(err, talenta.ErrWebhookDigestHeaderRequired),
			errors.Is(err, talenta.ErrInvalidWebhookAuthorization),
			errors.Is(err, talenta.ErrInvalidWebhookDate),
			errors.Is(err, talenta.ErrInvalidWebhookDigest),
			errors.Is(err, talenta.ErrWebhookClientIDMismatch),
			errors.Is(err, talenta.ErrWebhookDateSkewExceeded),
			errors.Is(err, talenta.ErrWebhookDigestMismatch),
			errors.Is(err, talenta.ErrWebhookSignatureMismatch):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connectorID,
		enterprise.HRISWebhookReceiptInput{
			EventType:   detectHRISWebhookEventType(r.Header, payload),
			RequestID:   detectHRISWebhookRequestID(r.Header, payload),
			ContentType: strings.TrimSpace(r.Header.Get("Content-Type")),
			Headers:     captureHRISWebhookHeaders(r.Header),
			RawPayload:  string(payload),
			SourceIP:    requestRemoteIP(r),
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrHRISConnectorNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, enterprise.ErrHRISConnectorInactive):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	s.appendAuditLog(
		r,
		receipt.TenantID,
		"enterprise_hris_webhook_received",
		fmt.Sprintf(
			"receipt_id=%s,connector_id=%s,vendor=%s,event_type=%s,request_id=%s",
			receipt.ID,
			receipt.ConnectorID,
			receipt.Vendor,
			receipt.EventType,
			receipt.RequestID,
		),
		"enterprise_webhook",
	)
	if !s.cfg.EnterpriseHRISWebhookReceiptWorkerEnabled {
		_ = s.processEnterpriseHRISWebhookReceipt(r, receipt, true)
	} else {
		s.notifyEnterpriseHRISWebhookReceiptWorker()
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"receipt_id":   receipt.ID,
		"connector_id": receipt.ConnectorID,
		"vendor":       receipt.Vendor,
		"event_type":   receipt.EventType,
		"request_id":   receipt.RequestID,
		"status":       receipt.Status,
		"received_at":  receipt.ReceivedAt,
	})
}

func (s *server) resolveEnterpriseHRISWebhookConnector(connectorID string) (enterprise.HRISConnector, error) {
	if s.enterpriseSvc == nil {
		return enterprise.HRISConnector{}, enterprise.ErrHRISConnectorNotFound
	}

	connector, err := s.enterpriseSvc.GetHRISConnectorByID(connectorID)
	if err != nil {
		return enterprise.HRISConnector{}, err
	}
	if strings.TrimSpace(connector.Status) != "active" {
		return enterprise.HRISConnector{}, enterprise.ErrHRISConnectorInactive
	}
	return connector, nil
}

func (s *server) validateEnterpriseHRISWebhookRequest(
	r *http.Request,
	connector enterprise.HRISConnector,
	payload []byte,
) error {
	switch strings.ToLower(strings.TrimSpace(connector.Vendor)) {
	case "talenta":
		return s.validateTalentaWebhookRequest(r, connector, payload)
	default:
		return nil
	}
}

func (s *server) validateTalentaWebhookRequest(
	r *http.Request,
	connector enterprise.HRISConnector,
	payload []byte,
) error {
	if s.hrisVaultSvc == nil {
		return talenta.ErrWebhookSecretRequired
	}

	clientSecretRef := strings.TrimSpace(connector.WebhookSecretRef)
	if clientSecretRef == "" {
		return talenta.ErrWebhookSecretRequired
	}

	resolvedSecret, err := s.hrisVaultSvc.ResolveSecretRef(clientSecretRef)
	if err != nil {
		return err
	}

	clientID := ""
	if credentialRef := strings.TrimSpace(connector.CredentialRef); credentialRef != "" {
		resolvedCredential, credentialErr := s.hrisVaultSvc.ResolveSecretRef(credentialRef)
		if credentialErr != nil {
			return credentialErr
		}
		parsedCredential, parseErr := talenta.ParsePullCredential(resolvedCredential.Value)
		if parseErr == nil {
			clientID = parsedCredential.WebhookClientID()
		} else {
			clientID = strings.TrimSpace(resolvedCredential.Value)
		}
	}

	proto := strings.TrimSpace(r.Proto)
	if proto == "" {
		proto = "HTTP/1.1"
	}

	return talenta.VerifyWebhookSignature(talenta.WebhookSignatureInput{
		Authorization: strings.TrimSpace(r.Header.Get("Authorization")),
		Date:          strings.TrimSpace(r.Header.Get("Date")),
		Digest:        strings.TrimSpace(r.Header.Get("Digest")),
		Method:        r.Method,
		RequestURI:    r.URL.RequestURI(),
		Proto:         proto,
		Body:          payload,
		ClientID:      strings.TrimSpace(clientID),
		ClientSecret:  strings.TrimSpace(resolvedSecret.Value),
		Now:           time.Now().UTC(),
	})
}

func (s *server) listEnterpriseHRISWebhookReceipts(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if s.enterpriseSvc == nil {
		writeError(w, http.StatusInternalServerError, "enterprise service is not configured")
		return
	}

	options, err := parseEnterpriseHRISWebhookReceiptListOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.enterpriseSvc.RefreshCoreState(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	now := time.Now().UTC()
	rawItems := s.enterpriseSvc.ListAllHRISWebhookReceipts(tenantID, options.ConnectorID)
	baseItems := make([]hrisWebhookReceiptListItem, 0, len(rawItems))
	for i := range rawItems {
		runtime := describeHRISWebhookReceiptQueueState(
			rawItems[i],
			s.cfg.EnterpriseHRISWebhookReceiptWorkerMaxAttempts,
			s.cfg.EnterpriseHRISWebhookReceiptWorkerRetryCooldown,
			s.cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff,
			s.cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout,
			now,
		)
		item := hrisWebhookReceiptListItem{
			HRISWebhookReceipt:   rawItems[i],
			QueueState:           runtime.State,
			NextRetryAt:          runtime.NextRetryAt,
			ProcessingDeadlineAt: runtime.ProcessingDeadlineAt,
			RemainingAttempts:    runtime.RemainingAttempts,
			CooldownRemainingSec: runtime.CooldownRemainingSec,
			StaleInFlight:        runtime.StaleInFlight,
		}
		if !matchesHRISWebhookReceiptListFilters(item, options) {
			continue
		}
		baseItems = append(baseItems, item)
	}

	queueCounts := buildHRISWebhookRuntimeCounts(baseItems, func(item hrisWebhookReceiptListItem) string {
		return item.QueueState
	})
	filteredItems := make([]hrisWebhookReceiptListItem, 0, len(baseItems))
	for i := range baseItems {
		if options.QueueState != "" && normalizeLifecycleToken(baseItems[i].QueueState) != options.QueueState {
			continue
		}
		filteredItems = append(filteredItems, baseItems[i])
	}
	if options.QueueState != "" {
		sortHRISWebhookReceiptRuntimeItems(filteredItems, options.QueueState)
	}

	result := buildHRISWebhookReceiptListResult(filteredItems, options.Offset, options.Limit)
	result.QueueCounts = queueCounts
	writeJSON(w, http.StatusOK, result)
}

func (s *server) listEnterpriseHRISWebhookDLQ(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if s.hrisDLQSvc == nil {
		writeError(w, http.StatusInternalServerError, "hris dlq service is not configured")
		return
	}

	options, err := parseEnterpriseHRISWebhookDLQListOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.hrisDLQSvc.RefreshState(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	now := time.Now().UTC()
	entries := s.hrisDLQSvc.ListEntries(tenantID, options.ConnectorID, 0)
	baseItems := make([]hrisWebhookDLQListItem, 0, len(entries))
	for i := range entries {
		runtime := describeHRISWebhookDLQReplayState(
			entries[i],
			s.cfg.EnterpriseHRISWebhookDLQWorkerMaxAttempts,
			s.cfg.EnterpriseHRISWebhookDLQWorkerRetryCooldown,
			s.cfg.EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff,
			s.cfg.EnterpriseHRISWebhookDLQWorkerProcessingTimeout,
			now,
		)
		item := hrisWebhookDLQListItem{
			DeadLetterEntry:      entries[i],
			ReplayState:          runtime.State,
			NextRetryAt:          runtime.NextRetryAt,
			ProcessingDeadlineAt: runtime.ProcessingDeadlineAt,
			RemainingAttempts:    runtime.RemainingAttempts,
			CooldownRemainingSec: runtime.CooldownRemainingSec,
			StaleInFlight:        runtime.StaleInFlight,
		}
		if !matchesHRISWebhookDLQListFilters(item, options) {
			continue
		}
		baseItems = append(baseItems, item)
	}

	replayCounts := buildHRISWebhookRuntimeCounts(baseItems, func(item hrisWebhookDLQListItem) string {
		return item.ReplayState
	})
	filteredItems := make([]hrisWebhookDLQListItem, 0, len(baseItems))
	for i := range baseItems {
		if options.ReplayState != "" && normalizeLifecycleToken(baseItems[i].ReplayState) != options.ReplayState {
			continue
		}
		filteredItems = append(filteredItems, baseItems[i])
	}
	if options.ReplayState != "" {
		sortHRISWebhookDLQRuntimeItems(filteredItems, options.ReplayState)
	}

	result := buildHRISWebhookDLQListResult(filteredItems, options.Offset, options.Limit)
	result.ReplayCounts = replayCounts
	writeJSON(w, http.StatusOK, result)
}

func (s *server) listEnterpriseHRISWebhookExecutions(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if s.enterpriseSvc == nil {
		writeError(w, http.StatusInternalServerError, "enterprise service is not configured")
		return
	}

	options, err := parseEnterpriseHRISWebhookExecutionListOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.enterpriseSvc.RefreshCoreState(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	rawItems := s.enterpriseSvc.ListAllHRISWebhookExecutions(tenantID)
	now := time.Now().UTC()
	baseItems := make([]hrisWebhookExecutionListItem, 0, len(rawItems))
	for i := range rawItems {
		item := rawItems[i]
		runtime := describeHRISWebhookExecutionQueueState(
			item,
			s.enterpriseHRISWebhookExecutionProcessingTimeout(item),
			now,
		)
		listItem := hrisWebhookExecutionListItem{
			HRISWebhookExecution: item,
			QueueState:           runtime.State,
			NextRetryAt:          runtime.NextRetryAt,
			ProcessingDeadlineAt: runtime.ProcessingDeadlineAt,
			CooldownRemainingSec: runtime.CooldownRemainingSec,
			StaleInFlight:        runtime.StaleInFlight,
		}
		if options.ConnectorID != "" && strings.TrimSpace(item.ConnectorID) != options.ConnectorID {
			continue
		}
		if options.Kind != "" && normalizeLifecycleToken(item.Kind) != options.Kind {
			continue
		}
		if options.ExecutionMode != "" && normalizeLifecycleToken(item.ExecutionMode) != options.ExecutionMode {
			continue
		}
		switch options.ReplayScope {
		case "replayed":
			if strings.TrimSpace(item.ReplaySourceExecutionID) == "" {
				continue
			}
		case "worker_required":
			if item.ReplayRequireWorker == nil || !*item.ReplayRequireWorker {
				continue
			}
		}
		if options.DispatchMode != "" && normalizeLifecycleToken(item.DispatchMode) != options.DispatchMode {
			continue
		}
		if options.TargetStatus != "" && normalizeLifecycleToken(item.TargetStatus) != options.TargetStatus {
			continue
		}
		if options.TargetID != "" && strings.TrimSpace(item.TargetID) != options.TargetID {
			continue
		}
		if options.Query != "" && !matchesHRISWebhookExecutionQuery(listItem, options.Query) {
			continue
		}
		baseItems = append(baseItems, listItem)
	}

	statusCounts := buildHRISWebhookExecutionStatusCounts(baseItems)
	statusFilteredItems := make([]hrisWebhookExecutionListItem, 0, len(baseItems))
	for i := range baseItems {
		if options.Status != "" && normalizeLifecycleToken(baseItems[i].Status) != options.Status {
			continue
		}
		statusFilteredItems = append(statusFilteredItems, baseItems[i])
	}

	queueCounts := buildHRISWebhookRuntimeCounts(statusFilteredItems, func(item hrisWebhookExecutionListItem) string {
		return item.QueueState
	})
	externalQueueSummaries := s.enrichHRISWebhookExecutionExternalQueueTelemetry(statusFilteredItems)
	filteredItems := make([]hrisWebhookExecutionListItem, 0, len(statusFilteredItems))
	for i := range statusFilteredItems {
		if options.QueueState != "" && normalizeLifecycleToken(statusFilteredItems[i].QueueState) != options.QueueState {
			continue
		}
		filteredItems = append(filteredItems, statusFilteredItems[i])
	}
	if options.QueueState != "" {
		sortHRISWebhookExecutionRuntimeItems(filteredItems, options.QueueState)
	}

	result := buildHRISWebhookExecutionListResult(filteredItems, options.Offset, options.Limit)
	result.QueueCounts = queueCounts
	result.StatusCounts = statusCounts
	result.ExternalQueues = externalQueueSummaries
	writeJSON(w, http.StatusOK, result)
}

func (s *server) getEnterpriseHRISWebhookExecution(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if s.enterpriseSvc == nil {
		writeError(w, http.StatusInternalServerError, "enterprise service is not configured")
		return
	}

	executionID := chi.URLParam(r, "executionID")
	if strings.TrimSpace(executionID) == "" {
		writeError(w, http.StatusBadRequest, "execution_id is required")
		return
	}
	if err := s.enterpriseSvc.RefreshCoreState(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	item, err := s.enterpriseSvc.GetHRISWebhookExecution(tenantID, executionID)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrHRISWebhookExecutionNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	runtime := describeHRISWebhookExecutionQueueState(
		item,
		s.enterpriseHRISWebhookExecutionProcessingTimeout(item),
		time.Now().UTC(),
	)
	detailItem := hrisWebhookExecutionListItem{
		HRISWebhookExecution: item,
		QueueState:           runtime.State,
		NextRetryAt:          runtime.NextRetryAt,
		ProcessingDeadlineAt: runtime.ProcessingDeadlineAt,
		CooldownRemainingSec: runtime.CooldownRemainingSec,
		StaleInFlight:        runtime.StaleInFlight,
	}
	detailItems := []hrisWebhookExecutionListItem{detailItem}
	s.enrichHRISWebhookExecutionExternalQueueTelemetry(detailItems)
	writeJSON(w, http.StatusOK, map[string]any{
		"item": detailItems[0],
	})
}

func (s *server) replayEnterpriseHRISWebhookExecution(w http.ResponseWriter, r *http.Request) {
	if s.enterpriseSvc == nil || s.accessSvc == nil || s.hrisNormalizerRegistry == nil {
		writeError(w, http.StatusInternalServerError, "hris webhook execution replay services are not configured")
		return
	}

	executionID := chi.URLParam(r, "executionID")
	var request struct {
		TenantID      string `json:"tenant_id"`
		ExecutionMode string `json:"execution_mode"`
		RequireWorker bool   `json:"require_worker"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	executionMode := normalizeEnterpriseExecutionMode(request.ExecutionMode)
	if executionMode == "" {
		writeError(w, http.StatusBadRequest, "execution_mode must be inline or queued")
		return
	}
	if err := s.enterpriseSvc.RefreshCoreState(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	item, err := s.enterpriseSvc.GetHRISWebhookExecution(tenantID, executionID)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrHRISWebhookExecutionNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	if strings.TrimSpace(item.Status) != enterprise.HRISWebhookExecutionStatusFailed {
		writeError(w, http.StatusConflict, "hris webhook execution replay requires failed execution status")
		return
	}
	if normalizeLifecycleToken(item.Kind) == enterprise.HRISWebhookExecutionKindDLQReplay && s.hrisDLQSvc != nil {
		if err := s.hrisDLQSvc.RefreshState(); err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}
	if err := s.requireEnterpriseHRISWebhookExecutionReplayWorker(item, executionMode, request.RequireWorker); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if executionMode == enterpriseExecutionModeQueued {
		if existing, ok := s.enterpriseSvc.FindActiveHRISWebhookReplayExecution(item.TenantID, item.ID); ok {
			replayConflictErr := &enterprise.HRISWebhookExecutionReplayConflictError{
				ExistingExecution: existing,
			}
			writeEnterpriseHRISWebhookExecutionReplayConflict(w, replayConflictErr, replayConflictErr)
			return
		}
	}

	response, err := s.replayFailedEnterpriseHRISWebhookExecution(r, item, executionMode, request.RequireWorker)
	if err != nil {
		var receiptConflictErr *hrisWebhookReceiptProcessConflictError
		var replayConflictErr *enterprise.HRISWebhookExecutionReplayConflictError
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrHRISWebhookReceiptNotFound),
			errors.Is(err, enterprise.ErrHRISWebhookExecutionNotFound),
			errors.Is(err, hris.ErrDLQEntryNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.As(err, &replayConflictErr):
			writeEnterpriseHRISWebhookExecutionReplayConflict(w, err, replayConflictErr)
		case errors.As(err, &receiptConflictErr),
			errors.Is(err, hris.ErrDLQEntryReplayInFlight),
			errors.Is(err, hris.ErrDLQEntryReplayNotAllowed):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	status := http.StatusOK
	if executionMode == enterpriseExecutionModeQueued {
		status = http.StatusAccepted
	}
	writeJSON(w, status, response)
}

func writeEnterpriseHRISWebhookExecutionReplayConflict(
	w http.ResponseWriter,
	err error,
	replayConflictErr *enterprise.HRISWebhookExecutionReplayConflictError,
) {
	payload := map[string]any{
		"error": err.Error(),
	}
	if replayConflictErr != nil && strings.TrimSpace(replayConflictErr.ExistingExecution.ID) != "" {
		payload["existing_execution_id"] = replayConflictErr.ExistingExecution.ID
		payload["existing_execution"] = replayConflictErr.ExistingExecution
	}
	writeJSON(w, http.StatusConflict, payload)
}

func (s *server) processBatchEnterpriseHRISWebhookReceipts(w http.ResponseWriter, r *http.Request) {
	if s.enterpriseSvc == nil || s.accessSvc == nil || s.hrisNormalizerRegistry == nil {
		writeError(w, http.StatusInternalServerError, "hris webhook processing services are not configured")
		return
	}

	var request struct {
		TenantID      string   `json:"tenant_id"`
		ReceiptIDs    []string `json:"receipt_ids"`
		ExecutionMode string   `json:"execution_mode"`
		RequireWorker bool     `json:"require_worker"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	receiptIDs := normalizeEnterpriseHRISWebhookReceiptBatchProcessIDs(request.ReceiptIDs)
	if len(receiptIDs) == 0 {
		writeError(w, http.StatusBadRequest, "receipt_ids must contain at least one receipt")
		return
	}
	if len(receiptIDs) > 50 {
		writeError(w, http.StatusBadRequest, "receipt_ids must contain at most 50 receipts")
		return
	}
	executionMode := normalizeEnterpriseExecutionMode(request.ExecutionMode)
	if executionMode == "" {
		writeError(w, http.StatusBadRequest, "execution_mode must be inline or queued")
		return
	}
	if err := s.requireEnterpriseHRISWebhookReceiptWorker(executionMode, request.RequireWorker); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	maxAttempts := s.cfg.EnterpriseHRISWebhookReceiptWorkerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryCooldown := s.cfg.EnterpriseHRISWebhookReceiptWorkerRetryCooldown
	retryMaxBackoff := s.cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff
	processingTimeout := s.cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout
	now := time.Now().UTC()

	result := hrisWebhookReceiptBatchProcessResult{
		TenantID:      tenantID,
		TotalReceipts: len(receiptIDs),
		ExecutionMode: executionMode,
		Items:         make([]hrisWebhookReceiptBatchProcessItem, 0, len(receiptIDs)),
		UpdatedAt:     now,
	}
	if executionMode == enterpriseExecutionModeQueued {
		result.DispatchMode = s.plannedEnterpriseHRISWebhookReceiptDispatchMode()
	}
	for i := range receiptIDs {
		receiptID := receiptIDs[i]
		receipt, err := s.enterpriseSvc.GetHRISWebhookReceipt(tenantID, receiptID)
		if err != nil {
			result.Failed += 1
			result.Items = append(result.Items, hrisWebhookReceiptBatchProcessItem{
				ReceiptID: receiptID,
				Status:    "failed",
				Reason:    "not_found",
				Error:     err.Error(),
			})
			continue
		}

		claimed, skipReason, err := s.enterpriseSvc.ClaimHRISWebhookReceiptForProcessingWithBackoff(
			tenantID,
			receiptID,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			processingTimeout,
			now,
		)
		if err != nil {
			result.Failed += 1
			result.Items = append(result.Items, hrisWebhookReceiptBatchProcessItem{
				ReceiptID: receiptID,
				Status:    "failed",
				Reason:    "claim_failed",
				Error:     err.Error(),
			})
			continue
		}
		switch skipReason {
		case "":
		case enterprise.HRISWebhookReceiptClaimReasonAttemptLimit,
			enterprise.HRISWebhookReceiptClaimReasonCooldown,
			enterprise.HRISWebhookReceiptClaimReasonInFlight,
			enterprise.HRISWebhookReceiptClaimReasonNotQueueable:
			result.Skipped += 1
			result.Items = append(result.Items, hrisWebhookReceiptBatchProcessItem{
				ReceiptID: receiptID,
				Status:    "skipped",
				Reason:    skipReason,
				Item:      pointerToHRISWebhookReceipt(receipt),
			})
			continue
		default:
			result.Skipped += 1
			result.Items = append(result.Items, hrisWebhookReceiptBatchProcessItem{
				ReceiptID: receiptID,
				Status:    "skipped",
				Reason:    skipReason,
				Item:      pointerToHRISWebhookReceipt(receipt),
			})
			continue
		}

		if executionMode == enterpriseExecutionModeQueued {
			executionID, err := s.dispatchQueuedEnterpriseHRISWebhookReceipt(
				r,
				receipt,
				claimed,
				claimed.AttemptCount >= maxAttempts,
				"enterprise_sync",
				"",
				nil,
			)
			if err != nil {
				current := claimed
				if refreshed, refreshErr := s.enterpriseSvc.GetHRISWebhookReceipt(tenantID, receiptID); refreshErr == nil {
					current = refreshed
				}
				result.Failed += 1
				result.Items = append(result.Items, hrisWebhookReceiptBatchProcessItem{
					ReceiptID: receiptID,
					Status:    "failed",
					Reason:    "queue_dispatch_failed",
					Error:     err.Error(),
					Item:      pointerToHRISWebhookReceipt(current),
				})
				continue
			}
			result.Queued += 1
			result.Items = append(result.Items, hrisWebhookReceiptBatchProcessItem{
				ReceiptID:   receiptID,
				Status:      "queued",
				ExecutionID: executionID,
				Item:        pointerToHRISWebhookReceipt(claimed),
			})
			s.appendAuditLog(
				r,
				tenantID,
				"enterprise_hris_webhook_receipt_processing_queued",
				fmt.Sprintf(
					"receipt_id=%s,connector_id=%s,vendor=%s,event_type=%s,request_id=%s",
					claimed.ID,
					claimed.ConnectorID,
					claimed.Vendor,
					claimed.EventType,
					claimed.RequestID,
				),
				"enterprise_sync",
			)
			continue
		}

		recordDLQ := claimed.AttemptCount >= maxAttempts
		processErr := s.processEnterpriseHRISWebhookReceipt(r, claimed, recordDLQ)
		updated, updatedErr := s.enterpriseSvc.GetHRISWebhookReceipt(tenantID, receiptID)
		if updatedErr != nil {
			result.Failed += 1
			result.Items = append(result.Items, hrisWebhookReceiptBatchProcessItem{
				ReceiptID: receiptID,
				Status:    "failed",
				Reason:    "refresh_failed",
				Error:     updatedErr.Error(),
			})
			continue
		}

		updatedStatus := strings.TrimSpace(updated.Status)
		switch updatedStatus {
		case "processed":
			result.Processed += 1
			result.Items = append(result.Items, hrisWebhookReceiptBatchProcessItem{
				ReceiptID: receiptID,
				Status:    "processed",
				Item:      pointerToHRISWebhookReceipt(updated),
			})
		case "skipped":
			result.Skipped += 1
			result.Items = append(result.Items, hrisWebhookReceiptBatchProcessItem{
				ReceiptID: receiptID,
				Status:    "skipped",
				Reason:    updated.LastError,
				Item:      pointerToHRISWebhookReceipt(updated),
			})
		case "dlq":
			result.DLQ += 1
			result.Items = append(result.Items, hrisWebhookReceiptBatchProcessItem{
				ReceiptID: receiptID,
				Status:    "dlq",
				Reason:    updated.LastError,
				Item:      pointerToHRISWebhookReceipt(updated),
			})
		default:
			result.Failed += 1
			result.Items = append(result.Items, hrisWebhookReceiptBatchProcessItem{
				ReceiptID: receiptID,
				Status:    "failed",
				Reason:    updatedStatus,
				Error:     updated.LastError,
				Item:      pointerToHRISWebhookReceipt(updated),
			})
		}
		if processErr != nil && updatedStatus == "processed" {
			result.Failed += 1
			result.Processed -= 1
			result.Items[len(result.Items)-1].Status = "failed"
			result.Items[len(result.Items)-1].Reason = "process_failed"
			result.Items[len(result.Items)-1].Error = processErr.Error()
		}
	}

	status := http.StatusOK
	if executionMode == enterpriseExecutionModeQueued {
		status = http.StatusAccepted
	}
	writeJSON(w, status, result)
}

func (s *server) processEnterpriseHRISWebhookReceiptEntry(w http.ResponseWriter, r *http.Request) {
	if s.enterpriseSvc == nil || s.accessSvc == nil || s.hrisNormalizerRegistry == nil {
		writeError(w, http.StatusInternalServerError, "hris webhook processing services are not configured")
		return
	}

	receiptID := chi.URLParam(r, "receiptID")
	var request struct {
		TenantID      string `json:"tenant_id"`
		ExecutionMode string `json:"execution_mode"`
		RequireWorker bool   `json:"require_worker"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	executionMode := normalizeEnterpriseExecutionMode(request.ExecutionMode)
	if executionMode == "" {
		writeError(w, http.StatusBadRequest, "execution_mode must be inline or queued")
		return
	}
	if err := s.requireEnterpriseHRISWebhookReceiptWorker(executionMode, request.RequireWorker); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	var (
		updated     enterprise.HRISWebhookReceipt
		executionID string
		err         error
	)
	if executionMode == enterpriseExecutionModeQueued {
		updated, executionID, err = s.queueSingleEnterpriseHRISWebhookReceipt(
			r,
			tenantID,
			receiptID,
			"enterprise_sync",
			"",
			nil,
		)
	} else {
		updated, err = s.processSingleEnterpriseHRISWebhookReceipt(r, tenantID, receiptID)
	}
	if err != nil {
		var conflictErr *hrisWebhookReceiptProcessConflictError
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrHRISWebhookReceiptNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.As(err, &conflictErr):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	status := http.StatusOK
	if executionMode == enterpriseExecutionModeQueued {
		status = http.StatusAccepted
	}
	response := map[string]any{
		"item":           updated,
		"execution_mode": executionMode,
	}
	if executionMode == enterpriseExecutionModeQueued {
		response["dispatch_mode"] = s.plannedEnterpriseHRISWebhookReceiptDispatchMode()
	}
	if executionID != "" {
		response["execution_id"] = executionID
	}
	writeJSON(w, status, response)
}

func (s *server) listEnterpriseHRISPullStates(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if s.hrisPullStateSvc == nil {
		writeError(w, http.StatusInternalServerError, "hris pull state service is not configured")
		return
	}
	if err := s.hrisPullStateSvc.RefreshState(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.hrisPullStateSvc.ListStates(tenantID),
	})
}

func (s *server) replayEnterpriseHRISWebhookDLQ(w http.ResponseWriter, r *http.Request) {
	entryID := chi.URLParam(r, "entryID")
	if s.hrisDLQSvc == nil || s.enterpriseSvc == nil || s.accessSvc == nil || s.hrisNormalizerRegistry == nil {
		writeError(w, http.StatusInternalServerError, "hris webhook replay services are not configured")
		return
	}
	executionMode := normalizeEnterpriseExecutionMode(r.URL.Query().Get("execution_mode"))
	if executionMode == "" {
		writeError(w, http.StatusBadRequest, "execution_mode must be inline or queued")
		return
	}
	requireWorker, err := parseOptionalEnterpriseBool(r.URL.Query().Get("require_worker"), "require_worker")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	entry, err := s.hrisDLQSvc.GetEntry(entryID)
	if err != nil {
		if errors.Is(err, hris.ErrDLQEntryNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, entry.TenantID)
	if !ok {
		return
	}
	if strings.TrimSpace(entry.ReceiptID) == "" {
		writeError(w, http.StatusConflict, "hris dlq entry cannot be replayed without receipt_id")
		return
	}
	if err := s.requireEnterpriseHRISWebhookDLQWorker(executionMode, requireWorker); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	var (
		updated     hris.DeadLetterEntry
		executionID string
	)
	if executionMode == enterpriseExecutionModeQueued {
		updated, executionID, err = s.queueEnterpriseHRISWebhookDLQEntry(
			r,
			tenantID,
			entry.ID,
			"enterprise_sync",
			"",
			nil,
		)
	} else {
		updated, err = s.processEnterpriseHRISWebhookDLQEntry(r, tenantID, entry.ID, "enterprise_sync")
	}
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrHRISConnectorNotFound),
			errors.Is(err, hris.ErrDLQEntryNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, hris.ErrDLQEntryReplayInFlight),
			errors.Is(err, hris.ErrDLQEntryReplayNotAllowed):
			writeError(w, http.StatusConflict, err.Error())
		case strings.Contains(err.Error(), "cannot be replayed without receipt_id"):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	status := http.StatusOK
	if executionMode == enterpriseExecutionModeQueued {
		status = http.StatusAccepted
	}
	response := map[string]any{
		"item":           updated,
		"execution_mode": executionMode,
	}
	if executionMode == enterpriseExecutionModeQueued {
		response["dispatch_mode"] = s.plannedEnterpriseHRISWebhookDLQDispatchMode()
	}
	if executionID != "" {
		response["execution_id"] = executionID
	}
	writeJSON(w, status, response)
}

func (s *server) processSingleEnterpriseHRISWebhookReceipt(
	r *http.Request,
	tenantID string,
	receiptID string,
) (enterprise.HRISWebhookReceipt, error) {
	if s.enterpriseSvc == nil || s.accessSvc == nil || s.hrisNormalizerRegistry == nil {
		return enterprise.HRISWebhookReceipt{}, errors.New("hris webhook processing services are not configured")
	}

	claimed, maxAttempts, err := s.claimEnterpriseHRISWebhookReceiptForManualProcessing(tenantID, receiptID)
	if err != nil {
		return enterprise.HRISWebhookReceipt{}, err
	}
	recordDLQ := claimed.AttemptCount >= maxAttempts
	processErr := s.processEnterpriseHRISWebhookReceipt(r, claimed, recordDLQ)
	updated, updatedErr := s.enterpriseSvc.GetHRISWebhookReceipt(tenantID, receiptID)
	if updatedErr != nil {
		return enterprise.HRISWebhookReceipt{}, updatedErr
	}
	if processErr != nil && strings.TrimSpace(updated.Status) == "processed" {
		return updated, processErr
	}
	return updated, nil
}

func (s *server) queueSingleEnterpriseHRISWebhookReceipt(
	r *http.Request,
	tenantID string,
	receiptID string,
	auditSource string,
	replaySourceExecutionID string,
	replayRequireWorker *bool,
) (enterprise.HRISWebhookReceipt, string, error) {
	if s.enterpriseSvc == nil || s.accessSvc == nil || s.hrisNormalizerRegistry == nil {
		return enterprise.HRISWebhookReceipt{}, "", errors.New("hris webhook processing services are not configured")
	}

	original, err := s.enterpriseSvc.GetHRISWebhookReceipt(tenantID, receiptID)
	if err != nil {
		return enterprise.HRISWebhookReceipt{}, "", err
	}
	claimed, maxAttempts, err := s.claimEnterpriseHRISWebhookReceiptForManualProcessing(tenantID, receiptID)
	if err != nil {
		return enterprise.HRISWebhookReceipt{}, "", err
	}
	recordDLQ := claimed.AttemptCount >= maxAttempts
	executionID, err := s.dispatchQueuedEnterpriseHRISWebhookReceipt(
		r,
		original,
		claimed,
		recordDLQ,
		auditSource,
		replaySourceExecutionID,
		replayRequireWorker,
	)
	if err != nil {
		return enterprise.HRISWebhookReceipt{}, "", err
	}
	s.appendAuditLog(
		r,
		tenantID,
		"enterprise_hris_webhook_receipt_processing_queued",
		fmt.Sprintf(
			"receipt_id=%s,connector_id=%s,vendor=%s,event_type=%s,request_id=%s",
			claimed.ID,
			claimed.ConnectorID,
			claimed.Vendor,
			claimed.EventType,
			claimed.RequestID,
		),
		auditSource,
	)
	return claimed, executionID, nil
}

func (s *server) claimEnterpriseHRISWebhookReceiptForManualProcessing(
	tenantID string,
	receiptID string,
) (enterprise.HRISWebhookReceipt, int, error) {
	if _, err := s.enterpriseSvc.GetHRISWebhookReceipt(tenantID, receiptID); err != nil {
		return enterprise.HRISWebhookReceipt{}, 0, err
	}

	maxAttempts := s.cfg.EnterpriseHRISWebhookReceiptWorkerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	claimed, skipReason, err := s.enterpriseSvc.ClaimHRISWebhookReceiptForProcessingWithBackoff(
		tenantID,
		receiptID,
		maxAttempts,
		s.cfg.EnterpriseHRISWebhookReceiptWorkerRetryCooldown,
		s.cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff,
		s.cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout,
		time.Now().UTC(),
	)
	if err != nil {
		return enterprise.HRISWebhookReceipt{}, 0, err
	}
	if skipReason != "" {
		return enterprise.HRISWebhookReceipt{}, 0, &hrisWebhookReceiptProcessConflictError{reason: skipReason}
	}
	return claimed, maxAttempts, nil
}

func (s *server) processQueuedEnterpriseHRISWebhookReceipt(
	receipt enterprise.HRISWebhookReceipt,
	recordDLQ bool,
	executionID string,
) {
	s.markHRISWebhookExecutionRunning(receipt.TenantID, executionID)
	if err := s.processEnterpriseHRISWebhookReceipt(nil, receipt, recordDLQ); err != nil {
		s.completeHRISWebhookReceiptExecution(receipt, executionID, err)
		s.loggerOrDefault().Error(
			"queued enterprise hris webhook receipt processing failed",
			"tenant_id", receipt.TenantID,
			"receipt_id", receipt.ID,
			"connector_id", receipt.ConnectorID,
			"vendor", receipt.Vendor,
			"event_type", receipt.EventType,
			"request_id", receipt.RequestID,
			"err", err,
		)
		return
	}
	s.completeHRISWebhookReceiptExecution(receipt, executionID, nil)
}

func (s *server) dispatchQueuedEnterpriseHRISWebhookReceipt(
	r *http.Request,
	original enterprise.HRISWebhookReceipt,
	receipt enterprise.HRISWebhookReceipt,
	recordDLQ bool,
	auditSource string,
	replaySourceExecutionID string,
	replayRequireWorker *bool,
) (string, error) {
	nextAuditSource := strings.TrimSpace(auditSource)
	if nextAuditSource == "" {
		nextAuditSource = "enterprise_sync"
	}
	dispatchMode := s.plannedEnterpriseHRISWebhookReceiptDispatchMode()
	executionID, err := s.createHRISWebhookExecutionRecord(r, enterprise.HRISWebhookExecutionInput{
		TenantID:                receipt.TenantID,
		Kind:                    enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:                receipt.ID,
		ReceiptID:               receipt.ID,
		ConnectorID:             receipt.ConnectorID,
		Vendor:                  receipt.Vendor,
		RequestID:               receipt.RequestID,
		EventType:               receipt.EventType,
		AuditSource:             nextAuditSource,
		ExecutionMode:           enterpriseExecutionModeQueued,
		DispatchMode:            dispatchMode,
		TargetStatus:            receipt.Status,
		RequestedBy:             enterpriseHRISWebhookExecutionRequestedBy(r),
		ReplaySourceExecutionID: strings.TrimSpace(replaySourceExecutionID),
		ReplayRequireWorker:     replayRequireWorker,
	})
	if err != nil {
		return "", s.restoreQueuedEnterpriseHRISWebhookReceiptDispatch(original, err)
	}
	if dispatchMode == enterpriseHRISWebhookExecutionDispatchModeWorkerTick {
		s.enqueueEnterpriseHRISWebhookExecution(
			enterpriseHRISWebhookReceiptExecutionQueue,
			executionID,
			receipt.TenantID,
			enterprise.HRISWebhookExecutionKindReceiptProcess,
		)
		s.notifyEnterpriseHRISWebhookReceiptWorker()
		return executionID, nil
	}
	go s.processQueuedEnterpriseHRISWebhookReceipt(receipt, recordDLQ, executionID)
	return executionID, nil
}

func (s *server) replayFailedEnterpriseHRISWebhookExecution(
	r *http.Request,
	item enterprise.HRISWebhookExecution,
	executionMode string,
	requireWorker bool,
) (hrisWebhookExecutionReplayResponse, error) {
	response := hrisWebhookExecutionReplayResponse{
		SourceExecutionID: item.ID,
		ExecutionMode:     executionMode,
	}
	if executionMode == enterpriseExecutionModeQueued {
		switch normalizeLifecycleToken(item.Kind) {
		case enterprise.HRISWebhookExecutionKindReceiptProcess:
			response.DispatchMode = s.plannedEnterpriseHRISWebhookReceiptDispatchMode()
		case enterprise.HRISWebhookExecutionKindDLQReplay:
			response.DispatchMode = s.plannedEnterpriseHRISWebhookDLQDispatchMode()
		}
	}

	switch normalizeLifecycleToken(item.Kind) {
	case enterprise.HRISWebhookExecutionKindReceiptProcess:
		if executionMode == enterpriseExecutionModeQueued {
			receipt, executionID, err := s.queueSingleEnterpriseHRISWebhookReceipt(
				r,
				item.TenantID,
				item.TargetID,
				enterpriseHRISWebhookExecutionReplayAuditSource,
				item.ID,
				pointerToBool(requireWorker),
			)
			if err != nil {
				return hrisWebhookExecutionReplayResponse{}, err
			}
			response.ExecutionID = executionID
			response.ReceiptItem = pointerToHRISWebhookReceipt(receipt)
			execution, err := s.enterpriseSvc.GetHRISWebhookExecution(item.TenantID, executionID)
			if err != nil {
				return hrisWebhookExecutionReplayResponse{}, err
			}
			response.Execution = &execution
			response.DispatchMode = execution.DispatchMode
		} else {
			receipt, err := s.processSingleEnterpriseHRISWebhookReceipt(r, item.TenantID, item.TargetID)
			if err != nil {
				return hrisWebhookExecutionReplayResponse{}, err
			}
			response.ReceiptItem = pointerToHRISWebhookReceipt(receipt)
		}
	case enterprise.HRISWebhookExecutionKindDLQReplay:
		if executionMode == enterpriseExecutionModeQueued {
			entry, executionID, err := s.queueEnterpriseHRISWebhookDLQEntry(
				r,
				item.TenantID,
				item.TargetID,
				enterpriseHRISWebhookExecutionReplayAuditSource,
				item.ID,
				pointerToBool(requireWorker),
			)
			if err != nil {
				return hrisWebhookExecutionReplayResponse{}, err
			}
			response.ExecutionID = executionID
			response.DLQItem = pointerToHRISWebhookDLQEntry(entry)
			execution, err := s.enterpriseSvc.GetHRISWebhookExecution(item.TenantID, executionID)
			if err != nil {
				return hrisWebhookExecutionReplayResponse{}, err
			}
			response.Execution = &execution
			response.DispatchMode = execution.DispatchMode
		} else {
			entry, err := s.processEnterpriseHRISWebhookDLQEntry(
				r,
				item.TenantID,
				item.TargetID,
				enterpriseHRISWebhookExecutionReplayAuditSource,
			)
			if err != nil {
				return hrisWebhookExecutionReplayResponse{}, err
			}
			response.DLQItem = pointerToHRISWebhookDLQEntry(entry)
		}
	default:
		return hrisWebhookExecutionReplayResponse{}, errors.New("hris webhook execution kind cannot be replayed")
	}

	s.appendAuditLog(
		r,
		item.TenantID,
		"enterprise_hris_webhook_execution_replayed",
		fmt.Sprintf(
			"source_execution_id=%s,kind=%s,target_id=%s,execution_mode=%s,replayed_execution_id=%s",
			item.ID,
			item.Kind,
			item.TargetID,
			executionMode,
			response.ExecutionID,
		),
		enterpriseHRISWebhookExecutionReplayAuditSource,
	)
	return response, nil
}

func (s *server) plannedEnterpriseHRISWebhookReceiptDispatchMode() string {
	if s != nil && s.cfg.EnterpriseHRISWebhookReceiptWorkerEnabled {
		return enterpriseHRISWebhookExecutionDispatchModeWorkerTick
	}
	return enterpriseHRISWebhookExecutionDispatchModeGoroutineFallback
}

func (s *server) replayBatchEnterpriseHRISWebhookDLQ(w http.ResponseWriter, r *http.Request) {
	if s.hrisDLQSvc == nil || s.enterpriseSvc == nil || s.accessSvc == nil || s.hrisNormalizerRegistry == nil {
		writeError(w, http.StatusInternalServerError, "hris webhook replay services are not configured")
		return
	}

	var request struct {
		TenantID      string   `json:"tenant_id"`
		EntryIDs      []string `json:"entry_ids"`
		ExecutionMode string   `json:"execution_mode"`
		RequireWorker bool     `json:"require_worker"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	entryIDs := normalizeEnterpriseHRISWebhookDLQBatchReplayIDs(request.EntryIDs)
	if len(entryIDs) == 0 {
		writeError(w, http.StatusBadRequest, "entry_ids must contain at least one entry")
		return
	}
	if len(entryIDs) > 50 {
		writeError(w, http.StatusBadRequest, "entry_ids must contain at most 50 entries")
		return
	}
	executionMode := normalizeEnterpriseExecutionMode(request.ExecutionMode)
	if executionMode == "" {
		writeError(w, http.StatusBadRequest, "execution_mode must be inline or queued")
		return
	}
	if err := s.requireEnterpriseHRISWebhookDLQWorker(executionMode, request.RequireWorker); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	result := hrisWebhookDLQBatchReplayResult{
		TenantID:      tenantID,
		TotalEntries:  len(entryIDs),
		ExecutionMode: executionMode,
		Items:         make([]hrisWebhookDLQBatchReplayItem, 0, len(entryIDs)),
		UpdatedAt:     time.Now().UTC(),
	}
	if executionMode == enterpriseExecutionModeQueued {
		result.DispatchMode = s.plannedEnterpriseHRISWebhookDLQDispatchMode()
	}
	for i := range entryIDs {
		entryID := entryIDs[i]
		entry, err := s.hrisDLQSvc.GetEntry(entryID)
		if err != nil {
			result.Failed += 1
			result.Items = append(result.Items, hrisWebhookDLQBatchReplayItem{
				EntryID: entryID,
				Status:  "failed",
				Reason:  "not_found",
				Error:   err.Error(),
			})
			continue
		}
		if strings.TrimSpace(entry.TenantID) != tenantID {
			result.Skipped += 1
			result.Items = append(result.Items, hrisWebhookDLQBatchReplayItem{
				EntryID: entryID,
				Status:  "skipped",
				Reason:  "tenant_mismatch",
				Error:   "hris dlq entry belongs to another tenant",
			})
			continue
		}

		if executionMode == enterpriseExecutionModeQueued {
			updated, executionID, err := s.queueEnterpriseHRISWebhookDLQEntry(
				r,
				tenantID,
				entryID,
				"enterprise_sync_batch",
				"",
				nil,
			)
			if err == nil {
				result.Queued += 1
				result.Items = append(result.Items, hrisWebhookDLQBatchReplayItem{
					EntryID:     entryID,
					Status:      "queued",
					ExecutionID: executionID,
					Item:        pointerToHRISWebhookDLQEntry(updated),
				})
				continue
			}

			switch {
			case errors.Is(err, hris.ErrDLQEntryReplayInFlight):
				result.Skipped += 1
				result.Items = append(result.Items, hrisWebhookDLQBatchReplayItem{
					EntryID: entryID,
					Status:  "skipped",
					Reason:  hris.DLQEntryClaimReasonInFlight,
					Error:   err.Error(),
				})
			case errors.Is(err, hris.ErrDLQEntryReplayNotAllowed):
				result.Skipped += 1
				result.Items = append(result.Items, hrisWebhookDLQBatchReplayItem{
					EntryID: entryID,
					Status:  "skipped",
					Reason:  "not_replayable",
					Error:   err.Error(),
				})
			default:
				result.Failed += 1
				result.Items = append(result.Items, hrisWebhookDLQBatchReplayItem{
					EntryID: entryID,
					Status:  "failed",
					Reason:  "replay_failed",
					Error:   err.Error(),
				})
			}
			continue
		}

		updated, err := s.processEnterpriseHRISWebhookDLQEntry(r, tenantID, entryID, "enterprise_sync_batch")
		if err == nil {
			result.Replayed += 1
			result.Items = append(result.Items, hrisWebhookDLQBatchReplayItem{
				EntryID: entryID,
				Status:  "replayed",
				Item:    pointerToHRISWebhookDLQEntry(updated),
			})
			continue
		}

		switch {
		case errors.Is(err, hris.ErrDLQEntryReplayInFlight):
			result.Skipped += 1
			result.Items = append(result.Items, hrisWebhookDLQBatchReplayItem{
				EntryID: entryID,
				Status:  "skipped",
				Reason:  hris.DLQEntryClaimReasonInFlight,
				Error:   err.Error(),
			})
		case errors.Is(err, hris.ErrDLQEntryReplayNotAllowed):
			result.Skipped += 1
			result.Items = append(result.Items, hrisWebhookDLQBatchReplayItem{
				EntryID: entryID,
				Status:  "skipped",
				Reason:  "not_replayable",
				Error:   err.Error(),
			})
		default:
			result.Failed += 1
			result.Items = append(result.Items, hrisWebhookDLQBatchReplayItem{
				EntryID: entryID,
				Status:  "failed",
				Reason:  "replay_failed",
				Error:   err.Error(),
			})
		}
	}

	status := http.StatusOK
	if executionMode == enterpriseExecutionModeQueued {
		status = http.StatusAccepted
	}
	writeJSON(w, status, result)
}

func (s *server) processEnterpriseHRISWebhookDLQEntry(
	r *http.Request,
	tenantID string,
	entryID string,
	auditSource string,
) (hris.DeadLetterEntry, error) {
	if s.hrisDLQSvc == nil || s.enterpriseSvc == nil || s.accessSvc == nil || s.hrisNormalizerRegistry == nil {
		return hris.DeadLetterEntry{}, errors.New("hris webhook replay services are not configured")
	}

	nextTenantID, _, claimed, err := s.claimEnterpriseHRISWebhookDLQEntryForManualReplay(tenantID, entryID)
	if err != nil {
		return hris.DeadLetterEntry{}, err
	}
	return s.replayEnterpriseHRISWebhookDLQClaimedEntry(r, nextTenantID, claimed, auditSource)
}

func (s *server) queueEnterpriseHRISWebhookDLQEntry(
	r *http.Request,
	tenantID string,
	entryID string,
	auditSource string,
	replaySourceExecutionID string,
	replayRequireWorker *bool,
) (hris.DeadLetterEntry, string, error) {
	if s.hrisDLQSvc == nil || s.enterpriseSvc == nil || s.accessSvc == nil || s.hrisNormalizerRegistry == nil {
		return hris.DeadLetterEntry{}, "", errors.New("hris webhook replay services are not configured")
	}

	nextTenantID, original, claimed, err := s.claimEnterpriseHRISWebhookDLQEntryForManualReplay(tenantID, entryID)
	if err != nil {
		return hris.DeadLetterEntry{}, "", err
	}
	executionID, err := s.dispatchQueuedEnterpriseHRISWebhookDLQEntry(
		r,
		nextTenantID,
		original,
		claimed,
		auditSource,
		replaySourceExecutionID,
		replayRequireWorker,
	)
	if err != nil {
		return hris.DeadLetterEntry{}, "", err
	}
	s.appendAuditLog(
		r,
		nextTenantID,
		"enterprise_hris_webhook_dlq_replay_queued",
		fmt.Sprintf(
			"entry_id=%s,receipt_id=%s,connector_id=%s,vendor=%s,event_type=%s,request_id=%s,failure_stage=%s",
			claimed.ID,
			claimed.ReceiptID,
			claimed.ConnectorID,
			claimed.Vendor,
			claimed.EventType,
			claimed.RequestID,
			claimed.FailureStage,
		),
		auditSource,
	)
	return claimed, executionID, nil
}

func (s *server) claimEnterpriseHRISWebhookDLQEntryForManualReplay(
	tenantID string,
	entryID string,
) (string, hris.DeadLetterEntry, hris.DeadLetterEntry, error) {
	entry, err := s.hrisDLQSvc.GetEntry(entryID)
	if err != nil {
		return "", hris.DeadLetterEntry{}, hris.DeadLetterEntry{}, err
	}
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		nextTenantID = strings.TrimSpace(entry.TenantID)
	}
	if nextTenantID == "" {
		return "", hris.DeadLetterEntry{}, hris.DeadLetterEntry{}, enterprise.ErrTenantIDRequired
	}
	if strings.TrimSpace(entry.ReceiptID) == "" {
		return "", hris.DeadLetterEntry{}, hris.DeadLetterEntry{}, errors.New("hris dlq entry cannot be replayed without receipt_id")
	}

	processingTimeout := s.cfg.EnterpriseHRISWebhookDLQWorkerProcessingTimeout
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	claimed, skipReason, err := s.hrisDLQSvc.ClaimEntryForReplay(entry.ID, 0, 0, processingTimeout, time.Now().UTC())
	if err != nil {
		return "", hris.DeadLetterEntry{}, hris.DeadLetterEntry{}, err
	}
	switch skipReason {
	case "":
		return nextTenantID, entry, claimed, nil
	case hris.DLQEntryClaimReasonInFlight:
		return "", hris.DeadLetterEntry{}, hris.DeadLetterEntry{}, hris.ErrDLQEntryReplayInFlight
	case hris.DLQEntryClaimReasonNotReplayable:
		return "", hris.DeadLetterEntry{}, hris.DeadLetterEntry{}, hris.ErrDLQEntryReplayNotAllowed
	default:
		return "", hris.DeadLetterEntry{}, hris.DeadLetterEntry{}, hris.ErrDLQEntryReplayNotAllowed
	}
}

func (s *server) replayQueuedEnterpriseHRISWebhookDLQEntry(
	tenantID string,
	entry hris.DeadLetterEntry,
	auditSource string,
	executionID string,
) {
	s.markHRISWebhookExecutionRunning(tenantID, executionID)
	updated, err := s.replayEnterpriseHRISWebhookDLQClaimedEntry(nil, tenantID, entry, auditSource)
	if err != nil {
		s.completeHRISWebhookDLQExecution(tenantID, entry, executionID, err)
		s.loggerOrDefault().Error(
			"queued enterprise hris webhook dlq replay failed",
			"tenant_id", tenantID,
			"entry_id", entry.ID,
			"receipt_id", entry.ReceiptID,
			"connector_id", entry.ConnectorID,
			"vendor", entry.Vendor,
			"event_type", entry.EventType,
			"request_id", entry.RequestID,
			"failure_stage", entry.FailureStage,
			"err", err,
		)
		return
	}
	s.completeHRISWebhookDLQExecutionSuccess(tenantID, updated, executionID)
}

func (s *server) dispatchQueuedEnterpriseHRISWebhookDLQEntry(
	r *http.Request,
	tenantID string,
	original hris.DeadLetterEntry,
	entry hris.DeadLetterEntry,
	auditSource string,
	replaySourceExecutionID string,
	replayRequireWorker *bool,
) (string, error) {
	dispatchMode := s.plannedEnterpriseHRISWebhookDLQDispatchMode()
	executionID, err := s.createHRISWebhookExecutionRecord(r, enterprise.HRISWebhookExecutionInput{
		TenantID:                tenantID,
		Kind:                    enterprise.HRISWebhookExecutionKindDLQReplay,
		TargetID:                entry.ID,
		ReceiptID:               entry.ReceiptID,
		ConnectorID:             entry.ConnectorID,
		Vendor:                  entry.Vendor,
		RequestID:               entry.RequestID,
		EventType:               entry.EventType,
		FailureStage:            entry.FailureStage,
		AuditSource:             auditSource,
		ExecutionMode:           enterpriseExecutionModeQueued,
		DispatchMode:            dispatchMode,
		TargetStatus:            entry.Status,
		RequestedBy:             enterpriseHRISWebhookExecutionRequestedBy(r),
		ReplaySourceExecutionID: strings.TrimSpace(replaySourceExecutionID),
		ReplayRequireWorker:     replayRequireWorker,
	})
	if err != nil {
		return "", s.restoreQueuedEnterpriseHRISWebhookDLQDispatch(original, err)
	}
	if dispatchMode == enterpriseHRISWebhookExecutionDispatchModeWorkerTick {
		s.enqueueEnterpriseHRISWebhookExecution(
			enterpriseHRISWebhookDLQExecutionQueue,
			executionID,
			tenantID,
			enterprise.HRISWebhookExecutionKindDLQReplay,
		)
		s.notifyEnterpriseHRISWebhookDLQWorker()
		return executionID, nil
	}
	go s.replayQueuedEnterpriseHRISWebhookDLQEntry(tenantID, entry, auditSource, executionID)
	return executionID, nil
}

func (s *server) enqueueEnterpriseHRISWebhookExecution(
	queueName string,
	executionID string,
	tenantID string,
	kind string,
) {
	if s == nil || s.workerQueueStore == nil {
		return
	}
	nextQueueName := strings.TrimSpace(queueName)
	nextExecutionID := strings.TrimSpace(executionID)
	if nextQueueName == "" || nextExecutionID == "" {
		return
	}
	if err := s.workerQueueStore.EnqueueWorkerQueue(nextQueueName, nextExecutionID); err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook execution queue enqueue failed",
			"tenant_id", tenantID,
			"kind", kind,
			"queue", nextQueueName,
			"execution_id", nextExecutionID,
			"err", err,
		)
	}
}

func (s *server) plannedEnterpriseHRISWebhookDLQDispatchMode() string {
	if s != nil && s.cfg.EnterpriseHRISWebhookDLQWorkerEnabled {
		return enterpriseHRISWebhookExecutionDispatchModeWorkerTick
	}
	return enterpriseHRISWebhookExecutionDispatchModeGoroutineFallback
}

func (s *server) requireEnterpriseHRISWebhookReceiptWorker(executionMode string, requireWorker bool) error {
	if executionMode != enterpriseExecutionModeQueued || !requireWorker {
		return nil
	}
	if s != nil && s.cfg.EnterpriseHRISWebhookReceiptWorkerEnabled {
		return nil
	}
	return errEnterpriseHRISWebhookQueuedReceiptWorkerRequired
}

func (s *server) requireEnterpriseHRISWebhookDLQWorker(executionMode string, requireWorker bool) error {
	if executionMode != enterpriseExecutionModeQueued || !requireWorker {
		return nil
	}
	if s != nil && s.cfg.EnterpriseHRISWebhookDLQWorkerEnabled {
		return nil
	}
	return errEnterpriseHRISWebhookQueuedDLQWorkerRequired
}

func (s *server) requireEnterpriseHRISWebhookExecutionReplayWorker(
	item enterprise.HRISWebhookExecution,
	executionMode string,
	requireWorker bool,
) error {
	if executionMode != enterpriseExecutionModeQueued || !requireWorker {
		return nil
	}
	switch normalizeLifecycleToken(item.Kind) {
	case enterprise.HRISWebhookExecutionKindReceiptProcess:
		return s.requireEnterpriseHRISWebhookReceiptWorker(executionMode, requireWorker)
	case enterprise.HRISWebhookExecutionKindDLQReplay:
		return s.requireEnterpriseHRISWebhookDLQWorker(executionMode, requireWorker)
	default:
		return nil
	}
}

func (s *server) restoreQueuedEnterpriseHRISWebhookReceiptDispatch(
	original enterprise.HRISWebhookReceipt,
	dispatchErr error,
) error {
	if dispatchErr == nil {
		return nil
	}
	if s == nil || s.enterpriseSvc == nil {
		return dispatchErr
	}
	if _, err := s.enterpriseSvc.RestoreHRISWebhookReceipt(original); err != nil {
		return fmt.Errorf("%w: restore receipt state transition: %v", dispatchErr, err)
	}
	return dispatchErr
}

func (s *server) restoreQueuedEnterpriseHRISWebhookDLQDispatch(
	original hris.DeadLetterEntry,
	dispatchErr error,
) error {
	if dispatchErr == nil {
		return nil
	}
	if s == nil || s.hrisDLQSvc == nil {
		return dispatchErr
	}
	if _, err := s.hrisDLQSvc.RestoreEntry(original); err != nil {
		return fmt.Errorf("%w: restore dlq state transition: %v", dispatchErr, err)
	}
	return dispatchErr
}

func (s *server) createHRISWebhookExecutionRecord(
	r *http.Request,
	input enterprise.HRISWebhookExecutionInput,
) (string, error) {
	if s == nil || s.enterpriseSvc == nil {
		return "", errors.New("hris webhook execution services are not configured")
	}
	record, err := s.enterpriseSvc.CreateHRISWebhookExecution(input)
	if err != nil {
		s.loggerOrDefault().Error(
			"create hris webhook execution record failed",
			"tenant_id", input.TenantID,
			"kind", input.Kind,
			"target_id", input.TargetID,
			"connector_id", input.ConnectorID,
			"vendor", input.Vendor,
			"event_type", input.EventType,
			"request_id", input.RequestID,
			"err", err,
		)
		return "", err
	}
	return record.ID, nil
}

func (s *server) updateHRISWebhookExecutionDispatchMode(tenantID, executionID, dispatchMode string) {
	if s == nil || s.enterpriseSvc == nil || strings.TrimSpace(executionID) == "" {
		return
	}
	if _, err := s.enterpriseSvc.UpdateHRISWebhookExecutionDispatchMode(tenantID, executionID, dispatchMode); err != nil {
		s.loggerOrDefault().Error(
			"update hris webhook execution dispatch mode failed",
			"tenant_id", tenantID,
			"execution_id", executionID,
			"dispatch_mode", dispatchMode,
			"err", err,
		)
	}
}

func (s *server) markHRISWebhookExecutionRunning(tenantID, executionID string) {
	if s == nil || s.enterpriseSvc == nil || strings.TrimSpace(executionID) == "" {
		return
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning(tenantID, executionID); err != nil {
		s.loggerOrDefault().Error(
			"mark hris webhook execution running failed",
			"tenant_id", tenantID,
			"execution_id", executionID,
			"err", err,
		)
	}
}

func (s *server) completeHRISWebhookReceiptExecution(
	receipt enterprise.HRISWebhookReceipt,
	executionID string,
	processErr error,
) {
	if s == nil || s.enterpriseSvc == nil || strings.TrimSpace(executionID) == "" {
		return
	}

	targetStatus := strings.TrimSpace(receipt.Status)
	updated, err := s.enterpriseSvc.GetHRISWebhookReceipt(receipt.TenantID, receipt.ID)
	if err == nil {
		targetStatus = strings.TrimSpace(updated.Status)
	}

	if processErr != nil {
		if _, markErr := s.enterpriseSvc.AcknowledgeHRISWebhookExecution(receipt.TenantID, executionID, targetStatus, processErr); markErr != nil {
			s.loggerOrDefault().Error(
				"mark hris webhook receipt execution failed failed",
				"tenant_id", receipt.TenantID,
				"execution_id", executionID,
				"target_status", targetStatus,
				"err", markErr,
			)
		}
		return
	}

	switch targetStatus {
	case "processed", "skipped":
		if _, markErr := s.enterpriseSvc.AcknowledgeHRISWebhookExecution(receipt.TenantID, executionID, targetStatus, nil); markErr != nil {
			s.loggerOrDefault().Error(
				"mark hris webhook receipt execution succeeded failed",
				"tenant_id", receipt.TenantID,
				"execution_id", executionID,
				"target_status", targetStatus,
				"err", markErr,
			)
		}
	case "failed", "dlq":
		failure := errors.New(firstNonEmptyString(updated.LastError, targetStatus))
		if _, markErr := s.enterpriseSvc.AcknowledgeHRISWebhookExecution(receipt.TenantID, executionID, targetStatus, failure); markErr != nil {
			s.loggerOrDefault().Error(
				"mark hris webhook receipt execution terminal failure failed",
				"tenant_id", receipt.TenantID,
				"execution_id", executionID,
				"target_status", targetStatus,
				"err", markErr,
			)
		}
	default:
		failure := fmt.Errorf("unexpected receipt status after queued execution: %s", targetStatus)
		if _, markErr := s.enterpriseSvc.AcknowledgeHRISWebhookExecution(receipt.TenantID, executionID, targetStatus, failure); markErr != nil {
			s.loggerOrDefault().Error(
				"mark hris webhook receipt execution unexpected status failed",
				"tenant_id", receipt.TenantID,
				"execution_id", executionID,
				"target_status", targetStatus,
				"err", markErr,
			)
		}
	}
}

func (s *server) completeHRISWebhookDLQExecutionSuccess(
	tenantID string,
	entry hris.DeadLetterEntry,
	executionID string,
) {
	if s == nil || s.enterpriseSvc == nil || strings.TrimSpace(executionID) == "" {
		return
	}
	if _, err := s.enterpriseSvc.AcknowledgeHRISWebhookExecution(tenantID, executionID, strings.TrimSpace(entry.Status), nil); err != nil {
		s.loggerOrDefault().Error(
			"mark hris webhook dlq execution succeeded failed",
			"tenant_id", tenantID,
			"execution_id", executionID,
			"entry_id", entry.ID,
			"target_status", entry.Status,
			"err", err,
		)
	}
}

func (s *server) completeHRISWebhookDLQExecution(
	tenantID string,
	entry hris.DeadLetterEntry,
	executionID string,
	replayErr error,
) {
	if s == nil || s.enterpriseSvc == nil || strings.TrimSpace(executionID) == "" {
		return
	}
	targetStatus := strings.TrimSpace(entry.Status)
	if s.hrisDLQSvc != nil {
		if updated, err := s.hrisDLQSvc.GetEntry(entry.ID); err == nil {
			targetStatus = strings.TrimSpace(updated.Status)
		}
	}
	if _, err := s.enterpriseSvc.AcknowledgeHRISWebhookExecution(tenantID, executionID, targetStatus, replayErr); err != nil {
		s.loggerOrDefault().Error(
			"mark hris webhook dlq execution failed failed",
			"tenant_id", tenantID,
			"execution_id", executionID,
			"entry_id", entry.ID,
			"target_status", targetStatus,
			"err", err,
		)
	}
}

func enterpriseHRISWebhookExecutionRequestedBy(r *http.Request) string {
	if r == nil {
		return ""
	}
	user, ok := authenticatedUser(r)
	if !ok {
		return ""
	}
	return firstNonEmptyString(user.Email, user.ID, user.Role)
}

func normalizeEnterpriseHRISWebhookDLQBatchReplayIDs(input []string) []string {
	seen := make(map[string]struct{}, len(input))
	output := make([]string, 0, len(input))
	for i := range input {
		entryID := strings.TrimSpace(input[i])
		if entryID == "" {
			continue
		}
		if _, ok := seen[entryID]; ok {
			continue
		}
		seen[entryID] = struct{}{}
		output = append(output, entryID)
	}
	return output
}

func normalizeEnterpriseHRISWebhookReceiptBatchProcessIDs(input []string) []string {
	seen := make(map[string]struct{}, len(input))
	output := make([]string, 0, len(input))
	for i := range input {
		receiptID := strings.TrimSpace(input[i])
		if receiptID == "" {
			continue
		}
		if _, ok := seen[receiptID]; ok {
			continue
		}
		seen[receiptID] = struct{}{}
		output = append(output, receiptID)
	}
	return output
}

func pointerToHRISWebhookReceipt(item enterprise.HRISWebhookReceipt) *enterprise.HRISWebhookReceipt {
	copyItem := item
	return &copyItem
}

func pointerToHRISWebhookDLQEntry(item hris.DeadLetterEntry) *hris.DeadLetterEntry {
	copyItem := item
	return &copyItem
}

func pointerToBool(value bool) *bool {
	copyValue := value
	return &copyValue
}

func isWorkerManagedHRISWebhookExecution(item enterprise.HRISWebhookExecution) bool {
	switch normalizeLifecycleToken(item.Kind) {
	case enterprise.HRISWebhookExecutionKindReceiptProcess, enterprise.HRISWebhookExecutionKindDLQReplay:
	default:
		return false
	}
	if normalizeLifecycleToken(item.ExecutionMode) != enterpriseExecutionModeQueued {
		return false
	}
	if normalizeLifecycleToken(item.DispatchMode) != enterpriseHRISWebhookExecutionDispatchModeWorkerTick {
		return false
	}
	return strings.TrimSpace(item.TargetID) != ""
}

func (s *server) enterpriseHRISWebhookExecutionProcessingTimeout(item enterprise.HRISWebhookExecution) time.Duration {
	if s == nil {
		return 5 * time.Minute
	}
	switch normalizeLifecycleToken(item.Kind) {
	case enterprise.HRISWebhookExecutionKindReceiptProcess:
		if s.cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout > 0 {
			return s.cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout
		}
	case enterprise.HRISWebhookExecutionKindDLQReplay:
		if s.cfg.EnterpriseHRISWebhookDLQWorkerProcessingTimeout > 0 {
			return s.cfg.EnterpriseHRISWebhookDLQWorkerProcessingTimeout
		}
	}
	return 5 * time.Minute
}

func describeHRISWebhookReceiptQueueState(
	item enterprise.HRISWebhookReceipt,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) hrisQueueRuntime {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryCooldown, retryMaxBackoff = retrybackoff.Normalize(retryCooldown, retryMaxBackoff)
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	status := strings.TrimSpace(item.Status)
	runtime := hrisQueueRuntime{}
	switch status {
	case "received":
		runtime.RemainingAttempts = remainingAttempts(maxAttempts, item.AttemptCount)
		if runtime.RemainingAttempts <= 0 {
			runtime.State = enterprise.HRISWebhookReceiptClaimReasonAttemptLimit
			return runtime
		}
		runtime.State = "ready"
		return runtime
	case "failed":
		runtime.RemainingAttempts = remainingAttempts(maxAttempts, item.AttemptCount)
		if runtime.RemainingAttempts <= 0 {
			runtime.State = enterprise.HRISWebhookReceiptClaimReasonAttemptLimit
			return runtime
		}
		runtime.NextRetryAt = nextRetryTime(item.LastAttemptAt, item.AttemptCount, retryCooldown, retryMaxBackoff)
		if runtime.NextRetryAt != nil && runtime.NextRetryAt.After(now) {
			runtime.State = enterprise.HRISWebhookReceiptClaimReasonCooldown
			runtime.CooldownRemainingSec = remainingSecondsUntil(runtime.NextRetryAt, now)
			return runtime
		}
		runtime.State = "ready"
		return runtime
	case "processing":
		runtime.RemainingAttempts = remainingAttempts(maxAttempts, item.AttemptCount)
		runtime.ProcessingDeadlineAt = processingDeadline(item.LastAttemptAt, processingTimeout)
		if runtime.ProcessingDeadlineAt != nil && runtime.ProcessingDeadlineAt.After(now) {
			runtime.State = enterprise.HRISWebhookReceiptClaimReasonInFlight
			return runtime
		}
		runtime.State = "ready"
		runtime.StaleInFlight = runtime.ProcessingDeadlineAt != nil
		return runtime
	case "processed", "skipped", "dlq":
		runtime.State = "terminal"
		return runtime
	default:
		runtime.State = status
		return runtime
	}
}

func describeHRISWebhookDLQReplayState(
	item hris.DeadLetterEntry,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) hrisQueueRuntime {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryCooldown, retryMaxBackoff = retrybackoff.Normalize(retryCooldown, retryMaxBackoff)
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	status := strings.TrimSpace(item.Status)
	runtime := hrisQueueRuntime{}
	switch status {
	case "dlq":
		runtime.RemainingAttempts = remainingAttempts(maxAttempts, item.ReplayCount)
		if runtime.RemainingAttempts <= 0 {
			runtime.State = hris.DLQEntryClaimReasonAttemptLimit
			return runtime
		}
		runtime.NextRetryAt = nextRetryTime(item.LastReplayAt, item.ReplayCount, retryCooldown, retryMaxBackoff)
		if runtime.NextRetryAt != nil && runtime.NextRetryAt.After(now) {
			runtime.State = hris.DLQEntryClaimReasonCooldown
			runtime.CooldownRemainingSec = remainingSecondsUntil(runtime.NextRetryAt, now)
			return runtime
		}
		runtime.State = "ready"
		return runtime
	case "replaying":
		runtime.RemainingAttempts = remainingAttempts(maxAttempts, item.ReplayCount)
		runtime.ProcessingDeadlineAt = processingDeadline(item.LastReplayAt, processingTimeout)
		if runtime.ProcessingDeadlineAt != nil && runtime.ProcessingDeadlineAt.After(now) {
			runtime.State = hris.DLQEntryClaimReasonInFlight
			return runtime
		}
		runtime.State = "ready"
		runtime.StaleInFlight = runtime.ProcessingDeadlineAt != nil
		return runtime
	case "resolved":
		runtime.State = "terminal"
		return runtime
	default:
		runtime.State = status
		return runtime
	}
}

func describeHRISWebhookExecutionQueueState(
	item enterprise.HRISWebhookExecution,
	processingTimeout time.Duration,
	now time.Time,
) hrisQueueRuntime {
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	runtime := hrisQueueRuntime{}
	if !isWorkerManagedHRISWebhookExecution(item) {
		return runtime
	}

	switch strings.TrimSpace(item.Status) {
	case enterprise.HRISWebhookExecutionStatusQueued:
		if !item.QueuedAt.IsZero() && item.QueuedAt.UTC().After(now) {
			retryAt := item.QueuedAt.UTC()
			runtime.State = enterprise.HRISWebhookExecutionClaimReasonCooldown
			runtime.NextRetryAt = &retryAt
			runtime.CooldownRemainingSec = remainingSecondsUntil(runtime.NextRetryAt, now)
			return runtime
		}
		runtime.State = "ready"
		return runtime
	case enterprise.HRISWebhookExecutionStatusRunning:
		runtime.ProcessingDeadlineAt = processingDeadline(item.StartedAt, processingTimeout)
		if runtime.ProcessingDeadlineAt != nil && runtime.ProcessingDeadlineAt.After(now) {
			runtime.State = enterprise.HRISWebhookExecutionClaimReasonInFlight
			return runtime
		}
		runtime.State = "ready"
		runtime.StaleInFlight = runtime.ProcessingDeadlineAt != nil
		return runtime
	case enterprise.HRISWebhookExecutionStatusSucceeded, enterprise.HRISWebhookExecutionStatusFailed:
		runtime.State = "terminal"
		return runtime
	default:
		runtime.State = strings.TrimSpace(item.Status)
		return runtime
	}
}

func nextRetryTime(lastAttemptAt *time.Time, attempts int, retryCooldown time.Duration, retryMaxBackoff time.Duration) *time.Time {
	if lastAttemptAt == nil {
		return nil
	}
	retryCooldown, retryMaxBackoff = retrybackoff.Normalize(retryCooldown, retryMaxBackoff)
	delay := retrybackoff.Exponential(attempts, retryCooldown, retryMaxBackoff)
	if delay <= 0 {
		return nil
	}
	next := lastAttemptAt.Add(delay)
	return &next
}

func remainingAttempts(maxAttempts int, attempts int) int {
	if maxAttempts <= 0 {
		return 0
	}
	if attempts >= maxAttempts {
		return 0
	}
	return maxAttempts - attempts
}

func remainingSecondsUntil(target *time.Time, now time.Time) int64 {
	if target == nil {
		return 0
	}
	if !target.After(now) {
		return 0
	}
	return int64(math.Ceil(target.Sub(now).Seconds()))
}

func processingDeadline(lastAttemptAt *time.Time, processingTimeout time.Duration) *time.Time {
	if lastAttemptAt == nil {
		return nil
	}
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	deadline := lastAttemptAt.Add(processingTimeout)
	return &deadline
}

func parseEnterpriseHRISWebhookReceiptListOptions(r *http.Request) (hrisWebhookReceiptListOptions, error) {
	options := hrisWebhookReceiptListOptions{
		ConnectorID: strings.TrimSpace(r.URL.Query().Get("connector_id")),
		Status:      normalizeLifecycleToken(r.URL.Query().Get("status")),
		QueueState:  normalizeLifecycleToken(r.URL.Query().Get("queue_state")),
		Query:       strings.TrimSpace(r.URL.Query().Get("q")),
		Limit:       50,
	}
	if options.Status != "" && !isValidEnterpriseHRISWebhookReceiptStatus(options.Status) {
		return hrisWebhookReceiptListOptions{}, errors.New("status must be one of received, processing, processed, failed, skipped, dlq")
	}
	if options.QueueState != "" && !isValidEnterpriseHRISWebhookRuntimeState(options.QueueState) {
		return hrisWebhookReceiptListOptions{}, errors.New("queue_state must be one of ready, cooldown, in_flight, attempt_limit, terminal")
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return hrisWebhookReceiptListOptions{}, errors.New("offset must be an integer >= 0")
		}
		options.Offset = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return hrisWebhookReceiptListOptions{}, errors.New("limit must be an integer > 0")
		}
		if parsed > enterpriseHRISWebhookQueueListMaxLimit {
			return hrisWebhookReceiptListOptions{}, fmt.Errorf("limit must be <= %d", enterpriseHRISWebhookQueueListMaxLimit)
		}
		options.Limit = parsed
	}
	return options, nil
}

func parseEnterpriseHRISWebhookDLQListOptions(r *http.Request) (hrisWebhookDLQListOptions, error) {
	options := hrisWebhookDLQListOptions{
		ConnectorID: strings.TrimSpace(r.URL.Query().Get("connector_id")),
		Status:      normalizeLifecycleToken(r.URL.Query().Get("status")),
		ReplayState: normalizeLifecycleToken(r.URL.Query().Get("replay_state")),
		Query:       strings.TrimSpace(r.URL.Query().Get("q")),
		Limit:       50,
	}
	if options.Status != "" && !isValidEnterpriseHRISWebhookDLQStatus(options.Status) {
		return hrisWebhookDLQListOptions{}, errors.New("status must be one of dlq, replaying, resolved")
	}
	if options.ReplayState != "" && !isValidEnterpriseHRISWebhookRuntimeState(options.ReplayState) {
		return hrisWebhookDLQListOptions{}, errors.New("replay_state must be one of ready, cooldown, in_flight, attempt_limit, terminal")
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return hrisWebhookDLQListOptions{}, errors.New("offset must be an integer >= 0")
		}
		options.Offset = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return hrisWebhookDLQListOptions{}, errors.New("limit must be an integer > 0")
		}
		if parsed > enterpriseHRISWebhookQueueListMaxLimit {
			return hrisWebhookDLQListOptions{}, fmt.Errorf("limit must be <= %d", enterpriseHRISWebhookQueueListMaxLimit)
		}
		options.Limit = parsed
	}
	return options, nil
}

func parseEnterpriseHRISWebhookExecutionListOptions(r *http.Request) (hrisWebhookExecutionListOptions, error) {
	options := hrisWebhookExecutionListOptions{
		ConnectorID:   strings.TrimSpace(r.URL.Query().Get("connector_id")),
		Kind:          normalizeLifecycleToken(r.URL.Query().Get("kind")),
		Status:        normalizeLifecycleToken(r.URL.Query().Get("status")),
		QueueState:    normalizeLifecycleToken(r.URL.Query().Get("queue_state")),
		ReplayScope:   normalizeLifecycleToken(r.URL.Query().Get("replay_scope")),
		ExecutionMode: normalizeLifecycleToken(r.URL.Query().Get("execution_mode")),
		DispatchMode:  normalizeLifecycleToken(r.URL.Query().Get("dispatch_mode")),
		TargetStatus:  normalizeLifecycleToken(r.URL.Query().Get("target_status")),
		TargetID:      strings.TrimSpace(r.URL.Query().Get("target_id")),
		Query:         strings.TrimSpace(r.URL.Query().Get("q")),
		Limit:         50,
	}
	if options.Kind != "" && !isValidEnterpriseHRISWebhookExecutionKind(options.Kind) {
		return hrisWebhookExecutionListOptions{}, errors.New("kind must be one of receipt_process, dlq_replay")
	}
	if options.Status != "" && !isValidEnterpriseHRISWebhookExecutionStatus(options.Status) {
		return hrisWebhookExecutionListOptions{}, errors.New("status must be one of queued, running, succeeded, failed")
	}
	if options.QueueState != "" && !isValidEnterpriseHRISWebhookRuntimeState(options.QueueState) {
		return hrisWebhookExecutionListOptions{}, errors.New("queue_state must be one of ready, cooldown, in_flight, attempt_limit, terminal")
	}
	if options.ReplayScope != "" && !isValidEnterpriseHRISWebhookExecutionReplayScope(options.ReplayScope) {
		return hrisWebhookExecutionListOptions{}, errors.New("replay_scope must be one of replayed, worker_required")
	}
	if options.ExecutionMode != "" && !isValidEnterpriseHRISWebhookExecutionMode(options.ExecutionMode) {
		return hrisWebhookExecutionListOptions{}, errors.New("execution_mode must be one of inline, queued")
	}
	if options.DispatchMode != "" && !isValidEnterpriseHRISWebhookExecutionDispatchMode(options.DispatchMode) {
		return hrisWebhookExecutionListOptions{}, errors.New("dispatch_mode must be one of worker_tick, worker_task_channel, goroutine_fallback")
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return hrisWebhookExecutionListOptions{}, errors.New("offset must be an integer >= 0")
		}
		options.Offset = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return hrisWebhookExecutionListOptions{}, errors.New("limit must be an integer > 0")
		}
		if parsed > enterpriseHRISWebhookQueueListMaxLimit {
			return hrisWebhookExecutionListOptions{}, fmt.Errorf("limit must be <= %d", enterpriseHRISWebhookQueueListMaxLimit)
		}
		options.Limit = parsed
	}
	return options, nil
}

func matchesHRISWebhookReceiptListFilters(item hrisWebhookReceiptListItem, options hrisWebhookReceiptListOptions) bool {
	if options.Status != "" && normalizeLifecycleToken(item.Status) != options.Status {
		return false
	}
	if options.Query != "" && !matchesHRISWebhookReceiptQuery(item, options.Query) {
		return false
	}
	return true
}

func matchesHRISWebhookDLQListFilters(item hrisWebhookDLQListItem, options hrisWebhookDLQListOptions) bool {
	if options.Status != "" && normalizeLifecycleToken(item.Status) != options.Status {
		return false
	}
	if options.Query != "" && !matchesHRISWebhookDLQQuery(item, options.Query) {
		return false
	}
	return true
}

func matchesHRISWebhookReceiptQuery(item hrisWebhookReceiptListItem, query string) bool {
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	if normalizedQuery == "" {
		return true
	}
	values := []string{
		item.ID,
		item.TenantID,
		item.ConnectorID,
		item.Vendor,
		item.EventType,
		item.RequestID,
		item.ContentType,
		item.SourceIP,
		item.Status,
		item.QueueState,
		item.LastError,
		item.RawPayload,
		strconv.Itoa(item.AttemptCount),
		strconv.Itoa(item.RemainingAttempts),
		strconv.FormatInt(item.CooldownRemainingSec, 10),
		formatQueryTime(item.ReceivedAt),
		formatQueryTimePointer(item.LastAttemptAt),
		formatQueryTimePointer(item.ProcessedAt),
		formatQueryTimePointer(item.NextRetryAt),
		formatQueryTimePointer(item.ProcessingDeadlineAt),
		strconv.FormatBool(item.StaleInFlight),
	}
	for i := range values {
		if strings.Contains(strings.ToLower(values[i]), normalizedQuery) {
			return true
		}
	}
	return false
}

func matchesHRISWebhookDLQQuery(item hrisWebhookDLQListItem, query string) bool {
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	if normalizedQuery == "" {
		return true
	}
	values := []string{
		item.ID,
		item.TenantID,
		item.ConnectorID,
		item.Vendor,
		item.ReceiptID,
		item.RequestID,
		item.EventType,
		item.FailureStage,
		item.Error,
		item.RawPayloadRef,
		item.Status,
		item.ReplayState,
		strconv.Itoa(item.ReplayCount),
		strconv.Itoa(item.RemainingAttempts),
		strconv.FormatInt(item.CooldownRemainingSec, 10),
		formatQueryTime(item.CreatedAt),
		formatQueryTime(item.UpdatedAt),
		formatQueryTimePointer(item.LastReplayAt),
		formatQueryTimePointer(item.ResolvedAt),
		formatQueryTimePointer(item.NextRetryAt),
		formatQueryTimePointer(item.ProcessingDeadlineAt),
		strconv.FormatBool(item.StaleInFlight),
	}
	for i := range values {
		if strings.Contains(strings.ToLower(values[i]), normalizedQuery) {
			return true
		}
	}
	return false
}

func matchesHRISWebhookExecutionQuery(item hrisWebhookExecutionListItem, query string) bool {
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	if normalizedQuery == "" {
		return true
	}
	values := []string{
		item.ID,
		item.TenantID,
		item.Kind,
		item.TargetID,
		item.ReceiptID,
		item.ConnectorID,
		item.Vendor,
		item.RequestID,
		item.EventType,
		item.FailureStage,
		item.AuditSource,
		item.ExecutionMode,
		item.DispatchMode,
		item.Status,
		item.TargetStatus,
		item.RequestedBy,
		item.QueueState,
		item.ExternalQueueName,
		item.ExternalQueueState,
		item.LastError,
		formatQueryTime(item.QueuedAt),
		formatQueryTimePointer(item.StartedAt),
		formatQueryTimePointer(item.FinishedAt),
		formatQueryTime(item.UpdatedAt),
		formatQueryTimePointer(item.NextRetryAt),
		formatQueryTimePointer(item.ProcessingDeadlineAt),
		formatQueryTimePointer(item.ExternalQueueVisibilityDeadlineAt),
		strconv.FormatInt(item.CooldownRemainingSec, 10),
		strconv.FormatBool(item.StaleInFlight),
	}
	for i := range values {
		if strings.Contains(strings.ToLower(values[i]), normalizedQuery) {
			return true
		}
	}
	return false
}

func buildHRISWebhookExecutionStatusCounts(items []hrisWebhookExecutionListItem) hrisWebhookExecutionStatusCounts {
	counts := hrisWebhookExecutionStatusCounts{
		All: len(items),
	}
	for i := range items {
		switch normalizeLifecycleToken(items[i].Status) {
		case enterprise.HRISWebhookExecutionStatusQueued:
			counts.Queued++
		case enterprise.HRISWebhookExecutionStatusRunning:
			counts.Running++
		case enterprise.HRISWebhookExecutionStatusSucceeded:
			counts.Succeeded++
		case enterprise.HRISWebhookExecutionStatusFailed:
			counts.Failed++
		}
	}
	return counts
}

func buildHRISWebhookRuntimeCounts[T any](items []T, extractState func(T) string) hrisWebhookRuntimeCounts {
	counts := hrisWebhookRuntimeCounts{}
	for i := range items {
		counts.All++
		switch normalizeLifecycleToken(extractState(items[i])) {
		case "ready":
			counts.Ready++
		case enterprise.HRISWebhookReceiptClaimReasonCooldown:
			counts.Cooldown++
		case enterprise.HRISWebhookReceiptClaimReasonInFlight:
			counts.InFlight++
		case enterprise.HRISWebhookReceiptClaimReasonAttemptLimit:
			counts.AttemptLimit++
		case "terminal":
			counts.Terminal++
		}
	}
	return counts
}

func buildHRISWebhookReceiptListResult(
	items []hrisWebhookReceiptListItem,
	offset int,
	limit int,
) hrisWebhookReceiptListResult {
	normalizedOffset, normalizedLimit, end := normalizeEnterpriseHRISWebhookListWindow(len(items), offset, limit)
	result := hrisWebhookReceiptListResult{
		Items:   append([]hrisWebhookReceiptListItem(nil), items[normalizedOffset:end]...),
		Total:   len(items),
		Offset:  normalizedOffset,
		Limit:   normalizedLimit,
		HasMore: end < len(items),
	}
	if result.HasMore {
		result.NextOffset = end
	}
	return result
}

func sortHRISWebhookReceiptRuntimeItems(items []hrisWebhookReceiptListItem, queueState string) {
	normalizedState := normalizeLifecycleToken(queueState)
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if normalizedState == "ready" && left.StaleInFlight != right.StaleInFlight {
			return left.StaleInFlight && !right.StaleInFlight
		}
		leftTime := receiptRuntimeSortTime(left, normalizedState)
		rightTime := receiptRuntimeSortTime(right, normalizedState)
		if leftTime.Equal(rightTime) {
			return left.ID > right.ID
		}
		return leftTime.After(rightTime)
	})
}

func buildHRISWebhookDLQListResult(
	items []hrisWebhookDLQListItem,
	offset int,
	limit int,
) hrisWebhookDLQListResult {
	normalizedOffset, normalizedLimit, end := normalizeEnterpriseHRISWebhookListWindow(len(items), offset, limit)
	result := hrisWebhookDLQListResult{
		Items:   append([]hrisWebhookDLQListItem(nil), items[normalizedOffset:end]...),
		Total:   len(items),
		Offset:  normalizedOffset,
		Limit:   normalizedLimit,
		HasMore: end < len(items),
	}
	if result.HasMore {
		result.NextOffset = end
	}
	return result
}

func buildHRISWebhookExecutionListResult(
	items []hrisWebhookExecutionListItem,
	offset int,
	limit int,
) hrisWebhookExecutionListResult {
	normalizedOffset, normalizedLimit, end := normalizeEnterpriseHRISWebhookListWindow(len(items), offset, limit)
	result := hrisWebhookExecutionListResult{
		Items:   append([]hrisWebhookExecutionListItem(nil), items[normalizedOffset:end]...),
		Total:   len(items),
		Offset:  normalizedOffset,
		Limit:   normalizedLimit,
		HasMore: end < len(items),
	}
	if result.HasMore {
		result.NextOffset = end
	}
	return result
}

func shouldExposeHRISWebhookExecutionExternalQueue(item enterprise.HRISWebhookExecution) bool {
	if normalizeLifecycleToken(item.ExecutionMode) != enterpriseExecutionModeQueued {
		return false
	}
	if normalizeLifecycleToken(item.DispatchMode) != enterpriseHRISWebhookExecutionDispatchModeWorkerTick {
		return false
	}
	switch normalizeLifecycleToken(item.Status) {
	case enterprise.HRISWebhookExecutionStatusQueued, enterprise.HRISWebhookExecutionStatusRunning:
	default:
		return false
	}
	return enterpriseHRISWebhookExecutionQueueName(item.Kind) != ""
}

func (s *server) enrichHRISWebhookExecutionExternalQueueTelemetry(
	items []hrisWebhookExecutionListItem,
) []hrisWebhookExecutionExternalQueueSummary {
	if len(items) == 0 {
		return nil
	}

	type queueExecutionGroup struct {
		Kind        string
		IDs         []string
		IndicesByID map[string][]int
	}

	groups := make(map[string]*queueExecutionGroup)
	for i := range items {
		if !shouldExposeHRISWebhookExecutionExternalQueue(items[i].HRISWebhookExecution) {
			continue
		}
		queueName := enterpriseHRISWebhookExecutionQueueName(items[i].Kind)
		if queueName == "" {
			continue
		}
		group, ok := groups[queueName]
		if !ok {
			group = &queueExecutionGroup{
				Kind:        normalizeLifecycleToken(items[i].Kind),
				IDs:         []string{},
				IndicesByID: make(map[string][]int),
			}
			groups[queueName] = group
		}
		if _, seen := group.IndicesByID[items[i].ID]; !seen {
			group.IDs = append(group.IDs, items[i].ID)
		}
		group.IndicesByID[items[i].ID] = append(group.IndicesByID[items[i].ID], i)
	}
	if len(groups) == 0 {
		return nil
	}

	summaries := make([]hrisWebhookExecutionExternalQueueSummary, 0, len(groups))
	if s == nil || s.workerQueueStore == nil {
		for queueName, group := range groups {
			summaries = append(summaries, hrisWebhookExecutionExternalQueueSummary{
				Kind:         group.Kind,
				QueueName:    queueName,
				PendingCount: 0,
				ClaimedCount: 0,
			})
			for executionID := range group.IndicesByID {
				indices := group.IndicesByID[executionID]
				for _, index := range indices {
					items[index].ExternalQueueName = queueName
					items[index].ExternalQueueState = redistore.WorkerQueueStateMissing
					items[index].ExternalQueueVisibilityDeadlineAt = nil
				}
			}
		}
		sort.Slice(summaries, func(i, j int) bool {
			if summaries[i].Kind == summaries[j].Kind {
				return summaries[i].QueueName < summaries[j].QueueName
			}
			return summaries[i].Kind < summaries[j].Kind
		})
		return summaries
	}

	for queueName, group := range groups {
		telemetry, err := s.workerQueueStore.DescribeWorkerQueue(queueName, group.IDs)
		if err != nil {
			s.loggerOrDefault().Error(
				"describe enterprise hris webhook execution external queue failed",
				"queue", queueName,
				"kind", group.Kind,
				"err", err,
			)
			continue
		}
		summaries = append(summaries, hrisWebhookExecutionExternalQueueSummary{
			Kind:         group.Kind,
			QueueName:    queueName,
			PendingCount: telemetry.PendingCount,
			ClaimedCount: telemetry.ClaimedCount,
		})
		for executionID, state := range telemetry.Items {
			indices := group.IndicesByID[executionID]
			for _, index := range indices {
				items[index].ExternalQueueName = queueName
				items[index].ExternalQueueState = normalizeLifecycleToken(state.State)
				items[index].ExternalQueueVisibilityDeadlineAt = cloneTimePointerLocal(state.VisibilityDeadlineAt)
			}
		}
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Kind == summaries[j].Kind {
			return summaries[i].QueueName < summaries[j].QueueName
		}
		return summaries[i].Kind < summaries[j].Kind
	})
	return summaries
}

func sortHRISWebhookDLQRuntimeItems(items []hrisWebhookDLQListItem, replayState string) {
	normalizedState := normalizeLifecycleToken(replayState)
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if normalizedState == "ready" && left.StaleInFlight != right.StaleInFlight {
			return left.StaleInFlight && !right.StaleInFlight
		}
		leftTime := dlqRuntimeSortTime(left, normalizedState)
		rightTime := dlqRuntimeSortTime(right, normalizedState)
		if leftTime.Equal(rightTime) {
			return left.ID > right.ID
		}
		return leftTime.After(rightTime)
	})
}

func sortHRISWebhookExecutionRuntimeItems(items []hrisWebhookExecutionListItem, queueState string) {
	normalizedState := normalizeLifecycleToken(queueState)
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if normalizedState == "ready" && left.StaleInFlight != right.StaleInFlight {
			return left.StaleInFlight && !right.StaleInFlight
		}
		leftTime := executionRuntimeSortTime(left, normalizedState)
		rightTime := executionRuntimeSortTime(right, normalizedState)
		if leftTime.Equal(rightTime) {
			return left.ID > right.ID
		}
		return leftTime.After(rightTime)
	})
}

func receiptRuntimeSortTime(item hrisWebhookReceiptListItem, queueState string) time.Time {
	if queueState == "ready" {
		return item.ReceivedAt
	}
	if item.LastAttemptAt != nil {
		return item.LastAttemptAt.UTC()
	}
	if item.ProcessedAt != nil {
		return item.ProcessedAt.UTC()
	}
	return item.ReceivedAt
}

func dlqRuntimeSortTime(item hrisWebhookDLQListItem, replayState string) time.Time {
	if replayState == "ready" {
		return item.CreatedAt
	}
	return item.UpdatedAt
}

func executionRuntimeSortTime(item hrisWebhookExecutionListItem, queueState string) time.Time {
	switch queueState {
	case "ready":
		if item.StaleInFlight && item.ProcessingDeadlineAt != nil {
			return item.ProcessingDeadlineAt.UTC()
		}
		if item.NextRetryAt != nil {
			return item.NextRetryAt.UTC()
		}
	case enterprise.HRISWebhookExecutionClaimReasonCooldown:
		if item.NextRetryAt != nil {
			return item.NextRetryAt.UTC()
		}
	case enterprise.HRISWebhookExecutionClaimReasonInFlight:
		if item.ProcessingDeadlineAt != nil {
			return item.ProcessingDeadlineAt.UTC()
		}
	}
	if item.StartedAt != nil {
		return item.StartedAt.UTC()
	}
	if !item.QueuedAt.IsZero() {
		return item.QueuedAt.UTC()
	}
	return item.UpdatedAt.UTC()
}

func normalizeEnterpriseHRISWebhookListWindow(total, offset, limit int) (int, int, int) {
	if limit <= 0 || limit > enterpriseHRISWebhookQueueListMaxLimit {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := total
	if offset+limit < end {
		end = offset + limit
	}
	return offset, limit, end
}

func isValidEnterpriseHRISWebhookReceiptStatus(status string) bool {
	switch normalizeLifecycleToken(status) {
	case "received", "processing", "processed", "failed", "skipped", "dlq":
		return true
	default:
		return false
	}
}

func isValidEnterpriseHRISWebhookDLQStatus(status string) bool {
	switch normalizeLifecycleToken(status) {
	case "dlq", "replaying", "resolved":
		return true
	default:
		return false
	}
}

func isValidEnterpriseHRISWebhookExecutionKind(kind string) bool {
	switch normalizeLifecycleToken(kind) {
	case enterprise.HRISWebhookExecutionKindReceiptProcess, enterprise.HRISWebhookExecutionKindDLQReplay:
		return true
	default:
		return false
	}
}

func isValidEnterpriseHRISWebhookExecutionReplayScope(scope string) bool {
	switch normalizeLifecycleToken(scope) {
	case "replayed", "worker_required":
		return true
	default:
		return false
	}
}

func isValidEnterpriseHRISWebhookExecutionStatus(status string) bool {
	switch normalizeLifecycleToken(status) {
	case enterprise.HRISWebhookExecutionStatusQueued,
		enterprise.HRISWebhookExecutionStatusRunning,
		enterprise.HRISWebhookExecutionStatusSucceeded,
		enterprise.HRISWebhookExecutionStatusFailed:
		return true
	default:
		return false
	}
}

func isValidEnterpriseHRISWebhookExecutionMode(mode string) bool {
	switch normalizeLifecycleToken(mode) {
	case enterpriseExecutionModeInline, enterpriseExecutionModeQueued:
		return true
	default:
		return false
	}
}

func isValidEnterpriseHRISWebhookExecutionDispatchMode(dispatchMode string) bool {
	switch normalizeLifecycleToken(dispatchMode) {
	case enterpriseHRISWebhookExecutionDispatchModeWorkerTick,
		enterpriseHRISWebhookExecutionDispatchModeWorkerTaskChannel,
		enterpriseHRISWebhookExecutionDispatchModeGoroutineFallback:
		return true
	default:
		return false
	}
}

func isValidEnterpriseHRISWebhookRuntimeState(state string) bool {
	switch normalizeLifecycleToken(state) {
	case "ready", enterprise.HRISWebhookReceiptClaimReasonCooldown, enterprise.HRISWebhookReceiptClaimReasonInFlight, enterprise.HRISWebhookReceiptClaimReasonAttemptLimit, "terminal":
		return true
	default:
		return false
	}
}

func normalizeLifecycleToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func formatQueryTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func formatQueryTimePointer(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func (s *server) replayEnterpriseHRISWebhookDLQClaimedEntry(
	r *http.Request,
	tenantID string,
	entry hris.DeadLetterEntry,
	auditSource string,
) (hris.DeadLetterEntry, error) {
	if s.hrisDLQSvc == nil || s.enterpriseSvc == nil || s.accessSvc == nil || s.hrisNormalizerRegistry == nil {
		return hris.DeadLetterEntry{}, errors.New("hris webhook replay services are not configured")
	}

	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		nextTenantID = strings.TrimSpace(entry.TenantID)
	}
	if nextTenantID == "" {
		return hris.DeadLetterEntry{}, enterprise.ErrTenantIDRequired
	}

	receiptID := strings.TrimSpace(entry.ReceiptID)
	if receiptID == "" {
		failure := errors.New("hris dlq entry cannot be replayed without receipt_id")
		return hris.DeadLetterEntry{}, s.markEnterpriseHRISWebhookDLQReplayFailed(
			r,
			nextTenantID,
			entry,
			receiptID,
			failure,
			auditSource,
		)
	}

	receipt, err := s.enterpriseSvc.GetHRISWebhookReceipt(nextTenantID, receiptID)
	if err != nil {
		return hris.DeadLetterEntry{}, s.markEnterpriseHRISWebhookDLQReplayFailed(
			r,
			nextTenantID,
			entry,
			receiptID,
			err,
			auditSource,
		)
	}

	if err := s.processEnterpriseHRISWebhookReceipt(r, receipt, false); err != nil {
		return hris.DeadLetterEntry{}, s.markEnterpriseHRISWebhookDLQReplayFailed(
			r,
			nextTenantID,
			entry,
			receipt.ID,
			err,
			auditSource,
		)
	}

	updated, err := s.hrisDLQSvc.MarkResolved(entry.ID)
	if err != nil {
		return hris.DeadLetterEntry{}, err
	}
	s.appendAuditLog(
		r,
		nextTenantID,
		"enterprise_hris_webhook_dlq_replayed",
		fmt.Sprintf("entry_id=%s,receipt_id=%s", updated.ID, receipt.ID),
		auditSource,
	)
	return updated, nil
}

func (s *server) markEnterpriseHRISWebhookDLQReplayFailed(
	r *http.Request,
	tenantID string,
	entry hris.DeadLetterEntry,
	receiptID string,
	failure error,
	auditSource string,
) error {
	if failure == nil {
		return nil
	}
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		nextTenantID = strings.TrimSpace(entry.TenantID)
	}
	nextReceiptID := strings.TrimSpace(receiptID)

	updated, err := s.hrisDLQSvc.MarkReplayFailedWithBackoff(
		entry.ID,
		failure,
		s.cfg.EnterpriseHRISWebhookDLQWorkerRetryCooldown,
		s.cfg.EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff,
	)
	if err != nil {
		return fmt.Errorf("%w: mark replay failed state transition: %v", failure, err)
	}
	if nextTenantID != "" && nextReceiptID != "" {
		if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptDLQ(nextTenantID, nextReceiptID, failure); err != nil {
			return fmt.Errorf("%w: restore receipt dlq state transition: %v", failure, err)
		}
	}

	s.appendAuditLog(
		r,
		nextTenantID,
		"enterprise_hris_webhook_dlq_replay_failed",
		fmt.Sprintf("entry_id=%s,receipt_id=%s,error=%s", updated.ID, nextReceiptID, failure.Error()),
		auditSource,
	)
	return failure
}

func (s *server) markEnterpriseHRISWebhookReceiptRetryableFailure(
	receipt enterprise.HRISWebhookReceipt,
	failure error,
) {
	if s == nil || s.enterpriseSvc == nil || failure == nil {
		return
	}
	_, _ = s.enterpriseSvc.MarkHRISWebhookReceiptFailedWithBackoff(
		receipt.TenantID,
		receipt.ID,
		failure,
		s.cfg.EnterpriseHRISWebhookReceiptWorkerRetryCooldown,
		s.cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff,
	)
}

func (s *server) processEnterpriseHRISWebhookReceipt(
	r *http.Request,
	receipt enterprise.HRISWebhookReceipt,
	recordDLQ bool,
) error {
	if s.enterpriseSvc == nil || s.accessSvc == nil || s.hrisNormalizerRegistry == nil {
		return nil
	}
	if strings.TrimSpace(receipt.Status) != "processing" {
		updated, err := s.enterpriseSvc.MarkHRISWebhookReceiptStarted(receipt.TenantID, receipt.ID)
		if err == nil {
			receipt = updated
		}
	}

	normalized, err := s.hrisNormalizerRegistry.NormalizeWebhook(receipt)
	if err != nil {
		switch {
		case errors.Is(err, hris.ErrDeferredWebhookEvent):
			s.appendAuditLog(
				r,
				receipt.TenantID,
				"enterprise_hris_webhook_processing_deferred",
				fmt.Sprintf(
					"receipt_id=%s,connector_id=%s,vendor=%s,event_type=%s,error=%s",
					receipt.ID,
					receipt.ConnectorID,
					receipt.Vendor,
					receipt.EventType,
					err.Error(),
				),
				"enterprise_webhook",
			)
			_, _ = s.enterpriseSvc.MarkHRISWebhookReceiptSkipped(receipt.TenantID, receipt.ID, err.Error())
		case errors.Is(err, hris.ErrNormalizerNotFound),
			errors.Is(err, hris.ErrUnsupportedWebhookEvent):
			s.appendAuditLog(
				r,
				receipt.TenantID,
				"enterprise_hris_webhook_processing_skipped",
				fmt.Sprintf(
					"receipt_id=%s,connector_id=%s,vendor=%s,event_type=%s,error=%s",
					receipt.ID,
					receipt.ConnectorID,
					receipt.Vendor,
					receipt.EventType,
					err.Error(),
				),
				"enterprise_webhook",
			)
			_, _ = s.enterpriseSvc.MarkHRISWebhookReceiptSkipped(receipt.TenantID, receipt.ID, err.Error())
		default:
			s.appendAuditLog(
				r,
				receipt.TenantID,
				"enterprise_hris_webhook_processing_failed",
				fmt.Sprintf(
					"receipt_id=%s,connector_id=%s,vendor=%s,event_type=%s,error=%s",
					receipt.ID,
					receipt.ConnectorID,
					receipt.Vendor,
					receipt.EventType,
					err.Error(),
				),
				"enterprise_webhook",
			)
			s.appendEnterpriseHRISWebhookProcessingAlertAudit(
				receipt.TenantID,
				receipt.ConnectorID,
				receipt.Vendor,
				receipt.EventType,
				receipt.RequestID,
				"normalize",
			)
			if recordDLQ {
				if dlqErr := s.appendEnterpriseHRISWebhookDLQFailure(hris.DeadLetterFailureInput{
					TenantID:      receipt.TenantID,
					ConnectorID:   receipt.ConnectorID,
					Vendor:        receipt.Vendor,
					ReceiptID:     receipt.ID,
					RequestID:     receipt.RequestID,
					EventType:     receipt.EventType,
					FailureStage:  "normalize",
					Error:         err.Error(),
					RawPayloadRef: hris.RawPayloadRef(receipt),
				}); dlqErr == nil {
					_, _ = s.enterpriseSvc.MarkHRISWebhookReceiptDLQ(receipt.TenantID, receipt.ID, err)
				} else {
					s.loggerOrDefault().Error(
						"enterprise hris webhook receipt dlq append failed",
						"tenant_id", receipt.TenantID,
						"receipt_id", receipt.ID,
						"failure_stage", "normalize",
						"err", dlqErr,
					)
					s.markEnterpriseHRISWebhookReceiptRetryableFailure(receipt, err)
				}
			} else {
				s.markEnterpriseHRISWebhookReceiptRetryableFailure(receipt, err)
			}
		}
		if errors.Is(err, hris.ErrDeferredWebhookEvent) ||
			errors.Is(err, hris.ErrNormalizerNotFound) ||
			errors.Is(err, hris.ErrUnsupportedWebhookEvent) {
			return nil
		}
		return err
	}
	if normalized, err = s.prepareEnterpriseHRISWebhookSync(normalized, receipt); err != nil {
		s.appendAuditLog(
			r,
			receipt.TenantID,
			"enterprise_hris_webhook_processing_failed",
			fmt.Sprintf(
				"receipt_id=%s,connector_id=%s,vendor=%s,event_type=%s,request_id=%s,error=%s",
				receipt.ID,
				receipt.ConnectorID,
				receipt.Vendor,
				normalized.EventType,
				normalized.RequestID,
				err.Error(),
			),
			"enterprise_webhook",
		)
		s.appendEnterpriseHRISWebhookProcessingAlertAudit(
			receipt.TenantID,
			receipt.ConnectorID,
			receipt.Vendor,
			normalized.EventType,
			normalized.RequestID,
			"merge",
		)
		if recordDLQ {
			if dlqErr := s.appendEnterpriseHRISWebhookDLQFailure(hris.DeadLetterFailureInput{
				TenantID:      receipt.TenantID,
				ConnectorID:   receipt.ConnectorID,
				Vendor:        receipt.Vendor,
				ReceiptID:     receipt.ID,
				RequestID:     normalized.RequestID,
				EventType:     normalized.EventType,
				FailureStage:  "merge",
				Error:         err.Error(),
				RawPayloadRef: normalized.RawPayloadRef,
			}); dlqErr == nil {
				_, _ = s.enterpriseSvc.MarkHRISWebhookReceiptDLQ(receipt.TenantID, receipt.ID, err)
			} else {
				s.loggerOrDefault().Error(
					"enterprise hris webhook receipt dlq append failed",
					"tenant_id", receipt.TenantID,
					"receipt_id", receipt.ID,
					"failure_stage", "merge",
					"err", dlqErr,
				)
				s.markEnterpriseHRISWebhookReceiptRetryableFailure(receipt, err)
			}
		} else {
			s.markEnterpriseHRISWebhookReceiptRetryableFailure(receipt, err)
		}
		return err
	}

	result, accessCreated, accessUpdated, accessRejected, err := s.enterpriseSvc.SyncEmployeesWithAccessUpsertMetadata(
		normalized.TenantID,
		normalized.Source,
		normalized.Actor,
		normalized.RequestID,
		normalized.ConnectorID,
		normalized.RawPayloadRef,
		normalized.Employees,
		func(items []enterprise.EnterpriseEmployee) (int, int, int, error) {
			return s.accessSvc.UpsertUsersByEmail(normalized.TenantID, enterpriseEmployeesToAccessBatchInputs(items))
		},
	)
	if err != nil {
		s.appendAuditLog(
			r,
			receipt.TenantID,
			"enterprise_hris_webhook_processing_failed",
			fmt.Sprintf(
				"receipt_id=%s,connector_id=%s,vendor=%s,event_type=%s,request_id=%s,error=%s",
				receipt.ID,
				receipt.ConnectorID,
				receipt.Vendor,
				normalized.EventType,
				normalized.RequestID,
				err.Error(),
			),
			"enterprise_sync",
		)
		s.appendEnterpriseHRISWebhookProcessingAlertAudit(
			receipt.TenantID,
			receipt.ConnectorID,
			receipt.Vendor,
			normalized.EventType,
			normalized.RequestID,
			"sync",
		)
		if recordDLQ {
			if dlqErr := s.appendEnterpriseHRISWebhookDLQFailure(hris.DeadLetterFailureInput{
				TenantID:      receipt.TenantID,
				ConnectorID:   receipt.ConnectorID,
				Vendor:        receipt.Vendor,
				ReceiptID:     receipt.ID,
				RequestID:     normalized.RequestID,
				EventType:     normalized.EventType,
				FailureStage:  "sync",
				Error:         err.Error(),
				RawPayloadRef: normalized.RawPayloadRef,
			}); dlqErr == nil {
				_, _ = s.enterpriseSvc.MarkHRISWebhookReceiptDLQ(receipt.TenantID, receipt.ID, err)
			} else {
				s.loggerOrDefault().Error(
					"enterprise hris webhook receipt dlq append failed",
					"tenant_id", receipt.TenantID,
					"receipt_id", receipt.ID,
					"failure_stage", "sync",
					"err", dlqErr,
				)
				s.markEnterpriseHRISWebhookReceiptRetryableFailure(receipt, err)
			}
		} else {
			s.markEnterpriseHRISWebhookReceiptRetryableFailure(receipt, err)
		}
		return err
	}

	s.appendAuditLog(
		r,
		receipt.TenantID,
		"enterprise_hris_webhook_processed",
		fmt.Sprintf(
			"receipt_id=%s,connector_id=%s,vendor=%s,event_type=%s,request_id=%s,job_id=%s,total=%d,created=%d,updated=%d,rejected=%d,access_created=%d,access_updated=%d,access_rejected=%d",
			receipt.ID,
			receipt.ConnectorID,
			receipt.Vendor,
			normalized.EventType,
			normalized.RequestID,
			result.Job.ID,
			result.Job.Total,
			result.Job.Created,
			result.Job.Updated,
			result.Job.Rejected,
			accessCreated,
			accessUpdated,
			accessRejected,
		),
		"enterprise_sync",
	)
	_, _ = s.enterpriseSvc.MarkHRISWebhookReceiptProcessed(receipt.TenantID, receipt.ID)
	return nil
}

func (s *server) prepareEnterpriseHRISWebhookSync(
	normalized hris.NormalizedSyncRequest,
	receipt enterprise.HRISWebhookReceipt,
) (hris.NormalizedSyncRequest, error) {
	if s.enterpriseSvc == nil {
		return normalized, nil
	}
	if !enterpriseHRISWebhookRequiresMerge(receipt.Vendor, normalized.EventType) {
		return normalized, nil
	}

	mergedEmployees, err := mergeEnterpriseHRISWebhookEmployees(
		receipt.Vendor,
		normalized.EventType,
		s.enterpriseSvc.ListEmployees(normalized.TenantID),
		normalized.Employees,
	)
	if err != nil {
		return normalized, err
	}
	normalized.Employees = mergedEmployees
	return normalized, nil
}

func enterpriseHRISWebhookRequiresMerge(vendor, eventType string) bool {
	switch hris.NormalizeVendor(vendor) {
	case "talenta":
		return talenta.RequiresExistingEmployeeMerge(eventType)
	default:
		return false
	}
}

func mergeEnterpriseHRISWebhookEmployees(
	vendor string,
	eventType string,
	existing []enterprise.EnterpriseEmployee,
	patches []enterprise.EmployeeSyncInput,
) ([]enterprise.EmployeeSyncInput, error) {
	if len(patches) == 0 {
		return nil, hris.ErrNormalizedEmployeesRequired
	}

	byExternalID := make(map[string]enterprise.EnterpriseEmployee, len(existing))
	byEmail := make(map[string]enterprise.EnterpriseEmployee, len(existing))
	for i := range existing {
		item := existing[i]
		if nextExternalID := strings.TrimSpace(item.ExternalID); nextExternalID != "" {
			if current, exists := byExternalID[nextExternalID]; !exists || item.LastSyncedAt.After(current.LastSyncedAt) {
				byExternalID[nextExternalID] = item
			}
		}
		if nextEmail := strings.ToLower(strings.TrimSpace(item.Email)); nextEmail != "" {
			if current, exists := byEmail[nextEmail]; !exists || item.LastSyncedAt.After(current.LastSyncedAt) {
				byEmail[nextEmail] = item
			}
		}
	}

	output := make([]enterprise.EmployeeSyncInput, 0, len(patches))
	for i := range patches {
		patch := patches[i]
		current, ok := resolveEnterpriseHRISWebhookEmployee(byExternalID, byEmail, patch)
		if !ok {
			return nil, fmt.Errorf(
				"%w: existing enterprise employee not found for external_id=%s email=%s",
				hris.ErrInvalidWebhookPayload,
				strings.TrimSpace(patch.ExternalID),
				strings.TrimSpace(patch.Email),
			)
		}
		output = append(output, mergeEnterpriseHRISWebhookEmployee(vendor, eventType, current, patch))
	}
	return output, nil
}

func resolveEnterpriseHRISWebhookEmployee(
	byExternalID map[string]enterprise.EnterpriseEmployee,
	byEmail map[string]enterprise.EnterpriseEmployee,
	patch enterprise.EmployeeSyncInput,
) (enterprise.EnterpriseEmployee, bool) {
	if nextExternalID := strings.TrimSpace(patch.ExternalID); nextExternalID != "" {
		if current, exists := byExternalID[nextExternalID]; exists {
			return current, true
		}
	}
	if nextEmail := strings.ToLower(strings.TrimSpace(patch.Email)); nextEmail != "" {
		current, exists := byEmail[nextEmail]
		return current, exists
	}
	return enterprise.EnterpriseEmployee{}, false
}

func mergeEnterpriseHRISWebhookEmployee(
	vendor string,
	eventType string,
	current enterprise.EnterpriseEmployee,
	patch enterprise.EmployeeSyncInput,
) enterprise.EmployeeSyncInput {
	merged := enterprise.EmployeeSyncInput{
		ExternalID:        current.ExternalID,
		EmployeeNumber:    current.EmployeeNumber,
		Email:             current.Email,
		FullName:          current.FullName,
		Department:        current.Department,
		JobTitle:          current.JobTitle,
		Location:          current.Location,
		Phone:             current.Phone,
		ManagerExternalID: current.ManagerExternalID,
		EmploymentStatus:  current.EmploymentStatus,
		JoinDate:          current.JoinDate,
		ResignDate:        current.ResignDate,
		ShiftCode:         current.ShiftCode,
		ScheduleWindow:    current.ScheduleWindow,
		LeaveStatus:       current.LeaveStatus,
		CostCenter:        current.CostCenter,
		PhotoURL:          current.PhotoURL,
		Status:            current.Status,
	}
	if next := strings.TrimSpace(patch.ExternalID); next != "" {
		merged.ExternalID = next
	}
	if next := strings.TrimSpace(patch.EmployeeNumber); next != "" {
		merged.EmployeeNumber = next
	}
	if next := strings.ToLower(strings.TrimSpace(patch.Email)); next != "" {
		merged.Email = next
	}
	if next := strings.TrimSpace(patch.FullName); next != "" {
		merged.FullName = next
	}
	if next := strings.TrimSpace(patch.Department); next != "" {
		merged.Department = next
	}
	if next := strings.TrimSpace(patch.JobTitle); next != "" {
		merged.JobTitle = next
	}
	if next := strings.TrimSpace(patch.Location); next != "" {
		merged.Location = next
	}
	if next := strings.TrimSpace(patch.Phone); next != "" {
		merged.Phone = next
	}
	if next := strings.TrimSpace(patch.ManagerExternalID); next != "" {
		merged.ManagerExternalID = next
	}
	if next := strings.TrimSpace(patch.EmploymentStatus); next != "" {
		merged.EmploymentStatus = next
	}
	if next := strings.TrimSpace(patch.JoinDate); next != "" {
		merged.JoinDate = next
	}
	if next := strings.TrimSpace(patch.ResignDate); next != "" {
		merged.ResignDate = next
	} else if shouldClearEnterpriseHRISWebhookResignDate(vendor, eventType) {
		merged.ResignDate = ""
	}
	if next := strings.TrimSpace(patch.ShiftCode); next != "" {
		merged.ShiftCode = next
	}
	if next := strings.TrimSpace(patch.ScheduleWindow); next != "" {
		merged.ScheduleWindow = next
	}
	if next := strings.TrimSpace(patch.LeaveStatus); next != "" {
		merged.LeaveStatus = next
	}
	if next := strings.TrimSpace(patch.CostCenter); next != "" {
		merged.CostCenter = next
	}
	if next := strings.TrimSpace(patch.PhotoURL); next != "" {
		merged.PhotoURL = next
	}
	if next := strings.TrimSpace(patch.Status); next != "" {
		merged.Status = next
	}
	return merged
}

func shouldClearEnterpriseHRISWebhookResignDate(vendor, eventType string) bool {
	switch hris.NormalizeVendor(vendor) {
	case "talenta":
		return talenta.NormalizeEventType(eventType) == talenta.EventEmployeeResignationCancelled
	default:
		return false
	}
}

func (s *server) appendEnterpriseHRISWebhookDLQFailure(input hris.DeadLetterFailureInput) error {
	if s.hrisDLQSvc == nil {
		return errors.New("enterprise hris webhook dlq service is not configured")
	}
	if _, err := s.hrisDLQSvc.AppendFailure(input); err != nil {
		return err
	}
	s.notifyEnterpriseHRISWebhookDLQWorker()
	return nil
}

func captureHRISWebhookHeaders(header http.Header) map[string]string {
	items := make(map[string]string)
	for key, values := range header {
		if len(values) == 0 {
			continue
		}
		nextKey := strings.ToLower(strings.TrimSpace(key))
		if nextKey == "" {
			continue
		}
		if nextKey != "content-type" && nextKey != "user-agent" && !strings.HasPrefix(nextKey, "x-") {
			continue
		}
		items[nextKey] = strings.TrimSpace(strings.Join(values, ","))
	}
	return items
}

func detectHRISWebhookEventType(header http.Header, payload []byte) string {
	if value := firstNonEmptyHeader(
		header,
		"X-Event-Type",
		"X-Webhook-Event",
		"X-Mekari-Event",
		"X-Mekari-Webhook-Event",
	); value != "" {
		return value
	}
	return firstNonEmptyJSONField(payload, "event_type", "event", "type")
}

func detectHRISWebhookRequestID(header http.Header, payload []byte) string {
	if value := firstNonEmptyHeader(
		header,
		"X-Request-ID",
		"X-Webhook-ID",
		"X-Event-ID",
		"X-Mekari-Request-ID",
		"X-Mekari-Event-ID",
	); value != "" {
		return value
	}
	return firstNonEmptyJSONField(payload, "request_id", "event_id", "webhook_id")
}

func firstNonEmptyHeader(header http.Header, keys ...string) string {
	for i := range keys {
		value := strings.TrimSpace(header.Get(keys[i]))
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyJSONField(payload []byte, keys ...string) string {
	if len(payload) == 0 {
		return ""
	}
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return ""
	}
	for i := range keys {
		value, ok := body[keys[i]]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func requestRemoteIP(r *http.Request) string {
	for _, key := range []string{"CF-Connecting-IP", "X-Real-IP", "X-Forwarded-For"} {
		value := strings.TrimSpace(r.Header.Get(key))
		if value == "" {
			continue
		}
		if key == "X-Forwarded-For" {
			parts := strings.Split(value, ",")
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
		return value
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func (s *server) materializeEnterpriseHRISConnectorSecrets(
	tenantID string,
	vendor string,
	credentialRef string,
	credentialValue string,
	webhookSecretRef string,
	webhookSecretValue string,
	updatedBy string,
) (string, string, error) {
	nextCredentialRef := strings.TrimSpace(credentialRef)
	nextWebhookSecretRef := strings.TrimSpace(webhookSecretRef)
	if s.hrisVaultSvc == nil {
		if strings.TrimSpace(credentialValue) != "" || strings.TrimSpace(webhookSecretValue) != "" {
			return "", "", errors.New("hris vault service is not configured")
		}
		return nextCredentialRef, nextWebhookSecretRef, nil
	}

	if strings.TrimSpace(credentialValue) != "" {
		item, err := s.hrisVaultSvc.UpsertSecret(
			tenantID,
			hris.ConnectorSecretName(vendor, "credential"),
			"connector_credential",
			credentialValue,
			updatedBy,
		)
		if err != nil {
			return "", "", err
		}
		nextCredentialRef = item.Ref
	}
	if strings.TrimSpace(webhookSecretValue) != "" {
		item, err := s.hrisVaultSvc.UpsertSecret(
			tenantID,
			hris.ConnectorSecretName(vendor, "webhook_secret"),
			"webhook_secret",
			webhookSecretValue,
			updatedBy,
		)
		if err != nil {
			return "", "", err
		}
		nextWebhookSecretRef = item.Ref
	}
	return nextCredentialRef, nextWebhookSecretRef, nil
}
