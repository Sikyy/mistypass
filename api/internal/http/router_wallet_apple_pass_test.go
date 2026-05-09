package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestAppApplePassSelfEnrollmentAndAdminLifecycle(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "wallet-apple-pass-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	appLoginBody := []byte(`{"email":"resident.jakarta@mistypass.local","password":"admin123"}`)
	appLoginRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/auth/login", "", appLoginBody)
	if appLoginRecorder.Code != http.StatusOK {
		t.Fatalf("expected app login 200, got %d body=%s", appLoginRecorder.Code, appLoginRecorder.Body.String())
	}
	var appLogin struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(appLoginRecorder.Body.Bytes(), &appLogin); err != nil {
		t.Fatalf("decode app login: %v", err)
	}
	if appLogin.AccessToken == "" {
		t.Fatal("expected resident app token")
	}

	enrollBody := []byte(`{"device_id":"iphone-15-pro","pass_serial":"apple-pass-self-001"}`)
	enrollRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/credentials/apple-pass", appLogin.AccessToken, enrollBody)
	if enrollRecorder.Code != http.StatusCreated {
		t.Fatalf("expected apple pass enroll 201, got %d body=%s", enrollRecorder.Code, enrollRecorder.Body.String())
	}
	var enrollResponse struct {
		CredentialKind   string `json:"credential_kind"`
		EnrollmentSource string `json:"enrollment_source"`
		Pass             struct {
			ID             string `json:"id"`
			Provider       string `json:"provider"`
			CredentialKind string `json:"credential_kind"`
			TargetType     string `json:"target_type"`
			TargetID       string `json:"target_id"`
			ObjectID       string `json:"object_id"`
			Token          string `json:"token"`
			Status         string `json:"status"`
		} `json:"pass"`
	}
	if err := json.Unmarshal(enrollRecorder.Body.Bytes(), &enrollResponse); err != nil {
		t.Fatalf("decode enroll response: %v", err)
	}
	if enrollResponse.CredentialKind != "apple_wallet" ||
		enrollResponse.EnrollmentSource != "self_service" ||
		enrollResponse.Pass.ID == "" ||
		enrollResponse.Pass.Provider != "apple" ||
		enrollResponse.Pass.CredentialKind != "apple_wallet" ||
		enrollResponse.Pass.TargetType != "user" ||
		enrollResponse.Pass.TargetID != "usr_resident_jkt_001" ||
		enrollResponse.Pass.ObjectID != "apple-pass-self-001" ||
		enrollResponse.Pass.Token != "iphone-15-pro" ||
		enrollResponse.Pass.Status != "active" {
		t.Fatalf("unexpected apple pass enrollment response: %+v body=%s", enrollResponse, enrollRecorder.Body.String())
	}

	appCredentialsRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/app/credentials", appLogin.AccessToken, nil)
	if appCredentialsRecorder.Code != http.StatusOK {
		t.Fatalf("expected app credentials 200, got %d body=%s", appCredentialsRecorder.Code, appCredentialsRecorder.Body.String())
	}
	if !strings.Contains(appCredentialsRecorder.Body.String(), `"credential_kind":"apple_wallet"`) ||
		!strings.Contains(appCredentialsRecorder.Body.String(), `"provider":"apple"`) {
		t.Fatalf("expected app credentials to include apple pass fields, body=%s", appCredentialsRecorder.Body.String())
	}

	adminToken := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	adminCardsRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/cards?tenant_id=tenant_demo_jakarta&credential_kind=apple_wallet", adminToken, nil)
	if adminCardsRecorder.Code != http.StatusOK {
		t.Fatalf("expected admin apple cards 200, got %d body=%s", adminCardsRecorder.Code, adminCardsRecorder.Body.String())
	}
	if !strings.Contains(adminCardsRecorder.Body.String(), `"id":"`+enrollResponse.Pass.ID+`"`) ||
		!strings.Contains(adminCardsRecorder.Body.String(), `"provider":"apple"`) ||
		!strings.Contains(adminCardsRecorder.Body.String(), `"credential_kind":"apple_wallet"`) {
		t.Fatalf("expected admin cards to list enrolled apple pass, body=%s", adminCardsRecorder.Body.String())
	}

	deactivateRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/cards/"+enrollResponse.Pass.ID+"/deactivate?tenant_id=tenant_demo_jakarta", adminToken, nil)
	if deactivateRecorder.Code != http.StatusOK {
		t.Fatalf("expected admin deactivate apple pass 200, got %d body=%s", deactivateRecorder.Code, deactivateRecorder.Body.String())
	}
	if !strings.Contains(deactivateRecorder.Body.String(), `"status":"deactivated"`) ||
		!strings.Contains(deactivateRecorder.Body.String(), `"credential_kind":"apple_wallet"`) {
		t.Fatalf("expected deactivated apple card wrapper, body=%s", deactivateRecorder.Body.String())
	}

	adminIssueAppleBody := []byte(`{"card":{"tenant_id":"tenant_demo_jakarta","type":"apple_wallet","user_id":"usr_1001"}}`)
	adminIssueAppleRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/cards", adminToken, adminIssueAppleBody)
	if adminIssueAppleRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected admin apple issue blocked with 422, got %d body=%s", adminIssueAppleRecorder.Code, adminIssueAppleRecorder.Body.String())
	}
	if !strings.Contains(adminIssueAppleRecorder.Body.String(), "apple wallet passes must be enrolled through the user app") {
		t.Fatalf("expected apple self enrollment error, body=%s", adminIssueAppleRecorder.Body.String())
	}
}
