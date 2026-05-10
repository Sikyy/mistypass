package credential

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

// generateTestKeyPair creates an EC P-256 keypair for testing.
func generateTestKeyPair(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})
	return priv, string(pemBlock)
}

func TestRegisterDevice(t *testing.T) {
	svc := NewService()
	_, pubPEM := generateTestKeyPair(t)

	cred, err := svc.RegisterDevice(RegisterDeviceInput{
		TenantID:      "tenant_factory_001",
		UserID:        "usr_worker_001",
		UserEmail:     "worker@factory.id",
		PublicKeyPEM:  pubPEM,
		Platform:      "android",
		DeviceID:      "device_samsung_a54_001",
		DeviceModel:   "Samsung Galaxy A54",
		KeystoreLevel: "tee",
	})
	if err != nil {
		t.Fatalf("RegisterDevice: %v", err)
	}
	if cred.ID == "" {
		t.Fatal("expected credential ID")
	}
	if cred.Status != "active" {
		t.Fatalf("expected active status, got %s", cred.Status)
	}
	if cred.ExpiresAt.Before(time.Now().Add(29 * 24 * time.Hour)) {
		t.Fatal("expected TTL of ~30 days for TEE")
	}
}

func TestRegisterDevice_InvalidPlatform(t *testing.T) {
	svc := NewService()
	_, pubPEM := generateTestKeyPair(t)

	_, err := svc.RegisterDevice(RegisterDeviceInput{
		TenantID:      "tenant_001",
		UserID:        "usr_001",
		PublicKeyPEM:  pubPEM,
		Platform:      "windows",
		DeviceID:      "dev_001",
		KeystoreLevel: "tee",
	})
	if err == nil {
		t.Fatal("expected error for invalid platform")
	}
}

func TestRegisterDevice_InvalidKey(t *testing.T) {
	svc := NewService()

	_, err := svc.RegisterDevice(RegisterDeviceInput{
		TenantID:      "tenant_001",
		UserID:        "usr_001",
		PublicKeyPEM:  "not a valid PEM",
		Platform:      "android",
		DeviceID:      "dev_001",
		KeystoreLevel: "tee",
	})
	if err == nil {
		t.Fatal("expected error for invalid public key")
	}
}

func TestRegisterDevice_ReRegistrationRevokesOld(t *testing.T) {
	svc := NewService()
	_, pubPEM1 := generateTestKeyPair(t)
	_, pubPEM2 := generateTestKeyPair(t)

	input := RegisterDeviceInput{
		TenantID:      "tenant_001",
		UserID:        "usr_001",
		UserEmail:     "test@example.com",
		Platform:      "android",
		DeviceID:      "same_device",
		DeviceModel:   "Test",
		KeystoreLevel: "tee",
	}

	input.PublicKeyPEM = pubPEM1
	cred1, _ := svc.RegisterDevice(input)

	input.PublicKeyPEM = pubPEM2
	cred2, _ := svc.RegisterDevice(input)

	// First credential should be revoked
	got1, _ := svc.GetCredential("tenant_001", cred1.ID)
	if got1.Status != "revoked" {
		t.Fatalf("expected first credential revoked, got %s", got1.Status)
	}

	got2, _ := svc.GetCredential("tenant_001", cred2.ID)
	if got2.Status != "active" {
		t.Fatalf("expected second credential active, got %s", got2.Status)
	}
}

func TestListActiveCredentials(t *testing.T) {
	svc := NewService()
	_, pubPEM := generateTestKeyPair(t)

	svc.RegisterDevice(RegisterDeviceInput{
		TenantID:      "tenant_001",
		UserID:        "usr_001",
		UserEmail:     "a@test.com",
		PublicKeyPEM:  pubPEM,
		Platform:      "android",
		DeviceID:      "dev_001",
		KeystoreLevel: "tee",
	})

	active := svc.ListActiveCredentials("tenant_001")
	if len(active) != 1 {
		t.Fatalf("expected 1 active credential, got %d", len(active))
	}

	// Different tenant should return empty
	active2 := svc.ListActiveCredentials("tenant_other")
	if len(active2) != 0 {
		t.Fatalf("expected 0 credentials for other tenant, got %d", len(active2))
	}
}

