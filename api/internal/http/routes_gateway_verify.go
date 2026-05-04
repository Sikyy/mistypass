package httpx

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/bus"
	"github.com/mistypass/cloud/api/internal/modules/event"
)

type verifyCredentialRequest struct {
	GatewayID      string `json:"gateway_id"`
	ReaderID       string `json:"reader_id"`
	LockID         string `json:"lock_id"`
	TenantID       string `json:"tenant_id"`
	CredentialType string `json:"credential_type"` // nfc_uid, ble_token, card_number, qr_code
	CredentialData string `json:"credential_data"`
}

type verifyCredentialResponse struct {
	Decision       string `json:"decision"` // allow, deny
	Reason         string `json:"reason"`
	UserID         string `json:"user_id,omitempty"`
	UserName       string `json:"user_name,omitempty"`
	UserEmail      string `json:"user_email,omitempty"`
	GroupName      string `json:"group_name,omitempty"`
	LockID         string `json:"lock_id"`
	GatewayID      string `json:"gateway_id,omitempty"`
	CredentialType string `json:"credential_type"`
	EvaluatedAt    string `json:"evaluated_at"`
}

func (s *server) verifyCredential(w http.ResponseWriter, r *http.Request) {
	var req verifyCredentialRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	credType := strings.TrimSpace(req.CredentialType)
	credData := strings.TrimSpace(req.CredentialData)
	lockID := strings.TrimSpace(req.LockID)
	tenantID := strings.TrimSpace(req.TenantID)
	gatewayID := strings.TrimSpace(req.GatewayID)
	now := time.Now().UTC()

	if credData == "" {
		writeJSON(w, http.StatusOK, verifyCredentialResponse{
			Decision:       "deny",
			Reason:         "empty_credential",
			LockID:         lockID,
			GatewayID:      gatewayID,
			CredentialType: credType,
			EvaluatedAt:    now.Format(time.RFC3339),
		})
		return
	}
	if lockID == "" {
		writeJSON(w, http.StatusOK, verifyCredentialResponse{
			Decision:       "deny",
			Reason:         "missing_lock_id",
			CredentialType: credType,
			EvaluatedAt:    now.Format(time.RFC3339),
		})
		return
	}

	// Step 1: Resolve credential → user_id
	userID, credentialFound := s.resolveCredentialToUser(tenantID, credType, credData)
	if !credentialFound {
		resp := verifyCredentialResponse{
			Decision:       "deny",
			Reason:         "credential_not_found",
			LockID:         lockID,
			GatewayID:      gatewayID,
			CredentialType: credType,
			EvaluatedAt:    now.Format(time.RFC3339),
		}
		s.recordVerifyEvent(tenantID, gatewayID, lockID, resp)
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Step 2: Guest QR tokens bypass normal user lookup
	if strings.HasPrefix(userID, "qr_guest:") {
		guestID := strings.TrimPrefix(userID, "qr_guest:")
		guest, err := s.accessSvc.GetGuest(tenantID, guestID)
		if err != nil {
			resp := verifyCredentialResponse{
				Decision:       "deny",
				Reason:         "guest_not_found",
				LockID:         lockID,
				GatewayID:      gatewayID,
				CredentialType: credType,
				EvaluatedAt:    now.Format(time.RFC3339),
			}
			s.recordVerifyEvent(tenantID, gatewayID, lockID, resp)
			writeJSON(w, http.StatusOK, resp)
			return
		}
		// Check if guest has door-level access
		guestAllowed := len(guest.DoorIDs) == 0 // no restriction = building-wide
		for _, did := range guest.DoorIDs {
			if did == lockID {
				guestAllowed = true
				break
			}
		}
		if !guestAllowed {
			resp := verifyCredentialResponse{
				Decision:       "deny",
				Reason:         "no_access",
				UserID:         guestID,
				UserName:       guest.Name,
				LockID:         lockID,
				GatewayID:      gatewayID,
				CredentialType: credType,
				EvaluatedAt:    now.Format(time.RFC3339),
			}
			s.recordVerifyEvent(tenantID, gatewayID, lockID, resp)
			writeJSON(w, http.StatusOK, resp)
			return
		}
		resp := verifyCredentialResponse{
			Decision:       "allow",
			Reason:         "guest_qr_access",
			UserID:         guestID,
			UserName:       guest.Name,
			GroupName:      "visitor",
			LockID:         lockID,
			GatewayID:      gatewayID,
			CredentialType: credType,
			EvaluatedAt:    now.Format(time.RFC3339),
		}
		s.recordVerifyEvent(tenantID, gatewayID, lockID, resp)
		if gatewayID != "" && s.messageBus.Enabled() {
			cmd := bus.GatewayCommand{
				RequestID: fmt.Sprintf("verify:%s:%s:%d", lockID, guestID, now.UnixNano()),
				GatewayID: gatewayID,
				Command:   "unlock",
				LockID:    lockID,
				TenantID:  tenantID,
				IssuedBy:  guest.Name,
				IssuedAt:  now.Format(time.RFC3339),
			}
			subject := fmt.Sprintf("gateway.%s.command", gatewayID)
			if err := s.messageBus.PublishJSON(r.Context(), subject, cmd, nil); err != nil {
				s.logger.Warn("failed to dispatch auto-unlock after guest verify", "error", err)
			}
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Step 2b: Get user details (regular user flow)
	user, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil || user.Status != "active" {
		reason := "user_not_found"
		if err == nil && user.Status != "active" {
			reason = "user_suspended"
		}
		resp := verifyCredentialResponse{
			Decision:       "deny",
			Reason:         reason,
			UserID:         userID,
			LockID:         lockID,
			GatewayID:      gatewayID,
			CredentialType: credType,
			EvaluatedAt:    now.Format(time.RFC3339),
		}
		s.recordVerifyEvent(tenantID, gatewayID, lockID, resp)
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Step 3: Check if user has access to this lock via group → door_group binding
	allowed, groupName := s.checkUserLockAccess(tenantID, user.GroupIDs, lockID)

	// Step 4: If not found via groups, check role assignments for place-level access
	if !allowed {
		door, doorErr := s.spaceSvc.GetDoor(tenantID, lockID)
		if doorErr == nil {
			allowed = s.checkRoleAssignmentAccess(tenantID, userID, door.BuildingID)
			if allowed {
				groupName = "role_assignment"
			}
		}
	}

	if !allowed {
		resp := verifyCredentialResponse{
			Decision:       "deny",
			Reason:         "no_access",
			UserID:         user.ID,
			UserName:       user.Name,
			UserEmail:      user.Email,
			LockID:         lockID,
			GatewayID:      gatewayID,
			CredentialType: credType,
			EvaluatedAt:    now.Format(time.RFC3339),
		}
		s.recordVerifyEvent(tenantID, gatewayID, lockID, resp)
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Step 5: Schedule / time window check (simplified for MVP)
	// Full implementation would evaluate TimeWindows, HolidayCalendars, ExceptionDates
	// from the user's group restrictions. For MVP, allow if group access exists.

	resp := verifyCredentialResponse{
		Decision:       "allow",
		Reason:         "access_granted",
		UserID:         user.ID,
		UserName:       user.Name,
		UserEmail:      user.Email,
		GroupName:      groupName,
		LockID:         lockID,
		GatewayID:      gatewayID,
		CredentialType: credType,
		EvaluatedAt:    now.Format(time.RFC3339),
	}
	s.recordVerifyEvent(tenantID, gatewayID, lockID, resp)

	// If gateway is connected, auto-dispatch unlock
	if gatewayID != "" && s.messageBus.Enabled() {
		cmd := bus.GatewayCommand{
			RequestID: fmt.Sprintf("verify:%s:%s:%d", lockID, userID, now.UnixNano()),
			GatewayID: gatewayID,
			Command:   "unlock",
			LockID:    lockID,
			TenantID:  tenantID,
			IssuedBy:  user.Email,
			IssuedAt:  now.Format(time.RFC3339),
		}
		subject := fmt.Sprintf("gateway.%s.command", gatewayID)
		if err := s.messageBus.PublishJSON(r.Context(), subject, cmd, nil); err != nil {
			s.logger.Warn("failed to dispatch auto-unlock after verify", "error", err)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// resolveCredentialToUser maps a credential to a user ID.
func (s *server) resolveCredentialToUser(tenantID, credType, credData string) (string, bool) {
	now := time.Now().UTC()

	switch credType {
	case "nfc_uid":
		// Check physical card inventory by UID
		for _, item := range s.walletSvc.ListPhysicalCardInventory(tenantID, "") {
			if strings.EqualFold(item.UID, credData) && item.AssignedPassID != "" {
				pass, err := s.walletSvc.GetPass(tenantID, item.AssignedPassID)
				if err == nil && pass.TargetType == "user" && pass.Status == "active" && !isPassExpired(pass.ExpiresAt, now) {
					return pass.TargetID, true
				}
			}
		}
		// Also check passes directly by UID
		for _, pass := range s.walletSvc.ListPasses(tenantID) {
			if strings.EqualFold(pass.UID, credData) && pass.TargetType == "user" && pass.Status == "active" && !isPassExpired(pass.ExpiresAt, now) {
				return pass.TargetID, true
			}
		}

	case "card_number":
		for _, item := range s.walletSvc.ListPhysicalCardInventory(tenantID, "") {
			if strings.EqualFold(item.CardNumber, credData) && item.AssignedPassID != "" {
				pass, err := s.walletSvc.GetPass(tenantID, item.AssignedPassID)
				if err == nil && pass.TargetType == "user" && pass.Status == "active" && !isPassExpired(pass.ExpiresAt, now) {
					return pass.TargetID, true
				}
			}
		}

	case "ble_token":
		// Check passes by token
		for _, pass := range s.walletSvc.ListPasses(tenantID) {
			if pass.Token == credData && pass.TargetType == "user" && pass.Status == "active" && !isPassExpired(pass.ExpiresAt, now) {
				return pass.TargetID, true
			}
		}

	case "qr_code":
		// Check guest QR tokens first (Tier 3: dynamic visitor QR codes)
		if guest, ok := s.accessSvc.GetGuestByAccessToken(tenantID, credData); ok {
			return "qr_guest:" + guest.ID, true
		}
		// Fall back to group links by token
		for _, link := range s.accessSvc.ListGroupLinks(tenantID) {
			if link.Secret == credData || link.QuickResponseCodeToken == credData {
				if !link.LinkEnabled {
					return "", false
				}
				if isGroupLinkExpired(link.ValidUntil, now) {
					return "", false
				}
				return "qr_visitor:" + link.GroupID, true
			}
		}
	}

	return "", false
}

// isPassExpired returns true if the pass has an expiration time that has passed.
func isPassExpired(expiresAt string, now time.Time) bool {
	ea := strings.TrimSpace(expiresAt)
	if ea == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, ea)
	if err != nil {
		return false
	}
	return now.After(t)
}

// isGroupLinkExpired returns true if the group link has a valid_until time that has passed.
func isGroupLinkExpired(validUntil string, now time.Time) bool {
	vu := strings.TrimSpace(validUntil)
	if vu == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, vu)
	if err != nil {
		return false
	}
	return now.After(t)
}

// checkUserLockAccess checks if any of the user's groups grant access to the lock
// via explicit group_locks binding (user_group_id == door_group_id && door_group contains lock).
func (s *server) checkUserLockAccess(tenantID string, userGroupIDs []string, lockID string) (bool, string) {
	if len(userGroupIDs) == 0 {
		return false, ""
	}

	userGroupSet := make(map[string]struct{}, len(userGroupIDs))
	for _, ugID := range userGroupIDs {
		userGroupSet[ugID] = struct{}{}
	}

	// Check door groups: user's group ID must match the door group ID,
	// and that door group must explicitly contain the target lock.
	doorGroups := s.spaceSvc.ListDoorGroups(tenantID)
	for _, dg := range doorGroups {
		if _, bound := userGroupSet[dg.ID]; !bound {
			continue
		}
		for _, doorID := range dg.DoorIDs {
			if doorID == lockID {
				return true, dg.Name
			}
		}
	}

	return false, ""
}

// checkRoleAssignmentAccess checks if the user has a role assignment for the place.
func (s *server) checkRoleAssignmentAccess(tenantID, userID, placeID string) bool {
	assignments := s.accessSvc.ListRoleAssignments(tenantID)
	for _, ra := range assignments {
		if ra.AssigneeType != "User" || ra.AssigneeID != userID {
			continue
		}
		if ra.AppliesToType == "Organization" {
			return true
		}
		if ra.AppliesToType == "Place" && ra.AppliesToID == placeID {
			return true
		}
	}
	return false
}

func (s *server) recordVerifyEvent(tenantID, gatewayID, lockID string, resp verifyCredentialResponse) {
	if tenantID == "" {
		return
	}
	eventType := "access_denied"
	if resp.Decision == "allow" {
		eventType = "access_granted"
	}
	gwID := gatewayID
	if gwID == "" {
		gwID = "api_verify"
	}
	s.eventSvc.IngestAccessEvent(event.IngestAccessEventInput{
		TenantID:  tenantID,
		Type:      eventType,
		Actor:     resp.UserEmail,
		DoorID:    lockID,
		GatewayID: gwID,
		Result:    resp.Reason,
		At:        time.Now().UTC(),
	})
}
