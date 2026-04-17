package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
)

func TestAuditWebhookConfigUpsertAndGet(t *testing.T) {
	s := &server{
		auditSvc: audit.NewService(),
	}

	upsertBody, _ := json.Marshal(map[string]any{
		"tenant_id":  "tenant_demo_jakarta",
		"enabled":    true,
		"endpoint":   "https://example.com/hooks/audit",
		"actions":    []string{"gateway_reboot", "tenant_update"},
		"updated_by": "qa",
	})
	upsertReq := httptest.NewRequest(http.MethodPut, "/api/v1/audit/webhook/config", bytes.NewReader(upsertBody))
	upsertReq.Header.Set("Content-Type", "application/json")
	upsertReq = withAuthUser(upsertReq, auth.User{Role: "super_admin"})
	upsertRec := httptest.NewRecorder()
	s.upsertAuditWebhookConfig(upsertRec, upsertReq)
	if upsertRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", upsertRec.Code, upsertRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/audit/webhook/config?tenant_id=tenant_demo_jakarta", nil)
	getReq = withAuthUser(getReq, auth.User{Role: "super_admin"})
	getRec := httptest.NewRecorder()
	s.getAuditWebhookConfig(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}

	var config struct {
		TenantID string `json:"tenant_id"`
		Enabled  bool   `json:"enabled"`
		Endpoint string `json:"endpoint"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &config); err != nil {
		t.Fatalf("failed decoding config response: %v", err)
	}
	if config.TenantID != "tenant_demo_jakarta" || !config.Enabled {
		t.Fatalf("unexpected config payload: %+v", config)
	}
	if config.Endpoint != "https://example.com/hooks/audit" {
		t.Fatalf("unexpected endpoint: %s", config.Endpoint)
	}
}

func TestDispatchAuditWebhookSuccess(t *testing.T) {
	received := false
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer webhookServer.Close()

	auditSvc := audit.NewService()
	_, err := auditSvc.UpsertWebhookConfig(
		"tenant_demo_jakarta",
		true,
		webhookServer.URL,
		nil,
		"qa",
	)
	if err != nil {
		t.Fatalf("expected webhook config upsert to succeed: %v", err)
	}

	s := &server{
		auditSvc: auditSvc,
	}

	dispatchBody, _ := json.Marshal(map[string]any{
		"tenant_id":    "tenant_demo_jakarta",
		"audit_log_id": "aud_3002",
	})
	dispatchReq := httptest.NewRequest(http.MethodPost, "/api/v1/audit/webhook/dispatch", bytes.NewReader(dispatchBody))
	dispatchReq.Header.Set("Content-Type", "application/json")
	dispatchReq = withAuthUser(dispatchReq, auth.User{Role: "super_admin"})
	dispatchRec := httptest.NewRecorder()
	s.dispatchAuditWebhook(dispatchRec, dispatchReq)
	if dispatchRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", dispatchRec.Code, dispatchRec.Body.String())
	}
	if !received {
		t.Fatalf("expected webhook server to receive request")
	}

	var response struct {
		Delivery struct {
			Status string `json:"status"`
		} `json:"delivery"`
	}
	if err := json.Unmarshal(dispatchRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed decoding dispatch response: %v", err)
	}
	if response.Delivery.Status != "success" {
		t.Fatalf("unexpected delivery status: %s", response.Delivery.Status)
	}
}

func TestDispatchAuditWebhookDisabled(t *testing.T) {
	auditSvc := audit.NewService()
	_, err := auditSvc.UpsertWebhookConfig(
		"tenant_demo_jakarta",
		false,
		"https://example.com/hooks/audit",
		nil,
		"qa",
	)
	if err != nil {
		t.Fatalf("expected webhook config upsert to succeed: %v", err)
	}

	s := &server{
		auditSvc: auditSvc,
	}

	dispatchBody, _ := json.Marshal(map[string]any{
		"tenant_id":    "tenant_demo_jakarta",
		"audit_log_id": "aud_3002",
	})
	dispatchReq := httptest.NewRequest(http.MethodPost, "/api/v1/audit/webhook/dispatch", bytes.NewReader(dispatchBody))
	dispatchReq.Header.Set("Content-Type", "application/json")
	dispatchReq = withAuthUser(dispatchReq, auth.User{Role: "super_admin"})
	dispatchRec := httptest.NewRecorder()
	s.dispatchAuditWebhook(dispatchRec, dispatchReq)
	if dispatchRec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", dispatchRec.Code, dispatchRec.Body.String())
	}
}

func withAuthUser(request *http.Request, user auth.User) *http.Request {
	ctx := context.WithValue(request.Context(), authUserContextKey, user)
	return request.WithContext(ctx)
}
