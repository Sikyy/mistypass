package gateway

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"testing"
	"time"
)

func TestNewDeviceCA(t *testing.T) {
	ca, err := NewDeviceCA()
	if err != nil {
		t.Fatalf("NewDeviceCA: %v", err)
	}
	if ca.caCert == nil {
		t.Fatal("CA cert is nil")
	}
	if !ca.caCert.IsCA {
		t.Fatal("CA cert IsCA should be true")
	}
	if len(ca.CACertPEM) == 0 {
		t.Fatal("CACertPEM is empty")
	}
}

func TestSignCSR(t *testing.T) {
	ca, err := NewDeviceCA()
	if err != nil {
		t.Fatalf("NewDeviceCA: %v", err)
	}

	// Generate a gateway key and CSR
	gwKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	csrTemplate := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   "gw_demo_001",
			Organization: []string{"tenant_demo_jakarta"},
		},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, csrTemplate, gwKey)
	if err != nil {
		t.Fatalf("create CSR: %v", err)
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})

	// Sign it
	certPEM, err := ca.SignCSR(csrPEM)
	if err != nil {
		t.Fatalf("SignCSR: %v", err)
	}

	// Parse and verify
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("no cert PEM returned")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	if cert.Subject.CommonName != "gw_demo_001" {
		t.Errorf("CN = %q, want gw_demo_001", cert.Subject.CommonName)
	}
	if len(cert.Subject.Organization) == 0 || cert.Subject.Organization[0] != "tenant_demo_jakarta" {
		t.Errorf("O = %v, want [tenant_demo_jakarta]", cert.Subject.Organization)
	}
	if cert.NotAfter.Sub(cert.NotBefore) > DeviceCACertLifetime+2*time.Minute {
		t.Errorf("cert lifetime too long: %v", cert.NotAfter.Sub(cert.NotBefore))
	}

	// Verify cert against CA pool
	pool := ca.CACertPool()
	_, err = cert.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	if err != nil {
		t.Fatalf("cert verification failed: %v", err)
	}
}

func TestSignCSR_InvalidCSR(t *testing.T) {
	ca, _ := NewDeviceCA()

	_, err := ca.SignCSR([]byte("not a pem"))
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}

	_, err = ca.SignCSR(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: []byte("garbage")}))
	if err == nil {
		t.Fatal("expected error for garbage CSR")
	}
}

func TestLoadDeviceCA(t *testing.T) {
	// Create a CA, export, reload
	ca, err := NewDeviceCA()
	if err != nil {
		t.Fatalf("NewDeviceCA: %v", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(ca.caKey)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	loaded, err := LoadDeviceCA(ca.CACertPEM, keyPEM)
	if err != nil {
		t.Fatalf("LoadDeviceCA: %v", err)
	}

	// Sign with loaded CA, verify with original
	gwKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "gw_test"},
	}, gwKey)
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})

	certPEM, err := loaded.SignCSR(csrPEM)
	if err != nil {
		t.Fatalf("sign with loaded CA: %v", err)
	}

	block, _ := pem.Decode(certPEM)
	cert, _ := x509.ParseCertificate(block.Bytes)
	_, err = cert.Verify(x509.VerifyOptions{
		Roots:     ca.CACertPool(),
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	if err != nil {
		t.Fatalf("cross-verify failed: %v", err)
	}
}

func TestMTLSHandshake(t *testing.T) {
	// Full mTLS round-trip: CA signs gateway cert, both sides verify
	ca, _ := NewDeviceCA()

	// Generate gateway keypair + CSR
	gwKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   "gw_mtls_test",
			Organization: []string{"tenant_test"},
		},
	}, gwKey)
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})

	// CA signs
	certPEM, _ := ca.SignCSR(csrPEM)

	// Build tls.Certificate for client
	keyDER, _ := x509.MarshalECPrivateKey(gwKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	clientCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}

	// Verify: client cert should be parseable and valid
	leaf, err := x509.ParseCertificate(clientCert.Certificate[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	if leaf.Subject.CommonName != "gw_mtls_test" {
		t.Errorf("leaf CN = %q", leaf.Subject.CommonName)
	}

	// Verify against CA pool (simulates server verification)
	_, err = leaf.Verify(x509.VerifyOptions{
		Roots:     ca.CACertPool(),
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	if err != nil {
		t.Fatalf("server-side verification failed: %v", err)
	}
}
