package enterprise

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/crewjam/saml"
)

var ErrSAMLResponseRequired = errors.New("saml_response is required")
var ErrInvalidSAMLResponse = errors.New("invalid saml_response")
var ErrInvalidSAMLX509Cert = errors.New("invalid saml_x509_cert")
var ErrSAMLAssertionIssuerMismatch = errors.New("saml assertion issuer mismatch")
var ErrSAMLAssertionAudienceMismatch = errors.New("saml assertion audience mismatch")
var ErrSAMLAssertionSubjectRequired = errors.New("saml assertion subject is required")
var ErrSAMLAssertionEmailRequired = errors.New("saml assertion email is required")
var ErrSAMLAssertionEmailMismatch = errors.New("saml assertion email mismatch")

type SAMLIdentity struct {
	Subject    string              `json:"subject"`
	Email      string              `json:"email"`
	Issuer     string              `json:"issuer"`
	Audience   []string            `json:"audience,omitempty"`
	ExpiresAt  time.Time           `json:"expires_at,omitempty"`
	IssuedAt   time.Time           `json:"issued_at,omitempty"`
	Attributes map[string][]string `json:"attributes,omitempty"`
}

func (s *Service) VerifySAMLResponse(config IDPConfig, rawResponse, expectedEmail string) (SAMLIdentity, error) {
	if strings.ToLower(strings.TrimSpace(config.Provider)) != "saml" {
		return SAMLIdentity{}, ErrInvalidIDPProvider
	}

	nextRawResponse := strings.TrimSpace(rawResponse)
	if nextRawResponse == "" {
		return SAMLIdentity{}, ErrSAMLResponseRequired
	}

	nextACSURL := strings.TrimSpace(config.SAMLACSURL)
	if nextACSURL == "" {
		// Backward compatibility for older clients.
		nextACSURL = strings.TrimSpace(config.AuthURL)
	}
	if nextACSURL == "" {
		return SAMLIdentity{}, ErrSAMLACSURLRequired
	}
	if !looksLikeHTTPSURL(nextACSURL) {
		return SAMLIdentity{}, ErrInvalidSAMLACSURL
	}

	normalizedCert, err := normalizeSAMLX509Certificate(strings.TrimSpace(config.SAMLX509Cert))
	if err != nil {
		return SAMLIdentity{}, err
	}

	responseXML, err := decodeSAMLResponse(nextRawResponse)
	if err != nil {
		return SAMLIdentity{}, err
	}

	sp, err := buildSAMLServiceProvider(strings.TrimSpace(config.IssuerURL), strings.TrimSpace(config.ClientID), nextACSURL, normalizedCert)
	if err != nil {
		return SAMLIdentity{}, err
	}

	acsURLParsed, err := url.Parse(nextACSURL)
	if err != nil {
		return SAMLIdentity{}, ErrInvalidSAMLACSURL
	}

	assertion, err := sp.ParseXMLResponse(responseXML, nil, *acsURLParsed)
	if err != nil || assertion == nil {
		return SAMLIdentity{}, ErrInvalidSAMLResponse
	}

	identity, err := buildSAMLIdentity(assertion)
	if err != nil {
		return SAMLIdentity{}, err
	}

	expectedIssuer := strings.TrimSpace(config.IssuerURL)
	if expectedIssuer != "" && strings.TrimSpace(identity.Issuer) != expectedIssuer {
		return SAMLIdentity{}, ErrSAMLAssertionIssuerMismatch
	}

	expectedAudience := strings.TrimSpace(config.ClientID)
	if expectedAudience != "" && !containsAudience(identity.Audience, expectedAudience) {
		return SAMLIdentity{}, ErrSAMLAssertionAudienceMismatch
	}

	expected := normalizeEmail(expectedEmail)
	if expected != "" && expected != identity.Email {
		return SAMLIdentity{}, ErrSAMLAssertionEmailMismatch
	}

	return identity, nil
}

