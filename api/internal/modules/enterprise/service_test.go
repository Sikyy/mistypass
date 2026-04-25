package enterprise

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type memoryStateStore struct {
	items              map[string][]byte
	compareAndSwapHook func(key string, expectedExists bool, expectedPayload []byte, nextPayload []byte)
}

func (s *memoryStateStore) Load(key string, dst any) (bool, error) {
	payload, ok := s.items[key]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(payload, dst); err != nil {
		return false, err
	}
	return true, nil
}

func (s *memoryStateStore) Save(key string, value any) error {
	if s.items == nil {
		s.items = make(map[string][]byte)
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.items[key] = payload
	return nil
}

func (s *memoryStateStore) CompareAndSwap(key string, expectedExists bool, expected any, next any) (bool, error) {
	if s.items == nil {
		s.items = make(map[string][]byte)
	}

	var expectedPayload []byte
	var err error
	if expectedExists {
		expectedPayload, err = json.Marshal(expected)
		if err != nil {
			return false, err
		}
	}

	nextPayload, err := json.Marshal(next)
	if err != nil {
		return false, err
	}

	if s.compareAndSwapHook != nil {
		hook := s.compareAndSwapHook
		s.compareAndSwapHook = nil
		hook(key, expectedExists, expectedPayload, nextPayload)
	}

	currentPayload, found := s.items[key]
	if found != expectedExists {
		return false, nil
	}
	if expectedExists {
		sameExpected, err := testJSONPayloadEqual(currentPayload, expectedPayload)
		if err != nil {
			return false, err
		}
		if !sameExpected {
			return false, nil
		}
	}
	if found {
		sameNext, err := testJSONPayloadEqual(currentPayload, nextPayload)
		if err != nil {
			return false, err
		}
		if sameNext {
			return true, nil
		}
	}
	s.items[key] = nextPayload
	return true, nil
}

func testJSONPayloadEqual(left, right []byte) (bool, error) {
	var leftValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		return false, err
	}
	var rightValue any
	if err := json.Unmarshal(right, &rightValue); err != nil {
		return false, err
	}
	return reflect.DeepEqual(leftValue, rightValue), nil
}

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

func TestCreateAndListHRISConnectors(t *testing.T) {
	svc := NewService()

	created, err := svc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"hybrid",
		"vault://tenant_demo_jakarta/hris/talenta/client_id",
		"vault://tenant_demo_jakarta/hris/talenta/webhook_secret",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected non-empty connector id")
	}
	if created.Vendor != "talenta" {
		t.Fatalf("expected vendor talenta, got %s", created.Vendor)
	}

	items := svc.ListHRISConnectors("tenant_demo_jakarta")
	if len(items) != 1 {
		t.Fatalf("expected one connector, got %d", len(items))
	}
	if items[0].ID != created.ID {
		t.Fatalf("expected connector %s in list, got %s", created.ID, items[0].ID)
	}
}

func TestGetHRISConnectorByID(t *testing.T) {
	svc := NewService()

	created, err := svc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"hybrid",
		"vault://tenant_demo_jakarta/hris/talenta/client_id",
		"vault://tenant_demo_jakarta/hris/talenta/webhook_secret",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	resolved, err := svc.GetHRISConnectorByID(created.ID)
	if err != nil {
		t.Fatalf("expected connector lookup by id to succeed: %v", err)
	}
	if resolved.ID != created.ID {
		t.Fatalf("expected connector %s, got %s", created.ID, resolved.ID)
	}
	if resolved.TenantID != "tenant_demo_jakarta" {
		t.Fatalf("unexpected tenant_id: %s", resolved.TenantID)
	}
}

func TestCreateHRISConnectorRejectsDuplicateVendorPerTenant(t *testing.T) {
	svc := NewService()

	_, err := svc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"hybrid",
		"vault://tenant_demo_jakarta/hris/talenta/client_id",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("initial connector create should succeed: %v", err)
	}

	_, err = svc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"hybrid",
		"vault://tenant_demo_jakarta/hris/talenta/client_id_v2",
		"",
		"qa",
	)
	if !errors.Is(err, ErrHRISConnectorAlreadyExists) {
		t.Fatalf("expected ErrHRISConnectorAlreadyExists, got %v", err)
	}
}

func TestUpdateHRISConnector(t *testing.T) {
	svc := NewService()

	created, err := svc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"gadjian",
		"active",
		"hybrid",
		"vault://tenant_demo_jakarta/hris/gadjian/api_key",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	updated, err := svc.UpdateHRISConnector(
		"tenant_demo_jakarta",
		created.ID,
		"inactive",
		"pull",
		"vault://tenant_demo_jakarta/hris/gadjian/api_key_v2",
		"vault://tenant_demo_jakarta/hris/gadjian/webhook_secret",
		"ops",
	)
	if err != nil {
		t.Fatalf("update connector should succeed: %v", err)
	}
	if updated.Status != "inactive" {
		t.Fatalf("expected status inactive, got %s", updated.Status)
	}
	if updated.SyncStrategy != "pull" {
		t.Fatalf("expected sync_strategy pull, got %s", updated.SyncStrategy)
	}
	if updated.CredentialRef != "vault://tenant_demo_jakarta/hris/gadjian/api_key_v2" {
		t.Fatalf("credential_ref mismatch: %s", updated.CredentialRef)
	}
	if updated.WebhookSecretRef != "vault://tenant_demo_jakarta/hris/gadjian/webhook_secret" {
		t.Fatalf("webhook_secret_ref mismatch: %s", updated.WebhookSecretRef)
	}
	if updated.UpdatedBy != "ops" {
		t.Fatalf("expected updated_by ops, got %s", updated.UpdatedBy)
	}
}

func TestReceiveHRISWebhookReceipt(t *testing.T) {
	svc := NewService()

	connector, err := svc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"vault://tenant_demo_jakarta/hris/talenta/webhook_secret",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	receipt, err := svc.ReceiveHRISWebhookReceipt(
		connector.ID,
		HRISWebhookReceiptInput{
			EventType:   "talenta.employee.detail.created",
			RequestID:   "mekari-evt-001",
			ContentType: "application/json",
			Headers: map[string]string{
				"x-event-type": "talenta.employee.detail.created",
				"x-request-id": "mekari-evt-001",
			},
			RawPayload: `{"event_type":"talenta.employee.detail.created","employee":{"id":"EMP-001"}}`,
			SourceIP:   "203.0.113.10",
		},
	)
	if err != nil {
		t.Fatalf("receive webhook receipt should succeed: %v", err)
	}
	if receipt.ID == "" {
		t.Fatalf("expected non-empty receipt id")
	}
	if receipt.TenantID != "tenant_demo_jakarta" {
		t.Fatalf("tenant_id mismatch: %s", receipt.TenantID)
	}
	if receipt.Vendor != "talenta" {
		t.Fatalf("vendor mismatch: %s", receipt.Vendor)
	}
	if receipt.Status != "received" {
		t.Fatalf("expected received status, got %s", receipt.Status)
	}
	if receipt.RequestID != "mekari-evt-001" {
		t.Fatalf("request_id mismatch: %s", receipt.RequestID)
	}

	items := svc.ListHRISWebhookReceipts("tenant_demo_jakarta", connector.ID, 10)
	if len(items) != 1 {
		t.Fatalf("expected one receipt, got %d", len(items))
	}
	if items[0].RawPayload == "" {
		t.Fatalf("expected raw_payload to be stored")
	}
	if items[0].Headers["x-event-type"] != "talenta.employee.detail.created" {
		t.Fatalf("expected x-event-type header to be stored, got %q", items[0].Headers["x-event-type"])
	}
}

func TestReceiveHRISWebhookReceiptRejectsInactiveConnector(t *testing.T) {
	svc := NewService()

	connector, err := svc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"gadjian",
		"inactive",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	_, err = svc.ReceiveHRISWebhookReceipt(connector.ID, HRISWebhookReceiptInput{
		RawPayload: `{"ok":true}`,
	})
	if !errors.Is(err, ErrHRISConnectorInactive) {
		t.Fatalf("expected ErrHRISConnectorInactive, got %v", err)
	}
}

func TestReceiveHRISWebhookReceiptPersistsToStateStore(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	connector, err := svc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"linovhr",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	_, err = svc.ReceiveHRISWebhookReceipt(connector.ID, HRISWebhookReceiptInput{
		EventType:  "employee.updated",
		RequestID:  "linovhr-req-001",
		RawPayload: `{"event_type":"employee.updated"}`,
	})
	if err != nil {
		t.Fatalf("receive webhook receipt should succeed: %v", err)
	}

	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restore service with state store to initialize: %v", err)
	}

	items := restored.ListHRISWebhookReceipts("tenant_demo_jakarta", connector.ID, 10)
	if len(items) != 1 {
		t.Fatalf("expected one restored receipt, got %d", len(items))
	}
	if items[0].RequestID != "linovhr-req-001" {
		t.Fatalf("restored request_id mismatch: %s", items[0].RequestID)
	}
}

func TestHRISWebhookReceiptStatusTransitionPersistsToStateStore(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	connector, err := svc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"gadjian",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	receipt, err := svc.ReceiveHRISWebhookReceipt(connector.ID, HRISWebhookReceiptInput{
		EventType:  "employee.updated",
		RequestID:  "gadjian-req-001",
		RawPayload: `{"event_type":"employee.updated"}`,
	})
	if err != nil {
		t.Fatalf("receive webhook receipt should succeed: %v", err)
	}

	started, err := svc.MarkHRISWebhookReceiptStarted("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("mark started should succeed: %v", err)
	}
	if started.Status != "processing" {
		t.Fatalf("expected processing status, got %s", started.Status)
	}
	if started.AttemptCount != 1 {
		t.Fatalf("expected attempt_count=1 after start, got %d", started.AttemptCount)
	}
	if started.LastAttemptAt == nil {
		t.Fatalf("expected processing receipt last_attempt_at to be set")
	}
	if started.ProcessedAt != nil {
		t.Fatalf("expected processing receipt processed_at to be nil")
	}

	failed, err := svc.MarkHRISWebhookReceiptFailed("tenant_demo_jakarta", receipt.ID, errors.New("forced worker failure"))
	if err != nil {
		t.Fatalf("mark failed should succeed: %v", err)
	}
	if failed.Status != "failed" {
		t.Fatalf("expected failed status, got %s", failed.Status)
	}
	if failed.AttemptCount != 1 {
		t.Fatalf("expected failed receipt attempt_count to remain 1, got %d", failed.AttemptCount)
	}
	if failed.LastAttemptAt == nil {
		t.Fatalf("expected failed receipt last_attempt_at to be preserved")
	}
	if failed.ProcessedAt == nil {
		t.Fatalf("expected failed receipt processed_at to be set")
	}
	if failed.LastError != "forced worker failure" {
		t.Fatalf("unexpected failed receipt last_error: %s", failed.LastError)
	}

	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored service with state store to initialize: %v", err)
	}
	record, err := restored.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("expected restored receipt lookup success: %v", err)
	}
	if record.Status != "failed" {
		t.Fatalf("expected restored failed status, got %s", record.Status)
	}
	if record.AttemptCount != 1 {
		t.Fatalf("expected restored attempt_count=1, got %d", record.AttemptCount)
	}
	if record.LastAttemptAt == nil {
		t.Fatalf("expected restored last_attempt_at to be set")
	}
	if record.ProcessedAt == nil {
		t.Fatalf("expected restored processed_at to be set")
	}
	if record.LastError != "forced worker failure" {
		t.Fatalf("unexpected restored last_error: %s", record.LastError)
	}

	if len(restored.ListPendingHRISWebhookReceipts("tenant_demo_jakarta", 10)) != 0 {
		t.Fatalf("expected failed receipt to be excluded from pending list")
	}
}

