package hikconnect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientGetAccessToken(t *testing.T) {
	expireMs := time.Now().Add(2 * time.Hour).UnixMilli()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/lapp/token/get" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.FormValue("appKey") != "test-key" {
			t.Fatalf("expected appKey test-key, got %q", r.FormValue("appKey"))
		}
		if r.FormValue("appSecret") != "test-secret" {
			t.Fatalf("expected appSecret test-secret, got %q", r.FormValue("appSecret"))
		}
		json.NewEncoder(w).Encode(TokenResponse{
			Code: "200",
			Msg:  "success",
			Data: TokenDataField{
				AccessToken: "at_mock_token",
				ExpireTime:  expireMs,
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "test-secret")
	token, expiresAt, err := client.GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAccessToken: %v", err)
	}
	if token != "at_mock_token" {
		t.Fatalf("expected at_mock_token, got %q", token)
	}
	if expiresAt.IsZero() {
		t.Fatal("expected non-zero expiry")
	}
}

func TestClientAddDevice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/lapp/device/add" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.FormValue("accessToken") != "tok123" {
			t.Fatalf("expected accessToken tok123, got %q", r.FormValue("accessToken"))
		}
		if r.FormValue("deviceSerial") != "DS123456" {
			t.Fatalf("expected deviceSerial DS123456, got %q", r.FormValue("deviceSerial"))
		}
		json.NewEncoder(w).Encode(DeviceResponse{Code: "200", Msg: "success"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "k", "s")
	err := client.AddDevice(context.Background(), "tok123", "DS123456", "ABCDEF")
	if err != nil {
		t.Fatalf("AddDevice: %v", err)
	}
}

func TestClientAddDeviceError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(DeviceResponse{Code: "20014", Msg: "device already added"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "k", "s")
	err := client.AddDevice(context.Background(), "tok", "SER", "CODE")
	if err == nil {
		t.Fatal("expected error for non-200 code")
	}
}

func TestClientGetDeviceAccessToken(t *testing.T) {
	expireMs := time.Now().Add(7 * 24 * time.Hour).UnixMilli()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/lapp/token/getDeviceAccessToken" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(DeviceAccessTokenResponse{
			Code: "200",
			Msg:  "success",
			Data: DeviceAccessTokenDataField{
				AccessToken: "device_at_mock",
				ExpireTime:  expireMs,
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "k", "s")
	token, expiresAt, err := client.GetDeviceAccessToken(context.Background(), "platform_tok", "SERIAL1")
	if err != nil {
		t.Fatalf("GetDeviceAccessToken: %v", err)
	}
	if token != "device_at_mock" {
		t.Fatalf("expected device_at_mock, got %q", token)
	}
	if expiresAt.IsZero() {
		t.Fatal("expected non-zero expiry")
	}
}

func TestClientListRecordings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/lapp/video/by-time" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(RecordingListResponse{
			Code: "200",
			Msg:  "success",
			Data: []RecordingItem{
				{StartTime: "2026-05-17 10:00:00", EndTime: "2026-05-17 10:30:00", Type: 1},
				{StartTime: "2026-05-17 14:00:00", EndTime: "2026-05-17 14:15:00", Type: 2},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "k", "s")
	start := time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 17, 23, 59, 59, 0, time.UTC)
	recordings, err := client.ListRecordings(context.Background(), "tok", "SER1", start, end)
	if err != nil {
		t.Fatalf("ListRecordings: %v", err)
	}
	if len(recordings) != 2 {
		t.Fatalf("expected 2 recordings, got %d", len(recordings))
	}
}
