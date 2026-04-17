package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

func (s *server) listEnterpriseDomainMappings(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.enterpriseSvc.ListDomainMappings(tenantID),
	})
}

func (s *server) createEnterpriseDomainMapping(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID string `json:"tenant_id"`
		Domain   string `json:"domain"`
		Status   string `json:"status"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	created, err := s.enterpriseSvc.CreateDomainMapping(tenantID, request.Domain, request.Status)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrDomainRequired),
			errors.Is(err, enterprise.ErrInvalidDomain),
			errors.Is(err, enterprise.ErrInvalidDomainMappingStatus):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrDomainAlreadyMapped):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) updateEnterpriseDomainMappingStatus(w http.ResponseWriter, r *http.Request) {
	mappingID := chi.URLParam(r, "mappingID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	var request struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := s.enterpriseSvc.UpdateDomainMappingStatus(tenantID, mappingID, request.Status)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrInvalidDomainMappingStatus):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrDomainMappingNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *server) getEnterpriseIDPConfig(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	config, err := s.enterpriseSvc.GetIDPConfig(tenantID)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrIDPConfigNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (s *server) upsertEnterpriseIDPConfig(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID     string   `json:"tenant_id"`
		Provider     string   `json:"provider"`
		IssuerURL    string   `json:"issuer_url"`
		ClientID     string   `json:"client_id"`
		AuthURL      string   `json:"auth_url"`
		TokenURL     string   `json:"token_url"`
		JWKSURL      string   `json:"jwks_url"`
		UserInfoURL  string   `json:"user_info_url"`
		SAMLACSURL   string   `json:"saml_acs_url"`
		SAMLX509Cert string   `json:"saml_x509_cert"`
		Scopes       []string `json:"scopes"`
		Status       string   `json:"status"`
		SyncMode     string   `json:"sync_mode"`
		Actor        string   `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	config, err := s.enterpriseSvc.UpsertIDPConfig(
		tenantID,
		request.Provider,
		request.IssuerURL,
		request.ClientID,
		request.AuthURL,
		request.TokenURL,
		request.JWKSURL,
		request.UserInfoURL,
		request.SAMLACSURL,
		request.SAMLX509Cert,
		request.Status,
		request.SyncMode,
		request.Actor,
		request.Scopes,
	)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrInvalidIDPProvider),
			errors.Is(err, enterprise.ErrIssuerURLRequired),
			errors.Is(err, enterprise.ErrClientIDRequired),
			errors.Is(err, enterprise.ErrSAMLACSURLRequired),
			errors.Is(err, enterprise.ErrInvalidSAMLACSURL),
			errors.Is(err, enterprise.ErrSAMLX509CertRequired),
			errors.Is(err, enterprise.ErrInvalidIDPStatus):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, config)
}

func (s *server) validateEnterpriseIDPConfig(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID     string `json:"tenant_id"`
		Provider     string `json:"provider"`
		IssuerURL    string `json:"issuer_url"`
		ClientID     string `json:"client_id"`
		AuthURL      string `json:"auth_url"`
		TokenURL     string `json:"token_url"`
		JWKSURL      string `json:"jwks_url"`
		UserInfoURL  string `json:"user_info_url"`
		SAMLACSURL   string `json:"saml_acs_url"`
		SAMLX509Cert string `json:"saml_x509_cert"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	result := s.enterpriseSvc.ValidateIDPConfig(
		tenantID,
		request.Provider,
		request.IssuerURL,
		request.ClientID,
		request.AuthURL,
		request.TokenURL,
		request.JWKSURL,
		request.UserInfoURL,
		request.SAMLACSURL,
		request.SAMLX509Cert,
	)
	writeJSON(w, http.StatusOK, result)
}

func (s *server) listEnterpriseEmployees(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.enterpriseSvc.ListEmployees(tenantID),
	})
}

func (s *server) listEnterpriseJITProvisionApprovals(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	limit := 0
	limitInput := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitInput != "" {
		parsedLimit, err := strconv.Atoi(limitInput)
		if err != nil || parsedLimit < 0 {
			writeError(w, http.StatusBadRequest, "limit must be an integer >= 0")
			return
		}
		limit = parsedLimit
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.enterpriseSvc.ListJITProvisionApprovals(tenantID, status, limit),
	})
}

func (s *server) reviewEnterpriseJITProvisionApproval(w http.ResponseWriter, r *http.Request) {
	approvalID := chi.URLParam(r, "approvalID")
	var request struct {
		TenantID   string `json:"tenant_id"`
		Decision   string `json:"decision"`
		ReviewedBy string `json:"reviewed_by"`
		Reason     string `json:"reason"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	reviewedBy := strings.TrimSpace(request.ReviewedBy)
	if reviewedBy == "" {
		if user, exists := authenticatedUser(r); exists {
			reviewedBy = strings.TrimSpace(user.Email)
		}
	}

	item, err := s.enterpriseSvc.ReviewJITProvisionApproval(
		tenantID,
		approvalID,
		request.Decision,
		reviewedBy,
		request.Reason,
	)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrInvalidJITProvisionApprovalDecision):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrJITProvisionApprovalNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(
		r,
		tenantID,
		"enterprise_jit_approval_reviewed",
		fmt.Sprintf(
			"approval_id=%s,status=%s,reviewed_by=%s",
			item.ID,
			item.Status,
			strings.TrimSpace(item.ReviewedBy),
		),
		"enterprise_auth",
	)
	writeJSON(w, http.StatusOK, map[string]any{
		"item": item,
	})
}

