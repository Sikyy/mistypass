package hikconnect

import (
	"context"
	"errors"
	"testing"
	"time"
)

// --- Mock client ---

type mockISCClient struct {
	accessToken       string
	accessTokenExpiry time.Time
	accessTokenErr    error

	addDeviceErr    error
	deleteDeviceErr error

	deviceToken       string
	deviceTokenExpiry time.Time
	deviceTokenErr    error

	recordings    []RecordingItem
	recordingsErr error
}

func (m *mockISCClient) GetAccessToken(_ context.Context) (string, time.Time, error) {
	return m.accessToken, m.accessTokenExpiry, m.accessTokenErr
}

func (m *mockISCClient) AddDevice(_ context.Context, _, _, _ string) error {
	return m.addDeviceErr
}

func (m *mockISCClient) DeleteDevice(_ context.Context, _, _ string) error {
	return m.deleteDeviceErr
}

func (m *mockISCClient) GetDeviceAccessToken(_ context.Context, _, _ string) (string, time.Time, error) {
	return m.deviceToken, m.deviceTokenExpiry, m.deviceTokenErr
}

func (m *mockISCClient) ListRecordings(_ context.Context, _, _ string, _, _ time.Time) ([]RecordingItem, error) {
	return m.recordings, m.recordingsErr
}

// --- Mock cache ---

type mockCache struct {
	accessToken      string
	accessTokenFound bool
	deviceTokens     map[string]string
}

func newMockCache() *mockCache {
	return &mockCache{deviceTokens: make(map[string]string)}
}

func (m *mockCache) GetHikConnectAccessToken() (string, bool, error) {
	return m.accessToken, m.accessTokenFound, nil
}

func (m *mockCache) SetHikConnectAccessToken(token string, _ time.Duration) error {
	m.accessToken = token
	m.accessTokenFound = true
	return nil
}

func (m *mockCache) DeleteHikConnectAccessToken() error {
	m.accessToken = ""
	m.accessTokenFound = false
	return nil
}

func (m *mockCache) GetHikConnectDeviceToken(serial string) (string, bool, error) {
	tok, ok := m.deviceTokens[serial]
	return tok, ok, nil
}

func (m *mockCache) SetHikConnectDeviceToken(serial, token string, _ time.Duration) error {
	m.deviceTokens[serial] = token
	return nil
}

// --- Tests ---

func TestServiceGetPlaybackToken_CacheMiss(t *testing.T) {
	expiry := time.Now().Add(7 * 24 * time.Hour) // ISC device tokens valid ~7 days
	client := &mockISCClient{
		accessToken:       "platform_tok",
		accessTokenExpiry: time.Now().Add(2 * time.Hour),
		deviceToken:       "device_tok_abc",
		deviceTokenExpiry: expiry,
	}
	cache := newMockCache()
	svc := NewService(client, cache, 115*time.Minute)

	token, err := svc.GetPlaybackToken(context.Background(), "SERIAL1", 1)
	if err != nil {
		t.Fatalf("GetPlaybackToken: %v", err)
	}
	if token.AccessToken != "device_tok_abc" {
		t.Fatalf("expected device_tok_abc, got %q", token.AccessToken)
	}
	if token.DeviceSerial != "SERIAL1" {
		t.Fatalf("expected SERIAL1, got %q", token.DeviceSerial)
	}
	if token.Channel != 1 {
		t.Fatalf("expected channel 1, got %d", token.Channel)
	}

	// Verify token was cached
	cached, found, _ := cache.GetHikConnectDeviceToken("SERIAL1")
	if !found || cached != "device_tok_abc" {
		t.Fatal("expected device token to be cached")
	}
}

func TestServiceGetPlaybackToken_CacheHit(t *testing.T) {
	client := &mockISCClient{
		deviceTokenErr: errors.New("should not be called"),
	}
	cache := newMockCache()
	cache.deviceTokens["SERIAL2"] = "cached_device_tok"
	svc := NewService(client, cache, 115*time.Minute)

	token, err := svc.GetPlaybackToken(context.Background(), "SERIAL2", 3)
	if err != nil {
		t.Fatalf("GetPlaybackToken: %v", err)
	}
	if token.AccessToken != "cached_device_tok" {
		t.Fatalf("expected cached_device_tok, got %q", token.AccessToken)
	}
	if token.Channel != 3 {
		t.Fatalf("expected channel 3, got %d", token.Channel)
	}
}

func TestServiceBindDevice(t *testing.T) {
	client := &mockISCClient{
		accessToken:       "plat_tok",
		accessTokenExpiry: time.Now().Add(2 * time.Hour),
	}
	cache := newMockCache()
	svc := NewService(client, cache, 115*time.Minute)

	err := svc.BindDevice(context.Background(), "SER123", "VERIFY456")
	if err != nil {
		t.Fatalf("BindDevice: %v", err)
	}
}

func TestServiceBindDevice_ISCError(t *testing.T) {
	client := &mockISCClient{
		accessToken:       "plat_tok",
		accessTokenExpiry: time.Now().Add(2 * time.Hour),
		addDeviceErr:      errors.New("ISC add device error: code=20014 msg=device already added"),
	}
	cache := newMockCache()
	svc := NewService(client, cache, 115*time.Minute)

	err := svc.BindDevice(context.Background(), "SER123", "CODE")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestServiceUnbindDevice(t *testing.T) {
	client := &mockISCClient{
		accessToken:       "plat_tok",
		accessTokenExpiry: time.Now().Add(2 * time.Hour),
	}
	cache := newMockCache()
	svc := NewService(client, cache, 115*time.Minute)

	err := svc.UnbindDevice(context.Background(), "SER123")
	if err != nil {
		t.Fatalf("UnbindDevice: %v", err)
	}
}

func TestServiceListRecordings(t *testing.T) {
	client := &mockISCClient{
		accessToken:       "plat_tok",
		accessTokenExpiry: time.Now().Add(2 * time.Hour),
		recordings: []RecordingItem{
			{StartTime: "2026-05-17 10:00:00", EndTime: "2026-05-17 10:30:00", Type: 1},
		},
	}
	cache := newMockCache()
	svc := NewService(client, cache, 115*time.Minute)

	start := time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 17, 23, 59, 59, 0, time.UTC)
	recs, err := svc.ListRecordings(context.Background(), "SER1", start, end)
	if err != nil {
		t.Fatalf("ListRecordings: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(recs))
	}
	if recs[0].Type != "motion" {
		t.Fatalf("expected type motion, got %q", recs[0].Type)
	}
}
