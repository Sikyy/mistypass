package httpx

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func oauth2TestRouter(t *testing.T) (http.Handler, string) {
	t.Helper()
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "oauth2-e2e-secret",
		EnableDemoUsers: true,
		OAuth2Enabled:   true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	return router, token
}

// oauth2RegisterClient creates a client and returns (clientID, clientSecret, redirectURI).
func oauth2RegisterClient(t *testing.T, router http.Handler, sessionToken string, scopes []string) (string, string, string) {
	t.Helper()
	redirectURI := "https://app.example.com/callback"
	scopeJSON, _ := json.Marshal(scopes)
	body := []byte(`{"name":"E2E Client","redirect_uris":["` + redirectURI + `"],"scopes":` + string(scopeJSON) + `}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/oauth2/clients", sessionToken, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected client create 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID           string `json:"id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created client: %v", err)
	}
	if created.ID == "" || created.ClientSecret == "" {
		t.Fatalf("expected client id and secret, got %s", rec.Body.String())
	}
	return created.ID, created.ClientSecret, redirectURI
}

// oauth2ObtainAccessToken runs authorize + token and returns the access token.
func oauth2ObtainAccessToken(t *testing.T, router http.Handler, sessionToken, clientID, clientSecret, redirectURI, scope string) string {
	t.Helper()
	authorizePath := "/oauth2/authorize?response_type=code&client_id=" + clientID + "&scope=" + url.QueryEscape(scope)
	authRec := referenceAPIRequest(t, router, http.MethodGet, authorizePath, sessionToken, nil)
	if authRec.Code != http.StatusOK {
		t.Fatalf("expected authorize 200, got %d body=%s", authRec.Code, authRec.Body.String())
	}
	var authResp struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(authRec.Body.Bytes(), &authResp); err != nil {
		t.Fatalf("decode authorize: %v", err)
	}
	if authResp.Code == "" {
		t.Fatalf("expected authorization code, got %s", authRec.Body.String())
	}

	tokenBody := []byte(`{"grant_type":"authorization_code","code":"` + authResp.Code + `","client_id":"` + clientID + `","client_secret":"` + clientSecret + `","redirect_uri":"` + redirectURI + `"}`)
	tokenRec := referenceAPIRequest(t, router, http.MethodPost, "/oauth2/token", "", tokenBody)
	if tokenRec.Code != http.StatusOK {
		t.Fatalf("expected token 200, got %d body=%s", tokenRec.Code, tokenRec.Body.String())
	}
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(tokenRec.Body.Bytes(), &tokenResp); err != nil {
		t.Fatalf("decode token: %v", err)
	}
	if tokenResp.AccessToken == "" {
		t.Fatalf("expected access token, got %s", tokenRec.Body.String())
	}
	return tokenResp.AccessToken
}

func TestOAuth2AccessTokenAuthenticatesProtectedAPI(t *testing.T) {
	router, session := oauth2TestRouter(t)
	clientID, secret, redirectURI := oauth2RegisterClient(t, router, session, []string{"read", "write"})
	accessToken := oauth2ObtainAccessToken(t, router, session, clientID, secret, redirectURI, "read")

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/places?tenant_id=tenant_demo_jakarta", accessToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected OAuth2 access token to authenticate protected GET, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOAuth2ReadScopeCannotMutate(t *testing.T) {
	router, session := oauth2TestRouter(t)
	clientID, secret, redirectURI := oauth2RegisterClient(t, router, session, []string{"read"})
	accessToken := oauth2ObtainAccessToken(t, router, session, clientID, secret, redirectURI, "read")

	// read scope: GET allowed
	getRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/places?tenant_id=tenant_demo_jakarta", accessToken, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected read-scope GET 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}

	// read scope: mutation rejected with 403 before reaching the handler
	createBody := []byte(`{"place":{"tenant_id":"tenant_demo_jakarta","name":"Scope Test Place"}}`)
	postRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/places", accessToken, createBody)
	if postRec.Code != http.StatusForbidden {
		t.Fatalf("expected read-scope mutation to be 403, got %d body=%s", postRec.Code, postRec.Body.String())
	}
}

func TestOAuth2WriteScopeCanMutate(t *testing.T) {
	router, session := oauth2TestRouter(t)
	clientID, secret, redirectURI := oauth2RegisterClient(t, router, session, []string{"read", "write"})
	accessToken := oauth2ObtainAccessToken(t, router, session, clientID, secret, redirectURI, "read write")

	createBody := []byte(`{"place":{"tenant_id":"tenant_demo_jakarta","name":"Scope Test Place"}}`)
	postRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/places", accessToken, createBody)
	if postRec.Code == http.StatusForbidden || postRec.Code == http.StatusUnauthorized {
		t.Fatalf("expected write-scope mutation to pass auth/scope, got %d body=%s", postRec.Code, postRec.Body.String())
	}
}

func TestOAuth2RevokeInvalidatesAccessToken(t *testing.T) {
	router, session := oauth2TestRouter(t)
	clientID, secret, redirectURI := oauth2RegisterClient(t, router, session, []string{"read"})
	accessToken := oauth2ObtainAccessToken(t, router, session, clientID, secret, redirectURI, "read")

	okRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/places?tenant_id=tenant_demo_jakarta", accessToken, nil)
	if okRec.Code != http.StatusOK {
		t.Fatalf("expected access token to work before revoke, got %d", okRec.Code)
	}

	revokeBody := []byte(`{"token":"` + accessToken + `"}`)
	revokeRec := referenceAPIRequest(t, router, http.MethodPost, "/oauth2/revoke", "", revokeBody)
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("expected revoke 200, got %d body=%s", revokeRec.Code, revokeRec.Body.String())
	}

	afterRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/places?tenant_id=tenant_demo_jakarta", accessToken, nil)
	if afterRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked access token to be 401, got %d body=%s", afterRec.Code, afterRec.Body.String())
	}
}
