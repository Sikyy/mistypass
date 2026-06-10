package httpx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

const badgeTokenPrefix = "MPB1"

func (s *server) badgeSigningKey() []byte {
	return []byte(s.cfg.JWTSecret)
}

// signBadgeToken returns base64url(payload) + "." + base64url(hmac[:16]).
func (s *server) signBadgeToken(tenantID, userID string) string {
	payload := strings.Join([]string{badgeTokenPrefix, tenantID, userID, time.Now().UTC().Format("20060102")}, ".")
	mac := hmac.New(sha256.New, s.badgeSigningKey())
	mac.Write([]byte(payload))
	sig := mac.Sum(nil)[:16]
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func (s *server) parseBadgeToken(token string) (tenantID, userID string, ok bool) {
	idx := strings.LastIndex(token, ".")
	if idx <= 0 || idx == len(token)-1 {
		return "", "", false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(token[:idx])
	if err != nil {
		return "", "", false
	}
	gotSig, err := base64.RawURLEncoding.DecodeString(token[idx+1:])
	if err != nil {
		return "", "", false
	}
	mac := hmac.New(sha256.New, s.badgeSigningKey())
	mac.Write(payloadBytes)
	if !hmac.Equal(gotSig, mac.Sum(nil)[:16]) {
		return "", "", false
	}
	parts := strings.Split(string(payloadBytes), ".")
	if len(parts) != 4 || parts[0] != badgeTokenPrefix {
		return "", "", false
	}
	return parts[1], parts[2], true
}

// verifyBadge handles GET /api/v1/badges/verify?token= — public, rate-limited.
// It is the QR target: a phone camera resolves the URL and shows the holder's
// identity and current employment status.
func (s *server) verifyBadge(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}
	tenantID, userID, ok := s.parseBadgeToken(token)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false})
		return
	}
	user, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"valid":        true,
		"name":         user.Name,
		"role":         user.Role,
		"organization": s.badgeOrganizationName(tenantID),
		"status":       user.Status,
	})
}

func (s *server) badgeOrganizationName(tenantID string) string {
	if name := strings.TrimSpace(s.accessSvc.GetOrganizationSettings(tenantID).Name); name != "" {
		return name
	}
	return s.resolveExportTenantName(tenantID)
}
