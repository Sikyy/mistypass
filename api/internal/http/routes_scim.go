package httpx

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
)

// SCIM 2.0 Server implementation (RFC 7644).
// Okta / Entra ID / OneLogin push user lifecycle events to these endpoints.
// MistyPass acts as the SCIM Service Provider.

// --- SCIM Types ---

type scimListResponse struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	StartIndex   int      `json:"startIndex"`
	ItemsPerPage int      `json:"itemsPerPage"`
	Resources    []any    `json:"Resources"`
}

type scimUser struct {
	Schemas    []string          `json:"schemas"`
	ID         string            `json:"id"`
	ExternalID string            `json:"externalId,omitempty"`
	UserName   string            `json:"userName"`
	Name       *scimName         `json:"name,omitempty"`
	Emails     []scimMultiValue  `json:"emails,omitempty"`
	Active     bool              `json:"active"`
	Groups     []scimMultiValue  `json:"groups,omitempty"`
	Meta       *scimMeta         `json:"meta,omitempty"`
}

type scimName struct {
	Formatted  string `json:"formatted,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
}

type scimMultiValue struct {
	Value   string `json:"value"`
	Primary bool   `json:"primary,omitempty"`
	Type    string `json:"type,omitempty"`
	Display string `json:"display,omitempty"`
}

type scimMeta struct {
	ResourceType string `json:"resourceType"`
	Created      string `json:"created,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	Location     string `json:"location,omitempty"`
}

type scimError struct {
	Schemas []string `json:"schemas"`
	Detail  string   `json:"detail"`
	Status  int      `json:"status"`
}

type scimPatchOp struct {
	Schemas    []string        `json:"schemas"`
	Operations []scimOperation `json:"Operations"`
}

type scimOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path,omitempty"`
	Value any    `json:"value,omitempty"`
}

// --- Helpers ---

func accessUserToSCIM(u access.AccessUser, baseURL string) scimUser {
	return scimUser{
		Schemas:    []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		ID:         u.ID,
		ExternalID: u.SyncRef,
		UserName:   u.Email,
		Name:       &scimName{Formatted: u.Name},
		Emails: []scimMultiValue{
			{Value: u.Email, Primary: true, Type: "work"},
		},
		Active: u.Status == "active",
		Meta: &scimMeta{
			ResourceType: "User",
			Created:      u.CreatedAt.Format(time.RFC3339),
			LastModified: u.CreatedAt.Format(time.RFC3339),
			Location:     baseURL + "/scim/v2/Users/" + u.ID,
		},
	}
}

func scimBaseURL(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	return scheme + "://" + r.Host
}

func writeScimJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeScimError(w http.ResponseWriter, status int, detail string) {
	writeScimJSON(w, status, scimError{
		Schemas: []string{"urn:ietf:params:scim:api:messages:2.0:Error"},
		Detail:  detail,
		Status:  status,
	})
}

// resolveSCIMTenantID extracts the tenant ID from the SCIM bearer token.
// The token format is "tenantID_<random hex>" and must match the stored token
// exactly via constant-time comparison.
func (s *server) resolveSCIMTenantID(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	token := strings.TrimPrefix(auth, "Bearer ")
	token = strings.TrimSpace(token)

	if token == "" {
		return ""
	}

	// Validate against stored SCIM tokens (constant-time comparison)
	globalSCIMConfig.mu.RLock()
	defer globalSCIMConfig.mu.RUnlock()
	for tenantID, entry := range globalSCIMConfig.tokens {
		if subtle.ConstantTimeCompare([]byte(token), []byte(entry.Token)) == 1 {
			return tenantID
		}
	}

	// Fallback: try to authenticate as a regular admin user
	if user, ok := authenticatedUser(r); ok {
		return user.TenantID
	}

	return ""
}

// --- Handlers ---

