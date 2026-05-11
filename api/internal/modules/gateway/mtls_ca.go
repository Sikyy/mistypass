// Package gateway provides the internal CA for mTLS device authentication.
//
// Architecture:
//
//	Registration flow (one-time):
//	  Gateway generates ECDSA P-256 keypair →
//	  Gateway sends CSR to POST /api/v1/gateway/register →
//	  Cloud CA signs CSR, returns client certificate →
//	  Gateway persists cert + private key locally
//
//	Every subsequent request:
//	  Gateway presents client cert during TLS handshake →
//	  Server verifies cert against CA →
//	  Server extracts gateway_id from cert CN →
//	  No bearer tokens needed (identity is in the cert)
//
//	Ephemeral key exchange:
//	  TLS 1.3 ECDHE provides forward secrecy automatically.
//	  Each TLS session negotiates new ephemeral keys.
//	  Compromising one session's keys doesn't expose others.
//
// Security invariants:
//   - CA private key MUST be stored in HSM/KMS in production (env: GATEWAY_CA_KEY)
//   - Client certs are short-lived (72h) matching MaxOfflineDuration
//   - Gateway must renew cert before expiry (online renewal via existing connection)
//   - Revocation: remove gateway from DB = cert rejected at verification time
package gateway

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// DeviceCACertLifetime is the maximum device client certificate lifetime.
// Production defaults should stay shorter than this to bound revocation delay.
const DeviceCACertLifetime = 72 * time.Hour

// DefaultDeviceCACertLifetime is the default issued client certificate lifetime.
const DefaultDeviceCACertLifetime = 24 * time.Hour

// DeviceCA is a minimal certificate authority for signing gateway client certs.
// In production, the CA private key should be backed by HSM/KMS.
type DeviceCA struct {
	caCert  *x509.Certificate
	caKey   *ecdsa.PrivateKey
	certTTL time.Duration

	// PEM-encoded CA certificate for clients to verify server-side
	CACertPEM []byte
}

// NewDeviceCA creates a new self-signed CA for gateway mTLS.
// In production, load the CA cert+key from secure storage instead.
func NewDeviceCA() (*DeviceCA, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Mistyislet"},
			CommonName:   "Mistyislet Gateway CA",
		},
		NotBefore:             time.Now().Add(-1 * time.Minute),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0, // only sign end-entity certs
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("create CA cert: %w", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, fmt.Errorf("parse CA cert: %w", err)
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})

	return &DeviceCA{
		caCert:    caCert,
		caKey:     caKey,
		certTTL:   DefaultDeviceCACertLifetime,
		CACertPEM: caCertPEM,
	}, nil
}

// LoadDeviceCA loads an existing CA from PEM-encoded cert and key.
func LoadDeviceCA(certPEM, keyPEM []byte) (*DeviceCA, error) {
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("no PEM certificate found")
	}
	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse CA cert: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("no PEM key found")
	}
	caKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse CA key: %w", err)
	}

	return &DeviceCA{
		caCert:    caCert,
		caKey:     caKey,
		certTTL:   DefaultDeviceCACertLifetime,
		CACertPEM: certPEM,
	}, nil
}

// SetCertificateLifetime overrides the issued client certificate lifetime.
func (ca *DeviceCA) SetCertificateLifetime(ttl time.Duration) error {
	if ca == nil {
		return fmt.Errorf("device CA is nil")
	}
	if ttl <= 0 {
		return fmt.Errorf("certificate lifetime must be positive")
	}
	if ttl > DeviceCACertLifetime {
		return fmt.Errorf("certificate lifetime must not exceed %s", DeviceCACertLifetime)
	}
	ca.certTTL = ttl
	return nil
}

// CertificateLifetime returns the effective issued client certificate lifetime.
func (ca *DeviceCA) CertificateLifetime() time.Duration {
	if ca == nil || ca.certTTL <= 0 {
		return DefaultDeviceCACertLifetime
	}
	return ca.certTTL
}

// SignCSR signs a gateway's Certificate Signing Request and returns the client cert PEM.
//
// Deprecated: use SignGatewayCSR so the caller must bind the CSR subject to the
// gateway record confirmed by registration.
func (ca *DeviceCA) SignCSR(csrPEM []byte) (certPEM []byte, err error) {
	csr, err := parseGatewayCSR(csrPEM)
	if err != nil {
		return nil, err
	}
	return ca.signParsedCSR(csr)
}

// SignGatewayCSR signs a CSR only when its subject matches the registered
// gateway identity. CN must be gatewayID and O[0] must be tenantID.
func (ca *DeviceCA) SignGatewayCSR(csrPEM []byte, gatewayID, tenantID string) (certPEM []byte, err error) {
	csr, err := parseGatewayCSR(csrPEM)
	if err != nil {
		return nil, err
	}

	expectedGatewayID := strings.TrimSpace(gatewayID)
	expectedTenantID := strings.TrimSpace(tenantID)
	if expectedGatewayID == "" {
		return nil, fmt.Errorf("gateway_id is required")
	}
	if expectedTenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}
	if strings.TrimSpace(csr.Subject.CommonName) != expectedGatewayID {
		return nil, fmt.Errorf("CSR common name %q does not match gateway_id %q", csr.Subject.CommonName, expectedGatewayID)
	}
	if len(csr.Subject.Organization) == 0 || strings.TrimSpace(csr.Subject.Organization[0]) != expectedTenantID {
		return nil, fmt.Errorf("CSR organization %q does not match tenant_id %q", strings.Join(csr.Subject.Organization, ","), expectedTenantID)
	}

	return ca.signParsedCSR(csr)
}

func parseGatewayCSR(csrPEM []byte) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("invalid CSR PEM")
	}

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse CSR: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("CSR signature invalid: %w", err)
	}
	return csr, nil
}

func (ca *DeviceCA) signParsedCSR(csr *x509.CertificateRequest) (certPEM []byte, err error) {
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      csr.Subject, // CN = gateway_id, O = tenant_id
		NotBefore:    now.Add(-5 * time.Minute),
		NotAfter:     now.Add(ca.CertificateLifetime()),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca.caCert, csr.PublicKey, ca.caKey)
	if err != nil {
		return nil, fmt.Errorf("sign certificate: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), nil
}

// CACertPool returns a CertPool containing this CA's certificate.
// Use this to configure tls.Config.ClientCAs on the server.
func (ca *DeviceCA) CACertPool() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(ca.caCert)
	return pool
}
