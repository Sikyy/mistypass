# Badge Printing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Generate printable employee ID badges (single or batch by place/group) as PDF, each with a QR that scans to a public verification endpoint.

**Architecture:** Extend the `pdfgen` package with a standalone badge template + render functions (kept out of the report-type machinery). Two HTTP endpoints: a role-gated `GET /api/v1/badges/export` and a public, rate-limited `GET /api/v1/badges/verify`. The QR encodes a verify URL carrying an HMAC-signed badge token (reuses `JWTSecret`).

**Tech Stack:** Go 1.25, chi router, existing `pdfgen` (Gotenberg HTML→PDF), new dep `github.com/skip2/go-qrcode`.

**Spec:** `docs/superpowers/specs/2026-06-10-badge-printing-design.md`

## File Structure

- Create: `api/internal/pdfgen/qr.go` — `EncodeQRPNGBase64(content)` (QR lib choke point).
- Create: `api/internal/pdfgen/qr_test.go`
- Create: `api/internal/pdfgen/badge.go` — `Badge`, `BadgeDoc`, `RenderBadgesHTML`, `RenderBadgesPDF`.
- Create: `api/internal/pdfgen/badge_test.go`
- Create: `api/internal/pdfgen/templates/badge.html` — standalone badge template (auto-embedded by existing `//go:embed templates/*.html`).
- Modify: `api/internal/pdfgen/renderer.go` — parse + hold the badge template.
- Modify: `api/internal/config/config.go` — add `BadgeVerifyBaseURL`.
- Modify: `api/internal/config/config_test.go` — cover the new config default/override.
- Create: `api/internal/http/routes_badges.go` — token sign/verify, `exportBadges`, `verifyBadge`.
- Create: `api/internal/http/routes_badges_test.go`
- Modify: `api/internal/http/router.go` — register both routes.

---

### Task 1: QR encoder in pdfgen

**Files:**
- Create: `api/internal/pdfgen/qr.go`
- Create: `api/internal/pdfgen/qr_test.go`
- Modify: `api/go.mod`, `api/go.sum` (via `go get`)

- [ ] **Step 1: Add the QR dependency**

Run: `cd api && go get github.com/skip2/go-qrcode@v0.0.0-20200617195104-da1b6568686e`
Expected: go.mod/go.sum updated. If this fails (no network), STOP and switch to the JS-in-template fallback from the spec §3.2 before continuing — note it in the task and ask.

- [ ] **Step 2: Write the failing test**

```go
package pdfgen

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncodeQRPNGBase64(t *testing.T) {
	out, err := EncodeQRPNGBase64("https://example.com/api/v1/badges/verify?token=abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(out)
	if err != nil {
		t.Fatalf("output is not valid base64: %v", err)
	}
	if len(raw) < 8 || string(raw[1:4]) != "PNG" {
		t.Fatalf("expected PNG bytes, got %d bytes prefix=%q", len(raw), raw[:min(8, len(raw))])
	}
}

func TestEncodeQRPNGBase64RejectsEmpty(t *testing.T) {
	if _, err := EncodeQRPNGBase64("   "); err == nil {
		t.Fatal("expected error for empty content")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd api && go test ./internal/pdfgen/ -run TestEncodeQRPNGBase64 -count=1`
Expected: FAIL — `EncodeQRPNGBase64` undefined.

- [ ] **Step 4: Implement**

Create `api/internal/pdfgen/qr.go`:
```go
package pdfgen

import (
	"encoding/base64"
	"errors"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// EncodeQRPNGBase64 renders content as a QR code PNG and returns it
// base64-encoded for direct embedding in an <img src="data:image/png;base64,...">.
func EncodeQRPNGBase64(content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", errors.New("qr content must not be empty")
	}
	png, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(png), nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd api && go test ./internal/pdfgen/ -run TestEncodeQRPNGBase64 -count=1`
Expected: PASS (both tests).

- [ ] **Step 6: Commit**

```bash
cd api && git add go.mod go.sum internal/pdfgen/qr.go internal/pdfgen/qr_test.go
git commit -m "feat(pdfgen): add QR PNG encoder for badges"
```

