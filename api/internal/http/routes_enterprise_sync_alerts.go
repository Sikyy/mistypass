package httpx

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/wallet"
)

func (s *server) defaultEnterpriseSyncWorkerAlertSubscription(
	tenantID string,
) enterprise.SyncWorkerAlertSubscription {
	return enterprise.SyncWorkerAlertSubscription{
		TenantID:             strings.TrimSpace(tenantID),
		Enabled:              true,
		WorkerAlertThreshold: 3,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email:    true,
			WhatsApp: false,
		},
		ReceiverGroups: []string{"security"},
		UpdatedAt:      time.Now().UTC(),
	}
}

func (s *server) getEnterpriseSyncWorkerAlertSubscription(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrTenantIDRequired.Error())
		return
	}

	record, found := s.enterpriseSvc.GetSyncWorkerAlertSubscription(tenantID)
	if !found {
		record = s.defaultEnterpriseSyncWorkerAlertSubscription(tenantID)
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *server) upsertEnterpriseSyncWorkerAlertSubscription(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID             string   `json:"tenant_id"`
		Enabled              *bool    `json:"enabled"`
		WorkerAlertThreshold int      `json:"worker_alert_threshold"`
		WindowSeconds        int      `json:"window_seconds"`
		CooldownSeconds      int      `json:"cooldown_seconds"`
		ReceiverGroups       []string `json:"receiver_groups"`
		Channels             *struct {
			Email    *bool `json:"email"`
			WhatsApp *bool `json:"whatsapp"`
		} `json:"channels"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrTenantIDRequired.Error())
		return
	}

	current, found := s.enterpriseSvc.GetSyncWorkerAlertSubscription(tenantID)
	if !found {
		current = s.defaultEnterpriseSyncWorkerAlertSubscription(tenantID)
	}

	enabled := current.Enabled
	if request.Enabled != nil {
		enabled = *request.Enabled
	}

	workerAlertThreshold := request.WorkerAlertThreshold
	if workerAlertThreshold == 0 {
		workerAlertThreshold = current.WorkerAlertThreshold
	}

	windowSeconds := request.WindowSeconds
	if windowSeconds == 0 {
		windowSeconds = int(current.WindowSeconds)
	}

	cooldownSeconds := request.CooldownSeconds
	if cooldownSeconds == 0 {
		cooldownSeconds = int(current.CooldownSeconds)
	}

	emailEnabled := current.Channels.Email
	whatsAppEnabled := current.Channels.WhatsApp
	if request.Channels != nil {
		if request.Channels.Email != nil {
			emailEnabled = *request.Channels.Email
		}
		if request.Channels.WhatsApp != nil {
			whatsAppEnabled = *request.Channels.WhatsApp
		}
	}

	receiverGroups := current.ReceiverGroups
	if request.ReceiverGroups != nil {
		receiverGroups = request.ReceiverGroups
	}

	record, err := s.enterpriseSvc.UpsertSyncWorkerAlertSubscription(
		enterprise.SyncWorkerAlertSubscriptionUpsertOptions{
			TenantID:             tenantID,
			Enabled:              enabled,
			WorkerAlertThreshold: workerAlertThreshold,
			Window:               time.Duration(windowSeconds) * time.Second,
			Cooldown:             time.Duration(cooldownSeconds) * time.Second,
			EmailEnabled:         emailEnabled,
			WhatsAppEnabled:      whatsAppEnabled,
			ReceiverGroups:       receiverGroups,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrInvalidSyncWorkerAlertSubscriptionOptions):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	s.appendAuditLog(
		r,
		tenantID,
		"enterprise_sync_worker_alert_subscription_upserted",
		fmt.Sprintf(
			"enabled=%t threshold=%d window_seconds=%d cooldown_seconds=%d",
			record.Enabled,
			record.WorkerAlertThreshold,
			record.WindowSeconds,
			record.CooldownSeconds,
		),
		"enterprise_sync",
	)
	writeJSON(w, http.StatusOK, record)
}

func (s *server) listEnterpriseSyncWorkerAlerts(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	since, until, err := parseRFC3339TimeRange(
		r.URL.Query().Get("since"),
		r.URL.Query().Get("until"),
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	limit := 50
	limitInput := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitInput != "" {
		parsedLimit, err := strconv.Atoi(limitInput)
		if err != nil || parsedLimit < 0 {
			writeError(w, http.StatusBadRequest, "limit must be an integer >= 0")
			return
		}
		limit = parsedLimit
	}

	logs := s.listEnterpriseSyncWorkerAlertLogs(tenantID)
	logs = filterAuditLogsByTimeRange(logs, since, until)
	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": buildEnterpriseSyncWorkerAlerts(logs),
	})
}

func (s *server) listEnterpriseSyncWorkerAlertSummary(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	since, until, err := parseRFC3339TimeRange(
		r.URL.Query().Get("since"),
		r.URL.Query().Get("until"),
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	limit := 50
	limitInput := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitInput != "" {
		parsedLimit, err := strconv.Atoi(limitInput)
		if err != nil || parsedLimit < 0 {
			writeError(w, http.StatusBadRequest, "limit must be an integer >= 0")
			return
		}
		limit = parsedLimit
	}

	logs := s.listEnterpriseSyncWorkerAlertLogs(tenantID)
	logs = filterAuditLogsByTimeRange(logs, since, until)
	items := buildEnterpriseSyncWorkerAlertSummary(logs)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) reconcileEnterpriseSyncWorkerAlertNotifications(tenantID string) error {
	if s.enterpriseSvc == nil || s.walletSvc == nil {
		return nil
	}
	_, err := s.enterpriseSvc.ConfirmSyncWorkerAlertNotifications(enterprise.SyncWorkerAlertNotificationConfirmInput{
		TenantID:    strings.TrimSpace(tenantID),
		ConfirmedAt: time.Now().UTC(),
		Confirm: func(input enterprise.SyncWorkerAlertConfirmationInput) enterprise.SyncWorkerAlertConfirmationResult {
			result := s.walletSvc.ConfirmAlertDelivery(wallet.AlertDeliveryConfirmationInput{
				TenantID:       input.TenantID,
				IdempotencyKey: input.IdempotencyKey,
				ChannelResults: input.ChannelResults,
			})
			return enterprise.SyncWorkerAlertConfirmationResult{
				Confirmed:      result.Confirmed,
				Retryable:      result.Retryable,
				Provider:       result.Provider,
				ProviderError:  result.ProviderError,
				ChannelResults: result.ChannelResults,
			}
		},
	})
	return err
}

func (s *server) listEnterpriseSyncWorkerAlertNotifications(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrTenantIDRequired.Error())
		return
	}

	options, err := parseEnterpriseSyncWorkerAlertNotificationListOptions(r, tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.reconcileEnterpriseSyncWorkerAlertNotifications(tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, s.enterpriseSvc.ListSyncWorkerAlertNotificationPageWithOptions(options))
}

func (s *server) exportEnterpriseSyncWorkerAlertNotificationsCSV(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrTenantIDRequired.Error())
		return
	}

	options, err := parseEnterpriseSyncWorkerAlertNotificationListOptions(r, tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.reconcileEnterpriseSyncWorkerAlertNotifications(tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	options.Offset = 0
	options.Limit = 0

	result := s.enterpriseSvc.ListSyncWorkerAlertNotificationPageWithOptions(options)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"enterprise-sync-worker-alert-notifications.csv\"")
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{
		"id",
		"tenant_id",
		"triggered_at",
		"worker_action",
		"worker_kind",
		"worker_label",
		"fingerprint",
		"connector_id",
		"vendor",
		"event_type",
		"request_id",
		"failure_stage",
		"mode",
		"count",
		"threshold",
		"failed",
		"processed",
		"applied",
		"status",
		"reason",
		"attempt",
		"retryable",
		"next_retry_at",
		"pending_age_seconds",
		"confirm_attempts",
		"last_confirm_attempt_at",
		"last_confirm_result",
		"channels",
		"receiver_groups",
		"provider",
		"provider_error",
		"source_notification_id",
		"restore_status",
		"idempotency_key",
		"channel_results",
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	for i := range result.Items {
		item := result.Items[i]
		nextRetryAt := ""
		if item.NextRetryAt != nil {
			nextRetryAt = item.NextRetryAt.UTC().Format(time.RFC3339)
		}
		pendingAgeSeconds := ""
		if item.PendingAgeSeconds > 0 || (strings.TrimSpace(item.Status) == "failed" &&
			item.Retryable &&
			strings.TrimSpace(item.Reason) == "dispatch_commit_unknown") {
			pendingAgeSeconds = strconv.FormatInt(item.PendingAgeSeconds, 10)
		}
		lastConfirmAttemptAt := ""
		if item.LastConfirmAttemptAt != nil {
			lastConfirmAttemptAt = item.LastConfirmAttemptAt.UTC().Format(time.RFC3339)
		}
		if err := writer.Write([]string{
			item.ID,
			item.TenantID,
			item.TriggeredAt.UTC().Format(time.RFC3339),
			item.WorkerAction,
			item.WorkerKind,
			item.WorkerLabel,
			item.Fingerprint,
			item.ConnectorID,
			item.Vendor,
			item.EventType,
			item.RequestID,
			item.FailureStage,
			item.Mode,
			strconv.Itoa(item.Count),
			strconv.Itoa(item.Threshold),
			strconv.Itoa(item.Failed),
			strconv.Itoa(item.Processed),
			strconv.Itoa(item.Applied),
			item.Status,
			item.Reason,
			strconv.Itoa(item.Attempt),
			strconv.FormatBool(item.Retryable),
			nextRetryAt,
			pendingAgeSeconds,
			strconv.Itoa(item.ConfirmAttempts),
			lastConfirmAttemptAt,
			item.LastConfirmResult,
			strings.Join(item.Channels, "|"),
			strings.Join(item.ReceiverGroups, "|"),
			item.Provider,
			item.ProviderError,
			item.SourceNotificationID,
			item.RestoreStatus,
			item.IdempotencyKey,
			formatEnterpriseSyncWorkerAlertNotificationChannelResultsCSV(item.ChannelResults),
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
}

func parseEnterpriseSyncWorkerAlertNotificationListOptions(
	r *http.Request,
	tenantID string,
) (enterprise.SyncWorkerAlertNotificationListOptions, error) {
	options := enterprise.SyncWorkerAlertNotificationListOptions{
		TenantID: tenantID,
		Limit:    50,
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return enterprise.SyncWorkerAlertNotificationListOptions{}, errors.New("limit must be an integer > 0")
		}
		if parsed > 500 {
			return enterprise.SyncWorkerAlertNotificationListOptions{}, errors.New("limit must be <= 500")
		}
		options.Limit = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return enterprise.SyncWorkerAlertNotificationListOptions{}, errors.New("offset must be an integer >= 0")
		}
		options.Offset = parsed
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" && !isValidEnterpriseSyncWorkerAlertNotificationStatus(status) {
		return enterprise.SyncWorkerAlertNotificationListOptions{}, errors.New("status must be one of sent, failed, skipped")
	}
	options.Status = status
	options.Reason = strings.TrimSpace(r.URL.Query().Get("reason"))
	options.Query = strings.TrimSpace(r.URL.Query().Get("q"))

	retryable, retryableSet, err := parseEnterpriseSyncWorkerAlertNotificationBooleanFilter(
		r.URL.Query().Get("retryable"),
		"retryable",
	)
	if err != nil {
		return enterprise.SyncWorkerAlertNotificationListOptions{}, err
	}
	options.Retryable = retryable
	options.RetryableSet = retryableSet

	dueNow, dueNowSet, err := parseEnterpriseSyncWorkerAlertNotificationBooleanFilter(
		r.URL.Query().Get("due_now"),
		"due_now",
	)
	if err != nil {
		return enterprise.SyncWorkerAlertNotificationListOptions{}, err
	}
	options.DueNow = dueNow
	options.DueNowSet = dueNowSet
	return options, nil
}

func parseEnterpriseSyncWorkerAlertNotificationBooleanFilter(raw string, field string) (bool, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false, false, nil
	}
	parsed, err := strconv.ParseBool(trimmed)
	if err != nil {
		return false, false, fmt.Errorf("%s must be a boolean", field)
	}
	return parsed, true, nil
}

func formatEnterpriseSyncWorkerAlertNotificationChannelResultsCSV(
	results []wallet.JobAlertChannelResult,
) string {
	parts := make([]string, 0, len(results))
	for i := range results {
		result := results[i]
		segments := []string{
			fmt.Sprintf("%s:%s", result.Channel, result.Status),
			conditionalEnterpriseSyncWorkerAlertCSVSegment("reason", result.Reason),
			conditionalEnterpriseSyncWorkerAlertCSVSegment("provider", result.Provider),
			conditionalEnterpriseSyncWorkerAlertCSVSegment("delivery_id", result.ProviderDeliveryID),
			conditionalEnterpriseSyncWorkerAlertCSVSegment("delivery_status", result.ProviderDeliveryStatus),
			conditionalEnterpriseSyncWorkerAlertCSVSegment("error", result.ProviderError),
			conditionalEnterpriseSyncWorkerAlertCSVSegment("receivers", strings.Join(result.Receivers, "|")),
		}
		filteredSegments := make([]string, 0, len(segments))
		for j := range segments {
			if strings.TrimSpace(segments[j]) == "" {
				continue
			}
			filteredSegments = append(filteredSegments, segments[j])
		}
		parts = append(parts, strings.Join(filteredSegments, " / "))
	}
	return strings.Join(parts, " || ")
}

func conditionalEnterpriseSyncWorkerAlertCSVSegment(label string, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return fmt.Sprintf("%s=%s", label, trimmed)
}

func (s *server) retryEnterpriseSyncWorkerAlertNotificationsBatch(w http.ResponseWriter, r *http.Request) {
	if s.enterpriseSvc == nil || s.walletSvc == nil {
		writeError(w, http.StatusInternalServerError, "enterprise sync worker alert dispatch services are not configured")
		return
	}

	var request struct {
		TenantID        string   `json:"tenant_id"`
		Actor           string   `json:"actor"`
		NotificationIDs []string `json:"notification_ids"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrTenantIDRequired.Error())
		return
	}

	notificationIDs := normalizeEnterpriseSyncWorkerAlertNotificationIDs(request.NotificationIDs)
	if len(notificationIDs) == 0 {
		writeError(w, http.StatusBadRequest, enterprise.ErrSyncWorkerAlertNotificationIDsRequired.Error())
		return
	}
	if len(notificationIDs) > 100 {
		writeError(w, http.StatusBadRequest, "notification_ids must contain at most 100 items")
		return
	}

	actor := strings.TrimSpace(request.Actor)
	if actor == "" {
		if user, exists := authenticatedUser(r); exists {
			actor = strings.TrimSpace(user.Email)
		}
	}
	if actor == "" {
		actor = "system"
	}
	if err := s.reconcileEnterpriseSyncWorkerAlertNotifications(tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	result, err := s.enterpriseSvc.BatchRetrySyncWorkerAlertNotifications(enterprise.SyncWorkerAlertNotificationBatchRetryInput{
		TenantID:        tenantID,
		NotificationIDs: notificationIDs,
		RetriedAt:       time.Now().UTC(),
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			delivery := s.walletSvc.DispatchAlert(wallet.AlertDeliveryInput{
				TenantID:       input.TenantID,
				Channels:       input.Channels,
				ReceiverGroups: input.ReceiverGroups,
				IdempotencyKey: input.IdempotencyKey,
				EmailSubject:   input.EmailSubject,
				EmailText:      input.EmailText,
				WhatsAppText:   input.WhatsAppText,
			})
			return enterprise.SyncWorkerAlertDeliveryResult{
				Status:         delivery.Status,
				Reason:         delivery.Reason,
				Provider:       delivery.Provider,
				ProviderError:  delivery.ProviderError,
				Retryable:      delivery.Retryable,
				ChannelResults: delivery.ChannelResults,
			}
		},
	})
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrSyncWorkerAlertNotificationNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrSyncWorkerAlertNotificationIDsRequired),
			errors.Is(err, enterprise.ErrSyncWorkerAlertDispatcherRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	s.appendAuditLog(
		r,
		tenantID,
		"enterprise_sync_worker_alert_notifications_batch_retried",
		fmt.Sprintf(
			"total_notifications=%d retried=%d skipped=%d failed=%d suppressed=%d actor=%s",
			result.TotalNotifications,
			result.Retried,
			result.Skipped,
			result.Failed,
			result.Suppressed,
			actor,
		),
		"enterprise_sync",
	)
	writeJSON(w, http.StatusOK, result)
}

