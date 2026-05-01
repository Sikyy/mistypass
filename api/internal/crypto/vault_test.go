package crypto

import (
	"strings"
	"testing"
)

func TestVaultEncryptDecryptRoundtrip(t *testing.T) {
	v := NewVault("test-master-key-12345", "totp-secrets")
	if v == nil {
		t.Fatal("expected vault")
	}

	secrets := []string{
		"JBSWY3DPEHPK3PXP",
		"GEZDGNBVGY3TQOJQ",
		"",
		"a very long secret that is much longer than typical totp secrets",
	}

	for _, secret := range secrets {
		nonce, ciphertext, err := v.Encrypt(secret)
		if err != nil {
			t.Fatalf("encrypt %q: %v", secret, err)
		}
		if secret != "" && ciphertext == secret {
			t.Fatalf("ciphertext should differ from plaintext")
		}

		decrypted, err := v.Decrypt(nonce, ciphertext)
		if err != nil {
			t.Fatalf("decrypt %q: %v", secret, err)
		}
		if decrypted != secret {
			t.Fatalf("roundtrip mismatch: got %q, want %q", decrypted, secret)
		}
	}
}

func TestVaultDifferentPurposesDifferentKeys(t *testing.T) {
	v1 := NewVault("same-master", "purpose-a")
	v2 := NewVault("same-master", "purpose-b")

	nonce, ct, _ := v1.Encrypt("hello")
	_, err := v2.Decrypt(nonce, ct)
	if err == nil {
		t.Fatal("different purposes should produce different keys; decrypt should fail")
	}
}

func TestVaultNilWhenNoKey(t *testing.T) {
	v := NewVault("", "test")
	if v != nil {
		t.Fatal("empty master key should return nil vault")
	}
	if v.IsConfigured() {
		t.Fatal("nil vault should not be configured")
	}
}

func TestDeriveKeyDeterministic(t *testing.T) {
	k1 := DeriveKey("master", "purpose")
	k2 := DeriveKey("master", "purpose")
	if string(k1) != string(k2) {
		t.Fatal("same inputs should produce same key")
	}
}

func TestDeriveKeyDifferentInputs(t *testing.T) {
	k1 := DeriveKey("master-a", "purpose")
	k2 := DeriveKey("master-b", "purpose")
	if string(k1) == string(k2) {
		t.Fatal("different master keys should produce different derived keys")
	}
}

func TestVersionedEncryptDecrypt(t *testing.T) {
	v := NewVault("master-key", "test")

	encrypted, err := v.EncryptVersioned("my-secret")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encrypted, "enc:v1:") {
		t.Fatalf("expected enc:v1: prefix, got %s", encrypted)
	}

	decrypted, err := v.DecryptVersioned(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "my-secret" {
		t.Fatalf("expected my-secret, got %s", decrypted)
	}
}

func TestVersionedDecryptPlaintext(t *testing.T) {
	v := NewVault("master-key", "test")

	// Plaintext values should pass through unchanged
	pt, err := v.DecryptVersioned("JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatal(err)
	}
	if pt != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("plaintext passthrough failed: got %q", pt)
	}

	// Empty string
	pt, err = v.DecryptVersioned("")
	if err != nil {
		t.Fatal(err)
	}
	if pt != "" {
		t.Fatalf("empty string passthrough failed: got %q", pt)
	}
}