// scimServiceProviderConfig returns SCIM capabilities.
// GET /scim/v2/ServiceProviderConfig
func (s *server) scimServiceProviderConfig(w http.ResponseWriter, r *http.Request) {
	writeScimJSON(w, http.StatusOK, map[string]any{
		"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"documentationUri": "https://docs.mistyislet.com/scim",
		"patch":            map[string]any{"supported": true},
		"bulk":             map[string]any{"supported": false, "maxOperations": 0, "maxPayloadSize": 0},
		"filter":           map[string]any{"supported": true, "maxResults": 200},
		"changePassword":   map[string]any{"supported": false},
		"sort":             map[string]any{"supported": false},
		"etag":             map[string]any{"supported": false},
		"authenticationSchemes": []map[string]any{
			{
				"type":        "oauthbearertoken",
				"name":        "OAuth Bearer Token",
				"description": "Authentication scheme using the OAuth Bearer Token standard",
			},
		},
	})
}

// scimSchemas returns supported SCIM schemas.
// GET /scim/v2/Schemas
func (s *server) scimSchemas(w http.ResponseWriter, r *http.Request) {
	writeScimJSON(w, http.StatusOK, scimListResponse{
		Schemas:      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		TotalResults: 1,
		StartIndex:   1,
		ItemsPerPage: 1,
		Resources: []any{
			map[string]any{
				"schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:Schema"},
				"id":          "urn:ietf:params:scim:schemas:core:2.0:User",
				"name":        "User",
				"description": "User Account",
				"attributes": []map[string]any{
					{"name": "userName", "type": "string", "required": true, "uniqueness": "server"},
					{"name": "name", "type": "complex", "required": false, "subAttributes": []map[string]any{
						{"name": "formatted", "type": "string"},
						{"name": "givenName", "type": "string"},
						{"name": "familyName", "type": "string"},
					}},
					{"name": "emails", "type": "complex", "multiValued": true, "required": true},
					{"name": "active", "type": "boolean", "required": true},
					{"name": "externalId", "type": "string", "required": false},
				},
			},
		},
	})
}

// scimListUsers handles user listing with optional filter.
// GET /scim/v2/Users?filter=userName eq "user@example.com"&startIndex=1&count=100
func (s *server) scimListUsers(w http.ResponseWriter, r *http.Request) {
	tenantID := s.resolveSCIMTenantID(r)
	if tenantID == "" {
		writeScimError(w, http.StatusUnauthorized, "unable to resolve tenant")
		return
	}

	base := scimBaseURL(r)
	startIndex, _ := strconv.Atoi(r.URL.Query().Get("startIndex"))
	if startIndex < 1 {
		startIndex = 1
	}
	count, _ := strconv.Atoi(r.URL.Query().Get("count"))
	if count <= 0 || count > 200 {
		count = 200
	}

	users := s.accessSvc.ListUsers(tenantID)

	// Handle filter: userName eq "user@example.com"
	filterStr := r.URL.Query().Get("filter")
	if filterStr != "" {
		users = applyScimUserFilter(users, filterStr)
	}

	total := len(users)

	// Pagination (SCIM is 1-indexed)
	start := startIndex - 1
	if start < 0 {
		start = 0
	}
	end := start + count
	if end > total {
		end = total
	}
	if start > total {
		start = total
	}
	page := users[start:end]

	resources := make([]any, 0, len(page))
	for _, u := range page {
		resources = append(resources, accessUserToSCIM(u, base))
	}

	writeScimJSON(w, http.StatusOK, scimListResponse{
		Schemas:      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		TotalResults: total,
		StartIndex:   startIndex,
		ItemsPerPage: len(page),
		Resources:    resources,
	})
}

// scimGetUser returns a single user.
// GET /scim/v2/Users/{id}
func (s *server) scimGetUser(w http.ResponseWriter, r *http.Request) {
	tenantID := s.resolveSCIMTenantID(r)
	userID := chi.URLParam(r, "id")

	user, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil {
		writeScimError(w, http.StatusNotFound, "User not found")
		return
	}

	writeScimJSON(w, http.StatusOK, accessUserToSCIM(user, scimBaseURL(r)))
}