func (s *server) suppressEnterpriseSyncWorkerAlertNotificationsBatch(w http.ResponseWriter, r *http.Request) {
	if s.enterpriseSvc == nil {
		writeError(w, http.StatusInternalServerError, "enterprise sync worker alert dispatch services are not configured")
		return
	}

	var request struct {
		TenantID        string   `json:"tenant_id"`
		Actor           string   `json:"actor"`
		NotificationIDs []string `json:"notification_ids"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrTenantIDRequired.Error())
		return
	}

	notificationIDs := normalizeEnterpriseSyncWorkerAlertNotificationIDs(request.NotificationIDs)
	if len(notificationIDs) == 0 {
		writeError(w, http.StatusBadRequest, enterprise.ErrSyncWorkerAlertNotificationIDsRequired.Error())
		return
	}
	if len(notificationIDs) > 100 {
		writeError(w, http.StatusBadRequest, "notification_ids must contain at most 100 items")
		return
	}

	actor := strings.TrimSpace(request.Actor)
	if actor == "" {
		if user, exists := authenticatedUser(r); exists {
			actor = strings.TrimSpace(user.Email)
		}
	}
	if actor == "" {
		actor = "system"
	}

	result, err := s.enterpriseSvc.BatchSuppressSyncWorkerAlertNotifications(enterprise.SyncWorkerAlertNotificationBatchSuppressInput{
		TenantID:        tenantID,
		NotificationIDs: notificationIDs,
		SuppressedAt:    time.Now().UTC(),
	})
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrSyncWorkerAlertNotificationNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrSyncWorkerAlertNotificationIDsRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	s.appendAuditLog(
		r,
		tenantID,
		"enterprise_sync_worker_alert_notifications_batch_suppressed",
		fmt.Sprintf(
			"total_notifications=%d suppressed=%d skipped=%d actor=%s",
			result.TotalNotifications,
			result.Suppressed,
			result.Skipped,
			actor,
		),
		"enterprise_sync",
	)
	writeJSON(w, http.StatusOK, result)
}

func (s *server) autoRetryEnterpriseSyncWorkerAlertNotifications(w http.ResponseWriter, r *http.Request) {
	if s.enterpriseSvc == nil || s.walletSvc == nil {
		writeError(w, http.StatusInternalServerError, "enterprise sync worker alert dispatch services are not configured")
		return
	}

	var request struct {
		TenantID      string `json:"tenant_id"`
		Actor         string `json:"actor"`
		Limit         int    `json:"limit"`
		MaxAttempts   int    `json:"max_attempts"`
		BaseBackoffMS int    `json:"base_backoff_ms"`
		MaxBackoffMS  int    `json:"max_backoff_ms"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrTenantIDRequired.Error())
		return
	}
	if request.Limit < 0 {
		writeError(w, http.StatusBadRequest, "limit must be an integer >= 0")
		return
	}
	if request.MaxAttempts < 0 {
		writeError(w, http.StatusBadRequest, "max_attempts must be an integer >= 0")
		return
	}
	if request.BaseBackoffMS < 0 {
		writeError(w, http.StatusBadRequest, "base_backoff_ms must be an integer >= 0")
		return
	}
	if request.MaxBackoffMS < 0 {
		writeError(w, http.StatusBadRequest, "max_backoff_ms must be an integer >= 0")
		return
	}

	actor := strings.TrimSpace(request.Actor)
	if actor == "" {
		if user, exists := authenticatedUser(r); exists {
			actor = strings.TrimSpace(user.Email)
		}
	}
	if actor == "" {
		actor = "system"
	}
	if err := s.reconcileEnterpriseSyncWorkerAlertNotifications(tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	options := enterprise.SyncWorkerAlertNotificationAutoRetryInput{
		TenantID:    tenantID,
		Limit:       request.Limit,
		MaxAttempts: request.MaxAttempts,
		RetriedAt:   time.Now().UTC(),
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			delivery := s.walletSvc.DispatchAlert(wallet.AlertDeliveryInput{
				TenantID:       input.TenantID,
				Channels:       input.Channels,
				ReceiverGroups: input.ReceiverGroups,
				IdempotencyKey: input.IdempotencyKey,
				EmailSubject:   input.EmailSubject,
				EmailText:      input.EmailText,
				WhatsAppText:   input.WhatsAppText,
			})
			return enterprise.SyncWorkerAlertDeliveryResult{
				Status:         delivery.Status,
				Reason:         delivery.Reason,
				Provider:       delivery.Provider,
				ProviderError:  delivery.ProviderError,
				Retryable:      delivery.Retryable,
				ChannelResults: delivery.ChannelResults,
			}
		},
	}
	if request.BaseBackoffMS > 0 {
		options.BaseBackoff = time.Duration(request.BaseBackoffMS) * time.Millisecond
	}
	if request.MaxBackoffMS > 0 {
		options.MaxBackoff = time.Duration(request.MaxBackoffMS) * time.Millisecond
	}

	result, err := s.enterpriseSvc.AutoRetrySyncWorkerAlertNotifications(options)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrSyncWorkerAlertDispatcherRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	s.appendAuditLog(
		r,
		tenantID,
		"enterprise_sync_worker_alert_notifications_auto_retried",
		fmt.Sprintf(
			"total_notifications=%d retried=%d skipped=%d failed=%d suppressed=%d actor=%s",
			result.TotalNotifications,
			result.Retried,
			result.Skipped,
			result.Failed,
			result.Suppressed,
			actor,
		),
		"enterprise_sync",
	)
	writeJSON(w, http.StatusOK, result)
}

