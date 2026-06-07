# 强制 + 验证的 OTA 固件签名 — 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让网关固件 OTA 强制带 Ed25519 签名,gateway-agent 在替换自身二进制前用固定公钥验签,验不过绝不刷写;私钥离线托管。

**Architecture:** 新增共享纯 Go 包 `internal/otasig`(规范化消息 + 签名/验签),被离线签名 CLI `cmd/ota-sign` 与 gateway-agent 共用以杜绝漂移;服务端 `CreateOTATask` 把 sha256+签名改为必填;agent 在 `pullConfig` 拿到 `pending_ota_tasks` 后下载→验签→原子替换→systemd 重启→健康确认/回滚。

**Tech Stack:** Go(标准库 `crypto/ed25519`/`crypto/sha256`/`crypto/x509`)、chi、systemd(`Restart=always` + `ExecStartPre` 守护脚本)。

设计依据:[2026-06-07-ota-firmware-signing-design.md](../specs/2026-06-07-ota-firmware-signing-design.md)

---

## 文件结构

| 文件 | 职责 | 动作 |
|---|---|---|
| `api/internal/otasig/otasig.go` | 规范化消息 + Ed25519 sign/verify/keygen/编解码(签名端与验签端唯一真相) | 新增 |
| `api/internal/otasig/otasig_test.go` | otasig 全单测 | 新增 |
| `api/internal/modules/gateway/service.go` | `CreateOTATask` 强制 sha256+签名;新增两个 `...Required` 错误 | 修改 |
| `api/internal/modules/gateway/service_test.go` | 更新既有 OTA 测试 + 新增必填用例 | 修改 |
| `api/internal/http/routes_gateway_management.go` | 新错误映射为 400 | 修改 |
| `api/cmd/ota-sign/main.go` | 离线签名 CLI(`gen-key`/`sign`) | 新增 |
| `api/cmd/ota-sign/main_test.go` | CLI 签名核心往返测试 | 新增 |
| `api/cmd/gateway-agent/ota.go` | OTA 执行器:任务选择/下载/验签/替换/回滚/上报 | 新增 |
| `api/cmd/gateway-agent/ota_test.go` | 执行器纯函数与文件操作单测 | 新增 |
| `api/cmd/gateway-agent/main.go` | `--ota-pubkey` flag、编译期 `version`、构造注入 | 修改 |
| `api/cmd/gateway-agent/agent.go` | Agent 字段、`pullConfig` 接入、`Start` 启动看门狗 | 修改 |
| `docs/deployment/mistypass-ota-guard.sh` | ExecStartPre 回滚守护脚本 | 新增 |
| `docs/ota-signing-runbook.md` | 签名/托管/轮换/systemd/真机验证手册 | 新增 |

**测试约定:** 所有 `go` 命令在 `api/` 目录下执行(`go.mod` 所在)。

---

## Task 1: 共享 `otasig` 包(签名/验签基元)

**Files:**
- Create: `api/internal/otasig/otasig.go`
- Test: `api/internal/otasig/otasig_test.go`

- [ ] **Step 1: 写失败测试**

Create `api/internal/otasig/otasig_test.go`:

```go
package otasig

import (
	"crypto/ed25519"
	"testing"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("fake firmware bytes")
	sha := SHA256Hex(data)
	sig := Sign(priv, "1.2.0", sha)
	if err := VerifyArtifact([]ed25519.PublicKey{pub}, "1.2.0", sha, sig, data); err != nil {
		t.Fatalf("expected verify ok, got %v", err)
	}
}

func TestVerifyRejectsTamperedData(t *testing.T) {
	pub, priv, _ := GenerateKey()
	data := []byte("fake firmware bytes")
	sha := SHA256Hex(data)
	sig := Sign(priv, "1.2.0", sha)
	if err := VerifyArtifact([]ed25519.PublicKey{pub}, "1.2.0", sha, sig, []byte("tampered")); err == nil {
		t.Fatal("expected sha256 mismatch error")
	}
}

func TestVerifyRejectsVersionSwap(t *testing.T) {
	pub, priv, _ := GenerateKey()
	data := []byte("fw")
	sha := SHA256Hex(data)
	sig := Sign(priv, "1.0.0", sha)
	if err := VerifyArtifact([]ed25519.PublicKey{pub}, "9.9.9", sha, sig, data); err == nil {
		t.Fatal("expected signature failure when version claim differs")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	_, priv, _ := GenerateKey()
	otherPub, _, _ := GenerateKey()
	data := []byte("fw")
	sha := SHA256Hex(data)
	sig := Sign(priv, "1.0.0", sha)
	if err := VerifyArtifact([]ed25519.PublicKey{otherPub}, "1.0.0", sha, sig, data); err == nil {
		t.Fatal("expected verify failure with wrong key")
	}
}

func TestPublicKeyHexRoundTrip(t *testing.T) {
	pub, _, _ := GenerateKey()
	got, err := ParsePublicKeyHex(MarshalPublicKeyHex(pub))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(pub) {
		t.Fatal("pubkey round trip mismatch")
	}
}

func TestPrivateKeyPEMRoundTrip(t *testing.T) {
	_, priv, _ := GenerateKey()
	pemBytes, err := MarshalPrivateKeyPEM(priv)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParsePrivateKeyPEM(pemBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(priv) {
		t.Fatal("privkey round trip mismatch")
	}
}

func TestParsePublicKeysHexMultiple(t *testing.T) {
	p1, _, _ := GenerateKey()
	p2, _, _ := GenerateKey()
	keys, err := ParsePublicKeysHex(MarshalPublicKeyHex(p1) + "," + MarshalPublicKeyHex(p2))
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("want 2 keys, got %d", len(keys))
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/otasig/ -v`
Expected: FAIL(编译错误:`undefined: GenerateKey` 等)。

- [ ] **Step 3: 写实现**

Create `api/internal/otasig/otasig.go`:

