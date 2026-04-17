package enterprise

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSyncEmployeesWithAccessUpsertSuccess(t *testing.T) {
	svc := NewService()
	targetEmail := "sync.success.case@sudirman.co"

	result, created, updated, rejected, err := svc.SyncEmployeesWithAccessUpsert(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		"req-success",
		[]EmployeeSyncInput{
			{
				ExternalID: "hr-success-1",
				Email:      targetEmail,
				FullName:   "Sync Success",
				Department: "IT",
				JobTitle:   "Engineer",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
		func(items []EnterpriseEmployee) (int, int, int, error) {
			if len(items) != 1 {
				t.Fatalf("expected 1 synced employee item, got %d", len(items))
			}
			return 1, 0, 0, nil
		},
	)
	if err != nil {
		t.Fatalf("sync with access upsert should succeed: %v", err)
	}
	if result.Job.Status != "completed" {
		t.Fatalf("unexpected job status: %s", result.Job.Status)
	}
	if created != 1 || updated != 0 || rejected != 0 {
		t.Fatalf("unexpected access sync counters: created=%d updated=%d rejected=%d", created, updated, rejected)
	}

	employees := svc.ListEmployees("tenant_demo_jakarta")
	if !containsEmployeeEmail(employees, targetEmail) {
		t.Fatalf("expected synced employee %q to be present", targetEmail)
	}
	if len(svc.ListSyncJobs("tenant_demo_jakarta")) == 0 {
		t.Fatalf("expected sync job to be recorded")
	}
}

func TestSyncEmployeesWithAccessUpsertRollbackOnFailure(t *testing.T) {
	svc := NewService()
	targetEmail := "sync.rollback.case@sudirman.co"

	beforeEmployees := svc.ListEmployees("tenant_demo_jakarta")
	beforeJobs := svc.ListSyncJobs("tenant_demo_jakarta")

	_, _, _, _, err := svc.SyncEmployeesWithAccessUpsert(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		"req-rollback",
		[]EmployeeSyncInput{
			{
				ExternalID: "hr-rollback-1",
				Email:      targetEmail,
				FullName:   "Sync Rollback",
				Department: "IT",
				JobTitle:   "Engineer",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
		func(items []EnterpriseEmployee) (int, int, int, error) {
			return 0, 0, 0, errors.New("forced access failure")
		},
	)
	if err == nil {
		t.Fatalf("expected sync with failed access upsert to return error")
	}

	afterEmployees := svc.ListEmployees("tenant_demo_jakarta")
	afterJobs := svc.ListSyncJobs("tenant_demo_jakarta")
	if len(afterEmployees) != len(beforeEmployees) {
		t.Fatalf("enterprise employees should be rolled back, before=%d after=%d", len(beforeEmployees), len(afterEmployees))
	}
	if len(afterJobs) != len(beforeJobs) {
		t.Fatalf("sync jobs should be rolled back, before=%d after=%d", len(beforeJobs), len(afterJobs))
	}
	if containsEmployeeEmail(afterEmployees, targetEmail) {
		t.Fatalf("rolled back employee %q should not exist", targetEmail)
	}
}

func TestSyncEmployeesWithAccessUpsertIdempotentRequestID(t *testing.T) {
	svc := NewService()
	requestID := "req-idempotent-1"
	targetEmail := "sync.idempotent.case@sudirman.co"
	applierCalls := 0

	firstResult, firstCreated, firstUpdated, firstRejected, err := svc.SyncEmployeesWithAccessUpsert(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		requestID,
		[]EmployeeSyncInput{
			{
				ExternalID: "hr-idempotent-1",
				Email:      targetEmail,
				FullName:   "Sync Idempotent",
				Department: "IT",
				JobTitle:   "Engineer",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
		func(items []EnterpriseEmployee) (int, int, int, error) {
			applierCalls++
			return 1, 0, 0, nil
		},
	)
	if err != nil {
		t.Fatalf("first sync should succeed: %v", err)
	}
	if firstCreated != 1 || firstUpdated != 0 || firstRejected != 0 {
		t.Fatalf("unexpected first access counters: created=%d updated=%d rejected=%d", firstCreated, firstUpdated, firstRejected)
	}

	secondResult, secondCreated, secondUpdated, secondRejected, err := svc.SyncEmployeesWithAccessUpsert(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		requestID,
		[]EmployeeSyncInput{
			{
				ExternalID: "hr-idempotent-1",
				Email:      targetEmail,
				FullName:   "Sync Idempotent Retry",
				Department: "IT",
				JobTitle:   "Engineer",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
		func(items []EnterpriseEmployee) (int, int, int, error) {
			applierCalls++
			return 99, 99, 99, nil
		},
	)
	if err != nil {
		t.Fatalf("second sync should be served by idempotency cache: %v", err)
	}
	if applierCalls != 1 {
		t.Fatalf("access applier should be called exactly once, got %d", applierCalls)
	}
	if secondCreated != firstCreated || secondUpdated != firstUpdated || secondRejected != firstRejected {
		t.Fatalf(
			"idempotent counters mismatch: first=%d/%d/%d second=%d/%d/%d",
			firstCreated, firstUpdated, firstRejected,
			secondCreated, secondUpdated, secondRejected,
		)
	}
	if secondResult.Job.ID != firstResult.Job.ID {
		t.Fatalf("idempotent request should return same job id, first=%s second=%s", firstResult.Job.ID, secondResult.Job.ID)
	}
}

func TestReconcileSyncRequestAccessNotFound(t *testing.T) {
	svc := NewService()
	_, _, _, _, err := svc.ReconcileSyncRequestAccess(
		"tenant_demo_jakarta",
		"req-not-found",
		func(items []EnterpriseEmployee) (int, int, int, error) {
			return 0, 0, 0, nil
		},
	)
	if !errors.Is(err, ErrSyncRequestNotFound) {
		t.Fatalf("expected ErrSyncRequestNotFound, got: %v", err)
	}
}

func TestReconcileSyncRequestAccessPendingRecord(t *testing.T) {
	svc := NewService()
	tenantID := "tenant_demo_jakarta"
	requestID := "req-reconcile-pending-1"
	targetEmail := "sync.reconcile.pending@sudirman.co"
	now := time.Now().UTC()

	pendingEmployee := EnterpriseEmployee{
		ID:           "emp_reconcile_pending_1",
		TenantID:     tenantID,
		ExternalID:   "hr-reconcile-pending-1",
		Email:        targetEmail,
		FullName:     "Reconcile Pending Access",
		Department:   "IT",
		JobTitle:     "Engineer",
		Location:     "Jakarta",
		AccessRole:   "resident",
		BuildingID:   "building_demo_001",
		GroupIDs:     []string{"ug_1001"},
		Status:       "active",
		Source:       "manual_sync",
		LastSyncedAt: now,
	}
	pendingResult := SyncResult{
		Job: SyncJob{
			ID:        "syn_reconcile_pending_1",
			TenantID:  tenantID,
			Source:    "manual_sync",
			Status:    "completed",
			Total:     1,
			Created:   1,
			Updated:   0,
			Rejected:  0,
			Actor:     "qa",
			StartedAt: now,
			EndedAt:   now,
		},
		Items: []EnterpriseEmployee{pendingEmployee},
	}
	recordKey := syncRequestRecordKey(tenantID, requestID)
	svc.syncRequestRecords[recordKey] = SyncRequestRecord{
		RequestID:     requestID,
		TenantID:      tenantID,
		Result:        pendingResult,
		AccessApplied: false,
		CreatedAt:     now,
	}

	applierCalls := 0
	firstResult, firstCreated, firstUpdated, firstRejected, err := svc.ReconcileSyncRequestAccess(
		tenantID,
		requestID,
		func(items []EnterpriseEmployee) (int, int, int, error) {
			applierCalls++
			if len(items) != 1 {
				t.Fatalf("expected one employee from pending record, got %d", len(items))
			}
			if normalizeEmail(items[0].Email) != normalizeEmail(targetEmail) {
				t.Fatalf("unexpected employee email: %s", items[0].Email)
			}
			return 1, 0, 0, nil
		},
	)
	if err != nil {
		t.Fatalf("reconcile should succeed: %v", err)
	}
	if applierCalls != 1 {
		t.Fatalf("expected applier to run once, got %d", applierCalls)
	}
	if firstResult.Job.ID != pendingResult.Job.ID {
		t.Fatalf("reconcile should preserve job id, expected=%s got=%s", pendingResult.Job.ID, firstResult.Job.ID)
	}
	if firstCreated != 1 || firstUpdated != 0 || firstRejected != 0 {
		t.Fatalf("unexpected reconcile counters: created=%d updated=%d rejected=%d", firstCreated, firstUpdated, firstRejected)
	}

	secondResult, secondCreated, secondUpdated, secondRejected, err := svc.ReconcileSyncRequestAccess(
		tenantID,
		requestID,
		func(items []EnterpriseEmployee) (int, int, int, error) {
			applierCalls++
			return 99, 99, 99, nil
		},
	)
	if err != nil {
		t.Fatalf("second reconcile should return cached result: %v", err)
	}
	if applierCalls != 1 {
		t.Fatalf("second reconcile should not re-run applier, calls=%d", applierCalls)
	}
	if secondResult.Job.ID != pendingResult.Job.ID {
		t.Fatalf("second reconcile should preserve job id, expected=%s got=%s", pendingResult.Job.ID, secondResult.Job.ID)
	}
	if secondCreated != firstCreated || secondUpdated != firstUpdated || secondRejected != firstRejected {
		t.Fatalf(
			"reconcile counters mismatch: first=%d/%d/%d second=%d/%d/%d",
			firstCreated, firstUpdated, firstRejected,
			secondCreated, secondUpdated, secondRejected,
		)
	}
	record := svc.syncRequestRecords[recordKey]
	if record.AccessAttemptCount != 1 {
		t.Fatalf("expected access_attempt_count=1, got %d", record.AccessAttemptCount)
	}
	if record.LastAccessAttemptAt == nil {
		t.Fatalf("expected last_access_attempt_at to be set")
	}
	if strings.TrimSpace(record.LastAccessError) != "" {
		t.Fatalf("expected last_access_error to be empty after success, got %q", record.LastAccessError)
	}
}

func TestReconcileSyncRequestAccessFailureAuditTrail(t *testing.T) {
	svc := NewService()
	tenantID := "tenant_demo_jakarta"
	requestID := "req-reconcile-failure-1"
	now := time.Now().UTC()
	recordKey := syncRequestRecordKey(tenantID, requestID)
	svc.syncRequestRecords[recordKey] = SyncRequestRecord{
		RequestID: requestID,
		TenantID:  tenantID,
		Result: SyncResult{
			Job: SyncJob{
				ID:       "syn_reconcile_failure_1",
				TenantID: tenantID,
				Status:   "completed",
			},
			Items: []EnterpriseEmployee{
				{
					ID:       "emp_reconcile_failure_1",
					TenantID: tenantID,
					Email:    "sync.reconcile.failure@sudirman.co",
				},
			},
		},
		AccessApplied: false,
		CreatedAt:     now,
	}

	_, _, _, _, err := svc.ReconcileSyncRequestAccess(
		tenantID,
		requestID,
		func(items []EnterpriseEmployee) (int, int, int, error) {
			return 0, 0, 0, errors.New("forced reconcile failure")
		},
	)
	if err == nil {
		t.Fatalf("expected reconcile failure")
	}
	record := svc.syncRequestRecords[recordKey]
	if record.AccessApplied {
		t.Fatalf("record should remain access_applied=false after failure")
	}
	if record.AccessAttemptCount != 1 {
		t.Fatalf("expected access_attempt_count=1 after failure, got %d", record.AccessAttemptCount)
	}
	if record.LastAccessAttemptAt == nil {
		t.Fatalf("expected last_access_attempt_at to be set after failure")
	}
	if !strings.Contains(record.LastAccessError, "forced reconcile failure") {
		t.Fatalf("expected last_access_error to contain reconcile failure, got %q", record.LastAccessError)
	}

	_, _, _, _, err = svc.ReconcileSyncRequestAccess(
		tenantID,
		requestID,
		func(items []EnterpriseEmployee) (int, int, int, error) {
			return 1, 0, 0, nil
		},
	)
	if err != nil {
		t.Fatalf("expected reconcile success after failure retry: %v", err)
	}
	record = svc.syncRequestRecords[recordKey]
	if !record.AccessApplied {
		t.Fatalf("record should be access_applied=true after successful retry")
	}
	if record.AccessAttemptCount != 2 {
		t.Fatalf("expected access_attempt_count=2 after retry success, got %d", record.AccessAttemptCount)
	}
	if strings.TrimSpace(record.LastAccessError) != "" {
		t.Fatalf("expected last_access_error to be cleared after success, got %q", record.LastAccessError)
	}
}

func TestReconcilePendingSyncRequestsMixedOutcome(t *testing.T) {
	svc := NewService()
	tenantID := "tenant_demo_jakarta"
	baseTime := time.Now().UTC().Add(-time.Minute)

	recordPendingSuccess := SyncRequestRecord{
		RequestID: "req-pending-success",
		TenantID:  tenantID,
		Result: SyncResult{
			Job: SyncJob{
				ID:       "syn_pending_success",
				TenantID: tenantID,
				Status:   "completed",
			},
			Items: []EnterpriseEmployee{
				{
					ID:       "emp_pending_success",
					TenantID: tenantID,
					Email:    "pending.success@sudirman.co",
				},
			},
		},
		AccessApplied: false,
		CreatedAt:     baseTime,
	}
	recordPendingFailure := SyncRequestRecord{
		RequestID: "req-pending-failure",
		TenantID:  tenantID,
		Result: SyncResult{
			Job: SyncJob{
				ID:       "syn_pending_failure",
				TenantID: tenantID,
				Status:   "completed",
			},
			Items: []EnterpriseEmployee{
				{
					ID:       "emp_pending_failure",
					TenantID: tenantID,
					Email:    "pending.failure@sudirman.co",
				},
			},
		},
		AccessApplied: false,
		CreatedAt:     baseTime.Add(time.Second),
	}
	recordApplied := SyncRequestRecord{
		RequestID:      "req-already-applied",
		TenantID:       tenantID,
		Result:         SyncResult{Job: SyncJob{ID: "syn_already_applied", TenantID: tenantID, Status: "completed"}},
		AccessApplied:  true,
		AccessCreated:  1,
		AccessUpdated:  0,
		AccessRejected: 0,
		CreatedAt:      baseTime.Add(2 * time.Second),
	}

	svc.syncRequestRecords[syncRequestRecordKey(tenantID, recordPendingSuccess.RequestID)] = recordPendingSuccess
	svc.syncRequestRecords[syncRequestRecordKey(tenantID, recordPendingFailure.RequestID)] = recordPendingFailure
	svc.syncRequestRecords[syncRequestRecordKey(tenantID, recordApplied.RequestID)] = recordApplied

	applierCalls := 0
	result, err := svc.ReconcilePendingSyncRequests(
		tenantID,
		10,
		func(items []EnterpriseEmployee) (int, int, int, error) {
			applierCalls++
			if len(items) != 1 {
				t.Fatalf("expected one employee item per pending request, got %d", len(items))
			}
			switch normalizeEmail(items[0].Email) {
			case "pending.success@sudirman.co":
				return 1, 0, 0, nil
			case "pending.failure@sudirman.co":
				return 0, 0, 0, errors.New("forced pending failure")
			default:
				t.Fatalf("unexpected email in pending reconcile applier: %s", items[0].Email)
				return 0, 0, 0, nil
			}
		},
	)
	if err != nil {
		t.Fatalf("pending reconcile should complete with mixed outcome: %v", err)
	}
	if applierCalls != 2 {
		t.Fatalf("expected two pending applier calls, got %d", applierCalls)
	}
	if result.Processed != 2 || result.Applied != 1 || result.Failed != 1 {
		t.Fatalf("unexpected pending reconcile summary: processed=%d applied=%d failed=%d", result.Processed, result.Applied, result.Failed)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected two pending reconcile item results, got %d", len(result.Items))
	}

	successRecord := svc.syncRequestRecords[syncRequestRecordKey(tenantID, recordPendingSuccess.RequestID)]
	if !successRecord.AccessApplied {
		t.Fatalf("success pending record should be marked access_applied")
	}
	if successRecord.AccessAttemptCount != 1 {
		t.Fatalf("success pending record attempt count should be 1, got %d", successRecord.AccessAttemptCount)
	}
	if strings.TrimSpace(successRecord.LastAccessError) != "" {
		t.Fatalf("success pending record last_access_error should be empty, got %q", successRecord.LastAccessError)
	}

	failureRecord := svc.syncRequestRecords[syncRequestRecordKey(tenantID, recordPendingFailure.RequestID)]
	if failureRecord.AccessApplied {
		t.Fatalf("failure pending record should remain access_applied=false")
	}
	if failureRecord.AccessAttemptCount != 1 {
		t.Fatalf("failure pending record attempt count should be 1, got %d", failureRecord.AccessAttemptCount)
	}
	if !strings.Contains(failureRecord.LastAccessError, "forced pending failure") {
		t.Fatalf("failure pending record last_access_error mismatch: %q", failureRecord.LastAccessError)
	}

	secondResult, err := svc.ReconcilePendingSyncRequests(
		tenantID,
		1,
		func(items []EnterpriseEmployee) (int, int, int, error) {
			applierCalls++
			return 1, 0, 0, nil
		},
	)
	if err != nil {
		t.Fatalf("second pending reconcile should succeed: %v", err)
	}
	if secondResult.Processed != 1 || secondResult.Applied != 1 || secondResult.Failed != 0 {
		t.Fatalf("unexpected second pending reconcile summary: processed=%d applied=%d failed=%d", secondResult.Processed, secondResult.Applied, secondResult.Failed)
	}
	failureRecord = svc.syncRequestRecords[syncRequestRecordKey(tenantID, recordPendingFailure.RequestID)]
	if !failureRecord.AccessApplied {
		t.Fatalf("failed pending record should be applied after second retry")
	}
	if failureRecord.AccessAttemptCount != 2 {
		t.Fatalf("failed pending record attempt count should be 2 after retry, got %d", failureRecord.AccessAttemptCount)
	}
}

func TestReconcilePendingSyncRequestsWithPolicySkipsAttemptLimitAndCooldown(t *testing.T) {
	svc := NewService()
	tenantID := "tenant_demo_jakarta"
	baseTime := time.Now().UTC().Add(-10 * time.Minute)

	exhaustedAttemptAt := time.Now().UTC().Add(-2 * time.Second)
	cooldownAttemptAt := time.Now().UTC().Add(-10 * time.Second)
	eligibleAttemptAt := time.Now().UTC().Add(-2 * time.Minute)

	recordExhausted := SyncRequestRecord{
		RequestID: "req-exhausted",
		TenantID:  tenantID,
		Result: SyncResult{
			Job: SyncJob{
				ID:       "syn_exhausted",
				TenantID: tenantID,
				Status:   "completed",
			},
			Items: []EnterpriseEmployee{
				{ID: "emp_exhausted", TenantID: tenantID, Email: "exhausted@sudirman.co"},
			},
		},
		AccessApplied:       false,
		AccessAttemptCount:  3,
		LastAccessAttemptAt: &exhaustedAttemptAt,
		CreatedAt:           baseTime,
	}
	recordCooldown := SyncRequestRecord{
		RequestID: "req-cooldown",
		TenantID:  tenantID,
		Result: SyncResult{
			Job: SyncJob{
				ID:       "syn_cooldown",
				TenantID: tenantID,
				Status:   "completed",
			},
			Items: []EnterpriseEmployee{
				{ID: "emp_cooldown", TenantID: tenantID, Email: "cooldown@sudirman.co"},
			},
		},
		AccessApplied:       false,
		AccessAttemptCount:  1,
		LastAccessAttemptAt: &cooldownAttemptAt,
		CreatedAt:           baseTime.Add(time.Second),
	}
	recordEligible := SyncRequestRecord{
		RequestID: "req-eligible",
		TenantID:  tenantID,
		Result: SyncResult{
			Job: SyncJob{
				ID:       "syn_eligible",
				TenantID: tenantID,
				Status:   "completed",
			},
			Items: []EnterpriseEmployee{
				{ID: "emp_eligible", TenantID: tenantID, Email: "eligible@sudirman.co"},
			},
		},
		AccessApplied:       false,
		AccessAttemptCount:  2,
		LastAccessAttemptAt: &eligibleAttemptAt,
		CreatedAt:           baseTime.Add(2 * time.Second),
	}

	svc.syncRequestRecords[syncRequestRecordKey(tenantID, recordExhausted.RequestID)] = recordExhausted
	svc.syncRequestRecords[syncRequestRecordKey(tenantID, recordCooldown.RequestID)] = recordCooldown
	svc.syncRequestRecords[syncRequestRecordKey(tenantID, recordEligible.RequestID)] = recordEligible

	applierCalls := 0
	result, err := svc.ReconcilePendingSyncRequestsWithPolicy(
		tenantID,
		10,
		3,
		time.Minute,
		func(items []EnterpriseEmployee) (int, int, int, error) {
			applierCalls++
			if len(items) != 1 {
				t.Fatalf("expected one eligible item, got %d", len(items))
			}
			if normalizeEmail(items[0].Email) != "eligible@sudirman.co" {
				t.Fatalf("unexpected eligible email: %s", items[0].Email)
			}
			return 1, 0, 0, nil
		},
	)
	if err != nil {
		t.Fatalf("policy pending reconcile should succeed: %v", err)
	}
	if applierCalls != 1 {
		t.Fatalf("expected one applier call, got %d", applierCalls)
	}
	if result.Processed != 1 || result.Applied != 1 || result.Failed != 0 {
		t.Fatalf("unexpected policy summary: processed=%d applied=%d failed=%d", result.Processed, result.Applied, result.Failed)
	}
	if result.SkippedByAttemptLimit != 1 {
		t.Fatalf("expected skipped_by_attempt_limit=1, got %d", result.SkippedByAttemptLimit)
	}
	if result.SkippedByCooldown != 1 {
		t.Fatalf("expected skipped_by_cooldown=1, got %d", result.SkippedByCooldown)
	}
	if len(result.Items) != 1 || result.Items[0].RequestID != "req-eligible" {
		t.Fatalf("unexpected policy reconcile items: %+v", result.Items)
	}

	exhaustedRecord := svc.syncRequestRecords[syncRequestRecordKey(tenantID, recordExhausted.RequestID)]
	if exhaustedRecord.AccessApplied {
		t.Fatalf("exhausted record should remain pending")
	}
	if exhaustedRecord.AccessAttemptCount != 3 {
		t.Fatalf("exhausted record attempt_count should remain 3, got %d", exhaustedRecord.AccessAttemptCount)
	}

	cooldownRecord := svc.syncRequestRecords[syncRequestRecordKey(tenantID, recordCooldown.RequestID)]
	if cooldownRecord.AccessApplied {
		t.Fatalf("cooldown record should remain pending")
	}
	if cooldownRecord.AccessAttemptCount != 1 {
		t.Fatalf("cooldown record attempt_count should remain 1, got %d", cooldownRecord.AccessAttemptCount)
	}

	eligibleRecord := svc.syncRequestRecords[syncRequestRecordKey(tenantID, recordEligible.RequestID)]
	if !eligibleRecord.AccessApplied {
		t.Fatalf("eligible record should be applied")
	}
	if eligibleRecord.AccessAttemptCount != 3 {
		t.Fatalf("eligible record attempt_count should be incremented to 3, got %d", eligibleRecord.AccessAttemptCount)
	}
}

func TestReconcilePendingSyncRequestsInvalidLimit(t *testing.T) {
	svc := NewService()
	_, err := svc.ReconcilePendingSyncRequests(
		"tenant_demo_jakarta",
		-1,
		func(items []EnterpriseEmployee) (int, int, int, error) {
			return 0, 0, 0, nil
		},
	)
	if !errors.Is(err, ErrInvalidReconcileLimit) {
		t.Fatalf("expected ErrInvalidReconcileLimit, got: %v", err)
	}
}

func TestSyncEmployeesWithAccessUpsertRetryPendingRequestRecord(t *testing.T) {
	svc := NewService()
	tenantID := "tenant_demo_jakarta"
	requestID := "req-pending-1"
	targetEmail := "sync.pending.case@sudirman.co"
	now := time.Now().UTC()

	pendingEmployee := EnterpriseEmployee{
		ID:           "emp_pending_1",
		TenantID:     tenantID,
		ExternalID:   "hr-pending-1",
		Email:        targetEmail,
		FullName:     "Pending Access Apply",
		Department:   "IT",
		JobTitle:     "Engineer",
		Location:     "Jakarta",
		AccessRole:   "resident",
		BuildingID:   "building_demo_001",
		GroupIDs:     []string{"ug_1001"},
		Status:       "active",
		Source:       "manual_sync",
		LastSyncedAt: now,
	}
	pendingResult := SyncResult{
		Job: SyncJob{
			ID:        "syn_pending_1",
			TenantID:  tenantID,
			Source:    "manual_sync",
			Status:    "completed",
			Total:     1,
			Created:   1,
			Updated:   0,
			Rejected:  0,
			Actor:     "qa",
			StartedAt: now,
			EndedAt:   now,
		},
		Items: []EnterpriseEmployee{pendingEmployee},
	}
	recordKey := syncRequestRecordKey(tenantID, requestID)
	svc.syncRequestRecords[recordKey] = SyncRequestRecord{
		RequestID:     requestID,
		TenantID:      tenantID,
		Result:        pendingResult,
		AccessApplied: false,
		CreatedAt:     now,
	}

	inputs := []EmployeeSyncInput{
		{
			ExternalID: "hr-pending-1",
			Email:      targetEmail,
			FullName:   "Pending Access Apply",
			Department: "IT",
			JobTitle:   "Engineer",
			Location:   "Jakarta",
			Status:     "active",
		},
	}

	applierCalls := 0
	firstResult, firstCreated, firstUpdated, firstRejected, err := svc.SyncEmployeesWithAccessUpsert(
		tenantID,
		"manual_sync",
		"qa",
		requestID,
		inputs,
		func(items []EnterpriseEmployee) (int, int, int, error) {
			applierCalls++
			if len(items) != 1 {
				t.Fatalf("expected one pending employee item, got %d", len(items))
			}
			if normalizeEmail(items[0].Email) != normalizeEmail(targetEmail) {
				t.Fatalf("unexpected pending employee email: %s", items[0].Email)
			}
			return 1, 0, 0, nil
		},
	)
	if err != nil {
		t.Fatalf("retry on pending request record should succeed: %v", err)
	}
	if applierCalls != 1 {
		t.Fatalf("access applier should be called once for pending record retry, got %d", applierCalls)
	}
	if firstResult.Job.ID != pendingResult.Job.ID {
		t.Fatalf("pending record retry should keep original job id, expected=%s got=%s", pendingResult.Job.ID, firstResult.Job.ID)
	}
	if firstCreated != 1 || firstUpdated != 0 || firstRejected != 0 {
		t.Fatalf("unexpected counters on pending record retry: created=%d updated=%d rejected=%d", firstCreated, firstUpdated, firstRejected)
	}

	secondResult, secondCreated, secondUpdated, secondRejected, err := svc.SyncEmployeesWithAccessUpsert(
		tenantID,
		"manual_sync",
		"qa",
		requestID,
		inputs,
		func(items []EnterpriseEmployee) (int, int, int, error) {
			applierCalls++
			return 99, 99, 99, nil
		},
	)
	if err != nil {
		t.Fatalf("second retry should be served from idempotency cache: %v", err)
	}
	if applierCalls != 1 {
		t.Fatalf("second retry should not re-run access applier, calls=%d", applierCalls)
	}
	if secondResult.Job.ID != pendingResult.Job.ID {
		t.Fatalf("second retry should keep original job id, expected=%s got=%s", pendingResult.Job.ID, secondResult.Job.ID)
	}
	if secondCreated != firstCreated || secondUpdated != firstUpdated || secondRejected != firstRejected {
		t.Fatalf(
			"pending retry idempotent counters mismatch: first=%d/%d/%d second=%d/%d/%d",
			firstCreated, firstUpdated, firstRejected,
			secondCreated, secondUpdated, secondRejected,
		)
	}
	if !svc.syncRequestRecords[recordKey].AccessApplied {
		t.Fatalf("pending request record should be marked as access_applied")
	}
}

func TestSyncEmployeesUsesExternalIDAsPrimaryIdentity(t *testing.T) {
	svc := NewService()

	result, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		[]EmployeeSyncInput{
			{
				ExternalID: "hris-jkt-1001",
				Email:      "arief.updated@sudirman.co",
				FullName:   "Arief Putra",
				Department: "Finance",
				JobTitle:   "Finance Manager",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("sync should succeed: %v", err)
	}
	if result.Job.Created != 0 || result.Job.Updated != 1 || result.Job.Rejected != 0 {
		t.Fatalf(
			"unexpected sync counters: created=%d updated=%d rejected=%d",
			result.Job.Created,
			result.Job.Updated,
			result.Job.Rejected,
		)
	}

	employees := svc.ListEmployees("tenant_demo_jakarta")
	matched, found := employeeByExternalID(employees, "hris-jkt-1001")
	if !found {
		t.Fatalf("expected external_id hris-jkt-1001 to exist")
	}
	if matched.Email != "arief.updated@sudirman.co" {
		t.Fatalf("expected email to be updated by external_id match, got %s", matched.Email)
	}
	if containsEmployeeEmail(employees, "arief.putra@sudirman.co") {
		t.Fatalf("old email should no longer exist after external_id-based update")
	}
}

func TestSyncEmployeesRejectsExternalIDEmailConflict(t *testing.T) {
	svc := NewService()

	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		[]EmployeeSyncInput{
			{
				ExternalID: "hris-jkt-2002",
				Email:      "conflict.target@sudirman.co",
				FullName:   "Conflict Target",
				Department: "IT",
				JobTitle:   "Engineer",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("seed sync should succeed: %v", err)
	}

	result, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		[]EmployeeSyncInput{
			{
				ExternalID: "hris-jkt-1001",
				Email:      "conflict.target@sudirman.co",
				FullName:   "Conflict Update",
				Department: "IT",
				JobTitle:   "Engineer",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("conflict sync should still return result with rejected item: %v", err)
	}
	if result.Job.Created != 0 || result.Job.Updated != 0 || result.Job.Rejected != 1 {
		t.Fatalf(
			"unexpected conflict counters: created=%d updated=%d rejected=%d",
			result.Job.Created,
			result.Job.Updated,
			result.Job.Rejected,
		)
	}

	employees := svc.ListEmployees("tenant_demo_jakarta")
	primary, found := employeeByExternalID(employees, "hris-jkt-1001")
	if !found {
		t.Fatalf("expected primary employee external_id hris-jkt-1001")
	}
	if primary.Email != "arief.putra@sudirman.co" {
		t.Fatalf("primary employee email should remain unchanged, got %s", primary.Email)
	}

	second, found := employeeByExternalID(employees, "hris-jkt-2002")
	if !found {
		t.Fatalf("expected secondary employee external_id hris-jkt-2002")
	}
	if second.Email != "conflict.target@sudirman.co" {
		t.Fatalf("secondary employee email mismatch, got %s", second.Email)
	}
}

func containsEmployeeEmail(items []EnterpriseEmployee, email string) bool {
	for i := range items {
		if normalizeEmail(items[i].Email) == normalizeEmail(email) {
			return true
		}
	}
	return false
}

func employeeByExternalID(items []EnterpriseEmployee, externalID string) (EnterpriseEmployee, bool) {
	nextExternalID := strings.TrimSpace(externalID)
	for i := range items {
		if strings.TrimSpace(items[i].ExternalID) == nextExternalID {
			return items[i], true
		}
	}
	return EnterpriseEmployee{}, false
}
