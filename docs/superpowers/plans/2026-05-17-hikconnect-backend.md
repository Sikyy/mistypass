# Hik-Connect Backend Integration Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add ISC OpenAPI integration to the MistyPass backend so the iOS app can request cloud video tokens and recording segments for Hikvision cameras bound via Hik-Connect.

**Architecture:** New `hikconnect/` module with HTTP client (ISC OpenAPI) and service layer. Redis caches access tokens and device tokens. New API routes expose cloud-token and cloud-recordings to the mobile app, plus admin endpoints for device binding.

**Tech Stack:** Go 1.22+, `net/http` (ISC client), `github.com/redis/go-redis/v9` (caching via existing redistore), `chi` router (existing)

**Note:** iOS client plan (ISC OpenSDK integration) is a separate plan — blocked on ISV account setup and SDK binary availability.

---

## File Structure

| File | Responsibility |
|------|---------------|
| Create: `api/internal/modules/hikconnect/models.go` | ISC API request/response types, domain types (PlaybackToken, Recording) |
| Create: `api/internal/modules/hikconnect/client.go` | HTTP client wrapping ISC OpenAPI endpoints |
| Create: `api/internal/modules/hikconnect/client_test.go` | Unit tests with httptest mock server |
| Create: `api/internal/modules/hikconnect/service.go` | Business logic: bind, unbind, get token, list recordings |
| Create: `api/internal/modules/hikconnect/service_test.go` | Unit tests with mock client and mock cache |
| Modify: `api/internal/modules/camera/models.go` | Add cloud fields (CloudProvider, CloudSerial, CloudVerified, CloudChannels) |
| Modify: `api/internal/redistore/store.go` | Add hikconnect token cache methods (Get/Set/Del with TTL) |
| Modify: `api/internal/redistore/store_test.go` | Tests for new cache methods |
| Modify: `api/internal/config/config.go` | Add HikISC config fields (Host, AppKey, AppSecret, TokenCacheTTL) |
| Modify: `api/internal/http/routes_app_redesign.go` | Add cloud-token, cloud-recordings, cloud-bind endpoints |
| Modify: `api/internal/http/router.go` | Wire hikconnect service into server struct |

---

### Task 1: ISC Domain Models

**Files:**
- Create: `api/internal/modules/hikconnect/models.go`

- [ ] **Step 1: Create the models file with ISC API types**

```go
package hikconnect

import "time"

// --- ISC OpenAPI request/response types ---

// TokenResponse is the ISC /api/lapp/token/get response.
type TokenResponse struct {
	Code string         `json:"code"`
	Msg  string         `json:"msg"`
	Data TokenDataField `json:"data"`
}

type TokenDataField struct {
	AccessToken string `json:"accessToken"`
	ExpireTime  int64  `json:"expireTime"` // milliseconds since epoch
}

// DeviceAddRequest is the body for /api/lapp/device/add.
type DeviceAddRequest struct {
	AccessToken string `json:"accessToken"`
	DeviceSerial string `json:"deviceSerial"`
	ValidateCode string `json:"validateCode"`
}

// DeviceResponse is a generic ISC device operation response.
type DeviceResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

// DeviceListResponse is the ISC /api/lapp/device/list response.
type DeviceListResponse struct {
	Code string              `json:"code"`
	Msg  string              `json:"msg"`
	Data []DeviceListItem    `json:"data"`
}

type DeviceListItem struct {
	DeviceSerial string `json:"deviceSerial"`
	DeviceName   string `json:"deviceName"`
	Status       int    `json:"status"` // 1=online, 2=offline
	Defence      int    `json:"defence"`
	DeviceType   string `json:"deviceType"`
}

// DeviceAccessTokenResponse is the ISC /api/lapp/token/getDeviceAccessToken response.
type DeviceAccessTokenResponse struct {
	Code string                      `json:"code"`
	Msg  string                      `json:"msg"`
	Data DeviceAccessTokenDataField  `json:"data"`
}

type DeviceAccessTokenDataField struct {
	AccessToken string `json:"accessToken"`
	ExpireTime  int64  `json:"expireTime"`
}

// RecordingListResponse is the ISC /api/lapp/video/by-time response.
type RecordingListResponse struct {
	Code string           `json:"code"`
	Msg  string           `json:"msg"`
	Data []RecordingItem  `json:"data"`
}

type RecordingItem struct {
	StartTime string `json:"startTime"` // "2026-05-17 10:00:00"
	EndTime   string `json:"endTime"`
	Type      int    `json:"type"`
}

// --- Domain types (returned to callers) ---

// PlaybackToken is what the mobile app needs to initialize the ISC SDK.
type PlaybackToken struct {
	AccessToken  string    `json:"access_token"`
	DeviceSerial string    `json:"device_serial"`
	Channel      int       `json:"channel"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Recording represents a cloud recording segment.
