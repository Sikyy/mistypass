package httpx

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

func TestEnterpriseAuthExchangeJITInactiveReturnsForbidden(t *testing.T) {
	s, issuerURL, clientID, emailDomain, signingKey := newEnterpriseJITOIDCTestServer(t)
	email := "jit.inactive.exchange@" + emailDomain
	externalID := "sub-jit-inactive-exchange-001"

	_, err := s.enterpriseSvc.SyncEmployees(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID: externalID,
				Email:      email,
				FullName:   "Inactive Exchange User",
				Department: "IT",
				Location:   "Jakarta",
				Status:     "inactive",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected seed inactive employee success: %v", err)
	}

	idToken := mustBuildSignedOIDCIDToken(t, signingKey, issuerURL, clientID, externalID, email)
	body := map[string]any{
		"email":     email,
		"provider":  "oidc",
		"idp_token": idToken,
	}
	requestBytes, _ := json.Marshal(body)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/exchange", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.enterpriseAuthExchange(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertResponseError(t, recorder, enterprise.ErrEmployeeInactive.Error())
}

func TestEnterpriseAuthExchangeJITExternalIDConflictReturnsConflict(t *testing.T) {
	s, issuerURL, clientID, emailDomain, signingKey := newEnterpriseJITOIDCTestServer(t)
	email := "jit.conflict.exchange@" + emailDomain

	_, err := s.enterpriseSvc.SyncEmployees(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID: "sub-jit-conflict-exchange-a",
				Email:      email,
				FullName:   "Conflict Exchange User",
				Department: "IT",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected seed active employee success: %v", err)
	}

	idToken := mustBuildSignedOIDCIDToken(t, signingKey, issuerURL, clientID, "sub-jit-conflict-exchange-b", email)
	body := map[string]any{
		"email":     email,
		"provider":  "oidc",
		"idp_token": idToken,
	}
	requestBytes, _ := json.Marshal(body)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/exchange", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.enterpriseAuthExchange(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertResponseError(t, recorder, enterprise.ErrEmployeeExternalIDConflict.Error())
}

func newEnterpriseJITOIDCTestServer(t *testing.T) (*server, string, string, string, *rsa.PrivateKey) {
	t.Helper()

	s := &server{
		authService:         auth.NewService("", "", 0, 0, true),
		auditSvc:            audit.NewService(),
		enterpriseSvc:       enterprise.NewService(),
		gatewayDeviceTokens: map[string]string{},
	}

	const (
		tenantID  = "tenant_demo_jakarta"
		issuerURL = "https://id.sudirman.co"
		clientID  = "mistypass-web-admin"
	)
	emailDomain := fmt.Sprintf("jit-exchange-%d.local", time.Now().UTC().UnixNano())

	_, err := s.enterpriseSvc.CreateDomainMapping(tenantID, emailDomain, "active")
	if err != nil {
		t.Fatalf("expected create domain mapping success: %v", err)
	}

	signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key for oidc test failed: %v", err)
	}

	jwksURL := mustBuildJWKSDataURL(t, signingKey)
	_, err = s.enterpriseSvc.UpsertIDPConfig(
		tenantID,
		"oidc",
		issuerURL,
		clientID,
		"", // auth_url
		"", // token_url
		jwksURL,
		"", // user_info_url
		"", // saml_acs_url
		"", // saml_x509_cert
		"active",
		"jit",
		"qa",
		[]string{"openid", "email"},
	)
	if err != nil {
		t.Fatalf("expected upsert idp config success: %v", err)
	}

	return s, issuerURL, clientID, emailDomain, signingKey
}

func assertResponseError(t *testing.T, recorder *httptest.ResponseRecorder, expected string) {
	t.Helper()
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json error response, decode err=%v body=%s", err, recorder.Body.String())
	}
	if payload.Error != expected {
		t.Fatalf("expected error=%q got=%q", expected, payload.Error)
	}
}

func mustBuildSignedOIDCIDToken(t *testing.T, signingKey *rsa.PrivateKey, issuer, audience, subject, email string) string {
	return mustBuildSignedOIDCIDTokenWithExtraClaims(
		t,
		signingKey,
		issuer,
		audience,
		subject,
		email,
		nil,
	)
}

func mustBuildSignedOIDCIDTokenWithExtraClaims(
	t *testing.T,
	signingKey *rsa.PrivateKey,
	issuer, audience, subject, email string,
	extraClaims map[string]any,
) string {
	t.Helper()

	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"iss":        issuer,
		"aud":        audience,
		"sub":        subject,
		"email":      email,
		"name":       "Exchange Test User",
		"department": "IT",
		"job_title":  "Engineer",
		"location":   "Jakarta",
		"exp":        now.Add(10 * time.Minute).Unix(),
		"iat":        now.Add(-1 * time.Minute).Unix(),
	}
	for k, v := range extraClaims {
		claims[k] = v
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-kid-001"

	signedToken, err := token.SignedString(signingKey)
	if err != nil {
		t.Fatalf("sign oidc token failed: %v", err)
	}
	return signedToken
}

func mustBuildJWKSDataURL(t *testing.T, signingKey *rsa.PrivateKey) string {
	t.Helper()

	nEncoded := base64.RawURLEncoding.EncodeToString(signingKey.PublicKey.N.Bytes())
	eEncoded := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(signingKey.PublicKey.E)).Bytes())
	if nEncoded == "" || eEncoded == "" {
		t.Fatalf("invalid jwks key material")
	}

	doc := map[string]any{
		"keys": []map[string]any{
			{
				"kid": "test-kid-001",
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"n":   nEncoded,
				"e":   eEncoded,
			},
		},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal jwks failed: %v", err)
	}

	return fmt.Sprintf("data:application/json,%s", url.QueryEscape(string(raw)))
}
