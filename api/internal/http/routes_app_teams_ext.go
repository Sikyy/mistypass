package httpx

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
)

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/teams/{teamId}/members
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminListTeamMembers(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	teamID := chi.URLParam(r, "teamId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(teamID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and team_id are required")
		return
	}

	tenantID := user.TenantID
	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	team, err := s.accessSvc.GetTeam(tenantID, teamID)
	if err != nil || team.PlaceID != placeID {
		writeError(w, http.StatusNotFound, "team not found in this place")
		return
	}

	allMemberships := s.accessSvc.ListTeamMemberships(tenantID)
	items := make([]map[string]any, 0)
	for _, m := range allMemberships {
		if m.TeamID != teamID {
			continue
		}
		items = append(items, map[string]any{
			"id":       m.ID,
			"user_id":  m.MemberID,
			"name":     m.MemberName,
			"email":    m.MemberEmail,
			"added_at": m.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/places/{placeId}/teams/{teamId}/members
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminAddTeamMember(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	teamID := chi.URLParam(r, "teamId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}
	team, err := s.accessSvc.GetTeam(tenantID, teamID)
	if err != nil || team.PlaceID != placeID {
		writeError(w, http.StatusNotFound, "team not found in this place")
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	// Resolve user name from access users
	memberName := email
	allUsers := s.accessSvc.ListUsers(tenantID)
	var memberID string
	for _, u := range allUsers {
		if strings.EqualFold(u.Email, email) {
			memberName = u.Name
			memberID = u.ID
			break
		}
	}

	membership, err := s.accessSvc.CreateTeamMembership(tenantID, teamID, "User", memberID, email, memberName, "Manual")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":       membership.ID,
		"user_id":  membership.MemberID,
		"name":     membership.MemberName,
		"email":    membership.MemberEmail,
		"added_at": membership.CreatedAt.Format(time.RFC3339),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// DELETE /api/v1/app/places/{placeId}/teams/{teamId}/members/{memberId}
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminRemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	teamID := chi.URLParam(r, "teamId")
	memberID := chi.URLParam(r, "memberId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}
	team, err := s.accessSvc.GetTeam(tenantID, teamID)
	if err != nil || team.PlaceID != placeID {
		writeError(w, http.StatusNotFound, "team not found in this place")
		return
	}

	if err := s.accessSvc.DeleteTeamMembership(tenantID, memberID); err != nil {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "removed"})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/teams/{teamId}/access-rights
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminListTeamAccessRights(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	teamID := chi.URLParam(r, "teamId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}
	team, err := s.accessSvc.GetTeam(tenantID, teamID)
	if err != nil || team.PlaceID != placeID {
		writeError(w, http.StatusNotFound, "team not found in this place")
		return
	}

	// Gather role assignments for this team
	allRA := s.accessSvc.ListRoleAssignments(tenantID)
	items := make([]map[string]any, 0)
	for _, ra := range allRA {
		if ra.AssigneeType == "Team" && ra.AssigneeID == teamID {
			items = append(items, map[string]any{
				"id":       ra.ID,
				"role":     ra.RoleID,
				"scope":    ra.AppliesToType,
				"scope_id": ra.AppliesToID,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/places/{placeId}/teams/{teamId}/access-rights
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminAssignTeamAccessRight(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	teamID := chi.URLParam(r, "teamId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}
	team, err := s.accessSvc.GetTeam(tenantID, teamID)
	if err != nil || team.PlaceID != placeID {
		writeError(w, http.StatusNotFound, "team not found in this place")
		return
	}

	var req struct {
		Role    string `json:"role"`
		Scope   string `json:"scope"`
		ScopeID string `json:"scope_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	scope := strings.TrimSpace(req.Scope)
	if scope == "" {
		scope = "Place"
	}
	scopeID := strings.TrimSpace(req.ScopeID)
	if scopeID == "" {
		scopeID = placeID
	}

	ra, err := s.accessSvc.CreateRoleAssignment(access.RoleAssignmentInput{
		TenantID:      tenantID,
		RoleID:        strings.TrimSpace(req.Role),
		AppliesToType: scope,
		AppliesToID:   scopeID,
		AssigneeType:  "Team",
		AssigneeID:    teamID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":       ra.ID,
		"role":     ra.RoleID,
		"scope":    ra.AppliesToType,
		"scope_id": ra.AppliesToID,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// DELETE /api/v1/app/places/{placeId}/teams/{teamId}/access-rights/{accessRightId}
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminRemoveTeamAccessRight(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	accessRightID := chi.URLParam(r, "accessRightId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	if err := s.accessSvc.DeleteRoleAssignment(tenantID, accessRightID); err != nil {
		writeError(w, http.StatusNotFound, "access right not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "removed"})
}
