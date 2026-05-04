package httpx

import (
	"net/http"
	"strings"

	"github.com/mistypass/cloud/api/internal/modules/integration"
)

// googleWorkspaceSyncUsers triggers a full employee directory sync from Google Workspace.
// POST /api/v1/integrations/google-workspace/sync
func (s *server) googleWorkspaceSyncUsers(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		ServiceAccountJSON string `json:"service_account_json"`
		DelegatedAdmin     string `json:"delegated_admin"`
		Domain             string `json:"domain"`
		TenantID           string `json:"tenant_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ServiceAccountJSON == "" || req.DelegatedAdmin == "" || req.Domain == "" {
		writeError(w, http.StatusBadRequest, "service_account_json, delegated_admin, and domain are required")
		return
	}

	// Resolve tenant: use request body if provided, otherwise fall back to auth user's tenant
	tenantID := strings.TrimSpace(req.TenantID)
	if tenantID == "" {
		tenantID = user.TenantID
	}
	if tenantID == "" {
		tenantID = "default"
	}

	// Create Google Workspace client
	client, err := integration.NewGoogleWorkspaceClient(integration.GoogleWorkspaceConfig{
		ServiceAccountJSON: []byte(req.ServiceAccountJSON),
		DelegatedAdmin:     req.DelegatedAdmin,
		Domain:             req.Domain,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"status": "failed",
			"error":  "google workspace auth: " + err.Error(),
		})
		return
	}

	// Fetch all users from Google Workspace
	gwsUsers, err := client.ListAllUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"status": "failed",
			"error":  "google workspace list users: " + err.Error(),
		})
		return
	}

	// Build existing email map from MistyPass access users
	existingUsers := s.accessSvc.ListUsers(tenantID)
	existingEmails := make(map[string]string, len(existingUsers))
	for _, u := range existingUsers {
		existingEmails[strings.ToLower(u.Email)] = u.ID
	}

	// Compute sync actions
	actions := integration.ComputeGWSyncActions(gwsUsers, existingEmails)

	// Apply sync actions
	var created, deactivated, skipped int
	var errors []string

	for _, action := range actions {
		switch action.Action {
		case "create":
			newUser, err := s.accessSvc.CreateUser(
				tenantID, "", action.LarkUser.Name, action.LarkUser.Email, "employee", "active", nil,
			)
			if err != nil {
				skipped++
				errors = append(errors, "create "+action.LarkUser.Email+": "+err.Error())
				s.logger.Warn("gws sync: create failed", "email", action.LarkUser.Email, "error", err)
				continue
			}
			created++
			s.auditSvc.Append(tenantID, "gws_sync", user.ID, "user_created_via_google_workspace", newUser.ID, action.LarkUser.Email)
			s.logger.Info("gws sync: user created", "user_id", newUser.ID, "email", action.LarkUser.Email)

		case "deactivate":
			userID, exists := existingEmails[strings.ToLower(action.LarkUser.Email)]
			if !exists {
				skipped++
				continue
			}

			// Revoke BLE credentials before deactivation
			if s.credentialSvc != nil {
				revoked := s.credentialSvc.RevokeAllUserCredentials(tenantID, userID)
				if revoked > 0 {
					s.logger.Info("gws sync: revoked BLE credentials", "user_id", userID, "count", revoked)
				}
			}

			existingUser, found := s.accessSvc.FindUserByEmail(tenantID, action.LarkUser.Email)
			if !found {
				skipped++
				continue
			}
			_, err := s.accessSvc.UpdateUser(
				tenantID, userID, existingUser.BuildingID,
				existingUser.Name, existingUser.Email, existingUser.Role, "suspended", existingUser.GroupIDs,
			)
			if err != nil {
				skipped++
				errors = append(errors, "deactivate "+action.LarkUser.Email+": "+err.Error())
				s.logger.Warn("gws sync: deactivate failed", "user_id", userID, "error", err)
				continue
			}
			deactivated++
			s.auditSvc.Append(tenantID, "gws_sync", user.ID, "user_suspended_via_google_workspace", userID, action.LarkUser.Email)
			s.logger.Info("gws sync: user suspended", "user_id", userID, "email", action.LarkUser.Email)

		default:
			skipped++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "success",
		"gws_users":   len(gwsUsers),
		"actions":     len(actions),
		"created":     created,
		"deactivated": deactivated,
		"skipped":     skipped,
		"errors":      errors,
	})
}
