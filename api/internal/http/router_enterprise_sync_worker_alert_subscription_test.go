package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

type enterpriseTestStateStore struct {
	items map[string][]byte
}

func (s *enterpriseTestStateStore) Load(key string, dst any) (bool, error) {
	payload, ok := s.items[key]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(payload, dst); err != nil {
		return false, err
	}
	return true, nil
}

func (s *enterpriseTestStateStore) Save(key string, value any) error {
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

func TestGetEnterpriseSyncWorkerAlertSubscriptionDefault(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/enterprise/sync-worker-alert-subscription", nil)
	request = withAuthUser(request, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
	})
	recorder := httptest.NewRecorder()

	s.getEnterpriseSyncWorkerAlertSubscription(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertSubscription
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid subscription payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.TenantID != "tenant_demo_jakarta" {
		t.Fatalf("unexpected tenant_id: %s", payload.TenantID)
	}
	if !payload.Enabled || payload.WorkerAlertThreshold != 3 {
		t.Fatalf("unexpected default subscription: %+v", payload)
	}
	if payload.WindowSeconds != 900 || payload.CooldownSeconds != 900 {
		t.Fatalf("unexpected default timings: %+v", payload)
	}
	if !payload.Channels.Email || payload.Channels.WhatsApp {
		t.Fatalf("unexpected default channels: %+v", payload.Channels)
	}
	if len(payload.ReceiverGroups) != 1 || payload.ReceiverGroups[0] != "security" {
		t.Fatalf("unexpected default receiver_groups: %+v", payload.ReceiverGroups)
	}
	if payload.UpdatedAt.IsZero() {
		t.Fatalf("expected updated_at to be set")
	}
}

func TestUpsertEnterpriseSyncWorkerAlertSubscriptionPersistsAndRestores(t *testing.T) {
	store := &enterpriseTestStateStore{}
	enterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected service with state store to initialize: %v", err)
	}
	s := &server{
		enterpriseSvc: enterpriseSvc,
		auditSvc:      audit.NewService(),
	}

	requestBody, _ := json.Marshal(map[string]any{
		"enabled":                true,
		"worker_alert_threshold": 5,
		"window_seconds":         600,
		"cooldown_seconds":       1200,
		"receiver_groups":        []string{"security", "ops"},
		"channels": map[string]any{
			"email":    true,
			"whatsapp": true,
		},
	})
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/enterprise/sync-worker-alert-subscription",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
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

	var upserted enterprise.SyncWorkerAlertSubscription
	if err := json.Unmarshal(recorder.Body.Bytes(), &upserted); err != nil {
		t.Fatalf("expected valid upsert payload: %v body=%s", err, recorder.Body.String())
	}
	if upserted.WorkerAlertThreshold != 5 || upserted.WindowSeconds != 600 || upserted.CooldownSeconds != 1200 {
		t.Fatalf("unexpected persisted timings: %+v", upserted)
	}
	if !upserted.Channels.Email || !upserted.Channels.WhatsApp {
		t.Fatalf("unexpected persisted channels: %+v", upserted.Channels)
	}
	if len(upserted.ReceiverGroups) != 2 || upserted.ReceiverGroups[1] != "ops" {
		t.Fatalf("unexpected persisted receiver_groups: %+v", upserted.ReceiverGroups)
	}

	logs := s.auditSvc.ListFiltered(
		"tenant_demo_jakarta",
		"enterprise_sync_worker_alert_subscription_upserted",
		"enterprise_sync",
		10,
	)
	if len(logs) != 1 {
		t.Fatalf("expected one subscription audit log, got %d", len(logs))
	}

	restoredSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored service to initialize: %v", err)
	}
	restoredServer := &server{
		enterpriseSvc: restoredSvc,
	}
	getRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alert-subscription",
		nil,
	)
	getRequest = withAuthUser(getRequest, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
	})
	getRecorder := httptest.NewRecorder()

	restoredServer.getEnterpriseSyncWorkerAlertSubscription(getRecorder, getRequest)

	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from restored get, got %d body=%s", getRecorder.Code, getRecorder.Body.String())
	}

	var restored enterprise.SyncWorkerAlertSubscription
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &restored); err != nil {
		t.Fatalf("expected valid restored payload: %v body=%s", err, getRecorder.Body.String())
	}
	if restored.WorkerAlertThreshold != 5 || restored.WindowSeconds != 600 || restored.CooldownSeconds != 1200 {
		t.Fatalf("unexpected restored timings: %+v", restored)
	}
	if len(restored.ReceiverGroups) != 2 || restored.ReceiverGroups[0] != "security" || restored.ReceiverGroups[1] != "ops" {
		t.Fatalf("unexpected restored receiver_groups: %+v", restored.ReceiverGroups)
	}
}

