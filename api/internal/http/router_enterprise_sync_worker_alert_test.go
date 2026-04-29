package httpx

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/wallet"
)

type stubWorkerLeaseStore struct {
	acquireOK    bool
	acquireErr   error
	releaseErr   error
	acquireCalls int
	releaseCalls int
	lastKey      string
	lastToken    string
	lastTTL      time.Duration
}

func (s *stubWorkerLeaseStore) TryAcquireLease(key, token string, ttl time.Duration) (bool, error) {
	s.acquireCalls++
	s.lastKey = key
	s.lastToken = token
	s.lastTTL = ttl
	if s.acquireErr != nil {
		return false, s.acquireErr
	}
	return s.acquireOK, nil
}

func (s *stubWorkerLeaseStore) ReleaseLease(key, token string) error {
	s.releaseCalls++
	s.lastKey = key
	s.lastToken = token
	return s.releaseErr
}

type observedWalletAlertDispatch struct {
	method         string
	path           string
	idempotencyKey string
}

type observableWalletAlertSender struct {
	mu       sync.Mutex
	requests []observedWalletAlertDispatch
}

func newObservableWalletAlertService(t *testing.T) (*wallet.Service, *observableWalletAlertSender) {
	t.Helper()

	observer := &observableWalletAlertSender{}
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observer.mu.Lock()
		observer.requests = append(observer.requests, observedWalletAlertDispatch{
			method:         r.Method,
			path:           r.URL.Path,
			idempotencyKey: strings.TrimSpace(r.Header.Get("Idempotency-Key")),
		})
		observer.mu.Unlock()
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"email_123"}`))
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"email_123","last_event":"delivered"}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(endpoint.Close)

	walletSvc := wallet.NewService()
	if err := walletSvc.SetJobAlertEmailDeliveryOptions(wallet.JobAlertEmailDeliveryOptions{
		Provider:       "resend",
		EmailFrom:      "alerts@mistypass.local",
		ReceiverMap:    map[string][]string{"security": {"security@mistypass.local"}},
		ResendEndpoint: endpoint.URL,
		ResendAPIKey:   "test-resend-key",
	}); err != nil {
		t.Fatalf("expected observable wallet alert sender to initialize: %v", err)
	}

	return walletSvc, observer
}

func (s *observableWalletAlertSender) idempotencyKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	keys := make([]string, 0, len(s.requests))
	for i := range s.requests {
		keys = append(keys, s.requests[i].idempotencyKey)
	}
	return keys
}

func assertObservedWalletAlertIdempotencyKeys(t *testing.T, observer *observableWalletAlertSender, want []string) {
	t.Helper()

	got := observer.idempotencyKeys()
	if len(got) != len(want) {
		t.Fatalf("expected %d wallet alert dispatches, got %d keys=%v", len(want), len(got), got)
	}
	for i := range got {
		if strings.TrimSpace(got[i]) == "" {
			t.Fatalf("expected wallet alert dispatch %d to receive idempotency key, got %v", i, got)
		}
	}

	sortedGot := append([]string(nil), got...)
	sortedWant := append([]string(nil), want...)
	sort.Strings(sortedGot)
	sort.Strings(sortedWant)
	if strings.Join(sortedGot, ",") != strings.Join(sortedWant, ",") {
		t.Fatalf("expected wallet alert idempotency keys %v, got %v", sortedWant, sortedGot)
	}
}

func TestListEnterpriseSyncWorkerAlertsRoute(t *testing.T) {
	auditSvc := audit.NewService()
	s := &server{
		auditSvc: auditSvc,
	}

	if _, err := auditSvc.Append(
		"tenant_demo_jakarta",
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_pull_worker_alert",
		"failed=2 threshold=1 processed=3 synced=1 skipped_attempt_limit=1 skipped_cooldown=0 connector_id=connector-talenta vendor=talenta mode=incremental",
		"enterprise_sync_worker",
	); err != nil {
		t.Fatalf("append pull alert should succeed: %v", err)
	}
	if _, err := auditSvc.Append(
		"tenant_demo_jakarta",
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_webhook_dlq_worker_alert",
		"failed=1 threshold=1 processed=1 replayed=1 skipped_attempt_limit=0 skipped_cooldown=0",
		"enterprise_sync_worker",
	); err != nil {
		t.Fatalf("append dlq alert should succeed: %v", err)
	}
	if _, err := auditSvc.Append(
		"tenant_demo_jakarta",
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_webhook_processing_alert",
		"failed=1 threshold=1 processed=1 applied=0 connector_id=connector-talenta vendor=talenta event_type=talenta.employee.detail.created request_id=wh-001 failure_stage=merge",
		"enterprise_sync_worker",
	); err != nil {
		t.Fatalf("append processing alert should succeed: %v", err)
	}
	if _, err := auditSvc.Append(
		"tenant_other",
		"enterprise_sync_worker",
		"system",
		"enterprise_sync_reconcile_worker_alert",
		"failed=3 threshold=2 processed=4 applied=1 skipped_attempt_limit=0 skipped_cooldown=1",
		"enterprise_sync_worker",
	); err != nil {
		t.Fatalf("append other-tenant alert should succeed: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts?tenant_id=tenant_demo_jakarta&limit=10",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseSyncWorkerAlerts(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Items []enterpriseSyncWorkerAlertItem `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid worker alerts payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 3 {
		t.Fatalf("expected 3 tenant-scoped worker alerts, got %d", len(payload.Items))
	}

	seen := map[string]bool{}
	var pullItem enterpriseSyncWorkerAlertItem
	var processingItem enterpriseSyncWorkerAlertItem
	for i := range payload.Items {
		if payload.Items[i].TenantID != "tenant_demo_jakarta" {
			t.Fatalf("unexpected tenant_id in worker alert payload: %+v", payload.Items[i])
		}
		seen[payload.Items[i].WorkerAction] = true
		switch payload.Items[i].WorkerAction {
		case "enterprise_hris_pull_worker_alert":
			pullItem = payload.Items[i]
		case "enterprise_hris_webhook_processing_alert":
			processingItem = payload.Items[i]
		}
	}
	if !seen["enterprise_hris_pull_worker_alert"] ||
		!seen["enterprise_hris_webhook_dlq_worker_alert"] ||
		!seen["enterprise_hris_webhook_processing_alert"] {
		t.Fatalf("expected pull + dlq worker actions, got %+v", seen)
	}
	if pullItem.ConnectorID != "connector-talenta" || pullItem.Vendor != "talenta" || pullItem.Mode != "incremental" {
		t.Fatalf("expected pull metadata to be parsed, got %+v", pullItem)
	}
	if processingItem.ConnectorID != "connector-talenta" ||
		processingItem.Vendor != "talenta" ||
		processingItem.EventType != "talenta.employee.detail.created" ||
		processingItem.RequestID != "wh-001" ||
		processingItem.FailureStage != "merge" {
		t.Fatalf("expected processing metadata to be parsed, got %+v", processingItem)
	}
}

