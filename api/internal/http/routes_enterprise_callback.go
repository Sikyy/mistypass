package httpx

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

func (s *server) enterpriseOIDCCallback(w http.ResponseWriter, r *http.Request) {
	stateToken := strings.TrimSpace(r.URL.Query().Get("state"))
	if stateToken == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrAuthStateTokenRequired.Error())
		return
	}

	authState, err := s.enterpriseSvc.ConsumeAuthStateToken(stateToken, "oidc")
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrAuthStateTokenRequired), errors.Is(err, enterprise.ErrInvalidIDPProvider):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrAuthStateTokenNotFound), errors.Is(err, enterprise.ErrAuthStateProviderMismatch):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	config, err := s.enterpriseSvc.GetIDPConfig(authState.TenantID)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrIDPConfigNotFound):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if config.Status != "active" || config.Provider != "oidc" {
		writeError(w, http.StatusUnauthorized, "enterprise idp config is inactive or provider mismatch")
		return
	}

	idpToken := strings.TrimSpace(r.URL.Query().Get("idp_token"))
	if idpToken == "" {
		idpToken = strings.TrimSpace(r.URL.Query().Get("id_token"))
	}
	if idpToken == "" {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			writeError(w, http.StatusBadRequest, enterprise.ErrIDPTokenRequired.Error())
			return
		}
		exchangedToken, statusCode, exchangeErr := exchangeEnterpriseOIDCCodeForIDToken(
			r.Context(),
			config,
			code,
			authState.RedirectURI,
		)
		if exchangeErr != nil {
			writeError(w, statusCode, exchangeErr.Error())
			return
		}
		idpToken = exchangedToken
	}

	expectedEmail := authState.Email
	if expectedEmail == "" {
		expectedEmail = strings.TrimSpace(r.URL.Query().Get("email"))
	}
	identity, err := s.enterpriseSvc.VerifyOIDCIDToken(config, idpToken, expectedEmail)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrIDPTokenRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrInvalidIDPProvider), errors.Is(err, enterprise.ErrIDPJWKSURLRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrInvalidIDPToken),
			errors.Is(err, enterprise.ErrOIDCJWKSFetchFailed),
			errors.Is(err, enterprise.ErrOIDCJWKSKeyNotFound),
			errors.Is(err, enterprise.ErrOIDCTokenIssuerMismatch),
			errors.Is(err, enterprise.ErrOIDCTokenAudienceMismatch),
			errors.Is(err, enterprise.ErrOIDCTokenSubjectRequired),
			errors.Is(err, enterprise.ErrOIDCTokenEmailRequired),
			errors.Is(err, enterprise.ErrOIDCTokenEmailMismatch):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	// Bind the ID token to the nonce issued in the authorize request. This
	// prevents replay/injection of a captured ID token at the callback.
	if authState.Nonce != "" && strings.TrimSpace(identity.Nonce) != authState.Nonce {
		writeError(w, http.StatusUnauthorized, enterprise.ErrOIDCTokenNonceMismatch.Error())
		return
	}

	loginEmail := strings.TrimSpace(expectedEmail)
	if identity.Email != "" {
		loginEmail = identity.Email
	}
	identitySubject := strings.TrimSpace(identity.Subject)
	jitProfile := enterpriseJITProfileFromOIDCIdentity(identity)
	response, jitApplied, finalExternalID, err := s.issueEnterpriseTrustedSession(
		authState.TenantID,
		config.SyncMode,
		loginEmail,
		identitySubject,
		jitProfile,
	)
	if err != nil {
		s.applyEnterpriseJITDeprovisionOnInactive(
			r,
			authState.TenantID,
			config.Provider,
			loginEmail,
			identitySubject,
			jitProfile.EmploymentStatus,
			err,
		)
		s.applyEnterpriseJITApprovalRequiredAudit(
			r,
			authState.TenantID,
			config.Provider,
			loginEmail,
			identitySubject,
			jitProfile.EmploymentStatus,
			err,
		)
		writeError(w, enterpriseTrustedSessionErrorStatusCode(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id":    authState.TenantID,
		"provider":     config.Provider,
		"sync_mode":    config.SyncMode,
		"jit_applied":  jitApplied,
		"external_id":  finalExternalID,
		"idp_identity": identity,
		"redirect_uri": authState.RedirectURI,
		"token":        response,
	})
}