func TestRevokeCredential(t *testing.T) {
	svc := NewService()
	_, pubPEM := generateTestKeyPair(t)

	cred, _ := svc.RegisterDevice(RegisterDeviceInput{
		TenantID:      "tenant_001",
		UserID:        "usr_001",
		UserEmail:     "a@test.com",
		PublicKeyPEM:  pubPEM,
		Platform:      "android",
		DeviceID:      "dev_001",
		KeystoreLevel: "tee",
	})

	err := svc.RevokeCredential("tenant_001", cred.ID)
	if err != nil {
		t.Fatalf("RevokeCredential: %v", err)
	}

	got, _ := svc.GetCredential("tenant_001", cred.ID)
	if got.Status != "revoked" {
		t.Fatalf("expected revoked, got %s", got.Status)
	}
	if got.RevokedAt == nil {
		t.Fatal("expected revoked_at timestamp")
	}

	// Revoking again should error
	err = svc.RevokeCredential("tenant_001", cred.ID)
	if err == nil {
		t.Fatal("expected error on double revocation")
	}
}

func TestRevokeAllUserCredentials(t *testing.T) {
	svc := NewService()
	_, pubPEM1 := generateTestKeyPair(t)
	_, pubPEM2 := generateTestKeyPair(t)

	svc.RegisterDevice(RegisterDeviceInput{
		TenantID: "tenant_001", UserID: "usr_001", UserEmail: "a@t.com",
		PublicKeyPEM: pubPEM1, Platform: "android", DeviceID: "dev_a", KeystoreLevel: "tee",
	})
	svc.RegisterDevice(RegisterDeviceInput{
		TenantID: "tenant_001", UserID: "usr_001", UserEmail: "a@t.com",
		PublicKeyPEM: pubPEM2, Platform: "ios", DeviceID: "dev_b", KeystoreLevel: "strongbox",
	})

	count := svc.RevokeAllUserCredentials("tenant_001", "usr_001")
	if count != 2 {
		t.Fatalf("expected 2 revoked, got %d", count)
	}

	active := svc.ListActiveCredentials("tenant_001")
	if len(active) != 0 {
		t.Fatalf("expected 0 active after revoke all, got %d", len(active))
	}
}

