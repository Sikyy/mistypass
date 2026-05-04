package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/credential"
)

// adminListMobileCredentials returns all mobile credentials for a tenant.
// GET /api/v1/credentials/mobile
func (s *server) adminListMobileCredentials(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		tenantID = user.TenantID
	}

	userID := r.URL.Query().Get("user_id")
	status := r.URL.Query().Get("status")

	var creds []credential.MobileCredential
	if userID != "" {
		creds = s.credentialSvc.ListUserCredentials(tenantID, userID)
	} else {
		creds = s.credentialSvc.ListActiveCredentials(tenantID)
	}

	// Filter by status if specified
	if status != "" && status != "active" {
		var filtered []credential.MobileCredential
		for _, c := range creds {
			if c.Status == status {
				filtered = append(filtered, c)
			}
		}
		creds = filtered
	}

	if creds == nil {
		creds = []credential.MobileCredential{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": creds,
		"total": len(creds),
	})
}

// adminGetMobileCredential returns a specific credential by ID.
// GET /api/v1/credentials/mobile/{credentialID}
func (s *server) adminGetMobileCredential(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	credentialID := chi.URLParam(r, "credentialID")
	cred, err := s.credentialSvc.GetCredential(user.TenantID, credentialID)
	if err != nil {
		writeError(w, http.StatusNotFound, "credential not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"credential": cred,
	})
}

// adminRevokeMobileCredential allows an admin to forcibly revoke a credential.
// POST /api/v1/credentials/mobile/{credentialID}/revoke
func (s *server) adminRevokeMobileCredential(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	credentialID := chi.URLParam(r, "credentialID")
	if err := s.credentialSvc.RevokeCredential(user.TenantID, credentialID); err != nil {
		if err.Error() == "credential not found" {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusConflict, err.Error())
		}
		return
	}

	s.auditSvc.Append(user.TenantID, user.Email, user.Role, "mobile_credential_revoked", credentialID, "admin_console")

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "revoked",
		"id":      credentialID,
		"message": "credential revoked by admin",
	})
}

// adminRevokeAllUserCredentials revokes all credentials for a user (employee termination).
// POST /api/v1/credentials/mobile/revoke-user
func (s *server) adminRevokeAllUserCredentials(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	count := s.credentialSvc.RevokeAllUserCredentials(user.TenantID, req.UserID)

	s.auditSvc.Append(user.TenantID, user.Email, user.Role, "mobile_credentials_revoked_all", req.UserID, "admin_console")

	writeJSON(w, http.StatusOK, map[string]any{
		"revoked_count": count,
		"user_id":       req.UserID,
	})
}