func buildSAMLServiceProvider(idpEntityID, spEntityID, acsURL, signingCert string) (saml.ServiceProvider, error) {
	acsParsed, err := url.Parse(strings.TrimSpace(acsURL))
	if err != nil {
		return saml.ServiceProvider{}, ErrInvalidSAMLResponse
	}

	entityID := strings.TrimSpace(spEntityID)
	if entityID == "" {
		entityID = acsParsed.String()
	}

	metadataURL := *acsParsed
	metadataURL.Path = strings.TrimSuffix(metadataURL.Path, "/")
	if metadataURL.Path == "" {
		metadataURL.Path = "/saml/metadata"
	}

	return saml.ServiceProvider{
		EntityID:          entityID,
		AcsURL:            *acsParsed,
		MetadataURL:       metadataURL,
		AllowIDPInitiated: true,
		IDPMetadata: &saml.EntityDescriptor{
			EntityID: strings.TrimSpace(idpEntityID),
			IDPSSODescriptors: []saml.IDPSSODescriptor{
				{
					SSODescriptor: saml.SSODescriptor{
						RoleDescriptor: saml.RoleDescriptor{
							ProtocolSupportEnumeration: "urn:oasis:names:tc:SAML:2.0:protocol",
							KeyDescriptors: []saml.KeyDescriptor{
								{
									Use: "signing",
									KeyInfo: saml.KeyInfo{
										X509Data: saml.X509Data{
											X509Certificates: []saml.X509Certificate{
												{Data: signingCert},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

func buildSAMLIdentity(assertion *saml.Assertion) (SAMLIdentity, error) {
	subject := ""
	if assertion.Subject != nil && assertion.Subject.NameID != nil {
		subject = strings.TrimSpace(assertion.Subject.NameID.Value)
	}
	if subject == "" {
		return SAMLIdentity{}, ErrSAMLAssertionSubjectRequired
	}

	attributes := collectSAMLAttributes(assertion)
	email := extractSAMLEmail(attributes)
	if email == "" && looksLikeEmail(subject) {
		email = normalizeEmail(subject)
	}
	if email == "" {
		return SAMLIdentity{}, ErrSAMLAssertionEmailRequired
	}

	identity := SAMLIdentity{
		Subject:  subject,
		Email:    email,
		Issuer:   strings.TrimSpace(assertion.Issuer.Value),
		Audience: collectSAMLAudience(assertion),
		IssuedAt: assertion.IssueInstant,
	}
	if assertion.Conditions != nil {
		identity.ExpiresAt = assertion.Conditions.NotOnOrAfter
	}
	if len(attributes) > 0 {
		identity.Attributes = attributes
	}

	return identity, nil
}

func decodeSAMLResponse(raw string) ([]byte, error) {
	next := strings.TrimSpace(raw)
	if next == "" {
		return nil, ErrSAMLResponseRequired
	}

	if strings.HasPrefix(next, "<") {
		return []byte(next), nil
	}

	for _, prefix := range []string{
		"data:application/samlassertion+xml;base64,",
		"data:application/xml;base64,",
		"data:text/xml;base64,",
	} {
		if strings.HasPrefix(next, prefix) {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(next, prefix))
			if err != nil {
				return nil, ErrInvalidSAMLResponse
			}
			return decoded, nil
		}
	}

	candidates := []string{next}
	if unescaped, err := url.QueryUnescape(next); err == nil && unescaped != next {
		candidates = append(candidates, unescaped)
	}

	for i := range candidates {
		value := compactWhitespace(candidates[i])
		if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
			return decoded, nil
		}
		if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil {
			return decoded, nil
		}
		if decoded, err := base64.URLEncoding.DecodeString(value); err == nil {
			return decoded, nil
		}
		if decoded, err := base64.RawURLEncoding.DecodeString(value); err == nil {
			return decoded, nil
		}
	}

	return nil, ErrInvalidSAMLResponse
}

func normalizeSAMLX509Certificate(raw string) (string, error) {
	next := strings.TrimSpace(raw)
	if next == "" {
		return "", ErrSAMLX509CertRequired
	}

	for _, prefix := range []string{
		"data:application/x-pem-file;base64,",
		"data:application/pkix-cert;base64,",
		"data:application/octet-stream;base64,",
	} {
		if strings.HasPrefix(next, prefix) {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(next, prefix))
			if err != nil {
				return "", ErrInvalidSAMLX509Cert
			}
			next = string(decoded)
			break
		}
	}

	var certDER []byte
	if strings.Contains(next, "-----BEGIN CERTIFICATE-----") {
		block, _ := pem.Decode([]byte(next))
		if block == nil || len(block.Bytes) == 0 {
			return "", ErrInvalidSAMLX509Cert
		}
		certDER = block.Bytes
	} else {
		compact := compactWhitespace(next)
		decoded, err := base64.StdEncoding.DecodeString(compact)
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(compact)
			if err != nil {
				return "", ErrInvalidSAMLX509Cert
			}
		}
		certDER = decoded
	}

	parsed, err := x509.ParseCertificate(certDER)
	if err != nil || parsed == nil {
		return "", ErrInvalidSAMLX509Cert
	}
	return base64.StdEncoding.EncodeToString(parsed.Raw), nil
}

func collectSAMLAttributes(assertion *saml.Assertion) map[string][]string {
	out := map[string][]string{}

	push := func(key, value string) {
		nextKey := strings.ToLower(strings.TrimSpace(key))
		nextValue := strings.TrimSpace(value)
		if nextKey == "" || nextValue == "" {
			return
		}
		values := out[nextKey]
		for i := range values {
			if values[i] == nextValue {
				return
			}
		}
		out[nextKey] = append(values, nextValue)
	}

	for i := range assertion.AttributeStatements {
		for j := range assertion.AttributeStatements[i].Attributes {
			attr := assertion.AttributeStatements[i].Attributes[j]
			keys := []string{attr.Name, attr.FriendlyName}
			for k := range attr.Values {
				value := strings.TrimSpace(attr.Values[k].Value)
				if value == "" && attr.Values[k].NameID != nil {
					value = strings.TrimSpace(attr.Values[k].NameID.Value)
				}
				for x := range keys {
					push(keys[x], value)
				}
			}
		}
	}

	return out
}

func collectSAMLAudience(assertion *saml.Assertion) []string {
	if assertion.Conditions == nil {
		return nil
	}

	items := make([]string, 0, 2)
	seen := map[string]struct{}{}
	for i := range assertion.Conditions.AudienceRestrictions {
		next := strings.TrimSpace(assertion.Conditions.AudienceRestrictions[i].Audience.Value)
		if next == "" {
			continue
		}
		if _, exists := seen[next]; exists {
			continue
		}
		seen[next] = struct{}{}
		items = append(items, next)
	}
	return items
}

func extractSAMLEmail(attributes map[string][]string) string {
	for _, key := range []string{
		"email",
		"mail",
		"emailaddress",
		"urn:oid:0.9.2342.19200300.100.1.3",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
		"urn:oasis:names:tc:saml:attribute:subject-id",
	} {
		values := attributes[key]
		for i := range values {
			next := normalizeEmail(values[i])
			if looksLikeEmail(next) {
				return next
			}
		}
	}
	return ""
}

func looksLikeEmail(value string) bool {
	next := normalizeEmail(value)
	if next == "" {
		return false
	}
	parts := strings.Split(next, "@")
	if len(parts) != 2 {
		return false
	}
	return strings.Contains(parts[1], ".")
}

func compactWhitespace(value string) string {
	return strings.Join(strings.Fields(value), "")
}
