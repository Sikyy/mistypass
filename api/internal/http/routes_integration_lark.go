package httpx

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mistypass/cloud/api/internal/modules/integration"
)

// --- Lark Event Subscription Callback ---

// larkEventCallback handles incoming events from Lark Event Subscription.
// POST /api/v1/integrations/lark/events
//
// Lark sends two types of requests:
// 1. Challenge verification (during subscription setup)
// 2. Event notifications (employee changes, etc.)
func (s *server) larkEventCallback(w http.ResponseWriter, r *http.Request) {
	// Limit body to 1 MB to prevent OOM
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}

	// Verify Lark event signature using Verification Token
	verificationToken := s.cfg.LarkVerificationToken
	if verificationToken != "" {
		// Extract token from event header or challenge body for validation
		var header struct {
			Token string `json:"token"`
		}
		_ = json.Unmarshal(body, &header)
		if header.Token != verificationToken {
			// Also check X-Lark-Signature header (v2 events)
			sig := r.Header.Get("X-Lark-Signature")
			if sig != "" {
				timestamp := r.Header.Get("X-Lark-Request-Timestamp")
				nonce := r.Header.Get("X-Lark-Request-Nonce")
				expected := fmt.Sprintf("%x", sha256.Sum256([]byte(timestamp+nonce+verificationToken+string(body))))
				if sig != expected {
					writeError(w, http.StatusUnauthorized, "invalid event signature")
					return
				}
			} else if header.Token == "" {
				writeError(w, http.StatusUnauthorized, "missing verification token")
				return
			} else {
				writeError(w, http.StatusUnauthorized, "invalid verification token")
				return
			}
		}
	}

	// Check if this is a challenge verification request
	var challenge struct {
		Challenge string `json:"challenge"`
		Token     string `json:"token"`
		Type      string `json:"type"`
	}
	if err := json.Unmarshal(body, &challenge); err == nil && challenge.Type == "url_verification" {
		// Respond with the challenge to verify callback URL
		writeJSON(w, http.StatusOK, map[string]string{
			"challenge": challenge.Challenge,
		})
		return
	}

	// Parse as event
	var event integration.LarkEvent
	if err := json.Unmarshal(body, &event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid event: "+err.Error())
		return
	}

	// Process event based on type
	switch event.Header.EventType {
	case "contact.user.created_v3":
		s.handleLarkUserCreated(event)
	case "contact.user.deleted_v3":
		s.handleLarkUserDeleted(event)
	case "contact.user.updated_v3":
		s.handleLarkUserUpdated(event)
	default:
		s.logger.Info("lark event received (unhandled)",
			"event_type", event.Header.EventType,
			"event_id", event.Header.EventID,
		)
	}

	// Always respond 200 to acknowledge receipt (Lark retries on non-200)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleLarkUserCreated(ev integration.LarkEvent) {
	var payload integration.LarkUserCreatedEvent
	if err := json.Unmarshal(ev.Event, &payload); err != nil {
		s.logger.Warn("lark user created: decode failed", "error", err)
		return
	}

	user := payload.Object
	s.logger.Info("lark employee onboarded",
		"name", user.Name,
		"email", user.Email,
		"user_id", user.UserID,
	)

	if user.Email == "" || !user.Status.IsActivated {
		return
	}

	// Check if user already exists (by email, across all tenants)
	if _, found := s.accessSvc.FindUserByEmail("", user.Email); found {
		s.logger.Info("lark user already exists in platform, skipping create", "email", user.Email)
		return
	}

	// Use Lark tenant_key as a hint; for MVP create under first available tenant
	// In production this would use a lark_app_id → tenant_id mapping table
	tenantID := ev.Header.TenantKey
	if tenantID == "" {
		tenantID = "default"
	}

	created, err := s.accessSvc.CreateUser(tenantID, "", user.Name, user.Email, "employee", "active", nil)
	if err != nil {
		s.logger.Warn("lark user auto-create failed", "email", user.Email, "error", err)
		return
	}
	s.auditSvc.Append(tenantID, "lark_sync", "system", "user_created_via_lark", created.ID, user.Email)
	s.logger.Info("lark user auto-created", "user_id", created.ID, "email", user.Email)
}

func (s *server) handleLarkUserDeleted(ev integration.LarkEvent) {
	var payload integration.LarkUserDeletedEvent
	if err := json.Unmarshal(ev.Event, &payload); err != nil {
		s.logger.Warn("lark user deleted: decode failed", "error", err)
		return
	}

	email := payload.Object.Email
	s.logger.Info("lark employee offboarded",
		"name", payload.Object.Name,
		"email", email,
	)

	if email == "" {
		return
	}

	// Find user by email
	user, found := s.accessSvc.FindUserByEmail("", email)
	if !found {
		s.logger.Info("lark deleted user not found in platform", "email", email)
		return
	}

	// Revoke all BLE mobile credentials
	if s.credentialSvc != nil {
		revoked := s.credentialSvc.RevokeAllUserCredentials(user.TenantID, user.ID)
		if revoked > 0 {
			s.logger.Info("lark offboard: revoked BLE credentials", "user_id", user.ID, "count", revoked)
		}
	}

	// Deactivate the user (set status to suspended)
	_, err := s.accessSvc.UpdateUser(user.TenantID, user.ID, user.BuildingID, user.Name, user.Email, user.Role, "suspended", user.GroupIDs)
	if err != nil {
		s.logger.Warn("lark offboard: failed to suspend user", "user_id", user.ID, "error", err)
		return
	}
	s.auditSvc.Append(user.TenantID, "lark_sync", "system", "user_suspended_via_lark", user.ID, email)
	s.logger.Info("lark offboard: user suspended", "user_id", user.ID, "email", email)
}