func TestHRISWebhookReceiptDLQStatusTransitionPersistsToStateStore(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	connector, err := svc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"gadjian",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	receipt, err := svc.ReceiveHRISWebhookReceipt(connector.ID, HRISWebhookReceiptInput{
		EventType:  "employee.updated",
		RequestID:  "gadjian-req-dlq-001",
		RawPayload: `{"event_type":"employee.updated"}`,
	})
	if err != nil {
		t.Fatalf("receive webhook receipt should succeed: %v", err)
	}

	started, err := svc.MarkHRISWebhookReceiptStarted("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("mark started should succeed: %v", err)
	}
	if started.LastAttemptAt == nil {
		t.Fatalf("expected started receipt last_attempt_at to be set")
	}

	dlqRecord, err := svc.MarkHRISWebhookReceiptDLQ("tenant_demo_jakarta", receipt.ID, errors.New("forced dlq handoff"))
	if err != nil {
		t.Fatalf("mark dlq should succeed: %v", err)
	}
	if dlqRecord.Status != "dlq" {
		t.Fatalf("expected dlq status, got %s", dlqRecord.Status)
	}
	if dlqRecord.AttemptCount != 1 {
		t.Fatalf("expected dlq receipt attempt_count to remain 1, got %d", dlqRecord.AttemptCount)
	}
	if dlqRecord.LastAttemptAt == nil {
		t.Fatalf("expected dlq receipt last_attempt_at to be preserved")
	}
	if dlqRecord.ProcessedAt == nil {
		t.Fatalf("expected dlq receipt processed_at to be set")
	}
	if dlqRecord.LastError != "forced dlq handoff" {
		t.Fatalf("unexpected dlq receipt last_error: %s", dlqRecord.LastError)
	}

	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored service with state store to initialize: %v", err)
	}
	record, err := restored.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("expected restored receipt lookup success: %v", err)
	}
	if record.Status != "dlq" {
		t.Fatalf("expected restored dlq status, got %s", record.Status)
	}
	if len(restored.ListRetryableHRISWebhookReceipts("tenant_demo_jakarta", 10)) != 0 {
		t.Fatalf("expected dlq receipt to be excluded from retryable list")
	}
}

func TestHRISWebhookExecutionPersistsToStateStore(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	record, err := svc.CreateHRISWebhookExecution(HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_001",
		ReceiptID:     "whr_exec_001",
		ConnectorID:   "hrc_exec_001",
		Vendor:        "talenta",
		RequestID:     "talenta-exec-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTaskChannel,
		TargetStatus:  "processing",
		RequestedBy:   "qa@example.com",
	})
	if err != nil {
		t.Fatalf("create execution should succeed: %v", err)
	}
	if record.Status != HRISWebhookExecutionStatusQueued {
		t.Fatalf("expected queued execution status, got %s", record.Status)
	}

	running, err := svc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", record.ID)
	if err != nil {
		t.Fatalf("mark running should succeed: %v", err)
	}
	if running.Status != HRISWebhookExecutionStatusRunning {
		t.Fatalf("expected running execution status, got %s", running.Status)
	}
	if running.StartedAt == nil {
		t.Fatalf("expected running execution started_at to be set")
	}
	if running.AttemptCount != 1 {
		t.Fatalf("expected running execution attempt_count=1, got %+v", running)
	}

	failed, err := svc.MarkHRISWebhookExecutionFailed(
		"tenant_demo_jakarta",
		record.ID,
		"dlq",
		errors.New("forced execution failure"),
	)
	if err != nil {
		t.Fatalf("mark failed should succeed: %v", err)
	}
	if failed.Status != HRISWebhookExecutionStatusFailed {
		t.Fatalf("expected failed execution status, got %s", failed.Status)
	}
	if failed.TargetStatus != "dlq" {
		t.Fatalf("expected target_status=dlq, got %s", failed.TargetStatus)
	}
	if failed.LastError != "forced execution failure" {
		t.Fatalf("unexpected execution last_error: %s", failed.LastError)
	}
	if failed.FinishedAt == nil {
		t.Fatalf("expected failed execution finished_at to be set")
	}

	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored service with state store to initialize: %v", err)
	}
	items := restored.ListHRISWebhookExecutions("tenant_demo_jakarta", 10)
	if len(items) != 1 {
		t.Fatalf("expected one restored execution, got %d", len(items))
	}
	if items[0].ID != record.ID {
		t.Fatalf("expected restored execution id %s, got %s", record.ID, items[0].ID)
	}
	if items[0].Status != HRISWebhookExecutionStatusFailed {
		t.Fatalf("expected restored failed execution status, got %s", items[0].Status)
	}
	if items[0].DispatchMode != HRISWebhookExecutionDispatchModeWorkerTaskChannel {
		t.Fatalf("unexpected restored dispatch_mode: %s", items[0].DispatchMode)
	}
	if items[0].StartedAt == nil || items[0].FinishedAt == nil {
		t.Fatalf("expected restored execution timestamps to be preserved")
	}
}

func TestListQueuedHRISWebhookExecutionsUsesPersistentIndexOrdering(t *testing.T) {
	svc := NewService()
	svc.hrisWebhookExecutions = []HRISWebhookExecution{
		{
			ID:            "hwe_receipt_newest",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_newest",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusQueued,
			QueuedAt:      time.Date(2026, 4, 24, 4, 3, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 4, 24, 4, 3, 0, 0, time.UTC),
		},
		{
			ID:            "hwe_dlq_ready",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindDLQReplay,
			TargetID:      "hdlq_ready",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusQueued,
			QueuedAt:      time.Date(2026, 4, 24, 4, 2, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 4, 24, 4, 2, 0, 0, time.UTC),
		},
		{
			ID:            "hwe_receipt_oldest",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_oldest",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusQueued,
			QueuedAt:      time.Date(2026, 4, 24, 4, 1, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 4, 24, 4, 1, 0, 0, time.UTC),
		},
		{
			ID:            "hwe_receipt_running",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_running",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusRunning,
			QueuedAt:      time.Date(2026, 4, 24, 4, 0, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 4, 24, 4, 0, 0, 0, time.UTC),
		},
		{
			ID:            "hwe_receipt_goroutine",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_goroutine",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeGoroutineFallback,
			Status:        HRISWebhookExecutionStatusQueued,
			QueuedAt:      time.Date(2026, 4, 24, 3, 59, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 4, 24, 3, 59, 0, 0, time.UTC),
		},
	}
	svc.syncQueuedHRISWebhookExecutionIndicesLocked()

	receiptItems := svc.ListQueuedHRISWebhookExecutions(HRISWebhookExecutionKindReceiptProcess, 10)
	if len(receiptItems) != 2 {
		t.Fatalf("expected 2 queued receipt executions, got %+v", receiptItems)
	}
	if receiptItems[0].ID != "hwe_receipt_oldest" || receiptItems[1].ID != "hwe_receipt_newest" {
		t.Fatalf("unexpected queued receipt execution order: %+v", receiptItems)
	}

	dlqItems := svc.ListQueuedHRISWebhookExecutions(HRISWebhookExecutionKindDLQReplay, 10)
	if len(dlqItems) != 1 || dlqItems[0].ID != "hwe_dlq_ready" {
		t.Fatalf("unexpected queued dlq executions: %+v", dlqItems)
	}
}

func TestQueuedHRISWebhookExecutionIndexPersistsAndUpdates(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	receiptExecution, err := svc.CreateHRISWebhookExecution(HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_idx_receipt_001",
		ReceiptID:     "whr_idx_receipt_001",
		ConnectorID:   "connector-talenta-index",
		Vendor:        "talenta",
		ExecutionMode: "queued",
		DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
	})
	if err != nil {
		t.Fatalf("expected queued receipt execution create to succeed: %v", err)
	}
	dlqExecution, err := svc.CreateHRISWebhookExecution(HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          HRISWebhookExecutionKindDLQReplay,
		TargetID:      "hdlq_idx_001",
		ConnectorID:   "connector-talenta-index",
		Vendor:        "talenta",
		ExecutionMode: "queued",
		DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
	})
	if err != nil {
		t.Fatalf("expected queued dlq execution create to succeed: %v", err)
	}

	var snapshot hrisWebhookStateSnapshot
	if err := json.Unmarshal(store.items[hrisWebhookStateKey], &snapshot); err != nil {
		t.Fatalf("expected persisted webhook snapshot to decode: %v", err)
	}
	if len(snapshot.QueuedReceiptExecutionIDs) != 1 || snapshot.QueuedReceiptExecutionIDs[0] != receiptExecution.ID {
		t.Fatalf("unexpected persisted queued receipt execution ids: %+v", snapshot.QueuedReceiptExecutionIDs)
	}
	if len(snapshot.QueuedDLQReplayExecutionIDs) != 1 || snapshot.QueuedDLQReplayExecutionIDs[0] != dlqExecution.ID {
		t.Fatalf("unexpected persisted queued dlq execution ids: %+v", snapshot.QueuedDLQReplayExecutionIDs)
	}

	if _, err := svc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", receiptExecution.ID); err != nil {
		t.Fatalf("expected mark running to succeed: %v", err)
	}
	if _, err := svc.MarkHRISWebhookExecutionFailed("tenant_demo_jakarta", dlqExecution.ID, "resolved", errors.New("forced finish")); err != nil {
		t.Fatalf("expected mark failed to succeed: %v", err)
	}

	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored service with state store to initialize: %v", err)
	}
	if items := restored.ListQueuedHRISWebhookExecutions(HRISWebhookExecutionKindReceiptProcess, 10); len(items) != 0 {
		t.Fatalf("expected queued receipt execution index to exclude running items, got %+v", items)
	}
	if items := restored.ListQueuedHRISWebhookExecutions(HRISWebhookExecutionKindDLQReplay, 10); len(items) != 0 {
		t.Fatalf("expected queued dlq execution index to exclude finished items, got %+v", items)
	}
}

