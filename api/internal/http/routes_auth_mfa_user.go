package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/mistypass/cloud/api/internal/modules/auth"
)

func (s *server) getUserMFAStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	status, err := s.authService.GetUserMFAStatus(user.ID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *server) setupUserMFA(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var request struct {
		Issuer string `json:"issuer"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	enrollment, err := s.authService.StartUserMFAEnrollment(user.ID, request.Issuer)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	s.appendAuditLog(r, user.TenantID, "user_mfa_setup_started", fmt.Sprintf("user_id=%s,email=%s", user.ID, user.Email), "auth")
	writeJSON(w, http.StatusOK, enrollment)
}

func (s *server) enableUserMFA(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	var request struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	status, recoveryCodes, err := s.authService.EnableUserMFA(user.ID, request.Code)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrAdminMFARequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, auth.ErrInvalidMFACode):
			writeError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, auth.ErrAdminMFANotConfigured):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	s.appendAuditLog(r, user.TenantID, "user_mfa_enabled", fmt.Sprintf("user_id=%s,email=%s", user.ID, user.Email), "auth")
	response := map[string]any{"status": status}
	if recoveryCodes != nil {
		response["recovery_codes"] = recoveryCodes
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) disableUserMFA(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	// Require current TOTP code or password to disable MFA (re-authentication)
	var req struct {
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Code) == "" && strings.TrimSpace(req.Password) == "" {
		writeError(w, http.StatusBadRequest, "current MFA code or password is required to disable MFA")
		return
	}
	// Verify re-authentication: a current TOTP/recovery code or the account password.
	if code := strings.TrimSpace(req.Code); code != "" {
		if err := s.authService.VerifyUserMFACode(user.ID, code); err != nil {
			if errors.Is(err, auth.ErrUserNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
			} else {
				writeError(w, http.StatusUnauthorized, "invalid MFA code")
			}
			return
		}
	} else {
		if err := s.authService.VerifyUserPassword(user.ID, strings.TrimSpace(req.Password)); err != nil {
			if errors.Is(err, auth.ErrUserNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
			} else {
				writeError(w, http.StatusUnauthorized, "invalid password")
			}
			return
		}
	}

	status, err := s.authService.DisableUserMFA(user.ID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	s.appendAuditLog(r, user.TenantID, "user_mfa_disabled", fmt.Sprintf("user_id=%s,email=%s", user.ID, user.Email), "auth")
	writeJSON(w, http.StatusOK, status)
}
