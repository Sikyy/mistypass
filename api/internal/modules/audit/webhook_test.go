package audit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpsertAndGetWebhookConfig(t *testing.T) {
	svc := NewService()

	config, err := svc.UpsertWebhookConfig(
		"tenant_demo_jakarta",
		true,
		"https://example.com/hooks/audit",
		[]string{"gateway_reboot", "gateway_reboot", "tenant_update"},
		"qa",
	)
	if err != nil {
		t.Fatalf("expected upsert webhook config success: %v", err)
	}
	if !config.Enabled {
		t.Fatalf("expected enabled config")
	}
	if len(config.Actions) != 2 {
		t.Fatalf("expected deduplicated actions, got %+v", config.Actions)
	}

	got, err := svc.GetWebhookConfig("tenant_demo_jakarta")
	if err != nil {
		t.Fatalf("expected get webhook config success: %v", err)
	}
	if got.Endpoint != "https://example.com/hooks/audit" {
		t.Fatalf("unexpected endpoint: %s", got.Endpoint)
	}
}

func TestDispatchWebhookForLogSuccess(t *testing.T) {
	received := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("X-MistyPass-Event-ID") == "" {
			t.Fatalf("missing event id header")
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	svc := NewService()
	_, err := svc.UpsertWebhookConfig(
		"tenant_demo_jakarta",
		true,
		server.URL,
		[]string{"gateway_reboot"},
		"qa",
	)
	if err != nil {
		t.Fatalf("expected upsert webhook config success: %v", err)
	}

	logs := svc.ListFiltered("tenant_demo_jakarta", "gateway_reboot", "", 1)
	if len(logs) != 1 {
		t.Fatalf("expected one gateway_reboot log, got %d", len(logs))
	}

	delivery, err := svc.DispatchWebhookForLog(context.Background(), "tenant_demo_jakarta", logs[0], server.Client())
	if err != nil {
		t.Fatalf("expected webhook dispatch success: %v", err)
	}
	if !received {
		t.Fatalf("expected webhook receiver to be called")
	}
	if delivery.Status != "success" {
		t.Fatalf("unexpected delivery status: %s", delivery.Status)
	}
	if delivery.HTTPStatus != http.StatusAccepted {
		t.Fatalf("unexpected delivery http_status: %d", delivery.HTTPStatus)
	}
	list := svc.ListWebhookDeliveries("tenant_demo_jakarta", 10)
	if len(list) != 1 {
		t.Fatalf("expected one delivery record, got %d", len(list))
	}
}

func TestDispatchWebhookForLogFailureRecorded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer server.Close()

	svc := NewService()
	_, err := svc.UpsertWebhookConfig(
		"tenant_demo_jakarta",
		true,
		server.URL,
		nil,
		"qa",
	)
	if err != nil {
		t.Fatalf("expected upsert webhook config success: %v", err)
	}

	logs := svc.ListFiltered("tenant_demo_jakarta", "", "", 1)
	if len(logs) == 0 {
		t.Fatalf("expected seeded audit logs")
	}

	delivery, err := svc.DispatchWebhookForLog(context.Background(), "tenant_demo_jakarta", logs[0], server.Client())
	if err == nil {
		t.Fatalf("expected webhook dispatch failure")
	}
	if delivery.Status != "failed" {
		t.Fatalf("expected failed delivery status, got %s", delivery.Status)
	}
	if delivery.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("unexpected failed http_status: %d", delivery.HTTPStatus)
	}
	if !strings.Contains(strings.ToLower(delivery.Error), "status") {
		t.Fatalf("unexpected error field: %s", delivery.Error)
	}
	list := svc.ListWebhookDeliveries("tenant_demo_jakarta", 10)
	if len(list) != 1 {
		t.Fatalf("expected failed delivery to be recorded, got %d", len(list))
	}
}

func TestDispatchWebhookForLogActionFiltered(t *testing.T) {
	svc := NewService()
	_, err := svc.UpsertWebhookConfig(
		"tenant_demo_jakarta",
		true,
		"https://example.com/hooks/audit",
		[]string{"tenant_update"},
		"qa",
	)
	if err != nil {
		t.Fatalf("expected upsert webhook config success: %v", err)
	}

	logs := svc.ListFiltered("tenant_demo_jakarta", "gateway_reboot", "", 1)
	if len(logs) != 1 {
		t.Fatalf("expected one gateway_reboot log")
	}

	_, err = svc.DispatchWebhookForLog(context.Background(), "tenant_demo_jakarta", logs[0], nil)
	if err != ErrWebhookActionFiltered {
		t.Fatalf("expected ErrWebhookActionFiltered, got %v", err)
	}
	if len(svc.ListWebhookDeliveries("tenant_demo_jakarta", 10)) != 0 {
		t.Fatalf("filtered events should not produce delivery record")
	}
}
