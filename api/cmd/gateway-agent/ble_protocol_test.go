package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"log/slog"
	"testing"
	"time"
)

func generateTestECKey(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	return priv, string(pemBlock)
}

func TestNewBLEChallenge(t *testing.T) {
	c, err := NewBLEChallenge()
	if err != nil {
		t.Fatalf("NewBLEChallenge: %v", err)
	}
	if c.IsExpired() {
		t.Fatal("fresh challenge should not be expired")
	}
	// Nonce should not be all zeros
	allZero := true
	for _, b := range c.Nonce {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("nonce should not be all zeros")
	}
}

func TestBLEChallengeEncode(t *testing.T) {
	c, _ := NewBLEChallenge()
	encoded := c.Encode()
	if len(encoded) != 48 {
		t.Fatalf("expected 48 bytes, got %d", len(encoded))
	}
}

func TestBLEChallengeExpiry(t *testing.T) {
	c := &BLEChallenge{
		Nonce:     [32]byte{1, 2, 3},
		IssuedAt:  time.Now().Add(-60 * time.Second),
		ExpiresAt: time.Now().Add(-30 * time.Second),
	}
	if !c.IsExpired() {
		t.Fatal("old challenge should be expired")
	}
}

func TestDecodeBLEAuthResponse(t *testing.T) {
	userID := "usr_001"
	sig := []byte{0x30, 0x44, 0x02, 0x20} // fake signature prefix

	encoded := EncodeBLEAuthResponse(userID, sig)
	decoded, err := DecodeBLEAuthResponse(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.UserID != userID {
		t.Fatalf("expected %q, got %q", userID, decoded.UserID)
	}
	if len(decoded.Signature) != len(sig) {
		t.Fatalf("expected sig len %d, got %d", len(sig), len(decoded.Signature))
	}
}

func TestDecodeBLEAuthResponse_TooShort(t *testing.T) {
	_, err := DecodeBLEAuthResponse([]byte{0x01})
	if err == nil {
		t.Fatal("expected error for too-short data")
	}
}

func TestDecodeBLEAuthResponse_EmptyUserID(t *testing.T) {
	_, err := DecodeBLEAuthResponse([]byte{0x00, 0x30, 0x44})
	if err == nil {
		t.Fatal("expected error for empty user_id")
	}
}

func TestVerifyBLESignature_ASN1(t *testing.T) {
	priv, pubPEM := generateTestECKey(t)

	var nonce [32]byte
	rand.Read(nonce[:])

	userID := "usr_test_001"
	message := append(nonce[:], []byte(userID)...)
	hash := sha256.Sum256(message)

	sig, err := ecdsa.SignASN1(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	err = VerifyBLESignature(pubPEM, nonce, userID, sig)
	if err != nil {
		t.Fatalf("VerifyBLESignature: %v", err)
	}
}

func TestVerifyBLESignature_RawRS(t *testing.T) {
	priv, pubPEM := generateTestECKey(t)

	var nonce [32]byte
	rand.Read(nonce[:])

	userID := "usr_raw_001"
	message := append(nonce[:], []byte(userID)...)
	hash := sha256.Sum256(message)

	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// Encode as raw 64 bytes
	rawSig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(rawSig[32-len(rBytes):32], rBytes)
	copy(rawSig[64-len(sBytes):64], sBytes)

	err = VerifyBLESignature(pubPEM, nonce, userID, rawSig)
	if err != nil {
		t.Fatalf("VerifyBLESignature raw: %v", err)
	}
}

func TestVerifyBLESignature_WrongKey(t *testing.T) {
	_, pubPEM := generateTestECKey(t)
	attackerKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	var nonce [32]byte
	rand.Read(nonce[:])

	userID := "usr_001"
	message := append(nonce[:], []byte(userID)...)
	hash := sha256.Sum256(message)

	sig, _ := ecdsa.SignASN1(rand.Reader, attackerKey, hash[:])

	err := VerifyBLESignature(pubPEM, nonce, userID, sig)
	if err == nil {
		t.Fatal("expected verification failure with wrong key")
	}
}

func TestVerifyBLESignature_WrongNonce(t *testing.T) {
	priv, pubPEM := generateTestECKey(t)

	var nonce [32]byte
	rand.Read(nonce[:])

	userID := "usr_001"
	message := append(nonce[:], []byte(userID)...)
	hash := sha256.Sum256(message)
	sig, _ := ecdsa.SignASN1(rand.Reader, priv, hash[:])

	// Verify with a different nonce
	var wrongNonce [32]byte
	rand.Read(wrongNonce[:])

	err := VerifyBLESignature(pubPEM, wrongNonce, userID, sig)
	if err == nil {
		t.Fatal("expected verification failure with wrong nonce")
	}
}

func TestVerifyBLESignature_InvalidPEM(t *testing.T) {
	var nonce [32]byte
	err := VerifyBLESignature("not-a-pem", nonce, "usr", []byte{0x30})
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

func TestBLEAuthResult_Encode(t *testing.T) {
	r := BLEAuthResult{Code: BLEResultGranted, Reason: "access_granted"}
	encoded := r.Encode()
	if encoded[0] != BLEResultGranted {
		t.Fatalf("expected code %d, got %d", BLEResultGranted, encoded[0])
	}
	if string(encoded[1:]) != "access_granted" {
		t.Fatalf("expected reason 'access_granted', got %q", string(encoded[1:]))
	}
}

func TestAgentVerifyBLEAuth_FullFlow(t *testing.T) {
	priv, pubPEM := generateTestECKey(t)

	agent := &Agent{
		logger:        testLogger(),
		rulesCacheTTL: 24 * time.Hour,
		rulesUpdatedAt: time.Now(),
		accessRules: []AccessRule{
			{
				CredentialType: "ble_signature",
				CredentialData: pubPEM,
				UserID:         "usr_factory_001",
				UserEmail:      "worker@factory.id",
				LockIDs:        []string{"door_factory_001", "door_factory_002"},
			},
		},
		relay: &DryRunRelay{logger: testLogger()},
	}

	// Generate challenge
	challenge, _ := NewBLEChallenge()

	// Phone signs the challenge
	message := append(challenge.Nonce[:], []byte("usr_factory_001")...)
	hash := sha256.Sum256(message)
	sig, _ := ecdsa.SignASN1(rand.Reader, priv, hash[:])

	response := &BLEAuthResponse{
		UserID:    "usr_factory_001",
		Signature: sig,
	}

	// Verify — should grant access to door_factory_001
	decision, userID, _ := agent.VerifyBLEAuth(challenge, response, "door_factory_001")
	if decision != "allow" {
		t.Fatalf("expected allow, got %s", decision)
	}
	if userID != "usr_factory_001" {
		t.Fatalf("expected usr_factory_001, got %s", userID)
	}

	// Verify — should deny access to door_factory_999 (not in lock list)
	decision2, _, _ := agent.VerifyBLEAuth(challenge, response, "door_factory_999")
	if decision2 != "deny" {
		t.Fatalf("expected deny for unauthorized door, got %s", decision2)
	}
}

func TestAgentVerifyBLEAuth_ExpiredChallenge(t *testing.T) {
	priv, pubPEM := generateTestECKey(t)

	agent := &Agent{
		logger:         testLogger(),
		rulesCacheTTL:  24 * time.Hour,
		rulesUpdatedAt: time.Now(),
		accessRules: []AccessRule{
			{
				CredentialType: "ble_signature",
				CredentialData: pubPEM,
				UserID:         "usr_001",
				UserEmail:      "a@b.c",
				LockIDs:        []string{"door_001"},
			},
		},
		relay: &DryRunRelay{logger: testLogger()},
	}

	// Create an already-expired challenge
	challenge := &BLEChallenge{
		Nonce:     [32]byte{1, 2, 3},
		IssuedAt:  time.Now().Add(-60 * time.Second),
		ExpiresAt: time.Now().Add(-30 * time.Second),
	}

	message := append(challenge.Nonce[:], []byte("usr_001")...)
	hash := sha256.Sum256(message)
	sig, _ := ecdsa.SignASN1(rand.Reader, priv, hash[:])

	response := &BLEAuthResponse{UserID: "usr_001", Signature: sig}

	decision, _, _ := agent.VerifyBLEAuth(challenge, response, "door_001")
	if decision != "deny" {
		t.Fatalf("expected deny for expired challenge, got %s", decision)
	}
}

func TestAgentVerifyBLEAuth_UnknownUser(t *testing.T) {
	agent := &Agent{
		logger:         testLogger(),
		rulesCacheTTL:  24 * time.Hour,
		rulesUpdatedAt: time.Now(),
		accessRules:    []AccessRule{}, // no rules
		relay:          &DryRunRelay{logger: testLogger()},
	}

	challenge, _ := NewBLEChallenge()
	response := &BLEAuthResponse{UserID: "unknown_user", Signature: []byte{0x30}}

	decision, _, _ := agent.VerifyBLEAuth(challenge, response, "door_001")
	if decision != "deny" {
		t.Fatalf("expected deny for unknown user, got %s", decision)
	}
}

func testLogger() *slog.Logger {
	return slog.Default()
}

// --- v2 protocol tests ---

func TestBLEChallengeV2_Encode(t *testing.T) {
	c, err := NewBLEChallengeV2(42)
	if err != nil {
		t.Fatalf("NewBLEChallengeV2: %v", err)
	}

	encoded := c.Encode()
	if len(encoded) != ChallengeV2Size {
		t.Fatalf("expected %d bytes, got %d", ChallengeV2Size, len(encoded))
	}

	// Nonce (bytes 0-31) must not be all zeros
	allZero := true
	for _, b := range encoded[:32] {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("nonce should not be all zeros")
	}

	// GatewayID (bytes 48-51) must encode the value 42
	gwID := binary.BigEndian.Uint32(encoded[48:52])
	if gwID != 42 {
		t.Fatalf("expected gateway_id 42, got %d", gwID)
	}

	// Validity window should be ~30 seconds
	issuedAt := int64(binary.BigEndian.Uint64(encoded[32:40]))
	expiresAt := int64(binary.BigEndian.Uint64(encoded[40:48]))
	window := expiresAt - issuedAt
	if window != int64(challengeValidDuration/time.Second) {
		t.Fatalf("expected 30s window, got %ds", window)
	}
}

func TestBLEChallengeV2_IsExpired(t *testing.T) {
	fresh, err := NewBLEChallengeV2(1)
	if err != nil {
		t.Fatalf("NewBLEChallengeV2: %v", err)
	}
	if fresh.IsExpired() {
		t.Fatal("fresh v2 challenge should not be expired")
	}

	old := &BLEChallengeV2{
		Nonce:     [32]byte{1},
		IssuedAt:  time.Now().Add(-60 * time.Second),
		ExpiresAt: time.Now().Add(-30 * time.Second),
		GatewayID: 1,
	}
	if !old.IsExpired() {
		t.Fatal("old v2 challenge should be expired")
	}
}

func TestVerifyBLESignatureV2_WithTransportTag(t *testing.T) {
	priv, pubPEM := generateTestECKey(t)

	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}

	userID := "usr_v2_001"

	// Build signature with BLE transport tag
	msg := append(nonce[:], []byte(userID)...)
	msg = append(msg, []byte(TransportTagBLE)...)
	hash := sha256.Sum256(msg)

	sig, err := ecdsa.SignASN1(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// Correct tag must pass
	if err := VerifyBLESignatureV2(pubPEM, nonce, userID, TransportTagBLE, sig); err != nil {
		t.Fatalf("VerifyBLESignatureV2 with correct tag: %v", err)
	}

	// Wrong tag (NFC_HCE) must fail — cross-transport replay protection
	if err := VerifyBLESignatureV2(pubPEM, nonce, userID, TransportTagNFCHCE, sig); err == nil {
		t.Fatal("expected failure when verifying BLE sig with NFC_HCE tag")
	}
}

func TestVerifyBLESignatureV2_RawRS(t *testing.T) {
	priv, pubPEM := generateTestECKey(t)

	var nonce [32]byte
	rand.Read(nonce[:])

	userID := "usr_v2_raw"
	msg := append(nonce[:], []byte(userID)...)
	msg = append(msg, []byte(TransportTagNFCHCE)...)
	hash := sha256.Sum256(msg)

	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	rawSig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(rawSig[32-len(rBytes):32], rBytes)
	copy(rawSig[64-len(sBytes):64], sBytes)

	if err := VerifyBLESignatureV2(pubPEM, nonce, userID, TransportTagNFCHCE, rawSig); err != nil {
		t.Fatalf("VerifyBLESignatureV2 raw r||s: %v", err)
	}
}

func TestDecodeBLEChallengeV2(t *testing.T) {
	original, err := NewBLEChallengeV2(99)
	if err != nil {
		t.Fatalf("NewBLEChallengeV2: %v", err)
	}

	encoded := original.Encode()
	decoded, err := DecodeBLEChallengeV2(encoded)
	if err != nil {
		t.Fatalf("DecodeBLEChallengeV2: %v", err)
	}

	if decoded.Nonce != original.Nonce {
		t.Fatal("nonce mismatch after decode")
	}
	if decoded.GatewayID != 99 {
		t.Fatalf("expected gateway_id 99, got %d", decoded.GatewayID)
	}
	if decoded.IssuedAt.Unix() != original.IssuedAt.Unix() {
		t.Fatal("issued_at mismatch after decode")
	}
	if decoded.ExpiresAt.Unix() != original.ExpiresAt.Unix() {
		t.Fatal("expires_at mismatch after decode")
	}
}
