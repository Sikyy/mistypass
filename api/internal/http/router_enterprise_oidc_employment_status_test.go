package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

func TestEnterpriseAuthExchangeJITEmploymentStatusInactiveReturnsForbidden(t *testing.T) {
	s, issuerURL, clientID, emailDomain, signingKey := newEnterpriseJITOIDCTestServer(t)
	email := "jit.status.inactive.exchange@" + emailDomain

	idToken := mustBuildSignedOIDCIDTokenWithExtraClaims(
		t,
		signingKey,
		issuerURL,
		clientID,
		"sub-jit-status-inactive-exchange-001",
		email,
		map[string]any{
			"employment_status": "inactive",
		},
	)

	body := map[string]any{
		"email":     email,
		"provider":  "oidc",
		"idp_token": idToken,
	}
	requestBytes, _ := json.Marshal(body)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/exchange", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.enterpriseAuthExchange(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertResponseError(t, recorder, enterprise.ErrEmployeeInactive.Error())

	_, err := s.enterpriseSvc.GetEmployeeByEmail("tenant_demo_jakarta", email)
	if !errors.Is(err, enterprise.ErrEmployeeNotFound) {
		t.Fatalf("expected no jit employee provisioned when employment_status=inactive, err=%v", err)
	}
}

func TestEnterpriseOIDCCallbackJITEmploymentStatusTerminatedReturnsForbidden(t *testing.T) {
	s, issuerURL, clientID, emailDomain, signingKey := newEnterpriseJITOIDCTestServer(t)
	email := "jit.status.terminated.callback@" + emailDomain

	stateToken, err := s.enterpriseSvc.StartAuthStateToken(
		"tenant_demo_jakarta",
		"oidc",
		email,
		"https://admin.mistypass.local/enterprise/callback",
		5*time.Minute,
	)
	if err != nil {
		t.Fatalf("expected start auth state token success: %v", err)
	}

	idToken := mustBuildSignedOIDCIDTokenWithExtraClaims(
		t,
		signingKey,
		issuerURL,
		clientID,
		"sub-jit-status-terminated-callback-001",
		email,
		map[string]any{
			"status": "terminated",
		},
	)

	rawURL := "/api/v1/enterprise/auth/oidc/callback?state=" +
		url.QueryEscape(stateToken.Token) +
		"&id_token=" + url.QueryEscape(idToken)
	request := httptest.NewRequest(http.MethodGet, rawURL, nil)
	recorder := httptest.NewRecorder()

	s.enterpriseOIDCCallback(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertResponseError(t, recorder, enterprise.ErrEmployeeInactive.Error())

	_, lookupErr := s.enterpriseSvc.GetEmployeeByEmail("tenant_demo_jakarta", email)
	if !errors.Is(lookupErr, enterprise.ErrEmployeeNotFound) {
		t.Fatalf("expected no jit employee provisioned when status=terminated, err=%v", lookupErr)
	}
}

func TestEnterpriseAuthExchangeJITKeepsSCIMSnapshotAttributes(t *testing.T) {
	s, issuerURL, clientID, emailDomain, signingKey := newEnterpriseJITOIDCTestServer(t)
	email := "jit.priority.snapshot.exchange@" + emailDomain
	externalID := "scim-jit-priority-oidc-001"

	_, err := s.enterpriseSvc.SyncEmployees(
		"tenant_demo_jakarta",
		"scim_sync",
		"qa",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID: externalID,
				Email:      email,
				FullName:   "SCIM Canonical Name",
				Department: "Finance",
				JobTitle:   "Analyst",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected seed scim employee success: %v", err)
	}

	idToken := mustBuildSignedOIDCIDTokenWithExtraClaims(
		t,
		signingKey,
		issuerURL,
		clientID,
		externalID,
		email,
		map[string]any{
			"name":       "OIDC Override Name",
			"department": "Facility",
			"job_title":  "Engineer",
			"location":   "Factory",
		},
	)

	body := map[string]any{
		"email":     email,
		"provider":  "oidc",
		"idp_token": idToken,
	}
	requestBytes, _ := json.Marshal(body)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/exchange", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.enterpriseAuthExchange(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Token auth.LoginResponse `json:"token"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json response, decode err=%v body=%s", err, recorder.Body.String())
	}
	if payload.Token.User.Role != "resident" {
		t.Fatalf("expected role resident from scim snapshot, got %s", payload.Token.User.Role)
	}

	employee, lookupErr := s.enterpriseSvc.GetEmployeeByEmail("tenant_demo_jakarta", email)
	if lookupErr != nil {
		t.Fatalf("expected employee lookup success, err=%v", lookupErr)
	}
	if employee.Source != "scim_sync" {
		t.Fatalf("expected source scim_sync, got %s", employee.Source)
	}
	if employee.FullName != "SCIM Canonical Name" ||
		employee.Department != "Finance" ||
		employee.JobTitle != "Analyst" ||
		employee.Location != "Jakarta" {
		t.Fatalf(
			"expected snapshot fields unchanged, got full_name=%s department=%s job_title=%s location=%s",
			employee.FullName,
			employee.Department,
			employee.JobTitle,
			employee.Location,
		)
	}
}

