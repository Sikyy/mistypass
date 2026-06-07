# OTA 固件制品仓库 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把签名固件收进平台:上传一次 → 平台存储 + 版本目录 + 分发;建 OTA 任务时引用某版本,服务端自动填 sha256/signature/url(对标 Kisi OTA 子项 #2)。

**Architecture:** 固件元数据存在 gateway service 的新 `firmwares` 切片(随 stateSnapshot 持久化);二进制复用 upload 的本地 FS blob 存储(`storageDir/{id[:2]}/{id}`),**分发直接复用现有 `/api/v1/uploads/{id}` + `signDownload`**(零新增加密/serve 代码);`CreateOTATask` 新增 `firmware_id` 分支,从仓库取 sha256/signature;config/pull 为 registry 任务动态生成签名下载 URL。

**Tech Stack:** Go、chi、`crypto/sha256`、`crypto/hmac`(经 `signDownload` 复用)、本地 FS。

设计依据:[2026-06-07-ota-firmware-registry-design.md](../specs/2026-06-07-ota-firmware-registry-design.md)

**测试约定:** `go` 命令在 `api/` 下;`gateway.NewService()` 预置 `gw_demo_001`/`tenant_demo_jakarta`。

**复用要点(已核实):** `signDownload(key, id, exp)` = `hex(HMAC_SHA256(key, "dl:"+id+":"+expUnix))`(`routes_uploads.go:256`);`GET /uploads/{id}`(`downloadFile`,`router.go:601`,**公开 + HMAC**)按 `storageDir/{filepath.Base(id)[:2]}/{filepath.Base(id)}` serve;cfg 字段 `UploadStorageDir`/`UploadSigningKey`。固件 id 形如 `fw_<32hex>` → `[:2]="fw"` → 与 downloadFile 路径逻辑一致。

---

## Task 1: 固件仓库 store + 持久化

**Files:**
- Create: `api/internal/modules/gateway/firmware.go`
- Test: `api/internal/modules/gateway/firmware_test.go`
- Modify: `api/internal/modules/gateway/service.go`(`Service`/`stateSnapshot` 加 `firmwares`/`Firmwares`;persist/restore 接线)

- [ ] **Step 1: 写失败测试**

Create `api/internal/modules/gateway/firmware_test.go`:

```go
package gateway

import "testing"

func TestCreateAndGetFirmware(t *testing.T) {
	svc := NewService()
	fw, err := svc.CreateFirmware(CreateFirmwareInput{
		TenantID:  "tenant_demo_jakarta",
		Version:   "1.4.0",
		Channel:   "stable",
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Signature: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		SizeBytes: 123,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if fw.ID == "" || fw.Version != "1.4.0" || fw.Channel != "stable" {
		t.Fatalf("unexpected fw: %+v", fw)
	}
	got, err := svc.GetFirmware("tenant_demo_jakarta", fw.ID)
	if err != nil || got.ID != fw.ID {
		t.Fatalf("get: %v %+v", err, got)
	}
	if _, err := svc.GetFirmware("tenant_other", fw.ID); err != ErrGatewayFirmwareNotFound {
		t.Fatalf("cross-tenant get should fail, got %v", err)
	}
	byID, err := svc.GetFirmwareByID(fw.ID)
	if err != nil || byID.ID != fw.ID {
		t.Fatalf("get-by-id: %v %+v", err, byID)
	}
}

func TestCreateFirmwareValidation(t *testing.T) {
	svc := NewService()
	if _, err := svc.CreateFirmware(CreateFirmwareInput{TenantID: "t", Version: ""}); err != ErrGatewayFirmwareVersionRequired {
		t.Fatalf("want version-required, got %v", err)
	}
	if _, err := svc.CreateFirmware(CreateFirmwareInput{TenantID: "t", Version: "1.0.0", SHA256: "short"}); err != ErrGatewayFirmwareSHA256Invalid {
		t.Fatalf("want sha-invalid, got %v", err)
	}
	if _, err := svc.CreateFirmware(CreateFirmwareInput{TenantID: "t", Version: "1.0.0", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Signature: "bad"}); err != ErrGatewayFirmwareSignatureInvalid {
		t.Fatalf("want sig-invalid, got %v", err)
	}
}

func TestListFirmwareByChannel(t *testing.T) {
	svc := NewService()
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sig := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	_, _ = svc.CreateFirmware(CreateFirmwareInput{TenantID: "t1", Version: "1.0.0", Channel: "stable", SHA256: sha, Signature: sig})
	_, _ = svc.CreateFirmware(CreateFirmwareInput{TenantID: "t1", Version: "1.1.0", Channel: "beta", SHA256: sha, Signature: sig})
	if got := svc.ListFirmware("t1", ""); len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got := svc.ListFirmware("t1", "beta"); len(got) != 1 || got[0].Channel != "beta" {
		t.Fatalf("want 1 beta, got %+v", got)
	}
	if got := svc.ListFirmware("t2", ""); len(got) != 0 {
		t.Fatalf("tenant isolation broken, got %d", len(got))
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/modules/gateway/ -run 'Firmware' -v`
Expected: FAIL(`undefined: CreateFirmwareInput`/`CreateFirmware` 等)。