func TestListEnterpriseSyncWorkerAlertSummaryRoute(t *testing.T) {
	auditSvc := audit.NewService()
	s := &server{
		auditSvc: auditSvc,
	}

	if _, err := auditSvc.Append(
		"tenant_demo_jakarta",
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_pull_worker_alert",
		"failed=1 threshold=1 processed=2 synced=1 skipped_attempt_limit=0 skipped_cooldown=0",
		"enterprise_sync_worker",
	); err != nil {
		t.Fatalf("append first pull alert should succeed: %v", err)
	}
	time.Sleep(time.Millisecond)
	if _, err := auditSvc.Append(
		"tenant_demo_jakarta",
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_pull_worker_alert",
		"failed=2 threshold=1 processed=3 synced=1 skipped_attempt_limit=1 skipped_cooldown=0",
		"enterprise_sync_worker",
	); err != nil {
		t.Fatalf("append second pull alert should succeed: %v", err)
	}
	if _, err := auditSvc.Append(
		"tenant_demo_jakarta",
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_webhook_dlq_worker_alert",
		"failed=1 threshold=1 processed=1 replayed=1 skipped_attempt_limit=0 skipped_cooldown=0",
		"enterprise_sync_worker",
	); err != nil {
		t.Fatalf("append dlq alert should succeed: %v", err)
	}
	if _, err := auditSvc.Append(
		"tenant_demo_jakarta",
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_webhook_processing_alert",
		"failed=1 threshold=1 processed=1 applied=0 failure_stage=merge",
		"enterprise_sync_worker",
	); err != nil {
		t.Fatalf("append processing alert should succeed: %v", err)
	}
	if _, err := auditSvc.Append(
		"tenant_other",
		"enterprise_sync_worker",
		"system",
		"enterprise_sync_reconcile_worker_alert",
		"failed=3 threshold=2 processed=4 applied=1 skipped_attempt_limit=0 skipped_cooldown=1",
		"enterprise_sync_worker",
	); err != nil {
		t.Fatalf("append other-tenant alert should succeed: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts/summary?tenant_id=tenant_demo_jakarta&limit=10",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseSyncWorkerAlertSummary(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Items []enterpriseSyncWorkerAlertSummaryItem `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid worker alert summary payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 3 {
		t.Fatalf("expected 3 tenant-scoped summary items, got %d", len(payload.Items))
	}

	var pullSummary enterpriseSyncWorkerAlertSummaryItem
	foundPull := false
	var dlqSummary enterpriseSyncWorkerAlertSummaryItem
	foundDLQ := false
	var processingSummary enterpriseSyncWorkerAlertSummaryItem
	foundProcessing := false
	for i := range payload.Items {
		if payload.Items[i].TenantID != "tenant_demo_jakarta" {
			t.Fatalf("unexpected tenant_id in summary payload: %+v", payload.Items[i])
		}
		switch payload.Items[i].WorkerAction {
		case "enterprise_hris_pull_worker_alert":
			pullSummary = payload.Items[i]
			foundPull = true
		case "enterprise_hris_webhook_dlq_worker_alert":
			dlqSummary = payload.Items[i]
			foundDLQ = true
		case "enterprise_hris_webhook_processing_alert":
			processingSummary = payload.Items[i]
			foundProcessing = true
		}
	}
	if !foundPull || !foundDLQ || !foundProcessing {
		t.Fatalf("expected pull + dlq + processing summary items, got %+v", payload.Items)
	}
	if pullSummary.Count != 2 {
		t.Fatalf("expected pull summary count=2, got %+v", pullSummary)
	}
	if pullSummary.LastFailed != 2 || pullSummary.LastProcessed != 3 || pullSummary.LastSkippedByAttemptLimit != 1 {
		t.Fatalf("unexpected latest pull summary metrics: %+v", pullSummary)
	}
	if dlqSummary.Count != 1 || dlqSummary.LastApplied != 1 {
		t.Fatalf("unexpected latest dlq summary metrics: %+v", dlqSummary)
	}
	if processingSummary.Count != 1 || processingSummary.LastFailed != 1 || processingSummary.LastProcessed != 1 {
		t.Fatalf("unexpected latest processing summary metrics: %+v", processingSummary)
	}
}

func TestListEnterpriseSyncWorkerAlertRoutesTimeRangeAndValidation(t *testing.T) {
	auditSvc := audit.NewService()
	s := &server{
		auditSvc: auditSvc,
	}

	if _, err := auditSvc.Append(
		"tenant_demo_jakarta",
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_pull_worker_alert",
		"failed=1 threshold=1 processed=2 synced=1 skipped_attempt_limit=0 skipped_cooldown=0",
		"enterprise_sync_worker",
	); err != nil {
		t.Fatalf("append pull alert should succeed: %v", err)
	}

	futureSince := time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339)
	alertsRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts?tenant_id=tenant_demo_jakarta&since="+futureSince,
		nil,
	)
	alertsRequest = withAuthUser(alertsRequest, auth.User{Role: "super_admin"})
	alertsRecorder := httptest.NewRecorder()

	s.listEnterpriseSyncWorkerAlerts(alertsRecorder, alertsRequest)

	if alertsRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from future-since alerts request, got %d body=%s", alertsRecorder.Code, alertsRecorder.Body.String())
	}
	var alertsPayload struct {
		Items []enterpriseSyncWorkerAlertItem `json:"items"`
	}
	if err := json.Unmarshal(alertsRecorder.Body.Bytes(), &alertsPayload); err != nil {
		t.Fatalf("expected valid alerts payload: %v body=%s", err, alertsRecorder.Body.String())
	}
	if len(alertsPayload.Items) != 0 {
		t.Fatalf("expected future since to filter all worker alerts, got %d", len(alertsPayload.Items))
	}

	summaryRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts/summary?tenant_id=tenant_demo_jakarta&limit=bad",
		nil,
	)
	summaryRequest = withAuthUser(summaryRequest, auth.User{Role: "super_admin"})
	summaryRecorder := httptest.NewRecorder()

	s.listEnterpriseSyncWorkerAlertSummary(summaryRecorder, summaryRequest)

	if summaryRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from invalid limit summary request, got %d body=%s", summaryRecorder.Code, summaryRecorder.Body.String())
	}

	badSinceRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts?tenant_id=tenant_demo_jakarta&since=bad-time",
		nil,
	)
	badSinceRequest = withAuthUser(badSinceRequest, auth.User{Role: "super_admin"})
	badSinceRecorder := httptest.NewRecorder()

	s.listEnterpriseSyncWorkerAlerts(badSinceRecorder, badSinceRequest)

	if badSinceRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from invalid since alerts request, got %d body=%s", badSinceRecorder.Code, badSinceRecorder.Body.String())
	}
}

func TestGetEnterpriseSyncWorkerAlertSubscriptionRoute(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alert-subscription?tenant_id=tenant_demo_jakarta",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.getEnterpriseSyncWorkerAlertSubscription(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertSubscription
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid sync worker alert subscription payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.TenantID != "tenant_demo_jakarta" ||
		!payload.Enabled ||
		payload.WorkerAlertThreshold != 3 ||
		payload.WindowSeconds != 900 ||
		payload.CooldownSeconds != 900 ||
		!payload.Channels.Email ||
		payload.Channels.WhatsApp {
		t.Fatalf("unexpected default subscription payload: %+v", payload)
	}
	if len(payload.ReceiverGroups) != 1 || payload.ReceiverGroups[0] != "security" {
		t.Fatalf("unexpected receiver groups: %+v", payload.ReceiverGroups)
	}
}

func TestUpsertEnterpriseSyncWorkerAlertSubscriptionRoute(t *testing.T) {
	enterpriseSvc := enterprise.NewService()
	auditSvc := audit.NewService()
	s := &server{
		enterpriseSvc: enterpriseSvc,
		auditSvc:      auditSvc,
	}

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/enterprise/sync-worker-alert-subscription",
		bytes.NewBufferString(`{
			"tenant_id":"tenant_demo_jakarta",
			"enabled":true,
			"worker_alert_threshold":2,
			"window_seconds":300,
			"cooldown_seconds":120,
			"receiver_groups":["security","ops"],
			"channels":{"email":true,"whatsapp":true}
		}`),
	)
	request = withAuthUser(request, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
		Email:    "tenant.admin@sudirman.co",
	})
	recorder := httptest.NewRecorder()

	s.upsertEnterpriseSyncWorkerAlertSubscription(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertSubscription
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid upsert subscription payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.TenantID != "tenant_demo_jakarta" ||
		!payload.Enabled ||
		payload.WorkerAlertThreshold != 2 ||
		payload.WindowSeconds != 300 ||
		payload.CooldownSeconds != 120 ||
		!payload.Channels.Email ||
		!payload.Channels.WhatsApp {
		t.Fatalf("unexpected upserted subscription payload: %+v", payload)
	}
	if len(payload.ReceiverGroups) != 2 || payload.ReceiverGroups[0] != "security" || payload.ReceiverGroups[1] != "ops" {
		t.Fatalf("unexpected receiver groups: %+v", payload.ReceiverGroups)
	}

	record, found := enterpriseSvc.GetSyncWorkerAlertSubscription("tenant_demo_jakarta")
	if !found {
		t.Fatalf("expected subscription to be persisted")
	}
	if record.WorkerAlertThreshold != payload.WorkerAlertThreshold ||
		record.WindowSeconds != payload.WindowSeconds ||
		record.CooldownSeconds != payload.CooldownSeconds {
		t.Fatalf("expected persisted subscription to match payload: %+v", record)
	}

	logs := auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_sync_worker_alert_subscription_upserted", "enterprise_sync", 10)
	if len(logs) != 1 {
		t.Fatalf("expected one audit log for upsert, got %d", len(logs))
	}
}

func TestUpsertEnterpriseSyncWorkerAlertSubscriptionRouteValidation(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
	}

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/enterprise/sync-worker-alert-subscription",
		bytes.NewBufferString(`{
			"tenant_id":"tenant_demo_jakarta",
			"enabled":true,
			"worker_alert_threshold":0,
			"window_seconds":300,
			"cooldown_seconds":120,
			"channels":{"email":false,"whatsapp":false}
		}`),
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.upsertEnterpriseSyncWorkerAlertSubscription(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid error payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.Error != enterprise.ErrInvalidSyncWorkerAlertSubscriptionOptions.Error() {
		t.Fatalf(
			"expected validation error %q, got %q",
			enterprise.ErrInvalidSyncWorkerAlertSubscriptionOptions.Error(),
			payload.Error,
		)
	}
}

func TestUpsertEnterpriseSyncWorkerAlertSubscriptionRouteTenantScopeForbidden(t *testing.T) {
	s := &server{}

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/enterprise/sync-worker-alert-subscription",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_surabaya","worker_alert_threshold":3,"window_seconds":300}`),
	)
	request = withAuthUser(request, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
	})
	recorder := httptest.NewRecorder()

	s.upsertEnterpriseSyncWorkerAlertSubscription(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid error payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.Error != "tenant scope forbidden" {
		t.Fatalf("expected tenant scope forbidden error, got %q", payload.Error)
	}
}

func TestDispatchEnterpriseSyncWorkerAlertsRoute(t *testing.T) {
	auditSvc := audit.NewService()
	enterpriseSvc := enterprise.NewService()
	walletSvc, sender := newObservableWalletAlertService(t)
	s := &server{
		auditSvc:      auditSvc,
		enterpriseSvc: enterpriseSvc,
		walletSvc:     walletSvc,
	}

	for i := 0; i < 3; i++ {
		if _, err := auditSvc.Append(
			"tenant_demo_jakarta",
			"enterprise_sync_worker",
			"system",
			"enterprise_hris_pull_worker_alert",
			"failed=2 threshold=1 processed=3 synced=1 skipped_attempt_limit=1 skipped_cooldown=0 connector_id=connector-talenta vendor=talenta mode=incremental",
			"enterprise_sync_worker",
		); err != nil {
			t.Fatalf("append pull alert should succeed: %v", err)
		}
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-worker-alerts/dispatch",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_jakarta","worker_actions":["enterprise_hris_pull_worker_alert"]}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withAuthUser(request, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
		Email:    "tenant.admin@sudirman.co",
	})
	recorder := httptest.NewRecorder()

	s.dispatchEnterpriseSyncWorkerAlerts(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertDispatchResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid dispatch payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.TotalAlerts != 1 || payload.Dispatched != 1 || payload.Skipped != 0 || payload.Failed != 0 {
		t.Fatalf("unexpected dispatch summary: %+v", payload)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected one dispatched notification item, got %d", len(payload.Items))
	}
	if payload.Items[0].WorkerAction != "enterprise_hris_pull_worker_alert" || payload.Items[0].Status != "sent" {
		t.Fatalf("unexpected dispatched notification item: %+v", payload.Items[0])
	}
	assertObservedWalletAlertIdempotencyKeys(t, sender, []string{payload.Items[0].IdempotencyKey})
	if payload.Items[0].Attempt != 1 || len(payload.Items[0].ChannelResults) != 1 || payload.Items[0].ChannelResults[0].Status != "sent" {
		t.Fatalf("expected successful channel dispatch details, got %+v", payload.Items[0])
	}

	stored := enterpriseSvc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(stored) != 1 || stored[0].Status != "sent" {
		t.Fatalf("expected sent notification to be persisted, got %+v", stored)
	}

	logs := auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_sync_worker_alert_dispatch_requested", "enterprise_sync", 10)
	if len(logs) != 1 {
		t.Fatalf("expected one dispatch audit log, got %d", len(logs))
	}
}

func TestDispatchEnterpriseSyncWorkerAlertsRouteCooldownAndValidation(t *testing.T) {
	auditSvc := audit.NewService()
	enterpriseSvc := enterprise.NewService()
	walletSvc := wallet.NewService()
	s := &server{
		auditSvc:      auditSvc,
		enterpriseSvc: enterpriseSvc,
		walletSvc:     walletSvc,
	}

	for i := 0; i < 3; i++ {
		if _, err := auditSvc.Append(
			"tenant_demo_jakarta",
			"enterprise_sync_worker",
			"system",
			"enterprise_hris_webhook_processing_alert",
			"failed=1 threshold=1 processed=1 applied=0 connector_id=connector-talenta vendor=talenta event_type=talenta.employee.detail.created failure_stage=merge",
			"enterprise_sync_worker",
		); err != nil {
			t.Fatalf("append processing alert should succeed: %v", err)
		}
	}

	firstRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-worker-alerts/dispatch",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_jakarta","worker_actions":["enterprise_hris_webhook_processing_alert"]}`),
	)
	firstRequest.Header.Set("Content-Type", "application/json")
	firstRequest = withAuthUser(firstRequest, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
		Email:    "tenant.admin@sudirman.co",
	})
	firstRecorder := httptest.NewRecorder()

	s.dispatchEnterpriseSyncWorkerAlerts(firstRecorder, firstRequest)

	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("expected first dispatch 200, got %d body=%s", firstRecorder.Code, firstRecorder.Body.String())
	}

	secondRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-worker-alerts/dispatch",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_jakarta","worker_actions":["enterprise_hris_webhook_processing_alert"]}`),
	)
	secondRequest.Header.Set("Content-Type", "application/json")
	secondRequest = withAuthUser(secondRequest, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
		Email:    "tenant.admin@sudirman.co",
	})
	secondRecorder := httptest.NewRecorder()

	s.dispatchEnterpriseSyncWorkerAlerts(secondRecorder, secondRequest)

	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("expected second dispatch 200, got %d body=%s", secondRecorder.Code, secondRecorder.Body.String())
	}
	var secondPayload enterprise.SyncWorkerAlertDispatchResult
	if err := json.Unmarshal(secondRecorder.Body.Bytes(), &secondPayload); err != nil {
		t.Fatalf("expected valid second dispatch payload: %v body=%s", err, secondRecorder.Body.String())
	}
	if secondPayload.Dispatched != 0 || secondPayload.Skipped != 1 || len(secondPayload.Items) != 1 || secondPayload.Items[0].Reason != "cooldown" {
		t.Fatalf("expected cooldown skip on second dispatch, got %+v", secondPayload)
	}

	badRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-worker-alerts/dispatch",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_jakarta","worker_actions":["bad-action"]}`),
	)
	badRequest.Header.Set("Content-Type", "application/json")
	badRequest = withAuthUser(badRequest, auth.User{Role: "super_admin"})
	badRecorder := httptest.NewRecorder()

	s.dispatchEnterpriseSyncWorkerAlerts(badRecorder, badRequest)

	if badRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid worker_action request to return 400, got %d body=%s", badRecorder.Code, badRecorder.Body.String())
	}
}

func TestDispatchEnterpriseSyncWorkerAlertsRouteSplitsConnectorBuckets(t *testing.T) {
	auditSvc := audit.NewService()
	enterpriseSvc := enterprise.NewService()
	walletSvc := wallet.NewService()
	s := &server{
		auditSvc:      auditSvc,
		enterpriseSvc: enterpriseSvc,
		walletSvc:     walletSvc,
	}

	for i := 0; i < 3; i++ {
		if _, err := auditSvc.Append(
			"tenant_demo_jakarta",
			"enterprise_sync_worker",
			"system",
			"enterprise_hris_webhook_processing_alert",
			"failed=1 threshold=1 processed=1 applied=0 connector_id=connector-talenta-a vendor=talenta event_type=talenta.employee.detail.updated failure_stage=merge",
			"enterprise_sync_worker",
		); err != nil {
			t.Fatalf("append connector A alert should succeed: %v", err)
		}
		if _, err := auditSvc.Append(
			"tenant_demo_jakarta",
			"enterprise_sync_worker",
			"system",
			"enterprise_hris_webhook_processing_alert",
			"failed=1 threshold=1 processed=1 applied=0 connector_id=connector-talenta-b vendor=talenta event_type=talenta.employee.detail.updated failure_stage=persist",
			"enterprise_sync_worker",
		); err != nil {
			t.Fatalf("append connector B alert should succeed: %v", err)
		}
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-worker-alerts/dispatch",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_jakarta","worker_actions":["enterprise_hris_webhook_processing_alert"]}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withAuthUser(request, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
		Email:    "tenant.admin@sudirman.co",
	})
	recorder := httptest.NewRecorder()

	s.dispatchEnterpriseSyncWorkerAlerts(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertDispatchResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid dispatch payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.TotalAlerts != 2 || payload.Dispatched != 2 || len(payload.Items) != 2 {
		t.Fatalf("expected two granular dispatch candidates, got %+v", payload)
	}

	seenConnectors := map[string]bool{}
	seenStages := map[string]bool{}
	for i := range payload.Items {
		seenConnectors[payload.Items[i].ConnectorID] = true
		seenStages[payload.Items[i].FailureStage] = true
	}
	if !seenConnectors["connector-talenta-a"] || !seenConnectors["connector-talenta-b"] {
		t.Fatalf("expected both connector buckets to dispatch, got %+v", payload.Items)
	}
	if !seenStages["merge"] || !seenStages["persist"] {
		t.Fatalf("expected both failure stages to dispatch, got %+v", payload.Items)
	}
}

func TestDispatchEnterpriseSyncWorkerAlertsRouteCooldownDoesNotBlockOtherConnectorBucket(t *testing.T) {
	auditSvc := audit.NewService()
	enterpriseSvc := enterprise.NewService()
	walletSvc := wallet.NewService()
	s := &server{
		auditSvc:      auditSvc,
		enterpriseSvc: enterpriseSvc,
		walletSvc:     walletSvc,
	}

	for i := 0; i < 3; i++ {
		if _, err := auditSvc.Append(
			"tenant_demo_jakarta",
			"enterprise_sync_worker",
			"system",
			"enterprise_hris_webhook_processing_alert",
			"failed=1 threshold=1 processed=1 applied=0 connector_id=connector-talenta-a vendor=talenta event_type=talenta.employee.detail.updated failure_stage=merge",
			"enterprise_sync_worker",
		); err != nil {
			t.Fatalf("append connector A alert should succeed: %v", err)
		}
	}

	firstRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-worker-alerts/dispatch",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_jakarta","worker_actions":["enterprise_hris_webhook_processing_alert"]}`),
	)
	firstRequest.Header.Set("Content-Type", "application/json")
	firstRequest = withAuthUser(firstRequest, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
		Email:    "tenant.admin@sudirman.co",
	})
	firstRecorder := httptest.NewRecorder()

	s.dispatchEnterpriseSyncWorkerAlerts(firstRecorder, firstRequest)

	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("expected first dispatch 200, got %d body=%s", firstRecorder.Code, firstRecorder.Body.String())
	}

	for i := 0; i < 3; i++ {
		if _, err := auditSvc.Append(
			"tenant_demo_jakarta",
			"enterprise_sync_worker",
			"system",
			"enterprise_hris_webhook_processing_alert",
			"failed=1 threshold=1 processed=1 applied=0 connector_id=connector-talenta-b vendor=talenta event_type=talenta.employee.detail.updated failure_stage=merge",
			"enterprise_sync_worker",
		); err != nil {
			t.Fatalf("append connector B alert should succeed: %v", err)
		}
	}

	secondRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-worker-alerts/dispatch",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_jakarta","worker_actions":["enterprise_hris_webhook_processing_alert"]}`),
	)
	secondRequest.Header.Set("Content-Type", "application/json")
	secondRequest = withAuthUser(secondRequest, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
		Email:    "tenant.admin@sudirman.co",
	})
	secondRecorder := httptest.NewRecorder()

	s.dispatchEnterpriseSyncWorkerAlerts(secondRecorder, secondRequest)

	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("expected second dispatch 200, got %d body=%s", secondRecorder.Code, secondRecorder.Body.String())
	}

	var secondPayload enterprise.SyncWorkerAlertDispatchResult
	if err := json.Unmarshal(secondRecorder.Body.Bytes(), &secondPayload); err != nil {
		t.Fatalf("expected valid second dispatch payload: %v body=%s", err, secondRecorder.Body.String())
	}
	if secondPayload.TotalAlerts != 2 || secondPayload.Dispatched != 1 || secondPayload.Skipped != 1 || len(secondPayload.Items) != 2 {
		t.Fatalf("expected connector A cooldown skip and connector B dispatch, got %+v", secondPayload)
	}

	seenStatuses := map[string]string{}
	seenReasons := map[string]string{}
	for i := range secondPayload.Items {
		seenStatuses[secondPayload.Items[i].ConnectorID] = secondPayload.Items[i].Status
		seenReasons[secondPayload.Items[i].ConnectorID] = secondPayload.Items[i].Reason
	}
	if seenStatuses["connector-talenta-a"] != "skipped" || seenReasons["connector-talenta-a"] != "cooldown" {
		t.Fatalf("expected connector A to stay in cooldown, got %+v", secondPayload.Items)
	}
	if seenStatuses["connector-talenta-b"] != "sent" || seenReasons["connector-talenta-b"] != "" {
		t.Fatalf("expected connector B to dispatch despite connector A cooldown, got %+v", secondPayload.Items)
	}
}

func TestDispatchEnterpriseSyncWorkerAlertsRouteDoesNotUseActionOnlyAggregateThreshold(t *testing.T) {
	auditSvc := audit.NewService()
	enterpriseSvc := enterprise.NewService()
	walletSvc := wallet.NewService()
	s := &server{
		auditSvc:      auditSvc,
		enterpriseSvc: enterpriseSvc,
		walletSvc:     walletSvc,
	}

	_, err := enterpriseSvc.UpsertSyncWorkerAlertSubscription(enterprise.SyncWorkerAlertSubscriptionUpsertOptions{
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

	if _, err := auditSvc.Append(
		"tenant_demo_jakarta",
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_webhook_processing_alert",
		"failed=1 threshold=1 processed=1 applied=0 connector_id=connector-talenta-a vendor=talenta event_type=talenta.employee.detail.updated failure_stage=merge",
		"enterprise_sync_worker",
	); err != nil {
		t.Fatalf("append merge alert should succeed: %v", err)
	}
	if _, err := auditSvc.Append(
		"tenant_demo_jakarta",
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_webhook_processing_alert",
		"failed=1 threshold=1 processed=1 applied=0 connector_id=connector-talenta-a vendor=talenta event_type=talenta.employee.detail.updated failure_stage=persist",
		"enterprise_sync_worker",
	); err != nil {
		t.Fatalf("append persist alert should succeed: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-worker-alerts/dispatch",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_jakarta","worker_actions":["enterprise_hris_webhook_processing_alert"]}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withAuthUser(request, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
		Email:    "tenant.admin@sudirman.co",
	})
	recorder := httptest.NewRecorder()

	s.dispatchEnterpriseSyncWorkerAlerts(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertDispatchResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid dispatch payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.TotalAlerts != 0 || payload.Dispatched != 0 || payload.Skipped != 0 || payload.Failed != 0 || len(payload.Items) != 0 {
		t.Fatalf("expected split buckets below threshold to avoid dispatch, got %+v", payload)
	}
}

func TestListEnterpriseSyncWorkerAlertNotificationsRoute(t *testing.T) {
	enterpriseSvc := enterprise.NewService()
	now := time.Date(2026, 4, 23, 3, 0, 0, 0, time.UTC)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	_, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			if strings.Contains(input.EmailSubject, "Webhook") {
				return enterprise.SyncWorkerAlertDeliveryResult{
					Status:        "failed",
					Reason:        "provider_transient_error",
					Provider:      "mock",
					ProviderError: "temporary outage",
					Retryable:     true,
				}
			}
			return enterprise.SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected dispatch to succeed: %v", err)
	}

	s := &server{
		enterpriseSvc: enterpriseSvc,
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts/notifications?tenant_id=tenant_demo_jakarta&status=failed&limit=10",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseSyncWorkerAlertNotifications(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Items []enterprise.SyncWorkerAlertNotification `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid notifications payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected one failed notification, got %d", len(payload.Items))
	}
	if payload.Items[0].Status != "failed" || !payload.Items[0].Retryable {
		t.Fatalf("expected retryable failed notification, got %+v", payload.Items[0])
	}

	badStatusRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts/notifications?tenant_id=tenant_demo_jakarta&status=bad",
		nil,
	)
	badStatusRequest = withAuthUser(badStatusRequest, auth.User{Role: "super_admin"})
	badStatusRecorder := httptest.NewRecorder()

	s.listEnterpriseSyncWorkerAlertNotifications(badStatusRecorder, badStatusRequest)

	if badStatusRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid status request to return 400, got %d body=%s", badStatusRecorder.Code, badStatusRecorder.Body.String())
	}
}

