package credential

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"
)

// StateStore is the persistence interface (same pattern as other modules).
type StateStore interface {
	Load(key string, dst any) (bool, error)
	Save(key string, value any) error
}

const stateKey = "module_credential"

type stateSnapshot struct {
	Credentials []MobileCredential `json:"credentials"`
}

// Service manages mobile credential lifecycle: registration, verification,
// revocation, and public key distribution to gateways.
type Service struct {
	mu          sync.RWMutex
	credentials []MobileCredential
	stateStore  StateStore
}

func NewService() *Service {
	now := time.Now().UTC()
	lastUsed := now.Add(-2 * time.Hour)
	return &Service{
		credentials: []MobileCredential{
			{
				ID:            "cred_andri_android",
				TenantID:      "tenant_demo_jakarta",
				UserID:        "usr_1001",
				UserEmail:     "andri.pratama@mistypass.local",
				DeviceID:      "dev_pixel8_001",
				Platform:      "android",
				DeviceModel:   "Google Pixel 8",
				KeystoreLevel: "strongbox",
				PublicKeyPEM:  "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEDemo1234567890abcdef\n-----END PUBLIC KEY-----",
				Status:        "active",
				IssuedAt:      now.Add(-30 * 24 * time.Hour),
				ExpiresAt:     now.Add(60 * 24 * time.Hour),
				LastUsedAt:    &lastUsed,
			},
			{
				ID:            "cred_rina_android",
				TenantID:      "tenant_demo_factory",
				UserID:        "usr_1002",
				UserEmail:     "rina.hartono@mistypass.local",
				DeviceID:      "dev_samsung_a54_001",
				Platform:      "android",
				DeviceModel:   "Samsung Galaxy A54",
				KeystoreLevel: "tee",
				PublicKeyPEM:  "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEDemo0987654321fedcba\n-----END PUBLIC KEY-----",
				Status:        "active",
				IssuedAt:      now.Add(-14 * 24 * time.Hour),
				ExpiresAt:     now.Add(16 * 24 * time.Hour),
				LastUsedAt:    &lastUsed,
			},
			{
				ID:            "cred_admin_ios",
				TenantID:      "tenant_demo_jakarta",
				UserID:        "usr_building_admin_jkt_001",
				UserEmail:     "building.admin.sudirman@mistypass.local",
				DeviceID:      "dev_iphone15_001",
				Platform:      "ios",
				DeviceModel:   "iPhone 15 Pro",
				KeystoreLevel: "strongbox",
				PublicKeyPEM:  "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEDemoAdminKey12345678\n-----END PUBLIC KEY-----",
				Status:        "active",
				IssuedAt:      now.Add(-60 * 24 * time.Hour),
				ExpiresAt:     now.Add(30 * 24 * time.Hour),
				LastUsedAt:    &lastUsed,
			},
		},
	}
}

func NewServiceWithStateStore(store StateStore) (*Service, error) {
	svc := NewService()
	svc.stateStore = store
	if err := svc.restoreFromStateStore(); err != nil {
		return nil, err
	}
	return svc, nil
}

// --- Registration ---

// RegisterDevice validates the input and stores the mobile credential.
// The public key is verified to be a valid EC P-256 key.
func (s *Service) RegisterDevice(input RegisterDeviceInput) (*MobileCredential, error) {
	if err := s.validateRegisterInput(input); err != nil {
		return nil, err
	}

	// Parse and validate the public key
	if _, err := ParseECPublicKey(input.PublicKeyPEM); err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}

	// Validate attestation (if provided)
	if len(input.AttestationCertChain) > 0 {
		if err := ValidateAttestation(input.AttestationCertChain, input.PublicKeyPEM); err != nil {
			return nil, fmt.Errorf("attestation validation failed: %w", err)
		}
	}

	now := time.Now().UTC()
	ttl := CredentialTTL(input.KeystoreLevel)

	cred := MobileCredential{
		ID:            generateCredentialID(),
		TenantID:      input.TenantID,
		UserID:        input.UserID,
		UserEmail:     input.UserEmail,
		DeviceID:      input.DeviceID,
		Platform:      input.Platform,
		DeviceModel:   input.DeviceModel,
		KeystoreLevel: input.KeystoreLevel,
		PublicKeyPEM:  input.PublicKeyPEM,
		Status:        "active",
		IssuedAt:      now,
		ExpiresAt:     now.Add(ttl),
	}

	s.mu.Lock()
	// Revoke any existing credential for same user+device (re-registration)
	for i := range s.credentials {
		if s.credentials[i].TenantID == input.TenantID &&
			s.credentials[i].UserID == input.UserID &&
			s.credentials[i].DeviceID == input.DeviceID &&
			s.credentials[i].Status == "active" {
			revokedAt := now
			s.credentials[i].Status = "revoked"
			s.credentials[i].RevokedAt = &revokedAt
		}
	}
	s.credentials = append(s.credentials, cred)
	s.persistLocked()
	s.mu.Unlock()

	return &cred, nil
}

