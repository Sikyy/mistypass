# Cross-Repo Architecture & Code Review — 2026-05-04

> Scope: Architecture document audit + code review of mistypass (web), ios-MistyisletPass, android-MistyisletPass
> Reviewer: Claude Code
> Goal: Ensure three repos can successfully integrate for BLE unlock and API communication

## Implementation Status (2026-05-04)

Sprint 1 + Sprint 2 fixes have been applied directly to all three repos:

| ID | Severity | Title | Status |
|---|---|---|---|
| P0-01 | Critical | iOS UUIDs realigned to Go/Android | DONE |
| P0-02 | Critical | Malformed UUIDs (Go + Android) truncated to valid 12-hex format | DONE |
| P0-03 | Critical | iOS API paths rewritten to match Go router | DONE |
| P0-04 | Critical | iOS public key export converted to PEM/PKIX | DONE |
| P1-01 | High | Android BLE MTU raised to 256 | DONE |
| P1-02 | High | iOS token refresh coalesced via actor lock | DONE |
| P1-03 | High | iOS BLE continuation guarded against double-resume | DONE |
| P2-01 | Medium | iOS `.gitignore` added | DONE |
| P2-03 | Medium | iOS environment switching (mock/staging/prod) | DONE |
| P2-02 | Medium | Android CI/CD pipeline | OPEN — Sprint 3 |
| P3-01 | Low | iOS controller identity verification | OPEN — Sprint 3 |

Go BLE protocol tests (`ble_protocol_test.go`) all pass with the new UUIDs.

---

---

## Part 1: Architecture Document Audit

The `Project_Repo_Architecture.md` describes a clean contract-first multi-repo structure with OpenAPI + Protobuf code generation. While the **principles are sound**, the document is disconnected from the current codebase reality. Adopting it as-is would require a near-total rewrite of all three repos. Below are the specific issues and recommended corrections.

### 1.1 Factual Mismatches

| Document States | Actual State | Impact |
|---|---|---|
| Repo names: `access-control-*` | `mistypass`, `ios-MistyisletPass`, `android-MistyisletPass` | Confusing for any developer reading the doc |
| Backend: "Go / Node.js" | Pure Go (chi/v5 router) | Node.js mention is misleading |
| Frontend: "Next.js App Router" | Vite + React 18 + react-router-dom | Wrong framework entirely |
| Monorepo: Turborepo + pnpm workspace | No monorepo tooling; Go backend + React frontend coexist without orchestration | Proposed config files won't work |
| 4 repos including controller firmware | Only 3 repos exist; no controller repo | Entire Section 5 is aspirational |
| Static `openapi.yaml` in `packages/api-spec/` | OpenAPI spec generated at runtime in `api/internal/http/routes_openapi.go` (~123KB) | Code generation pipeline assumes a file that doesn't exist |
| Protobuf for BLE messages | BLE uses raw binary over GATT characteristics, no .proto files anywhere | Entire Section 7 is aspirational |
| `generate.sh` + openapi-generator-cli | No code generation scripts exist | CI workflow from Section 9 would fail |
| iOS uses CocoaPods or SPM | iOS uses zero external dependencies (pure Apple frameworks + XcodeGen) | Dependency assumptions are wrong |

### 1.2 Missing Context

The document does not account for significant existing infrastructure:

- **MQTT/EMQX** — gateway-to-server communication (already in production)
- **NATS JetStream** — internal event bus
- **Redis** — sessions and caching
- **PgBouncer** — connection pooling
- **sqlc** — type-safe SQL code generation (already working)
- **Enterprise features** — SAML, OIDC, SCIM, HRIS integration, multi-tenant
- **Existing CI** — 6 GitHub Actions workflows (API smoke, CJK guard, web-admin CI, dependency audit, security scan, soak tests)
- **Playwright + Vitest** — existing test infrastructure

### 1.3 Recommended Revisions

Instead of adopting the document wholesale, use it as a **target architecture** with these corrections:

