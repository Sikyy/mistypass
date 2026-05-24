package httpx

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
)

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/users — List users in a place
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminListUsers(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	if strings.TrimSpace(placeID) == "" {
		writeError(w, http.StatusBadRequest, "place id is required")
		return
	}

	tenantID := user.TenantID

	// Verify place (building) exists and belongs to tenant
	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	allUsers := s.accessSvc.ListUsers(tenantID)
	items := make([]map[string]any, 0, len(allUsers))
	for _, u := range allUsers {
		if u.BuildingID != placeID {
			continue
		}
		items = append(items, map[string]any{
			"id":         u.ID,
			"name":       u.Name,
			"email":      u.Email,
			"role":       u.Role,
			"status":     u.Status,
			"place_id":   placeID,
			"group_ids":  u.GroupIDs,
			"created_at": u.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"pagination": map[string]any{
			"offset":   0,
			"limit":    len(items),
			"total":    len(items),
			"has_more": false,
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/users/search?q= — Search users in a place
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminSearchUsers(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	if strings.TrimSpace(placeID) == "" {
		writeError(w, http.StatusBadRequest, "place id is required")
		return
	}

	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	allUsers := s.accessSvc.ListUsers(tenantID)
	items := make([]map[string]any, 0, len(allUsers))
	for _, u := range allUsers {
		if u.BuildingID != placeID {
			continue
		}
		if query != "" &&
			!strings.Contains(strings.ToLower(u.Name), query) &&
			!strings.Contains(strings.ToLower(u.Email), query) {
			continue
		}
		items = append(items, map[string]any{
			"id":         u.ID,
			"name":       u.Name,
			"email":      u.Email,
			"role":       u.Role,
			"status":     u.Status,
			"place_id":   placeID,
			"group_ids":  u.GroupIDs,
			"created_at": u.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"pagination": map[string]any{
			"offset":   0,
			"limit":    len(items),
			"total":    len(items),
			"has_more": false,
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/users/{userId} — Get user details
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminGetUser(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	userID := chi.URLParam(r, "userId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(userID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and user_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	accessUser, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if accessUser.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "user not found in this place")
		return
	}

	// Collect group names for this user
	groups := s.accessSvc.ListUserGroups(tenantID)
	userGroups := make([]map[string]any, 0)
	for _, g := range groups {
		for _, gid := range accessUser.GroupIDs {
			if g.ID == gid {
				userGroups = append(userGroups, map[string]any{
					"id":   g.ID,
					"name": g.Name,
				})
				break
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         accessUser.ID,
		"name":       accessUser.Name,
		"email":      accessUser.Email,
		"role":       accessUser.Role,
		"status":     accessUser.Status,
		"place_id":   placeID,
		"group_ids":  accessUser.GroupIDs,
		"groups":     userGroups,
		"created_at": accessUser.CreatedAt.Format(time.RFC3339),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/places/{placeId}/users — Add user to place
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminAddUser(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	if strings.TrimSpace(placeID) == "" {
		writeError(w, http.StatusBadRequest, "place id is required")
		return
	}

	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Role  string `json:"role"`
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

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role == "" {
		role = "employee"
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = email
	}

	created, err := s.accessSvc.CreateUser(tenantID, placeID, name, email, role, "active", nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         created.ID,
		"name":       created.Name,
		"email":      created.Email,
		"role":       created.Role,
		"status":     created.Status,
		"place_id":   placeID,
		"created_at": created.CreatedAt.Format(time.RFC3339),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// PUT /api/v1/app/places/{placeId}/users/{userId}/role — Update user role
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	userID := chi.URLParam(r, "userId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(userID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and user_id are required")
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role == "" {
		writeError(w, http.StatusBadRequest, "role is required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	accessUser, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if accessUser.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "user not found in this place")
		return
	}

	updated, err := s.accessSvc.UpdateUser(
		tenantID, userID, accessUser.BuildingID,
		accessUser.Name, accessUser.Email, role, accessUser.Status,
		accessUser.GroupIDs,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":       updated.ID,
		"name":     updated.Name,
		"email":    updated.Email,
		"role":     updated.Role,
		"status":   updated.Status,
		"place_id": placeID,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/users/{userId}/logins — User login sessions
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminGetUserLogins(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	userID := chi.URLParam(r, "userId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(userID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and user_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	accessUser, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if accessUser.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "user not found in this place")
		return
	}

	sessions := s.authService.ListActiveSessions(userID)
	items := make([]map[string]any, 0, len(sessions))
	for _, sess := range sessions {
		items = append(items, map[string]any{
			"session_id":   sess.SessionID,
			"ip_address":   sess.IPAddress,
			"user_agent":   sess.UserAgent,
			"login_method": sess.LoginMethod,
			"created_at":   sess.CreatedAt.Format(time.RFC3339),
			"expires_at":   sess.ExpiresAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id": userID,
		"items":   items,
		"pagination": map[string]any{
			"offset":   0,
			"limit":    len(items),
			"total":    len(items),
			"has_more": false,
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/users/{userId}/access-rights — User access rights
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminGetAccessRights(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	userID := chi.URLParam(r, "userId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(userID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and user_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	accessUser, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if accessUser.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "user not found in this place")
		return
	}

	// Find all doors in this place that the user can access through their groups.
	// Door groups map group IDs to door IDs — if a user belongs to a group,
	// they can access the doors assigned to that group.
	allDoors := s.spaceSvc.ListDoors(tenantID)
	doorGroups := s.spaceSvc.ListDoorGroups(tenantID)

	// Build a set of door IDs accessible through user's groups
	accessibleDoorIDs := make(map[string]struct{})
	for _, dg := range doorGroups {
		for _, gid := range accessUser.GroupIDs {
			if dg.ID == gid {
				for _, doorID := range dg.DoorIDs {
					accessibleDoorIDs[doorID] = struct{}{}
				}
				break
			}
		}
	}

	// Also check role assignments for this user that apply to this place
	roleAssignments := s.accessSvc.ListRoleAssignments(tenantID)
	hasPlaceRoleAccess := false
	for _, ra := range roleAssignments {
		if ra.AssigneeType == "User" && ra.AssigneeID == userID &&
			ra.AppliesToType == "Place" && ra.AppliesToID == placeID {
			hasPlaceRoleAccess = true
			break
		}
	}

	items := make([]map[string]any, 0)
	for _, door := range allDoors {
		if door.BuildingID != placeID {
			continue
		}
		_, hasGroupAccess := accessibleDoorIDs[door.ID]
		if hasGroupAccess || hasPlaceRoleAccess {
			items = append(items, map[string]any{
				"door_id":    door.ID,
				"door_name":  door.Name,
				"place_id":   placeID,
				"kind":       door.Kind,
				"status":     door.Status,
				"source":     accessRightSource(hasGroupAccess, hasPlaceRoleAccess),
				"can_access": true,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id": userID,
		"items":   items,
		"pagination": map[string]any{
			"offset":   0,
			"limit":    len(items),
			"total":    len(items),
			"has_more": false,
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/places/{placeId}/users/{userId}/share-access — Share access
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminShareAccess(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	userID := chi.URLParam(r, "userId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(userID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and user_id are required")
		return
	}

	var req struct {
		DoorIDs []string `json:"door_ids"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.DoorIDs) == 0 {
		writeError(w, http.StatusBadRequest, "door_ids is required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	accessUser, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if accessUser.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "user not found in this place")
		return
	}

	// Validate that requested doors belong to this place
	allDoors := s.spaceSvc.ListDoors(tenantID)
	placeDoorIDs := make(map[string]struct{})
	for _, door := range allDoors {
		if door.BuildingID == placeID {
			placeDoorIDs[door.ID] = struct{}{}
		}
	}

	validDoorIDs := make([]string, 0, len(req.DoorIDs))
	for _, doorID := range req.DoorIDs {
		doorID = strings.TrimSpace(doorID)
		if doorID == "" {
			continue
		}
		if _, ok := placeDoorIDs[doorID]; !ok {
			writeError(w, http.StatusBadRequest, "door "+doorID+" does not belong to this place")
			return
		}
		validDoorIDs = append(validDoorIDs, doorID)
	}

	// Create temporary access entries for each door
	now := time.Now().UTC()
	granted := make([]map[string]any, 0, len(validDoorIDs))
	for _, doorID := range validDoorIDs {
		ta, err := s.accessSvc.CreateTemporaryAccessWithInput(access.TemporaryAccessInput{
			TenantID:          tenantID,
			ScopeType:         "door",
			BuildingID:        placeID,
			DoorID:            doorID,
			DeliveryMethod:    "email_qr",
			GranteeName:       accessUser.Name,
			GranteePhone:      "not_provided",
			GranteeEmail:      accessUser.Email,
			ValidFrom:         now.Format(time.RFC3339),
			ValidUntil:        now.Add(24 * time.Hour).Format(time.RFC3339),
			AuthorizedByID:    user.ID,
			AuthorizedByEmail: user.Email,
			AuthorizedByRole:  user.Role,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to grant access: "+err.Error())
			return
		}
		granted = append(granted, map[string]any{
			"id":          ta.ID,
			"door_id":     doorID,
			"valid_from":  ta.ValidFrom,
			"valid_until": ta.ValidUntil,
		})
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"user_id":  userID,
		"place_id": placeID,
		"granted":  granted,
	})
}

// accessRightSource returns the human-readable source of the access right.
func accessRightSource(fromGroup, fromRole bool) string {
	if fromGroup && fromRole {
		return "group+role"
	}
	if fromGroup {
		return "group"
	}
	return "role"
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/places/{placeId}/users/invite — Invite user by email
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminInviteUser(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	if strings.TrimSpace(placeID) == "" {
		writeError(w, http.StatusBadRequest, "place id is required")
		return
	}

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
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

	tenantID := user.TenantID
	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role == "" {
		role = "employee"
	}

	created, err := s.accessSvc.CreateUser(tenantID, placeID, email, email, role, "invited", nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":       created.ID,
		"name":     created.Name,
		"email":    created.Email,
		"role":     created.Role,
		"status":   created.Status,
		"place_id": placeID,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// DELETE /api/v1/app/places/{placeId}/users/{userId} — Remove user from place
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminRemoveUser(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	userID := chi.URLParam(r, "userId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(userID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and user_id are required")
		return
	}

	tenantID := user.TenantID
	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	accessUser, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if accessUser.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "user not found in this place")
		return
	}

	if _, err := s.accessSvc.DeleteUser(tenantID, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "removed"})
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/places/{placeId}/users/{userId}/sign-out — Force sign-out
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminSignOutUser(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	userID := chi.URLParam(r, "userId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(userID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and user_id are required")
		return
	}

	tenantID := user.TenantID
	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	accessUser, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if accessUser.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "user not found in this place")
		return
	}

	// Revoke all sessions for this user by email
	count := s.authService.RevokeRefreshTokensByUserEmail(accessUser.Email)
	_ = count

	writeJSON(w, http.StatusOK, map[string]any{"status": "signed_out"})
}
