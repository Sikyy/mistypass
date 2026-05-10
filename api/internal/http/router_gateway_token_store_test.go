package httpx

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

type stubGatewayTokenPersistence struct {
	loadFound     bool
	loadErr       error
	loadedTokens  map[string]string
	upsertErr     error
	verifyExists  bool
	verifyMatched bool
	verifyErr     error
	upsertCalls   []struct{ gatewayID, token string }
	verifyCalls   []struct{ gatewayID, token string }
}

func (s *stubGatewayTokenPersistence) Load(_ string, dst any) (bool, error) {
	if s.loadErr != nil {
		return false, s.loadErr
	}
	snapshot, ok := dst.(*gatewayBootstrapStateSnapshot)
	if !ok {
		return false, errors.New("unexpected snapshot type")
	}
	snapshot.DeviceTokens = map[string]string{}
	for gatewayID, token := range s.loadedTokens {
		snapshot.DeviceTokens[gatewayID] = token
	}
	return s.loadFound, nil
}

func (s *stubGatewayTokenPersistence) Save(_ string, _ any) error {
	return nil
}

func (s *stubGatewayTokenPersistence) UpsertGatewayDeviceToken(gatewayID, deviceToken string) error {
	s.upsertCalls = append(s.upsertCalls, struct{ gatewayID, token string }{gatewayID: gatewayID, token: deviceToken})
	return s.upsertErr
}

func (s *stubGatewayTokenPersistence) VerifyGatewayDeviceToken(gatewayID, providedToken string) (bool, bool, error) {
	s.verifyCalls = append(s.verifyCalls, struct{ gatewayID, token string }{gatewayID: gatewayID, token: providedToken})
	return s.verifyExists, s.verifyMatched, s.verifyErr
}

func TestRestoreGatewayBootstrapStateMigratesLegacyTokens(t *testing.T) {
	store := &stubGatewayTokenPersistence{
		loadFound: true,
		loadedTokens: map[string]string{
			"gw_1": "token_1",
			"gw_2": "token_2",
		},
	}
	s := &server{
		stateStore:        store,
		gatewayTokenStore: store,
	}

	if err := s.restoreGatewayBootstrapState(); err != nil {
		t.Fatalf("restoreGatewayBootstrapState failed: %v", err)
	}
	if len(store.upsertCalls) != 2 {
		t.Fatalf("expected 2 migrated tokens, got %d", len(store.upsertCalls))
	}
}

func TestSetGatewayDeviceTokenUsesTokenStore(t *testing.T) {
	store := &stubGatewayTokenPersistence{}
	s := &server{gatewayTokenStore: store}

	s.setGatewayDeviceToken("gw_1", "token_1")
	if len(store.upsertCalls) != 1 {
		t.Fatalf("expected 1 upsert call, got %d", len(store.upsertCalls))
	}
	if store.upsertCalls[0].gatewayID != "gw_1" || store.upsertCalls[0].token != "token_1" {
		t.Fatalf("unexpected upsert payload: %+v", store.upsertCalls[0])
	}
}

func TestAuthorizeGatewayDeviceTokenUsesTokenStore(t *testing.T) {
	store := &stubGatewayTokenPersistence{verifyExists: true, verifyMatched: true}
	s := &server{gatewayTokenStore: store}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/heartbeat", nil)
	req.Header.Set("X-Device-Token", "token_1")
	rec := httptest.NewRecorder()
	if ok := s.authorizeGatewayDeviceToken(rec, req, "gw_1"); !ok {
		t.Fatalf("expected authorization success")
	}
	if len(store.verifyCalls) != 1 {
		t.Fatalf("expected 1 verify call, got %d", len(store.verifyCalls))
	}
}

