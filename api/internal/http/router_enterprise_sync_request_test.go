package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

type staticEnterpriseStateStore struct {
	payload []byte
}

func (s *staticEnterpriseStateStore) Load(_ string, dst any) (bool, error) {
	if len(s.payload) == 0 {
		return false, nil
	}
	return true, json.Unmarshal(s.payload, dst)
}

func (s *staticEnterpriseStateStore) Save(_ string, _ any) error {
	return nil
}

type failingAccessStateStore struct {
	saveErr   error
	saveCalls int
}

func (s *failingAccessStateStore) Load(_ string, _ any) (bool, error) {
	return false, nil
}

func (s *failingAccessStateStore) Save(_ string, _ any) error {
	s.saveCalls++
	if s.saveCalls == 1 {
		return nil
	}
	return s.saveErr
}

func reconcilePendingSyncRequestsStatePayload() []byte {
	return []byte(`{
		"domain_mappings": [],
		"hris_connectors": [],
		"hris_webhook_receipts": [],
		"idp_configs": {},
		"employees": [],
		"sync_jobs": [],
		"sync_request_records": {
			"tenant_demo_jakarta:sync-req-pending-1": {
				"request_id": "sync-req-pending-1",
				"tenant_id": "tenant_demo_jakarta",
				"connector_id": "connector-talenta",
				"result": {
					"job": {
						"id": "syn-pending-1",
						"tenant_id": "tenant_demo_jakarta",
						"source": "hris_talenta",
						"status": "completed",
						"total": 1,
						"created": 1,
						"updated": 0,
						"deactivated": 0,
						"rejected": 0,
						"actor": "enterprise.sync.worker",
						"started_at": "2026-04-22T09:00:00Z",
						"ended_at": "2026-04-22T09:01:00Z"
					},
					"items": [
						{
							"id": "emp-pending-1",
							"tenant_id": "tenant_demo_jakarta",
							"external_id": "EMP-PENDING-1",
							"email": "pending.reconcile@sync.local",
							"full_name": "Pending Reconcile",
							"access_role": "employee",
							"status": "active"
						}
					]
				},
				"access_applied": false,
				"access_created": 0,
				"access_updated": 0,
				"access_rejected": 0,
				"access_attempt_count": 1,
				"last_access_error": "access service throttled",
				"last_access_attempt_at": "2026-04-22T09:40:00Z",
				"created_at": "2026-04-22T09:10:00Z"
			},
			"tenant_demo_jakarta:sync-req-applied-1": {
				"request_id": "sync-req-applied-1",
				"tenant_id": "tenant_demo_jakarta",
				"connector_id": "connector-talenta",
				"result": {
					"job": {
						"id": "syn-applied-1",
						"tenant_id": "tenant_demo_jakarta",
						"source": "hris_talenta",
						"status": "completed",
						"total": 1,
						"created": 1,
						"updated": 0,
						"deactivated": 0,
						"rejected": 0,
						"actor": "enterprise.sync.worker",
						"started_at": "2026-04-22T08:00:00Z",
						"ended_at": "2026-04-22T08:01:00Z"
					},
					"items": []
				},
				"access_applied": true,
				"access_created": 1,
				"access_updated": 0,
				"access_rejected": 0,
				"access_attempt_count": 1,
				"last_access_attempt_at": "2026-04-22T08:01:00Z",
				"created_at": "2026-04-22T08:00:00Z"
			}
		},
		"jit_provision_approvals": []
	}`)
}

