package httpx

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
	"golang.org/x/crypto/bcrypt"
)

// ---------------------------------------------------------------------------
// Data model — authorization codes only (clients now live in access service)
// ---------------------------------------------------------------------------

type OAuth2AuthorizationCode struct {
	Code        string    `json:"code"`
	ClientID    string    `json:"client_id"`
	TenantID    string    `json:"tenant_id"`
	UserID      string    `json:"user_id"`
	Scopes      []string  `json:"scopes"`
	RedirectURI string    `json:"redirect_uri"`
	ExpiresAt   time.Time `json:"expires_at"`
	Used        bool      `json:"used"`
}

// ---------------------------------------------------------------------------
// In-memory store for transient authorization codes (10-min TTL)
// ---------------------------------------------------------------------------

type oauth2Store struct {
	mu    sync.RWMutex
	codes map[string]OAuth2AuthorizationCode // key = code
}

func newOAuth2Store() *oauth2Store {
	return &oauth2Store{
		codes: make(map[string]OAuth2AuthorizationCode),
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func oauth2RandomHex(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

const oauth2AuthCodeTTL = 10 * time.Minute

// validOAuth2Scopes is the set of accepted scope values.
var validOAuth2Scopes = map[string]struct{}{
	"read":  {},
	"write": {},
	"admin": {},
}

func normalizeOAuth2Scopes(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" {
			continue
		}
		if _, ok := validOAuth2Scopes[s]; !ok {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func scopesSubset(requested, allowed []string) bool {
	set := make(map[string]struct{}, len(allowed))
	for _, s := range allowed {
		set[s] = struct{}{}
	}
	for _, s := range requested {
		if _, ok := set[s]; !ok {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Middleware: oauth2 feature gate
// ---------------------------------------------------------------------------

func (s *server) oauth2Enabled(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.cfg.OAuth2Enabled {
			writeError(w, http.StatusNotImplemented, "oauth2 is not enabled")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// Client CRUD handlers (admin, protected) — delegates to access service
// ---------------------------------------------------------------------------

type registerOAuth2ClientRequest struct {
	Name         string   `json:"name"`
	RedirectURIs []string `json:"redirect_uris"`
	Scopes       []string `json:"scopes"`
}

func (s *server) registerOAuth2Client(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req registerOAuth2ClientRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(req.RedirectURIs) == 0 {
		writeError(w, http.StatusBadRequest, "at least one redirect_uri is required")
		return
	}
	for _, uri := range req.RedirectURIs {
		if strings.TrimSpace(uri) == "" {
			writeError(w, http.StatusBadRequest, "redirect_uri must not be empty")
			return
		}
	}
	scopes := normalizeOAuth2Scopes(req.Scopes)
	if len(scopes) == 0 {
		scopes = []string{"read"}
	}

	clientID, err := oauth2RandomHex(16)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate client id")
		return
	}
	clientID = "oac_" + clientID

	rawSecret, err := oauth2RandomHex(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate client secret")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(rawSecret), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash client secret")
		return
	}

	now := time.Now().UTC()
	client := access.OAuth2Client{
		ID:           clientID,
		TenantID:     user.TenantID,
		Name:         name,
		SecretHash:   string(hashed),
		RedirectURIs: req.RedirectURIs,
		Scopes:       scopes,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.accessSvc.CreateOAuth2Client(client); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create oauth2 client")
		return
	}

	s.appendAuditLog(r, user.TenantID, "oauth2_client_created",
		fmt.Sprintf("client_id=%s,name=%s", clientID, name), "oauth2")

	// Return with plaintext secret (only time it's shown)
	client.ClientSecret = rawSecret
	client.SecretHash = "" // never expose hash in API response
	writeJSON(w, http.StatusCreated, client)
}

func (s *server) listOAuth2Clients(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	items := s.accessSvc.ListOAuth2Clients(user.TenantID)
	// Scrub secret hash from response
	result := make([]access.OAuth2Client, 0, len(items))
	for _, c := range items {
		c.ClientSecret = ""
		c.SecretHash = ""
		result = append(result, c)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": result,
		"total": len(result),
	})
}

func (s *server) getOAuth2Client(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	clientID := chi.URLParam(r, "clientID")

	client, exists := s.accessSvc.GetOAuth2Client(clientID)
	if !exists || client.TenantID != user.TenantID {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}

	client.ClientSecret = ""
	client.SecretHash = ""
	writeJSON(w, http.StatusOK, client)
}

type updateOAuth2ClientRequest struct {
	Name         *string  `json:"name,omitempty"`
	RedirectURIs []string `json:"redirect_uris,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
	Enabled      *bool    `json:"enabled,omitempty"`
}

func (s *server) updateOAuth2Client(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	clientID := chi.URLParam(r, "clientID")

	var req updateOAuth2ClientRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Fetch existing client
	client, exists := s.accessSvc.GetOAuth2Client(clientID)
	if !exists || client.TenantID != user.TenantID {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name must not be empty")
			return
		}
		client.Name = name
	}
	if len(req.RedirectURIs) > 0 {
		client.RedirectURIs = req.RedirectURIs
	}
	if len(req.Scopes) > 0 {
		scopes := normalizeOAuth2Scopes(req.Scopes)
		if len(scopes) > 0 {
			client.Scopes = scopes
		}
	}
	if req.Enabled != nil {
		client.Enabled = *req.Enabled
	}
	client.UpdatedAt = time.Now().UTC()

	updated, err := s.accessSvc.UpdateOAuth2Client(user.TenantID, clientID, client)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update oauth2 client")
		return
	}

	s.appendAuditLog(r, user.TenantID, "oauth2_client_updated",
		fmt.Sprintf("client_id=%s", clientID), "oauth2")

	updated.ClientSecret = ""
	updated.SecretHash = ""
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) deleteOAuth2Client(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	clientID := chi.URLParam(r, "clientID")

	client, exists := s.accessSvc.GetOAuth2Client(clientID)
	if !exists || client.TenantID != user.TenantID {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}

	if err := s.accessSvc.DeleteOAuth2Client(user.TenantID, clientID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete oauth2 client")
		return
	}

	// Also clean up any outstanding codes for this client
	s.oauth2.mu.Lock()
	for code, ac := range s.oauth2.codes {
		if ac.ClientID == clientID {
			delete(s.oauth2.codes, code)
		}
	}
	s.oauth2.mu.Unlock()

	s.appendAuditLog(r, user.TenantID, "oauth2_client_deleted",
		fmt.Sprintf("client_id=%s,name=%s", clientID, client.Name), "oauth2")

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Protocol endpoints (public / semi-public)
// ---------------------------------------------------------------------------

// GET /oauth2/authorize
//
// For MVP this is a JSON endpoint (not an HTML form). The caller must be
// authenticated via a bearer token (the user's session). A frontend consent
// page can wrap this later.
func (s *server) oauth2Authorize(w http.ResponseWriter, r *http.Request) {
	// The user must be authenticated (session bearer token).
	token, err := bearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	user, err := s.authService.VerifyAccessToken(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	q := r.URL.Query()
	responseType := q.Get("response_type")
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	scope := q.Get("scope")
	state := q.Get("state")

	if responseType != "code" {
		writeError(w, http.StatusBadRequest, "unsupported response_type, must be 'code'")
		return
	}
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "client_id is required")
		return
	}

	client, exists := s.accessSvc.GetOAuth2Client(clientID)
	if !exists {
		writeError(w, http.StatusBadRequest, "unknown client_id")
		return
	}
	if !client.Enabled {
		writeError(w, http.StatusBadRequest, "client is disabled")
		return
	}
	// Tenant isolation: the authenticated user must belong to the same tenant as the client.
	if client.TenantID != user.TenantID {
		writeError(w, http.StatusForbidden, "client does not belong to your tenant")
		return
	}

	// Validate redirect_uri
	if redirectURI == "" {
		if len(client.RedirectURIs) == 1 {
			redirectURI = client.RedirectURIs[0]
		} else {
			writeError(w, http.StatusBadRequest, "redirect_uri is required when multiple URIs are registered")
			return
		}
	}
	if !oauth2URIAllowed(redirectURI, client.RedirectURIs) {
		writeError(w, http.StatusBadRequest, "redirect_uri does not match any registered URI")
		return
	}

	// Resolve scopes
	var requestedScopes []string
	if scope != "" {
		requestedScopes = normalizeOAuth2Scopes(strings.Split(scope, " "))
	}
	if len(requestedScopes) == 0 {
		requestedScopes = client.Scopes
	}
	if !scopesSubset(requestedScopes, client.Scopes) {
		writeError(w, http.StatusBadRequest, "requested scopes exceed client's allowed scopes")
		return
	}

	code, err := oauth2RandomHex(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate authorization code")
		return
	}

	now := time.Now().UTC()
	authCode := OAuth2AuthorizationCode{
		Code:        code,
		ClientID:    clientID,
		TenantID:    user.TenantID,
		UserID:      user.ID,
		Scopes:      requestedScopes,
		RedirectURI: redirectURI,
		ExpiresAt:   now.Add(oauth2AuthCodeTTL),
		Used:        false,
	}

	s.oauth2.mu.Lock()
	s.oauth2.codes[code] = authCode
	s.oauth2.mu.Unlock()

	resp := map[string]any{
		"code":         code,
		"redirect_uri": redirectURI,
	}
	if state != "" {
		resp["state"] = state
	}

	writeJSON(w, http.StatusOK, resp)
}

// POST /oauth2/token
//
// Accepts application/x-www-form-urlencoded OR application/json.
func (s *server) oauth2Token(w http.ResponseWriter, r *http.Request) {
	grantType, clientID, clientSecret, code, redirectURI := parseOAuth2TokenParams(r)

	if grantType != "authorization_code" {
		oauth2TokenError(w, "unsupported_grant_type", "only authorization_code is supported")
		return
	}
	if code == "" {
		oauth2TokenError(w, "invalid_request", "code is required")
		return
	}
	if clientID == "" {
		oauth2TokenError(w, "invalid_request", "client_id is required")
		return
	}
	if clientSecret == "" {
		oauth2TokenError(w, "invalid_request", "client_secret is required")
		return
	}

	// Look up authorization code
	s.oauth2.mu.Lock()
	authCode, exists := s.oauth2.codes[code]
	if !exists {
		s.oauth2.mu.Unlock()
		oauth2TokenError(w, "invalid_grant", "unknown or expired authorization code")
		return
	}
	if authCode.Used {
		// Code replay: delete the code and reject
		delete(s.oauth2.codes, code)
		s.oauth2.mu.Unlock()
		oauth2TokenError(w, "invalid_grant", "authorization code already used")
		return
	}
	if time.Now().UTC().After(authCode.ExpiresAt) {
		delete(s.oauth2.codes, code)
		s.oauth2.mu.Unlock()
		oauth2TokenError(w, "invalid_grant", "authorization code has expired")
		return
	}
	if authCode.ClientID != clientID {
		s.oauth2.mu.Unlock()
		oauth2TokenError(w, "invalid_grant", "client_id mismatch")
		return
	}
	if redirectURI != "" && redirectURI != authCode.RedirectURI {
		s.oauth2.mu.Unlock()
		oauth2TokenError(w, "invalid_grant", "redirect_uri mismatch")
		return
	}
	// Mark code as used
	authCode.Used = true
	s.oauth2.codes[code] = authCode
	s.oauth2.mu.Unlock()

	// Validate client secret
	client, clientExists := s.accessSvc.GetOAuth2Client(clientID)
	if !clientExists {
		oauth2TokenError(w, "invalid_client", "unknown client")
		return
	}
	if !client.Enabled {
		oauth2TokenError(w, "invalid_client", "client is disabled")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(client.SecretHash), []byte(clientSecret)); err != nil {
		oauth2TokenError(w, "invalid_client", "invalid client credentials")
		return
	}

	// Issue JWT access token using same signing config as the main auth
	accessToken, expiresIn, err := s.oauth2IssueAccessToken(authCode)
	if err != nil {
		oauth2TokenError(w, "server_error", "failed to issue access token")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   expiresIn,
		"scope":        strings.Join(authCode.Scopes, " "),
	})
}

// POST /oauth2/revoke -- token revocation (RFC 7009)
func (s *server) oauth2Revoke(w http.ResponseWriter, r *http.Request) {
	tokenValue := ""

	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		var body struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			tokenValue = body.Token
		}
	} else {
		_ = r.ParseForm()
		tokenValue = r.FormValue("token")
	}

	if tokenValue == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	// Try to revoke via auth service (best-effort -- if it's a valid JWT the
	// auth service can blacklist it).
	_ = s.authService.Logout(tokenValue)

	// Always return 200 per RFC 7009 regardless of whether revocation succeeded
	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// JWT issuing for OAuth2
// ---------------------------------------------------------------------------

type oauth2AccessTokenClaims struct {
	UserID   string   `json:"user_id"`
	TenantID string   `json:"tenant_id,omitempty"`
	ClientID string   `json:"client_id"`
	Scopes   []string `json:"scopes"`
	jwt.RegisteredClaims
}

func (s *server) oauth2IssueAccessToken(authCode OAuth2AuthorizationCode) (string, int, error) {
	secret := strings.TrimSpace(s.cfg.JWTSecret)
	if secret == "" {
		secret = "dev-secret"
	}
	issuer := strings.TrimSpace(s.cfg.JWTIssuer)
	if issuer == "" {
		issuer = "mistypass-api"
	}
	ttl := s.cfg.JWTAccessTTL
	if ttl <= 0 {
		ttl = time.Hour
	}

	jti, err := oauth2RandomHex(12)
	if err != nil {
		return "", 0, err
	}

	now := time.Now().UTC()
	claims := oauth2AccessTokenClaims{
		UserID:   authCode.UserID,
		TenantID: authCode.TenantID,
		ClientID: authCode.ClientID,
		Scopes:   authCode.Scopes,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   authCode.UserID,
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", 0, err
	}

	return signed, int(ttl.Seconds()), nil
}

// ---------------------------------------------------------------------------
// Token param parsing helpers
// ---------------------------------------------------------------------------

func parseOAuth2TokenParams(r *http.Request) (grantType, clientID, clientSecret, code, redirectURI string) {
	ct := r.Header.Get("Content-Type")

	if strings.Contains(ct, "application/json") {
		var body struct {
			GrantType    string `json:"grant_type"`
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
			Code         string `json:"code"`
			RedirectURI  string `json:"redirect_uri"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			return body.GrantType, body.ClientID, body.ClientSecret, body.Code, body.RedirectURI
		}
	}

	_ = r.ParseForm()
	return r.FormValue("grant_type"),
		r.FormValue("client_id"),
		r.FormValue("client_secret"),
		r.FormValue("code"),
		r.FormValue("redirect_uri")
}

func oauth2TokenError(w http.ResponseWriter, errorCode, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             errorCode,
		"error_description": description,
	})
}

func oauth2URIAllowed(uri string, allowed []string) bool {
	for _, a := range allowed {
		if uri == a {
			return true
		}
	}
	return false
}
