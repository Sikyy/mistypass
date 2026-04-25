package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
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