func TestVerifyBLESignature_Valid(t *testing.T) {
	svc := NewService()
	priv, pubPEM := generateTestKeyPair(t)

	svc.RegisterDevice(RegisterDeviceInput{
		TenantID: "tenant_001", UserID: "usr_001", UserEmail: "a@t.com",
		PublicKeyPEM: pubPEM, Platform: "android", DeviceID: "dev_001", KeystoreLevel: "tee",
	})

	// Simulate BLE challenge-response
	nonce := make([]byte, 32)
	rand.Read(nonce)

	// Sign: SHA256(nonce || userID)
	message := append(nonce, []byte("usr_001")...)
	hash := sha256.Sum256(message)
	signature, err := ecdsa.SignASN1(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	cred, err := svc.VerifyBLESignature("tenant_001", "usr_001", nonce, signature)
	if err != nil {
		t.Fatalf("VerifyBLESignature: %v", err)
	}
	if cred.UserID != "usr_001" {
		t.Fatalf("expected usr_001, got %s", cred.UserID)
	}
}

func TestVerifyBLESignature_Invalid(t *testing.T) {
	svc := NewService()
	_, pubPEM := generateTestKeyPair(t)

	svc.RegisterDevice(RegisterDeviceInput{
		TenantID: "tenant_001", UserID: "usr_001", UserEmail: "a@t.com",
		PublicKeyPEM: pubPEM, Platform: "android", DeviceID: "dev_001", KeystoreLevel: "tee",
	})

	nonce := make([]byte, 32)
	rand.Read(nonce)

	// Sign with a DIFFERENT key (attacker scenario)
	attackerKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	message := append(nonce, []byte("usr_001")...)
	hash := sha256.Sum256(message)
	badSig, _ := ecdsa.SignASN1(rand.Reader, attackerKey, hash[:])

	_, err := svc.VerifyBLESignature("tenant_001", "usr_001", nonce, badSig)
	if err == nil {
		t.Fatal("expected signature verification failure")
	}
}

func TestVerifyBLESignature_RevokedCredential(t *testing.T) {
	svc := NewService()
	priv, pubPEM := generateTestKeyPair(t)

	cred, _ := svc.RegisterDevice(RegisterDeviceInput{
		TenantID: "tenant_001", UserID: "usr_001", UserEmail: "a@t.com",
		PublicKeyPEM: pubPEM, Platform: "android", DeviceID: "dev_001", KeystoreLevel: "tee",
	})

	// Revoke
	svc.RevokeCredential("tenant_001", cred.ID)

	// Try to verify — should fail
	nonce := make([]byte, 32)
	rand.Read(nonce)
	message := append(nonce, []byte("usr_001")...)
	hash := sha256.Sum256(message)
	sig, _ := ecdsa.SignASN1(rand.Reader, priv, hash[:])

	_, err := svc.VerifyBLESignature("tenant_001", "usr_001", nonce, sig)
	if err == nil {
		t.Fatal("expected error for revoked credential")
	}
}

func TestVerifyBLESignature_RawFormat(t *testing.T) {
	svc := NewService()
	priv, pubPEM := generateTestKeyPair(t)

	svc.RegisterDevice(RegisterDeviceInput{
		TenantID: "tenant_001", UserID: "usr_001", UserEmail: "a@t.com",
		PublicKeyPEM: pubPEM, Platform: "android", DeviceID: "dev_001", KeystoreLevel: "tee",
	})

	nonce := make([]byte, 32)
	rand.Read(nonce)

	// Use standalone verification with raw r||s format
	message := append(nonce, []byte("usr_001")...)
	hash := sha256.Sum256(message)
	r, s, _ := ecdsa.Sign(rand.Reader, priv, hash[:])

	// Encode as raw 64 bytes (r=32 + s=32)
	rawSig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(rawSig[32-len(rBytes):32], rBytes)
	copy(rawSig[64-len(sBytes):64], sBytes)

	ok, err := VerifyECDSASignature(pubPEM, nonce, "usr_001", rawSig)
	if err != nil {
		t.Fatalf("VerifyECDSASignature: %v", err)
	}
	if !ok {
		t.Fatal("expected valid raw signature to verify")
	}
}

func TestRefreshCredential(t *testing.T) {
	svc := NewService()
	_, pubPEM := generateTestKeyPair(t)

	cred, _ := svc.RegisterDevice(RegisterDeviceInput{
		TenantID: "tenant_001", UserID: "usr_001", UserEmail: "a@t.com",
		PublicKeyPEM: pubPEM, Platform: "android", DeviceID: "dev_001", KeystoreLevel: "tee",
	})

	originalExpiry := cred.ExpiresAt

	// Wait a tiny bit so timestamps differ
	time.Sleep(time.Millisecond)

	refreshed, err := svc.RefreshCredential("tenant_001", cred.ID)
	if err != nil {
		t.Fatalf("RefreshCredential: %v", err)
	}
	if !refreshed.ExpiresAt.After(originalExpiry) {
		t.Fatal("expected refreshed expiry to be later")
	}
}

func TestCredentialTTL(t *testing.T) {
	tests := []struct {
		level    string
		expected time.Duration
	}{
		{"strongbox", TTLStrongBox},
		{"tee", TTLTee},
		{"software", TTLSoftware},
		{"unknown", TTLSoftware},
	}
	for _, tc := range tests {
		got := CredentialTTL(tc.level)
		if got != tc.expected {
			t.Errorf("CredentialTTL(%q) = %v, want %v", tc.level, got, tc.expected)
		}
	}
}

func TestParseECPublicKey_RejectsNonP256(t *testing.T) {
	// Generate a P-384 key (should be rejected)
	priv, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	pubBytes, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})

	_, err := ParseECPublicKey(string(pemBlock))
	if err == nil {
		t.Fatal("expected P-384 key to be rejected")
	}
}

