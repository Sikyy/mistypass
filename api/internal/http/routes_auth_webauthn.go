package httpx

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/auth"
)

// --- Registration (authenticated) ---

func (s *server) webAuthnRegisterBegin(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	if !s.isWebAuthnEnabledForUser(user) {
		writeError(w, http.StatusForbidden, "webauthn is not enabled for this organization")
		return
	}

	options, err := s.webAuthnEngine.BeginRegistration(user)
	if err != nil {
		if errors.Is(err, auth.ErrWebAuthnNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "webauthn registration failed")
		}
		return
	}

	s.appendAuditLog(r, user.TenantID, "webauthn_register_begin", fmt.Sprintf("user_id=%s,email=%s", user.ID, user.Email), "auth")
	writeJSON(w, http.StatusOK, options)
}

func (s *server) webAuthnRegisterFinish(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	if !s.isWebAuthnEnabledForUser(user) {
		writeError(w, http.StatusForbidden, "webauthn is not enabled for this organization")
		return
	}

	displayName := r.URL.Query().Get("display_name")

	cred, err := s.webAuthnEngine.FinishRegistration(user, displayName, r)
	if err != nil {
		if errors.Is(err, auth.ErrWebAuthnNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, err.Error())
		} else if errors.Is(err, auth.ErrWebAuthnSessionExpired) {
			writeError(w, http.StatusBadRequest, "registration challenge expired")
		} else {
			writeError(w, http.StatusBadRequest, "registration verification failed")
		}
		return
	}

	s.appendAuditLog(r, user.TenantID, "webauthn_credential_registered",
		fmt.Sprintf("user_id=%s,email=%s,credential_id=%s,display_name=%s", user.ID, user.Email, cred.ID, cred.DisplayName), "auth")
	writeJSON(w, http.StatusCreated, cred)
}

// --- Login (unauthenticated) ---

func (s *server) webAuthnLoginBegin(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := s.authService.FindUserByEmail(request.Email)
	if err != nil {
		// Don't reveal whether the user exists.
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !s.isWebAuthnEnabledForUser(user) {
		writeError(w, http.StatusForbidden, "webauthn is not enabled for this organization")
		return
	}

	options, err := s.webAuthnEngine.BeginLogin(user)
	if err != nil {
		if errors.Is(err, auth.ErrWebAuthnNoCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
		} else if errors.Is(err, auth.ErrWebAuthnNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, options)
}

func (s *server) webAuthnLoginFinish(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")

	// Detect Path A vs Path B by checking for MFA completion body.
	// Path A sends webauthn_token + mfa_code in POST body (no WebAuthn credential).
	// Path B sends WebAuthn credential in POST body with email in query param.

	// --- Path A: MFA completion with a previously issued webauthn_token ---
	// Try to detect path A by peeking at the body for webauthn_token.
	if email == "" {
		var mfaBody struct {
			WebAuthnToken string `json:"webauthn_token"`
			MFACode       string `json:"mfa_code"`
		}
		if err := decodeJSON(r, &mfaBody); err != nil || mfaBody.WebAuthnToken == "" {
			writeError(w, http.StatusBadRequest, "email query parameter or webauthn_token body field is required")
			return
		}

		verifiedEmail, err := s.webAuthnEngine.ConsumeVerifiedSession(mfaBody.WebAuthnToken)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "webauthn session expired, please retry")
			return
		}
		response, err := s.authService.LoginByWebAuthn(verifiedEmail, mfaBody.MFACode)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrAdminMFARequired), errors.Is(err, auth.ErrInvalidMFACode):
				writeError(w, http.StatusUnauthorized, err.Error())
			default:
				writeError(w, http.StatusUnauthorized, "invalid credentials")
			}
			return
		}
		user, _ := s.authService.FindUserByEmail(verifiedEmail)
		s.appendAuditLog(r, user.TenantID, "webauthn_login",
			fmt.Sprintf("user_id=%s,email=%s", user.ID, user.Email), "auth")
		writeJSON(w, http.StatusOK, response)
		return
	}

	// --- Path B: Initial WebAuthn verification ---
	user, err := s.authService.FindUserByEmail(email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !s.isWebAuthnEnabledForUser(user) {
		writeError(w, http.StatusForbidden, "webauthn is not enabled for this organization")
		return
	}

	if err := s.webAuthnEngine.FinishLogin(user, r); err != nil {
		if errors.Is(err, auth.ErrWebAuthnSessionExpired) {
			writeError(w, http.StatusUnauthorized, "challenge expired, please retry")
		} else {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
		}
		return
	}

	// WebAuthn verified — try issuing JWT (MFA check happens inside).
	response, err := s.authService.LoginByWebAuthn(user.Email, "")
	if err != nil {
		if errors.Is(err, auth.ErrAdminMFARequired) {
			// MFA required — issue a short-lived verified token so the frontend
			// can complete MFA without repeating the WebAuthn ceremony.
			token, tokenErr := s.webAuthnEngine.CreateVerifiedSession(user.Email)
			if tokenErr != nil {
				writeError(w, http.StatusInternalServerError, "failed to create verified session")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"mfa_required":   true,
				"webauthn_token": token,
			})
			return
		}
		switch {
		case errors.Is(err, auth.ErrAdminMFAEnrollmentRequired):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, auth.ErrInvalidMFACode):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			writeError(w, http.StatusUnauthorized, "invalid credentials")
		}
		return
	}

	s.appendAuditLog(r, user.TenantID, "webauthn_login",
		fmt.Sprintf("user_id=%s,email=%s", user.ID, user.Email), "auth")
	writeJSON(w, http.StatusOK, response)
}

// --- Credential management (authenticated) ---

func (s *server) webAuthnListCredentials(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	creds := s.webAuthnEngine.ListCredentials(user.ID)
	if creds == nil {
		creds = []auth.WebAuthnCredential{}
	}
	writeJSON(w, http.StatusOK, creds)
}

func (s *server) webAuthnDeleteCredential(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	credentialID := chi.URLParam(r, "credentialID")
	if credentialID == "" {
		writeError(w, http.StatusBadRequest, "credential_id is required")
		return
	}

	if err := s.webAuthnEngine.DeleteCredential(user.ID, credentialID); err != nil {
		if errors.Is(err, auth.ErrWebAuthnCredentialNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else if errors.Is(err, auth.ErrWebAuthnNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "failed to delete credential")
		}
		return
	}

	s.appendAuditLog(r, user.TenantID, "webauthn_credential_deleted",
		fmt.Sprintf("user_id=%s,email=%s,credential_id=%s", user.ID, user.Email, credentialID), "auth")
	w.WriteHeader(http.StatusNoContent)
}

// isWebAuthnEnabledForUser checks the organization-level WebAuthn toggle
// and verifies that the organization has SSO (Enterprise IdP) configured.
func (s *server) isWebAuthnEnabledForUser(user auth.User) bool {
	if user.TenantID == "" {
		return false
	}
	settings := s.accessSvc.GetOrganizationSettings(user.TenantID)
	if !settings.WebAuthnEnabled {
		return false
	}
	// WebAuthn requires SSO — reject if no Enterprise IdP is configured.
	_, err := s.enterpriseSvc.GetIDPConfig(user.TenantID)
	return err == nil
}