func TestAuthorizeGatewayDeviceTokenStoreFailureReturns500(t *testing.T) {
	store := &stubGatewayTokenPersistence{verifyErr: errors.New("boom")}
	s := &server{gatewayTokenStore: store}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/heartbeat", nil)
	req.Header.Set("X-Device-Token", "token_1")
	rec := httptest.NewRecorder()
	if ok := s.authorizeGatewayDeviceToken(rec, req, "gw_1"); ok {
		t.Fatalf("expected authorization failure")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestAuthorizeGatewayDeviceTokenAcceptsVerifiedClientCertificate(t *testing.T) {
	gatewaySvc := gateway.NewService()
	s := &server{gatewaySvc: gatewaySvc}
	leaf := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:   "gw_demo_001",
			Organization: []string{"tenant_demo_jakarta"},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/heartbeat", nil)
	req.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{leaf},
		VerifiedChains:   [][]*x509.Certificate{{leaf}},
	}
	rec := httptest.NewRecorder()
	if ok := s.authorizeGatewayDeviceToken(rec, req, "gw_demo_001"); !ok {
		t.Fatalf("expected client certificate authorization success, status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthorizeGatewayDeviceTokenRejectsUnverifiedClientCertificate(t *testing.T) {
	leaf := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:   "gw_demo_001",
			Organization: []string{"tenant_demo_jakarta"},
		},
	}
	s := &server{
		gatewayDeviceTokens: map[string]string{"gw_demo_001": "token_1"},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/heartbeat", nil)
	req.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{leaf},
	}
	rec := httptest.NewRecorder()
	if ok := s.authorizeGatewayDeviceToken(rec, req, "gw_demo_001"); ok {
		t.Fatalf("expected unverified client certificate authorization failure")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
}

func TestAuthorizeGatewayDeviceTokenRejectsClientCertificateTenantMismatch(t *testing.T) {
	gatewaySvc := gateway.NewService()
	leaf := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:   "gw_demo_001",
			Organization: []string{"tenant_other"},
		},
	}
	s := &server{
		gatewaySvc:          gatewaySvc,
		gatewayDeviceTokens: map[string]string{"gw_demo_001": "token_1"},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/heartbeat", nil)
	req.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{leaf},
		VerifiedChains:   [][]*x509.Certificate{{leaf}},
	}
	rec := httptest.NewRecorder()
	if ok := s.authorizeGatewayDeviceToken(rec, req, "gw_demo_001"); ok {
		t.Fatalf("expected tenant mismatch authorization failure")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
}

func TestAuthorizeGatewayDeviceTokenRejectsRevokedClientCertificateSerial(t *testing.T) {
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(0xabc123),
		Subject: pkix.Name{
			CommonName:   "gw_demo_001",
			Organization: []string{"tenant_demo_jakarta"},
		},
	}
	s := &server{
		cfg:        config.Config{GatewayMTLSRevokedSerials: []string{"AB:C1:23"}},
		gatewaySvc: gateway.NewService(),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/heartbeat", nil)
	req.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{leaf},
		VerifiedChains:   [][]*x509.Certificate{{leaf}},
	}
	rec := httptest.NewRecorder()
	if ok := s.authorizeGatewayDeviceToken(rec, req, "gw_demo_001"); ok {
		t.Fatalf("expected revoked client certificate authorization failure")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
}

func TestAuthorizeGatewayDeviceTokenRejectsRuntimeRevokedClientCertificateSerial(t *testing.T) {
	gatewaySvc := gateway.NewService()
	if _, err := gatewaySvc.RevokeCertificateSerial("tenant_demo_jakarta", "gw_demo_001", "AB:C1:23", "test", "tester@example.com"); err != nil {
		t.Fatalf("revoke certificate serial: %v", err)
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(0xabc123),
		Subject: pkix.Name{
			CommonName:   "gw_demo_001",
			Organization: []string{"tenant_demo_jakarta"},
		},
	}
	s := &server{
		gatewaySvc: gatewaySvc,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/heartbeat", nil)
	req.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{leaf},
		VerifiedChains:   [][]*x509.Certificate{{leaf}},
	}
	rec := httptest.NewRecorder()
	if ok := s.authorizeGatewayDeviceToken(rec, req, "gw_demo_001"); ok {
		t.Fatalf("expected runtime revoked client certificate authorization failure")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
}

func TestAuthorizeGatewayDeviceTokenRejectsRevokedGatewayStatus(t *testing.T) {
	gatewaySvc := gateway.NewService()
	if _, err := gatewaySvc.UpdateGatewayStatus("tenant_demo_jakarta", "gw_demo_001", "revoked"); err != nil {
		t.Fatalf("update gateway status: %v", err)
	}
	leaf := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:   "gw_demo_001",
			Organization: []string{"tenant_demo_jakarta"},
		},
	}
	s := &server{gatewaySvc: gatewaySvc}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/heartbeat", nil)
	req.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{leaf},
		VerifiedChains:   [][]*x509.Certificate{{leaf}},
	}
	rec := httptest.NewRecorder()
	if ok := s.authorizeGatewayDeviceToken(rec, req, "gw_demo_001"); ok {
		t.Fatalf("expected revoked gateway certificate authorization failure")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
}