```go
// Package otasig implements the canonical message format and Ed25519
// sign/verify primitives shared by the OTA signing CLI (cmd/ota-sign) and the
// gateway-agent firmware verifier. One copy guarantees signer and verifier
// agree byte-for-byte on what is signed.
package otasig

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// Domain separates OTA signatures from any other use of the key and versions
// the message format.
const Domain = "mistypass-ota-v1"

// SignedMessage builds the canonical bytes that are Ed25519-signed:
//
//	"mistypass-ota-v1\n" + version + "\n" + lowercaseHex(sha256(binary))
func SignedMessage(version, sha256Hex string) []byte {
	return []byte(Domain + "\n" + strings.TrimSpace(version) + "\n" + strings.ToLower(strings.TrimSpace(sha256Hex)))
}

// SHA256Hex returns the lowercase hex SHA-256 of data.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Sign returns the hex Ed25519 signature over SignedMessage(version, sha256Hex).
func Sign(priv ed25519.PrivateKey, version, sha256Hex string) string {
	return hex.EncodeToString(ed25519.Sign(priv, SignedMessage(version, sha256Hex)))
}

// VerifyArtifact confirms data hashes to sha256Hex AND that sigHex is a valid
// Ed25519 signature (by any of keys) over SignedMessage(version, sha256Hex).
func VerifyArtifact(keys []ed25519.PublicKey, version, sha256Hex, sigHex string, data []byte) error {
	if len(keys) == 0 {
		return errors.New("no pinned public keys configured")
	}
	got := SHA256Hex(data)
	if !strings.EqualFold(got, strings.TrimSpace(sha256Hex)) {
		return fmt.Errorf("sha256 mismatch: computed %s, task declared %s", got, sha256Hex)
	}
	sig, err := hex.DecodeString(strings.TrimSpace(sigHex))
	if err != nil {
		return fmt.Errorf("signature hex decode: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("signature length %d, want %d", len(sig), ed25519.SignatureSize)
	}
	msg := SignedMessage(version, got)
	for _, k := range keys {
		if ed25519.Verify(k, msg, sig) {
			return nil
		}
	}
	return errors.New("signature not verified by any pinned public key")
}

// GenerateKey returns a fresh Ed25519 key pair (crypto/rand).
func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(nil)
}

// MarshalPublicKeyHex encodes the 32-byte raw public key as hex.
func MarshalPublicKeyHex(pub ed25519.PublicKey) string {
	return hex.EncodeToString(pub)
}

// ParsePublicKeyHex decodes a 32-byte raw Ed25519 public key from hex.
func ParsePublicKeyHex(s string) (ed25519.PublicKey, error) {
	b, err := hex.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("public key hex decode: %w", err)
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key length %d, want %d", len(b), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(b), nil
}

// ParsePublicKeysHex parses a comma-separated list of hex public keys (rotation).
func ParsePublicKeysHex(csv string) ([]ed25519.PublicKey, error) {
	var keys []ed25519.PublicKey
	for _, part := range strings.Split(csv, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		k, err := ParsePublicKeyHex(part)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

// MarshalPrivateKeyPEM encodes priv as a PKCS#8 PEM block.
func MarshalPrivateKeyPEM(priv ed25519.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// ParsePrivateKeyPEM decodes a PKCS#8 PEM Ed25519 private key.
func ParsePrivateKeyPEM(pemBytes []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("not an Ed25519 private key")
	}
	return priv, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd api && go test ./internal/otasig/ -v`
Expected: PASS(7 个测试全绿)。

- [ ] **Step 5: 提交**

```bash
git add api/internal/otasig/
git commit -m "feat: add otasig shared OTA signing primitives"
```

---

## Task 2: 服务端强制要求签名

**Files:**
- Modify: `api/internal/modules/gateway/service.go`(错误变量 ~L60-66;`CreateOTATask` 校验 L1730-1737;错误映射在 handler)
- Modify: `api/internal/http/routes_gateway_management.go:1134-1138`
- Test: `api/internal/modules/gateway/service_test.go`(更新 L709-717,扩展 `TestCreateOTATaskValidation`)

- [ ] **Step 1: 更新既有测试 + 新增必填断言(先让其失败)**

在 `api/internal/modules/gateway/service_test.go` 顶部 import 块加入 `"strings"`(当前未导入):

```go
import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)
```

把 `TestCreateListAndUpdateOTATask`(L709-717)中**空签名**改为合法的 128 hex 签名,否则改强制后会失败:

```go
	created, err := svc.CreateOTATask(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"v2.4.1",
		"https://cdn.example.com/firmware/gw_demo_001/v2.4.1.bin",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		strings.Repeat("b", 128), // firmware_signature (now mandatory; format-only check)
		"tenant-admin@example.com",
	)
```

在 `TestCreateOTATaskValidation`(末尾 L790 之前)追加三个必填/格式用例:

```go
	// empty sha256 → required
	_, err = svc.CreateOTATask(
		"tenant_demo_jakarta", "gw_demo_001", "v2.4.1",
		"https://cdn.example.com/firmware/gw_demo_001/v2.4.1.bin",
		"", strings.Repeat("b", 128), "",
	)
	if err != ErrGatewayOTAFirmwareSHA256Required {
		t.Fatalf("unexpected missing sha256 error: %v", err)
	}

	// valid sha256, empty signature → required
	_, err = svc.CreateOTATask(
		"tenant_demo_jakarta", "gw_demo_001", "v2.4.1",
		"https://cdn.example.com/firmware/gw_demo_001/v2.4.1.bin",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"", "",
	)
	if err != ErrGatewayOTAFirmwareSignatureRequired {
		t.Fatalf("unexpected missing signature error: %v", err)
	}

	// valid sha256, malformed signature → invalid
	_, err = svc.CreateOTATask(
		"tenant_demo_jakarta", "gw_demo_001", "v2.4.1",
		"https://cdn.example.com/firmware/gw_demo_001/v2.4.1.bin",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"not-a-valid-signature", "",
	)
	if err != ErrGatewayOTAFirmwareSignatureInvalid {
		t.Fatalf("unexpected invalid signature error: %v", err)
	}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/modules/gateway/ -run 'TestCreate.*OTATask' -v`
Expected: FAIL(`undefined: ErrGatewayOTAFirmwareSHA256Required` / `...SignatureRequired`,以及强制前空签名仍被接受)。

- [ ] **Step 3: 加错误变量**

在 `api/internal/modules/gateway/service.go` 紧接 L64(`ErrGatewayOTAFirmwareSignatureInvalid` 那行)后新增:

