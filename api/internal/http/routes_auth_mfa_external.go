package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mistypass/cloud/api/internal/modules/auth"
)

func (s *server) getAdminMFAStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	status, err := s.authService.GetAdminMFAStatus(user.ID)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserRoleUnsupported):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *server) setupAdminMFA(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var request struct {
		Issuer string `json:"issuer"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	enrollment, err := s.authService.StartAdminMFAEnrollment(user.ID, request.Issuer)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserRoleUnsupported):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, enrollment)
}

func (s *server) enableAdminMFA(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	var request struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	status, err := s.authService.EnableAdminMFA(user.ID, request.Code)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrAdminMFARequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, auth.ErrInvalidMFACode):
			writeError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, auth.ErrAdminMFANotConfigured):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, auth.ErrUserRoleUnsupported):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *server) disableAdminMFA(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	status, err := s.authService.DisableAdminMFA(user.ID)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserRoleUnsupported):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *server) externalLogin(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.ExternalAuthEnabled {
		writeError(w, http.StatusServiceUnavailable, "external auth is not enabled")
		return
	}

	var request struct {
		AccessToken string `json:"access_token"`
		TenantID    string `json:"tenant_id"`
		Provider    string `json:"provider"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	nextToken := strings.TrimSpace(request.AccessToken)
	if nextToken == "" {
		writeError(w, http.StatusBadRequest, "access_token is required")
		return
	}

	requestProvider := strings.ToLower(strings.TrimSpace(request.Provider))
	configuredProvider := strings.ToLower(strings.TrimSpace(s.cfg.ExternalAuthProvider))
	if requestProvider != "" && requestProvider != configuredProvider {
		writeError(w, http.StatusUnauthorized, "external auth provider mismatch")
		return
	}

	identity, err := s.resolveExternalIdentity(r.Context(), nextToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "external identity verification failed")
		return
	}
	role := normalizeExternalRole(identity.Role, s.cfg.ExternalAuthDefaultRole)
	tenantID := strings.TrimSpace(identity.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(request.TenantID)
	}
	if role != "super_admin" && tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id is required for non-super_admin external login")
		return
	}

	userID := strings.TrimSpace(identity.ExternalID)
	if userID == "" {
		userID = buildExternalUserID(configuredProvider, identity.Email)
	}
	response, err := s.authService.LoginByTrustedUser(auth.User{
		ID:          userID,
		Email:       strings.TrimSpace(identity.Email),
		Role:        role,
		TenantID:    tenantID,
		BuildingIDs: append([]string(nil), identity.BuildingIDs...),
	})
	if err != nil {
		writeError(w, http.StatusUnauthorized, "external login failed")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

type externalIdentity struct {
	ExternalID  string
	Email       string
	Role        string
	TenantID    string
	BuildingIDs []string
}

func (s *server) resolveExternalIdentity(ctx context.Context, accessToken string) (externalIdentity, error) {
	userInfoURL := strings.TrimSpace(s.cfg.ExternalAuthUserInfoURL)
	if userInfoURL == "" {
		return externalIdentity{}, errors.New("external auth userinfo url is empty")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return externalIdentity{}, err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	request.Header.Set("Accept", "application/json")

	client := s.externalAuthHTTPClient
	if client == nil {
		client = &http.Client{}
	}
	response, err := client.Do(request)
	if err != nil {
		return externalIdentity{}, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return externalIdentity{}, fmt.Errorf("external auth userinfo status=%d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 512*1024))
	if err != nil {
		return externalIdentity{}, err
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return externalIdentity{}, err
	}

	email := firstString(payload, "email", "preferred_username", "username", "login")
	if email == "" {
		if identityMap, ok := payload["identity"].(map[string]any); ok {
			if traits, ok := identityMap["traits"].(map[string]any); ok {
				email = firstString(traits, "email", "preferred_username", "username", "login")
			}
		}
	}
	if email == "" {
		return externalIdentity{}, errors.New("external identity missing email")
	}

	externalID := firstString(payload, "sub", "id", "user_id", "external_id")
	if externalID == "" {
		if identityMap, ok := payload["identity"].(map[string]any); ok {
			externalID = firstString(identityMap, "id", "sub")
		}
	}
	role := firstString(payload, "role")
	if role == "" {
		role = firstStringFromArray(payload, "roles")
	}
	tenantID := firstString(payload, "tenant_id", "tenant", "org_id", "organization_id")
	buildingIDs := firstStringSlice(payload, "building_ids", "building_scope")

	return externalIdentity{
		ExternalID:  strings.TrimSpace(externalID),
		Email:       strings.TrimSpace(email),
		Role:        strings.TrimSpace(role),
		TenantID:    strings.TrimSpace(tenantID),
		BuildingIDs: buildingIDs,
	}, nil
}

func buildExternalUserID(provider, email string) string {
	nextProvider := strings.ToLower(strings.TrimSpace(provider))
	if nextProvider == "" {
		nextProvider = "external"
	}
	nextEmail := strings.ToLower(strings.TrimSpace(email))
	nextEmail = strings.ReplaceAll(nextEmail, "@", "_")
	nextEmail = strings.ReplaceAll(nextEmail, ".", "_")
	return "ext_" + nextProvider + "_" + nextEmail
}

func normalizeExternalRole(role, fallback string) string {
	nextRole := strings.ToLower(strings.TrimSpace(role))
	switch nextRole {
	case "super_admin", "tenant_admin", "operator", "building_admin", "resident":
		return nextRole
	}
	nextFallback := strings.ToLower(strings.TrimSpace(fallback))
	switch nextFallback {
	case "super_admin", "tenant_admin", "operator", "building_admin", "resident":
		return nextFallback
	default:
		return "resident"
	}
}

func firstString(payload map[string]any, keys ...string) string {
	for i := range keys {
		raw, exists := payload[keys[i]]
		if !exists {
			continue
		}
		switch value := raw.(type) {
		case string:
			next := strings.TrimSpace(value)
			if next != "" {
				return next
			}
		}
	}
	return ""
}

func firstStringFromArray(payload map[string]any, key string) string {
	raw, exists := payload[key]
	if !exists {
		return ""
	}
	items, ok := raw.([]any)
	if !ok {
		return ""
	}
	for i := range items {
		value, ok := items[i].(string)
		if !ok {
			continue
		}
		next := strings.TrimSpace(value)
		if next != "" {
			return next
		}
	}
	return ""
}

func firstStringSlice(payload map[string]any, keys ...string) []string {
	items := make([]string, 0, 4)
	seen := map[string]struct{}{}
	for i := range keys {
		raw, exists := payload[keys[i]]
		if !exists {
			continue
		}
		list, ok := raw.([]any)
		if !ok {
			continue
		}
		for j := range list {
			value, ok := list[j].(string)
			if !ok {
				continue
			}
			next := strings.TrimSpace(value)
			if next == "" {
				continue
			}
			if _, exists := seen[next]; exists {
				continue
			}
			seen[next] = struct{}{}
			items = append(items, next)
		}
	}
	if len(items) == 0 {
		return nil
	}
	return items
}
