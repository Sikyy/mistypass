package httpx

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mistypass/cloud/api/internal/modules/auth"
)

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var request auth.LoginRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	response, err := s.authService.Login(request, auth.SessionMetadata{
		IPAddress:   r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		LoginMethod: "password",
	})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrPasswordAuthDisabled):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, auth.ErrAdminMFAEnrollmentRequired):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, auth.ErrAdminMFARequired), errors.Is(err, auth.ErrInvalidMFACode):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			writeError(w, http.StatusUnauthorized, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *server) refresh(w http.ResponseWriter, r *http.Request) {
	var request auth.RefreshRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	response, err := s.authService.Refresh(request)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	token, err := bearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "missing bearer token")
		return
	}

	if err := s.authService.Logout(token); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *server) appLogin(w http.ResponseWriter, r *http.Request) {
	var request auth.LoginRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	response, err := s.authService.Login(request)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if strings.ToLower(strings.TrimSpace(response.User.Role)) != "resident" {
		writeError(w, http.StatusUnauthorized, "invalid app credentials")
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *server) appRefresh(w http.ResponseWriter, r *http.Request) {
	var request auth.RefreshRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	response, err := s.authService.Refresh(request)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if strings.ToLower(strings.TrimSpace(response.User.Role)) != "resident" {
		writeError(w, http.StatusUnauthorized, "invalid app credentials")
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

type currentUserProfileUpdateRequest struct {
	Name     *string `json:"name,omitempty"`
	Language *string `json:"language,omitempty"`
}

func (s *server) getCurrentUserProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (s *server) updateCurrentUserProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var request currentUserProfileUpdateRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	nextName := user.Name
	if request.Name != nil {
		nextName = *request.Name
	}
	nextLanguage := user.Language
	if request.Language != nil {
		language, valid := normalizeCurrentUserProfileLanguage(*request.Language)
		if !valid {
			writeError(w, http.StatusBadRequest, "unsupported language")
			return
		}
		nextLanguage = language
	}

	updated, err := s.authService.UpdateUserProfile(user.ID, nextName, nextLanguage)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	changed := make([]string, 0, 2)
	if request.Name != nil {
		changed = append(changed, "name")
	}
	if request.Language != nil {
		changed = append(changed, "language")
	}
	s.appendAuditLog(
		r,
		updated.TenantID,
		"auth_user_profile_updated",
		"user_id="+updated.ID+",changed="+strings.Join(changed, "|"),
		"auth",
	)

	writeJSON(w, http.StatusOK, updated)
}

func normalizeCurrentUserProfileLanguage(language string) (string, bool) {
	nextLanguage := strings.TrimSpace(language)
	if nextLanguage == "" {
		return "", true
	}
	switch nextLanguage {
	case "en-US", "id-ID", "zh-CN":
		return nextLanguage, true
	default:
		return "", false
	}
}
