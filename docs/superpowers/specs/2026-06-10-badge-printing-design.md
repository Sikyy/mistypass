# Badge Printing — Design

> Date: 2026-06-10
> Status: approved (pending spec review)
> Source: docs/CODE-REVIEW-2026-06-10.md / docs/kisi-gap-analysis.md — closes the "工牌打印 / Badge Printing" gap vs Kisi
> Author: Claude Code

## 1. Goal

Generate printable employee ID badges as PDF, for a single user or in bulk for a
place / group. Each badge carries the organization identity, the holder's name /
role / org / building / status, and a QR code that scans to a public verification
page confirming the holder's identity and employment status. This matches Kisi's
badge-printing feature (employee badge generation + PDF download), which Kisi
implements as a self-built software feature (no external service).

Non-goals for v1: holder photos (AccessUser has no avatar field — deferred to a
separate feature), badge template customization UI, physical-printer integration,
functional door-access QR (the QR is a visual identifier, not a credential).

## 2. Scope decisions (confirmed)

- **QR semantics:** visual identifier — scans to a verification page, NOT a door
  credential. No new credential lifecycle/revocation surface.
- **Generation scope:** single user OR batch by place / group (one endpoint).
- **Photo:** deferred. Badge is text + QR + org logo.
- **Verify endpoint:** included.

## 3. Architecture

Extend the existing `pdfgen` package with a dedicated badge template and render
path, separate from the report-type machinery (badges are not time-ranged
reports, so they must not enter `validReportTypes` / `ReportMeta`). Reuse the
Gotenberg HTML→PDF client and asset-embedding already in `pdfgen`.

```
GET /api/v1/badges/export ─┐
                           ├─ assemble BadgeData[] (access service, tenant-scoped)
                           ├─ pdfgen.RenderBadgesHTML(BadgeDoc) ── embeds QR PNGs (base64)
                           └─ gotenbergClient.ConvertHTML(...) ── multi-page PDF
GET /api/v1/badges/verify ── badge token → {valid,name,role,organization,status}
```

### 3.1 Components

- `pdfgen/badge.go` — `BadgeDoc`/`Badge` types, `RenderBadgesHTML(doc) ([]byte, error)`,
  `RenderBadgesPDF(client, doc) ([]byte, error)`. Owns the badge template
  (`pdfgen/templates/badge.html`, embedded via the existing `templateFS`).
- `pdfgen/qr.go` — `EncodeQRPNGBase64(content string) (string, error)` wrapping the
  QR library. Single choke point so the dependency is isolated and unit-testable.
- `internal/http/routes_badges.go` — `exportBadges` and `verifyBadge` handlers,
  badge-token sign/verify helpers.
- Badge token signing reuses `cfg.JWTSecret` (HMAC-SHA256); no new secret.

### 3.2 QR library

Add `github.com/skip2/go-qrcode` (pure Go, MIT). go.mod currently has no QR
encoder and the server only ever produced QR *token strings* (the mobile app
renders the image), so a server-side image encoder is genuinely new.
Fallback if `go get` is unavailable in the build environment: vendor a small JS
QR library into the template and let Gotenberg's headless Chromium render it
(the report templates already execute JS). Preference is the Go library for
deterministic, Go-unit-testable output.

## 4. Endpoint contracts

### 4.1 `GET /api/v1/badges/export`

Roles: `super_admin, tenant_admin, operator, building_admin` (mirrors
`/reports/export`). Tenant resolved from the token; never trusted from input.

Query params (exactly one scope selector required):
- `user_id` — single badge.
- `place_id` — all active users whose `BuildingID == place_id`.
- `group_id` — all active users whose `GroupIDs` contains `group_id`.
- `format` — `pdf` (default) or `html` (preview/testing).

Behavior:
- Resolve members tenant-scoped. `place_id`/`group_id` must belong to the tenant
  (reuse existing space/group lookups); otherwise 404.
- For `user_id`, the user must exist in the tenant; otherwise 404.
- Suspended/inactive users are included by default (badges are still printed for
  them), but their badge shows the real `status`. Empty member set → 400.
- `pdf`: `Content-Type: application/pdf`, `Content-Disposition: attachment;
  filename="badges_<scope>_<YYYY-MM-DD>.pdf"`. `html`: `text/html` body.
- Gotenberg unreachable / render error → 502.

### 4.2 `GET /api/v1/badges/verify`

Public (no bearer), rate-limited via `withEnterprisePublicRateLimit`. This is the
QR target.

- `token` (required) — the signed badge token.
- Valid token → 200 `{ "valid": true, "name", "role", "organization", "status" }`.
- Malformed / bad-signature / unknown user → 200 `{ "valid": false }` (do not
  distinguish reasons; avoid an enumeration oracle). Missing token → 400.
- Returns no information beyond what is already printed on the badge; `status`
  (active/suspended) is the useful live check.

## 5. Badge token

Compact, URL-safe, self-contained, long-lived (badges are physical):

```
token = base64url( "MPB1." + tenantID + "." + userID + "." + issuedYYYYMMDD ) + "." + sig
sig   = base64url( HMAC-SHA256( JWTSecret, payloadBeforeSig ) )[:16 bytes]
```

Verify recomputes the HMAC (constant-time compare), then loads the user by
`tenantID`+`userID` for the current name/role/status. No expiry in v1 (a future
revision can add a not-after or a per-tenant key version for revocation).

## 6. QR content / base URL

QR encodes a verification URL: `{base}/api/v1/badges/verify?token=<token>`.

`base` resolution order:
1. `BADGE_VERIFY_BASE_URL` env (new `envStringOrDefault`, default empty).
2. else the tenant's `primary_domain` (from organization settings), as `https://<domain>`.
3. else a relative path (`/api/v1/badges/verify?token=…`) — still valid for a
   reader pointed at the API host, just not a tappable absolute URL.

## 7. Badge layout (badge.html)

Conventional ID-card proportions, one badge per printed page (`page-break-after:
always`), print-friendly. Elements: org logo (existing embedded asset) + org
name header; holder name (large); role; building/org line; status chip; QR at
the lower corner; small "scan to verify" caption. Uses the same base64 asset
embedding as report templates. Tenant accent color from organization settings if
present, else the existing default.

## 8. Testing (TDD)

Unit (`pdfgen`):
- `EncodeQRPNGBase64` returns a non-empty base64 PNG for typical content; errors
  on empty content.
- `RenderBadgesHTML` for a 1-badge and a 3-badge doc: output contains each name,
  role, an `<img` QR tag per badge, and N-1 page breaks for N badges.

Unit/integration (`internal/http`):
- Badge token sign→verify round-trip; tampered token → invalid; unknown user →
  invalid.
- `exportBadges`: single user (200 pdf), batch by place (200, multi-member),
  cross-tenant `place_id`/`user_id` (404), empty set (400), `format=html` (200
  html), unauthenticated/insufficient role (401/403).
- `verifyBadge`: valid token (200 valid:true with status), tampered (200
  valid:false), missing token (400). Confirm the route is reachable without a
  bearer and is rate-limited (registered under the public-rate-limit group).

All via the existing `referenceAPILogin`/`referenceAPIRequest` harness and demo
users; Gotenberg-dependent assertions use `format=html` to avoid requiring a live
Gotenberg in unit tests (PDF path is exercised only where a renderer is present,
mirroring existing report-export tests).

## 9. Out of scope / future

Holder photos (needs avatar upload + AccessUser field); badge template/theme
customization; token expiry & per-tenant key rotation for revocation; a
front-end admin button (web-admin) to trigger export; functional door-access QR.
