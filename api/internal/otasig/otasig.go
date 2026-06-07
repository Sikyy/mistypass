// Package otasig implements the canonical message format and Ed25519
// sign/verify primitives shared by the OTA signing CLI (cmd/ota-sign) and the
// gateway-agent firmware verifier. One copy guarantees signer and verifier
// agree byte-for-byte on what is signed.
package otasig

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// Domain separates OTA signatures from any other use of the key and versions
// the message format.
const Domain = "mistypass-ota-v1"

// SignedMessage builds the canonical bytes that are Ed25519-signed:
//
//	"mistypass-ota-v1\n" + version + "\n" + lowercaseHex(sha256(binary))
func SignedMessage(version, sha256Hex string) []byte {
	return []byte(Domain + "\n" + strings.TrimSpace(version) + "\n" + strings.ToLower(strings.TrimSpace(sha256Hex)))
}

// SHA256Hex returns the lowercase hex SHA-256 of data.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Sign returns the hex Ed25519 signature over SignedMessage(version, sha256Hex).
func Sign(priv ed25519.PrivateKey, version, sha256Hex string) string {
	return hex.EncodeToString(ed25519.Sign(priv, SignedMessage(version, sha256Hex)))
}

// VerifyArtifact confirms data hashes to sha256Hex AND that sigHex is a valid
// Ed25519 signature (by any of keys) over SignedMessage(version, sha256Hex).
func VerifyArtifact(keys []ed25519.PublicKey, version, sha256Hex, sigHex string, data []byte) error {
	if len(keys) == 0 {
		return errors.New("no pinned public keys configured")
	}
	got := SHA256Hex(data)
	if !strings.EqualFold(got, strings.TrimSpace(sha256Hex)) {
		return fmt.Errorf("sha256 mismatch: computed %s, task declared %s", got, sha256Hex)
	}
	sig, err := hex.DecodeString(strings.TrimSpace(sigHex))
	if err != nil {
		return fmt.Errorf("signature hex decode: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("signature length %d, want %d", len(sig), ed25519.SignatureSize)
	}
	msg := SignedMessage(version, got)
	for _, k := range keys {
		if ed25519.Verify(k, msg, sig) {
			return nil
		}
	}
	return errors.New("signature not verified by any pinned public key")
}

// GenerateKey returns a fresh Ed25519 key pair (crypto/rand).
func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(nil)
}

// MarshalPublicKeyHex encodes the 32-byte raw public key as hex.
func MarshalPublicKeyHex(pub ed25519.PublicKey) string {
	return hex.EncodeToString(pub)
}

// ParsePublicKeyHex decodes a 32-byte raw Ed25519 public key from hex.
func ParsePublicKeyHex(s string) (ed25519.PublicKey, error) {
	b, err := hex.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("public key hex decode: %w", err)
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key length %d, want %d", len(b), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(b), nil
}

// ParsePublicKeysHex parses a comma-separated list of hex public keys (rotation).
func ParsePublicKeysHex(csv string) ([]ed25519.PublicKey, error) {
	var keys []ed25519.PublicKey
	for _, part := range strings.Split(csv, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		k, err := ParsePublicKeyHex(part)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

// MarshalPrivateKeyPEM encodes priv as a PKCS#8 PEM block.
func MarshalPrivateKeyPEM(priv ed25519.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// ParsePrivateKeyPEM decodes a PKCS#8 PEM Ed25519 private key.
func ParsePrivateKeyPEM(pemBytes []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("not an Ed25519 private key")
	}
	return priv, nil
}
