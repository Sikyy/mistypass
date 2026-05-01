// Package crypto provides AES-256-GCM encryption with HKDF-SHA256 key derivation
// for encrypting secrets at rest (TOTP secrets, API keys, etc.).
//
// Key rotation:
//   - Encrypted values are tagged with a key version: "enc:v{N}:{nonce}:{ciphertext}"
//   - Encrypt always uses the current (latest) key version
//   - Decrypt tries the tagged version first, then falls back to all known keys
//   - Re-encrypt migrates a value from any old key to the current key
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/hkdf"
)

// vaultKey is a versioned encryption key.
type vaultKey struct {
	version int
	key     []byte // 32-byte AES-256 key
}

// Vault encrypts and decrypts secrets using AES-256-GCM with versioned keys.
// Supports zero-downtime key rotation: encrypts with the latest key,
// decrypts with any known key version.
type Vault struct {
	current vaultKey
	keys    map[int]vaultKey // version -> key (includes current)
}

// NewVault creates a Vault with a single key (version 1) derived via HKDF-SHA256.
// Returns nil if masterKey is empty (encryption disabled).
func NewVault(masterKey, purpose string) *Vault {
	master := strings.TrimSpace(masterKey)
	if master == "" {
		return nil
	}
	k := vaultKey{
		version: 1,
		key:     DeriveKey(master, purpose),
	}
	return &Vault{
		current: k,
		keys:    map[int]vaultKey{1: k},
	}
}

// NewVaultWithRotation creates a Vault with a current key and optional previous keys
// for decryption during rotation. Each master key gets its own version.
//
// Usage:
//
//	vault := NewVaultWithRotation("new-master", "purpose", "old-master-1", "old-master-2")
//
// Version numbering: previousKeys[0] = v1, previousKeys[1] = v2, ..., currentKey = vN.
func NewVaultWithRotation(currentMasterKey, purpose string, previousMasterKeys ...string) *Vault {
	current := strings.TrimSpace(currentMasterKey)
	if current == "" {
		return nil
	}

	keys := make(map[int]vaultKey, len(previousMasterKeys)+1)

	// Previous keys get versions 1..N-1
	for i, prev := range previousMasterKeys {
		p := strings.TrimSpace(prev)
		if p == "" {
			continue
		}
		ver := i + 1
		keys[ver] = vaultKey{
			version: ver,
			key:     DeriveKey(p, purpose),
		}
	}

	// Current key gets the next version
	currentVersion := len(previousMasterKeys) + 1
	k := vaultKey{
		version: currentVersion,
		key:     DeriveKey(current, purpose),
	}
	keys[currentVersion] = k

	return &Vault{
		current: k,
		keys:    keys,
	}
}

// DeriveKey derives a 32-byte key from a master key using HKDF-SHA256.
func DeriveKey(masterKey, purpose string) []byte {
	salt := sha256.Sum256([]byte(purpose))
	reader := hkdf.New(sha256.New, []byte(masterKey), salt[:], []byte("mistypass/"+purpose))
	key := make([]byte, 32)
	if _, err := io.ReadFull(reader, key); err != nil {
		panic("hkdf key derivation failed: " + err.Error())
	}
	return key
}

// Encrypt encrypts plaintext with AES-256-GCM using the current key.
// Returns base64-encoded nonce and ciphertext.
func (v *Vault) Encrypt(plaintext string) (nonce string, ciphertext string, err error) {
	if v == nil {
		return "", "", errors.New("vault is not configured")
	}
	return encryptWithKey(v.current, plaintext)
}

// Decrypt decrypts base64-encoded nonce+ciphertext with AES-256-GCM.
func (v *Vault) Decrypt(nonce, ciphertext string) (string, error) {
	if v == nil {
		return "", errors.New("vault is not configured")
	}
	// Try current key first (most common case)
	pt, err := decryptWithKey(v.current, nonce, ciphertext)
	if err == nil {
		return pt, nil
	}
	// Fallback: try all other keys
	for ver, k := range v.keys {
		if ver == v.current.version {
			continue
		}
		if pt, err := decryptWithKey(k, nonce, ciphertext); err == nil {
			return pt, nil
		}
	}
	return "", fmt.Errorf("decrypt failed with all %d key versions", len(v.keys))
}

