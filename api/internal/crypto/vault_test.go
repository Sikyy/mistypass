package crypto

import (
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
