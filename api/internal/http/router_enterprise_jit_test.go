package httpx

import (
	"errors"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

func TestEnterpriseEmploymentStatusBlocksSession(t *testing.T) {
	tests := []struct {
		status string
		block  bool
	}{
		{status: "active", block: false},
		{status: "inactive", block: true},
		{status: "terminated", block: true},
		{status: "disabled", block: true},
		{status: "suspended", block: true},
		{status: "deprovisioned", block: true},
		{status: "false", block: true},
		{status: "true", block: false},
		{status: "", block: false},
	}

	for _, tt := range tests {
		got := enterpriseEmploymentStatusBlocksSession(tt.status)
		if got != tt.block {
			t.Fatalf("status=%q expected block=%v got=%v", tt.status, tt.block, got)
		}
	}
}

func TestEnterpriseJITEmploymentStatusFromSAML(t *testing.T) {
	identity := enterprise.SAMLIdentity{
		Attributes: map[string][]string{
			"employment_status": {"terminated"},
			"active":            {"true"},
		},
	}
	if got := enterpriseJITEmploymentStatusFromSAML(identity); got != "terminated" {
		t.Fatalf("expected terminated, got=%q", got)
	}

	identity = enterprise.SAMLIdentity{
		Attributes: map[string][]string{
			"active": {"false"},
		},
	}
	if got := enterpriseJITEmploymentStatusFromSAML(identity); got != "inactive" {
		t.Fatalf("expected inactive from active=false, got=%q", got)
	}
}

func TestEnterpriseJITProfileFromOIDCIdentityDeepAttributes(t *testing.T) {
	identity := enterprise.OIDCIdentity{
		Name:              "OIDC Deep User",
		Department:        "IT",
		JobTitle:          "Engineer",
		Location:          "Jakarta",
		Phone:             "+62811111111",
		ManagerExternalID: "mgr-oidc-001",
		EmploymentStatus:  "active",
	}
	profile := enterpriseJITProfileFromOIDCIdentity(identity)
	if profile.Phone != "+62811111111" {
		t.Fatalf("expected phone from oidc identity, got=%q", profile.Phone)
	}
	if profile.ManagerExternalID != "mgr-oidc-001" {
		t.Fatalf("expected manager_external_id from oidc identity, got=%q", profile.ManagerExternalID)
	}
}

func TestEnterpriseJITProfileFromSAMLIdentityDeepAttributes(t *testing.T) {
	identity := enterprise.SAMLIdentity{
		Attributes: map[string][]string{
			"phone_number":        {"+628122222222"},
			"manager_external_id": {"mgr-saml-001"},
			"employment_status":   {"active"},
		},
	}
	profile := enterpriseJITProfileFromSAMLIdentity(identity)
	if profile.Phone != "+628122222222" {
		t.Fatalf("expected phone from saml attributes, got=%q", profile.Phone)
	}
	if profile.ManagerExternalID != "mgr-saml-001" {
		t.Fatalf("expected manager_external_id from saml attributes, got=%q", profile.ManagerExternalID)
	}
}

func TestIssueEnterpriseTrustedSessionJITBlocksInactiveEmploymentStatusClaim(t *testing.T) {
	s := &server{
		authService:         auth.NewService("", "", 0, 0, true),
		enterpriseSvc:       enterprise.NewService(),
		gatewayDeviceTokens: map[string]string{},
	}

	_, _, _, err := s.issueEnterpriseTrustedSession(
		"tenant_demo_jakarta",
		"jit",
		"jit.status.block@sudirman.co",
		"sub-jit-status-block-001",
		enterpriseJITProvisionProfile{
			Department:       "IT",
			Location:         "Jakarta",
			EmploymentStatus: "inactive",
		},
	)
	if !errors.Is(err, enterprise.ErrEmployeeInactive) {
		t.Fatalf("expected ErrEmployeeInactive, got: %v", err)
	}

	_, lookupErr := s.enterpriseSvc.GetEmployeeByEmail("tenant_demo_jakarta", "jit.status.block@sudirman.co")
	if !errors.Is(lookupErr, enterprise.ErrEmployeeNotFound) {
		t.Fatalf("expected no jit employee provisioned, err=%v", lookupErr)
	}
}

func TestEnterpriseJITTrustedUserMapping(t *testing.T) {
	employee := enterprise.EnterpriseEmployee{
		ExternalID: "hris-jkt-2026-1001",
		Email:      "jit.employee@sudirman.co",
		AccessRole: "building_admin",
		BuildingID: "building_demo_001",
	}

	user := enterpriseJITTrustedUser("tenant_demo_jakarta", employee, "fallback@sudirman.co", "sub-001")
	if user.TenantID != "tenant_demo_jakarta" {
		t.Fatalf("unexpected tenant_id: %s", user.TenantID)
	}
	if user.Email != "jit.employee@sudirman.co" {
		t.Fatalf("unexpected email: %s", user.Email)
	}
	if user.Role != "building_admin" {
		t.Fatalf("unexpected role: %s", user.Role)
	}
	if len(user.BuildingIDs) != 1 || user.BuildingIDs[0] != "building_demo_001" {
		t.Fatalf("unexpected building scope: %+v", user.BuildingIDs)
	}
	if !strings.HasPrefix(user.ID, "usr_ent_jit_") {
		t.Fatalf("unexpected user id prefix: %s", user.ID)
	}
}

func TestEnterpriseJITUserIDDeterministicAndScoped(t *testing.T) {
	idA1 := enterpriseJITUserID("tenant_demo_jakarta", "jit.employee@sudirman.co", "hris-1001", "sub-001")
	idA2 := enterpriseJITUserID("tenant_demo_jakarta", "jit.employee@sudirman.co", "hris-1001", "sub-001")
	if idA1 != idA2 {
		t.Fatalf("expected deterministic id, got %s vs %s", idA1, idA2)
	}

	idB := enterpriseJITUserID("tenant_demo_factory", "jit.employee@sudirman.co", "hris-1001", "sub-001")
	if idA1 == idB {
		t.Fatalf("expected tenant-scoped id to differ, got identical %s", idA1)
	}
}

func TestIssueEnterpriseTrustedSessionJITAutoProvisionWhenDirectoryMissing(t *testing.T) {
	s := &server{
		authService:         auth.NewService("", "", 0, 0, true),
		enterpriseSvc:       enterprise.NewService(),
		gatewayDeviceTokens: map[string]string{},
	}

	response, jitApplied, externalID, err := s.issueEnterpriseTrustedSession(
		"tenant_demo_jakarta",
		"jit",
		"jit.auto.session@sudirman.co",
		"sub-jit-session-001",
		enterpriseJITProvisionProfile{
			FullName:   "JIT Session User",
			Department: "Engineering",
			JobTitle:   "Facility Engineer",
			Location:   "Jakarta",
		},
	)
	if err != nil {
		t.Fatalf("expected jit trusted session to succeed: %v", err)
	}
	if !jitApplied {
		t.Fatalf("expected jit_applied=true")
	}
	if externalID != "sub-jit-session-001" {
		t.Fatalf("unexpected external id: %s", externalID)
	}
	if response.AccessToken == "" || response.RefreshToken == "" {
		t.Fatalf("expected token pair")
	}
	if response.User.Email != "jit.auto.session@sudirman.co" {
		t.Fatalf("unexpected user email: %s", response.User.Email)
	}
	if response.User.Role != "building_admin" {
		t.Fatalf("unexpected user role: %s", response.User.Role)
	}

	employee, err := s.enterpriseSvc.GetEmployeeByEmail("tenant_demo_jakarta", "jit.auto.session@sudirman.co")
	if err != nil {
		t.Fatalf("expected jit employee to be provisioned: %v", err)
	}
	if employee.Source != "jit_provision" {
		t.Fatalf("unexpected employee source: %s", employee.Source)
	}
	if employee.ExternalID != "sub-jit-session-001" {
		t.Fatalf("unexpected employee external id: %s", employee.ExternalID)
	}
}

func TestIssueEnterpriseTrustedSessionJITBlocksInactiveEmployee(t *testing.T) {
	s := &server{
		authService:         auth.NewService("", "", 0, 0, true),
		enterpriseSvc:       enterprise.NewService(),
		gatewayDeviceTokens: map[string]string{},
	}

	_, syncErr := s.enterpriseSvc.SyncEmployees(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID: "sub-jit-inactive-session-001",
				Email:      "jit.inactive.session@sudirman.co",
				FullName:   "Inactive Session User",
				Department: "IT",
				Location:   "Jakarta",
				Status:     "inactive",
			},
		},
	)
	if syncErr != nil {
		t.Fatalf("expected seed inactive employee to succeed: %v", syncErr)
	}

	_, _, _, err := s.issueEnterpriseTrustedSession(
		"tenant_demo_jakarta",
		"jit",
		"jit.inactive.session@sudirman.co",
		"sub-jit-inactive-session-001",
		enterpriseJITProvisionProfile{
			Department: "IT",
			Location:   "Jakarta",
		},
	)
	if !errors.Is(err, enterprise.ErrEmployeeInactive) {
		t.Fatalf("expected ErrEmployeeInactive, got: %v", err)
	}
}