---

### Task 2: Badge types, template, and renderer

**Files:**
- Create: `api/internal/pdfgen/badge.go`
- Create: `api/internal/pdfgen/templates/badge.html`
- Modify: `api/internal/pdfgen/renderer.go` (add badge template field + parse)
- Create: `api/internal/pdfgen/badge_test.go`

- [ ] **Step 1: Create the badge template**

Create `api/internal/pdfgen/templates/badge.html`:
```html
{{define "badge"}}<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><style>
  @page { size: 54mm 86mm; margin: 0; }
  * { box-sizing: border-box; }
  body { margin: 0; font-family: -apple-system, "Segoe UI", Roboto, sans-serif; color: #141510; }
  .badge { width: 54mm; height: 86mm; padding: 6mm 5mm; display: flex; flex-direction: column;
           page-break-after: always; border: 0.3mm solid #e8dfd1; }
  .badge:last-child { page-break-after: auto; }
  .org { display: flex; align-items: center; gap: 2mm; border-bottom: 0.3mm solid #e8dfd1; padding-bottom: 3mm; }
  .org img { height: 8mm; }
  .org .name { font-size: 9pt; font-weight: 700; color: #4A69FF; }
  .holder { flex: 1; display: flex; flex-direction: column; justify-content: center; }
  .holder .full-name { font-size: 16pt; font-weight: 800; line-height: 1.1; }
  .holder .role { font-size: 10pt; color: #7c7568; margin-top: 1mm; }
  .holder .where { font-size: 8pt; color: #9a9486; margin-top: 2mm; }
  .status { display: inline-block; margin-top: 2mm; padding: 0.5mm 2mm; border-radius: 2mm;
            font-size: 7pt; font-weight: 700; text-transform: uppercase; background: #eef1ff; color: #4A69FF; }
  .qr { display: flex; align-items: center; gap: 2mm; border-top: 0.3mm solid #e8dfd1; padding-top: 3mm; }
  .qr img { width: 18mm; height: 18mm; }
  .qr .cap { font-size: 7pt; color: #9a9486; }
</style></head><body>
{{range .Badges}}
  <div class="badge">
    <div class="org">{{if $.LogoBase64}}<img src="data:image/png;base64,{{$.LogoBase64}}" alt="">{{end}}<span class="name">{{$.Organization}}</span></div>
    <div class="holder">
      <div class="full-name">{{.Name}}</div>
      <div class="role">{{.Role}}</div>
      <div class="where">{{.Building}}</div>
      <span class="status">{{.Status}}</span>
    </div>
    <div class="qr"><img src="data:image/png;base64,{{.QRBase64}}" alt="QR"><span class="cap">Scan to verify</span></div>
  </div>
{{end}}
</body></html>{{end}}
```

- [ ] **Step 2: Write the failing test**

Create `api/internal/pdfgen/badge_test.go`:
```go
package pdfgen

import (
	"strings"
	"testing"
)

func TestRenderBadgesHTML(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	doc := BadgeDoc{
		Organization: "Acme Jakarta",
		Badges: []Badge{
			{Name: "Andri Pratama", Role: "operator", Building: "HQ Tower", Status: "active", QRBase64: "AAAA"},
			{Name: "Siky", Role: "tenant_admin", Building: "HQ Tower", Status: "active", QRBase64: "BBBB"},
			{Name: "Rina", Role: "operator", Building: "HQ Tower", Status: "suspended", QRBase64: "CCCC"},
		},
	}
	out, err := r.RenderBadgesHTML(doc)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{"Andri Pratama", "Siky", "Rina", "tenant_admin", "suspended", "Acme Jakarta", "Scan to verify"} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected html to contain %q", want)
		}
	}
	if n := strings.Count(html, "data:image/png;base64,BBBB"); n != 1 {
		t.Fatalf("expected each badge QR embedded once, got %d for BBBB", n)
	}
	if n := strings.Count(html, `class="badge"`); n != 3 {
		t.Fatalf("expected 3 badge cards, got %d", n)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd api && go test ./internal/pdfgen/ -run TestRenderBadgesHTML -count=1`