func TestListEnterpriseSyncWorkerAlertNotificationsRouteReconcilesDispatchCommitUnknown(t *testing.T) {
	enterpriseSvc := enterprise.NewService()
	walletSvc, sender := newObservableWalletAlertService(t)
	now := time.Date(2026, 4, 23, 3, 25, 0, 0, time.UTC)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			return enterprise.SyncWorkerAlertDeliveryResult{
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
		t.Fatalf("expected pending dispatch commit history to succeed: %v", err)
	}
	if len(initial.Items) != 1 || initial.Items[0].Reason != "dispatch_commit_unknown" {
		t.Fatalf("expected one pending dispatch commit item, got %+v", initial.Items)
	}

	s := &server{
		enterpriseSvc: enterpriseSvc,
		walletSvc:     walletSvc,
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts/notifications?tenant_id=tenant_demo_jakarta&status=sent&limit=10",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseSyncWorkerAlertNotifications(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertNotificationListResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid notifications payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected one confirmed notification, got %+v", payload.Items)
	}
	if payload.Items[0].Reason != "dispatch_commit_confirmed" || payload.Items[0].SourceNotificationID != initial.Items[0].ID {
		t.Fatalf("expected confirmed history to be visible after reconciliation, got %+v", payload.Items[0])
	}
	if len(payload.Items[0].ChannelResults) != 1 || payload.Items[0].ChannelResults[0].ProviderDeliveryStatus != "delivered" {
		t.Fatalf("expected confirmed provider status in channel results, got %+v", payload.Items[0].ChannelResults)
	}

	history := enterpriseSvc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) != 2 || history[0].Reason != "dispatch_commit_confirmed" {
		t.Fatalf("expected reconciliation to prepend confirmed history, got %+v", history)
	}
	if len(sender.requests) != 1 || sender.requests[0].method != http.MethodGet || sender.requests[0].path != "/email_123" {
		t.Fatalf("expected one provider confirm request, got %+v", sender.requests)
	}
}

