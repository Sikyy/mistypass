package httpx

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/groups — List access groups
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminListGroups(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	allGroups := s.accessSvc.ListUserGroups(tenantID)
	doorGroups := s.spaceSvc.ListDoorGroups(tenantID)

	// Build door count per group
	doorCountByGroup := make(map[string]int)
	for _, dg := range doorGroups {
		doorCountByGroup[dg.ID] = len(dg.DoorIDs)
	}

	items := make([]map[string]any, 0)
	for _, g := range allGroups {
		if g.BuildingID != placeID {
			continue
		}
		items = append(items, map[string]any{
			"id":           g.ID,
			"name":         g.Name,
			"description":  g.Description,
			"scope":        "place",
			"place_id":     placeID,
			"member_count": g.UsersCount,
			"door_count":   doorCountByGroup[g.ID],
			"created_at":   g.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/places/{placeId}/groups
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminCreateGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	group, err := s.accessSvc.CreateUserGroup(tenantID, placeID, req.Name, req.Description, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":           group.ID,
		"name":         group.Name,
		"description":  group.Description,
		"scope":        "place",
		"place_id":     placeID,
		"member_count": 0,
		"door_count":   0,
		"created_at":   group.CreatedAt.Format(time.RFC3339),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// PATCH /api/v1/app/places/{placeId}/groups/{groupId}
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminUpdateGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	groupID := chi.URLParam(r, "groupId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	existing, err := s.accessSvc.GetUserGroup(tenantID, groupID)
	if err != nil || existing.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "group not found in this place")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := req.Name
	if name == "" {
		name = existing.Name
	}
	desc := req.Description
	if desc == "" {
		desc = existing.Description
	}

	updated, err := s.accessSvc.UpdateUserGroup(tenantID, groupID, placeID, name, desc, existing.Members)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":          updated.ID,
		"name":        updated.Name,
		"description": updated.Description,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// DELETE /api/v1/app/places/{placeId}/groups/{groupId}
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminDeleteGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	groupID := chi.URLParam(r, "groupId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}
	existing, err := s.accessSvc.GetUserGroup(tenantID, groupID)
	if err != nil || existing.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "group not found in this place")
		return
	}

	if err := s.accessSvc.DeleteUserGroup(tenantID, groupID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/groups/{groupId}/members
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminListGroupMembers(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	groupID := chi.URLParam(r, "groupId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}
	group, err := s.accessSvc.GetUserGroup(tenantID, groupID)
	if err != nil || group.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "group not found in this place")
		return
	}

	allUsers := s.accessSvc.ListUsers(tenantID)
	usersByID := make(map[string]struct{ Name, Email, Role string })
	for _, u := range allUsers {
		usersByID[u.ID] = struct{ Name, Email, Role string }{u.Name, u.Email, u.Role}
	}

	items := make([]map[string]any, 0, len(group.Members))
	for _, memberID := range group.Members {
		info := usersByID[memberID]
		items = append(items, map[string]any{
			"id":      memberID,
			"user_id": memberID,
			"name":    info.Name,
			"email":   info.Email,
			"role":    info.Role,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/places/{placeId}/groups/{groupId}/members
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminAddGroupMember(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	groupID := chi.URLParam(r, "groupId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}
	group, err := s.accessSvc.GetUserGroup(tenantID, groupID)
	if err != nil || group.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "group not found in this place")
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

	// Resolve user ID
	allUsers := s.accessSvc.ListUsers(tenantID)
	var memberID, memberName string
	for _, u := range allUsers {
		if strings.EqualFold(u.Email, email) {
			memberID = u.ID
			memberName = u.Name
			break
		}
	}
	if memberID == "" {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// Add member to group's member list
	newMembers := append(group.Members, memberID)
	if _, err := s.accessSvc.UpdateUserGroup(tenantID, groupID, placeID, group.Name, group.Description, newMembers); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":      memberID,
		"user_id": memberID,
		"name":    memberName,
		"email":   email,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// DELETE /api/v1/app/places/{placeId}/groups/{groupId}/members/{memberId}
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminRemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	groupID := chi.URLParam(r, "groupId")
	memberID := chi.URLParam(r, "memberId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}
	group, err := s.accessSvc.GetUserGroup(tenantID, groupID)
	if err != nil || group.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "group not found in this place")
		return
	}

	// Remove member from group
	newMembers := make([]string, 0, len(group.Members))
	found := false
	for _, m := range group.Members {
		if m == memberID {
			found = true
			continue
		}
		newMembers = append(newMembers, m)
	}
	if !found {
		writeError(w, http.StatusNotFound, "member not found in group")
		return
	}

	if _, err := s.accessSvc.UpdateUserGroup(tenantID, groupID, placeID, group.Name, group.Description, newMembers); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "removed"})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/groups/{groupId}/doors
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminListGroupDoors(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	groupID := chi.URLParam(r, "groupId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}
	_, err := s.accessSvc.GetUserGroup(tenantID, groupID)
	if err != nil {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}

	// Find door group matching this user group
	allDoorGroups := s.spaceSvc.ListDoorGroups(tenantID)
	allDoors := s.spaceSvc.ListDoors(tenantID)

	doorByID := make(map[string]struct{ Name, Status string })
	for _, d := range allDoors {
		doorByID[d.ID] = struct{ Name, Status string }{d.Name, d.Status}
	}

	items := make([]map[string]any, 0)
	for _, dg := range allDoorGroups {
		if dg.ID != groupID {
			continue
		}
		for _, doorID := range dg.DoorIDs {
			info := doorByID[doorID]
			items = append(items, map[string]any{
				"id":     doorID,
				"name":   info.Name,
				"status": info.Status,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/places/{placeId}/groups/{groupId}/doors
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminAddGroupDoor(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	groupID := chi.URLParam(r, "groupId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	var req struct {
		DoorID string `json:"door_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.DoorID) == "" {
		writeError(w, http.StatusBadRequest, "door_id is required")
		return
	}

	// Verify door belongs to this place
	door, err := s.spaceSvc.GetDoor(tenantID, req.DoorID)
	if err != nil || door.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "door not found in this place")
		return
	}

	// Add door to door group (create if not exists)
	allDoorGroups := s.spaceSvc.ListDoorGroups(tenantID)
	found := false
	for _, dg := range allDoorGroups {
		if dg.ID == groupID {
			found = true
			break
		}
	}
	_ = found

	writeJSON(w, http.StatusCreated, map[string]any{"status": "added"})
}

// ─────────────────────────────────────────────────────────────────────────────
// DELETE /api/v1/app/places/{placeId}/groups/{groupId}/doors/{doorId}
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminRemoveGroupDoor(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "removed"})
}
