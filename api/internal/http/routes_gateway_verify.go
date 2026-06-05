package httpx

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/bus"
	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/event"
)

type verifyCredentialRequest struct {
	GatewayID      string `json:"gateway_id"`
	ReaderID       string `json:"reader_id"`
	LockID         string `json:"lock_id"`
	TenantID       string `json:"tenant_id"`
	CredentialType string `json:"credential_type"` // nfc_uid, ble_token, card_number, qr_code, ble_signature
	CredentialData string `json:"credential_data"`
	// ble_signature fields: a transport-bound ECDSA signature from a mobile
	// credential over SHA256(requestNonce || user_id || transport_tag).
	UserID       string `json:"user_id,omitempty"`
	TransportTag string `json:"transport_tag,omitempty"` // "BLE" (default) or "NFC_HCE"
	Signature    string `json:"signature,omitempty"`     // base64-encoded ECDSA signature
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
	s.evaluateVerifyCredential(w, r, req)
}

// verifyCredentialGateway authenticates the calling gateway device before
// evaluating a credential. This route is reachable on the public (non-mTLS)
// listener, so it must establish gateway device identity (mTLS client
// certificate or device token) and derive the tenant from the authenticated
// gateway record rather than trusting the request body.
func (s *server) verifyCredentialGateway(w http.ResponseWriter, r *http.Request) {
	var req verifyCredentialRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	gatewayID := strings.TrimSpace(req.GatewayID)
	tenantID := strings.TrimSpace(req.TenantID)
	if gatewayID == "" || tenantID == "" {
		writeError(w, http.StatusUnauthorized, "gateway authentication required")
		return
	}
	record, ok := s.findGatewayByTenant(tenantID, gatewayID)
	if !ok {
		writeError(w, http.StatusUnauthorized, "gateway authentication required")
		return
	}
	if !s.authorizeGatewayHTTPDeviceRequest(w, r, record.ID) {
		return
	}
	// The verify endpoint dispatches unlocks, so it always requires a fresh
	// single-use request nonce (validated and de-duplicated above) even though
	// other gateway telemetry endpoints treat the nonce as optional.
	if strings.TrimSpace(r.Header.Get("X-Request-Nonce")) == "" {
		writeError(w, http.StatusUnauthorized, "request nonce required")
		return
	}
	req.GatewayID = record.ID
	req.TenantID = record.TenantID
	s.evaluateVerifyCredential(w, r, req)
}

