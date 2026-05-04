package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// BLE GATT Service and Characteristic UUIDs (128-bit custom UUIDs).
// These define the MistyPass BLE authentication protocol.
const (
	// MistyPass BLE Auth Service UUID
	BLEServiceUUID = "4d495354-5950-4153-532d-424c45415554" // "MISTYPASS-BLEAUT"

	// CHALLENGE characteristic: Reader → Phone (Read)
	// Contains: [32 bytes nonce] + [8 bytes timestamp] + [8 bytes expiry]
	BLECharChallengeUUID = "4d495354-5950-4153-532d-4348414c4c4e" // "MISTYPASS-CHALLN"

	// AUTH_RESPONSE characteristic: Phone → Reader (Write)
	// Contains: [variable user_id length byte] + [user_id bytes] + [signature bytes]
	BLECharAuthResponseUUID = "4d495354-5950-4153-532d-41555448524553" // "MISTYPASS-AUTHRES"

	// READER_IDENTITY characteristic: Reader → Phone (Read)
	// Contains: reader_id string (for phone to verify reader identity)
	BLECharReaderIdentityUUID = "4d495354-5950-4153-532d-52454144455249" // "MISTYPASS-READERI"

	// AUTH_RESULT characteristic: Reader → Phone (Notify)
	// Contains: [1 byte result code] + [variable reason string]
	BLECharAuthResultUUID = "4d495354-5950-4153-532d-524553554c5400" // "MISTYPASS-RESULT"
)

// BLE Auth result codes
const (
	BLEResultGranted          byte = 0x01
	BLEResultDenied           byte = 0x02
	BLEResultExpiredChallenge byte = 0x03
	BLEResultInvalidSignature byte = 0x04
	BLEResultUnknownUser      byte = 0x05
	BLEResultCredentialExpired byte = 0x06
)

// challengeValidDuration is how long a BLE challenge nonce remains valid.
const challengeValidDuration = 30 * time.Second

// BLEChallenge represents a challenge issued by the reader to a phone.
type BLEChallenge struct {
	Nonce     [32]byte
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// NewBLEChallenge generates a fresh challenge with a cryptographic nonce.
func NewBLEChallenge() (*BLEChallenge, error) {
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	now := time.Now()
	return &BLEChallenge{
		Nonce:     nonce,
		IssuedAt:  now,
		ExpiresAt: now.Add(challengeValidDuration),
	}, nil
}

// Encode serializes the challenge for the CHALLENGE characteristic.
// Format: [32 bytes nonce][8 bytes issued_at unix][8 bytes expires_at unix]
func (c *BLEChallenge) Encode() []byte {
	buf := make([]byte, 48) // 32 + 8 + 8
	copy(buf[:32], c.Nonce[:])
	binary.BigEndian.PutUint64(buf[32:40], uint64(c.IssuedAt.Unix()))
	binary.BigEndian.PutUint64(buf[40:48], uint64(c.ExpiresAt.Unix()))
	return buf
}

// IsExpired returns true if the challenge has passed its validity window.
func (c *BLEChallenge) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// BLEAuthResponse represents a signed response from the phone.
type BLEAuthResponse struct {
	UserID    string
	Signature []byte // ECDSA signature over SHA256(nonce || userID)
}

// DecodeBLEAuthResponse parses a raw AUTH_RESPONSE characteristic value.
// Format: [1 byte userID length][userID bytes][remaining = signature]
func DecodeBLEAuthResponse(data []byte) (*BLEAuthResponse, error) {
	if len(data) < 3 { // minimum: 1 byte len + 1 byte userID + 1 byte sig
		return nil, errors.New("auth response too short")
	}

	userIDLen := int(data[0])
	if userIDLen == 0 || 1+userIDLen >= len(data) {
		return nil, errors.New("invalid user_id length in auth response")
	}

	userID := string(data[1 : 1+userIDLen])
	signature := data[1+userIDLen:]

	if len(signature) == 0 {
		return nil, errors.New("empty signature in auth response")
	}

	return &BLEAuthResponse{
		UserID:    userID,
		Signature: signature,
	}, nil
}

// EncodeBLEAuthResponse serializes an auth response (used in tests/simulator).
func EncodeBLEAuthResponse(userID string, signature []byte) []byte {
	buf := make([]byte, 1+len(userID)+len(signature))
	buf[0] = byte(len(userID))
	copy(buf[1:1+len(userID)], userID)
	copy(buf[1+len(userID):], signature)
	return buf
}

// BLEAuthResult represents the authentication result sent back to the phone.
type BLEAuthResult struct {
	Code   byte
	Reason string
}

// Encode serializes the result for the AUTH_RESULT characteristic.
// Format: [1 byte code][reason string bytes]
func (r *BLEAuthResult) Encode() []byte {
	buf := make([]byte, 1+len(r.Reason))
	buf[0] = r.Code
	copy(buf[1:], r.Reason)
	return buf
}

// --- Signature Verification (pure Go, no hardware dependency) ---

// VerifyBLESignature verifies an ECDSA signature from a BLE auth response.
// The message signed is SHA256(nonce || userID).
// Supports both ASN.1 DER format and raw r||s (64 bytes) format.
func VerifyBLESignature(publicKeyPEM string, nonce [32]byte, userID string, signature []byte) error {
	pubKey, err := parseBLEPublicKey(publicKeyPEM)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}

	// Compute message hash: SHA256(nonce || userID)
	message := append(nonce[:], []byte(userID)...)
	hash := sha256.Sum256(message)

	// Try ASN.1 DER format first (standard ECDSA signature encoding)
	if ecdsa.VerifyASN1(pubKey, hash[:], signature) {
		return nil
	}

	// Try raw r||s format (64 bytes for P-256: r=32 + s=32)
	// Some Android implementations use this format
	if len(signature) == 64 {
		r := new(big.Int).SetBytes(signature[:32])
		s := new(big.Int).SetBytes(signature[32:])
		if ecdsa.Verify(pubKey, hash[:], r, s) {
			return nil
		}
	}

	return errors.New("ECDSA signature verification failed")
}

// parseBLEPublicKey parses a PEM-encoded EC P-256 public key.
func parseBLEPublicKey(pemStr string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	ecPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("not an EC public key")
	}
	if ecPub.Curve != elliptic.P256() {
		return nil, fmt.Errorf("expected P-256, got %s", ecPub.Curve.Params().Name)
	}

	return ecPub, nil
}
