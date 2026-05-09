package httpx

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/me/change-password
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appChangePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "current_password and new_password are required")
		return
	}

	if err := s.authService.ChangePassword(user.ID, req.CurrentPassword, req.NewPassword); err != nil {
		if err.Error() == "invalid credentials" {
			writeError(w, http.StatusForbidden, "current password is incorrect")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "password_changed"})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/me/logins — List active sessions for the current user
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appListMyLogins(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	sessions := s.authService.ListActiveSessions(user.ID)
	currentToken := extractBearerToken(r)

	items := make([]map[string]any, 0, len(sessions))
	for _, sess := range sessions {
		items = append(items, map[string]any{
			"id":          sess.SessionID,
			"device_name": parseDeviceName(sess.UserAgent),
			"platform":    parsePlatform(sess.UserAgent),
			"last_active": sess.CreatedAt.Format(time.RFC3339),
			"is_current":  isCurrentSession(r, currentToken, sess.SessionID),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ─────────────────────────────────────────────────────────────────────────────
// DELETE /api/v1/app/me/logins/{loginId} — Revoke a specific session
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appRevokeMyLogin(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	loginID := chi.URLParam(r, "loginId")
	if strings.TrimSpace(loginID) == "" {
		writeError(w, http.StatusBadRequest, "login id is required")
		return
	}

	if err := s.authService.RevokeSession(user.ID, loginID); err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "revoked"})
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/me/avatar — Upload avatar image (multipart/form-data)
// ─────────────────────────────────────────────────────────────────────────────

const maxAvatarSize = 5 << 20 // 5 MB

func (s *server) appUploadAvatar(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	if err := r.ParseMultipartForm(maxAvatarSize); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form or file too large")
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		writeError(w, http.StatusBadRequest, "avatar file is required")
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}
	if !strings.HasPrefix(ct, "image/") {
		writeError(w, http.StatusBadRequest, "file must be an image")
		return
	}

	data, err := io.ReadAll(io.LimitReader(file, maxAvatarSize))
	if err != nil {
		writeInternalError(w, r, err)
		return
	}

	// Store avatar reference in state store keyed by user ID
	avatarKey := "avatar:" + user.ID
	avatarRecord := map[string]any{
		"user_id":      user.ID,
		"content_type": ct,
		"size":         len(data),
		"updated_at":   time.Now().UTC().Format(time.RFC3339),
	}
	if s.stateStore != nil {
		if err := s.stateStore.Save(avatarKey, avatarRecord); err != nil {
			writeInternalError(w, r, err)
			return
		}
	} else {
		s.memStoreMu.Lock()
		s.memStore[avatarKey] = avatarRecord
		s.memStoreMu.Unlock()
	}

	writeJSON(w, http.StatusOK, s.buildAppMePayload(
		user.ID, user.Name, user.Email,
		user.Role, user.TenantID, user.Language,
		user.BuildingIDs,
	))
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/me/primary-device — Mark current device as primary
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appSetPrimaryDevice(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	// Find device token from push devices for this user
	var deviceID string
	s.pushDeviceMu.Lock()
	for _, pd := range s.pushDevices {
		if pd.UserID == user.ID {
			deviceID = pd.DeviceID
			break
		}
	}
	s.pushDeviceMu.Unlock()

	if deviceID == "" {
		deviceID = "current"
	}

	primaryKey := "primary_device:" + user.ID
	primaryData := map[string]string{
		"user_id":   user.ID,
		"device_id": deviceID,
	}
	if s.stateStore != nil {
		_ = s.stateStore.Save(primaryKey, primaryData)
	} else {
		s.memStoreMu.Lock()
		s.memStore[primaryKey] = primaryData
		s.memStoreMu.Unlock()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "primary_device_set",
		"device_id": deviceID,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/devices/apns — Register APNS device token (iOS)
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appRegisterAPNSDevice(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		DeviceToken string `json:"device_token"`
		Platform    string `json:"platform"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.DeviceToken == "" {
		writeError(w, http.StatusBadRequest, "device_token is required")
		return
	}
	if req.Platform == "" {
		req.Platform = "ios"
	}

	s.pushDeviceMu.Lock()
	s.pushDevices[req.DeviceToken] = pushDevice{
		UserID:       user.ID,
		TenantID:     user.TenantID,
		FCMToken:     req.DeviceToken,
		Platform:     req.Platform,
		RegisteredAt: time.Now(),
	}
	// Evict stale device registrations (older than 90 days) to prevent unbounded growth.
	if len(s.pushDevices) > 1000 {
		cutoff := time.Now().Add(-90 * 24 * time.Hour)
		for k, v := range s.pushDevices {
			if v.RegisteredAt.Before(cutoff) {
				delete(s.pushDevices, k)
			}
		}
	}
	s.pushDeviceMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"status": "registered"})
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return auth[7:]
	}
	return ""
}

func parseDeviceName(ua string) string {
	if ua == "" {
		return "Unknown Device"
	}
	ua = strings.ToLower(ua)
	if strings.Contains(ua, "iphone") {
		return "iPhone"
	}
	if strings.Contains(ua, "ipad") {
		return "iPad"
	}
	if strings.Contains(ua, "android") {
		return "Android"
	}
	if strings.Contains(ua, "mac") {
		return "Mac"
	}
	if strings.Contains(ua, "windows") {
		return "Windows"
	}
	return "Unknown Device"
}

func parsePlatform(ua string) string {
	if ua == "" {
		return "unknown"
	}
	ua = strings.ToLower(ua)
	if strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") || strings.Contains(ua, "ios") {
		return "ios"
	}
	if strings.Contains(ua, "android") {
		return "android"
	}
	return "web"
}

func isCurrentSession(_ *http.Request, currentToken, sessionID string) bool {
	// Heuristic: we can't reliably match access token to session, so return false
	// The iOS app uses this for display only
	_ = currentToken
	_ = sessionID
	return false
}
