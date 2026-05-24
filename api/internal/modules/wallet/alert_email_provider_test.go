package wallet

import (
	"context"
	"encoding/json"
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

func TestCloudflareSenderSend(t *testing.T) {
	var capturedAuth string
	var capturedIdempotencyKey string
	var capturedPayload struct {
		From    string            `json:"from"`
		To      []string          `json:"to"`
		Subject string            `json:"subject"`
		Text    string            `json:"text"`
		Headers map[string]string `json:"headers"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		capturedIdempotencyKey = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&capturedPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"result":{"delivered":[],"permanent_bounces":[],"queued":["ops@sudirman.co"]}}`))
	}))
	defer server.Close()

	sender, err := newCloudflareSender(
		server.URL,
		"",
		"cf_email_token",
		"alerts@mistypass.test",
		3*time.Second,
	)
	if err != nil {
		t.Fatalf("new cloudflare sender failed: %v", err)
	}

	result, err := sender.Send(context.Background(), AlertEmailSendInput{
		TenantID:       "tenant_demo_jakarta",
		To:             []string{"ops@sudirman.co"},
		IdempotencyKey: "wallet-email-idem-001",
		Subject:        "wallet alert",
		Text:           "provider smoke",
	})
	if err != nil {
		t.Fatalf("cloudflare sender send failed: %v", err)
	}
	if capturedAuth != "Bearer cf_email_token" {
		t.Fatalf("unexpected auth header: %s", capturedAuth)
	}
	if capturedIdempotencyKey != "wallet-email-idem-001" {
		t.Fatalf("unexpected idempotency header: %s", capturedIdempotencyKey)
	}
	if capturedPayload.From != "alerts@mistypass.test" || len(capturedPayload.To) != 1 || capturedPayload.To[0] != "ops@sudirman.co" {
		t.Fatalf("unexpected payload: %+v", capturedPayload)
	}
	if capturedPayload.Headers["X-MistyPass-Tenant-ID"] != "tenant_demo_jakarta" {
		t.Fatalf("expected tenant header, got %+v", capturedPayload.Headers)
	}
	if result.ProviderDeliveryID != "wallet-email-idem-001" || result.ProviderDeliveryStatus != "queued" {
		t.Fatalf("unexpected send result: %+v", result)
	}
}