func TestReconcilePendingEnterpriseSyncRequestsRoute(t *testing.T) {
	tenantID := "tenant_demo_jakarta"
	pendingRequestID := "sync-req-pending-1"

	store := &staticEnterpriseStateStore{
		payload: reconcilePendingSyncRequestsStatePayload(),
	}

	enterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected enterprise service with state store: %v", err)
	}
	accessSvc := access.NewService()
	s := &server{
		enterpriseSvc: enterpriseSvc,
		accessSvc:     accessSvc,
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-requests/reconcile-pending",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_jakarta","limit":20}`),
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.reconcilePendingEnterpriseSyncRequests(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.BatchPendingSyncReconcileResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid reconcile-pending payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.Processed != 1 || payload.Applied != 1 || payload.Failed != 0 {
		t.Fatalf("unexpected reconcile-pending summary: %+v", payload)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected one reconciled item, got %d", len(payload.Items))
	}
	if payload.Items[0].RequestID != pendingRequestID ||
		payload.Items[0].JobID != "syn-pending-1" ||
		!payload.Items[0].AccessApplied ||
		payload.Items[0].AccessCreated != 1 ||
		payload.Items[0].AttemptCount != 2 {
		t.Fatalf("unexpected reconciled item payload: %+v", payload.Items[0])
	}
	if payload.Items[0].AttemptedAt == nil {
		t.Fatalf("expected attempted_at to be set after reconcile")
	}

	record, err := enterpriseSvc.GetSyncRequestRecord(tenantID, pendingRequestID)
	if err != nil {
		t.Fatalf("expected pending record lookup success: %v", err)
	}
	if !record.AccessApplied ||
		record.AccessCreated != 1 ||
		record.AccessAttemptCount != 2 ||
		record.LastAccessError != "" ||
		record.LastAccessAttemptAt == nil {
		t.Fatalf("unexpected updated sync request record: %+v", record)
	}

	users := accessSvc.ListUsers(tenantID)
	found := false
	for i := range users {
		if users[i].Email == "pending.reconcile@sync.local" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reconciled user to be written into access")
	}
}

func TestRunEnterpriseSyncReconcileWorkerTickAppendsAlertAuditOnAttemptLimitSkip(t *testing.T) {
	store := &staticEnterpriseStateStore{
		payload: reconcilePendingSyncRequestsStatePayload(),
	}

	enterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected enterprise service with state store: %v", err)
	}

	s := &server{
		enterpriseSvc: enterpriseSvc,
		accessSvc:     access.NewService(),
		auditSvc:      audit.NewService(),
	}

	s.runEnterpriseSyncReconcileWorkerTick(10, 1, 0, 1, false, "")

	alertLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_sync_reconcile_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 1 {
		t.Fatalf("expected one reconcile worker alert log for attempt-limit skip, got %d", len(alertLogs))
	}
	if !strings.Contains(alertLogs[0].Target, "failed=0") ||
		!strings.Contains(alertLogs[0].Target, "processed=0") ||
		!strings.Contains(alertLogs[0].Target, "skipped_attempt_limit=1") {
		t.Fatalf("unexpected reconcile worker skip alert payload: %s", alertLogs[0].Target)
	}
}

func TestReconcilePendingEnterpriseSyncRequestsRouteInvalidLimit(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
		accessSvc:     access.NewService(),
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-requests/reconcile-pending",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_jakarta","limit":-1}`),
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.reconcilePendingEnterpriseSyncRequests(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid error payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.Error != enterprise.ErrInvalidReconcileLimit.Error() {
		t.Fatalf("expected invalid limit error %q, got %q", enterprise.ErrInvalidReconcileLimit.Error(), payload.Error)
	}
}

func TestReconcilePendingEnterpriseSyncRequestsRouteTenantScopeForbidden(t *testing.T) {
	s := &server{}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-requests/reconcile-pending",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_surabaya","limit":20}`),
	)
	request = withAuthUser(request, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
	})
	recorder := httptest.NewRecorder()

	s.reconcilePendingEnterpriseSyncRequests(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid forbidden payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.Error != "tenant scope forbidden" {
		t.Fatalf("expected tenant scope forbidden error, got %q", payload.Error)
	}
}

func TestReconcilePendingEnterpriseSyncRequestsRouteAccessApplyFailureSummary(t *testing.T) {
	tenantID := "tenant_demo_jakarta"
	pendingRequestID := "sync-req-pending-1"

	enterpriseSvc, err := enterprise.NewServiceWithStateStore(&staticEnterpriseStateStore{
		payload: reconcilePendingSyncRequestsStatePayload(),
	})
	if err != nil {
		t.Fatalf("expected enterprise service with state store: %v", err)
	}

	accessSvc, err := access.NewServiceWithStateStore(&failingAccessStateStore{
		saveErr: errors.New("forced access persist failure"),
	})
	if err != nil {
		t.Fatalf("expected access service with state store: %v", err)
	}

	s := &server{
		enterpriseSvc: enterpriseSvc,
		accessSvc:     accessSvc,
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-requests/reconcile-pending",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_jakarta","limit":20}`),
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.reconcilePendingEnterpriseSyncRequests(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.BatchPendingSyncReconcileResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid reconcile-pending payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.Processed != 1 || payload.Applied != 0 || payload.Failed != 1 {
		t.Fatalf("unexpected reconcile-pending failure summary: %+v", payload)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected one failed reconcile item, got %d", len(payload.Items))
	}
	if payload.Items[0].RequestID != pendingRequestID ||
		payload.Items[0].AccessApplied ||
		payload.Items[0].AttemptCount != 2 {
		t.Fatalf("unexpected failed reconcile item payload: %+v", payload.Items[0])
	}
	if payload.Items[0].AttemptedAt == nil {
		t.Fatalf("expected attempted_at to be set on reconcile failure")
	}
	if payload.Items[0].LastError != "forced access persist failure" {
		t.Fatalf("unexpected last_error in failed reconcile item: %q", payload.Items[0].LastError)
	}

	record, err := enterpriseSvc.GetSyncRequestRecord(tenantID, pendingRequestID)
	if err != nil {
		t.Fatalf("expected pending record lookup success: %v", err)
	}
	if record.AccessApplied {
		t.Fatalf("expected failed reconcile record to remain pending: %+v", record)
	}
	if record.AccessAttemptCount != 2 || record.LastAccessAttemptAt == nil {
		t.Fatalf("expected failed reconcile attempt tracking to be updated: %+v", record)
	}
	if record.LastAccessError != payload.Items[0].LastError {
		t.Fatalf("expected record last_access_error to match payload, got %q", record.LastAccessError)
	}
	users := accessSvc.ListUsers(tenantID)
	for i := range users {
		if users[i].Email == "pending.reconcile@sync.local" {
			t.Fatalf("expected failed reconcile to avoid creating target access user")
		}
	}
}
