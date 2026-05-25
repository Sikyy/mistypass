# MistyPass Repository Architecture

> Last updated: 2026-05-05
> Status: Reflects actual codebase as of Sprint 3 completion

## 1. Repository Layout

| Repo | GitHub | Description |
|---|---|---|
| `mistypass` | `Sikyy/mistypass` | Go API server + React web admin + gateway agent + docs |
| `ios-MistyisletPass` | `Sikyy/IOS-mistypass` | iOS resident app (SwiftUI, Secure Enclave, CoreBluetooth) |
| `android-MistyisletPass` | `Sikyy/Android-mistypass` | Android resident app (Compose, Keystore, Nordic BLE) |

No monorepo tooling. Each repo is independently buildable.

## 2. Backend (`mistypass/api/`)

**Stack:** Go 1.25, chi/v5 router, PostgreSQL, Redis, NATS JetStream, MQTT/EMQX

**Key packages:**
- `cmd/api/` — HTTP server entry point
- `cmd/gateway-agent/` — BLE reader + OSDP relay daemon
- `cmd/openapi-extract/` — Dumps runtime OpenAPI spec to static JSON
- `internal/http/` — chi routes, middleware, OpenAPI generation
- `internal/state/` — PostgreSQL store (sqlc for type-safe SQL)

**API structure:** All routes under `/api/v1/`. Mobile app endpoints nested under `/api/v1/app/`.

**Authentication:** JWT (access + refresh tokens). Mobile uses `/app/auth/login` and `/app/auth/refresh`. Admin uses `/auth/login`.

**Enterprise features:** SAML, OIDC, SCIM, HRIS integration, multi-tenant team hierarchy, WebAuthn.

## 3. Web Admin (`mistypass/web-admin/`)

**Stack:** Vite + React 18 + TypeScript, react-router-dom, TanStack Query + Table, Radix UI / shadcn, Tailwind CSS 4, Zustand, react-hook-form + Zod, i18next

**Testing:** Vitest + Playwright

## 4. iOS App (`ios-MistyisletPass/`)

**Stack:** SwiftUI, iOS 17+, Xcode 16, XcodeGen (`project.yml`)

**Key layers:**
- `Services/` — APIService (URLSession + JWT refresh), BLEManager (CoreBluetooth), SecureEnclaveService (EC P-256), KeychainService
- `ViewModels/` — @Observable view models with SwiftData offline cache
- `Views/` — SwiftUI screens (Doors, Scanner, Visitors, History, Profile)

**BLE:** CoreBluetooth with GATT challenge-response. UUIDs derived from ASCII (matching Go/Android). Reader identity characteristic read before challenge.

**Key format:** Secure Enclave P-256 key → X9.63 raw → prepend PKIX ASN.1 header → PEM. Backend receives standard PEM.

**Environment:** `APP_ENV` Info.plist key or env var → mock/staging/production base URL.

## 5. Android App (`android-MistyisletPass/`)

**Stack:** Kotlin, Jetpack Compose, Hilt DI, Room, Retrofit + kotlinx.serialization, Min SDK 26, Target 35, Java 17

**Key layers:**
- `core/ble/` — Nordic BLE Manager, KeystoreManager (EC P-256 PKIX/DER → PEM)
- `core/network/` — Retrofit + OkHttp with AuthInterceptor (JWT refresh)
- `ui/` — Compose screens + Navigation

**BLE:** Nordic BLE library with suspend-based API. MTU 256 for ECDSA signature payload. Reader identity read before challenge.

**Build variants:** debug (localhost), staging, release (with ProGuard).

**CI:** GitHub Actions — lint, unit test, build debug APK (`.github/workflows/ci.yml`).

## 6. BLE Protocol

Canonical source: `api/cmd/gateway-agent/ble_protocol.go`

**Service UUID:** `4D495354-5950-4153-532D-424C45415554` ("MISTYPASS-BLEAUT")

| Characteristic | UUID | Direction | Content |
|---|---|---|---|
| Challenge | `...4348414C4C4E` | Reader → Phone (Read) | 48B: [32B nonce][8B issued_at][8B expires_at] |
| Auth Response | `...415554485245` | Phone → Reader (Write) | [1B userId_len][userId][ECDSA signature] |
| Reader Identity | `...524541444552` | Reader → Phone (Read) | UTF-8 reader/lock ID string |
| Auth Result | `...524553554C54` | Reader → Phone (Notify) | [1B code][reason string] |

**Result codes:** 0x01 Granted, 0x02 Denied, 0x03 Expired, 0x04 Invalid Signature, 0x05 Unknown User, 0x06 Credential Expired

**Signing:** SHA256(nonce || userId) signed with ECDSA P-256. Go verifies both ASN.1 DER and raw r||s (64 bytes).

## 7. API Contract

OpenAPI 3.0.3 spec generated at runtime (`/api/v1/openapi.json`). Static extraction:

```bash
cd api && make openapi-mobile
```

Produces `docs/openapi-mobile.json` (128 `/app` paths / 154 mobile operations as of 2026-05-25). Route constant and client codegen targets:

```bash
make mobile-route-constants        # generated Swift/Kotlin typed route constants
make mobile-route-constants-check  # verify generated constants are current
make openapi-swift   # swift6 + async/await
make openapi-kotlin  # retrofit2 + kotlinx.serialization
```

`docs/testing/check-mobile-app-route-drift.zsh` also compares the generated Swift/Kotlin route copies in the iOS and Android app repositories against `docs/generated/mobile-routes/`, then scans app source for method/path drift and hand-written `/app/*` route literals.

### Mobile Endpoints (`/api/v1/app/`)

| Method | Path | Purpose |
|---|---|---|
| POST | `/app/auth/login` | Login |
| POST | `/app/auth/refresh` | Refresh tokens |
| GET | `/app/me` | User profile |
| GET | `/app/access/my-doors` | List accessible doors |
| POST | `/app/access/unlock` | Remote unlock (server-side) |
| POST | `/app/access/qr-unlock` | QR-based unlock |
| GET | `/app/access/ble-token` | Legacy BLE token |
| GET | `/app/access/logs` | Access event history |
| POST | `/app/visitor-passes` | Create visitor pass |
| GET | `/app/credentials` | List credentials |
| POST | `/app/credentials/register` | Register mobile keypair |
| GET | `/app/credentials/mobile` | List mobile credentials |
| DELETE | `/app/credentials/mobile/{id}` | Self-revoke credential |
| POST | `/app/credentials/mobile/{id}/refresh` | Refresh credential TTL |

## 8. Infrastructure

| Component | Purpose | Status |
|---|---|---|
| PostgreSQL + PgBouncer | Primary store + connection pooling | Production |
| Redis | Sessions, caching, rate limiting | Production |
| NATS JetStream | Internal event bus | Production |
| MQTT / EMQX | Gateway agent ↔ server communication | Production |
| OpenTelemetry | Traces + metrics | Production |
| Prometheus | Metrics collection | Production |
| GitHub Actions | CI for all repos | Production |

## 9. Phase 2 (Not Yet Implemented)

These items from the original architecture proposal are planned but not started:

- **Controller firmware repo** — ESP32/nRF BLE + OSDP relay (currently simulated via TCP)
- **Protobuf for BLE messages** — currently raw binary; may adopt if message complexity grows
- **OpenAPI codegen in CI** — tooling exists (`make openapi-swift`/`openapi-kotlin`), not yet wired into CI
- **End-to-end BLE integration tests** — blocked on hardware availability
