package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/hris"
	"github.com/mistypass/cloud/api/internal/modules/hris/talenta"
)

func waitForEnterpriseHRISWebhookDLQStatus(
	t *testing.T,
	s *server,
	entryID string,
	expectedStatus string,
) hris.DeadLetterEntry {
	t.Helper()

	var last hris.DeadLetterEntry
	for attempt := 0; attempt < 50; attempt++ {
		item, err := s.hrisDLQSvc.GetEntry(entryID)
		if err == nil {
			last = item
			if strings.TrimSpace(item.Status) == expectedStatus {
				return item
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected dlq entry %s status %s, got %+v", entryID, expectedStatus, last)
	return hris.DeadLetterEntry{}
}

func waitForEnterpriseHRISWebhookExecutionStatus(
	t *testing.T,
	s *server,
	tenantID string,
	executionID string,
	expectedStatus string,
) enterprise.HRISWebhookExecution {
	t.Helper()

	var last enterprise.HRISWebhookExecution
	for attempt := 0; attempt < 50; attempt++ {
		item, err := s.enterpriseSvc.GetHRISWebhookExecution(tenantID, executionID)
		if err == nil {
			last = item
			if strings.TrimSpace(item.Status) == expectedStatus {
				return item
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected execution %s status %s, got %+v", executionID, expectedStatus, last)
	return enterprise.HRISWebhookExecution{}
}

type flakyGadjianNormalizer struct {
	calls int
}

func (n *flakyGadjianNormalizer) Vendor() string {
	return "gadjian"
}

func (n *flakyGadjianNormalizer) NormalizeWebhook(receipt enterprise.HRISWebhookReceipt) (hris.NormalizedSyncRequest, error) {
	n.calls++
	if n.calls == 1 {
		return hris.NormalizedSyncRequest{}, errors.New("forced webhook normalization failure")
	}
	return hris.NormalizeSyncRequest(hris.NormalizedSyncRequest{
		TenantID:      receipt.TenantID,
		Source:        "hris_gadjian",
		Actor:         hris.SyncActor,
		RequestID:     "gadjian-dlq-replay-001",
		ConnectorID:   receipt.ConnectorID,
		RawPayloadRef: hris.RawPayloadRef(receipt),
		EventType:     "gadjian.employee.updated",
		Employees: []enterprise.EmployeeSyncInput{
			{
				ExternalID:       "GADJIAN-EMP-001",
				Email:            "dlq.replay@replay-sync.local",
				FullName:         "DLQ Replay User",
				Department:       "Operations",
				JobTitle:         "Ops Staff",
				Location:         "Jakarta",
				EmploymentStatus: "active",
				Status:           "active",
			},
		},
	})
}

type failNTimesGadjianNormalizer struct {
	failUntil int
	calls     int
}

func (n *failNTimesGadjianNormalizer) Vendor() string {
	return "gadjian"
}

func (n *failNTimesGadjianNormalizer) NormalizeWebhook(receipt enterprise.HRISWebhookReceipt) (hris.NormalizedSyncRequest, error) {
	n.calls++
	if n.calls <= n.failUntil {
		return hris.NormalizedSyncRequest{}, errors.New("forced webhook normalization failure")
	}
	return hris.NormalizeSyncRequest(hris.NormalizedSyncRequest{
		TenantID:      receipt.TenantID,
		Source:        "hris_gadjian",
		Actor:         hris.SyncActor,
		RequestID:     "gadjian-webhook-fail-n-times-001",
		ConnectorID:   receipt.ConnectorID,
		RawPayloadRef: hris.RawPayloadRef(receipt),
		EventType:     "gadjian.employee.updated",
		Employees: []enterprise.EmployeeSyncInput{
			{
				ExternalID:       "GADJIAN-EMP-FAIL-N-001",
				Email:            "fail.ntimes@replay-sync.local",
				FullName:         "Fail N Times User",
				Department:       "Operations",
				JobTitle:         "Ops Staff",
				Location:         "Jakarta",
				EmploymentStatus: "active",
				Status:           "active",
			},
		},
	})
}

func TestAppendEnterpriseHRISWebhookDLQFailureQueuesWakeSignal(t *testing.T) {
	s := &server{
		hrisDLQSvc:               hris.NewDLQService(),
		hrisWebhookDLQWorkerWake: make(chan struct{}, 1),
	}

	err := s.appendEnterpriseHRISWebhookDLQFailure(hris.DeadLetterFailureInput{
		TenantID:     "tenant_demo_jakarta",
		ConnectorID:  "connector-talenta",
		Vendor:       "talenta",
		ReceiptID:    "receipt-001",
		RequestID:    "request-001",
		EventType:    "talenta.employee.detail.created",
		FailureStage: "merge",
		Error:        "merge target missing",
	})
	if err != nil {
		t.Fatalf("append dlq failure should succeed: %v", err)
	}

	entries := s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", "connector-talenta", 10)
	if len(entries) != 1 {
		t.Fatalf("expected one dlq entry after append, got %d", len(entries))
	}
	select {
	case <-s.hrisWebhookDLQWorkerWake:
	default:
		t.Fatalf("expected dlq append to notify worker wake signal")
	}
}

func TestEnterpriseHRISWebhookDLQReplayFlow(t *testing.T) {
	normalizer := &flakyGadjianNormalizer{}
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
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

	body := `{"event_type":"employee.updated","employee":{"id":"GADJIAN-EMP-001"}}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "gadjian-webhook-001")
	request = withURLParam(request, "connectorID", connector.ID)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	processingAlertLogs := s.auditSvc.ListFiltered(
		"tenant_demo_jakarta",
		"enterprise_hris_webhook_processing_alert",
		"enterprise_sync_worker",
		10,
	)
	if len(processingAlertLogs) != 1 {
		t.Fatalf("expected one webhook processing alert log, got %d", len(processingAlertLogs))
	}
	if !strings.Contains(processingAlertLogs[0].Target, "failure_stage=normalize") ||
		!strings.Contains(processingAlertLogs[0].Target, "failed=1") {
		t.Fatalf("unexpected webhook processing alert payload: %s", processingAlertLogs[0].Target)
	}

	listRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-dlq?tenant_id=tenant_demo_jakarta&connector_id="+connector.ID,
		nil,
	)
	listRequest = withAuthUser(listRequest, auth.User{Role: "super_admin"})
	listRecorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookDLQ(listRecorder, listRequest)

	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from dlq list, got %d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var listPayload struct {
		Items []hris.DeadLetterEntry `json:"items"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("expected valid dlq list payload: %v body=%s", err, listRecorder.Body.String())
	}
	if len(listPayload.Items) != 1 {
		t.Fatalf("expected one dlq item, got %d", len(listPayload.Items))
	}
	if listPayload.Items[0].ReceiptID == "" {
		t.Fatalf("expected dlq entry receipt_id to be set")
	}
	if listPayload.Items[0].Status != "dlq" {
		t.Fatalf("expected dlq status, got %s", listPayload.Items[0].Status)
	}

	replayRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-dlq/"+listPayload.Items[0].ID+"/replay",
		nil,
	)
	replayRequest = withAuthUser(replayRequest, auth.User{Role: "super_admin"})
	replayRequest = withURLParam(replayRequest, "entryID", listPayload.Items[0].ID)
	replayRecorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookDLQ(replayRecorder, replayRequest)

	if replayRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from dlq replay, got %d body=%s", replayRecorder.Code, replayRecorder.Body.String())
	}
	var replayPayload struct {
		Item hris.DeadLetterEntry `json:"item"`
	}
	if err := json.Unmarshal(replayRecorder.Body.Bytes(), &replayPayload); err != nil {
		t.Fatalf("expected valid replay payload: %v body=%s", err, replayRecorder.Body.String())
	}
	if replayPayload.Item.Status != "resolved" {
		t.Fatalf("expected resolved dlq status, got %s", replayPayload.Item.Status)
	}
	if replayPayload.Item.ReplayCount != 1 {
		t.Fatalf("expected replay_count=1, got %d", replayPayload.Item.ReplayCount)
	}
	if replayPayload.Item.LastReplayAt == nil || replayPayload.Item.ResolvedAt == nil {
		t.Fatalf("expected replay success timestamps to be set, got %+v", replayPayload.Item)
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	foundEmployee := false
	for i := range employees {
		if employees[i].Email == "dlq.replay@replay-sync.local" {
			foundEmployee = true
			break
		}
	}
	if !foundEmployee {
		t.Fatalf("expected replayed employee to be synced")
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	foundAccessUser := false
	for i := range accessUsers {
		if accessUsers[i].Email == "dlq.replay@replay-sync.local" {
			foundAccessUser = true
			break
		}
	}
	if !foundAccessUser {
		t.Fatalf("expected replayed access user to be synced")
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replayed", "enterprise_sync", 10)
	if len(logs) == 0 {
		t.Fatalf("expected dlq replay audit log")
	}
}

func TestEnterpriseHRISWebhookDLQReplayQueuedFlow(t *testing.T) {
	normalizer := &failNTimesGadjianNormalizer{}
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
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

	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.updated",
			RequestID:  "gadjian-dlq-queued-001",
			RawPayload: `{"employee_id":"GADJIAN-EMP-FAIL-N-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued dlq receipt should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptDLQ("tenant_demo_jakarta", receipt.ID, errors.New("seed dlq for queued replay")); err != nil {
		t.Fatalf("mark queued dlq receipt should succeed: %v", err)
	}
	entry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   connector.ID,
		Vendor:        "gadjian",
		ReceiptID:     receipt.ID,
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		FailureStage:  "normalize",
		Error:         "seed queued replay",
		RawPayloadRef: hris.RawPayloadRef(receipt),
	})
	if err != nil {
		t.Fatalf("append queued dlq entry should succeed: %v", err)
	}

	replayRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-dlq/"+entry.ID+"/replay?execution_mode=queued",
		nil,
	)
	replayRequest = withAuthUser(replayRequest, auth.User{Role: "super_admin"})
	replayRequest = withURLParam(replayRequest, "entryID", entry.ID)
	replayRecorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookDLQ(replayRecorder, replayRequest)

	if replayRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from queued dlq replay, got %d body=%s", replayRecorder.Code, replayRecorder.Body.String())
	}
	var replayPayload struct {
		ExecutionMode string               `json:"execution_mode"`
		DispatchMode  string               `json:"dispatch_mode"`
		ExecutionID   string               `json:"execution_id"`
		Item          hris.DeadLetterEntry `json:"item"`
	}
	if err := json.Unmarshal(replayRecorder.Body.Bytes(), &replayPayload); err != nil {
		t.Fatalf("expected valid queued dlq replay payload: %v body=%s", err, replayRecorder.Body.String())
	}
	if replayPayload.ExecutionMode != "queued" ||
		replayPayload.DispatchMode != enterprise.HRISWebhookExecutionDispatchModeGoroutineFallback ||
		replayPayload.Item.ID != entry.ID ||
		replayPayload.Item.Status != "replaying" {
		t.Fatalf("unexpected queued dlq replay payload: %+v", replayPayload)
	}
	if replayPayload.ExecutionID == "" {
		t.Fatalf("expected queued dlq replay payload to include execution_id")
	}

	updatedEntry := waitForEnterpriseHRISWebhookDLQStatus(t, s, entry.ID, "resolved")
	if updatedEntry.ResolvedAt == nil {
		t.Fatalf("expected queued dlq replay to resolve asynchronously, got %+v", updatedEntry)
	}
	updatedReceipt, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("lookup queued replay receipt should succeed: %v", err)
	}
	if updatedReceipt.Status != "processed" {
		t.Fatalf("expected queued replay receipt processed, got %+v", updatedReceipt)
	}
	execution, err := s.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", replayPayload.ExecutionID)
	if err != nil {
		t.Fatalf("lookup queued dlq replay execution should succeed: %v", err)
	}
	if execution.Kind != enterprise.HRISWebhookExecutionKindDLQReplay ||
		execution.Status != enterprise.HRISWebhookExecutionStatusSucceeded ||
		execution.TargetStatus != "resolved" ||
		execution.DispatchMode != enterprise.HRISWebhookExecutionDispatchModeGoroutineFallback {
		t.Fatalf("unexpected queued dlq replay execution record: %+v", execution)
	}
	if execution.StartedAt == nil || execution.FinishedAt == nil {
		t.Fatalf("expected queued dlq replay execution timestamps to be set: %+v", execution)
	}

	queuedLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replay_queued", "enterprise_sync", 10)
	if len(queuedLogs) != 1 {
		t.Fatalf("expected one queued dlq replay audit log, got %d", len(queuedLogs))
	}
}

func TestEnterpriseHRISWebhookDLQReplayQueuedFlowRejectsRequireWorkerWithoutWorker(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
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
	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "talenta.employee.detail.created",
			RequestID:  "talenta-dlq-require-worker-001",
			RawPayload: `{"event_type":"talenta.employee.detail.created"}`,
		},
	)
	if err != nil {
		t.Fatalf("create receipt should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptDLQ("tenant_demo_jakarta", receipt.ID, errors.New("seed dlq")); err != nil {
		t.Fatalf("mark receipt dlq should succeed: %v", err)
	}
	entry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		ReceiptID:     receipt.ID,
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		FailureStage:  "normalize",
		Error:         "seed dlq replay require worker",
		RawPayloadRef: hris.RawPayloadRef(receipt),
	})
	if err != nil {
		t.Fatalf("append dlq entry should succeed: %v", err)
	}

	replayRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-dlq/"+entry.ID+"/replay?execution_mode=queued&require_worker=true",
		nil,
	)
	replayRequest = withAuthUser(replayRequest, auth.User{Role: "super_admin"})
	replayRequest = withURLParam(replayRequest, "entryID", entry.ID)
	replayRecorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookDLQ(replayRecorder, replayRequest)

	if replayRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 from queued dlq replay without worker, got %d body=%s", replayRecorder.Code, replayRecorder.Body.String())
	}
	if !strings.Contains(replayRecorder.Body.String(), errEnterpriseHRISWebhookQueuedDLQWorkerRequired.Error()) {
		t.Fatalf("expected require_worker conflict message, got body=%s", replayRecorder.Body.String())
	}

	updatedEntry, err := s.hrisDLQSvc.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("lookup dlq entry should succeed: %v", err)
	}
	if updatedEntry.Status != "dlq" || updatedEntry.ReplayCount != 0 {
		t.Fatalf("expected dlq entry to remain unreplayed after require_worker conflict, got %+v", updatedEntry)
	}
	if len(s.enterpriseSvc.ListAllHRISWebhookExecutions("tenant_demo_jakarta")) != 0 {
		t.Fatalf("expected no execution record after dlq require_worker conflict")
	}
}

func TestReplayEnterpriseHRISWebhookExecutionQueuedDLQFlow(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "execution-replay-dlq.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
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

	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "talenta-execution-dlq-replay-001",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-EXEC-DLQ-001",
						"employee_number":"TAL-EXEC-DLQ-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"Execution",
						"last_name":"DLQ",
						"email":"execution.dlq@execution-replay-dlq.local",
						"mobile_phone":"+628110000992"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create execution replay dlq receipt should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptDLQ("tenant_demo_jakarta", receipt.ID, errors.New("seed dlq receipt")); err != nil {
		t.Fatalf("mark execution replay receipt dlq should succeed: %v", err)
	}
	entry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		ReceiptID:     receipt.ID,
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		FailureStage:  "normalize",
		Error:         "seed dlq replay execution failure",
		RawPayloadRef: hris.RawPayloadRef(receipt),
	})
	if err != nil {
		t.Fatalf("append execution replay dlq entry should succeed: %v", err)
	}

	sourceExecution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindDLQReplay,
		TargetID:      entry.ID,
		ReceiptID:     receipt.ID,
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		FailureStage:  entry.FailureStage,
		AuditSource:   "enterprise_sync",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeGoroutineFallback,
		TargetStatus:  "dlq",
		RequestedBy:   "qa@example.com",
	})
	if err != nil {
		t.Fatalf("create source dlq execution should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", sourceExecution.ID); err != nil {
		t.Fatalf("mark source dlq execution running should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionFailed(
		"tenant_demo_jakarta",
		sourceExecution.ID,
		"dlq",
		errors.New("seed dlq execution failure"),
	); err != nil {
		t.Fatalf("mark source dlq execution failed should succeed: %v", err)
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"execution_mode": "queued",
	})
	if err != nil {
		t.Fatalf("marshal dlq execution replay request should succeed: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-executions/"+sourceExecution.ID+"/replay",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withURLParam(request, "executionID", sourceExecution.ID)
	request = withAuthUser(request, auth.User{
		Role:  "super_admin",
		Email: "qa@example.com",
	})
	recorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookExecution(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from queued dlq execution replay, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		SourceExecutionID string                          `json:"source_execution_id"`
		ExecutionMode     string                          `json:"execution_mode"`
		DispatchMode      string                          `json:"dispatch_mode"`
		ExecutionID       string                          `json:"execution_id"`
		Execution         enterprise.HRISWebhookExecution `json:"execution"`
		DLQItem           hris.DeadLetterEntry            `json:"dlq_item"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid queued dlq execution replay payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.SourceExecutionID != sourceExecution.ID ||
		payload.ExecutionMode != "queued" ||
		payload.DispatchMode != enterprise.HRISWebhookExecutionDispatchModeGoroutineFallback {
		t.Fatalf("unexpected queued dlq execution replay payload: %+v", payload)
	}
	if payload.ExecutionID == "" || payload.Execution.ID != payload.ExecutionID {
		t.Fatalf("expected queued dlq execution replay payload to include new execution: %+v", payload)
	}
	if payload.Execution.Kind != enterprise.HRISWebhookExecutionKindDLQReplay ||
		payload.Execution.AuditSource != enterpriseHRISWebhookExecutionReplayAuditSource {
		t.Fatalf("unexpected queued dlq replay execution metadata: %+v", payload.Execution)
	}
	if payload.Execution.ReplaySourceExecutionID != sourceExecution.ID ||
		payload.Execution.ReplayRequireWorker == nil ||
		*payload.Execution.ReplayRequireWorker {
		t.Fatalf("unexpected queued dlq replay execution audit metadata: %+v", payload.Execution)
	}
	if payload.DLQItem.ID != entry.ID || payload.DLQItem.Status != "replaying" {
		t.Fatalf("expected dlq replay to claim entry for replaying, got %+v", payload.DLQItem)
	}

	updatedEntry := waitForEnterpriseHRISWebhookDLQStatus(t, s, entry.ID, "resolved")
	if updatedEntry.ResolvedAt == nil {
		t.Fatalf("expected replayed dlq entry to resolve asynchronously, got %+v", updatedEntry)
	}
	updatedReceipt, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("lookup replayed dlq receipt should succeed: %v", err)
	}
	if updatedReceipt.Status != "processed" {
		t.Fatalf("expected replayed dlq receipt to be processed, got %+v", updatedReceipt)
	}
	newExecution := waitForEnterpriseHRISWebhookExecutionStatus(
		t,
		s,
		"tenant_demo_jakarta",
		payload.ExecutionID,
		enterprise.HRISWebhookExecutionStatusSucceeded,
	)
	if newExecution.Status != enterprise.HRISWebhookExecutionStatusSucceeded ||
		newExecution.TargetStatus != "resolved" ||
		newExecution.AuditSource != enterpriseHRISWebhookExecutionReplayAuditSource {
		t.Fatalf("unexpected replayed dlq execution record: %+v", newExecution)
	}
	if newExecution.ReplaySourceExecutionID != sourceExecution.ID ||
		newExecution.ReplayRequireWorker == nil ||
		*newExecution.ReplayRequireWorker {
		t.Fatalf("unexpected replayed dlq execution audit record: %+v", newExecution)
	}
	sourceAfterReplay, err := s.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", sourceExecution.ID)
	if err != nil {
		t.Fatalf("lookup source dlq execution after replay should succeed: %v", err)
	}
	if sourceAfterReplay.Status != enterprise.HRISWebhookExecutionStatusFailed {
		t.Fatalf("expected source dlq execution to remain failed history, got %+v", sourceAfterReplay)
	}

	logs := s.auditSvc.ListFiltered(
		"tenant_demo_jakarta",
		"enterprise_hris_webhook_execution_replayed",
		enterpriseHRISWebhookExecutionReplayAuditSource,
		10,
	)
	if len(logs) != 1 {
		t.Fatalf("expected one execution replay audit log for dlq flow, got %d", len(logs))
	}
}

func TestReplayEnterpriseHRISWebhookExecutionQueuedDLQFlowRejectsRequireWorkerWithoutWorker(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
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
	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "talenta.employee.detail.created",
			RequestID:  "talenta-execution-dlq-require-worker-001",
			RawPayload: `{"event_type":"talenta.employee.detail.created"}`,
		},
	)
	if err != nil {
		t.Fatalf("create receipt should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptDLQ("tenant_demo_jakarta", receipt.ID, errors.New("seed dlq receipt")); err != nil {
		t.Fatalf("mark receipt dlq should succeed: %v", err)
	}
	entry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		ReceiptID:     receipt.ID,
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		FailureStage:  "normalize",
		Error:         "seed execution replay dlq require worker",
		RawPayloadRef: hris.RawPayloadRef(receipt),
	})
	if err != nil {
		t.Fatalf("append dlq entry should succeed: %v", err)
	}
	sourceExecution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindDLQReplay,
		TargetID:      entry.ID,
		ReceiptID:     receipt.ID,
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		FailureStage:  entry.FailureStage,
		AuditSource:   "enterprise_sync",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeGoroutineFallback,
		TargetStatus:  "dlq",
	})
	if err != nil {
		t.Fatalf("create source execution should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", sourceExecution.ID); err != nil {
		t.Fatalf("mark source execution running should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionFailed(
		"tenant_demo_jakarta",
		sourceExecution.ID,
		"dlq",
		errors.New("seed dlq execution failure"),
	); err != nil {
		t.Fatalf("mark source execution failed should succeed: %v", err)
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"execution_mode": "queued",
		"require_worker": true,
	})
	if err != nil {
		t.Fatalf("marshal dlq execution replay request should succeed: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-executions/"+sourceExecution.ID+"/replay",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withURLParam(request, "executionID", sourceExecution.ID)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookExecution(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 from queued dlq execution replay without worker, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), errEnterpriseHRISWebhookQueuedDLQWorkerRequired.Error()) {
		t.Fatalf("expected require_worker conflict message, got body=%s", recorder.Body.String())
	}

	updatedEntry, err := s.hrisDLQSvc.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("lookup dlq entry should succeed: %v", err)
	}
	if updatedEntry.Status != "dlq" {
		t.Fatalf("expected dlq entry to remain terminal after replay conflict, got %+v", updatedEntry)
	}
	if len(s.enterpriseSvc.ListAllHRISWebhookExecutions("tenant_demo_jakarta")) != 1 {
		t.Fatalf("expected replay conflict to avoid creating a new execution")
	}
}

func TestReplayEnterpriseHRISWebhookExecutionQueuedDLQFlowRejectsDuplicateReplay(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookDLQWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
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
	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "talenta-execution-replay-dlq-duplicate-001",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-EXEC-DLQ-DUP-001",
						"employee_number":"TAL-EXEC-DLQ-DUP-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"Execution",
						"last_name":"DLQDuplicate",
						"email":"execution.dlq.duplicate@example.local",
						"mobile_phone":"+628110001202"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create duplicate dlq receipt should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptFailed("tenant_demo_jakarta", receipt.ID, errors.New("seed receipt failure")); err != nil {
		t.Fatalf("mark duplicate dlq receipt failed should succeed: %v", err)
	}
	entry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		ReceiptID:     receipt.ID,
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		FailureStage:  "normalize",
		Error:         "seed dlq failure",
		RawPayloadRef: "state://payloads/duplicate-dlq",
	})
	if err != nil {
		t.Fatalf("create duplicate dlq entry should succeed: %v", err)
	}

	sourceExecution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindDLQReplay,
		TargetID:      entry.ID,
		ReceiptID:     receipt.ID,
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		AuditSource:   "enterprise_sync",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "resolved",
		RequestedBy:   "qa@example.com",
	})
	if err != nil {
		t.Fatalf("create source dlq execution should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", sourceExecution.ID); err != nil {
		t.Fatalf("mark source dlq execution running should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionFailed(
		"tenant_demo_jakarta",
		sourceExecution.ID,
		"resolved",
		errors.New("seed execution failure"),
	); err != nil {
		t.Fatalf("mark source dlq execution failed should succeed: %v", err)
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"execution_mode": "queued",
		"require_worker": true,
	})
	if err != nil {
		t.Fatalf("marshal duplicate dlq replay request should succeed: %v", err)
	}

	firstRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-executions/"+sourceExecution.ID+"/replay",
		bytes.NewReader(requestBody),
	)
	firstRequest.Header.Set("Content-Type", "application/json")
	firstRequest = withURLParam(firstRequest, "executionID", sourceExecution.ID)
	firstRequest = withAuthUser(firstRequest, auth.User{
		Role:  "super_admin",
		Email: "qa@example.com",
	})
	firstRecorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookExecution(firstRecorder, firstRequest)

	if firstRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from first duplicate dlq replay request, got %d body=%s", firstRecorder.Code, firstRecorder.Body.String())
	}
	var firstPayload struct {
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(firstRecorder.Body.Bytes(), &firstPayload); err != nil {
		t.Fatalf("expected valid first duplicate dlq replay payload: %v body=%s", err, firstRecorder.Body.String())
	}
	if firstPayload.ExecutionID == "" {
		t.Fatalf("expected first duplicate dlq replay payload to include execution_id")
	}

	secondRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-executions/"+sourceExecution.ID+"/replay",
		bytes.NewReader(requestBody),
	)
	secondRequest.Header.Set("Content-Type", "application/json")
	secondRequest = withURLParam(secondRequest, "executionID", sourceExecution.ID)
	secondRequest = withAuthUser(secondRequest, auth.User{
		Role:  "super_admin",
		Email: "qa@example.com",
	})
	secondRecorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookExecution(secondRecorder, secondRequest)

	if secondRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 from duplicate dlq replay request, got %d body=%s", secondRecorder.Code, secondRecorder.Body.String())
	}
	var secondPayload struct {
		Error               string                          `json:"error"`
		ExistingExecutionID string                          `json:"existing_execution_id"`
		ExistingExecution   enterprise.HRISWebhookExecution `json:"existing_execution"`
	}
	if err := json.Unmarshal(secondRecorder.Body.Bytes(), &secondPayload); err != nil {
		t.Fatalf("expected valid duplicate dlq replay conflict payload: %v body=%s", err, secondRecorder.Body.String())
	}
	if secondPayload.ExistingExecutionID != firstPayload.ExecutionID ||
		secondPayload.ExistingExecution.ID != firstPayload.ExecutionID ||
		secondPayload.ExistingExecution.Status != enterprise.HRISWebhookExecutionStatusQueued {
		t.Fatalf("unexpected duplicate dlq replay conflict payload: %+v first=%s", secondPayload, firstPayload.ExecutionID)
	}
	if !strings.Contains(secondPayload.Error, "already queued or running") {
		t.Fatalf("unexpected duplicate dlq replay conflict error payload: %+v", secondPayload)
	}
	executions := s.enterpriseSvc.ListAllHRISWebhookExecutions("tenant_demo_jakarta")
	if len(executions) != 2 {
		t.Fatalf("expected duplicate dlq replay protection to keep one child execution, got %+v", executions)
	}
}