func TestGenerateNonce(t *testing.T) {
	n1, err := GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce: %v", err)
	}
	if len(n1) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(n1))
	}

	n2, _ := GenerateNonce()
	if bytesEqual(n1, n2) {
		t.Fatal("two nonces should not be identical")
	}
}

func TestBuildGatewayCredentialSync(t *testing.T) {
	svc := NewService()
	_, pubPEM := generateTestKeyPair(t)

	svc.RegisterDevice(RegisterDeviceInput{
		TenantID: "tenant_001", UserID: "usr_001", UserEmail: "a@t.com",
		PublicKeyPEM: pubPEM, Platform: "android", DeviceID: "dev_001", KeystoreLevel: "tee",
	})

	gatewayLocks := []string{"door_001", "door_002"}
	userAccess := map[string][]string{
		"usr_001": {"door_001", "door_003"}, // only door_001 overlaps with gateway
	}

	result := svc.BuildGatewayCredentialSync("tenant_001", gatewayLocks, userAccess)
	if len(result) != 1 {
		t.Fatalf("expected 1 sync entry, got %d", len(result))
	}
	if result[0].UserID != "usr_001" {
		t.Fatalf("expected usr_001, got %s", result[0].UserID)
	}
	if len(result[0].LockIDs) != 1 || result[0].LockIDs[0] != "door_001" {
		t.Fatalf("expected [door_001], got %v", result[0].LockIDs)
	}
}

func TestBuildGatewayCredentialSyncSinceScopesByTenantLocksAndVersion(t *testing.T) {
	svc := NewService()
	_, pubPEM := generateTestKeyPair(t)

	cred, err := svc.RegisterDevice(RegisterDeviceInput{
		TenantID: "tenant_001", UserID: "usr_sync_001", UserEmail: "sync@example.test",
		PublicKeyPEM: pubPEM, Platform: "android", DeviceID: "dev_sync_001", KeystoreLevel: "tee",
	})
	if err != nil {
		t.Fatalf("register scoped credential: %v", err)
	}

	_, pubPEM2 := generateTestKeyPair(t)
	if _, err := svc.RegisterDevice(RegisterDeviceInput{
		TenantID: "tenant_002", UserID: "usr_sync_002", UserEmail: "other@example.test",
		PublicKeyPEM: pubPEM2, Platform: "android", DeviceID: "dev_sync_002", KeystoreLevel: "tee",
	}); err != nil {
		t.Fatalf("register other tenant credential: %v", err)
	}

	gatewayLocks := []string{"door_001", "door_002"}
	userAccess := map[string][]string{
		"usr_sync_001": {"door_001", "door_003"},
		"usr_sync_002": {"door_001"},
	}

	result := svc.BuildGatewayCredentialSyncSince("tenant_001", gatewayLocks, userAccess, 0)
	entry, ok := findGatewayCredentialSyncEntry(result, "usr_sync_001")
	if !ok {
		t.Fatalf("expected scoped credential in sync result, got %+v", result)
	}
	if len(entry.LockIDs) != 1 || entry.LockIDs[0] != "door_001" {
		t.Fatalf("expected only intersected lock door_001, got %v", entry.LockIDs)
	}
	if entry.SyncVersion != cred.IssuedAt.UnixNano() {
		t.Fatalf("expected issued_at sync version %d, got %d", cred.IssuedAt.UnixNano(), entry.SyncVersion)
	}
	if _, ok := findGatewayCredentialSyncEntry(result, "usr_sync_002"); ok {
		t.Fatalf("expected other tenant credential to be excluded: %+v", result)
	}

	result = svc.BuildGatewayCredentialSyncSince("tenant_001", gatewayLocks, userAccess, cred.IssuedAt.UnixNano())
	if _, ok := findGatewayCredentialSyncEntry(result, "usr_sync_001"); ok {
		t.Fatalf("expected since_version to exclude already-seen credential, got %+v", result)
	}
}