func TestEnterpriseAuthExchangeJITEmploymentStatusInactiveRevokesRefreshAndAudits(t *testing.T) {
	s, issuerURL, clientID, emailDomain, signingKey := newEnterpriseJITOIDCTestServer(t)
	email := "jit.deprovision.exchange@" + emailDomain

	login, err := s.authService.LoginByTrustedUser(auth.User{
		ID:       "usr_ent_jit_deprovision_001",
		Email:    email,
		Role:     "building_admin",
		TenantID: "tenant_demo_jakarta",
		BuildingIDs: []string{
			"building_demo_001",
		},
	})
	if err != nil {
		t.Fatalf("expected seed trusted user login success: %v", err)
	}

	idToken := mustBuildSignedOIDCIDTokenWithExtraClaims(
		t,
		signingKey,
		issuerURL,
		clientID,
		"sub-jit-deprovision-exchange-001",
		email,
		map[string]any{
			"employment_status": "inactive",
		},
	)

	body := map[string]any{
		"email":     email,
		"provider":  "oidc",
		"idp_token": idToken,
	}
	requestBytes, _ := json.Marshal(body)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/exchange", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.enterpriseAuthExchange(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertResponseError(t, recorder, enterprise.ErrEmployeeInactive.Error())

	if _, err := s.authService.Refresh(auth.RefreshRequest{RefreshToken: login.RefreshToken}); err != auth.ErrInvalidRefreshToken {
		t.Fatalf("expected refresh token revoked after deprovision, got: %v", err)
	}
	postDowngradeLogin, loginErr := s.authService.LoginByTrustedIdentity(email)
	if loginErr != nil {
		t.Fatalf("expected trusted identity login after downgrade success: %v", loginErr)
	}
	if postDowngradeLogin.User.Role != "resident" {
		t.Fatalf("expected downgraded role resident, got %s", postDowngradeLogin.User.Role)
	}
	if len(postDowngradeLogin.User.BuildingIDs) != 0 {
		t.Fatalf("expected downgraded building scope empty, got %+v", postDowngradeLogin.User.BuildingIDs)
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_jit_deprovision_applied", "enterprise_auth", 20)
	if len(logs) == 0 {
		t.Fatalf("expected deprovision audit log to be appended")
	}
	target := logs[0].Target
	if !strings.Contains(target, "email="+email) {
		t.Fatalf("expected audit target contains email, got: %s", target)
	}
	if !strings.Contains(target, "revoked_refresh=") {
		t.Fatalf("expected audit target contains revoked_refresh count, got: %s", target)
	}
	if !strings.Contains(target, "downgraded_local=true") {
		t.Fatalf("expected audit target contains downgraded_local=true, got: %s", target)
	}
	if !strings.Contains(target, "old_role=building_admin") {
		t.Fatalf("expected audit target contains old role building_admin, got: %s", target)
	}
	if !strings.Contains(target, "new_role=resident") {
		t.Fatalf("expected audit target contains new role resident, got: %s", target)
	}
}

func TestEnterpriseAuthExchangeJITApprovalRequiredReturnsForbidden(t *testing.T) {
	s, issuerURL, clientID, emailDomain, signingKey := newEnterpriseJITOIDCTestServer(t)
	s.cfg.EnterpriseJITProvisionApprovalRequired = true
	email := "jit.approval.required.exchange@" + emailDomain

	idToken := mustBuildSignedOIDCIDTokenWithExtraClaims(
		t,
		signingKey,
		issuerURL,
		clientID,
		"sub-jit-approval-required-001",
		email,
		map[string]any{
			"employment_status": "active",
		},
	)

	body := map[string]any{
		"email":     email,
		"provider":  "oidc",
		"idp_token": idToken,
	}
	requestBytes, _ := json.Marshal(body)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/exchange", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.enterpriseAuthExchange(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertResponseError(t, recorder, enterprise.ErrJITProvisionApprovalRequired.Error())

	_, err := s.enterpriseSvc.GetEmployeeByEmail("tenant_demo_jakarta", email)
	if !errors.Is(err, enterprise.ErrEmployeeNotFound) {
		t.Fatalf("expected no jit employee provisioned when approval required, err=%v", err)
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_jit_approval_required", "enterprise_auth", 20)
	if len(logs) == 0 {
		t.Fatalf("expected approval required audit log to be appended")
	}
	if !strings.Contains(logs[0].Target, "reason=jit_auto_provision_requires_approval") {
		t.Fatalf("expected approval required reason in audit target, got: %s", logs[0].Target)
	}
}

func TestEnterpriseAuthExchangeJITApprovalReviewAllowsProvision(t *testing.T) {
	s, issuerURL, clientID, emailDomain, signingKey := newEnterpriseJITOIDCTestServer(t)
	s.cfg.EnterpriseJITProvisionApprovalRequired = true
	email := "jit.approval.review.exchange@" + emailDomain
	externalID := "sub-jit-approval-review-001"

	idToken := mustBuildSignedOIDCIDTokenWithExtraClaims(
		t,
		signingKey,
		issuerURL,
		clientID,
		externalID,
		email,
		map[string]any{
			"employment_status": "active",
		},
	)

	body := map[string]any{
		"email":     email,
		"provider":  "oidc",
		"idp_token": idToken,
	}
	requestBytes, _ := json.Marshal(body)
	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/exchange", bytes.NewReader(requestBytes))
	firstReq.Header.Set("Content-Type", "application/json")
	firstRecorder := httptest.NewRecorder()

	s.enterpriseAuthExchange(firstRecorder, firstReq)

	if firstRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected first attempt 403, got %d body=%s", firstRecorder.Code, firstRecorder.Body.String())
	}
	assertResponseError(t, firstRecorder, enterprise.ErrJITProvisionApprovalRequired.Error())

	pending := s.enterpriseSvc.ListJITProvisionApprovals("tenant_demo_jakarta", "pending", 10)
	if len(pending) == 0 {
		t.Fatalf("expected pending approval request created")
	}

	_, reviewErr := s.enterpriseSvc.ReviewJITProvisionApproval(
		"tenant_demo_jakarta",
		pending[0].ID,
		"approved",
		"tenant.admin@sudirman.co",
		"approved for onboarding",
	)
	if reviewErr != nil {
		t.Fatalf("expected approval review success: %v", reviewErr)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/exchange", bytes.NewReader(requestBytes))
	secondReq.Header.Set("Content-Type", "application/json")
	secondRecorder := httptest.NewRecorder()

	s.enterpriseAuthExchange(secondRecorder, secondReq)

	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("expected second attempt 200 after approval, got %d body=%s", secondRecorder.Code, secondRecorder.Body.String())
	}

	employee, lookupErr := s.enterpriseSvc.GetEmployeeByEmail("tenant_demo_jakarta", email)
	if lookupErr != nil {
		t.Fatalf("expected jit employee provisioned after approval, err=%v", lookupErr)
	}
	if employee.ExternalID != externalID {
		t.Fatalf("unexpected employee external_id after approval, got %s", employee.ExternalID)
	}
}
