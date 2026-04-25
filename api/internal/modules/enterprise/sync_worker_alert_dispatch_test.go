package enterprise

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/wallet"
	"github.com/mistypass/cloud/api/internal/modules/wallet/alertdispatch"
)

func TestDispatchSyncWorkerAlertsPersistsNotificationAndCooldown(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 22, 22, 45, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}
	alert := SyncWorkerAlertDispatchAlert{
		WorkerAction: "enterprise_hris_pull_worker_alert",
		WorkerKind:   "hris_pull",
		WorkerLabel:  "HRIS Pull Reconcile",
		Count:        3,
		Threshold:    2,
		Failed:       2,
		Processed:    3,
		Applied:      1,
		ConnectorID:  "connector-talenta-001",
		Vendor:       "talenta",
		Mode:         "incremental",
	}

	dispatchCalls := 0
	result, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Actor:        "tenant.admin@sudirman.co",
		Subscription: subscription,
		Alerts:       []SyncWorkerAlertDispatchAlert{alert},
		TriggeredAt:  now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			dispatchCalls++
			if input.Attempt != 1 {
				t.Fatalf("expected first dispatch attempt=1, got %d", input.Attempt)
			}
			if input.EmailSubject == "" || input.EmailText == "" || input.WhatsAppText == "" {
				t.Fatalf("expected delivery messages to be populated: %+v", input)
			}
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
				ChannelResults: []wallet.JobAlertChannelResult{
					{
						Channel:   "email",
						Status:    "sent",
						Provider:  "mock",
						Receivers: []string{"security@mistypass.local"},
					},
				},
			}
		},
	})
	if err != nil {
		t.Fatalf("expected dispatch to succeed: %v", err)
	}
	if dispatchCalls != 1 {
		t.Fatalf("expected one delivery call, got %d", dispatchCalls)
	}
	if result.TotalAlerts != 1 || result.Dispatched != 1 || result.Failed != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected first dispatch summary: %+v", result)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one notification item, got %d", len(result.Items))
	}
	if result.Items[0].Status != "sent" || result.Items[0].Attempt != 1 {
		t.Fatalf("unexpected sent notification item: %+v", result.Items[0])
	}

	stored := svc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(stored) != 1 {
		t.Fatalf("expected one stored notification, got %d", len(stored))
	}
	if stored[0].Fingerprint != "enterprise_hris_pull_worker_alert|connector-talenta-001|talenta|incremental" {
		t.Fatalf("unexpected notification fingerprint: %s", stored[0].Fingerprint)
	}

	secondResult, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Actor:        "tenant.admin@sudirman.co",
		Subscription: subscription,
		Alerts:       []SyncWorkerAlertDispatchAlert{alert},
		TriggeredAt:  now.Add(5 * time.Minute),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			dispatchCalls++
			return SyncWorkerAlertDeliveryResult{Status: "sent", Provider: "mock"}
		},
	})
	if err != nil {
		t.Fatalf("expected second dispatch to succeed: %v", err)
	}
	if dispatchCalls != 1 {
		t.Fatalf("expected cooldown to avoid second delivery call, got %d", dispatchCalls)
	}
	if secondResult.Dispatched != 0 || secondResult.Skipped != 1 {
		t.Fatalf("unexpected second dispatch summary: %+v", secondResult)
	}
	if len(secondResult.Items) != 1 || secondResult.Items[0].Status != "skipped" || secondResult.Items[0].Reason != "cooldown" {
		t.Fatalf("expected cooldown skip item, got %+v", secondResult.Items)
	}
}

func TestDispatchSyncWorkerAlertsSubscriptionDisabledSkipsWithoutDispatcher(t *testing.T) {
	svc := NewService()
	result, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID: "tenant_demo_jakarta",
		Subscription: SyncWorkerAlertSubscription{
			TenantID:             "tenant_demo_jakarta",
			Enabled:              false,
			WorkerAlertThreshold: 3,
			WindowSeconds:        int64((15 * time.Minute).Seconds()),
			CooldownSeconds:      int64((15 * time.Minute).Seconds()),
			Channels: SyncWorkerAlertSubscriptionChannels{
				Email: true,
			},
			ReceiverGroups: []string{"security"},
		},
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_sync_reconcile_worker_alert",
				WorkerKind:   "sync_reconcile",
				WorkerLabel:  "Enterprise Sync Reconcile",
				Count:        3,
				Threshold:    3,
				Failed:       3,
				Processed:    4,
				Applied:      1,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected disabled subscription dispatch to skip cleanly: %v", err)
	}
	if result.Dispatched != 0 || result.Failed != 0 || result.Skipped != 1 {
		t.Fatalf("unexpected disabled subscription result: %+v", result)
	}
	if len(result.Items) != 1 || result.Items[0].Reason != "subscription_disabled" {
		t.Fatalf("expected subscription_disabled skip, got %+v", result.Items)
	}
	if result.Items[0].ChannelResults[0].Status != "skipped" {
		t.Fatalf("expected skipped channel result, got %+v", result.Items[0].ChannelResults)
	}
}

func TestListSyncWorkerAlertNotificationsWithOptionsFiltersByStatusAndLimit(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 1, 0, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	_, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_pull_worker_alert",
				WorkerKind:   "hris_pull",
				WorkerLabel:  "HRIS Pull Reconcile",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				Mode:         "incremental",
			},
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        2,
				Threshold:    2,
				Failed:       1,
				Processed:    2,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
		},
		TriggeredAt: now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			if strings.Contains(input.EmailSubject, "Webhook") {
				return SyncWorkerAlertDeliveryResult{
					Status:        "failed",
					Reason:        "provider_transient_error",
					Provider:      "mock",
					ProviderError: "temporary outage",
					Retryable:     true,
				}
			}
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected dispatch to succeed: %v", err)
	}

	failedItems := svc.ListSyncWorkerAlertNotificationsWithOptions(SyncWorkerAlertNotificationListOptions{
		TenantID: "tenant_demo_jakarta",
		Status:   "failed",
		Limit:    10,
	})
	if len(failedItems) != 1 || failedItems[0].WorkerAction != "enterprise_hris_webhook_processing_alert" {
		t.Fatalf("expected one failed notification item, got %+v", failedItems)
	}

	limitedItems := svc.ListSyncWorkerAlertNotificationsWithOptions(SyncWorkerAlertNotificationListOptions{
		TenantID: "tenant_demo_jakarta",
		Limit:    1,
	})
	if len(limitedItems) != 1 {
		t.Fatalf("expected one limited notification item, got %d", len(limitedItems))
	}
}

func TestListSyncWorkerAlertNotificationPageWithOptionsSupportsCountsQueryAndPagination(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 1, 30, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	_, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_pull_worker_alert",
				WorkerKind:   "hris_pull",
				WorkerLabel:  "HRIS Pull Reconcile",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				Mode:         "incremental",
			},
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        2,
				Threshold:    2,
				Failed:       1,
				Processed:    2,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
		},
		TriggeredAt: now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			if strings.Contains(input.EmailSubject, "Webhook") {
				return SyncWorkerAlertDeliveryResult{
					Status:        "failed",
					Reason:        "provider_transient_error",
					Provider:      "mock",
					ProviderError: "temporary outage",
					Retryable:     true,
				}
			}
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected dispatch to succeed: %v", err)
	}

	failedItems := svc.ListSyncWorkerAlertNotificationsWithOptions(SyncWorkerAlertNotificationListOptions{
		TenantID: "tenant_demo_jakarta",
		Status:   "failed",
		Limit:    10,
	})
	if len(failedItems) != 1 {
		t.Fatalf("expected one failed notification, got %+v", failedItems)
	}

	_, err = svc.BatchSuppressSyncWorkerAlertNotifications(SyncWorkerAlertNotificationBatchSuppressInput{
		TenantID:        "tenant_demo_jakarta",
		NotificationIDs: []string{failedItems[0].ID},
		SuppressedAt:    now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("expected suppress to succeed: %v", err)
	}

	page := svc.ListSyncWorkerAlertNotificationPageWithOptions(SyncWorkerAlertNotificationListOptions{
		TenantID: "tenant_demo_jakarta",
		Query:    "talenta",
		Offset:   1,
		Limit:    1,
		Now:      now.Add(10 * time.Minute),
	})
	if page.Total != 3 {
		t.Fatalf("expected total 3, got %+v", page)
	}
	if len(page.Items) != 1 || !page.HasMore || page.NextOffset != 2 {
		t.Fatalf("expected second page with next offset, got %+v", page)
	}
	if page.FilterCounts.All != 3 || page.FilterCounts.Failed != 1 || page.FilterCounts.Retryable != 1 || page.FilterCounts.Suppressed != 1 || page.FilterCounts.DueNow != 1 {
		t.Fatalf("unexpected filter counts: %+v", page.FilterCounts)
	}
	if page.StatusCounts.Sent != 1 || page.StatusCounts.Failed != 1 || page.StatusCounts.Skipped != 1 {
		t.Fatalf("unexpected status counts: %+v", page.StatusCounts)
	}

	dueNowPage := svc.ListSyncWorkerAlertNotificationPageWithOptions(SyncWorkerAlertNotificationListOptions{
		TenantID:  "tenant_demo_jakarta",
		DueNowSet: true,
		DueNow:    true,
		Limit:     10,
		Now:       now.Add(10 * time.Minute),
	})
	if dueNowPage.Total != 1 || len(dueNowPage.Items) != 1 {
		t.Fatalf("expected one due-now notification, got %+v", dueNowPage)
	}
	if dueNowPage.Items[0].Status != "failed" || !dueNowPage.Items[0].Retryable {
		t.Fatalf("expected failed retryable due-now notification, got %+v", dueNowPage.Items[0])
	}
}