// --- Query ---

// GetCredential returns a credential by ID.
func (s *Service) GetCredential(tenantID, credentialID string) (*MobileCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.credentials {
		if s.credentials[i].ID == credentialID && s.credentials[i].TenantID == tenantID {
			return &s.credentials[i], nil
		}
	}
	return nil, errors.New("credential not found")
}

// ListUserCredentials returns all credentials for a user.
func (s *Service) ListUserCredentials(tenantID, userID string) []MobileCredential {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []MobileCredential
	for _, c := range s.credentials {
		if c.TenantID == tenantID && c.UserID == userID {
			result = append(result, c)
		}
	}
	return result
}

// ListActiveCredentials returns all active (non-revoked, non-expired) credentials for a tenant.
// Used by gateway config pull to sync public keys.
func (s *Service) ListActiveCredentials(tenantID string) []MobileCredential {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UTC()
	var result []MobileCredential
	for _, c := range s.credentials {
		if c.TenantID == tenantID && c.Status == "active" && c.ExpiresAt.After(now) {
			result = append(result, c)
		}
	}
	return result
}

// --- Revocation ---

// RevokeCredential immediately marks a credential as revoked.
func (s *Service) RevokeCredential(tenantID, credentialID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.credentials {
		if s.credentials[i].ID == credentialID && s.credentials[i].TenantID == tenantID {
			if s.credentials[i].Status == "revoked" {
				return errors.New("credential already revoked")
			}
			now := time.Now().UTC()
			s.credentials[i].Status = "revoked"
			s.credentials[i].RevokedAt = &now
			s.persistLocked()
			return nil
		}
	}
	return errors.New("credential not found")
}

// SuspendCredential temporarily suspends a credential (can be reactivated later).
func (s *Service) SuspendCredential(tenantID, credentialID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.credentials {
		if s.credentials[i].ID == credentialID && s.credentials[i].TenantID == tenantID {
			if s.credentials[i].Status == "revoked" {
				return errors.New("cannot suspend a revoked credential")
			}
			if s.credentials[i].Status == "suspended" {
				return errors.New("credential already suspended")
			}
			s.credentials[i].Status = "suspended"
			s.persistLocked()
			return nil
		}
	}
	return errors.New("credential not found")
}

// ActivateCredential reactivates a suspended credential.
func (s *Service) ActivateCredential(tenantID, credentialID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.credentials {
		if s.credentials[i].ID == credentialID && s.credentials[i].TenantID == tenantID {
			if s.credentials[i].Status == "revoked" {
				return errors.New("cannot activate a revoked credential")
			}
			if s.credentials[i].Status == "active" {
				return errors.New("credential already active")
			}
			s.credentials[i].Status = "active"
			s.persistLocked()
			return nil
		}
	}
	return errors.New("credential not found")
}

// RevokeAllUserCredentials revokes all active credentials for a user (e.g. on employee termination).
func (s *Service) RevokeAllUserCredentials(tenantID, userID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	count := 0
	for i := range s.credentials {
		if s.credentials[i].TenantID == tenantID &&
			s.credentials[i].UserID == userID &&
			s.credentials[i].Status == "active" {
			s.credentials[i].Status = "revoked"
			s.credentials[i].RevokedAt = &now
			count++
		}
	}
	if count > 0 {
		s.persistLocked()
	}
	return count
}

// --- BLE Signature Verification ---

