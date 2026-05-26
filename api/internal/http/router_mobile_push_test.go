package httpx

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

type mobilePushMemoryStateStore struct {
	mu   sync.Mutex
	data map[string]json.RawMessage
}

func newMobilePushMemoryStateStore() *mobilePushMemoryStateStore {
	return &mobilePushMemoryStateStore{data: map[string]json.RawMessage{}}
}

func (s *mobilePushMemoryStateStore) Load(key string, dst any) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, ok := s.data[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, dst)
}

func (s *mobilePushMemoryStateStore) Save(key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.data[key] = raw
	return nil
}

func TestAppRegisterDevicePersistsPushTokenForSmokeTargeting(t *testing.T) {
	store := newMobilePushMemoryStateStore()
	cfg := config.Config{
		JWTSecret:       "mobile-push-test-jwt",
		JWTIssuer:       "test",
		EnableDemoUsers: true,
	}
	router, _, err := NewRouter(cfg, store)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	registerBody := []byte(`{"fcm_token":"fcm-token-1","device_id":"android-device-1","device_model":"Xiaomi 15","platform":"android"}`)
	registerRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/devices/register", token, registerBody)
	if registerRecorder.Code != http.StatusOK {
		t.Fatalf("register status %d body=%s", registerRecorder.Code, registerRecorder.Body.String())
	}

	statusRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/mobile-push/provider-status?tenant_id=tenant_demo_jakarta", token, nil)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", statusRecorder.Code, statusRecorder.Body.String())
	}
	var status struct {
		Enabled                 bool     `json:"enabled"`
		Missing                 []string `json:"missing"`
		RegisteredAndroidTokens int      `json:"registered_android_tokens"`
	}
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Enabled {
		t.Fatalf("expected fcm disabled")
	}
	if status.RegisteredAndroidTokens != 1 {
		t.Fatalf("expected one registered token, got %d", status.RegisteredAndroidTokens)
	}
	if len(status.Missing) == 0 || status.Missing[0] != "FCM_ENABLED" {
		t.Fatalf("expected FCM_ENABLED missing, got %#v", status.Missing)
	}

	restartedRouter, _, err := NewRouter(cfg, store)
	if err != nil {
		t.Fatalf("NewRouter restarted: %v", err)
	}
	restartedToken := referenceAPILogin(t, restartedRouter, "organization.admin@mistypass.local")
	restartedStatusRecorder := referenceAPIRequest(t, restartedRouter, http.MethodGet, "/api/v1/mobile-push/provider-status?tenant_id=tenant_demo_jakarta", restartedToken, nil)
	if restartedStatusRecorder.Code != http.StatusOK {
		t.Fatalf("restarted status %d body=%s", restartedStatusRecorder.Code, restartedStatusRecorder.Body.String())
	}
	if err := json.Unmarshal(restartedStatusRecorder.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode restarted status: %v", err)
	}
	if status.RegisteredAndroidTokens != 1 {
		t.Fatalf("expected restored token count 1, got %d", status.RegisteredAndroidTokens)
	}
}
