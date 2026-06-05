package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

// SAML allows IdP-initiated assertions (no InResponseTo correlation), so the
// stateless exchange path must reject a replayed assertion: the same signed
// assertion must not be usable twice within its validity window.
func TestEnterpriseAuthExchangeSAMLRejectsAssertionReplay(t *testing.T) {
	setEnterpriseSAMLFixtureClock(t)
	s := newEnterpriseJITSAMLTestServer(t)

	// Seed a conflicting employee so the post-verification JIT step returns 409.
	// VerifySAMLResponse still succeeds (and consumes the assertion) on the first
	// call, so the second call must fail at the replay check (401), not 409.
	if _, err := s.enterpriseSvc.SyncEmployees(
		enterpriseSAMLFixtureTenantID,
		"manual_sync",
		"qa",
		[]enterprise.EmployeeSyncInput{{
			ExternalID: "external-conflict-existing",
			Email:      enterpriseSAMLFixtureEmail,
			FullName:   "SAML Replay",
			Department: "IT",
			Location:   "Jakarta",
			Status:     "active",
		}},
	); err != nil {
		t.Fatalf("seed conflicting employee: %v", err)
	}

	samlResponse := mustReadEnterpriseSAMLFixtureResponse(t)
	doExchange := func() *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{
			"email":     enterpriseSAMLFixtureEmail,
			"provider":  "saml",
			"idp_token": samlResponse,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/exchange", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.enterpriseAuthExchange(rec, req)
		return rec
	}

	// First use: assertion verifies (and is consumed); JIT conflict yields 409.
	first := doExchange()
	if first.Code == http.StatusUnauthorized {
		t.Fatalf("first SAML exchange must pass verification, but got 401: %s", first.Body.String())
	}
	if first.Code != http.StatusConflict {
		t.Fatalf("expected first SAML exchange to reach JIT conflict (409), got %d body=%s", first.Code, first.Body.String())
	}

	// Second use of the same assertion must be rejected as a replay (401),
	// before reaching the JIT conflict.
	second := doExchange()
	if second.Code != http.StatusUnauthorized {
		t.Fatalf("expected replayed SAML assertion to be rejected (401), got %d body=%s", second.Code, second.Body.String())
	}
}