type Recording struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Type      string    `json:"type"` // "motion", "continuous", "alarm"
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/siky/code/MistyPass/api && go build ./internal/modules/hikconnect/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add api/internal/modules/hikconnect/models.go
git commit -m "feat(hikconnect): add ISC OpenAPI domain models"
```

---

### Task 2: Camera Model Cloud Extensions

**Files:**
- Modify: `api/internal/modules/camera/models.go:6-22`

- [ ] **Step 1: Write test for cloud fields on Camera struct**

Add to `api/internal/modules/camera/service_test.go`:

```go
func TestCameraCloudFields(t *testing.T) {
	svc := NewService(nil)
	cam, err := svc.Create(CameraCreateRequest{
		TenantID: "t1",
		PlaceID:  "p1",
		Name:     "Cloud Camera",
		Provider: "hikvision",
		Host:     "192.168.1.100",
		Username: "admin",
		Password: "pass123",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Update cloud fields
	cloudProvider := "hikconnect"
	cloudSerial := "DS-2CD1023G2-LIU1234567"
	cloudVerified := true
	cloudChannels := 1
	updated, err := svc.Update(cam.TenantID, cam.ID, CameraUpdateRequest{
		CloudProvider: &cloudProvider,
		CloudSerial:   &cloudSerial,
		CloudVerified: &cloudVerified,
		CloudChannels: &cloudChannels,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.CloudProvider != "hikconnect" {
		t.Fatalf("expected CloudProvider hikconnect, got %q", updated.CloudProvider)
	}
	if updated.CloudSerial != "DS-2CD1023G2-LIU1234567" {
		t.Fatalf("expected CloudSerial DS-2CD1023G2-LIU1234567, got %q", updated.CloudSerial)
	}
	if !updated.CloudVerified {
		t.Fatal("expected CloudVerified true")
	}
	if updated.CloudChannels != 1 {
		t.Fatalf("expected CloudChannels 1, got %d", updated.CloudChannels)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/siky/code/MistyPass/api && go test ./internal/modules/camera/ -run TestCameraCloudFields -v`
Expected: FAIL — `CloudProvider` field does not exist

- [ ] **Step 3: Add cloud fields to Camera struct and CameraUpdateRequest**

In `api/internal/modules/camera/models.go`, add fields to the `Camera` struct after `LastSnapshotAt`:

```go
CloudProvider string `json:"cloud_provider,omitempty"`
CloudSerial   string `json:"cloud_serial,omitempty"`
CloudVerified bool   `json:"cloud_verified,omitempty"`
CloudChannels int    `json:"cloud_channels,omitempty"`
```

In `CameraUpdateRequest`, add:

```go
CloudProvider *string `json:"cloud_provider,omitempty"`
CloudSerial   *string `json:"cloud_serial,omitempty"`
CloudVerified *bool   `json:"cloud_verified,omitempty"`
CloudChannels *int    `json:"cloud_channels,omitempty"`
```

- [ ] **Step 4: Handle cloud fields in Update method**

In `api/internal/modules/camera/service.go`, inside the `Update` function (after the `if req.Status != nil` block), add:

```go
if req.CloudProvider != nil {
	cam.CloudProvider = strings.TrimSpace(*req.CloudProvider)
}
if req.CloudSerial != nil {
	cam.CloudSerial = strings.TrimSpace(*req.CloudSerial)
}
if req.CloudVerified != nil {
	cam.CloudVerified = *req.CloudVerified
}
if req.CloudChannels != nil {
	cam.CloudChannels = *req.CloudChannels
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /Users/siky/code/MistyPass/api && go test ./internal/modules/camera/ -run TestCameraCloudFields -v`
Expected: PASS

- [ ] **Step 6: Run all camera tests to check for regressions**

Run: `cd /Users/siky/code/MistyPass/api && go test ./internal/modules/camera/ -v`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add api/internal/modules/camera/models.go api/internal/modules/camera/service.go api/internal/modules/camera/service_test.go
git commit -m "feat(camera): add cloud provider fields for Hik-Connect binding"
```

---

### Task 3: Redis Cache Methods for Hik-Connect Tokens

**Files:**
- Modify: `api/internal/redistore/store.go`
- Modify: `api/internal/redistore/store_test.go`

- [ ] **Step 1: Write tests for hikconnect cache methods**

Add to `api/internal/redistore/store_test.go`:

```go
func TestHikConnectTokenCache(t *testing.T) {
	store := setupTestStore(t)

	// Set and get access token
	err := store.SetHikConnectAccessToken("test-access-token-123", 2*time.Hour)
	if err != nil {
		t.Fatalf("SetHikConnectAccessToken: %v", err)
	}

	token, found, err := store.GetHikConnectAccessToken()
	if err != nil {
		t.Fatalf("GetHikConnectAccessToken: %v", err)
	}
	if !found {
		t.Fatal("expected access token to be found")
	}
	if token != "test-access-token-123" {
		t.Fatalf("expected token test-access-token-123, got %q", token)
	}

	// Delete
	err = store.DeleteHikConnectAccessToken()
	if err != nil {
		t.Fatalf("DeleteHikConnectAccessToken: %v", err)
	}
	_, found, err = store.GetHikConnectAccessToken()
	if err != nil {
		t.Fatalf("GetHikConnectAccessToken after delete: %v", err)
	}
	if found {
		t.Fatal("expected access token to not be found after delete")
	}
}

func TestHikConnectDeviceTokenCache(t *testing.T) {
	store := setupTestStore(t)

	// Set and get device token
	err := store.SetHikConnectDeviceToken("SERIAL123", "device-token-abc", 5*time.Minute)
	if err != nil {
		t.Fatalf("SetHikConnectDeviceToken: %v", err)
	}

	token, found, err := store.GetHikConnectDeviceToken("SERIAL123")
	if err != nil {
		t.Fatalf("GetHikConnectDeviceToken: %v", err)
	}
	if !found {
		t.Fatal("expected device token to be found")
	}
	if token != "device-token-abc" {
		t.Fatalf("expected token device-token-abc, got %q", token)
	}

	// Different serial returns not found
	_, found, err = store.GetHikConnectDeviceToken("OTHER_SERIAL")
	if err != nil {
		t.Fatalf("GetHikConnectDeviceToken other: %v", err)
	}
	if found {
		t.Fatal("expected other serial to not be found")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/siky/code/MistyPass/api && go test ./internal/redistore/ -run TestHikConnect -v`
Expected: FAIL — methods do not exist

- [ ] **Step 3: Implement cache methods**

Add to `api/internal/redistore/store.go`:

```go
func (s *Store) SetHikConnectAccessToken(token string, ttl time.Duration) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("hikconnect access token is empty")
	}
	if ttl <= 0 {
		return errors.New("hikconnect access token ttl must be positive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.client.Set(ctx, s.hikConnectAccessTokenKey(), token, ttl).Err()
}

func (s *Store) GetHikConnectAccessToken() (string, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	value, err := s.client.Get(ctx, s.hikConnectAccessTokenKey()).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (s *Store) DeleteHikConnectAccessToken() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.client.Del(ctx, s.hikConnectAccessTokenKey()).Err()
}

func (s *Store) SetHikConnectDeviceToken(serial, token string, ttl time.Duration) error {
	nextSerial := strings.TrimSpace(serial)
	if nextSerial == "" || strings.TrimSpace(token) == "" {
		return errors.New("hikconnect device serial/token are required")
	}
	if ttl <= 0 {
		return errors.New("hikconnect device token ttl must be positive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.client.Set(ctx, s.hikConnectDeviceTokenKey(nextSerial), token, ttl).Err()
}

func (s *Store) GetHikConnectDeviceToken(serial string) (string, bool, error) {
	nextSerial := strings.TrimSpace(serial)
	if nextSerial == "" {
		return "", false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	value, err := s.client.Get(ctx, s.hikConnectDeviceTokenKey(nextSerial)).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (s *Store) hikConnectAccessTokenKey() string {
	return s.key("hikconnect", "access_token")
}

func (s *Store) hikConnectDeviceTokenKey(serial string) string {
	return s.key("hikconnect", "device_token", serial)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/siky/code/MistyPass/api && go test ./internal/redistore/ -run TestHikConnect -v`
Expected: PASS

- [ ] **Step 5: Run all redistore tests**

Run: `cd /Users/siky/code/MistyPass/api && go test ./internal/redistore/ -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/redistore/store.go api/internal/redistore/store_test.go
git commit -m "feat(redistore): add Hik-Connect token cache methods"
```

---

### Task 4: ISC OpenAPI HTTP Client

**Files:**
- Create: `api/internal/modules/hikconnect/client.go`
- Create: `api/internal/modules/hikconnect/client_test.go`

- [ ] **Step 1: Write tests for the ISC client**

Create `api/internal/modules/hikconnect/client_test.go`:

```go
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
	expireMs := time.Now().Add(5 * time.Minute).UnixMilli()
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/siky/code/MistyPass/api && go test ./internal/modules/hikconnect/ -run TestClient -v`
Expected: FAIL — `NewClient` not defined

- [ ] **Step 3: Implement the ISC client**

Create `api/internal/modules/hikconnect/client.go`:

```go
package hikconnect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL   string
	appKey    string
	appSecret string
	http      *http.Client
}

func NewClient(baseURL, appKey, appSecret string) *Client {
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		appKey:    appKey,
		appSecret: appSecret,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) GetAccessToken(ctx context.Context) (string, time.Time, error) {
	form := url.Values{
		"appKey":    {c.appKey},
		"appSecret": {c.appSecret},
	}
	var resp TokenResponse
	if err := c.postForm(ctx, "/api/lapp/token/get", form, &resp); err != nil {
		return "", time.Time{}, fmt.Errorf("get access token: %w", err)
	}
	if resp.Code != "200" {
		return "", time.Time{}, fmt.Errorf("ISC token error: code=%s msg=%s", resp.Code, resp.Msg)
	}
	expiresAt := time.UnixMilli(resp.Data.ExpireTime).UTC()
	return resp.Data.AccessToken, expiresAt, nil
}

func (c *Client) AddDevice(ctx context.Context, accessToken, serial, validateCode string) error {
	form := url.Values{
		"accessToken":  {accessToken},
		"deviceSerial": {serial},
		"validateCode": {validateCode},
	}
	var resp DeviceResponse
	if err := c.postForm(ctx, "/api/lapp/device/add", form, &resp); err != nil {
		return fmt.Errorf("add device: %w", err)
	}
	if resp.Code != "200" {
		return fmt.Errorf("ISC add device error: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *Client) DeleteDevice(ctx context.Context, accessToken, serial string) error {
	form := url.Values{
		"accessToken":  {accessToken},
		"deviceSerial": {serial},
	}
	var resp DeviceResponse
	if err := c.postForm(ctx, "/api/lapp/device/delete", form, &resp); err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	if resp.Code != "200" {
		return fmt.Errorf("ISC delete device error: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *Client) GetDeviceAccessToken(ctx context.Context, accessToken, serial string) (string, time.Time, error) {
	form := url.Values{
		"accessToken":  {accessToken},
		"deviceSerial": {serial},
	}
	var resp DeviceAccessTokenResponse
	if err := c.postForm(ctx, "/api/lapp/token/getDeviceAccessToken", form, &resp); err != nil {
		return "", time.Time{}, fmt.Errorf("get device access token: %w", err)
	}
	if resp.Code != "200" {
		return "", time.Time{}, fmt.Errorf("ISC device token error: code=%s msg=%s", resp.Code, resp.Msg)
	}
	expiresAt := time.UnixMilli(resp.Data.ExpireTime).UTC()
	return resp.Data.AccessToken, expiresAt, nil
}

func (c *Client) ListRecordings(ctx context.Context, accessToken, serial string, start, end time.Time) ([]RecordingItem, error) {
	form := url.Values{
		"accessToken":  {accessToken},
		"deviceSerial": {serial},
		"startTime":    {start.Format("2006-01-02 15:04:05")},
		"endTime":      {end.Format("2006-01-02 15:04:05")},
	}
	var resp RecordingListResponse
	if err := c.postForm(ctx, "/api/lapp/video/by-time", form, &resp); err != nil {
		return nil, fmt.Errorf("list recordings: %w", err)
	}
	if resp.Code != "200" {
		return nil, fmt.Errorf("ISC recordings error: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return resp.Data, nil
}

func (c *Client) postForm(ctx context.Context, path string, form url.Values, dest any) error {
	reqURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ISC HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/siky/code/MistyPass/api && go test ./internal/modules/hikconnect/ -run TestClient -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/modules/hikconnect/client.go api/internal/modules/hikconnect/client_test.go
git commit -m "feat(hikconnect): implement ISC OpenAPI HTTP client with tests"
```

---

### Task 5: Hik-Connect Service Layer

**Files:**
- Create: `api/internal/modules/hikconnect/service.go`
- Create: `api/internal/modules/hikconnect/service_test.go`

- [ ] **Step 1: Write tests for the service**

Create `api/internal/modules/hikconnect/service_test.go`:

```go
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
	expiry := time.Now().Add(5 * time.Minute)
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/siky/code/MistyPass/api && go test ./internal/modules/hikconnect/ -run TestService -v`
Expected: FAIL — `NewService` not defined

- [ ] **Step 3: Implement the service**

Create `api/internal/modules/hikconnect/service.go`:

```go
package hikconnect

import (
	"context"
	"fmt"
	"time"
)

// ISCClient defines the interface for ISC OpenAPI calls (mockable).
type ISCClient interface {
	GetAccessToken(ctx context.Context) (string, time.Time, error)
	AddDevice(ctx context.Context, accessToken, serial, validateCode string) error
	DeleteDevice(ctx context.Context, accessToken, serial string) error
	GetDeviceAccessToken(ctx context.Context, accessToken, serial string) (string, time.Time, error)
	ListRecordings(ctx context.Context, accessToken, serial string, start, end time.Time) ([]RecordingItem, error)
}

// TokenCache defines the interface for caching ISC tokens.
type TokenCache interface {
	GetHikConnectAccessToken() (string, bool, error)
	SetHikConnectAccessToken(token string, ttl time.Duration) error
	DeleteHikConnectAccessToken() error
	GetHikConnectDeviceToken(serial string) (string, bool, error)
	SetHikConnectDeviceToken(serial, token string, ttl time.Duration) error
}

type Service struct {
	client          ISCClient
	cache           TokenCache
	accessTokenTTL  time.Duration
	deviceTokenTTL  time.Duration
}

func NewService(client ISCClient, cache TokenCache, accessTokenTTL time.Duration) *Service {
	if accessTokenTTL <= 0 {
		accessTokenTTL = 115 * time.Minute
	}
	return &Service{
		client:         client,
		cache:          cache,
		accessTokenTTL: accessTokenTTL,
		deviceTokenTTL: 5 * time.Minute,
	}
}

func (s *Service) BindDevice(ctx context.Context, serial, verifyCode string) error {
	accessToken, err := s.ensureAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("bind device: %w", err)
	}
	return s.client.AddDevice(ctx, accessToken, serial, verifyCode)
}

func (s *Service) UnbindDevice(ctx context.Context, serial string) error {
	accessToken, err := s.ensureAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("unbind device: %w", err)
	}
	return s.client.DeleteDevice(ctx, accessToken, serial)
}

func (s *Service) GetPlaybackToken(ctx context.Context, serial string, channel int) (PlaybackToken, error) {
	// Check device token cache first
	if cached, found, err := s.cache.GetHikConnectDeviceToken(serial); err == nil && found {
		return PlaybackToken{
			AccessToken:  cached,
			DeviceSerial: serial,
			Channel:      channel,
			ExpiresAt:    time.Now().Add(s.deviceTokenTTL),
		}, nil
	}

	accessToken, err := s.ensureAccessToken(ctx)
	if err != nil {
		return PlaybackToken{}, fmt.Errorf("get playback token: %w", err)
	}

	deviceToken, expiresAt, err := s.client.GetDeviceAccessToken(ctx, accessToken, serial)
	if err != nil {
		return PlaybackToken{}, fmt.Errorf("get playback token: %w", err)
	}

	// Cache device token
	ttl := time.Until(expiresAt)
	if ttl > s.deviceTokenTTL {
		ttl = s.deviceTokenTTL
	}
	if ttl > 0 {
		_ = s.cache.SetHikConnectDeviceToken(serial, deviceToken, ttl)
	}

	return PlaybackToken{
		AccessToken:  deviceToken,
		DeviceSerial: serial,
		Channel:      channel,
		ExpiresAt:    expiresAt,
	}, nil
}

func (s *Service) ListRecordings(ctx context.Context, serial string, start, end time.Time) ([]Recording, error) {
	accessToken, err := s.ensureAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recordings: %w", err)
	}

	items, err := s.client.ListRecordings(ctx, accessToken, serial, start, end)
	if err != nil {
		return nil, err
	}

	recordings := make([]Recording, 0, len(items))
	for _, item := range items {
		startTime, _ := time.Parse("2006-01-02 15:04:05", item.StartTime)
		endTime, _ := time.Parse("2006-01-02 15:04:05", item.EndTime)
		recordings = append(recordings, Recording{
			StartTime: startTime,
			EndTime:   endTime,
			Type:      recordingTypeName(item.Type),
		})
	}
	return recordings, nil
}

func (s *Service) ensureAccessToken(ctx context.Context) (string, error) {
	if cached, found, err := s.cache.GetHikConnectAccessToken(); err == nil && found {
		return cached, nil
	}

	token, expiresAt, err := s.client.GetAccessToken(ctx)
	if err != nil {
		return "", err
	}

	ttl := time.Until(expiresAt) - 5*time.Minute
	if ttl > s.accessTokenTTL {
		ttl = s.accessTokenTTL
	}
	if ttl > 0 {
		_ = s.cache.SetHikConnectAccessToken(token, ttl)
	}

	return token, nil
}

func recordingTypeName(code int) string {
	switch code {
	case 1:
		return "motion"
	case 2:
		return "continuous"
	case 3:
		return "alarm"
	default:
		return "unknown"
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/siky/code/MistyPass/api && go test ./internal/modules/hikconnect/ -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/modules/hikconnect/service.go api/internal/modules/hikconnect/service_test.go
git commit -m "feat(hikconnect): implement service layer with token caching"
```

---

### Task 6: Configuration

**Files:**
- Modify: `api/internal/config/config.go`

- [ ] **Step 1: Add HikISC config fields to Config struct**

In `api/internal/config/config.go`, add after the `CameraMaxSnapshotsPerEvent` field (around line 154):

```go
HikISCEnabled       bool
HikISCHost          string
HikISCAppKey        string
HikISCAppSecret     string
HikISCTokenCacheTTL time.Duration
```

- [ ] **Step 2: Add env parsing in LoadFromEnv**

Find where `CameraEnabled` is parsed (around line 1040) and add after the camera config block:

```go
cfg.HikISCEnabled = parseBoolOrFallback(envString("HIK_ISC_ENABLED"), false)
cfg.HikISCHost = envStringOrFallback("HIK_ISC_HOST", "https://open.hikconnect.com")
cfg.HikISCAppKey = envString("HIK_ISC_APP_KEY")
cfg.HikISCAppSecret = envString("HIK_ISC_APP_SECRET")
cfg.HikISCTokenCacheTTL = parseDurationOrFallback(envString("HIK_ISC_TOKEN_CACHE_TTL"), 115*time.Minute)
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/siky/code/MistyPass/api && go build ./...`
Expected: no errors

- [ ] **Step 4: Run config tests**

Run: `cd /Users/siky/code/MistyPass/api && go test ./internal/config/ -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/config/config.go
git commit -m "feat(config): add Hik-Connect ISC configuration fields"
```

---

### Task 7: API Routes — Cloud Token and Cloud Recordings

**Files:**
- Modify: `api/internal/http/routes_app_redesign.go`

- [ ] **Step 1: Add cloud-token endpoint handler**

Add to `api/internal/http/routes_app_redesign.go` after the `appCameraSnapshot` function:

```go
func (s *server) appCameraCloudToken(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	cameraID := chi.URLParam(r, "cameraID")
	if strings.TrimSpace(cameraID) == "" {
		writeError(w, http.StatusBadRequest, "camera ID is required")
		return
	}

	tenantID := user.TenantID
	cam, err := s.cameraSvc.Get(tenantID, cameraID)
	if err != nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	if cam.CloudProvider == "" || cam.CloudSerial == "" {
		writeError(w, http.StatusBadRequest, "camera has no cloud provider configured")
		return
	}

	if !cam.CloudVerified {
		writeError(w, http.StatusConflict, "camera cloud binding not verified")
		return
	}

	// Check access: admin or resident with door access
	if user.Role == "resident" {
		accessibleDoorIDs := s.getUserAccessibleDoorIDs(tenantID, user.ID)
		if cam.DoorID == "" || !accessibleDoorIDs[cam.DoorID] {
			writeError(w, http.StatusForbidden, "no access to this camera")
			return
		}
	}

	if s.hikConnectSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "cloud video service not configured")
		return
	}

	channel := cam.CloudChannels
	if channel <= 0 {
		channel = 1
	}

	token, err := s.hikConnectSvc.GetPlaybackToken(r.Context(), cam.CloudSerial, channel)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  token.AccessToken,
		"device_serial": token.DeviceSerial,
		"channel":       token.Channel,
		"expires_at":    token.ExpiresAt.Format(time.RFC3339),
	})
}
```

- [ ] **Step 2: Add cloud-recordings endpoint handler**

Add after `appCameraCloudToken`:

```go
func (s *server) appCameraCloudRecordings(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	cameraID := chi.URLParam(r, "cameraID")
	if strings.TrimSpace(cameraID) == "" {
		writeError(w, http.StatusBadRequest, "camera ID is required")
		return
	}

	tenantID := user.TenantID
	cam, err := s.cameraSvc.Get(tenantID, cameraID)
	if err != nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	if cam.CloudProvider == "" || cam.CloudSerial == "" {
		writeError(w, http.StatusBadRequest, "camera has no cloud provider configured")
		return
	}

	// Check access
	if user.Role == "resident" {
		accessibleDoorIDs := s.getUserAccessibleDoorIDs(tenantID, user.ID)
		if cam.DoorID == "" || !accessibleDoorIDs[cam.DoorID] {
			writeError(w, http.StatusForbidden, "no access to this camera")
			return
		}
	}

	if s.hikConnectSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "cloud video service not configured")
		return
	}

	// Parse date query param (default: today)
	dateStr := r.URL.Query().Get("date")
	var start, end time.Time
	if dateStr != "" {
		parsed, parseErr := time.Parse("2006-01-02", dateStr)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid date format, expected YYYY-MM-DD")
			return
		}
		start = parsed
		end = parsed.Add(24*time.Hour - time.Second)
	} else {
		now := time.Now().UTC()
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		end = start.Add(24*time.Hour - time.Second)
	}

	recordings, err := s.hikConnectSvc.ListRecordings(r.Context(), cam.CloudSerial, start, end)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}

	items := make([]map[string]any, 0, len(recordings))
	for _, rec := range recordings {
		items = append(items, map[string]any{
			"start_time": rec.StartTime.Format(time.RFC3339),
			"end_time":   rec.EndTime.Format(time.RFC3339),
			"type":       rec.Type,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"camera_id":  cameraID,
		"date":       start.Format("2006-01-02"),
		"recordings": items,
	})
}
```

- [ ] **Step 3: Add admin cloud-bind endpoint**

Add after `appCameraCloudRecordings`:

```go
func (s *server) adminCameraCloudBind(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	if user.Role != "tenant_admin" && user.Role != "admin" {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	cameraID := chi.URLParam(r, "cameraID")
	if strings.TrimSpace(cameraID) == "" {
		writeError(w, http.StatusBadRequest, "camera ID is required")
		return
	}

	tenantID := user.TenantID
	cam, err := s.cameraSvc.Get(tenantID, cameraID)
	if err != nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	if s.hikConnectSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "cloud video service not configured")
		return
	}

	var body struct {
		DeviceSerial string `json:"device_serial"`
		ValidateCode string `json:"validate_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(body.DeviceSerial) == "" || strings.TrimSpace(body.ValidateCode) == "" {
		writeError(w, http.StatusBadRequest, "device_serial and validate_code are required")
		return
	}

	if err := s.hikConnectSvc.BindDevice(r.Context(), body.DeviceSerial, body.ValidateCode); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Update camera record with cloud binding info
	cloudProvider := "hikconnect"
	cloudSerial := body.DeviceSerial
	cloudVerified := true
	cloudChannels := 1
	_, updateErr := s.cameraSvc.Update(tenantID, cameraID, camera.CameraUpdateRequest{
		CloudProvider: &cloudProvider,
		CloudSerial:   &cloudSerial,
		CloudVerified: &cloudVerified,
		CloudChannels: &cloudChannels,
	})
	if updateErr != nil {
		writeInternalError(w, r, updateErr)
		return
	}

	_ = cam
	writeJSON(w, http.StatusOK, map[string]any{
		"camera_id":      cameraID,
		"device_serial":  body.DeviceSerial,
		"cloud_provider": "hikconnect",
		"verified":       true,
	})
}

func (s *server) adminCameraCloudUnbind(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	if user.Role != "tenant_admin" && user.Role != "admin" {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	cameraID := chi.URLParam(r, "cameraID")
	if strings.TrimSpace(cameraID) == "" {
		writeError(w, http.StatusBadRequest, "camera ID is required")
		return
	}

	tenantID := user.TenantID
	cam, err := s.cameraSvc.Get(tenantID, cameraID)
	if err != nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	if cam.CloudSerial == "" {
		writeError(w, http.StatusBadRequest, "camera has no cloud binding")
		return
	}

	if s.hikConnectSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "cloud video service not configured")
		return
	}

	if err := s.hikConnectSvc.UnbindDevice(r.Context(), cam.CloudSerial); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Clear cloud fields
	emptyStr := ""
	falseBool := false
	zeroInt := 0
	_, updateErr := s.cameraSvc.Update(tenantID, cameraID, camera.CameraUpdateRequest{
		CloudProvider: &emptyStr,
		CloudSerial:   &emptyStr,
		CloudVerified: &falseBool,
		CloudChannels: &zeroInt,
	})
	if updateErr != nil {
		writeInternalError(w, r, updateErr)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"camera_id": cameraID,
		"unbound":   true,
	})
}
```

- [ ] **Step 4: Verify it compiles (will fail until router wiring in Task 8)**

Run: `cd /Users/siky/code/MistyPass/api && go vet ./internal/http/...`
Expected: may have unused import or missing field errors — that's expected, will fix in Task 8

- [ ] **Step 5: Commit**

```bash
git add api/internal/http/routes_app_redesign.go
git commit -m "feat(api): add cloud-token, cloud-recordings, cloud-bind endpoints"
```

---

### Task 8: Router Wiring

**Files:**
- Modify: `api/internal/http/router.go`

- [ ] **Step 1: Add hikConnectSvc field to server struct**

In `api/internal/http/router.go`, add to the server struct (near `cameraSvc`):

```go
hikConnectSvc *hikconnect.Service
```

Add the import:

```go
"github.com/mistypass/cloud/api/internal/modules/hikconnect"
```

- [ ] **Step 2: Initialize hikconnect service in router constructor**

In the router constructor function (where `cameraSvc` is initialized), add after camera setup:

```go
var hikConnectSvc *hikconnect.Service
if cfg.HikISCEnabled && cfg.HikISCAppKey != "" && cfg.HikISCAppSecret != "" {
	iscClient := hikconnect.NewClient(cfg.HikISCHost, cfg.HikISCAppKey, cfg.HikISCAppSecret)
	hikConnectSvc = hikconnect.NewService(iscClient, volatileStore, cfg.HikISCTokenCacheTTL)
}
```

And wire it into the server struct:

```go
hikConnectSvc: hikConnectSvc,
```

- [ ] **Step 3: Register routes**

Find where camera app routes are registered (search for `appCameraVideoLink`) and add the cloud routes nearby:

```go
r.Get("/app/cameras/{cameraID}/cloud-token", s.appCameraCloudToken)
r.Get("/app/cameras/{cameraID}/cloud-recordings", s.appCameraCloudRecordings)
r.Post("/admin/cameras/{cameraID}/cloud-bind", s.adminCameraCloudBind)
r.Delete("/admin/cameras/{cameraID}/cloud-bind", s.adminCameraCloudUnbind)
```

- [ ] **Step 4: Verify the full project compiles**

Run: `cd /Users/siky/code/MistyPass/api && go build ./...`
Expected: no errors

- [ ] **Step 5: Run all tests**

Run: `cd /Users/siky/code/MistyPass/api && go test ./... 2>&1 | tail -30`
Expected: all PASS (some packages may skip if they require external deps)

- [ ] **Step 6: Commit**

```bash
git add api/internal/http/router.go
git commit -m "feat(router): wire hikconnect service and register cloud routes"
```

---

## Notes

**Not included in this plan (separate work):**
- iOS client plan (ISC OpenSDK integration) — requires ISV account + SDK binary
- ISV account registration at partner.hikvision.com — prerequisite, manual step
- Admin UI for cloud binding — future frontend task
- `SyncDeviceStatus` background worker — can be added after core flow works

**Testing the full flow manually (after all tasks):**
1. Set `HIK_ISC_ENABLED=true`, `HIK_ISC_APP_KEY`, `HIK_ISC_APP_SECRET` in `.env`
2. Register a camera via existing POST `/api/v1/cameras`
3. Bind it: POST `/admin/cameras/{id}/cloud-bind` with `{"device_serial": "...", "validate_code": "..."}`
4. Get token: GET `/app/cameras/{id}/cloud-token`
5. List recordings: GET `/app/cameras/{id}/cloud-recordings?date=2026-05-17`