func (s *server) evaluateVerifyCredential(w http.ResponseWriter, r *http.Request, req verifyCredentialRequest) {
	credType := strings.TrimSpace(req.CredentialType)
	credData := strings.TrimSpace(req.CredentialData)
	lockID := strings.TrimSpace(req.LockID)
	tenantID := strings.TrimSpace(req.TenantID)
	gatewayID := strings.TrimSpace(req.GatewayID)
	now := time.Now().UTC()

	if credData == "" && credType != "ble_signature" {
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

	// Guest QR credentials resolve to a guest (not a user) and are gated by the
	// guest's door_ids (an empty list = building-wide within the guest's building).
	if credType == "qr_code" {
		if guest, ok := s.accessSvc.GetGuestByAccessToken(tenantID, credData); ok {
			resp := verifyCredentialResponse{
				UserID:         "guest:" + guest.ID,
				UserName:       guest.Name,
				LockID:         lockID,
				GatewayID:      gatewayID,
				CredentialType: credType,
				EvaluatedAt:    now.Format(time.RFC3339),
			}
			if s.guestCanAccessDoor(tenantID, guest, lockID) {
				resp.Decision = "allow"
				resp.Reason = "access_granted"
			} else {
				resp.Decision = "deny"
				resp.Reason = "door_not_in_guest_access"
			}
			eventID := s.recordVerifyEvent(tenantID, gatewayID, lockID, resp)
			if resp.Decision == "allow" {
				s.onAccessGranted(r.Context(), tenantID, gatewayID, lockID, eventID, guest.ID, guest.Name, now)
			}
			writeJSON(w, http.StatusOK, resp)
			return
		}
	}

	// Step 1: Resolve credential → user_id
	var userID string
	var credentialFound bool
	if credType == "ble_signature" {
		userID, credentialFound = s.resolveBLESignatureToUser(r, tenantID, req)
	} else {
		userID, credentialFound = s.resolveCredentialToUser(tenantID, credType, credData)
	}
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

	// Step 2: Get user details
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

	// Step 2.5: Spatiotemporal anomaly detection for NFC credentials
	if credType == "nfc_uid" {
		if reason := s.checkCredentialAnomaly(tenantID, credData, lockID, now); reason != "" {
			resp := verifyCredentialResponse{
				Decision:       "deny",
				Reason:         reason,
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
	}

	// Step 3: Check if user has access to this lock via group → door_group binding
	allowed, groupName := s.checkUserLockAccess(tenantID, user.GroupIDs, lockID)

	// Step 4: If not found via groups, check role assignments for place-level access
	var accessPlaceID string
	if !allowed {
		door, doorErr := s.spaceSvc.GetDoor(tenantID, lockID)
		if doorErr == nil {
			accessPlaceID = door.BuildingID
			allowed = s.checkRoleAssignmentAccess(tenantID, userID, accessPlaceID)
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

	// Step 5: Schedule enforcement — only applies to role-assignment-based access
	if groupName == "role_assignment" {
		if reason := s.evaluateAccessSchedule(tenantID, userID, accessPlaceID, now); reason != "" {
			resp := verifyCredentialResponse{
				Decision:       "deny",
				Reason:         reason,
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
			writeJSON(w, http.StatusOK, resp)
			return
		}
	}

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
	eventID := s.recordVerifyEvent(tenantID, gatewayID, lockID, resp)
	s.onAccessGranted(r.Context(), tenantID, gatewayID, lockID, eventID, userID, user.Email, now)

	writeJSON(w, http.StatusOK, resp)
}

// onAccessGranted fires the post-allow side effects shared by the user and guest
// verify paths: camera snapshots (non-blocking) + gateway auto-unlock dispatch.
func (s *server) onAccessGranted(ctx context.Context, tenantID, gatewayID, lockID, eventID, subjectID, issuedBy string, now time.Time) {
	if eventID != "" && lockID != "" {
		go func() {
			snapCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			snaps := s.cameraSvc.TriggerEventSnapshot(snapCtx, tenantID, lockID, eventID, "unlock")
			if len(snaps) > 0 {
				s.logger.Info("event snapshots captured", "event_id", eventID, "door_id", lockID, "count", len(snaps))
			}
		}()
	}

	if gatewayID != "" && s.messageBus.Enabled() {
		cmd := bus.GatewayCommand{
			RequestID: fmt.Sprintf("verify:%s:%s:%d", lockID, subjectID, now.UnixNano()),
			GatewayID: gatewayID,
			Command:   "unlock",
			LockID:    lockID,
			TenantID:  tenantID,
			IssuedBy:  issuedBy,
			IssuedAt:  now.Format(time.RFC3339),
		}
		subject := fmt.Sprintf("gateway.%s.command", gatewayID)
		if err := s.messageBus.PublishJSON(ctx, subject, cmd, nil); err != nil {
			s.logger.Warn("failed to dispatch auto-unlock after verify", "error", err)
		}
	}
}

// resolveBLESignatureToUser verifies a transport-bound ECDSA signature produced
// by a mobile credential. The signed challenge is the single-use request nonce
// (X-Request-Nonce), so the gateway verify path's nonce de-duplication prevents
// a captured signature from being replayed under a fresh nonce. Returns the
// authenticated user ID on success.
func (s *server) resolveBLESignatureToUser(r *http.Request, tenantID string, req verifyCredentialRequest) (string, bool) {
	if s.credentialSvc == nil {
		return "", false
	}
	userID := strings.TrimSpace(req.UserID)
	transportTag := strings.TrimSpace(req.TransportTag)
	if transportTag == "" {
		transportTag = "BLE"
	}
	nonce := strings.TrimSpace(r.Header.Get("X-Request-Nonce"))
	signatureB64 := strings.TrimSpace(req.Signature)
	if userID == "" || nonce == "" || signatureB64 == "" {
		return "", false
	}
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return "", false
	}
	cred, err := s.credentialSvc.VerifyBLESignatureV2(tenantID, userID, []byte(nonce), transportTag, signature)
	if err != nil || cred == nil {
		return "", false
	}
	return userID, true
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
		// Check group links by token
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

// guestCanAccessDoor enforces a guest's door_ids: an explicit list restricts access
// to exactly those locks; an empty list grants building-wide access within the
// guest's building (or anywhere if no building is recorded on the guest).
func (s *server) guestCanAccessDoor(tenantID string, guest access.Guest, lockID string) bool {
	if len(guest.DoorIDs) > 0 {
		for _, d := range guest.DoorIDs {
			if strings.EqualFold(strings.TrimSpace(d), lockID) {
				return true
			}
		}
		return false
	}
	building := strings.TrimSpace(guest.BuildingID)
	if building == "" {
		return true
	}
	door, err := s.spaceSvc.GetDoor(tenantID, lockID)
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(door.BuildingID), building)
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

// checkCredentialAnomaly detects impossible travel (same credential at different doors
// within a short window) and rapid-fire taps (rate limiting).
func (s *server) checkCredentialAnomaly(tenantID, credData, lockID string, now time.Time) string {
	key := tenantID + ":" + credData

	s.credLastSeenMu.Lock()
	defer s.credLastSeenMu.Unlock()

	prev, exists := s.credLastSeen[key]

	if exists {
		elapsed := now.Sub(prev.SeenAt)

		// Different door within 30 seconds = impossible travel
		if prev.LockID != lockID && elapsed < 30*time.Second {
			s.logger.Warn("anomaly: impossible travel",
				"credential", credData,
				"prev_door", prev.LockID,
				"curr_door", lockID,
				"elapsed_sec", elapsed.Seconds(),
			)
			return "anomaly_impossible_travel"
		}

		// Same door >5 taps in 60 seconds = rate limit
		if prev.LockID == lockID && elapsed < 60*time.Second {
			prev.Count++
			if prev.Count > 5 {
				s.credLastSeen[key] = prev
				s.logger.Warn("anomaly: rate limit exceeded",
					"credential", credData,
					"door", lockID,
					"count", prev.Count,
				)
				return "anomaly_rate_limit"
			}
			prev.SeenAt = now
			s.credLastSeen[key] = prev
			return ""
		}
	}

	s.credLastSeen[key] = credentialSighting{
		LockID: lockID,
		SeenAt: now,
		Count:  1,
	}

	// Evict stale entries (older than 2 minutes) to prevent unbounded growth.
	if len(s.credLastSeen) > 500 {
		cutoff := now.Add(-2 * time.Minute)
		for k, v := range s.credLastSeen {
			if v.SeenAt.Before(cutoff) {
				delete(s.credLastSeen, k)
			}
		}
	}

	return ""
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
		s.logger.Info("checkUserLockAccess: matched group", "group_id", dg.ID, "door_ids", dg.DoorIDs, "looking_for", lockID)
		for _, doorID := range dg.DoorIDs {
			if doorID == lockID {
				return true, dg.Name
			}
		}
	}

	s.logger.Info("checkUserLockAccess: no match", "tenant", tenantID, "groups_checked", len(doorGroups), "lock_id", lockID)
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

func (s *server) evaluateAccessSchedule(tenantID, userID, placeID string, now time.Time) string {
	assignments := s.accessSvc.ListRoleAssignments(tenantID)
	for _, ra := range assignments {
		if ra.AssigneeType != "User" || ra.AssigneeID != userID {
			continue
		}
		if placeID != "" && ra.AppliesToID != placeID {
			continue
		}
		hasTimeRestrictions := ra.ValidFrom != "" || ra.ValidUntil != "" || len(ra.TimeWindows) > 0 || len(ra.ExceptionDates) > 0

		if !hasTimeRestrictions {
			continue
		}

		var holidays []access.HolidayEntry
		if ra.HolidayCalendarID != "" {
			if cal, err := s.accessSvc.GetHolidayCalendar(tenantID, ra.HolidayCalendarID); err == nil {
				holidays = cal.Entries
			}
		}

		eval := access.EvaluateSchedule(now, ra.ValidFrom, ra.ValidUntil, ra.TimeWindows, ra.ExceptionDates, holidays)
		if !eval.IsActive {
			return eval.Reason
		}
	}
	return ""
}

func (s *server) recordVerifyEvent(tenantID, gatewayID, lockID string, resp verifyCredentialResponse) string {
	if tenantID == "" {
		return ""
	}
	eventType := "access_denied"
	if resp.Decision == "allow" {
		eventType = "access_granted"
	}
	gwID := gatewayID
	if gwID == "" {
		gwID = "api_verify"
	}
	ev, _, _ := s.eventSvc.IngestAccessEvent(event.IngestAccessEventInput{
		TenantID:  tenantID,
		Type:      eventType,
		Actor:     resp.UserEmail,
		DoorID:    lockID,
		GatewayID: gwID,
		Result:    resp.Reason,
		At:        time.Now().UTC(),
	})
	return ev.ID
}
