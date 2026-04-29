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

func TestUserInvitationResendProviderDispatchesAndRecordsReceipt(t *testing.T) {
	var providerRequest struct {
		Authorization  string
		IdempotencyKey string
		To             []string `json:"to"`
		From           string   `json:"from"`
		Subject        string   `json:"subject"`
		Text           string   `json:"text"`
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerRequest.Authorization = r.Header.Get("Authorization")
		providerRequest.IdempotencyKey = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&providerRequest); err != nil {
			t.Fatalf("decode provider payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"re_invite_001"}`))
	}))
	defer provider.Close()

	router, err := NewRouter(config.Config{
		JWTSecret:                    "user-invitation-resend-test-secret",
		EnableDemoUsers:              true,
		UserInvitationEmailProvider:  "resend",
		UserInvitationEmailFrom:      "invites@mistypass.local",
		UserInvitationResendEndpoint: provider.URL,
		UserInvitationResendAPIKey:   "re_invite_test_token",
		UserInvitationResendTimeout:  5 * time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	createUserBody := []byte(`{"tenant_id":"tenant_demo_jakarta","building_id":"building_demo_001","name":"Provider Invite User","email":"provider.invite.user@example.com","role":"employee","status":"inactive"}`)
	createUserRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users", token, createUserBody)
	if createUserRecorder.Code != http.StatusCreated {
		t.Fatalf("expected user create status 201, got %d body=%s", createUserRecorder.Code, createUserRecorder.Body.String())
	}
	var createdUser struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createUserRecorder.Body.Bytes(), &createdUser); err != nil {
		t.Fatalf("decode created user: %v", err)
	}

	inviteRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users/"+createdUser.ID+"/invite", token, []byte(`{"tenant_id":"tenant_demo_jakarta","delivery_method":"email"}`))
	if inviteRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected invitation status 202, got %d body=%s", inviteRecorder.Code, inviteRecorder.Body.String())
	}
	var invitation struct {
		ID                 string `json:"id"`
		Status             string `json:"status"`
		Provider           string `json:"provider"`
		ProviderDeliveryID string `json:"provider_delivery_id"`
		DeliveredAt        string `json:"delivered_at"`
	}
	if err := json.Unmarshal(inviteRecorder.Body.Bytes(), &invitation); err != nil {
		t.Fatalf("decode invitation: %v", err)
	}
	if invitation.ID == "" || invitation.Status != "sent" || invitation.Provider != "resend" || invitation.ProviderDeliveryID != "re_invite_001" || invitation.DeliveredAt == "" {
		t.Fatalf("expected resend receipt fields, got %#v body=%s", invitation, inviteRecorder.Body.String())
	}
	if providerRequest.Authorization != "Bearer re_invite_test_token" {
		t.Fatalf("expected resend authorization header, got %q", providerRequest.Authorization)
	}
	if providerRequest.IdempotencyKey != invitation.ID {
		t.Fatalf("expected idempotency key to match invitation id, got %q", providerRequest.IdempotencyKey)
	}
	if providerRequest.From != "invites@mistypass.local" || len(providerRequest.To) != 1 || providerRequest.To[0] != "provider.invite.user@example.com" {
		t.Fatalf("unexpected provider email payload: %#v", providerRequest)
	}

	assertReferenceAuditLog(t, router, token, "reference_user_invitation_receipt", "user_id="+createdUser.ID, "status=sent", "provider=resend", "provider_delivery_id=re_invite_001")
}

func TestUserInvitationProviderWebhookRecordsSignedReceipt(t *testing.T) {
	const webhookSecret = "provider-webhook-secret-001"
	router, err := NewRouter(config.Config{
		JWTSecret:                           "user-invitation-provider-webhook-test-secret",
		EnableDemoUsers:                     true,
		UserInvitationEmailProvider:         "queue",
		UserInvitationProviderWebhookSecret: webhookSecret,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	createUserBody := []byte(`{"tenant_id":"tenant_demo_jakarta","building_id":"building_demo_001","name":"Webhook Invite User","email":"webhook.invite.user@example.com","role":"employee","status":"inactive"}`)
	createUserRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users", token, createUserBody)
	if createUserRecorder.Code != http.StatusCreated {
		t.Fatalf("expected user create status 201, got %d body=%s", createUserRecorder.Code, createUserRecorder.Body.String())
	}
	var createdUser struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createUserRecorder.Body.Bytes(), &createdUser); err != nil {
		t.Fatalf("decode created user: %v", err)
	}

	inviteRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users/"+createdUser.ID+"/invite", token, []byte(`{"tenant_id":"tenant_demo_jakarta","delivery_method":"email"}`))
	if inviteRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected invitation status 202, got %d body=%s", inviteRecorder.Code, inviteRecorder.Body.String())
	}
	var invitation struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(inviteRecorder.Body.Bytes(), &invitation); err != nil {
		t.Fatalf("decode invitation: %v", err)
	}
	if invitation.ID == "" || invitation.Status != "queued" {
		t.Fatalf("expected queued invitation, got %#v", invitation)
	}

	payload := []byte(`{"tenant_id":"tenant_demo_jakarta","user_id":"` + createdUser.ID + `","delivery_id":"` + invitation.ID + `","status":"bounced","provider":"resend","provider_delivery_id":"re_evt_001","provider_error":"mailbox unavailable"}`)
	invalidRequest := httptest.NewRequest(http.MethodPost, "/api/v1/users/invitations/provider-receipts", bytes.NewReader(payload))
	invalidRequest.Header.Set("Content-Type", "application/json")
	invalidRequest.Header.Set(userInvitationProviderSignatureTimestamp, strconv.FormatInt(time.Now().UTC().Unix(), 10))
	invalidRequest.Header.Set(userInvitationProviderSignatureHeader, "sha256=invalid")
	invalidRecorder := httptest.NewRecorder()
	router.ServeHTTP(invalidRecorder, invalidRequest)
	if invalidRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected invalid signature status 401, got %d body=%s", invalidRecorder.Code, invalidRecorder.Body.String())
	}

	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/users/invitations/provider-receipts", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(userInvitationProviderSignatureTimestamp, timestamp)
	request.Header.Set(userInvitationProviderSignatureHeader, signUserInvitationProviderWebhookPayload(webhookSecret, timestamp, payload))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected provider receipt status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"status":"failed"`) ||
		!strings.Contains(recorder.Body.String(), `"provider":"resend"`) ||
		!strings.Contains(recorder.Body.String(), `"provider_delivery_id":"re_evt_001"`) ||
		!strings.Contains(recorder.Body.String(), `"provider_error":"mailbox unavailable"`) {
		t.Fatalf("expected signed provider receipt fields, body=%s", recorder.Body.String())
	}

	auditRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/audit-logs?tenant_id=tenant_demo_jakarta&action=reference_user_invitation_receipt&source=user_invitation_provider&limit=1", token, nil)
	if auditRecorder.Code != http.StatusOK {
		t.Fatalf("expected audit logs status 200, got %d body=%s", auditRecorder.Code, auditRecorder.Body.String())
	}
	if !strings.Contains(auditRecorder.Body.String(), "provider_delivery_id=re_evt_001") ||
		!strings.Contains(auditRecorder.Body.String(), `"actor":"system"`) {
		t.Fatalf("expected provider audit log, body=%s", auditRecorder.Body.String())
	}
}
