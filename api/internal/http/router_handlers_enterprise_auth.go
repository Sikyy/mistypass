package httpx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

func (s *server) issueEnterpriseTrustedSession(
	tenantID string,
	syncMode string,
	loginEmail string,
	identitySubject string,
	jitProfile enterpriseJITProvisionProfile,
) (auth.LoginResponse, bool, string, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	nextEmail := strings.TrimSpace(loginEmail)
	nextIdentitySubject := strings.TrimSpace(identitySubject)
	nextSyncMode := strings.TrimSpace(syncMode)

	if nextSyncMode != "jit" {
		response, err := s.authService.LoginByTrustedIdentity(nextEmail)
		if err != nil {
			return auth.LoginResponse{}, false, nextIdentitySubject, err
		}
		return response, false, nextIdentitySubject, nil
	}
	if enterpriseEmploymentStatusBlocksSession(jitProfile.EmploymentStatus) {
		return auth.LoginResponse{}, false, nextIdentitySubject, enterprise.ErrEmployeeInactive
	}
	if s.cfg.EnterpriseJITProvisionApprovalRequired {
		matched, matchErr := s.enterpriseSvc.HasActiveJITEmployeeIdentity(
			nextTenantID,
			nextEmail,
			nextIdentitySubject,
		)
		if matchErr != nil {
			return auth.LoginResponse{}, false, nextIdentitySubject, matchErr
		}
		if !matched {
			approved, approvalErr := s.enterpriseSvc.HasApprovedJITProvisionApproval(
				nextTenantID,
				nextEmail,
				nextIdentitySubject,
			)
			if approvalErr != nil {
				return auth.LoginResponse{}, false, nextIdentitySubject, approvalErr
			}
			if !approved {
				return auth.LoginResponse{}, false, nextIdentitySubject, enterprise.ErrJITProvisionApprovalRequired
			}
		}
	}

	employee, employeeCreated, employeeErr := s.enterpriseSvc.ResolveOrProvisionJITEmployeeWithProfile(
		nextTenantID,
		nextEmail,
		nextIdentitySubject,
		enterprise.JITProvisionProfile{
			FullName:          jitProfile.FullName,
			Department:        jitProfile.Department,
			JobTitle:          jitProfile.JobTitle,
			Location:          jitProfile.Location,
			Phone:             jitProfile.Phone,
			ManagerExternalID: jitProfile.ManagerExternalID,
			EmploymentStatus:  jitProfile.EmploymentStatus,
		},
	)
	if employeeErr != nil {
		return auth.LoginResponse{}, false, nextIdentitySubject, employeeErr
	}
	if nextIdentitySubject == "" {
		nextIdentitySubject = strings.TrimSpace(employee.ExternalID)
	}

	userExisted := false
	_, probeErr := s.authService.LoginByTrustedIdentity(nextEmail)
	if probeErr == nil {
		userExisted = true
	} else if !errors.Is(probeErr, auth.ErrInvalidCredentials) {
		return auth.LoginResponse{}, false, nextIdentitySubject, probeErr
	}

	jitUser := enterpriseJITTrustedUser(nextTenantID, employee, nextEmail, nextIdentitySubject)
	response, err := s.authService.LoginByTrustedUser(jitUser)
	if err != nil {
		return auth.LoginResponse{}, false, nextIdentitySubject, err
	}
	jitApplied := employeeCreated || !userExisted
	return response, jitApplied, nextIdentitySubject, nil
}

