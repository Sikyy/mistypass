package enterprise

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/crewjam/saml"
)

func TestDecodeSAMLResponseSupportsMultipleEncodings(t *testing.T) {
	xmlPayload := []byte(`<Assertion ID="abc">ok</Assertion>`)
	base64Payload := base64.StdEncoding.EncodeToString(xmlPayload)

	cases := []struct {
		name string
		raw  string
	}{
		{
			name: "raw xml",
			raw:  string(xmlPayload),
		},
		{
			name: "data url",
			raw:  "data:application/xml;base64," + base64Payload,
		},
		{
			name: "query escaped base64",
			raw:  url.QueryEscape(base64Payload),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeSAMLResponse(tc.raw)
			if err != nil {
				t.Fatalf("decodeSAMLResponse returned error: %v", err)
			}
			if string(got) != string(xmlPayload) {
				t.Fatalf("unexpected decoded payload: %q", string(got))
			}
		})
	}

	if _, err := decodeSAMLResponse("%%%"); !errors.Is(err, ErrInvalidSAMLResponse) {
		t.Fatalf("expected invalid saml response error, got %v", err)
	}
}

func TestNormalizeSAMLX509CertificateSupportsPEMAndDataURL(t *testing.T) {
	pemCert, base64Cert := generateTestCertificate(t)

	gotPEM, err := normalizeSAMLX509Certificate(pemCert)
	if err != nil {
		t.Fatalf("normalizeSAMLX509Certificate(pem) returned error: %v", err)
	}
	if gotPEM != base64Cert {
		t.Fatalf("unexpected normalized pem cert")
	}

	gotDataURL, err := normalizeSAMLX509Certificate("data:application/x-pem-file;base64," + base64.StdEncoding.EncodeToString([]byte(pemCert)))
	if err != nil {
		t.Fatalf("normalizeSAMLX509Certificate(data-url) returned error: %v", err)
	}
	if gotDataURL != base64Cert {
		t.Fatalf("unexpected normalized data url cert")
	}

	if _, err := normalizeSAMLX509Certificate("invalid"); !errors.Is(err, ErrInvalidSAMLX509Cert) {
		t.Fatalf("expected invalid x509 cert error, got %v", err)
	}
}

func TestBuildSAMLIdentityValidatesFieldsAndCollectsMetadata(t *testing.T) {
	issuedAt := time.Date(2026, time.April, 24, 9, 0, 0, 0, time.UTC)
	expiresAt := issuedAt.Add(30 * time.Minute)
	assertion := &saml.Assertion{
		IssueInstant: issuedAt,
		Issuer:       saml.Issuer{Value: "https://idp.example.com"},
		Subject: &saml.Subject{
			NameID: &saml.NameID{Value: "subject-001"},
		},
		Conditions: &saml.Conditions{
			NotOnOrAfter: expiresAt,
			AudienceRestrictions: []saml.AudienceRestriction{
				{Audience: saml.Audience{Value: "sp://mistypass"}},
				{Audience: saml.Audience{Value: "sp://mistypass"}},
				{Audience: saml.Audience{Value: "sp://secondary"}},
			},
		},
		AttributeStatements: []saml.AttributeStatement{
			{
				Attributes: []saml.Attribute{
					{
						Name:         "email",
						FriendlyName: "mail",
						Values: []saml.AttributeValue{
							{Value: " User@Example.com "},
						},
					},
					{
						FriendlyName: "department",
						Values: []saml.AttributeValue{
							{Value: "Finance"},
						},
					},
				},
			},
		},
	}

	identity, err := buildSAMLIdentity(assertion)
	if err != nil {
		t.Fatalf("buildSAMLIdentity returned error: %v", err)
	}
	if identity.Subject != "subject-001" {
		t.Fatalf("unexpected subject: %s", identity.Subject)
	}
	if identity.Email != "user@example.com" {
		t.Fatalf("unexpected email: %s", identity.Email)
	}
	if identity.Issuer != "https://idp.example.com" {
		t.Fatalf("unexpected issuer: %s", identity.Issuer)
	}
	if len(identity.Audience) != 2 {
		t.Fatalf("expected deduped audience entries, got %+v", identity.Audience)
	}
	if identity.ExpiresAt != expiresAt || identity.IssuedAt != issuedAt {
		t.Fatalf("unexpected issued/expires timestamps: %+v", identity)
	}
	if got := identity.Attributes["department"]; len(got) != 1 || got[0] != "Finance" {
		t.Fatalf("unexpected department attributes: %+v", identity.Attributes)
	}
	if got := identity.Attributes["mail"]; len(got) != 1 || got[0] != "User@Example.com" {
		t.Fatalf("unexpected mail attributes: %+v", identity.Attributes)
	}

	if _, err := buildSAMLIdentity(&saml.Assertion{}); !errors.Is(err, ErrSAMLAssertionSubjectRequired) {
		t.Fatalf("expected subject required error, got %v", err)
	}

	if _, err := buildSAMLIdentity(&saml.Assertion{
		Subject: &saml.Subject{
			NameID: &saml.NameID{Value: "subject-without-email"},
		},
	}); !errors.Is(err, ErrSAMLAssertionEmailRequired) {
		t.Fatalf("expected email required error, got %v", err)
	}
}

func generateTestCertificate(t *testing.T) (string, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	template := &x509.Certificate{
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		NotAfter:              time.Now().Add(time.Hour),
		NotBefore:             time.Now().Add(-time.Hour),
		SerialNumber:          big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "mistypass-saml-test",
		},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate returned error: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: der,
	})), base64.StdEncoding.EncodeToString(der)
}
