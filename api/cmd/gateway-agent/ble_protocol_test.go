package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
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
