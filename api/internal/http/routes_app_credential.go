package httpx

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/credential"
)

// appRegisterMobileCredential handles mobile device key registration.
// POST /api/v1/app/credentials/register
func (s *server) appRegisterMobileCredential(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		PublicKeyPEM         string   `json:"public_key_pem"`
		Platform             string   `json:"platform"`
		DeviceID             string   `json:"device_id"`
		DeviceModel          string   `json:"device_model"`
		KeystoreLevel        string   `json:"keystore_level"`
		AttestationCertChain []string `json:"attestation_cert_chain"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	input := credential.RegisterDeviceInput{
		TenantID:             user.TenantID,
		UserID:               user.ID,
		UserEmail:            user.Email,
		PublicKeyPEM:         req.PublicKeyPEM,
		Platform:             req.Platform,
		DeviceID:             req.DeviceID,
		DeviceModel:          req.DeviceModel,
		KeystoreLevel:        req.KeystoreLevel,
		AttestationCertChain: req.AttestationCertChain,
	}

	// Auto-detect keystore level from attestation if not explicitly provided
	if input.KeystoreLevel == "" && len(req.AttestationCertChain) > 0 {
		input.KeystoreLevel = credential.DetectKeystoreLevel(req.AttestationCertChain)
	}
	if input.KeystoreLevel == "" {
		input.KeystoreLevel = "software"
	}

	cred, err := s.credentialSvc.RegisterDevice(input)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "attestation") {
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"credential": cred,
	})
}

// appListMobileCredentials returns the current user's registered credentials.
// GET /api/v1/app/credentials/mobile
func (s *server) appListMobileCredentials(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	creds := s.credentialSvc.ListUserCredentials(user.TenantID, user.ID)
	if creds == nil {
		creds = []credential.MobileCredential{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": creds,
		"total": len(creds),
	})
}

// appRevokeMobileCredential allows a user to self-revoke a credential (lost phone).
// DELETE /api/v1/app/credentials/mobile/{credentialID}
func (s *server) appRevokeMobileCredential(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	credentialID := chi.URLParam(r, "credentialID")
	if credentialID == "" {
		writeError(w, http.StatusBadRequest, "credential_id is required")
		return
	}

	// Verify the credential belongs to this user
	cred, err := s.credentialSvc.GetCredential(user.TenantID, credentialID)
	if err != nil {
		writeError(w, http.StatusNotFound, "credential not found")
		return
	}
	if cred.UserID != user.ID {
		writeError(w, http.StatusForbidden, "not your credential")
		return
	}

	if err := s.credentialSvc.RevokeCredential(user.TenantID, credentialID); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "revoked",
		"id":      credentialID,
		"message": "credential revoked successfully",
	})
}

// appRefreshMobileCredential extends the TTL of an active credential.
// POST /api/v1/app/credentials/mobile/{credentialID}/refresh
func (s *server) appRefreshMobileCredential(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	credentialID := chi.URLParam(r, "credentialID")
	if credentialID == "" {
		writeError(w, http.StatusBadRequest, "credential_id is required")
		return
	}

	// Verify ownership
	cred, err := s.credentialSvc.GetCredential(user.TenantID, credentialID)
	if err != nil {
		writeError(w, http.StatusNotFound, "credential not found")
		return
	}
	if cred.UserID != user.ID {
		writeError(w, http.StatusForbidden, "not your credential")
		return
	}

	refreshed, err := s.credentialSvc.RefreshCredential(user.TenantID, credentialID)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"credential": refreshed,
	})
}