func TestListEnterpriseSyncWorkerAlertNotificationsRouteExposesPendingAgeSeconds(t *testing.T) {
	enterpriseSvc := enterprise.NewService()
	triggeredAt := time.Now().UTC().Add(-13 * time.Minute).Truncate(time.Second)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	_, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     subscription.TenantID,
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			return enterprise.SyncWorkerAlertDeliveryResult{
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
		t.Fatalf("expected pending dispatch commit history to succeed: %v", err)
	}

	s := &server{
		enterpriseSvc: enterpriseSvc,
	}

	beforeRequest := time.Now().UTC()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts/notifications?tenant_id=tenant_demo_jakarta&status=failed&reason=dispatch_commit_unknown&limit=10",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseSyncWorkerAlertNotifications(recorder, request)
	afterRequest := time.Now().UTC()

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertNotificationListResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid notifications payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected one pending notification, got %+v", payload.Items)
	}
	item := payload.Items[0]
	if item.Reason != "dispatch_commit_unknown" {
		t.Fatalf("expected pending dispatch_commit_unknown record, got %+v", item)
	}
	minAge := int64(beforeRequest.Sub(triggeredAt).Seconds())
	maxAge := int64(afterRequest.Sub(triggeredAt).Seconds()) + 1
	if item.PendingAgeSeconds < minAge || item.PendingAgeSeconds > maxAge {
		t.Fatalf("expected pending_age_seconds in [%d,%d], got %+v", minAge, maxAge, item)
	}
}

