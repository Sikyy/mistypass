package wallet

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestConfirmAlertDeliveryWithResendProvider(t *testing.T) {
	var sendAuth string
	var confirmPath string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			sendAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"email_123"}`))
		case http.MethodGet:
			confirmPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"email_123","last_event":"delivered"}`))
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer provider.Close()

	svc := NewService()
	if err := svc.SetJobAlertEmailDeliveryOptions(JobAlertEmailDeliveryOptions{
		Provider:       "resend",
		EmailFrom:      "alerts@mistypass.local",
		ReceiverMap:    map[string][]string{"security": {"security@example.com"}},
		ResendEndpoint: provider.URL,
		ResendAPIKey:   "re_test_token",
		ResendTimeout:  3 * time.Second,
	}); err != nil {
		t.Fatalf("set resend options failed: %v", err)
	}

	dispatched := svc.DispatchAlert(AlertDeliveryInput{
		TenantID:       "tenant_demo_jakarta",
		Channels:       []string{"email"},
		ReceiverGroups: []string{"security"},
		IdempotencyKey: "ent_alert_lineage_key_001",
		EmailSubject:   "Enterprise alert",
		EmailText:      "provider should receive idempotency key",
	})
	if dispatched.Status != "sent" {
		t.Fatalf("expected sent dispatch result, got %+v", dispatched)
	}
	if sendAuth != "Bearer re_test_token" {
		t.Fatalf("unexpected send auth header: %s", sendAuth)
	}
	if len(dispatched.ChannelResults) != 1 {
		t.Fatalf("expected one channel result, got %+v", dispatched.ChannelResults)
	}
	if dispatched.ChannelResults[0].ProviderDeliveryID != "email_123" {
		t.Fatalf("expected provider delivery id on dispatch, got %+v", dispatched.ChannelResults[0])
	}

	confirmed := svc.ConfirmAlertDelivery(AlertDeliveryConfirmationInput{
		TenantID:       "tenant_demo_jakarta",
		IdempotencyKey: "ent_alert_lineage_key_001",
		ChannelResults: dispatched.ChannelResults,
	})
	if !confirmed.Confirmed || confirmed.Retryable {
		t.Fatalf("expected confirmed non-retryable result, got %+v", confirmed)
	}
	if confirmPath != "/email_123" {
		t.Fatalf("unexpected confirm path: %s", confirmPath)
	}
	if len(confirmed.ChannelResults) != 1 {
		t.Fatalf("expected one confirmed channel result, got %+v", confirmed.ChannelResults)
	}
	if confirmed.ChannelResults[0].ProviderDeliveryStatus != "delivered" {
		t.Fatalf("expected confirmed provider status, got %+v", confirmed.ChannelResults[0])
	}
}

func TestConfirmAlertDeliveryRequiresAllSentChannelsConfirmed(t *testing.T) {
	svc := NewService()
	result := svc.ConfirmAlertDelivery(AlertDeliveryConfirmationInput{
		TenantID:       "tenant_demo_jakarta",
		IdempotencyKey: "ent_alert_lineage_key_001",
		ChannelResults: []JobAlertChannelResult{
			{
				Channel:   "email",
				Status:    "sent",
				Provider:  "resend",
				Retryable: false,
			},
			{
				Channel:   "whatsapp",
				Status:    "sent",
				Provider:  "meta",
				Retryable: false,
			},
		},
	})
	if result.Confirmed {
		t.Fatalf("expected unconfirmed result when sent channels are not fully confirmable, got %+v", result)
	}
}

func TestConfirmAlertDeliveryWithResendRejectedStatus(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"email_123"}`))
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"email_123","last_event":"bounced"}`))
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer provider.Close()

	svc := NewService()
	if err := svc.SetJobAlertEmailDeliveryOptions(JobAlertEmailDeliveryOptions{
		Provider:       "resend",
		EmailFrom:      "alerts@mistypass.local",
		ReceiverMap:    map[string][]string{"security": {"security@example.com"}},
		ResendEndpoint: provider.URL,
		ResendAPIKey:   "re_test_token",
		ResendTimeout:  3 * time.Second,
	}); err != nil {
		t.Fatalf("set resend options failed: %v", err)
	}

	dispatched := svc.DispatchAlert(AlertDeliveryInput{
		TenantID:       "tenant_demo_jakarta",
		Channels:       []string{"email"},
		ReceiverGroups: []string{"security"},
		IdempotencyKey: "ent_alert_lineage_key_001",
		EmailSubject:   "Enterprise alert",
		EmailText:      "provider should receive idempotency key",
	})

	result := svc.ConfirmAlertDelivery(AlertDeliveryConfirmationInput{
		TenantID:       "tenant_demo_jakarta",
		IdempotencyKey: "ent_alert_lineage_key_001",
		ChannelResults: dispatched.ChannelResults,
	})
	if result.Confirmed {
		t.Fatalf("expected bounced confirm result to stay unconfirmed, got %+v", result)
	}
	if result.Retryable {
		t.Fatalf("expected bounced confirm result to be non-retryable at provider confirm layer, got %+v", result)
	}
	if len(result.ChannelResults) != 1 {
		t.Fatalf("expected one channel result, got %+v", result.ChannelResults)
	}
	if result.ChannelResults[0].Status != "failed" || result.ChannelResults[0].Reason != "provider_delivery_failed" {
		t.Fatalf("expected channel to be marked as provider delivery failure, got %+v", result.ChannelResults[0])
	}
	if result.ChannelResults[0].ProviderDeliveryStatus != "bounced" {
		t.Fatalf("expected bounced provider delivery status, got %+v", result.ChannelResults[0])
	}
}
