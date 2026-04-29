package httpx

import (
	"errors"
	"fmt"
	"net/http"

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

	status, err := s.authService.EnableUserMFA(user.ID, request.Code)
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
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	s.appendAuditLog(r, user.TenantID, "user_mfa_enabled", fmt.Sprintf("user_id=%s,email=%s", user.ID, user.Email), "auth")
	writeJSON(w, http.StatusOK, status)
}

func (s *server) disableUserMFA(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	status, err := s.authService.DisableUserMFA(user.ID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	s.appendAuditLog(r, user.TenantID, "user_mfa_disabled", fmt.Sprintf("user_id=%s,email=%s", user.ID, user.Email), "auth")
	writeJSON(w, http.StatusOK, status)
}
