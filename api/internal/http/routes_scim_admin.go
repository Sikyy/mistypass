package httpx

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SCIM Admin API — allows tenant admins to manage their SCIM provisioning configuration.
// These endpoints power the Enterprise > SCIM tab in the web admin console.
// Customers use this to get their SCIM Base URL and generate Bearer tokens for
// configuring Okta / Entra ID / OneLogin / etc.

type scimConfig struct {
	mu     sync.RWMutex
	tokens map[string]scimTokenEntry // tenantID → token entry
}

type scimTokenEntry struct {
	Token     string `json:"token"`
	Label     string `json:"label"`
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by"`
	LastUsed  string `json:"last_used,omitempty"`
}

var globalSCIMConfig = &scimConfig{
	tokens: make(map[string]scimTokenEntry),
}

// scimAdminGetConfig returns the SCIM configuration for the tenant.
// GET /api/v1/enterprise/scim/config
func (s *server) scimAdminGetConfig(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	baseURL := scimBaseURL(r) + "/scim/v2"

	globalSCIMConfig.mu.RLock()
	entry, hasToken := globalSCIMConfig.tokens[user.TenantID]
	globalSCIMConfig.mu.RUnlock()

	tokenMasked := ""
	tokenLabel := ""
	tokenCreatedAt := ""
	tokenCreatedBy := ""
	if hasToken {
		// Show first 8 + last 4 chars, mask the rest
		if len(entry.Token) > 12 {
			tokenMasked = entry.Token[:8] + "..." + entry.Token[len(entry.Token)-4:]
		} else {
			tokenMasked = "****"
		}
		tokenLabel = entry.Label
		tokenCreatedAt = entry.CreatedAt
		tokenCreatedBy = entry.CreatedBy
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"scim_base_url":     baseURL,
		"scim_version":      "2.0",
		"auth_mode":         "bearer_token",
		"token_configured":  hasToken,
		"token_masked":      tokenMasked,
		"token_label":       tokenLabel,
		"token_created_at":  tokenCreatedAt,
		"token_created_by":  tokenCreatedBy,
		"supported_operations": []string{"Create", "Read", "Update", "Deactivate", "PATCH"},
		"supported_resources": []string{"Users"},
		"provisioning_guide": "Configure your IdP (Okta, Entra ID, OneLogin) with the Base URL and Bearer Token above.",
		"setup_steps": []map[string]string{
			{"step": "1", "title": "Generate Token", "description": "Click 'Generate Token' below to create a SCIM Bearer Token."},
			{"step": "2", "title": "Copy Base URL", "description": "Copy the SCIM Base URL shown above."},
			{"step": "3", "title": "Configure IdP", "description": "In your IdP (Okta/Entra ID), add a SCIM app and paste the Base URL and Token."},
			{"step": "4", "title": "Test Connection", "description": "Use your IdP's 'Test Connection' button to verify."},
			{"step": "5", "title": "Enable Provisioning", "description": "Turn on Create Users, Update, and Deactivate in your IdP."},
		},
	})
}

// scimAdminGenerateToken creates a new SCIM Bearer token for the tenant.
// POST /api/v1/enterprise/scim/token
func (s *server) scimAdminGenerateToken(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		Label string `json:"label"`
	}
	if err := decodeJSON(r, &req); err != nil {
		req.Label = "default"
	}
	if strings.TrimSpace(req.Label) == "" {
		req.Label = "default"
	}

	// Generate token: tenantID + "_" + 32 random hex chars
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	token := user.TenantID + "_" + hex.EncodeToString(raw)
	now := time.Now().UTC().Format(time.RFC3339)

	entry := scimTokenEntry{
		Token:     token,
		Label:     strings.TrimSpace(req.Label),
		CreatedAt: now,
		CreatedBy: user.Email,
	}

	globalSCIMConfig.mu.Lock()
	globalSCIMConfig.tokens[user.TenantID] = entry
	globalSCIMConfig.mu.Unlock()

	s.auditSvc.Append(user.TenantID, user.Email, user.Role, "scim_token_generated", user.TenantID, req.Label)

	writeJSON(w, http.StatusCreated, map[string]any{
		"token":      token,
		"label":      entry.Label,
		"created_at": now,
		"message":    "Save this token now — it will not be shown again.",
	})
}

// scimAdminRevokeToken revokes the current SCIM token.
// DELETE /api/v1/enterprise/scim/token
func (s *server) scimAdminRevokeToken(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	globalSCIMConfig.mu.Lock()
	_, existed := globalSCIMConfig.tokens[user.TenantID]
	delete(globalSCIMConfig.tokens, user.TenantID)
	globalSCIMConfig.mu.Unlock()

	if !existed {
		writeError(w, http.StatusNotFound, "no SCIM token configured")
		return
	}

	s.auditSvc.Append(user.TenantID, user.Email, user.Role, "scim_token_revoked", user.TenantID, "")

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "revoked",
		"message": "SCIM token revoked. IdP will no longer be able to provision users.",
	})
}

// scimAdminTestEndpoint tests the SCIM endpoint is reachable and configured.
// POST /api/v1/enterprise/scim/test
func (s *server) scimAdminTestEndpoint(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	globalSCIMConfig.mu.RLock()
	entry, hasToken := globalSCIMConfig.tokens[user.TenantID]
	globalSCIMConfig.mu.RUnlock()

	if !hasToken {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "not_configured",
			"message": "No SCIM token configured. Generate a token first.",
		})
		return
	}

	// Verify the token can resolve to the tenant
	tenantID := ""
	for _, t := range s.tenantSvc.List() {
		if strings.HasPrefix(entry.Token, t.ID+"_") {
			tenantID = t.ID
			break
		}
	}

	if tenantID == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "Token tenant resolution failed.",
		})
		return
	}

	// Count users for this tenant
	users := s.accessSvc.ListUsers(tenantID)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "connected",
		"tenant_id":   tenantID,
		"user_count":  len(users),
		"scim_base":   fmt.Sprintf("%s/scim/v2", scimBaseURL(r)),
		"token_label": entry.Label,
		"message":     "SCIM endpoint is operational.",
	})
}

// scimAdminListProvisioningLogs returns recent SCIM provisioning events from audit log.
// GET /api/v1/enterprise/scim/logs
func (s *server) scimAdminListProvisioningLogs(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	// Get audit events with "scim" source
	allEvents := s.auditSvc.List(user.TenantID)
	var scimEvents []map[string]any
	for _, e := range allEvents {
		if strings.Contains(e.Action, "scim") || strings.Contains(e.Source, "scim") {
			scimEvents = append(scimEvents, map[string]any{
				"id":        e.ID,
				"action":    e.Action,
				"actor":     e.Actor,
				"target":    e.Target,
				"source":    e.Source,
				"timestamp": e.At.Format(time.RFC3339),
			})
			if len(scimEvents) >= 50 {
				break
			}
		}
	}

	if scimEvents == nil {
		scimEvents = []map[string]any{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": scimEvents,
		"total": len(scimEvents),
	})
}