func TestListEnterpriseSyncWorkerAlertNotificationsRouteExposesConfirmationObservability(t *testing.T) {
	enterpriseSvc := enterprise.NewService()
	triggeredAt := time.Date(2026, 4, 23, 7, 0, 0, 0, time.UTC)
	confirmedAt := triggeredAt.Add(7 * time.Minute)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     subscription.TenantID,
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			return enterprise.SyncWorkerAlertDeliveryResult{
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
		t.Fatalf("expected pending dispatch commit history to succeed: %v", err)
	}
	if len(initial.Items) != 1 {
		t.Fatalf("expected one pending dispatch item, got %+v", initial.Items)
	}

	_, err = enterpriseSvc.ConfirmSyncWorkerAlertNotifications(enterprise.SyncWorkerAlertNotificationConfirmInput{
		TenantID:    subscription.TenantID,
		ConfirmedAt: confirmedAt,
		Confirm: func(input enterprise.SyncWorkerAlertConfirmationInput) enterprise.SyncWorkerAlertConfirmationResult {
			return enterprise.SyncWorkerAlertConfirmationResult{
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
		t.Fatalf("expected pending confirmation attempt to succeed: %v", err)
	}

	s := &server{
		enterpriseSvc: enterpriseSvc,
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts/notifications?tenant_id=tenant_demo_jakarta&status=failed&reason=dispatch_commit_unknown&limit=10",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseSyncWorkerAlertNotifications(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertNotificationListResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid notifications payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected one pending notification, got %+v", payload.Items)
	}
	item := payload.Items[0]
	if item.ConfirmAttempts != 1 || item.LastConfirmAttemptAt == nil || !item.LastConfirmAttemptAt.Equal(confirmedAt) || item.LastConfirmResult != "provider_error" {
		t.Fatalf("expected confirmation observability fields in list response, got %+v", item)
	}
}

func TestListEnterpriseSyncWorkerAlertNotificationsRouteSupportsQueryPaginationAndCounts(t *testing.T) {
	enterpriseSvc := enterprise.NewService()
	now := time.Date(2026, 4, 23, 3, 20, 0, 0, time.UTC)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	_, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			if strings.Contains(input.EmailSubject, "Webhook") {
				return enterprise.SyncWorkerAlertDeliveryResult{
					Status:        "failed",
					Reason:        "provider_transient_error",
					Provider:      "mock",
					ProviderError: "temporary outage",
					Retryable:     true,
				}
			}
			return enterprise.SyncWorkerAlertDeliveryResult{
				Status:   "sent",
				Provider: "mock",
			}
		},
	})
	if err != nil {
		t.Fatalf("expected dispatch to succeed: %v", err)
	}

	failedItems := enterpriseSvc.ListSyncWorkerAlertNotificationsWithOptions(enterprise.SyncWorkerAlertNotificationListOptions{
		TenantID: "tenant_demo_jakarta",
		Status:   "failed",
		Limit:    10,
	})
	if len(failedItems) != 1 {
		t.Fatalf("expected failed notification item, got %+v", failedItems)
	}
	if _, err := enterpriseSvc.BatchSuppressSyncWorkerAlertNotifications(enterprise.SyncWorkerAlertNotificationBatchSuppressInput{
		TenantID:        "tenant_demo_jakarta",
		NotificationIDs: []string{failedItems[0].ID},
		SuppressedAt:    now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("expected suppress to succeed: %v", err)
	}

	s := &server{
		enterpriseSvc: enterpriseSvc,
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts/notifications?tenant_id=tenant_demo_jakarta&q=talenta&offset=1&limit=1",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseSyncWorkerAlertNotifications(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertNotificationListResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid notifications payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.Total != 3 || len(payload.Items) != 1 || !payload.HasMore || payload.NextOffset != 2 {
		t.Fatalf("unexpected paged payload: %+v", payload)
	}
	if payload.FilterCounts.All != 3 || payload.FilterCounts.Suppressed != 1 {
		t.Fatalf("unexpected filter counts: %+v", payload.FilterCounts)
	}
	if payload.StatusCounts.Sent != 1 || payload.StatusCounts.Failed != 1 || payload.StatusCounts.Skipped != 1 {
		t.Fatalf("unexpected status counts: %+v", payload.StatusCounts)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected one paged item, got %+v", payload.Items)
	}

	badRetryableRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts/notifications?tenant_id=tenant_demo_jakarta&retryable=maybe",
		nil,
	)
	badRetryableRequest = withAuthUser(badRetryableRequest, auth.User{Role: "super_admin"})
	badRetryableRecorder := httptest.NewRecorder()

	s.listEnterpriseSyncWorkerAlertNotifications(badRetryableRecorder, badRetryableRequest)

	if badRetryableRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid retryable request to return 400, got %d body=%s", badRetryableRecorder.Code, badRetryableRecorder.Body.String())
	}

	suppressedRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts/notifications?tenant_id=tenant_demo_jakarta&status=skipped&reason=manual_suppressed",
		nil,
	)
	suppressedRequest = withAuthUser(suppressedRequest, auth.User{Role: "super_admin"})
	suppressedRecorder := httptest.NewRecorder()

	s.listEnterpriseSyncWorkerAlertNotifications(suppressedRecorder, suppressedRequest)

	if suppressedRecorder.Code != http.StatusOK {
		t.Fatalf("expected suppressed request 200, got %d body=%s", suppressedRecorder.Code, suppressedRecorder.Body.String())
	}
	var suppressedPayload enterprise.SyncWorkerAlertNotificationListResult
	if err := json.Unmarshal(suppressedRecorder.Body.Bytes(), &suppressedPayload); err != nil {
		t.Fatalf("expected valid suppressed payload: %v body=%s", err, suppressedRecorder.Body.String())
	}
	if len(suppressedPayload.Items) != 1 || suppressedPayload.Items[0].RestoreStatus != "ready" {
		t.Fatalf("expected suppressed item to expose restore readiness, got %+v", suppressedPayload.Items)
	}
}

