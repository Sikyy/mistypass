package push

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFCMProviderSendBuildsHTTPV1Payload(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/v1/projects/mistypass-test/messages:send" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"projects/mistypass-test/messages/msg-1"}`))
	}))
	defer server.Close()

	provider, err := NewFCMProvider(FCMOptions{
		ProjectID: "mistypass-test",
		Endpoint:  server.URL + "/v1/projects/{project_id}/messages:send",
		Client:    server.Client(),
	})
	if err != nil {
		t.Fatalf("NewFCMProvider: %v", err)
	}

	receipt, err := provider.Send(context.Background(), Message{
		Token:            "fcm-token-1",
		Type:             "credential_updated",
		Title:            "Credential updated",
		Body:             "Your mobile credential has changed.",
		AndroidChannelID: "credential_alerts",
		TTL:              30 * time.Second,
		Data: map[string]string{
			"credential_id": "cred_1",
		},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if receipt.Provider != FCMProviderName || receipt.ProviderStatus != "sent" || receipt.ProviderMessageID == "" {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}

	message, ok := captured["message"].(map[string]any)
	if !ok {
		t.Fatalf("message payload missing: %#v", captured)
	}
	if message["token"] != "fcm-token-1" {
		t.Fatalf("token mismatch: %#v", message["token"])
	}
	data, ok := message["data"].(map[string]any)
	if !ok {
		t.Fatalf("data payload missing: %#v", message)
	}
	if data["type"] != "credential_updated" || data["credential_id"] != "cred_1" {
		t.Fatalf("data mismatch: %#v", data)
	}
	android, ok := message["android"].(map[string]any)
	if !ok {
		t.Fatalf("android payload missing: %#v", message)
	}
	if android["ttl"] != "30s" {
		t.Fatalf("ttl mismatch: %#v", android["ttl"])
	}
}

func TestFCMProviderReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"status":"UNREGISTERED"}}`, http.StatusNotFound)
	}))
	defer server.Close()

	provider, err := NewFCMProvider(FCMOptions{
		ProjectID: "mistypass-test",
		Endpoint:  server.URL + "/v1/projects/{project_id}/messages:send",
		Client:    server.Client(),
	})
	if err != nil {
		t.Fatalf("NewFCMProvider: %v", err)
	}

	_, err = provider.Send(context.Background(), Message{Token: "expired-token"})
	if err == nil {
		t.Fatalf("expected error")
	}
	httpErr, ok := err.(HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if httpErr.StatusCode != http.StatusNotFound || httpErr.Retryable() {
		t.Fatalf("unexpected http error: %+v", httpErr)
	}
}