func TestListSyncWorkerAlertNotificationPageIncludesPendingAgeSeconds(t *testing.T) {
	svc := NewService()
	triggeredAt := time.Date(2026, 4, 23, 6, 0, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	_, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     subscription.TenantID,
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        2,
				Threshold:    2,
				Failed:       1,
				Processed:    2,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
		},
		TriggeredAt: triggeredAt,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "dispatch_commit_unknown",
				Provider:      "resend",
				ProviderError: "dispatch finalize missing after provider call",
				Retryable:     true,
				ChannelResults: []wallet.JobAlertChannelResult{
					{
						Channel:                "email",
						Status:                 "sent",
						Provider:               "resend",
						ProviderDeliveryID:     "email_123",
						ProviderDeliveryStatus: "accepted",
					},
				},
			}
		},
	})
	if err != nil {
		t.Fatalf("expected pending dispatch commit notification: %v", err)
	}

	listedAt := triggeredAt.Add(13 * time.Minute)
	expectedPendingAgeSeconds := int64((13 * time.Minute).Seconds())
	page := svc.ListSyncWorkerAlertNotificationPageWithOptions(SyncWorkerAlertNotificationListOptions{
		TenantID: subscription.TenantID,
		Status:   "failed",
		Query:    strconv.FormatInt(expectedPendingAgeSeconds, 10),
		Limit:    10,
		Now:      listedAt,
	})
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("expected one pending notification matched by pending age query, got %+v", page)
	}
	if page.Items[0].PendingAgeSeconds != expectedPendingAgeSeconds {
		t.Fatalf("expected pending_age_seconds=%d, got %+v", expectedPendingAgeSeconds, page.Items[0])
	}
	if page.Items[0].Reason != "dispatch_commit_unknown" {
		t.Fatalf("expected pending dispatch_commit_unknown record, got %+v", page.Items[0])
	}
}

func TestConfirmSyncWorkerAlertNotificationsTracksPendingAttemptObservability(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 6, 30, 0, 0, time.UTC)
	previousAttemptAt := now.Add(-2 * time.Minute)
	source := SyncWorkerAlertNotification{
		ID:                   "swa_pending_commit_observe_001",
		TenantID:             "tenant_demo_jakarta",
		WorkerAction:         "enterprise_hris_webhook_processing_alert",
		WorkerKind:           "hris_webhook",
		WorkerLabel:          "HRIS Webhook Merge",
		Fingerprint:          "enterprise_hris_webhook_processing_alert|connector-talenta-001|talenta|merge",
		Count:                3,
		Threshold:            2,
		Failed:               2,
		Processed:            3,
		Applied:              1,
		ConnectorID:          "connector-talenta-001",
		Vendor:               "talenta",
		FailureStage:         "merge",
		Channels:             []string{"email"},
		ReceiverGroups:       []string{"security"},
		Status:               "failed",
		Reason:               "dispatch_commit_unknown",
		IdempotencyKey:       "pending-commit-observe-key",
		Attempt:              1,
		Retryable:            true,
		Provider:             "resend",
		ConfirmAttempts:      1,
		LastConfirmAttemptAt: &previousAttemptAt,
		LastConfirmResult:    "pending",
		ChannelResults: []wallet.JobAlertChannelResult{
			{
				Channel:                "email",
				Status:                 "sent",
				Provider:               "resend",
				ProviderDeliveryID:     "email_123",
				ProviderDeliveryStatus: "accepted",
			},
		},
		TriggeredAt: now.Add(-10 * time.Minute),
	}
	svc.syncWorkerAlertNotifications = []SyncWorkerAlertNotification{source}

	result, err := svc.ConfirmSyncWorkerAlertNotifications(SyncWorkerAlertNotificationConfirmInput{
		TenantID:    source.TenantID,
		ConfirmedAt: now,
		Confirm: func(input SyncWorkerAlertConfirmationInput) SyncWorkerAlertConfirmationResult {
			return SyncWorkerAlertConfirmationResult{
				Confirmed:     false,
				Provider:      "resend",
				ProviderError: "provider receipt temporary outage",
				ChannelResults: []wallet.JobAlertChannelResult{
					{
						Channel:                "email",
						Status:                 "sent",
						Provider:               "resend",
						ProviderDeliveryID:     "email_123",
						ProviderDeliveryStatus: "accepted",
					},
				},
			}
		},
	})
	if err != nil {
		t.Fatalf("expected pending confirmation reconciliation to succeed: %v", err)
	}
	if result.TotalNotifications != 1 || result.Confirmed != 0 || result.Failed != 0 || result.Pending != 1 || len(result.Items) != 0 {
		t.Fatalf("unexpected pending confirmation result: %+v", result)
	}

	page := svc.ListSyncWorkerAlertNotificationPageWithOptions(SyncWorkerAlertNotificationListOptions{
		TenantID: source.TenantID,
		Status:   "failed",
		Reason:   "dispatch_commit_unknown",
		Query:    "provider_error",
		Limit:    10,
		Now:      now,
	})
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("expected one pending notification matched by last_confirm_result query, got %+v", page)
	}
	if page.Items[0].ConfirmAttempts != 2 {
		t.Fatalf("expected confirm_attempts=2, got %+v", page.Items[0])
	}
	if page.Items[0].LastConfirmAttemptAt == nil || !page.Items[0].LastConfirmAttemptAt.Equal(now) {
		t.Fatalf("expected last_confirm_attempt_at=%s, got %+v", now.Format(time.RFC3339), page.Items[0])
	}
	if page.Items[0].LastConfirmResult != "provider_error" {
		t.Fatalf("expected last_confirm_result=provider_error, got %+v", page.Items[0])
	}
}

func TestRetrySyncWorkerAlertNotificationCreatesAttemptHistory(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 2, 15, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}
	alert := SyncWorkerAlertDispatchAlert{
		WorkerAction: "enterprise_hris_webhook_processing_alert",
		WorkerKind:   "hris_webhook",
		WorkerLabel:  "HRIS Webhook Merge",
		Count:        3,
		Threshold:    2,
		Failed:       2,
		Processed:    3,
		Applied:      1,
		ConnectorID:  "connector-talenta-001",
		Vendor:       "talenta",
		FailureStage: "merge",
	}

	_, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts:       []SyncWorkerAlertDispatchAlert{alert},
		TriggeredAt:  now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected initial failed dispatch to succeed: %v", err)
	}

	stored := svc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(stored) != 1 || stored[0].Status != "failed" || !stored[0].Retryable {
		t.Fatalf("expected one retryable failed notification, got %+v", stored)
	}
	if stored[0].NextRetryAt == nil {
		t.Fatalf("expected failed retryable notification to schedule next retry, got %+v", stored[0])
	}
	expectedNextRetryAt := now.Add(defaultSyncWorkerAlertRetryBaseBackoff)
	if !stored[0].NextRetryAt.Equal(expectedNextRetryAt) {
		t.Fatalf("expected next_retry_at=%s, got %s", expectedNextRetryAt.Format(time.RFC3339), stored[0].NextRetryAt.Format(time.RFC3339))
	}

	retried, err := svc.RetrySyncWorkerAlertNotification(SyncWorkerAlertNotificationRetryInput{
		TenantID:       "tenant_demo_jakarta",
		NotificationID: stored[0].ID,
		RetriedAt:      now.Add(2 * time.Minute),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			if input.Attempt != 2 {
				t.Fatalf("expected retry attempt=2, got %d", input.Attempt)
			}
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected retry to succeed: %v", err)
	}
	if retried.Status != "sent" || retried.Attempt != 2 {
		t.Fatalf("unexpected retried notification: %+v", retried)
	}
	if retried.NextRetryAt != nil {
		t.Fatalf("expected successful retry to clear next_retry_at, got %+v", retried)
	}
	if retried.SourceNotificationID != stored[0].ID {
		t.Fatalf("expected retry to point at source notification, got %+v", retried)
	}

	history := svc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) != 2 {
		t.Fatalf("expected retry history to contain two items, got %d", len(history))
	}
	if history[0].ID != retried.ID || history[1].ID != stored[0].ID {
		t.Fatalf("expected latest retry to be prepended, got %+v", history)
	}
}