1. **Rename repos in doc** to match actual names
2. **Extract the runtime OpenAPI spec to a static file** — run the Go server once, capture `/api/v1/openapi.json`, and use that as the source of truth. This is the lowest-friction path to contract-first development.
3. **Skip Protobuf for now** — BLE protocol is already implemented with raw binary. Protobuf adds complexity without solving a current pain point. Revisit when controller firmware repo exists.
4. **Skip Turborepo/pnpm** — the Go + React frontend coexistence works fine. Monorepo tooling is only justified when you have 3+ JS packages to coordinate.
5. **Use OpenAPI codegen only for mobile clients** — generate Swift and Kotlin API clients from the extracted spec. This is the highest-value part of the document.
6. **Replace cross-repo sync approach** — `dmnemec/copy_file_to_another_repo_action` is unmaintained. Use a git submodule or a dedicated `api-contracts` repo consumed as a dependency.
7. **Add controller firmware section** as "Phase 2" — it doesn't exist yet and shouldn't block mobile integration.

---

## Part 2: Cross-Repo Code Review

### Severity Definitions

| Level | Meaning |
|---|---|
| **P0 CRITICAL** | Feature is broken. BLE unlock or API communication will fail. |
| **P1 HIGH** | Will cause crashes or data corruption in specific scenarios. |
| **P2 MEDIUM** | Security risk or significant code quality issue. |
| **P3 LOW** | Improvement recommended but not blocking. |

---

### P0-01: BLE Characteristic UUIDs — iOS vs Go/Android Mismatch

**Files:**
- `ios-MistyisletPass/MistyisletPass/Utilities/Constants.swift:23-26`
- `mistypass/api/cmd/gateway-agent/ble_protocol.go:25-37`
- `android-MistyisletPass/.../core/ble/MistyisletBleManager.kt:34-38`

**Issue:** iOS uses sequential placeholder UUIDs while Go and Android use ASCII-derived UUIDs. Only the service UUID matches.

| Characteristic | Go / Android | iOS |
|---|---|---|
| Service | `4d495354-5950-4153-532d-424c45415554` | `4D495354-5950-4153-532D-424C45415554` (match) |
| Challenge | `4d495354-5950-4153-532d-4348414c4c4e` | `4D495354-0002-4153-532D-424C45415554` (MISMATCH) |
| AuthResponse | `4d495354-5950-4153-532d-41555448524553` | `4D495354-0003-4153-532D-424C45415554` (MISMATCH) |
| AuthResult | `4d495354-5950-4153-532d-524553554c5400` | `4D495354-0004-4153-532D-424C45415554` (MISMATCH) |
| ReaderIdentity | `4d495354-5950-4153-532d-52454144455249` | `4D495354-0001-4153-532D-424C45415554` (MISMATCH) |

**Impact:** iOS discovers the BLE service but finds zero matching characteristics. `didDiscoverCharacteristicsFor` returns empty. BLE unlock times out silently. **iOS BLE unlock is 100% broken.**

**Fix:** iOS must adopt the Go/Android UUIDs (after P0-02 is fixed first).

---

### P0-02: BLE UUIDs Have Invalid Format (Go + Android)

**Files:**
- `mistypass/api/cmd/gateway-agent/ble_protocol.go:29-37`
- `android-MistyisletPass/.../core/ble/MistyisletBleManager.kt:36-38`

**Issue:** Standard UUID format is `8-4-4-4-12` hex characters. Three UUIDs have 14 characters in the last segment (7 ASCII chars encoded as hex instead of 6):

```
AUTH_RESPONSE:    ...532d-41555448524553    ← 14 hex = "AUTHRES" (7 chars) INVALID
READER_IDENTITY:  ...532d-52454144455249    ← 14 hex = "READERI" (7 chars) INVALID
AUTH_RESULT:      ...532d-524553554c5400    ← 14 hex = "RESULT\0" (7 chars) INVALID

Valid:
SERVICE:          ...532d-424c45415554      ← 12 hex = "BLEAUT" (6 chars) OK
CHALLENGE:        ...532d-4348414c4c4e      ← 12 hex = "CHALLN" (6 chars) OK
```