func TestExportEnterpriseSyncWorkerAlertNotificationsCSVRoute(t *testing.T) {
	enterpriseSvc := enterprise.NewService()
	now := time.Date(2026, 4, 23, 3, 45, 0, 0, time.UTC)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	_, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			return enterprise.SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected dispatch to succeed: %v", err)
	}

	failedItems := enterpriseSvc.ListSyncWorkerAlertNotificationsWithOptions(enterprise.SyncWorkerAlertNotificationListOptions{
		TenantID: "tenant_demo_jakarta",
		Status:   "failed",
		Limit:    10,
	})
	if len(failedItems) != 1 {
		t.Fatalf("expected failed notification item, got %+v", failedItems)
	}
	if _, err := enterpriseSvc.BatchSuppressSyncWorkerAlertNotifications(enterprise.SyncWorkerAlertNotificationBatchSuppressInput{
		TenantID:        "tenant_demo_jakarta",
		NotificationIDs: []string{failedItems[0].ID},
		SuppressedAt:    now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("expected suppress to succeed: %v", err)
	}

	s := &server{
		enterpriseSvc: enterpriseSvc,
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts/notifications/export-csv?tenant_id=tenant_demo_jakarta&status=skipped&reason=manual_suppressed",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.exportEnterpriseSyncWorkerAlertNotificationsCSV(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/csv") {
		t.Fatalf("expected csv content type, got %q", contentType)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "restore_status") ||
		!strings.Contains(body, "ready") ||
		!strings.Contains(body, "manual_suppressed") ||
		!strings.Contains(body, "connector-talenta-001") {
		t.Fatalf("expected csv body to contain suppressed notification fields, got %s", body)
	}
}

