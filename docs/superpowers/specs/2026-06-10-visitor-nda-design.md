# Visitor NDA — Design

> Date: 2026-06-10
> Status: approved
> Source: docs/kisi-gap-analysis.md §2.2/§4 — Kisi visitor management includes NDA
> signing at check-in (self-built lightweight e-sign). Strategy doc: 自研 — NDA
> 模板 + 签名 + 归档进审计链.

## 1. Goal

Tenant-level NDA for visitors: admins manage an NDA template and a "required"
flag; a guest signs (name + signature image) against the current template
version; when required, an unsigned guest cannot be checked in. Signing is
recorded on the guest and archived in the tamper-evident audit log.

Non-goals (v1): kiosk PWA UI (future feature consumes these endpoints), public
unauthenticated signing token, PDF rendering of the signed document, retaining
the raw signature blob in external storage (bounded inline storage v1; the
signed-upload infra is the future home for full documents).

## 2. Data model

NDA template — httpx-level tenant store (mirrors the incident-policy override
store): `visitorNDATemplate { TenantID, Title, Body, Version int, Required bool,
UpdatedAt }`. Persisted under state key `module_visitor_nda`. Default when unset:
empty template, `Version 0`, `Required false`. `PUT` bumps `Version` only when
Title/Body change (toggling Required alone does not re-version).

Guest (access service) gains:
- `NDASignedAt string` (RFC3339, empty = unsigned)
- `NDASignerName string`
- `NDATemplateVersion int` (version signed)
- `NDASignatureDataURL string` (inline `data:image/png;base64,...`, capped 64KB —
  signature-pad PNGs are a few KB; cap keeps state snapshots bounded)
- `NDASignatureHash string` (SHA-256 hex of the submitted data URL — survives any
  future blob offload and is what the audit entry records)

New service method `SignGuestNDA(tenantID, guestID string, input GuestNDAInput)
(Guest, error)`; `GuestNDAInput { SignerName, SignatureDataURL string,
TemplateVersion int }`. Validation: guest exists in tenant; signer name required;
data URL must start `data:image/` and be ≤ 64KB; sets the five fields;
`persistLocked`. Re-signing overwrites (latest signature wins).

## 3. Endpoints

- `GET /api/v1/visitor-nda/template?tenant_id=` — roles: super_admin,
  tenant_admin, operator, building_admin. Returns current template (default when
  unset).
- `PUT /api/v1/visitor-nda/template` — roles: super_admin, tenant_admin. Body
  `{tenant_id, title, body, required}` (all optional except tenant scope; partial
  update semantics: only provided fields change). Audit `visitor_nda_template_updated`.
- `POST /api/v1/guests/{guestID}/nda/sign` — roles: super_admin, tenant_admin,
  building_admin (same as guest mutation). Body `{tenant_id, signer_name,
  signature_data_url}`. Signs against the CURRENT template version; 409 if no
  template configured (Version 0). Audit `guest_nda_signed` with
  `template_version` + `signature_hash`. Returns the updated guest.

## 4. Enforcement

When the tenant template has `Required == true` and the guest has
`NDASignedAt == ""`, transitioning a guest to `checked_in` is rejected with 409
`{"error":"nda_required"}`. Enforced in both check-in handlers
(`updateGuestStatus` reference route and `appAdminUpdateGuestStatus` mobile
route) via a shared helper `guestNDACheckInBlocked(tenantID, guestID) bool`
evaluated only for the `checked_in` target status. Other transitions
(checked_out, cancelled, expected) are never blocked.

## 5. Testing (TDD)

- Template: GET default (version 0, required false); PUT title/body → version 1
  and fields round-trip; PUT required-only toggle keeps version; restart with the
  same state store restores the template.
- Sign: 200 sets signed_at/signer/version/hash and returns them on GET guest;
  unknown guest 404; missing signer or signature 400; oversize data URL 400;
  non-image data URL 400; sign with no template configured 409.
- Enforcement: required template + unsigned guest → check-in 409 nda_required on
  BOTH routes; after signing → check-in 200; required=false → unsigned check-in
  200; checked_out/cancelled transitions unaffected by required flag.

## 6. Out of scope / future
Kiosk PWA signing UI; public signing token for unattended kiosks; storing the
rendered signed document via the signed-upload blob store; per-place templates;
multi-language template bodies.