func enterpriseTrustedSessionErrorStatusCode(err error) int {
	switch {
	case errors.Is(err, enterprise.ErrEmployeeInactive):
		return http.StatusForbidden
	case errors.Is(err, enterprise.ErrJITProvisionApprovalRequired):
		return http.StatusForbidden
	case errors.Is(err, enterprise.ErrEmployeeExternalIDConflict):
		return http.StatusConflict
	case errors.Is(err, enterprise.ErrEmailRequired), errors.Is(err, enterprise.ErrInvalidDomain):
		return http.StatusBadRequest
	case errors.Is(err, auth.ErrInvalidCredentials):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

func (s *server) applyEnterpriseJITDeprovisionOnInactive(
	r *http.Request,
	tenantID, provider, email, externalID, employmentStatus string,
	err error,
) {
	if !errors.Is(err, enterprise.ErrEmployeeInactive) {
		return
	}

	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return
	}

	revokedRefreshCount := 0
	downgradedLocal := false
	beforeRole := ""
	afterRole := ""
	beforeBuildingScopeCount := 0
	afterBuildingScopeCount := 0
	if s.authService != nil {
		revokedRefreshCount = s.authService.RevokeRefreshTokensByUserEmail(email)
		beforeUser, afterUser, applied := s.authService.DowngradeTrustedUserToLeastPrivilegeByEmail(email, nextTenantID)
		downgradedLocal = applied
		beforeRole = strings.TrimSpace(beforeUser.Role)
		afterRole = strings.TrimSpace(afterUser.Role)
		beforeBuildingScopeCount = len(beforeUser.BuildingIDs)
		afterBuildingScopeCount = len(afterUser.BuildingIDs)
	}

	target := strings.TrimSpace(
		fmt.Sprintf(
			"provider=%s,email=%s,external_id=%s,employment_status=%s,revoked_refresh=%d,downgraded_local=%t,old_role=%s,new_role=%s,old_building_scope=%d,new_building_scope=%d",
			strings.TrimSpace(provider),
			strings.ToLower(strings.TrimSpace(email)),
			strings.TrimSpace(externalID),
			normalizeEmploymentStatus(employmentStatus),
			revokedRefreshCount,
			downgradedLocal,
			beforeRole,
			afterRole,
			beforeBuildingScopeCount,
			afterBuildingScopeCount,
		),
	)
	if target == "" {
		target = "enterprise_jit_deprovision_applied"
	}
	s.appendAuditLog(r, nextTenantID, "enterprise_jit_deprovision_applied", target, "enterprise_auth")
}

func (s *server) applyEnterpriseJITApprovalRequiredAudit(
	r *http.Request,
	tenantID, provider, email, externalID string,
	employmentStatus string,
	err error,
) {
	if !errors.Is(err, enterprise.ErrJITProvisionApprovalRequired) {
		return
	}

	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return
	}

	approvalID := ""
	if s.enterpriseSvc != nil {
		approval, approvalErr := s.enterpriseSvc.UpsertJITProvisionApprovalRequest(
			nextTenantID,
			email,
			externalID,
			provider,
			employmentStatus,
		)
		if approvalErr == nil {
			approvalID = strings.TrimSpace(approval.ID)
		}
	}

	target := strings.TrimSpace(
		fmt.Sprintf(
			"provider=%s,email=%s,external_id=%s,employment_status=%s,approval_id=%s,reason=jit_auto_provision_requires_approval",
			strings.TrimSpace(provider),
			strings.ToLower(strings.TrimSpace(email)),
			strings.TrimSpace(externalID),
			normalizeEmploymentStatus(employmentStatus),
			approvalID,
		),
	)
	if target == "" {
		target = "enterprise_jit_approval_required"
	}
	s.appendAuditLog(r, nextTenantID, "enterprise_jit_approval_required", target, "enterprise_auth")
}

type enterpriseJITProvisionProfile struct {
	FullName          string
	Department        string
	JobTitle          string
	Location          string
	Phone             string
	ManagerExternalID string
	EmploymentStatus  string
}

func enterpriseJITProfileFromOIDCIdentity(identity enterprise.OIDCIdentity) enterpriseJITProvisionProfile {
	return enterpriseJITProvisionProfile{
		FullName:          strings.TrimSpace(identity.Name),
		Department:        strings.TrimSpace(identity.Department),
		JobTitle:          strings.TrimSpace(identity.JobTitle),
		Location:          strings.TrimSpace(identity.Location),
		Phone:             strings.TrimSpace(identity.Phone),
		ManagerExternalID: strings.TrimSpace(identity.ManagerExternalID),
		EmploymentStatus:  normalizeEmploymentStatus(identity.EmploymentStatus),
	}
}