func TestCreateHRISWebhookExecutionRejectsActiveReplayDuplicate(t *testing.T) {
	svc := NewService()
	requireWorker := true

	first, err := svc.CreateHRISWebhookExecution(HRISWebhookExecutionInput{
		TenantID:                "tenant_demo_jakarta",
		Kind:                    HRISWebhookExecutionKindReceiptProcess,
		TargetID:                "whr_replay_conflict_001",
		ReceiptID:               "whr_replay_conflict_001",
		ConnectorID:             "connector-talenta-replay-conflict",
		Vendor:                  "talenta",
		RequestID:               "req-replay-conflict-001",
		EventType:               "talenta.employee.detail.updated",
		ExecutionMode:           "queued",
		DispatchMode:            HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:            "processing",
		ReplaySourceExecutionID: "hwe_source_failed_001",
		ReplayRequireWorker:     &requireWorker,
	})
	if err != nil {
		t.Fatalf("create first replay execution should succeed: %v", err)
	}

	_, err = svc.CreateHRISWebhookExecution(HRISWebhookExecutionInput{
		TenantID:                "tenant_demo_jakarta",
		Kind:                    HRISWebhookExecutionKindReceiptProcess,
		TargetID:                "whr_replay_conflict_001",
		ReceiptID:               "whr_replay_conflict_001",
		ConnectorID:             "connector-talenta-replay-conflict",
		Vendor:                  "talenta",
		RequestID:               "req-replay-conflict-002",
		EventType:               "talenta.employee.detail.updated",
		ExecutionMode:           "queued",
		DispatchMode:            HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:            "processing",
		ReplaySourceExecutionID: "hwe_source_failed_001",
		ReplayRequireWorker:     &requireWorker,
	})
	var replayConflictErr *HRISWebhookExecutionReplayConflictError
	if !errors.As(err, &replayConflictErr) {
		t.Fatalf("expected replay conflict error, got %v", err)
	}
	if replayConflictErr.ExistingExecution.ID != first.ID ||
		replayConflictErr.ExistingExecution.Status != HRISWebhookExecutionStatusQueued {
		t.Fatalf("unexpected replay conflict execution: %+v first=%+v", replayConflictErr.ExistingExecution, first)
	}

	if _, err := svc.MarkHRISWebhookExecutionFailed(
		"tenant_demo_jakarta",
		first.ID,
		"failed",
		errors.New("forced replay failure"),
	); err != nil {
		t.Fatalf("mark first replay execution failed should succeed: %v", err)
	}

	second, err := svc.CreateHRISWebhookExecution(HRISWebhookExecutionInput{
		TenantID:                "tenant_demo_jakarta",
		Kind:                    HRISWebhookExecutionKindReceiptProcess,
		TargetID:                "whr_replay_conflict_001",
		ReceiptID:               "whr_replay_conflict_001",
		ConnectorID:             "connector-talenta-replay-conflict",
		Vendor:                  "talenta",
		RequestID:               "req-replay-conflict-003",
		EventType:               "talenta.employee.detail.updated",
		ExecutionMode:           "queued",
		DispatchMode:            HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:            "processing",
		ReplaySourceExecutionID: "hwe_source_failed_001",
		ReplayRequireWorker:     &requireWorker,
	})
	if err != nil {
		t.Fatalf("create replay execution after terminal state should succeed: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("expected replay retry to create a new execution, got %+v", second)
	}
}

func TestClaimHRISWebhookExecutionSupportsFreshSkipAndStaleRecovery(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 24, 8, 0, 0, 0, time.UTC)
	freshStartedAt := now.Add(-2 * time.Minute)
	staleStartedAt := now.Add(-20 * time.Minute)

	svc.hrisWebhookExecutions = []HRISWebhookExecution{
		{
			ID:            "hwe_queue_ready",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_queue_ready",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusQueued,
			QueuedAt:      now.Add(-30 * time.Minute),
			UpdatedAt:     now.Add(-30 * time.Minute),
		},
		{
			ID:            "hwe_running_fresh",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_running_fresh",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusRunning,
			QueuedAt:      now.Add(-25 * time.Minute),
			StartedAt:     &freshStartedAt,
			UpdatedAt:     freshStartedAt,
		},
		{
			ID:            "hwe_running_stale",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_running_stale",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusRunning,
			QueuedAt:      now.Add(-20 * time.Minute),
			StartedAt:     &staleStartedAt,
			UpdatedAt:     staleStartedAt,
		},
		{
			ID:            "hwe_failed_terminal",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_failed_terminal",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusFailed,
			QueuedAt:      now.Add(-15 * time.Minute),
			UpdatedAt:     now.Add(-10 * time.Minute),
		},
	}
	svc.syncQueuedHRISWebhookExecutionIndicesLocked()

	claimedQueued, reason, err := svc.ClaimHRISWebhookExecution("tenant_demo_jakarta", "hwe_queue_ready", 10*time.Minute, now)
	if err != nil {
		t.Fatalf("claim queued execution should succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected no skip reason for queued execution, got %s", reason)
	}
	if claimedQueued.Status != HRISWebhookExecutionStatusRunning || claimedQueued.StartedAt == nil || !claimedQueued.StartedAt.Equal(now) {
		t.Fatalf("unexpected queued claimed execution: %+v", claimedQueued)
	}
	if claimedQueued.AttemptCount != 1 {
		t.Fatalf("expected queued claim to increment attempt_count, got %+v", claimedQueued)
	}

	freshClaim, reason, err := svc.ClaimHRISWebhookExecution("tenant_demo_jakarta", "hwe_running_fresh", 10*time.Minute, now)
	if err != nil {
		t.Fatalf("claim fresh running execution should not error: %v", err)
	}
	if reason != HRISWebhookExecutionClaimReasonInFlight {
		t.Fatalf("expected in-flight claim reason for fresh running execution, got %s", reason)
	}
	if freshClaim.StartedAt == nil || !freshClaim.StartedAt.Equal(freshStartedAt) {
		t.Fatalf("expected fresh running execution to keep original started_at, got %+v", freshClaim)
	}
	if freshClaim.AttemptCount != 0 {
		t.Fatalf("expected fresh skipped execution to keep attempt_count=0, got %+v", freshClaim)
	}

	staleClaim, reason, err := svc.ClaimHRISWebhookExecution("tenant_demo_jakarta", "hwe_running_stale", 10*time.Minute, now)
	if err != nil {
		t.Fatalf("claim stale running execution should succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected stale running execution to be reclaimed, got %s", reason)
	}
	if staleClaim.Status != HRISWebhookExecutionStatusRunning || staleClaim.StartedAt == nil || !staleClaim.StartedAt.Equal(now) {
		t.Fatalf("unexpected stale reclaimed execution: %+v", staleClaim)
	}
	if staleClaim.AttemptCount != 1 {
		t.Fatalf("expected stale reclaim to increment attempt_count, got %+v", staleClaim)
	}

	failedClaim, reason, err := svc.ClaimHRISWebhookExecution("tenant_demo_jakarta", "hwe_failed_terminal", 10*time.Minute, now)
	if err != nil {
		t.Fatalf("claim failed execution should not error: %v", err)
	}
	if reason != HRISWebhookExecutionClaimReasonNotQueueable {
		t.Fatalf("expected not-queueable claim reason for terminal execution, got %s", reason)
	}
	if failedClaim.Status != HRISWebhookExecutionStatusFailed {
		t.Fatalf("expected failed execution to remain terminal, got %+v", failedClaim)
	}
}

func TestListClaimableHRISWebhookExecutionsIncludesQueuedAndStaleRunning(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 24, 8, 30, 0, 0, time.UTC)
	staleStartedAt := now.Add(-20 * time.Minute)
	freshStartedAt := now.Add(-2 * time.Minute)

	svc.hrisWebhookExecutions = []HRISWebhookExecution{
		{
			ID:            "hwe_queue_oldest",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_oldest",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusQueued,
			QueuedAt:      now.Add(-30 * time.Minute),
			UpdatedAt:     now.Add(-30 * time.Minute),
		},
		{
			ID:            "hwe_running_stale",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_stale",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusRunning,
			QueuedAt:      now.Add(-25 * time.Minute),
			StartedAt:     &staleStartedAt,
			UpdatedAt:     staleStartedAt,
		},
		{
			ID:            "hwe_running_fresh",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_fresh",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusRunning,
			QueuedAt:      now.Add(-15 * time.Minute),
			StartedAt:     &freshStartedAt,
			UpdatedAt:     freshStartedAt,
		},
		{
			ID:            "hwe_queue_newest",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_newest",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusQueued,
			QueuedAt:      now.Add(-10 * time.Minute),
			UpdatedAt:     now.Add(-10 * time.Minute),
		},
		{
			ID:            "hwe_failed_terminal",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_failed",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusFailed,
			QueuedAt:      now.Add(-5 * time.Minute),
			UpdatedAt:     now.Add(-5 * time.Minute),
		},
	}

	items := svc.ListClaimableHRISWebhookExecutions(HRISWebhookExecutionKindReceiptProcess, 10*time.Minute, now, 10)
	if len(items) != 3 {
		t.Fatalf("expected 3 claimable executions, got %+v", items)
	}
	if items[0].ID != "hwe_queue_oldest" || items[1].ID != "hwe_running_stale" || items[2].ID != "hwe_queue_newest" {
		t.Fatalf("unexpected claimable execution order: %+v", items)
	}
}

func TestListIndexedClaimableHRISWebhookExecutionsMergesQueuedIndexAndStaleRunning(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 24, 8, 30, 0, 0, time.UTC)
	staleStartedAt := now.Add(-20 * time.Minute)

	svc.hrisWebhookExecutions = []HRISWebhookExecution{
		{
			ID:            "hwe_queue_oldest",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_oldest",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusQueued,
			QueuedAt:      now.Add(-30 * time.Minute),
			UpdatedAt:     now.Add(-30 * time.Minute),
		},
		{
			ID:            "hwe_queue_future",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_future",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusQueued,
			QueuedAt:      now.Add(10 * time.Minute),
			UpdatedAt:     now.Add(-5 * time.Minute),
		},
		{
			ID:            "hwe_running_stale",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_stale",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusRunning,
			QueuedAt:      now.Add(-25 * time.Minute),
			StartedAt:     &staleStartedAt,
			UpdatedAt:     staleStartedAt,
		},
		{
			ID:            "hwe_queue_newest",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindReceiptProcess,
			TargetID:      "whr_newest",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusQueued,
			QueuedAt:      now.Add(-10 * time.Minute),
			UpdatedAt:     now.Add(-10 * time.Minute),
		},
	}
	svc.syncQueuedHRISWebhookExecutionIndicesLocked()

	items := svc.ListIndexedClaimableHRISWebhookExecutions(
		HRISWebhookExecutionKindReceiptProcess,
		10*time.Minute,
		now,
		10,
	)
	if len(items) != 3 {
		t.Fatalf("expected 3 indexed claimable executions, got %+v", items)
	}
	if items[0].ID != "hwe_queue_oldest" || items[1].ID != "hwe_running_stale" || items[2].ID != "hwe_queue_newest" {
		t.Fatalf("unexpected indexed claimable execution order: %+v", items)
	}
}