```go
var ErrGatewayOTAFirmwareSHA256Required = errors.New("gateway ota firmware_sha256 is required")
var ErrGatewayOTAFirmwareSignatureRequired = errors.New("gateway ota firmware_signature is required")
```

- [ ] **Step 4: 改 `CreateOTATask` 校验为必填**

把 `api/internal/modules/gateway/service.go` L1730-1737 替换为:

```go
	nextSHA256 := strings.ToLower(strings.TrimSpace(firmwareSHA256))
	if nextSHA256 == "" {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareSHA256Required
	}
	if !isValidSHA256Hex(nextSHA256) {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareSHA256Invalid
	}
	nextSignature := strings.ToLower(strings.TrimSpace(firmwareSignature))
	if nextSignature == "" {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareSignatureRequired
	}
	if !isValidEd25519SignatureHex(nextSignature) {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareSignatureInvalid
	}
```

- [ ] **Step 5: 把新错误映射为 400**

在 `api/internal/http/routes_gateway_management.go` 的 `createGatewayOTATask` 错误 switch(L1134-1138)里,把两个新错误加入 400 分支:

```go
		case errors.Is(err, gateway.ErrGatewayIDRequired),
			errors.Is(err, gateway.ErrGatewayOTAFirmwareVersionRequired),
			errors.Is(err, gateway.ErrGatewayOTAFirmwareURLRequired),
			errors.Is(err, gateway.ErrGatewayOTAFirmwareSHA256Required),
			errors.Is(err, gateway.ErrGatewayOTAFirmwareSHA256Invalid),
			errors.Is(err, gateway.ErrGatewayOTAFirmwareSignatureRequired),
			errors.Is(err, gateway.ErrGatewayOTAFirmwareSignatureInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
```

- [ ] **Step 6: 运行测试确认通过**

Run: `cd api && go test ./internal/modules/gateway/ -run 'TestCreate.*OTATask' -v && cd api && go build ./...`
Expected: PASS;`go build ./...` 无错误。

- [ ] **Step 7: 提交**

```bash
git add api/internal/modules/gateway/service.go api/internal/modules/gateway/service_test.go api/internal/http/routes_gateway_management.go
git commit -m "feat: require signed firmware for OTA tasks"
```

---

## Task 3: 离线签名 CLI `ota-sign`

**Files:**
- Create: `api/cmd/ota-sign/main.go`
- Test: `api/cmd/ota-sign/main_test.go`

- [ ] **Step 1: 写失败测试**

Create `api/cmd/ota-sign/main_test.go`:

```go
package main

import (
	"crypto/ed25519"
	"testing"

	"github.com/mistypass/cloud/api/internal/otasig"
)

func TestSignFileProducesAgentVerifiableSignature(t *testing.T) {
	pub, priv, _ := otasig.GenerateKey()
	pemBytes, _ := otasig.MarshalPrivateKeyPEM(priv)
	data := []byte("agent binary v1.4.0")

	sha, sig, err := signFile(pemBytes, "1.4.0", data)
	if err != nil {
		t.Fatal(err)
	}
	// The agent-side verifier must accept what the CLI produced.
	if err := otasig.VerifyArtifact([]ed25519.PublicKey{pub}, "1.4.0", sha, sig, data); err != nil {
		t.Fatalf("agent would reject CLI signature: %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./cmd/ota-sign/ -v`
Expected: FAIL(`undefined: signFile`)。

- [ ] **Step 3: 写实现**

Create `api/cmd/ota-sign/main.go`:

```go
// Command ota-sign generates the OTA signing key pair and signs gateway-agent
// firmware. The private key never leaves the machine this runs on (offline
// signing): the API/staging server only ever sees the signature, never the key.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/mistypass/cloud/api/internal/otasig"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "gen-key":
		if err := runGenKey(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "gen-key:", err)
			os.Exit(1)
		}
	case "sign":
		if err := runSign(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "sign:", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  ota-sign gen-key --out-priv priv.pem --out-pub pub.hex")
	fmt.Fprintln(os.Stderr, "  ota-sign sign --key priv.pem --version <ver> --in <binary> [--gateway <id>] [--tenant <id>] [--url <url>]")
}

func runGenKey(args []string) error {
	fs := flag.NewFlagSet("gen-key", flag.ExitOnError)
	outPriv := fs.String("out-priv", "ota-priv.pem", "output path for PKCS#8 private key PEM")
	outPub := fs.String("out-pub", "ota-pub.hex", "output path for hex public key")
	_ = fs.Parse(args)

	pub, priv, err := otasig.GenerateKey()
	if err != nil {
		return err
	}
	pemBytes, err := otasig.MarshalPrivateKeyPEM(priv)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*outPriv, pemBytes, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(*outPub, []byte(otasig.MarshalPublicKeyHex(pub)+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf("private key: %s  (keep OFFLINE — never copy to the API/staging server)\n", *outPriv)
	fmt.Printf("public key:  %s\n", *outPub)
	fmt.Printf("pin on agent: --ota-pubkey %s\n", otasig.MarshalPublicKeyHex(pub))
	return nil
}

// signFile is the testable core: hash + sign with the PEM private key.
func signFile(privPEM []byte, version string, data []byte) (sha, sig string, err error) {
	priv, err := otasig.ParsePrivateKeyPEM(privPEM)
	if err != nil {
		return "", "", err
	}
	sha = otasig.SHA256Hex(data)
	sig = otasig.Sign(priv, version, sha)
	return sha, sig, nil
}

func runSign(args []string) error {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	keyPath := fs.String("key", "", "path to PKCS#8 Ed25519 private key PEM")
	version := fs.String("version", "", "firmware version (e.g. 1.4.0)")
	in := fs.String("in", "", "path to firmware binary to sign")
	gateway := fs.String("gateway", "", "target gateway id (for printed task JSON)")
	tenant := fs.String("tenant", "", "tenant id (for printed task JSON)")
	url := fs.String("url", "", "firmware download URL (for printed task JSON)")
	_ = fs.Parse(args)

	if *keyPath == "" || *version == "" || *in == "" {
		return fmt.Errorf("--key, --version and --in are required")
	}
	privPEM, err := os.ReadFile(*keyPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	sha, sig, err := signFile(privPEM, *version, data)
	if err != nil {
		return err
	}
	task := map[string]string{
		"tenant_id":          *tenant,
		"firmware_version":   *version,
		"firmware_url":       *url,
		"firmware_sha256":    sha,
		"firmware_signature": sig,
	}
	body, _ := json.MarshalIndent(task, "", "  ")
	fmt.Printf("firmware_sha256:    %s\n", sha)
	fmt.Printf("firmware_signature: %s\n", sig)
	fmt.Printf("\nPOST /api/v1/gateways/%s/ota/tasks\n%s\n", *gateway, string(body))
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd api && go test ./cmd/ota-sign/ -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add api/cmd/ota-sign/
git commit -m "feat: add ota-sign offline signing CLI"
```

