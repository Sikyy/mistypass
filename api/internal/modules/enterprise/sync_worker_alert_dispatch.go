package enterprise

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/wallet"
	"github.com/mistypass/cloud/api/internal/modules/wallet/alertdispatch"
)

const (
	maxSyncWorkerAlertNotificationLimit     = 5000
	maxSyncWorkerAlertCooldownLimit         = 2000
	defaultSyncWorkerAlertAutoRetryLimit    = 20
	maxSyncWorkerAlertAutoRetryLimit        = 100
	defaultSyncWorkerAlertAutoRetryAttempts = 3
	defaultSyncWorkerAlertDispatchFlightTTL = 5 * time.Minute
	defaultSyncWorkerAlertConfirmationTTL   = 30 * time.Minute
)

var (
	defaultSyncWorkerAlertRetryBaseBackoff = 5 * time.Minute
	defaultSyncWorkerAlertRetryMaxBackoff  = time.Hour
)

type SyncWorkerAlertNotification struct {
	ID                    string                         `json:"id"`
	TenantID              string                         `json:"tenant_id"`
	WorkerAction          string                         `json:"worker_action"`
	WorkerKind            string                         `json:"worker_kind"`
	WorkerLabel           string                         `json:"worker_label"`
	Fingerprint           string                         `json:"fingerprint"`
	Count                 int                            `json:"count"`
	Threshold             int                            `json:"threshold"`
	Failed                int                            `json:"failed"`
	Processed             int                            `json:"processed"`
	Applied               int                            `json:"applied"`
	SkippedByAttemptLimit int                            `json:"skipped_by_attempt_limit,omitempty"`
	SkippedByCooldown     int                            `json:"skipped_by_cooldown,omitempty"`
	ConnectorID           string                         `json:"connector_id,omitempty"`
	Vendor                string                         `json:"vendor,omitempty"`
	EventType             string                         `json:"event_type,omitempty"`
	RequestID             string                         `json:"request_id,omitempty"`
	FailureStage          string                         `json:"failure_stage,omitempty"`
	Mode                  string                         `json:"mode,omitempty"`
	Channels              []string                       `json:"channels,omitempty"`
	ReceiverGroups        []string                       `json:"receiver_groups,omitempty"`
	Status                string                         `json:"status"`
	Reason                string                         `json:"reason,omitempty"`
	IdempotencyKey        string                         `json:"idempotency_key,omitempty"`
	Attempt               int                            `json:"attempt,omitempty"`
	Retryable             bool                           `json:"retryable"`
	Provider              string                         `json:"provider,omitempty"`
	ProviderError         string                         `json:"provider_error,omitempty"`
	ChannelResults        []wallet.JobAlertChannelResult `json:"channel_results,omitempty"`
	SourceNotificationID  string                         `json:"source_notification_id,omitempty"`
	RestoreStatus         string                         `json:"restore_status,omitempty"`
	NextRetryAt           *time.Time                     `json:"next_retry_at,omitempty"`
	PendingAgeSeconds     int64                          `json:"pending_age_seconds,omitempty"`
	ConfirmAttempts       int                            `json:"confirm_attempts,omitempty"`
	LastConfirmAttemptAt  *time.Time                     `json:"last_confirm_attempt_at,omitempty"`
	LastConfirmResult     string                         `json:"last_confirm_result,omitempty"`
	TriggeredAt           time.Time                      `json:"triggered_at"`
}

type SyncWorkerAlertCooldown struct {
	TenantID    string    `json:"tenant_id"`
	Fingerprint string    `json:"fingerprint"`
	LastSentAt  time.Time `json:"last_sent_at"`
}

type SyncWorkerAlertInFlight struct {
	TenantID             string                      `json:"tenant_id"`
	Key                  string                      `json:"key"`
	Token                string                      `json:"token"`
	Kind                 string                      `json:"kind,omitempty"`
	NotificationID       string                      `json:"notification_id,omitempty"`
	SourceNotificationID string                      `json:"source_notification_id,omitempty"`
	Notification         SyncWorkerAlertNotification `json:"notification,omitempty"`
	AcquiredAt           time.Time                   `json:"acquired_at"`
	ExpiresAt            time.Time                   `json:"expires_at"`
}

type SyncWorkerAlertDispatchAlert struct {
	WorkerAction          string
	WorkerKind            string
	WorkerLabel           string
	Count                 int
	Threshold             int
	Failed                int
	Processed             int
	Applied               int
	SkippedByAttemptLimit int
	SkippedByCooldown     int
	ConnectorID           string
	Vendor                string
	EventType             string
	RequestID             string
	FailureStage          string
	Mode                  string
}

type SyncWorkerAlertDeliveryInput struct {
	TenantID       string
	Channels       []string
	ReceiverGroups []string
	IdempotencyKey string
	Attempt        int
	EmailSubject   string
	EmailText      string
	WhatsAppText   string
}

type SyncWorkerAlertDeliveryResult struct {
	Status         string
	Reason         string
	Provider       string
	ProviderError  string
	Retryable      bool
	ChannelResults []wallet.JobAlertChannelResult
}

type SyncWorkerAlertConfirmationInput struct {
	TenantID       string
	NotificationID string
	IdempotencyKey string
	ChannelResults []wallet.JobAlertChannelResult
}

type SyncWorkerAlertConfirmationResult struct {
	Confirmed      bool
	Retryable      bool
	Provider       string
	ProviderError  string
	ChannelResults []wallet.JobAlertChannelResult
}

