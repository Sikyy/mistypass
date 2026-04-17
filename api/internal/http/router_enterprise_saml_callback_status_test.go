package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	dsig "github.com/russellhaering/goxmldsig"
)

const (
	enterpriseSAMLFixtureTenantID     = "tenant_demo_jakarta"
	enterpriseSAMLFixtureEmail        = "ross@octolabs.io"
	enterpriseSAMLFixtureSubject      = "_ce3d2948b4cf20146dee0a0b3dd6f69b6cf86f62d7"
	enterpriseSAMLFixtureIssuerURL    = "https://accounts.google.com/o/saml2?idpid=C02dfl1r1"
	enterpriseSAMLFixtureAudience     = "https://29ee6d2e.ngrok.io/saml/metadata"
	enterpriseSAMLFixtureACSURL       = "https://29ee6d2e.ngrok.io/saml/acs"
	enterpriseSAMLFixtureResponsePath = "testdata/saml/TestSPCanHandlePlaintextResponse_response.b64"
	enterpriseSAMLFixtureMetadataPath = "testdata/saml/TestSPCanHandlePlaintextResponse_IDPMetadata.xml"
)

func TestEnterpriseSAMLCallbackJITInactiveReturnsForbidden(t *testing.T) {
	setEnterpriseSAMLFixtureClock(t)
	s := newEnterpriseJITSAMLTestServer(t)

	_, err := s.enterpriseSvc.SyncEmployees(
		enterpriseSAMLFixtureTenantID,
		"manual_sync",
		"qa",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID: enterpriseSAMLFixtureSubject,
				Email:      enterpriseSAMLFixtureEmail,
				FullName:   "SAML Callback Inactive",
				Department: "IT",
				Location:   "Jakarta",
				Status:     "inactive",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected seed inactive employee success: %v", err)
	}

	stateToken, err := s.enterpriseSvc.StartAuthStateToken(
		enterpriseSAMLFixtureTenantID,
		"saml",
		enterpriseSAMLFixtureEmail,
		"https://admin.mistypass.local/enterprise/callback",
		5*time.Minute,
	)
	if err != nil {
		t.Fatalf("expected start auth state token success: %v", err)
	}

	requestBytes, _ := json.Marshal(map[string]any{
		"state":         stateToken.Token,
		"saml_response": mustReadEnterpriseSAMLFixtureResponse(t),
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/saml/callback", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.enterpriseSAMLCallback(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertResponseError(t, recorder, enterprise.ErrEmployeeInactive.Error())
}

func TestEnterpriseSAMLCallbackJITExternalIDConflictReturnsConflict(t *testing.T) {
	setEnterpriseSAMLFixtureClock(t)
	s := newEnterpriseJITSAMLTestServer(t)

	_, err := s.enterpriseSvc.SyncEmployees(
		enterpriseSAMLFixtureTenantID,
		"manual_sync",
		"qa",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID: "external-conflict-existing",
				Email:      enterpriseSAMLFixtureEmail,
				FullName:   "SAML Callback Conflict",
				Department: "IT",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected seed active employee success: %v", err)
	}

	stateToken, err := s.enterpriseSvc.StartAuthStateToken(
		enterpriseSAMLFixtureTenantID,
		"saml",
		enterpriseSAMLFixtureEmail,
		"https://admin.mistypass.local/enterprise/callback",
		5*time.Minute,
	)
	if err != nil {
		t.Fatalf("expected start auth state token success: %v", err)
	}

	requestBytes, _ := json.Marshal(map[string]any{
		"state":         stateToken.Token,
		"saml_response": mustReadEnterpriseSAMLFixtureResponse(t),
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/saml/callback", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.enterpriseSAMLCallback(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertResponseError(t, recorder, enterprise.ErrEmployeeExternalIDConflict.Error())
}

func TestEnterpriseAuthExchangeSAMLJITInactiveReturnsForbidden(t *testing.T) {
	setEnterpriseSAMLFixtureClock(t)
	s := newEnterpriseJITSAMLTestServer(t)

	_, err := s.enterpriseSvc.SyncEmployees(
		enterpriseSAMLFixtureTenantID,
		"manual_sync",
		"qa",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID: enterpriseSAMLFixtureSubject,
				Email:      enterpriseSAMLFixtureEmail,
				FullName:   "SAML Exchange Inactive",
				Department: "IT",
				Location:   "Jakarta",
				Status:     "inactive",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected seed inactive employee success: %v", err)
	}

	requestBytes, _ := json.Marshal(map[string]any{
		"email":     enterpriseSAMLFixtureEmail,
		"provider":  "saml",
		"idp_token": mustReadEnterpriseSAMLFixtureResponse(t),
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/exchange", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.enterpriseAuthExchange(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertResponseError(t, recorder, enterprise.ErrEmployeeInactive.Error())
}

func TestEnterpriseAuthExchangeSAMLJITExternalIDConflictReturnsConflict(t *testing.T) {
	setEnterpriseSAMLFixtureClock(t)
	s := newEnterpriseJITSAMLTestServer(t)

	_, err := s.enterpriseSvc.SyncEmployees(
		enterpriseSAMLFixtureTenantID,
		"manual_sync",
		"qa",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID: "external-conflict-existing",
				Email:      enterpriseSAMLFixtureEmail,
				FullName:   "SAML Exchange Conflict",
				Department: "IT",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected seed active employee success: %v", err)
	}

	requestBytes, _ := json.Marshal(map[string]any{
		"email":     enterpriseSAMLFixtureEmail,
		"provider":  "saml",
		"idp_token": mustReadEnterpriseSAMLFixtureResponse(t),
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/exchange", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.enterpriseAuthExchange(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertResponseError(t, recorder, enterprise.ErrEmployeeExternalIDConflict.Error())
}

func newEnterpriseJITSAMLTestServer(t *testing.T) *server {
	t.Helper()

	s := &server{
		authService:         auth.NewService("", "", 0, 0, true),
		enterpriseSvc:       enterprise.NewService(),
		gatewayDeviceTokens: map[string]string{},
	}

	_, err := s.enterpriseSvc.CreateDomainMapping(enterpriseSAMLFixtureTenantID, "octolabs.io", "active")
	if err != nil {
		t.Fatalf("expected create domain mapping success: %v", err)
	}

	_, err = s.enterpriseSvc.UpsertIDPConfig(
		enterpriseSAMLFixtureTenantID,
		"saml",
		enterpriseSAMLFixtureIssuerURL,
		enterpriseSAMLFixtureAudience,
		"", // auth_url
		"", // token_url
		"", // jwks_url
		"", // user_info_url
		enterpriseSAMLFixtureACSURL,
		mustReadEnterpriseSAMLFixtureSigningCert(t),
		"active",
		"jit",
		"qa",
		nil,
	)
	if err != nil {
		t.Fatalf("expected upsert saml idp config success: %v", err)
	}

	return s
}

func setEnterpriseSAMLFixtureClock(t *testing.T) {
	t.Helper()

	fixedNow := time.Date(2016, time.January, 5, 16, 56, 0, 0, time.UTC)
	originalNow := saml.TimeNow
	originalClock := saml.Clock
	saml.TimeNow = func() time.Time {
		return fixedNow
	}
	saml.Clock = dsig.NewFakeClockAt(fixedNow)
	t.Cleanup(func() {
		saml.TimeNow = originalNow
		saml.Clock = originalClock
	})
}

func mustReadEnterpriseSAMLFixtureResponse(t *testing.T) string {
	t.Helper()

	contents, err := os.ReadFile(enterpriseSAMLFixtureResponsePath)
	if err != nil {
		t.Fatalf("read saml response fixture failed: %v", err)
	}
	return strings.TrimSpace(string(contents))
}

func mustReadEnterpriseSAMLFixtureSigningCert(t *testing.T) string {
	t.Helper()

	metadataBytes, err := os.ReadFile(enterpriseSAMLFixtureMetadataPath)
	if err != nil {
		t.Fatalf("read saml metadata fixture failed: %v", err)
	}

	const (
		beginTag = "<ds:X509Certificate>"
		endTag   = "</ds:X509Certificate>"
	)
	metadata := string(metadataBytes)
	start := strings.Index(metadata, beginTag)
	if start < 0 {
		t.Fatalf("metadata fixture missing begin cert tag")
	}
	start += len(beginTag)
	end := strings.Index(metadata[start:], endTag)
	if end < 0 {
		t.Fatalf("metadata fixture missing end cert tag")
	}
	cert := strings.TrimSpace(metadata[start : start+end])
	if cert == "" {
		t.Fatalf("metadata fixture cert empty")
	}
	return cert
}
