package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/camera"
	"github.com/mistypass/cloud/api/internal/modules/hikconnect"
)

type appCameraCloudMockClient struct {
	mu               sync.Mutex
	accessTokenCalls int
	deviceTokenCalls int
	recordingCalls   int
}

func (m *appCameraCloudMockClient) GetAccessToken(context.Context) (string, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accessTokenCalls++
	return "platform-token", time.Now().UTC().Add(time.Hour), nil
}

func (m *appCameraCloudMockClient) AddDevice(context.Context, string, string, string) error {
	return nil
}

func (m *appCameraCloudMockClient) DeleteDevice(context.Context, string, string) error {
	return nil
}

func (m *appCameraCloudMockClient) GetDeviceAccessToken(context.Context, string, string) (string, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deviceTokenCalls++
	return "device-token-cloud-001", time.Now().UTC().Add(24 * time.Hour), nil
}

func (m *appCameraCloudMockClient) ListRecordings(context.Context, string, string, time.Time, time.Time) ([]hikconnect.RecordingItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordingCalls++
	return []hikconnect.RecordingItem{
		{StartTime: "2026-05-24 09:00:00", EndTime: "2026-05-24 09:30:00", Type: 2},
	}, nil
}

type appCameraCloudMockCache struct {
	mu           sync.Mutex
	accessToken  string
	deviceTokens map[string]string
}

func (m *appCameraCloudMockCache) GetHikConnectAccessToken() (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.accessToken == "" {
		return "", false, nil
	}
	return m.accessToken, true, nil
}

func (m *appCameraCloudMockCache) SetHikConnectAccessToken(token string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accessToken = token
	return nil
}

func (m *appCameraCloudMockCache) DeleteHikConnectAccessToken() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accessToken = ""
	return nil
}

func (m *appCameraCloudMockCache) GetHikConnectDeviceToken(serial string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deviceTokens == nil {
		return "", false, nil
	}
	token := m.deviceTokens[serial]
	if token == "" {
		return "", false, nil
	}
	return token, true, nil
}

func (m *appCameraCloudMockCache) SetHikConnectDeviceToken(serial, token string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deviceTokens == nil {
		m.deviceTokens = make(map[string]string)
	}
	m.deviceTokens[serial] = token
	return nil
}

func TestAppCameraCloudTokenAndRecordings(t *testing.T) {
	srv, handler := newTestServerWithHandler(t, config.Config{
		JWTSecret:       "app-camera-cloud-test",
		EnableDemoUsers: true,
	})

	cam, err := srv.cameraSvc.Create(camera.CameraCreateRequest{
		TenantID: "tenant_demo_jakarta",
		PlaceID:  "building_demo_001",
		DoorID:   "door_jkt_001",
		Name:     "Cloud Lobby Camera",
		Provider: camera.ProviderHikvision,
		Host:     "192.0.2.10",
		Channel:  1,
	})
	if err != nil {
		t.Fatalf("create camera: %v", err)
	}

	cloudProvider := "hikconnect"
	cloudSerial := "DS-2CD1023G2-LIU1234567"
	cloudVerified := true
	cloudChannels := 2
	if _, err := srv.cameraSvc.Update("tenant_demo_jakarta", cam.ID, camera.CameraUpdateRequest{
		CloudProvider: &cloudProvider,
		CloudSerial:   &cloudSerial,
		CloudVerified: &cloudVerified,
		CloudChannels: &cloudChannels,
	}); err != nil {
		t.Fatalf("update camera cloud binding: %v", err)
	}

	client := &appCameraCloudMockClient{}
	srv.hikConnectSvc = hikconnect.NewService(client, &appCameraCloudMockCache{}, time.Hour)

	token := referenceAPILogin(t, handler, "tenant.admin@sudirman.co")
	tokenRec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/app/cameras/"+cam.ID+"/cloud-token", token, nil)
	if tokenRec.Code != http.StatusOK {
		t.Fatalf("expected cloud-token 200, got %d body=%s", tokenRec.Code, tokenRec.Body.String())
	}
	var tokenPayload struct {
		AccessToken  string `json:"access_token"`
		DeviceSerial string `json:"device_serial"`
		Channel      int    `json:"channel"`
		ExpiresAt    string `json:"expires_at"`
	}
	if err := json.Unmarshal(tokenRec.Body.Bytes(), &tokenPayload); err != nil {
		t.Fatalf("decode cloud-token: %v", err)
	}
	if tokenPayload.AccessToken != "device-token-cloud-001" ||
		tokenPayload.DeviceSerial != cloudSerial ||
		tokenPayload.Channel != cloudChannels ||
		tokenPayload.ExpiresAt == "" {
		t.Fatalf("unexpected cloud-token payload: %+v", tokenPayload)
	}

	recordingsRec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/app/cameras/"+cam.ID+"/cloud-recordings?date=2026-05-24", token, nil)
	if recordingsRec.Code != http.StatusOK {
		t.Fatalf("expected cloud-recordings 200, got %d body=%s", recordingsRec.Code, recordingsRec.Body.String())
	}
	var recordingsPayload struct {
		CameraID   string `json:"camera_id"`
		Date       string `json:"date"`
		Recordings []struct {
			StartTime string `json:"start_time"`
			EndTime   string `json:"end_time"`
			Type      string `json:"type"`
		} `json:"recordings"`
	}
	if err := json.Unmarshal(recordingsRec.Body.Bytes(), &recordingsPayload); err != nil {
		t.Fatalf("decode cloud-recordings: %v", err)
	}
	if recordingsPayload.CameraID != cam.ID || recordingsPayload.Date != "2026-05-24" {
		t.Fatalf("unexpected cloud-recordings envelope: %+v", recordingsPayload)
	}
	if len(recordingsPayload.Recordings) != 1 ||
		recordingsPayload.Recordings[0].Type != "continuous" ||
		recordingsPayload.Recordings[0].StartTime == "" ||
		recordingsPayload.Recordings[0].EndTime == "" {
		t.Fatalf("unexpected cloud-recordings items: %+v", recordingsPayload.Recordings)
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	if client.deviceTokenCalls != 1 || client.recordingCalls != 1 {
		t.Fatalf("expected device token and recording calls once, got device=%d recordings=%d", client.deviceTokenCalls, client.recordingCalls)
	}
}