func (s *server) handleLarkUserUpdated(ev integration.LarkEvent) {
	var payload integration.LarkUserCreatedEvent // same shape as created_v3
	if err := json.Unmarshal(ev.Event, &payload); err != nil {
		s.logger.Info("lark user updated: decode failed", "event_id", ev.Header.EventID)
		return
	}

	larkUser := payload.Object
	if larkUser.Email == "" {
		return
	}

	user, found := s.accessSvc.FindUserByEmail("", larkUser.Email)
	if !found {
		s.logger.Info("lark updated user not found in platform", "email", larkUser.Email)
		return
	}

	// Sync name change
	newName := larkUser.Name
	if newName == "" {
		newName = user.Name
	}

	// Handle resignation/freeze from Lark
	newStatus := user.Status
	if larkUser.Status.IsResigned || larkUser.Status.IsFrozen {
		newStatus = "suspended"
		// Also revoke credentials on resignation
		if s.credentialSvc != nil {
			s.credentialSvc.RevokeAllUserCredentials(user.TenantID, user.ID)
		}
	} else if larkUser.Status.IsActivated && user.Status == "suspended" {
		newStatus = "active"
	}

	if newName != user.Name || newStatus != user.Status {
		_, err := s.accessSvc.UpdateUser(user.TenantID, user.ID, user.BuildingID, newName, user.Email, user.Role, newStatus, user.GroupIDs)
		if err != nil {
			s.logger.Warn("lark user update failed", "user_id", user.ID, "error", err)
			return
		}
		s.auditSvc.Append(user.TenantID, "lark_sync", "system", "user_updated_via_lark", user.ID, larkUser.Email)
		s.logger.Info("lark user synced", "user_id", user.ID, "name", newName, "status", newStatus)
	}
}

// --- Lark Bot Test Endpoint ---

// larkBotTest sends a test message to verify webhook configuration.
// POST /api/v1/integrations/lark/bot/test
func (s *server) larkBotTest(w http.ResponseWriter, r *http.Request) {
	_, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		WebhookURL string `json:"webhook_url"`
		Message    string `json:"message"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.WebhookURL == "" {
		writeError(w, http.StatusBadRequest, "webhook_url is required")
		return
	}
	if req.Message == "" {
		req.Message = "MistyPass Lark integration test - connection successful!"
	}

	sender := integration.NewLarkBotSender()
	if err := sender.SendText(r.Context(), req.WebhookURL, req.Message); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"status": "failed",
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "sent",
		"message": "Test message delivered to Lark group",
	})
}

// larkBotSendAlert sends an access alert to a configured Lark group.
// POST /api/v1/integrations/lark/bot/alert
func (s *server) larkBotSendAlert(w http.ResponseWriter, r *http.Request) {
	_, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		WebhookURL string `json:"webhook_url"`
		EventType  string `json:"event_type"`
		DoorName   string `json:"door_name"`
		UserName   string `json:"user_name"`
		Reason     string `json:"reason"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.WebhookURL == "" || req.EventType == "" {
		writeError(w, http.StatusBadRequest, "webhook_url and event_type are required")
		return
	}

	sender := integration.NewLarkBotSender()
	alert := integration.AccessAlertMessage{
		EventType: req.EventType,
		DoorName:  req.DoorName,
		UserName:  req.UserName,
		Reason:    req.Reason,
		Time:      "now",
	}
	if err := sender.SendAccessAlert(r.Context(), req.WebhookURL, alert); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"status": "failed",
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "sent"})
}

// larkSyncUsers triggers a full employee directory sync from Lark.
// POST /api/v1/integrations/lark/sync
func (s *server) larkSyncUsers(w http.ResponseWriter, r *http.Request) {
	_, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		AppID     string `json:"app_id"`
		AppSecret string `json:"app_secret"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.AppID == "" || req.AppSecret == "" {
		writeError(w, http.StatusBadRequest, "app_id and app_secret are required")
		return
	}

	client := integration.NewLarkClient(integration.LarkConfig{
		AppID:     req.AppID,
		AppSecret: req.AppSecret,
	})
	syncSvc := integration.NewLarkContactSync(client)

	users, err := syncSvc.ListAllUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"status": "failed",
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "success",
		"user_count": len(users),
		"users":      users,
	})
}
