package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/auth"
)

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/orgs — List organizations the authenticated user belongs to
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appListOrgs(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	if s.orgStore == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	memberships, err := s.orgStore.ListUserOrgMemberships(user.ID)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}

	// Build a tenant lookup map from the tenant service for name resolution.
	tenantMap := map[string]string{} // tenantID → name
	for _, t := range s.tenantSvc.List() {
		tenantMap[t.ID] = t.Name
	}

	type orgItem struct {
		ID         string  `json:"id"`
		Name       string  `json:"name"`
		Domain     string  `json:"domain"`
		Logo       *string `json:"logo"`
		Role       string  `json:"role"`
		LastUsedAt *string `json:"last_used_at"`
	}

	orgs := make([]orgItem, 0, len(memberships))
	for _, m := range memberships {
		name := tenantMap[m.TenantID]
		if name == "" {
			name = m.TenantID // fallback to ID if tenant name unavailable
		}

		var lastUsed *string
		if m.LastUsedAt != nil {
			ts := m.LastUsedAt.UTC().Format("2006-01-02T15:04:05Z")
			lastUsed = &ts
		}

		orgs = append(orgs, orgItem{
			ID:         m.TenantID,
			Name:       name,
			Domain:     "",
			Logo:       nil,
			Role:       m.Role,
			LastUsedAt: lastUsed,
		})
	}

	writeJSON(w, http.StatusOK, orgs)
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/orgs/{orgId}/switch — Switch active org context
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appSwitchOrg(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	orgID := chi.URLParam(r, "orgId")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "org id is required")
		return
	}

	if s.orgStore == nil {
		writeError(w, http.StatusServiceUnavailable, "org membership store not available")
		return
	}

	// Verify the user has a membership in the target org.
	membership, found, err := s.orgStore.GetUserOrgMembership(user.ID, orgID)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	if !found {
		writeError(w, http.StatusForbidden, "not a member of this organization")
		return
	}

	// Update last_used_at timestamp (best-effort, do not fail the request).
	_ = s.orgStore.UpdateOrgMembershipLastUsed(user.ID, orgID)

	// Build a new user scoped to the target org and issue fresh tokens.
	orgUser := auth.User{
		ID:          user.ID,
		Name:        user.Name,
		Email:       user.Email,
		Role:        membership.Role,
		TenantID:    orgID,
		BuildingIDs: nil, // org switch resets building scope
		Language:    user.Language,
	}

	loginResp, err := s.authService.LoginByTrustedUser(orgUser)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  loginResp.AccessToken,
		"refresh_token": loginResp.RefreshToken,
		"expires_in":    loginResp.ExpiresIn,
		"org_id":        orgID,
	})
}