func enterpriseJITProfileFromSAMLIdentity(identity enterprise.SAMLIdentity) enterpriseJITProvisionProfile {
	return enterpriseJITProvisionProfile{
		FullName: samlAttributeFirst(identity.Attributes,
			"name",
			"displayname",
			"display_name",
			"full_name",
			"fullname",
			"cn",
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
		),
		Department: samlAttributeFirst(identity.Attributes,
			"department",
			"dept",
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/department",
		),
		JobTitle: samlAttributeFirst(identity.Attributes,
			"job_title",
			"title",
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/title",
		),
		Location: samlAttributeFirst(identity.Attributes,
			"location",
			"city",
			"office",
			"physicaldeliveryofficename",
		),
		Phone: samlAttributeFirst(identity.Attributes,
			"phone_number",
			"phone",
			"mobile",
			"telephone",
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/mobilephone",
		),
		ManagerExternalID: samlAttributeFirst(identity.Attributes,
			"manager_external_id",
			"manager_id",
			"manager",
		),
		EmploymentStatus: enterpriseJITEmploymentStatusFromSAML(identity),
	}
}

func enterpriseJITEmploymentStatusFromSAML(identity enterprise.SAMLIdentity) string {
	status := samlAttributeFirst(identity.Attributes,
		"employment_status",
		"employmentstatus",
		"status",
	)
	if status != "" {
		return normalizeEmploymentStatus(status)
	}

	activeFlag := samlAttributeFirst(identity.Attributes, "active")
	if activeFlag == "" {
		return ""
	}
	return normalizeEmploymentStatus(activeFlag)
}

func normalizeEmploymentStatus(raw string) string {
	return enterprise.NormalizeEmploymentStatus(raw)
}

func enterpriseEmploymentStatusBlocksSession(status string) bool {
	return enterprise.EmploymentStatusBlocksSession(status)
}