func TestBuildGatewayCredentialSyncSinceIncludesRelevantRevocations(t *testing.T) {
	svc := NewService()
	_, pubPEM := generateTestKeyPair(t)

	cred, err := svc.RegisterDevice(RegisterDeviceInput{
		TenantID: "tenant_001", UserID: "usr_revoke_001", UserEmail: "revoked@example.test",
		PublicKeyPEM: pubPEM, Platform: "ios", DeviceID: "dev_revoke_001", KeystoreLevel: "strongbox",
	})
	if err != nil {
		t.Fatalf("register credential: %v", err)
	}
	if err := svc.RevokeCredential("tenant_001", cred.ID); err != nil {
		t.Fatalf("revoke credential: %v", err)
	}

	result := svc.BuildGatewayCredentialSyncSince(
		"tenant_001",
		[]string{"door_001"},
		map[string][]string{"usr_revoke_001": {"door_001"}},
		cred.IssuedAt.UnixNano(),
	)
	entry, ok := findGatewayCredentialSyncEntry(result, "usr_revoke_001")
	if !ok {
		t.Fatalf("expected revoked credential delta, got %+v", result)
	}
	if entry.RevokedAt == nil {
		t.Fatalf("expected revoked_at to be set, got %+v", entry)
	}
	if entry.SyncVersion <= cred.IssuedAt.UnixNano() {
		t.Fatalf("expected revocation sync version to advance past issue version %d, got %d", cred.IssuedAt.UnixNano(), entry.SyncVersion)
	}
	if *entry.RevokedAt == 0 {
		t.Fatalf("expected revoked_at unix timestamp, got %+v", entry)
	}
}

func TestGetGatewayCredentialSyncSince(t *testing.T) {
	svc := NewService()
	_, pubPEM := generateTestKeyPair(t)

	// Register two credentials at slightly different times
	cred1, err := svc.RegisterDevice(RegisterDeviceInput{
		TenantID: "tenant_001", UserID: "usr_001", UserEmail: "a@t.com",
		PublicKeyPEM: pubPEM, Platform: "android", DeviceID: "dev_001", KeystoreLevel: "tee",
	})
	if err != nil {
		t.Fatalf("register cred1: %v", err)
	}

	_, pubPEM2 := generateTestKeyPair(t)
	cred2, err := svc.RegisterDevice(RegisterDeviceInput{
		TenantID: "tenant_001", UserID: "usr_002", UserEmail: "b@t.com",
		PublicKeyPEM: pubPEM2, Platform: "android", DeviceID: "dev_002", KeystoreLevel: "tee",
	})
	if err != nil {
		t.Fatalf("register cred2: %v", err)
	}

	// Sync since version 0 — should return all credentials (seed + newly registered)
	result, err := svc.GetGatewayCredentialSyncSince(0)
	if err != nil {
		t.Fatalf("sync since 0: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected at least some credentials for sinceVersion=0")
	}

	// Sync since a version after cred1 but before cred2 (use cred1's issued_at unix)
	sinceV := cred1.IssuedAt.Unix()
	result, err = svc.GetGatewayCredentialSyncSince(sinceV)
	if err != nil {
		t.Fatalf("sync since cred1 version: %v", err)
	}
	// Credentials issued at the same second may or may not appear depending on ordering;
	// but a future sinceVersion should exclude them.
	futureVersion := cred2.IssuedAt.Unix() + 100
	result, err = svc.GetGatewayCredentialSyncSince(futureVersion)
	if err != nil {
		t.Fatalf("sync since future version: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 results for future sinceVersion, got %d", len(result))
	}

	// Revoke a credential and verify it appears with RevokedAt set
	svc.RevokeCredential("tenant_001", cred1.ID)
	result, err = svc.GetGatewayCredentialSyncSince(0)
	if err != nil {
		t.Fatalf("sync after revoke: %v", err)
	}
	var foundRevoked bool
	for _, entry := range result {
		if entry.UserID == "usr_001" && entry.RevokedAt != nil {
			foundRevoked = true
			break
		}
	}
	if !foundRevoked {
		t.Fatal("expected to find revoked credential in sync results")
	}

	// suppress unused variable warning
	_ = cred2
}