func (s *server) listEnterpriseJITProvisionApprovalExternalSyncPending(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	limit := 0
	limitInput := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitInput != "" {
		parsedLimit, err := strconv.Atoi(limitInput)
		if err != nil || parsedLimit < 0 {
			writeError(w, http.StatusBadRequest, "limit must be an integer >= 0")
			return
		}
		limit = parsedLimit
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.enterpriseSvc.ListPendingJITProvisionApprovalExternalSync(tenantID, limit),
	})
}

func (s *server) updateEnterpriseJITProvisionApprovalExternalSync(w http.ResponseWriter, r *http.Request) {
	approvalID := chi.URLParam(r, "approvalID")
	var request struct {
		TenantID        string `json:"tenant_id"`
		Status          string `json:"status"`
		ExternalSyncRef string `json:"external_sync_ref"`
		LastError       string `json:"last_error"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	item, err := s.enterpriseSvc.UpdateJITProvisionApprovalExternalSync(
		tenantID,
		approvalID,
		request.Status,
		request.ExternalSyncRef,
		request.LastError,
	)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrInvalidJITProvisionApprovalExternalSyncStatus):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrJITProvisionApprovalNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(
		r,
		tenantID,
		"enterprise_jit_approval_external_sync_updated",
		fmt.Sprintf(
			"approval_id=%s,status=%s,external_sync_ref=%s,attempt_count=%d",
			item.ID,
			item.ExternalSyncStatus,
			item.ExternalSyncRef,
			item.ExternalSyncAttemptCount,
		),
		"enterprise_auth",
	)
	writeJSON(w, http.StatusOK, map[string]any{
		"item": item,
	})
}

func (s *server) enterpriseJITApprovalExternalSyncCallback(w http.ResponseWriter, r *http.Request) {
	callbackToken := strings.TrimSpace(s.cfg.EnterpriseJITApprovalExternalSyncCallbackToken)
	if callbackToken == "" {
		writeError(w, http.StatusServiceUnavailable, "enterprise jit approval external sync callback is disabled")
		return
	}

	var request struct {
		TenantID        string `json:"tenant_id"`
		ApprovalID      string `json:"approval_id"`
		Status          string `json:"status"`
		ExternalSyncRef string `json:"external_sync_ref"`
		LastError       string `json:"last_error"`
		CallbackToken   string `json:"callback_token"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	providedToken := strings.TrimSpace(r.Header.Get("X-Enterprise-Callback-Token"))
	if providedToken == "" {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			providedToken = strings.TrimSpace(authHeader[len("Bearer "):])
		}
	}
	if providedToken == "" {
		providedToken = strings.TrimSpace(request.CallbackToken)
	}
	if providedToken == "" || providedToken != callbackToken {
		writeError(w, http.StatusUnauthorized, "invalid callback token")
		return
	}

	tenantID := strings.TrimSpace(request.TenantID)
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrTenantIDRequired.Error())
		return
	}
	approvalID := strings.TrimSpace(request.ApprovalID)
	if approvalID == "" {
		writeError(w, http.StatusBadRequest, enterprise.ErrJITProvisionApprovalNotFound.Error())
		return
	}

	item, err := s.enterpriseSvc.UpdateJITProvisionApprovalExternalSync(
		tenantID,
		approvalID,
		request.Status,
		request.ExternalSyncRef,
		request.LastError,
	)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrInvalidJITProvisionApprovalExternalSyncStatus):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrJITProvisionApprovalNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(
		r,
		tenantID,
		"enterprise_jit_approval_external_sync_callback",
		fmt.Sprintf(
			"approval_id=%s,status=%s,external_sync_ref=%s",
			item.ID,
			item.ExternalSyncStatus,
			item.ExternalSyncRef,
		),
		"enterprise_auth",
	)
	writeJSON(w, http.StatusOK, map[string]any{
		"item": item,
	})
}