func TestEnterpriseHRISWebhookDLQReplayQueuedFlowRestoresClaimWhenExecutionCreateFails(t *testing.T) {
	store := &httpMemoryStateStore{}
	enterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create enterprise service with state store should succeed: %v", err)
	}
	dlqSvc, err := hris.NewDLQServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create dlq service with state store should succeed: %v", err)
	}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookDLQWorkerEnabled:           true,
			EnterpriseHRISWebhookDLQWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookDLQWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          enterpriseSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             dlqSvc,
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	_, err = s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "dlq-queue-restore.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
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

	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "dlq-queued-create-execution-fail",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-DLQ-QUEUED-FAIL-001"
					},
					"personal":{
						"first_name":"DLQ",
						"last_name":"Restore",
						"email":"dlq.restore@dlq-queue-restore.local"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued dlq receipt should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptDLQ("tenant_demo_jakarta", receipt.ID, errors.New("seed dlq before queued replay failure")); err != nil {
		t.Fatalf("mark queued dlq receipt should succeed: %v", err)
	}
	entry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		ReceiptID:     receipt.ID,
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		FailureStage:  "merge",
		Error:         "seed queued replay failure restore",
		RawPayloadRef: hris.RawPayloadRef(receipt),
	})
	if err != nil {
		t.Fatalf("append queued dlq entry should succeed: %v", err)
	}

	store.failCASCall = store.compareAndSwapCall + 2
	store.compareAndSwapErr = errors.New("forced queued dlq execution record create failure")

	replayRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-dlq/"+entry.ID+"/replay?execution_mode=queued",
		nil,
	)
	replayRequest = withAuthUser(replayRequest, auth.User{Role: "super_admin"})
	replayRequest = withURLParam(replayRequest, "entryID", entry.ID)
	replayRecorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookDLQ(replayRecorder, replayRequest)

	if replayRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from queued dlq replay when execution create fails, got %d body=%s", replayRecorder.Code, replayRecorder.Body.String())
	}

	updatedEntry, err := s.hrisDLQSvc.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("lookup restored dlq entry should succeed: %v", err)
	}
	if updatedEntry.Status != "dlq" || updatedEntry.ReplayCount != 0 || updatedEntry.LastReplayAt != nil || updatedEntry.ResolvedAt != nil {
		t.Fatalf("expected dlq claim to be restored after queued dispatch failure, got %+v", updatedEntry)
	}
	updatedReceipt, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("lookup dlq receipt should succeed: %v", err)
	}
	if updatedReceipt.Status != "dlq" {
		t.Fatalf("expected receipt to remain dlq after queued replay dispatch failure, got %+v", updatedReceipt)
	}
	if len(s.enterpriseSvc.ListAllHRISWebhookExecutions("tenant_demo_jakarta")) != 0 {
		t.Fatalf("expected no execution record to persist after queued dlq dispatch failure")
	}
	queuedLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replay_queued", "enterprise_sync", 10)
	if len(queuedLogs) != 0 {
		t.Fatalf("expected no queued dlq audit log after queued dispatch failure, got %d", len(queuedLogs))
	}
}

