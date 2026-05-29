package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func callGatewayVerifyCredential(t *testing.T, s *server, deviceToken string, headers, body map[string]string) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal verify body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/verify-credential", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if deviceToken != "" {
		req.Header.Set("X-Device-Token", deviceToken)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	s.verifyCredentialGateway(rec, req)
	out := map[string]any{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

// The /api/v1/gateway/verify-credential route is served on the public (non-mTLS)
// listener. It must reject callers that present no gateway device identity, and
// must never return an access decision or trigger an unlock for them.
func TestGatewayVerifyCredentialRejectsUnauthenticated(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "verify-gw-auth-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	body := []byte(`{
		"gateway_id": "gw_demo_001",
		"reader_id": "gdv_demo_001",
		"lock_id": "door_jkt_001",
		"tenant_id": "tenant_demo_jakarta",
		"credential_type": "nfc_uid",
		"credential_data": "UID-1001"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/verify-credential", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated gateway verify, got %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"decision":"allow"`) {
		t.Fatalf("unauthenticated request must not return an access decision: %s", rec.Body.String())
	}
}

// A gateway presenting a valid device token reaches credential evaluation.
// An empty credential is used so the assertion exercises the auth gate without
// depending on wallet/event services in the lightweight test server.
func TestGatewayVerifyCredentialAllowsAuthenticatedGateway(t *testing.T) {
	s := newGatewayDeviceRequestTestServer()
	s.setGatewayDeviceToken("gw_demo_001", "dev-token-verify")

	code, body := callGatewayVerifyCredential(t, s, "dev-token-verify", gatewayRequestHeaders("nonce-allow-1"), map[string]string{
		"gateway_id":      "gw_demo_001",
		"tenant_id":       "tenant_demo_jakarta",
		"lock_id":         "door_jkt_001",
		"credential_type": "nfc_uid",
		"credential_data": "",
	})
	if code != http.StatusOK {
		t.Fatalf("expected authenticated gateway verify to reach evaluation (200), got %d body=%v", code, body)
	}
	if body["decision"] != "deny" || body["reason"] != "empty_credential" {
		t.Fatalf("expected deny/empty_credential after auth, got %v", body)
	}
}

// A device token authenticates only the gateway it was issued for. It must not
// let the caller claim a tenant the gateway does not belong to.
func TestGatewayVerifyCredentialRejectsTenantForgery(t *testing.T) {
	s := newGatewayDeviceRequestTestServer()
	s.setGatewayDeviceToken("gw_demo_001", "dev-token-verify")

	code, _ := callGatewayVerifyCredential(t, s, "dev-token-verify", nil, map[string]string{
		"gateway_id":      "gw_demo_001",
		"tenant_id":       "tenant_demo_factory",
		"lock_id":         "door_jkt_001",
		"credential_type": "nfc_uid",
		"credential_data": "UID-1001",
	})
	if code != http.StatusUnauthorized {
		t.Fatalf("expected tenant forgery to be rejected (401), got %d", code)
	}
}

// The verify endpoint triggers unlocks, so it must require a per-request nonce
// even though other gateway telemetry endpoints treat it as optional.
func TestGatewayVerifyCredentialRequiresRequestNonce(t *testing.T) {
	s := newGatewayDeviceRequestTestServer()
	s.setGatewayDeviceToken("gw_demo_001", "dev-token-verify")

	// Authenticated, but no X-Request-Nonce header.
	code, _ := callGatewayVerifyCredential(t, s, "dev-token-verify", nil, map[string]string{
		"gateway_id":      "gw_demo_001",
		"tenant_id":       "tenant_demo_jakarta",
		"lock_id":         "door_jkt_001",
		"credential_type": "nfc_uid",
		"credential_data": "",
	})
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when request nonce is missing, got %d", code)
	}
}

// A request nonce is single-use: replaying it must be rejected, which prevents
// replaying a captured verify request.
func TestGatewayVerifyCredentialRejectsReplayedNonce(t *testing.T) {
	s := newGatewayDeviceRequestTestServer()
	s.setGatewayDeviceToken("gw_demo_001", "dev-token-verify")

	headers := gatewayRequestHeaders("nonce-replay-1")
	body := map[string]string{
		"gateway_id":      "gw_demo_001",
		"tenant_id":       "tenant_demo_jakarta",
		"lock_id":         "door_jkt_001",
		"credential_type": "nfc_uid",
		"credential_data": "",
	}

	code, _ := callGatewayVerifyCredential(t, s, "dev-token-verify", headers, body)
	if code != http.StatusOK {
		t.Fatalf("expected first request with fresh nonce to be accepted (200), got %d", code)
	}

	code, _ = callGatewayVerifyCredential(t, s, "dev-token-verify", headers, body)
	if code != http.StatusConflict {
		t.Fatalf("expected replayed nonce to be rejected (409), got %d", code)
	}
}