Expected: FAIL — `BadgeDoc`/`RenderBadgesHTML` undefined.

- [ ] **Step 4: Add the badge template field to the Renderer**

In `api/internal/pdfgen/renderer.go`, add a field to the `Renderer` struct (after `templates map[string]*template.Template`):
```go
	badgeTemplate *template.Template
```
In `NewRenderer()`, after the `for rt := range validReportTypes { ... }` loop and before `return &Renderer{...}`, add:
```go
	badgeTmpl, err := template.ParseFS(templateFS, "templates/badge.html")
	if err != nil {
		return nil, fmt.Errorf("parse badge template: %w", err)
	}
```
Then add `badgeTemplate: badgeTmpl,` to the returned `&Renderer{...}` literal.

- [ ] **Step 5: Implement badge.go**

Create `api/internal/pdfgen/badge.go`:
```go
package pdfgen

import (
	"bytes"
	"fmt"
)

// Badge is one printable ID badge.
type Badge struct {
	Name         string
	Role         string
	Building     string
	Status       string
	QRBase64     string
}

// BadgeDoc is a set of badges sharing an organization header.
type BadgeDoc struct {
	Organization string
	LogoBase64   string
	Badges       []Badge
}

// RenderBadgesHTML renders the badge document to standalone HTML.
func (r *Renderer) RenderBadgesHTML(doc BadgeDoc) ([]byte, error) {
	if doc.LogoBase64 == "" {
		doc.LogoBase64 = r.logoBase64
	}
	var buf bytes.Buffer
	if err := r.badgeTemplate.ExecuteTemplate(&buf, "badge", doc); err != nil {
		return nil, fmt.Errorf("execute badge template: %w", err)
	}
	return buf.Bytes(), nil
}

// RenderBadgesPDF renders the badge document to PDF via Gotenberg.
func (r *Renderer) RenderBadgesPDF(client *GotenbergClient, doc BadgeDoc) ([]byte, error) {
	html, err := r.RenderBadgesHTML(doc)
	if err != nil {
		return nil, err
	}
	return client.ConvertHTML(html, DefaultPDFOptions())
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `cd api && go test ./internal/pdfgen/ -run TestRenderBadgesHTML -count=1`
Expected: PASS.

- [ ] **Step 7: Run the whole pdfgen package to confirm no regression**

Run: `cd api && go test ./internal/pdfgen/ -count=1`
Expected: ok (all existing report tests still pass — the badge template is additive).

- [ ] **Step 8: Commit**

```bash
cd api && git add internal/pdfgen/badge.go internal/pdfgen/badge_test.go internal/pdfgen/templates/badge.html internal/pdfgen/renderer.go
git commit -m "feat(pdfgen): render employee ID badges to HTML/PDF"
```

---

### Task 3: BadgeVerifyBaseURL config

**Files:**
- Modify: `api/internal/config/config.go`
- Modify: `api/internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

In `api/internal/config/config_test.go`, find the test that asserts `GotenbergURL` defaults (search `GotenbergURL != "http://localhost:3000"`). In that same default-asserting test, add after the GotenbergURL default check:
```go
	if cfg.BadgeVerifyBaseURL != "" {
		t.Fatalf("default badge verify base url should be empty, got %q", cfg.BadgeVerifyBaseURL)
	}
```
And in the override-asserting test (search `GotenbergURL != "http://gotenberg:3000"`), set the env before load (mirror how that test sets `GOTENBERG_URL`) with `t.Setenv("BADGE_VERIFY_BASE_URL", "https://id.example.com")` and assert:
```go
	if cfg.BadgeVerifyBaseURL != "https://id.example.com" {
		t.Fatalf("badge verify base url override mismatch: got %q", cfg.BadgeVerifyBaseURL)
	}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd api && go test ./internal/config/ -count=1`
Expected: FAIL — `cfg.BadgeVerifyBaseURL` undefined.

- [ ] **Step 3: Implement**