func TestEnterpriseHRISWebhookDLQReplayQueuedFlowPersistsForWorkerTickWhenEnabled(t *testing.T) {
	normalizer := &failNTimesGadjianNormalizer{}
	queueStore := &stubWorkerQueueStore{}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookDLQWorkerEnabled:           true,
			EnterpriseHRISWebhookDLQWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookDLQWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:            enterprise.NewService(),
		accessSvc:                access.NewService(),
		auditSvc:                 audit.NewService(),
		hrisDLQSvc:               hris.NewDLQService(),
		hrisNormalizerRegistry:   hris.NewRegistry(normalizer),
		hrisWebhookDLQWorkerWake: make(chan struct{}, 1),
		workerQueueStore:         queueStore,
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	connector, err := s.enterpriseSvc.CreateHRISConnector(
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

	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.updated",
			RequestID:  "gadjian-dlq-queued-worker-dispatch-001",
			RawPayload: `{"employee_id":"GADJIAN-EMP-FAIL-N-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued dlq receipt should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptDLQ("tenant_demo_jakarta", receipt.ID, errors.New("seed dlq for queued replay")); err != nil {
		t.Fatalf("mark queued dlq receipt should succeed: %v", err)
	}
	entry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   connector.ID,
		Vendor:        "gadjian",
		ReceiptID:     receipt.ID,
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		FailureStage:  "normalize",
		Error:         "seed queued replay worker dispatch",
		RawPayloadRef: hris.RawPayloadRef(receipt),
	})
	if err != nil {
		t.Fatalf("append queued dlq entry should succeed: %v", err)
	}

	replayRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-dlq/"+entry.ID+"/replay?execution_mode=queued",
		nil,
	)
	replayRequest = withAuthUser(replayRequest, auth.User{Role: "super_admin"})
	replayRequest = withURLParam(replayRequest, "entryID", entry.ID)
	replayRecorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookDLQ(replayRecorder, replayRequest)

	if replayRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from queued dlq replay, got %d body=%s", replayRecorder.Code, replayRecorder.Body.String())
	}

	select {
	case <-s.hrisWebhookDLQWorkerWake:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected queued dlq replay to notify worker wake")
	}

	var payload struct {
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(replayRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid queued dlq replay payload: %v body=%s", err, replayRecorder.Body.String())
	}
	if payload.ExecutionID == "" {
		t.Fatalf("expected queued dlq replay payload to include execution_id")
	}
	execution, err := s.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", payload.ExecutionID)
	if err != nil {
		t.Fatalf("lookup queued dlq execution should succeed: %v", err)
	}
	if execution.DispatchMode != enterprise.HRISWebhookExecutionDispatchModeWorkerTick ||
		execution.Status != enterprise.HRISWebhookExecutionStatusQueued ||
		execution.TargetStatus != "replaying" {
		t.Fatalf("unexpected queued dlq execution record: %+v", execution)
	}
	if queueStore.enqueueCalls != 1 {
		t.Fatalf("expected queued dlq replay dispatch to enqueue once, got %d", queueStore.enqueueCalls)
	}
	if queueStore.lastQueueName != enterpriseHRISWebhookDLQExecutionQueue {
		t.Fatalf("expected queued dlq execution queue %s, got %s", enterpriseHRISWebhookDLQExecutionQueue, queueStore.lastQueueName)
	}
	if len(queueStore.enqueuedIDs) != 1 || queueStore.enqueuedIDs[0] != payload.ExecutionID {
		t.Fatalf("expected queued dlq execution id %s to be enqueued, got %v", payload.ExecutionID, queueStore.enqueuedIDs)
	}

	updatedEntry, err := s.hrisDLQSvc.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("lookup queued dlq entry should succeed: %v", err)
	}
	if updatedEntry.Status != "replaying" || updatedEntry.ResolvedAt != nil {
		t.Fatalf("expected queued dlq entry to remain claimed but unresolved, got %+v", updatedEntry)
	}
	updatedReceipt, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("lookup queued replay receipt should succeed: %v", err)
	}
	if updatedReceipt.Status != "dlq" {
		t.Fatalf("expected queued replay receipt to remain dlq before worker drain, got %+v", updatedReceipt)
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount {
		t.Fatalf("expected queued dlq worker-tick path to avoid inline employee sync")
	}

	s.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(10, 3, 5*time.Minute, 15*time.Minute, 10*time.Minute, 1)

	resolvedEntry := waitForEnterpriseHRISWebhookDLQStatus(t, s, entry.ID, "resolved")
	if resolvedEntry.ResolvedAt == nil {
		t.Fatalf("expected worker tick to resolve queued dlq replay, got %+v", resolvedEntry)
	}
	execution, err = s.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", payload.ExecutionID)
	if err != nil {
		t.Fatalf("lookup queued dlq execution after worker tick should succeed: %v", err)
	}
	if execution.Status != enterprise.HRISWebhookExecutionStatusSucceeded || execution.TargetStatus != "resolved" {
		t.Fatalf("expected queued dlq execution to succeed after worker tick, got %+v", execution)
	}
	if queueStore.dequeueCalls == 0 {
		t.Fatalf("expected worker tick to dequeue queued dlq execution from external queue")
	}
}

func TestEnterpriseHRISWebhookDLQQueuedExecutionRefreshesSharedStateForRunningWorker(t *testing.T) {
	store := &httpMemoryStateStore{}
	firstEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create first enterprise service with state store should succeed: %v", err)
	}
	secondEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create second enterprise service with state store should succeed: %v", err)
	}
	firstDLQSvc, err := hris.NewDLQServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create first dlq service with state store should succeed: %v", err)
	}
	secondDLQSvc, err := hris.NewDLQServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create second dlq service with state store should succeed: %v", err)
	}

	firstServer := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookDLQWorkerEnabled:           true,
			EnterpriseHRISWebhookDLQWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookDLQWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          firstEnterpriseSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             firstDLQSvc,
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	secondServer := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookDLQWorkerEnabled:           true,
			EnterpriseHRISWebhookDLQWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookDLQWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          secondEnterpriseSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             secondDLQSvc,
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	_, err = firstServer.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "shared-refresh.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := firstServer.enterpriseSvc.CreateHRISConnector(
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

	receipt, err := firstServer.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "dlq-queued-shared-worker-001",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-DLQ-SHARED-001",
						"employee_number":"TAL-DLQ-SHARED-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"DLQ",
						"last_name":"Shared",
						"email":"dlq.shared@shared-refresh.local",
						"mobile_phone":"+628110000661"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued dlq receipt should succeed: %v", err)
	}

	entry, err := firstServer.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		ReceiptID:     receipt.ID,
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		FailureStage:  "merge",
		Error:         "forced dlq replay for shared worker refresh",
		RawPayloadRef: hris.RawPayloadRef(receipt),
	})
	if err != nil {
		t.Fatalf("append dlq failure should succeed: %v", err)
	}
	if _, err := firstServer.enterpriseSvc.MarkHRISWebhookReceiptDLQ(
		"tenant_demo_jakarta",
		receipt.ID,
		errors.New("forced dlq replay for shared worker refresh"),
	); err != nil {
		t.Fatalf("mark receipt dlq should succeed: %v", err)
	}

	if _, err := secondServer.hrisDLQSvc.GetEntry(entry.ID); !errors.Is(err, hris.ErrDLQEntryNotFound) {
		t.Fatalf("expected second server dlq view to be stale before refresh, got err=%v", err)
	}
	if len(secondServer.enterpriseSvc.ListAllHRISWebhookExecutions("")) != 0 {
		t.Fatalf("expected second server execution view to be empty before refresh")
	}

	replayRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-dlq/"+entry.ID+"/replay?execution_mode=queued",
		nil,
	)
	replayRequest = withAuthUser(replayRequest, auth.User{Role: "super_admin"})
	replayRequest = withURLParam(replayRequest, "entryID", entry.ID)
	replayRecorder := httptest.NewRecorder()

	firstServer.replayEnterpriseHRISWebhookDLQ(replayRecorder, replayRequest)

	if replayRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from queued dlq replay, got %d body=%s", replayRecorder.Code, replayRecorder.Body.String())
	}

	var queuedPayload struct {
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(replayRecorder.Body.Bytes(), &queuedPayload); err != nil {
		t.Fatalf("expected valid queued dlq replay payload: %v body=%s", err, replayRecorder.Body.String())
	}
	if queuedPayload.ExecutionID == "" {
		t.Fatalf("expected queued dlq replay payload to include execution_id")
	}

	if len(secondServer.enterpriseSvc.ListAllHRISWebhookExecutions("")) != 0 {
		t.Fatalf("expected second server execution view to remain stale before worker refresh")
	}

	secondServer.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(
		10,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		1,
	)

	resolvedEntry := waitForEnterpriseHRISWebhookDLQStatus(t, secondServer, entry.ID, "resolved")
	if resolvedEntry.ResolvedAt == nil {
		t.Fatalf("expected running worker on second server to resolve queued dlq replay after refresh, got %+v", resolvedEntry)
	}
	updatedReceipt, err := secondServer.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("lookup refreshed replay receipt should succeed: %v", err)
	}
	if updatedReceipt.Status != "processed" {
		t.Fatalf("expected refreshed replay receipt to be processed, got %+v", updatedReceipt)
	}
	execution, err := secondServer.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", queuedPayload.ExecutionID)
	if err != nil {
		t.Fatalf("lookup refreshed queued dlq execution should succeed: %v", err)
	}
	if execution.DispatchMode != enterprise.HRISWebhookExecutionDispatchModeWorkerTick ||
		execution.Status != enterprise.HRISWebhookExecutionStatusSucceeded ||
		execution.TargetStatus != "resolved" {
		t.Fatalf("unexpected refreshed queued dlq execution: %+v", execution)
	}
}

func TestEnterpriseHRISWebhookDLQQueuedExecutionRefreshesSharedStateAfterExternalQueueDequeueMiss(t *testing.T) {
	store := &httpMemoryStateStore{}
	firstEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create first enterprise service with state store should succeed: %v", err)
	}
	secondEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create second enterprise service with state store should succeed: %v", err)
	}
	firstDLQSvc, err := hris.NewDLQServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create first dlq service with state store should succeed: %v", err)
	}
	secondDLQSvc, err := hris.NewDLQServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create second dlq service with state store should succeed: %v", err)
	}
	queueStore := &stubWorkerQueueStore{}

	firstServer := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookDLQWorkerEnabled:           true,
			EnterpriseHRISWebhookDLQWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookDLQWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          firstEnterpriseSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             firstDLQSvc,
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
		workerQueueStore:       queueStore,
	}
	secondServer := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookDLQWorkerEnabled:           true,
			EnterpriseHRISWebhookDLQWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookDLQWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          secondEnterpriseSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             secondDLQSvc,
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
		workerQueueStore:       queueStore,
	}

	_, err = firstServer.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "queue-dlq-refresh.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := firstServer.enterpriseSvc.CreateHRISConnector(
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
	receipt, err := firstServer.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "dlq-queued-queue-refresh-001",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-DLQ-QUEUE-REFRESH-001",
						"employee_number":"TAL-DLQ-QUEUE-REFRESH-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"DLQ",
						"last_name":"QueueRefresh",
						"email":"dlq.queue.refresh@queue-dlq-refresh.local",
						"mobile_phone":"+628110000662"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued dlq receipt should succeed: %v", err)
	}
	entry, err := firstServer.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		ReceiptID:     receipt.ID,
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		FailureStage:  "merge",
		Error:         "forced dlq replay for queue refresh",
		RawPayloadRef: hris.RawPayloadRef(receipt),
	})
	if err != nil {
		t.Fatalf("append dlq failure should succeed: %v", err)
	}
	if _, err := firstServer.enterpriseSvc.MarkHRISWebhookReceiptDLQ(
		"tenant_demo_jakarta",
		receipt.ID,
		errors.New("forced dlq replay for queue refresh"),
	); err != nil {
		t.Fatalf("mark receipt dlq should succeed: %v", err)
	}
	if _, skipReason, err := firstServer.hrisDLQSvc.ClaimEntryForReplay(
		entry.ID,
		3,
		5*time.Minute,
		10*time.Minute,
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("claim queued dlq entry for replay should succeed: %v", err)
	} else if skipReason != "" {
		t.Fatalf("expected queued dlq entry replay claim without skip reason, got %s", skipReason)
	}

	if _, err := secondServer.hrisDLQSvc.GetEntry(entry.ID); !errors.Is(err, hris.ErrDLQEntryNotFound) {
		t.Fatalf("expected second server dlq view to be stale before refresh, got err=%v", err)
	}

	var queuedExecutionID string
	var enqueueOnce sync.Once
	queueStore.beforeDequeue = func(queueName string, batchSize int) {
		if queueName != enterpriseHRISWebhookDLQExecutionQueue {
			return
		}
		enqueueOnce.Do(func() {
			record, createErr := firstServer.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
				TenantID:      "tenant_demo_jakarta",
				Kind:          enterprise.HRISWebhookExecutionKindDLQReplay,
				TargetID:      entry.ID,
				ReceiptID:     receipt.ID,
				ConnectorID:   connector.ID,
				Vendor:        "talenta",
				RequestID:     receipt.RequestID,
				EventType:     receipt.EventType,
				FailureStage:  entry.FailureStage,
				AuditSource:   "enterprise_sync",
				ExecutionMode: "queued",
				DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
				TargetStatus:  "replaying",
			})
			if createErr != nil {
				t.Fatalf("create queued dlq execution during dequeue should succeed: %v", createErr)
			}
			record, createErr = firstServer.enterpriseSvc.RequeueHRISWebhookExecution(
				"tenant_demo_jakarta",
				record.ID,
				"replaying",
				time.Now().UTC().Add(-time.Second),
				nil,
			)
			if createErr != nil {
				t.Fatalf("backdate queued dlq execution during dequeue should succeed: %v", createErr)
			}
			queuedExecutionID = record.ID
			if enqueueErr := queueStore.EnqueueWorkerQueue(enterpriseHRISWebhookDLQExecutionQueue, record.ID); enqueueErr != nil {
				t.Fatalf("enqueue queued dlq execution during dequeue should succeed: %v", enqueueErr)
			}
		})
	}

	secondServer.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(
		10,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		1,
	)

	if queuedExecutionID == "" {
		t.Fatalf("expected dequeue hook to create a queued dlq execution")
	}
	if queueStore.dequeueCalls == 0 {
		t.Fatalf("expected worker tick to dequeue queued dlq execution from external queue")
	}
	resolvedEntry := waitForEnterpriseHRISWebhookDLQStatus(t, secondServer, entry.ID, "resolved")
	if resolvedEntry.ResolvedAt == nil {
		t.Fatalf("expected queue-miss refresh to resolve queued dlq replay, got %+v", resolvedEntry)
	}
	updatedReceipt, err := secondServer.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("lookup queue-miss refreshed replay receipt should succeed: %v", err)
	}
	if updatedReceipt.Status != "processed" {
		t.Fatalf("expected queue-miss refreshed replay receipt to be processed, got %+v", updatedReceipt)
	}
	execution, err := secondServer.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", queuedExecutionID)
	if err != nil {
		t.Fatalf("lookup queue-miss refreshed queued dlq execution should succeed: %v", err)
	}
	if execution.DispatchMode != enterprise.HRISWebhookExecutionDispatchModeWorkerTick ||
		execution.Status != enterprise.HRISWebhookExecutionStatusSucceeded ||
		execution.TargetStatus != "resolved" {
		t.Fatalf("unexpected queue-miss refreshed queued dlq execution: %+v", execution)
	}
}

func TestEnterpriseHRISWebhookDLQQueuedExecutionRecoversStaleRunningExecution(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookDLQWorkerEnabled:           true,
			EnterpriseHRISWebhookDLQWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookDLQWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerProcessingTimeout: time.Millisecond,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "stale-dlq-execution.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
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

	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "dlq-stale-execution-001",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-DLQ-STALE-EXECUTION-001",
						"employee_number":"TAL-DLQ-STALE-EXECUTION-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"DLQ",
						"last_name":"Execution",
						"email":"dlq.execution@stale-dlq-execution.local",
						"mobile_phone":"+628110000882"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued dlq receipt should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptDLQ("tenant_demo_jakarta", receipt.ID, errors.New("seed stale execution dlq")); err != nil {
		t.Fatalf("mark receipt dlq should succeed: %v", err)
	}
	entry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		ReceiptID:     receipt.ID,
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		FailureStage:  "merge",
		Error:         "seed stale execution replay",
		RawPayloadRef: hris.RawPayloadRef(receipt),
	})
	if err != nil {
		t.Fatalf("append dlq entry should succeed: %v", err)
	}

	replayRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-dlq/"+entry.ID+"/replay?execution_mode=queued",
		nil,
	)
	replayRequest = withAuthUser(replayRequest, auth.User{Role: "super_admin"})
	replayRequest = withURLParam(replayRequest, "entryID", entry.ID)
	replayRecorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookDLQ(replayRecorder, replayRequest)

	if replayRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from queued dlq replay, got %d body=%s", replayRecorder.Code, replayRecorder.Body.String())
	}

	var queuedPayload struct {
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(replayRecorder.Body.Bytes(), &queuedPayload); err != nil {
		t.Fatalf("expected valid queued dlq payload: %v body=%s", err, replayRecorder.Body.String())
	}
	if queuedPayload.ExecutionID == "" {
		t.Fatalf("expected queued dlq payload to include execution_id")
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", queuedPayload.ExecutionID); err != nil {
		t.Fatalf("mark queued dlq execution running should succeed: %v", err)
	}

	time.Sleep(5 * time.Millisecond)

	s.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(
		10,
		3,
		5*time.Minute,
		15*time.Minute,
		time.Millisecond,
		1,
	)

	updatedEntry := waitForEnterpriseHRISWebhookDLQStatus(t, s, entry.ID, "resolved")
	if updatedEntry.ResolvedAt == nil {
		t.Fatalf("expected stale running dlq execution recovery to resolve entry, got %+v", updatedEntry)
	}
	updatedReceipt, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("lookup stale recovered replay receipt should succeed: %v", err)
	}
	if updatedReceipt.Status != "processed" {
		t.Fatalf("expected stale running dlq execution recovery to mark receipt processed, got %+v", updatedReceipt)
	}
	execution, err := s.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", queuedPayload.ExecutionID)
	if err != nil {
		t.Fatalf("lookup stale recovered dlq execution should succeed: %v", err)
	}
	if execution.Status != enterprise.HRISWebhookExecutionStatusSucceeded || execution.TargetStatus != "resolved" {
		t.Fatalf("expected stale running dlq execution to recover to succeeded, got %+v", execution)
	}
}

func TestEnterpriseHRISWebhookDLQQueuedExecutionRequeuesFreshReplayingTarget(t *testing.T) {
	queueStore := &stubWorkerQueueStore{}
	s := &server{
		enterpriseSvc:    enterprise.NewService(),
		hrisDLQSvc:       hris.NewDLQService(),
		workerQueueStore: queueStore,
	}

	entry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   "connector-talenta-requeue",
		Vendor:        "talenta",
		ReceiptID:     "whr_dlq_requeue_001",
		RequestID:     "talenta-dlq-requeue-001",
		EventType:     "talenta.employee.detail.updated",
		FailureStage:  "merge",
		Error:         "seed fresh replaying target",
		RawPayloadRef: "hris_webhook_receipt:whr_dlq_requeue_001",
	})
	if err != nil {
		t.Fatalf("append dlq failure should succeed: %v", err)
	}
	execution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindDLQReplay,
		TargetID:      entry.ID,
		ReceiptID:     entry.ReceiptID,
		ConnectorID:   entry.ConnectorID,
		Vendor:        entry.Vendor,
		RequestID:     entry.RequestID,
		EventType:     entry.EventType,
		FailureStage:  entry.FailureStage,
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "replaying",
	})
	if err != nil {
		t.Fatalf("create queued dlq execution should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", execution.ID); err != nil {
		t.Fatalf("mark queued dlq execution running should succeed: %v", err)
	}

	time.Sleep(80 * time.Millisecond)

	replayingEntry, skipReason, err := s.hrisDLQSvc.ClaimEntryForReplay(
		entry.ID,
		3,
		5*time.Minute,
		50*time.Millisecond,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("claim dlq entry for replay should succeed: %v", err)
	}
	if skipReason != "" {
		t.Fatalf("expected replaying claim without skip reason, got %s", skipReason)
	}
	if replayingEntry.LastReplayAt == nil {
		t.Fatalf("expected replaying entry to set last_replay_at")
	}

	processed := s.runQueuedEnterpriseHRISWebhookDLQExecutionsWithRetryBackoffAndProcessingTimeout(
		10,
		3,
		5*time.Minute,
		15*time.Minute,
		50*time.Millisecond,
	)
	if processed != 1 {
		t.Fatalf("expected one queued dlq execution to be requeued, got %d", processed)
	}

	updatedExecution, err := s.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", execution.ID)
	if err != nil {
		t.Fatalf("lookup requeued dlq execution should succeed: %v", err)
	}
	expectedRetryAt := replayingEntry.LastReplayAt.Add(50 * time.Millisecond)
	if updatedExecution.Status != enterprise.HRISWebhookExecutionStatusQueued {
		t.Fatalf("expected dlq execution to be requeued, got %+v", updatedExecution)
	}
	if updatedExecution.TargetStatus != "replaying" {
		t.Fatalf("expected requeued dlq execution target_status=replaying, got %+v", updatedExecution)
	}
	if !updatedExecution.QueuedAt.Equal(expectedRetryAt) {
		t.Fatalf("expected requeued dlq execution queued_at=%s, got %s", expectedRetryAt, updatedExecution.QueuedAt)
	}
	if queueStore.enqueueCalls != 1 {
		t.Fatalf("expected requeued dlq execution to be re-enqueued once, got %d", queueStore.enqueueCalls)
	}
	items := queueStore.itemsByQueue[enterpriseHRISWebhookDLQExecutionQueue]
	if len(items) != 1 || items[0] != execution.ID {
		t.Fatalf("expected requeued dlq execution to return to external queue, got %v", items)
	}
	claimed, reason, err := s.enterpriseSvc.ClaimHRISWebhookExecution(
		"tenant_demo_jakarta",
		execution.ID,
		50*time.Millisecond,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("claim requeued dlq execution should not error: %v", err)
	}
	if reason != enterprise.HRISWebhookExecutionClaimReasonCooldown {
		t.Fatalf("expected cooldown reason for requeued dlq execution, got %s item=%+v", reason, claimed)
	}
}

func TestListEnterpriseHRISWebhookDLQRefreshesSharedState(t *testing.T) {
	store := &httpMemoryStateStore{}
	firstDLQSvc, err := hris.NewDLQServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create first dlq service with state store should succeed: %v", err)
	}
	secondDLQSvc, err := hris.NewDLQServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create second dlq service with state store should succeed: %v", err)
	}

	entry, err := firstDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   "connector-talenta-refresh",
		Vendor:        "talenta",
		ReceiptID:     "whr_dlq_refresh_001",
		RequestID:     "talenta-dlq-refresh-001",
		EventType:     "talenta.employee.detail.updated",
		FailureStage:  "merge",
		Error:         "refresh this dlq list entry",
		RawPayloadRef: "hris_webhook_receipt:whr_dlq_refresh_001",
	})
	if err != nil {
		t.Fatalf("append dlq failure should succeed: %v", err)
	}
	if len(secondDLQSvc.ListEntries("tenant_demo_jakarta", "connector-talenta-refresh", 10)) != 0 {
		t.Fatalf("expected second dlq service view to be stale before list refresh")
	}

	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookDLQWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookDLQWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerProcessingTimeout: 10 * time.Minute,
		},
		hrisDLQSvc: secondDLQSvc,
	}
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-dlq?tenant_id=tenant_demo_jakarta&connector_id=connector-talenta-refresh",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookDLQ(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from dlq list, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload hrisWebhookDLQListResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid dlq list payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != entry.ID {
		t.Fatalf("expected refreshed dlq list to include latest entry, got %+v", payload.Items)
	}
}

func TestRunQueuedEnterpriseHRISWebhookDLQExecutionsReenqueuesCooldownExternalQueueCandidate(t *testing.T) {
	queueStore := &stubWorkerQueueStore{}
	s := &server{
		enterpriseSvc:    enterprise.NewService(),
		hrisDLQSvc:       hris.NewDLQService(),
		workerQueueStore: queueStore,
	}

	execution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindDLQReplay,
		TargetID:      "dlq_exec_cooldown_queue_001",
		ReceiptID:     "whr_exec_cooldown_queue_001",
		ConnectorID:   "connector-talenta-dlq-cooldown-queue",
		Vendor:        "talenta",
		RequestID:     "talenta-dlq-exec-cooldown-queue-001",
		EventType:     "talenta.employee.detail.updated",
		FailureStage:  "normalize",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "replaying",
	})
	if err != nil {
		t.Fatalf("create queued dlq execution should succeed: %v", err)
	}

	retryAt := time.Now().UTC().Add(5 * time.Minute)
	if _, err := s.enterpriseSvc.RequeueHRISWebhookExecution(
		"tenant_demo_jakarta",
		execution.ID,
		"replaying",
		retryAt,
		nil,
	); err != nil {
		t.Fatalf("requeue queued dlq execution should succeed: %v", err)
	}
	if err := queueStore.EnqueueWorkerQueue(enterpriseHRISWebhookDLQExecutionQueue, execution.ID); err != nil {
		t.Fatalf("seed dlq execution external queue should succeed: %v", err)
	}

	processed := s.runQueuedEnterpriseHRISWebhookDLQExecutionsWithRetryBackoffAndProcessingTimeout(
		10,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
	)
	if processed != 0 {
		t.Fatalf("expected cooldown dlq execution to remain deferred, got processed=%d", processed)
	}
	if queueStore.dequeueCalls != 1 {
		t.Fatalf("expected one external queue dequeue call, got %d", queueStore.dequeueCalls)
	}
	if queueStore.enqueueCalls != 1 {
		t.Fatalf("expected one seed enqueue call, got %d", queueStore.enqueueCalls)
	}
	if queueStore.requeueCalls != 1 {
		t.Fatalf("expected cooldown dlq execution to be requeued once, got %d requeue calls", queueStore.requeueCalls)
	}
	items := queueStore.itemsByQueue[enterpriseHRISWebhookDLQExecutionQueue]
	if len(items) != 1 || items[0] != execution.ID {
		t.Fatalf("expected cooldown dlq execution to remain in external queue, got %v", items)
	}

	updatedExecution, err := s.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", execution.ID)
	if err != nil {
		t.Fatalf("lookup cooldown dlq execution should succeed: %v", err)
	}
	if updatedExecution.Status != enterprise.HRISWebhookExecutionStatusQueued {
		t.Fatalf("expected cooldown dlq execution to stay queued, got %+v", updatedExecution)
	}
	if !updatedExecution.QueuedAt.Equal(retryAt) {
		t.Fatalf("expected cooldown dlq execution queued_at=%s, got %s", retryAt, updatedExecution.QueuedAt)
	}
}

func TestEnterpriseHRISWebhookDLQBatchReplayFlow(t *testing.T) {
	normalizer := &flakyGadjianNormalizer{}
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
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

	body := `{"event_type":"employee.updated","employee":{"id":"GADJIAN-EMP-001"}}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "gadjian-webhook-batch-001")
	request = withURLParam(request, "connectorID", connector.ID)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	listRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-dlq?tenant_id=tenant_demo_jakarta&connector_id="+connector.ID,
		nil,
	)
	listRequest = withAuthUser(listRequest, auth.User{Role: "super_admin"})
	listRecorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookDLQ(listRecorder, listRequest)

	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from dlq list, got %d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var listPayload struct {
		Items []hris.DeadLetterEntry `json:"items"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("expected valid dlq list payload: %v body=%s", err, listRecorder.Body.String())
	}
	if len(listPayload.Items) != 1 {
		t.Fatalf("expected one dlq item, got %d", len(listPayload.Items))
	}

	skippedEntry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:     "tenant_demo_jakarta",
		ConnectorID:  connector.ID,
		Vendor:       "gadjian",
		ReceiptID:    "receipt-skip-batch-001",
		RequestID:    "skip-batch-001",
		EventType:    "gadjian.employee.updated",
		FailureStage: "normalize",
		Error:        "skip replay because it is already in flight",
	})
	if err != nil {
		t.Fatalf("append skipped batch dlq entry should succeed: %v", err)
	}
	if _, _, err := s.hrisDLQSvc.ClaimEntryForReplay(skippedEntry.ID, 0, 0, 5*time.Minute, time.Now().UTC()); err != nil {
		t.Fatalf("claim skipped batch dlq entry should succeed: %v", err)
	}

	batchBody, err := json.Marshal(map[string]any{
		"tenant_id": "tenant_demo_jakarta",
		"entry_ids": []string{listPayload.Items[0].ID, skippedEntry.ID},
	})
	if err != nil {
		t.Fatalf("marshal batch replay request should succeed: %v", err)
	}
	batchRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-dlq/replay-batch",
		bytes.NewReader(batchBody),
	)
	batchRequest.Header.Set("Content-Type", "application/json")
	batchRequest = withAuthUser(batchRequest, auth.User{Role: "super_admin"})
	batchRecorder := httptest.NewRecorder()

	s.replayBatchEnterpriseHRISWebhookDLQ(batchRecorder, batchRequest)

	if batchRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from batch dlq replay, got %d body=%s", batchRecorder.Code, batchRecorder.Body.String())
	}
	var batchPayload struct {
		TenantID     string `json:"tenant_id"`
		TotalEntries int    `json:"total_entries"`
		Replayed     int    `json:"replayed"`
		Skipped      int    `json:"skipped"`
		Failed       int    `json:"failed"`
		Items        []struct {
			EntryID string                `json:"entry_id"`
			Status  string                `json:"status"`
			Reason  string                `json:"reason"`
			Item    *hris.DeadLetterEntry `json:"item"`
		} `json:"items"`
	}
	if err := json.Unmarshal(batchRecorder.Body.Bytes(), &batchPayload); err != nil {
		t.Fatalf("expected valid batch replay payload: %v body=%s", err, batchRecorder.Body.String())
	}
	if batchPayload.TenantID != "tenant_demo_jakarta" {
		t.Fatalf("unexpected tenant_id in batch replay payload: %+v", batchPayload)
	}
	if batchPayload.TotalEntries != 2 || batchPayload.Replayed != 1 || batchPayload.Skipped != 1 || batchPayload.Failed != 0 {
		t.Fatalf("unexpected batch replay summary: %+v", batchPayload)
	}
	if len(batchPayload.Items) != 2 {
		t.Fatalf("expected two batch replay items, got %d", len(batchPayload.Items))
	}

	statusByEntryID := make(map[string]struct {
		Status string
		Reason string
		Item   *hris.DeadLetterEntry
	})
	for i := range batchPayload.Items {
		statusByEntryID[batchPayload.Items[i].EntryID] = struct {
			Status string
			Reason string
			Item   *hris.DeadLetterEntry
		}{
			Status: batchPayload.Items[i].Status,
			Reason: batchPayload.Items[i].Reason,
			Item:   batchPayload.Items[i].Item,
		}
	}

	successItem, ok := statusByEntryID[listPayload.Items[0].ID]
	if !ok || successItem.Status != "replayed" || successItem.Item == nil || successItem.Item.Status != "resolved" {
		t.Fatalf("expected successful batch replay result for %s, got %+v", listPayload.Items[0].ID, successItem)
	}
	skippedItem, ok := statusByEntryID[skippedEntry.ID]
	if !ok || skippedItem.Status != "skipped" || skippedItem.Reason != hris.DLQEntryClaimReasonInFlight {
		t.Fatalf("expected skipped in-flight batch replay result for %s, got %+v", skippedEntry.ID, skippedItem)
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	foundEmployee := false
	for i := range employees {
		if employees[i].Email == "dlq.replay@replay-sync.local" {
			foundEmployee = true
			break
		}
	}
	if !foundEmployee {
		t.Fatalf("expected batch replayed employee to be synced")
	}
}

func TestEnterpriseHRISWebhookDLQBatchReplayQueuedFlow(t *testing.T) {
	normalizer := &failNTimesGadjianNormalizer{}
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
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

	receiptOne, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.updated",
			RequestID:  "gadjian-dlq-batch-queued-001",
			RawPayload: `{"employee_id":"GADJIAN-EMP-FAIL-N-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create first queued dlq receipt should succeed: %v", err)
	}
	receiptTwo, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.updated",
			RequestID:  "gadjian-dlq-batch-queued-002",
			RawPayload: `{"employee_id":"GADJIAN-EMP-FAIL-N-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create second queued dlq receipt should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptDLQ("tenant_demo_jakarta", receiptOne.ID, errors.New("seed batch queued replay one")); err != nil {
		t.Fatalf("mark first queued dlq receipt should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptDLQ("tenant_demo_jakarta", receiptTwo.ID, errors.New("seed batch queued replay two")); err != nil {
		t.Fatalf("mark second queued dlq receipt should succeed: %v", err)
	}

	entryOne, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   connector.ID,
		Vendor:        "gadjian",
		ReceiptID:     receiptOne.ID,
		RequestID:     receiptOne.RequestID,
		EventType:     receiptOne.EventType,
		FailureStage:  "normalize",
		Error:         "seed batch queued replay one",
		RawPayloadRef: hris.RawPayloadRef(receiptOne),
	})
	if err != nil {
		t.Fatalf("append first queued dlq entry should succeed: %v", err)
	}
	entryTwo, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   connector.ID,
		Vendor:        "gadjian",
		ReceiptID:     receiptTwo.ID,
		RequestID:     receiptTwo.RequestID,
		EventType:     receiptTwo.EventType,
		FailureStage:  "normalize",
		Error:         "seed batch queued replay two",
		RawPayloadRef: hris.RawPayloadRef(receiptTwo),
	})
	if err != nil {
		t.Fatalf("append second queued dlq entry should succeed: %v", err)
	}

	batchBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"entry_ids":      []string{entryOne.ID, entryTwo.ID},
		"execution_mode": "queued",
	})
	if err != nil {
		t.Fatalf("marshal queued batch replay request should succeed: %v", err)
	}
	batchRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-dlq/replay-batch",
		bytes.NewReader(batchBody),
	)
	batchRequest.Header.Set("Content-Type", "application/json")
	batchRequest = withAuthUser(batchRequest, auth.User{Role: "super_admin"})
	batchRecorder := httptest.NewRecorder()

	s.replayBatchEnterpriseHRISWebhookDLQ(batchRecorder, batchRequest)

	if batchRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from queued batch dlq replay, got %d body=%s", batchRecorder.Code, batchRecorder.Body.String())
	}
	var batchPayload struct {
		ExecutionMode string `json:"execution_mode"`
		DispatchMode  string `json:"dispatch_mode"`
		Queued        int    `json:"queued"`
		Replayed      int    `json:"replayed"`
		Items         []struct {
			EntryID string                `json:"entry_id"`
			Status  string                `json:"status"`
			Item    *hris.DeadLetterEntry `json:"item"`
		} `json:"items"`
	}
	if err := json.Unmarshal(batchRecorder.Body.Bytes(), &batchPayload); err != nil {
		t.Fatalf("expected valid queued batch replay payload: %v body=%s", err, batchRecorder.Body.String())
	}
	if batchPayload.ExecutionMode != "queued" ||
		batchPayload.DispatchMode != enterprise.HRISWebhookExecutionDispatchModeGoroutineFallback ||
		batchPayload.Queued != 2 ||
		batchPayload.Replayed != 0 {
		t.Fatalf("unexpected queued batch replay summary: %+v", batchPayload)
	}
	if len(batchPayload.Items) != 2 {
		t.Fatalf("expected two queued batch replay items, got %d", len(batchPayload.Items))
	}
	for i := range batchPayload.Items {
		if batchPayload.Items[i].Status != "queued" || batchPayload.Items[i].Item == nil || batchPayload.Items[i].Item.Status != "replaying" {
			t.Fatalf("expected queued dlq batch item, got %+v", batchPayload.Items[i])
		}
	}

	waitForEnterpriseHRISWebhookDLQStatus(t, s, entryOne.ID, "resolved")
	waitForEnterpriseHRISWebhookDLQStatus(t, s, entryTwo.ID, "resolved")

	queuedLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replay_queued", "enterprise_sync_batch", 10)
	if len(queuedLogs) != 2 {
		t.Fatalf("expected two queued batch dlq replay audit logs, got %d", len(queuedLogs))
	}
}

func TestEnterpriseHRISWebhookDLQBatchReplayQueuedFlowRejectsRequireWorkerWithoutWorker(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
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
	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "talenta.employee.detail.created",
			RequestID:  "talenta-dlq-batch-require-worker-001",
			RawPayload: `{"event_type":"talenta.employee.detail.created"}`,
		},
	)
	if err != nil {
		t.Fatalf("create receipt should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptDLQ("tenant_demo_jakarta", receipt.ID, errors.New("seed batch dlq")); err != nil {
		t.Fatalf("mark receipt dlq should succeed: %v", err)
	}
	entry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		ReceiptID:     receipt.ID,
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		FailureStage:  "normalize",
		Error:         "seed batch dlq require worker",
		RawPayloadRef: hris.RawPayloadRef(receipt),
	})
	if err != nil {
		t.Fatalf("append dlq entry should succeed: %v", err)
	}

	batchBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"entry_ids":      []string{entry.ID},
		"execution_mode": "queued",
		"require_worker": true,
	})
	if err != nil {
		t.Fatalf("marshal queued batch replay request should succeed: %v", err)
	}
	batchRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-dlq/replay-batch",
		bytes.NewReader(batchBody),
	)
	batchRequest.Header.Set("Content-Type", "application/json")
	batchRequest = withAuthUser(batchRequest, auth.User{Role: "super_admin"})
	batchRecorder := httptest.NewRecorder()

	s.replayBatchEnterpriseHRISWebhookDLQ(batchRecorder, batchRequest)

	if batchRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 from queued batch dlq replay without worker, got %d body=%s", batchRecorder.Code, batchRecorder.Body.String())
	}
	if !strings.Contains(batchRecorder.Body.String(), errEnterpriseHRISWebhookQueuedDLQWorkerRequired.Error()) {
		t.Fatalf("expected require_worker conflict message, got body=%s", batchRecorder.Body.String())
	}

	updatedEntry, err := s.hrisDLQSvc.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("lookup dlq entry should succeed: %v", err)
	}
	if updatedEntry.Status != "dlq" || updatedEntry.ReplayCount != 0 {
		t.Fatalf("expected batch dlq entry to remain untouched after require_worker conflict, got %+v", updatedEntry)
	}
	if len(s.enterpriseSvc.ListAllHRISWebhookExecutions("tenant_demo_jakarta")) != 0 {
		t.Fatalf("expected no execution record after batch dlq require_worker conflict")
	}
}

func TestListEnterpriseHRISWebhookDLQIncludesReplayRuntimeFields(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookDLQWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookDLQWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerProcessingTimeout: 10 * time.Minute,
		},
		hrisDLQSvc: hris.NewDLQService(),
	}

	readyEntry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:     "tenant_demo_jakarta",
		ConnectorID:  "connector-gadjian",
		Vendor:       "gadjian",
		ReceiptID:    "receipt-ready",
		RequestID:    "dlq-ready",
		EventType:    "gadjian.employee.updated",
		FailureStage: "normalize",
		Error:        "ready failure",
	})
	if err != nil {
		t.Fatalf("append ready dlq entry should succeed: %v", err)
	}

	cooldownEntry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:     "tenant_demo_jakarta",
		ConnectorID:  "connector-gadjian",
		Vendor:       "gadjian",
		ReceiptID:    "receipt-cooldown",
		RequestID:    "dlq-cooldown",
		EventType:    "gadjian.employee.updated",
		FailureStage: "normalize",
		Error:        "cooldown failure",
	})
	if err != nil {
		t.Fatalf("append cooldown dlq entry should succeed: %v", err)
	}
	cooldownClaimedAt := time.Now().UTC().Add(-2 * time.Minute)
	cooldownClaimed, reason, err := s.hrisDLQSvc.ClaimEntryForReplayWithBackoff(
		cooldownEntry.ID,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		cooldownClaimedAt,
	)
	if err != nil {
		t.Fatalf("claim cooldown dlq entry should succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected empty claim skip reason for cooldown dlq entry, got %q", reason)
	}
	if _, err := s.hrisDLQSvc.MarkReplayFailed(cooldownEntry.ID, errors.New("cooldown replay failed")); err != nil {
		t.Fatalf("mark replay failed should succeed: %v", err)
	}

	processingEntry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:     "tenant_demo_jakarta",
		ConnectorID:  "connector-gadjian",
		Vendor:       "gadjian",
		ReceiptID:    "receipt-processing",
		RequestID:    "dlq-processing",
		EventType:    "gadjian.employee.updated",
		FailureStage: "sync",
		Error:        "processing failure",
	})
	if err != nil {
		t.Fatalf("append processing dlq entry should succeed: %v", err)
	}
	processingClaimedAt := time.Now().UTC().Add(-3 * time.Minute)
	processingClaimed, reason, err := s.hrisDLQSvc.ClaimEntryForReplayWithBackoff(
		processingEntry.ID,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		processingClaimedAt,
	)
	if err != nil {
		t.Fatalf("claim processing dlq entry should succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected empty claim skip reason for processing dlq entry, got %q", reason)
	}

	staleEntry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:     "tenant_demo_jakarta",
		ConnectorID:  "connector-gadjian",
		Vendor:       "gadjian",
		ReceiptID:    "receipt-stale",
		RequestID:    "dlq-stale-processing",
		EventType:    "gadjian.employee.updated",
		FailureStage: "sync",
		Error:        "stale processing failure",
	})
	if err != nil {
		t.Fatalf("append stale dlq entry should succeed: %v", err)
	}
	staleClaimedAt := time.Now().UTC().Add(-20 * time.Minute)
	staleClaimed, reason, err := s.hrisDLQSvc.ClaimEntryForReplayWithBackoff(
		staleEntry.ID,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		staleClaimedAt,
	)
	if err != nil {
		t.Fatalf("claim stale dlq entry should succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected empty claim skip reason for stale dlq entry, got %q", reason)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-dlq?tenant_id=tenant_demo_jakarta&connector_id=connector-gadjian",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookDLQ(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Items []struct {
			ID                   string     `json:"id"`
			RequestID            string     `json:"request_id"`
			Status               string     `json:"status"`
			ReplayState          string     `json:"replay_state"`
			ReplayCount          int        `json:"replay_count"`
			RemainingAttempts    int        `json:"remaining_attempts"`
			CooldownRemainingSec int64      `json:"cooldown_remaining_seconds"`
			StaleInFlight        bool       `json:"stale_in_flight"`
			NextRetryAt          *time.Time `json:"next_retry_at,omitempty"`
			ProcessingDeadlineAt *time.Time `json:"processing_deadline_at,omitempty"`
		} `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid dlq list payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 4 {
		t.Fatalf("expected four dlq items, got %d", len(payload.Items))
	}

	itemsByRequestID := map[string]struct {
		Status               string
		ReplayState          string
		ReplayCount          int
		RemainingAttempts    int
		CooldownRemainingSec int64
		StaleInFlight        bool
		NextRetryAt          *time.Time
		ProcessingDeadlineAt *time.Time
	}{}
	for i := range payload.Items {
		itemsByRequestID[payload.Items[i].RequestID] = struct {
			Status               string
			ReplayState          string
			ReplayCount          int
			RemainingAttempts    int
			CooldownRemainingSec int64
			StaleInFlight        bool
			NextRetryAt          *time.Time
			ProcessingDeadlineAt *time.Time
		}{
			Status:               payload.Items[i].Status,
			ReplayState:          payload.Items[i].ReplayState,
			ReplayCount:          payload.Items[i].ReplayCount,
			RemainingAttempts:    payload.Items[i].RemainingAttempts,
			CooldownRemainingSec: payload.Items[i].CooldownRemainingSec,
			StaleInFlight:        payload.Items[i].StaleInFlight,
			NextRetryAt:          payload.Items[i].NextRetryAt,
			ProcessingDeadlineAt: payload.Items[i].ProcessingDeadlineAt,
		}
	}

	readyItem, ok := itemsByRequestID[readyEntry.RequestID]
	if !ok {
		t.Fatalf("expected ready dlq entry to be listed")
	}
	if readyItem.ReplayState != "ready" ||
		readyItem.RemainingAttempts != 3 ||
		readyItem.CooldownRemainingSec != 0 ||
		readyItem.StaleInFlight ||
		readyItem.NextRetryAt != nil ||
		readyItem.ProcessingDeadlineAt != nil {
		t.Fatalf("unexpected ready dlq runtime fields: %+v", readyItem)
	}

	cooldownItem, ok := itemsByRequestID[cooldownEntry.RequestID]
	if !ok {
		t.Fatalf("expected cooldown dlq entry to be listed")
	}
	expectedNextRetryAt := cooldownClaimed.LastReplayAt.Add(5 * time.Minute)
	if cooldownItem.Status != "dlq" ||
		cooldownItem.ReplayState != hris.DLQEntryClaimReasonCooldown ||
		cooldownItem.ReplayCount != 1 ||
		cooldownItem.RemainingAttempts != 2 ||
		cooldownItem.CooldownRemainingSec != 180 ||
		cooldownItem.StaleInFlight ||
		cooldownItem.NextRetryAt == nil ||
		!cooldownItem.NextRetryAt.Equal(expectedNextRetryAt) ||
		cooldownItem.ProcessingDeadlineAt != nil {
		t.Fatalf("unexpected cooldown dlq runtime fields: %+v expected_next_retry_at=%s", cooldownItem, expectedNextRetryAt.Format(time.RFC3339))
	}

	processingItem, ok := itemsByRequestID[processingEntry.RequestID]
	if !ok {
		t.Fatalf("expected processing dlq entry to be listed")
	}
	expectedProcessingDeadline := processingClaimed.LastReplayAt.Add(10 * time.Minute)
	if processingItem.Status != "replaying" ||
		processingItem.ReplayState != hris.DLQEntryClaimReasonInFlight ||
		processingItem.ReplayCount != 1 ||
		processingItem.RemainingAttempts != 2 ||
		processingItem.CooldownRemainingSec != 0 ||
		processingItem.StaleInFlight ||
		processingItem.NextRetryAt != nil ||
		processingItem.ProcessingDeadlineAt == nil ||
		!processingItem.ProcessingDeadlineAt.Equal(expectedProcessingDeadline) {
		t.Fatalf("unexpected processing dlq runtime fields: %+v expected_deadline=%s", processingItem, expectedProcessingDeadline.Format(time.RFC3339))
	}

	staleItem, ok := itemsByRequestID[staleEntry.RequestID]
	if !ok {
		t.Fatalf("expected stale dlq entry to be listed")
	}
	expectedStaleDeadline := staleClaimed.LastReplayAt.Add(10 * time.Minute)
	if staleItem.Status != "replaying" ||
		staleItem.ReplayState != "ready" ||
		staleItem.ReplayCount != 1 ||
		staleItem.RemainingAttempts != 2 ||
		staleItem.CooldownRemainingSec != 0 ||
		!staleItem.StaleInFlight ||
		staleItem.NextRetryAt != nil ||
		staleItem.ProcessingDeadlineAt == nil ||
		!staleItem.ProcessingDeadlineAt.Equal(expectedStaleDeadline) {
		t.Fatalf("unexpected stale dlq runtime fields: %+v expected_deadline=%s", staleItem, expectedStaleDeadline.Format(time.RFC3339))
	}
}