func TestAuthorizeGatewayDeviceTokenRejectsRevokedGatewayStatusForTokenFallback(t *testing.T) {
	gatewaySvc := gateway.NewService()
	if _, err := gatewaySvc.UpdateGatewayStatus("tenant_demo_jakarta", "gw_demo_001", "disabled"); err != nil {
		t.Fatalf("update gateway status: %v", err)
	}
	store := &stubGatewayTokenPersistence{verifyExists: true, verifyMatched: true}
	s := &server{
		gatewaySvc:        gatewaySvc,
		gatewayTokenStore: store,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/heartbeat", nil)
	req.Header.Set("X-Device-Token", "token_1")
	rec := httptest.NewRecorder()
	if ok := s.authorizeGatewayDeviceToken(rec, req, "gw_demo_001"); ok {
		t.Fatalf("expected disabled gateway token authorization failure")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
	if len(store.verifyCalls) != 0 {
		t.Fatalf("expected revoked gateway to bypass token store, got %d calls", len(store.verifyCalls))
	}
}

func TestAuthorizeGatewayDeviceTokenRejectsTokenFallbackWhenGatewayMTLSRequired(t *testing.T) {
	store := &stubGatewayTokenPersistence{verifyExists: true, verifyMatched: true}
	s := &server{gatewayTokenStore: store}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/heartbeat", nil)
	req = req.WithContext(context.WithValue(req.Context(), gatewayMTLSRequiredContextKey{}, true))
	req.Header.Set("X-Device-Token", "token_1")
	rec := httptest.NewRecorder()
	if ok := s.authorizeGatewayDeviceToken(rec, req, "gw_1"); ok {
		t.Fatalf("expected token fallback to fail when gateway mTLS is required")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
	if len(store.verifyCalls) != 0 {
		t.Fatalf("expected no token-store fallback, got %d calls", len(store.verifyCalls))
	}
}

func TestAuthorizeGatewayWebSocketDeviceTokenPrefersHeader(t *testing.T) {
	store := &stubGatewayTokenPersistence{verifyExists: true, verifyMatched: true}
	s := &server{gatewayTokenStore: store}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gateway/ws", nil)
	req.Header.Set("X-Device-Token", "header_token")
	rec := httptest.NewRecorder()
	if ok := s.authorizeGatewayWebSocketDeviceToken(rec, req, "gw_1"); !ok {
		t.Fatalf("expected authorization success")
	}
	if len(store.verifyCalls) != 1 {
		t.Fatalf("expected 1 verify call, got %d", len(store.verifyCalls))
	}
	if store.verifyCalls[0].token != "header_token" {
		t.Fatalf("expected header token, got %q", store.verifyCalls[0].token)
	}
}

func TestAuthorizeGatewayWebSocketDeviceTokenRejectsQueryToken(t *testing.T) {
	store := &stubGatewayTokenPersistence{verifyExists: true, verifyMatched: true}
	s := &server{gatewayTokenStore: store}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gateway/ws?token=legacy_query_token", nil)
	rec := httptest.NewRecorder()
	if ok := s.authorizeGatewayWebSocketDeviceToken(rec, req, "gw_1"); ok {
		t.Fatalf("expected query token authorization failure")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
	if len(store.verifyCalls) != 0 {
		t.Fatalf("expected query token to bypass token store, got %d calls", len(store.verifyCalls))
	}
}