In `api/internal/config/config.go`, add a struct field near `GotenbergURL`:
```go
	BadgeVerifyBaseURL                                           string
```
And near `cfg.GotenbergURL = envStringOrDefault("GOTENBERG_URL", "http://localhost:3000")` add:
```go
	cfg.BadgeVerifyBaseURL = envStringOrDefault("BADGE_VERIFY_BASE_URL", "")
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd api && go test ./internal/config/ -count=1`
Expected: ok.

- [ ] **Step 5: Commit**

```bash
cd api && git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add BADGE_VERIFY_BASE_URL"
```

---

### Task 4: Badge token sign/verify + verify handler

**Files:**
- Create: `api/internal/http/routes_badges.go`
- Create: `api/internal/http/routes_badges_test.go`
- Modify: `api/internal/http/router.go` (register verify route)

- [ ] **Step 1: Write the failing test (token round-trip + verify endpoint)**

Create `api/internal/http/routes_badges_test.go`:
```go
package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func badgeTestRouter(t *testing.T) (http.Handler, *server) {
	t.Helper()
	handler, _, err := NewRouter(config.Config{
		JWTSecret:       "badge-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	return handler, nil
}

func TestBadgeTokenRoundTrip(t *testing.T) {
	s := &server{cfg: config.Config{JWTSecret: "badge-secret"}}
	token := s.signBadgeToken("tenant_demo_jakarta", "usr_1001")
	tenantID, userID, ok := s.parseBadgeToken(token)
	if !ok || tenantID != "tenant_demo_jakarta" || userID != "usr_1001" {
		t.Fatalf("round trip failed: tenant=%q user=%q ok=%v", tenantID, userID, ok)
	}
	if _, _, ok := s.parseBadgeToken(token + "x"); ok {
		t.Fatal("tampered token must not verify")
	}
	if _, _, ok := s.parseBadgeToken("garbage"); ok {
		t.Fatal("garbage token must not verify")
	}
}

func TestVerifyBadgeEndpoint(t *testing.T) {
	handler, _ := badgeTestRouter(t)
	signer := &server{cfg: config.Config{JWTSecret: "badge-test-secret"}}
	token := signer.signBadgeToken("tenant_demo_jakarta", "usr_1001")

	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/badges/verify?token="+token, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Valid        bool   `json:"valid"`
		Name         string `json:"name"`
		Status       string `json:"status"`
		Organization string `json:"organization"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Valid || resp.Name == "" || resp.Status == "" {
		t.Fatalf("expected valid badge with name+status, got %+v body=%s", resp, rec.Body.String())
	}

	bad := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/badges/verify?token="+token+"x", "", nil)
	var badResp struct{ Valid bool `json:"valid"` }
	_ = json.Unmarshal(bad.Body.Bytes(), &badResp)
	if bad.Code != http.StatusOK || badResp.Valid {
		t.Fatalf("expected 200 valid:false for tampered token, got %d body=%s", bad.Code, bad.Body.String())
	}

	missing := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/badges/verify", "", nil)
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing token, got %d", missing.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd api && go test ./internal/http/ -run "TestBadgeTokenRoundTrip|TestVerifyBadgeEndpoint" -count=1`
Expected: FAIL — `signBadgeToken`/`parseBadgeToken`/route undefined.

- [ ] **Step 3: Implement token helpers + verify handler**