func TestListEnterpriseHRISWebhookDLQSupportsReplayFilterPaginationAndCounts(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookDLQWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookDLQWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerProcessingTimeout: 10 * time.Minute,
		},
		hrisDLQSvc: hris.NewDLQService(),
	}

	if _, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:     "tenant_demo_jakarta",
		ConnectorID:  "connector-gadjian",
		Vendor:       "gadjian",
		ReceiptID:    "receipt-ready",
		RequestID:    "dlq-ready",
		EventType:    "gadjian.employee.updated",
		FailureStage: "normalize",
		Error:        "ready failure",
	}); err != nil {
		t.Fatalf("append ready entry should succeed: %v", err)
	}

	cooldownEntry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:     "tenant_demo_jakarta",
		ConnectorID:  "connector-gadjian",
		Vendor:       "gadjian",
		ReceiptID:    "receipt-cooldown",
		RequestID:    "dlq-cooldown",
		EventType:    "gadjian.employee.updated",
		FailureStage: "normalize",
		Error:        "cooldown failure",
	})
	if err != nil {
		t.Fatalf("append cooldown entry should succeed: %v", err)
	}
	cooldownClaimedAt := time.Now().UTC().Add(-2 * time.Minute)
	if _, reason, err := s.hrisDLQSvc.ClaimEntryForReplayWithBackoff(
		cooldownEntry.ID,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		cooldownClaimedAt,
	); err != nil {
		t.Fatalf("claim cooldown entry should succeed: %v", err)
	} else if reason != "" {
		t.Fatalf("expected empty claim reason for cooldown entry, got %q", reason)
	}
	if _, err := s.hrisDLQSvc.MarkReplayFailed(cooldownEntry.ID, errors.New("cooldown replay failed")); err != nil {
		t.Fatalf("mark cooldown replay failed should succeed: %v", err)
	}

	inFlightEntry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:     "tenant_demo_jakarta",
		ConnectorID:  "connector-gadjian",
		Vendor:       "gadjian",
		ReceiptID:    "receipt-processing",
		RequestID:    "dlq-processing",
		EventType:    "gadjian.employee.updated",
		FailureStage: "sync",
		Error:        "processing failure",
	})
	if err != nil {
		t.Fatalf("append in-flight entry should succeed: %v", err)
	}
	inFlightClaimedAt := time.Now().UTC().Add(-3 * time.Minute)
	if _, reason, err := s.hrisDLQSvc.ClaimEntryForReplayWithBackoff(
		inFlightEntry.ID,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		inFlightClaimedAt,
	); err != nil {
		t.Fatalf("claim in-flight entry should succeed: %v", err)
	} else if reason != "" {
		t.Fatalf("expected empty claim reason for in-flight entry, got %q", reason)
	}

	staleReadyEntry, err := s.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:     "tenant_demo_jakarta",
		ConnectorID:  "connector-gadjian",
		Vendor:       "gadjian",
		ReceiptID:    "receipt-stale",
		RequestID:    "dlq-stale-ready",
		EventType:    "gadjian.employee.updated",
		FailureStage: "sync",
		Error:        "stale replay failure",
	})
	if err != nil {
		t.Fatalf("append stale ready entry should succeed: %v", err)
	}
	staleClaimedAt := time.Now().UTC().Add(-20 * time.Minute)
	if _, reason, err := s.hrisDLQSvc.ClaimEntryForReplayWithBackoff(
		staleReadyEntry.ID,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		staleClaimedAt,
	); err != nil {
		t.Fatalf("claim stale ready entry should succeed: %v", err)
	} else if reason != "" {
		t.Fatalf("expected empty claim reason for stale ready entry, got %q", reason)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-dlq?tenant_id=tenant_demo_jakarta&connector_id=connector-gadjian&replay_state=ready&limit=1",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookDLQ(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Items []struct {
			RequestID string `json:"request_id"`
		} `json:"items"`
		Total        int  `json:"total"`
		Offset       int  `json:"offset"`
		Limit        int  `json:"limit"`
		NextOffset   int  `json:"next_offset,omitempty"`
		HasMore      bool `json:"has_more"`
		ReplayCounts struct {
			All          int `json:"all"`
			Ready        int `json:"ready"`
			Cooldown     int `json:"cooldown"`
			InFlight     int `json:"in_flight"`
			AttemptLimit int `json:"attempt_limit"`
			Terminal     int `json:"terminal"`
		} `json:"replay_counts"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid paged dlq payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 1 || payload.Items[0].RequestID != staleReadyEntry.RequestID {
		t.Fatalf("expected first ready replay page to contain stale ready entry, got %+v", payload.Items)
	}
	if payload.Total != 2 || payload.Offset != 0 || payload.Limit != 1 || !payload.HasMore || payload.NextOffset != 1 {
		t.Fatalf("unexpected pagination payload: %+v", payload)
	}
	if payload.ReplayCounts.All != 4 ||
		payload.ReplayCounts.Ready != 2 ||
		payload.ReplayCounts.Cooldown != 1 ||
		payload.ReplayCounts.InFlight != 1 ||
		payload.ReplayCounts.AttemptLimit != 0 ||
		payload.ReplayCounts.Terminal != 0 {
		t.Fatalf("unexpected replay counts: %+v", payload.ReplayCounts)
	}
}

func TestEnterpriseHRISWebhookDLQReplayFlowTalentaMergeMiss(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.attendance.scheduler.changeschedule",
		"changes":[
			{
				"employee_id":"EMP-REPLAY-001",
				"full_name":"Replay User",
				"shifts":[
					{
						"date":"2026-04-22",
						"name":"SHIFT-R",
						"schedule_in":"12:00",
						"schedule_out":"21:00"
					}
				]
			}
		]
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-talenta-merge-miss-001")
	request.Header.Set("X-Event-Type", "talenta.attendance.scheduler.changeschedule")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	listRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-dlq?tenant_id=tenant_demo_jakarta&connector_id="+connector.ID,
		nil,
	)
	listRequest = withAuthUser(listRequest, auth.User{Role: "super_admin"})
	listRecorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookDLQ(listRecorder, listRequest)

	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from dlq list, got %d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var listPayload struct {
		Items []hris.DeadLetterEntry `json:"items"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("expected valid dlq list payload: %v body=%s", err, listRecorder.Body.String())
	}
	if len(listPayload.Items) != 1 {
		t.Fatalf("expected one dlq item, got %d", len(listPayload.Items))
	}
	entry := listPayload.Items[0]
	if entry.Vendor != "talenta" {
		t.Fatalf("expected talenta dlq entry, got %s", entry.Vendor)
	}
	if entry.FailureStage != "merge" {
		t.Fatalf("expected merge failure_stage, got %s", entry.FailureStage)
	}
	if entry.RequestID != "mekari-talenta-merge-miss-001" {
		t.Fatalf("request_id mismatch: %s", entry.RequestID)
	}
	if !strings.Contains(entry.Error, "existing enterprise employee not found") {
		t.Fatalf("expected merge miss error, got %s", entry.Error)
	}

	seedTalentaDLQReplayEmployee(
		t,
		s.enterpriseSvc,
		"EMP-REPLAY-001",
		"TAL-REPLAY-001",
		"replay.user@replay-sync.local",
	)

	replayRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-dlq/"+entry.ID+"/replay",
		nil,
	)
	replayRequest = withAuthUser(replayRequest, auth.User{Role: "super_admin"})
	replayRequest = withURLParam(replayRequest, "entryID", entry.ID)
	replayRecorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookDLQ(replayRecorder, replayRequest)

	if replayRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from dlq replay, got %d body=%s", replayRecorder.Code, replayRecorder.Body.String())
	}
	var replayPayload struct {
		Item hris.DeadLetterEntry `json:"item"`
	}
	if err := json.Unmarshal(replayRecorder.Body.Bytes(), &replayPayload); err != nil {
		t.Fatalf("expected valid replay payload: %v body=%s", err, replayRecorder.Body.String())
	}
	if replayPayload.Item.Status != "resolved" {
		t.Fatalf("expected resolved dlq status, got %s", replayPayload.Item.Status)
	}
	if replayPayload.Item.ReplayCount != 1 {
		t.Fatalf("expected replay_count=1, got %d", replayPayload.Item.ReplayCount)
	}
	if replayPayload.Item.LastReplayAt == nil || replayPayload.Item.ResolvedAt == nil {
		t.Fatalf("expected replay success timestamps to be set, got %+v", replayPayload.Item)
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "EMP-REPLAY-001" {
			continue
		}
		foundEmployee = true
		if employees[i].Email != "replay.user@replay-sync.local" {
			t.Fatalf("expected merged replay to preserve email, got %s", employees[i].Email)
		}
		if employees[i].ShiftCode != "SHIFT-R" {
			t.Fatalf("shift_code mismatch: %s", employees[i].ShiftCode)
		}
		if employees[i].ScheduleWindow != "2026-04-22:12:00-21:00" {
			t.Fatalf("schedule_window mismatch: %s", employees[i].ScheduleWindow)
		}
		if employees[i].EmployeeNumber != "TAL-REPLAY-001" {
			t.Fatalf("expected merged replay to preserve employee_number, got %s", employees[i].EmployeeNumber)
		}
		if employees[i].JoinDate != "2024-01-15" {
			t.Fatalf("expected merged replay to preserve join_date, got %s", employees[i].JoinDate)
		}
		if employees[i].LeaveStatus != "annual_leave" {
			t.Fatalf("expected merged replay to preserve leave_status, got %s", employees[i].LeaveStatus)
		}
		if employees[i].CostCenter != "CC-REPLAY-01" {
			t.Fatalf("expected merged replay to preserve cost_center, got %s", employees[i].CostCenter)
		}
		if employees[i].PhotoURL != "https://cdn.example.com/photos/replay-user.jpg" {
			t.Fatalf("expected merged replay to preserve photo_url, got %s", employees[i].PhotoURL)
		}
	}
	if !foundEmployee {
		t.Fatalf("expected replayed employee to remain present")
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	foundAccessUser := false
	for i := range accessUsers {
		if accessUsers[i].Email == "replay.user@replay-sync.local" {
			foundAccessUser = true
			break
		}
	}
	if !foundAccessUser {
		t.Fatalf("expected replayed access user to be synced")
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replayed", "enterprise_sync", 10)
	if len(logs) == 0 {
		t.Fatalf("expected talenta dlq replay audit log")
	}
}

func TestEnterpriseHRISWebhookDLQReplayFlowTalentaMergeMissFromReceiptWorker(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "receipt-replay.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.attendance.scheduler.changeschedule",
		"changes":[
			{
				"employee_id":"EMP-RECEIPT-REPLAY-001",
				"full_name":"Receipt Replay User",
				"shifts":[
					{
						"date":"2026-04-23",
						"name":"SHIFT-RW",
						"schedule_in":"11:00",
						"schedule_out":"20:00"
					}
				]
			}
		]
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-talenta-receipt-dlq-replay-001")
	request.Header.Set("X-Event-Type", "talenta.attendance.scheduler.changeschedule")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)) != 0 {
		t.Fatalf("expected async receipt path to avoid immediate dlq append")
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 1, 0, time.Minute, 1)

	record, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup success after dlq handoff: %v", err)
	}
	if record.Status != "dlq" {
		t.Fatalf("expected dlq receipt status after merge miss handoff, got %s", record.Status)
	}
	if record.AttemptCount != 1 {
		t.Fatalf("expected attempt_count=1 after merge miss handoff, got %d", record.AttemptCount)
	}
	if record.ProcessedAt == nil {
		t.Fatalf("expected failed receipt processed_at to be set")
	}
	if !strings.Contains(record.LastError, "existing enterprise employee not found") {
		t.Fatalf("unexpected failed receipt last_error: %s", record.LastError)
	}
	if len(s.enterpriseSvc.ListRetryableHRISWebhookReceipts("tenant_demo_jakarta", 10)) != 0 {
		t.Fatalf("expected dlq receipt to be removed from retryable queue")
	}

	entries := s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)
	if len(entries) != 1 {
		t.Fatalf("expected one dlq entry after receipt worker merge miss, got %d", len(entries))
	}
	entry := entries[0]
	if entry.Status != "dlq" {
		t.Fatalf("expected initial dlq entry status, got %s", entry.Status)
	}
	if entry.ReceiptID != response.ReceiptID {
		t.Fatalf("expected dlq receipt_id %s, got %s", response.ReceiptID, entry.ReceiptID)
	}
	if entry.FailureStage != "merge" {
		t.Fatalf("expected merge failure_stage, got %s", entry.FailureStage)
	}
	if entry.RequestID != "mekari-talenta-receipt-dlq-replay-001" {
		t.Fatalf("request_id mismatch: %s", entry.RequestID)
	}
	if entry.EventType != "talenta.attendance.scheduler.changeschedule" {
		t.Fatalf("event_type mismatch: %s", entry.EventType)
	}

	seedTalentaDLQReplayEmployee(
		t,
		s.enterpriseSvc,
		"EMP-RECEIPT-REPLAY-001",
		"TAL-RECEIPT-REPLAY-001",
		"receipt.replay@receipt-replay.local",
	)

	replayRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-dlq/"+entry.ID+"/replay",
		nil,
	)
	replayRequest = withAuthUser(replayRequest, auth.User{Role: "super_admin"})
	replayRequest = withURLParam(replayRequest, "entryID", entry.ID)
	replayRecorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookDLQ(replayRecorder, replayRequest)

	if replayRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from dlq replay, got %d body=%s", replayRecorder.Code, replayRecorder.Body.String())
	}
	var replayPayload struct {
		Item hris.DeadLetterEntry `json:"item"`
	}
	if err := json.Unmarshal(replayRecorder.Body.Bytes(), &replayPayload); err != nil {
		t.Fatalf("expected valid replay payload: %v body=%s", err, replayRecorder.Body.String())
	}
	if replayPayload.Item.Status != "resolved" {
		t.Fatalf("expected resolved dlq status, got %s", replayPayload.Item.Status)
	}
	if replayPayload.Item.ReplayCount != 1 {
		t.Fatalf("expected replay_count=1, got %d", replayPayload.Item.ReplayCount)
	}
	if replayPayload.Item.LastReplayAt == nil || replayPayload.Item.ResolvedAt == nil {
		t.Fatalf("expected replay success timestamps to be set, got %+v", replayPayload.Item)
	}

	record, err = s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup success after replay: %v", err)
	}
	if record.Status != "processed" {
		t.Fatalf("expected replayed receipt status processed, got %s", record.Status)
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "EMP-RECEIPT-REPLAY-001" {
			continue
		}
		foundEmployee = true
		if employees[i].Email != "receipt.replay@receipt-replay.local" {
			t.Fatalf("expected merged replay to preserve email, got %s", employees[i].Email)
		}
		if employees[i].ShiftCode != "SHIFT-RW" {
			t.Fatalf("shift_code mismatch: %s", employees[i].ShiftCode)
		}
		if employees[i].ScheduleWindow != "2026-04-23:11:00-20:00" {
			t.Fatalf("schedule_window mismatch: %s", employees[i].ScheduleWindow)
		}
		if employees[i].EmployeeNumber != "TAL-RECEIPT-REPLAY-001" {
			t.Fatalf("expected merged replay to preserve employee_number, got %s", employees[i].EmployeeNumber)
		}
		if employees[i].JoinDate != "2024-01-15" {
			t.Fatalf("expected merged replay to preserve join_date, got %s", employees[i].JoinDate)
		}
		if employees[i].LeaveStatus != "annual_leave" {
			t.Fatalf("expected merged replay to preserve leave_status, got %s", employees[i].LeaveStatus)
		}
		if employees[i].CostCenter != "CC-REPLAY-01" {
			t.Fatalf("expected merged replay to preserve cost_center, got %s", employees[i].CostCenter)
		}
		if employees[i].PhotoURL != "https://cdn.example.com/photos/replay-user.jpg" {
			t.Fatalf("expected merged replay to preserve photo_url, got %s", employees[i].PhotoURL)
		}
	}
	if !foundEmployee {
		t.Fatalf("expected replayed employee to remain present")
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	foundAccessUser := false
	for i := range accessUsers {
		if accessUsers[i].Email == "receipt.replay@receipt-replay.local" {
			foundAccessUser = true
			break
		}
	}
	if !foundAccessUser {
		t.Fatalf("expected replayed access user to be synced")
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replayed", "enterprise_sync", 10)
	if len(logs) == 0 {
		t.Fatalf("expected receipt-worker talenta dlq replay audit log")
	}
}

func TestRunEnterpriseHRISWebhookDLQWorkerTick(t *testing.T) {
	normalizer := &flakyGadjianNormalizer{}
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-worker.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
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

	body := `{"event_type":"employee.updated","employee":{"id":"GADJIAN-EMP-002"}}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "gadjian-webhook-worker-001")
	request = withURLParam(request, "connectorID", connector.ID)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	entries := s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)
	if len(entries) != 1 {
		t.Fatalf("expected one dlq entry before worker tick, got %d", len(entries))
	}
	if entries[0].Status != "dlq" {
		t.Fatalf("expected initial dlq status, got %s", entries[0].Status)
	}

	s.runEnterpriseHRISWebhookDLQWorkerTick(10, 5, 0, 1)

	updated, err := s.hrisDLQSvc.GetEntry(entries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup success after worker tick: %v", err)
	}
	if updated.Status != "resolved" {
		t.Fatalf("expected worker to resolve dlq entry, got %s", updated.Status)
	}
	if updated.ReplayCount != 1 {
		t.Fatalf("expected worker replay_count=1, got %d", updated.ReplayCount)
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replayed", "enterprise_sync_worker", 10)
	if len(logs) == 0 {
		t.Fatalf("expected worker replay audit log")
	}
}

func TestRunEnterpriseHRISWebhookDLQWorkerTickRecoversStaleReplayingEntry(t *testing.T) {
	normalizer := &flakyGadjianNormalizer{}
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-worker.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
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

	body := `{"event_type":"employee.updated","employee":{"id":"GADJIAN-EMP-STALE-001"}}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "gadjian-webhook-stale-replaying-001")
	request = withURLParam(request, "connectorID", connector.ID)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	entries := s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)
	if len(entries) != 1 {
		t.Fatalf("expected one dlq entry before stale replay recovery, got %d", len(entries))
	}

	staleClaimed, reason, err := s.hrisDLQSvc.ClaimEntryForReplay(
		entries[0].ID,
		5,
		0,
		5*time.Minute,
		time.Now().UTC().Add(-10*time.Minute),
	)
	if err != nil {
		t.Fatalf("expected stale pre-claim to succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected no skip reason when creating stale replaying entry, got %s", reason)
	}
	if staleClaimed.Status != "replaying" {
		t.Fatalf("expected replaying status before worker recovery, got %s", staleClaimed.Status)
	}

	s.runEnterpriseHRISWebhookDLQWorkerTickWithProcessingTimeout(10, 5, 0, 5*time.Minute, 1)

	updated, err := s.hrisDLQSvc.GetEntry(entries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup success after stale recovery tick: %v", err)
	}
	if updated.Status != "resolved" {
		t.Fatalf("expected stale replaying entry to be resolved, got %s", updated.Status)
	}
	if updated.ReplayCount != 2 {
		t.Fatalf("expected stale recovery to increment replay_count to 2, got %d", updated.ReplayCount)
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replayed", "enterprise_sync_worker", 10)
	if len(logs) == 0 {
		t.Fatalf("expected worker replay audit log after stale recovery")
	}
}