---

## Task 4: agent OTA 决策 + 下载(纯函数,无文件改动)

**Files:**
- Create: `api/cmd/gateway-agent/ota.go`
- Test: `api/cmd/gateway-agent/ota_test.go`

- [ ] **Step 1: 写失败测试**

Create `api/cmd/gateway-agent/ota_test.go`:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.0", "1.2.0", 0},
		{"1.3.0", "1.2.9", 1},
		{"1.2.0", "1.10.0", -1},
		{"v2.0.0", "1.9.9", 1},
		{"1.2", "1.2.0", 0},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSelectOTATaskPicksNewestAboveCurrent(t *testing.T) {
	tasks := []otaTask{
		{FirmwareVersion: "1.0.0", FirmwareURL: "u", FirmwareSignature: "s"},
		{FirmwareVersion: "1.3.0", FirmwareURL: "u", FirmwareSignature: "s"},
		{FirmwareVersion: "1.2.0", FirmwareURL: "u", FirmwareSignature: "s"},
	}
	got, ok := selectOTATask(tasks, "1.1.0")
	if !ok || got.FirmwareVersion != "1.3.0" {
		t.Fatalf("want 1.3.0, got %+v ok=%v", got, ok)
	}
}

func TestSelectOTATaskSkipsDowngradeEqualAndUnsigned(t *testing.T) {
	tasks := []otaTask{
		{FirmwareVersion: "1.0.0", FirmwareURL: "u", FirmwareSignature: "s"},
		{FirmwareVersion: "1.1.0", FirmwareURL: "u", FirmwareSignature: "s"},
		{FirmwareVersion: "2.0.0", FirmwareURL: "u", FirmwareSignature: ""}, // unsigned → ignored
	}
	if _, ok := selectOTATask(tasks, "1.1.0"); ok {
		t.Fatal("must not select <=current or unsigned tasks")
	}
}

func TestDownloadFirmware(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("firmware-bytes"))
	}))
	defer srv.Close()
	data, err := downloadFirmware(srv.Client(), srv.URL, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "firmware-bytes" {
		t.Fatalf("unexpected body %q", data)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./cmd/gateway-agent/ -run 'TestCompareVersions|TestSelectOTATask|TestDownloadFirmware' -v`
Expected: FAIL(`undefined: compareVersions` 等)。

- [ ] **Step 3: 写实现**

Create `api/cmd/gateway-agent/ota.go`:

```go
package main

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/mistypass/cloud/api/internal/otasig"
)

const otaMaxFirmwareBytes = 256 << 20 // 256 MiB download cap

// otaTask mirrors gateway.GatewayOTATask as delivered in the config/pull
// response under "pending_ota_tasks".
type otaTask struct {
	ID                string `json:"id"`
	GatewayID         string `json:"gateway_id"`
	TenantID          string `json:"tenant_id"`
	FirmwareVersion   string `json:"firmware_version"`
	FirmwareURL       string `json:"firmware_url"`
	FirmwareSHA256    string `json:"firmware_sha256"`
	FirmwareSignature string `json:"firmware_signature"`
	Status            string `json:"status"`
}

// compareVersions compares dotted numeric versions (a leading "v" is ignored).
// Returns -1 if a<b, 0 if equal, +1 if a>b. Non-numeric parts count as 0
// (MVP scope: no pre-release/build-metadata handling).
func compareVersions(a, b string) int {
	as := strings.Split(strings.TrimPrefix(strings.TrimSpace(a), "v"), ".")
	bs := strings.Split(strings.TrimPrefix(strings.TrimSpace(b), "v"), ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var ai, bi int
		if i < len(as) {
			ai, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bi, _ = strconv.Atoi(bs[i])
		}
		if ai != bi {
			if ai < bi {
				return -1
			}
			return 1
		}
	}
	return 0
}

// selectOTATask returns the newest signed task strictly newer than
// currentVersion. ok=false when there is nothing to apply.
func selectOTATask(tasks []otaTask, currentVersion string) (otaTask, bool) {
	var best otaTask
	found := false
	for _, t := range tasks {
		if strings.TrimSpace(t.FirmwareURL) == "" || strings.TrimSpace(t.FirmwareSignature) == "" {
			continue
		}
		if compareVersions(t.FirmwareVersion, currentVersion) <= 0 {
			continue // anti-downgrade
		}
		if !found || compareVersions(t.FirmwareVersion, best.FirmwareVersion) > 0 {
			best, found = t, true
		}
	}
	return best, found
}

// downloadFirmware fetches up to maxBytes from url.
func downloadFirmware(client *http.Client, url string, maxBytes int64) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBytes))
}

// (used by Task 5/6) keep otasig referenced so imports stay tidy across tasks.
var _ = otasig.Domain
```

> 注:`var _ = otasig.Domain` 仅为 Task 4 单独编译时保留 import;Task 5/6 真正调用 `otasig.VerifyArtifact` 后可删除此行(Task 6 Step 提示会删)。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd api && go test ./cmd/gateway-agent/ -run 'TestCompareVersions|TestSelectOTATask|TestDownloadFirmware' -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add api/cmd/gateway-agent/ota.go api/cmd/gateway-agent/ota_test.go
git commit -m "feat: add gateway-agent OTA task selection and download"
```

---

## Task 5: agent 二进制替换 + 回滚 + 标记(文件操作)

**Files:**
- Modify: `api/cmd/gateway-agent/ota.go`(追加文件操作 + 标记函数)
- Modify: `api/cmd/gateway-agent/ota_test.go`(追加文件操作测试)

