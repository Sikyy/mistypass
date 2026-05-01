// Package crypto provides AES-256-GCM encryption with HKDF-SHA256 key derivation
// for encrypting secrets at rest (TOTP secrets, API keys, etc.).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"

	"golang.org/x/crypto/hkdf"
)

// Vault encrypts and decrypts secrets using AES-256-GCM.
// Key is derived from a master key using HKDF-SHA256 with a purpose-specific info string.
type Vault struct {
	key []byte // 32-byte AES-256 key
}

// NewVault creates a Vault with a key derived via HKDF-SHA256 from the master key.
// The purpose string provides domain separation (e.g. "totp-secrets", "api-keys").
// Returns nil if masterKey is empty (encryption disabled).
func NewVault(masterKey, purpose string) *Vault {
	master := strings.TrimSpace(masterKey)
	if master == "" {
		return nil
	}
	key := DeriveKey(master, purpose)
	return &Vault{key: key}
}

// DeriveKey derives a 32-byte key from a master key using HKDF-SHA256.
// The purpose string provides domain separation so different uses of the same
// master key produce different derived keys.
func DeriveKey(masterKey, purpose string) []byte {
	// HKDF extract + expand with SHA256
	// salt = SHA256(purpose) — deterministic but unique per purpose
	salt := sha256.Sum256([]byte(purpose))
	reader := hkdf.New(sha256.New, []byte(masterKey), salt[:], []byte("mistypass/"+purpose))
	key := make([]byte, 32)
	if _, err := io.ReadFull(reader, key); err != nil {
		// HKDF should never fail with valid inputs
		panic("hkdf key derivation failed: " + err.Error())
	}
	return key
}

// Encrypt encrypts plaintext with AES-256-GCM.
// Returns base64-encoded nonce and ciphertext.
func (v *Vault) Encrypt(plaintext string) (nonce string, ciphertext string, err error) {
	if v == nil {
		return "", "", errors.New("vault is not configured (missing master key)")
	}
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	rawNonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, rawNonce); err != nil {
		return "", "", err
	}
	sealed := gcm.Seal(nil, rawNonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(rawNonce),
		base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts base64-encoded nonce+ciphertext with AES-256-GCM.
func (v *Vault) Decrypt(nonce, ciphertext string) (string, error) {
	if v == nil {
		return "", errors.New("vault is not configured (missing master key)")
	}
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	rawNonce, err := base64.StdEncoding.DecodeString(strings.TrimSpace(nonce))
	if err != nil {
		return "", err
	}
	rawCiphertext, err := base64.StdEncoding.DecodeString(strings.TrimSpace(ciphertext))
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, rawNonce, rawCiphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// IsConfigured returns true if the vault has a key configured.
func (v *Vault) IsConfigured() bool {
	return v != nil && len(v.key) == 32
}
