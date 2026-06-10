package httpx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/pdfgen"
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

// exportBadges handles GET /api/v1/badges/export?user_id=|place_id=|group_id=&format=pdf|html
func (s *server) exportBadges(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	placeID := strings.TrimSpace(r.URL.Query().Get("place_id"))
	groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
	selectors := 0
	for _, v := range []string{userID, placeID, groupID} {
		if v != "" {
			selectors++
		}
	}
	if selectors != 1 {
		writeError(w, http.StatusBadRequest, "exactly one of user_id, place_id, group_id is required")
		return
	}

	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "pdf"
	}
	if format != "pdf" && format != "html" {
		writeError(w, http.StatusBadRequest, "format must be pdf or html")
		return
	}

	var members []access.AccessUser
	var scopeLabel string
	switch {
	case userID != "":
		user, err := s.accessSvc.GetUser(tenantID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		members = []access.AccessUser{user}
		scopeLabel = "user"
	case placeID != "":
		if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
			writeError(w, http.StatusNotFound, "place not found")
			return
		}
		for _, u := range s.accessSvc.ListUsers(tenantID) {
			if u.BuildingID == placeID {
				members = append(members, u)
			}
		}
		scopeLabel = "place"
	default:
		for _, u := range s.accessSvc.ListUsers(tenantID) {
			for _, g := range u.GroupIDs {
				if g == groupID {
					members = append(members, u)
					break
				}
			}
		}
		scopeLabel = "group"
	}

	if len(members) == 0 {
		writeError(w, http.StatusBadRequest, "no users match the requested scope")
		return
	}

	doc := pdfgen.BadgeDoc{Organization: s.badgeOrganizationName(tenantID)}
	for _, u := range members {
		qrBase64, err := pdfgen.EncodeQRPNGBase64(s.badgeVerifyURL(tenantID, s.signBadgeToken(tenantID, u.ID)))
		if err != nil {
			s.logger.Error("badge qr encode failed", "error", err, "user_id", u.ID)
			writeError(w, http.StatusInternalServerError, "failed to render badge QR")
			return
		}
		doc.Badges = append(doc.Badges, pdfgen.Badge{
			Name:     firstNonEmptyString(u.Name, u.Email, u.ID),
			Role:     u.Role,
			Building: u.BuildingID,
			Status:   u.Status,
			QRBase64: qrBase64,
		})
	}

	if format == "html" {
		html, err := s.pdfRenderer.RenderBadgesHTML(doc)
		if err != nil {
			s.logger.Error("badge html render failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to render badges")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(html)
		return
	}

	pdfBytes, err := s.pdfRenderer.RenderBadgesPDF(s.gotenbergClient, doc)
	if err != nil {
		s.logger.Error("badge pdf render failed", "error", err)
		writeError(w, http.StatusBadGateway, "PDF rendering failed: "+err.Error())
		return
	}
	filename := fmt.Sprintf("badges_%s_%s.pdf", scopeLabel, time.Now().UTC().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
	w.Write(pdfBytes)
}

// badgeVerifyURL builds the QR target URL. Base resolution: configured base URL,
// else the tenant's primary domain, else a relative path.
func (s *server) badgeVerifyURL(tenantID, token string) string {
	base := strings.TrimRight(strings.TrimSpace(s.cfg.BadgeVerifyBaseURL), "/")
	if base == "" {
		if domain := strings.TrimSpace(s.accessSvc.GetOrganizationSettings(tenantID).PrimaryDomain); domain != "" {
			base = "https://" + domain
		}
	}
	return base + "/api/v1/badges/verify?token=" + token
}