func TestRetrySyncWorkerAlertNotificationRejectsStaleHistory(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 2, 25, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
		},
		TriggeredAt: now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected initial failed dispatch to succeed: %v", err)
	}

	latestFailure, err := svc.RetrySyncWorkerAlertNotification(SyncWorkerAlertNotificationRetryInput{
		TenantID:       "tenant_demo_jakarta",
		NotificationID: initial.Items[0].ID,
		RetriedAt:      now.Add(2 * time.Minute),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected second failed retry to succeed: %v", err)
	}
	if latestFailure.ID == initial.Items[0].ID {
		t.Fatalf("expected retry to create a new history record")
	}

	_, err = svc.RetrySyncWorkerAlertNotification(SyncWorkerAlertNotificationRetryInput{
		TenantID:       "tenant_demo_jakarta",
		NotificationID: initial.Items[0].ID,
		RetriedAt:      now.Add(4 * time.Minute),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if !errors.Is(err, ErrSyncWorkerAlertRetryNotAllowed) {
		t.Fatalf("expected stale retry to be rejected, got %v", err)
	}
}

func TestRetrySyncWorkerAlertNotificationAllowsDistinctRequestIDLineages(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 2, 35, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	firstDispatch, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
				RequestID:    "req-merge-001",
			},
		},
		TriggeredAt: now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected first failed dispatch to succeed: %v", err)
	}

	secondDispatch, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
				RequestID:    "req-merge-002",
			},
		},
		TriggeredAt: now.Add(2 * time.Minute),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			if input.Attempt != 1 {
				t.Fatalf("expected distinct request lineage to start at attempt=1, got %d", input.Attempt)
			}
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected second failed dispatch to succeed: %v", err)
	}
	if secondDispatch.Items[0].Attempt != 1 {
		t.Fatalf("expected second request lineage to keep attempt=1, got %+v", secondDispatch.Items[0])
	}

	retried, err := svc.RetrySyncWorkerAlertNotification(SyncWorkerAlertNotificationRetryInput{
		TenantID:       "tenant_demo_jakarta",
		NotificationID: firstDispatch.Items[0].ID,
		RetriedAt:      now.Add(4 * time.Minute),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			if input.Attempt != 2 {
				t.Fatalf("expected retry on first request lineage to continue at attempt=2, got %d", input.Attempt)
			}
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected retry on distinct request lineage to succeed: %v", err)
	}
	if retried.Attempt != 2 || retried.SourceNotificationID != firstDispatch.Items[0].ID {
		t.Fatalf("unexpected retried record for distinct request lineage: %+v", retried)
	}
}

func TestDispatchSyncWorkerAlertsKeepsLineageAcrossThresholdChange(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 2, 55, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	firstDispatch, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
		},
		TriggeredAt: now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected first failed dispatch to succeed: %v", err)
	}
	if firstDispatch.Items[0].Attempt != 1 {
		t.Fatalf("expected first dispatch attempt=1, got %+v", firstDispatch.Items[0])
	}

	secondDispatch, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        4,
				Threshold:    3,
				Failed:       3,
				Processed:    4,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
		},
		TriggeredAt: now.Add(2 * time.Minute),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			if input.Attempt != 2 {
				t.Fatalf("expected threshold change to stay in same lineage and use attempt=2, got %d", input.Attempt)
			}
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected second failed dispatch with new threshold to succeed: %v", err)
	}
	if secondDispatch.Items[0].Attempt != 2 {
		t.Fatalf("expected threshold change to continue attempt count, got %+v", secondDispatch.Items[0])
	}

	_, err = svc.RetrySyncWorkerAlertNotification(SyncWorkerAlertNotificationRetryInput{
		TenantID:       "tenant_demo_jakarta",
		NotificationID: firstDispatch.Items[0].ID,
		RetriedAt:      now.Add(4 * time.Minute),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if !errors.Is(err, ErrSyncWorkerAlertRetryNotAllowed) {
		t.Fatalf("expected older threshold history retry to be rejected as stale, got %v", err)
	}
}

func TestDispatchSyncWorkerAlertsSkipsProviderWhenFlightExists(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	now := time.Date(2026, 4, 23, 3, 5, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}
	alert := SyncWorkerAlertDispatchAlert{
		WorkerAction: "enterprise_hris_webhook_processing_alert",
		WorkerKind:   "hris_webhook",
		WorkerLabel:  "HRIS Webhook Merge",
		Count:        3,
		Threshold:    2,
		Failed:       2,
		Processed:    3,
		Applied:      1,
		ConnectorID:  "connector-talenta-001",
		Vendor:       "talenta",
		FailureStage: "merge",
	}

	planned := alertdispatch.Plan(alertdispatch.PlanInput{
		Subscription: alertdispatch.Subscription{
			TenantID:       subscription.TenantID,
			Enabled:        subscription.Enabled,
			Channels:       []string{"email"},
			ReceiverGroups: subscription.ReceiverGroups,
		},
		Alert: alertdispatch.Alert{
			Type:      alert.WorkerAction,
			ErrorCode: buildSyncWorkerAlertFingerprint(alert),
			Count:     alert.Count,
			Threshold: alert.Threshold,
		},
	})
	record := SyncWorkerAlertNotification{
		TenantID:       subscription.TenantID,
		WorkerAction:   alert.WorkerAction,
		Fingerprint:    buildSyncWorkerAlertFingerprint(alert),
		RequestID:      alert.RequestID,
		IdempotencyKey: planned.IdempotencyKey,
	}

	var snapshot syncWorkerAlertStateSnapshot
	found, err := store.Load(syncWorkerAlertStateKey, &snapshot)
	if err != nil {
		t.Fatalf("expected sync worker alert state load to succeed: %v", err)
	}
	if found {
		t.Fatalf("expected no prior alert snapshot, got %+v", snapshot)
	}
	flightNow := time.Now().UTC()
	snapshot.SyncWorkerAlertInFlights = []SyncWorkerAlertInFlight{
		{
			TenantID:   subscription.TenantID,
			Key:        syncWorkerAlertNotificationDispatchFlightKey(record),
			Token:      "swf_existing_dispatch",
			Kind:       "dispatch",
			AcquiredAt: flightNow.Add(-time.Minute),
			ExpiresAt:  flightNow.Add(time.Minute),
		},
	}
	if err := store.Save(syncWorkerAlertStateKey, snapshot); err != nil {
		t.Fatalf("expected competing in-flight snapshot save to succeed: %v", err)
	}

	providerCalls := 0
	result, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts:       []SyncWorkerAlertDispatchAlert{alert},
		TriggeredAt:  now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			providerCalls++
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected dispatch with competing in-flight to succeed as skipped: %v", err)
	}
	if providerCalls != 0 {
		t.Fatalf("expected provider dispatch to be skipped while in-flight exists, got %d calls", providerCalls)
	}
	if result.Dispatched != 0 || result.Skipped != 1 || len(result.Items) != 1 {
		t.Fatalf("unexpected dispatch result with competing in-flight: %+v", result)
	}
	if result.Items[0].Reason != "dispatch_inflight" {
		t.Fatalf("expected dispatch_inflight reason, got %+v", result.Items[0])
	}

	history := svc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) != 0 {
		t.Fatalf("expected no persisted history entry for losing in-flight dispatch, got %+v", history)
	}
}

