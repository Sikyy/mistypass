package credential

import "time"

// MobileCredential represents a registered mobile device's public key and metadata.
// The private key never leaves the device (Android Keystore / iOS Secure Enclave).
type MobileCredential struct {
	ID            string     `json:"id"`
	TenantID      string     `json:"tenant_id"`
	UserID        string     `json:"user_id"`
	UserEmail     string     `json:"user_email"`
	DeviceID      string     `json:"device_id"`
	Platform      string     `json:"platform"`       // "android" | "ios"
	DeviceModel   string     `json:"device_model"`   // e.g. "Samsung Galaxy A54"
	KeystoreLevel string     `json:"keystore_level"` // "strongbox" | "tee" | "software"
	PublicKeyPEM  string     `json:"public_key_pem"` // EC P-256 public key in PEM format
	Status        string     `json:"status"`         // "active" | "revoked" | "expired"
	IssuedAt      time.Time  `json:"issued_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
}

// RegisterDeviceInput is the request payload when a mobile app registers its keypair.
type RegisterDeviceInput struct {
	TenantID             string   `json:"tenant_id"`
	UserID               string   `json:"user_id"`
	UserEmail            string   `json:"user_email"`
	PublicKeyPEM         string   `json:"public_key_pem"`
	Platform             string   `json:"platform"`
	DeviceID             string   `json:"device_id"`
	DeviceModel          string   `json:"device_model"`
	KeystoreLevel        string   `json:"keystore_level"`
	AttestationCertChain []string `json:"attestation_cert_chain"` // DER-encoded X.509 cert chain (base64)
}

// BLEChallenge is issued by a Gateway/reader for a BLE authentication handshake.
type BLEChallenge struct {
	Nonce     []byte `json:"nonce"`      // 32 bytes cryptographically random
	ReaderID  string `json:"reader_id"`  // identifies which reader issued this
	IssuedAt  int64  `json:"issued_at"`  // unix timestamp
	ExpiresAt int64  `json:"expires_at"` // unix timestamp (typically +30s)
}

// BLEAuthRequest is what the mobile app sends back after signing the challenge.
type BLEAuthRequest struct {
	UserID    string `json:"user_id"`
	Nonce     []byte `json:"nonce"`     // echo back the challenge nonce
	Signature []byte `json:"signature"` // ECDSA signature of SHA256(nonce || user_id)
}

// BLEAuthResult is returned by the reader/controller after verification.
type BLEAuthResult struct {
	Decision string `json:"decision"` // "allow" | "deny"
	Reason   string `json:"reason"`   // "access_granted", "signature_invalid", "credential_revoked", etc.
	UserID   string `json:"user_id,omitempty"`
}

// GatewayCredentialSync is the payload sent to gateways during config pull,
// containing all active mobile credentials for local BLE verification.
type GatewayCredentialSync struct {
	UserID       string   `json:"user_id"`
	UserEmail    string   `json:"user_email"`
	PublicKeyPEM string   `json:"public_key_pem"`
	LockIDs      []string `json:"lock_ids"`
	ExpiresAt    int64    `json:"expires_at"`
	RevokedAt    *int64   `json:"revoked_at,omitempty"`
	SuspendedAt  *int64   `json:"suspended_at,omitempty"`
	SyncVersion  int64    `json:"sync_version"`
}

// TTL constants based on keystore security level.
const (
	TTLStrongBox = 90 * 24 * time.Hour // 90 days
	TTLTee       = 30 * 24 * time.Hour // 30 days
	TTLSoftware  = 7 * 24 * time.Hour  // 7 days
)

// CredentialTTL returns the appropriate TTL based on the keystore security level.
func CredentialTTL(keystoreLevel string) time.Duration {
	switch keystoreLevel {
	case "strongbox":
		return TTLStrongBox
	case "tee":
		return TTLTee
	default:
		return TTLSoftware
	}
}
