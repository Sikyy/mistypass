package audit

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUpsertAndGetWebhookConfig(t *testing.T) {
	svc := NewService()

	config, err := svc.UpsertWebhookConfig(
		"tenant_demo_jakarta",
		true,
		"https://example.com/hooks/audit",
		[]string{"gateway_reboot", "gateway_reboot", "tenant_update"},
		"",
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
		"",
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
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
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
		"",
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
	if delivery.AttemptCount != webhookDispatchMaxAttempts {
		t.Fatalf("expected max attempts=%d, got %d", webhookDispatchMaxAttempts, delivery.AttemptCount)
	}
	if requestCount != webhookDispatchMaxAttempts {
		t.Fatalf("expected request count=%d, got %d", webhookDispatchMaxAttempts, requestCount)
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
		"",
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

func TestDispatchWebhookForLogRetriesAndEventuallySucceeds(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"transient"}`))
			return
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
		"",
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
	if delivery.Status != "success" {
		t.Fatalf("expected success status, got %s", delivery.Status)
	}
	if delivery.AttemptCount != 3 {
		t.Fatalf("expected attempt_count=3, got %d", delivery.AttemptCount)
	}
	if requestCount != 3 {
		t.Fatalf("expected request count=3, got %d", requestCount)
	}
}

func TestDispatchWebhookForLogDoesNotRetryOnBadRequest(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid"}`))
	}))
	defer server.Close()

	svc := NewService()
	_, err := svc.UpsertWebhookConfig(
		"tenant_demo_jakarta",
		true,
		server.URL,
		nil,
		"",
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
	if delivery.AttemptCount != 1 {
		t.Fatalf("expected attempt_count=1, got %d", delivery.AttemptCount)
	}
	if requestCount != 1 {
		t.Fatalf("expected request count=1, got %d", requestCount)
	}
}

func TestDispatchWebhookForLogIncludesSignatureHeaders(t *testing.T) {
	var signatureHeader string
	var timestampHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signatureHeader = strings.TrimSpace(r.Header.Get(webhookSignatureHeader))
		timestampHeader = strings.TrimSpace(r.Header.Get(webhookSignatureTimestampHeader))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	svc := NewService()
	secret := "audit-webhook-secret-001"
	_, err := svc.UpsertWebhookConfig(
		"tenant_demo_jakarta",
		true,
		server.URL,
		[]string{"gateway_reboot"},
		secret,
		"qa",
	)
	if err != nil {
		t.Fatalf("expected upsert webhook config success: %v", err)
	}

	logs := svc.ListFiltered("tenant_demo_jakarta", "gateway_reboot", "", 1)
	if len(logs) != 1 {
		t.Fatalf("expected one gateway_reboot log, got %d", len(logs))
	}
	if _, err := svc.DispatchWebhookForLog(context.Background(), "tenant_demo_jakarta", logs[0], server.Client()); err != nil {
		t.Fatalf("expected webhook dispatch success: %v", err)
	}

	if signatureHeader == "" {
		t.Fatalf("expected signature header")
	}
	if !strings.HasPrefix(signatureHeader, webhookSignatureAlgorithm+"=") {
		t.Fatalf("unexpected signature header format: %s", signatureHeader)
	}
	if timestampHeader == "" {
		t.Fatalf("expected signature timestamp header")
	}
}

func TestRecordWebhookDeliveryCapsStoredRecords(t *testing.T) {
	svc := NewService()
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)

	for i := 0; i < webhookDeliveryMaxRecords+17; i++ {
		svc.recordWebhookDelivery(WebhookDelivery{
			TenantID:     "tenant_demo_jakarta",
			ID:           fmt.Sprintf("awd_test_%04d", i),
			AuditLogID:   fmt.Sprintf("aud_%04d", i),
			Action:       "gateway_reboot",
			Endpoint:     "https://example.com/hooks/audit",
			Status:       "success",
			DispatchedAt: now.Add(time.Duration(i) * time.Second),
		})
	}

	list := svc.ListWebhookDeliveries("tenant_demo_jakarta", 0)
	if len(list) != webhookDeliveryMaxRecords {
		t.Fatalf("expected delivery list capped at %d, got %d", webhookDeliveryMaxRecords, len(list))
	}
	if list[0].ID != "awd_test_1016" {
		t.Fatalf("expected newest delivery to be retained first, got %s", list[0].ID)
	}
	if list[len(list)-1].ID != "awd_test_0017" {
		t.Fatalf("expected oldest retained delivery to be awd_test_0017, got %s", list[len(list)-1].ID)
	}
}
