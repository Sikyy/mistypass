package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

func TestEnterpriseOIDCTokenURLFallback(t *testing.T) {
	tokenURL, err := enterpriseOIDCTokenURL(enterprise.IDPConfig{
		IssuerURL: "https://id.sudirman.co/",
	})
	if err != nil {
		t.Fatalf("expected token url fallback to succeed: %v", err)
	}
	if tokenURL != "https://id.sudirman.co/oauth2/token" {
		t.Fatalf("unexpected token url: %s", tokenURL)
	}
}

func TestExchangeEnterpriseOIDCCodeForIDTokenWithClientSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if !strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			t.Fatalf("expected form content type, got: %s", r.Header.Get("Content-Type"))
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		if r.Form.Get("grant_type") != "authorization_code" {
			t.Fatalf("unexpected grant_type: %s", r.Form.Get("grant_type"))
		}
		if r.Form.Get("code") != "auth_code_demo" {
			t.Fatalf("unexpected code: %s", r.Form.Get("code"))
		}
		if r.Form.Get("client_id") != "mistypass-web-admin" {
			t.Fatalf("unexpected client_id: %s", r.Form.Get("client_id"))
		}
		if r.Form.Get("redirect_uri") != "https://admin.mistypass.local/enterprise/callback" {
			t.Fatalf("unexpected redirect_uri: %s", r.Form.Get("redirect_uri"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id_token":"oidc.id_token.demo"}`))
	}))
	defer server.Close()

	idToken, statusCode, err := exchangeEnterpriseOIDCCodeForIDTokenWithClient(
		context.Background(),
		server.Client(),
		enterprise.IDPConfig{
			TokenURL: server.URL,
			ClientID: "mistypass-web-admin",
		},
		"auth_code_demo",
		"https://admin.mistypass.local/enterprise/callback",
	)
	if err != nil {
		t.Fatalf("expected code exchange success: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", statusCode)
	}
	if idToken != "oidc.id_token.demo" {
		t.Fatalf("unexpected id_token: %s", idToken)
	}
}

func TestExchangeEnterpriseOIDCCodeForIDTokenWithClientIDPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"authorization code expired"}`))
	}))
	defer server.Close()

	_, statusCode, err := exchangeEnterpriseOIDCCodeForIDTokenWithClient(
		context.Background(),
		server.Client(),
		enterprise.IDPConfig{
			TokenURL: server.URL,
			ClientID: "mistypass-web-admin",
		},
		"expired_code",
		"https://admin.mistypass.local/enterprise/callback",
	)
	if err == nil {
		t.Fatalf("expected code exchange to fail")
	}
	if statusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected status code: %d", statusCode)
	}
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("expected invalid_grant in error, got: %v", err)
	}
}

func TestExchangeEnterpriseOIDCCodeForIDTokenWithClientMissingIDToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at_demo_001"}`))
	}))
	defer server.Close()

	_, statusCode, err := exchangeEnterpriseOIDCCodeForIDTokenWithClient(
		context.Background(),
		server.Client(),
		enterprise.IDPConfig{
			TokenURL: server.URL,
			ClientID: "mistypass-web-admin",
		},
		"auth_code_demo",
		"https://admin.mistypass.local/enterprise/callback",
	)
	if err == nil {
		t.Fatalf("expected missing id_token to fail")
	}
	if statusCode != http.StatusBadGateway {
		t.Fatalf("unexpected status code: %d", statusCode)
	}
	if !strings.Contains(err.Error(), "missing id_token") {
		t.Fatalf("unexpected error: %v", err)
	}
}