- [ ] **Step 1: 追加失败测试**

把 `api/cmd/gateway-agent/ota_test.go` 顶部 import 改为:

```go
import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)
```

在文件末尾追加:

```go
func TestSwapBinaryBacksUpAndReplaces(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "agent")
	bak := bin + ".bak"
	if err := os.WriteFile(bin, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := swapBinary([]byte("NEW"), bin, bak); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(bin); string(b) != "NEW" {
		t.Fatalf("bin not replaced: %q", b)
	}
	if b, _ := os.ReadFile(bak); string(b) != "OLD" {
		t.Fatalf("backup not written: %q", b)
	}
}

func TestRestoreBinary(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "agent")
	bak := bin + ".bak"
	_ = os.WriteFile(bin, []byte("NEW-BROKEN"), 0o755)
	_ = os.WriteFile(bak, []byte("OLD-GOOD"), 0o755)
	if err := restoreBinary(bin, bak); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(bin); string(b) != "OLD-GOOD" {
		t.Fatalf("restore failed: %q", b)
	}
}

func TestOTAMarkerRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ota-pending.json")
	in := otaMarker{TaskID: "t1", NewVersion: "1.4.0", BakPath: "/x.bak", Attempts: 1}
	if err := writeOTAMarker(path, in); err != nil {
		t.Fatal(err)
	}
	got, ok, err := readOTAMarker(path)
	if err != nil || !ok {
		t.Fatalf("read marker ok=%v err=%v", ok, err)
	}
	if got.TaskID != "t1" || got.NewVersion != "1.4.0" {
		t.Fatalf("unexpected marker %+v", got)
	}
}

func TestReadOTAMarkerAbsent(t *testing.T) {
	_, ok, err := readOTAMarker(filepath.Join(t.TempDir(), "none.json"))
	if err != nil || ok {
		t.Fatalf("expected absent marker, ok=%v err=%v", ok, err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./cmd/gateway-agent/ -run 'TestSwapBinary|TestRestoreBinary|TestOTAMarker|TestReadOTAMarker' -v`
Expected: FAIL(`undefined: swapBinary` 等)。

- [ ] **Step 3: 追加实现**

在 `api/cmd/gateway-agent/ota.go` 的 import 块加入 `"encoding/json"`、`"os"`、`"path/filepath"`(与已有 import 合并),并在文件末尾追加:

```go
// otaMarker records an in-flight self-update awaiting post-restart confirmation.
type otaMarker struct {
	TaskID     string `json:"task_id"`
	TenantID   string `json:"tenant_id"`
	GatewayID  string `json:"gateway_id"`
	NewVersion string `json:"new_version"`
	BakPath    string `json:"bak_path"`
	Attempts   int    `json:"attempts"`
	Confirmed  bool   `json:"confirmed"`
}

func writeOTAMarker(path string, m otaMarker) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func readOTAMarker(path string) (otaMarker, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return otaMarker{}, false, nil
	}
	if err != nil {
		return otaMarker{}, false, err
	}
	var m otaMarker
	if err := json.Unmarshal(data, &m); err != nil {
		return otaMarker{}, false, err
	}
	return m, true, nil
}

// swapBinary backs up binPath→bakPath, then atomically replaces binPath with
// newData. The temp file is created in binPath's directory so rename stays on
// one filesystem (atomic). Replacing a running binary via rename is safe on
// Linux: the running process keeps the old (unlinked) inode until it exits.
func swapBinary(newData []byte, binPath, bakPath string) error {
	dir := filepath.Dir(binPath)
	tmp, err := os.CreateTemp(dir, ".ota-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once renamed
	if _, err := tmp.Write(newData); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}
	if cur, err := os.ReadFile(binPath); err == nil {
		if err := os.WriteFile(bakPath, cur, 0o755); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
	}
	return os.Rename(tmpName, binPath)
}

// restoreBinary copies bakPath back over binPath (rollback).
func restoreBinary(binPath, bakPath string) error {
	data, err := os.ReadFile(bakPath)
	if err != nil {
		return err
	}
	return os.WriteFile(binPath, data, 0o755)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd api && go test ./cmd/gateway-agent/ -run 'TestSwapBinary|TestRestoreBinary|TestOTAMarker|TestReadOTAMarker' -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add api/cmd/gateway-agent/ota.go api/cmd/gateway-agent/ota_test.go
git commit -m "feat: add gateway-agent firmware swap and rollback"
```

---

## Task 6: 接入 agent(flag / 版本 / pullConfig / 上报 / 看门狗)

**Files:**
- Modify: `api/cmd/gateway-agent/ota.go`(编排 + 上报方法)
- Modify: `api/cmd/gateway-agent/agent.go`(struct 字段、`pullConfig` 解码与调用、`Start` 看门狗)
- Modify: `api/cmd/gateway-agent/main.go`(`--ota-pubkey`、`version`、构造注入)
- Test: `api/cmd/gateway-agent/ota_test.go`(上报体构造测试)

- [ ] **Step 1: 追加失败测试(上报体构造)**

在 `api/cmd/gateway-agent/ota_test.go` 末尾追加:

```go
func TestOTAReportBody(t *testing.T) {
	body := otaReportBody("gw1", "t1", "task1", "failed", "boom")
	got := string(body)
	for _, want := range []string{`"gateway_id":"gw1"`, `"task_id":"task1"`, `"status":"failed"`, `"error_message":"boom"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("body %s missing %s", got, want)
		}
	}
}
```

并把该测试文件 import 块补上 `"strings"`:

```go
import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./cmd/gateway-agent/ -run TestOTAReportBody -v`
Expected: FAIL(`undefined: otaReportBody`)。

- [ ] **Step 3: 追加编排 + 上报实现**

在 `api/cmd/gateway-agent/ota.go` 的 import 块补上 `"net/http"` 已有、再加 `"time"`;删除 Task 4 末尾的占位 `var _ = otasig.Domain` 行(下面开始真正用到 `otasig`)。在文件末尾追加:

```go
func (a *Agent) otaHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Minute}
}

func (a *Agent) otaMarkerPath() string {
	return filepath.Join(filepath.Dir(a.deviceTokenFile), "ota-pending.json")
}

