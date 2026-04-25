package wallet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewResendSenderValidation(t *testing.T) {
	if _, err := newResendSender("", "", "alerts@mistypass.test", 3*time.Second); err == nil {
		t.Fatalf("expected error when api key is missing")
	}
	if _, err := newResendSender("", "re_test_key", "", 3*time.Second); err == nil {
		t.Fatalf("expected error when from address is missing")
	}
}

func TestResendSenderSend(t *testing.T) {
	requestCount := 0
	var capturedAuth string
	var capturedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		capturedAuth = r.Header.Get("Authorization")
		capturedContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender, err := newResendSender(
		server.URL,
		"re_test_key",
		"alerts@mistypass.test",
		3*time.Second,
	)
	if err != nil {
		t.Fatalf("new resend sender failed: %v", err)
	}

	result, err := sender.Send(context.Background(), AlertEmailSendInput{
		TenantID: "tenant_demo_jakarta",
		To:       []string{"ops@sudirman.co"},
		Subject:  "wallet alert",
		Text:     "provider smoke",
	})
	if err != nil {
		t.Fatalf("resend sender send failed: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("expected 1 send request, got %d", requestCount)
	}
	if capturedAuth != "Bearer re_test_key" {
		t.Fatalf("unexpected auth header: %s", capturedAuth)
	}
	if capturedContentType != "application/json" {
		t.Fatalf("unexpected content type: %s", capturedContentType)
	}
	if result.ProviderDeliveryStatus != "accepted" {
		t.Fatalf("expected accepted send status, got %+v", result)
	}
}

func TestResendSenderSendRetryableFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()

	sender, err := newResendSender(
		server.URL,
		"re_test_key",
		"alerts@mistypass.test",
		3*time.Second,
	)
	if err != nil {
		t.Fatalf("new resend sender failed: %v", err)
	}

	_, err = sender.Send(context.Background(), AlertEmailSendInput{
		TenantID: "tenant_demo_jakarta",
		To:       []string{"ops@sudirman.co"},
		Subject:  "wallet alert",
		Text:     "provider smoke",
	})
	if err == nil {
		t.Fatalf("expected send error")
	}
	httpErr, ok := err.(AlertEmailHTTPError)
	if !ok {
		t.Fatalf("expected AlertEmailHTTPError, got %T", err)
	}
	if !httpErr.Retryable() {
		t.Fatalf("expected retryable http error, got %+v", httpErr)
	}
}

func TestResendSenderSendIncludesIdempotencyKeyHeader(t *testing.T) {
	const expectedKey = "wallet-email-idem-001"

	var capturedIdempotencyKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedIdempotencyKey = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender, err := newResendSender(
		server.URL,
		"re_test_key",
		"alerts@mistypass.test",
		3*time.Second,
	)
	if err != nil {
		t.Fatalf("new resend sender failed: %v", err)
	}

	input := AlertEmailSendInput{
		TenantID:       "tenant_demo_jakarta",
		To:             []string{"ops@sudirman.co"},
		IdempotencyKey: expectedKey,
		Subject:        "wallet alert",
		Text:           "provider smoke",
	}

	result, err := sender.Send(context.Background(), input)
	if err != nil {
		t.Fatalf("resend sender send failed: %v", err)
	}
	if capturedIdempotencyKey != expectedKey {
		t.Fatalf("expected Idempotency-Key header %q, got %q", expectedKey, capturedIdempotencyKey)
	}
	if result.ProviderDeliveryStatus != "accepted" {
		t.Fatalf("expected accepted send status, got %+v", result)
	}
}

func TestResendSenderSendReturnsProviderDeliveryID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"email_123"}`))
	}))
	defer server.Close()

	sender, err := newResendSender(
		server.URL,
		"re_test_key",
		"alerts@mistypass.test",
		3*time.Second,
	)
	if err != nil {
		t.Fatalf("new resend sender failed: %v", err)
	}

	result, err := sender.Send(context.Background(), AlertEmailSendInput{
		TenantID: "tenant_demo_jakarta",
		To:       []string{"ops@sudirman.co"},
		Subject:  "wallet alert",
		Text:     "provider smoke",
	})
	if err != nil {
		t.Fatalf("resend sender send failed: %v", err)
	}
	if result.ProviderDeliveryID != "email_123" {
		t.Fatalf("expected provider delivery id email_123, got %+v", result)
	}
}

func TestResendSenderConfirm(t *testing.T) {
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"email_123","last_event":"delivered"}`))
	}))
	defer server.Close()

	sender, err := newResendSender(
		server.URL,
		"re_test_key",
		"alerts@mistypass.test",
		3*time.Second,
	)
	if err != nil {
		t.Fatalf("new resend sender failed: %v", err)
	}

	result, err := sender.Confirm(context.Background(), AlertEmailConfirmInput{
		ProviderDeliveryID: "email_123",
	})
	if err != nil {
		t.Fatalf("resend sender confirm failed: %v", err)
	}
	if capturedPath != "/email_123" {
		t.Fatalf("unexpected confirm request path: %s", capturedPath)
	}
	if !result.Confirmed {
		t.Fatalf("expected confirmed result, got %+v", result)
	}
	if result.ProviderDeliveryID != "email_123" || result.ProviderDeliveryStatus != "delivered" {
		t.Fatalf("unexpected confirm result: %+v", result)
	}
}

func TestResendSenderConfirmRejectedStatusDoesNotConfirm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"email_123","last_event":"bounced"}`))
	}))
	defer server.Close()

	sender, err := newResendSender(
		server.URL,
		"re_test_key",
		"alerts@mistypass.test",
		3*time.Second,
	)
	if err != nil {
		t.Fatalf("new resend sender failed: %v", err)
	}

	result, err := sender.Confirm(context.Background(), AlertEmailConfirmInput{
		ProviderDeliveryID: "email_123",
	})
	if err != nil {
		t.Fatalf("resend sender confirm failed: %v", err)
	}
	if result.Confirmed {
		t.Fatalf("expected bounced status to stay unconfirmed, got %+v", result)
	}
	if result.ProviderDeliveryStatus != "bounced" {
		t.Fatalf("unexpected rejected confirm result: %+v", result)
	}
}