func TestIssueEnterpriseTrustedSessionJITBlocksInactiveEvenWhenLocalUserExists(t *testing.T) {
	s := &server{
		authService:         auth.NewService("", "", 0, 0, true),
		enterpriseSvc:       enterprise.NewService(),
		gatewayDeviceTokens: map[string]string{},
	}

	_, loginErr := s.authService.LoginByTrustedUser(auth.User{
		ID:       "usr_local_inactive_jit_001",
		Email:    "jit.local.inactive@sudirman.co",
		Role:     "resident",
		TenantID: "tenant_demo_jakarta",
	})
	if loginErr != nil {
		t.Fatalf("expected local trusted user seed to succeed: %v", loginErr)
	}

	_, syncErr := s.enterpriseSvc.SyncEmployees(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID: "sub-jit-local-inactive-001",
				Email:      "jit.local.inactive@sudirman.co",
				FullName:   "Local Inactive JIT",
				Department: "IT",
				Location:   "Jakarta",
				Status:     "inactive",
			},
		},
	)
	if syncErr != nil {
		t.Fatalf("expected inactive employee seed to succeed: %v", syncErr)
	}

	_, _, _, err := s.issueEnterpriseTrustedSession(
		"tenant_demo_jakarta",
		"jit",
		"jit.local.inactive@sudirman.co",
		"sub-jit-local-inactive-001",
		enterpriseJITProvisionProfile{
			Department: "IT",
			Location:   "Jakarta",
		},
	)
	if !errors.Is(err, enterprise.ErrEmployeeInactive) {
		t.Fatalf("expected ErrEmployeeInactive with local user existing, got: %v", err)
	}
}