- [ ] **Step 3: 写 store(firmware.go)**

Create `api/internal/modules/gateway/firmware.go`:

```go
package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"
)

var ErrGatewayFirmwareNotFound = errors.New("gateway firmware not found")
var ErrGatewayFirmwareVersionRequired = errors.New("gateway firmware version is required")
var ErrGatewayFirmwareSHA256Invalid = errors.New("gateway firmware sha256 is invalid")
var ErrGatewayFirmwareSignatureInvalid = errors.New("gateway firmware signature is invalid (expected hex-encoded Ed25519 signature)")

// GatewayFirmware is one stored, signed firmware artifact in the registry.
type GatewayFirmware struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	Version    string    `json:"version"`
	Channel    string    `json:"channel,omitempty"`
	SHA256     string    `json:"sha256"`
	Signature  string    `json:"signature"`
	SizeBytes  int64     `json:"size_bytes"`
	UploadedBy string    `json:"uploaded_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// CreateFirmwareInput carries validated metadata for a firmware record. The
// HTTP layer verifies sha256 against the uploaded bytes and stores the blob.
type CreateFirmwareInput struct {
	TenantID   string
	Version    string
	Channel    string
	SHA256     string
	Signature  string
	SizeBytes  int64
	UploadedBy string
}

func firmwareRecordID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "fw_" + hex.EncodeToString(b), nil
}

// CreateFirmware validates metadata, mints an id, prepends the record, persists.
func (s *Service) CreateFirmware(in CreateFirmwareInput) (GatewayFirmware, error) {
	version := strings.TrimSpace(in.Version)
	if version == "" {
		return GatewayFirmware{}, ErrGatewayFirmwareVersionRequired
	}
	sha := strings.ToLower(strings.TrimSpace(in.SHA256))
	if !isValidSHA256Hex(sha) {
		return GatewayFirmware{}, ErrGatewayFirmwareSHA256Invalid
	}
	sig := strings.ToLower(strings.TrimSpace(in.Signature))
	if !isValidEd25519SignatureHex(sig) {
		return GatewayFirmware{}, ErrGatewayFirmwareSignatureInvalid
	}
	id, err := firmwareRecordID()
	if err != nil {
		return GatewayFirmware{}, err
	}
	fw := GatewayFirmware{
		ID:         id,
		TenantID:   strings.TrimSpace(in.TenantID),
		Version:    version,
		Channel:    strings.TrimSpace(in.Channel),
		SHA256:     sha,
		Signature:  sig,
		SizeBytes:  in.SizeBytes,
		UploadedBy: strings.TrimSpace(in.UploadedBy),
		CreatedAt:  time.Now().UTC(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.firmwares = append([]GatewayFirmware{fw}, s.firmwares...)
	if err := s.persistLocked(); err != nil {
		return GatewayFirmware{}, err
	}
	return fw, nil
}

// ListFirmware returns a tenant's firmware (optionally filtered by channel), newest first.
func (s *Service) ListFirmware(tenantID, channel string) []GatewayFirmware {
	ft := strings.TrimSpace(tenantID)
	fc := strings.TrimSpace(channel)
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]GatewayFirmware, 0, len(s.firmwares))
	for i := range s.firmwares {
		if ft != "" && s.firmwares[i].TenantID != ft {
			continue
		}
		if fc != "" && s.firmwares[i].Channel != fc {
			continue
		}
		items = append(items, s.firmwares[i])
	}
	sort.Slice(items, func(a, b int) bool { return items[a].CreatedAt.After(items[b].CreatedAt) })
	return items
}

// GetFirmware returns a tenant-scoped firmware record (for task creation / listing).
func (s *Service) GetFirmware(tenantID, id string) (GatewayFirmware, error) {
	ft := strings.TrimSpace(tenantID)
	nid := strings.TrimSpace(id)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if fw, ok := s.findFirmwareLocked(nid, ft); ok {
		return fw, nil
	}
	return GatewayFirmware{}, ErrGatewayFirmwareNotFound
}

// GetFirmwareByID returns a firmware record without a tenant filter — only the
// HMAC-signed download path uses this (isolation: task creation checks tenant,
// the signed URL is server-minted, the id is unguessable).
func (s *Service) GetFirmwareByID(id string) (GatewayFirmware, error) {
	nid := strings.TrimSpace(id)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if fw, ok := s.findFirmwareLocked(nid, ""); ok {
		return fw, nil
	}
	return GatewayFirmware{}, ErrGatewayFirmwareNotFound
}

