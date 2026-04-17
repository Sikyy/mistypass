package wallet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewMetaWhatsAppSenderValidation(t *testing.T) {
	if _, err := newMetaWhatsAppSender("", "", "123456", 3*time.Second); err == nil {
		t.Fatalf("expected error when api key is missing")
	}
	if _, err := newMetaWhatsAppSender("", "wa_token", "", 3*time.Second); err == nil {
		t.Fatalf("expected error when phone number id is missing")
	}
}

func TestMetaWhatsAppSenderSend(t *testing.T) {
	requestCount := 0
	var capturedAuth string
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		capturedAuth = r.Header.Get("Authorization")
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender, err := newMetaWhatsAppSender(
		server.URL,
		"wa_test_token",
		"112233445566",
		3*time.Second,
	)
	if err != nil {
		t.Fatalf("new meta sender failed: %v", err)
	}
	err = sender.Send(context.Background(), AlertWhatsAppSendInput{
		TenantID: "tenant_demo_jakarta",
		To:       []string{"+62811111111", "+62822222222"},
		Text:     "wallet alert",
	})
	if err != nil {
		t.Fatalf("meta sender send failed: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("expected 2 send requests, got %d", requestCount)
	}
	if capturedAuth != "Bearer wa_test_token" {
		t.Fatalf("unexpected auth header: %s", capturedAuth)
	}
	if capturedPath != "/112233445566/messages" {
		t.Fatalf("unexpected request path: %s", capturedPath)
	}
}

func TestMetaWhatsAppSenderSendRetryableFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"temporary unavailable"}`))
	}))
	defer server.Close()

	sender, err := newMetaWhatsAppSender(
		server.URL,
		"wa_test_token",
		"112233445566",
		3*time.Second,
	)
	if err != nil {
		t.Fatalf("new meta sender failed: %v", err)
	}
	err = sender.Send(context.Background(), AlertWhatsAppSendInput{
		TenantID: "tenant_demo_jakarta",
		To:       []string{"+62811111111"},
		Text:     "wallet alert",
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