func TestIssueEnterpriseTrustedSessionJITSyncsLocalUserRoleWhenEmployeeActive(t *testing.T) {
	s := &server{
		authService:         auth.NewService("", "", 0, 0, true),
		enterpriseSvc:       enterprise.NewService(),
		gatewayDeviceTokens: map[string]string{},
	}

	_, loginErr := s.authService.LoginByTrustedUser(auth.User{
		ID:          "usr_local_role_sync_jit_001",
		Email:       "jit.local.rolesync@sudirman.co",
		Role:        "resident",
		TenantID:    "tenant_demo_jakarta",
		BuildingIDs: nil,
	})
	if loginErr != nil {
		t.Fatalf("expected local trusted user seed to succeed: %v", loginErr)
	}

	_, syncErr := s.enterpriseSvc.SyncEmployees(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID: "sub-jit-local-rolesync-001",
				Email:      "jit.local.rolesync@sudirman.co",
				FullName:   "Local Role Sync JIT",
				Department: "Facility",
				JobTitle:   "Engineer",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
	)
	if syncErr != nil {
		t.Fatalf("expected active employee seed to succeed: %v", syncErr)
	}

	response, jitApplied, _, err := s.issueEnterpriseTrustedSession(
		"tenant_demo_jakarta",
		"jit",
		"jit.local.rolesync@sudirman.co",
		"sub-jit-local-rolesync-001",
		enterpriseJITProvisionProfile{
			Department: "Facility",
			JobTitle:   "Engineer",
			Location:   "Jakarta",
		},
	)
	if err != nil {
		t.Fatalf("expected jit trusted session success: %v", err)
	}
	if jitApplied {
		t.Fatalf("expected jit_applied=false when local user already exists")
	}
	if response.User.Role != "building_admin" {
		t.Fatalf("expected role synced to building_admin, got %s", response.User.Role)
	}
	if len(response.User.BuildingIDs) != 1 || response.User.BuildingIDs[0] != "building_demo_001" {
		t.Fatalf("expected building scope synced, got %+v", response.User.BuildingIDs)
	}
}
