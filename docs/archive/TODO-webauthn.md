# WebAuthn Implementation Plan (T0)

Next session task. Adds passkey/biometric login support (Kisi parity).

## Behavior (Like Kisi)

- Organization-level toggle: `Settings → Security → WebAuthn Sign-in`
- Requires SSO enabled for the organization
- Users register passkeys (Touch ID, Face ID, Windows Hello, security keys)
- Login flow: email → WebAuthn challenge → signed response → JWT issued
- Co-exists with TOTP MFA (WebAuthn replaces password, not 2FA)

## Dependencies

```bash
go get github.com/go-webauthn/webauthn@latest
```

## Data Model

```go
// Organization setting (already in organization settings)
type OrganizationSecuritySettings struct {
    WebAuthnEnabled bool `json:"webauthn_enabled"`
    // ... existing fields
}

// Per-user credential storage
type WebAuthnCredential struct {
    ID              string    `json:"id"`               // credential ID (base64url)
    UserID          string    `json:"user_id"`
    PublicKey       []byte    `json:"public_key"`       // COSE public key
    AttestationType string    `json:"attestation_type"` // "none", "packed", etc.
    AAGUID          string    `json:"aaguid"`           // authenticator ID
    SignCount       uint32    `json:"sign_count"`       // replay counter
    DisplayName     string    `json:"display_name"`     // "MacBook Touch ID", "YubiKey 5"
    CreatedAt       time.Time `json:"created_at"`
}
```

## API Endpoints

```
# Registration (adding a passkey)
POST /api/v1/auth/webauthn/register/begin     → returns PublicKeyCredentialCreationOptions
POST /api/v1/auth/webauthn/register/finish    → stores credential, returns success

# Authentication (login with passkey)
POST /api/v1/auth/webauthn/login/begin        → returns PublicKeyCredentialRequestOptions
POST /api/v1/auth/webauthn/login/finish       → verifies signature, returns JWT

# Management
GET  /api/v1/auth/webauthn/credentials        → list user's registered passkeys
DELETE /api/v1/auth/webauthn/credentials/{id}  → remove a passkey
```

## Implementation Steps

1. Add `github.com/go-webauthn/webauthn` dependency
2. Add `WebAuthnCredential` storage to auth service (in-memory + persistence)
3. Implement `webauthn.User` interface on `auth.User`
4. Add session store for WebAuthn challenges (in-memory map, 5-min TTL)
5. Add 6 HTTP handlers (register begin/finish, login begin/finish, list, delete)
6. Wire routes in `router.go` under `/auth/webauthn/`
7. Add organization-level `webauthn_enabled` check (reject if disabled or no SSO)
8. Tests

## Config

```go
// WebAuthn relying party config (from environment)
WEBAUTHN_RP_DISPLAY_NAME=MistyPass        // shown in browser prompt
WEBAUTHN_RP_ID=mistyislet.com             // domain (must match cookie domain)
WEBAUTHN_RP_ORIGINS=https://admin.mistyislet.com,https://app.mistyislet.com
```

## Security Notes

- Challenge is single-use, 5-minute TTL
- SignCount verified on each login (detects cloned credentials)
- Attestation: accept "none" (don't require hardware attestation for now)
- WebAuthn replaces password, not MFA — user still needs TOTP if MFA is enabled
- Credentials stored per-user, not per-device (passkeys are synced by OS)
