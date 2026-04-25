package httpx

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/event"
	"github.com/mistypass/cloud/api/internal/modules/hris"
	"github.com/mistypass/cloud/api/internal/modules/space"
)

func ptrTime(t time.Time) *time.Time {
	return &t
}

func TestPendingSyncRequestTenantIDs(t *testing.T) {
	now := time.Now().UTC()
	items := []enterprise.SyncRequestRecord{
		{
			RequestID:     "req-1",
			TenantID:      "tenant_b",
			AccessApplied: false,
			CreatedAt:     now,
		},
		{
			RequestID:     "req-2",
			TenantID:      "tenant_a",
			AccessApplied: false,
			CreatedAt:     now,
		},
		{
			RequestID:     "req-3",
			TenantID:      "tenant_b",
			AccessApplied: false,
			CreatedAt:     now,
		},
		{
			RequestID:     "req-4",
			TenantID:      "tenant_c",
			AccessApplied: true,
			CreatedAt:     now,
		},
		{
			RequestID:     "req-5",
			TenantID:      "",
			AccessApplied: false,
			CreatedAt:     now,
		},
	}

	got := pendingSyncRequestTenantIDs(items)
	if len(got) != 2 {
		t.Fatalf("expected 2 tenant ids, got %d (%v)", len(got), got)
	}
	if got[0] != "tenant_a" || got[1] != "tenant_b" {
		t.Fatalf("unexpected tenant ids order/content: %v", got)
	}
}

func TestPendingJITApprovalExternalSyncTenantIDs(t *testing.T) {
	now := time.Now().UTC()
	cooldown := 30 * time.Second
	items := []enterprise.JITProvisionApproval{
		{
			ID:                 "jap_1",
			TenantID:           "tenant_b",
			Status:             "approved",
			ExternalSyncStatus: "pending",
			UpdatedAt:          now,
		},
		{
			ID:                       "jap_2",
			TenantID:                 "tenant_a",
			Status:                   "approved",
			ExternalSyncStatus:       "failed",
			ExternalSyncAttemptCount: 1,
			ExternalSyncUpdatedAt:    ptrTime(now.Add(-1 * time.Minute)),
			UpdatedAt:                now,
		},
		{
			ID:                       "jap_3",
			TenantID:                 "tenant_c",
			Status:                   "approved",
			ExternalSyncStatus:       "failed",
			ExternalSyncAttemptCount: 5,
			ExternalSyncUpdatedAt:    ptrTime(now.Add(-1 * time.Minute)),
			UpdatedAt:                now,
		},
		{
			ID:                       "jap_4",
			TenantID:                 "tenant_d",
			Status:                   "approved",
			ExternalSyncStatus:       "failed",
			ExternalSyncAttemptCount: 1,
			ExternalSyncUpdatedAt:    ptrTime(now.Add(-10 * time.Second)),
			UpdatedAt:                now,
		},
		{
			ID:                 "jap_5",
			TenantID:           "tenant_e",
			Status:             "approved",
			ExternalSyncStatus: "synced",
			UpdatedAt:          now,
		},
	}

	got := pendingJITApprovalExternalSyncTenantIDs(items, 5, cooldown, now)
	if len(got) != 2 {
		t.Fatalf("expected 2 tenant ids, got %d (%v)", len(got), got)
	}
	if got[0] != "tenant_a" || got[1] != "tenant_b" {
		t.Fatalf("unexpected tenant ids order/content: %v", got)
	}
}