func TestListSyncWorkerAlertNotificationsPrunesExpiredInFlightState(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	now := time.Now().UTC()
	snapshot := syncWorkerAlertStateSnapshot{
		SyncWorkerAlertInFlights: []SyncWorkerAlertInFlight{
			{
				TenantID:   "tenant_demo_jakarta",
				Key:        "flight-expired",
				Token:      "swf_expired_cleanup",
				Kind:       "dispatch",
				AcquiredAt: now.Add(-10 * time.Minute),
				ExpiresAt:  now.Add(-time.Minute),
			},
		},
	}
	if err := store.Save(syncWorkerAlertStateKey, snapshot); err != nil {
		t.Fatalf("expected expired in-flight snapshot save to succeed: %v", err)
	}

	history := svc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) != 0 {
		t.Fatalf("expected no notification history, got %+v", history)
	}

	var stored syncWorkerAlertStateSnapshot
	found, err := store.Load(syncWorkerAlertStateKey, &stored)
	if err != nil || !found {
		t.Fatalf("expected cleaned alert snapshot load to succeed, found=%v err=%v", found, err)
	}
	if len(stored.SyncWorkerAlertInFlights) != 0 {
		t.Fatalf("expected expired in-flight state to be pruned on refresh, got %+v", stored.SyncWorkerAlertInFlights)
	}
}

func TestListSyncWorkerAlertNotificationsRecoversExpiredDispatchFlight(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	now := time.Date(2026, 4, 23, 4, 0, 0, 0, time.UTC)
	record := SyncWorkerAlertNotification{
		ID:             "swa_recover_expired_001",
		TenantID:       "tenant_demo_jakarta",
		WorkerAction:   "enterprise_hris_webhook_processing_alert",
		WorkerKind:     "hris_webhook",
		WorkerLabel:    "HRIS Webhook Merge",
		Fingerprint:    "enterprise_hris_webhook_processing_alert|connector-talenta-001|talenta|merge",
		Count:          3,
		Threshold:      2,
		Failed:         2,
		Processed:      3,
		Applied:        1,
		ConnectorID:    "connector-talenta-001",
		Vendor:         "talenta",
		FailureStage:   "merge",
		Channels:       []string{"email"},
		ReceiverGroups: []string{"security"},
		IdempotencyKey: "recover-expired-dispatch-key",
		Attempt:        1,
		TriggeredAt:    now.Add(-10 * time.Minute),
	}
	if err := store.Save(syncWorkerAlertStateKey, syncWorkerAlertStateSnapshot{
		SyncWorkerAlertInFlights: []SyncWorkerAlertInFlight{
			{
				TenantID:       record.TenantID,
				Key:            syncWorkerAlertNotificationDispatchFlightKey(record),
				Token:          "swf_recover_expired_001",
				Kind:           "dispatch",
				NotificationID: record.ID,
				Notification:   record,
				AcquiredAt:     now.Add(-10 * time.Minute),
				ExpiresAt:      now.Add(-time.Minute),
			},
		},
	}); err != nil {
		t.Fatalf("expected expired dispatch flight snapshot save to succeed: %v", err)
	}

	history := svc.ListSyncWorkerAlertNotifications(record.TenantID, 10)
	if len(history) != 1 {
		t.Fatalf("expected one recovered notification, got %+v", history)
	}
	if history[0].ID != record.ID {
		t.Fatalf("expected recovered notification id %s, got %+v", record.ID, history[0])
	}
	if history[0].Status != "failed" || history[0].Reason != "dispatch_commit_unknown" {
		t.Fatalf("expected recovered notification to be failed/dispatch_commit_unknown, got %+v", history[0])
	}
	if !history[0].Retryable || history[0].NextRetryAt == nil {
		t.Fatalf("expected recovered notification to be retryable with next_retry_at, got %+v", history[0])
	}
	if history[0].ProviderError != "dispatch finalize missing after provider call" {
		t.Fatalf("unexpected recovered provider_error: %+v", history[0])
	}

	var stored syncWorkerAlertStateSnapshot
	found, err := store.Load(syncWorkerAlertStateKey, &stored)
	if err != nil || !found {
		t.Fatalf("expected recovered alert snapshot load to succeed, found=%v err=%v", found, err)
	}
	if len(stored.SyncWorkerAlertInFlights) != 0 {
		t.Fatalf("expected expired dispatch flight to be removed after recovery, got %+v", stored.SyncWorkerAlertInFlights)
	}
	if len(stored.SyncWorkerAlertNotifications) != 1 {
		t.Fatalf("expected recovered notification to persist, got %+v", stored.SyncWorkerAlertNotifications)
	}
	if len(stored.SyncWorkerAlertCooldowns) != 1 {
		t.Fatalf("expected recovered dispatch flight to seed one cooldown entry, got %+v", stored.SyncWorkerAlertCooldowns)
	}
}

func TestDispatchSyncWorkerAlertsAllowsExpiredInFlightDispatch(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	now := time.Now().UTC()
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}
	alert := SyncWorkerAlertDispatchAlert{
		WorkerAction: "enterprise_hris_webhook_processing_alert",
		WorkerKind:   "hris_webhook",
		WorkerLabel:  "HRIS Webhook Merge",
		Count:        3,
		Threshold:    2,
		Failed:       2,
		Processed:    3,
		Applied:      1,
		ConnectorID:  "connector-talenta-001",
		Vendor:       "talenta",
		FailureStage: "merge",
	}
	planned := alertdispatch.Plan(alertdispatch.PlanInput{
		Subscription: alertdispatch.Subscription{
			TenantID:       subscription.TenantID,
			Enabled:        subscription.Enabled,
			Channels:       []string{"email"},
			ReceiverGroups: subscription.ReceiverGroups,
		},
		Alert: alertdispatch.Alert{
			Type:      alert.WorkerAction,
			ErrorCode: buildSyncWorkerAlertFingerprint(alert),
			Count:     alert.Count,
			Threshold: alert.Threshold,
		},
	})
	record := SyncWorkerAlertNotification{
		TenantID:       subscription.TenantID,
		WorkerAction:   alert.WorkerAction,
		Fingerprint:    buildSyncWorkerAlertFingerprint(alert),
		RequestID:      alert.RequestID,
		IdempotencyKey: planned.IdempotencyKey,
	}

	if err := store.Save(syncWorkerAlertStateKey, syncWorkerAlertStateSnapshot{
		SyncWorkerAlertInFlights: []SyncWorkerAlertInFlight{
			{
				TenantID:   subscription.TenantID,
				Key:        syncWorkerAlertNotificationDispatchFlightKey(record),
				Token:      "swf_expired_dispatch",
				Kind:       "dispatch",
				AcquiredAt: now.Add(-10 * time.Minute),
				ExpiresAt:  now.Add(-time.Minute),
			},
		},
	}); err != nil {
		t.Fatalf("expected expired in-flight snapshot save to succeed: %v", err)
	}

	providerCalls := 0
	result, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     subscription.TenantID,
		Subscription: subscription,
		Alerts:       []SyncWorkerAlertDispatchAlert{alert},
		TriggeredAt:  now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			providerCalls++
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected expired in-flight dispatch to recover and succeed: %v", err)
	}
	if providerCalls != 1 {
		t.Fatalf("expected provider dispatch to run after expired in-flight cleanup, got %d calls", providerCalls)
	}
	if result.Dispatched != 1 || result.Skipped != 0 || len(result.Items) != 1 {
		t.Fatalf("unexpected dispatch result after expired in-flight cleanup: %+v", result)
	}

	var stored syncWorkerAlertStateSnapshot
	found, err := store.Load(syncWorkerAlertStateKey, &stored)
	if err != nil || !found {
		t.Fatalf("expected stored alert snapshot load to succeed, found=%v err=%v", found, err)
	}
	if len(stored.SyncWorkerAlertInFlights) != 0 {
		t.Fatalf("expected expired and completed dispatch flights to be absent after finalize, got %+v", stored.SyncWorkerAlertInFlights)
	}
}