Create `api/internal/http/routes_badges.go`:
```go
package httpx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

const badgeTokenPrefix = "MPB1"

func (s *server) badgeSigningKey() []byte {
	return []byte(s.cfg.JWTSecret)
}

// signBadgeToken returns base64url(payload) + "." + base64url(hmac[:16]).
func (s *server) signBadgeToken(tenantID, userID string) string {
	payload := strings.Join([]string{badgeTokenPrefix, tenantID, userID, time.Now().UTC().Format("20060102")}, ".")
	mac := hmac.New(sha256.New, s.badgeSigningKey())
	mac.Write([]byte(payload))
	sig := mac.Sum(nil)[:16]
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func (s *server) parseBadgeToken(token string) (tenantID, userID string, ok bool) {
	idx := strings.LastIndex(token, ".")
	if idx <= 0 || idx == len(token)-1 {
		return "", "", false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(token[:idx])
	if err != nil {
		return "", "", false
	}
	gotSig, err := base64.RawURLEncoding.DecodeString(token[idx+1:])
	if err != nil {
		return "", "", false
	}
	mac := hmac.New(sha256.New, s.badgeSigningKey())
	mac.Write(payloadBytes)
	if !hmac.Equal(gotSig, mac.Sum(nil)[:16]) {
		return "", "", false
	}
	parts := strings.Split(string(payloadBytes), ".")
	if len(parts) != 4 || parts[0] != badgeTokenPrefix {
		return "", "", false
	}
	return parts[1], parts[2], true
}

// GET /api/v1/badges/verify?token= — public, rate-limited. The QR target.
func (s *server) verifyBadge(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}
	tenantID, userID, ok := s.parseBadgeToken(token)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false})
		return
	}
	user, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"valid":        true,
		"name":         user.Name,
		"role":         user.Role,
		"organization": s.badgeOrganizationName(tenantID),
		"status":       user.Status,
	})
}

func (s *server) badgeOrganizationName(tenantID string) string {
	if name := strings.TrimSpace(s.accessSvc.GetOrganizationSettings(tenantID).Name); name != "" {
		return name
	}
	return s.resolveExportTenantName(tenantID)
}
```

- [ ] **Step 4: Register the verify route**

In `api/internal/http/router.go`, find the public route group that registers `/organizations/find` (search `s.findOrganizations`). Add directly below it, in the same `r.With(s.withEnterprisePublicRateLimit)...` style:
```go
		r.With(s.withEnterprisePublicRateLimit).Get("/badges/verify", s.verifyBadge)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd api && go test ./internal/http/ -run "TestBadgeTokenRoundTrip|TestVerifyBadgeEndpoint" -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd api && git add internal/http/routes_badges.go internal/http/routes_badges_test.go internal/http/router.go
git commit -m "feat(badges): signed badge token + public verify endpoint"
```

---

### Task 5: Export handler + member assembly + route

**Files:**
- Modify: `api/internal/http/routes_badges.go` (add `exportBadges` + helpers)
- Modify: `api/internal/http/routes_badges_test.go` (add export tests)
- Modify: `api/internal/http/router.go` (register export route)

- [ ] **Step 1: Write the failing tests**

Append to `api/internal/http/routes_badges_test.go`:
```go
func TestExportBadgesSingleUserHTML(t *testing.T) {
	handler, _ := badgeTestRouter(t)
	token := referenceAPILogin(t, handler, "organization.admin@mistypass.local")
	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/badges/export?user_id=usr_1001&format=html", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("expected html content-type, got %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Andri Pratama") || !strings.Contains(body, "Scan to verify") {
		t.Fatalf("expected badge html for usr_1001, body=%s", body[:min(400, len(body))])
	}
}

func TestExportBadgesBatchByPlaceHTML(t *testing.T) {
	handler, _ := badgeTestRouter(t)
	token := referenceAPILogin(t, handler, "organization.admin@mistypass.local")
	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/badges/export?place_id=building_demo_001&format=html", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if n := strings.Count(rec.Body.String(), `class="badge"`); n < 2 {
		t.Fatalf("expected multiple badges for building_demo_001, got %d", n)
	}
}

func TestExportBadgesBatchByGroupHTML(t *testing.T) {
	handler, _ := badgeTestRouter(t)
	token := referenceAPILogin(t, handler, "organization.admin@mistypass.local")
	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/badges/export?group_id=ug_common_office_jkt&format=html", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if n := strings.Count(rec.Body.String(), `class="badge"`); n < 2 {
		t.Fatalf("expected multiple badges for group, got %d", n)
	}
}

func TestExportBadgesCrossTenantUser404(t *testing.T) {
	handler, _ := badgeTestRouter(t)
	token := referenceAPILogin(t, handler, "organization.admin@mistypass.local")
	// usr_1002 belongs to tenant_demo_factory.
	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/badges/export?user_id=usr_1002&format=html", token, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-tenant user, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestExportBadgesEmptyGroup400(t *testing.T) {
	handler, _ := badgeTestRouter(t)
	token := referenceAPILogin(t, handler, "organization.admin@mistypass.local")
	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/badges/export?group_id=ug_does_not_exist&format=html", token, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty group, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestExportBadgesRequiresScopeSelector(t *testing.T) {
	handler, _ := badgeTestRouter(t)
	token := referenceAPILogin(t, handler, "organization.admin@mistypass.local")
	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/badges/export?format=html", token, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 with no scope selector, got %d", rec.Code)
	}
}

func TestExportBadgesRequiresAuth(t *testing.T) {
	handler, _ := badgeTestRouter(t)
	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/badges/export?user_id=usr_1001&format=html", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without bearer, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/http/ -run "TestExportBadges" -count=1`
