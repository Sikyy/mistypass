package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DeviceMTLS manages the gateway's client certificate for mTLS authentication.
//
// Lifecycle:
//  1. First boot: GenerateCSR() creates ECDSA P-256 keypair + CSR
//  2. Registration: CSR sent to Cloud, Cloud CA signs it, cert returned
//  3. StoreCert() persists cert + key to disk
//  4. TLSConfig() returns tls.Config with client cert for all connections
//  5. Renewal: before cert expires, gateway requests a new cert using existing connection
//
// Security:
//   - Private key never leaves the device (only CSR is sent)
//   - Cert lifetime = 72h (matches MaxOfflineDuration)
//   - On iMX93 production hardware, private key should be stored in SE050 secure element
type DeviceMTLS struct {
	certDir string // directory to store cert + key (e.g. /var/lib/mistypass/mtls/)

	mu         sync.RWMutex
	privateKey *ecdsa.PrivateKey
	certPEM    []byte
	caCertPEM  []byte // CA cert for server verification
	cert       *tls.Certificate
}

// NewDeviceMTLS creates a new mTLS manager.
func NewDeviceMTLS(certDir string) *DeviceMTLS {
	return &DeviceMTLS{certDir: certDir}
}

// Load attempts to load existing cert + key from disk.
// Returns false if no cert exists (need to register).
func (m *DeviceMTLS) Load() bool {
	certPath := filepath.Join(m.certDir, "device.crt")
	keyPath := filepath.Join(m.certDir, "device.key")
	caPath := filepath.Join(m.certDir, "ca.crt")

	certPEM, err := os.ReadFile(certPath) // #nosec G304 -- path from trusted certDir config
	if err != nil {
		return false
	}
	keyPEM, err := os.ReadFile(keyPath) // #nosec G304 -- path from trusted certDir config
	if err != nil {
		return false
	}

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return false
	}

	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return false
	}

	if time.Until(leaf.NotAfter) < 1*time.Hour {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.certPEM = certPEM
	m.cert = &tlsCert

	block, _ := pem.Decode(keyPEM)
	if block != nil {
		if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
			m.privateKey = key
		}
	}

	caPEM, caErr := os.ReadFile(caPath) // #nosec G304 -- path from trusted certDir config
	if caErr == nil {
		m.caCertPEM = caPEM
	}

	return true
}

// GenerateCSR creates a new ECDSA P-256 keypair and returns a CSR PEM.
// The private key is held in memory until StoreCert() persists it.
func (m *DeviceMTLS) GenerateCSR(gatewayID, tenantID string) (csrPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	m.privateKey = key

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   gatewayID,
			Organization: []string{tenantID},
		},
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		return nil, fmt.Errorf("create CSR: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}), nil
}

// StoreCert saves the signed certificate and private key to disk.
// Uses atomic write-to-temp + rename to avoid partial files on crash.
func (m *DeviceMTLS) StoreCert(certPEM, caCertPEM []byte) error {
	if m.privateKey == nil {
		return fmt.Errorf("no private key (call GenerateCSR first)")
	}

	if err := os.MkdirAll(m.certDir, 0700); err != nil {
		return fmt.Errorf("mkdir %s: %w", m.certDir, err)
	}

	keyDER, err := x509.MarshalECPrivateKey(m.privateKey)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	// Validate the keypair before writing anything to disk
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("build tls cert: %w", err)
	}

	certPath := filepath.Join(m.certDir, "device.crt")
	keyPath := filepath.Join(m.certDir, "device.key")
	caPath := filepath.Join(m.certDir, "ca.crt")

	if err := atomicWriteFile(certPath, certPEM, 0600); err != nil {
		return fmt.Errorf("write cert: %w", err)
	}
	if err := atomicWriteFile(keyPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("write key: %w", err)
	}
	if len(caCertPEM) > 0 {
		if err := atomicWriteFile(caPath, caCertPEM, 0600); err != nil {
			return fmt.Errorf("write CA cert: %w", err)
		}
	}

	m.mu.Lock()
	m.certPEM = certPEM
	if len(caCertPEM) > 0 {
		m.caCertPEM = caCertPEM
	}
	m.cert = &tlsCert
	m.mu.Unlock()

	return nil
}

// atomicWriteFile writes data to path via a temp file in the same directory
// followed by an atomic rename, so a crash mid-write can never leave a partial
// file at path. perm is applied to the final file. Shared by the mTLS cert
// store and the OTA self-update path.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// HasValidCert returns true if a valid, non-expired client cert is loaded.
func (m *DeviceMTLS) HasValidCert() bool {
	m.mu.RLock()
	cert := m.cert
	m.mu.RUnlock()

	if cert == nil || len(cert.Certificate) == 0 {
		return false
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return false
	}
	return time.Until(leaf.NotAfter) > 1*time.Hour
}

// NeedsRenewal returns true if the cert expires within the given window.
func (m *DeviceMTLS) NeedsRenewal(window time.Duration) bool {
	m.mu.RLock()
	cert := m.cert
	m.mu.RUnlock()

	if cert == nil || len(cert.Certificate) == 0 {
		return true
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return true
	}
	return time.Until(leaf.NotAfter) < window
}

// TLSConfig returns a tls.Config with the client certificate for mTLS.
// Falls back to standard TLS if no client cert is loaded.
// Note: certificate pinning is handled separately in Agent.buildTLSConfig().
func (m *DeviceMTLS) TLSConfig(_ string) *tls.Config {
	m.mu.RLock()
	cert := m.cert
	caCertPEM := m.caCertPEM
	m.mu.RUnlock()

	cfg := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	if cert != nil {
		cfg.Certificates = []tls.Certificate{*cert}
	}

	if len(caCertPEM) > 0 {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caCertPEM)
		cfg.RootCAs = pool
	}

	return cfg
}

// CertInfo returns human-readable info about the loaded cert.
func (m *DeviceMTLS) CertInfo() (cn string, notAfter time.Time, ok bool) {
	m.mu.RLock()
	cert := m.cert
	m.mu.RUnlock()

	if cert == nil || len(cert.Certificate) == 0 {
		return "", time.Time{}, false
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return "", time.Time{}, false
	}
	return leaf.Subject.CommonName, leaf.NotAfter, true
}