func TestDispatchSyncWorkerAlertsSkipsWhileRecoveredDispatchCommitIsPending(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	now := time.Date(2026, 4, 23, 4, 30, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}
	alert := SyncWorkerAlertDispatchAlert{
		WorkerAction: "enterprise_hris_webhook_processing_alert",
		WorkerKind:   "hris_webhook",
		WorkerLabel:  "HRIS Webhook Merge",
		Count:        3,
		Threshold:    2,
		Failed:       2,
		Processed:    3,
		Applied:      1,
		ConnectorID:  "connector-talenta-001",
		Vendor:       "talenta",
		FailureStage: "merge",
	}
	record := SyncWorkerAlertNotification{
		ID:             "swa_pending_commit_001",
		TenantID:       subscription.TenantID,
		WorkerAction:   alert.WorkerAction,
		WorkerKind:     alert.WorkerKind,
		WorkerLabel:    alert.WorkerLabel,
		Fingerprint:    buildSyncWorkerAlertFingerprint(alert),
		Count:          alert.Count,
		Threshold:      alert.Threshold,
		Failed:         alert.Failed,
		Processed:      alert.Processed,
		Applied:        alert.Applied,
		ConnectorID:    alert.ConnectorID,
		Vendor:         alert.Vendor,
		FailureStage:   alert.FailureStage,
		Channels:       []string{"email"},
		ReceiverGroups: []string{"security"},
		Status:         "failed",
		Reason:         "dispatch_commit_unknown",
		IdempotencyKey: "pending-commit-key",
		Attempt:        1,
		Retryable:      true,
		TriggeredAt:    now.Add(-5 * time.Minute),
	}
	nextRetryAt := now.Add(time.Minute)
	record.NextRetryAt = &nextRetryAt
	if err := store.Save(syncWorkerAlertStateKey, syncWorkerAlertStateSnapshot{
		SyncWorkerAlertNotifications: []SyncWorkerAlertNotification{record},
	}); err != nil {
		t.Fatalf("expected pending commit snapshot save to succeed: %v", err)
	}

	providerCalls := 0
	result, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     subscription.TenantID,
		Subscription: subscription,
		Alerts:       []SyncWorkerAlertDispatchAlert{alert},
		TriggeredAt:  now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			providerCalls++
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected dispatch with pending recovered commit to succeed as skipped: %v", err)
	}
	if providerCalls != 0 {
		t.Fatalf("expected provider dispatch to be skipped while recovered commit is pending, got %d calls", providerCalls)
	}
	if result.Dispatched != 0 || result.Skipped != 1 || len(result.Items) != 1 {
		t.Fatalf("unexpected dispatch result with pending recovered commit: %+v", result)
	}
	if result.Items[0].Reason != "dispatch_recovery_pending" {
		t.Fatalf("expected dispatch_recovery_pending reason, got %+v", result.Items[0])
	}

	history := svc.ListSyncWorkerAlertNotifications(subscription.TenantID, 10)
	if len(history) != 1 || history[0].ID != record.ID {
		t.Fatalf("expected original pending recovered history to remain untouched, got %+v", history)
	}
}

func TestConfirmSyncWorkerAlertNotificationsCreatesConfirmedHistory(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 4, 35, 0, 0, time.UTC)
	nextRetryAt := now.Add(5 * time.Minute)
	source := SyncWorkerAlertNotification{
		ID:             "swa_pending_commit_001",
		TenantID:       "tenant_demo_jakarta",
		WorkerAction:   "enterprise_hris_webhook_processing_alert",
		WorkerKind:     "hris_webhook",
		WorkerLabel:    "HRIS Webhook Merge",
		Fingerprint:    "enterprise_hris_webhook_processing_alert|connector-talenta-001|talenta|merge",
		Count:          3,
		Threshold:      2,
		Failed:         2,
		Processed:      3,
		Applied:        1,
		ConnectorID:    "connector-talenta-001",
		Vendor:         "talenta",
		FailureStage:   "merge",
		Channels:       []string{"email"},
		ReceiverGroups: []string{"security"},
		Status:         "failed",
		Reason:         "dispatch_commit_unknown",
		IdempotencyKey: "pending-commit-key",
		Attempt:        1,
		Retryable:      true,
		Provider:       "resend",
		ChannelResults: []wallet.JobAlertChannelResult{
			{
				Channel:                "email",
				Status:                 "sent",
				Provider:               "resend",
				ProviderDeliveryID:     "email_123",
				ProviderDeliveryStatus: "accepted",
			},
		},
		TriggeredAt: now.Add(-5 * time.Minute),
		NextRetryAt: &nextRetryAt,
	}
	svc.syncWorkerAlertNotifications = []SyncWorkerAlertNotification{source}
	svc.syncWorkerAlertCooldowns = []SyncWorkerAlertCooldown{
		{
			TenantID:    source.TenantID,
			Fingerprint: syncWorkerAlertNotificationFingerprint(source),
			LastSentAt:  source.TriggeredAt,
		},
	}

	result, err := svc.ConfirmSyncWorkerAlertNotifications(SyncWorkerAlertNotificationConfirmInput{
		TenantID:    source.TenantID,
		ConfirmedAt: now,
		Confirm: func(input SyncWorkerAlertConfirmationInput) SyncWorkerAlertConfirmationResult {
			if input.NotificationID != source.ID || input.IdempotencyKey != source.IdempotencyKey {
				t.Fatalf("unexpected confirmation input: %+v", input)
			}
			return SyncWorkerAlertConfirmationResult{
				Confirmed: true,
				Provider:  "resend",
				ChannelResults: []wallet.JobAlertChannelResult{
					{
						Channel:                "email",
						Status:                 "sent",
						Provider:               "resend",
						ProviderDeliveryID:     "email_123",
						ProviderDeliveryStatus: "delivered",
					},
				},
			}
		},
	})
	if err != nil {
		t.Fatalf("expected confirmation reconciliation to succeed: %v", err)
	}
	if result.TotalNotifications != 1 || result.Confirmed != 1 || result.Failed != 0 || result.Pending != 0 || len(result.Items) != 1 {
		t.Fatalf("unexpected confirmation result: %+v", result)
	}
	if result.Items[0].Status != "sent" || result.Items[0].Reason != "dispatch_commit_confirmed" {
		t.Fatalf("expected confirmed sent history item, got %+v", result.Items[0])
	}
	if result.Items[0].SourceNotificationID != source.ID || result.Items[0].Attempt != source.Attempt {
		t.Fatalf("expected confirmation history to preserve attempt and source, got %+v", result.Items[0])
	}
	if result.Items[0].ConfirmAttempts != 1 || result.Items[0].LastConfirmAttemptAt == nil || !result.Items[0].LastConfirmAttemptAt.Equal(now) || result.Items[0].LastConfirmResult != "confirmed" {
		t.Fatalf("expected confirmation observability fields to be preserved, got %+v", result.Items[0])
	}
	if len(result.Items[0].ChannelResults) != 1 || result.Items[0].ChannelResults[0].ProviderDeliveryStatus != "delivered" {
		t.Fatalf("expected delivered provider status after confirmation, got %+v", result.Items[0].ChannelResults)
	}

	history := svc.ListSyncWorkerAlertNotifications(source.TenantID, 10)
	if len(history) != 2 || history[0].Reason != "dispatch_commit_confirmed" || history[1].ID != source.ID {
		t.Fatalf("expected confirmed history to prepend source item, got %+v", history)
	}
	if len(svc.syncWorkerAlertCooldowns) != 1 || !svc.syncWorkerAlertCooldowns[0].LastSentAt.Equal(now) {
		t.Fatalf("expected confirmation to refresh cooldown timestamp, got %+v", svc.syncWorkerAlertCooldowns)
	}
}