func (s *server) restoreEnterpriseSyncWorkerAlertNotificationsBatch(w http.ResponseWriter, r *http.Request) {
	if s.enterpriseSvc == nil {
		writeError(w, http.StatusInternalServerError, "enterprise sync worker alert dispatch services are not configured")
		return
	}

	var request struct {
		TenantID        string   `json:"tenant_id"`
		Actor           string   `json:"actor"`
		NotificationIDs []string `json:"notification_ids"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrTenantIDRequired.Error())
		return
	}

	notificationIDs := normalizeEnterpriseSyncWorkerAlertNotificationIDs(request.NotificationIDs)
	if len(notificationIDs) == 0 {
		writeError(w, http.StatusBadRequest, enterprise.ErrSyncWorkerAlertNotificationIDsRequired.Error())
		return
	}
	if len(notificationIDs) > 100 {
		writeError(w, http.StatusBadRequest, "notification_ids must contain at most 100 items")
		return
	}
	restoreSourceByID := indexEnterpriseSyncWorkerAlertNotificationsByID(
		s.enterpriseSvc.ListSyncWorkerAlertNotificationsWithOptions(enterprise.SyncWorkerAlertNotificationListOptions{
			TenantID: tenantID,
		}),
		notificationIDs,
	)

	actor := strings.TrimSpace(request.Actor)
	if actor == "" {
		if user, exists := authenticatedUser(r); exists {
			actor = strings.TrimSpace(user.Email)
		}
	}
	if actor == "" {
		actor = "system"
	}

	result, err := s.enterpriseSvc.BatchRestoreSyncWorkerAlertNotifications(enterprise.SyncWorkerAlertNotificationBatchRestoreInput{
		TenantID:        tenantID,
		NotificationIDs: notificationIDs,
		RestoredAt:      time.Now().UTC(),
	})
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrSyncWorkerAlertNotificationNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrSyncWorkerAlertNotificationIDsRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	s.appendAuditLog(
		r,
		tenantID,
		"enterprise_sync_worker_alert_notifications_batch_restored",
		buildEnterpriseSyncWorkerAlertBatchRestoreAuditTarget(notificationIDs, restoreSourceByID, result, actor),
		"enterprise_sync",
	)
	writeJSON(w, http.StatusOK, result)
}

func indexEnterpriseSyncWorkerAlertNotificationsByID(
	items []enterprise.SyncWorkerAlertNotification,
	notificationIDs []string,
) map[string]enterprise.SyncWorkerAlertNotification {
	if len(items) == 0 || len(notificationIDs) == 0 {
		return map[string]enterprise.SyncWorkerAlertNotification{}
	}
	requested := make(map[string]struct{}, len(notificationIDs))
	for i := range notificationIDs {
		requested[notificationIDs[i]] = struct{}{}
	}
	indexed := make(map[string]enterprise.SyncWorkerAlertNotification, len(notificationIDs))
	for i := range items {
		if _, exists := requested[items[i].ID]; !exists {
			continue
		}
		indexed[items[i].ID] = items[i]
	}
	return indexed
}

func buildEnterpriseSyncWorkerAlertBatchRestoreAuditTarget(
	notificationIDs []string,
	restoreSourceByID map[string]enterprise.SyncWorkerAlertNotification,
	result enterprise.SyncWorkerAlertNotificationBatchRestoreResult,
	actor string,
) string {
	restoredSourceIDs := make([]string, 0, len(result.Items))
	restoredNotificationIDs := make([]string, 0, len(result.Items))
	restoredSourceSet := make(map[string]struct{}, len(result.Items))
	for i := range result.Items {
		sourceID := strings.TrimSpace(result.Items[i].SourceNotificationID)
		if sourceID != "" {
			restoredSourceIDs = append(restoredSourceIDs, sourceID)
			restoredSourceSet[sourceID] = struct{}{}
		}
		if notificationID := strings.TrimSpace(result.Items[i].ID); notificationID != "" {
			restoredNotificationIDs = append(restoredNotificationIDs, notificationID)
		}
	}

	skippedDetails := make([]string, 0, len(notificationIDs))
	for i := range notificationIDs {
		notificationID := notificationIDs[i]
		if _, restored := restoredSourceSet[notificationID]; restored {
			continue
		}
		source, exists := restoreSourceByID[notificationID]
		if !exists {
			skippedDetails = append(skippedDetails, fmt.Sprintf("%s:not_found", notificationID))
			continue
		}
		skippedDetails = append(
			skippedDetails,
			fmt.Sprintf("%s:%s", notificationID, syncWorkerAlertBatchRestoreSkipReason(source)),
		)
	}

	return fmt.Sprintf(
		"total_notifications=%d restored=%d skipped=%d restored_source_ids=%s restored_notification_ids=%s skipped_details=%s actor=%s",
		result.TotalNotifications,
		result.Restored,
		result.Skipped,
		formatEnterpriseSyncWorkerAlertAuditListValue(restoredSourceIDs),
		formatEnterpriseSyncWorkerAlertAuditListValue(restoredNotificationIDs),
		formatEnterpriseSyncWorkerAlertAuditListValue(skippedDetails),
		strings.TrimSpace(actor),
	)
}

func syncWorkerAlertBatchRestoreSkipReason(item enterprise.SyncWorkerAlertNotification) string {
	if strings.TrimSpace(item.Status) != "skipped" || strings.TrimSpace(item.Reason) != "manual_suppressed" {
		return "not_manual_suppressed"
	}
	restoreStatus := strings.TrimSpace(item.RestoreStatus)
	if restoreStatus == "" {
		return "restore_unavailable"
	}
	return restoreStatus
}

func formatEnterpriseSyncWorkerAlertAuditListValue(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ",")
}

func (s *server) dispatchEnterpriseSyncWorkerAlerts(w http.ResponseWriter, r *http.Request) {
	if s.enterpriseSvc == nil || s.walletSvc == nil {
		writeError(w, http.StatusInternalServerError, "enterprise sync worker alert dispatch services are not configured")
		return
	}

	var request struct {
		TenantID      string   `json:"tenant_id"`
		Actor         string   `json:"actor"`
		WorkerActions []string `json:"worker_actions"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrTenantIDRequired.Error())
		return
	}

	workerActions, err := normalizeEnterpriseSyncWorkerAlertActionFilter(request.WorkerActions)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	subscription, found := s.enterpriseSvc.GetSyncWorkerAlertSubscription(tenantID)
	if !found {
		subscription = s.defaultEnterpriseSyncWorkerAlertSubscription(tenantID)
	}

	now := time.Now().UTC()
	logs := s.listEnterpriseSyncWorkerAlertLogs(tenantID)
	since := now.Add(-time.Duration(subscription.WindowSeconds) * time.Second)
	logs = filterAuditLogsByTimeRange(logs, &since, &now)
	latestItems := buildEnterpriseSyncWorkerAlerts(logs)
	dispatchAlerts := buildEnterpriseSyncWorkerAlertDispatchAlerts(
		latestItems,
		subscription.WorkerAlertThreshold,
		workerActions,
	)

	actor := strings.TrimSpace(request.Actor)
	if actor == "" {
		if user, exists := authenticatedUser(r); exists {
			actor = strings.TrimSpace(user.Email)
		}
	}
	if actor == "" {
		actor = "system"
	}
	if err := s.reconcileEnterpriseSyncWorkerAlertNotifications(tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	result, err := s.enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     tenantID,
		Actor:        actor,
		Subscription: subscription,
		Alerts:       dispatchAlerts,
		TriggeredAt:  now,
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			delivery := s.walletSvc.DispatchAlert(wallet.AlertDeliveryInput{
				TenantID:       input.TenantID,
				Channels:       input.Channels,
				ReceiverGroups: input.ReceiverGroups,
				IdempotencyKey: input.IdempotencyKey,
				EmailSubject:   input.EmailSubject,
				EmailText:      input.EmailText,
				WhatsAppText:   input.WhatsAppText,
			})
			return enterprise.SyncWorkerAlertDeliveryResult{
				Status:         delivery.Status,
				Reason:         delivery.Reason,
				Provider:       delivery.Provider,
				ProviderError:  delivery.ProviderError,
				Retryable:      delivery.Retryable,
				ChannelResults: delivery.ChannelResults,
			}
		},
	})
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrSyncWorkerAlertDispatcherRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	s.appendAuditLog(
		r,
		tenantID,
		"enterprise_sync_worker_alert_dispatch_requested",
		fmt.Sprintf(
			"total_alerts=%d dispatched=%d skipped=%d failed=%d",
			result.TotalAlerts,
			result.Dispatched,
			result.Skipped,
			result.Failed,
		),
		"enterprise_sync",
	)
	writeJSON(w, http.StatusOK, result)
}