func TestUpsertEnterpriseSyncWorkerAlertSubscriptionPartialUpdateInheritsCurrent(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
	}

	initialBody, _ := json.Marshal(map[string]any{
		"enabled":                true,
		"worker_alert_threshold": 8,
		"window_seconds":         1800,
		"cooldown_seconds":       300,
		"receiver_groups":        []string{"security", "ops"},
		"channels": map[string]any{
			"email":    true,
			"whatsapp": false,
		},
	})
	initialRequest := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/enterprise/sync-worker-alert-subscription",
		bytes.NewReader(initialBody),
	)
	initialRequest.Header.Set("Content-Type", "application/json")
	initialRequest = withAuthUser(initialRequest, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
	})
	initialRecorder := httptest.NewRecorder()

	s.upsertEnterpriseSyncWorkerAlertSubscription(initialRecorder, initialRequest)

	if initialRecorder.Code != http.StatusOK {
		t.Fatalf("expected initial upsert 200, got %d body=%s", initialRecorder.Code, initialRecorder.Body.String())
	}

	partialBody, _ := json.Marshal(map[string]any{
		"channels": map[string]any{
			"whatsapp": true,
		},
	})
	partialRequest := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/enterprise/sync-worker-alert-subscription",
		bytes.NewReader(partialBody),
	)
	partialRequest.Header.Set("Content-Type", "application/json")
	partialRequest = withAuthUser(partialRequest, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
	})
	partialRecorder := httptest.NewRecorder()

	s.upsertEnterpriseSyncWorkerAlertSubscription(partialRecorder, partialRequest)

	if partialRecorder.Code != http.StatusOK {
		t.Fatalf("expected partial upsert 200, got %d body=%s", partialRecorder.Code, partialRecorder.Body.String())
	}

	var payload enterprise.SyncWorkerAlertSubscription
	if err := json.Unmarshal(partialRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid partial payload: %v body=%s", err, partialRecorder.Body.String())
	}
	if payload.WorkerAlertThreshold != 8 || payload.WindowSeconds != 1800 || payload.CooldownSeconds != 300 {
		t.Fatalf("expected existing numeric values to be preserved, got %+v", payload)
	}
	if !payload.Channels.Email || !payload.Channels.WhatsApp {
		t.Fatalf("expected partial channel update to preserve email and enable whatsapp, got %+v", payload.Channels)
	}
	if len(payload.ReceiverGroups) != 2 || payload.ReceiverGroups[0] != "security" || payload.ReceiverGroups[1] != "ops" {
		t.Fatalf("expected receiver_groups to be preserved, got %+v", payload.ReceiverGroups)
	}
}

func TestUpsertEnterpriseSyncWorkerAlertSubscriptionRejectsInvalidConfig(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
	}

	requestBody, _ := json.Marshal(map[string]any{
		"enabled": true,
		"channels": map[string]any{
			"email":    false,
			"whatsapp": false,
		},
	})
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/enterprise/sync-worker-alert-subscription",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withAuthUser(request, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
	})
	recorder := httptest.NewRecorder()

	s.upsertEnterpriseSyncWorkerAlertSubscription(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), enterprise.ErrInvalidSyncWorkerAlertSubscriptionOptions.Error()) {
		t.Fatalf("expected validation error, got body=%s", recorder.Body.String())
	}
}

func TestGetEnterpriseSyncWorkerAlertSubscriptionTenantScopeForbidden(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/sync-worker-alert-subscription?tenant_id=tenant_other",
		nil,
	)
	request = withAuthUser(request, auth.User{
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
	})
	recorder := httptest.NewRecorder()

	s.getEnterpriseSyncWorkerAlertSubscription(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