func TestConfirmSyncWorkerAlertNotificationsCreatesRejectedFailureHistory(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 4, 40, 0, 0, time.UTC)
	nextRetryAt := now.Add(5 * time.Minute)
	source := SyncWorkerAlertNotification{
		ID:             "swa_pending_commit_rejected_001",
		TenantID:       "tenant_demo_jakarta",
		WorkerAction:   "enterprise_hris_webhook_processing_alert",
		WorkerKind:     "hris_webhook",
		WorkerLabel:    "HRIS Webhook Merge",
		Fingerprint:    "enterprise_hris_webhook_processing_alert|connector-talenta-001|talenta|merge",
		Count:          3,
		Threshold:      2,
		Failed:         2,
		Processed:      3,
		Applied:        1,
		ConnectorID:    "connector-talenta-001",
		Vendor:         "talenta",
		FailureStage:   "merge",
		Channels:       []string{"email"},
		ReceiverGroups: []string{"security"},
		Status:         "failed",
		Reason:         "dispatch_commit_unknown",
		IdempotencyKey: "pending-commit-rejected-key",
		Attempt:        1,
		Retryable:      true,
		Provider:       "resend",
		ChannelResults: []wallet.JobAlertChannelResult{
			{
				Channel:                "email",
				Status:                 "sent",
				Provider:               "resend",
				ProviderDeliveryID:     "email_123",
				ProviderDeliveryStatus: "accepted",
			},
		},
		TriggeredAt: now.Add(-10 * time.Minute),
		NextRetryAt: &nextRetryAt,
	}
	svc.syncWorkerAlertNotifications = []SyncWorkerAlertNotification{source}

	result, err := svc.ConfirmSyncWorkerAlertNotifications(SyncWorkerAlertNotificationConfirmInput{
		TenantID:    source.TenantID,
		ConfirmedAt: now,
		Confirm: func(input SyncWorkerAlertConfirmationInput) SyncWorkerAlertConfirmationResult {
			return SyncWorkerAlertConfirmationResult{
				Confirmed:     false,
				Provider:      "resend",
				ProviderError: "provider delivery status bounced",
				ChannelResults: []wallet.JobAlertChannelResult{
					{
						Channel:                "email",
						Status:                 "failed",
						Reason:                 "provider_delivery_failed",
						Provider:               "resend",
						ProviderDeliveryID:     "email_123",
						ProviderDeliveryStatus: "bounced",
					},
				},
			}
		},
	})
	if err != nil {
		t.Fatalf("expected rejected confirmation reconciliation to succeed: %v", err)
	}
	if result.TotalNotifications != 1 || result.Confirmed != 0 || result.Failed != 1 || result.Pending != 0 || len(result.Items) != 1 {
		t.Fatalf("unexpected rejected confirmation result: %+v", result)
	}
	if result.Items[0].Status != "failed" || result.Items[0].Reason != "dispatch_commit_rejected" || !result.Items[0].Retryable {
		t.Fatalf("expected rejected failure history item, got %+v", result.Items[0])
	}
	if result.Items[0].NextRetryAt != nil {
		t.Fatalf("expected rejected failure to require manual follow-up without next_retry_at, got %+v", result.Items[0])
	}
	if len(result.Items[0].ChannelResults) != 1 || result.Items[0].ChannelResults[0].ProviderDeliveryStatus != "bounced" {
		t.Fatalf("expected bounced provider status to be preserved, got %+v", result.Items[0].ChannelResults)
	}
	if result.Items[0].ConfirmAttempts != 1 || result.Items[0].LastConfirmAttemptAt == nil || !result.Items[0].LastConfirmAttemptAt.Equal(now) || result.Items[0].LastConfirmResult != "rejected" {
		t.Fatalf("expected rejected confirmation observability fields, got %+v", result.Items[0])
	}

	history := svc.ListSyncWorkerAlertNotifications(source.TenantID, 10)
	if len(history) != 2 || history[0].Reason != "dispatch_commit_rejected" || history[1].ID != source.ID {
		t.Fatalf("expected rejected confirmation history to prepend source item, got %+v", history)
	}
}

func TestConfirmSyncWorkerAlertNotificationsAgesOutPendingHistory(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 5, 0, 0, 0, time.UTC)
	nextRetryAt := now.Add(-time.Minute)
	source := SyncWorkerAlertNotification{
		ID:             "swa_pending_commit_timeout_001",
		TenantID:       "tenant_demo_jakarta",
		WorkerAction:   "enterprise_hris_webhook_processing_alert",
		WorkerKind:     "hris_webhook",
		WorkerLabel:    "HRIS Webhook Merge",
		Fingerprint:    "enterprise_hris_webhook_processing_alert|connector-talenta-001|talenta|merge",
		Count:          3,
		Threshold:      2,
		Failed:         2,
		Processed:      3,
		Applied:        1,
		ConnectorID:    "connector-talenta-001",
		Vendor:         "talenta",
		FailureStage:   "merge",
		Channels:       []string{"email"},
		ReceiverGroups: []string{"security"},
		Status:         "failed",
		Reason:         "dispatch_commit_unknown",
		IdempotencyKey: "pending-commit-timeout-key",
		Attempt:        1,
		Retryable:      true,
		Provider:       "resend",
		ChannelResults: []wallet.JobAlertChannelResult{
			{
				Channel:                "email",
				Status:                 "sent",
				Provider:               "resend",
				ProviderDeliveryID:     "email_123",
				ProviderDeliveryStatus: "accepted",
			},
		},
		TriggeredAt: now.Add(-(defaultSyncWorkerAlertConfirmationTTL + time.Minute)),
		NextRetryAt: &nextRetryAt,
	}
	svc.syncWorkerAlertNotifications = []SyncWorkerAlertNotification{source}

	result, err := svc.ConfirmSyncWorkerAlertNotifications(SyncWorkerAlertNotificationConfirmInput{
		TenantID:    source.TenantID,
		ConfirmedAt: now,
		Confirm: func(input SyncWorkerAlertConfirmationInput) SyncWorkerAlertConfirmationResult {
			return SyncWorkerAlertConfirmationResult{
				Confirmed: false,
				Provider:  "resend",
				ChannelResults: []wallet.JobAlertChannelResult{
					{
						Channel:                "email",
						Status:                 "sent",
						Provider:               "resend",
						ProviderDeliveryID:     "email_123",
						ProviderDeliveryStatus: "accepted",
					},
				},
			}
		},
	})
	if err != nil {
		t.Fatalf("expected timeout confirmation reconciliation to succeed: %v", err)
	}
	if result.TotalNotifications != 1 || result.Confirmed != 0 || result.Failed != 1 || result.Pending != 0 || len(result.Items) != 1 {
		t.Fatalf("unexpected timeout confirmation result: %+v", result)
	}
	if result.Items[0].Status != "failed" || result.Items[0].Reason != "dispatch_commit_confirmation_timeout" || !result.Items[0].Retryable {
		t.Fatalf("expected timeout failure history item, got %+v", result.Items[0])
	}
	if result.Items[0].NextRetryAt == nil || !result.Items[0].NextRetryAt.Equal(now) {
		t.Fatalf("expected timeout failure to become due now, got %+v", result.Items[0])
	}
	if result.Items[0].ConfirmAttempts != 1 || result.Items[0].LastConfirmAttemptAt == nil || !result.Items[0].LastConfirmAttemptAt.Equal(now) || result.Items[0].LastConfirmResult != "timeout" {
		t.Fatalf("expected timeout confirmation observability fields, got %+v", result.Items[0])
	}

	history := svc.ListSyncWorkerAlertNotifications(source.TenantID, 10)
	if len(history) != 2 || history[0].Reason != "dispatch_commit_confirmation_timeout" || history[1].ID != source.ID {
		t.Fatalf("expected timeout confirmation history to prepend source item, got %+v", history)
	}
}