func (s *server) syncEnterpriseEmployees(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID  string                         `json:"tenant_id"`
		Source    string                         `json:"source"`
		Actor     string                         `json:"actor"`
		RequestID string                         `json:"request_id"`
		Employees []enterprise.EmployeeSyncInput `json:"employees"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	result, accessCreated, accessUpdated, accessRejected, err := s.enterpriseSvc.SyncEmployeesWithAccessUpsert(
		tenantID,
		request.Source,
		request.Actor,
		request.RequestID,
		request.Employees,
		func(items []enterprise.EnterpriseEmployee) (int, int, int, error) {
			accessInputs := enterpriseEmployeesToAccessBatchInputs(items)
			return s.accessSvc.UpsertUsersByEmail(tenantID, accessInputs)
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrEmployeesRequired),
			errors.Is(err, enterprise.ErrAccessSyncApplierRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"job":   result.Job,
		"items": result.Items,
		"access_sync": map[string]int{
			"created":  accessCreated,
			"updated":  accessUpdated,
			"rejected": accessRejected,
		},
	})
}

func (s *server) reconcileEnterpriseEmployeeSync(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID  string `json:"tenant_id"`
		RequestID string `json:"request_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	result, accessCreated, accessUpdated, accessRejected, err := s.enterpriseSvc.ReconcileSyncRequestAccess(
		tenantID,
		request.RequestID,
		func(items []enterprise.EnterpriseEmployee) (int, int, int, error) {
			accessInputs := enterpriseEmployeesToAccessBatchInputs(items)
			return s.accessSvc.UpsertUsersByEmail(tenantID, accessInputs)
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrSyncRequestIDRequired),
			errors.Is(err, enterprise.ErrAccessSyncApplierRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, enterprise.ErrSyncRequestNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": strings.TrimSpace(request.RequestID),
		"job":        result.Job,
		"items":      result.Items,
		"access_sync": map[string]int{
			"created":  accessCreated,
			"updated":  accessUpdated,
			"rejected": accessRejected,
		},
	})
}

func (s *server) reconcilePendingEnterpriseSyncRequests(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID string `json:"tenant_id"`
		Limit    int    `json:"limit"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	result, err := s.enterpriseSvc.ReconcilePendingSyncRequests(
		tenantID,
		request.Limit,
		func(items []enterprise.EnterpriseEmployee) (int, int, int, error) {
			accessInputs := enterpriseEmployeesToAccessBatchInputs(items)
			return s.accessSvc.UpsertUsersByEmail(tenantID, accessInputs)
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, enterprise.ErrTenantIDRequired),
			errors.Is(err, enterprise.ErrInvalidReconcileLimit),
			errors.Is(err, enterprise.ErrAccessSyncApplierRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *server) listEnterpriseSyncJobs(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.enterpriseSvc.ListSyncJobs(tenantID),
	})
}

func (s *server) listEnterpriseSyncRequests(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	requestID := strings.TrimSpace(r.URL.Query().Get("request_id"))
	if requestID != "" {
		record, err := s.enterpriseSvc.GetSyncRequestRecord(tenantID, requestID)
		if err != nil {
			switch {
			case errors.Is(err, enterprise.ErrTenantIDRequired),
				errors.Is(err, enterprise.ErrSyncRequestIDRequired):
				writeError(w, http.StatusBadRequest, err.Error())
			case errors.Is(err, enterprise.ErrSyncRequestNotFound):
				writeError(w, http.StatusNotFound, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"item": record,
		})
		return
	}

	limit := 50
	limitInput := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitInput != "" {
		parsedLimit, err := strconv.Atoi(limitInput)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be an integer")
			return
		}
		limit = parsedLimit
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.enterpriseSvc.ListSyncRequestRecords(tenantID, limit),
	})
}

func (s *server) listEnterpriseSyncWorkerAlerts(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	since, until, err := parseRFC3339TimeRange(
		r.URL.Query().Get("since"),
		r.URL.Query().Get("until"),
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	limit := 50
	limitInput := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitInput != "" {
		parsedLimit, err := strconv.Atoi(limitInput)
		if err != nil || parsedLimit < 0 {
			writeError(w, http.StatusBadRequest, "limit must be an integer >= 0")
			return
		}
		limit = parsedLimit
	}

	logs := s.auditSvc.ListFiltered(
		tenantID,
		"enterprise_sync_reconcile_worker_alert",
		"enterprise_sync_worker",
		0,
	)
	logs = filterAuditLogsByTimeRange(logs, since, until)
	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": buildEnterpriseSyncWorkerAlerts(logs),
	})
}

func (s *server) listEnterpriseSyncWorkerAlertSummary(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	since, until, err := parseRFC3339TimeRange(
		r.URL.Query().Get("since"),
		r.URL.Query().Get("until"),
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	limit := 50
	limitInput := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitInput != "" {
		parsedLimit, err := strconv.Atoi(limitInput)
		if err != nil || parsedLimit < 0 {
			writeError(w, http.StatusBadRequest, "limit must be an integer >= 0")
			return
		}
		limit = parsedLimit
	}

	logs := s.auditSvc.ListFiltered(
		tenantID,
		"enterprise_sync_reconcile_worker_alert",
		"enterprise_sync_worker",
		0,
	)
	logs = filterAuditLogsByTimeRange(logs, since, until)
	items := buildEnterpriseSyncWorkerAlertSummary(logs)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}