func (s *server) retryEnterpriseSyncWorkerAlertNotification(w http.ResponseWriter, r *http.Request) {
	if s.enterpriseSvc == nil || s.walletSvc == nil {
		writeError(w, http.StatusInternalServerError, "enterprise sync worker alert dispatch services are not configured")
		return
	}

	var request struct {
		TenantID string `json:"tenant_id"`
		Actor    string `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrTenantIDRequired.Error())
		return
	}

	notificationID := strings.TrimSpace(chi.URLParam(r, "notificationID"))
	if notificationID == "" {
		writeError(w, http.StatusBadRequest, "notificationID is required")
		return
	}

	actor := strings.TrimSpace(request.Actor)
	if actor == "" {
		if user, exists := authenticatedUser(r); exists {
			actor = strings.TrimSpace(user.Email)
		}
	}
	if actor == "" {
		actor = "system"
	}
	if err := s.reconcileEnterpriseSyncWorkerAlertNotifications(tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	record, err := s.enterpriseSvc.RetrySyncWorkerAlertNotification(enterprise.SyncWorkerAlertNotificationRetryInput{
		TenantID:       tenantID,
		NotificationID: notificationID,
		RetriedAt:      time.Now().UTC(),
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			delivery := s.walletSvc.DispatchAlert(wallet.AlertDeliveryInput{
				TenantID:       input.TenantID,
				Channels:       input.Channels,
				ReceiverGroups: input.ReceiverGroups,
				IdempotencyKey: input.IdempotencyKey,
				EmailSubject:   input.EmailSubject,
				EmailText:      input.EmailText,
				WhatsAppText:   input.WhatsAppText,
			})
			return enterprise.SyncWorkerAlertDeliveryResult{
				Status:         delivery.Status,
				Reason:         delivery.Reason,
				Provider:       delivery.Provider,
				ProviderError:  delivery.ProviderError,
				Retryable:      delivery.Retryable,
				ChannelResults: delivery.ChannelResults,
			}
		},
	})
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrSyncWorkerAlertNotificationNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, enterprise.ErrSyncWorkerAlertRetryNotAllowed),
			errors.Is(err, enterprise.ErrSyncWorkerAlertDispatchInFlight):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrSyncWorkerAlertDispatcherRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	s.appendAuditLog(
		r,
		tenantID,
		"enterprise_sync_worker_alert_notification_retried",
		fmt.Sprintf(
			"notification_id=%s source_notification_id=%s status=%s attempt=%d actor=%s",
			record.ID,
			record.SourceNotificationID,
			record.Status,
			record.Attempt,
			actor,
		),
		"enterprise_sync",
	)
	writeJSON(w, http.StatusOK, record)
}

func (s *server) listEnterpriseSyncWorkerAlertLogs(tenantID string) []audit.Log {
	if s.auditSvc == nil {
		return nil
	}

	items := make([]audit.Log, 0)
	for _, action := range enterpriseSyncWorkerAlertActions() {
		items = append(items, s.auditSvc.ListFiltered(tenantID, action, "enterprise_sync_worker", 0)...)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].At.Equal(items[j].At) {
			if items[i].TenantID != items[j].TenantID {
				return items[i].TenantID < items[j].TenantID
			}
			if items[i].Action != items[j].Action {
				return items[i].Action < items[j].Action
			}
			return items[i].ID > items[j].ID
		}
		return items[i].At.After(items[j].At)
	})
	return items
}

func buildEnterpriseSyncWorkerAlertDispatchAlerts(
	items []enterpriseSyncWorkerAlertItem,
	threshold int,
	workerActions []string,
) []enterprise.SyncWorkerAlertDispatchAlert {
	type dispatchBucket struct {
		count  int
		latest enterpriseSyncWorkerAlertItem
	}

	for i := range items {
		items[i].WorkerAction = strings.TrimSpace(items[i].WorkerAction)
	}

	allowedActions := make(map[string]struct{}, len(workerActions))
	for i := range workerActions {
		allowedActions[strings.TrimSpace(workerActions[i])] = struct{}{}
	}

	buckets := make(map[string]dispatchBucket, len(items))
	for i := range items {
		action := items[i].WorkerAction
		if action == "" {
			continue
		}
		if len(allowedActions) > 0 {
			if _, allowed := allowedActions[action]; !allowed {
				continue
			}
		}

		key := enterpriseSyncWorkerAlertDispatchBucketKey(items[i])
		bucket, exists := buckets[key]
		if !exists {
			buckets[key] = dispatchBucket{
				count:  1,
				latest: items[i],
			}
			continue
		}

		bucket.count++
		if items[i].At.After(bucket.latest.At) {
			bucket.latest = items[i]
		}
		buckets[key] = bucket
	}

	keys := make([]string, 0, len(buckets))
	for key, bucket := range buckets {
		if bucket.count < threshold {
			continue
		}
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left := buckets[keys[i]].latest
		right := buckets[keys[j]].latest
		if left.At.Equal(right.At) {
			if left.WorkerAction != right.WorkerAction {
				return left.WorkerAction < right.WorkerAction
			}
			return keys[i] < keys[j]
		}
		return left.At.After(right.At)
	})

	output := make([]enterprise.SyncWorkerAlertDispatchAlert, 0, len(keys))
	for _, key := range keys {
		bucket := buckets[key]
		latest := bucket.latest
		output = append(output, enterprise.SyncWorkerAlertDispatchAlert{
			WorkerAction:          latest.WorkerAction,
			WorkerKind:            latest.WorkerKind,
			WorkerLabel:           latest.WorkerLabel,
			Count:                 bucket.count,
			Threshold:             threshold,
			Failed:                latest.Failed,
			Processed:             latest.Processed,
			Applied:               latest.Applied,
			SkippedByAttemptLimit: latest.SkippedByAttemptLimit,
			SkippedByCooldown:     latest.SkippedByCooldown,
			ConnectorID:           latest.ConnectorID,
			Vendor:                latest.Vendor,
			EventType:             latest.EventType,
			RequestID:             latest.RequestID,
			FailureStage:          latest.FailureStage,
			Mode:                  latest.Mode,
		})
	}
	return output
}

func enterpriseSyncWorkerAlertDispatchBucketKey(item enterpriseSyncWorkerAlertItem) string {
	parts := []string{
		strings.TrimSpace(item.WorkerAction),
		strings.TrimSpace(item.ConnectorID),
		strings.TrimSpace(item.Vendor),
		strings.TrimSpace(item.FailureStage),
		strings.TrimSpace(item.Mode),
		strings.TrimSpace(item.EventType),
	}
	normalized := make([]string, 0, len(parts))
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		normalized = append(normalized, parts[i])
	}
	if len(normalized) == 0 {
		return "unknown"
	}
	return strings.Join(normalized, "|")
}

func normalizeEnterpriseSyncWorkerAlertActionFilter(items []string) ([]string, error) {
	if len(items) == 0 {
		return nil, nil
	}
	allowed := make(map[string]struct{}, len(enterpriseSyncWorkerAlertActions()))
	for _, action := range enterpriseSyncWorkerAlertActions() {
		allowed[action] = struct{}{}
	}

	output := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for i := range items {
		next := strings.TrimSpace(items[i])
		if next == "" {
			continue
		}
		if _, exists := allowed[next]; !exists {
			return nil, fmt.Errorf("unsupported worker_action: %s", next)
		}
		if _, exists := seen[next]; exists {
			continue
		}
		seen[next] = struct{}{}
		output = append(output, next)
	}
	if len(output) == 0 {
		return nil, nil
	}
	return output, nil
}

func normalizeEnterpriseSyncWorkerAlertNotificationIDs(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	output := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for i := range items {
		next := strings.TrimSpace(items[i])
		if next == "" {
			continue
		}
		if _, exists := seen[next]; exists {
			continue
		}
		seen[next] = struct{}{}
		output = append(output, next)
	}
	return output
}

func isValidEnterpriseSyncWorkerAlertNotificationStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "sent", "failed", "skipped":
		return true
	default:
		return false
	}
}