func TestKeyRotation(t *testing.T) {
	purpose := "rotation-test"
	oldKey := "old-master-key-2025"
	newKey := "new-master-key-2026"

	// Phase 1: encrypt with old key
	vOld := NewVault(oldKey, purpose)
	encrypted, err := vOld.EncryptVersioned("totp-secret-abc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encrypted, "enc:v1:") {
		t.Fatalf("expected v1 prefix, got %s", encrypted)
	}

	// Phase 2: rotate — new vault knows both keys
	vNew := NewVaultWithRotation(newKey, purpose, oldKey)
	if vNew.CurrentVersion() != 2 {
		t.Fatalf("expected current version 2, got %d", vNew.CurrentVersion())
	}
	if vNew.KeyVersionCount() != 2 {
		t.Fatalf("expected 2 key versions, got %d", vNew.KeyVersionCount())
	}

	// Can decrypt old v1 ciphertext
	decrypted, err := vNew.DecryptVersioned(encrypted)
	if err != nil {
		t.Fatalf("decrypt old v1 with rotated vault: %v", err)
	}
	if decrypted != "totp-secret-abc" {
		t.Fatalf("decrypted mismatch: got %q", decrypted)
	}

	// New encryptions use v2
	newEncrypted, err := vNew.EncryptVersioned("new-secret")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(newEncrypted, "enc:v2:") {
		t.Fatalf("expected v2 prefix, got %s", newEncrypted)
	}

	// Can decrypt new v2 ciphertext
	dec2, err := vNew.DecryptVersioned(newEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	if dec2 != "new-secret" {
		t.Fatalf("decrypted v2 mismatch: got %q", dec2)
	}

	// Old vault CANNOT decrypt v2 ciphertext
	parts := strings.SplitN(newEncrypted[4:], ":", 3) // strip "enc:"
	_, err = vOld.Decrypt(parts[1], parts[2])
	if err == nil {
		t.Fatal("old vault should not decrypt new key's ciphertext")
	}
}

func TestNeedsReEncrypt(t *testing.T) {
	v := NewVaultWithRotation("new-key", "test", "old-key")

	tests := []struct {
		value string
		want  bool
	}{
		{"plaintext-secret", true},            // plaintext needs encryption
		{"", false},                            // empty is fine
		{"enc:v1:abc:def", true},              // old version
		{"enc:v2:abc:def", false},             // current version
		{"enc:abc:def", true},                 // legacy format
	}

	for _, tt := range tests {
		got := v.NeedsReEncrypt(tt.value)
		if got != tt.want {
			t.Errorf("NeedsReEncrypt(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestReEncrypt(t *testing.T) {
	oldKey := "old-master"
	newKey := "new-master"
	purpose := "reencrypt-test"

	// Encrypt with old key
	vOld := NewVault(oldKey, purpose)
	oldEncrypted, _ := vOld.EncryptVersioned("secret-data")

	// Create rotated vault
	vNew := NewVaultWithRotation(newKey, purpose, oldKey)

	// Re-encrypt
	newEncrypted, err := vNew.ReEncrypt(oldEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(newEncrypted, "enc:v2:") {
		t.Fatalf("re-encrypted should use v2, got %s", newEncrypted)
	}

	// Verify decrypts to same value
	dec, _ := vNew.DecryptVersioned(newEncrypted)
	if dec != "secret-data" {
		t.Fatalf("re-encrypted value mismatch: got %q", dec)
	}

	// Re-encrypt plaintext
	ptReenc, err := vNew.ReEncrypt("plain-secret")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ptReenc, "enc:v2:") {
		t.Fatalf("plaintext re-encrypt should use v2, got %s", ptReenc)
	}
	dec2, _ := vNew.DecryptVersioned(ptReenc)
	if dec2 != "plain-secret" {
		t.Fatalf("plaintext re-encrypt mismatch: got %q", dec2)
	}
}

func TestLegacyFormatDecrypt(t *testing.T) {
	// Simulate legacy "enc:{nonce}:{ct}" format (no version tag)
	v := NewVault("master-key", "test")

	// Encrypt using low-level (no version tag)
	nonce, ct, err := v.Encrypt("legacy-secret")
	if err != nil {
		t.Fatal(err)
	}
	legacyFormat := "enc:" + nonce + ":" + ct

	// DecryptVersioned should handle legacy format
	dec, err := v.DecryptVersioned(legacyFormat)
	if err != nil {
		t.Fatalf("legacy format decrypt: %v", err)
	}
	if dec != "legacy-secret" {
		t.Fatalf("legacy decrypt mismatch: got %q", dec)
	}
}

func TestMultipleRotations(t *testing.T) {
	purpose := "multi-rotate"

	// 3 generations of keys
	v := NewVaultWithRotation("key-v3", purpose, "key-v1", "key-v2")
	if v.CurrentVersion() != 3 {
		t.Fatalf("expected version 3, got %d", v.CurrentVersion())
	}
	if v.KeyVersionCount() != 3 {
		t.Fatalf("expected 3 versions, got %d", v.KeyVersionCount())
	}

	// Encrypt with v1
	v1 := NewVault("key-v1", purpose)
	enc1, _ := v1.EncryptVersioned("data-from-v1")

	// Encrypt with v2
	v2 := NewVaultWithRotation("key-v2", purpose, "key-v1")
	enc2, _ := v2.EncryptVersioned("data-from-v2")

	// v3 vault can decrypt both
	dec1, err := v.DecryptVersioned(enc1)
	if err != nil || dec1 != "data-from-v1" {
		t.Fatalf("v3 should decrypt v1: err=%v dec=%q", err, dec1)
	}
	dec2, err := v.DecryptVersioned(enc2)
	if err != nil || dec2 != "data-from-v2" {
		t.Fatalf("v3 should decrypt v2: err=%v dec=%q", err, dec2)
	}
}