// VerifyBLESignature verifies an ECDSA signature from a BLE authentication handshake.
// The signature is over SHA256(nonce || userID).
// Returns the matched credential on success.
func (s *Service) VerifyBLESignature(tenantID, userID string, nonce, signature []byte) (*MobileCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UTC()

	// Find active credential for this user
	var activeCred *MobileCredential
	for i := range s.credentials {
		c := &s.credentials[i]
		if c.TenantID == tenantID && c.UserID == userID && c.Status == "active" && c.ExpiresAt.After(now) {
			activeCred = c
			break
		}
	}

	if activeCred == nil {
		return nil, errors.New("no active credential found for user")
	}

	// Parse the public key
	pubKey, err := ParseECPublicKey(activeCred.PublicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("stored public key is invalid: %w", err)
	}

	// Verify ECDSA signature over SHA256(nonce || userID)
	message := append(nonce, []byte(userID)...)
	hash := sha256.Sum256(message)

	if !ecdsa.VerifyASN1(pubKey, hash[:], signature) {
		return nil, errors.New("signature verification failed")
	}

	return activeCred, nil
}

// VerifyBLESignatureV2 verifies an ECDSA signature that is transport-tag-bound.
// The signature is over SHA256(nonce || userID || transportTag).
// transportTag must match what was committed at sign time (e.g. "BLE" or "NFC_HCE").
// Returns the matched credential on success.
func (s *Service) VerifyBLESignatureV2(tenantID, userID string, nonce []byte, transportTag string, signature []byte) (*MobileCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UTC()

	// Find active credential for this user
	var activeCred *MobileCredential
	for i := range s.credentials {
		c := &s.credentials[i]
		if c.TenantID == tenantID && c.UserID == userID && c.Status == "active" && c.ExpiresAt.After(now) {
			activeCred = c
			break
		}
	}

	if activeCred == nil {
		return nil, errors.New("no active credential found for user")
	}

	// Parse the public key
	pubKey, err := ParseECPublicKey(activeCred.PublicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("stored public key is invalid: %w", err)
	}

	// Construct transport-bound message: nonce || userID || transportTag
	// Pre-allocate to avoid aliasing the caller's nonce slice.
	msg := make([]byte, 0, len(nonce)+len(userID)+len(transportTag))
	msg = append(msg, nonce...)
	msg = append(msg, []byte(userID)...)
	msg = append(msg, []byte(transportTag)...)
	hash := sha256.Sum256(msg)

	// Try ASN.1 DER signature first
	if ecdsa.VerifyASN1(pubKey, hash[:], signature) {
		return activeCred, nil
	}

	// Fall back to raw r||s (64 bytes) — Android may use this encoding
	if len(signature) == 64 {
		r := new(big.Int).SetBytes(signature[:32])
		sigS := new(big.Int).SetBytes(signature[32:])
		if ecdsa.Verify(pubKey, hash[:], r, sigS) {
			return activeCred, nil
		}
	}

	return nil, fmt.Errorf("v2 signature verification failed (transport=%s)", transportTag)
}

// --- Credential Refresh ---

// RefreshCredential extends the expiry of an active credential.
// Called when the app periodically re-attests while online.
func (s *Service) RefreshCredential(tenantID, credentialID string) (*MobileCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for i := range s.credentials {
		c := &s.credentials[i]
		if c.ID == credentialID && c.TenantID == tenantID {
			if c.Status != "active" {
				return nil, errors.New("credential is not active")
			}
			ttl := CredentialTTL(c.KeystoreLevel)
			c.ExpiresAt = now.Add(ttl)
			s.persistLocked()
			return c, nil
		}
	}
	return nil, errors.New("credential not found")
}

// --- Touch (update last used) ---

// TouchCredential updates the last_used_at timestamp (called after successful BLE auth).
func (s *Service) TouchCredential(tenantID, credentialID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for i := range s.credentials {
		if s.credentials[i].ID == credentialID && s.credentials[i].TenantID == tenantID {
			s.credentials[i].LastUsedAt = &now
			break
		}
	}
	// Don't persist on every touch — batched during next state save
}

// --- Gateway Sync ---