func findGatewayCredentialSyncEntry(items []GatewayCredentialSync, userID string) (GatewayCredentialSync, bool) {
	for _, item := range items {
		if item.UserID == userID {
			return item, true
		}
	}
	return GatewayCredentialSync{}, false
}

// buildAttestationExtensionValue builds a DER-encoded KeyDescription ASN.1
// value with the given attestationSecurityLevel for use in test certificates.
func buildAttestationExtensionValue(securityLevel int) []byte {
	// KeyDescription SEQUENCE:
	//   attestationVersion       INTEGER (3)
	//   attestationSecurityLevel ENUMERATED (securityLevel)
	//   keymasterVersion         INTEGER (4)
	//   keymasterSecurityLevel   ENUMERATED (securityLevel)
	//   attestationChallenge     OCTET STRING ("test")
	//   uniqueId                 OCTET STRING ("")
	//   softwareEnforced         SEQUENCE {}
	//   teeEnforced              SEQUENCE {}

	type testKeyDescription struct {
		AttestationVersion       int
		AttestationSecurityLevel asn1.Enumerated
		KeymasterVersion         int
		KeymasterSecurityLevel   asn1.Enumerated
		AttestationChallenge     []byte
		UniqueID                 []byte
		SoftwareEnforced         asn1.RawValue
		TeeEnforced              asn1.RawValue
	}

	emptySeq, _ := asn1.Marshal(struct{}{})

	kd := testKeyDescription{
		AttestationVersion:       3,
		AttestationSecurityLevel: asn1.Enumerated(securityLevel),
		KeymasterVersion:         4,
		KeymasterSecurityLevel:   asn1.Enumerated(securityLevel),
		AttestationChallenge:     []byte("test-challenge"),
		UniqueID:                 []byte{},
		SoftwareEnforced:         asn1.RawValue{FullBytes: emptySeq},
		TeeEnforced:              asn1.RawValue{FullBytes: emptySeq},
	}

	data, _ := asn1.Marshal(kd)
	return data
}

// createTestCertWithAttestationExt generates a self-signed X.509 certificate
// that contains the Android Key Attestation extension with the given security level.
func createTestCertWithAttestationExt(t *testing.T, securityLevel int) string {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	attestOID := asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 11129, 2, 1, 17}
	extValue := buildAttestationExtensionValue(securityLevel)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test Attestation Cert"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		ExtraExtensions: []pkix.Extension{
			{
				Id:    attestOID,
				Value: extValue,
			},
		},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return base64.StdEncoding.EncodeToString(certDER)
}

func TestDetectKeystoreLevel_StrongBoxFromExtension(t *testing.T) {
	certB64 := createTestCertWithAttestationExt(t, 2) // StrongBox
	got := DetectKeystoreLevel([]string{certB64})
	if got != "strongbox" {
		t.Fatalf("expected strongbox, got %s", got)
	}
}