func TestListIndexedClaimableHRISWebhookExecutionsReturnsStaleRunningWhenQueuedIndexCoolingDown(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 24, 8, 30, 0, 0, time.UTC)
	staleStartedAt := now.Add(-20 * time.Minute)

	svc.hrisWebhookExecutions = []HRISWebhookExecution{
		{
			ID:            "hwe_queue_future",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindDLQReplay,
			TargetID:      "hdlq_future",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusQueued,
			QueuedAt:      now.Add(10 * time.Minute),
			UpdatedAt:     now.Add(-5 * time.Minute),
		},
		{
			ID:            "hwe_running_stale",
			TenantID:      "tenant_demo_jakarta",
			Kind:          HRISWebhookExecutionKindDLQReplay,
			TargetID:      "hdlq_stale",
			ExecutionMode: "queued",
			DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
			Status:        HRISWebhookExecutionStatusRunning,
			QueuedAt:      now.Add(-25 * time.Minute),
			StartedAt:     &staleStartedAt,
			UpdatedAt:     staleStartedAt,
		},
	}
	svc.syncQueuedHRISWebhookExecutionIndicesLocked()

	items := svc.ListIndexedClaimableHRISWebhookExecutions(
		HRISWebhookExecutionKindDLQReplay,
		10*time.Minute,
		now,
		10,
	)
	if len(items) != 1 || items[0].ID != "hwe_running_stale" {
		t.Fatalf("expected stale running execution to remain claimable when queued index is cooling down, got %+v", items)
	}
}

func TestHRISWebhookExecutionRequeueAndAcknowledgeLifecycle(t *testing.T) {
	svc := NewService()
	record, err := svc.CreateHRISWebhookExecution(HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_requeue_001",
		ReceiptID:     "whr_exec_requeue_001",
		ConnectorID:   "connector-talenta-requeue",
		Vendor:        "talenta",
		RequestID:     "talenta-exec-requeue-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "processing",
	})
	if err != nil {
		t.Fatalf("create execution should succeed: %v", err)
	}
	if _, err := svc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", record.ID); err != nil {
		t.Fatalf("mark execution running should succeed: %v", err)
	}

	now := time.Date(2026, 4, 24, 9, 0, 0, 0, time.UTC)
	retryAt := now.Add(2 * time.Minute)
	requeued, err := svc.RequeueHRISWebhookExecution(
		"tenant_demo_jakarta",
		record.ID,
		"processing",
		retryAt,
		errors.New("target is still in flight"),
	)
	if err != nil {
		t.Fatalf("requeue execution should succeed: %v", err)
	}
	if requeued.Status != HRISWebhookExecutionStatusQueued {
		t.Fatalf("expected queued execution after requeue, got %+v", requeued)
	}
	if requeued.TargetStatus != "processing" || requeued.LastError != "target is still in flight" {
		t.Fatalf("unexpected requeued execution payload: %+v", requeued)
	}
	if !requeued.QueuedAt.Equal(retryAt) {
		t.Fatalf("expected requeued execution queued_at=%s, got %s", retryAt, requeued.QueuedAt)
	}
	if requeued.AttemptCount != 1 || requeued.RequeueCount != 1 {
		t.Fatalf("expected requeued execution attempt/requeue counts to be tracked, got %+v", requeued)
	}
	if requeued.StartedAt != nil || requeued.FinishedAt != nil {
		t.Fatalf("expected requeued execution runtime timestamps to be cleared, got %+v", requeued)
	}

	claimed, reason, err := svc.ClaimHRISWebhookExecution("tenant_demo_jakarta", record.ID, 30*time.Second, now)
	if err != nil {
		t.Fatalf("claim future queued execution should not error: %v", err)
	}
	if reason != HRISWebhookExecutionClaimReasonCooldown {
		t.Fatalf("expected cooldown reason for future queued execution, got %s", reason)
	}
	if claimed.Status != HRISWebhookExecutionStatusQueued {
		t.Fatalf("expected future queued execution to remain queued, got %+v", claimed)
	}

	acked, err := svc.AcknowledgeHRISWebhookExecution("tenant_demo_jakarta", record.ID, "processed", nil)
	if err != nil {
		t.Fatalf("acknowledge execution should succeed: %v", err)
	}
	if acked.Status != HRISWebhookExecutionStatusSucceeded || acked.TargetStatus != "processed" {
		t.Fatalf("unexpected acknowledged execution: %+v", acked)
	}
	if acked.AttemptCount != 1 || acked.RequeueCount != 1 {
		t.Fatalf("expected acknowledged execution to retain attempt/requeue audit counts, got %+v", acked)
	}
	if acked.FinishedAt == nil {
		t.Fatalf("expected acknowledged execution to set finished_at")
	}
	if acked.LastError != "" {
		t.Fatalf("expected acknowledged execution to clear last_error, got %+v", acked)
	}
}

func TestHRISWebhookReceiptDueIndexPersistsAndListsDueItemsWithFallback(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	now := time.Date(2026, 4, 24, 7, 0, 0, 0, time.UTC)
	staleAttemptAt := now.Add(-20 * time.Minute)
	freshAttemptAt := now.Add(-1 * time.Minute)
	processedAt := now.Add(-30 * time.Second)

	svc.mu.Lock()
	svc.hrisWebhookReceipts = []HRISWebhookReceipt{
		{
			ID:          "whr_ready",
			TenantID:    "tenant_demo_jakarta",
			ConnectorID: "connector-talenta",
			Vendor:      "talenta",
			Status:      "received",
			ReceivedAt:  now.Add(-25 * time.Minute),
		},
		{
			ID:            "whr_cooldown",
			TenantID:      "tenant_demo_jakarta",
			ConnectorID:   "connector-talenta",
			Vendor:        "talenta",
			Status:        "failed",
			AttemptCount:  1,
			ReceivedAt:    now.Add(-24 * time.Minute),
			LastAttemptAt: &freshAttemptAt,
			ProcessedAt:   &processedAt,
		},
		{
			ID:            "whr_stale",
			TenantID:      "tenant_demo_jakarta",
			ConnectorID:   "connector-talenta",
			Vendor:        "talenta",
			Status:        "processing",
			AttemptCount:  1,
			ReceivedAt:    now.Add(-23 * time.Minute),
			LastAttemptAt: &staleAttemptAt,
		},
	}
	svc.dueReceiptIDs = []hrisWebhookReceiptDueIndexEntry{
		{ReceiptID: "whr_stale", DueAt: staleAttemptAt.Add(10 * time.Minute)},
		{ReceiptID: "whr_ready", DueAt: now.Add(-25 * time.Minute)},
		{ReceiptID: "whr_cooldown", DueAt: freshAttemptAt.Add(5 * time.Minute)},
	}
	svc.normalizeHRISWebhookReceiptDueIndexLocked()
	if err := svc.persistHRISWebhookStateLocked(); err != nil {
		svc.mu.Unlock()
		t.Fatalf("expected webhook runtime state to persist: %v", err)
	}
	svc.mu.Unlock()

	var snapshot hrisWebhookStateSnapshot
	if err := json.Unmarshal(store.items[hrisWebhookStateKey], &snapshot); err != nil {
		t.Fatalf("expected webhook runtime snapshot to decode: %v", err)
	}
	if len(snapshot.DueReceiptIDs) != 3 {
		t.Fatalf("expected 3 persisted due receipt ids, got %+v", snapshot.DueReceiptIDs)
	}
	if snapshot.DueReceiptIDs[0].ReceiptID != "whr_ready" ||
		snapshot.DueReceiptIDs[1].ReceiptID != "whr_stale" ||
		snapshot.DueReceiptIDs[2].ReceiptID != "whr_cooldown" {
		t.Fatalf("unexpected persisted due receipt ordering: %+v", snapshot.DueReceiptIDs)
	}

	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored service with state store to initialize: %v", err)
	}

	items := restored.ListDueHRISWebhookReceiptsWithBackoff(
		"tenant_demo_jakarta",
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		now,
		10,
	)
	if len(items) != 2 {
		t.Fatalf("expected 2 due receipts, got %+v", items)
	}
	if items[0].ID != "whr_ready" || items[1].ID != "whr_stale" {
		t.Fatalf("unexpected due receipt ordering: %+v", items)
	}

	restored.mu.Lock()
	restored.dueReceiptIDs = nil
	restored.mu.Unlock()

	fallbackItems := restored.ListDueHRISWebhookReceiptsWithBackoff(
		"tenant_demo_jakarta",
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		now,
		10,
	)
	if len(fallbackItems) != 2 {
		t.Fatalf("expected fallback due receipts, got %+v", fallbackItems)
	}
	if fallbackItems[0].ID != "whr_ready" || fallbackItems[1].ID != "whr_stale" {
		t.Fatalf("unexpected fallback due receipt ordering: %+v", fallbackItems)
	}
}

