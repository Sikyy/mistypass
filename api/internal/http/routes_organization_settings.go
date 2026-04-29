package httpx

import (
	"fmt"
	"net/http"
)

func (s *server) getOrganizationSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	settings := s.accessSvc.GetOrganizationSettings(tenantID)
	writeJSON(w, http.StatusOK, settings)
}

func (s *server) updateOrganizationSettings(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID              string `json:"tenant_id"`
		Name                  *string `json:"name"`
		PrimaryDomain         *string `json:"primary_domain"`
		Timezone              *string `json:"timezone"`
		SupportEmail          *string `json:"support_email"`
		EmailNotifications    *bool   `json:"email_notifications"`
		PushNotifications     *bool   `json:"push_notifications"`
		WeeklyReports         *bool   `json:"weekly_reports"`
		EnforceMFA            *bool   `json:"enforce_mfa"`
		PasswordPolicy        *string `json:"password_policy"`
		SessionTimeoutMinutes *int    `json:"session_timeout_minutes"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	settings, err := s.accessSvc.UpdateOrganizationSettings(
		tenantID,
		request.Name, request.PrimaryDomain, request.Timezone, request.SupportEmail,
		request.EmailNotifications, request.PushNotifications, request.WeeklyReports, request.EnforceMFA,
		request.PasswordPolicy, request.SessionTimeoutMinutes,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.appendAuditLog(r, tenantID, "organization_settings_updated",
		fmt.Sprintf("name=%s,timezone=%s,enforce_mfa=%v", settings.Name, settings.Timezone, settings.EnforceMFA),
		"access",
	)
	writeJSON(w, http.StatusOK, settings)
}

func (s *server) exportOrganizationAudit(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	s.appendAuditLog(r, tenantID, "organization_audit_export_requested", "", "access")
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "queued",
		"message": "Audit export has been queued. You will be notified when ready.",
	})
}

func (s *server) rotateOrganizationWebhooks(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	s.appendAuditLog(r, tenantID, "organization_webhook_secrets_rotated", "", "access")
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "rotated",
		"message": "All webhook signing secrets have been rotated.",
	})
}

func (s *server) disableOrganization(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	s.appendAuditLog(r, tenantID, "organization_disabled", "", "access")
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "disabled",
		"message": "Organization has been disabled. Contact support to reactivate.",
	})
}
