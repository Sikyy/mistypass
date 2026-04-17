package enterprise

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrIDPTokenRequired = errors.New("idp_token is required")
var ErrInvalidIDPToken = errors.New("invalid idp_token")
var ErrIDPJWKSURLRequired = errors.New("jwks_url is required")
var ErrOIDCJWKSFetchFailed = errors.New("failed to fetch jwks")
var ErrOIDCJWKSKeyNotFound = errors.New("jwks key not found")
var ErrOIDCTokenIssuerMismatch = errors.New("token issuer mismatch")
var ErrOIDCTokenAudienceMismatch = errors.New("token audience mismatch")
var ErrOIDCTokenSubjectRequired = errors.New("token subject is required")
var ErrOIDCTokenEmailRequired = errors.New("token email is required")
var ErrOIDCTokenEmailMismatch = errors.New("token email mismatch")

type OIDCIdentity struct {
	Subject           string    `json:"subject"`
	Email             string    `json:"email"`
	Name              string    `json:"name,omitempty"`
	Department        string    `json:"department,omitempty"`
	JobTitle          string    `json:"job_title,omitempty"`
	Location          string    `json:"location,omitempty"`
	Phone             string    `json:"phone,omitempty"`
	ManagerExternalID string    `json:"manager_external_id,omitempty"`
	EmploymentStatus  string    `json:"employment_status,omitempty"`
	Issuer            string    `json:"issuer"`
	Audience          []string  `json:"audience"`
	ExpiresAt         time.Time `json:"expires_at"`
	IssuedAt          time.Time `json:"issued_at"`
}

type oidcClaims struct {
	Email             string `json:"email"`
	Name              string `json:"name"`
	Department        string `json:"department"`
	JobTitle          string `json:"job_title"`
	Title             string `json:"title"`
	Location          string `json:"location"`
	PhoneNumber       string `json:"phone_number"`
	Phone             string `json:"phone"`
	ManagerExternalID string `json:"manager_external_id"`
	ManagerID         string `json:"manager_id"`
	EmploymentStatus  string `json:"employment_status"`
	Status            string `json:"status"`
	Active            *bool  `json:"active"`
	jwt.RegisteredClaims
}

type jwksDocument struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (s *Service) VerifyOIDCIDToken(config IDPConfig, rawToken, expectedEmail string) (OIDCIdentity, error) {
	if strings.ToLower(strings.TrimSpace(config.Provider)) != "oidc" {
		return OIDCIdentity{}, ErrInvalidIDPProvider
	}

	nextToken := strings.TrimSpace(rawToken)
	if nextToken == "" {
		return OIDCIdentity{}, ErrIDPTokenRequired
	}

	jwksURL := effectiveJWKSURL(config)
	if jwksURL == "" {
		return OIDCIdentity{}, ErrIDPJWKSURLRequired
	}

	keys, err := fetchJWKS(jwksURL)
	if err != nil {
		return OIDCIdentity{}, ErrOIDCJWKSFetchFailed
	}

	claims := oidcClaims{}
	parsed, err := jwt.ParseWithClaims(nextToken, &claims, func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)
		kid = strings.TrimSpace(kid)
		if kid == "" {
			return nil, ErrOIDCJWKSKeyNotFound
		}

		key, exists := keys[kid]
		if !exists {
			return nil, ErrOIDCJWKSKeyNotFound
		}
		return key, nil
	}, jwt.WithValidMethods([]string{
		jwt.SigningMethodRS256.Alg(),
		jwt.SigningMethodRS384.Alg(),
		jwt.SigningMethodRS512.Alg(),
	}))
	if err != nil || parsed == nil || !parsed.Valid {
		return OIDCIdentity{}, ErrInvalidIDPToken
	}

	expectedIssuer := strings.TrimSpace(config.IssuerURL)
	if expectedIssuer != "" && strings.TrimSpace(claims.Issuer) != expectedIssuer {
		return OIDCIdentity{}, ErrOIDCTokenIssuerMismatch
	}

	expectedAudience := strings.TrimSpace(config.ClientID)
	if expectedAudience != "" && !containsAudience(claims.Audience, expectedAudience) {
		return OIDCIdentity{}, ErrOIDCTokenAudienceMismatch
	}

	subject := strings.TrimSpace(claims.Subject)
	if subject == "" {
		return OIDCIdentity{}, ErrOIDCTokenSubjectRequired
	}

	tokenEmail := normalizeEmail(claims.Email)
	if tokenEmail == "" {
		return OIDCIdentity{}, ErrOIDCTokenEmailRequired
	}

	expected := normalizeEmail(expectedEmail)
	if expected != "" && expected != tokenEmail {
		return OIDCIdentity{}, ErrOIDCTokenEmailMismatch
	}

	identity := OIDCIdentity{
		Subject:           subject,
		Email:             tokenEmail,
		Name:              strings.TrimSpace(claims.Name),
		Department:        strings.TrimSpace(claims.Department),
		JobTitle:          strings.TrimSpace(claims.JobTitle),
		Location:          strings.TrimSpace(claims.Location),
		Phone:             strings.TrimSpace(claims.PhoneNumber),
		ManagerExternalID: strings.TrimSpace(claims.ManagerExternalID),
		EmploymentStatus:  normalizeOIDCEmploymentStatus(claims.EmploymentStatus, claims.Status, claims.Active),
		Issuer:            strings.TrimSpace(claims.Issuer),
		Audience:          append([]string(nil), claims.Audience...),
	}
	if identity.JobTitle == "" {
		identity.JobTitle = strings.TrimSpace(claims.Title)
	}
	if identity.Phone == "" {
		identity.Phone = strings.TrimSpace(claims.Phone)
	}
	if identity.ManagerExternalID == "" {
		identity.ManagerExternalID = strings.TrimSpace(claims.ManagerID)
	}
	if claims.ExpiresAt != nil {
		identity.ExpiresAt = claims.ExpiresAt.Time
	}
	if claims.IssuedAt != nil {
		identity.IssuedAt = claims.IssuedAt.Time
	}

	return identity, nil
}

