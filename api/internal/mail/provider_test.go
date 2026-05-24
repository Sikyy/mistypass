package mail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewResendProviderValidation(t *testing.T) {
	if _, err := NewResendProvider(ResendOptions{APIKey: "", From: "reports@mistypass.test"}); err == nil {
		t.Fatal("expected error when api key is missing")
	}
	if _, err := NewResendProvider(ResendOptions{APIKey: "re_test_key", From: ""}); err == nil {
		t.Fatal("expected error when from address is missing")
	}
}

func TestResendProviderSend(t *testing.T) {
	var capturedAuth string
	var capturedContentType string
	var capturedIdempotencyKey string
	var capturedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		capturedContentType = r.Header.Get("Content-Type")
		capturedIdempotencyKey = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&capturedPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"email_123"}`))
	}))
	defer server.Close()

	provider, err := NewResendProvider(ResendOptions{
		Endpoint: server.URL,
		APIKey:   "re_test_key",
		From:     "reports@mistypass.test",
		Timeout:  3 * time.Second,
	})
	if err != nil {
		t.Fatalf("new resend provider: %v", err)
	}

	receipt, err := provider.Send(context.Background(), Message{
		TenantID:       "tenant_demo_jakarta",
		To:             []string{"ops@sudirman.co"},
		IdempotencyKey: "report-email-001",
		Subject:        "weekly report",
		HTML:           "<p>attached</p>",
		Attachments: []Attachment{{
			Filename: "weekly.pdf",
			Content:  "JVBERi0x",
		}},
		Metadata: map[string]string{"schedule_id": "rs_000001"},
	})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if capturedAuth != "Bearer re_test_key" {
		t.Fatalf("unexpected auth header: %s", capturedAuth)
	}
	if capturedContentType != "application/json" {
		t.Fatalf("unexpected content type: %s", capturedContentType)
	}
	if capturedIdempotencyKey != "report-email-001" {
		t.Fatalf("unexpected idempotency key: %s", capturedIdempotencyKey)
	}
	if capturedPayload["from"] != "reports@mistypass.test" {
		t.Fatalf("unexpected from payload: %#v", capturedPayload["from"])
	}
	if capturedPayload["html"] != "<p>attached</p>" {
		t.Fatalf("unexpected html payload: %#v", capturedPayload["html"])
	}
	attachments, ok := capturedPayload["attachments"].([]any)
	if !ok || len(attachments) != 1 {
		t.Fatalf("expected one attachment, got %#v", capturedPayload["attachments"])
	}
	if receipt.Provider != "resend" || receipt.ProviderDeliveryID != "email_123" || receipt.ProviderDeliveryStatus != "accepted" {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
}

func TestResendProviderSendRetryableFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()

	provider, err := NewResendProvider(ResendOptions{
		Endpoint: server.URL,
		APIKey:   "re_test_key",
		From:     "reports@mistypass.test",
		Timeout:  3 * time.Second,
	})
	if err != nil {
		t.Fatalf("new resend provider: %v", err)
	}

	_, err = provider.Send(context.Background(), Message{
		To:      []string{"ops@sudirman.co"},
		Subject: "weekly report",
		Text:    "attached",
	})
	if err == nil {
		t.Fatal("expected send error")
	}
	httpErr, ok := err.(HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if !httpErr.Retryable() {
		t.Fatalf("expected retryable http error, got %+v", httpErr)
	}
}

func TestResendProviderConfirm(t *testing.T) {
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"email_123","last_event":"delivered"}`))
	}))
	defer server.Close()

	provider, err := NewResendProvider(ResendOptions{
		Endpoint: server.URL,
		APIKey:   "re_test_key",
		From:     "reports@mistypass.test",
	})
	if err != nil {
		t.Fatalf("new resend provider: %v", err)
	}

	confirmation, err := provider.Confirm(context.Background(), "email_123")
	if err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if capturedPath != "/email_123" {
		t.Fatalf("unexpected confirm path: %s", capturedPath)
	}
	if !confirmation.Confirmed {
		t.Fatalf("expected confirmed delivery, got %+v", confirmation)
	}
	if confirmation.ProviderDeliveryStatus != "delivered" {
		t.Fatalf("unexpected status: %+v", confirmation)
	}
}