**Impact:** On Android, `UUID.fromString()` throws `IllegalArgumentException` at runtime. This is currently masked by the TCP simulator path. On real BLE hardware, the app will crash. Go uses these as string constants for TCP simulation — no runtime parsing — but any future real BLE integration will break.

**Fix:** Truncate all UUIDs to 12 hex chars in the last segment:

```
AUTH_RESPONSE:   4d495354-5950-4153-532d-415554485245   ("AUTHRE")
READER_IDENTITY: 4d495354-5950-4153-532d-524541444552   ("READER")
AUTH_RESULT:     4d495354-5950-4153-532d-524553554c54   ("RESULT")
```

Update all three repos simultaneously.

---

### P0-03: iOS API Paths Don't Match Backend Routes

**Files:**
- `ios-MistyisletPass/MistyisletPass/Utilities/Constants.swift:7-15`
- `ios-MistyisletPass/MistyisletPass/Services/APIService.swift`
- `mistypass/api/internal/http/router.go:518-544`

**Issue:** 7 of 8 iOS API paths are wrong.

| iOS Path | Go Backend Path | Problem |
|---|---|---|
| `/auth/login` | `/app/auth/login` | Missing `app/` prefix — hits admin login, not mobile |
| `/auth/refresh` | `/app/auth/refresh` | Missing `app/` prefix |
| `/app/doors` | `/app/access/doors` | Missing `access/` segment |
| `/app/events` | `/app/access/logs` | Wrong name entirely |
| `/app/visitors` | `/app/visitor-passes` | Wrong name |
| `/app/doors/{doorId}/unlock` | `/app/access/unlock` (POST body: `{lock_id}`) | Different path AND parameter passing method |
| `/app/profile` (hardcoded in APIService) | `/app/me` | Wrong path |
| `/app/credentials` | `/app/credentials` | OK |

**Impact:** Every iOS network call except credential listing returns 404 (or worse, hits the wrong admin endpoint for login). **iOS app is non-functional against the real backend.**

**Fix:** Align all iOS paths to match the Go router. Note that Android paths are all correct.

---

### P0-04: iOS Public Key Format Incompatible With Backend

**Files:**
- `ios-MistyisletPass/MistyisletPass/Services/SecureEnclaveService.swift:101-113`
- `mistypass/api/cmd/gateway-agent/ble_protocol.go:177-195`
- `android-MistyisletPass/.../core/ble/KeystoreManager.kt:95-101`

**Issue:**
- **Go expects:** PEM-encoded PKIX public key (`-----BEGIN PUBLIC KEY-----\n<base64 DER>\n-----END PUBLIC KEY-----`)
- **Android sends:** PEM-encoded PKIX — `getPublicKeyPEM()` wraps DER in PEM headers. **Correct.**
- **iOS sends:** Raw X9.63 format (65 bytes for P-256: `0x04 || x || y`) base64-encoded via `SecKeyCopyExternalRepresentation`. **No PEM wrapping, wrong DER structure.**

**Impact:** iOS mobile credential registration will fail. `parseBLEPublicKey()` calls `pem.Decode()` which returns nil on non-PEM input. BLE signature verification impossible.

**Fix:** iOS must convert the raw X9.63 public key to PKIX DER, then wrap in PEM:

```swift
func exportPublicKeyPEM() throws -> String {
    guard let publicKey = getPublicKey() else {
        throw SecureEnclaveError.publicKeyExportFailed
    }
    var error: Unmanaged<CFError>?
    guard let rawKey = SecKeyCopyExternalRepresentation(publicKey, &error) as Data? else {
        throw SecureEnclaveError.publicKeyExportFailed
    }
    // Wrap X9.63 in SubjectPublicKeyInfo (PKIX) ASN.1 structure
    let pkixHeader: [UInt8] = [
        0x30, 0x59, 0x30, 0x13, 0x06, 0x07, 0x2a, 0x86,
        0x48, 0xce, 0x3d, 0x02, 0x01, 0x06, 0x08, 0x2a,
        0x86, 0x48, 0xce, 0x3d, 0x03, 0x01, 0x07, 0x03,
        0x42, 0x00
    ]
    let der = Data(pkixHeader) + rawKey
    let b64 = der.base64EncodedString()
    return "-----BEGIN PUBLIC KEY-----\n\(b64)\n-----END PUBLIC KEY-----\n"
}
```

