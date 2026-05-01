package httpx

import (
	"errors"
	"net/http"

	"github.com/mistypass/cloud/api/internal/modules/auth"
)

func (s *server) requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	token, err := s.authService.RequestPasswordReset(request.Email)
	if err != nil {
		// Don't reveal internal errors for password reset
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
		})
		return
	}

	// In production, dispatch the token via email.
	// For now, include it in the response in dev mode for testing.
	if token != "" && s.cfg.AppEnv == "development" {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":      "ok",
			"reset_token": token,
		})
		return
	}

	// Always return success to avoid email enumeration.
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *server) confirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if request.Token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}
	if request.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "new_password is required")
		return
	}

	if err := s.authService.ConfirmPasswordReset(request.Token, request.NewPassword); err != nil {
		switch {
		case errors.Is(err, auth.ErrPasswordResetTokenInvalid),
			errors.Is(err, auth.ErrPasswordResetTokenExpired):
			writeError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, auth.ErrPasswordTooWeak):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusUnauthorized, "invalid reset token")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	s.appendAuditLog(r, "", "password_reset_confirmed", "", "auth")
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