// findFirmwareLocked returns the firmware by id (tenant filter if non-empty).
// Caller must hold s.mu.
func (s *Service) findFirmwareLocked(id, tenantID string) (GatewayFirmware, bool) {
	for i := range s.firmwares {
		if s.firmwares[i].ID == id && (tenantID == "" || s.firmwares[i].TenantID == tenantID) {
			return s.firmwares[i], true
		}
	}
	return GatewayFirmware{}, false
}

func cloneGatewayFirmwares(in []GatewayFirmware) []GatewayFirmware {
	if in == nil {
		return nil
	}
	out := make([]GatewayFirmware, len(in))
	copy(out, in)
	return out
}
```

- [ ] **Step 4: 接线持久化(service.go)**

在 `api/internal/modules/gateway/service.go`:

(a) `stateSnapshot` struct(`OTATasks` 字段那行之后)加:
```go
	Firmwares              []GatewayFirmware              `json:"firmwares,omitempty"`
```

(b) `Service` struct(`otaTasks` 字段那行之后)加:
```go
	firmwares              []GatewayFirmware
```

(c) `restoreFromStateStore` 中(`s.otaTasks = cloneGatewayOTATasks(snapshot.OTATasks)` 那行之后)加:
```go
	s.firmwares = cloneGatewayFirmwares(snapshot.Firmwares)
```

(d) `persistLocked` 构建 snapshot 处(`OTATasks: cloneGatewayOTATasks(snapshot.OTATasks)` 或 `s.otaTasks` 那行之后,同一字面量内)加:
```go
		Firmwares:              cloneGatewayFirmwares(s.firmwares),
```
(注:`persistLocked` 在 service.go ~L2030;它构建一个 `stateSnapshot{...}` 字面量,在其中 `OTATasks:` 之后补 `Firmwares:` 一行。)

- [ ] **Step 5: 运行测试确认通过 + 回归**

Run: `cd api && go test ./internal/modules/gateway/ -run 'Firmware' -v && go test ./internal/modules/gateway/ && go vet ./internal/modules/gateway/ && gofmt -l internal/modules/gateway/firmware.go internal/modules/gateway/service.go`
Expected: 新测试 PASS;全包 ok;vet/gofmt 无输出。

- [ ] **Step 6: 提交**

```bash
git add api/internal/modules/gateway/firmware.go api/internal/modules/gateway/firmware_test.go api/internal/modules/gateway/service.go
git commit -m "feat: add gateway firmware registry store"
```

---

## Task 2: 固件上传端点(multipart + sha256 校验 + 存储)

**Files:**
- Create: `api/internal/http/routes_gateway_firmware.go`
- Test: `api/internal/http/routes_gateway_firmware_test.go`

- [ ] **Step 1: 写失败测试**

Create `api/internal/http/routes_gateway_firmware_test.go`:

```go
package httpx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

func firmwareUploadRequest(t *testing.T, version, channel, sha, sig string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("version", version)
	_ = mw.WriteField("channel", channel)
	_ = mw.WriteField("sha256", sha)
	_ = mw.WriteField("signature", sig)
	fw, _ := mw.CreateFormFile("file", "gateway-agent")
	_, _ = fw.Write(data)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateways/firmware?tenant_id=tenant_demo_jakarta", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return withGatewayMQTTUser(req, auth.User{ID: "u1", Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})
}