---

### P1-01: Android BLE MTU Too Low for ECDSA Signatures

**File:** `android-MistyisletPass/.../core/ble/MistyisletBleManager.kt:108`

**Issue:** `requestMtu(64)` gives 61 usable bytes (64 - 3 ATT overhead). Auth response format: `[1B userIdLen][userId bytes][signature bytes]`. ECDSA ASN.1 DER signature is 70-72 bytes. Even with a 1-char userId: 1 + 1 + 71 = 73 bytes. **Always exceeds MTU.**

**Impact:** BLE write will be truncated. The reader receives an incomplete signature and verification fails. Unlock denied.

**Fix:** `requestMtu(256)`. BLE 4.2+ supports up to 517 bytes.

---

### P1-02: iOS Token Refresh Race Condition

**File:** `ios-MistyisletPass/MistyisletPass/Services/APIService.swift`

**Issue:** `isRefreshing` is a plain `Bool` on a non-`actor` class. Two concurrent requests getting 401 can both see `isRefreshing == false`, both fire refresh calls, and the second one invalidates the first refresh token.

**Impact:** Intermittent auth failures under concurrent requests (e.g., loading a screen that makes multiple API calls).

**Fix:** Use an `actor` or a shared `Task` with `withCheckedContinuation` to coalesce concurrent refresh attempts.

---

### P1-03: iOS BLE Continuation Double-Resume Crash

**File:** `ios-MistyisletPass/MistyisletPass/Services/BLEManager.swift`

**Issue:** The timeout closure on `bleQueue` and the `didUpdateValueFor` delegate callback both access `authResultContinuation`. If the BLE result arrives right as the timeout fires, both code paths can resume the continuation. Resuming a Swift continuation twice is undefined behavior (crash).

**Impact:** Intermittent crash during BLE unlock, especially on slower BLE connections near the 5-second timeout.

**Fix:** Guard with an atomic flag or use `withCheckedContinuation` (which traps on double-resume in debug builds) and coordinate access through a single serial queue.

---

### P2-01: iOS Missing .gitignore

**File:** `ios-MistyisletPass/` (root)

**Issue:** No `.gitignore` file. Standard Xcode artifacts (`.DS_Store`, `xcuserdata/`, `DerivedData/`, `*.xcscmblueprint`) will be committed.

**Fix:** Add standard Xcode `.gitignore` (use github.com/github/gitignore/Swift.gitignore as template).

---

### P2-02: Android Missing CI/CD Pipeline

**File:** `android-MistyisletPass/` (no `.github/workflows/`)

**Issue:** No CI pipeline. No automated build verification, no lint checks, no test execution on PR.

**Fix:** Add a minimal GitHub Actions workflow: build debug + run unit tests on PR.

---

### P2-03: iOS Hardcoded Production Base URL

**File:** `ios-MistyisletPass/MistyisletPass/Utilities/Constants.swift:7`

**Issue:** `baseURL = "https://api.mistyislet.com/v1"` is hardcoded. No mechanism to switch between mock/staging/production. Android has build variants with per-environment URLs.

**Fix:** Use Xcode build configurations or XcodeGen scheme environment variables (similar to the pattern shown in the architecture doc Section 8).

---

### P3-01: iOS Missing BLE Controller Identity Verification

**File:** `ios-MistyisletPass/MistyisletPass/Utilities/Constants.swift:23`

**Issue:** `controllerIdentityUUID` is defined but never read during BLE service discovery. The phone doesn't verify which door controller it connected to. This enables relay/impersonation attacks (a rogue device advertising the same service UUID).