func TestLegacyHRISWebhookStateMigratesToDedicatedStateKey(t *testing.T) {
	store := &memoryStateStore{}
	seed := NewService()
	legacySnapshot := seed.coreStateSnapshotLocked()
	legacySnapshot.HRISWebhookReceipts = []HRISWebhookReceipt{
		{
			ID:          "whr_legacy_001",
			TenantID:    "tenant_demo_jakarta",
			ConnectorID: "connector-talenta-legacy",
			Vendor:      "talenta",
			RequestID:   "talenta-legacy-receipt-001",
			Status:      "received",
			ReceivedAt:  time.Date(2026, 4, 24, 2, 0, 0, 0, time.UTC),
		},
	}
	legacySnapshot.HRISWebhookExecutions = []HRISWebhookExecution{
		{
			ID:           "hwe_legacy_001",
			TenantID:     "tenant_demo_jakarta",
			Kind:         HRISWebhookExecutionKindReceiptProcess,
			TargetID:     "whr_legacy_001",
			ReceiptID:    "whr_legacy_001",
			ConnectorID:  "connector-talenta-legacy",
			Vendor:       "talenta",
			Status:       HRISWebhookExecutionStatusQueued,
			DispatchMode: HRISWebhookExecutionDispatchModeWorkerTick,
			QueuedAt:     time.Date(2026, 4, 24, 2, 5, 0, 0, time.UTC),
			UpdatedAt:    time.Date(2026, 4, 24, 2, 5, 0, 0, time.UTC),
		},
	}
	if err := store.Save(stateKey, legacySnapshot); err != nil {
		t.Fatalf("expected legacy state snapshot save to succeed: %v", err)
	}

	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored service with legacy state store to initialize: %v", err)
	}

	receipts := restored.ListHRISWebhookReceipts("tenant_demo_jakarta", "connector-talenta-legacy", 10)
	if len(receipts) != 1 || receipts[0].ID != "whr_legacy_001" {
		t.Fatalf("expected migrated legacy receipt to be restored, got %+v", receipts)
	}
	execution, err := restored.GetHRISWebhookExecution("tenant_demo_jakarta", "hwe_legacy_001")
	if err != nil {
		t.Fatalf("expected migrated legacy execution lookup success: %v", err)
	}
	if execution.DispatchMode != HRISWebhookExecutionDispatchModeWorkerTick {
		t.Fatalf("unexpected migrated legacy execution: %+v", execution)
	}

	rawWebhookState, ok := store.items[hrisWebhookStateKey]
	if !ok {
		t.Fatalf("expected legacy hris webhook state to be migrated into %s", hrisWebhookStateKey)
	}
	var webhookSnapshot hrisWebhookStateSnapshot
	if err := json.Unmarshal(rawWebhookState, &webhookSnapshot); err != nil {
		t.Fatalf("expected migrated webhook snapshot to decode: %v", err)
	}
	if len(webhookSnapshot.HRISWebhookReceipts) != 1 || len(webhookSnapshot.HRISWebhookExecutions) != 1 {
		t.Fatalf("expected migrated webhook snapshot to keep receipt/execution state, got %+v", webhookSnapshot)
	}

	var coreSnapshot stateSnapshot
	if err := json.Unmarshal(store.items[stateKey], &coreSnapshot); err != nil {
		t.Fatalf("expected rewritten core snapshot to decode: %v", err)
	}
	if len(coreSnapshot.HRISWebhookReceipts) != 0 || len(coreSnapshot.HRISWebhookExecutions) != 0 {
		t.Fatalf("expected rewritten core snapshot to exclude migrated webhook state: %+v", coreSnapshot)
	}
}

func TestReceiveHRISWebhookReceiptMergesCompareAndSwapConflict(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	connector, err := svc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	competingReceipt := HRISWebhookReceipt{
		ID:          "whr_competing_001",
		TenantID:    "tenant_demo_jakarta",
		ConnectorID: connector.ID,
		Vendor:      "talenta",
		RequestID:   "talenta-competing-receipt-001",
		Status:      "received",
		ReceivedAt:  time.Date(2026, 4, 24, 2, 10, 0, 0, time.UTC),
	}
	store.compareAndSwapHook = func(key string, expectedExists bool, _ []byte, _ []byte) {
		if key != hrisWebhookStateKey || expectedExists {
			return
		}
		payload, err := json.Marshal(hrisWebhookStateSnapshot{
			HRISWebhookReceipts: []HRISWebhookReceipt{competingReceipt},
		})
		if err != nil {
			t.Fatalf("expected competing webhook snapshot to encode: %v", err)
		}
		store.items[key] = payload
	}

	record, err := svc.ReceiveHRISWebhookReceipt(connector.ID, HRISWebhookReceiptInput{
		EventType:  "talenta.employee.detail.updated",
		RequestID:  "talenta-receipt-cas-001",
		RawPayload: `{"event_type":"talenta.employee.detail.updated"}`,
	})
	if err != nil {
		t.Fatalf("expected receipt create to succeed after CAS conflict: %v", err)
	}

	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored service with state store to initialize: %v", err)
	}
	items := restored.ListHRISWebhookReceipts("tenant_demo_jakarta", connector.ID, 10)
	if len(items) != 2 {
		t.Fatalf("expected both receipts to survive CAS retry, got %+v", items)
	}

	foundCreated := false
	foundCompeting := false
	for i := range items {
		switch items[i].ID {
		case record.ID:
			foundCreated = true
		case competingReceipt.ID:
			foundCompeting = true
		}
	}
	if !foundCreated || !foundCompeting {
		t.Fatalf("expected created and competing receipts to be preserved, got %+v", items)
	}
}

func TestMarkHRISWebhookExecutionFailedMergesCompareAndSwapConflict(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	record, err := svc.CreateHRISWebhookExecution(HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_conflict_001",
		ReceiptID:     "whr_exec_conflict_001",
		ConnectorID:   "connector-talenta-conflict",
		Vendor:        "talenta",
		RequestID:     "talenta-exec-conflict-001",
		EventType:     "talenta.employee.detail.created",
		ExecutionMode: "queued",
		DispatchMode:  HRISWebhookExecutionDispatchModeWorkerTick,
	})
	if err != nil {
		t.Fatalf("expected execution create to succeed: %v", err)
	}

	competingExecution := HRISWebhookExecution{
		ID:           "hwe_competing_001",
		TenantID:     "tenant_demo_jakarta",
		Kind:         HRISWebhookExecutionKindDLQReplay,
		TargetID:     "hdlq_competing_001",
		ConnectorID:  "connector-talenta-conflict",
		Vendor:       "talenta",
		Status:       HRISWebhookExecutionStatusQueued,
		DispatchMode: HRISWebhookExecutionDispatchModeWorkerTick,
		QueuedAt:     time.Date(2026, 4, 24, 2, 15, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 4, 24, 2, 15, 0, 0, time.UTC),
	}
	store.compareAndSwapHook = func(key string, expectedExists bool, expectedPayload []byte, _ []byte) {
		if key != hrisWebhookStateKey || !expectedExists {
			return
		}
		var snapshot hrisWebhookStateSnapshot
		if err := json.Unmarshal(expectedPayload, &snapshot); err != nil {
			t.Fatalf("expected current webhook snapshot to decode: %v", err)
		}
		snapshot.HRISWebhookExecutions = append(
			[]HRISWebhookExecution{competingExecution},
			snapshot.HRISWebhookExecutions...,
		)
		payload, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatalf("expected competing webhook snapshot to encode: %v", err)
		}
		store.items[key] = payload
	}

	failed, err := svc.MarkHRISWebhookExecutionFailed(
		"tenant_demo_jakarta",
		record.ID,
		"dlq",
		errors.New("forced execution conflict failure"),
	)
	if err != nil {
		t.Fatalf("expected execution update to succeed after CAS conflict: %v", err)
	}
	if failed.Status != HRISWebhookExecutionStatusFailed || failed.TargetStatus != "dlq" {
		t.Fatalf("unexpected failed execution after CAS retry: %+v", failed)
	}

	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored service with state store to initialize: %v", err)
	}
	items := restored.ListHRISWebhookExecutions("tenant_demo_jakarta", 10)
	if len(items) != 2 {
		t.Fatalf("expected both executions to survive CAS retry, got %+v", items)
	}

	foundFailed := false
	foundCompeting := false
	for i := range items {
		switch items[i].ID {
		case record.ID:
			foundFailed = items[i].Status == HRISWebhookExecutionStatusFailed &&
				items[i].TargetStatus == "dlq" &&
				items[i].LastError == "forced execution conflict failure"
		case competingExecution.ID:
			foundCompeting = true
		}
	}
	if !foundFailed || !foundCompeting {
		t.Fatalf("expected failed and competing executions to be preserved, got %+v", items)
	}
}

func TestListRetryableHRISWebhookReceipts(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC()
	lastAttemptOld := now.Add(-2 * time.Minute)
	lastAttemptNew := now.Add(-10 * time.Second)

	svc.hrisWebhookReceipts = []HRISWebhookReceipt{
		{
			ID:            "whr_failed_old",
			TenantID:      "tenant_demo_jakarta",
			Status:        "failed",
			AttemptCount:  1,
			ReceivedAt:    now.Add(-10 * time.Minute),
			LastAttemptAt: &lastAttemptOld,
		},
		{
			ID:         "whr_received",
			TenantID:   "tenant_demo_jakarta",
			Status:     "received",
			ReceivedAt: now.Add(-30 * time.Second),
		},
		{
			ID:            "whr_failed_new",
			TenantID:      "tenant_demo_jakarta",
			Status:        "failed",
			AttemptCount:  2,
			ReceivedAt:    now.Add(-5 * time.Minute),
			LastAttemptAt: &lastAttemptNew,
		},
		{
			ID:         "whr_processed",
			TenantID:   "tenant_demo_jakarta",
			Status:     "processed",
			ReceivedAt: now.Add(-time.Minute),
		},
		{
			ID:         "whr_dlq",
			TenantID:   "tenant_demo_jakarta",
			Status:     "dlq",
			ReceivedAt: now.Add(-2 * time.Minute),
		},
		{
			ID:         "whr_other_tenant",
			TenantID:   "tenant_other",
			Status:     "received",
			ReceivedAt: now.Add(-3 * time.Minute),
		},
	}

	items := svc.ListRetryableHRISWebhookReceipts("tenant_demo_jakarta", 10)
	if len(items) != 3 {
		t.Fatalf("expected 3 retryable receipts, got %d", len(items))
	}
	if items[0].ID != "whr_failed_old" || items[1].ID != "whr_received" || items[2].ID != "whr_failed_new" {
		t.Fatalf("unexpected retryable receipt order: %+v", items)
	}
	if items[0].LastAttemptAt == nil {
		t.Fatalf("expected cloned retryable receipt to preserve last_attempt_at")
	}
	if items[0].LastAttemptAt == &lastAttemptOld {
		t.Fatalf("expected retryable receipt last_attempt_at to be cloned")
	}
}

func TestListQueueableHRISWebhookReceiptsIncludesProcessing(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC()
	lastAttemptOld := now.Add(-2 * time.Minute)
	lastAttemptNew := now.Add(-10 * time.Second)
	lastAttemptProcessing := now.Add(-time.Minute)

	svc.hrisWebhookReceipts = []HRISWebhookReceipt{
		{
			ID:            "whr_failed_old",
			TenantID:      "tenant_demo_jakarta",
			Status:        "failed",
			AttemptCount:  1,
			ReceivedAt:    now.Add(-10 * time.Minute),
			LastAttemptAt: &lastAttemptOld,
		},
		{
			ID:            "whr_processing",
			TenantID:      "tenant_demo_jakarta",
			Status:        "processing",
			AttemptCount:  1,
			ReceivedAt:    now.Add(-8 * time.Minute),
			LastAttemptAt: &lastAttemptProcessing,
		},
		{
			ID:         "whr_received",
			TenantID:   "tenant_demo_jakarta",
			Status:     "received",
			ReceivedAt: now.Add(-30 * time.Second),
		},
		{
			ID:            "whr_failed_new",
			TenantID:      "tenant_demo_jakarta",
			Status:        "failed",
			AttemptCount:  2,
			ReceivedAt:    now.Add(-5 * time.Minute),
			LastAttemptAt: &lastAttemptNew,
		},
		{
			ID:         "whr_processed",
			TenantID:   "tenant_demo_jakarta",
			Status:     "processed",
			ReceivedAt: now.Add(-time.Minute),
		},
	}

	items := svc.ListQueueableHRISWebhookReceipts("tenant_demo_jakarta", 10)
	if len(items) != 4 {
		t.Fatalf("expected 4 queueable receipts, got %d", len(items))
	}
	if items[0].ID != "whr_failed_old" || items[1].ID != "whr_processing" || items[2].ID != "whr_received" || items[3].ID != "whr_failed_new" {
		t.Fatalf("unexpected queueable receipt order: %+v", items)
	}
	if items[1].LastAttemptAt == nil {
		t.Fatalf("expected queueable processing receipt to preserve last_attempt_at")
	}
	if items[1].LastAttemptAt == &lastAttemptProcessing {
		t.Fatalf("expected queueable processing receipt last_attempt_at to be cloned")
	}
}