func effectiveJWKSURL(config IDPConfig) string {
	next := strings.TrimSpace(config.JWKSURL)
	if next != "" {
		return next
	}

	issuer := strings.TrimSpace(config.IssuerURL)
	if issuer == "" {
		return ""
	}
	return strings.TrimSuffix(issuer, "/") + "/.well-known/jwks.json"
}

func fetchJWKS(rawURL string) (map[string]*rsa.PublicKey, error) {
	jwksBytes, err := readJWKSBytes(rawURL)
	if err != nil {
		return nil, err
	}

	var doc jwksDocument
	if err := json.Unmarshal(jwksBytes, &doc); err != nil {
		return nil, err
	}

	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for i := range doc.Keys {
		key := doc.Keys[i]
		if strings.ToUpper(strings.TrimSpace(key.Kty)) != "RSA" {
			continue
		}
		kid := strings.TrimSpace(key.Kid)
		if kid == "" {
			continue
		}
		pub, err := rsaPublicKeyFromJWK(key.N, key.E)
		if err != nil {
			continue
		}
		keys[kid] = pub
	}

	if len(keys) == 0 {
		return nil, ErrOIDCJWKSFetchFailed
	}
	return keys, nil
}

func readJWKSBytes(rawURL string) ([]byte, error) {
	next := strings.TrimSpace(rawURL)
	if next == "" {
		return nil, ErrIDPJWKSURLRequired
	}

	if strings.HasPrefix(next, "data:application/json;base64,") {
		encoded := strings.TrimPrefix(next, "data:application/json;base64,")
		return base64.StdEncoding.DecodeString(encoded)
	}
	if strings.HasPrefix(next, "data:application/json,") {
		escaped := strings.TrimPrefix(next, "data:application/json,")
		unescaped, err := url.QueryUnescape(escaped)
		if err != nil {
			return nil, err
		}
		return []byte(unescaped), nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, next, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, ErrOIDCJWKSFetchFailed
	}
	return io.ReadAll(resp.Body)
}

func rsaPublicKeyFromJWK(nRaw, eRaw string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(nRaw))
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(eRaw))
	if err != nil {
		return nil, err
	}
	if len(nBytes) == 0 || len(eBytes) == 0 {
		return nil, ErrOIDCJWKSFetchFailed
	}

	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for i := range eBytes {
		e = (e << 8) | int(eBytes[i])
	}
	if e <= 0 {
		return nil, ErrOIDCJWKSFetchFailed
	}

	return &rsa.PublicKey{
		N: n,
		E: e,
	}, nil
}

func containsAudience(values []string, expected string) bool {
	nextExpected := strings.TrimSpace(expected)
	if nextExpected == "" {
		return true
	}
	for i := range values {
		if strings.TrimSpace(values[i]) == nextExpected {
			return true
		}
	}
	return false
}

func normalizeOIDCEmploymentStatus(employmentStatus, status string, active *bool) string {
	for _, value := range []string{employmentStatus, status} {
		next := NormalizeEmploymentStatus(value)
		if next != "" {
			return next
		}
	}
	if active == nil {
		return ""
	}
	if *active {
		return "active"
	}
	return "inactive"
}