**Fix:** After discovering the service, read the reader identity characteristic and verify the controller's certificate against a trusted root.

---

## Part 3: Fix Schedule

### Sprint 1 — BLE Protocol Alignment (3 days)

Must be done across all 3 repos simultaneously to avoid further divergence.

| Day | Task | Owner | Repo |
|---|---|---|---|
| Day 1 | Fix malformed UUIDs (P0-02): truncate to 12 hex chars | Backend | mistypass |
| Day 1 | Update Android UUIDs to match fixed Go UUIDs | Android | android-MistyisletPass |
| Day 1 | Update iOS UUIDs to match fixed Go UUIDs (P0-01) | iOS | ios-MistyisletPass |
| Day 2 | Fix Android MTU: `requestMtu(256)` (P1-01) | Android | android-MistyisletPass |
| Day 2 | Fix iOS public key export to PEM/PKIX format (P0-04) | iOS | ios-MistyisletPass |
| Day 2 | Fix iOS BLE continuation double-resume (P1-03) | iOS | ios-MistyisletPass |
| Day 3 | Integration test: Android BLE unlock against real gateway-agent | Android + Backend | — |
| Day 3 | Integration test: iOS BLE unlock against real gateway-agent | iOS + Backend | — |

### Sprint 2 — iOS API Path Alignment (2 days)

| Day | Task | Owner | Repo |
|---|---|---|---|
| Day 1 | Fix all iOS API paths to match Go router (P0-03) | iOS | ios-MistyisletPass |
| Day 1 | Fix iOS token refresh race condition (P1-02) | iOS | ios-MistyisletPass |
| Day 1 | Add .gitignore (P2-01) | iOS | ios-MistyisletPass |
| Day 1 | Add environment switching for base URL (P2-03) | iOS | ios-MistyisletPass |
| Day 2 | End-to-end test: iOS app login + door list + unlock against staging backend | iOS + Backend | — |

### Sprint 3 — CI & Contract Automation (2 days)

| Day | Task | Owner | Repo |
|---|---|---|---|
| Day 1 | Add Android CI pipeline (P2-02) | Android | android-MistyisletPass |
| Day 1 | Extract static OpenAPI spec from Go runtime | Backend | mistypass |
| Day 2 | Set up openapi-generator for Swift + Kotlin client generation | Backend | mistypass |
| Day 2 | Replace hand-written API clients with generated code (iOS + Android) | iOS + Android | Both |

### Sprint 4 — Architecture Document Update (1 day)

| Task | Owner |
|---|---|
| Revise architecture doc to match actual repo names, tech stack, and structure | Tech Lead |
| Add sections for existing infra (MQTT, NATS, Redis, sqlc, enterprise features) | Tech Lead |
| Mark controller firmware and Protobuf as "Phase 2" | Tech Lead |
| Remove Next.js, Turborepo, pnpm references | Tech Lead |

---

## Summary

**Architecture document:** Directionally correct but factually disconnected from the codebase. Needs major revision before it can serve as a working reference. The contract-first principle (OpenAPI code generation for mobile clients) is the most valuable part and should be prioritized.

**iOS app:** In the worst shape. 4 P0 bugs (BLE UUIDs, API paths, public key format, BLE UUIDs mismatch), 2 P1 bugs (token refresh race, BLE crash), and missing fundamentals (.gitignore, environment switching). Cannot communicate with the backend or perform BLE unlock in its current state.

**Android app:** Structurally sound. API paths are correct. BLE protocol aligns with Go backend except for the shared UUID format bug (P0-02) and MTU issue (P1-01). Missing CI/CD pipeline.

**Go backend:** Solid. API contracts are well-defined. BLE protocol implementation is clean with good test coverage. The only issue is the 3 malformed UUID constants.

**Critical path for integration:** Fix BLE UUIDs across all repos (Sprint 1) > Fix iOS API paths (Sprint 2) > Set up contract automation (Sprint 3).