func TestUploadGatewayFirmware(t *testing.T) {
	dir := t.TempDir()
	s := &server{
		gatewaySvc: gateway.NewService(),
		cfg:        config.Config{UploadStorageDir: dir, UploadSigningKey: "k"},
	}
	data := []byte("fake firmware bytes")
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	sig := strings.Repeat("b", 128)

	rec := httptest.NewRecorder()
	s.uploadGatewayFirmware(rec, firmwareUploadRequest(t, "1.4.0", "stable", sha, sig, data))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var fw gateway.GatewayFirmware
	_ = json.Unmarshal(rec.Body.Bytes(), &fw)
	if fw.ID == "" || fw.Version != "1.4.0" || fw.SHA256 != sha {
		t.Fatalf("unexpected fw: %+v", fw)
	}

	// sha mismatch → 400
	badRec := httptest.NewRecorder()
	s.uploadGatewayFirmware(badRec, firmwareUploadRequest(t, "1.4.1", "", strings.Repeat("a", 64), sig, data))
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("sha mismatch expected 400, got %d", badRec.Code)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/http/ -run TestUploadGatewayFirmware -v`
Expected: FAIL(`s.uploadGatewayFirmware undefined`)。

- [ ] **Step 3: 写 handler**

Create `api/internal/http/routes_gateway_firmware.go`:

```go
package httpx

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

const firmwareMaxUploadBytes = 256 << 20 // 256 MiB ceiling

// uploadGatewayFirmware stores a signed firmware artifact + registry record.
// The binary lands in the shared signed-blob store, served later via /uploads/{id}.
func (s *server) uploadGatewayFirmware(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	storageDir := strings.TrimSpace(s.cfg.UploadStorageDir)
	signingKey := strings.TrimSpace(s.cfg.UploadSigningKey)
	if storageDir == "" || signingKey == "" {
		writeError(w, http.StatusServiceUnavailable, "file uploads are not configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, firmwareMaxUploadBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	declaredSHA := strings.ToLower(strings.TrimSpace(r.FormValue("sha256")))

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing firmware file")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read firmware")
		return
	}
	sum := sha256.Sum256(data)
	gotSHA := hex.EncodeToString(sum[:])
	if declaredSHA == "" || gotSHA != declaredSHA {
		writeError(w, http.StatusBadRequest, "sha256 does not match uploaded bytes")
		return
	}

	fw, err := s.gatewaySvc.CreateFirmware(gateway.CreateFirmwareInput{
		TenantID:   tenantID,
		Version:    strings.TrimSpace(r.FormValue("version")),
		Channel:    strings.TrimSpace(r.FormValue("channel")),
		SHA256:     gotSHA,
		Signature:  strings.ToLower(strings.TrimSpace(r.FormValue("signature"))),
		SizeBytes:  int64(len(data)),
		UploadedBy: requestActor(r),
	})
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayFirmwareVersionRequired),
			errors.Is(err, gateway.ErrGatewayFirmwareSHA256Invalid),
			errors.Is(err, gateway.ErrGatewayFirmwareSignatureInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	// Store the blob at storageDir/{id[:2]}/{id} — the same layout downloadFile serves.
	cleanID := filepath.Base(fw.ID)
	dir := filepath.Join(storageDir, cleanID[:2])
	if err := os.MkdirAll(dir, 0o750); err != nil {
		writeError(w, http.StatusInternalServerError, "storage error")
		return
	}
	if err := os.WriteFile(filepath.Join(dir, cleanID), data, 0o640); err != nil { // #nosec G304 -- cleanID is the minted firmware id (fw_<hex>), not user-controlled
		writeError(w, http.StatusInternalServerError, "storage error")
		return
	}

	writeJSON(w, http.StatusCreated, fw)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd api && go test ./internal/http/ -run TestUploadGatewayFirmware -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add api/internal/http/routes_gateway_firmware.go api/internal/http/routes_gateway_firmware_test.go
git commit -m "feat: add gateway firmware upload endpoint"
```

---

## Task 3: 列表端点 + 路由注册

**Files:**
- Modify: `api/internal/http/routes_gateway_firmware.go`(加 list handler)
- Modify: `api/internal/http/router.go`(注册上传 + 列表路由)
- Test: `api/internal/http/routes_gateway_firmware_test.go`(list 测试)

- [ ] **Step 1: 写失败测试**

在 `api/internal/http/routes_gateway_firmware_test.go` 末尾追加:

```go
func TestListGatewayFirmware(t *testing.T) {
	svc := gateway.NewService()
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sig := strings.Repeat("b", 128)
	_, _ = svc.CreateFirmware(gateway.CreateFirmwareInput{TenantID: "tenant_demo_jakarta", Version: "1.4.0", Channel: "stable", SHA256: sha, Signature: sig})
	s := &server{gatewaySvc: svc, cfg: config.Config{UploadStorageDir: t.TempDir(), UploadSigningKey: "k"}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/gateways/firmware?tenant_id=tenant_demo_jakarta", nil)
	req = withGatewayMQTTUser(req, auth.User{ID: "u1", Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})
	rec := httptest.NewRecorder()
	s.listGatewayFirmware(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Items []gateway.GatewayFirmware `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if len(payload.Items) != 1 || payload.Items[0].Version != "1.4.0" {
		t.Fatalf("unexpected items: %+v", payload.Items)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/http/ -run TestListGatewayFirmware -v`
Expected: FAIL(`s.listGatewayFirmware undefined`)。

- [ ] **Step 3: 写 list handler**

在 `api/internal/http/routes_gateway_firmware.go` 末尾追加:

```go
func (s *server) listGatewayFirmware(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	items := s.gatewaySvc.ListFirmware(tenantID, strings.TrimSpace(r.URL.Query().Get("channel")))
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
```

- [ ] **Step 4: 注册路由(router.go)**

在 `api/internal/http/router.go` 中,紧接 `Get("/gateways/firmware-summary", s.gatewayFirmwareSummary)` 那行之后加入:

```go
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/gateways/firmware", s.uploadGatewayFirmware)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/gateways/firmware", s.listGatewayFirmware)
```

- [ ] **Step 5: 运行测试 + 全 http 包**

Run: `cd api && go test ./internal/http/ -run 'GatewayFirmware' -v && go build ./... && go test ./internal/http/ 2>&1 | tail -3`
Expected: PASS;build OK;整包 ok。

- [ ] **Step 6: 提交**

```bash
git add api/internal/http/routes_gateway_firmware.go api/internal/http/router.go api/internal/http/routes_gateway_firmware_test.go
git commit -m "feat: add gateway firmware list endpoint + routes"
```

---

## Task 4: CreateOTATask 引用仓库(firmware_id)

**Files:**
- Modify: `api/internal/modules/gateway/service.go`(`GatewayOTATask` 加 `FirmwareID`;`CreateOTATask` 加 `firmwareID` 分支)
- Modify: `api/internal/http/routes_gateway_management.go`(`createGatewayOTATask` 请求体加 `firmware_id` + 传参)
- Test: `api/internal/modules/gateway/service_test.go`

- [ ] **Step 1: 写失败测试**

在 `api/internal/modules/gateway/service_test.go` 末尾追加:

```go
func TestCreateOTATaskFromFirmwareRegistry(t *testing.T) {
	svc := NewService()
	fw, err := svc.CreateFirmware(CreateFirmwareInput{
		TenantID:  "tenant_demo_jakarta",
		Version:   "1.4.0",
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Signature: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	})
	if err != nil {
		t.Fatal(err)
	}
	// firmware_id provided → version/sha/sig sourced from the registry; url empty ok.
	task, err := svc.CreateOTATask("tenant_demo_jakarta", "gw_demo_001", "", "", "", "", fw.ID, "admin@example.com")
	if err != nil {
		t.Fatalf("create from registry: %v", err)
	}
	if task.FirmwareID != fw.ID || task.FirmwareSHA256 != fw.SHA256 || task.FirmwareSignature != fw.Signature || task.FirmwareVersion != "1.4.0" {
		t.Fatalf("task not sourced from registry: %+v", task)
	}
	// unknown firmware_id → error
	if _, err := svc.CreateOTATask("tenant_demo_jakarta", "gw_demo_001", "", "", "", "", "fw_nope", "a"); err != ErrGatewayFirmwareNotFound {
		t.Fatalf("want firmware-not-found, got %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/modules/gateway/ -run TestCreateOTATaskFromFirmwareRegistry -v`
Expected: FAIL(`CreateOTATask` 参数个数不符 / `task.FirmwareID undefined`)。

- [ ] **Step 3: `GatewayOTATask` 加 `FirmwareID`**

在 `api/internal/modules/gateway/service.go` 的 `GatewayOTATask` struct 中(`FirmwareSignature` 字段那行之后)加:
```go
	FirmwareID        string    `json:"firmware_id,omitempty"`
```

- [ ] **Step 4: 重构 `CreateOTATask` 加 `firmwareID` 分支**

把 `CreateOTATask` 的签名加一个 `firmwareID` 参数(放在 `firmwareSignature` 之后、`requestedBy` 之前):
```go
func (s *Service) CreateOTATask(
	tenantID,
	gatewayID,
	firmwareVersion,
	firmwareURL,
	firmwareSHA256,
	firmwareSignature,
	firmwareID,
	requestedBy string,
) (GatewayOTATask, error) {
```
把函数体改为:**先在锁内定位网关拿到 `taskTenantID`,再(若给了 firmwareID)从仓库取 sha/sig/version,最后做必填校验**。完整替换函数体为:
```go
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return GatewayOTATask{}, ErrGatewayIDRequired
	}
	nextFirmwareID := strings.TrimSpace(firmwareID)
	nextVersion := strings.TrimSpace(firmwareVersion)
	nextURL := strings.TrimSpace(firmwareURL)
	nextSHA256 := strings.ToLower(strings.TrimSpace(firmwareSHA256))
	nextSignature := strings.ToLower(strings.TrimSpace(firmwareSignature))
	filterTenantID := strings.TrimSpace(tenantID)
	now := time.Now().UTC()
	nextRequestedBy := strings.TrimSpace(requestedBy)

	s.mu.Lock()
	defer s.mu.Unlock()

	taskTenantID := ""
	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return GatewayOTATask{}, ErrGatewayNotFound
		}
		taskTenantID = s.gateways[i].TenantID
		s.gateways[i].LastSeenAt = now
		break
	}
	if taskTenantID == "" {
		return GatewayOTATask{}, ErrGatewayNotFound
	}

	if nextFirmwareID != "" {
		fw, ok := s.findFirmwareLocked(nextFirmwareID, taskTenantID)
		if !ok {
			return GatewayOTATask{}, ErrGatewayFirmwareNotFound
		}
		nextSHA256 = fw.SHA256
		nextSignature = fw.Signature
		if nextVersion == "" {
			nextVersion = fw.Version
		}
		nextURL = "" // firmware_url filled dynamically at config/pull for registry tasks
	}

	if nextVersion == "" {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareVersionRequired
	}
	if nextFirmwareID == "" && nextURL == "" {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareURLRequired
	}
	if nextSHA256 == "" {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareSHA256Required
	}
	if !isValidSHA256Hex(nextSHA256) {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareSHA256Invalid
	}
	if nextSignature == "" {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareSignatureRequired
	}
	if !isValidEd25519SignatureHex(nextSignature) {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareSignatureInvalid
	}

	taskID, err := otaTaskID()
	if err != nil {
		return GatewayOTATask{}, err
	}
	task := GatewayOTATask{
		ID:                taskID,
		GatewayID:         gwID,
		TenantID:          taskTenantID,
		FirmwareVersion:   nextVersion,
		FirmwareURL:       nextURL,
		FirmwareSHA256:    nextSHA256,
		FirmwareSignature: nextSignature,
		FirmwareID:        nextFirmwareID,
		Status:            gatewayOTATaskStatusQueued,
		RequestedBy:       nextRequestedBy,
		UpdatedBy:         nextRequestedBy,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	s.otaTasks = append([]GatewayOTATask{task}, s.otaTasks...)
	if err := s.persistLocked(); err != nil {
		return GatewayOTATask{}, err
	}
	return task, nil
```
(此版合并了原先"定位网关"和"校验"两段;原 `CreateOTATask` 里单独的网关定位循环被上面这段取代,删掉重复部分。)

- [ ] **Step 5: HTTP handler 传 firmware_id**

在 `api/internal/http/routes_gateway_management.go` 的 `createGatewayOTATask`:请求体 struct 加 `FirmwareID string json:"firmware_id"`;调用 `s.gatewaySvc.CreateOTATask(...)` 处把 `request.FirmwareID` 作为新增参数传入(放在 signature 之后、`requestActor(r)` 之前);错误 switch 的 400 分支加 `errors.Is(err, gateway.ErrGatewayFirmwareNotFound)` → 改为 404 分支(与 `ErrGatewayNotFound` 并列):
```go
		case errors.Is(err, gateway.ErrGatewayNotFound),
			errors.Is(err, gateway.ErrGatewayFirmwareNotFound):
			writeError(w, http.StatusNotFound, err.Error())
```

- [ ] **Step 6: 运行测试 + 回归 + 既有 OTA 测试不破**

Run: `cd api && go test ./internal/modules/gateway/ -run 'OTATask|Firmware' -v && go build ./... && go test ./internal/http/ -run 'OTA' && go vet ./internal/modules/gateway/`
Expected: 全 PASS(注意:既有 `TestCreateOTATask*` 调用 `CreateOTATask` 处会因签名变更需补一个 `""` firmwareID 参数 —— 若编译报参数个数,给那些调用点加 `""`)。

- [ ] **Step 7: 修既有调用点(若 Step 6 编译失败)**

既有 `service_test.go`/`routes_gateway_ota_test.go`/`router_handlers_gateway.go`/`routes_gateway_management.go` 里所有 `CreateOTATask(...)` 调用需在 signature 后补一个空 firmwareID `""`。逐个修正后重跑 Step 6。

- [ ] **Step 8: 提交**

```bash
git add api/internal/modules/gateway/service.go api/internal/modules/gateway/service_test.go api/internal/http/routes_gateway_management.go
git commit -m "feat: OTA tasks can reference a registry firmware by id"
```

---

## Task 5: config/pull 为 registry 任务动态填签名 URL

**Files:**
- Modify: `api/internal/http/router_handlers_gateway.go`(pending 任务填 firmware_url)
- Test: `api/internal/http/routes_gateway_bootstrap_test.go`

- [ ] **Step 1: 写失败测试**

在 `api/internal/http/routes_gateway_bootstrap_test.go` 末尾追加:

```go
func TestConfigPullFillsRegistryFirmwareURL(t *testing.T) {
	svc := gateway.NewService()
	fw, _ := svc.CreateFirmware(gateway.CreateFirmwareInput{
		TenantID: "tenant_demo_jakarta", Version: "1.4.0",
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Signature: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	})
	if _, err := svc.CreateOTATask("tenant_demo_jakarta", "gw_demo_001", "", "", "", "", fw.ID, "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	s := &server{
		gatewaySvc:          svc,
		gatewayDeviceTokens: map[string]string{"gw_demo_001": "gw_test_token_001"},
		cfg:                 config.Config{UploadStorageDir: t.TempDir(), UploadSigningKey: "k"},
	}
	body, _ := json.Marshal(map[string]any{"gateway_id": "gw_demo_001", "tenant_id": "tenant_demo_jakarta"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/config/pull", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer gw_test_token_001")
	rec := httptest.NewRecorder()
	s.gatewayBootstrapConfigPull(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("config/pull expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		PendingOTATasks []struct {
			FirmwareURL string `json:"firmware_url"`
			FirmwareID  string `json:"firmware_id"`
		} `json:"pending_ota_tasks"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.PendingOTATasks) == 0 {
		t.Fatal("no pending tasks returned")
	}
	url := resp.PendingOTATasks[0].FirmwareURL
	if !strings.Contains(url, "/api/v1/uploads/"+fw.ID) || !strings.Contains(url, "sig=") || !strings.Contains(url, "expires=") {
		t.Fatalf("firmware_url not a signed registry URL: %q", url)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/http/ -run TestConfigPullFillsRegistryFirmwareURL -v`
Expected: FAIL(`firmware_url` 为空,不含签名 URL)。

- [ ] **Step 3: config/pull 填签名 URL**

在 `api/internal/http/router_handlers_gateway.go` 的 `gatewayBootstrapConfigPull` 中,定位现有构建 `pendingOTA` 的循环(`if task.Status == "queued" || task.Status == "dispatching"` 那段)。把 append 改为:对带 `FirmwareID` 的任务先填一个新鲜签名 URL:
```go
	var pendingOTA []gateway.GatewayOTATask
	if allOTA, otaErr := s.gatewaySvc.ListOTATasks(request.TenantID, request.GatewayID); otaErr == nil {
		signingKey := strings.TrimSpace(s.cfg.UploadSigningKey)
		base := requestBaseURL(r)
		for _, task := range allOTA {
			if task.Status != "queued" && task.Status != "dispatching" {
				continue
			}
			if task.FirmwareID != "" && signingKey != "" {
				exp := time.Now().UTC().Add(10 * time.Minute)
				sig := signDownload(signingKey, task.FirmwareID, exp)
				task.FirmwareURL = fmt.Sprintf("%s/api/v1/uploads/%s?sig=%s&expires=%d", base, task.FirmwareID, sig, exp.Unix())
			}
			pendingOTA = append(pendingOTA, task)
		}
	}
```
并在文件中新增一个 helper(若不存在)构造请求的绝对 base URL:
```go
// requestBaseURL returns scheme://host for r, honoring X-Forwarded-Proto.
func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		scheme = proto
	}
	return scheme + "://" + r.Host
}
```
确保 `router_handlers_gateway.go` 已 import `"fmt"`、`"strings"`、`"time"`(缺则补)。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd api && go test ./internal/http/ -run TestConfigPullFillsRegistryFirmwareURL -v`
Expected: PASS。

- [ ] **Step 5: 端到端冒烟:签名 URL 真能被 downloadFile 验过**

在 `routes_gateway_firmware_test.go` 末尾追加一个端到端测试(上传 → 建任务 → config/pull 拿 URL → 用该 URL 打 downloadFile → 200 + 字节):
```go
func TestFirmwareSignedURLServesViaDownloadFile(t *testing.T) {
	dir := t.TempDir()
	s := &server{
		gatewaySvc:          gateway.NewService(),
		gatewayDeviceTokens: map[string]string{"gw_demo_001": "gw_test_token_001"},
		cfg:                 config.Config{UploadStorageDir: dir, UploadSigningKey: "k"},
	}
	data := []byte("real firmware payload")
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	upRec := httptest.NewRecorder()
	s.uploadGatewayFirmware(upRec, firmwareUploadRequest(t, "1.4.0", "", sha, strings.Repeat("b", 128), data))
	if upRec.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", upRec.Code, upRec.Body.String())
	}
	var fw gateway.GatewayFirmware
	_ = json.Unmarshal(upRec.Body.Bytes(), &fw)
	if _, err := s.gatewaySvc.CreateOTATask("tenant_demo_jakarta", "gw_demo_001", "", "", "", "", fw.ID, "a"); err != nil {
		t.Fatal(err)
	}
	pullBody, _ := json.Marshal(map[string]any{"gateway_id": "gw_demo_001", "tenant_id": "tenant_demo_jakarta"})
	pullReq := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/config/pull", bytes.NewReader(pullBody))
	pullReq.Header.Set("Authorization", "Bearer gw_test_token_001")
	pullRec := httptest.NewRecorder()
	s.gatewayBootstrapConfigPull(pullRec, pullReq)
	var resp struct {
		PendingOTATasks []struct {
			FirmwareURL string `json:"firmware_url"`
		} `json:"pending_ota_tasks"`
	}
	_ = json.Unmarshal(pullRec.Body.Bytes(), &resp)
	if len(resp.PendingOTATasks) == 0 {
		t.Fatal("no pending tasks")
	}
	// Replay the signed URL against downloadFile via chi route param.
	u := resp.PendingOTATasks[0].FirmwareURL
	q := u[strings.Index(u, "?"):]
	dlReq := httptest.NewRequest(http.MethodGet, "/api/v1/uploads/"+fw.ID+q, nil)
	dlReq = withGatewayOTAURLParams(dlReq, "", "") // reuse chi route ctx helper; then set uploadID
	dlReq = withUploadIDParam(dlReq, fw.ID)
	dlRec := httptest.NewRecorder()
	s.downloadFile(dlRec, dlReq)
	if dlRec.Code != http.StatusOK {
		t.Fatalf("download expected 200, got %d body=%s", dlRec.Code, dlRec.Body.String())
	}
	if !bytes.Equal(dlRec.Body.Bytes(), data) {
		t.Fatal("served bytes mismatch")
	}
}

func withUploadIDParam(r *http.Request, id string) *http.Request {
	rc := chi.NewRouteContext()
	rc.URLParams.Add("uploadID", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
}
```
(import 补 `context` 与 `github.com/go-chi/chi/v5`;`withGatewayOTAURLParams` 那行可删,直接用 `withUploadIDParam`。)

Run: `cd api && go test ./internal/http/ -run 'TestFirmwareSignedURLServesViaDownloadFile' -v`
Expected: PASS(端到端:上传→任务→config/pull 签名 URL→downloadFile 验签并 serve 原字节)。

- [ ] **Step 6: 全量构建 + http 整包 + gosec(安全敏感)**

Run: `cd api && go build ./... && go test ./internal/http/ 2>&1 | tail -3 && go run github.com/securego/gosec/v2/cmd/gosec@v2.22.10 -severity medium -confidence medium -exclude G115 -quiet ./internal/http/... ./internal/modules/gateway/... 2>&1 | tail -5`
Expected:build OK;http 整包 ok;gosec 无新增中危项(固件 WriteFile 的 `#nosec G304` 已注释,路径用 minted id)。

- [ ] **Step 7: 提交**

```bash
git add api/internal/http/router_handlers_gateway.go api/internal/http/routes_gateway_bootstrap_test.go api/internal/http/routes_gateway_firmware_test.go
git commit -m "feat: config pull mints signed download URL for registry firmware"
```

---

## 自检(Self-Review)

**1. Spec 覆盖**
- §3 数据模型 → Task 1 ✓
- §4.1 store(Create/List/Get/GetByID + 隔离)→ Task 1 ✓
- §4.2 上传(multipart + sha256 + 存储 + 503)→ Task 2 ✓
- §4.3 列表 → Task 3 ✓
- §4.4 CreateOTATask firmware_id → Task 4 ✓
- §4.5 config/pull 签名 URL → Task 5 ✓
- §4.6 下载 → **复用 `/uploads/{id}` + `signDownload`**(无新端点),端到端测试在 Task 5 Step 5 验证 ✓
- §6 错误/边界(sha 不符 400、未配 503、firmware_id 不存在 404、签名过期 403[由 downloadFile 既有逻辑保证])→ Task 2/4/5 ✓
- §7 测试 → 各 Task ✓

**2. 占位符扫描**:无 TODO/TBD;代码步骤均完整。

**3. 类型一致性**:`GatewayFirmware`/`CreateFirmwareInput` 字段在 store/handler/测试一致;`CreateFirmware`/`ListFirmware`/`GetFirmware`/`GetFirmwareByID`/`findFirmwareLocked` 签名一致;`CreateOTATask` 新签名(+`firmwareID`)在 service/handler/所有既有调用点统一(Task 4 Step 7 兜底);`signDownload(key,id,exp)` 复用既有实现;firmware id `fw_<hex>` 与 downloadFile 的 `{id[:2]}/{id}` 路径逻辑一致。

**4. 关键安全点**:上传 `MaxBytesReader` 限 256MiB;sha256 服务端比对;写盘路径用 minted id(`#nosec G304` 注释);下载复用已审计的 `signDownload` + downloadFile(HMAC + `filepath.Base` 防穿越);firmware 按租户隔离,下载 by-id 的隔离论证见 spec §4.1。

---

## 执行交接(建议 Subagent-Driven)
本子项**安全敏感**(multipart 上传、sha256、HMAC 签名 URL、`CreateOTATask` 重构),建议用 **superpowers:subagent-driven-development**(每 Task 实现者 + spec/质量两阶段审查)。或 superpowers:executing-plans 内联批量执行。