func samlAttributeFirst(attributes map[string][]string, keys ...string) string {
	if len(attributes) == 0 {
		return ""
	}
	for i := range keys {
		key := strings.ToLower(strings.TrimSpace(keys[i]))
		if key == "" {
			continue
		}
		values := attributes[key]
		for j := range values {
			value := strings.TrimSpace(values[j])
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func buildEnterpriseOIDCAuthorizeURL(config enterprise.IDPConfig, stateToken, redirectURI string) (string, error) {
	baseURL := strings.TrimSpace(config.AuthURL)
	if baseURL == "" {
		baseURL = strings.TrimSuffix(strings.TrimSpace(config.IssuerURL), "/") + "/oauth2/auth"
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid oidc auth_url: %s", baseURL)
	}

	scopes := config.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", "email"}
	}
	query := parsed.Query()
	query.Set("client_id", config.ClientID)
	query.Set("response_type", "code")
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(scopes, " "))
	query.Set("state", stateToken)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func enterpriseOIDCTokenURL(config enterprise.IDPConfig) (string, error) {
	baseURL := strings.TrimSpace(config.TokenURL)
	if baseURL == "" {
		baseURL = strings.TrimSuffix(strings.TrimSpace(config.IssuerURL), "/") + "/oauth2/token"
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid oidc token_url: %s", baseURL)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid oidc token_url: %s", baseURL)
	}
	return parsed.String(), nil
}

func exchangeEnterpriseOIDCCodeForIDToken(
	ctx context.Context,
	config enterprise.IDPConfig,
	code string,
	redirectURI string,
) (string, int, error) {
	return exchangeEnterpriseOIDCCodeForIDTokenWithClient(
		ctx,
		&http.Client{Timeout: 10 * time.Second},
		config,
		code,
		redirectURI,
	)
}

func exchangeEnterpriseOIDCCodeForIDTokenWithClient(
	ctx context.Context,
	client *http.Client,
	config enterprise.IDPConfig,
	code string,
	redirectURI string,
) (string, int, error) {
	nextCode := strings.TrimSpace(code)
	if nextCode == "" {
		return "", http.StatusBadRequest, errors.New("oidc authorization code is required")
	}
	nextClientID := strings.TrimSpace(config.ClientID)
	if nextClientID == "" {
		return "", http.StatusBadRequest, errors.New("oidc client_id is required")
	}

	tokenURL, err := enterpriseOIDCTokenURL(config)
	if err != nil {
		return "", http.StatusBadRequest, err
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", nextCode)
	form.Set("client_id", nextClientID)
	nextRedirectURI := strings.TrimSpace(redirectURI)
	if nextRedirectURI != "" {
		form.Set("redirect_uri", nextRedirectURI)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("failed to build oidc token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", http.StatusBadGateway, fmt.Errorf("oidc token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", http.StatusBadGateway, fmt.Errorf("failed to read oidc token response: %w", err)
	}

	var tokenPayload struct {
		IDToken          string `json:"id_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(bodyBytes, &tokenPayload)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(tokenPayload.Error)
		if message == "" {
			message = strings.TrimSpace(string(bodyBytes))
		}
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		if detail := strings.TrimSpace(tokenPayload.ErrorDescription); detail != "" && !strings.Contains(message, detail) {
			message = message + ": " + detail
		}
		return "", http.StatusUnauthorized, fmt.Errorf("oidc code exchange failed: %s", message)
	}

	idToken := strings.TrimSpace(tokenPayload.IDToken)
	if idToken == "" {
		return "", http.StatusBadGateway, errors.New("oidc token response missing id_token")
	}
	return idToken, http.StatusOK, nil
}

func buildEnterpriseSAMLSSOURL(config enterprise.IDPConfig, stateToken, redirectURI string) (string, error) {
	baseURL := strings.TrimSpace(config.AuthURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(config.IssuerURL)
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid saml sso url: %s", baseURL)
	}

	query := parsed.Query()
	query.Set("RelayState", stateToken)
	query.Set("redirect_uri", redirectURI)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func enterpriseJITTrustedUser(
	tenantID string,
	employee enterprise.EnterpriseEmployee,
	fallbackEmail string,
	identitySubject string,
) auth.User {
	nextTenantID := strings.TrimSpace(tenantID)
	nextEmail := strings.TrimSpace(employee.Email)
	if nextEmail == "" {
		nextEmail = strings.TrimSpace(fallbackEmail)
	}

	nextRole := strings.ToLower(strings.TrimSpace(employee.AccessRole))
	if nextRole == "" {
		nextRole = "resident"
	}

	var buildingIDs []string
	buildingID := strings.TrimSpace(employee.BuildingID)
	if buildingID != "" {
		buildingIDs = []string{buildingID}
	}

	return auth.User{
		ID: enterpriseJITUserID(
			nextTenantID,
			nextEmail,
			strings.TrimSpace(employee.ExternalID),
			identitySubject,
		),
		Email:       nextEmail,
		Role:        nextRole,
		TenantID:    nextTenantID,
		BuildingIDs: buildingIDs,
	}
}

func enterpriseJITUserID(tenantID, email, externalID, identitySubject string) string {
	nextTenantID := strings.TrimSpace(tenantID)
	nextEmail := strings.ToLower(strings.TrimSpace(email))
	nextExternalID := strings.TrimSpace(externalID)
	nextIdentitySubject := strings.TrimSpace(identitySubject)
	if nextExternalID == "" {
		nextExternalID = nextIdentitySubject
	}
	if nextExternalID == "" {
		nextExternalID = nextEmail
	}

	seed := nextTenantID + "|" + nextEmail + "|" + nextExternalID
	digest := sha256.Sum256([]byte(seed))
	return "usr_ent_jit_" + hex.EncodeToString(digest[:8])
}

type enterpriseSyncWorkerAlertItem struct {
	ID                    string    `json:"id"`
	TenantID              string    `json:"tenant_id"`
	Actor                 string    `json:"actor"`
	Role                  string    `json:"role"`
	Action                string    `json:"action"`
	WorkerAction          string    `json:"worker_action"`
	WorkerKind            string    `json:"worker_kind"`
	WorkerLabel           string    `json:"worker_label"`
	Source                string    `json:"source"`
	At                    time.Time `json:"at"`
	Failed                int       `json:"failed"`
	Threshold             int       `json:"threshold"`
	Processed             int       `json:"processed"`
	Applied               int       `json:"applied"`
	ConsecutiveFailures   int       `json:"consecutive_failures,omitempty"`
	FailureAgeSeconds     int       `json:"failure_age_seconds,omitempty"`
	SkippedByAttemptLimit int       `json:"skipped_by_attempt_limit"`
	SkippedByCooldown     int       `json:"skipped_by_cooldown"`
	ConnectorID           string    `json:"connector_id,omitempty"`
	Vendor                string    `json:"vendor,omitempty"`
	EventType             string    `json:"event_type,omitempty"`
	RequestID             string    `json:"request_id,omitempty"`
	FailureStage          string    `json:"failure_stage,omitempty"`
	Mode                  string    `json:"mode,omitempty"`
	RawTarget             string    `json:"raw_target"`
}

type enterpriseSyncWorkerAlertSummaryItem struct {
	TenantID                  string    `json:"tenant_id"`
	WorkerAction              string    `json:"worker_action"`
	WorkerKind                string    `json:"worker_kind"`
	WorkerLabel               string    `json:"worker_label"`
	Count                     int       `json:"count"`
	FirstSeenAt               time.Time `json:"first_seen_at"`
	LastSeenAt                time.Time `json:"last_seen_at"`
	LastFailed                int       `json:"last_failed"`
	LastThreshold             int       `json:"last_threshold"`
	LastProcessed             int       `json:"last_processed"`
	LastApplied               int       `json:"last_applied"`
	LastConsecutiveFailures   int       `json:"last_consecutive_failures,omitempty"`
	LastFailureAgeSeconds     int       `json:"last_failure_age_seconds,omitempty"`
	LastSkippedByAttemptLimit int       `json:"last_skipped_by_attempt_limit"`
	LastSkippedByCooldown     int       `json:"last_skipped_by_cooldown"`
}

func buildEnterpriseSyncWorkerAlerts(logs []audit.Log) []enterpriseSyncWorkerAlertItem {
	items := make([]enterpriseSyncWorkerAlertItem, 0, len(logs))
	for i := range logs {
		metrics := parseEnterpriseSyncWorkerAlertMetrics(logs[i].Target)
		action := strings.TrimSpace(logs[i].Action)
		items = append(items, enterpriseSyncWorkerAlertItem{
			ID:                    logs[i].ID,
			TenantID:              logs[i].TenantID,
			Actor:                 logs[i].Actor,
			Role:                  logs[i].Role,
			Action:                action,
			WorkerAction:          action,
			WorkerKind:            enterpriseSyncWorkerAlertKind(action),
			WorkerLabel:           enterpriseSyncWorkerAlertLabel(action),
			Source:                logs[i].Source,
			At:                    logs[i].At,
			Failed:                metrics.Failed,
			Threshold:             metrics.Threshold,
			Processed:             metrics.Processed,
			Applied:               metrics.Applied,
			ConsecutiveFailures:   metrics.ConsecutiveFailures,
			FailureAgeSeconds:     metrics.FailureAgeSeconds,
			SkippedByAttemptLimit: metrics.SkippedByAttemptLimit,
			SkippedByCooldown:     metrics.SkippedByCooldown,
			ConnectorID:           metrics.ConnectorID,
			Vendor:                metrics.Vendor,
			EventType:             metrics.EventType,
			RequestID:             metrics.RequestID,
			FailureStage:          metrics.FailureStage,
			Mode:                  metrics.Mode,
			RawTarget:             strings.TrimSpace(logs[i].Target),
		})
	}
	return items
}

func buildEnterpriseSyncWorkerAlertSummary(logs []audit.Log) []enterpriseSyncWorkerAlertSummaryItem {
	summaries := make(map[string]enterpriseSyncWorkerAlertSummaryItem)
	for i := range logs {
		tenantID := strings.TrimSpace(logs[i].TenantID)
		if tenantID == "" {
			continue
		}
		action := strings.TrimSpace(logs[i].Action)
		summaryKey := tenantID + "|" + action
		entry, exists := summaries[summaryKey]
		if !exists {
			entry = enterpriseSyncWorkerAlertSummaryItem{
				TenantID:     tenantID,
				WorkerAction: action,
				WorkerKind:   enterpriseSyncWorkerAlertKind(action),
				WorkerLabel:  enterpriseSyncWorkerAlertLabel(action),
				FirstSeenAt:  logs[i].At,
				LastSeenAt:   logs[i].At,
			}
		}
		entry.Count++
		if logs[i].At.Before(entry.FirstSeenAt) {
			entry.FirstSeenAt = logs[i].At
		}
		if !exists || logs[i].At.After(entry.LastSeenAt) {
			metrics := parseEnterpriseSyncWorkerAlertMetrics(logs[i].Target)
			entry.LastSeenAt = logs[i].At
			entry.LastFailed = metrics.Failed
			entry.LastThreshold = metrics.Threshold
			entry.LastProcessed = metrics.Processed
			entry.LastApplied = metrics.Applied
			entry.LastConsecutiveFailures = metrics.ConsecutiveFailures
			entry.LastFailureAgeSeconds = metrics.FailureAgeSeconds
			entry.LastSkippedByAttemptLimit = metrics.SkippedByAttemptLimit
			entry.LastSkippedByCooldown = metrics.SkippedByCooldown
		}
		summaries[summaryKey] = entry
	}

	items := make([]enterpriseSyncWorkerAlertSummaryItem, 0, len(summaries))
	for _, item := range summaries {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].LastSeenAt.Equal(items[j].LastSeenAt) {
			if items[i].TenantID != items[j].TenantID {
				return items[i].TenantID < items[j].TenantID
			}
			return items[i].WorkerAction < items[j].WorkerAction
		}
		return items[i].LastSeenAt.After(items[j].LastSeenAt)
	})
	return items
}

type enterpriseSyncWorkerAlertMetrics struct {
	Failed                int
	Threshold             int
	Processed             int
	Applied               int
	ConsecutiveFailures   int
	FailureAgeSeconds     int
	SkippedByAttemptLimit int
	SkippedByCooldown     int
	ConnectorID           string
	Vendor                string
	EventType             string
	RequestID             string
	FailureStage          string
	Mode                  string
}

func parseEnterpriseSyncWorkerAlertMetrics(rawTarget string) enterpriseSyncWorkerAlertMetrics {
	parts := strings.Fields(strings.TrimSpace(rawTarget))
	if len(parts) == 0 {
		return enterpriseSyncWorkerAlertMetrics{}
	}

	values := make(map[string]string, len(parts))
	for i := range parts {
		pair := strings.SplitN(parts[i], "=", 2)
		if len(pair) != 2 {
			continue
		}
		key := strings.TrimSpace(pair[0])
		value := strings.TrimSpace(pair[1])
		if key == "" || value == "" {
			continue
		}
		values[key] = value
	}

	return enterpriseSyncWorkerAlertMetrics{
		Failed:                parseIntOrZero(values["failed"]),
		Threshold:             parseIntOrZero(values["threshold"]),
		Processed:             parseIntOrZero(values["processed"]),
		Applied:               parseEnterpriseSyncWorkerAlertMetricWithFallback(values, "applied", "replayed", "synced"),
		ConsecutiveFailures:   parseIntOrZero(values["consecutive_failures"]),
		FailureAgeSeconds:     parseIntOrZero(values["failure_age_seconds"]),
		SkippedByAttemptLimit: parseIntOrZero(values["skipped_attempt_limit"]),
		SkippedByCooldown:     parseIntOrZero(values["skipped_cooldown"]),
		ConnectorID:           strings.TrimSpace(values["connector_id"]),
		Vendor:                strings.TrimSpace(values["vendor"]),
		EventType:             strings.TrimSpace(values["event_type"]),
		RequestID:             strings.TrimSpace(values["request_id"]),
		FailureStage:          strings.TrimSpace(values["failure_stage"]),
		Mode:                  strings.TrimSpace(values["mode"]),
	}
}

func parseEnterpriseSyncWorkerAlertMetricWithFallback(values map[string]string, keys ...string) int {
	for i := range keys {
		value, ok := values[keys[i]]
		if !ok {
			continue
		}
		return parseIntOrZero(value)
	}
	return 0
}

func enterpriseSyncWorkerAlertActions() []string {
	return []string{
		"enterprise_sync_reconcile_worker_alert",
		"enterprise_hris_webhook_receipt_worker_alert",
		"enterprise_hris_webhook_dlq_worker_alert",
		"enterprise_hris_pull_worker_alert",
		"enterprise_hris_webhook_processing_alert",
	}
}

func enterpriseSyncWorkerAlertKind(action string) string {
	switch strings.TrimSpace(action) {
	case "enterprise_sync_reconcile_worker_alert":
		return "sync_reconcile"
	case "enterprise_hris_webhook_receipt_worker_alert":
		return "hris_webhook_receipt_queue"
	case "enterprise_hris_webhook_dlq_worker_alert":
		return "hris_webhook_dlq"
	case "enterprise_hris_pull_worker_alert":
		return "hris_pull"
	case "enterprise_hris_webhook_processing_alert":
		return "hris_webhook_processing"
	default:
		return "unknown"
	}
}

func enterpriseSyncWorkerAlertLabel(action string) string {
	switch strings.TrimSpace(action) {
	case "enterprise_sync_reconcile_worker_alert":
		return "Enterprise Sync Reconcile"
	case "enterprise_hris_webhook_receipt_worker_alert":
		return "HRIS Webhook Receipt Queue"
	case "enterprise_hris_webhook_dlq_worker_alert":
		return "HRIS Webhook DLQ"
	case "enterprise_hris_pull_worker_alert":
		return "HRIS Pull Reconcile"
	case "enterprise_hris_webhook_processing_alert":
		return "HRIS Webhook Processing"
	default:
		return strings.TrimSpace(action)
	}
}

func parseIntOrZero(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return value
}

func parseRFC3339TimeRange(rawSince, rawUntil string) (*time.Time, *time.Time, error) {
	sinceInput := strings.TrimSpace(rawSince)
	untilInput := strings.TrimSpace(rawUntil)

	var since *time.Time
	var until *time.Time
	if sinceInput != "" {
		parsed, err := time.Parse(time.RFC3339, sinceInput)
		if err != nil {
			return nil, nil, errors.New("since must be RFC3339 timestamp")
		}
		parsedUTC := parsed.UTC()
		since = &parsedUTC
	}
	if untilInput != "" {
		parsed, err := time.Parse(time.RFC3339, untilInput)
		if err != nil {
			return nil, nil, errors.New("until must be RFC3339 timestamp")
		}
		parsedUTC := parsed.UTC()
		until = &parsedUTC
	}
	if since != nil && until != nil && since.After(*until) {
		return nil, nil, errors.New("since must be <= until")
	}
	return since, until, nil
}

func filterAuditLogsByTimeRange(logs []audit.Log, since, until *time.Time) []audit.Log {
	if since == nil && until == nil {
		return logs
	}
	items := make([]audit.Log, 0, len(logs))
	for i := range logs {
		at := logs[i].At
		if since != nil && at.Before(*since) {
			continue
		}
		if until != nil && at.After(*until) {
			continue
		}
		items = append(items, logs[i])
	}
	return items
}

const enterpriseAccessSyncSource = "enterprise_employee_sync"

func enterpriseEmployeesToAccessBatchInputs(items []enterprise.EnterpriseEmployee) []access.BatchUpsertUserByEmailInput {
	accessInputs := make([]access.BatchUpsertUserByEmailInput, 0, len(items))
	for i := range items {
		employee := items[i]
		accessInputs = append(accessInputs, access.BatchUpsertUserByEmailInput{
			BuildingID: employee.BuildingID,
			Name:       employee.FullName,
			Email:      employee.Email,
			Role:       employee.AccessRole,
			Status:     employee.Status,
			GroupIDs:   employee.GroupIDs,
			SyncSource: enterpriseAccessSyncSource,
			SyncRef:    enterpriseEmployeeAccessSyncRef(employee),
		})
	}
	return accessInputs
}

func enterpriseEmployeeAccessSyncRef(employee enterprise.EnterpriseEmployee) string {
	externalID := strings.TrimSpace(employee.ExternalID)
	if externalID != "" {
		return "external_id:" + externalID
	}
	email := strings.ToLower(strings.TrimSpace(employee.Email))
	if email != "" {
		return "email:" + email
	}
	return ""
}

func (s *server) startEnterpriseSyncReconcileWorker() {
	if !s.cfg.EnterpriseSyncReconcileWorkerEnabled {
		return
	}
	interval := s.cfg.EnterpriseSyncReconcileWorkerInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	batchSize := s.cfg.EnterpriseSyncReconcileWorkerBatchSize
	if batchSize <= 0 {
		batchSize = 1
	}
	maxAttempts := s.cfg.EnterpriseSyncReconcileWorkerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryCooldown := s.cfg.EnterpriseSyncReconcileWorkerRetryCooldown
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	alertFailureThreshold := s.cfg.EnterpriseSyncReconcileWorkerAlertFailureThreshold
	if alertFailureThreshold <= 0 {
		alertFailureThreshold = 1
	}
	forceError := s.cfg.EnterpriseSyncReconcileWorkerForceError
	forceErrorTenantID := strings.TrimSpace(s.cfg.EnterpriseSyncReconcileWorkerForceErrorTenantID)

	s.loggerOrDefault().Info(
		"enterprise sync reconcile worker enabled",
		"interval", interval,
		"batch_size", batchSize,
		"max_attempts", maxAttempts,
		"retry_cooldown", retryCooldown,
		"alert_threshold", alertFailureThreshold,
		"force_error", forceError,
		"force_error_tenant_id", forceErrorTenantID,
	)

	go func() {
		s.runEnterpriseSyncReconcileWorkerTick(
			batchSize,
			maxAttempts,
			retryCooldown,
			alertFailureThreshold,
			forceError,
			forceErrorTenantID,
		)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.runEnterpriseSyncReconcileWorkerTick(
				batchSize,
				maxAttempts,
				retryCooldown,
				alertFailureThreshold,
				forceError,
				forceErrorTenantID,
			)
		}
	}()
}

func (s *server) startEnterpriseSyncWorkerAlertAutoRetryWorker() {
	if !s.cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerEnabled {
		return
	}
	interval := s.cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	batchSize := s.cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBatchSize
	if batchSize <= 0 {
		batchSize = 20
	}
	maxAttempts := s.cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	baseBackoff := s.cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBaseBackoff
	if baseBackoff <= 0 {
		baseBackoff = 5 * time.Minute
	}
	maxBackoff := s.cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = time.Hour
	}
	if maxBackoff < baseBackoff {
		maxBackoff = baseBackoff
	}
	lockTTL := s.cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerLockTTL
	if lockTTL <= 0 {
		lockTTL = 10 * time.Minute
	}

	s.loggerOrDefault().Info(
		"enterprise sync worker alert auto retry worker enabled",
		"interval", interval,
		"batch_size", batchSize,
		"max_attempts", maxAttempts,
		"base_backoff", baseBackoff,
		"max_backoff", maxBackoff,
		"lock_ttl", lockTTL,
		"lease_enabled", s.workerLeaseStore != nil,
	)
	if s.workerLeaseStore == nil {
		s.loggerOrDefault().Warn(
			"enterprise sync worker alert auto retry worker running without redis lease; duplicate retries remain possible in multi-instance deployments",
		)
	}

	go func() {
		s.runEnterpriseSyncWorkerAlertAutoRetryWorkerTickWithLease(
			batchSize,
			maxAttempts,
			baseBackoff,
			maxBackoff,
			lockTTL,
		)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.runEnterpriseSyncWorkerAlertAutoRetryWorkerTickWithLease(
				batchSize,
				maxAttempts,
				baseBackoff,
				maxBackoff,
				lockTTL,
			)
		}
	}()
}

