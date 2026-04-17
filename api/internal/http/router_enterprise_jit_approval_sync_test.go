package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

func TestListEnterpriseJITProvisionApprovalExternalSyncPending(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
	}

	item, err := s.enterpriseSvc.UpsertJITProvisionApprovalRequest(
		"tenant_demo_jakarta",
		"jit.approval.pending.sync@sudirman.co",
		"sub-jit-approval-pending-sync-001",
		"oidc",
		"active",
	)
	if err != nil {
		t.Fatalf("expected upsert approval request success: %v", err)
	}
	_, err = s.enterpriseSvc.ReviewJITProvisionApproval(
		"tenant_demo_jakarta",
		item.ID,
		"approved",
		"tenant.admin@sudirman.co",
		"",
	)
	if err != nil {
		t.Fatalf("expected review approval success: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/jit-provision-approvals/external-sync-pending?tenant_id=tenant_demo_jakarta",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseJITProvisionApprovalExternalSyncPending(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Items []enterprise.JITProvisionApproval `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json response decode success: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) == 0 {
		t.Fatalf("expected pending external sync item")
	}
	if payload.Items[0].ExternalSyncStatus != "pending" {
		t.Fatalf("expected pending external sync status, got %s", payload.Items[0].ExternalSyncStatus)
	}
}

func TestUpdateEnterpriseJITProvisionApprovalExternalSync(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
		auditSvc:      audit.NewService(),
	}

	item, err := s.enterpriseSvc.UpsertJITProvisionApprovalRequest(
		"tenant_demo_jakarta",
		"jit.approval.update.sync@sudirman.co",
		"sub-jit-approval-update-sync-001",
		"oidc",
		"active",
	)
	if err != nil {
		t.Fatalf("expected upsert approval request success: %v", err)
	}
	_, err = s.enterpriseSvc.ReviewJITProvisionApproval(
		"tenant_demo_jakarta",
		item.ID,
		"approved",
		"tenant.admin@sudirman.co",
		"",
	)
	if err != nil {
		t.Fatalf("expected review approval success: %v", err)
	}

	body := map[string]any{
		"tenant_id":         "tenant_demo_jakarta",
		"status":            "failed",
		"external_sync_ref": "hris-sync-job-001",
		"last_error":        "upstream timeout",
	}
	requestBytes, _ := json.Marshal(body)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/jit-provision-approvals/"+item.ID+"/external-sync",
		bytes.NewReader(requestBytes),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	request = withURLParam(request, "approvalID", item.ID)
	recorder := httptest.NewRecorder()

	s.updateEnterpriseJITProvisionApprovalExternalSync(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Item enterprise.JITProvisionApproval `json:"item"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json response decode success: %v body=%s", err, recorder.Body.String())
	}
	if payload.Item.ExternalSyncStatus != "failed" {
		t.Fatalf("expected failed external sync status, got %s", payload.Item.ExternalSyncStatus)
	}
	if payload.Item.ExternalSyncAttemptCount != 1 {
		t.Fatalf("expected external sync attempt count 1, got %d", payload.Item.ExternalSyncAttemptCount)
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_jit_approval_external_sync_updated", "enterprise_auth", 10)
	if len(logs) == 0 {
		t.Fatalf("expected external sync update audit log")
	}
}

func TestEnterpriseJITProvisionApprovalExternalSyncCallback(t *testing.T) {
	s := &server{
		cfg:           config.Config{EnterpriseJITApprovalExternalSyncCallbackToken: "cb-token"},
		enterpriseSvc: enterprise.NewService(),
		auditSvc:      audit.NewService(),
	}

	item, err := s.enterpriseSvc.UpsertJITProvisionApprovalRequest(
		"tenant_demo_jakarta",
		"jit.approval.callback.sync@sudirman.co",
		"sub-jit-approval-callback-sync-001",
		"oidc",
		"active",
	)
	if err != nil {
		t.Fatalf("expected upsert approval request success: %v", err)
	}
	_, err = s.enterpriseSvc.ReviewJITProvisionApproval(
		"tenant_demo_jakarta",
		item.ID,
		"approved",
		"tenant.admin@sudirman.co",
		"",
	)
	if err != nil {
		t.Fatalf("expected review approval success: %v", err)
	}

	body := map[string]any{
		"tenant_id":         "tenant_demo_jakarta",
		"approval_id":       item.ID,
		"status":            "synced",
		"external_sync_ref": "hris-callback-job-001",
	}
	requestBytes, _ := json.Marshal(body)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/jit-provision-approvals/external-sync/callback",
		bytes.NewReader(requestBytes),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Enterprise-Callback-Token", "cb-token")
	recorder := httptest.NewRecorder()

	s.enterpriseJITApprovalExternalSyncCallback(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Item enterprise.JITProvisionApproval `json:"item"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json response decode success: %v body=%s", err, recorder.Body.String())
	}
	if payload.Item.ExternalSyncStatus != "synced" {
		t.Fatalf("expected synced external sync status, got %s", payload.Item.ExternalSyncStatus)
	}
	if payload.Item.ExternalSyncRef != "hris-callback-job-001" {
		t.Fatalf("expected external sync ref persisted, got %s", payload.Item.ExternalSyncRef)
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_jit_approval_external_sync_callback", "enterprise_auth", 10)
	if len(logs) == 0 {
		t.Fatalf("expected external sync callback audit log")
	}
}

func TestEnterpriseJITProvisionApprovalExternalSyncCallbackRejectsInvalidToken(t *testing.T) {
	s := &server{
		cfg:           config.Config{EnterpriseJITApprovalExternalSyncCallbackToken: "cb-token"},
		enterpriseSvc: enterprise.NewService(),
	}

	body := map[string]any{
		"tenant_id":   "tenant_demo_jakarta",
		"approval_id": "jap_123",
		"status":      "failed",
		"last_error":  "upstream error",
	}
	requestBytes, _ := json.Marshal(body)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/jit-provision-approvals/external-sync/callback",
		bytes.NewReader(requestBytes),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer wrong-token")
	recorder := httptest.NewRecorder()

	s.enterpriseJITApprovalExternalSyncCallback(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestEnterpriseJITProvisionApprovalExternalSyncCallbackDisabled(t *testing.T) {
	s := &server{
		cfg: config.Config{},
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/jit-provision-approvals/external-sync/callback",
		bytes.NewReader([]byte(`{"tenant_id":"tenant_demo_jakarta","approval_id":"jap_x","status":"synced"}`)),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.enterpriseJITApprovalExternalSyncCallback(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRunEnterpriseJITApprovalExternalSyncWorkerTick(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
	}

	itemA, err := s.enterpriseSvc.UpsertJITProvisionApprovalRequest(
		"tenant_demo_jakarta",
		"jit.worker.sync.a@sudirman.co",
		"sub-jit-worker-sync-a",
		"oidc",
		"active",
	)
	if err != nil {
		t.Fatalf("expected upsert approval request A success: %v", err)
	}
	_, err = s.enterpriseSvc.ReviewJITProvisionApproval(
		"tenant_demo_jakarta",
		itemA.ID,
		"approved",
		"tenant.admin@sudirman.co",
		"",
	)
	if err != nil {
		t.Fatalf("expected review approval A success: %v", err)
	}
	itemB, err := s.enterpriseSvc.UpsertJITProvisionApprovalRequest(
		"tenant_demo_factory",
		"jit.worker.sync.b@factory.local",
		"sub-jit-worker-sync-b",
		"oidc",
		"active",
	)
	if err != nil {
		t.Fatalf("expected upsert approval request B success: %v", err)
	}
	_, err = s.enterpriseSvc.ReviewJITProvisionApproval(
		"tenant_demo_factory",
		itemB.ID,
		"rejected",
		"tenant.admin@factory.local",
		"duplicate request",
	)
	if err != nil {
		t.Fatalf("expected review approval B success: %v", err)
	}

	s.runEnterpriseJITApprovalExternalSyncWorkerTick(10, 5, 0, 3, false, "")

	afterA := s.enterpriseSvc.ListJITProvisionApprovals("tenant_demo_jakarta", "", 10)
	if len(afterA) == 0 || afterA[0].ExternalSyncStatus != "synced" {
		t.Fatalf("expected tenant A approval synced, items=%+v", afterA)
	}
	afterB := s.enterpriseSvc.ListJITProvisionApprovals("tenant_demo_factory", "", 10)
	if len(afterB) == 0 || afterB[0].ExternalSyncStatus != "synced" {
		t.Fatalf("expected tenant B approval synced, items=%+v", afterB)
	}
}

func TestRunEnterpriseJITApprovalExternalSyncWorkerTickForceErrorAlert(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
		auditSvc:      audit.NewService(),
	}

	item, err := s.enterpriseSvc.UpsertJITProvisionApprovalRequest(
		"tenant_demo_jakarta",
		"jit.worker.force.error@sudirman.co",
		"sub-jit-worker-force-error",
		"oidc",
		"active",
	)
	if err != nil {
		t.Fatalf("expected upsert approval request success: %v", err)
	}
	_, err = s.enterpriseSvc.ReviewJITProvisionApproval(
		"tenant_demo_jakarta",
		item.ID,
		"approved",
		"tenant.admin@sudirman.co",
		"",
	)
	if err != nil {
		t.Fatalf("expected review approval success: %v", err)
	}

	s.runEnterpriseJITApprovalExternalSyncWorkerTick(5, 5, time.Second, 1, true, "tenant_demo_jakarta")

	items := s.enterpriseSvc.ListJITProvisionApprovals("tenant_demo_jakarta", "", 10)
	if len(items) == 0 {
		t.Fatalf("expected approval items after worker tick")
	}
	updated := items[0]
	if updated.ExternalSyncStatus != "failed" {
		t.Fatalf("expected worker force error to set failed, got %s", updated.ExternalSyncStatus)
	}
	if updated.ExternalSyncAttemptCount != 1 {
		t.Fatalf("expected worker force error attempt count=1, got %d", updated.ExternalSyncAttemptCount)
	}

	alerts := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_jit_approval_external_sync_worker_alert", "enterprise_sync_worker", 10)
	if len(alerts) == 0 {
		t.Fatalf("expected worker alert audit entry")
	}
	if !strings.Contains(alerts[0].Target, "failed=1") || !strings.Contains(alerts[0].Target, "threshold=1") {
		t.Fatalf("unexpected worker alert target: %s", alerts[0].Target)
	}
}

func withURLParam(request *http.Request, key, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	ctx := context.WithValue(request.Context(), chi.RouteCtxKey, routeCtx)
	return request.WithContext(ctx)
}