func TestPendingHRISWebhookDLQTenantIDs(t *testing.T) {
	now := time.Now().UTC()
	cooldown := 30 * time.Second
	items := []hris.DeadLetterEntry{
		{
			ID:        "hdlq_1",
			TenantID:  "tenant_b",
			Status:    "dlq",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:           "hdlq_2",
			TenantID:     "tenant_a",
			Status:       "dlq",
			ReplayCount:  1,
			LastReplayAt: ptrTime(now.Add(-1 * time.Minute)),
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "hdlq_3",
			TenantID:     "tenant_c",
			Status:       "dlq",
			ReplayCount:  5,
			LastReplayAt: ptrTime(now.Add(-1 * time.Minute)),
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "hdlq_4",
			TenantID:     "tenant_d",
			Status:       "dlq",
			ReplayCount:  1,
			LastReplayAt: ptrTime(now.Add(-10 * time.Second)),
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:        "hdlq_5",
			TenantID:  "tenant_e",
			Status:    "replaying",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "hdlq_6",
			TenantID:  "tenant_e",
			Status:    "resolved",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	got := pendingHRISWebhookDLQTenantIDs(items, 5, cooldown, now)
	if len(got) != 5 {
		t.Fatalf("expected 5 tenant ids, got %d (%v)", len(got), got)
	}
	if got[0] != "tenant_a" || got[1] != "tenant_b" || got[2] != "tenant_c" || got[3] != "tenant_d" || got[4] != "tenant_e" {
		t.Fatalf("unexpected tenant ids order/content: %v", got)
	}
}

func TestPendingHRISWebhookReceiptTenantIDs(t *testing.T) {
	now := time.Now().UTC()
	items := []enterprise.HRISWebhookReceipt{
		{
			ID:         "whr_1",
			TenantID:   "tenant_b",
			Status:     "received",
			ReceivedAt: now,
		},
		{
			ID:         "whr_2",
			TenantID:   "tenant_a",
			Status:     "received",
			ReceivedAt: now,
		},
		{
			ID:         "whr_3",
			TenantID:   "tenant_b",
			Status:     "received",
			ReceivedAt: now,
		},
		{
			ID:         "whr_4",
			TenantID:   "tenant_c",
			Status:     "failed",
			ReceivedAt: now,
		},
		{
			ID:         "whr_5",
			TenantID:   "tenant_d",
			Status:     "processing",
			ReceivedAt: now,
		},
		{
			ID:         "whr_6",
			TenantID:   "",
			Status:     "received",
			ReceivedAt: now,
		},
	}

	got := pendingHRISWebhookReceiptTenantIDs(items)
	if len(got) != 4 {
		t.Fatalf("expected 4 tenant ids, got %d (%v)", len(got), got)
	}
	if got[0] != "tenant_a" || got[1] != "tenant_b" || got[2] != "tenant_c" || got[3] != "tenant_d" {
		t.Fatalf("unexpected tenant ids order/content: %v", got)
	}
}

func TestAppendEnterpriseSyncWorkerAlertAudit(t *testing.T) {
	auditSvc := audit.NewService()
	s := &server{
		auditSvc: auditSvc,
	}

	before := len(auditSvc.List("tenant_demo_jakarta"))
	result := enterprise.BatchPendingSyncReconcileResult{
		Processed:             4,
		Applied:               1,
		Failed:                3,
		SkippedByAttemptLimit: 2,
		SkippedByCooldown:     1,
	}

	s.appendEnterpriseSyncWorkerAlertAudit("tenant_demo_jakarta", result, 3)

	afterItems := auditSvc.List("tenant_demo_jakarta")
	if len(afterItems) != before+1 {
		t.Fatalf("expected one new audit item, before=%d after=%d", before, len(afterItems))
	}
	latest := afterItems[0]
	if latest.Action != "enterprise_sync_reconcile_worker_alert" {
		t.Fatalf("unexpected action: %s", latest.Action)
	}
	if latest.Actor != "enterprise_sync_worker" {
		t.Fatalf("unexpected actor: %s", latest.Actor)
	}
	if latest.Role != "system" {
		t.Fatalf("unexpected role: %s", latest.Role)
	}
	if latest.Source != "enterprise_sync_worker" {
		t.Fatalf("unexpected source: %s", latest.Source)
	}
	if !strings.Contains(latest.Target, "failed=3") || !strings.Contains(latest.Target, "threshold=3") {
		t.Fatalf("unexpected alert target payload: %s", latest.Target)
	}
}

func TestAppendEnterpriseSyncWorkerAlertAuditBelowThresholdNoop(t *testing.T) {
	auditSvc := audit.NewService()
	s := &server{
		auditSvc: auditSvc,
	}

	before := len(auditSvc.List("tenant_demo_jakarta"))
	result := enterprise.BatchPendingSyncReconcileResult{
		Processed: 3,
		Applied:   2,
		Failed:    1,
	}

	s.appendEnterpriseSyncWorkerAlertAudit("tenant_demo_jakarta", result, 2)

	after := len(auditSvc.List("tenant_demo_jakarta"))
	if after != before {
		t.Fatalf("expected no audit append when below threshold, before=%d after=%d", before, after)
	}
}

func TestAppendEnterpriseJITApprovalExternalSyncWorkerAlertAudit(t *testing.T) {
	auditSvc := audit.NewService()
	s := &server{
		auditSvc: auditSvc,
	}

	before := len(auditSvc.List("tenant_demo_jakarta"))
	result := enterpriseJITApprovalExternalSyncWorkerResult{
		Processed:             4,
		Synced:                1,
		Failed:                3,
		SkippedByAttemptLimit: 2,
		SkippedByCooldown:     1,
	}

	s.appendEnterpriseJITApprovalExternalSyncWorkerAlertAudit("tenant_demo_jakarta", result, 3)

	afterItems := auditSvc.List("tenant_demo_jakarta")
	if len(afterItems) != before+1 {
		t.Fatalf("expected one new audit item, before=%d after=%d", before, len(afterItems))
	}
	latest := afterItems[0]
	if latest.Action != "enterprise_jit_approval_external_sync_worker_alert" {
		t.Fatalf("unexpected action: %s", latest.Action)
	}
	if !strings.Contains(latest.Target, "failed=3") || !strings.Contains(latest.Target, "threshold=3") {
		t.Fatalf("unexpected alert target payload: %s", latest.Target)
	}
}

func TestAppendEnterpriseHRISWebhookDLQWorkerAlertAudit(t *testing.T) {
	auditSvc := audit.NewService()
	s := &server{
		auditSvc: auditSvc,
	}

	before := len(auditSvc.List("tenant_demo_jakarta"))
	result := enterpriseHRISWebhookDLQWorkerResult{
		Processed:             4,
		Replayed:              1,
		Failed:                3,
		SkippedByInFlight:     2,
		SkippedByAttemptLimit: 2,
		SkippedByCooldown:     1,
	}

	s.appendEnterpriseHRISWebhookDLQWorkerAlertAudit("tenant_demo_jakarta", result, 3)

	afterItems := auditSvc.List("tenant_demo_jakarta")
	if len(afterItems) != before+1 {
		t.Fatalf("expected one new audit item, before=%d after=%d", before, len(afterItems))
	}
	latest := afterItems[0]
	if latest.Action != "enterprise_hris_webhook_dlq_worker_alert" {
		t.Fatalf("unexpected action: %s", latest.Action)
	}
	if !strings.Contains(latest.Target, "failed=3") ||
		!strings.Contains(latest.Target, "threshold=3") ||
		!strings.Contains(latest.Target, "skipped_in_flight=2") {
		t.Fatalf("unexpected alert target payload: %s", latest.Target)
	}
}

func TestAppendEnterpriseHRISWebhookReceiptWorkerAlertAudit(t *testing.T) {
	auditSvc := audit.NewService()
	s := &server{
		auditSvc: auditSvc,
	}

	before := len(auditSvc.List("tenant_demo_jakarta"))
	result := enterpriseHRISWebhookReceiptWorkerResult{
		Processed:             4,
		Synced:                1,
		Skipped:               1,
		Failed:                3,
		SkippedByAttemptLimit: 2,
		SkippedByCooldown:     1,
		LastConnectorID:       "connector-talenta-001",
		LastVendor:            "talenta",
		LastRequestID:         "mekari-webhook-001",
		LastEventType:         "talenta.employee.detail.created",
	}

	s.appendEnterpriseHRISWebhookReceiptWorkerAlertAudit("tenant_demo_jakarta", result, 3)

	afterItems := auditSvc.List("tenant_demo_jakarta")
	if len(afterItems) != before+1 {
		t.Fatalf("expected one new audit item, before=%d after=%d", before, len(afterItems))
	}
	latest := afterItems[0]
	if latest.Action != "enterprise_hris_webhook_receipt_worker_alert" {
		t.Fatalf("unexpected action: %s", latest.Action)
	}
	if !strings.Contains(latest.Target, "skipped_attempt_limit=2") || !strings.Contains(latest.Target, "skipped_cooldown=1") {
		t.Fatalf("unexpected retry metrics in alert target payload: %s", latest.Target)
	}
	if !strings.Contains(latest.Target, "connector_id=connector-talenta-001") || !strings.Contains(latest.Target, "event_type=talenta.employee.detail.created") {
		t.Fatalf("unexpected connector metadata in alert target payload: %s", latest.Target)
	}
}

func TestAppendEnterpriseHRISPullWorkerAlertAudit(t *testing.T) {
	auditSvc := audit.NewService()
	s := &server{
		auditSvc: auditSvc,
	}

	before := len(auditSvc.List("tenant_demo_jakarta"))
	result := enterpriseHRISPullWorkerResult{
		Processed:             4,
		Synced:                1,
		Failed:                3,
		ConsecutiveFailures:   5,
		FailureAgeSeconds:     7200,
		SkippedByInFlight:     2,
		SkippedByAttemptLimit: 2,
		SkippedByCooldown:     1,
	}

	s.appendEnterpriseHRISPullWorkerAlertAudit("tenant_demo_jakarta", result, 3)

	afterItems := auditSvc.List("tenant_demo_jakarta")
	if len(afterItems) != before+1 {
		t.Fatalf("expected one new audit item, before=%d after=%d", before, len(afterItems))
	}
	latest := afterItems[0]
	if latest.Action != "enterprise_hris_pull_worker_alert" {
		t.Fatalf("unexpected action: %s", latest.Action)
	}
	if !strings.Contains(latest.Target, "failed=3") ||
		!strings.Contains(latest.Target, "threshold=3") ||
		!strings.Contains(latest.Target, "skipped_in_flight=2") ||
		!strings.Contains(latest.Target, "consecutive_failures=5") ||
		!strings.Contains(latest.Target, "failure_age_seconds=7200") {
		t.Fatalf("unexpected alert target payload: %s", latest.Target)
	}
}

func TestAppendEnterpriseHRISWebhookProcessingAlertAudit(t *testing.T) {
	auditSvc := audit.NewService()
	s := &server{
		auditSvc: auditSvc,
	}

	before := len(auditSvc.List("tenant_demo_jakarta"))
	s.appendEnterpriseHRISWebhookProcessingAlertAudit(
		"tenant_demo_jakarta",
		"connector-talenta-001",
		"talenta",
		"talenta.attendance.scheduler.changeschedule",
		"mekari-webhook-001",
		"merge",
	)

	afterItems := auditSvc.List("tenant_demo_jakarta")
	if len(afterItems) != before+1 {
		t.Fatalf("expected one new audit item, before=%d after=%d", before, len(afterItems))
	}
	latest := afterItems[0]
	if latest.Action != "enterprise_hris_webhook_processing_alert" {
		t.Fatalf("unexpected action: %s", latest.Action)
	}
	if latest.Actor != "enterprise_sync_worker" {
		t.Fatalf("unexpected actor: %s", latest.Actor)
	}
	if latest.Source != "enterprise_sync_worker" {
		t.Fatalf("unexpected source: %s", latest.Source)
	}
	if !strings.Contains(latest.Target, "failed=1") ||
		!strings.Contains(latest.Target, "threshold=1") ||
		!strings.Contains(latest.Target, "failure_stage=merge") {
		t.Fatalf("unexpected alert target payload: %s", latest.Target)
	}
}

func TestParseEnterpriseSyncWorkerAlertMetrics(t *testing.T) {
	metrics := parseEnterpriseSyncWorkerAlertMetrics(
		"failed=4 threshold=3 processed=5 applied=1 consecutive_failures=4 failure_age_seconds=1800 skipped_attempt_limit=2 skipped_cooldown=1",
	)
	if metrics.Failed != 4 {
		t.Fatalf("failed mismatch: got %d", metrics.Failed)
	}
	if metrics.Threshold != 3 {
		t.Fatalf("threshold mismatch: got %d", metrics.Threshold)
	}
	if metrics.Processed != 5 {
		t.Fatalf("processed mismatch: got %d", metrics.Processed)
	}
	if metrics.Applied != 1 {
		t.Fatalf("applied mismatch: got %d", metrics.Applied)
	}
	if metrics.ConsecutiveFailures != 4 {
		t.Fatalf("consecutive_failures mismatch: got %d", metrics.ConsecutiveFailures)
	}
	if metrics.FailureAgeSeconds != 1800 {
		t.Fatalf("failure_age_seconds mismatch: got %d", metrics.FailureAgeSeconds)
	}
	if metrics.SkippedByAttemptLimit != 2 {
		t.Fatalf("skipped_by_attempt_limit mismatch: got %d", metrics.SkippedByAttemptLimit)
	}
	if metrics.SkippedByCooldown != 1 {
		t.Fatalf("skipped_by_cooldown mismatch: got %d", metrics.SkippedByCooldown)
	}

	metrics = parseEnterpriseSyncWorkerAlertMetrics(
		"failed=2 threshold=1 processed=3 synced=2 skipped_attempt_limit=0 skipped_cooldown=0",
	)
	if metrics.Applied != 2 {
		t.Fatalf("expected synced fallback to populate applied metric, got %d", metrics.Applied)
	}
}

func TestParseEnterpriseSyncWorkerAlertMetricsInvalidInput(t *testing.T) {
	metrics := parseEnterpriseSyncWorkerAlertMetrics("failed=bad threshold=2 malformed-pair")
	if metrics.Failed != 0 {
		t.Fatalf("expected invalid failed to fallback 0, got %d", metrics.Failed)
	}
	if metrics.Threshold != 2 {
		t.Fatalf("expected threshold=2, got %d", metrics.Threshold)
	}
	if metrics.Processed != 0 || metrics.Applied != 0 || metrics.SkippedByAttemptLimit != 0 || metrics.SkippedByCooldown != 0 {
		t.Fatalf("unexpected non-zero metrics: %+v", metrics)
	}
}

func TestBuildEnterpriseSyncWorkerAlerts(t *testing.T) {
	now := time.Now().UTC()
	items := buildEnterpriseSyncWorkerAlerts([]audit.Log{
		{
			TenantID: "tenant_demo_jakarta",
			ID:       "aud_worker_1",
			Actor:    "enterprise_sync_worker",
			Role:     "system",
			Action:   "enterprise_sync_reconcile_worker_alert",
			Target:   "failed=1 threshold=1 processed=2 applied=1 skipped_attempt_limit=0 skipped_cooldown=0",
			Source:   "enterprise_sync_worker",
			At:       now,
		},
	})
	if len(items) != 1 {
		t.Fatalf("expected one alert item, got %d", len(items))
	}
	if items[0].ID != "aud_worker_1" {
		t.Fatalf("id mismatch: %s", items[0].ID)
	}
	if items[0].Failed != 1 || items[0].Threshold != 1 || items[0].Processed != 2 || items[0].Applied != 1 {
		t.Fatalf("metric mismatch: %+v", items[0])
	}
	if items[0].WorkerKind != "sync_reconcile" {
		t.Fatalf("unexpected worker kind: %s", items[0].WorkerKind)
	}
	if items[0].WorkerLabel != "Enterprise Sync Reconcile" {
		t.Fatalf("unexpected worker label: %s", items[0].WorkerLabel)
	}
	if items[0].RawTarget == "" {
		t.Fatalf("expected raw_target to be set")
	}
}

func TestBuildEnterpriseSyncWorkerAlertSummary(t *testing.T) {
	now := time.Now().UTC()
	items := buildEnterpriseSyncWorkerAlertSummary([]audit.Log{
		{
			TenantID: "tenant_b",
			Action:   "enterprise_sync_reconcile_worker_alert",
			Target:   "failed=1 threshold=1 processed=2 applied=0 skipped_attempt_limit=0 skipped_cooldown=0",
			At:       now.Add(-2 * time.Minute),
		},
		{
			TenantID: "tenant_a",
			Action:   "enterprise_hris_pull_worker_alert",
			Target:   "failed=2 threshold=1 processed=3 applied=0 consecutive_failures=4 failure_age_seconds=3600 skipped_attempt_limit=1 skipped_cooldown=0",
			At:       now.Add(-time.Minute),
		},
		{
			TenantID: "tenant_b",
			Action:   "enterprise_sync_reconcile_worker_alert",
			Target:   "failed=3 threshold=2 processed=4 applied=1 skipped_attempt_limit=0 skipped_cooldown=1",
			At:       now,
		},
		{
			TenantID: "tenant_b",
			Action:   "enterprise_hris_webhook_dlq_worker_alert",
			Target:   "failed=1 threshold=1 processed=1 replayed=1 skipped_attempt_limit=0 skipped_cooldown=0",
			At:       now.Add(-30 * time.Second),
		},
	})
	if len(items) != 3 {
		t.Fatalf("expected three summary items, got %d", len(items))
	}
	if items[0].TenantID != "tenant_b" || items[0].WorkerAction != "enterprise_sync_reconcile_worker_alert" {
		t.Fatalf("expected tenant_b sync reconcile first by last_seen desc, got %+v", items[0])
	}
	if items[0].Count != 2 {
		t.Fatalf("tenant_b count mismatch: %d", items[0].Count)
	}
	if items[0].LastFailed != 3 || items[0].LastThreshold != 2 || items[0].LastProcessed != 4 || items[0].LastApplied != 1 {
		t.Fatalf("tenant_b latest metrics mismatch: %+v", items[0])
	}
	if !items[0].FirstSeenAt.Equal(now.Add(-2 * time.Minute)) {
		t.Fatalf("tenant_b first_seen mismatch: %s", items[0].FirstSeenAt)
	}
	if !items[0].LastSeenAt.Equal(now) {
		t.Fatalf("tenant_b last_seen mismatch: %s", items[0].LastSeenAt)
	}
	if items[1].TenantID != "tenant_b" || items[1].WorkerAction != "enterprise_hris_webhook_dlq_worker_alert" || items[1].LastApplied != 1 {
		t.Fatalf("tenant_b dlq summary mismatch: %+v", items[1])
	}
	if items[2].TenantID != "tenant_a" || items[2].WorkerAction != "enterprise_hris_pull_worker_alert" || items[2].Count != 1 {
		t.Fatalf("tenant_a pull summary mismatch: %+v", items[2])
	}
	if items[2].LastConsecutiveFailures != 4 || items[2].LastFailureAgeSeconds != 3600 {
		t.Fatalf("tenant_a pull stateful metrics mismatch: %+v", items[2])
	}
}

func TestBuildEnterpriseSyncWorkerAlertDispatchAlertsUsesGranularFingerprintBuckets(t *testing.T) {
	now := time.Now().UTC()
	items := []enterpriseSyncWorkerAlertItem{
		{
			TenantID:     "tenant_demo_jakarta",
			WorkerAction: "enterprise_hris_webhook_processing_alert",
			WorkerKind:   "hris_webhook",
			WorkerLabel:  "HRIS Webhook Processing",
			ConnectorID:  "connector-talenta-a",
			Vendor:       "talenta",
			FailureStage: "merge",
			EventType:    "talenta.employee.updated",
			Failed:       2,
			Threshold:    2,
			Processed:    3,
			Applied:      1,
			At:           now,
		},
		{
			TenantID:     "tenant_demo_jakarta",
			WorkerAction: "enterprise_hris_webhook_processing_alert",
			WorkerKind:   "hris_webhook",
			WorkerLabel:  "HRIS Webhook Processing",
			ConnectorID:  "connector-talenta-a",
			Vendor:       "talenta",
			FailureStage: "merge",
			EventType:    "talenta.employee.updated",
			Failed:       1,
			Threshold:    2,
			Processed:    2,
			Applied:      1,
			At:           now.Add(-1 * time.Minute),
		},
		{
			TenantID:     "tenant_demo_jakarta",
			WorkerAction: "enterprise_hris_webhook_processing_alert",
			WorkerKind:   "hris_webhook",
			WorkerLabel:  "HRIS Webhook Processing",
			ConnectorID:  "connector-talenta-b",
			Vendor:       "talenta",
			FailureStage: "persist",
			EventType:    "talenta.employee.updated",
			Failed:       1,
			Threshold:    2,
			Processed:    1,
			Applied:      0,
			At:           now.Add(-30 * time.Second),
		},
	}

	dispatchAlerts := buildEnterpriseSyncWorkerAlertDispatchAlerts(items, 2, []string{"enterprise_hris_webhook_processing_alert"})
	if len(dispatchAlerts) != 1 {
		t.Fatalf("expected only one granular dispatch alert above threshold, got %d (%+v)", len(dispatchAlerts), dispatchAlerts)
	}
	if dispatchAlerts[0].ConnectorID != "connector-talenta-a" || dispatchAlerts[0].FailureStage != "merge" {
		t.Fatalf("expected dispatch alert to preserve granular connector/stage, got %+v", dispatchAlerts[0])
	}
	if dispatchAlerts[0].Count != 2 {
		t.Fatalf("expected granular bucket count=2, got %+v", dispatchAlerts[0])
	}
}

func TestListEnterpriseSyncWorkerAlertLogs(t *testing.T) {
	auditSvc := audit.NewService()
	s := &server{
		auditSvc: auditSvc,
	}

	if _, err := auditSvc.Append("tenant_demo_jakarta", "enterprise_sync_worker", "system", "enterprise_sync_reconcile_worker_alert", "failed=1 threshold=1 processed=2 applied=1", "enterprise_sync_worker"); err != nil {
		t.Fatalf("append sync reconcile alert: %v", err)
	}
	if _, err := auditSvc.Append("tenant_demo_jakarta", "enterprise_sync_worker", "system", "enterprise_hris_webhook_dlq_worker_alert", "failed=2 threshold=1 processed=2 replayed=1", "enterprise_sync_worker"); err != nil {
		t.Fatalf("append hris dlq alert: %v", err)
	}
	if _, err := auditSvc.Append("tenant_demo_jakarta", "enterprise_sync_worker", "system", "enterprise_hris_pull_worker_alert", "failed=3 threshold=1 processed=3 synced=1", "enterprise_sync_worker"); err != nil {
		t.Fatalf("append hris pull alert: %v", err)
	}
	if _, err := auditSvc.Append("tenant_demo_jakarta", "enterprise_sync_worker", "system", "enterprise_hris_webhook_processing_alert", "failed=1 threshold=1 processed=1 applied=0 failure_stage=merge", "enterprise_sync_worker"); err != nil {
		t.Fatalf("append hris webhook processing alert: %v", err)
	}

	items := s.listEnterpriseSyncWorkerAlertLogs("tenant_demo_jakarta")
	if len(items) < 4 {
		t.Fatalf("expected at least four worker alert logs, got %d", len(items))
	}

	seen := map[string]bool{}
	for i := range items {
		switch items[i].Action {
		case "enterprise_sync_reconcile_worker_alert", "enterprise_hris_webhook_dlq_worker_alert", "enterprise_hris_pull_worker_alert", "enterprise_hris_webhook_processing_alert":
			seen[items[i].Action] = true
		}
	}
	if !seen["enterprise_sync_reconcile_worker_alert"] ||
		!seen["enterprise_hris_webhook_dlq_worker_alert"] ||
		!seen["enterprise_hris_pull_worker_alert"] ||
		!seen["enterprise_hris_webhook_processing_alert"] {
		t.Fatalf("expected combined worker alert actions, got %+v", seen)
	}
}

func TestParseRFC3339TimeRange(t *testing.T) {
	since, until, err := parseRFC3339TimeRange(
		"2026-04-12T00:00:00Z",
		"2026-04-13T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("expected valid range, got error: %v", err)
	}
	if since == nil || until == nil {
		t.Fatalf("expected both since/until to be set")
	}
	if !since.Before(*until) {
		t.Fatalf("expected since < until")
	}
}

func TestParseRFC3339TimeRangeInvalid(t *testing.T) {
	_, _, err := parseRFC3339TimeRange("bad-time", "")
	if err == nil || !strings.Contains(err.Error(), "since must be RFC3339") {
		t.Fatalf("expected since parse error, got: %v", err)
	}

	_, _, err = parseRFC3339TimeRange("", "bad-time")
	if err == nil || !strings.Contains(err.Error(), "until must be RFC3339") {
		t.Fatalf("expected until parse error, got: %v", err)
	}

	_, _, err = parseRFC3339TimeRange("2026-04-13T00:00:00Z", "2026-04-12T00:00:00Z")
	if err == nil || !strings.Contains(err.Error(), "since must be <= until") {
		t.Fatalf("expected range ordering error, got: %v", err)
	}
}

func TestFilterAuditLogsByTimeRange(t *testing.T) {
	now := time.Now().UTC()
	logs := []audit.Log{
		{ID: "a1", At: now.Add(-2 * time.Hour)},
		{ID: "a2", At: now.Add(-time.Hour)},
		{ID: "a3", At: now},
	}
	since := now.Add(-90 * time.Minute)
	until := now.Add(-30 * time.Minute)

	filtered := filterAuditLogsByTimeRange(logs, &since, &until)
	if len(filtered) != 1 {
		t.Fatalf("expected one filtered log, got %d", len(filtered))
	}
	if filtered[0].ID != "a2" {
		t.Fatalf("unexpected filtered log id: %s", filtered[0].ID)
	}
}

func TestIsGatewayBatchFailureRetryable(t *testing.T) {
	if isGatewayBatchFailureRetryable(nil) {
		t.Fatalf("nil error should not be retryable")
	}
	if isGatewayBatchFailureRetryable(event.ErrGatewayIDRequired) {
		t.Fatalf("validation error should not be retryable")
	}
	if !isGatewayBatchFailureRetryable(errors.New("state store unavailable")) {
		t.Fatalf("non-validation error should be retryable")
	}
}

func TestGatewayBatchSuggestedCheckpointID(t *testing.T) {
	receivedAt := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	checkpointID := gatewayBatchSuggestedCheckpointID("gw_demo_001", "priority", receivedAt)
	if !strings.HasPrefix(checkpointID, "gw_demo_001-priority-") {
		t.Fatalf("unexpected checkpoint id prefix: %s", checkpointID)
	}
	defaultCheckpointID := gatewayBatchSuggestedCheckpointID("", "", receivedAt)
	if !strings.HasPrefix(defaultCheckpointID, "gateway-default-") {
		t.Fatalf("unexpected default checkpoint id prefix: %s", defaultCheckpointID)
	}
}

func TestGatewayBatchQueueHintDecision(t *testing.T) {
	if code := gatewayBatchQueueStatusCode(0, 0); code != "QUEUE_READY_TO_CHECKPOINT" {
		t.Fatalf("unexpected status code for clean batch: %s", code)
	}
	if action := gatewayBatchNextAction(0, 0); action != "report_checkpoint" {
		t.Fatalf("unexpected action for clean batch: %s", action)
	}
	if code := gatewayBatchQueueStatusCode(1, 1); code != "QUEUE_RETRY_SUBSET_REQUIRED" {
		t.Fatalf("unexpected status code for retryable failures: %s", code)
	}
	if action := gatewayBatchNextAction(1, 1); action != "replay_retry_subset_then_report_checkpoint" {
		t.Fatalf("unexpected action for retryable failures: %s", action)
	}
	if code := gatewayBatchQueueStatusCode(1, 0); code != "QUEUE_PARTIAL_NON_RETRYABLE" {
		t.Fatalf("unexpected status code for non-retryable failures: %s", code)
	}
	if action := gatewayBatchNextAction(1, 0); action != "report_checkpoint_with_non_retryable_failures" {
		t.Fatalf("unexpected action for non-retryable failures: %s", action)
	}
}

func TestGatewayEventAuditActionMapping(t *testing.T) {
	if action := gatewayAccessAuditAction("accepted"); action != "gateway_access_grant_recorded" {
		t.Fatalf("unexpected accepted mapping: %s", action)
	}
	if action := gatewayAccessAuditAction("denied"); action != "gateway_access_deny_recorded" {
		t.Fatalf("unexpected denied mapping: %s", action)
	}
	if action := gatewayAccessAuditAction("manual_review"); action != "gateway_access_event_recorded" {
		t.Fatalf("unexpected fallback mapping: %s", action)
	}

	if action := gatewayDeviceAuditAction("tamper_opened"); action != "gateway_tamper_event_recorded" {
		t.Fatalf("unexpected tamper mapping: %s", action)
	}
	if action := gatewayDeviceAuditAction("door_timeout"); action != "gateway_door_timeout_recorded" {
		t.Fatalf("unexpected timeout mapping: %s", action)
	}
	if action := gatewayDeviceAuditAction("rex_pressed"); action != "gateway_rex_event_recorded" {
		t.Fatalf("unexpected rex mapping: %s", action)
	}
}

func TestGatewayEventAuditTargetFormatting(t *testing.T) {
	occurredAt := time.Date(2026, 4, 17, 2, 3, 4, 0, time.UTC)

	accessTarget := gatewayAccessEventAuditTarget(
		"gw_demo_001",
		"default",
		"evt-1",
		"access_granted",
		"accepted",
		"door_jkt_001",
		"legacy_reader",
		"idem-1",
		false,
		occurredAt,
	)
	if !strings.Contains(accessTarget, "gateway=gw_demo_001") ||
		!strings.Contains(accessTarget, "event=evt-1") ||
		!strings.Contains(accessTarget, "deduplicated=false") {
		t.Fatalf("unexpected access target: %s", accessTarget)
	}

	deviceTarget := gatewayDeviceEventAuditTarget(
		"gw_demo_001",
		"priority",
		"evt-2",
		"tamper_opened",
		"warning",
		"panel open",
		"idem-2",
		true,
		occurredAt,
	)
	if !strings.Contains(deviceTarget, "queue=priority") ||
		!strings.Contains(deviceTarget, "type=tamper_opened") ||
		!strings.Contains(deviceTarget, "deduplicated=true") {
		t.Fatalf("unexpected device target: %s", deviceTarget)
	}
}

func TestGatewayConfigAuthzResolveStatus(t *testing.T) {
	codes := gatewayConfigAuthzStatusCodes{
		Fresh:   "AUTHZ_CACHE_FRESH",
		Stale:   "AUTHZ_CACHE_STALE",
		Missing: "AUTHZ_CACHE_MISSING",
		Drift:   "AUTHZ_CACHE_DRIFT",
	}
	if status := gatewayConfigAuthzResolveStatus(codes, "", "authz-new", "authz-old"); status != codes.Missing {
		t.Fatalf("missing status mismatch: %s", status)
	}
	if status := gatewayConfigAuthzResolveStatus(codes, "authz-new", "authz-new", "authz-old"); status != codes.Fresh {
		t.Fatalf("fresh status mismatch: %s", status)
	}
	if status := gatewayConfigAuthzResolveStatus(codes, "authz-old", "authz-new", "authz-old"); status != codes.Stale {
		t.Fatalf("stale status mismatch: %s", status)
	}
	if status := gatewayConfigAuthzResolveStatus(codes, "authz-x", "authz-new", "authz-old"); status != codes.Drift {
		t.Fatalf("drift status mismatch: %s", status)
	}
}

func TestParseGatewayCheckpointAuditMetrics(t *testing.T) {
	metrics := parseGatewayCheckpointAuditMetrics(
		"gateway=gw_demo_001 queue=priority checkpoint=seq-42 acked=12 last_request=rq-42",
	)
	if metrics.GatewayID != "gw_demo_001" {
		t.Fatalf("gateway mismatch: %s", metrics.GatewayID)
	}
	if metrics.Queue != "priority" {
		t.Fatalf("queue mismatch: %s", metrics.Queue)
	}
	if metrics.CheckpointID != "seq-42" {
		t.Fatalf("checkpoint mismatch: %s", metrics.CheckpointID)
	}
	if metrics.AckedCount != 12 {
		t.Fatalf("acked mismatch: %d", metrics.AckedCount)
	}
	if metrics.LastRequest != "rq-42" {
		t.Fatalf("last_request mismatch: %s", metrics.LastRequest)
	}

	defaultQueue := parseGatewayCheckpointAuditMetrics("gateway=gw_demo_001 acked=bad")
	if defaultQueue.Queue != "default" {
		t.Fatalf("expected default queue, got %s", defaultQueue.Queue)
	}
	if defaultQueue.AckedCount != 0 {
		t.Fatalf("invalid acked should fallback 0, got %d", defaultQueue.AckedCount)
	}
}

func TestBuildGatewayCheckpointWindowTrends(t *testing.T) {
	now := time.Now().UTC()
	logs := []audit.Log{
		{
			At:     now.Add(-3 * time.Minute),
			Target: "gateway=gw_demo_001 queue=default checkpoint=seq-1 acked=2 last_request=rq-1",
		},
		{
			At:     now.Add(-2 * time.Minute),
			Target: "gateway=gw_demo_002 queue=default checkpoint=seq-1 acked=5 last_request=rq-1",
		},
		{
			At:     now.Add(-time.Minute),
			Target: "gateway=gw_demo_001 queue=default checkpoint=seq-2 acked=7 last_request=rq-2",
		},
		{
			At:     now.Add(-30 * time.Second),
			Target: "gateway=gw_demo_001 queue=priority checkpoint=seq-9 acked=4 last_request=rq-9",
		},
	}

	trends, summary := buildGatewayCheckpointWindowTrends(
		logs,
		map[string]struct{}{"gw_demo_001": {}},
		"",
		"",
	)
	if summary.ReportTotal != 3 {
		t.Fatalf("report_total mismatch: %d", summary.ReportTotal)
	}
	if summary.GatewayTotal != 1 {
		t.Fatalf("gateway_total mismatch: %d", summary.GatewayTotal)
	}
	if summary.QueueTotal != 2 {
		t.Fatalf("queue_total mismatch: %d", summary.QueueTotal)
	}
	if summary.AckedDeltaTotal != 5 {
		t.Fatalf("acked_delta_total mismatch: %d", summary.AckedDeltaTotal)
	}
	if summary.Direction != "up" {
		t.Fatalf("direction mismatch: %s", summary.Direction)
	}
	if summary.LastReportAt == nil || !summary.LastReportAt.Equal(now.Add(-30*time.Second)) {
		t.Fatalf("last_report_at mismatch: %v", summary.LastReportAt)
	}

	defaultTrend, ok := trends[gatewayCheckpointTrendKey("gw_demo_001", "default")]
	if !ok {
		t.Fatalf("default queue trend not found")
	}
	if defaultTrend.ReportTotal != 2 {
		t.Fatalf("default report_total mismatch: %d", defaultTrend.ReportTotal)
	}
	if defaultTrend.AckedDelta != 5 {
		t.Fatalf("default acked_delta mismatch: %d", defaultTrend.AckedDelta)
	}
	if defaultTrend.Direction != "up" {
		t.Fatalf("default direction mismatch: %s", defaultTrend.Direction)
	}
	if defaultTrend.FirstReportAt == nil || !defaultTrend.FirstReportAt.Equal(now.Add(-3*time.Minute)) {
		t.Fatalf("default first_report_at mismatch: %v", defaultTrend.FirstReportAt)
	}
	if defaultTrend.LastReportAt == nil || !defaultTrend.LastReportAt.Equal(now.Add(-time.Minute)) {
		t.Fatalf("default last_report_at mismatch: %v", defaultTrend.LastReportAt)
	}

	filtered, filteredSummary := buildGatewayCheckpointWindowTrends(
		logs,
		map[string]struct{}{"gw_demo_001": {}},
		"gw_demo_001",
		"default",
	)
	if filteredSummary.ReportTotal != 2 || filteredSummary.QueueTotal != 1 {
		t.Fatalf("filtered summary mismatch: %+v", filteredSummary)
	}
	if _, exists := filtered[gatewayCheckpointTrendKey("gw_demo_001", "priority")]; exists {
		t.Fatalf("priority queue should be filtered out")
	}
}

func TestGatewayBatchForcedRetryableError(t *testing.T) {
	s := &server{
		cfg: config.Config{
			GatewayEventsBatchForceRetryableError:  true,
			GatewayEventsBatchForceRetryablePrefix: "force-retry-",
		},
		gatewayBatchFailureSeen: map[string]struct{}{},
	}

	first := s.gatewayBatchForcedRetryableError("force-retry-001")
	if first == nil {
		t.Fatalf("expected first forced failure")
	}
	if !isGatewayBatchFailureRetryable(first) {
		t.Fatalf("forced failure should be retryable")
	}

	second := s.gatewayBatchForcedRetryableError("force-retry-001")
	if second != nil {
		t.Fatalf("expected forced failure to trigger only once per event id")
	}

	other := s.gatewayBatchForcedRetryableError("gwea-001")
	if other != nil {
		t.Fatalf("non-prefixed event should not be forced failure")
	}
}

func TestGatewayBatchForcedRetryableErrorDisabled(t *testing.T) {
	s := &server{
		cfg: config.Config{
			GatewayEventsBatchForceRetryableError: false,
		},
		gatewayBatchFailureSeen: map[string]struct{}{},
	}
	if err := s.gatewayBatchForcedRetryableError("force-retry-001"); err != nil {
		t.Fatalf("expected no forced failure when feature disabled")
	}
}

func TestBuildGatewayConfigAuthzCacheFiltersByBoundDoors(t *testing.T) {
	spaceSvc := space.NewService()
	accessSvc := access.NewService()

	allPolicy, err := accessSvc.CreatePolicy(
		"tenant_demo_jakarta",
		"Tenant Wide Access",
		"all",
		"",
		"",
		"",
		"24x7",
		1,
		"active",
	)
	if err != nil {
		t.Fatalf("create all policy error: %v", err)
	}
	inactivePolicy, err := accessSvc.CreatePolicy(
		"tenant_demo_jakarta",
		"Inactive Building Policy",
		"building",
		"building_demo_001",
		"",
		"",
		"24x7",
		1,
		"inactive",
	)
	if err != nil {
		t.Fatalf("create inactive policy error: %v", err)
	}
	matchedTemporary, err := accessSvc.CreateTemporaryAccess(
		"tenant_demo_jakarta",
		"door",
		"building_demo_001",
		"area_demo_001",
		"door_jkt_001",
		"wallet",
		"Scoped Temp User",
		"",
		"+62-811-0000-0001",
		"scoped.temp@example.com",
		"",
		"",
		"2026-12-31 23:59",
		"usr_tenant_admin_jkt_001",
		"tenant.admin@sudirman.co",
		"tenant_admin",
	)
	if err != nil {
		t.Fatalf("create matched temporary access error: %v", err)
	}
	unmatchedTemporary, err := accessSvc.CreateTemporaryAccess(
		"tenant_demo_jakarta",
		"building",
		"building_demo_002",
		"",
		"",
		"wallet",
		"Out Of Scope Temp",
		"",
		"+62-811-0000-0002",
		"out.scope.temp@example.com",
		"",
		"",
		"2026-12-31 23:59",
		"usr_tenant_admin_jkt_001",
		"tenant.admin@sudirman.co",
		"tenant_admin",
	)
	if err != nil {
		t.Fatalf("create unmatched temporary access error: %v", err)
	}
	unmatchedVisitor, err := accessSvc.CreateVisitorPass(
		"tenant_demo_jakarta",
		"building_demo_002",
		"QA Host",
		"Out Visitor",
		"email_qr",
		"2026-12-31 23:59",
	)
	if err != nil {
		t.Fatalf("create unmatched visitor pass error: %v", err)
	}
	groupInScope, err := accessSvc.CreateUserGroup(
		"tenant_demo_jakarta",
		"building_demo_001",
		"Scoped Group",
		"in scope",
		[]string{"usr_1001"},
	)
	if err != nil {
		t.Fatalf("create in-scope user group error: %v", err)
	}
	groupOutOfScope, err := accessSvc.CreateUserGroup(
		"tenant_demo_jakarta",
		"building_demo_002",
		"Out Scope Group",
		"out of scope",
		nil,
	)
	if err != nil {
		t.Fatalf("create out-of-scope user group error: %v", err)
	}
	userInScope, err := accessSvc.CreateUser(
		"tenant_demo_jakarta",
		"building_demo_001",
		"Scoped Active User",
		"scoped.active@example.com",
		"employee",
		"active",
		[]string{groupInScope.ID, groupOutOfScope.ID},
	)
	if err != nil {
		t.Fatalf("create in-scope user error: %v", err)
	}
	userSuspended, err := accessSvc.CreateUser(
		"tenant_demo_jakarta",
		"building_demo_001",
		"Scoped Suspended User",
		"scoped.suspended@example.com",
		"employee",
		"suspended",
		[]string{groupInScope.ID},
	)
	if err != nil {
		t.Fatalf("create suspended user error: %v", err)
	}
	userOutOfScope, err := accessSvc.CreateUser(
		"tenant_demo_jakarta",
		"building_demo_002",
		"Out Scope User",
		"out.scope.user@example.com",
		"employee",
		"active",
		[]string{groupOutOfScope.ID},
	)
	if err != nil {
		t.Fatalf("create out-of-scope user error: %v", err)
	}

	s := &server{
		spaceSvc:  spaceSvc,
		accessSvc: accessSvc,
	}
	generatedAt := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	cache := s.buildGatewayConfigAuthzCache(
		"tenant_demo_jakarta",
		"gw_demo_001",
		[]string{"door_unknown", "door_jkt_001", "door_jkt_001"},
		generatedAt,
	)

	if !cache.GeneratedAt.Equal(generatedAt) {
		t.Fatalf("generated_at mismatch: %s", cache.GeneratedAt)
	}
	if cache.Version == "" || cache.Version == "authz-unavailable" {
		t.Fatalf("unexpected authz cache version: %s", cache.Version)
	}
	if cache.TTLSeconds != gatewayConfigAuthzCacheTTLSeconds {
		t.Fatalf("ttl mismatch: %d", cache.TTLSeconds)
	}
	if !cache.ExpiresAt.Equal(generatedAt.Add(time.Duration(gatewayConfigAuthzCacheTTLSeconds) * time.Second)) {
		t.Fatalf("expires_at mismatch: %s", cache.ExpiresAt)
	}
	if cache.Policy.FallbackMode != "use_last_acknowledged" || cache.Policy.NoCacheBehavior != "deny_all" {
		t.Fatalf("authz policy mode mismatch: %+v", cache.Policy)
	}
	if cache.Policy.MaxStaleSeconds != gatewayConfigAuthzCacheMaxStaleSeconds {
		t.Fatalf("max_stale_seconds mismatch: %d", cache.Policy.MaxStaleSeconds)
	}
	if !cache.Policy.StaleUntil.Equal(generatedAt.Add(time.Duration(gatewayConfigAuthzCacheMaxStaleSeconds) * time.Second)) {
		t.Fatalf("stale_until mismatch: %s", cache.Policy.StaleUntil)
	}
	if cache.Policy.RefreshRetrySeconds != gatewayConfigAuthzCacheRefreshRetrySeconds {
		t.Fatalf("refresh_retry_seconds mismatch: %d", cache.Policy.RefreshRetrySeconds)
	}
	if cache.Policy.RollbackVersion != "" {
		t.Fatalf("rollback_version should be empty before ack, got: %s", cache.Policy.RollbackVersion)
	}
	if cache.StatusCodes.Fresh != "AUTHZ_CACHE_FRESH" ||
		cache.StatusCodes.Stale != "AUTHZ_CACHE_STALE" ||
		cache.StatusCodes.Missing != "AUTHZ_CACHE_MISSING" ||
		cache.StatusCodes.Drift != "AUTHZ_CACHE_DRIFT" {
		t.Fatalf("status_codes mismatch: %+v", cache.StatusCodes)
	}
	if len(cache.Scope.DoorIDs) != 2 || cache.Scope.DoorIDs[0] != "door_jkt_001" || cache.Scope.DoorIDs[1] != "door_unknown" {
		t.Fatalf("door scope mismatch: %+v", cache.Scope.DoorIDs)
	}
	if len(cache.Scope.BuildingIDs) != 1 || cache.Scope.BuildingIDs[0] != "building_demo_001" {
		t.Fatalf("building scope mismatch: %+v", cache.Scope.BuildingIDs)
	}
	if len(cache.Scope.AreaIDs) != 1 || cache.Scope.AreaIDs[0] != "area_demo_001" {
		t.Fatalf("area scope mismatch: %+v", cache.Scope.AreaIDs)
	}
	if len(cache.Doors) != 1 || cache.Doors[0].ID != "door_jkt_001" {
		t.Fatalf("door payload mismatch: %+v", cache.Doors)
	}

	policyIDs := make(map[string]struct{}, len(cache.Policies))
	for i := range cache.Policies {
		policyIDs[cache.Policies[i].ID] = struct{}{}
	}
	if _, exists := policyIDs["plc_1001"]; !exists {
		t.Fatalf("expected default in-scope policy plc_1001")
	}
	if _, exists := policyIDs[allPolicy.ID]; !exists {
		t.Fatalf("expected scope=all policy to be included")
	}
	if _, exists := policyIDs[inactivePolicy.ID]; exists {
		t.Fatalf("inactive policy should be excluded")
	}

	temporaryIDs := make(map[string]struct{}, len(cache.TemporaryAccess))
	for i := range cache.TemporaryAccess {
		temporaryIDs[cache.TemporaryAccess[i].ID] = struct{}{}
	}
	if _, exists := temporaryIDs[matchedTemporary.ID]; !exists {
		t.Fatalf("expected matched temporary access to be included")
	}
	if _, exists := temporaryIDs[unmatchedTemporary.ID]; exists {
		t.Fatalf("out-of-scope temporary access should be excluded")
	}

	visitorIDs := make(map[string]struct{}, len(cache.VisitorPasses))
	for i := range cache.VisitorPasses {
		visitorIDs[cache.VisitorPasses[i].ID] = struct{}{}
	}
	if _, exists := visitorIDs["vst_2201"]; !exists {
		t.Fatalf("expected default in-scope visitor pass vst_2201")
	}
	if _, exists := visitorIDs[unmatchedVisitor.ID]; exists {
		t.Fatalf("out-of-scope visitor pass should be excluded")
	}

	userGroupIDs := make(map[string]struct{}, len(cache.UserGroups))
	for i := range cache.UserGroups {
		userGroupIDs[cache.UserGroups[i].ID] = struct{}{}
	}
	if _, exists := userGroupIDs[groupInScope.ID]; !exists {
		t.Fatalf("in-scope user group should be included")
	}
	if _, exists := userGroupIDs[groupOutOfScope.ID]; exists {
		t.Fatalf("out-of-scope user group should be excluded")
	}

	userByID := make(map[string]access.AccessUser, len(cache.Users))
	for i := range cache.Users {
		userByID[cache.Users[i].ID] = cache.Users[i]
	}
	if _, exists := userByID["usr_1001"]; !exists {
		t.Fatalf("default active in-scope user should be included")
	}
	if _, exists := userByID[userInScope.ID]; !exists {
		t.Fatalf("created in-scope active user should be included")
	}
	if _, exists := userByID[userSuspended.ID]; exists {
		t.Fatalf("suspended user should be excluded")
	}
	if _, exists := userByID[userOutOfScope.ID]; exists {
		t.Fatalf("out-of-scope user should be excluded")
	}
	if scopedUser, exists := userByID[userInScope.ID]; exists {
		if len(scopedUser.GroupIDs) != 1 || scopedUser.GroupIDs[0] != groupInScope.ID {
			t.Fatalf("scoped user group filtering mismatch: %+v", scopedUser.GroupIDs)
		}
	}

	if cache.Counts.Doors != len(cache.Doors) ||
		cache.Counts.Policies != len(cache.Policies) ||
		cache.Counts.TemporaryAccess != len(cache.TemporaryAccess) ||
		cache.Counts.VisitorPasses != len(cache.VisitorPasses) ||
		cache.Counts.Users != len(cache.Users) ||
		cache.Counts.UserGroups != len(cache.UserGroups) {
		t.Fatalf("cache counts mismatch: %+v", cache.Counts)
	}
}

func TestBuildGatewayConfigAuthzCacheVersionStable(t *testing.T) {
	s := &server{
		spaceSvc:  space.NewService(),
		accessSvc: access.NewService(),
	}

	baseAt := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	cache1 := s.buildGatewayConfigAuthzCache("tenant_demo_jakarta", "gw_demo_001", []string{"door_jkt_001"}, baseAt)
	cache2 := s.buildGatewayConfigAuthzCache("tenant_demo_jakarta", "gw_demo_001", []string{"door_jkt_001"}, baseAt.Add(2*time.Hour))

	if cache1.Version == "" {
		t.Fatalf("expected non-empty version")
	}
	if cache1.Version != cache2.Version {
		t.Fatalf("version should be stable when payload is unchanged: %s != %s", cache1.Version, cache2.Version)
	}

	if _, err := s.accessSvc.CreatePolicy(
		"tenant_demo_jakarta",
		"Version Drift Policy",
		"door",
		"building_demo_001",
		"area_demo_001",
		"door_jkt_001",
		"24x7",
		1,
		"active",
	); err != nil {
		t.Fatalf("create policy for version drift error: %v", err)
	}
	cache3 := s.buildGatewayConfigAuthzCache("tenant_demo_jakarta", "gw_demo_001", []string{"door_jkt_001"}, baseAt.Add(3*time.Hour))
	if cache3.Version == cache1.Version {
		t.Fatalf("expected version to change after payload mutation")
	}
}

func TestBuildGatewayConfigAuthzCacheRollbackVersionHint(t *testing.T) {
	s := &server{
		spaceSvc:               space.NewService(),
		accessSvc:              access.NewService(),
		gatewayAuthzAckVersion: map[string]string{},
	}
	baseAt := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)

	cache1 := s.buildGatewayConfigAuthzCache("tenant_demo_jakarta", "gw_demo_001", []string{"door_jkt_001"}, baseAt)
	if cache1.Policy.RollbackVersion != "" {
		t.Fatalf("rollback_version should be empty before ack, got %s", cache1.Policy.RollbackVersion)
	}

	s.setGatewayAuthzCacheAckVersion("gw_demo_001", cache1.Version)
	cache2 := s.buildGatewayConfigAuthzCache("tenant_demo_jakarta", "gw_demo_001", []string{"door_jkt_001"}, baseAt.Add(time.Minute))
	if cache2.Policy.RollbackVersion != cache1.Version {
		t.Fatalf("rollback_version mismatch: expected %s got %s", cache1.Version, cache2.Policy.RollbackVersion)
	}
}

func TestEnterpriseEmployeesToAccessBatchInputsUsesStableSyncIdentity(t *testing.T) {
	inputs := []enterprise.EnterpriseEmployee{
		{
			ExternalID: "hr-5001",
			Email:      "first.employee@sudirman.co",
			FullName:   "First Employee",
			AccessRole: "resident",
			Status:     "active",
			BuildingID: "building_demo_001",
			GroupIDs:   []string{"ug_common_office_jkt"},
		},
		{
			ExternalID: "",
			Email:      "second.employee@sudirman.co",
			FullName:   "Second Employee",
			AccessRole: "operator",
			Status:     "active",
			BuildingID: "building_demo_001",
			GroupIDs:   []string{"ug_security_jkt"},
		},
	}

	accessInputs := enterpriseEmployeesToAccessBatchInputs(inputs)
	if len(accessInputs) != 2 {
		t.Fatalf("expected 2 access inputs, got %d", len(accessInputs))
	}

	first := accessInputs[0]
	if first.SyncSource != enterpriseAccessSyncSource {
		t.Fatalf("unexpected first sync source: %s", first.SyncSource)
	}
	if first.SyncRef != "external_id:hr-5001" {
		t.Fatalf("unexpected first sync ref: %s", first.SyncRef)
	}

	second := accessInputs[1]
	if second.SyncSource != enterpriseAccessSyncSource {
		t.Fatalf("unexpected second sync source: %s", second.SyncSource)
	}
	if second.SyncRef != "email:second.employee@sudirman.co" {
		t.Fatalf("unexpected second sync ref fallback: %s", second.SyncRef)
	}
}
