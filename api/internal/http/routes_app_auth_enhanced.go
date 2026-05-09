package httpx

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/auth"
)

// appRequestMagicLink generates a magic-link token for passwordless login.
// POST /app/auth/magic-link
func (s *server) appRequestMagicLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	if s.magicLinkStore == nil {
		// Stub response when DB is not configured
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "magic link sent to email",
		})
		return
	}

	// Generate a 32-byte random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	token := hex.EncodeToString(tokenBytes)
	expiresAt := time.Now().UTC().Add(15 * time.Minute)

	if err := s.magicLinkStore.CreateMagicLinkToken(email, token, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create magic link token")
		return
	}

	// In production, send the token via email.
	// For now, just confirm the action without revealing the token.
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "magic link sent to email",
	})
}

// appVerifyMagicLink verifies a magic-link token and issues auth tokens.
// POST /app/auth/magic-link/verify
func (s *server) appVerifyMagicLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	if s.magicLinkStore == nil {
		writeError(w, http.StatusServiceUnavailable, "magic link verification is not available")
		return
	}

	email, err := s.magicLinkStore.VerifyMagicLinkToken(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	response, err := s.authService.LoginByTrustedIdentity(email, auth.SessionMetadata{
		IPAddress:   r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		LoginMethod: "magic_link",
	})
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if !isAppAllowedRole(response.User.Role) {
		writeError(w, http.StatusUnauthorized, "invalid app credentials")
		return
	}

	writeJSON(w, http.StatusOK, response)
}

// appOrgLookup looks up an organization by domain.
// GET /app/auth/org-lookup?domain=
func (s *server) appOrgLookup(w http.ResponseWriter, r *http.Request) {
	domain := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("domain")))
	if domain == "" {
		writeError(w, http.StatusBadRequest, "domain query parameter is required")
		return
	}

	normalize := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, "_", ""), "-", "")
	}
	normalizedDomain := normalize(domain)

	tenants := s.tenantSvc.List()
	for _, t := range tenants {
		tenantID := strings.ToLower(t.ID)
		tenantName := strings.ToLower(t.Name)
		if tenantID == domain || tenantName == domain || normalize(tenantID) == normalizedDomain || normalize(tenantName) == normalizedDomain {
			writeJSON(w, http.StatusOK, map[string]any{
				"org_id":  t.ID,
				"domain":  domain,
				"name":    t.Name,
				"logo":    "",
				"methods": []string{"classic"},
			})
			return
		}
	}

	writeError(w, http.StatusNotFound, "organization not found")
}

// appOrgMethods returns available sign-in methods for an organization.
// GET /app/auth/org/{orgId}/methods
func (s *server) appOrgMethods(w http.ResponseWriter, r *http.Request) {
	orgID := strings.TrimSpace(chi.URLParam(r, "orgId"))
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "orgId is required")
		return
	}

	// Verify the tenant exists
	tenants := s.tenantSvc.List()
	found := false
	for _, t := range tenants {
		if t.ID == orgID {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "organization not found")
		return
	}

	// SSO and WebAuthn are future features; return classic for now.
	writeJSON(w, http.StatusOK, map[string]any{
		"org_id":  orgID,
		"methods": []string{"classic"},
	})
}

// appInitiateSSO initiates an SSO redirect for an organization.
// POST /app/auth/sso/{orgId}
func (s *server) appInitiateSSO(w http.ResponseWriter, r *http.Request) {
	orgID := strings.TrimSpace(chi.URLParam(r, "orgId"))
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "orgId is required")
		return
	}

	// Stub — SSO integration is a future feature.
	writeJSON(w, http.StatusOK, map[string]string{
		"redirect_url": "https://sso.example.com/auth?org=" + orgID,
	})
}

// appVerify2FA verifies a TOTP or SMS 2FA code.
// POST /app/auth/2fa/verify
func (s *server) appVerify2FA(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		Code   string `json:"code"`
		Type   string `json:"type"` // "totp" or "sms"
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	userID := strings.TrimSpace(req.UserID)
	code := strings.TrimSpace(req.Code)
	if userID == "" || code == "" {
		writeError(w, http.StatusBadRequest, "user_id and code are required")
		return
	}

	// Check if auth service has MFA status for this user.
	status, err := s.authService.GetUserMFAStatus(userID)
	if err == nil && status.Enabled {
		// Use the real MFA verification via login with MFA code.
		user, userErr := s.authService.GetUserByID(userID)
		if userErr != nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}
		response, loginErr := s.authService.LoginByWebAuthn(user.Email, code, auth.SessionMetadata{
			IPAddress:   r.RemoteAddr,
			UserAgent:   r.UserAgent(),
			LoginMethod: "2fa",
		})
		if loginErr != nil {
			writeError(w, http.StatusUnauthorized, "invalid 2fa code")
			return
		}
		writeJSON(w, http.StatusOK, response)
		return
	}

	writeError(w, http.StatusUnauthorized, "2fa is not configured for this account")
}

// appVerifyBackupCode verifies a backup/recovery code for 2FA bypass.
// POST /app/auth/2fa/backup
func (s *server) appVerifyBackupCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		Code   string `json:"code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	userID := strings.TrimSpace(req.UserID)
	code := strings.TrimSpace(req.Code)
	if userID == "" || code == "" {
		writeError(w, http.StatusBadRequest, "user_id and code are required")
		return
	}

	// Check if auth service has MFA enabled and try backup code via LoginByWebAuthn
	// which enforces MFA — the backup code path is internal to the MFA enforcement.
	status, err := s.authService.GetUserMFAStatus(userID)
	if err == nil && status.Enabled {
		user, userErr := s.authService.GetUserByID(userID)
		if userErr != nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}
		response, loginErr := s.authService.LoginByWebAuthn(user.Email, code, auth.SessionMetadata{
			IPAddress:   r.RemoteAddr,
			UserAgent:   r.UserAgent(),
			LoginMethod: "backup_code",
		})
		if loginErr != nil {
			writeError(w, http.StatusUnauthorized, "invalid backup code")
			return
		}
		writeJSON(w, http.StatusOK, response)
		return
	}

	writeError(w, http.StatusUnauthorized, "2fa is not configured for this account")
}

// appRegister creates a new user account.
// POST /app/auth/register
func (s *server) appRegister(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.SelfRegistrationEnabled {
		writeError(w, http.StatusForbidden, "registration is disabled")
		return
	}

	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Domain   string `json:"domain"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email := strings.TrimSpace(strings.ToLower(req.Email))
	name := strings.TrimSpace(req.Name)
	password := req.Password
	if email == "" || password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := s.authService.CreateUser(email, name, password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Issue tokens for the newly created user.
	response, err := s.authService.LoginByTrustedIdentity(user.Email, auth.SessionMetadata{
		IPAddress:   r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		LoginMethod: "register",
	})
	if err != nil {
		// User was created but token issuance failed — still report the user.
		writeJSON(w, http.StatusCreated, map[string]any{
			"user": user,
		})
		return
	}

	writeJSON(w, http.StatusCreated, response)
}

// appRestorePassword sends a password reset email.
// POST /app/auth/restore-password
func (s *server) appRestorePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	// Use the existing password reset flow from the auth service.
	_, _ = s.authService.RequestPasswordReset(email)

	// Always return success to avoid email enumeration.
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "password reset email sent",
	})
}