func TestRunEnterpriseHRISWebhookDLQWorkerTickSkipsFreshReplayingEntry(t *testing.T) {
	normalizer := &flakyGadjianNormalizer{}
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-worker.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
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

	body := `{"event_type":"employee.updated","employee":{"id":"GADJIAN-EMP-INFLIGHT-001"}}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "gadjian-webhook-fresh-replaying-001")
	request = withURLParam(request, "connectorID", connector.ID)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	entries := s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)
	if len(entries) != 1 {
		t.Fatalf("expected one dlq entry before fresh replay skip, got %d", len(entries))
	}

	freshClaimed, reason, err := s.hrisDLQSvc.ClaimEntryForReplay(
		entries[0].ID,
		5,
		0,
		5*time.Minute,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("expected fresh pre-claim to succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected no skip reason when creating fresh replaying entry, got %s", reason)
	}
	if freshClaimed.ReplayCount != 1 {
		t.Fatalf("expected replay_count=1 after fresh pre-claim, got %d", freshClaimed.ReplayCount)
	}

	s.runEnterpriseHRISWebhookDLQWorkerTickWithProcessingTimeout(10, 5, 0, 5*time.Minute, 1)

	updated, err := s.hrisDLQSvc.GetEntry(entries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup success after fresh replay skip: %v", err)
	}
	if updated.Status != "replaying" {
		t.Fatalf("expected fresh replaying entry to stay replaying, got %s", updated.Status)
	}
	if updated.ReplayCount != 1 {
		t.Fatalf("expected in-flight skip to avoid replay_count increment, got %d", updated.ReplayCount)
	}

	replayedLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replayed", "enterprise_sync_worker", 10)
	if len(replayedLogs) != 0 {
		t.Fatalf("expected no replay success audit log on in-flight skip, got %d", len(replayedLogs))
	}
	failedReplayLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replay_failed", "enterprise_sync_worker", 10)
	if len(failedReplayLogs) != 0 {
		t.Fatalf("expected no replay failure audit log on in-flight skip, got %d", len(failedReplayLogs))
	}
	alertLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 0 {
		t.Fatalf("expected no worker alert audit when only skipped_in_flight, got %d", len(alertLogs))
	}
}

func TestRunEnterpriseHRISWebhookDLQWorkerTickWithLeaseRunsWhenAcquired(t *testing.T) {
	normalizer := &flakyGadjianNormalizer{}
	leaseStore := &stubWorkerLeaseStore{acquireOK: true}
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
		workerLeaseStore:       leaseStore,
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-worker.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
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

	body := `{"event_type":"employee.updated","employee":{"id":"GADJIAN-EMP-LEASE-001"}}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "gadjian-webhook-lease-001")
	request = withURLParam(request, "connectorID", connector.ID)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	entries := s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)
	if len(entries) != 1 {
		t.Fatalf("expected one dlq entry before leased worker tick, got %d", len(entries))
	}

	s.runEnterpriseHRISWebhookDLQWorkerTickWithLease(10, 5, 0, 1, 10*time.Minute)

	updated, err := s.hrisDLQSvc.GetEntry(entries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup success after leased worker tick: %v", err)
	}
	if updated.Status != "resolved" {
		t.Fatalf("expected leased worker to resolve dlq entry, got %s", updated.Status)
	}
	if updated.ReplayCount != 1 {
		t.Fatalf("expected leased worker replay_count=1, got %d", updated.ReplayCount)
	}
	if leaseStore.acquireCalls != 1 || leaseStore.releaseCalls != 1 {
		t.Fatalf("expected one lease acquire/release, got acquire=%d release=%d", leaseStore.acquireCalls, leaseStore.releaseCalls)
	}
	if leaseStore.lastKey != enterpriseHRISWebhookDLQLeaseKey {
		t.Fatalf("unexpected lease key: %s", leaseStore.lastKey)
	}
	if leaseStore.lastTTL != 10*time.Minute {
		t.Fatalf("unexpected lease ttl: %s", leaseStore.lastTTL)
	}
}