type SyncWorkerAlertDispatchInput struct {
	TenantID     string
	Actor        string
	Subscription SyncWorkerAlertSubscription
	Alerts       []SyncWorkerAlertDispatchAlert
	Dispatch     func(SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult
	TriggeredAt  time.Time
}

type SyncWorkerAlertDispatchResult struct {
	TenantID    string                        `json:"tenant_id"`
	TotalAlerts int                           `json:"total_alerts"`
	Dispatched  int                           `json:"dispatched"`
	Skipped     int                           `json:"skipped"`
	Failed      int                           `json:"failed"`
	Items       []SyncWorkerAlertNotification `json:"items,omitempty"`
	UpdatedAt   time.Time                     `json:"updated_at"`
}

type SyncWorkerAlertNotificationListOptions struct {
	TenantID     string
	Status       string
	Reason       string
	Query        string
	RetryableSet bool
	Retryable    bool
	DueNowSet    bool
	DueNow       bool
	Offset       int
	Limit        int
	Now          time.Time
}

type SyncWorkerAlertNotificationFilterCounts struct {
	All        int `json:"all"`
	Failed     int `json:"failed"`
	Retryable  int `json:"retryable"`
	Suppressed int `json:"suppressed"`
	DueNow     int `json:"due_now"`
}

type SyncWorkerAlertNotificationStatusCounts struct {
	Sent    int `json:"sent"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

type SyncWorkerAlertNotificationListResult struct {
	Items        []SyncWorkerAlertNotification           `json:"items"`
	Total        int                                     `json:"total"`
	Offset       int                                     `json:"offset"`
	Limit        int                                     `json:"limit"`
	NextOffset   int                                     `json:"next_offset,omitempty"`
	HasMore      bool                                    `json:"has_more"`
	FilterCounts SyncWorkerAlertNotificationFilterCounts `json:"filter_counts"`
	StatusCounts SyncWorkerAlertNotificationStatusCounts `json:"status_counts"`
}

type SyncWorkerAlertNotificationRetryInput struct {
	TenantID       string
	NotificationID string
	Dispatch       func(SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult
	RetriedAt      time.Time
}

type SyncWorkerAlertNotificationConfirmInput struct {
	TenantID    string
	Confirm     func(SyncWorkerAlertConfirmationInput) SyncWorkerAlertConfirmationResult
	ConfirmedAt time.Time
}

type SyncWorkerAlertNotificationConfirmResult struct {
	TenantID           string                        `json:"tenant_id"`
	TotalNotifications int                           `json:"total_notifications"`
	Confirmed          int                           `json:"confirmed"`
	Failed             int                           `json:"failed"`
	Pending            int                           `json:"pending"`
	Items              []SyncWorkerAlertNotification `json:"items,omitempty"`
	UpdatedAt          time.Time                     `json:"updated_at"`
}

type SyncWorkerAlertNotificationBatchRetryInput struct {
	TenantID        string
	NotificationIDs []string
	Dispatch        func(SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult
	RetriedAt       time.Time
}

type SyncWorkerAlertNotificationBatchRetryResult struct {
	TenantID           string                        `json:"tenant_id"`
	TotalNotifications int                           `json:"total_notifications"`
	Retried            int                           `json:"retried"`
	Skipped            int                           `json:"skipped"`
	Failed             int                           `json:"failed"`
	Suppressed         int                           `json:"suppressed"`
	Items              []SyncWorkerAlertNotification `json:"items,omitempty"`
	UpdatedAt          time.Time                     `json:"updated_at"`
}

type SyncWorkerAlertNotificationBatchSuppressInput struct {
	TenantID        string
	NotificationIDs []string
	SuppressedAt    time.Time
}

type SyncWorkerAlertNotificationBatchSuppressResult struct {
	TenantID           string                        `json:"tenant_id"`
	TotalNotifications int                           `json:"total_notifications"`
	Suppressed         int                           `json:"suppressed"`
	Skipped            int                           `json:"skipped"`
	Items              []SyncWorkerAlertNotification `json:"items,omitempty"`
	UpdatedAt          time.Time                     `json:"updated_at"`
}

type SyncWorkerAlertNotificationBatchRestoreInput struct {
	TenantID        string
	NotificationIDs []string
	RestoredAt      time.Time
}

type SyncWorkerAlertNotificationBatchRestoreResult struct {
	TenantID           string                        `json:"tenant_id"`
	TotalNotifications int                           `json:"total_notifications"`
	Restored           int                           `json:"restored"`
	Skipped            int                           `json:"skipped"`
	Items              []SyncWorkerAlertNotification `json:"items,omitempty"`
	UpdatedAt          time.Time                     `json:"updated_at"`
}

type SyncWorkerAlertNotificationAutoRetryInput struct {
	TenantID    string
	Limit       int
	MaxAttempts int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	Dispatch    func(SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult
	RetriedAt   time.Time
}

type SyncWorkerAlertNotificationAutoRetryResult struct {
	TenantID           string                        `json:"tenant_id"`
	TotalNotifications int                           `json:"total_notifications"`
	Retried            int                           `json:"retried"`
	Skipped            int                           `json:"skipped"`
	Failed             int                           `json:"failed"`
	Suppressed         int                           `json:"suppressed"`
	Items              []SyncWorkerAlertNotification `json:"items,omitempty"`
	UpdatedAt          time.Time                     `json:"updated_at"`
}

func (s *Service) ListSyncWorkerAlertNotifications(tenantID string, limit int) []SyncWorkerAlertNotification {
	return s.ListSyncWorkerAlertNotificationsWithOptions(SyncWorkerAlertNotificationListOptions{
		TenantID: tenantID,
		Limit:    limit,
	})
}

func (s *Service) ListSyncWorkerAlertNotificationsWithOptions(
	input SyncWorkerAlertNotificationListOptions,
) []SyncWorkerAlertNotification {
	return s.ListSyncWorkerAlertNotificationPageWithOptions(input).Items
}

func (s *Service) ListSyncWorkerAlertNotificationPageWithOptions(
	input SyncWorkerAlertNotificationListOptions,
) SyncWorkerAlertNotificationListResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshSyncWorkerAlertStateLocked(); err != nil {
		return SyncWorkerAlertNotificationListResult{}
	}

	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	filterTenantID := strings.TrimSpace(input.TenantID)
	baseItems := make([]SyncWorkerAlertNotification, 0, len(s.syncWorkerAlertNotifications))
	for i := range s.syncWorkerAlertNotifications {
		item := s.decorateSyncWorkerAlertNotificationLocked(s.syncWorkerAlertNotifications[i], now)
		if !matchesSyncWorkerAlertNotificationBaseFilters(item, filterTenantID, input.Query) {
			continue
		}
		baseItems = append(baseItems, item)
	}

	filteredItems := make([]SyncWorkerAlertNotification, 0, len(baseItems))
	for i := range baseItems {
		item := baseItems[i]
		if !matchesSyncWorkerAlertNotificationViewFilters(item, input, now) {
			continue
		}
		filteredItems = append(filteredItems, cloneSyncWorkerAlertNotification(item))
	}

	limit := normalizeSyncWorkerAlertNotificationListLimit(input.Limit)
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}
	total := len(filteredItems)
	if offset > total {
		offset = total
	}
	end := total
	if offset+limit < end {
		end = offset + limit
	}
	items := append([]SyncWorkerAlertNotification(nil), filteredItems[offset:end]...)
	result := SyncWorkerAlertNotificationListResult{
		Items:        items,
		Total:        total,
		Offset:       offset,
		Limit:        limit,
		HasMore:      end < total,
		FilterCounts: buildSyncWorkerAlertNotificationFilterCounts(baseItems, now),
		StatusCounts: buildSyncWorkerAlertNotificationStatusCounts(filteredItems),
	}
	if result.HasMore {
		result.NextOffset = end
	}
	return result
}

func normalizeSyncWorkerAlertNotificationListLimit(limit int) int {
	if limit <= 0 || limit > maxSyncWorkerAlertNotificationLimit {
		return maxSyncWorkerAlertNotificationLimit
	}
	return limit
}

func matchesSyncWorkerAlertNotificationBaseFilters(
	item SyncWorkerAlertNotification,
	tenantID string,
	query string,
) bool {
	if tenantID != "" && strings.TrimSpace(item.TenantID) != tenantID {
		return false
	}
	if !matchesSyncWorkerAlertNotificationQuery(item, query) {
		return false
	}
	return true
}

func matchesSyncWorkerAlertNotificationViewFilters(
	item SyncWorkerAlertNotification,
	input SyncWorkerAlertNotificationListOptions,
	now time.Time,
) bool {
	filterStatus := strings.TrimSpace(input.Status)
	if filterStatus != "" && strings.TrimSpace(item.Status) != filterStatus {
		return false
	}
	filterReason := strings.TrimSpace(input.Reason)
	if filterReason != "" && strings.TrimSpace(item.Reason) != filterReason {
		return false
	}
	if input.RetryableSet && item.Retryable != input.Retryable {
		return false
	}
	if input.DueNowSet && isSyncWorkerAlertNotificationDueNow(item, now) != input.DueNow {
		return false
	}
	return true
}

func matchesSyncWorkerAlertNotificationQuery(item SyncWorkerAlertNotification, query string) bool {
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	if normalizedQuery == "" {
		return true
	}
	fields := []string{
		item.ID,
		item.TenantID,
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
		item.Status,
		item.Reason,
		item.IdempotencyKey,
		item.Provider,
		item.ProviderError,
		item.SourceNotificationID,
	}
	if item.NextRetryAt != nil {
		fields = append(fields, item.NextRetryAt.UTC().Format(time.RFC3339))
	}
	if item.PendingAgeSeconds > 0 || isSyncWorkerAlertNotificationConfirmationPending(item) {
		fields = append(fields, strconv.FormatInt(item.PendingAgeSeconds, 10))
	}
	if item.ConfirmAttempts > 0 {
		fields = append(fields, strconv.Itoa(item.ConfirmAttempts))
	}
	if item.LastConfirmAttemptAt != nil {
		fields = append(fields, item.LastConfirmAttemptAt.UTC().Format(time.RFC3339))
	}
	if lastConfirmResult := strings.TrimSpace(item.LastConfirmResult); lastConfirmResult != "" {
		fields = append(fields, lastConfirmResult)
	}
	fields = append(fields, item.TriggeredAt.UTC().Format(time.RFC3339))
	fields = append(fields, item.Channels...)
	fields = append(fields, item.ReceiverGroups...)
	for i := range fields {
		if strings.Contains(strings.ToLower(strings.TrimSpace(fields[i])), normalizedQuery) {
			return true
		}
	}
	for i := range item.ChannelResults {
		channelResult := item.ChannelResults[i]
		if strings.Contains(
			strings.ToLower(strings.Join([]string{
				channelResult.Channel,
				channelResult.Status,
				channelResult.Reason,
				channelResult.Provider,
				channelResult.ProviderDeliveryID,
				channelResult.ProviderDeliveryStatus,
				channelResult.ProviderError,
				strings.Join(channelResult.Receivers, " "),
			}, " ")),
			normalizedQuery,
		) {
			return true
		}
	}
	return false
}

func isSyncWorkerAlertNotificationDueNow(item SyncWorkerAlertNotification, now time.Time) bool {
	if item.Status != "failed" || !item.Retryable || item.NextRetryAt == nil {
		return false
	}
	return !item.NextRetryAt.After(now)
}

func buildSyncWorkerAlertNotificationFilterCounts(
	items []SyncWorkerAlertNotification,
	now time.Time,
) SyncWorkerAlertNotificationFilterCounts {
	counts := SyncWorkerAlertNotificationFilterCounts{}
	for i := range items {
		item := items[i]
		counts.All++
		if item.Status == "failed" {
			counts.Failed++
		}
		if item.Retryable {
			counts.Retryable++
		}
		if item.Status == "skipped" && item.Reason == "manual_suppressed" {
			counts.Suppressed++
		}
		if isSyncWorkerAlertNotificationDueNow(item, now) {
			counts.DueNow++
		}
	}
	return counts
}

func buildSyncWorkerAlertNotificationStatusCounts(
	items []SyncWorkerAlertNotification,
) SyncWorkerAlertNotificationStatusCounts {
	counts := SyncWorkerAlertNotificationStatusCounts{}
	for i := range items {
		switch items[i].Status {
		case "sent":
			counts.Sent++
		case "failed":
			counts.Failed++
		case "skipped":
			counts.Skipped++
		}
	}
	return counts
}

func (s *Service) DispatchSyncWorkerAlerts(input SyncWorkerAlertDispatchInput) (SyncWorkerAlertDispatchResult, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	if tenantID == "" {
		return SyncWorkerAlertDispatchResult{}, ErrTenantIDRequired
	}
	now := input.TriggeredAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	result := SyncWorkerAlertDispatchResult{
		TenantID:    tenantID,
		TotalAlerts: len(input.Alerts),
		UpdatedAt:   now,
	}
	if len(input.Alerts) == 0 {
		return result, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshSyncWorkerAlertStateLocked(); err != nil {
		return SyncWorkerAlertDispatchResult{}, err
	}

	channels := syncWorkerAlertDispatchChannels(input.Subscription.Channels)
	receiverGroups := normalizeSyncWorkerAlertSubscriptionReceiverGroups(input.Subscription.ReceiverGroups)
	cooldown := time.Duration(input.Subscription.CooldownSeconds) * time.Second
	result.Items = make([]SyncWorkerAlertNotification, 0, len(input.Alerts))
	newItems := make([]SyncWorkerAlertNotification, 0, len(input.Alerts))
	cooldownUpdates := make([]SyncWorkerAlertCooldown, 0, len(input.Alerts))
	flights := make([]SyncWorkerAlertInFlight, 0, len(input.Alerts))

	for i := range input.Alerts {
		alert := input.Alerts[i]
		fingerprint := buildSyncWorkerAlertFingerprint(alert)
		threshold := alert.Threshold
		if threshold <= 0 {
			threshold = input.Subscription.WorkerAlertThreshold
		}
		planned := alertdispatch.Plan(alertdispatch.PlanInput{
			Subscription: alertdispatch.Subscription{
				TenantID:       tenantID,
				Enabled:        input.Subscription.Enabled,
				Channels:       channels,
				ReceiverGroups: receiverGroups,
			},
			Alert: alertdispatch.Alert{
				Type:      strings.TrimSpace(alert.WorkerAction),
				ErrorCode: fingerprint,
				Count:     alert.Count,
				Threshold: threshold,
			},
			InCooldown: s.isSyncWorkerAlertInCooldownLocked(tenantID, fingerprint, cooldown, now),
		})

		record := SyncWorkerAlertNotification{
			TenantID:              tenantID,
			WorkerAction:          strings.TrimSpace(alert.WorkerAction),
			WorkerKind:            strings.TrimSpace(alert.WorkerKind),
			WorkerLabel:           strings.TrimSpace(alert.WorkerLabel),
			Fingerprint:           fingerprint,
			Count:                 alert.Count,
			Threshold:             threshold,
			Failed:                alert.Failed,
			Processed:             alert.Processed,
			Applied:               alert.Applied,
			SkippedByAttemptLimit: alert.SkippedByAttemptLimit,
			SkippedByCooldown:     alert.SkippedByCooldown,
			ConnectorID:           strings.TrimSpace(alert.ConnectorID),
			Vendor:                strings.TrimSpace(alert.Vendor),
			EventType:             strings.TrimSpace(alert.EventType),
			RequestID:             strings.TrimSpace(alert.RequestID),
			FailureStage:          strings.TrimSpace(alert.FailureStage),
			Mode:                  strings.TrimSpace(alert.Mode),
			Channels:              append([]string(nil), planned.Channels...),
			ReceiverGroups:        append([]string(nil), planned.ReceiverGroups...),
			Status:                planned.Status,
			Reason:                planned.Reason,
			IdempotencyKey:        planned.IdempotencyKey,
			TriggeredAt:           now,
		}
		id, err := randomID("swa_")
		if err != nil {
			id = fmt.Sprintf("swa_fallback_%d", time.Now().UnixNano())
		}
		record.ID = id
		if record.Status == "ready" {
			if s.hasPendingSyncWorkerAlertRecoveryLocked(tenantID, syncWorkerAlertNotificationLineageKey(record)) {
				record.ID = ""
				record.Status = "skipped"
				record.Reason = "dispatch_recovery_pending"
				record.ChannelResults = buildStaticSyncWorkerAlertChannelResults(record.Channels, record.Status, record.Reason)
			} else {
				dispatchRecord := cloneSyncWorkerAlertNotification(record)
				dispatchRecord.Attempt = s.nextSyncWorkerAlertAttemptLocked(
					tenantID,
					syncWorkerAlertNotificationLineageKey(dispatchRecord),
				)
				flight, acquired, err := s.acquireSyncWorkerAlertDispatchFlightLocked(dispatchRecord, "dispatch", now)
				if err != nil {
					return SyncWorkerAlertDispatchResult{}, err
				}
				if !acquired {
					record.ID = ""
					record.Status = "skipped"
					record.Reason = "dispatch_inflight"
					record.ChannelResults = buildStaticSyncWorkerAlertChannelResults(record.Channels, record.Status, record.Reason)
				} else {
					dispatched, err := s.dispatchSyncWorkerAlertNotificationLocked(dispatchRecord, input.Dispatch, 0, 0)
					if err != nil {
						releaseErr := s.finalizeSyncWorkerAlertDispatchFlightsLocked([]SyncWorkerAlertInFlight{flight}, nil, nil, now)
						if releaseErr != nil {
							return SyncWorkerAlertDispatchResult{}, releaseErr
						}
						return SyncWorkerAlertDispatchResult{}, err
					}
					record = dispatched
					flights = append(flights, flight)
					if record.Status == "sent" {
						cooldownUpdates = append(cooldownUpdates, SyncWorkerAlertCooldown{
							TenantID:    tenantID,
							Fingerprint: fingerprint,
							LastSentAt:  now,
						})
					}
				}
			}
		} else {
			record.ChannelResults = buildStaticSyncWorkerAlertChannelResults(record.Channels, record.Status, record.Reason)
		}

		switch record.Status {
		case "sent":
			result.Dispatched++
		case "failed":
			result.Failed++
		default:
			result.Skipped++
		}
		result.Items = append(result.Items, cloneSyncWorkerAlertNotification(record))
		if record.Reason != "dispatch_inflight" && record.Reason != "dispatch_recovery_pending" {
			newItems = append(newItems, cloneSyncWorkerAlertNotification(record))
		}
	}

	if err := s.finalizeSyncWorkerAlertDispatchFlightsLocked(flights, newItems, cooldownUpdates, now); err != nil {
		return SyncWorkerAlertDispatchResult{}, err
	}
	return result, nil
}

func (s *Service) RetrySyncWorkerAlertNotification(
	input SyncWorkerAlertNotificationRetryInput,
) (SyncWorkerAlertNotification, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	if tenantID == "" {
		return SyncWorkerAlertNotification{}, ErrTenantIDRequired
	}
	notificationID := strings.TrimSpace(input.NotificationID)
	if notificationID == "" {
		return SyncWorkerAlertNotification{}, ErrSyncWorkerAlertNotificationNotFound
	}
	now := input.RetriedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshSyncWorkerAlertStateLocked(); err != nil {
		return SyncWorkerAlertNotification{}, err
	}

	source, found := s.findSyncWorkerAlertNotificationLocked(tenantID, notificationID)
	if !found {
		return SyncWorkerAlertNotification{}, ErrSyncWorkerAlertNotificationNotFound
	}
	record, flight, err := s.retrySyncWorkerAlertNotificationLocked(source, input.Dispatch, now, 0, 0)
	if err != nil {
		return SyncWorkerAlertNotification{}, err
	}
	cooldownUpdates := []SyncWorkerAlertCooldown(nil)
	if record.Status == "sent" {
		cooldownUpdates = append(cooldownUpdates, SyncWorkerAlertCooldown{
			TenantID:    tenantID,
			Fingerprint: syncWorkerAlertNotificationFingerprint(record),
			LastSentAt:  now,
		})
	}
	if err := s.finalizeSyncWorkerAlertDispatchFlightsLocked(
		[]SyncWorkerAlertInFlight{flight},
		[]SyncWorkerAlertNotification{record},
		cooldownUpdates,
		now,
	); err != nil {
		return SyncWorkerAlertNotification{}, err
	}
	return cloneSyncWorkerAlertNotification(record), nil
}

func (s *Service) ConfirmSyncWorkerAlertNotifications(
	input SyncWorkerAlertNotificationConfirmInput,
) (SyncWorkerAlertNotificationConfirmResult, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	if tenantID == "" {
		return SyncWorkerAlertNotificationConfirmResult{}, ErrTenantIDRequired
	}
	if input.Confirm == nil {
		return SyncWorkerAlertNotificationConfirmResult{}, ErrSyncWorkerAlertConfirmationRequired
	}
	now := input.ConfirmedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	result := SyncWorkerAlertNotificationConfirmResult{
		TenantID:  tenantID,
		UpdatedAt: now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.mutateSyncWorkerAlertStateLocked(func() error {
		result.TotalNotifications = 0
		result.Confirmed = 0
		result.Failed = 0
		result.Pending = 0
		result.Items = nil

		seenLineages := make(map[string]struct{}, len(s.syncWorkerAlertNotifications))
		newItems := make([]SyncWorkerAlertNotification, 0)
		cooldownUpdates := make([]SyncWorkerAlertCooldown, 0)

		for i := range s.syncWorkerAlertNotifications {
			item := s.syncWorkerAlertNotifications[i]
			if strings.TrimSpace(item.TenantID) != tenantID {
				continue
			}

			lineageKey := syncWorkerAlertNotificationLineageKey(item)
			if lineageKey == "" {
				lineageKey = "notification:" + strings.TrimSpace(item.ID)
			}
			if _, exists := seenLineages[lineageKey]; exists {
				continue
			}
			seenLineages[lineageKey] = struct{}{}

			if !isSyncWorkerAlertNotificationConfirmationPending(item) {
				continue
			}

			result.TotalNotifications++
			confirmed := input.Confirm(SyncWorkerAlertConfirmationInput{
				TenantID:       tenantID,
				NotificationID: strings.TrimSpace(item.ID),
				IdempotencyKey: strings.TrimSpace(item.IdempotencyKey),
				ChannelResults: cloneSyncWorkerAlertChannelResults(item.ChannelResults),
			})
			item = observeSyncWorkerAlertConfirmationAttempt(item, confirmed, now)
			s.syncWorkerAlertNotifications[i] = cloneSyncWorkerAlertNotification(item)
			if record, finalized := finalizeSyncWorkerAlertConfirmationOutcome(item, confirmed, now); finalized {
				if record.Status == "sent" {
					result.Confirmed++
					result.Items = append(result.Items, cloneSyncWorkerAlertNotification(record))
					newItems = append(newItems, cloneSyncWorkerAlertNotification(record))
					cooldownUpdates = append(cooldownUpdates, SyncWorkerAlertCooldown{
						TenantID:    tenantID,
						Fingerprint: syncWorkerAlertNotificationFingerprint(record),
						LastSentAt:  now.UTC(),
					})
					continue
				}
				result.Failed++
				result.Items = append(result.Items, cloneSyncWorkerAlertNotification(record))
				newItems = append(newItems, cloneSyncWorkerAlertNotification(record))
				continue
			}
			if !confirmed.Confirmed {
				result.Pending++
				continue
			}
		}

		s.prependSyncWorkerAlertNotificationsLocked(newItems)
		s.applySyncWorkerAlertCooldownUpdatesLocked(cooldownUpdates)
		return nil
	}); err != nil {
		return SyncWorkerAlertNotificationConfirmResult{}, err
	}
	return result, nil
}

func (s *Service) BatchRetrySyncWorkerAlertNotifications(
	input SyncWorkerAlertNotificationBatchRetryInput,
) (SyncWorkerAlertNotificationBatchRetryResult, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	if tenantID == "" {
		return SyncWorkerAlertNotificationBatchRetryResult{}, ErrTenantIDRequired
	}
	notificationIDs := normalizeSyncWorkerAlertNotificationIDs(input.NotificationIDs)
	if len(notificationIDs) == 0 {
		return SyncWorkerAlertNotificationBatchRetryResult{}, ErrSyncWorkerAlertNotificationIDsRequired
	}
	now := input.RetriedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	result := SyncWorkerAlertNotificationBatchRetryResult{
		TenantID:           tenantID,
		TotalNotifications: len(notificationIDs),
		UpdatedAt:          now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshSyncWorkerAlertStateLocked(); err != nil {
		return SyncWorkerAlertNotificationBatchRetryResult{}, err
	}

	sources := make([]SyncWorkerAlertNotification, 0, len(notificationIDs))
	for i := range notificationIDs {
		source, found := s.findSyncWorkerAlertNotificationLocked(tenantID, notificationIDs[i])
		if !found {
			return SyncWorkerAlertNotificationBatchRetryResult{}, ErrSyncWorkerAlertNotificationNotFound
		}
		sources = append(sources, source)
	}

	seenFingerprints := make(map[string]struct{}, len(sources))
	newItems := make([]SyncWorkerAlertNotification, 0, len(sources))
	cooldownUpdates := make([]SyncWorkerAlertCooldown, 0, len(sources))
	flights := make([]SyncWorkerAlertInFlight, 0, len(sources))
	for i := range sources {
		source := sources[i]
		fingerprint := syncWorkerAlertNotificationFingerprint(source)
		if _, exists := seenFingerprints[fingerprint]; exists {
			result.Suppressed++
			continue
		}
		seenFingerprints[fingerprint] = struct{}{}

		if source.Status != "failed" || !source.Retryable || !s.isLatestSyncWorkerAlertNotificationHistoryLocked(source) {
			result.Skipped++
			continue
		}

		record, flight, err := s.retrySyncWorkerAlertNotificationLocked(source, input.Dispatch, now, 0, 0)
		if err != nil {
			if errors.Is(err, ErrSyncWorkerAlertDispatchInFlight) {
				result.Skipped++
				continue
			}
			return SyncWorkerAlertNotificationBatchRetryResult{}, err
		}
		switch record.Status {
		case "sent":
			result.Retried++
			flights = append(flights, flight)
			cooldownUpdates = append(cooldownUpdates, SyncWorkerAlertCooldown{
				TenantID:    tenantID,
				Fingerprint: syncWorkerAlertNotificationFingerprint(record),
				LastSentAt:  now,
			})
		case "failed":
			result.Failed++
			flights = append(flights, flight)
		default:
			result.Skipped++
			flights = append(flights, flight)
		}
		result.Items = append(result.Items, cloneSyncWorkerAlertNotification(record))
		newItems = append(newItems, cloneSyncWorkerAlertNotification(record))
	}

	if len(newItems) > 0 {
		if err := s.finalizeSyncWorkerAlertDispatchFlightsLocked(flights, newItems, cooldownUpdates, now); err != nil {
			return SyncWorkerAlertNotificationBatchRetryResult{}, err
		}
	}
	return result, nil
}

func (s *Service) BatchSuppressSyncWorkerAlertNotifications(
	input SyncWorkerAlertNotificationBatchSuppressInput,
) (SyncWorkerAlertNotificationBatchSuppressResult, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	if tenantID == "" {
		return SyncWorkerAlertNotificationBatchSuppressResult{}, ErrTenantIDRequired
	}
	notificationIDs := normalizeSyncWorkerAlertNotificationIDs(input.NotificationIDs)
	if len(notificationIDs) == 0 {
		return SyncWorkerAlertNotificationBatchSuppressResult{}, ErrSyncWorkerAlertNotificationIDsRequired
	}
	now := input.SuppressedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	result := SyncWorkerAlertNotificationBatchSuppressResult{
		TenantID:           tenantID,
		TotalNotifications: len(notificationIDs),
		UpdatedAt:          now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.mutateSyncWorkerAlertStateLocked(func() error {
		result.Suppressed = 0
		result.Skipped = 0
		result.Items = nil

		newItems := make([]SyncWorkerAlertNotification, 0, len(notificationIDs))
		for i := range notificationIDs {
			source, found := s.findSyncWorkerAlertNotificationLocked(tenantID, notificationIDs[i])
			if !found {
				return ErrSyncWorkerAlertNotificationNotFound
			}
			if source.Status != "failed" || !s.isLatestSyncWorkerAlertNotificationHistoryLocked(source) {
				result.Skipped++
				continue
			}

			record := buildSyncWorkerAlertRetryRecord(source, now)
			record.Status = "skipped"
			record.Reason = "manual_suppressed"
			record.Attempt = s.nextSyncWorkerAlertAttemptLocked(tenantID, syncWorkerAlertNotificationLineageKey(record))
			record.NextRetryAt = nil
			record.ChannelResults = buildStaticSyncWorkerAlertChannelResults(record.Channels, record.Status, record.Reason)
			result.Suppressed++
			result.Items = append(result.Items, cloneSyncWorkerAlertNotification(record))
			newItems = append(newItems, cloneSyncWorkerAlertNotification(record))
		}

		s.prependSyncWorkerAlertNotificationsLocked(newItems)
		return nil
	}); err != nil {
		return SyncWorkerAlertNotificationBatchSuppressResult{}, err
	}
	return result, nil
}

func (s *Service) BatchRestoreSyncWorkerAlertNotifications(
	input SyncWorkerAlertNotificationBatchRestoreInput,
) (SyncWorkerAlertNotificationBatchRestoreResult, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	if tenantID == "" {
		return SyncWorkerAlertNotificationBatchRestoreResult{}, ErrTenantIDRequired
	}
	notificationIDs := normalizeSyncWorkerAlertNotificationIDs(input.NotificationIDs)
	if len(notificationIDs) == 0 {
		return SyncWorkerAlertNotificationBatchRestoreResult{}, ErrSyncWorkerAlertNotificationIDsRequired
	}
	now := input.RestoredAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	result := SyncWorkerAlertNotificationBatchRestoreResult{
		TenantID:           tenantID,
		TotalNotifications: len(notificationIDs),
		UpdatedAt:          now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.mutateSyncWorkerAlertStateLocked(func() error {
		result.Restored = 0
		result.Skipped = 0
		result.Items = nil

		newItems := make([]SyncWorkerAlertNotification, 0, len(notificationIDs))
		for i := range notificationIDs {
			source, found := s.findSyncWorkerAlertNotificationLocked(tenantID, notificationIDs[i])
			if !found {
				return ErrSyncWorkerAlertNotificationNotFound
			}
			restoreStatus := s.syncWorkerAlertNotificationRestoreStatusLocked(source)
			if restoreStatus != "ready" {
				result.Skipped++
				continue
			}

			template := cloneSyncWorkerAlertNotification(source)
			if parentID := strings.TrimSpace(source.SourceNotificationID); parentID != "" {
				if parent, found := s.findSyncWorkerAlertNotificationLocked(tenantID, parentID); found {
					template = cloneSyncWorkerAlertNotification(parent)
				}
			}

			record := buildSyncWorkerAlertRetryRecord(template, now)
			record.SourceNotificationID = source.ID
			record.Status = "failed"
			record.Reason = "manual_suppressed_restored"
			record.Attempt = s.nextSyncWorkerAlertAttemptLocked(tenantID, syncWorkerAlertNotificationLineageKey(record))
			record.Retryable = true
			record.Provider = strings.TrimSpace(template.Provider)
			record.ProviderError = strings.TrimSpace(template.ProviderError)
			record.NextRetryAt = cloneTimePointer(&now)
			record.ChannelResults = buildStaticSyncWorkerAlertChannelResults(record.Channels, record.Status, record.Reason)
			result.Restored++
			result.Items = append(result.Items, cloneSyncWorkerAlertNotification(record))
			newItems = append(newItems, cloneSyncWorkerAlertNotification(record))
		}

		s.prependSyncWorkerAlertNotificationsLocked(newItems)
		return nil
	}); err != nil {
		return SyncWorkerAlertNotificationBatchRestoreResult{}, err
	}
	return result, nil
}

func (s *Service) AutoRetrySyncWorkerAlertNotifications(
	input SyncWorkerAlertNotificationAutoRetryInput,
) (SyncWorkerAlertNotificationAutoRetryResult, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	if tenantID == "" {
		return SyncWorkerAlertNotificationAutoRetryResult{}, ErrTenantIDRequired
	}
	if input.Dispatch == nil {
		return SyncWorkerAlertNotificationAutoRetryResult{}, ErrSyncWorkerAlertDispatcherRequired
	}
	now := input.RetriedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	limit := input.Limit
	if limit <= 0 {
		limit = defaultSyncWorkerAlertAutoRetryLimit
	}
	if limit > maxSyncWorkerAlertAutoRetryLimit {
		limit = maxSyncWorkerAlertAutoRetryLimit
	}
	maxAttempts := input.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultSyncWorkerAlertAutoRetryAttempts
	}
	baseBackoff, maxBackoff := normalizeSyncWorkerAlertRetryBackoff(input.BaseBackoff, input.MaxBackoff)

	result := SyncWorkerAlertNotificationAutoRetryResult{
		TenantID:  tenantID,
		UpdatedAt: now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshSyncWorkerAlertStateLocked(); err != nil {
		return SyncWorkerAlertNotificationAutoRetryResult{}, err
	}

	newItems := make([]SyncWorkerAlertNotification, 0, limit)
	cooldownUpdates := make([]SyncWorkerAlertCooldown, 0, limit)
	flights := make([]SyncWorkerAlertInFlight, 0, limit)
	seenFingerprints := make(map[string]struct{}, limit)
	processed := 0

	for i := range s.syncWorkerAlertNotifications {
		item := s.syncWorkerAlertNotifications[i]
		if strings.TrimSpace(item.TenantID) != tenantID {
			continue
		}
		fingerprint := syncWorkerAlertNotificationFingerprint(item)
		if fingerprint == "" {
			fingerprint = "unknown"
		}
		if _, exists := seenFingerprints[fingerprint]; exists {
			if isSyncWorkerAlertNotificationAutoRetryDue(item, now) {
				result.TotalNotifications++
				result.Suppressed++
			}
			continue
		}
		seenFingerprints[fingerprint] = struct{}{}
		if !isSyncWorkerAlertNotificationAutoRetryDue(item, now) {
			continue
		}
		result.TotalNotifications++
		if processed >= limit {
			continue
		}
		processed++
		if item.Attempt >= maxAttempts {
			record := buildSyncWorkerAlertRetryRecord(item, now)
			record.Status = "skipped"
			record.Reason = "auto_retry_attempt_limit"
			record.Attempt = s.nextSyncWorkerAlertAttemptLocked(tenantID, syncWorkerAlertNotificationLineageKey(record))
			record.NextRetryAt = nil
			record.ChannelResults = buildStaticSyncWorkerAlertChannelResults(record.Channels, record.Status, record.Reason)
			result.Skipped++
			result.Items = append(result.Items, cloneSyncWorkerAlertNotification(record))
			newItems = append(newItems, cloneSyncWorkerAlertNotification(record))
			continue
		}

		record, flight, err := s.retrySyncWorkerAlertNotificationLocked(item, input.Dispatch, now, baseBackoff, maxBackoff)
		if err != nil {
			if errors.Is(err, ErrSyncWorkerAlertDispatchInFlight) {
				result.Skipped++
				continue
			}
			return SyncWorkerAlertNotificationAutoRetryResult{}, err
		}
		switch record.Status {
		case "sent":
			result.Retried++
			flights = append(flights, flight)
			cooldownUpdates = append(cooldownUpdates, SyncWorkerAlertCooldown{
				TenantID:    tenantID,
				Fingerprint: syncWorkerAlertNotificationFingerprint(record),
				LastSentAt:  now,
			})
		case "failed":
			result.Failed++
			flights = append(flights, flight)
		default:
			result.Skipped++
			flights = append(flights, flight)
		}
		result.Items = append(result.Items, cloneSyncWorkerAlertNotification(record))
		newItems = append(newItems, cloneSyncWorkerAlertNotification(record))
	}

	if len(newItems) > 0 {
		if err := s.finalizeSyncWorkerAlertDispatchFlightsLocked(flights, newItems, cooldownUpdates, now); err != nil {
			return SyncWorkerAlertNotificationAutoRetryResult{}, err
		}
	}
	return result, nil
}

func normalizeSyncWorkerAlertNotificationIDs(items []string) []string {
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

func (s *Service) findSyncWorkerAlertNotificationLocked(
	tenantID string,
	notificationID string,
) (SyncWorkerAlertNotification, bool) {
	for i := range s.syncWorkerAlertNotifications {
		if s.syncWorkerAlertNotifications[i].TenantID != tenantID {
			continue
		}
		if s.syncWorkerAlertNotifications[i].ID != notificationID {
			continue
		}
		return cloneSyncWorkerAlertNotification(s.syncWorkerAlertNotifications[i]), true
	}
	return SyncWorkerAlertNotification{}, false
}

func (s *Service) decorateSyncWorkerAlertNotificationLocked(
	input SyncWorkerAlertNotification,
	now time.Time,
) SyncWorkerAlertNotification {
	output := cloneSyncWorkerAlertNotification(input)
	output.RestoreStatus = s.syncWorkerAlertNotificationRestoreStatusLocked(input)
	if isSyncWorkerAlertNotificationConfirmationPending(input) {
		if now.IsZero() {
			now = time.Now().UTC()
		}
		pendingAge := now.Sub(input.TriggeredAt.UTC())
		if pendingAge < 0 {
			pendingAge = 0
		}
		output.PendingAgeSeconds = int64(pendingAge.Seconds())
	}
	return output
}

func (s *Service) syncWorkerAlertNotificationRestoreStatusLocked(
	item SyncWorkerAlertNotification,
) string {
	if item.Status != "skipped" || item.Reason != "manual_suppressed" {
		return ""
	}
	tenantID := strings.TrimSpace(item.TenantID)
	lineageKey := syncWorkerAlertNotificationLineageKey(item)
	if lineageKey == "" {
		return "restore_unavailable"
	}
	if latest, found := s.latestSyncWorkerAlertNotificationByLineageLocked(tenantID, lineageKey); found {
		if latest.ID != item.ID {
			return "newer_history_exists"
		}
	}
	if s.hasSentSyncWorkerAlertByLineageLocked(tenantID, lineageKey) {
		return "already_sent"
	}
	return "ready"
}

func (s *Service) isLatestSyncWorkerAlertNotificationHistoryLocked(
	item SyncWorkerAlertNotification,
) bool {
	tenantID := strings.TrimSpace(item.TenantID)
	lineageKey := syncWorkerAlertNotificationLineageKey(item)
	if tenantID == "" || lineageKey == "" {
		return false
	}
	latest, found := s.latestSyncWorkerAlertNotificationByLineageLocked(tenantID, lineageKey)
	if !found {
		return false
	}
	return latest.ID == item.ID
}

func (s *Service) retrySyncWorkerAlertNotificationLocked(
	source SyncWorkerAlertNotification,
	dispatch func(SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult,
	now time.Time,
	baseBackoff time.Duration,
	maxBackoff time.Duration,
) (SyncWorkerAlertNotification, SyncWorkerAlertInFlight, error) {
	if source.Status != "failed" || !source.Retryable || !s.isLatestSyncWorkerAlertNotificationHistoryLocked(source) {
		return SyncWorkerAlertNotification{}, SyncWorkerAlertInFlight{}, ErrSyncWorkerAlertRetryNotAllowed
	}

	record := buildSyncWorkerAlertRetryRecord(source, now)
	tenantID := strings.TrimSpace(record.TenantID)
	if s.hasSentSyncWorkerAlertByLineageLocked(tenantID, syncWorkerAlertNotificationLineageKey(record)) {
		record.Status = "skipped"
		record.Reason = "idempotent_already_sent"
		record.ChannelResults = buildStaticSyncWorkerAlertChannelResults(record.Channels, record.Status, record.Reason)
		return record, SyncWorkerAlertInFlight{}, nil
	}
	record.Attempt = s.nextSyncWorkerAlertAttemptLocked(tenantID, syncWorkerAlertNotificationLineageKey(record))

	flight, acquired, err := s.acquireSyncWorkerAlertDispatchFlightLocked(record, "retry", now)
	if err != nil {
		return SyncWorkerAlertNotification{}, SyncWorkerAlertInFlight{}, err
	}
	if !acquired {
		return SyncWorkerAlertNotification{}, SyncWorkerAlertInFlight{}, ErrSyncWorkerAlertDispatchInFlight
	}

	dispatched, err := s.dispatchSyncWorkerAlertNotificationLocked(record, dispatch, baseBackoff, maxBackoff)
	if err != nil {
		releaseErr := s.finalizeSyncWorkerAlertDispatchFlightsLocked([]SyncWorkerAlertInFlight{flight}, nil, nil, now)
		if releaseErr != nil {
			return SyncWorkerAlertNotification{}, SyncWorkerAlertInFlight{}, releaseErr
		}
		return SyncWorkerAlertNotification{}, SyncWorkerAlertInFlight{}, err
	}
	return dispatched, flight, nil
}

func buildSyncWorkerAlertRetryRecord(
	source SyncWorkerAlertNotification,
	now time.Time,
) SyncWorkerAlertNotification {
	record := cloneSyncWorkerAlertNotification(source)
	id, err := randomID("swa_")
	if err != nil {
		id = fmt.Sprintf("swa_fallback_%d", time.Now().UnixNano())
	}
	record.ID = id
	record.SourceNotificationID = source.ID
	record.TriggeredAt = now
	record.Attempt = 0
	record.Reason = ""
	record.Provider = ""
	record.ProviderError = ""
	record.Retryable = false
	record.ChannelResults = nil
	record.NextRetryAt = nil
	record.ConfirmAttempts = 0
	record.LastConfirmAttemptAt = nil
	record.LastConfirmResult = ""
	record.IdempotencyKey = syncWorkerAlertNotificationIdempotencyKey(record)
	return record
}

func observeSyncWorkerAlertConfirmationAttempt(
	source SyncWorkerAlertNotification,
	confirmed SyncWorkerAlertConfirmationResult,
	now time.Time,
) SyncWorkerAlertNotification {
	record := cloneSyncWorkerAlertNotification(source)
	record.ConfirmAttempts++
	record.LastConfirmAttemptAt = cloneTimePointer(&now)
	record.LastConfirmResult = buildSyncWorkerAlertLastConfirmResult(source, confirmed, now)
	return record
}

func buildSyncWorkerAlertLastConfirmResult(
	source SyncWorkerAlertNotification,
	confirmed SyncWorkerAlertConfirmationResult,
	now time.Time,
) string {
	switch {
	case confirmed.Confirmed:
		return "confirmed"
	case isSyncWorkerAlertConfirmationRejected(confirmed):
		return "rejected"
	case isSyncWorkerAlertConfirmationAgedOut(source, confirmed, now):
		return "timeout"
	case strings.TrimSpace(confirmed.ProviderError) != "":
		return "provider_error"
	default:
		return "pending"
	}
}

func buildSyncWorkerAlertConfirmationRecord(
	source SyncWorkerAlertNotification,
	confirmed SyncWorkerAlertConfirmationResult,
	now time.Time,
) SyncWorkerAlertNotification {
	record := cloneSyncWorkerAlertNotification(source)
	id, err := randomID("swa_")
	if err != nil {
		id = fmt.Sprintf("swa_fallback_%d", time.Now().UnixNano())
	}
	record.ID = id
	record.SourceNotificationID = source.ID
	record.Status = "sent"
	record.Reason = "dispatch_commit_confirmed"
	if record.Attempt <= 0 {
		record.Attempt = 1
	}
	record.Retryable = false
	record.ProviderError = ""
	record.NextRetryAt = nil
	record.TriggeredAt = now.UTC()
	record.Provider = strings.TrimSpace(confirmed.Provider)
	if record.Provider == "" {
		record.Provider = strings.TrimSpace(source.Provider)
	}
	record.ChannelResults = cloneSyncWorkerAlertChannelResults(confirmed.ChannelResults)
	if len(record.ChannelResults) == 0 {
		record.ChannelResults = cloneSyncWorkerAlertChannelResults(source.ChannelResults)
	}
	record.IdempotencyKey = syncWorkerAlertNotificationIdempotencyKey(record)
	return record
}

func buildSyncWorkerAlertConfirmationFailureRecord(
	source SyncWorkerAlertNotification,
	confirmed SyncWorkerAlertConfirmationResult,
	now time.Time,
	reason string,
	nextRetryAt *time.Time,
) SyncWorkerAlertNotification {
	record := cloneSyncWorkerAlertNotification(source)
	id, err := randomID("swa_")
	if err != nil {
		id = fmt.Sprintf("swa_fallback_%d", time.Now().UnixNano())
	}
	record.ID = id
	record.SourceNotificationID = source.ID
	record.Status = "failed"
	record.Reason = strings.TrimSpace(reason)
	if record.Attempt <= 0 {
		record.Attempt = 1
	}
	record.Retryable = true
	record.TriggeredAt = now.UTC()
	record.NextRetryAt = cloneTimePointer(nextRetryAt)
	record.Provider = strings.TrimSpace(confirmed.Provider)
	if record.Provider == "" {
		record.Provider = strings.TrimSpace(source.Provider)
	}
	record.ProviderError = strings.TrimSpace(confirmed.ProviderError)
	if record.ProviderError == "" {
		record.ProviderError = source.ProviderError
	}
	record.ChannelResults = cloneSyncWorkerAlertChannelResults(confirmed.ChannelResults)
	if len(record.ChannelResults) == 0 {
		record.ChannelResults = cloneSyncWorkerAlertChannelResults(source.ChannelResults)
	}
	record.IdempotencyKey = syncWorkerAlertNotificationIdempotencyKey(record)
	return record
}

func finalizeSyncWorkerAlertConfirmationOutcome(
	source SyncWorkerAlertNotification,
	confirmed SyncWorkerAlertConfirmationResult,
	now time.Time,
) (SyncWorkerAlertNotification, bool) {
	if confirmed.Confirmed {
		return buildSyncWorkerAlertConfirmationRecord(source, confirmed, now), true
	}
	if isSyncWorkerAlertConfirmationRejected(confirmed) {
		return buildSyncWorkerAlertConfirmationFailureRecord(
			source,
			confirmed,
			now,
			"dispatch_commit_rejected",
			nil,
		), true
	}
	if isSyncWorkerAlertConfirmationAgedOut(source, confirmed, now) {
		nextRetryAt := now.UTC()
		return buildSyncWorkerAlertConfirmationFailureRecord(
			source,
			confirmed,
			now,
			"dispatch_commit_confirmation_timeout",
			&nextRetryAt,
		), true
	}
	return SyncWorkerAlertNotification{}, false
}

func buildSyncWorkerAlertFingerprint(alert SyncWorkerAlertDispatchAlert) string {
	parts := []string{
		strings.TrimSpace(alert.WorkerAction),
		strings.TrimSpace(alert.ConnectorID),
		strings.TrimSpace(alert.Vendor),
		strings.TrimSpace(alert.FailureStage),
		strings.TrimSpace(alert.Mode),
		strings.TrimSpace(alert.EventType),
	}
	normalized := make([]string, 0, len(parts))
	for i := range parts {
		next := strings.TrimSpace(parts[i])
		if next == "" {
			continue
		}
		normalized = append(normalized, next)
	}
	if len(normalized) == 0 {
		return "unknown"
	}
	return strings.Join(normalized, "|")
}

func syncWorkerAlertDispatchChannels(channels SyncWorkerAlertSubscriptionChannels) []string {
	output := make([]string, 0, 2)
	if channels.Email {
		output = append(output, "email")
	}
	if channels.WhatsApp {
		output = append(output, "whatsapp")
	}
	return output
}

func normalizeSyncWorkerAlertNotificationStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "sent", "failed", "skipped":
		return strings.TrimSpace(status)
	default:
		return "failed"
	}
}

func (s *Service) dispatchSyncWorkerAlertNotificationLocked(
	record SyncWorkerAlertNotification,
	dispatch func(SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult,
	baseBackoff time.Duration,
	maxBackoff time.Duration,
) (SyncWorkerAlertNotification, error) {
	if dispatch == nil {
		return SyncWorkerAlertNotification{}, ErrSyncWorkerAlertDispatcherRequired
	}
	attempt := record.Attempt
	if attempt <= 0 {
		attempt = s.nextSyncWorkerAlertAttemptLocked(
			strings.TrimSpace(record.TenantID),
			syncWorkerAlertNotificationLineageKey(record),
		)
	}
	record.Attempt = attempt
	delivery := dispatch(SyncWorkerAlertDeliveryInput{
		TenantID:       strings.TrimSpace(record.TenantID),
		Channels:       append([]string(nil), record.Channels...),
		ReceiverGroups: append([]string(nil), record.ReceiverGroups...),
		IdempotencyKey: record.IdempotencyKey,
		Attempt:        attempt,
		EmailSubject:   buildSyncWorkerAlertEmailSubject(record),
		EmailText:      buildSyncWorkerAlertEmailText(record),
		WhatsAppText:   buildSyncWorkerAlertWhatsAppText(record),
	})
	record.Status = normalizeSyncWorkerAlertNotificationStatus(delivery.Status)
	record.Reason = strings.TrimSpace(delivery.Reason)
	record.Provider = strings.TrimSpace(delivery.Provider)
	record.ProviderError = strings.TrimSpace(delivery.ProviderError)
	record.Retryable = delivery.Retryable
	record.ChannelResults = cloneSyncWorkerAlertChannelResults(delivery.ChannelResults)
	record.NextRetryAt = nil
	if record.Status == "failed" && record.Retryable {
		record.NextRetryAt = buildSyncWorkerAlertNextRetryAt(record.Attempt, record.TriggeredAt, baseBackoff, maxBackoff)
	}
	return record, nil
}

func (s *Service) nextSyncWorkerAlertAttemptLocked(tenantID, lineageKey string) int {
	nextAttempt := 1
	if lineageKey == "" {
		return nextAttempt
	}
	for i := range s.syncWorkerAlertNotifications {
		if s.syncWorkerAlertNotifications[i].TenantID != tenantID {
			continue
		}
		if syncWorkerAlertNotificationLineageKey(s.syncWorkerAlertNotifications[i]) != lineageKey {
			continue
		}
		if s.syncWorkerAlertNotifications[i].Attempt >= nextAttempt {
			nextAttempt = s.syncWorkerAlertNotifications[i].Attempt + 1
		}
	}
	return nextAttempt
}

func (s *Service) hasSentSyncWorkerAlertByLineageLocked(tenantID, lineageKey string) bool {
	if lineageKey == "" {
		return false
	}
	for i := range s.syncWorkerAlertNotifications {
		if s.syncWorkerAlertNotifications[i].TenantID != tenantID {
			continue
		}
		if syncWorkerAlertNotificationLineageKey(s.syncWorkerAlertNotifications[i]) != lineageKey {
			continue
		}
		if s.syncWorkerAlertNotifications[i].Status == "sent" {
			return true
		}
	}
	return false
}

func (s *Service) latestSyncWorkerAlertNotificationByLineageLocked(
	tenantID string,
	lineageKey string,
) (SyncWorkerAlertNotification, bool) {
	if lineageKey == "" {
		return SyncWorkerAlertNotification{}, false
	}
	for i := range s.syncWorkerAlertNotifications {
		if s.syncWorkerAlertNotifications[i].TenantID != tenantID {
			continue
		}
		if syncWorkerAlertNotificationLineageKey(s.syncWorkerAlertNotifications[i]) != lineageKey {
			continue
		}
		return cloneSyncWorkerAlertNotification(s.syncWorkerAlertNotifications[i]), true
	}
	return SyncWorkerAlertNotification{}, false
}

func (s *Service) acquireSyncWorkerAlertDispatchFlightLocked(
	record SyncWorkerAlertNotification,
	kind string,
	now time.Time,
) (SyncWorkerAlertInFlight, bool, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	key := syncWorkerAlertNotificationDispatchFlightKey(record)
	if key == "" {
		return SyncWorkerAlertInFlight{}, false, ErrSyncWorkerAlertDispatchInFlight
	}
	token, err := randomID("swf_")
	if err != nil {
		token = fmt.Sprintf("swf_fallback_%d", time.Now().UnixNano())
	}
	flight := SyncWorkerAlertInFlight{
		TenantID:             strings.TrimSpace(record.TenantID),
		Key:                  key,
		Token:                token,
		Kind:                 strings.TrimSpace(kind),
		NotificationID:       strings.TrimSpace(record.ID),
		SourceNotificationID: strings.TrimSpace(record.SourceNotificationID),
		Notification:         cloneSyncWorkerAlertNotification(record),
		AcquiredAt:           now.UTC(),
		ExpiresAt:            now.UTC().Add(defaultSyncWorkerAlertDispatchFlightTTL),
	}

	acquired := false
	if err := s.mutateSyncWorkerAlertStateLocked(func() error {
		s.pruneExpiredSyncWorkerAlertInFlightsLocked(now)
		if s.hasActiveSyncWorkerAlertDispatchFlightLocked(flight.TenantID, flight.Key, now) {
			return nil
		}
		s.syncWorkerAlertInFlights = append(
			[]SyncWorkerAlertInFlight{flight},
			s.syncWorkerAlertInFlights...,
		)
		acquired = true
		return nil
	}); err != nil {
		return SyncWorkerAlertInFlight{}, false, err
	}
	return flight, acquired, nil
}

func (s *Service) finalizeSyncWorkerAlertDispatchFlightsLocked(
	flights []SyncWorkerAlertInFlight,
	notifications []SyncWorkerAlertNotification,
	cooldowns []SyncWorkerAlertCooldown,
	now time.Time,
) error {
	if len(flights) == 0 && len(notifications) == 0 && len(cooldowns) == 0 {
		return nil
	}
	return s.mutateSyncWorkerAlertStateLocked(func() error {
		s.pruneExpiredSyncWorkerAlertInFlightsLocked(now)
		for i := range flights {
			s.releaseSyncWorkerAlertDispatchFlightLocked(flights[i].Token)
		}
		s.prependSyncWorkerAlertNotificationsLocked(notifications)
		s.applySyncWorkerAlertCooldownUpdatesLocked(cooldowns)
		return nil
	})
}

func (s *Service) pruneExpiredSyncWorkerAlertInFlightsLocked(now time.Time) {
	if len(s.syncWorkerAlertInFlights) == 0 {
		return
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	filtered := s.syncWorkerAlertInFlights[:0]
	for i := range s.syncWorkerAlertInFlights {
		if s.syncWorkerAlertInFlights[i].ExpiresAt.After(now) {
			filtered = append(filtered, s.syncWorkerAlertInFlights[i])
		}
	}
	s.syncWorkerAlertInFlights = filtered
}

func (s *Service) recoverExpiredSyncWorkerAlertInFlightsLocked(
	now time.Time,
) ([]SyncWorkerAlertNotification, []SyncWorkerAlertCooldown) {
	if len(s.syncWorkerAlertInFlights) == 0 {
		return nil, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	recovered := make([]SyncWorkerAlertNotification, 0)
	cooldowns := make([]SyncWorkerAlertCooldown, 0)
	for i := range s.syncWorkerAlertInFlights {
		flight := s.syncWorkerAlertInFlights[i]
		if flight.ExpiresAt.After(now) {
			continue
		}
		record, cooldown, ok := s.recoverExpiredSyncWorkerAlertInFlightLocked(flight, now)
		if !ok {
			continue
		}
		recovered = append(recovered, record)
		if strings.TrimSpace(cooldown.TenantID) != "" && strings.TrimSpace(cooldown.Fingerprint) != "" {
			cooldowns = append(cooldowns, cooldown)
		}
	}
	if len(recovered) == 0 && len(cooldowns) == 0 {
		return nil, nil
	}
	s.prependSyncWorkerAlertNotificationsLocked(recovered)
	s.applySyncWorkerAlertCooldownUpdatesLocked(cooldowns)
	return recovered, cooldowns
}

func (s *Service) recoverExpiredSyncWorkerAlertInFlightLocked(
	flight SyncWorkerAlertInFlight,
	now time.Time,
) (SyncWorkerAlertNotification, SyncWorkerAlertCooldown, bool) {
	record := cloneSyncWorkerAlertNotification(flight.Notification)
	if strings.TrimSpace(record.ID) == "" {
		return SyncWorkerAlertNotification{}, SyncWorkerAlertCooldown{}, false
	}
	if strings.TrimSpace(record.TenantID) == "" {
		record.TenantID = strings.TrimSpace(flight.TenantID)
	}
	if strings.TrimSpace(record.TenantID) == "" {
		return SyncWorkerAlertNotification{}, SyncWorkerAlertCooldown{}, false
	}
	if _, found := s.findSyncWorkerAlertNotificationLocked(record.TenantID, record.ID); found {
		return SyncWorkerAlertNotification{}, SyncWorkerAlertCooldown{}, false
	}
	if record.Attempt <= 0 {
		record.Attempt = 1
	}
	if record.TriggeredAt.IsZero() {
		if !flight.AcquiredAt.IsZero() {
			record.TriggeredAt = flight.AcquiredAt.UTC()
		} else {
			record.TriggeredAt = now.UTC()
		}
	}
	record.Status = "failed"
	record.Reason = "dispatch_commit_unknown"
	record.Retryable = true
	if strings.TrimSpace(record.ProviderError) == "" {
		record.ProviderError = "dispatch finalize missing after provider call"
	}
	record.ChannelResults = buildStaticSyncWorkerAlertChannelResults(record.Channels, record.Status, record.Reason)
	nextRetryAt := now.UTC()
	record.NextRetryAt = &nextRetryAt

	cooldownAt := record.TriggeredAt.UTC()
	if cooldownAt.IsZero() {
		cooldownAt = now.UTC()
	}
	return record, SyncWorkerAlertCooldown{
		TenantID:    strings.TrimSpace(record.TenantID),
		Fingerprint: syncWorkerAlertNotificationFingerprint(record),
		LastSentAt:  cooldownAt,
	}, true
}

func (s *Service) hasActiveSyncWorkerAlertDispatchFlightLocked(
	tenantID, key string,
	now time.Time,
) bool {
	nextTenantID := strings.TrimSpace(tenantID)
	nextKey := strings.TrimSpace(key)
	if nextTenantID == "" || nextKey == "" {
		return false
	}
	for i := range s.syncWorkerAlertInFlights {
		if s.syncWorkerAlertInFlights[i].TenantID != nextTenantID {
			continue
		}
		if s.syncWorkerAlertInFlights[i].Key != nextKey {
			continue
		}
		if !s.syncWorkerAlertInFlights[i].ExpiresAt.After(now) {
			continue
		}
		return true
	}
	return false
}

func (s *Service) hasPendingSyncWorkerAlertRecoveryLocked(tenantID, lineageKey string) bool {
	if tenantID == "" || lineageKey == "" {
		return false
	}
	latest, found := s.latestSyncWorkerAlertNotificationByLineageLocked(tenantID, lineageKey)
	if !found {
		return false
	}
	return latest.Status == "failed" &&
		latest.Retryable &&
		strings.TrimSpace(latest.Reason) == "dispatch_commit_unknown"
}

func isSyncWorkerAlertNotificationConfirmationPending(item SyncWorkerAlertNotification) bool {
	return strings.TrimSpace(item.Status) == "failed" &&
		item.Retryable &&
		strings.TrimSpace(item.Reason) == "dispatch_commit_unknown"
}

func isSyncWorkerAlertConfirmationRejected(confirmed SyncWorkerAlertConfirmationResult) bool {
	for i := range confirmed.ChannelResults {
		channelResult := confirmed.ChannelResults[i]
		if strings.TrimSpace(channelResult.Status) == "failed" &&
			strings.TrimSpace(channelResult.Reason) == "provider_delivery_failed" {
			return true
		}
		if walletProviderDeliveryStatusRejected(channelResult.ProviderDeliveryStatus) {
			return true
		}
	}
	return false
}

func isSyncWorkerAlertConfirmationAgedOut(
	source SyncWorkerAlertNotification,
	confirmed SyncWorkerAlertConfirmationResult,
	now time.Time,
) bool {
	if strings.TrimSpace(confirmed.ProviderError) != "" {
		return false
	}
	if source.TriggeredAt.IsZero() {
		return false
	}
	return now.UTC().Sub(source.TriggeredAt.UTC()) >= defaultSyncWorkerAlertConfirmationTTL
}

func walletProviderDeliveryStatusRejected(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "bounced", "failed", "rejected", "cancelled", "canceled", "complained", "complaint", "undelivered", "delivery_failed", "blocked":
		return true
	default:
		return false
	}
}

func (s *Service) releaseSyncWorkerAlertDispatchFlightLocked(token string) {
	nextToken := strings.TrimSpace(token)
	if nextToken == "" || len(s.syncWorkerAlertInFlights) == 0 {
		return
	}
	filtered := s.syncWorkerAlertInFlights[:0]
	for i := range s.syncWorkerAlertInFlights {
		if s.syncWorkerAlertInFlights[i].Token == nextToken {
			continue
		}
		filtered = append(filtered, s.syncWorkerAlertInFlights[i])
	}
	s.syncWorkerAlertInFlights = filtered
}

func (s *Service) isSyncWorkerAlertInCooldownLocked(
	tenantID, fingerprint string,
	cooldown time.Duration,
	now time.Time,
) bool {
	if cooldown <= 0 {
		return false
	}
	for i := range s.syncWorkerAlertCooldowns {
		if s.syncWorkerAlertCooldowns[i].TenantID != tenantID {
			continue
		}
		if s.syncWorkerAlertCooldowns[i].Fingerprint != fingerprint {
			continue
		}
		return now.Sub(s.syncWorkerAlertCooldowns[i].LastSentAt) < cooldown
	}
	return false
}

func (s *Service) upsertSyncWorkerAlertCooldownLocked(tenantID, fingerprint string, now time.Time) {
	for i := range s.syncWorkerAlertCooldowns {
		if s.syncWorkerAlertCooldowns[i].TenantID != tenantID {
			continue
		}
		if s.syncWorkerAlertCooldowns[i].Fingerprint != fingerprint {
			continue
		}
		s.syncWorkerAlertCooldowns[i].LastSentAt = now
		return
	}
	s.syncWorkerAlertCooldowns = append(
		[]SyncWorkerAlertCooldown{
			{
				TenantID:    tenantID,
				Fingerprint: fingerprint,
				LastSentAt:  now,
			},
		},
		s.syncWorkerAlertCooldowns...,
	)
	if len(s.syncWorkerAlertCooldowns) > maxSyncWorkerAlertCooldownLimit {
		s.syncWorkerAlertCooldowns = s.syncWorkerAlertCooldowns[:maxSyncWorkerAlertCooldownLimit]
	}
}

func buildStaticSyncWorkerAlertChannelResults(channels []string, status, reason string) []wallet.JobAlertChannelResult {
	if len(channels) == 0 {
		return nil
	}
	output := make([]wallet.JobAlertChannelResult, 0, len(channels))
	seen := make(map[string]struct{}, len(channels))
	for i := range channels {
		next := strings.ToLower(strings.TrimSpace(channels[i]))
		if next == "" {
			continue
		}
		if _, exists := seen[next]; exists {
			continue
		}
		seen[next] = struct{}{}
		output = append(output, wallet.JobAlertChannelResult{
			Channel: next,
			Status:  status,
			Reason:  reason,
		})
	}
	if len(output) == 0 {
		return nil
	}
	return output
}

func normalizeSyncWorkerAlertRetryBackoff(
	base time.Duration,
	max time.Duration,
) (time.Duration, time.Duration) {
	if base <= 0 {
		base = defaultSyncWorkerAlertRetryBaseBackoff
	}
	if max <= 0 {
		max = defaultSyncWorkerAlertRetryMaxBackoff
	}
	if max < base {
		max = base
	}
	return base, max
}

func buildSyncWorkerAlertNextRetryAt(
	attempt int,
	now time.Time,
	baseBackoff time.Duration,
	maxBackoff time.Duration,
) *time.Time {
	baseBackoff, maxBackoff = normalizeSyncWorkerAlertRetryBackoff(baseBackoff, maxBackoff)
	delay := syncWorkerAlertExponentialBackoff(attempt, baseBackoff, maxBackoff)
	if delay <= 0 {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	nextRetryAt := now.UTC().Add(delay)
	return &nextRetryAt
}

func syncWorkerAlertExponentialBackoff(retryCount int, base, max time.Duration) time.Duration {
	if retryCount <= 0 || base <= 0 {
		return 0
	}
	delay := base
	for i := 1; i < retryCount; i++ {
		delay *= 2
		if delay >= max {
			return max
		}
	}
	if delay > max {
		return max
	}
	return delay
}

func isSyncWorkerAlertNotificationAutoRetryDue(
	item SyncWorkerAlertNotification,
	now time.Time,
) bool {
	if item.Status != "failed" || !item.Retryable || item.NextRetryAt == nil {
		return false
	}
	return !item.NextRetryAt.After(now)
}

func cloneTimePointer(input *time.Time) *time.Time {
	if input == nil {
		return nil
	}
	output := input.UTC()
	return &output
}

func cloneSyncWorkerAlertChannelResults(items []wallet.JobAlertChannelResult) []wallet.JobAlertChannelResult {
	if len(items) == 0 {
		return nil
	}
	output := make([]wallet.JobAlertChannelResult, 0, len(items))
	for i := range items {
		item := items[i]
		item.Receivers = append([]string(nil), items[i].Receivers...)
		output = append(output, item)
	}
	return output
}

func syncWorkerAlertNotificationFingerprint(record SyncWorkerAlertNotification) string {
	if fingerprint := strings.TrimSpace(record.Fingerprint); fingerprint != "" {
		return fingerprint
	}
	return buildSyncWorkerAlertFingerprint(SyncWorkerAlertDispatchAlert{
		WorkerAction: strings.TrimSpace(record.WorkerAction),
		ConnectorID:  strings.TrimSpace(record.ConnectorID),
		Vendor:       strings.TrimSpace(record.Vendor),
		EventType:    strings.TrimSpace(record.EventType),
		FailureStage: strings.TrimSpace(record.FailureStage),
		Mode:         strings.TrimSpace(record.Mode),
	})
}

func syncWorkerAlertNotificationIdempotencyKey(record SyncWorkerAlertNotification) string {
	if key := strings.TrimSpace(record.IdempotencyKey); key != "" {
		return key
	}
	return alertdispatch.BuildNotificationIdempotencyKey(
		strings.TrimSpace(record.TenantID),
		strings.TrimSpace(record.WorkerAction),
		syncWorkerAlertNotificationFingerprint(record),
		record.Threshold,
	)
}

func syncWorkerAlertNotificationLineageKey(record SyncWorkerAlertNotification) string {
	tenantID := strings.TrimSpace(record.TenantID)
	workerAction := strings.TrimSpace(record.WorkerAction)
	fingerprint := syncWorkerAlertNotificationFingerprint(record)
	if tenantID == "" || workerAction == "" || fingerprint == "" {
		return ""
	}
	parts := []string{
		strings.ToLower(tenantID),
		strings.ToLower(workerAction),
		strings.ToLower(fingerprint),
	}
	if requestID := strings.TrimSpace(record.RequestID); requestID != "" {
		parts = append(parts, strings.ToLower(requestID))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:12])
}

func syncWorkerAlertNotificationDispatchFlightKey(record SyncWorkerAlertNotification) string {
	if lineageKey := syncWorkerAlertNotificationLineageKey(record); lineageKey != "" {
		return lineageKey
	}
	if key := strings.TrimSpace(record.IdempotencyKey); key != "" {
		return key
	}
	return syncWorkerAlertNotificationFingerprint(record)
}

func buildSyncWorkerAlertEmailSubject(record SyncWorkerAlertNotification) string {
	label := strings.TrimSpace(record.WorkerLabel)
	if label == "" {
		label = strings.TrimSpace(record.WorkerAction)
	}
	if label == "" {
		label = "Enterprise Sync Worker Alert"
	}
	return fmt.Sprintf("[Enterprise Sync Worker Alert] %s %s", strings.TrimSpace(record.TenantID), label)
}

func buildSyncWorkerAlertEmailText(record SyncWorkerAlertNotification) string {
	lines := []string{
		fmt.Sprintf("Tenant: %s", strings.TrimSpace(record.TenantID)),
		fmt.Sprintf("Worker: %s", syncWorkerAlertDisplayName(record)),
		fmt.Sprintf("Action: %s", strings.TrimSpace(record.WorkerAction)),
		fmt.Sprintf("Alert count in window: %d", record.Count),
		fmt.Sprintf("Threshold: %d", record.Threshold),
		fmt.Sprintf("Latest failed: %d", record.Failed),
		fmt.Sprintf("Latest processed: %d", record.Processed),
		fmt.Sprintf("Latest applied: %d", record.Applied),
	}
	appendSyncWorkerAlertMetadataLines(&lines, record)
	lines = append(lines, fmt.Sprintf("Triggered at: %s", record.TriggeredAt.UTC().Format(time.RFC3339)))
	return strings.Join(lines, "\n")
}

func buildSyncWorkerAlertWhatsAppText(record SyncWorkerAlertNotification) string {
	parts := []string{
		"Enterprise sync worker alert",
		syncWorkerAlertDisplayName(record),
		fmt.Sprintf("count=%d", record.Count),
		fmt.Sprintf("threshold=%d", record.Threshold),
		fmt.Sprintf("failed=%d", record.Failed),
	}
	if connectorID := strings.TrimSpace(record.ConnectorID); connectorID != "" {
		parts = append(parts, "connector="+connectorID)
	}
	if failureStage := strings.TrimSpace(record.FailureStage); failureStage != "" {
		parts = append(parts, "failure_stage="+failureStage)
	}
	if mode := strings.TrimSpace(record.Mode); mode != "" {
		parts = append(parts, "mode="+mode)
	}
	return strings.Join(parts, " | ")
}

func syncWorkerAlertDisplayName(record SyncWorkerAlertNotification) string {
	if label := strings.TrimSpace(record.WorkerLabel); label != "" {
		return label
	}
	if kind := strings.TrimSpace(record.WorkerKind); kind != "" {
		return kind
	}
	return strings.TrimSpace(record.WorkerAction)
}

func appendSyncWorkerAlertMetadataLines(lines *[]string, record SyncWorkerAlertNotification) {
	if connectorID := strings.TrimSpace(record.ConnectorID); connectorID != "" {
		*lines = append(*lines, fmt.Sprintf("Connector ID: %s", connectorID))
	}
	if vendor := strings.TrimSpace(record.Vendor); vendor != "" {
		*lines = append(*lines, fmt.Sprintf("Vendor: %s", vendor))
	}
	if eventType := strings.TrimSpace(record.EventType); eventType != "" {
		*lines = append(*lines, fmt.Sprintf("Event type: %s", eventType))
	}
	if requestID := strings.TrimSpace(record.RequestID); requestID != "" {
		*lines = append(*lines, fmt.Sprintf("Request ID: %s", requestID))
	}
	if failureStage := strings.TrimSpace(record.FailureStage); failureStage != "" {
		*lines = append(*lines, fmt.Sprintf("Failure stage: %s", failureStage))
	}
	if mode := strings.TrimSpace(record.Mode); mode != "" {
		*lines = append(*lines, fmt.Sprintf("Mode: %s", mode))
	}
}
