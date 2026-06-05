package httpx

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

func (s *server) resolveEnterpriseTenantByEmail(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := s.enterpriseSvc.ResolveTenantByEmail(request.Email)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrEmailRequired), errors.Is(err, enterprise.ErrInvalidDomain):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrDomainNotMapped):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *server) enterpriseAuthStart(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email       string `json:"email"`
		TenantID    string `json:"tenant_id"`
		Provider    string `json:"provider"`
		RedirectURI string `json:"redirect_uri"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	requestedTenantID := strings.TrimSpace(request.TenantID)
	requestedEmail := strings.TrimSpace(request.Email)
	if requestedTenantID == "" && requestedEmail == "" {
		writeError(w, http.StatusBadRequest, "email or tenant_id is required")
		return
	}

	resolvedTenantID := requestedTenantID
	if requestedEmail != "" {
		resolution, err := s.enterpriseSvc.ResolveTenantByEmail(requestedEmail)
		if err != nil {
			switch {
			case errors.Is(err, enterprise.ErrEmailRequired), errors.Is(err, enterprise.ErrInvalidDomain):
				writeError(w, http.StatusBadRequest, err.Error())
			case errors.Is(err, enterprise.ErrDomainNotMapped):
				writeError(w, http.StatusUnauthorized, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		resolvedTenantID = resolution.TenantID
	}
	if requestedTenantID != "" && resolvedTenantID != "" && requestedTenantID != resolvedTenantID {
		writeError(w, http.StatusForbidden, "tenant scope forbidden")
		return
	}
	if resolvedTenantID == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrTenantIDRequired.Error())
		return
	}

	config, err := s.enterpriseSvc.GetIDPConfig(resolvedTenantID)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrIDPConfigNotFound):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if config.Status != "active" {
		writeError(w, http.StatusUnauthorized, "enterprise idp config is inactive")
		return
	}

	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	if provider == "" {
		provider = config.Provider
	}
	if provider != config.Provider {
		writeError(w, http.StatusUnauthorized, "idp provider mismatch")
		return
	}

	stateToken, err := s.enterpriseSvc.StartAuthStateToken(
		resolvedTenantID,
		provider,
		requestedEmail,
		request.RedirectURI,
		5*time.Minute,
	)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrInvalidIDPProvider),
			errors.Is(err, enterprise.ErrRedirectURIRequired),
			errors.Is(err, enterprise.ErrInvalidRedirectURI):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	response := map[string]any{
		"tenant_id":    resolvedTenantID,
		"provider":     provider,
		"sync_mode":    config.SyncMode,
		"state_token":  stateToken.Token,
		"redirect_uri": stateToken.RedirectURI,
		"expires_at":   stateToken.ExpiresAt,
	}

	switch provider {
	case "oidc":
		authorizeURL, buildErr := buildEnterpriseOIDCAuthorizeURL(config, stateToken.Token, stateToken.RedirectURI, stateToken.Nonce)
		if buildErr != nil {
			writeError(w, http.StatusInternalServerError, buildErr.Error())
			return
		}
		response["authorize_url"] = authorizeURL
	case "saml":
		ssoURL, buildErr := buildEnterpriseSAMLSSOURL(config, stateToken.Token, stateToken.RedirectURI)
		if buildErr != nil {
			writeError(w, http.StatusInternalServerError, buildErr.Error())
			return
		}
		response["sso_url"] = ssoURL
	default:
		writeError(w, http.StatusBadRequest, enterprise.ErrInvalidIDPProvider.Error())
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *server) enterpriseAuthLogout(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID     string `json:"tenant_id"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := strings.TrimSpace(request.TenantID)
	accessToken := strings.TrimSpace(request.AccessToken)
	refreshToken := strings.TrimSpace(request.RefreshToken)
	if accessToken == "" && refreshToken == "" {
		writeError(w, http.StatusBadRequest, "access_token or refresh_token is required")
		return
	}
	if tenantID != "" && accessToken == "" {
		writeError(w, http.StatusBadRequest, "tenant_id validation requires access_token")
		return
	}
	if tenantID != "" {
		user, err := s.authService.VerifyAccessToken(accessToken)
		if err != nil {
			writeError(w, http.StatusUnauthorized, auth.ErrInvalidAccessToken.Error())
			return
		}
		if strings.TrimSpace(user.TenantID) != tenantID {
			writeError(w, http.StatusForbidden, "tenant scope forbidden")
			return
		}
	}

	revokedAccess := false
	revokedRefresh := false
	accessErr := ""
	refreshErr := ""

	if accessToken != "" {
		if err := s.authService.Logout(accessToken); err != nil {
			accessErr = err.Error()
		} else {
			revokedAccess = true
		}
	}
	if refreshToken != "" {
		if err := s.authService.RevokeRefreshToken(refreshToken); err != nil {
			refreshErr = err.Error()
		} else {
			revokedRefresh = true
		}
	}

	response := map[string]any{
		"revoked_access":  revokedAccess,
		"revoked_refresh": revokedRefresh,
	}
	if tenantID != "" {
		response["tenant_id"] = tenantID
	}
	if accessErr != "" {
		response["access_error"] = accessErr
	}
	if refreshErr != "" {
		response["refresh_error"] = refreshErr
	}
	if !revokedAccess && !revokedRefresh && (accessErr != "" || refreshErr != "") {
		writeJSON(w, http.StatusUnauthorized, response)
		return
	}
	writeJSON(w, http.StatusOK, response)
}
