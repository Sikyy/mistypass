# Visitor Kiosk (Self Check-In) — Design

> Date: 2026-06-11
> Status: implemented (this doc records the design as built)
> Source: docs/kisi-gap-analysis.md §2.3 — Kisi Kiosk (software + Kiosk Pro
> hardware). Software-first route per §4: kiosk PWA page + commodity tablet.

## 1. Goal

A full-screen visitor self check-in experience inside web-admin, run on a tablet
left signed in by front-desk staff. Consumes only existing backend capability
(guests CRUD, visitor NDA template/signing, check-in enforcement) — no new API.

## 2. Flow

`/kiosk` (authenticated, rendered WITHOUT the admin shell — special-cased in
App.tsx like the access-link claim route):

1. **Welcome** — "I have an appointment" / "Walk-in registration". Auto-returns
   here 8s after a completed check-in.
2. **Find** — expected guests (status `expected`), searchable by name / phone /
   company / host (`filterExpectedGuests`, 30s refetch). Tap a guest to proceed.
3. **Walk-in** — name / phone / host (required) + company / purpose; creates the
   guest with `notify_host: true`.
4. **NDA** — shown when a template is configured (`version > 0`,
   `ndaStepNeeded`): template body, signer name (prefilled), canvas signature
   pad. Required template → no skip button (server enforces 409 nda_required
   anyway); optional template → skip allowed. Signing posts the
   `data:image/png` signature then proceeds.
5. **Done** — success + host-notified note, auto-reset.

Entry point: "Kiosk Mode" button on the visitors page. Exit button returns to
`/visitors`.

## 3. Implementation notes

- `features/kiosk/pages/kiosk-page.tsx` (single page + `SignaturePad`).
  Signature pad uses pointer events; `setPointerCapture` is wrapped in
  try/catch — capture failure must not break signing (found via preview
  verification with synthetic pointers).
- Pure helpers + tests: `kiosk-page-utils.ts` (`filterExpectedGuests`,
  `ndaStepNeeded`).
- API client: `lib/api/visitor-nda.ts` (template get/update, sign) + Guest NDA
  fields on the shared type.
- i18n: `kiosk.*` (35 keys) in zh-CN / en-US / id-ID (parity-tested).

## 4. Verification

vitest 13 files / 73 tests green; tsc clean. Browser-verified end to end against
the demo backend: walk-in → check-in (no template configured), then template
PUT (required) → walk-in → NDA screen → signature → sign 200 → check-in 200,
confirmed via network log and backend guest state (`nda_signed_at`,
`nda_template_version`, `nda_signature_hash`, `status=checked_in`).

## 5. Out of scope / future

PWA manifest/service worker (installable kiosk); public unattended kiosk token
(today the tablet runs a staff session); badge printing from the kiosk; photo
capture; multi-place kiosk filtering (lists tenant-wide expected guests).
