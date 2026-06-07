package otasig

import (
	"crypto/ed25519"
	"testing"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("fake firmware bytes")
	sha := SHA256Hex(data)
	sig := Sign(priv, "1.2.0", sha)
	if err := VerifyArtifact([]ed25519.PublicKey{pub}, "1.2.0", sha, sig, data); err != nil {
		t.Fatalf("expected verify ok, got %v", err)
	}
}

func TestVerifyRejectsTamperedData(t *testing.T) {
	pub, priv, _ := GenerateKey()
	data := []byte("fake firmware bytes")
	sha := SHA256Hex(data)
	sig := Sign(priv, "1.2.0", sha)
	if err := VerifyArtifact([]ed25519.PublicKey{pub}, "1.2.0", sha, sig, []byte("tampered")); err == nil {
		t.Fatal("expected sha256 mismatch error")
	}
}

func TestVerifyRejectsVersionSwap(t *testing.T) {
	pub, priv, _ := GenerateKey()
	data := []byte("fw")
	sha := SHA256Hex(data)
	sig := Sign(priv, "1.0.0", sha)
	if err := VerifyArtifact([]ed25519.PublicKey{pub}, "9.9.9", sha, sig, data); err == nil {
		t.Fatal("expected signature failure when version claim differs")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	_, priv, _ := GenerateKey()
	otherPub, _, _ := GenerateKey()
	data := []byte("fw")
	sha := SHA256Hex(data)
	sig := Sign(priv, "1.0.0", sha)
	if err := VerifyArtifact([]ed25519.PublicKey{otherPub}, "1.0.0", sha, sig, data); err == nil {
		t.Fatal("expected verify failure with wrong key")
	}
}

func TestPublicKeyHexRoundTrip(t *testing.T) {
	pub, _, _ := GenerateKey()
	got, err := ParsePublicKeyHex(MarshalPublicKeyHex(pub))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(pub) {
		t.Fatal("pubkey round trip mismatch")
	}
}

func TestPrivateKeyPEMRoundTrip(t *testing.T) {
	_, priv, _ := GenerateKey()
	pemBytes, err := MarshalPrivateKeyPEM(priv)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParsePrivateKeyPEM(pemBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(priv) {
		t.Fatal("privkey round trip mismatch")
	}
}

func TestParsePublicKeysHexMultiple(t *testing.T) {
	p1, _, _ := GenerateKey()
	p2, _, _ := GenerateKey()
	keys, err := ParsePublicKeysHex(MarshalPublicKeyHex(p1) + "," + MarshalPublicKeyHex(p2))
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("want 2 keys, got %d", len(keys))
	}
}
