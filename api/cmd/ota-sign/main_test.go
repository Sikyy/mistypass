package main

import (
	"crypto/ed25519"
	"testing"

	"github.com/mistypass/cloud/api/internal/otasig"
)

func TestSignFileProducesAgentVerifiableSignature(t *testing.T) {
	pub, priv, _ := otasig.GenerateKey()
	pemBytes, _ := otasig.MarshalPrivateKeyPEM(priv)
	data := []byte("agent binary v1.4.0")

	sha, sig, err := signFile(pemBytes, "1.4.0", data)
	if err != nil {
		t.Fatal(err)
	}
	// The agent-side verifier must accept what the CLI produced.
	if err := otasig.VerifyArtifact([]ed25519.PublicKey{pub}, "1.4.0", sha, sig, data); err != nil {
		t.Fatalf("agent would reject CLI signature: %v", err)
	}
}

func TestSignFileRejectsEmptyVersion(t *testing.T) {
	_, priv, _ := otasig.GenerateKey()
	pemBytes, _ := otasig.MarshalPrivateKeyPEM(priv)
	if _, _, err := signFile(pemBytes, "", []byte("fw")); err == nil {
		t.Fatal("expected error for empty version")
	}
}
