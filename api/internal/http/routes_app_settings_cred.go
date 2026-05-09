package httpx

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/credentials/nfc — Bind NFC card
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appBindNFCCard(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		CardUID  string `json:"card_uid"`
		CardType string `json:"card_type"`
		Label    string `json:"label"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.CardUID) == "" {
		writeError(w, http.StatusBadRequest, "card_uid is required")
		return
	}
	if req.CardType == "" {
		req.CardType = "desfire_ev3"
	}
	if req.Label == "" {
		req.Label = "NFC Card"
	}

	normalizedUID := normalizeNFCUID(req.CardUID)
	for _, p := range s.walletSvc.ListPasses(user.TenantID) {
		if strings.EqualFold(p.UID, normalizedUID) && p.Status == "active" {
			writeError(w, http.StatusConflict, "card already bound to another user")
			return
		}
	}

	expiresAt := time.Now().UTC().AddDate(2, 0, 0).Format(time.RFC3339)

	pass, err := s.walletSvc.RegisterNFCCard(
		user.TenantID,
		req.CardUID,
		req.CardType,
		user.ID,
		req.Label,
		expiresAt,
		user.Email,
	)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":                   pass.ID,
		"device_name":         req.Label,
		"public_key_fingerprint": pass.UID,
		"card_uid":            pass.UID,
		"card_type":           req.CardType,
		"user_id":             user.ID,
		"tenant_id":           user.TenantID,
		"is_active":           pass.Status == "active",
		"created_at":          pass.CreatedAt.Format(time.RFC3339),
		"expires_at":          pass.ExpiresAt,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// DELETE /api/v1/app/credentials/nfc/{credentialId} — Unbind NFC card
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appUnbindNFCCard(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	credID := chi.URLParam(r, "credentialId")
	if credID == "" {
		writeError(w, http.StatusBadRequest, "credential_id is required")
		return
	}

	pass, err := s.walletSvc.GetPass(user.TenantID, credID)
	if err != nil {
		writeError(w, http.StatusNotFound, "credential not found")
		return
	}
	if pass.TargetID != user.ID {
		writeError(w, http.StatusForbidden, "not your credential")
		return
	}

	if _, err := s.walletSvc.RevokePass(user.TenantID, credID, user.Email); err != nil {
		writeInternalError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "revoked",
		"id":      credID,
		"message": "NFC card unbound successfully",
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/qr-token — Generate QR token for door unlock
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appGenerateQRToken(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		DoorID string `json:"door_id"`
	}
	// Body is optional
	_ = decodeJSON(r, &req)

	token := randomHex(32)
	expiresAt := time.Now().UTC().Add(5 * time.Minute)

	tokenKey := "qr_token:" + user.TenantID + ":" + token
	tokenData := map[string]any{
		"token":      token,
		"user_id":    user.ID,
		"door_id":    req.DoorID,
		"expires_at": expiresAt.Format(time.RFC3339),
	}
	if s.stateStore != nil {
		if err := s.stateStore.Save(tokenKey, tokenData); err != nil {
			s.loggerOrDefault().Warn("state store save failed", "key", tokenKey, "error", err)
		}
	} else {
		s.memStoreMu.Lock()
		s.memStore[tokenKey] = tokenData
		s.memStoreMu.Unlock()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/orgs/{orgId}/settings — Org settings
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appGetOrgSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	orgID := chi.URLParam(r, "orgId")
	tenantID := user.TenantID
	if orgID != tenantID {
		writeError(w, http.StatusForbidden, "not your organization")
		return
	}

	settingsKey := "org_settings:" + tenantID
	var settings map[string]any
	found := false
	if s.stateStore != nil {
		var loadErr error
		found, loadErr = s.stateStore.Load(settingsKey, &settings)
		if loadErr != nil {
			s.loggerOrDefault().Warn("state store load failed", "key", settingsKey, "error", loadErr)
		}
	} else {
		s.memStoreMu.RLock()
		if v, ok := s.memStore[settingsKey]; ok {
			if m, ok2 := v.(map[string]any); ok2 {
				settings = m
				found = true
			}
		}
		s.memStoreMu.RUnlock()
	}
	if !found {
		tenants := s.tenantSvc.List()
		orgName := ""
		for _, t := range tenants {
			if t.ID == tenantID {
				orgName = t.Name
				break
			}
		}
		settings = map[string]any{
			"name":                          orgName,
			"timezone":                      "Asia/Jakarta",
			"send_emails":                   true,
			"email_access_assignment":       true,
			"email_credential_assignment":   true,
			"email_incident_alerts":         true,
			"email_reports":                 false,
			"whatsapp_enabled":              false,
			"whatsapp_access_assignment":    false,
			"whatsapp_credential_assignment":false,
			"whatsapp_incident_alerts":      false,
			"webauthn_enabled":              false,
		}
	}

	writeJSON(w, http.StatusOK, settings)
}

// ─────────────────────────────────────────────────────────────────────────────
// PUT /api/v1/app/orgs/{orgId}/settings
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appUpdateOrgSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	orgID := chi.URLParam(r, "orgId")
	tenantID := user.TenantID
	if orgID != tenantID {
		writeError(w, http.StatusForbidden, "not your organization")
		return
	}

	var settings map[string]any
	if err := decodeJSON(r, &settings); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	settingsKey := "org_settings:" + tenantID
	if s.stateStore != nil {
		if err := s.stateStore.Save(settingsKey, settings); err != nil {
			s.loggerOrDefault().Warn("state store save failed", "key", settingsKey, "error", err)
		}
	} else {
		s.memStoreMu.Lock()
		s.memStore[settingsKey] = settings
		s.memStoreMu.Unlock()
	}

	writeJSON(w, http.StatusOK, settings)
}

// ─────────────────────────────────────────────────────────────────────────────
// PUT /api/v1/app/places/{placeId}/settings — Update place settings
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appUpdatePlaceSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	tenantID := user.TenantID

	building, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	var req struct {
		Name     *string `json:"name"`
		Address  *string `json:"address"`
		Timezone *string `json:"timezone"`
		Capacity *int    `json:"capacity"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Store place settings update
	placeKey := "place_settings:" + tenantID + ":" + placeID
	settings := map[string]any{
		"id":      placeID,
		"name":    building.Name,
		"address": building.Address,
	}
	if req.Name != nil {
		settings["name"] = *req.Name
	}
	if req.Address != nil {
		settings["address"] = *req.Address
	}
	if req.Timezone != nil {
		settings["timezone"] = *req.Timezone
	}
	if req.Capacity != nil {
		settings["capacity"] = *req.Capacity
	}

	if s.stateStore != nil {
		if err := s.stateStore.Save(placeKey, settings); err != nil {
			s.loggerOrDefault().Warn("state store save failed", "key", placeKey, "error", err)
		}
	} else {
		s.memStoreMu.Lock()
		s.memStore[placeKey] = settings
		s.memStoreMu.Unlock()
	}

	writeJSON(w, http.StatusOK, settings)
}

// normalizeNFCUID strips colons, dashes, and spaces then uppercases.
// iOS sends "04:4F:C9:BA:CC:1F:90", reader sends "044FC9BACC1F90" — this ensures both match.
func normalizeNFCUID(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return strings.ToUpper(s)
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/credentials/nfc — List user's NFC cards
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appListNFCCards(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var cards []map[string]any
	for _, p := range s.walletSvc.ListPasses(user.TenantID) {
		if p.CredentialKind != "physical_card" || p.TargetID != user.ID {
			continue
		}
		name := p.DeviceName
		if name == "" {
			name = "NFC Card"
		}
		cards = append(cards, map[string]any{
			"id":                     p.ID,
			"device_name":           name,
			"public_key_fingerprint": p.UID,
			"is_active":             p.Status == "active",
			"created_at":            p.CreatedAt.Format(time.RFC3339),
			"expires_at":            p.ExpiresAt,
		})
	}
	if cards == nil {
		cards = []map[string]any{}
	}

	writeJSON(w, http.StatusOK, cards)
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