// otaReportBody builds the JSON for POST /api/v1/gateway/ota/report.
// status ∈ {dispatching, succeeded, failed} (server enum has no "rolled_back";
// a rollback is reported as failed with an explanatory error_message).
func otaReportBody(gatewayID, tenantID, taskID, status, errMsg string) []byte {
	b, _ := json.Marshal(map[string]string{
		"gateway_id":    gatewayID,
		"tenant_id":     tenantID,
		"task_id":       taskID,
		"status":        status,
		"error_message": errMsg,
	})
	return b
}

func (a *Agent) reportOTA(task otaTask, status, errMsg string) error {
	resp, err := a.apiRequest("POST", "/api/v1/gateway/ota/report",
		otaReportBody(a.gatewayID, a.tenantID, task.ID, status, errMsg))
	if err != nil {
		a.logger.Warn("OTA report failed", "task", task.ID, "status", status, "error", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		a.logger.Warn("OTA report non-200", "task", task.ID, "status", status, "code", resp.StatusCode)
		return fmt.Errorf("report status %d", resp.StatusCode)
	}
	return nil
}

// maybeApplyOTA is invoked from pullConfig with the cloud's pending tasks.
func (a *Agent) maybeApplyOTA(tasks []otaTask) {
	if len(a.otaPublicKeys) == 0 {
		if len(tasks) > 0 {
			a.logger.Warn("OTA tasks present but no --ota-pubkey configured; ignoring", "count", len(tasks))
		}
		return
	}
	task, ok := selectOTATask(tasks, a.agentVersion)
	if !ok {
		return
	}
	if err := a.runOTA(task); err != nil {
		a.logger.Error("OTA apply failed", "task", task.ID, "error", err)
		_ = a.reportOTA(task, "failed", err.Error())
	}
}

// runOTA downloads, verifies, and installs one firmware task, then exits so
// systemd restarts into the new binary. On any pre-install failure it returns
// an error WITHOUT touching the running binary.
func (a *Agent) runOTA(task otaTask) error {
	_ = a.reportOTA(task, "dispatching", "")

	data, err := downloadFirmware(a.otaHTTPClient(), task.FirmwareURL, otaMaxFirmwareBytes)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	if err := otasig.VerifyArtifact(a.otaPublicKeys, task.FirmwareVersion, task.FirmwareSHA256, task.FirmwareSignature, data); err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate self: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(binPath); err == nil {
		binPath = resolved
	}
	bakPath := binPath + ".bak"

	if err := swapBinary(data, binPath, bakPath); err != nil {
		return fmt.Errorf("install: %w", err)
	}
	if err := writeOTAMarker(a.otaMarkerPath(), otaMarker{
		TaskID:     task.ID,
		TenantID:   task.TenantID,
		GatewayID:  task.GatewayID,
		NewVersion: task.FirmwareVersion,
		BakPath:    bakPath,
	}); err != nil {
		_ = restoreBinary(binPath, bakPath) // no confirm channel → abort safely
		return fmt.Errorf("write marker: %w", err)
	}

	a.logger.Info("OTA installed; exiting for systemd restart into new binary", "version", task.FirmwareVersion)
	os.Exit(0) // systemd Restart=always relaunches the new binary
	return nil  // unreachable
}

// confirmPendingOTA finalizes a self-update once a post-restart pullConfig
// succeeds (proxy for "new binary is healthy"). Idempotent; retried each pull
// until the success report lands.
func (a *Agent) confirmPendingOTA() {
	path := a.otaMarkerPath()
	m, ok, err := readOTAMarker(path)
	if err != nil || !ok || m.Confirmed {
		return
	}
	if err := a.reportOTA(otaTask{ID: m.TaskID, TenantID: m.TenantID, GatewayID: m.GatewayID}, "succeeded", ""); err != nil {
		return // keep marker; retry next successful pull
	}
	_ = os.Remove(m.BakPath)
	_ = os.Remove(path)
	a.logger.Info("OTA confirmed healthy", "version", m.NewVersion)
}

// otaWatchdog rolls back if a pending update is not confirmed within timeout
// (covers "new binary starts but never reaches the cloud").
func (a *Agent) otaWatchdog(timeout time.Duration) {
	m, ok, err := readOTAMarker(a.otaMarkerPath())
	if err != nil || !ok || m.Confirmed {
		return
	}
	select {
	case <-time.After(timeout):
	case <-a.stopCh:
		return
	}
	if m2, ok, _ := readOTAMarker(a.otaMarkerPath()); !ok || m2.Confirmed {
		return // confirmed meanwhile
	}
	binPath, _ := os.Executable()
	if resolved, err := filepath.EvalSymlinks(binPath); err == nil {
		binPath = resolved
	}
	if err := restoreBinary(binPath, m.BakPath); err != nil {
		a.logger.Error("OTA rollback restore failed", "error", err)
		return
	}
	_ = a.reportOTA(otaTask{ID: m.TaskID, TenantID: m.TenantID, GatewayID: m.GatewayID}, "failed", "post-update health check timed out; rolled back")
	_ = os.Remove(a.otaMarkerPath())
	a.logger.Warn("OTA rolled back after health timeout; exiting for systemd restart into previous binary", "version", m.NewVersion)
	os.Exit(0)
}
```

- [ ] **Step 4: 给 Agent 加字段(agent.go)**

在 `api/cmd/gateway-agent/agent.go` 的 import 块加入 `"crypto/ed25519"`。在 Agent struct(L46 `mtlsCertDir` 之后、L48 `mu` 之前)加入两个字段:

```go
	mtlsCertDir   string // directory for mTLS client cert + key (e.g. /var/lib/mistypass/mtls/)
	agentVersion  string // build-time version, used for OTA anti-downgrade
	otaPublicKeys []ed25519.PublicKey // pinned Ed25519 keys for OTA verification (empty = OTA disabled)
```

- [ ] **Step 5: 接入 `pullConfig`(agent.go L392-413)**

把 `pullConfig` 里的 `result` 解码结构体扩展为包含 OTA 任务,并在成功更新规则后调用确认与应用。替换 L392-412:

```go
	var result struct {
		AuthzCache struct {
			Version     string       `json:"version"`
			AccessRules []AccessRule `json:"access_rules"`
		} `json:"authz_cache"`
		PendingOTATasks []otaTask `json:"pending_ota_tasks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("config/pull decode: %w", err)
	}

	a.mu.Lock()
	a.accessRules = result.AuthzCache.AccessRules
	a.ruleVersion = result.AuthzCache.Version
	a.rulesUpdatedAt = time.Now().UTC()
	a.mu.Unlock()

	a.logger.Info("config pulled",
		"version", result.AuthzCache.Version,
		"access_rules", len(result.AuthzCache.AccessRules),
	)

	a.confirmPendingOTA()                 // finalize a prior self-update on a healthy pull
	a.maybeApplyOTA(result.PendingOTATasks) // may os.Exit to restart into new binary
	return nil
```

- [ ] **Step 6: 启动看门狗(agent.go `Start`)**

在 `api/cmd/gateway-agent/agent.go` 的 `Start()` 中,`a.stopCh = make(chan struct{})`(L101)之后加入:

```go
	// OTA rollback watchdog: if a pending self-update is not confirmed in time,
	// restore the previous binary. No-op when there is no pending marker.
	go a.otaWatchdog(90 * time.Second)
```

- [ ] **Step 7: 加 flag 与版本(main.go)**

在 `api/cmd/gateway-agent/main.go` 的 import 块加入 `"crypto/ed25519"`、`"strings"`、`"github.com/mistypass/cloud/api/internal/otasig"`。在 `package main` 下、`func main()` 上方加入包级版本变量:

```go
// version is set at build time: -ldflags "-X main.version=1.4.0". Used for
// OTA anti-downgrade. Defaults to "dev" (which is < any numeric release).
var version = "dev"
```

在 L59(`mtlsCertDir` flag)之后加入:

```go
	otaPubKey := flag.String("ota-pubkey", "", "Comma-separated hex Ed25519 public key(s) pinned for OTA firmware verification. Empty = OTA disabled.")
```

在 `flag.Parse()`(L60)之后、构造 `agent` 之前加入公钥解析:

```go
	var otaKeys []ed25519.PublicKey
	if strings.TrimSpace(*otaPubKey) != "" {
		ks, err := otasig.ParsePublicKeysHex(*otaPubKey)
		if err != nil {
			logger.Error("invalid --ota-pubkey; OTA disabled (agent keeps running for door duty)", "error", err)
		} else {
			otaKeys = ks
		}
	}
```

在 `agent := &Agent{...}`(L88 `mtlsCertDir: *mtlsCertDir,` 之后)加入两个字段:

```go
		mtlsCertDir:   *mtlsCertDir,
		agentVersion:  version,
		otaPublicKeys: otaKeys,
```

- [ ] **Step 8: 运行测试 + 全量构建**

Run: `cd api && go test ./cmd/gateway-agent/ -v && cd api && go build ./...`
Expected: PASS;`go build ./...` 无错误。

- [ ] **Step 9: ARM64 交叉编译冒烟(确认可部署到 Orange Pi)**

Run: `cd api && GOOS=linux GOARCH=arm64 go build -o /tmp/gateway-agent-arm64 ./cmd/gateway-agent && ls -lh /tmp/gateway-agent-arm64`
Expected: 成功产出二进制(数十 MB)。

- [ ] **Step 10: 提交**

```bash
git add api/cmd/gateway-agent/ota.go api/cmd/gateway-agent/agent.go api/cmd/gateway-agent/main.go api/cmd/gateway-agent/ota_test.go
git commit -m "feat: wire verified OTA self-update into gateway-agent"
```

---

## Task 7: 回滚守护脚本 + 运维手册

**Files:**
- Create: `docs/deployment/mistypass-ota-guard.sh`
- Create: `docs/ota-signing-runbook.md`

- [ ] **Step 1: 写 ExecStartPre 守护脚本**

Create `docs/deployment/mistypass-ota-guard.sh`:

```sh
#!/bin/sh
# ExecStartPre guard for gateway-agent self-update auto-rollback.
# Runs before every agent start. If a pending OTA marker is not confirmed after
# MAX_ATTEMPTS boots, restore the backed-up binary. Covers "new binary won't
# start at all" (the agent itself can't roll back if it never runs).
set -eu

MARKER="${MISTYPASS_OTA_MARKER:-/var/lib/mistypass/ota-pending.json}"
BIN="${MISTYPASS_AGENT_BIN:-/usr/local/bin/gateway-agent}"
MAX_ATTEMPTS="${MISTYPASS_OTA_MAX_ATTEMPTS:-3}"

[ -f "$MARKER" ] || exit 0

confirmed=$(grep -o '"confirmed":[^,}]*' "$MARKER" | head -1 | sed 's/.*://; s/[^a-z]//g')
[ "$confirmed" = "true" ] && exit 0

attempts=$(grep -o '"attempts":[0-9]*' "$MARKER" | head -1 | sed 's/.*://')
[ -n "$attempts" ] || attempts=0
attempts=$((attempts + 1))

bak=$(grep -o '"bak_path":"[^"]*"' "$MARKER" | head -1 | sed 's/.*:"//; s/"$//')

if [ "$attempts" -ge "$MAX_ATTEMPTS" ] && [ -n "$bak" ] && [ -f "$bak" ]; then
  cp "$bak" "$BIN"
  rm -f "$MARKER"
  echo "mistypass-ota-guard: rolled back to $bak after $attempts failed boots" >&2
  exit 0
fi

tmp="$(mktemp)"
sed "s/\"attempts\":[0-9]*/\"attempts\":$attempts/" "$MARKER" > "$tmp" && mv "$tmp" "$MARKER"
echo "mistypass-ota-guard: pending OTA boot attempt $attempts" >&2
exit 0
```

- [ ] **Step 2: 写运维手册**

Create `docs/ota-signing-runbook.md`:

````markdown
# OTA 固件签名运维手册

## 信任模型(必读)
- 信任锚 = agent 内固定的公钥(`--ota-pubkey`)。
- **私钥离线托管**:只在你本地/一台不对外的机器,绝不复制到 API/staging(Mac mini)。被攻破的服务器没有私钥 → 伪造不出能过验签的固件。
- 验签公钥固定在 agent,绝不与固件一起动态下发。

## 1. 一次性:生成密钥对
```bash
cd api && go run ./cmd/ota-sign gen-key --out-priv ota-priv.pem --out-pub ota-pub.hex
```
- `ota-priv.pem` 离线保管(密码管理器/离线盘),`chmod 600`。
- `ota-pub.hex` 填进 agent 的 `--ota-pubkey`。

## 2. agent 端固定公钥(systemd)
`/etc/systemd/system/gateway-agent.service`:
```ini
[Service]
Restart=always
RestartSec=3
ExecStartPre=/usr/local/bin/mistypass-ota-guard.sh
ExecStart=/usr/local/bin/gateway-agent \
  --api https://api.example.com \
  --gateway gw_demo_001 --tenant tenant_demo_jakarta \
  --ota-pubkey <ota-pub.hex 内容>
```
把 `docs/deployment/mistypass-ota-guard.sh` 部署到 `/usr/local/bin/mistypass-ota-guard.sh` 并 `chmod +x`。
`Restart=always` 与 `ExecStartPre` 守护是自动回滚的前提。

## 3. 发布一次签名更新
```bash
# 构建目标平台二进制(版本号经 ldflags 注入,用于防降级)
cd api && GOOS=linux GOARCH=arm64 go build -ldflags "-X main.version=1.4.0" -o gateway-agent-1.4.0 ./cmd/gateway-agent
# 离线签名(私钥不离开本机)
go run ./cmd/ota-sign sign --key ota-priv.pem --version 1.4.0 --in gateway-agent-1.4.0 \
  --gateway gw_demo_001 --tenant tenant_demo_jakarta \
  --url https://cdn.example.com/firmware/gw_demo_001/1.4.0
# 上传 gateway-agent-1.4.0 到上面的 --url(任意静态托管;签名而非 TLS 是完整性锚)
# 用打印出的 JSON 创建任务:
curl -X POST https://api.example.com/api/v1/gateways/gw_demo_001/ota/tasks \
  -H "Authorization: Bearer <admin-token>" -H "Content-Type: application/json" \
  -d @task.json
```
服务端会拒绝缺少 `firmware_sha256` / `firmware_signature` 的任务(400)。

## 4. 密钥轮换
`--ota-pubkey` 支持逗号分隔多把:先把新公钥追加进去部署(agent 接受新旧两把)→ 之后改用新私钥签名 → 全部网关切换后,移除旧公钥。

## 5. 真机验证(Orange Pi)
- **正常路径**:发布 version 高于当前的签名更新 → agent 日志出现 `OTA installed; exiting...` → systemd 重启 → 下次 pull 成功后 `OTA confirmed healthy` → 后台任务状态变 `succeeded`。
- **强制回滚**:用一个**起不来**的二进制(如 `printf '#!/bin/sh\nexit 1' > bad; ` 包装,或截断的二进制)走同样流程签名发布 → 观察 `ExecStartPre` 守护在 3 次启动后 `rolled back`,旧二进制恢复运行;后台任务状态最终为 `failed`,error_message 记录回滚原因。
- **验签失败**:把 `--url` 指向被篡改 1 字节的二进制 → agent `verify:` 失败 → `report failed`,**二进制原封不动**继续跑旧版。
````

- [ ] **Step 3: 提交**

```bash
git add docs/deployment/mistypass-ota-guard.sh docs/ota-signing-runbook.md
chmod +x docs/deployment/mistypass-ota-guard.sh
git add docs/deployment/mistypass-ota-guard.sh
git commit -m "docs: add OTA signing runbook and rollback guard"
```

---

## 自检(Self-Review)

**1. Spec 覆盖(逐节核对)**
- §5.1 离线签名 CLI → Task 3 ✓
- §5.2 服务端强制 → Task 2 ✓
- §5.3 agent 固定公钥(`--ota-pubkey` + fail-closed)→ Task 6 Step 7 + `maybeApplyOTA` ✓
- §5.4 OTA 执行器 → Task 4/5/6 ✓
- §6 规范化消息(域分隔 + version + sha256)+ 防降级 → Task 1 `SignedMessage` + Task 4 `compareVersions`/`selectOTATask` ✓
- §7 自更新流程 + 回滚(机制 A:原子替换/marker/重启/健康确认/`.bak`/ExecStartPre)→ Task 5 + Task 6 + Task 7 守护脚本 ✓
- §8 fail-closed 错误处理(替换前中止、未配公钥忽略、回滚)→ `runOTA`/`maybeApplyOTA`/`otaWatchdog` ✓
- §9 安全(私钥隔离/公钥固定/绑定/防降级/轮换/fail-closed)→ Task 1/6/7 ✓
- §10 测试 → Task 1/2/3/4/5/6 单测 + Task 7 真机步骤 ✓
- §11 改动文件清单 → 全部覆盖 ✓
- §12 待确认小项(版本变量/`Restart=always`)→ Task 6 Step 7 注入 version,Task 7 unit 含 `Restart=always` ✓

**2. 占位符扫描**:无 TODO/TBD;每个代码步骤含完整代码与确切命令/预期输出。Task 4 的占位 `var _ = otasig.Domain` 已在 Task 6 Step 3 显式删除。

**3. 类型一致性**:`otaTask`/`otaMarker` 字段在 Task 4/5/6 一致;`otasig.VerifyArtifact(keys, version, sha256Hex, sigHex, data)` 签名在 Task 1 定义、Task 3 测试、Task 6 调用三处一致;状态串统一为 `dispatching|succeeded|failed`(对齐服务端枚举与 `/gateway/ota/report` 校验)。

**4. 歧义检查**:健康 = 首次 post-restart `pullConfig` 成功;回滚阈值 attempts≥3(守护脚本)/ 看门狗 90s;状态映射(回滚→failed+error_message)已显式说明。

---

## 执行交接

计划已就绪。两种执行方式:

1. **Subagent-Driven(推荐)** — 每个 Task 派发新 subagent,任务间审查,迭代快。REQUIRED SUB-SKILL: superpowers:subagent-driven-development。
2. **Inline Execution** — 本会话内按 executing-plans 批量执行 + 检查点。REQUIRED SUB-SKILL: superpowers:executing-plans。
