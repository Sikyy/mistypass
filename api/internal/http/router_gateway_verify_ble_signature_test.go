package httpx

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/credential"
)

// The ble_signature credential type verifies a transport-bound ECDSA signature
// from a mobile credential. The signed challenge is the single-use request
// nonce, so a captured signature cannot be replayed under a new nonce.
func TestResolveBLESignatureToUser(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))

	svc := credential.NewService()
	const tenantID = "tenant_demo_jakarta"
	const userID = "usr_ble_signer_001"
	if _, err := svc.RegisterDevice(credential.RegisterDeviceInput{
		TenantID:      tenantID,
		UserID:        userID,
		UserEmail:     "signer@demo.local",
		PublicKeyPEM:  pubPEM,
		Platform:      "android",
		DeviceID:      "device_ble_001",
		DeviceModel:   "Pixel",
		KeystoreLevel: "strongbox",
	}); err != nil {
		t.Fatalf("register device: %v", err)
	}

	s := &server{credentialSvc: svc}

	const nonce = "nonce-ble-123"
	const transportTag = "BLE"
	// Sign SHA256(nonce || userID || transportTag), matching VerifyBLESignatureV2.
	msg := append(append([]byte(nonce), []byte(userID)...), []byte(transportTag)...)
	hash := sha256.Sum256(msg)
	sig, err := ecdsa.SignASN1(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	makeReq := func(nonceHeader string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/verify-credential", nil)
		if nonceHeader != "" {
			r.Header.Set("X-Request-Nonce", nonceHeader)
		}
		return r
	}
	validReq := verifyCredentialRequest{
		CredentialType: "ble_signature",
		UserID:         userID,
		TransportTag:   transportTag,
		Signature:      base64.StdEncoding.EncodeToString(sig),
	}

	// Valid signature bound to the matching request nonce resolves to the user.
	if uid, ok := s.resolveBLESignatureToUser(makeReq(nonce), tenantID, validReq); !ok || uid != userID {
		t.Fatalf("expected valid signature to resolve %s, got (%q,%v)", userID, uid, ok)
	}

	// A different nonce breaks the signature (prevents replay under a new nonce).
	if _, ok := s.resolveBLESignatureToUser(makeReq("different-nonce"), tenantID, validReq); ok {
		t.Fatalf("expected signature over a different nonce to fail")
	}

	// Missing nonce is rejected.
	if _, ok := s.resolveBLESignatureToUser(makeReq(""), tenantID, validReq); ok {
		t.Fatalf("expected missing nonce to fail")
	}

	// Tampered signature is rejected.
	bad := validReq
	bad.Signature = base64.StdEncoding.EncodeToString([]byte("not-a-real-signature"))
	if _, ok := s.resolveBLESignatureToUser(makeReq(nonce), tenantID, bad); ok {
		t.Fatalf("expected tampered signature to fail")
	}
}
