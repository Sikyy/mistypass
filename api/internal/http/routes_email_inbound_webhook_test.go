package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestEmailInboundWebhookRecordsEventAndAudit(t *testing.T) {
	secret := "email-inbound-test-secret"
	router, _, err := NewRouter(config.Config{
		JWTSecret:                 "email-inbound-test-jwt",
		EnableDemoUsers:           true,
		EmailInboundWebhookSecret: secret,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	body := []byte(`{
		"tenant_id":"tenant_demo_jakarta",
		"provider":"cloudflare_email_worker",
		"event_type":"reply",
		"message_id":"msg-report-reply-001",
		"provider_delivery_id":"email_report_mock_1",
		"from":"manager@example.com",
		"to":["reports@mistyislet.com"],
		"subject":"Re: Daily report",
		"received_at":"2026-05-24T10:30:00Z",
		"metadata":{"report_schedule_id":"rs_000123"},
		"attachments":[{"filename":"reply.eml","content_type":"message/rfc822","size_bytes":512}]
	}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/email/inbound", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	timestamp := time.Now().UTC().Unix()
	request.Header.Set(emailInboundWebhookTimestampHeader, " "+strconvFormatInt(timestamp)+" ")
	request.Header.Set(emailInboundWebhookSignatureHeader, signEmailInboundWebhookPayload(secret, strconvFormatInt(timestamp), body))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected webhook 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var event emailInboundEvent
	if err := json.Unmarshal(recorder.Body.Bytes(), &event); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if event.ID == "" || event.MessageID != "msg-report-reply-001" || event.ProviderDeliveryID != "email_report_mock_1" {
		t.Fatalf("unexpected event identity: %+v", event)
	}
	if event.Metadata["report_schedule_id"] != "rs_000123" || len(event.Attachments) != 1 {
		t.Fatalf("unexpected event metadata/attachments: %+v", event)
	}

	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	listRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/webhooks/email/inbound/events?tenant_id=tenant_demo_jakarta&event_type=reply&limit=5", token, nil)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var listResponse struct {
		Items []emailInboundEvent `json:"items"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResponse.Items) != 1 || listResponse.Items[0].ID != event.ID {
		t.Fatalf("expected listed event %s, got %+v", event.ID, listResponse.Items)
	}

	auditRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/audit-logs?tenant_id=tenant_demo_jakarta&action=email_inbound_event_received&source=email_inbound_webhook&limit=1", token, nil)
	if auditRecorder.Code != http.StatusOK {
		t.Fatalf("expected audit 200, got %d body=%s", auditRecorder.Code, auditRecorder.Body.String())
	}
	if !strings.Contains(auditRecorder.Body.String(), "related=report_schedule_id:rs_000123") {
		t.Fatalf("expected audit target to include related schedule, body=%s", auditRecorder.Body.String())
	}
}

func TestEmailInboundWebhookRejectsInvalidSignature(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:                 "email-inbound-reject-test-jwt",
		EnableDemoUsers:           true,
		EmailInboundWebhookSecret: "email-inbound-test-secret",
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	body := []byte(`{"tenant_id":"tenant_demo_jakarta"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/email/inbound", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(emailInboundWebhookTimestampHeader, strconvFormatInt(time.Now().UTC().Unix()))
	request.Header.Set(emailInboundWebhookSignatureHeader, "sha256=invalid")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected invalid signature 401, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestEmailInboundWebhookRequiresConfiguredSecret(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "email-inbound-no-secret-test-jwt",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	recorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/webhooks/email/inbound", "", []byte(`{"tenant_id":"tenant_demo_jakarta"}`))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing secret 503, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func strconvFormatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
