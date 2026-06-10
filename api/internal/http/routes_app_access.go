package httpx

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/bus"
	"github.com/mistypass/cloud/api/internal/modules/event"
	"github.com/mistypass/cloud/api/internal/modules/space"
)

// appUnlockDoor handles BLE-based unlock from mobile app.
// POST /api/v1/app/access/unlock
func (s *server) appUnlockDoor(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		LockID    string   `json:"lock_id"`
		BLEToken  string   `json:"ble_token"`
		Latitude  *float64 `json:"latitude"`
		Longitude *float64 `json:"longitude"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	lockID := strings.TrimSpace(req.LockID)
	if lockID == "" {
		writeError(w, http.StatusBadRequest, "lock_id is required")
		return
	}

	now := time.Now().UTC()
	tenantID := user.TenantID

	// Verify the door exists
	door, err := s.spaceSvc.GetDoor(tenantID, lockID)
	if err != nil {
		writeError(w, http.StatusNotFound, "door not found")
		return
	}

	// Check user access to this door
	accessUser, err := s.accessSvc.GetUser(tenantID, user.ID)
	if err != nil || accessUser.Status != "active" {
		s.recordAppAccessEvent(tenantID, lockID, user.Email, "access_denied", "user_not_active")
		writeJSON(w, http.StatusForbidden, map[string]any{
			"decision": "deny",
			"reason":   "user_not_active",
			"lock_id":  lockID,
		})
		return
	}

	groupAllowed, groupName := s.checkUserLockAccess(tenantID, accessUser.GroupIDs, lockID)
	roleAllowed := false
	if !groupAllowed {
		roleAllowed = s.checkRoleAssignmentAccess(tenantID, user.ID, door.BuildingID)
		if roleAllowed {
			groupName = "role_assignment"
		}
	}

	if !groupAllowed && !roleAllowed {
		s.recordAppAccessEvent(tenantID, lockID, user.Email, "access_denied", "no_access")
		writeJSON(w, http.StatusForbidden, map[string]any{
			"decision": "deny",
			"reason":   "no_access",
			"lock_id":  lockID,
		})
		return
	}

	// Geofence enforcement: when access comes only through geofenced group(s),
	// the request must carry coordinates within the granting group's radius.
	hasLoc := req.Latitude != nil && req.Longitude != nil
	var lat, lon float64
	if hasLoc {
		lat, lon = *req.Latitude, *req.Longitude
	}
	if ok, reason := evaluateGeofenceAccess(s.grantingUserGroupsForLock(tenantID, accessUser.GroupIDs, lockID), roleAllowed, hasLoc, lat, lon); !ok {
		s.recordAppAccessEvent(tenantID, lockID, user.Email, "access_denied", reason)
		writeJSON(w, http.StatusForbidden, map[string]any{
			"decision": "deny",
			"reason":   reason,
			"lock_id":  lockID,
		})
		return
	}

	// Dispatch unlock command
	requestID := fmt.Sprintf("app:%s:%s:%d", lockID, user.ID, now.UnixNano())
	dispatched := false

	if gw, found := s.gatewaySvc.FindGatewayByDoorID(tenantID, lockID); found {
		cmd := bus.GatewayCommand{
			RequestID: requestID,
			GatewayID: gw.ID,
			Command:   "unlock",
			LockID:    lockID,
			PlaceID:   door.BuildingID,
			TenantID:  tenantID,
			IssuedBy:  user.Email,
			IssuedAt:  now.Format(time.RFC3339),
		}
		subject := fmt.Sprintf("gateway.%s.command", gw.ID)
		if err := s.messageBus.PublishJSON(r.Context(), subject, cmd, nil); err != nil {
			s.logger.Warn("app unlock: failed to dispatch", "error", err)
		} else if s.messageBus.Enabled() {
			dispatched = true
		}
	}

	status := "accepted"
	if dispatched {
		status = "dispatched"
	}

	s.recordAppAccessEvent(tenantID, lockID, user.Email, "access_granted", "app_unlock")

	writeJSON(w, http.StatusOK, map[string]any{
		"decision":   "allow",
		"reason":     "access_granted",
		"status":     status,
		"request_id": requestID,
		"lock_id":    lockID,
		"lock_name":  door.Name,
		"user_id":    user.ID,
		"group_name": groupName,
		"issued_at":  now.Format(time.RFC3339),
	})
}

// appQRUnlock handles QR code / access link based unlock.
// POST /api/v1/app/access/qr-unlock
func (s *server) appQRUnlock(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		LockID  string `json:"lock_id"`
		QRToken string `json:"qr_token"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	lockID := strings.TrimSpace(req.LockID)
	qrToken := strings.TrimSpace(req.QRToken)
	if lockID == "" || qrToken == "" {
		writeError(w, http.StatusBadRequest, "lock_id and qr_token are required")
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
			// Check expiry
			if link.ValidUntil != "" {
				expiry, err := time.Parse(time.RFC3339, link.ValidUntil)
				if err == nil && now.After(expiry) {
					writeJSON(w, http.StatusForbidden, map[string]any{
						"decision": "deny",
						"reason":   "link_expired",
						"lock_id":  lockID,
					})
					return
				}
			}
			matchedGroupID = link.GroupID
			break
		}
	}

	if matchedGroupID == "" {
		s.recordAppAccessEvent(tenantID, lockID, user.Email, "access_denied", "invalid_qr_token")
		writeJSON(w, http.StatusForbidden, map[string]any{
			"decision": "deny",
			"reason":   "invalid_qr_token",
			"lock_id":  lockID,
		})
		return
	}

	// Verify the door exists
	door, err := s.spaceSvc.GetDoor(tenantID, lockID)
	if err != nil {
		writeError(w, http.StatusNotFound, "door not found")
		return
	}

	// Dispatch unlock
	requestID := fmt.Sprintf("qr:%s:%s:%d", lockID, user.ID, now.UnixNano())
	dispatched := false

	if gw, found := s.gatewaySvc.FindGatewayByDoorID(tenantID, lockID); found {
		cmd := bus.GatewayCommand{
			RequestID: requestID,
			GatewayID: gw.ID,
			Command:   "unlock",
			LockID:    lockID,
			PlaceID:   door.BuildingID,
			TenantID:  tenantID,
			IssuedBy:  user.Email,
			IssuedAt:  now.Format(time.RFC3339),
		}
		subject := fmt.Sprintf("gateway.%s.command", gw.ID)
		if err := s.messageBus.PublishJSON(r.Context(), subject, cmd, nil); err != nil {
			s.logger.Warn("qr unlock: failed to dispatch", "error", err)
		} else if s.messageBus.Enabled() {
			dispatched = true
		}
	}

	status := "accepted"
	if dispatched {
		status = "dispatched"
	}

	s.recordAppAccessEvent(tenantID, lockID, user.Email, "access_granted", "qr_unlock")

	writeJSON(w, http.StatusOK, map[string]any{
		"decision":   "allow",
		"reason":     "access_granted",
		"status":     status,
		"request_id": requestID,
		"lock_id":    lockID,
		"lock_name":  door.Name,
		"user_id":    user.ID,
		"group_id":   matchedGroupID,
		"issued_at":  now.Format(time.RFC3339),
	})
}