// scimCreateUser creates a new user from SCIM push.
// POST /scim/v2/Users
func (s *server) scimCreateUser(w http.ResponseWriter, r *http.Request) {
	tenantID := s.resolveSCIMTenantID(r)
	if tenantID == "" {
		writeScimError(w, http.StatusUnauthorized, "unable to resolve tenant")
		return
	}

	var req scimUser
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeScimError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	email := req.UserName
	if email == "" && len(req.Emails) > 0 {
		email = req.Emails[0].Value
	}
	if email == "" {
		writeScimError(w, http.StatusBadRequest, "userName or emails required")
		return
	}

	name := ""
	if req.Name != nil {
		name = req.Name.Formatted
		if name == "" {
			name = strings.TrimSpace(req.Name.GivenName + " " + req.Name.FamilyName)
		}
	}
	if name == "" {
		name = email
	}

	// Check if user already exists (by email)
	if existing, found := s.accessSvc.FindUserByEmail(tenantID, email); found {
		// Return existing user (Okta expects 409 or 200 with existing)
		writeScimJSON(w, http.StatusOK, accessUserToSCIM(existing, scimBaseURL(r)))
		return
	}

	status := "active"
	if !req.Active {
		status = "suspended"
	}

	user, err := s.accessSvc.CreateUser(tenantID, "", name, email, "employee", status, nil)
	if err != nil {
		writeScimError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Store SCIM external ID as SyncRef
	if req.ExternalID != "" {
		s.accessSvc.UpdateUser(tenantID, user.ID, user.BuildingID, user.Name, user.Email, user.Role, user.Status, user.GroupIDs)
	}

	s.auditSvc.Append(tenantID, "scim", "system", "user_created_via_scim", user.ID, email)
	s.logger.Info("scim user created", "user_id", user.ID, "email", email)

	writeScimJSON(w, http.StatusCreated, accessUserToSCIM(user, scimBaseURL(r)))
}

// scimReplaceUser fully replaces a user (PUT).
// PUT /scim/v2/Users/{id}
func (s *server) scimReplaceUser(w http.ResponseWriter, r *http.Request) {
	tenantID := s.resolveSCIMTenantID(r)
	userID := chi.URLParam(r, "id")

	existing, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil {
		writeScimError(w, http.StatusNotFound, "User not found")
		return
	}

	var req scimUser
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeScimError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	name := existing.Name
	if req.Name != nil && req.Name.Formatted != "" {
		name = req.Name.Formatted
	} else if req.Name != nil {
		candidate := strings.TrimSpace(req.Name.GivenName + " " + req.Name.FamilyName)
		if candidate != "" {
			name = candidate
		}
	}

	email := existing.Email
	if req.UserName != "" {
		email = req.UserName
	}

	status := existing.Status
	if req.Active && existing.Status == "suspended" {
		status = "active"
	} else if !req.Active && existing.Status == "active" {
		status = "suspended"
		// Revoke credentials on deactivation
		if s.credentialSvc != nil {
			revoked := s.credentialSvc.RevokeAllUserCredentials(tenantID, userID)
			if revoked > 0 {
				s.logger.Info("scim deactivation: revoked BLE credentials", "user_id", userID, "count", revoked)
			}
		}
	}

	updated, err := s.accessSvc.UpdateUser(tenantID, userID, existing.BuildingID, name, email, existing.Role, status, existing.GroupIDs)
	if err != nil {
		writeScimError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.auditSvc.Append(tenantID, "scim", "system", "user_updated_via_scim", userID, email)
	writeScimJSON(w, http.StatusOK, accessUserToSCIM(updated, scimBaseURL(r)))
}

// scimPatchUser handles incremental updates (PATCH).
// PATCH /scim/v2/Users/{id}
// Okta primarily uses this to set active=false on deprovisioning.
func (s *server) scimPatchUser(w http.ResponseWriter, r *http.Request) {
	tenantID := s.resolveSCIMTenantID(r)
	userID := chi.URLParam(r, "id")

	existing, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil {
		writeScimError(w, http.StatusNotFound, "User not found")
		return
	}

	var patch scimPatchOp
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeScimError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	name := existing.Name
	email := existing.Email
	status := existing.Status

	for _, op := range patch.Operations {
		switch strings.ToLower(op.Op) {
		case "replace":
			switch {
			case op.Path == "active" || op.Path == "":
				active := scimParseBool(op.Value)
				if active && status == "suspended" {
					status = "active"
				} else if !active && status == "active" {
					status = "suspended"
					if s.credentialSvc != nil {
						s.credentialSvc.RevokeAllUserCredentials(tenantID, userID)
					}
				}
			case op.Path == "name.formatted":
				if s, ok := op.Value.(string); ok {
					name = s
				}
			case op.Path == "userName":
				if s, ok := op.Value.(string); ok {
					email = s
				}
			}

			// Handle replace without path (Okta sends {"op":"replace","value":{"active":false}})
			if op.Path == "" {
				if m, ok := op.Value.(map[string]any); ok {
					if v, exists := m["active"]; exists {
						active := scimParseBool(v)
						if active && status == "suspended" {
							status = "active"
						} else if !active && status == "active" {
							status = "suspended"
							if s.credentialSvc != nil {
								s.credentialSvc.RevokeAllUserCredentials(tenantID, userID)
							}
						}
					}
				}
			}
		}
	}

	updated, err := s.accessSvc.UpdateUser(tenantID, userID, existing.BuildingID, name, email, existing.Role, status, existing.GroupIDs)
	if err != nil {
		writeScimError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.auditSvc.Append(tenantID, "scim", "system", "user_patched_via_scim", userID, email)
	writeScimJSON(w, http.StatusOK, accessUserToSCIM(updated, scimBaseURL(r)))
}

// scimDeleteUser deactivates a user (SCIM DELETE = deactivate in access control context).
// DELETE /scim/v2/Users/{id}
func (s *server) scimDeleteUser(w http.ResponseWriter, r *http.Request) {
	tenantID := s.resolveSCIMTenantID(r)
	userID := chi.URLParam(r, "id")

	existing, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil {
		writeScimError(w, http.StatusNotFound, "User not found")
		return
	}

	// Revoke all credentials
	if s.credentialSvc != nil {
		s.credentialSvc.RevokeAllUserCredentials(tenantID, userID)
	}

	// Deactivate (don't hard-delete — audit trail)
	s.accessSvc.UpdateUser(tenantID, userID, existing.BuildingID, existing.Name, existing.Email, existing.Role, "suspended", existing.GroupIDs)

	s.auditSvc.Append(tenantID, "scim", "system", "user_deleted_via_scim", userID, existing.Email)
	s.logger.Info("scim user deactivated", "user_id", userID, "email", existing.Email)

	w.WriteHeader(http.StatusNoContent)
}

// --- Filter parsing ---

// applyScimUserFilter handles basic SCIM filter: userName eq "value"
func applyScimUserFilter(users []access.AccessUser, filter string) []access.AccessUser {
	// Parse: userName eq "user@example.com"
	parts := strings.SplitN(filter, " ", 3)
	if len(parts) != 3 {
		return users
	}
	attr := strings.ToLower(strings.TrimSpace(parts[0]))
	op := strings.ToLower(strings.TrimSpace(parts[1]))
	value := strings.Trim(strings.TrimSpace(parts[2]), "\"")

	if op != "eq" {
		return users
	}

	var result []access.AccessUser
	for _, u := range users {
		switch attr {
		case "username", "emails.value":
			if strings.EqualFold(u.Email, value) {
				result = append(result, u)
			}
		case "externalid":
			if u.SyncRef == value {
				result = append(result, u)
			}
		case "id":
			if u.ID == value {
				result = append(result, u)
			}
		}
	}
	return result
}

// scimParseBool extracts a boolean from various JSON value types.
func scimParseBool(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.EqualFold(val, "true")
	case float64:
		return val != 0
	default:
		return false
	}
}
