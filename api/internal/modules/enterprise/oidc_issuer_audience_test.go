package enterprise

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func newOIDCTestSigner(t *testing.T, kid string) (*rsa.PrivateKey, []byte) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	doc := jwksDocument{Keys: []jwkKey{{
		Kid: kid,
		Kty: "RSA",
		Alg: "RS256",
		Use: "sig",
		N:   base64.RawURLEncoding.EncodeToString(priv.PublicKey.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.PublicKey.E)).Bytes()),
	}}}
	payload, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal jwks: %v", err)
	}
	return priv, payload
}

func signOIDCTestToken(t *testing.T, priv *rsa.PrivateKey, kid, issuer, audience, email string) string {
	t.Helper()
	claims := oidcClaims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "sub-123",
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// An IdP config without a configured issuer or audience must NOT silently skip
// those checks — otherwise any token signed by the configured JWKS (any issuer,
// any audience) is accepted, enabling cross-IdP token injection.
func TestVerifyOIDCIDTokenRequiresIssuerAndAudienceConfig(t *testing.T) {
	clearOIDCJWKSCache()
	priv, jwks := newOIDCTestSigner(t, "kid-issaud")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(jwks)
	}))
	defer server.Close()

	const issuer = "https://idp.example"
	const audience = "client-123"
	const email = "user@corp.example"
	token := signOIDCTestToken(t, priv, "kid-issaud", issuer, audience, email)

	svc := &Service{}

	// Positive control: a fully configured IdP verifies a valid token.
	okCfg := IDPConfig{Provider: "oidc", JWKSURL: server.URL, IssuerURL: issuer, ClientID: audience}
	identity, err := svc.VerifyOIDCIDToken(okCfg, token, "")
	if err != nil {
		t.Fatalf("expected valid token to verify with full config, got %v", err)
	}
	if identity.Email != email {
		t.Fatalf("expected email %q, got %q", email, identity.Email)
	}

	// Blank issuer must be rejected (issuer check must not be skipped).
	blankIssuer := IDPConfig{Provider: "oidc", JWKSURL: server.URL, IssuerURL: "", ClientID: audience}
	if _, err := svc.VerifyOIDCIDToken(blankIssuer, token, ""); err == nil {
		t.Fatalf("expected rejection when issuer_url is not configured, but verification succeeded")
	}

	// Blank audience must be rejected (audience check must not be skipped).
	blankAudience := IDPConfig{Provider: "oidc", JWKSURL: server.URL, IssuerURL: issuer, ClientID: ""}
	if _, err := svc.VerifyOIDCIDToken(blankAudience, token, ""); err == nil {
		t.Fatalf("expected rejection when client_id (audience) is not configured, but verification succeeded")
	}
}