func TestRetrySyncWorkerAlertNotificationRejectsInFlightDispatch(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	now := time.Date(2026, 4, 23, 3, 15, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
		},
		TriggeredAt: now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected initial failed dispatch to succeed: %v", err)
	}

	retryRecord := buildSyncWorkerAlertRetryRecord(initial.Items[0], now.Add(2*time.Minute))
	var snapshot syncWorkerAlertStateSnapshot
	found, err := store.Load(syncWorkerAlertStateKey, &snapshot)
	if err != nil || !found {
		t.Fatalf("expected alert snapshot load to succeed, found=%v err=%v", found, err)
	}
	flightNow := time.Now().UTC()
	snapshot.SyncWorkerAlertInFlights = append(
		[]SyncWorkerAlertInFlight{
			{
				TenantID:             "tenant_demo_jakarta",
				Key:                  syncWorkerAlertNotificationDispatchFlightKey(retryRecord),
				Token:                "swf_existing_retry",
				Kind:                 "retry",
				NotificationID:       retryRecord.ID,
				SourceNotificationID: initial.Items[0].ID,
				AcquiredAt:           flightNow.Add(-time.Minute),
				ExpiresAt:            flightNow.Add(time.Minute),
			},
		},
		snapshot.SyncWorkerAlertInFlights...,
	)
	if err := store.Save(syncWorkerAlertStateKey, snapshot); err != nil {
		t.Fatalf("expected competing in-flight retry snapshot save to succeed: %v", err)
	}

	providerCalls := 0
	_, err = svc.RetrySyncWorkerAlertNotification(SyncWorkerAlertNotificationRetryInput{
		TenantID:       "tenant_demo_jakarta",
		NotificationID: initial.Items[0].ID,
		RetriedAt:      now.Add(2 * time.Minute),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			providerCalls++
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if !errors.Is(err, ErrSyncWorkerAlertDispatchInFlight) {
		t.Fatalf("expected retry to fail with in-flight conflict, got %v", err)
	}
	if providerCalls != 0 {
		t.Fatalf("expected retry provider call to be skipped while in-flight exists, got %d calls", providerCalls)
	}
}

func TestAutoRetrySyncWorkerAlertNotificationsAllowsExpiredInFlightDispatch(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	now := time.Now().UTC().Add(-time.Hour)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
		},
		TriggeredAt: now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected initial failed dispatch to succeed: %v", err)
	}
	if len(initial.Items) != 1 || initial.Items[0].NextRetryAt == nil {
		t.Fatalf("expected one retryable failed notification with next_retry_at, got %+v", initial.Items)
	}

	dueAt := initial.Items[0].NextRetryAt.UTC()
	retryRecord := buildSyncWorkerAlertRetryRecord(initial.Items[0], dueAt)
	var snapshot syncWorkerAlertStateSnapshot
	found, err := store.Load(syncWorkerAlertStateKey, &snapshot)
	if err != nil || !found {
		t.Fatalf("expected alert snapshot load to succeed, found=%v err=%v", found, err)
	}
	snapshot.SyncWorkerAlertInFlights = append(
		[]SyncWorkerAlertInFlight{
			{
				TenantID:             "tenant_demo_jakarta",
				Key:                  syncWorkerAlertNotificationDispatchFlightKey(retryRecord),
				Token:                "swf_expired_auto_retry",
				Kind:                 "retry",
				NotificationID:       retryRecord.ID,
				SourceNotificationID: initial.Items[0].ID,
				AcquiredAt:           dueAt.Add(-10 * time.Minute),
				ExpiresAt:            dueAt.Add(-time.Minute),
			},
		},
		snapshot.SyncWorkerAlertInFlights...,
	)
	if err := store.Save(syncWorkerAlertStateKey, snapshot); err != nil {
		t.Fatalf("expected expired auto-retry in-flight snapshot save to succeed: %v", err)
	}

	providerCalls := 0
	result, err := svc.AutoRetrySyncWorkerAlertNotifications(SyncWorkerAlertNotificationAutoRetryInput{
		TenantID:    "tenant_demo_jakarta",
		Limit:       10,
		RetriedAt:   dueAt,
		MaxAttempts: 3,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			providerCalls++
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected auto retry to recover from expired in-flight and succeed: %v", err)
	}
	if providerCalls != 1 {
		t.Fatalf("expected auto retry provider dispatch after expired in-flight cleanup, got %d calls", providerCalls)
	}
	if result.TotalNotifications != 1 || result.Retried != 1 || result.Failed != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected auto retry result after expired in-flight cleanup: %+v", result)
	}

	var stored syncWorkerAlertStateSnapshot
	if found, err := store.Load(syncWorkerAlertStateKey, &stored); err != nil || !found {
		t.Fatalf("expected cleaned alert snapshot load to succeed after auto retry, found=%v err=%v", found, err)
	}
	if len(stored.SyncWorkerAlertInFlights) != 0 {
		t.Fatalf("expected expired and completed auto-retry flights to be absent after finalize, got %+v", stored.SyncWorkerAlertInFlights)
	}
}

func TestBatchRetrySyncWorkerAlertNotificationsSuppressesDuplicateFingerprints(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 2, 45, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	firstDispatch, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Persist",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "persist",
			},
		},
		TriggeredAt: now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected initial failed dispatches to succeed: %v", err)
	}
	if len(firstDispatch.Items) != 2 {
		t.Fatalf("expected two initial failed notifications, got %+v", firstDispatch)
	}

	secondFailure, err := svc.RetrySyncWorkerAlertNotification(SyncWorkerAlertNotificationRetryInput{
		TenantID:       "tenant_demo_jakarta",
		NotificationID: firstDispatch.Items[0].ID,
		RetriedAt:      now.Add(2 * time.Minute),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected second failed retry to succeed: %v", err)
	}
	if secondFailure.Status != "failed" || !secondFailure.Retryable {
		t.Fatalf("expected second failure to remain retryable, got %+v", secondFailure)
	}

	batchResult, err := svc.BatchRetrySyncWorkerAlertNotifications(SyncWorkerAlertNotificationBatchRetryInput{
		TenantID: "tenant_demo_jakarta",
		NotificationIDs: []string{
			secondFailure.ID,
			firstDispatch.Items[0].ID,
			firstDispatch.Items[1].ID,
		},
		RetriedAt: now.Add(4 * time.Minute),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected batch retry to succeed: %v", err)
	}
	if batchResult.TotalNotifications != 3 || batchResult.Retried != 2 || batchResult.Suppressed != 1 || batchResult.Failed != 0 || batchResult.Skipped != 0 {
		t.Fatalf("unexpected batch retry result: %+v", batchResult)
	}
	if len(batchResult.Items) != 2 {
		t.Fatalf("expected only non-suppressed retries to create records, got %+v", batchResult.Items)
	}

	history := svc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) != 5 {
		t.Fatalf("expected two additional retry records in history, got %d", len(history))
	}
}

func TestBatchSuppressSyncWorkerAlertNotificationsCreatesSkippedHistory(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 3, 0, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
		},
		TriggeredAt: now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected initial failed dispatch to succeed: %v", err)
	}

	result, err := svc.BatchSuppressSyncWorkerAlertNotifications(SyncWorkerAlertNotificationBatchSuppressInput{
		TenantID:        "tenant_demo_jakarta",
		NotificationIDs: []string{initial.Items[0].ID},
		SuppressedAt:    now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("expected batch suppress to succeed: %v", err)
	}
	if result.TotalNotifications != 1 || result.Suppressed != 1 || result.Skipped != 0 || len(result.Items) != 1 {
		t.Fatalf("unexpected batch suppress result: %+v", result)
	}
	if result.Items[0].Status != "skipped" || result.Items[0].Reason != "manual_suppressed" {
		t.Fatalf("expected manual_suppressed record, got %+v", result.Items[0])
	}

	history := svc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) != 2 || history[0].Reason != "manual_suppressed" {
		t.Fatalf("expected suppressed record to be prepended, got %+v", history)
	}
}

func TestBatchSuppressSyncWorkerAlertNotificationsSkipsStaleFailedHistory(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 3, 5, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
		},
		TriggeredAt: now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected initial failed dispatch to succeed: %v", err)
	}

	_, err = svc.RetrySyncWorkerAlertNotification(SyncWorkerAlertNotificationRetryInput{
		TenantID:       "tenant_demo_jakarta",
		NotificationID: initial.Items[0].ID,
		RetriedAt:      now.Add(2 * time.Minute),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected latest failed retry to succeed: %v", err)
	}

	result, err := svc.BatchSuppressSyncWorkerAlertNotifications(SyncWorkerAlertNotificationBatchSuppressInput{
		TenantID:        "tenant_demo_jakarta",
		NotificationIDs: []string{initial.Items[0].ID},
		SuppressedAt:    now.Add(4 * time.Minute),
	})
	if err != nil {
		t.Fatalf("expected batch suppress on stale failed history to succeed: %v", err)
	}
	if result.Suppressed != 0 || result.Skipped != 1 || len(result.Items) != 0 {
		t.Fatalf("expected stale failed history suppress to be skipped, got %+v", result)
	}
}

