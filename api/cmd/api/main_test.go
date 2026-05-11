package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/config"
	httpx "github.com/mistypass/cloud/api/internal/http"
)

func TestNewGatewayMTLSServerDisabledWhenAddrEmpty(t *testing.T) {
	server, err := newGatewayMTLSServer(config.Config{}, http.NotFoundHandler())
	if err != nil {
		t.Fatalf("newGatewayMTLSServer returned error: %v", err)
	}
	if server != nil {
		t.Fatalf("expected nil server when GATEWAY_MTLS_ADDR is empty")
	}
}

func TestNewGatewayMTLSServerConfiguresClientCertificateVerification(t *testing.T) {
	serverCertPEM, serverKeyPEM := testSelfSignedServerCertificate(t)
	caCertPEM := testSelfSignedCACertificate(t)

	server, err := newGatewayMTLSServer(config.Config{
		GatewayMTLSAddr:          ":9443",
		GatewayMTLSServerCertPEM: string(serverCertPEM),
		GatewayMTLSServerKeyPEM:  string(serverKeyPEM),
		GatewayCACertPEM:         string(caCertPEM),
	}, http.NotFoundHandler())
	if err != nil {
		t.Fatalf("newGatewayMTLSServer returned error: %v", err)
	}
	if server == nil {
		t.Fatalf("expected mTLS server")
	}
	if server.TLSConfig.MinVersion != tls.VersionTLS13 {
		t.Fatalf("expected TLS 1.3 minimum, got %d", server.TLSConfig.MinVersion)
	}
	if server.TLSConfig.ClientAuth != tls.VerifyClientCertIfGiven {
		t.Fatalf("expected VerifyClientCertIfGiven, got %v", server.TLSConfig.ClientAuth)
	}
	if server.TLSConfig.ClientCAs == nil {
		t.Fatalf("expected client CA pool")
	}
	if len(server.TLSConfig.Certificates) != 1 {
		t.Fatalf("expected one server certificate, got %d", len(server.TLSConfig.Certificates))
	}
}

func TestGatewayMTLSOnlyHandlerRestrictsRoutes(t *testing.T) {
	handler := httpx.GatewayMTLSOnlyHandler(http.NotFoundHandler())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected non-gateway route to be hidden, got %d", rec.Code)
	}
}

func testSelfSignedServerCertificate(t *testing.T) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate server key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate server serial: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "gateway-api.local",
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"gateway-api.local"},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create server certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal server key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

func testSelfSignedCACertificate(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate CA serial: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "Mistyislet Gateway CA",
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