// EncryptVersioned encrypts and returns a version-tagged string: "enc:v{N}:{nonce}:{ct}"
func (v *Vault) EncryptVersioned(plaintext string) (string, error) {
	if v == nil {
		return "", errors.New("vault is not configured")
	}
	nonce, ct, err := encryptWithKey(v.current, plaintext)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("enc:v%d:%s:%s", v.current.version, nonce, ct), nil
}

// DecryptVersioned decrypts a version-tagged string. Supports:
//   - "enc:v{N}:{nonce}:{ct}" — versioned format (tries tagged version first)
//   - "enc:{nonce}:{ct}" — legacy unversioned format (tries all keys)
//   - plaintext without "enc:" prefix — returned as-is (backward compat)
func (v *Vault) DecryptVersioned(value string) (string, error) {
	if v == nil {
		return "", errors.New("vault is not configured")
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if !strings.HasPrefix(trimmed, "enc:") {
		return trimmed, nil // plaintext backward compat
	}

	parts := strings.SplitN(trimmed[4:], ":", 3) // strip "enc:" prefix

	// Versioned format: "v{N}:{nonce}:{ct}"
	if len(parts) == 3 && strings.HasPrefix(parts[0], "v") {
		ver, err := strconv.Atoi(parts[0][1:])
		if err == nil {
			if k, ok := v.keys[ver]; ok {
				pt, err := decryptWithKey(k, parts[1], parts[2])
				if err == nil {
					return pt, nil
				}
			}
			// Tagged version failed — fall through to try all keys
		}
		return v.Decrypt(parts[1], parts[2])
	}

	// Legacy format: "{nonce}:{ct}" (no version tag)
	if len(parts) == 2 {
		return v.Decrypt(parts[0], parts[1])
	}

	return "", fmt.Errorf("invalid encrypted value format")
}

// NeedsReEncrypt checks if a value was encrypted with an older key version.
// Returns true if re-encryption with the current key is recommended.
func (v *Vault) NeedsReEncrypt(value string) bool {
	if v == nil {
		return false
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false // empty values don't need encryption
	}
	if !strings.HasPrefix(trimmed, "enc:") {
		// Plaintext — needs encryption
		return true
	}
	parts := strings.SplitN(trimmed[4:], ":", 3)
	if len(parts) == 3 && strings.HasPrefix(parts[0], "v") {
		ver, err := strconv.Atoi(parts[0][1:])
		if err == nil && ver == v.current.version {
			return false // already on current version
		}
	}
	return true // old version or legacy format
}

// ReEncrypt decrypts a value (any version or plaintext) and re-encrypts with the current key.
func (v *Vault) ReEncrypt(value string) (string, error) {
	if v == nil {
		return "", errors.New("vault is not configured")
	}
	plaintext, err := v.DecryptVersioned(value)
	if err != nil {
		return "", fmt.Errorf("re-encrypt decrypt: %w", err)
	}
	if plaintext == "" {
		return "", nil
	}
	return v.EncryptVersioned(plaintext)
}

// CurrentVersion returns the current key version used for encryption.
func (v *Vault) CurrentVersion() int {
	if v == nil {
		return 0
	}
	return v.current.version
}

// KeyVersionCount returns the number of key versions loaded.
func (v *Vault) KeyVersionCount() int {
	if v == nil {
		return 0
	}
	return len(v.keys)
}

// IsConfigured returns true if the vault has at least one key configured.
func (v *Vault) IsConfigured() bool {
	return v != nil && len(v.keys) > 0
}

func encryptWithKey(k vaultKey, plaintext string) (nonce string, ciphertext string, err error) {
	block, err := aes.NewCipher(k.key)
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

func decryptWithKey(k vaultKey, nonce, ciphertext string) (string, error) {
	block, err := aes.NewCipher(k.key)
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
