package httpx

import (
	"net/url"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

func TestBuildEnterpriseOIDCAuthorizeURL(t *testing.T) {
	config := enterprise.IDPConfig{
		IssuerURL: "https://id.sudirman.co",
		ClientID:  "mistypass-web-admin",
		Scopes:    []string{"openid", "email"},
	}

	authorizeURL, err := buildEnterpriseOIDCAuthorizeURL(
		config,
		"st_demo_001",
		"https://admin.mistypass.local/enterprise/callback",
	)
	if err != nil {
		t.Fatalf("expected authorize URL build to succeed: %v", err)
	}
	parsed, err := url.Parse(authorizeURL)
	if err != nil {
		t.Fatalf("expected valid authorize URL: %v", err)
	}
	if parsed.Scheme != "https" || parsed.Host != "id.sudirman.co" {
		t.Fatalf("unexpected authorize URL host: %s", authorizeURL)
	}
	query := parsed.Query()
	if query.Get("client_id") != "mistypass-web-admin" {
		t.Fatalf("unexpected client_id: %s", query.Get("client_id"))
	}
	if query.Get("state") != "st_demo_001" {
		t.Fatalf("unexpected state param: %s", query.Get("state"))
	}
	if query.Get("scope") != "openid email" {
		t.Fatalf("unexpected scope: %s", query.Get("scope"))
	}
}

func TestBuildEnterpriseSAMLSSOURL(t *testing.T) {
	config := enterprise.IDPConfig{
		AuthURL: "https://idp.sudirman.co/saml/sso",
	}

	ssoURL, err := buildEnterpriseSAMLSSOURL(
		config,
		"st_demo_002",
		"https://admin.mistypass.local/enterprise/callback",
	)
	if err != nil {
		t.Fatalf("expected sso URL build to succeed: %v", err)
	}
	parsed, err := url.Parse(ssoURL)
	if err != nil {
		t.Fatalf("expected valid sso URL: %v", err)
	}
	query := parsed.Query()
	if query.Get("RelayState") != "st_demo_002" {
		t.Fatalf("unexpected RelayState: %s", query.Get("RelayState"))
	}
	if query.Get("redirect_uri") == "" {
		t.Fatalf("expected redirect_uri to be set")
	}
}