// appAccessDoorsEnhanced returns doors the user can access with current schedule status.
// GET /api/v1/app/access/my-doors
func (s *server) appAccessMyDoors(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	tenantID := user.TenantID
	accessUser, err := s.accessSvc.GetUser(tenantID, user.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}

	allDoors := s.spaceSvc.ListDoors(tenantID)
	doorGroups := s.spaceSvc.ListDoorGroups(tenantID)
	userGroups := s.accessSvc.ListUserGroups(tenantID)

	// Build set of accessible door IDs
	accessibleDoorIDs := make(map[string]string) // doorID → group name
	for _, dg := range doorGroups {
		for _, doorID := range dg.DoorIDs {
			// Check if user has a group in the same building
			door := findDoorByID(allDoors, doorID)
			if door == nil {
				continue
			}
			for _, ug := range userGroups {
				if ug.BuildingID != door.BuildingID && ug.PlaceID != door.BuildingID {
					continue
				}
				for _, ugID := range accessUser.GroupIDs {
					if ugID == ug.ID {
						accessibleDoorIDs[doorID] = ug.Name
					}
				}
			}
		}
	}

	// Also add doors from building-level user group membership
	for _, ug := range userGroups {
		for _, ugID := range accessUser.GroupIDs {
			if ugID != ug.ID {
				continue
			}
			for _, door := range allDoors {
				if door.BuildingID != ug.BuildingID && door.BuildingID != ug.PlaceID {
					continue
				}
				if _, exists := accessibleDoorIDs[door.ID]; !exists {
					accessibleDoorIDs[door.ID] = ug.Name
				}
			}
		}
	}

	// Also add doors from role assignments
	assignments := s.accessSvc.ListRoleAssignments(tenantID)
	for _, ra := range assignments {
		if ra.AssigneeType != "User" || ra.AssigneeID != user.ID {
			continue
		}
		for _, door := range allDoors {
			if ra.AppliesToType == "Organization" || (ra.AppliesToType == "Place" && ra.AppliesToID == door.BuildingID) {
				if _, exists := accessibleDoorIDs[door.ID]; !exists {
					accessibleDoorIDs[door.ID] = "role_assignment"
				}
			}
		}
	}

	items := make([]map[string]any, 0, len(accessibleDoorIDs))
	for _, door := range allDoors {
		groupName, accessible := accessibleDoorIDs[door.ID]
		if !accessible {
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
			"area_id":        door.AreaID,
			"status":         door.Status,
			"gateway_status": gwStatus,
			"group_name":     groupName,
			"can_unlock":     hasGateway && gw.Status == "online",
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

func (s *server) recordAppAccessEvent(tenantID, lockID, actor, eventType, reason string) {
	gwID := "app"
	if gw, found := s.gatewaySvc.FindGatewayByDoorID(tenantID, lockID); found {
		gwID = gw.ID
	}
	var buildingID string
	if door, err := s.spaceSvc.GetDoor(tenantID, lockID); err == nil {
		buildingID = door.BuildingID
	}
	s.eventSvc.IngestAccessEvent(event.IngestAccessEventInput{
		TenantID:   tenantID,
		BuildingID: buildingID,
		Type:       eventType,
		Actor:      actor,
		DoorID:     lockID,
		GatewayID:  gwID,
		Result:     reason,
		At:         time.Now().UTC(),
	})
}

func findDoorByID(doors []space.Door, id string) *space.Door {
	for i := range doors {
		if doors[i].ID == id {
			return &doors[i]
		}
	}
	return nil
}
