package httpx

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/bus"
)

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/orgs/{orgId}/places — List places user can access in an org
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appListPlaces(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	orgID := chi.URLParam(r, "orgId")
	if strings.TrimSpace(orgID) == "" {
		writeError(w, http.StatusBadRequest, "org id is required")
		return
	}

	tenantID := user.TenantID

	// Verify org matches the user's current tenant context
	if tenantID != orgID {
		writeJSON(w, http.StatusOK, map[string]any{
			"items": []any{},
		})
		return
	}

	buildings := s.spaceSvc.ListBuildings(tenantID)
	allDoors := s.spaceSvc.ListDoors(tenantID)

	// Count doors per building
	doorCount := make(map[string]int, len(buildings))
	for _, door := range allDoors {
		doorCount[door.BuildingID]++
	}

	items := make([]map[string]any, 0, len(buildings))
	for _, b := range buildings {
		items = append(items, map[string]any{
			"id":          b.ID,
			"name":        b.Name,
			"address":     b.Address,
			"org_id":      b.TenantID,
			"status":      b.Status,
			"is_lockdown": false,
			"door_count":  doorCount[b.ID],
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
// GET /api/v1/app/orgs/{orgId}/places/search?q= — Search places
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appSearchPlaces(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	orgID := chi.URLParam(r, "orgId")
	if strings.TrimSpace(orgID) == "" {
		writeError(w, http.StatusBadRequest, "org id is required")
		return
	}

	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	tenantID := user.TenantID

	if tenantID != orgID {
		writeJSON(w, http.StatusOK, map[string]any{
			"items": []any{},
		})
		return
	}

	buildings := s.spaceSvc.ListBuildings(tenantID)
	allDoors := s.spaceSvc.ListDoors(tenantID)

	doorCount := make(map[string]int, len(buildings))
	for _, door := range allDoors {
		doorCount[door.BuildingID]++
	}

	items := make([]map[string]any, 0, len(buildings))
	for _, b := range buildings {
		if query != "" && !strings.Contains(strings.ToLower(b.Name), query) && !strings.Contains(strings.ToLower(b.Address), query) {
			continue
		}
		items = append(items, map[string]any{
			"id":          b.ID,
			"name":        b.Name,
			"address":     b.Address,
			"org_id":      b.TenantID,
			"status":      b.Status,
			"is_lockdown": false,
			"door_count":  doorCount[b.ID],
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
// GET /api/v1/app/places/{placeId}/doors — List doors in a place
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appPlaceListDoors(w http.ResponseWriter, r *http.Request) {
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

	allDoors := s.spaceSvc.ListDoors(tenantID)
	items := make([]map[string]any, 0)
	for _, door := range allDoors {
		if door.BuildingID != placeID {
			continue
		}
		gw, hasGateway := s.gatewaySvc.FindGatewayByDoorID(tenantID, door.ID)
		gwStatus := "disconnected"
		if hasGateway {
			gwStatus = gw.Status
		}
		items = append(items, map[string]any{
			"id":             door.ID,
			"name":           door.Name,
			"building_id":    door.BuildingID,
			"floor_id":       door.FloorID,
			"area_id":        door.AreaID,
			"kind":           door.Kind,
			"status":         door.Status,
			"gateway_status": gwStatus,
			"can_unlock":     hasGateway && gw.Status == "online",
			"is_favorite":    s.isDoorFavorite(user.ID, door.ID),
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
// GET /api/v1/app/places/{placeId}/doors/search?q= — Search doors in a place
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appPlaceSearchDoors(w http.ResponseWriter, r *http.Request) {
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

	allDoors := s.spaceSvc.ListDoors(tenantID)
	items := make([]map[string]any, 0)
	for _, door := range allDoors {
		if door.BuildingID != placeID {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(door.Name), query) {
			continue
		}
		gw, hasGateway := s.gatewaySvc.FindGatewayByDoorID(tenantID, door.ID)
		gwStatus := "disconnected"
		if hasGateway {
			gwStatus = gw.Status
		}
		items = append(items, map[string]any{
			"id":             door.ID,
			"name":           door.Name,
			"building_id":    door.BuildingID,
			"floor_id":       door.FloorID,
			"area_id":        door.AreaID,
			"kind":           door.Kind,
			"status":         door.Status,
			"gateway_status": gwStatus,
			"can_unlock":     hasGateway && gw.Status == "online",
			"is_favorite":    s.isDoorFavorite(user.ID, door.ID),
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
// POST /api/v1/app/places/{placeId}/doors/{doorId}/unlock — Unlock door
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appPlaceUnlockDoor(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	doorID := chi.URLParam(r, "doorId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(doorID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and door_id are required")
		return
	}

	now := time.Now().UTC()
	tenantID := user.TenantID

	// Verify the door exists
	door, err := s.spaceSvc.GetDoor(tenantID, doorID)
	if err != nil {
		writeError(w, http.StatusNotFound, "door not found")
		return
	}

	// Validate that the door belongs to the specified place
	if door.BuildingID != placeID {
		writeError(w, http.StatusBadRequest, "door does not belong to this place")
		return
	}

	// Check user access to this door
	accessUser, err := s.accessSvc.GetUser(tenantID, user.ID)
	if err != nil || accessUser.Status != "active" {
		s.recordAppAccessEvent(tenantID, doorID, user.Email, "access_denied", "user_not_active")
		writeJSON(w, http.StatusForbidden, map[string]any{
			"decision": "deny",
			"reason":   "user_not_active",
			"lock_id":  doorID,
		})
		return
	}

	allowed, groupName := s.checkUserLockAccess(tenantID, accessUser.GroupIDs, doorID)
	if !allowed {
		allowed = s.checkRoleAssignmentAccess(tenantID, user.ID, door.BuildingID)
		if allowed {
			groupName = "role_assignment"
		}
	}

	if !allowed {
		s.recordAppAccessEvent(tenantID, doorID, user.Email, "access_denied", "no_access")
		writeJSON(w, http.StatusForbidden, map[string]any{
			"decision": "deny",
			"reason":   "no_access",
			"lock_id":  doorID,
		})
		return
	}

	// Dispatch unlock command
	requestID := fmt.Sprintf("app:%s:%s:%d", doorID, user.ID, now.UnixNano())
	dispatched := false

	if gw, found := s.gatewaySvc.FindGatewayByDoorID(tenantID, doorID); found {
		cmd := bus.GatewayCommand{
			RequestID: requestID,
			GatewayID: gw.ID,
			Command:   "unlock",
			LockID:    doorID,
			PlaceID:   door.BuildingID,
			TenantID:  tenantID,
			IssuedBy:  user.Email,
			IssuedAt:  now.Format(time.RFC3339),
		}
		subject := fmt.Sprintf("gateway.%s.command", gw.ID)
		if err := s.messageBus.PublishJSON(r.Context(), subject, cmd, nil); err != nil {
			s.logger.Warn("place unlock: failed to dispatch", "error", err)
		} else if s.messageBus.Enabled() {
			dispatched = true
		}
	}

	status := "accepted"
	if dispatched {
		status = "dispatched"
	}

	s.recordAppAccessEvent(tenantID, doorID, user.Email, "access_granted", "app_unlock")

	writeJSON(w, http.StatusOK, map[string]any{
		"decision":   "allow",
		"reason":     "access_granted",
		"status":     status,
		"request_id": requestID,
		"lock_id":    doorID,
		"lock_name":  door.Name,
		"place_id":   placeID,
		"user_id":    user.ID,
		"group_name": groupName,
		"issued_at":  now.Format(time.RFC3339),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/places/{placeId}/doors/{doorId}/qr-unlock — QR unlock
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appPlaceQRUnlock(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	doorID := chi.URLParam(r, "doorId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(doorID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and door_id are required")
		return
	}

	var req struct {
		QRToken string `json:"qr_token"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	qrToken := strings.TrimSpace(req.QRToken)
	if qrToken == "" {
		writeError(w, http.StatusBadRequest, "qr_token is required")
		return
	}

	now := time.Now().UTC()
	tenantID := user.TenantID

	// Verify QR token via group_links
	links := s.accessSvc.ListGroupLinks(tenantID)
	var matchedGroupID string
	for _, link := range links {
		if !link.LinkEnabled {
			continue
		}
		if link.Secret == qrToken || link.QuickResponseCodeToken == qrToken {
			if link.ValidUntil != "" {
				expiry, err := time.Parse(time.RFC3339, link.ValidUntil)
				if err == nil && now.After(expiry) {
					writeJSON(w, http.StatusForbidden, map[string]any{
						"decision": "deny",
						"reason":   "link_expired",
						"lock_id":  doorID,
					})
					return
				}
			}
			matchedGroupID = link.GroupID
			break
		}
	}

	if matchedGroupID == "" {
		s.recordAppAccessEvent(tenantID, doorID, user.Email, "access_denied", "invalid_qr_token")
		writeJSON(w, http.StatusForbidden, map[string]any{
			"decision": "deny",
			"reason":   "invalid_qr_token",
			"lock_id":  doorID,
		})
		return
	}

	// Verify the door exists
	door, err := s.spaceSvc.GetDoor(tenantID, doorID)
	if err != nil {
		writeError(w, http.StatusNotFound, "door not found")
		return
	}

	// Validate that the door belongs to the specified place
	if door.BuildingID != placeID {
		writeError(w, http.StatusBadRequest, "door does not belong to this place")
		return
	}

	// Dispatch unlock
	requestID := fmt.Sprintf("qr:%s:%s:%d", doorID, user.ID, now.UnixNano())
	dispatched := false

	if gw, found := s.gatewaySvc.FindGatewayByDoorID(tenantID, doorID); found {
		cmd := bus.GatewayCommand{
			RequestID: requestID,
			GatewayID: gw.ID,
			Command:   "unlock",
			LockID:    doorID,
			PlaceID:   door.BuildingID,
			TenantID:  tenantID,
			IssuedBy:  user.Email,
			IssuedAt:  now.Format(time.RFC3339),
		}
		subject := fmt.Sprintf("gateway.%s.command", gw.ID)
		if err := s.messageBus.PublishJSON(r.Context(), subject, cmd, nil); err != nil {
			s.logger.Warn("place qr unlock: failed to dispatch", "error", err)
		} else if s.messageBus.Enabled() {
			dispatched = true
		}
	}

	status := "accepted"
	if dispatched {
		status = "dispatched"
	}

	s.recordAppAccessEvent(tenantID, doorID, user.Email, "access_granted", "qr_unlock")

	writeJSON(w, http.StatusOK, map[string]any{
		"decision":   "allow",
		"reason":     "access_granted",
		"status":     status,
		"request_id": requestID,
		"lock_id":    doorID,
		"lock_name":  door.Name,
		"place_id":   placeID,
		"user_id":    user.ID,
		"group_id":   matchedGroupID,
		"issued_at":  now.Format(time.RFC3339),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/places/{placeId}/lockdown — Enable lockdown
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appPlaceEnableLockdown(w http.ResponseWriter, r *http.Request) {
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

	// Verify place exists
	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"place_id":   placeID,
		"status":     "active",
		"enabled_by": user.Email,
		"enabled_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// DELETE /api/v1/app/places/{placeId}/lockdown — Disable lockdown
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appPlaceDisableLockdown(w http.ResponseWriter, r *http.Request) {
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

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"place_id":    placeID,
		"status":      "inactive",
		"disabled_by": user.Email,
		"disabled_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// PUT /api/v1/app/places/{placeId}/doors/{doorId}/favorite — Favorite door
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appPlaceFavoriteDoor(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	doorID := chi.URLParam(r, "doorId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(doorID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and door_id are required")
		return
	}

	tenantID := user.TenantID

	// Verify door exists
	door, err := s.spaceSvc.GetDoor(tenantID, doorID)
	if err != nil {
		writeError(w, http.StatusNotFound, "door not found")
		return
	}

	// Validate that the door belongs to the specified place
	if door.BuildingID != placeID {
		writeError(w, http.StatusBadRequest, "door does not belong to this place")
		return
	}

	s.setDoorFavorite(user.ID, doorID, true)

	writeJSON(w, http.StatusOK, map[string]any{
		"door_id":  doorID,
		"place_id": placeID,
		"favorite": true,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// DELETE /api/v1/app/places/{placeId}/doors/{doorId}/favorite — Unfavorite
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appPlaceUnfavoriteDoor(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	doorID := chi.URLParam(r, "doorId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(doorID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and door_id are required")
		return
	}

	tenantID := user.TenantID

	// Verify door exists
	door, err := s.spaceSvc.GetDoor(tenantID, doorID)
	if err != nil {
		writeError(w, http.StatusNotFound, "door not found")
		return
	}

	// Validate that the door belongs to the specified place
	if door.BuildingID != placeID {
		writeError(w, http.StatusBadRequest, "door does not belong to this place")
		return
	}

	s.setDoorFavorite(user.ID, doorID, false)

	writeJSON(w, http.StatusOK, map[string]any{
		"door_id":  doorID,
		"place_id": placeID,
		"favorite": false,
	})
}