func TestBatchRestoreSyncWorkerAlertNotificationsCreatesRetryableFailedHistory(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 3, 10, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
		},
		TriggeredAt: now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected initial failed dispatch to succeed: %v", err)
	}

	suppressed, err := svc.BatchSuppressSyncWorkerAlertNotifications(SyncWorkerAlertNotificationBatchSuppressInput{
		TenantID:        "tenant_demo_jakarta",
		NotificationIDs: []string{initial.Items[0].ID},
		SuppressedAt:    now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("expected batch suppress to succeed: %v", err)
	}
	if len(suppressed.Items) != 1 || suppressed.Items[0].Reason != "manual_suppressed" {
		t.Fatalf("expected manual_suppressed history item, got %+v", suppressed.Items)
	}

	restored, err := svc.BatchRestoreSyncWorkerAlertNotifications(SyncWorkerAlertNotificationBatchRestoreInput{
		TenantID:        "tenant_demo_jakarta",
		NotificationIDs: []string{suppressed.Items[0].ID},
		RestoredAt:      now.Add(3 * time.Minute),
	})
	if err != nil {
		t.Fatalf("expected batch restore to succeed: %v", err)
	}
	if restored.TotalNotifications != 1 || restored.Restored != 1 || restored.Skipped != 0 || len(restored.Items) != 1 {
		t.Fatalf("unexpected batch restore result: %+v", restored)
	}
	if restored.Items[0].Status != "failed" || restored.Items[0].Reason != "manual_suppressed_restored" || !restored.Items[0].Retryable {
		t.Fatalf("expected restored item to be failed+retryable, got %+v", restored.Items[0])
	}
	if restored.Items[0].SourceNotificationID != suppressed.Items[0].ID {
		t.Fatalf("expected restored item to point to suppressed record, got %+v", restored.Items[0])
	}
	if restored.Items[0].NextRetryAt == nil || !restored.Items[0].NextRetryAt.Equal(now.Add(3*time.Minute)) {
		t.Fatalf("expected restored item to be due immediately, got %+v", restored.Items[0])
	}
	if restored.Items[0].ProviderError != "temporary outage" {
		t.Fatalf("expected restored item to preserve provider error, got %+v", restored.Items[0])
	}

	history := svc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) != 3 || history[0].Reason != "manual_suppressed_restored" {
		t.Fatalf("expected restored record to be prepended, got %+v", history)
	}
	if history[1].Reason != "manual_suppressed" || history[1].RestoreStatus != "newer_history_exists" {
		t.Fatalf("expected suppressed record to be marked stale after restore, got %+v", history[1])
	}
}

func TestBatchRestoreSyncWorkerAlertNotificationsSkipsStaleSuppressedHistory(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 3, 40, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
		},
		TriggeredAt: now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected initial failed dispatch to succeed: %v", err)
	}

	firstSuppress, err := svc.BatchSuppressSyncWorkerAlertNotifications(SyncWorkerAlertNotificationBatchSuppressInput{
		TenantID:        "tenant_demo_jakarta",
		NotificationIDs: []string{initial.Items[0].ID},
		SuppressedAt:    now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("expected first suppress to succeed: %v", err)
	}

	restored, err := svc.BatchRestoreSyncWorkerAlertNotifications(SyncWorkerAlertNotificationBatchRestoreInput{
		TenantID:        "tenant_demo_jakarta",
		NotificationIDs: []string{firstSuppress.Items[0].ID},
		RestoredAt:      now.Add(3 * time.Minute),
	})
	if err != nil {
		t.Fatalf("expected restore to succeed: %v", err)
	}

	secondSuppress, err := svc.BatchSuppressSyncWorkerAlertNotifications(SyncWorkerAlertNotificationBatchSuppressInput{
		TenantID:        "tenant_demo_jakarta",
		NotificationIDs: []string{restored.Items[0].ID},
		SuppressedAt:    now.Add(4 * time.Minute),
	})
	if err != nil {
		t.Fatalf("expected second suppress to succeed: %v", err)
	}

	result, err := svc.BatchRestoreSyncWorkerAlertNotifications(SyncWorkerAlertNotificationBatchRestoreInput{
		TenantID:        "tenant_demo_jakarta",
		NotificationIDs: []string{firstSuppress.Items[0].ID, secondSuppress.Items[0].ID},
		RestoredAt:      now.Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("expected batch restore to succeed: %v", err)
	}
	if result.Restored != 1 || result.Skipped != 1 || len(result.Items) != 1 {
		t.Fatalf("expected one restored and one skipped stale suppression, got %+v", result)
	}
	if result.Items[0].SourceNotificationID != secondSuppress.Items[0].ID {
		t.Fatalf("expected latest suppressed record to be restored, got %+v", result.Items[0])
	}

	history := svc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	foundOlderSuppressed := false
	for i := range history {
		if history[i].ID == firstSuppress.Items[0].ID {
			foundOlderSuppressed = true
			if history[i].RestoreStatus != "newer_history_exists" {
				t.Fatalf("expected older suppressed history to be blocked by newer history, got %+v", history[i])
			}
		}
	}
	if !foundOlderSuppressed {
		t.Fatalf("expected older suppressed history to remain in list")
	}
}

func TestAutoRetrySyncWorkerAlertNotificationsRetriesDueItems(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 3, 20, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
		},
		TriggeredAt: now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected initial failed dispatch to succeed: %v", err)
	}
	if len(initial.Items) != 1 || initial.Items[0].NextRetryAt == nil {
		t.Fatalf("expected one retryable failed notification with next_retry_at, got %+v", initial.Items)
	}

	dueAt := initial.Items[0].NextRetryAt.UTC()
	result, err := svc.AutoRetrySyncWorkerAlertNotifications(SyncWorkerAlertNotificationAutoRetryInput{
		TenantID:    "tenant_demo_jakarta",
		Limit:       10,
		RetriedAt:   dueAt,
		MaxAttempts: 3,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			if input.Attempt != 2 {
				t.Fatalf("expected due auto retry to run attempt=2, got %d", input.Attempt)
			}
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected auto retry to succeed: %v", err)
	}
	if result.TotalNotifications != 1 || result.Retried != 1 || result.Failed != 0 || result.Skipped != 0 || result.Suppressed != 0 {
		t.Fatalf("unexpected auto retry result: %+v", result)
	}
	if len(result.Items) != 1 || result.Items[0].Status != "sent" || result.Items[0].Attempt != 2 {
		t.Fatalf("expected one sent retry record, got %+v", result.Items)
	}
	if result.Items[0].NextRetryAt != nil {
		t.Fatalf("expected successful auto retry to clear next_retry_at, got %+v", result.Items[0])
	}
}

func TestAutoRetrySyncWorkerAlertNotificationsSkipsAttemptLimitAndSuppressesDuplicateFingerprints(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 23, 3, 40, 0, 0, time.UTC)
	subscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []SyncWorkerAlertDispatchAlert{
			{
				WorkerAction: "enterprise_hris_webhook_processing_alert",
				WorkerKind:   "hris_webhook",
				WorkerLabel:  "HRIS Webhook Merge",
				Count:        3,
				Threshold:    2,
				Failed:       2,
				Processed:    3,
				Applied:      1,
				ConnectorID:  "connector-talenta-001",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
		},
		TriggeredAt: now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected initial failed dispatch to succeed: %v", err)
	}

	secondFailure, err := svc.RetrySyncWorkerAlertNotification(SyncWorkerAlertNotificationRetryInput{
		TenantID:       "tenant_demo_jakarta",
		NotificationID: initial.Items[0].ID,
		RetriedAt:      now.Add(5 * time.Minute),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected second failure retry to succeed: %v", err)
	}
	if secondFailure.NextRetryAt == nil {
		t.Fatalf("expected latest failed retry to have next_retry_at, got %+v", secondFailure)
	}

	dispatchCalls := 0
	result, err := svc.AutoRetrySyncWorkerAlertNotifications(SyncWorkerAlertNotificationAutoRetryInput{
		TenantID:    "tenant_demo_jakarta",
		Limit:       10,
		MaxAttempts: 2,
		RetriedAt:   secondFailure.NextRetryAt.UTC(),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			dispatchCalls++
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected auto retry attempt-limit path to succeed: %v", err)
	}
	if dispatchCalls != 0 {
		t.Fatalf("expected attempt-limit path to skip dispatch, got %d calls", dispatchCalls)
	}
	if result.TotalNotifications != 2 || result.Retried != 0 || result.Failed != 0 || result.Skipped != 1 || result.Suppressed != 1 {
		t.Fatalf("unexpected auto retry attempt-limit result: %+v", result)
	}
	if len(result.Items) != 1 || result.Items[0].Reason != "auto_retry_attempt_limit" || result.Items[0].Status != "skipped" {
		t.Fatalf("expected one auto_retry_attempt_limit history item, got %+v", result.Items)
	}
	if result.Items[0].NextRetryAt != nil {
		t.Fatalf("expected attempt-limit history to clear next_retry_at, got %+v", result.Items[0])
	}
}
