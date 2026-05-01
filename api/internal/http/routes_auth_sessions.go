package httpx

import (
	"fmt"
	"net/http"

	"github.com/mistypass/cloud/api/internal/modules/auth"
)

func (s *server) listLoginSessions(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	sessions := s.authService.ListActiveSessions(user.ID)
	if sessions == nil {
		sessions = []auth.LoginSession{}
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *server) revokeLoginSession(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var request struct {
		SessionID string `json:"session_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.SessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	if err := s.authService.RevokeSession(user.ID, request.SessionID); err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	s.appendAuditLog(r, user.TenantID, "login_session_revoked",
		fmt.Sprintf("user_id=%s,session_id=%s", user.ID, request.SessionID), "auth")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) revokeAllLoginSessions(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	revoked := s.authService.RevokeRefreshTokensByUserEmail(user.Email)

	s.appendAuditLog(r, user.TenantID, "login_sessions_revoked_all",
		fmt.Sprintf("user_id=%s,revoked=%d", user.ID, revoked), "auth")
	writeJSON(w, http.StatusOK, map[string]any{
		"revoked": revoked,
	})
}