func TestExportEnterpriseSyncWorkerAlertNotificationsCSVRouteIncludesPendingAgeSeconds(t *testing.T) {
	enterpriseSvc := enterprise.NewService()
	triggeredAt := time.Now().UTC().Add(-11 * time.Minute).Truncate(time.Second)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	_, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     subscription.TenantID,
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			return enterprise.SyncWorkerAlertDeliveryResult{
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
		t.Fatalf("expected pending dispatch commit history to succeed: %v", err)
	}

	s := &server{
		enterpriseSvc: enterpriseSvc,
	}

	beforeRequest := time.Now().UTC()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts/notifications/export-csv?tenant_id=tenant_demo_jakarta&status=failed&reason=dispatch_commit_unknown",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.exportEnterpriseSyncWorkerAlertNotificationsCSV(recorder, request)
	afterRequest := time.Now().UTC()

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	records, err := csv.NewReader(strings.NewReader(recorder.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("expected valid csv body: %v body=%s", err, recorder.Body.String())
	}
	if len(records) != 2 {
		t.Fatalf("expected header and one pending row, got %+v", records)
	}
	header := records[0]
	row := records[1]
	pendingAgeIndex := -1
	reasonIndex := -1
	for i := range header {
		switch header[i] {
		case "pending_age_seconds":
			pendingAgeIndex = i
		case "reason":
			reasonIndex = i
		}
	}
	if pendingAgeIndex == -1 {
		t.Fatalf("expected pending_age_seconds column, got %+v", header)
	}
	if reasonIndex == -1 {
		t.Fatalf("expected reason column, got %+v", header)
	}
	if row[reasonIndex] != "dispatch_commit_unknown" {
		t.Fatalf("expected pending dispatch reason in csv row, got %+v", row)
	}
	pendingAgeSeconds, err := strconv.ParseInt(row[pendingAgeIndex], 10, 64)
	if err != nil {
		t.Fatalf("expected numeric pending_age_seconds, got %q err=%v", row[pendingAgeIndex], err)
	}
	minAge := int64(beforeRequest.Sub(triggeredAt).Seconds())
	maxAge := int64(afterRequest.Sub(triggeredAt).Seconds()) + 1
	if pendingAgeSeconds < minAge || pendingAgeSeconds > maxAge {
		t.Fatalf("expected csv pending_age_seconds in [%d,%d], got %d row=%+v", minAge, maxAge, pendingAgeSeconds, row)
	}
}

func TestExportEnterpriseSyncWorkerAlertNotificationsCSVRouteIncludesConfirmationObservability(t *testing.T) {
	enterpriseSvc := enterprise.NewService()
	triggeredAt := time.Date(2026, 4, 23, 7, 30, 0, 0, time.UTC)
	confirmedAt := triggeredAt.Add(9 * time.Minute)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	_, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     subscription.TenantID,
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			return enterprise.SyncWorkerAlertDeliveryResult{
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
		t.Fatalf("expected pending dispatch commit history to succeed: %v", err)
	}

	_, err = enterpriseSvc.ConfirmSyncWorkerAlertNotifications(enterprise.SyncWorkerAlertNotificationConfirmInput{
		TenantID:    subscription.TenantID,
		ConfirmedAt: confirmedAt,
		Confirm: func(input enterprise.SyncWorkerAlertConfirmationInput) enterprise.SyncWorkerAlertConfirmationResult {
			return enterprise.SyncWorkerAlertConfirmationResult{
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
		t.Fatalf("expected pending confirmation attempt to succeed: %v", err)
	}

	s := &server{
		enterpriseSvc: enterpriseSvc,
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alerts/notifications/export-csv?tenant_id=tenant_demo_jakarta&status=failed&reason=dispatch_commit_unknown",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.exportEnterpriseSyncWorkerAlertNotificationsCSV(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	records, err := csv.NewReader(strings.NewReader(recorder.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("expected valid csv body: %v body=%s", err, recorder.Body.String())
	}
	if len(records) != 2 {
		t.Fatalf("expected header and one pending row, got %+v", records)
	}
	header := records[0]
	row := records[1]
	confirmAttemptsIndex := -1
	lastConfirmAttemptAtIndex := -1
	lastConfirmResultIndex := -1
	for i := range header {
		switch header[i] {
		case "confirm_attempts":
			confirmAttemptsIndex = i
		case "last_confirm_attempt_at":
			lastConfirmAttemptAtIndex = i
		case "last_confirm_result":
			lastConfirmResultIndex = i
		}
	}
	if confirmAttemptsIndex == -1 || lastConfirmAttemptAtIndex == -1 || lastConfirmResultIndex == -1 {
		t.Fatalf("expected confirmation observability columns, got %+v", header)
	}
	confirmAttempts, err := strconv.Atoi(row[confirmAttemptsIndex])
	if err != nil {
		t.Fatalf("expected numeric confirm_attempts, got %q err=%v", row[confirmAttemptsIndex], err)
	}
	if confirmAttempts != 1 {
		t.Fatalf("expected confirm_attempts=1, got row=%+v", row)
	}
	if row[lastConfirmAttemptAtIndex] != confirmedAt.Format(time.RFC3339) {
		t.Fatalf("expected last_confirm_attempt_at=%s, got row=%+v", confirmedAt.Format(time.RFC3339), row)
	}
	if row[lastConfirmResultIndex] != "provider_error" {
		t.Fatalf("expected last_confirm_result=provider_error, got row=%+v", row)
	}
}

func TestRetryEnterpriseSyncWorkerAlertNotificationRoute(t *testing.T) {
	auditSvc := audit.NewService()
	enterpriseSvc := enterprise.NewService()
	walletSvc, sender := newObservableWalletAlertService(t)
	now := time.Date(2026, 4, 23, 3, 30, 0, 0, time.UTC)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	_, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			return enterprise.SyncWorkerAlertDeliveryResult{
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

	history := enterpriseSvc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) != 1 {
		t.Fatalf("expected one notification before retry, got %d", len(history))
	}

	s := &server{
		auditSvc:      auditSvc,
		enterpriseSvc: enterpriseSvc,
		walletSvc:     walletSvc,
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-worker-alerts/notifications/"+history[0].ID+"/retry",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_jakarta"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withURLParam(request, "notificationID", history[0].ID)
	request = withAuthUser(request, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
		Email:    "tenant.admin@sudirman.co",
	})
	recorder := httptest.NewRecorder()

	s.retryEnterpriseSyncWorkerAlertNotification(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected retry route 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertNotification
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid retry payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.Status != "sent" || payload.Attempt != 2 {
		t.Fatalf("expected sent retry payload with second attempt, got %+v", payload)
	}
	if payload.SourceNotificationID != history[0].ID {
		t.Fatalf("expected source notification reference, got %+v", payload)
	}
	assertObservedWalletAlertIdempotencyKeys(t, sender, []string{payload.IdempotencyKey})

	reloaded := enterpriseSvc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(reloaded) != 2 || reloaded[0].ID != payload.ID {
		t.Fatalf("expected retry history to prepend new record, got %+v", reloaded)
	}

	logs := auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_sync_worker_alert_notification_retried", "enterprise_sync", 10)
	if len(logs) != 1 {
		t.Fatalf("expected one retry audit log, got %d", len(logs))
	}

	conflictRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-worker-alerts/notifications/"+payload.ID+"/retry",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_jakarta"}`),
	)
	conflictRequest.Header.Set("Content-Type", "application/json")
	conflictRequest = withURLParam(conflictRequest, "notificationID", payload.ID)
	conflictRequest = withAuthUser(conflictRequest, auth.User{Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})
	conflictRecorder := httptest.NewRecorder()

	s.retryEnterpriseSyncWorkerAlertNotification(conflictRecorder, conflictRequest)

	if conflictRecorder.Code != http.StatusConflict {
		t.Fatalf("expected sent notification retry to return 409, got %d body=%s", conflictRecorder.Code, conflictRecorder.Body.String())
	}
}

func TestBatchRetryEnterpriseSyncWorkerAlertNotificationsRoute(t *testing.T) {
	auditSvc := audit.NewService()
	enterpriseSvc := enterprise.NewService()
	walletSvc, sender := newObservableWalletAlertService(t)
	now := time.Date(2026, 4, 23, 4, 0, 0, 0, time.UTC)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			return enterprise.SyncWorkerAlertDeliveryResult{
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
	retriedFailure, err := enterpriseSvc.RetrySyncWorkerAlertNotification(enterprise.SyncWorkerAlertNotificationRetryInput{
		TenantID:       "tenant_demo_jakarta",
		NotificationID: initial.Items[0].ID,
		RetriedAt:      now.Add(2 * time.Minute),
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			return enterprise.SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
			}
		},
	})
	if err != nil {
		t.Fatalf("expected failed retry to succeed: %v", err)
	}

	s := &server{
		auditSvc:      auditSvc,
		enterpriseSvc: enterpriseSvc,
		walletSvc:     walletSvc,
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-worker-alerts/notifications/retry-batch",
		bytes.NewBufferString(
			fmt.Sprintf(
				`{"tenant_id":"tenant_demo_jakarta","notification_ids":["%s","%s","%s"]}`,
				retriedFailure.ID,
				initial.Items[0].ID,
				initial.Items[1].ID,
			),
		),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withAuthUser(request, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
		Email:    "tenant.admin@sudirman.co",
	})
	recorder := httptest.NewRecorder()

	s.retryEnterpriseSyncWorkerAlertNotificationsBatch(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected batch retry route 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertNotificationBatchRetryResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid batch retry payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.TotalNotifications != 3 || payload.Retried != 2 || payload.Suppressed != 1 || payload.Failed != 0 || payload.Skipped != 0 {
		t.Fatalf("unexpected batch retry summary: %+v", payload)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected two concrete retry records, got %+v", payload.Items)
	}
	assertObservedWalletAlertIdempotencyKeys(t, sender, []string{
		payload.Items[0].IdempotencyKey,
		payload.Items[1].IdempotencyKey,
	})

	history := enterpriseSvc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) != 5 {
		t.Fatalf("expected history to prepend batch retry records, got %d", len(history))
	}

	logs := auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_sync_worker_alert_notifications_batch_retried", "enterprise_sync", 10)
	if len(logs) != 1 {
		t.Fatalf("expected one batch retry audit log, got %d", len(logs))
	}
}

func TestBatchSuppressEnterpriseSyncWorkerAlertNotificationsRoute(t *testing.T) {
	auditSvc := audit.NewService()
	enterpriseSvc := enterprise.NewService()
	now := time.Date(2026, 4, 23, 4, 15, 0, 0, time.UTC)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			return enterprise.SyncWorkerAlertDeliveryResult{
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

	s := &server{
		auditSvc:      auditSvc,
		enterpriseSvc: enterpriseSvc,
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-worker-alerts/notifications/suppress-batch",
		bytes.NewBufferString(
			fmt.Sprintf(`{"tenant_id":"tenant_demo_jakarta","notification_ids":["%s"]}`, initial.Items[0].ID),
		),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withAuthUser(request, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
		Email:    "tenant.admin@sudirman.co",
	})
	recorder := httptest.NewRecorder()

	s.suppressEnterpriseSyncWorkerAlertNotificationsBatch(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected batch suppress route 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertNotificationBatchSuppressResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid batch suppress payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.TotalNotifications != 1 || payload.Suppressed != 1 || payload.Skipped != 0 || len(payload.Items) != 1 {
		t.Fatalf("unexpected batch suppress payload: %+v", payload)
	}
	if payload.Items[0].Status != "skipped" || payload.Items[0].Reason != "manual_suppressed" {
		t.Fatalf("expected manual_suppressed payload item, got %+v", payload.Items[0])
	}

	logs := auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_sync_worker_alert_notifications_batch_suppressed", "enterprise_sync", 10)
	if len(logs) != 1 {
		t.Fatalf("expected one batch suppress audit log, got %d", len(logs))
	}
}

func TestBatchRestoreEnterpriseSyncWorkerAlertNotificationsRoute(t *testing.T) {
	auditSvc := audit.NewService()
	enterpriseSvc := enterprise.NewService()
	now := time.Date(2026, 4, 23, 4, 20, 0, 0, time.UTC)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
				WorkerLabel:  "HRIS Webhook Sync",
				Count:        4,
				Threshold:    2,
				Failed:       3,
				Processed:    4,
				Applied:      1,
				ConnectorID:  "connector-talenta-002",
				Vendor:       "talenta",
				FailureStage: "sync",
			},
		},
		TriggeredAt: now,
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			return enterprise.SyncWorkerAlertDeliveryResult{
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
	if len(initial.Items) != 2 {
		t.Fatalf("expected two initial failed notifications, got %d", len(initial.Items))
	}
	suppressed, err := enterpriseSvc.BatchSuppressSyncWorkerAlertNotifications(enterprise.SyncWorkerAlertNotificationBatchSuppressInput{
		TenantID:        "tenant_demo_jakarta",
		NotificationIDs: []string{initial.Items[0].ID, initial.Items[1].ID},
		SuppressedAt:    now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("expected batch suppress to succeed: %v", err)
	}
	if len(suppressed.Items) != 2 {
		t.Fatalf("expected two suppressed notifications, got %d", len(suppressed.Items))
	}
	staleSuppressedID := suppressed.Items[0].ID
	readySuppressedID := suppressed.Items[1].ID
	if _, err := enterpriseSvc.BatchRestoreSyncWorkerAlertNotifications(enterprise.SyncWorkerAlertNotificationBatchRestoreInput{
		TenantID:        "tenant_demo_jakarta",
		NotificationIDs: []string{staleSuppressedID},
		RestoredAt:      now.Add(3 * time.Minute),
	}); err != nil {
		t.Fatalf("expected initial restore to create newer history: %v", err)
	}

	s := &server{
		auditSvc:      auditSvc,
		enterpriseSvc: enterpriseSvc,
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-worker-alerts/notifications/restore-batch",
		bytes.NewBufferString(
			fmt.Sprintf(
				`{"tenant_id":"tenant_demo_jakarta","notification_ids":["%s","%s"]}`,
				staleSuppressedID,
				readySuppressedID,
			),
		),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withAuthUser(request, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
		Email:    "tenant.admin@sudirman.co",
	})
	recorder := httptest.NewRecorder()

	s.restoreEnterpriseSyncWorkerAlertNotificationsBatch(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected batch restore route 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertNotificationBatchRestoreResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid batch restore payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.TotalNotifications != 2 || payload.Restored != 1 || payload.Skipped != 1 || len(payload.Items) != 1 {
		t.Fatalf("unexpected batch restore payload: %+v", payload)
	}
	if payload.Items[0].Status != "failed" || payload.Items[0].Reason != "manual_suppressed_restored" || !payload.Items[0].Retryable {
		t.Fatalf("expected restored retryable failed payload item, got %+v", payload.Items[0])
	}
	if payload.Items[0].SourceNotificationID != readySuppressedID {
		t.Fatalf("expected route restore to recover ready suppression %s, got %+v", readySuppressedID, payload.Items[0])
	}

	logs := auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_sync_worker_alert_notifications_batch_restored", "enterprise_sync", 10)
	if len(logs) != 1 {
		t.Fatalf("expected one batch restore audit log, got %d", len(logs))
	}
	target := parseAuditTargetKeyValues(logs[0].Target)
	if target["total_notifications"] != "2" || target["restored"] != "1" || target["skipped"] != "1" {
		t.Fatalf("unexpected batch restore audit metrics: %s", logs[0].Target)
	}
	if target["restored_source_ids"] != readySuppressedID {
		t.Fatalf("expected restored_source_ids=%s, got %s", readySuppressedID, target["restored_source_ids"])
	}
	if target["restored_notification_ids"] != payload.Items[0].ID {
		t.Fatalf("expected restored_notification_ids=%s, got %s", payload.Items[0].ID, target["restored_notification_ids"])
	}
	if target["skipped_details"] != staleSuppressedID+":newer_history_exists" {
		t.Fatalf("unexpected skipped_details target: %s", logs[0].Target)
	}
	if target["actor"] != "tenant.admin@sudirman.co" {
		t.Fatalf("expected batch restore actor in audit target, got %s", target["actor"])
	}
}

func TestAutoRetryEnterpriseSyncWorkerAlertNotificationsRoute(t *testing.T) {
	auditSvc := audit.NewService()
	enterpriseSvc := enterprise.NewService()
	walletSvc, sender := newObservableWalletAlertService(t)
	now := time.Now().UTC().Add(-30 * time.Minute)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			return enterprise.SyncWorkerAlertDeliveryResult{
				Status:        "failed",
				Reason:        "provider_transient_error",
				Provider:      "mock",
				ProviderError: "temporary outage",
				Retryable:     true,
				ChannelResults: []wallet.JobAlertChannelResult{
					{
						Channel: "email",
						Status:  "failed",
						Reason:  "provider_transient_error",
					},
				},
			}
		},
	})
	if err != nil {
		t.Fatalf("expected initial failed dispatch to succeed: %v", err)
	}
	if len(initial.Items) != 1 || initial.Items[0].NextRetryAt == nil {
		t.Fatalf("expected one due retryable notification, got %+v", initial.Items)
	}

	s := &server{
		auditSvc:      auditSvc,
		enterpriseSvc: enterpriseSvc,
		walletSvc:     walletSvc,
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/sync-worker-alerts/notifications/auto-retry",
		bytes.NewBufferString(`{"tenant_id":"tenant_demo_jakarta","limit":10,"max_attempts":3}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withAuthUser(request, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
		Email:    "tenant.admin@sudirman.co",
	})
	recorder := httptest.NewRecorder()

	s.autoRetryEnterpriseSyncWorkerAlertNotifications(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected auto retry route 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertNotificationAutoRetryResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid auto retry payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.TotalNotifications != 1 || payload.Retried != 1 || payload.Failed != 0 || payload.Skipped != 0 || payload.Suppressed != 0 {
		t.Fatalf("unexpected auto retry payload: %+v", payload)
	}
	if len(payload.Items) != 1 || payload.Items[0].Status != "sent" {
		t.Fatalf("expected one sent retry item, got %+v", payload.Items)
	}
	assertObservedWalletAlertIdempotencyKeys(t, sender, []string{payload.Items[0].IdempotencyKey})

	logs := auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_sync_worker_alert_notifications_auto_retried", "enterprise_sync", 10)
	if len(logs) != 1 {
		t.Fatalf("expected one auto retry audit log, got %d", len(logs))
	}
}

func TestEnterpriseSyncWorkerAlertAutoRetryWorkerTick(t *testing.T) {
	auditSvc := audit.NewService()
	enterpriseSvc := enterprise.NewService()
	walletSvc := wallet.NewService()
	now := time.Now().UTC().Add(-30 * time.Minute)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			return enterprise.SyncWorkerAlertDeliveryResult{
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
		t.Fatalf("expected one retryable notification with next_retry_at, got %+v", initial.Items)
	}

	s := &server{
		auditSvc:      auditSvc,
		enterpriseSvc: enterpriseSvc,
		walletSvc:     walletSvc,
	}
	s.runEnterpriseSyncWorkerAlertAutoRetryWorkerTick(20, 3, 5*time.Minute, time.Hour)

	history := enterpriseSvc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) < 2 || history[0].Status != "sent" {
		t.Fatalf("expected worker tick to prepend sent retry history, got %+v", history)
	}

	logs := auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_sync_worker_alert_auto_retry_worker_completed", "enterprise_sync_worker", 10)
	if len(logs) != 1 {
		t.Fatalf("expected one auto retry worker audit log, got %d", len(logs))
	}
}

func TestEnterpriseSyncWorkerAlertAutoRetryWorkerTickWithLeaseRunsWhenAcquired(t *testing.T) {
	auditSvc := audit.NewService()
	enterpriseSvc := enterprise.NewService()
	walletSvc := wallet.NewService()
	leaseStore := &stubWorkerLeaseStore{acquireOK: true}
	now := time.Now().UTC().Add(-30 * time.Minute)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			return enterprise.SyncWorkerAlertDeliveryResult{
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
		t.Fatalf("expected one retryable notification with next_retry_at, got %+v", initial.Items)
	}

	s := &server{
		auditSvc:         auditSvc,
		enterpriseSvc:    enterpriseSvc,
		walletSvc:        walletSvc,
		workerLeaseStore: leaseStore,
	}
	s.runEnterpriseSyncWorkerAlertAutoRetryWorkerTickWithLease(20, 3, 5*time.Minute, time.Hour, 10*time.Minute)

	history := enterpriseSvc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) < 2 || history[0].Status != "sent" {
		t.Fatalf("expected leased worker tick to prepend sent retry history, got %+v", history)
	}
	if leaseStore.acquireCalls != 1 || leaseStore.releaseCalls != 1 {
		t.Fatalf("expected one lease acquire/release, got acquire=%d release=%d", leaseStore.acquireCalls, leaseStore.releaseCalls)
	}
	if leaseStore.lastKey != enterpriseSyncWorkerAlertAutoRetryLeaseKey {
		t.Fatalf("unexpected lease key: %s", leaseStore.lastKey)
	}
	logs := auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_sync_worker_alert_auto_retry_worker_completed", "enterprise_sync_worker", 10)
	if len(logs) != 1 {
		t.Fatalf("expected one leased auto retry worker audit log, got %d", len(logs))
	}
}

func TestEnterpriseSyncWorkerAlertAutoRetryWorkerTickWithLeaseSkipsWhenUnavailable(t *testing.T) {
	auditSvc := audit.NewService()
	enterpriseSvc := enterprise.NewService()
	walletSvc := wallet.NewService()
	leaseStore := &stubWorkerLeaseStore{acquireOK: false}
	now := time.Now().UTC().Add(-30 * time.Minute)
	subscription := enterprise.SyncWorkerAlertSubscription{
		TenantID:             "tenant_demo_jakarta",
		Enabled:              true,
		WorkerAlertThreshold: 2,
		WindowSeconds:        int64((15 * time.Minute).Seconds()),
		CooldownSeconds:      int64((15 * time.Minute).Seconds()),
		Channels: enterprise.SyncWorkerAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}

	initial, err := enterpriseSvc.DispatchSyncWorkerAlerts(enterprise.SyncWorkerAlertDispatchInput{
		TenantID:     "tenant_demo_jakarta",
		Subscription: subscription,
		Alerts: []enterprise.SyncWorkerAlertDispatchAlert{
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
		Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
			return enterprise.SyncWorkerAlertDeliveryResult{
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
		t.Fatalf("expected one retryable notification with next_retry_at, got %+v", initial.Items)
	}

	s := &server{
		auditSvc:         auditSvc,
		enterpriseSvc:    enterpriseSvc,
		walletSvc:        walletSvc,
		workerLeaseStore: leaseStore,
	}
	s.runEnterpriseSyncWorkerAlertAutoRetryWorkerTickWithLease(20, 3, 5*time.Minute, time.Hour, 10*time.Minute)

	history := enterpriseSvc.ListSyncWorkerAlertNotifications("tenant_demo_jakarta", 10)
	if len(history) == 0 || history[0].Status != "failed" {
		t.Fatalf("expected lease miss to preserve failed history, got %+v", history)
	}
	if leaseStore.acquireCalls != 1 || leaseStore.releaseCalls != 0 {
		t.Fatalf("expected one lease acquire and no release on lease miss, got acquire=%d release=%d", leaseStore.acquireCalls, leaseStore.releaseCalls)
	}
	logs := auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_sync_worker_alert_auto_retry_worker_completed", "enterprise_sync_worker", 10)
	if len(logs) != 0 {
		t.Fatalf("expected lease miss to avoid worker completion audit, got %d", len(logs))
	}
}
