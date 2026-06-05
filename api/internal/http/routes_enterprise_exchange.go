package httpx

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

func (s *server) enterpriseAuthExchange(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email      string `json:"email"`
		Provider   string `json:"provider"`
		TenantID   string `json:"tenant_id"`
		IDPToken   string `json:"idp_token"`
		ExternalID string `json:"external_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(request.IDPToken) == "" {
		writeError(w, http.StatusBadRequest, "idp_token is required")
		return
	}

	resolution, err := s.enterpriseSvc.ResolveTenantByEmail(request.Email)
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

	requestedTenantID := strings.TrimSpace(request.TenantID)
	if requestedTenantID != "" && requestedTenantID != resolution.TenantID {
		writeError(w, http.StatusForbidden, "tenant scope forbidden")
		return
	}

	config, err := s.enterpriseSvc.GetIDPConfig(resolution.TenantID)
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

	requestProvider := strings.ToLower(strings.TrimSpace(request.Provider))
	if requestProvider != "" && requestProvider != config.Provider {
		writeError(w, http.StatusUnauthorized, "idp provider mismatch")
		return
	}

	loginEmail := strings.TrimSpace(request.Email)
	identitySubject := strings.TrimSpace(request.ExternalID)
	var idpIdentity any
	jitProfile := enterpriseJITProvisionProfile{}

	switch config.Provider {
	case "oidc":
		identity, err := s.enterpriseSvc.VerifyOIDCIDToken(config, request.IDPToken, request.Email)
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
		if identity.Email != "" {
			loginEmail = identity.Email
		}
		if identitySubject == "" {
			identitySubject = identity.Subject
		}
		jitProfile = enterpriseJITProfileFromOIDCIdentity(identity)
		idpIdentity = identity

	case "saml":
		identity, err := s.enterpriseSvc.VerifySAMLResponse(config, request.IDPToken, request.Email)
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
				errors.Is(err, enterprise.ErrSAMLAssertionEmailMismatch),
				errors.Is(err, enterprise.ErrSAMLAssertionReplayed):
				writeError(w, http.StatusUnauthorized, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		if identity.Email != "" {
			loginEmail = identity.Email
		}
		if identitySubject == "" {
			identitySubject = identity.Subject
		}
		jitProfile = enterpriseJITProfileFromSAMLIdentity(identity)
		idpIdentity = identity

	default:
		writeError(w, http.StatusBadRequest, enterprise.ErrInvalidIDPProvider.Error())
		return
	}

	response, jitApplied, finalExternalID, err := s.issueEnterpriseTrustedSession(
		resolution.TenantID,
		config.SyncMode,
		loginEmail,
		identitySubject,
		jitProfile,
	)
	if err != nil {
		s.applyEnterpriseJITDeprovisionOnInactive(
			r,
			resolution.TenantID,
			config.Provider,
			loginEmail,
			identitySubject,
			jitProfile.EmploymentStatus,
			err,
		)
		s.applyEnterpriseJITApprovalRequiredAudit(
			r,
			resolution.TenantID,
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
		"tenant_id":    resolution.TenantID,
		"provider":     config.Provider,
		"sync_mode":    config.SyncMode,
		"jit_applied":  jitApplied,
		"external_id":  finalExternalID,
		"idp_identity": idpIdentity,
		"token":        response,
	})
}