Expected: FAIL — route not registered (404) / `exportBadges` undefined.

- [ ] **Step 3: Implement export handler + assembly**

Append to `api/internal/http/routes_badges.go` (add `"github.com/mistypass/cloud/api/internal/modules/access"`, `"github.com/mistypass/cloud/api/internal/pdfgen"`, `"fmt"`, `"strconv"` to the import block):
```go
// GET /api/v1/badges/export?user_id=|place_id=|group_id=&format=pdf|html
func (s *server) exportBadges(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	placeID := strings.TrimSpace(r.URL.Query().Get("place_id"))
	groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
	selectors := 0
	for _, v := range []string{userID, placeID, groupID} {
		if v != "" {
			selectors++
		}
	}
	if selectors != 1 {
		writeError(w, http.StatusBadRequest, "exactly one of user_id, place_id, group_id is required")
		return
	}

	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "pdf"
	}
	if format != "pdf" && format != "html" {
		writeError(w, http.StatusBadRequest, "format must be pdf or html")
		return
	}

	var members []access.AccessUser
	var scopeLabel string
	switch {
	case userID != "":
		user, err := s.accessSvc.GetUser(tenantID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		members = []access.AccessUser{user}
		scopeLabel = "user"
	case placeID != "":
		if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
			writeError(w, http.StatusNotFound, "place not found")
			return
		}
		for _, u := range s.accessSvc.ListUsers(tenantID) {
			if u.BuildingID == placeID {
				members = append(members, u)
			}
		}
		scopeLabel = "place"
	default:
		for _, u := range s.accessSvc.ListUsers(tenantID) {
			for _, g := range u.GroupIDs {
				if g == groupID {
					members = append(members, u)
					break
				}
			}
		}
		scopeLabel = "group"
	}

	if len(members) == 0 {
		writeError(w, http.StatusBadRequest, "no users match the requested scope")
		return
	}

	orgName := s.badgeOrganizationName(tenantID)
	doc := pdfgen.BadgeDoc{Organization: orgName}
	for _, u := range members {
		qrBase64, err := pdfgen.EncodeQRPNGBase64(s.badgeVerifyURL(tenantID, s.signBadgeToken(tenantID, u.ID)))
		if err != nil {
			s.logger.Error("badge qr encode failed", "error", err, "user_id", u.ID)
			writeError(w, http.StatusInternalServerError, "failed to render badge QR")
			return
		}
		doc.Badges = append(doc.Badges, pdfgen.Badge{
			Name:     firstNonEmptyString(u.Name, u.Email, u.ID),
			Role:     u.Role,
			Building: u.BuildingID,
			Status:   u.Status,
			QRBase64: qrBase64,
		})
	}

	if format == "html" {
		html, err := s.pdfRenderer.RenderBadgesHTML(doc)
		if err != nil {
			s.logger.Error("badge html render failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to render badges")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(html)
		return
	}

	pdfBytes, err := s.pdfRenderer.RenderBadgesPDF(s.gotenbergClient, doc)
	if err != nil {
		s.logger.Error("badge pdf render failed", "error", err)
		writeError(w, http.StatusBadGateway, "PDF rendering failed: "+err.Error())
		return
	}
	filename := fmt.Sprintf("badges_%s_%s.pdf", scopeLabel, time.Now().UTC().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
	w.Write(pdfBytes)
}

// badgeVerifyURL builds the QR target URL. Base resolution: configured base URL,
// else the tenant's primary domain, else a relative path.
func (s *server) badgeVerifyURL(tenantID, token string) string {
	base := strings.TrimRight(strings.TrimSpace(s.cfg.BadgeVerifyBaseURL), "/")
	if base == "" {
		if domain := strings.TrimSpace(s.accessSvc.GetOrganizationSettings(tenantID).PrimaryDomain); domain != "" {
			base = "https://" + domain
		}
	}
	return base + "/api/v1/badges/verify?token=" + token
}
```