func TestListClaimableHRISWebhookReceiptsWithBackoffFiltersCooldownAttemptLimitAndInFlight(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC()
	oldAttemptAt := now.Add(-20 * time.Minute)
	freshAttemptAt := now.Add(-time.Minute)

	svc.hrisWebhookReceipts = []HRISWebhookReceipt{
		{
			ID:         "whr_received",
			TenantID:   "tenant_demo_jakarta",
			Status:     "received",
			ReceivedAt: now.Add(-3 * time.Minute),
		},
		{
			ID:            "whr_failed_stale",
			TenantID:      "tenant_demo_jakarta",
			Status:        "failed",
			AttemptCount:  1,
			ReceivedAt:    now.Add(-10 * time.Minute),
			LastAttemptAt: &oldAttemptAt,
		},
		{
			ID:            "whr_failed_cooldown",
			TenantID:      "tenant_demo_jakarta",
			Status:        "failed",
			AttemptCount:  1,
			ReceivedAt:    now.Add(-9 * time.Minute),
			LastAttemptAt: &freshAttemptAt,
		},
		{
			ID:            "whr_failed_attempt_limit",
			TenantID:      "tenant_demo_jakarta",
			Status:        "failed",
			AttemptCount:  3,
			ReceivedAt:    now.Add(-8 * time.Minute),
			LastAttemptAt: &oldAttemptAt,
		},
		{
			ID:            "whr_processing_fresh",
			TenantID:      "tenant_demo_jakarta",
			Status:        "processing",
			AttemptCount:  1,
			ReceivedAt:    now.Add(-7 * time.Minute),
			LastAttemptAt: &freshAttemptAt,
		},
		{
			ID:            "whr_processing_stale",
			TenantID:      "tenant_demo_jakarta",
			Status:        "processing",
			AttemptCount:  1,
			ReceivedAt:    now.Add(-6 * time.Minute),
			LastAttemptAt: &oldAttemptAt,
		},
		{
			ID:         "whr_other_tenant",
			TenantID:   "tenant_other",
			Status:     "received",
			ReceivedAt: now.Add(-5 * time.Minute),
		},
	}

	items := svc.ListClaimableHRISWebhookReceiptsWithBackoff(
		"tenant_demo_jakarta",
		3,
		5*time.Minute,
		15*time.Minute,
		5*time.Minute,
		now,
		10,
	)
	if len(items) != 3 {
		t.Fatalf("expected 3 claimable receipts, got %d", len(items))
	}
	if items[0].ID != "whr_processing_stale" || items[1].ID != "whr_failed_stale" || items[2].ID != "whr_received" {
		t.Fatalf("unexpected claimable receipt order: %+v", items)
	}
	if items[0].LastAttemptAt == nil {
		t.Fatalf("expected stale failed receipt to preserve last_attempt_at")
	}
	if items[0].LastAttemptAt == &oldAttemptAt {
		t.Fatalf("expected claimable receipt last_attempt_at to be cloned")
	}
}

func TestClaimHRISWebhookReceiptForProcessingAppliesQueuePolicy(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC()
	oldAttemptAt := now.Add(-10 * time.Minute)
	freshAttemptAt := now.Add(-time.Minute)

	svc.hrisWebhookReceipts = []HRISWebhookReceipt{
		{
			ID:         "whr_received",
			TenantID:   "tenant_demo_jakarta",
			Status:     "received",
			ReceivedAt: now.Add(-20 * time.Minute),
		},
		{
			ID:            "whr_failed_cooldown",
			TenantID:      "tenant_demo_jakarta",
			Status:        "failed",
			AttemptCount:  1,
			ReceivedAt:    now.Add(-20 * time.Minute),
			LastAttemptAt: &freshAttemptAt,
		},
		{
			ID:            "whr_processing_fresh",
			TenantID:      "tenant_demo_jakarta",
			Status:        "processing",
			AttemptCount:  1,
			ReceivedAt:    now.Add(-20 * time.Minute),
			LastAttemptAt: &freshAttemptAt,
		},
		{
			ID:            "whr_processing_stale",
			TenantID:      "tenant_demo_jakarta",
			Status:        "processing",
			AttemptCount:  1,
			ReceivedAt:    now.Add(-20 * time.Minute),
			LastAttemptAt: &oldAttemptAt,
		},
		{
			ID:         "whr_processed",
			TenantID:   "tenant_demo_jakarta",
			Status:     "processed",
			ReceivedAt: now.Add(-20 * time.Minute),
		},
	}

	claimed, reason, err := svc.ClaimHRISWebhookReceiptForProcessing(
		"tenant_demo_jakarta",
		"whr_received",
		3,
		5*time.Minute,
		5*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("expected received claim to succeed: %v", err)
	}
	if reason != "" || claimed.Status != "processing" || claimed.AttemptCount != 1 || claimed.LastAttemptAt == nil {
		t.Fatalf("unexpected received claim result reason=%q item=%+v", reason, claimed)
	}

	cooldown, reason, err := svc.ClaimHRISWebhookReceiptForProcessing(
		"tenant_demo_jakarta",
		"whr_failed_cooldown",
		3,
		5*time.Minute,
		5*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("expected cooldown claim check to succeed: %v", err)
	}
	if reason != HRISWebhookReceiptClaimReasonCooldown || cooldown.AttemptCount != 1 {
		t.Fatalf("unexpected cooldown claim result reason=%q item=%+v", reason, cooldown)
	}

	inFlight, reason, err := svc.ClaimHRISWebhookReceiptForProcessing(
		"tenant_demo_jakarta",
		"whr_processing_fresh",
		3,
		5*time.Minute,
		5*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("expected in-flight claim check to succeed: %v", err)
	}
	if reason != HRISWebhookReceiptClaimReasonInFlight || inFlight.AttemptCount != 1 {
		t.Fatalf("unexpected in-flight claim result reason=%q item=%+v", reason, inFlight)
	}

	stale, reason, err := svc.ClaimHRISWebhookReceiptForProcessing(
		"tenant_demo_jakarta",
		"whr_processing_stale",
		3,
		5*time.Minute,
		5*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("expected stale processing claim to succeed: %v", err)
	}
	if reason != "" || stale.Status != "processing" || stale.AttemptCount != 2 || stale.LastAttemptAt == nil {
		t.Fatalf("unexpected stale processing claim result reason=%q item=%+v", reason, stale)
	}

	processed, reason, err := svc.ClaimHRISWebhookReceiptForProcessing(
		"tenant_demo_jakarta",
		"whr_processed",
		3,
		5*time.Minute,
		5*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("expected processed claim check to succeed: %v", err)
	}
	if reason != HRISWebhookReceiptClaimReasonNotQueueable || processed.Status != "processed" {
		t.Fatalf("unexpected processed claim result reason=%q item=%+v", reason, processed)
	}
}