func TestRunEnterpriseHRISWebhookDLQWorkerTickWithLeaseSkipsWhenUnavailable(t *testing.T) {
	normalizer := &flakyGadjianNormalizer{}
	leaseStore := &stubWorkerLeaseStore{acquireOK: false}
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
		workerLeaseStore:       leaseStore,
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
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

	body := `{"event_type":"employee.updated","employee":{"id":"GADJIAN-EMP-LEASE-SKIP-001"}}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "gadjian-webhook-lease-skip-001")
	request = withURLParam(request, "connectorID", connector.ID)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	entries := s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)
	if len(entries) != 1 {
		t.Fatalf("expected one dlq entry before lease-miss tick, got %d", len(entries))
	}

	s.runEnterpriseHRISWebhookDLQWorkerTickWithLease(10, 5, 0, 1, 10*time.Minute)

	updated, err := s.hrisDLQSvc.GetEntry(entries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup success after lease-miss tick: %v", err)
	}
	if updated.Status != "dlq" {
		t.Fatalf("expected lease miss to preserve dlq entry, got %s", updated.Status)
	}
	if updated.ReplayCount != 0 {
		t.Fatalf("expected lease miss to avoid replay_count increment, got %d", updated.ReplayCount)
	}
	if leaseStore.acquireCalls != 1 || leaseStore.releaseCalls != 0 {
		t.Fatalf("expected one lease acquire and no release on lease miss, got acquire=%d release=%d", leaseStore.acquireCalls, leaseStore.releaseCalls)
	}
	if leaseStore.lastKey != enterpriseHRISWebhookDLQLeaseKey {
		t.Fatalf("unexpected lease key: %s", leaseStore.lastKey)
	}
	if leaseStore.lastTTL != 10*time.Minute {
		t.Fatalf("unexpected lease ttl: %s", leaseStore.lastTTL)
	}
}

func TestRunEnterpriseHRISWebhookDLQWorkerTickTalentaFailureAlertAndSkipControls(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.attendance.scheduler.changeschedule",
		"changes":[
			{
				"employee_id":"EMP-DLQ-001",
				"full_name":"DLQ User",
				"shifts":[
					{
						"date":"2026-04-22",
						"name":"SHIFT-D",
						"schedule_in":"09:00",
						"schedule_out":"18:00"
					}
				]
			}
		]
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-talenta-dlq-alert-001")
	request.Header.Set("X-Event-Type", "talenta.attendance.scheduler.changeschedule")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	initialEntries := s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)
	if len(initialEntries) != 1 {
		t.Fatalf("expected one dlq entry after merge miss, got %d", len(initialEntries))
	}
	if initialEntries[0].ReplayCount != 0 {
		t.Fatalf("expected initial replay_count=0, got %d", initialEntries[0].ReplayCount)
	}

	s.runEnterpriseHRISWebhookDLQWorkerTick(10, 5, 0, 1)

	updated, err := s.hrisDLQSvc.GetEntry(initialEntries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup success after failed worker tick: %v", err)
	}
	if updated.Status != "dlq" {
		t.Fatalf("expected entry to remain dlq after replay failure, got %s", updated.Status)
	}
	if updated.ReplayCount != 1 {
		t.Fatalf("expected replay_count=1 after failed worker replay, got %d", updated.ReplayCount)
	}
	if updated.LastReplayAt == nil {
		t.Fatalf("expected last_replay_at to be set after failed worker replay")
	}

	failedReplayLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replay_failed", "enterprise_sync_worker", 10)
	if len(failedReplayLogs) == 0 {
		t.Fatalf("expected dlq replay failed audit log")
	}
	alertLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 1 {
		t.Fatalf("expected one dlq worker alert log, got %d", len(alertLogs))
	}
	if !strings.Contains(alertLogs[0].Target, "failed=1") || !strings.Contains(alertLogs[0].Target, "threshold=1") {
		t.Fatalf("unexpected worker alert payload: %s", alertLogs[0].Target)
	}

	s.runEnterpriseHRISWebhookDLQWorkerTick(10, 5, 1*time.Hour, 1)
	afterCooldown, err := s.hrisDLQSvc.GetEntry(initialEntries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup success after cooldown skip: %v", err)
	}
	if afterCooldown.ReplayCount != 1 {
		t.Fatalf("expected cooldown skip to avoid replay_count increment, got %d", afterCooldown.ReplayCount)
	}
	alertLogs = s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 1 {
		t.Fatalf("expected cooldown dlq entry to stay out of worker alerts once it is not claimable, got %d", len(alertLogs))
	}

	s.runEnterpriseHRISWebhookDLQWorkerTick(10, 1, 0, 1)
	afterAttemptLimit, err := s.hrisDLQSvc.GetEntry(initialEntries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup success after attempt-limit skip: %v", err)
	}
	if afterAttemptLimit.ReplayCount != 1 {
		t.Fatalf("expected attempt-limit skip to avoid replay_count increment, got %d", afterAttemptLimit.ReplayCount)
	}
	alertLogs = s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 1 {
		t.Fatalf("expected attempt-limit dlq entry to avoid appending extra worker alerts, got %d", len(alertLogs))
	}
}

func TestRunEnterpriseHRISWebhookDLQWorkerTickTalentaResolvesMergeMissAfterSeed(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-worker.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.attendance.scheduler.changeschedule",
		"changes":[
			{
				"employee_id":"EMP-DLQ-SUCCESS-001",
				"full_name":"DLQ Success User",
				"shifts":[
					{
						"date":"2026-04-22",
						"name":"SHIFT-S",
						"schedule_in":"07:00",
						"schedule_out":"16:00"
					}
				]
			}
		]
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-talenta-dlq-success-001")
	request.Header.Set("X-Event-Type", "talenta.attendance.scheduler.changeschedule")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	initialEntries := s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)
	if len(initialEntries) != 1 {
		t.Fatalf("expected one dlq entry after merge miss, got %d", len(initialEntries))
	}
	if initialEntries[0].Status != "dlq" {
		t.Fatalf("expected initial dlq status, got %s", initialEntries[0].Status)
	}

	seedTalentaDLQReplayEmployee(
		t,
		s.enterpriseSvc,
		"EMP-DLQ-SUCCESS-001",
		"TAL-DLQ-SUCCESS-001",
		"dlq.success@replay-worker.local",
	)

	s.runEnterpriseHRISWebhookDLQWorkerTick(10, 5, 0, 1)

	updated, err := s.hrisDLQSvc.GetEntry(initialEntries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup success after worker replay: %v", err)
	}
	if updated.Status != "resolved" {
		t.Fatalf("expected worker to resolve merge-miss dlq entry, got %s", updated.Status)
	}
	if updated.ReplayCount != 1 {
		t.Fatalf("expected replay_count=1 after worker replay, got %d", updated.ReplayCount)
	}
	if updated.LastReplayAt == nil || updated.ResolvedAt == nil {
		t.Fatalf("expected worker replay success timestamps to be set, got %+v", updated)
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "EMP-DLQ-SUCCESS-001" {
			continue
		}
		foundEmployee = true
		if employees[i].Email != "dlq.success@replay-worker.local" {
			t.Fatalf("expected merged worker replay to preserve email, got %s", employees[i].Email)
		}
		if employees[i].EmployeeNumber != "TAL-DLQ-SUCCESS-001" {
			t.Fatalf("expected merged worker replay to preserve employee_number, got %s", employees[i].EmployeeNumber)
		}
		if employees[i].JoinDate != "2024-01-15" {
			t.Fatalf("expected merged worker replay to preserve join_date, got %s", employees[i].JoinDate)
		}
		if employees[i].LeaveStatus != "annual_leave" {
			t.Fatalf("expected merged worker replay to preserve leave_status, got %s", employees[i].LeaveStatus)
		}
		if employees[i].CostCenter != "CC-REPLAY-01" {
			t.Fatalf("expected merged worker replay to preserve cost_center, got %s", employees[i].CostCenter)
		}
		if employees[i].PhotoURL != "https://cdn.example.com/photos/replay-user.jpg" {
			t.Fatalf("expected merged worker replay to preserve photo_url, got %s", employees[i].PhotoURL)
		}
		if employees[i].ShiftCode != "SHIFT-S" {
			t.Fatalf("shift_code mismatch: %s", employees[i].ShiftCode)
		}
		if employees[i].ScheduleWindow != "2026-04-22:07:00-16:00" {
			t.Fatalf("schedule_window mismatch: %s", employees[i].ScheduleWindow)
		}
	}
	if !foundEmployee {
		t.Fatalf("expected replayed employee to remain present")
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	foundAccessUser := false
	for i := range accessUsers {
		if accessUsers[i].Email == "dlq.success@replay-worker.local" {
			foundAccessUser = true
			break
		}
	}
	if !foundAccessUser {
		t.Fatalf("expected worker replayed access user to be synced")
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replayed", "enterprise_sync_worker", 10)
	if len(logs) == 0 {
		t.Fatalf("expected worker replay audit log")
	}
}

func TestRunEnterpriseHRISWebhookDLQWorkerTickTalentaResolvesMergeMissFromReceiptWorker(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "receipt-worker.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.attendance.scheduler.changeschedule",
		"changes":[
			{
				"employee_id":"EMP-RECEIPT-WORKER-001",
				"full_name":"Receipt Worker User",
				"shifts":[
					{
						"date":"2026-04-23",
						"name":"SHIFT-W",
						"schedule_in":"06:00",
						"schedule_out":"15:00"
					}
				]
			}
		]
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-talenta-receipt-dlq-worker-001")
	request.Header.Set("X-Event-Type", "talenta.attendance.scheduler.changeschedule")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 1, 0, time.Minute, 1)

	record, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup success after dlq handoff: %v", err)
	}
	if record.Status != "dlq" {
		t.Fatalf("expected dlq receipt status after merge miss handoff, got %s", record.Status)
	}

	initialEntries := s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)
	if len(initialEntries) != 1 {
		t.Fatalf("expected one dlq entry after receipt worker merge miss, got %d", len(initialEntries))
	}
	if initialEntries[0].ReceiptID != response.ReceiptID {
		t.Fatalf("expected dlq receipt_id %s, got %s", response.ReceiptID, initialEntries[0].ReceiptID)
	}
	if len(s.enterpriseSvc.ListRetryableHRISWebhookReceipts("tenant_demo_jakarta", 10)) != 0 {
		t.Fatalf("expected dlq receipt to be removed from retryable queue")
	}

	seedTalentaDLQReplayEmployee(
		t,
		s.enterpriseSvc,
		"EMP-RECEIPT-WORKER-001",
		"TAL-RECEIPT-WORKER-001",
		"receipt.worker@receipt-worker.local",
	)

	s.runEnterpriseHRISWebhookDLQWorkerTick(10, 5, 0, 1)

	updated, err := s.hrisDLQSvc.GetEntry(initialEntries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup success after worker replay: %v", err)
	}
	if updated.Status != "resolved" {
		t.Fatalf("expected worker to resolve merge-miss dlq entry, got %s", updated.Status)
	}
	if updated.ReplayCount != 1 {
		t.Fatalf("expected replay_count=1 after worker replay, got %d", updated.ReplayCount)
	}
	if updated.LastReplayAt == nil || updated.ResolvedAt == nil {
		t.Fatalf("expected worker replay success timestamps to be set, got %+v", updated)
	}

	record, err = s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup success after worker replay: %v", err)
	}
	if record.Status != "processed" {
		t.Fatalf("expected worker replayed receipt status processed, got %s", record.Status)
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "EMP-RECEIPT-WORKER-001" {
			continue
		}
		foundEmployee = true
		if employees[i].Email != "receipt.worker@receipt-worker.local" {
			t.Fatalf("expected merged worker replay to preserve email, got %s", employees[i].Email)
		}
		if employees[i].EmployeeNumber != "TAL-RECEIPT-WORKER-001" {
			t.Fatalf("expected merged worker replay to preserve employee_number, got %s", employees[i].EmployeeNumber)
		}
		if employees[i].JoinDate != "2024-01-15" {
			t.Fatalf("expected merged worker replay to preserve join_date, got %s", employees[i].JoinDate)
		}
		if employees[i].LeaveStatus != "annual_leave" {
			t.Fatalf("expected merged worker replay to preserve leave_status, got %s", employees[i].LeaveStatus)
		}
		if employees[i].CostCenter != "CC-REPLAY-01" {
			t.Fatalf("expected merged worker replay to preserve cost_center, got %s", employees[i].CostCenter)
		}
		if employees[i].PhotoURL != "https://cdn.example.com/photos/replay-user.jpg" {
			t.Fatalf("expected merged worker replay to preserve photo_url, got %s", employees[i].PhotoURL)
		}
		if employees[i].ShiftCode != "SHIFT-W" {
			t.Fatalf("shift_code mismatch: %s", employees[i].ShiftCode)
		}
		if employees[i].ScheduleWindow != "2026-04-23:06:00-15:00" {
			t.Fatalf("schedule_window mismatch: %s", employees[i].ScheduleWindow)
		}
	}
	if !foundEmployee {
		t.Fatalf("expected replayed employee to remain present")
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	foundAccessUser := false
	for i := range accessUsers {
		if accessUsers[i].Email == "receipt.worker@receipt-worker.local" {
			foundAccessUser = true
			break
		}
	}
	if !foundAccessUser {
		t.Fatalf("expected worker replayed access user to be synced")
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replayed", "enterprise_sync_worker", 10)
	if len(logs) == 0 {
		t.Fatalf("expected receipt-worker talenta worker replay audit log")
	}
}

func TestRunEnterpriseHRISWebhookDLQWorkerTickTalentaRecoversStaleReplayingMergeMissFromReceiptWorker(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "receipt-stale.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.attendance.scheduler.changeschedule",
		"changes":[
			{
				"employee_id":"EMP-RECEIPT-STALE-001",
				"full_name":"Receipt Stale User",
				"shifts":[
					{
						"date":"2026-04-24",
						"name":"SHIFT-RS",
						"schedule_in":"05:00",
						"schedule_out":"14:00"
					}
				]
			}
		]
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-talenta-receipt-stale-replaying-001")
	request.Header.Set("X-Event-Type", "talenta.attendance.scheduler.changeschedule")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 1, 0, time.Minute, 1)

	record, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup success after dlq handoff: %v", err)
	}
	if record.Status != "dlq" {
		t.Fatalf("expected dlq receipt status after merge miss handoff, got %s", record.Status)
	}

	initialEntries := s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)
	if len(initialEntries) != 1 {
		t.Fatalf("expected one dlq entry before stale replay recovery, got %d", len(initialEntries))
	}

	staleClaimed, reason, err := s.hrisDLQSvc.ClaimEntryForReplay(
		initialEntries[0].ID,
		5,
		0,
		5*time.Minute,
		time.Now().UTC().Add(-10*time.Minute),
	)
	if err != nil {
		t.Fatalf("expected stale pre-claim to succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected no skip reason when creating stale replaying entry, got %s", reason)
	}
	if staleClaimed.Status != "replaying" {
		t.Fatalf("expected replaying status before worker recovery, got %s", staleClaimed.Status)
	}
	if staleClaimed.ReplayCount != 1 {
		t.Fatalf("expected replay_count=1 after stale pre-claim, got %d", staleClaimed.ReplayCount)
	}

	seedTalentaDLQReplayEmployee(
		t,
		s.enterpriseSvc,
		"EMP-RECEIPT-STALE-001",
		"TAL-RECEIPT-STALE-001",
		"receipt.stale@receipt-stale.local",
	)

	s.runEnterpriseHRISWebhookDLQWorkerTickWithProcessingTimeout(10, 5, 0, 5*time.Minute, 1)

	updated, err := s.hrisDLQSvc.GetEntry(initialEntries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup success after stale recovery tick: %v", err)
	}
	if updated.Status != "resolved" {
		t.Fatalf("expected stale replaying entry to be resolved, got %s", updated.Status)
	}
	if updated.ReplayCount != 2 {
		t.Fatalf("expected stale recovery to increment replay_count to 2, got %d", updated.ReplayCount)
	}
	if updated.LastReplayAt == nil || updated.ResolvedAt == nil {
		t.Fatalf("expected stale recovery success timestamps to be set, got %+v", updated)
	}

	record, err = s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup success after stale recovery replay: %v", err)
	}
	if record.Status != "processed" {
		t.Fatalf("expected stale recovery to mark receipt processed, got %s", record.Status)
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "EMP-RECEIPT-STALE-001" {
			continue
		}
		foundEmployee = true
		if employees[i].Email != "receipt.stale@receipt-stale.local" {
			t.Fatalf("expected stale recovery to preserve email, got %s", employees[i].Email)
		}
		if employees[i].EmployeeNumber != "TAL-RECEIPT-STALE-001" {
			t.Fatalf("expected stale recovery to preserve employee_number, got %s", employees[i].EmployeeNumber)
		}
		if employees[i].JoinDate != "2024-01-15" {
			t.Fatalf("expected stale recovery to preserve join_date, got %s", employees[i].JoinDate)
		}
		if employees[i].LeaveStatus != "annual_leave" {
			t.Fatalf("expected stale recovery to preserve leave_status, got %s", employees[i].LeaveStatus)
		}
		if employees[i].CostCenter != "CC-REPLAY-01" {
			t.Fatalf("expected stale recovery to preserve cost_center, got %s", employees[i].CostCenter)
		}
		if employees[i].PhotoURL != "https://cdn.example.com/photos/replay-user.jpg" {
			t.Fatalf("expected stale recovery to preserve photo_url, got %s", employees[i].PhotoURL)
		}
		if employees[i].ShiftCode != "SHIFT-RS" {
			t.Fatalf("shift_code mismatch: %s", employees[i].ShiftCode)
		}
		if employees[i].ScheduleWindow != "2026-04-24:05:00-14:00" {
			t.Fatalf("schedule_window mismatch: %s", employees[i].ScheduleWindow)
		}
	}
	if !foundEmployee {
		t.Fatalf("expected replayed employee to remain present")
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	foundAccessUser := false
	for i := range accessUsers {
		if accessUsers[i].Email == "receipt.stale@receipt-stale.local" {
			foundAccessUser = true
			break
		}
	}
	if !foundAccessUser {
		t.Fatalf("expected stale recovery to sync access user")
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replayed", "enterprise_sync_worker", 10)
	if len(logs) == 0 {
		t.Fatalf("expected worker replay audit log after stale recovery")
	}
}

func TestRunEnterpriseHRISWebhookDLQWorkerTickTalentaSkipsFreshReplayingMergeMissFromReceiptWorker(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "receipt-fresh.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.attendance.scheduler.changeschedule",
		"changes":[
			{
				"employee_id":"EMP-RECEIPT-FRESH-001",
				"full_name":"Receipt Fresh User",
				"shifts":[
					{
						"date":"2026-04-24",
						"name":"SHIFT-RF",
						"schedule_in":"09:00",
						"schedule_out":"18:00"
					}
				]
			}
		]
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-talenta-receipt-fresh-replaying-001")
	request.Header.Set("X-Event-Type", "talenta.attendance.scheduler.changeschedule")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 1, 0, time.Minute, 1)

	record, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup success after dlq handoff: %v", err)
	}
	if record.Status != "dlq" {
		t.Fatalf("expected dlq receipt status after merge miss handoff, got %s", record.Status)
	}

	initialEntries := s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)
	if len(initialEntries) != 1 {
		t.Fatalf("expected one dlq entry before fresh replay skip, got %d", len(initialEntries))
	}

	freshClaimed, reason, err := s.hrisDLQSvc.ClaimEntryForReplay(
		initialEntries[0].ID,
		5,
		0,
		5*time.Minute,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("expected fresh pre-claim to succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected no skip reason when creating fresh replaying entry, got %s", reason)
	}
	if freshClaimed.Status != "replaying" {
		t.Fatalf("expected replaying status before fresh skip, got %s", freshClaimed.Status)
	}
	if freshClaimed.ReplayCount != 1 {
		t.Fatalf("expected replay_count=1 after fresh pre-claim, got %d", freshClaimed.ReplayCount)
	}

	seedTalentaDLQReplayEmployee(
		t,
		s.enterpriseSvc,
		"EMP-RECEIPT-FRESH-001",
		"TAL-RECEIPT-FRESH-001",
		"receipt.fresh@receipt-fresh.local",
	)
	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	s.runEnterpriseHRISWebhookDLQWorkerTickWithProcessingTimeout(10, 5, 0, 5*time.Minute, 1)

	updated, err := s.hrisDLQSvc.GetEntry(initialEntries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup success after fresh replay skip: %v", err)
	}
	if updated.Status != "replaying" {
		t.Fatalf("expected fresh replaying entry to stay replaying, got %s", updated.Status)
	}
	if updated.ReplayCount != 1 {
		t.Fatalf("expected in-flight skip to avoid replay_count increment, got %d", updated.ReplayCount)
	}

	record, err = s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup success after fresh replay skip: %v", err)
	}
	if record.Status != "dlq" {
		t.Fatalf("expected fresh replay skip to keep receipt in dlq, got %s", record.Status)
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "EMP-RECEIPT-FRESH-001" {
			continue
		}
		foundEmployee = true
		if employees[i].ShiftCode != "" {
			t.Fatalf("expected fresh replay skip to avoid shift update, got %s", employees[i].ShiftCode)
		}
		if employees[i].ScheduleWindow != "" {
			t.Fatalf("expected fresh replay skip to avoid schedule update, got %s", employees[i].ScheduleWindow)
		}
	}
	if !foundEmployee {
		t.Fatalf("expected seeded employee to remain present")
	}
	if len(s.accessSvc.ListUsers("tenant_demo_jakarta")) != initialAccessUserCount {
		t.Fatalf("expected no additional access user sync on fresh replay skip")
	}

	replayedLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replayed", "enterprise_sync_worker", 10)
	if len(replayedLogs) != 0 {
		t.Fatalf("expected no replay success audit log on fresh in-flight skip, got %d", len(replayedLogs))
	}
	failedReplayLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_replay_failed", "enterprise_sync_worker", 10)
	if len(failedReplayLogs) != 0 {
		t.Fatalf("expected no replay failure audit log on fresh in-flight skip, got %d", len(failedReplayLogs))
	}
	alertLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 0 {
		t.Fatalf("expected no worker alert audit when only skipped_in_flight, got %d", len(alertLogs))
	}
}

func TestRunEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffHonorsMaxBackoff(t *testing.T) {
	normalizer := &failNTimesGadjianNormalizer{failUntil: 3}
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
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

	body := `{"event_type":"employee.updated","employee":{"id":"GADJIAN-EMP-DLQ-BACKOFF-001"}}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "gadjian-dlq-backoff-001")
	request = withURLParam(request, "connectorID", connector.ID)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	entries := s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)
	if len(entries) != 1 {
		t.Fatalf("expected one dlq entry after initial failure, got %d", len(entries))
	}

	baseBackoff := 25 * time.Millisecond
	maxBackoff := 100 * time.Millisecond
	s.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(10, 5, baseBackoff, maxBackoff, time.Minute, 1)

	entry, err := s.hrisDLQSvc.GetEntry(entries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup after first replay failure: %v", err)
	}
	if entry.Status != "dlq" || entry.ReplayCount != 1 {
		t.Fatalf("unexpected dlq entry after first replay failure: %+v", entry)
	}
	record, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", entry.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup after first replay failure: %v", err)
	}
	if record.Status != "dlq" {
		t.Fatalf("expected first replay failure to keep receipt in dlq, got %s", record.Status)
	}
	if normalizer.calls != 2 {
		t.Fatalf("expected two normalizer calls after first replay attempt, got %d", normalizer.calls)
	}

	time.Sleep(baseBackoff + 10*time.Millisecond)
	s.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(10, 5, baseBackoff, maxBackoff, time.Minute, 1)

	entry, err = s.hrisDLQSvc.GetEntry(entries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup after second replay failure: %v", err)
	}
	if entry.Status != "dlq" || entry.ReplayCount != 2 {
		t.Fatalf("unexpected dlq entry after second replay failure: %+v", entry)
	}
	record, err = s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", entry.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup after second replay failure: %v", err)
	}
	if record.Status != "dlq" {
		t.Fatalf("expected second replay failure to keep receipt in dlq, got %s", record.Status)
	}
	if normalizer.calls != 3 {
		t.Fatalf("expected three normalizer calls after second replay attempt, got %d", normalizer.calls)
	}

	time.Sleep(baseBackoff + 10*time.Millisecond)
	s.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(10, 5, baseBackoff, maxBackoff, time.Minute, 1)

	entry, err = s.hrisDLQSvc.GetEntry(entries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup during exponential cooldown: %v", err)
	}
	if entry.ReplayCount != 2 {
		t.Fatalf("expected exponential cooldown to preserve replay_count=2, got %d", entry.ReplayCount)
	}
	if normalizer.calls != 3 {
		t.Fatalf("expected exponential cooldown to skip third replay attempt, calls=%d", normalizer.calls)
	}
	alertLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_dlq_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 2 {
		t.Fatalf("expected exponential cooldown tick to avoid appending a new dlq worker alert, got %d", len(alertLogs))
	}

	time.Sleep(baseBackoff)
	s.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(10, 5, baseBackoff, maxBackoff, time.Minute, 1)

	entry, err = s.hrisDLQSvc.GetEntry(entries[0].ID)
	if err != nil {
		t.Fatalf("expected dlq entry lookup after exponential cooldown expiry: %v", err)
	}
	if entry.Status != "resolved" || entry.ReplayCount != 3 {
		t.Fatalf("expected dlq entry resolved after cooldown expiry, got %+v", entry)
	}
	record, err = s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", entry.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup after dlq replay success: %v", err)
	}
	if record.Status != "processed" {
		t.Fatalf("expected successful dlq replay to mark receipt processed, got %s", record.Status)
	}
	if normalizer.calls != 4 {
		t.Fatalf("expected fourth normalizer call after exponential cooldown expiry, got %d", normalizer.calls)
	}
	foundEmployee := false
	for _, employee := range s.enterpriseSvc.ListEmployees("tenant_demo_jakarta") {
		if employee.Email == "fail.ntimes@replay-sync.local" {
			foundEmployee = true
			break
		}
	}
	if !foundEmployee {
		t.Fatalf("expected resolved dlq replay to sync fail.ntimes@replay-sync.local")
	}
	foundAccessUser := false
	for _, user := range s.accessSvc.ListUsers("tenant_demo_jakarta") {
		if user.Email == "fail.ntimes@replay-sync.local" {
			foundAccessUser = true
			break
		}
	}
	if !foundAccessUser {
		t.Fatalf("expected resolved dlq replay to sync access user fail.ntimes@replay-sync.local")
	}
}

