package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func badgeTestRouter(t *testing.T) (http.Handler, *server) {
	t.Helper()
	handler, _, err := NewRouter(config.Config{
		JWTSecret:       "badge-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	return handler, nil
}

func TestBadgeTokenRoundTrip(t *testing.T) {
	s := &server{cfg: config.Config{JWTSecret: "badge-secret"}}
	token := s.signBadgeToken("tenant_demo_jakarta", "usr_1001")
	tenantID, userID, ok := s.parseBadgeToken(token)
	if !ok || tenantID != "tenant_demo_jakarta" || userID != "usr_1001" {
		t.Fatalf("round trip failed: tenant=%q user=%q ok=%v", tenantID, userID, ok)
	}
	if _, _, ok := s.parseBadgeToken(token + "x"); ok {
		t.Fatal("tampered token must not verify")
	}
	if _, _, ok := s.parseBadgeToken("garbage"); ok {
		t.Fatal("garbage token must not verify")
	}
}

func TestVerifyBadgeEndpoint(t *testing.T) {
	handler, _ := badgeTestRouter(t)
	signer := &server{cfg: config.Config{JWTSecret: "badge-test-secret"}}
	token := signer.signBadgeToken("tenant_demo_jakarta", "usr_1001")

	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/badges/verify?token="+token, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Valid        bool   `json:"valid"`
		Name         string `json:"name"`
		Status       string `json:"status"`
		Organization string `json:"organization"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Valid || resp.Name == "" || resp.Status == "" {
		t.Fatalf("expected valid badge with name+status, got %+v body=%s", resp, rec.Body.String())
	}

	bad := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/badges/verify?token="+token+"x", "", nil)
	var badResp struct {
		Valid bool `json:"valid"`
	}
	_ = json.Unmarshal(bad.Body.Bytes(), &badResp)
	if bad.Code != http.StatusOK || badResp.Valid {
		t.Fatalf("expected 200 valid:false for tampered token, got %d body=%s", bad.Code, bad.Body.String())
	}

	missing := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/badges/verify", "", nil)
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing token, got %d", missing.Code)
	}
}