func TestClaimHRISWebhookReceiptForProcessingWithBackoffAppliesExponentialCooldown(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC()
	lastAttemptAt := now.Add(-6 * time.Minute)

	svc.hrisWebhookReceipts = []HRISWebhookReceipt{
		{
			ID:            "whr_failed_backoff",
			TenantID:      "tenant_demo_jakarta",
			Status:        "failed",
			AttemptCount:  2,
			ReceivedAt:    now.Add(-30 * time.Minute),
			LastAttemptAt: &lastAttemptAt,
		},
	}

	cooldown, reason, err := svc.ClaimHRISWebhookReceiptForProcessingWithBackoff(
		"tenant_demo_jakarta",
		"whr_failed_backoff",
		5,
		5*time.Minute,
		20*time.Minute,
		5*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("expected exponential cooldown check to succeed: %v", err)
	}
	if reason != HRISWebhookReceiptClaimReasonCooldown {
		t.Fatalf("expected cooldown skip reason, got %q", reason)
	}
	if cooldown.AttemptCount != 2 || cooldown.Status != "failed" {
		t.Fatalf("unexpected cooldown claim result: %+v", cooldown)
	}

	claimed, reason, err := svc.ClaimHRISWebhookReceiptForProcessingWithBackoff(
		"tenant_demo_jakarta",
		"whr_failed_backoff",
		5,
		5*time.Minute,
		20*time.Minute,
		5*time.Minute,
		now.Add(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("expected claim after exponential cooldown to succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected empty skip reason after cooldown expiry, got %q", reason)
	}
	if claimed.Status != "processing" || claimed.AttemptCount != 3 || claimed.LastAttemptAt == nil {
		t.Fatalf("unexpected claimed receipt after cooldown expiry: %+v", claimed)
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

func TestSyncEmployeesRejectsInvalidSyncSource(t *testing.T) {
	svc := NewService()

	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"custom_connector_unknown",
		"qa",
		[]EmployeeSyncInput{
			{
				ExternalID: "hr-source-invalid-1",
				Email:      "invalid.source@sudirman.co",
				FullName:   "Invalid Source",
				Department: "IT",
				JobTitle:   "Engineer",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
	)
	if !errors.Is(err, ErrInvalidSyncSource) {
		t.Fatalf("expected ErrInvalidSyncSource, got: %v", err)
	}
}

func TestSyncEmployeesAcceptsVendorSyncSource(t *testing.T) {
	svc := NewService()

	result, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"hris_talenta",
		"qa",
		[]EmployeeSyncInput{
			{
				ExternalID: "hr-source-vendor-1",
				Email:      "vendor.source@sudirman.co",
				FullName:   "Vendor Source",
				Department: "IT",
				JobTitle:   "Engineer",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("vendor source sync should succeed: %v", err)
	}
	if result.Job.Source != "hris_talenta" {
		t.Fatalf("expected job source hris_talenta, got %s", result.Job.Source)
	}
	if len(result.Items) != 1 || result.Items[0].Source != "hris_talenta" {
		t.Fatalf("expected employee item source hris_talenta, got %+v", result.Items)
	}
}

func TestSyncEmployeesStoresExtendedCanonicalFields(t *testing.T) {
	svc := NewService()
	email := "extended.fields@sudirman.co"

	result, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"manual",
		"qa",
		[]EmployeeSyncInput{
			{
				ExternalID:        "hr-extended-001",
				EmployeeNumber:    "EMP-7788",
				Email:             email,
				FullName:          "Extended Fields",
				Department:        "Operations",
				JobTitle:          "Shift Lead",
				Location:          "Jakarta HQ",
				Phone:             "+62 811 7788 9900",
				ManagerExternalID: "hr-manager-1",
				EmploymentStatus:  "active",
				JoinDate:          "2024-01-15",
				ResignDate:        "2027-12-31",
				ShiftCode:         "SHIFT-A",
				ScheduleWindow:    "mon-fri:09:00-18:00",
				LeaveStatus:       "none",
				CostCenter:        "CC-OPS-01",
				PhotoURL:          "https://cdn.example.com/photos/hr-extended-001.jpg",
				Status:            "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("extended field sync should succeed: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one synced item, got %d", len(result.Items))
	}

	employee := result.Items[0]
	if employee.EmployeeNumber != "EMP-7788" {
		t.Fatalf("employee_number mismatch: %s", employee.EmployeeNumber)
	}
	if employee.JoinDate != "2024-01-15" {
		t.Fatalf("join_date mismatch: %s", employee.JoinDate)
	}
	if employee.ResignDate != "2027-12-31" {
		t.Fatalf("resign_date mismatch: %s", employee.ResignDate)
	}
	if employee.ShiftCode != "SHIFT-A" {
		t.Fatalf("shift_code mismatch: %s", employee.ShiftCode)
	}
	if employee.ScheduleWindow != "mon-fri:09:00-18:00" {
		t.Fatalf("schedule_window mismatch: %s", employee.ScheduleWindow)
	}
	if employee.LeaveStatus != "none" {
		t.Fatalf("leave_status mismatch: %s", employee.LeaveStatus)
	}
	if employee.CostCenter != "CC-OPS-01" {
		t.Fatalf("cost_center mismatch: %s", employee.CostCenter)
	}
	if employee.PhotoURL != "https://cdn.example.com/photos/hr-extended-001.jpg" {
		t.Fatalf("photo_url mismatch: %s", employee.PhotoURL)
	}
}

func TestSyncEmployeesWithAccessUpsertMetadataPersistsConnectorContext(t *testing.T) {
	svc := NewService()
	requestID := "req-meta-connector-001"
	tenantID := "tenant_demo_jakarta"

	_, _, _, _, err := svc.SyncEmployeesWithAccessUpsertMetadata(
		tenantID,
		"hris_talenta",
		"qa",
		requestID,
		"connector_talenta_jakarta",
		"s3://mistypass-sync-raw/talenta/2026-04-22/event-001.json",
		[]EmployeeSyncInput{
			{
				ExternalID: "hr-meta-001",
				Email:      "meta.connector@sudirman.co",
				FullName:   "Meta Connector",
				Department: "IT",
				JobTitle:   "Engineer",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
		func(items []EnterpriseEmployee) (int, int, int, error) {
			return 1, 0, 0, nil
		},
	)
	if err != nil {
		t.Fatalf("sync with metadata should succeed: %v", err)
	}

	record, getErr := svc.GetSyncRequestRecord(tenantID, requestID)
	if getErr != nil {
		t.Fatalf("expected sync request record, got error: %v", getErr)
	}
	if record.ConnectorID != "connector_talenta_jakarta" {
		t.Fatalf("connector_id mismatch: %s", record.ConnectorID)
	}
	if record.RawPayloadRef != "s3://mistypass-sync-raw/talenta/2026-04-22/event-001.json" {
		t.Fatalf("raw_payload_ref mismatch: %s", record.RawPayloadRef)
	}
}

func TestSyncWorkerAlertStatePersistsToDedicatedStateKey(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	subscription, err := svc.UpsertSyncWorkerAlertSubscription(SyncWorkerAlertSubscriptionUpsertOptions{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		Window:               15 * time.Minute,
		Cooldown:             15 * time.Minute,
		EmailEnabled:         true,
		ReceiverGroups:       []string{"security"},
	})
	if err != nil {
		t.Fatalf("expected sync worker alert subscription upsert to succeed: %v", err)
	}

	now := time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC)
	_, err = svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
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
		},
		TriggeredAt: now,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected sync worker alert dispatch to succeed: %v", err)
	}

	rawCore, ok := store.items[stateKey]
	if !ok {
		t.Fatalf("expected %s to be persisted", stateKey)
	}
	var coreSnapshot stateSnapshot
	if err := json.Unmarshal(rawCore, &coreSnapshot); err != nil {
		t.Fatalf("expected %s payload to decode: %v", stateKey, err)
	}
	if len(coreSnapshot.SyncWorkerAlertSubscriptions) != 0 ||
		len(coreSnapshot.SyncWorkerAlertNotifications) != 0 ||
		len(coreSnapshot.SyncWorkerAlertCooldowns) != 0 {
		t.Fatalf("expected sync worker alert state to be excluded from %s snapshot: %+v", stateKey, coreSnapshot)
	}

	rawAlert, ok := store.items[syncWorkerAlertStateKey]
	if !ok {
		t.Fatalf("expected %s to be persisted", syncWorkerAlertStateKey)
	}
	var alertSnapshot syncWorkerAlertStateSnapshot
	if err := json.Unmarshal(rawAlert, &alertSnapshot); err != nil {
		t.Fatalf("expected %s payload to decode: %v", syncWorkerAlertStateKey, err)
	}
	if len(alertSnapshot.SyncWorkerAlertSubscriptions) != 1 {
		t.Fatalf("expected one persisted sync worker alert subscription, got %d", len(alertSnapshot.SyncWorkerAlertSubscriptions))
	}
	if len(alertSnapshot.SyncWorkerAlertNotifications) != 1 {
		t.Fatalf("expected one persisted sync worker alert notification, got %d", len(alertSnapshot.SyncWorkerAlertNotifications))
	}
	if len(alertSnapshot.SyncWorkerAlertCooldowns) != 1 {
		t.Fatalf("expected one persisted sync worker alert cooldown, got %d", len(alertSnapshot.SyncWorkerAlertCooldowns))
	}

	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored service with state store to initialize: %v", err)
	}
	restoredSubscription, found := restored.GetSyncWorkerAlertSubscription("tenant_demo_jakarta")
	if !found || !restoredSubscription.Enabled {
		t.Fatalf("expected restored sync worker alert subscription to be enabled, found=%v record=%+v", found, restoredSubscription)
	}
	history := restored.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) != 1 || history[0].Status != "sent" {
		t.Fatalf("expected restored sync worker alert history to include sent notification, got %+v", history)
	}
}

func TestCoreStatePersistDoesNotOverwriteDedicatedSyncWorkerAlertState(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	subscription, err := svc.UpsertSyncWorkerAlertSubscription(SyncWorkerAlertSubscriptionUpsertOptions{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		Window:               15 * time.Minute,
		Cooldown:             15 * time.Minute,
		EmailEnabled:         true,
		ReceiverGroups:       []string{"security"},
	})
	if err != nil {
		t.Fatalf("expected sync worker alert subscription upsert to succeed: %v", err)
	}

	_, err = svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
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
		TriggeredAt: time.Date(2026, 4, 23, 9, 30, 0, 0, time.UTC),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected sync worker alert dispatch to succeed: %v", err)
	}

	connector, err := svc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"linovhr",
		"active",
		"webhook",
		"vault://tenant_demo_jakarta/hris/linovhr/api_key",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected core state write to succeed: %v", err)
	}

	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored service with state store to initialize: %v", err)
	}
	history := restored.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) != 1 || history[0].WorkerAction != "enterprise_hris_webhook_processing_alert" {
		t.Fatalf("expected dedicated sync worker alert state to survive core state save, got %+v", history)
	}
	connectors := restored.ListHRISConnectors("tenant_demo_jakarta")
	if len(connectors) != 1 || connectors[0].ID != connector.ID {
		t.Fatalf("expected restored core state to include connector %s, got %+v", connector.ID, connectors)
	}

	var coreSnapshot stateSnapshot
	if err := json.Unmarshal(store.items[stateKey], &coreSnapshot); err != nil {
		t.Fatalf("expected %s payload to decode: %v", stateKey, err)
	}
	if len(coreSnapshot.SyncWorkerAlertNotifications) != 0 {
		t.Fatalf("expected core snapshot to exclude sync worker alert notifications after core save, got %d", len(coreSnapshot.SyncWorkerAlertNotifications))
	}
}

func TestNewServiceWithStateStoreMigratesLegacySyncWorkerAlertState(t *testing.T) {
	store := &memoryStateStore{}
	base := NewService()
	base.mu.Lock()
	legacySnapshot := base.coreStateSnapshotLocked()
	base.mu.Unlock()

	now := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
	nextRetryAt := now.Add(5 * time.Minute)
	legacySnapshot.SyncWorkerAlertSubscriptions = []SyncWorkerAlertSubscription{
		{
			TenantID:             "tenant_demo_jakarta",
			Enabled:              true,
			WorkerAlertThreshold: 2,
			WindowSeconds:        int64((15 * time.Minute).Seconds()),
			CooldownSeconds:      int64((15 * time.Minute).Seconds()),
			Channels: SyncWorkerAlertSubscriptionChannels{
				Email: true,
			},
			ReceiverGroups: []string{"security"},
			UpdatedAt:      now,
		},
	}
	legacySnapshot.SyncWorkerAlertNotifications = []SyncWorkerAlertNotification{
		{
			ID:             "swa_legacy_001",
			TenantID:       "tenant_demo_jakarta",
			WorkerAction:   "enterprise_hris_pull_worker_alert",
			WorkerKind:     "hris_pull",
			WorkerLabel:    "HRIS Pull Reconcile",
			Fingerprint:    "enterprise_hris_pull_worker_alert|connector-talenta-001|talenta|incremental",
			Count:          3,
			Threshold:      2,
			Failed:         2,
			Processed:      3,
			Applied:        1,
			Channels:       []string{"email"},
			ReceiverGroups: []string{"security"},
			Status:         "failed",
			Reason:         "provider_transient_error",
			IdempotencyKey: "legacy-sync-worker-alert",
			Attempt:        1,
			Retryable:      true,
			NextRetryAt:    &nextRetryAt,
			ConnectorID:    "connector-talenta-001",
			Vendor:         "talenta",
			Mode:           "incremental",
			Provider:       "mock",
			ProviderError:  "temporary outage",
			TriggeredAt:    now,
		},
	}
	legacySnapshot.SyncWorkerAlertCooldowns = []SyncWorkerAlertCooldown{
		{
			TenantID:    "tenant_demo_jakarta",
			Fingerprint: "enterprise_hris_pull_worker_alert|connector-talenta-001|talenta|incremental",
			LastSentAt:  now.Add(-2 * time.Minute),
		},
	}
	if err := store.Save(stateKey, legacySnapshot); err != nil {
		t.Fatalf("expected legacy state snapshot save to succeed: %v", err)
	}

	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored service with legacy state store to initialize: %v", err)
	}
	subscription, found := restored.GetSyncWorkerAlertSubscription("tenant_demo_jakarta")
	if !found || !subscription.Enabled {
		t.Fatalf("expected legacy sync worker alert subscription to be restored, found=%v record=%+v", found, subscription)
	}
	history := restored.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) != 1 || history[0].ID != "swa_legacy_001" {
		t.Fatalf("expected legacy sync worker alert history to be restored, got %+v", history)
	}

	rawAlert, ok := store.items[syncWorkerAlertStateKey]
	if !ok {
		t.Fatalf("expected legacy sync worker alert state to be migrated into %s", syncWorkerAlertStateKey)
	}
	var alertSnapshot syncWorkerAlertStateSnapshot
	if err := json.Unmarshal(rawAlert, &alertSnapshot); err != nil {
		t.Fatalf("expected migrated alert snapshot to decode: %v", err)
	}
	if len(alertSnapshot.SyncWorkerAlertNotifications) != 1 {
		t.Fatalf("expected one migrated sync worker alert notification, got %d", len(alertSnapshot.SyncWorkerAlertNotifications))
	}

	var coreSnapshot stateSnapshot
	if err := json.Unmarshal(store.items[stateKey], &coreSnapshot); err != nil {
		t.Fatalf("expected rewritten core snapshot to decode: %v", err)
	}
	if len(coreSnapshot.SyncWorkerAlertSubscriptions) != 0 ||
		len(coreSnapshot.SyncWorkerAlertNotifications) != 0 ||
		len(coreSnapshot.SyncWorkerAlertCooldowns) != 0 {
		t.Fatalf("expected migrated core snapshot to exclude legacy sync worker alert state: %+v", coreSnapshot)
	}
}