func seedTalentaDLQReplayEmployee(
	t *testing.T,
	svc *enterprise.Service,
	externalID string,
	employeeNumber string,
	email string,
) {
	t.Helper()

	if svc == nil {
		t.Fatalf("enterprise service is required")
	}
	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"hris_talenta",
		"seed",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID:       externalID,
				EmployeeNumber:   employeeNumber,
				Email:            email,
				FullName:         "Replay User",
				Department:       "Operations",
				JobTitle:         "Operator",
				Location:         "Jakarta",
				EmploymentStatus: "active",
				JoinDate:         "2024-01-15",
				LeaveStatus:      "annual_leave",
				CostCenter:       "CC-REPLAY-01",
				PhotoURL:         "https://cdn.example.com/photos/replay-user.jpg",
				Status:           "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected seed sync success: %v", err)
	}

	employee, err := svc.GetEmployeeByEmail("tenant_demo_jakarta", email)
	if err != nil {
		t.Fatalf("expected seeded replay employee lookup success: %v", err)
	}
	if employee.ExternalID != externalID ||
		employee.EmployeeNumber != employeeNumber ||
		employee.JoinDate != "2024-01-15" ||
		employee.LeaveStatus != "annual_leave" ||
		employee.CostCenter != "CC-REPLAY-01" ||
		employee.PhotoURL != "https://cdn.example.com/photos/replay-user.jpg" {
		t.Fatalf("expected seeded replay employee extended fields to be stored, got %+v", employee)
	}
}
