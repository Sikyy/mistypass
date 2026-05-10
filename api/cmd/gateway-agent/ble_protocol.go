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
	BLECharAuthResponseUUID = "4d495354-5950-4153-532d-415554485245" // "MISTYPASS-AUTHRE"

	// READER_IDENTITY characteristic: Reader → Phone (Read)
	// Contains: reader_id string (for phone to verify reader identity)
	BLECharReaderIdentityUUID = "4d495354-5950-4153-532d-524541444552" // "MISTYPASS-READER"

	// AUTH_RESULT characteristic: Reader → Phone (Notify)
	// Contains: [1 byte result code] + [variable reason string]
	BLECharAuthResultUUID = "4d495354-5950-4153-532d-524553554c54" // "MISTYPASS-RESULT"
)

// BLE Auth result codes
const (
	BLEResultGranted              byte = 0x01
	BLEResultDenied               byte = 0x02
	BLEResultExpiredChallenge     byte = 0x03
	BLEResultInvalidSignature     byte = 0x04
	BLEResultUnknownUser          byte = 0x05
	BLEResultCredentialExpired    byte = 0x06
	BLEResultCredentialRevoked    byte = 0x07
	BLEResultCredentialSuspended  byte = 0x08
	BLEResultGatewayOfflineLimit  byte = 0x09
)

// ChallengeV2Size is the byte length of an encoded BLEChallengeV2.
// Layout: 32B nonce + 8B issued_at + 8B expires_at + 4B gateway_id = 52 bytes.
const ChallengeV2Size = 52

// Transport tag constants identify the bearer channel in v2 signatures,
// preventing cross-transport replay attacks.
const (
	TransportTagBLE    = "BLE"
	TransportTagNFCHCE = "NFC_HCE"
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

// --- Protocol v2 ---

// BLEChallengeV2 extends the v1 challenge with a GatewayID field.
// This allows the signing phone to bind its response to a specific gateway,
// providing cross-gateway replay protection in addition to the nonce.
type BLEChallengeV2 struct {
	Nonce     [32]byte
	IssuedAt  time.Time
	ExpiresAt time.Time
	GatewayID uint32
}

// NewBLEChallengeV2 generates a fresh v2 challenge for the given gateway.
func NewBLEChallengeV2(gatewayID uint32) (*BLEChallengeV2, error) {
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	now := time.Now()
	return &BLEChallengeV2{
		Nonce:     nonce,
		IssuedAt:  now,
		ExpiresAt: now.Add(challengeValidDuration),
		GatewayID: gatewayID,
	}, nil
}

// Encode serializes the v2 challenge to ChallengeV2Size (52) bytes.
// Format: [0:32] nonce | [32:40] issued_at BigEndian uint64 | [40:48] expires_at BigEndian uint64 | [48:52] gateway_id BigEndian uint32
func (c *BLEChallengeV2) Encode() []byte {
	buf := make([]byte, ChallengeV2Size)
	copy(buf[:32], c.Nonce[:])
	binary.BigEndian.PutUint64(buf[32:40], uint64(c.IssuedAt.Unix()))
	binary.BigEndian.PutUint64(buf[40:48], uint64(c.ExpiresAt.Unix()))
	binary.BigEndian.PutUint32(buf[48:52], c.GatewayID)
	return buf
}

// DecodeBLEChallengeV2 deserializes a 52-byte encoded v2 challenge.
func DecodeBLEChallengeV2(data []byte) (*BLEChallengeV2, error) {
	if len(data) != ChallengeV2Size {
		return nil, fmt.Errorf("v2 challenge must be %d bytes, got %d", ChallengeV2Size, len(data))
	}
	var nonce [32]byte
	copy(nonce[:], data[:32])
	issuedAt := time.Unix(int64(binary.BigEndian.Uint64(data[32:40])), 0)
	expiresAt := time.Unix(int64(binary.BigEndian.Uint64(data[40:48])), 0)
	gatewayID := binary.BigEndian.Uint32(data[48:52])
	return &BLEChallengeV2{
		Nonce:     nonce,
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
		GatewayID: gatewayID,
	}, nil
}

// IsExpired returns true if the v2 challenge has passed its validity window.
func (c *BLEChallengeV2) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// VerifyBLESignatureV2 verifies an ECDSA signature produced under the v2 protocol.
// The message signed is SHA256(nonce || userID || transportTag).
// The transportTag parameter (e.g. TransportTagBLE, TransportTagNFCHCE) binds the
// signature to a single transport channel, preventing cross-transport replay attacks.
// Supports both ASN.1 DER and raw r||s (64 bytes) signature formats.
func VerifyBLESignatureV2(publicKeyPEM string, nonce [32]byte, userID string, transportTag string, signature []byte) error {
	pubKey, err := parseBLEPublicKey(publicKeyPEM)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}

	// Message: SHA256(nonce || userID || transportTag)
	msg := make([]byte, 0, 32+len(userID)+len(transportTag))
	msg = append(msg, nonce[:]...)
	msg = append(msg, userID...)
	msg = append(msg, transportTag...)
	hash := sha256.Sum256(msg)

	// Try ASN.1 DER format first (standard ECDSA encoding)
	if ecdsa.VerifyASN1(pubKey, hash[:], signature) {
		return nil
	}

	// Try raw r||s format (64 bytes for P-256: r=32 + s=32)
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
