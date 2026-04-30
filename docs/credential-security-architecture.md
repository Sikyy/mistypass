# MistyPass Credential Security Architecture

Full-chain security specification for credential issuance, management, verification, and revocation.

**Version:** 1.1
**Date:** 2026-04-30
**Scope:** Cloud API, Mobile App, Web Admin, Gateway Agent, Reader/Lock Hardware

---

## Table of Contents

1. [System Overview](#1-system-overview)
2. [Communication Link Map](#2-communication-link-map)
3. [Credential Issuance (Full Chain)](#3-credential-issuance)
4. [Credential Storage](#4-credential-storage)
5. [Credential Verification (Access Decision)](#5-credential-verification)
6. [Credential Management & Revocation](#6-credential-management--revocation)
7. [Transport Security](#7-transport-security)
8. [Authentication & Authorization](#8-authentication--authorization)
9. [Cryptographic Operations](#9-cryptographic-operations)
10. [Audit & Compliance](#10-audit--compliance)
11. [Threat Mitigations](#11-threat-mitigations)

---

## 1. System Overview

MistyPass is a cloud-based physical access control system. The system consists of five components:

```
+----------------+      HTTPS/TLS       +----------------+     HTTPS/TLS      +------------------+
|  Mobile App    | <------------------> |   Cloud API    | <----------------> |   Web Admin      |
|  (iOS/Android) |  JWT Bearer Token    | (Go, PostgreSQL|  JWT Bearer Token  |  (React SPA)     |
+----------------+                      |  Redis, NATS)  |                    +------------------+
       |                                +----------------+
       | BLE / NFC                            |
       v                                      | HTTPS + Device Token
+----------------+                            v
|  Door Reader   | <-- RS485/GPIO -->  +------------------+
|  (NFC/RFID)    |   Wired Protocol    |  Gateway Agent   |
+----------------+                     |  (Edge Device)   |
                                       +------------------+
```

**Credential Types Supported:**

| Type | Medium | Presentation | Use Case |
|------|--------|-------------|----------|
| NFC UID | Physical Card (MIFARE/DESFire/NTAG, fixed UID only) | Contactless tap at reader | Primary door access |
| BLE Token | Mobile App | Bluetooth proximity / app unlock | Smartphone-based access |
| Card Number | Physical RFID Card | RFID scan at reader | Legacy card readers |
| QR Code | Mobile App / Printed Paper | Camera scan / visual display | Visitor / temporary access |

---

## 2. Communication Link Map

All communication paths between components and their security measures:

### Link 1: User (Mobile App) <-> Cloud API

```
Protocol:    HTTPS (TLS 1.2+)
Auth:        JWT Bearer Token (HMAC-SHA256)
Direction:   Bidirectional REST API
Rate Limit:  Login 10/min per IP, API 600/min per IP
```

| Endpoint | Purpose | Auth |
|----------|---------|------|
| `POST /api/v1/app/auth/login` | User login, returns JWT token pair | Email + Password |
| `POST /api/v1/app/auth/refresh` | Refresh token exchange | Refresh Token |
| `GET /api/v1/app/credentials` | List user's credentials | Access Token |
| `POST /api/v1/app/credentials/apple-pass` | Enroll Apple Wallet pass | Access Token |
| `POST /api/v1/app/access/unlock` | BLE-based unlock request | Access Token |
| `POST /api/v1/app/access/qr-unlock` | QR code-based unlock | Access Token |
| `GET /api/v1/app/access/my-doors` | List accessible doors | Access Token |
| `GET /api/v1/app/access/logs` | User's access history | Access Token |

**Code:** `api/internal/http/routes_app_access.go`, `routes_auth.go`

**Security chain:**
1. App connects to Cloud API over HTTPS (TLS certificate validated by OS)
2. User authenticates with email + bcrypt-hashed password
3. Cloud issues JWT access token (1h TTL) + refresh token (7d TTL), both signed with HMAC-SHA256
4. All subsequent requests carry `Authorization: Bearer {access_token}`
5. Cloud validates JWT signature, checks expiration, checks revocation list
6. User identity (user_id, tenant_id, role) extracted from JWT claims

### Link 2: Admin (Web Admin) <-> Cloud API

```
Protocol:    HTTPS (TLS 1.2+)
Auth:        JWT Bearer Token (HMAC-SHA256) + Optional TOTP MFA
Direction:   Bidirectional REST API
CORS:        Configurable origin whitelist (CORS_ORIGIN env)
Rate Limit:  Login 10/min, API 600/min
```

| Endpoint | Purpose | Required Role |
|----------|---------|---------------|
| `POST /api/v1/wallet/passes/issue` | Issue credential to user | tenant_admin+ |
| `POST /api/v1/wallet/passes/batch-issue` | Batch issue credentials | tenant_admin+ |
| `PATCH /api/v1/wallet/passes/{id}/suspend` | Suspend credential | tenant_admin+ |
| `PATCH /api/v1/wallet/passes/{id}/activate` | Re-activate credential | tenant_admin+ |
| `POST /api/v1/wallet/passes/{id}/revoke` | Permanently revoke | tenant_admin+ |
| `POST /api/v1/group_locks` | Bind user group to door | building_admin+ |
| `DELETE /api/v1/group_locks/{id}` | Remove group-door binding | building_admin+ |

**Code:** `api/internal/http/routes_wallet.go`, `router.go`

**Security chain:**
1. Admin authenticates with email + password (bcrypt verified)
2. If MFA enabled: TOTP code required (HMAC-SHA1, 30-second window)
3. JWT issued with role claim (super_admin/tenant_admin/operator/building_admin)
4. RBAC middleware (`requireRoles()`) enforces per-endpoint role requirements
5. Building-scoped admins can only manage resources in their assigned buildings
6. All credential operations logged to audit trail with actor identity

### Link 3: Cloud API <-> Gateway Agent (Edge Device)

```
Protocol:    HTTPS (TLS 1.2+)
Auth:        Device Token (X-Device-Token header) or Bearer Token
Direction:   Gateway polls Cloud (pull-based)
Interval:    Config pull every 30s, event upload every 10s
```

**Bootstrap Registration Flow:**

```
Step 1: Factory provisioning
  - Serial number pre-registered in mistypass_gateway_serial_inventory
  - GATEWAY_BOOTSTRAP_TOKEN configured as pre-shared secret

Step 2: Device registration
  POST /api/v1/gateway/register
  Header: X-Bootstrap-Token: {GATEWAY_BOOTSTRAP_TOKEN}
  Body: { serial_number, tenant_id, building_id, device_capacity }
  Response: { gateway_id, device_token: "gw_{random_128bit_hex}" }
  - Bootstrap token verified with constant-time comparison (crypto/subtle)
  - Serial number validated against inventory (product type, tenant match)
  - Device token generated: crypto/rand 16 bytes → hex → "gw_" prefix
  - Token hash (SHA256) stored in mistypass_gateway_device_tokens table
  - Plaintext token returned to device ONCE (never stored on server)

Step 3: Ongoing communication
  POST /api/v1/gateway/config/pull
  Header: X-Device-Token: {device_token}
  Body: { gateway_id, tenant_id, current_version, authz_cache_version }
  Response: { authz_cache: { version, access_rules[], doors[], users[] } }
  - Device token verified: SHA256(provided) compared with stored hash
  - Comparison uses crypto/subtle.ConstantTimeCompare (timing-attack resistant)
```

**Code:** `api/internal/http/routes_gateway_bootstrap.go`, `router.go:8544-8682`, `api/cmd/gateway-agent/agent.go`

**Security chain:**
1. Gateway boots, attempts to load persisted device token from local file (`-token-file`, default `/var/lib/mistypass/device-token`, permissions `0600`)
2. If no device token found, registers with Cloud API using bootstrap token, receives unique device token
3. Device token persisted to local file for survival across restarts (written once, owner read/write only)
4. All subsequent requests authenticated with device token (bootstrap token no longer used after registration)
5. Token verified server-side: SHA256(provided) compared with stored hash via `crypto/subtle.ConstantTimeCompare`
6. Gateway HTTP client supports TLS certificate pinning (`-tls-pin-sha256`): verifies Cloud API certificate SPKI hash, rejects connections if pin mismatch (prevents MITM with rogue CA)
7. Access rules pulled every 30s and cached locally on gateway with configurable TTL (`-rules-cache-ttl`, default 24h)

### Link 4: Cloud API -> Gateway Agent (Command Push)

```
Protocol:    NATS (over TLS when configured)
Auth:        NATS credentials
Direction:   Cloud pushes commands to gateway
Topic:       gateway.{gateway_id}.command
```

**Unlock Command Structure:**
```json
{
  "request_id": "verify:{lock_id}:{user_id}:{timestamp_ns}",
  "gateway_id": "gw_demo_001",
  "command": "unlock",
  "lock_id": "door_jkt_001",
  "tenant_id": "tenant_demo_jakarta",
  "issued_by": "user@example.com",
  "issued_at": "2026-04-30T10:00:00Z"
}
```

**Code:** `api/internal/bus/commands.go`, `publisher.go`

### Link 5: Gateway Agent <-> Door Reader/Lock (Wired)

```
Protocol:    GPIO (direct pin control) or RS485 Modbus RTU (serial)
Auth:        Physical wired connection (not network-accessible)
Direction:   Gateway controls lock relay
```

**GPIO Relay Control:**
```
Export:     /sys/class/gpio/export → pin number
Direction: /sys/class/gpio/gpio{N}/direction → "out"
Unlock:    /sys/class/gpio/gpio{N}/value → "0" (active-low)
Relock:    /sys/class/gpio/gpio{N}/value → "1" (after configurable duration)
```

**RS485 Modbus RTU:**
```
Device:    /dev/ttyUSB0 (or configured serial port)
Protocol:  Modbus RTU, Function 0x05 (Write Single Coil)
Unlock:    [0x01 0x05 0x00 0x00 0xFF 0x00 CRC16]
Relock:    [0x01 0x05 0x00 0x00 0x00 0x00 CRC16]
Duration:  Configurable (default 5 seconds)
```

**Code:** `api/cmd/gateway-agent/relay.go`

**Security:**
- Physical wired connection only (RS485/GPIO), not exposed to network
- Gateway device is the trust boundary — all access decisions made before relay activation
- Relay auto-relocks after configurable timeout (prevents stuck-open doors)

### Link 6: Cloud API <-> External Identity Providers

```
Protocol:    HTTPS (TLS 1.2+)
Auth:        OIDC / SAML 2.0
Direction:   Redirect-based flow (browser)
```

**OIDC Flow:**
1. Admin initiates: `POST /api/v1/enterprise/auth/start` with provider="oidc"
2. Cloud returns authorize_url with state token (6-byte random hex, 5-min TTL)
3. User redirected to IdP, authenticates, redirected back to callback
4. Cloud exchanges authorization code for ID token
5. ID token verified: JWKS fetched from IdP (1-hour cache), RSA signature verified (RS256/384/512)
6. Claims validated: issuer, audience, subject, email
7. User matched or provisioned, JWT issued

**SAML Flow:**
1. Cloud generates AuthnRequest with state token
2. User redirected to IdP SSO URL
3. IdP posts SAMLResponse to ACS URL
4. Cloud validates: X.509 certificate, assertion signature, subject/email claims
5. ACS URL enforced to HTTPS only

**HRIS Webhook:**
```
Endpoint:  POST /api/v1/enterprise/hris-webhook/{connectorID}
Auth:      HMAC-SHA256 signature in Authorization header
Format:    hmac username="{clientID}", algorithm="hmac-sha256",
           headers="date request-line", signature="{base64}"
Verify:    Constant-time comparison (crypto/subtle)
Clock:     300-second skew tolerance
Rate:      240 requests/min per tenant
```

**Code:** `api/internal/http/routes_enterprise_auth.go`, `api/internal/modules/hris/talenta/hmac.go`

---

## 3. Credential Issuance

### 3.1 Full Issuance Flow

```
Admin                    Cloud API                  Wallet Provider         User
  |                         |                            |                   |
  |-- Issue Pass Request -->|                            |                   |
  |   (template, user,      |                            |                   |
  |    expires_at)           |                            |                   |
  |                         |-- Create Pass Class ------>|                   |
  |                         |   (template → class)       |                   |
  |                         |                            |                   |
  |                         |-- Issue Pass Object ------>|                   |
  |                         |   (user, NFC payload,      |                   |
  |                         |    barcode, save_link)     |                   |
  |                         |                            |                   |
  |                         |<-- Pass Object + JWT ------|                   |
  |                         |                            |                   |
  |                         |-- Store Pass Record ------>|                   |
  |                         |   (DB: status=issued)      |  (database)      |
  |                         |                            |                   |
  |                         |-- Deliver Save Link -------|------------------>|
  |                         |   (Email/SMS/In-App)       |                   |
  |                         |                            |                   |
  |<-- Issue Response ------|                            |   (clicks link)   |
  |   (pass_id, save_link,  |                            |                   |
  |    status=issued)        |                            |                   |
  |                         |                            |<-- Save to Wallet-|
  |                         |                            |                   |
  |                         |-- Assign Pass ----------->|                   |
  |                         |   (DB: status=active,      |                   |
  |                         |    activated_at=now)        |                   |
```

### 3.2 Credential Generation Details

**Pass ID:**
```go
// 16-byte cryptographically random hex with "wps_" prefix
passID = "wps_" + hex.EncodeToString(crypto/rand.Read(16))
// Example: "wps_a3f8b2c1d4e5f6a7b8c9d0e1f2a3b4c5"
```

**NFC Payload:**
```go
// 8-byte random hex appended to pass ID
nfcPayload = fmt.Sprintf("mistyislet:%s:%s", passID, hex.EncodeToString(crypto/rand.Read(8)))
// Example: "mistyislet:wps_a3f8b2c1...:e7f8a9b0c1d2e3f4"
```

**BLE Token:**
```go
// 16-byte random hex with "ble_" prefix
bleToken = "ble_" + hex.EncodeToString(crypto/rand.Read(16))
```

**Physical Card UID:**
- From manufacturer (NFC chip serial number)
- Registered via inventory scan: `POST /api/v1/wallet/physical-card-inventory/scan`
- Linked to pass via `AssignedPassID` field

**QR Code Token:**
- Generated as random secret in GroupLink record
- Includes `ValidFrom` and `ValidUntil` time bounds
- Type: "online" (app display) or "paper" (printed)

**Code:** `api/internal/modules/wallet/service.go:1130-1160`

### 3.3 Wallet Pass Structure

**Google Wallet Pass:**
```json
{
  "id": "{issuerID}.{classPrefix}.{passID}",
  "classId": "{issuerID}.{classPrefix}.class.{templateID}",
  "cardTitle": "Mistyislet Access",
  "header": "{holder_name}",
  "subheader": "{holder_email}",
  "barcode": {
    "type": "QR_CODE",
    "value": "mistyislet:{passID}:{random_hex}"
  },
  "state": "ACTIVE"
}
```
Save link: `https://pay.google.com/gp/v/save/{RS256_signed_JWT}`

**Apple Wallet Pass (.pkpass bundle):**
```
pass.json        → NFC message, barcode, holder info, web service URL
manifest.json    → SHA256 hash of every file in bundle
signature        → PKCS#7 signed manifest (Apple Pass Type Certificate)
```

Pass.json NFC section:
```json
{
  "nfc": {
    "message": "mistyislet:{passID}:{random_hex}",
    "requiresAuthentication": false
  },
  "barcode": {
    "format": "PKBarcodeFormatQR",
    "message": "mistyislet:{passID}:{random_hex}"
  }
}
```

**Code:** `api/internal/modules/wallet/google_wallet_provider.go`, `apple_pass_provider.go`

### 3.4 Batch Issuance

Two modes:
- **Inline:** Synchronous processing, immediate result
- **Queued:** Asynchronous with job queue, retry on failure

Job states: `pending → processing → success | failed → dlq`

Retry: max 3 attempts, dead-letter queue for persistent failures.

**Code:** `api/internal/modules/wallet/service.go:1339+`

---

## 4. Credential Storage

### 4.1 Cloud (PostgreSQL)

| Data | Table | Protection |
|------|-------|-----------|
| User passwords | `mistypass_auth_users.password_hash` | bcrypt (cost=10) |
| Gateway device tokens | `mistypass_gateway_device_tokens.token_hash` | SHA256 hash |
| Refresh sessions | `mistypass_auth_refresh_sessions` | Session ID + user binding + TTL |
| Revoked access tokens | `mistypass_auth_revoked_access_tokens` | Token ID + TTL |
| Wallet passes | `mistypass_wallet_passes` | Tenant-isolated, status tracking |
| HRIS secrets | HRIS vault | AES-256-GCM (per-secret random nonce) |
| Audit logs | `mistypass_audit_logs` | Append-only, immutable |
| MFA secrets | `mistypass_auth_admin_mfa_states` | Stored for TOTP generation |

**Multi-tenancy:** Every table includes `tenant_id` column. All queries scoped by tenant.

### 4.2 Gateway Agent (Edge Device)

| Data | Storage | Protection | Lifecycle |
|------|---------|-----------|-----------|
| Device token | Local file (`-token-file`) | File permissions `0600` (owner read/write only) | Written once at registration, persisted across restarts |
| Access rules | In-memory struct | Not persisted to disk | Refreshed every 30s from Cloud |
| Rule version | In-memory string | N/A | Updated on each config pull |
| Rules updated timestamp | In-memory `time.Time` | N/A | Set on each successful config pull |
| Event queue | In-memory slice | N/A | Flushed every 10s, re-queued on failure |

- No credential secrets stored on gateway (only credential identifiers for matching)
- Access rules contain: credential_type, credential_data (UID/token), user_id, allowed lock_ids
- **Cache TTL enforcement:** If rules are older than `-rules-cache-ttl` (default 24h), all access is denied until Cloud resync succeeds. This prevents stale rules from granting access indefinitely during prolonged network outages
- Device token never transmitted in logs or error messages

### 4.3 Mobile App

| Data | Storage | Protection |
|------|---------|-----------|
| JWT access token | App memory | 1-hour TTL, cleared on logout |
| JWT refresh token | Secure storage (Keychain/Keystore) | 7-day TTL |
| BLE token | From API response | Per-session, not persisted |
| Wallet pass | Apple Wallet / Google Wallet | OS-managed secure storage |

### 4.4 Key Management

| Key | Source | Rotation |
|-----|--------|----------|
| JWT signing key | `JWT_SECRET` env var | Manual rotation (invalidates all tokens) |
| HRIS vault master key | `HRIS_VAULT_MASTER_KEY` env var | Manual (requires re-encryption) |
| Gateway bootstrap token | `GATEWAY_BOOTSTRAP_TOKEN` env var | Manual (requires re-registration) |
| Per-device token | `crypto/rand` at registration | One-time generation, no rotation |

---

## 5. Credential Verification

### 5.1 Online Verification (Cloud-Mediated)

Used when: App-based unlock (BLE, QR), admin-triggered unlock

```
Mobile App           Cloud API              NATS              Gateway Agent      Lock
    |                    |                    |                     |              |
    |-- Unlock Request ->|                    |                     |              |
    |   (lock_id,        |                    |                     |              |
    |    ble_token)       |                    |                     |              |
    |                    |                    |                     |              |
    |  [1] Verify JWT    |                    |                     |              |
    |  [2] Resolve cred  |                    |                     |              |
    |      → user_id     |                    |                     |              |
    |  [3] Check user    |                    |                     |              |
    |      status=active |                    |                     |              |
    |  [4] Check pass    |                    |                     |              |
    |      not expired   |                    |                     |              |
    |  [5] Check group   |                    |                     |              |
    |      → door binding|                    |                     |              |
    |  [6] Check role    |                    |                     |              |
    |      assignment    |                    |                     |              |
    |                    |-- Publish Command ->|                     |              |
    |                    |   (unlock, lock_id) |                     |              |
    |                    |                    |-- Deliver Command -->|              |
    |                    |                    |                     |-- GPIO/RS485->|
    |                    |                    |                     |   UNLOCK      |
    |                    |                    |                     |              |
    |                    |                    |                     |-- (5s timer)->|
    |                    |                    |                     |   RELOCK      |
    |<-- Decision -------|                    |                     |              |
    |   (allow/deny,     |                    |                     |              |
    |    reason)         |                    |                     |              |
    |                    |                    |                     |              |
    |  [7] Log access    |                    |                     |              |
    |      event         |                    |                     |              |
```

**Verification Steps (routes_gateway_verify.go):**

1. **Credential resolution:** Match credential_data against physical card inventory (by UID or card_number), pass instances (by UID or BLE token), or group links (by QR token)
2. **Status check:** `pass.Status == "active"` required
3. **Expiration check:** `pass.ExpiresAt` parsed as RFC3339, compared against current UTC time
4. **User validation:** User record looked up, `user.Status == "active"` required
5. **Group-door binding:** User's group IDs matched against door groups; door group must explicitly contain the target lock_id
6. **Role-based fallback:** If no group match, check role assignments for building-level access
7. **QR-specific checks:** `LinkEnabled` must be true, `ValidUntil` must not be past

### 5.2 Offline Verification (Gateway Local Decision)

Used when: NFC tap at reader, gateway cannot reach Cloud

```
Reader              Gateway Agent                     Lock
  |                      |                              |
  |-- NFC UID / Card --->|                              |
  |                      |                              |
  |  [1] Lookup in       |                              |
  |      cached rules    |                              |
  |  [2] Match cred type |                              |
  |      + cred data     |                              |
  |  [3] Check lock_id   |                              |
  |      in rule.LockIDs |                              |
  |                      |                              |
  |  If match found:     |                              |
  |                      |-- GPIO/RS485: UNLOCK ------->|
  |                      |                              |
  |                      |-- (5s timer) RELOCK -------->|
  |                      |                              |
  |  [4] Queue event     |                              |
  |      for upload      |                              |
```

**Local Decision Logic (agent.go):**
```go
// Step 0: Check cache TTL — deny all if rules are stale
if rulesCacheTTL > 0 && time.Since(rulesUpdatedAt) > rulesCacheTTL {
    return "deny"  // rules expired, block until Cloud resync
}

// Step 1-3: Match credential against cached rules
for _, rule := range agent.accessRules {
    if rule.CredentialType == credType &&
       strings.EqualFold(rule.CredentialData, credData) {
        for _, lockID := range rule.LockIDs {
            if lockID == targetLockID {
                return "allow"
            }
        }
    }
}
return "deny"
```

**Security properties:**
- Rules refreshed from Cloud every 30 seconds
- Revoked credentials removed from rules on next pull
- No credential secrets transmitted — only identifiers for matching
- Events queued locally and uploaded when connectivity restored (every 10s)
- **Cache TTL (default 24h):** If gateway cannot reach Cloud for longer than the configured TTL, all access is denied. This prevents indefinite use of stale rules during prolonged outages (e.g., network cut, Cloud down)

---

## 6. Credential Management & Revocation

### 6.1 Credential Lifecycle States

```
                                    +---> suspended ---+
                                    |                  |
issued ---> active -----------------+---> revoked      |
   |           ^                                       |
   |           +---------------------------------------+
   |           |
   +-- (assign)
```

| State | Meaning | Can Open Door | Reversible |
|-------|---------|--------------|------------|
| issued | Created, not yet assigned to user | No | Yes |
| active | Assigned and usable | Yes (if not expired) | Yes |
| suspended | Temporarily disabled | No | Yes (→ active) |
| revoked | Permanently invalidated | No | **No** |

### 6.2 Revocation Flow

```
Admin                    Cloud API                   Gateway Agent
  |                         |                             |
  |-- Revoke Request ------>|                             |
  |   POST /wallet/passes/  |                             |
  |   {passID}/revoke       |                             |
  |                         |                             |
  |  [1] Set status=revoked |                             |
  |  [2] Set revoked_at=now |                             |
  |  [3] Log audit event    |                             |
  |      (actor, timestamp) |                             |
  |                         |                             |
  |<-- 200 OK --------------|                             |
  |                         |                             |
  |                         |-- Next config pull -------->|
  |                         |   (revoked cred removed     |
  |                         |    from access_rules)       |
  |                         |                             |
  |                         |  Gateway removes rule from  |
  |                         |  local cache. Credential    |
  |                         |  can no longer open door.   |
```

**Revocation propagation latency:** Up to 30 seconds (gateway config pull interval).

**Emergency revocation:** Admin can also issue `lock_down` command via NATS for immediate door lockdown.

### 6.3 Token Revocation

| Token Type | Revocation Method | Propagation |
|-----------|-------------------|-------------|
| Access token (JWT) | Added to `mistypass_auth_revoked_access_tokens` | Immediate (checked on every request) |
| Refresh token | Deleted from `mistypass_auth_refresh_sessions` | Immediate |
| All user sessions | `RevokeRefreshTokensByUserEmail()` | Immediate (bulk) |
| Gateway device token | Update hash in database | Next request from gateway |

### 6.4 Credential Expiration

- `ExpiresAt` field on every pass (RFC3339 timestamp)
- Checked during both online and offline verification
- Expired credentials denied with reason `credential_not_found`
- QR group links: `ValidUntil` field enforced separately

---

## 7. Transport Security

### 7.1 Per-Link Security Summary

| Link | Protocol | Encryption | Auth | Integrity |
|------|----------|-----------|------|-----------|
| App ↔ Cloud | HTTPS | TLS 1.2+ | JWT Bearer | TLS MAC |
| Admin ↔ Cloud | HTTPS | TLS 1.2+ | JWT Bearer + MFA | TLS MAC |
| Gateway ↔ Cloud | HTTPS | TLS 1.2+ with SPKI certificate pinning | Device Token (SHA256 hash + constant-time compare) | TLS MAC + pin verification |
| Cloud → Gateway (push) | NATS | TLS (configurable) | NATS credentials | NATS protocol |
| Gateway ↔ Lock | RS485/GPIO | Physical wire | Wired connection | Modbus CRC16 |
| Cloud ↔ IdP | HTTPS | TLS 1.2+ | OIDC/SAML (JWKS/X.509) | Digital signature |
| HRIS Webhook | HTTPS | TLS 1.2+ | HMAC-SHA256 | Body digest + signature |

### 7.2 HTTP Server Hardening

```go
Server: &http.Server{
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       10 * time.Second,
    WriteTimeout:      10 * time.Second,
    IdleTimeout:       60 * time.Second,
}
```

**Request body limit:** 1 MB max (`maxBodyBytes = 1 << 20`)
**JSON validation:** Strict schema (disallow unknown fields, exactly one JSON object)

**Code:** `api/cmd/api/main.go:67-74`, `api/internal/http/json.go`

---

## 8. Authentication & Authorization

### 8.1 JWT Token Architecture

```
+-------------------+     HS256 Sign      +-------------------+
| Token Claims      | ==================> | JWT Token         |
|                   |   JWT_SECRET key    | (compact format)  |
| user_id           |                     |                   |
| email             |                     | Header.Payload.   |
| role              |                     | Signature         |
| tenant_id         |                     |                   |
| building_ids[]    |                     |                   |
| token_type        |                     |                   |
| jti (random 12B)  |                     |                   |
| iat, exp, nbf     |                     |                   |
+-------------------+                     +-------------------+
```

| Token | TTL | Usage | Revocable |
|-------|-----|-------|-----------|
| Access token | 1 hour | API authorization | Yes (blacklist) |
| Refresh token | 7 days | Exchange for new access token | Yes (session delete) |

**Verification steps:**
1. Parse JWT, verify HMAC-SHA256 signature with `JWT_SECRET`
2. Check `token_type == "access"`
3. Verify `issuer` matches configured JWT_ISSUER
4. Check `exp` (expiration), `nbf` (not before), `iat` (issued at)
5. Lookup `jti` in revoked tokens table — deny if found
6. Extract user identity from claims

**Code:** `api/internal/modules/auth/service.go:983-1045`

### 8.2 Role-Based Access Control (RBAC)

```
super_admin       → Full system access (all tenants)
tenant_admin      → Full access within one tenant
operator          → Read-only monitoring
building_admin    → Scoped to assigned buildings
resident          → End-user (app-only access)
```

**Enforcement:**
```go
protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/passes/issue", ...)
```

Every API endpoint declares required roles. Middleware returns `403 Forbidden` if user's JWT role is not in the allowed set.

**Building scope:**
- `building_admin` can only access resources in their assigned `building_ids`
- `buildingScopeForRequest()` intersects JWT building_ids with role assignments
- Returns `403` if resource is in a building outside scope

### 8.3 Multi-Factor Authentication (Admin)

- Algorithm: TOTP (RFC 6238), HMAC-SHA1, 30-second window
- Enrollment: Cloud generates random secret, returns `otpauth://` URL for authenticator app
- Verification: Admin submits 6-digit code, verified against current and adjacent time windows
- Configurable: `AUTH_ADMIN_MFA_REQUIRED=true` enforces MFA for all admin logins

**Code:** `api/internal/modules/auth/service.go` (MFA section)

### 8.4 Gateway Device Authentication

```
First boot:      1. Load persisted device token from file (-token-file)
                 2. If not found → register using X-Bootstrap-Token
                 3. Server returns device-specific token "gw_{random_128bit}"
                 4. Token persisted to local file (permissions 0600)
                 5. Bootstrap token no longer used for subsequent requests

Ongoing:         X-Device-Token → per-device token
                 Verified → SHA256(token) compared with stored hash
                 Comparison → crypto/subtle.ConstantTimeCompare
                 Fallback → bootstrap token (constant-time compare, dev/unregistered only)

TLS Pinning:     Gateway HTTP client verifies Cloud API certificate SPKI hash
                 Configured via -tls-pin-sha256 flag (hex-encoded SHA256)
                 Rejects all connections if no certificate matches the pin
```

**Token lifecycle:**
- Bootstrap token: used **only** for initial registration (`POST /api/v1/gateway/register`)
- Device token: used for all subsequent API calls (config pull, heartbeat, event upload)
- After registration: bootstrap token is never sent again unless device token is unavailable
- Re-registration: if device is already registered (HTTP 409), continues with bootstrap token as fallback

---

## 9. Cryptographic Operations

### 9.1 Algorithm Inventory

| Purpose | Algorithm | Key Size | Implementation |
|---------|-----------|----------|----------------|
| Password hashing | bcrypt | Cost factor 10 | golang.org/x/crypto/bcrypt |
| JWT signing | HMAC-SHA256 | 256-bit | golang-jwt/jwt/v5 |
| Device token hash | SHA256 | 256-bit | crypto/sha256 |
| HRIS vault encryption | AES-256-GCM | 256-bit | crypto/aes + cipher.NewGCM |
| Webhook verification | HMAC-SHA256 | Variable | crypto/hmac + crypto/sha256 |
| MFA | TOTP (HMAC-SHA1) | 160-bit | Standard TOTP (RFC 6238) |
| OIDC token verification | RSA (RS256/384/512) | 2048+ bit | From IdP JWKS |
| SAML assertion verification | RSA (X.509) | 2048+ bit | From IdP certificate |
| Random generation | crypto/rand | 128-256 bit | crypto/rand.Read |
| Token comparison | Constant-time | N/A | crypto/subtle.ConstantTimeCompare |
| Apple Pass signing | PKCS#7 | N/A | Apple Pass Type Certificate |
| Pass manifest integrity | SHA256 | 256-bit | crypto/sha256 |
| OTA firmware integrity | SHA256 | 256-bit | Provided by admin |
| OTA firmware authenticity | Ed25519 | 256-bit | Signature verified against embedded public key |

### 9.2 Random Number Generation

All security-critical random values use `crypto/rand` (cryptographically secure PRNG):
- Token IDs: 12-byte random hex
- Device tokens: 16-byte random hex
- NFC payloads: 8-byte random hex
- BLE tokens: 16-byte random hex
- Apple Pass serial: 12-byte random hex
- Apple Pass auth token: 16-byte random hex
- OIDC state tokens: 6-byte random hex

### 9.3 HRIS Vault Encryption Detail

```go
// Key derivation
key = SHA256(HRIS_VAULT_MASTER_KEY)  // 32 bytes

// Encryption (per secret)
nonce = crypto/rand.Read(12)         // GCM standard nonce
ciphertext = AES-256-GCM.Seal(plaintext, nonce, key)

// Storage
stored = base64(nonce) + base64(ciphertext)
```

Authenticated encryption: GCM provides both confidentiality and integrity. Tampering with ciphertext causes decryption failure.

### 9.4 TLS Certificate Pinning (Gateway Agent)

Gateway agent supports SPKI (Subject Public Key Info) certificate pinning to prevent MITM attacks using rogue CA-signed certificates.

```go
// Configuration via CLI flag:
//   -tls-pin-sha256 "hex-encoded SHA256 of Cloud API TLS cert SPKI"

// Verification (on every HTTPS connection):
for _, cert := range connectionState.PeerCertificates {
    spkiHash = SHA256(cert.RawSubjectPublicKeyInfo)
    if spkiHash == configuredPin {
        return nil  // connection allowed
    }
}
return error("TLS certificate pinning failed")
```

**How to obtain the pin:**
```bash
# Extract SPKI SHA256 from Cloud API's TLS certificate:
openssl s_client -connect api.mistyislet.com:443 </dev/null 2>/dev/null \
  | openssl x509 -pubkey -noout \
  | openssl pkey -pubin -outform DER \
  | openssl dgst -sha256 -hex
```

**Properties:**
- Pins the public key, not the certificate — survives certificate renewal with same key pair
- Falls back to standard TLS verification if `-tls-pin-sha256` is empty (development mode)
- Invalid pin format (non-hex or wrong length) triggers warning log and falls back to default TLS

### 9.5 OTA Firmware Integrity & Authenticity

OTA firmware updates support dual verification:

| Field | Algorithm | Purpose |
|-------|-----------|---------|
| `firmware_sha256` | SHA256 | Integrity — detects accidental corruption or tampering |
| `firmware_signature` | Ed25519 | Authenticity — proves firmware was signed by authorized publisher |

```
Admin uploads firmware:
  1. Computes SHA256 hash of firmware binary
  2. Signs firmware binary with Ed25519 private key (kept offline/in HSM)
  3. Creates OTA task with firmware_url, firmware_sha256, firmware_signature

Gateway downloads firmware:
  1. Downloads firmware from firmware_url over HTTPS
  2. Computes SHA256 of downloaded binary, compares with firmware_sha256
  3. Verifies Ed25519 signature against embedded public key
  4. Only applies firmware if both checks pass
```

**Signature format:** 64-byte Ed25519 signature, hex-encoded (128 hex characters).
Validated at API level: `isValidEd25519SignatureHex()` rejects malformed signatures at task creation time.

**Code:** `api/internal/modules/gateway/service.go`, `api/internal/http/routes_gateway_management.go`

---

## 10. Audit & Compliance

### 10.1 Audit Log Coverage

| Category | Actions Logged |
|----------|---------------|
| Credential | `wallet_pass_issued`, `wallet_pass_batch_issued`, `wallet_pass_status_changed` |
| Physical Card | `physical_card_inventory_created`, `physical_card_inventory_scanned` |
| Access | `access_granted`, `access_denied` (per-event in time-series table) |
| Auth | Login success/failure (via rate limit tracking) |
| Admin | `wallet_google_config_upserted`, `enterprise_jit_deprovision_applied` |
| Gateway | `gateway_event_checkpoint_reported`, `gateway_reboot` |
| Group Binding | `reference_group_lock_created`, `reference_group_lock_deleted` |

### 10.2 Audit Log Record

```json
{
  "id": "aud_a1b2c3d4e5",
  "tenant_id": "tenant_demo_jakarta",
  "actor": "admin@example.com",
  "role": "tenant_admin",
  "action": "wallet_pass_status_changed",
  "target": "wps_a3f8b2c1d4e5",
  "source": "wallet",
  "at": "2026-04-30T10:00:00Z",
  "raw": { "new_status": "revoked", "pass_id": "wps_a3f8b2c1d4e5" }
}
```

### 10.3 Access Event Log (Time-Series)

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "type": "access_granted",
  "actor": "user@example.com",
  "door_id": "door_jkt_001",
  "gateway_id": "gw_demo_001",
  "result": "access_granted",
  "at": "2026-04-30T10:00:00Z"
}
```

Table `mistypass_access_events` is time-partitioned for scalable retention and compliance queries.

### 10.4 Rate Limiting

| Scope | Limit | Window | Backend |
|-------|-------|--------|---------|
| Login (per IP) | 10 attempts | 1 minute | Redis + in-memory fallback |
| API (per IP) | 600 requests | 1 minute | Redis + in-memory fallback |
| Enterprise public | 60 requests | 1 minute | Redis + in-memory fallback |
| HRIS webhook | 240 requests | 1 minute | Redis + in-memory fallback |

Response on exceed: `HTTP 429 Too Many Requests` with `Retry-After` header.

---

## 11. Threat Mitigations

### 11.1 Credential Theft / Replay

| Threat | Mitigation |
|--------|-----------|
| Stolen NFC UID | Credential can be immediately revoked; propagates to gateway within 30s. Random-UID cards (bank cards) are inherently incompatible — each tap produces different UID, never matches registered entry |
| NFC UID cloning | Current: UID-only matching is vulnerable to cloning with specialized hardware (~$30 Proxmark). Mitigation: Gen 2 upgrade to DESFire mutual authentication (encrypted sector verification) |
| Stolen JWT token | 1-hour expiration; revocation via blacklist (immediate) |
| Stolen device token | SHA256 hashed at rest; constant-time comparison prevents timing attacks; token file permissions 0600 |
| Stolen refresh token | Single-use (deleted on refresh); bulk revocation by email |
| Replay attack on webhook | HMAC signature includes timestamp; 300-second clock skew tolerance |
| QR code sharing | Expiration time enforced (`ValidUntil`); link can be disabled (`LinkEnabled=false`) |

### 11.2 Unauthorized Access

| Threat | Mitigation |
|--------|-----------|
| Bypass tenant isolation | Every query scoped by `tenant_id`; JWT claim binding |
| Privilege escalation | RBAC middleware on every endpoint; role checked from signed JWT |
| Building scope bypass | `buildingScopeForRequest()` enforces building_admin boundaries |
| Brute-force login | Rate limiting: 10 attempts/minute per IP |
| Expired credential use | `ExpiresAt` checked at verification time (both online and offline) |

### 11.3 Infrastructure

| Threat | Mitigation |
|--------|-----------|
| Database password leak | Passwords stored as bcrypt hash (one-way) |
| Gateway token leak | Only SHA256 hash stored; plaintext returned once at registration |
| Secret exposure | HRIS secrets encrypted with AES-256-GCM; env-based key management |
| Forged firmware | Ed25519 signature verification; SHA256 integrity check; HTTPS-only download |
| Timing attack | All sensitive comparisons use `crypto/subtle.ConstantTimeCompare` |
| Input injection | JSON strict parsing; body size limit (1 MB); unknown fields rejected |
| Man-in-the-middle | All links use HTTPS/TLS; Gateway uses TLS SPKI certificate pinning; OIDC uses JWKS verification |

### 11.4 Physical Security

| Threat | Mitigation |
|--------|-----------|
| Gateway compromise | Gateway stores only credential identifiers, not secrets |
| Reader tampering | Wired RS485/GPIO connection; not network-accessible |
| Door held open | Auto-relock timer (configurable, default 5 seconds) |
| Network outage | Gateway makes local access decisions from cached rules; cache expires after configurable TTL (default 24h), denying all access until resync |

---

## Appendix A: Key File Reference

| Component | File |
|-----------|------|
| JWT Auth | `api/internal/modules/auth/service.go` |
| Auth Endpoints | `api/internal/http/routes_auth.go` |
| App Access | `api/internal/http/routes_app_access.go` |
| Credential Verify | `api/internal/http/routes_gateway_verify.go` |
| Wallet Service | `api/internal/modules/wallet/service.go` |
| Google Wallet | `api/internal/modules/wallet/google_wallet_provider.go` |
| Apple Wallet | `api/internal/modules/wallet/apple_pass_provider.go` |
| Gateway Bootstrap | `api/internal/http/routes_gateway_bootstrap.go` |
| Gateway Agent | `api/cmd/gateway-agent/agent.go` |
| Gateway Agent Entry | `api/cmd/gateway-agent/main.go` |
| NFC Reader (PC/SC) | `api/cmd/gateway-agent/reader.go` |
| Relay Control | `api/cmd/gateway-agent/relay.go` |
| NATS Bus | `api/internal/bus/commands.go` |
| HRIS Vault | `api/internal/modules/hris/vault.go` |
| HMAC Webhook | `api/internal/modules/hris/talenta/hmac.go` |
| Enterprise Auth | `api/internal/http/routes_enterprise_auth.go` |
| Router/RBAC | `api/internal/http/router.go` |
| DB Schema | `api/internal/state/sqlc/schema.sql` |
| Config | `api/internal/config/config.go` |
| Audit Service | `api/internal/modules/audit/service.go` |

## Appendix B: Comparison with Industry (Kisi Reference)

| Kisi Capability | MistyPass Equivalent | Status |
|----------------|---------------------|--------|
| Mobile credentials in secure enclave | JWT + OS Wallet (Apple Keychain / Google Wallet) | Implemented |
| At-rest encryption (AES) | AES-256-GCM for HRIS vault; bcrypt for passwords | Implemented |
| HTTPS + TLS 1.2 everywhere | All links use HTTPS/TLS | Implemented |
| Mutual authentication (device↔cloud) | Device token (SHA256 hash + constant-time compare) | Implemented (not mTLS) |
| Offline mode (BLE/NFC direct) | Gateway local decision from cached rules | Implemented |
| RBAC credential issuance | 5-level RBAC (super_admin → resident) | Implemented |
| Credential revocation | Immediate revocation, propagates within 30s | Implemented |
| Full audit trail | Access events + audit logs (time-partitioned) | Implemented |
| OTA firmware updates | SHA256 integrity + Ed25519 signature verification | Implemented |
| Supply chain (Secure Boot) | Serial number inventory + bootstrap token | Partial |
| Anti-tamper hardware | Not yet implemented | Planned |
| Certificate pinning | TLS SPKI pinning in Gateway agent HTTP client | Implemented |
| Replay protection (nonce) | Token expiration + idempotency keys; access rule cache TTL (24h) | Partial |

## Appendix C: Security Roadmap

Prioritized next steps based on current system maturity, deployment stage, and threat model.

### Immediate (Before Production Deployment)

| # | Item | Action | Dependency |
|---|------|--------|-----------|
| 1 | Google Wallet Corporate Badge API | Rewrite `google_wallet_provider.go` to use Corporate Badge API (separate from Generic Pass API). Requires NDA + Terms of Service with Google. | Google onboarding approval |
| 2 | Key management upgrade | Move `JWT_SECRET`, `HRIS_VAULT_MASTER_KEY`, `GATEWAY_BOOTSTRAP_TOKEN` from env vars to GCP Secret Manager (or equivalent managed secret store). | Deployment infrastructure |
| 3 | Logging & monitoring | Integrate Cloud API audit logs with GCP Cloud Logging / Stackdriver. Set alert rules for: abnormal access denial rates, rules cache approaching TTL, failed device registrations. | Deployment infrastructure |
| 4 | Automated security scanning | Add `gosec` (Go static analysis) and `trivy` (container vulnerability scan) to CI pipeline. | CI/CD setup |

### Short-term (1-3 Months Post-Launch)

| # | Item | Action | Rationale |
|---|------|--------|-----------|
| 5 | Per-request nonce for Gateway API | Add crypto/rand nonce + timestamp to config pull and event upload requests. Server validates nonce uniqueness via Redis (5-min dedup window). | Defense-in-depth: protects against replay if TLS is compromised |
| 6 | Apple Wallet production signing | Replace mock PKCS#7 with real Apple Pass Type Certificate signing. Implement APNs push for pass updates on revocation. | Required for App Store distribution |
| 7 | Password policy enforcement | Add minimum length (8+), complexity requirements, and breached password check (HaveIBeenPwned API k-anonymity) at user creation. | Compliance baseline |
| 8 | Penetration test | Engage external security firm for one-time pentest before enterprise sales. Focus on: credential lifecycle, gateway auth bypass, tenant isolation. | Pre-enterprise sales |

### Medium-term (3-6 Months)

| # | Item | Action | Rationale |
|---|------|--------|-----------|
| 9 | mTLS for Gateway ↔ Cloud | Issue client certificates from internal CA (e.g., step-ca or HashiCorp Vault PKI). Requires hardware with secure key storage. | Strongest device-cloud auth; prerequisite: hardware with TPM or secure enclave |
| 10 | Audit log integrity | Add HMAC-SHA256 chain to audit log entries (each entry signs previous hash). Separate audit storage from operational database. | Tamper-evident audit trail for compliance (SOC 2, ISO 27001) |
| 11 | SIEM integration | Connect audit logs and access events to SIEM platform (ELK Stack, Datadog, or Splunk). Alert on: bulk revocation attempts, credential enumeration patterns, OTA task creation outside maintenance windows. | Real-time threat detection |

### Long-term (Next-Generation Hardware)

| # | Item | Action | Rationale |
|---|------|--------|-----------|
| 12 | Secure Boot | Select SoC with TPM 2.0 or ARM TrustZone support. Implement signed bootloader chain (U-Boot signature verification). | Prevents firmware tampering at boot time |
| 13 | Anti-tamper hardware | Custom PCB with tamper-detect sensors (microswitch, accelerometer). Trigger: NATS alarm to Cloud + auto-lockdown of all doors on gateway. | Physical attack detection |
| 14 | Hardware-bound credentials | Store device token and mTLS private key in TPM/secure enclave. Keys never exportable. | Prevents credential extraction even with physical access |
| 15 | BLE challenge-response | Replace static BLE token with challenge-response protocol (gateway issues nonce, app signs with device-bound key). | Prevents BLE token replay/cloning |

### Explicitly Deprioritized

| Item | Reason |
|------|--------|
| HSM for JWT_SECRET | GCP Secret Manager provides sufficient protection at current scale. HSM cost ($1.5/hr for CloudHSM) not justified until financial/healthcare verticals. |
| HPKP (HTTP Public Key Pinning) | Deprecated browser standard (Chrome removed 2018). Our SPKI pinning in Gateway agent is the correct approach. |
| Zero-trust internal NATS mTLS | NATS runs on same host or private network. mTLS overhead not justified when network is already isolated. |
| Quarterly pentests | Replace with event-driven approach: pentest at major milestones (pre-launch, pre-enterprise sales, post-architecture change), not on fixed schedule. |