func (s *server) enterpriseSAMLCallback(w http.ResponseWriter, r *http.Request) {
	var request struct {
		State        string `json:"state"`
		IDPToken     string `json:"idp_token"`
		SAMLResponse string `json:"saml_response"`
		Email        string `json:"email"`
		ExternalID   string `json:"external_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	stateToken := strings.TrimSpace(request.State)
	if stateToken == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrAuthStateTokenRequired.Error())
		return
	}

	idpToken := strings.TrimSpace(request.IDPToken)
	if idpToken == "" {
		idpToken = strings.TrimSpace(request.SAMLResponse)
	}
	if idpToken == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrSAMLResponseRequired.Error())
		return
	}

	authState, err := s.enterpriseSvc.ConsumeAuthStateToken(stateToken, "saml")
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrAuthStateTokenRequired), errors.Is(err, enterprise.ErrInvalidIDPProvider):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrAuthStateTokenNotFound), errors.Is(err, enterprise.ErrAuthStateProviderMismatch):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	config, err := s.enterpriseSvc.GetIDPConfig(authState.TenantID)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrIDPConfigNotFound):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if config.Status != "active" || config.Provider != "saml" {
		writeError(w, http.StatusUnauthorized, "enterprise idp config is inactive or provider mismatch")
		return
	}

	expectedEmail := authState.Email
	if expectedEmail == "" {
		expectedEmail = strings.TrimSpace(request.Email)
	}
	identity, err := s.enterpriseSvc.VerifySAMLResponse(config, idpToken, expectedEmail)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrSAMLResponseRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrInvalidIDPProvider),
			errors.Is(err, enterprise.ErrSAMLACSURLRequired),
			errors.Is(err, enterprise.ErrInvalidSAMLACSURL),
			errors.Is(err, enterprise.ErrSAMLX509CertRequired),
			errors.Is(err, enterprise.ErrInvalidSAMLX509Cert):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrInvalidSAMLResponse),
			errors.Is(err, enterprise.ErrSAMLAssertionIssuerMismatch),
			errors.Is(err, enterprise.ErrSAMLAssertionAudienceMismatch),
			errors.Is(err, enterprise.ErrSAMLAssertionSubjectRequired),
			errors.Is(err, enterprise.ErrSAMLAssertionEmailRequired),
			errors.Is(err, enterprise.ErrSAMLAssertionEmailMismatch):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	loginEmail := strings.TrimSpace(expectedEmail)
	if identity.Email != "" {
		loginEmail = identity.Email
	}
	identitySubject := strings.TrimSpace(request.ExternalID)
	if identitySubject == "" {
		identitySubject = strings.TrimSpace(identity.Subject)
	}
	jitProfile := enterpriseJITProfileFromSAMLIdentity(identity)

	response, jitApplied, finalExternalID, err := s.issueEnterpriseTrustedSession(
		authState.TenantID,
		config.SyncMode,
		loginEmail,
		identitySubject,
		jitProfile,
	)
	if err != nil {
		s.applyEnterpriseJITDeprovisionOnInactive(
			r,
			authState.TenantID,
			config.Provider,
			loginEmail,
			identitySubject,
			jitProfile.EmploymentStatus,
			err,
		)
		s.applyEnterpriseJITApprovalRequiredAudit(
			r,
			authState.TenantID,
			config.Provider,
			loginEmail,
			identitySubject,
			jitProfile.EmploymentStatus,
			err,
		)
		writeError(w, enterpriseTrustedSessionErrorStatusCode(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id":    authState.TenantID,
		"provider":     config.Provider,
		"sync_mode":    config.SyncMode,
		"jit_applied":  jitApplied,
		"external_id":  finalExternalID,
		"idp_identity": identity,
		"redirect_uri": authState.RedirectURI,
		"token":        response,
	})
}