func TestSyncWorkerAlertReadPathRefreshesAcrossServices(t *testing.T) {
	store := &memoryStateStore{}
	writer, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected writer service to initialize: %v", err)
	}
	reader, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected reader service to initialize: %v", err)
	}

	subscription, err := writer.UpsertSyncWorkerAlertSubscription(SyncWorkerAlertSubscriptionUpsertOptions{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		Window:               15 * time.Minute,
		Cooldown:             15 * time.Minute,
		EmailEnabled:         true,
		ReceiverGroups:       []string{"security"},
	})
	if err != nil {
		t.Fatalf("expected writer subscription upsert to succeed: %v", err)
	}

	_, err = writer.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
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
		},
		TriggeredAt: time.Date(2026, 4, 23, 11, 0, 0, 0, time.UTC),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected writer dispatch to succeed: %v", err)
	}

	restoredSubscription, found := reader.GetSyncWorkerAlertSubscription("tenant_demo_jakarta")
	if !found || !restoredSubscription.Enabled {
		t.Fatalf("expected reader to observe refreshed subscription, found=%v record=%+v", found, restoredSubscription)
	}

	history := reader.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) != 1 {
		t.Fatalf("expected reader to observe one refreshed notification, got %d", len(history))
	}
	if history[0].WorkerAction != "enterprise_hris_pull_worker_alert" || history[0].Status != "sent" {
		t.Fatalf("unexpected refreshed notification history: %+v", history)
	}
}

func TestSyncWorkerAlertWritePathRefreshesAcrossServices(t *testing.T) {
	store := &memoryStateStore{}
	writer, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected writer service to initialize: %v", err)
	}
	reader, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected reader service to initialize: %v", err)
	}

	subscription, err := writer.UpsertSyncWorkerAlertSubscription(SyncWorkerAlertSubscriptionUpsertOptions{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		Window:               15 * time.Minute,
		Cooldown:             15 * time.Minute,
		EmailEnabled:         true,
		ReceiverGroups:       []string{"security"},
	})
	if err != nil {
		t.Fatalf("expected writer subscription upsert to succeed: %v", err)
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
	triggeredAt := time.Date(2026, 4, 23, 11, 30, 0, 0, time.UTC)
	writerCalls := 0
	_, err = writer.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts:       []SyncWorkerAlertDispatchAlert{alert},
		TriggeredAt:  triggeredAt,
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			writerCalls++
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected writer dispatch to succeed: %v", err)
	}
	if writerCalls != 1 {
		t.Fatalf("expected writer to dispatch once, got %d", writerCalls)
	}

	readerCalls := 0
	result, err := reader.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts:       []SyncWorkerAlertDispatchAlert{alert},
		TriggeredAt:  triggeredAt.Add(5 * time.Minute),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			readerCalls++
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected reader dispatch to succeed: %v", err)
	}
	if readerCalls != 0 {
		t.Fatalf("expected reader dispatch to observe refreshed cooldown and skip provider call, got %d calls", readerCalls)
	}
	if result.Dispatched != 0 || result.Skipped != 1 {
		t.Fatalf("unexpected refreshed writer/reader dispatch result: %+v", result)
	}
	if len(result.Items) != 1 || result.Items[0].Reason != "cooldown" {
		t.Fatalf("expected refreshed cooldown skip result, got %+v", result.Items)
	}
}

func TestUpsertSyncWorkerAlertSubscriptionRetriesOnCompareAndSwapConflict(t *testing.T) {
	store := &memoryStateStore{}
	otherSubscription := SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_factory",
		Enabled:              true,
		WorkerAlertThreshold: 3,
		WindowSeconds:        int64((20 * time.Minute).Seconds()),
		CooldownSeconds:      int64((10 * time.Minute).Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"ops"},
		UpdatedAt:      time.Date(2026, 4, 23, 11, 30, 0, 0, time.UTC),
	}
	store.compareAndSwapHook = func(key string, expectedExists bool, expectedPayload []byte, _ []byte) {
		if key != syncWorkerAlertStateKey || expectedExists {
			return
		}
		payload, err := json.Marshal(syncWorkerAlertStateSnapshot{
			SyncWorkerAlertSubscriptions: []SyncWorkerAlertSubscription{otherSubscription},
		})
		if err != nil {
			t.Fatalf("expected competing alert snapshot to encode: %v", err)
		}
		store.items[key] = payload
	}

	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	record, err := svc.UpsertSyncWorkerAlertSubscription(SyncWorkerAlertSubscriptionUpsertOptions{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		Window:               15 * time.Minute,
		Cooldown:             15 * time.Minute,
		EmailEnabled:         true,
		ReceiverGroups:       []string{"security"},
	})
	if err != nil {
		t.Fatalf("expected subscription upsert to retry after CAS conflict: %v", err)
	}
	if record.TenantID != "tenant_demo_jakarta" || !record.Enabled {
		t.Fatalf("unexpected upserted subscription: %+v", record)
	}

	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored service with state store to initialize: %v", err)
	}
	jakartaSubscription, found := restored.GetSyncWorkerAlertSubscription("tenant_demo_jakarta")
	if !found || !jakartaSubscription.Enabled {
		t.Fatalf("expected jakarta subscription to survive CAS retry, found=%v record=%+v", found, jakartaSubscription)
	}
	factorySubscription, found := restored.GetSyncWorkerAlertSubscription("tenant_demo_factory")
	if !found || factorySubscription.WorkerAlertThreshold != otherSubscription.WorkerAlertThreshold {
		t.Fatalf("expected competing subscription to be preserved, found=%v record=%+v", found, factorySubscription)
	}
}

func TestSyncWorkerAlertDispatchMergesCompareAndSwapConflict(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}

	subscription, err := svc.UpsertSyncWorkerAlertSubscription(SyncWorkerAlertSubscriptionUpsertOptions{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		Window:               15 * time.Minute,
		Cooldown:             15 * time.Minute,
		EmailEnabled:         true,
		ReceiverGroups:       []string{"security"},
	})
	if err != nil {
		t.Fatalf("expected subscription upsert to succeed: %v", err)
	}

	competingNotification := SyncWorkerAlertNotification{
		ID:             "swa_competing_001",
		TenantID:       "tenant_demo_jakarta",
		WorkerAction:   "enterprise_hris_pull_worker_alert",
		WorkerKind:     "hris_pull",
		WorkerLabel:    "HRIS Pull Reconcile",
		Fingerprint:    "enterprise_hris_pull_worker_alert|connector-talenta-001|talenta|incremental",
		Count:          3,
		Threshold:      2,
		Failed:         2,
		Processed:      3,
		Applied:        1,
		ConnectorID:    "connector-talenta-001",
		Vendor:         "talenta",
		Mode:           "incremental",
		Status:         "failed",
		Reason:         "provider_transient_error",
		IdempotencyKey: "competing-idempotency",
		Attempt:        1,
		Retryable:      true,
		TriggeredAt:    time.Date(2026, 4, 23, 11, 35, 0, 0, time.UTC),
	}

	store.compareAndSwapHook = func(key string, expectedExists bool, expectedPayload []byte, _ []byte) {
		if key != syncWorkerAlertStateKey || !expectedExists {
			return
		}
		var snapshot syncWorkerAlertStateSnapshot
		if err := json.Unmarshal(expectedPayload, &snapshot); err != nil {
			t.Fatalf("expected current alert snapshot to decode: %v", err)
		}
		snapshot.SyncWorkerAlertNotifications = append(
			[]SyncWorkerAlertNotification{competingNotification},
			snapshot.SyncWorkerAlertNotifications...,
		)
		payload, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatalf("expected competing alert snapshot to encode: %v", err)
		}
		store.items[key] = payload
	}

	result, err := svc.DispatchSyncWorkerAlerts(SyncWorkerAlertDispatchInput{
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
				ConnectorID:  "connector-talenta-002",
				Vendor:       "talenta",
				FailureStage: "merge",
			},
		},
		TriggeredAt: time.Date(2026, 4, 23, 11, 40, 0, 0, time.UTC),
		Dispatch: func(input SyncWorkerAlertDeliveryInput) SyncWorkerAlertDeliveryResult {
			return SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected dispatch to succeed after CAS conflict merge: %v", err)
	}
	if result.Dispatched != 1 || len(result.Items) != 1 {
		t.Fatalf("unexpected dispatch result: %+v", result)
	}

	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored service with state store to initialize: %v", err)
	}
	history := restored.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) != 2 {
		t.Fatalf("expected merged history to preserve competing and new notification, got %+v", history)
	}
	if history[0].ID != result.Items[0].ID || history[1].ID != competingNotification.ID {
		t.Fatalf("expected new notification to prepend competing history after CAS retry, got %+v", history)
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