func TestDetectKeystoreLevel_TEEFromExtension(t *testing.T) {
	certB64 := createTestCertWithAttestationExt(t, 1) // TrustedEnvironment
	got := DetectKeystoreLevel([]string{certB64})
	if got != "tee" {
		t.Fatalf("expected tee, got %s", got)
	}
}

func TestDetectKeystoreLevel_SoftwareFromExtension(t *testing.T) {
	certB64 := createTestCertWithAttestationExt(t, 0) // Software
	got := DetectKeystoreLevel([]string{certB64})
	if got != "software" {
		t.Fatalf("expected software, got %s", got)
	}
}

func TestDetectKeystoreLevel_EmptyChain(t *testing.T) {
	got := DetectKeystoreLevel(nil)
	if got != "software" {
		t.Fatalf("expected software for nil chain, got %s", got)
	}
	got = DetectKeystoreLevel([]string{})
	if got != "software" {
		t.Fatalf("expected software for empty chain, got %s", got)
	}
}

func TestDetectKeystoreLevel_NoExtensionFallsBackToHeuristic(t *testing.T) {
	// Create a cert WITHOUT the attestation extension
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Plain Cert"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certB64 := base64.StdEncoding.EncodeToString(certDER)

	// Single cert, no intermediate — should fall back to "software"
	got := DetectKeystoreLevel([]string{certB64})
	if got != "software" {
		t.Fatalf("expected software for cert without attestation ext (single cert), got %s", got)
	}
}

func TestDetectKeystoreLevel_InvalidBase64(t *testing.T) {
	got := DetectKeystoreLevel([]string{"not-valid-base64!!!"})
	if got != "software" {
		t.Fatalf("expected software for invalid base64, got %s", got)
	}
}

func TestSecurityLevelToString(t *testing.T) {
	tests := []struct {
		level    int
		expected string
	}{
		{0, "software"},
		{1, "tee"},
		{2, "strongbox"},
		{3, "software"}, // unknown value
		{99, "software"},
	}
	for _, tc := range tests {
		got := securityLevelToString(asn1.Enumerated(tc.level))
		if got != tc.expected {
			t.Errorf("securityLevelToString(%d) = %q, want %q", tc.level, got, tc.expected)
		}
	}
}

func TestVerifyBLESignatureV2_TransportBinding(t *testing.T) {
	svc := NewService()
	priv, pubPEM := generateTestKeyPair(t)

	svc.RegisterDevice(RegisterDeviceInput{
		TenantID:      "tenant_001",
		UserID:        "usr_001",
		UserEmail:     "a@t.com",
		PublicKeyPEM:  pubPEM,
		Platform:      "android",
		DeviceID:      "dev_001",
		KeystoreLevel: "tee",
	})

	nonce := make([]byte, 32)
	rand.Read(nonce)

	const bleTag = "BLE"
	const nfcTag = "NFC_HCE"

	// Sign with BLE transport tag: SHA256(nonce || userID || "BLE")
	msg := make([]byte, 0, len(nonce)+len("usr_001")+len(bleTag))
	msg = append(msg, nonce...)
	msg = append(msg, []byte("usr_001")...)
	msg = append(msg, []byte(bleTag)...)
	hash := sha256.Sum256(msg)
	signature, err := ecdsa.SignASN1(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// Verify with correct BLE tag — should succeed
	cred, err := svc.VerifyBLESignatureV2("tenant_001", "usr_001", nonce, bleTag, signature)
	if err != nil {
		t.Fatalf("VerifyBLESignatureV2 with BLE tag: %v", err)
	}
	if cred.UserID != "usr_001" {
		t.Fatalf("expected usr_001, got %s", cred.UserID)
	}

	// Verify with wrong transport tag (NFC_HCE) — should fail
	_, err = svc.VerifyBLESignatureV2("tenant_001", "usr_001", nonce, nfcTag, signature)
	if err == nil {
		t.Fatal("expected verification failure when transport tag is NFC_HCE but signature used BLE")
	}
}

// Ensure unused import doesn't break
var _ = big.NewInt