// BuildGatewayCredentialSync returns the credential data needed by a specific gateway
// for local BLE verification. Requires the list of lock IDs the gateway controls.
func (s *Service) BuildGatewayCredentialSync(tenantID string, gatewayLockIDs []string, userLockAccess map[string][]string) []GatewayCredentialSync {
	active := s.ListActiveCredentials(tenantID)

	var result []GatewayCredentialSync
	for _, cred := range active {
		// Find which of this gateway's locks the user can access
		userLocks, ok := userLockAccess[cred.UserID]
		if !ok {
			continue
		}
		var matchedLocks []string
		for _, gwLock := range gatewayLockIDs {
			for _, uLock := range userLocks {
				if gwLock == uLock {
					matchedLocks = append(matchedLocks, gwLock)
					break
				}
			}
		}
		if len(matchedLocks) == 0 {
			continue
		}
		result = append(result, GatewayCredentialSync{
			UserID:       cred.UserID,
			UserEmail:    cred.UserEmail,
			PublicKeyPEM: cred.PublicKeyPEM,
			LockIDs:      matchedLocks,
			ExpiresAt:    cred.ExpiresAt.Unix(),
		})
	}
	return result
}

// --- Helpers ---

func (s *Service) validateRegisterInput(input RegisterDeviceInput) error {
	if input.TenantID == "" {
		return errors.New("tenant_id is required")
	}
	if input.UserID == "" {
		return errors.New("user_id is required")
	}
	if input.PublicKeyPEM == "" {
		return errors.New("public_key_pem is required")
	}
	if input.DeviceID == "" {
		return errors.New("device_id is required")
	}
	platform := strings.ToLower(input.Platform)
	if platform != "android" && platform != "ios" {
		return errors.New("platform must be 'android' or 'ios'")
	}
	level := strings.ToLower(input.KeystoreLevel)
	if level != "strongbox" && level != "tee" && level != "software" {
		return errors.New("keystore_level must be 'strongbox', 'tee', or 'software'")
	}
	return nil
}

func (s *Service) persistLocked() {
	if s.stateStore == nil {
		return
	}
	snap := stateSnapshot{
		Credentials: s.credentials,
	}
	if err := s.stateStore.Save(stateKey, snap); err != nil {
		slog.Warn("credential persist failed", "error", err)
	}
}

func (s *Service) restoreFromStateStore() error {
	if s.stateStore == nil {
		return nil
	}
	var snap stateSnapshot
	found, err := s.stateStore.Load(stateKey, &snap)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	s.credentials = snap.Credentials
	if s.credentials == nil {
		s.credentials = []MobileCredential{}
	}
	return nil
}

// ParseECPublicKey parses a PEM-encoded EC P-256 public key.
func ParseECPublicKey(pemStr string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	ecPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("key is not an EC public key")
	}

	if ecPub.Curve != elliptic.P256() {
		return nil, fmt.Errorf("expected P-256 curve, got %s", ecPub.Curve.Params().Name)
	}

	return ecPub, nil
}

func generateCredentialID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return fmt.Sprintf("mcred_%x", b)
}

// GenerateNonce creates a cryptographically random 32-byte nonce for BLE challenges.
func GenerateNonce() ([]byte, error) {
	nonce := make([]byte, 32)
	_, err := rand.Read(nonce)
	if err != nil {
		return nil, err
	}
	return nonce, nil
}

// VerifyECDSASignature is a standalone verification function usable by the gateway agent
// without the full Service (for offline local verification).
func VerifyECDSASignature(publicKeyPEM string, nonce []byte, userID string, signature []byte) (bool, error) {
	pubKey, err := ParseECPublicKey(publicKeyPEM)
	if err != nil {
		return false, err
	}

	message := append(nonce, []byte(userID)...)
	hash := sha256.Sum256(message)

	// Parse ASN.1 DER signature
	if ecdsa.VerifyASN1(pubKey, hash[:], signature) {
		return true, nil
	}

	// Try raw r||s format (64 bytes) as Android sometimes uses this
	if len(signature) == 64 {
		r := new(big.Int).SetBytes(signature[:32])
		sigS := new(big.Int).SetBytes(signature[32:])
		if ecdsa.Verify(pubKey, hash[:], r, sigS) {
			return true, nil
		}
	}

	return false, nil
}