- [ ] **Step 4: Register the export route**

In `api/internal/http/router.go`, find the protected `GET /reports/export` registration (search `s.exportReport`). Add directly below it:
```go
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/badges/export", s.exportBadges)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd api && go test ./internal/http/ -run "TestExportBadges|TestVerifyBadgeEndpoint|TestBadgeTokenRoundTrip" -count=1`
Expected: PASS (all).

- [ ] **Step 6: Commit**

```bash
cd api && git add internal/http/routes_badges.go internal/http/routes_badges_test.go internal/http/router.go
git commit -m "feat(badges): export single/batch badges as PDF/HTML"
```

---

### Task 6: Full verification + optional OpenAPI doc

**Files:** none new (verification) + optional `api/internal/http/routes_openapi.go`

- [ ] **Step 1: Build + vet**

Run: `cd api && go build ./... && go vet ./...`
Expected: clean.

- [ ] **Step 2: Run the affected packages**

Run: `cd api && go test ./internal/pdfgen/ ./internal/config/ ./internal/http/ -count=1`
Expected: ok for all three.

- [ ] **Step 3: (Optional) Document the endpoints in OpenAPI**

The OpenAPI spec is hand-assembled in `api/internal/http/routes_openapi.go` and is NOT enforced over every route (the coverage test only spot-checks specific operations), so this is additive polish, not a gate. If time permits, add `/api/v1/badges/export` and `/api/v1/badges/verify` entries mirroring an existing simple GET (e.g. follow how `/reports/export` is described). Skip if the existing file's structure makes a clean addition non-trivial — note the skip.

- [ ] **Step 4: Final commit (if OpenAPI updated)**

```bash
cd api && git add internal/http/routes_openapi.go
git commit -m "docs(openapi): document badge export/verify endpoints"
```

---

## Self-Review

**Spec coverage:**
- §3 architecture (extend pdfgen, two endpoints, QR→verify URL) → Tasks 1,2,4,5. ✓
- §3.2 QR library + fallback → Task 1 Step 1. ✓
- §4.1 export contract (user/place/group, format, roles, 404/400/502) → Task 5. ✓
- §4.2 verify contract (public, rate-limited, valid:false on bad) → Task 4. ✓
- §5 token format → Task 4 Step 3. ✓
- §6 QR content / base URL resolution → Task 5 `badgeVerifyURL`. ✓
- §7 badge layout → Task 2 badge.html. ✓
- §8 testing matrix → Tasks 1,2,4,5 tests. ✓ (PDF path asserted via html to avoid live Gotenberg, per spec §8.)
- §9 out of scope (photos etc.) → not implemented. ✓

**Placeholder scan:** No TBD/TODO; all code blocks complete. ✓

**Type consistency:** `BadgeDoc{Organization, LogoBase64, Badges}` and `Badge{Name, Role, Building, Status, QRBase64}` used identically in Task 2 (def + test) and Task 5 (construction). `signBadgeToken`/`parseBadgeToken`/`badgeOrganizationName`/`badgeVerifyURL`/`verifyBadge`/`exportBadges` names consistent across Tasks 4–5. `EncodeQRPNGBase64` consistent Task 1↔5. ✓

**Note for executor:** `min()` is a Go 1.21+ builtin (module is Go 1.25) — used in two tests. `access.AccessUser`, `s.spaceSvc.GetBuilding`, `s.accessSvc.ListUsers/GetUser/GetOrganizationSettings`, `s.resolveTenantID`, `firstNonEmptyString`, `s.resolveExportTenantName` are all existing symbols verified during planning.
