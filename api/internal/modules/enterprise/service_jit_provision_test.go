package enterprise

import (
	"errors"
	"testing"
)

func TestResolveOrProvisionJITEmployeeCreatesNewEmployee(t *testing.T) {
	svc := NewService()

	employee, created, err := svc.ResolveOrProvisionJITEmployee(
		"tenant_demo_jakarta",
		"jit.auto.provision@sudirman.co",
		"sub-jit-auto-001",
		"JIT Auto Provision",
		"Facility",
		"Engineer",
		"Jakarta",
	)
	if err != nil {
		t.Fatalf("expected jit auto provision to succeed: %v", err)
	}
	if !created {
		t.Fatalf("expected employee to be created")
	}
	if employee.ExternalID != "sub-jit-auto-001" {
		t.Fatalf("unexpected external_id: %s", employee.ExternalID)
	}
	if employee.Status != "active" {
		t.Fatalf("expected active status, got %s", employee.Status)
	}
	if employee.Source != "jit_provision" {
		t.Fatalf("expected source jit_provision, got %s", employee.Source)
	}
	if employee.AccessRole != "building_admin" {
		t.Fatalf("expected role building_admin from template, got %s", employee.AccessRole)
	}
	if employee.BuildingID != "building_demo_001" {
		t.Fatalf("unexpected building id: %s", employee.BuildingID)
	}

	lookup, err := svc.GetEmployeeByEmail("tenant_demo_jakarta", "jit.auto.provision@sudirman.co")
	if err != nil {
		t.Fatalf("expected created employee to be queryable: %v", err)
	}
	if lookup.ID != employee.ID {
		t.Fatalf("lookup employee mismatch: got=%s want=%s", lookup.ID, employee.ID)
	}
}

func TestResolveOrProvisionJITEmployeeRejectsInactiveEmployee(t *testing.T) {
	svc := NewService()

	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		[]EmployeeSyncInput{
			{
				ExternalID: "sub-jit-inactive-001",
				Email:      "jit.inactive@sudirman.co",
				FullName:   "JIT Inactive",
				Department: "IT",
				Location:   "Jakarta",
				Status:     "inactive",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected sync create inactive employee to succeed: %v", err)
	}

	_, created, err := svc.ResolveOrProvisionJITEmployee(
		"tenant_demo_jakarta",
		"jit.inactive@sudirman.co",
		"sub-jit-inactive-001",
		"JIT Inactive",
		"IT",
		"Admin",
		"Jakarta",
	)
	if !errors.Is(err, ErrEmployeeInactive) {
		t.Fatalf("expected ErrEmployeeInactive, got: %v", err)
	}
	if created {
		t.Fatalf("inactive employee should not be recreated")
	}
}

func TestResolveOrProvisionJITEmployeeRejectsExternalIDConflict(t *testing.T) {
	svc := NewService()

	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		[]EmployeeSyncInput{
			{
				ExternalID: "sub-jit-conflict-a",
				Email:      "jit.conflict@sudirman.co",
				FullName:   "JIT Conflict",
				Department: "IT",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected sync create active employee to succeed: %v", err)
	}

	_, created, err := svc.ResolveOrProvisionJITEmployee(
		"tenant_demo_jakarta",
		"jit.conflict@sudirman.co",
		"sub-jit-conflict-b",
		"JIT Conflict",
		"IT",
		"Admin",
		"Jakarta",
	)
	if !errors.Is(err, ErrEmployeeExternalIDConflict) {
		t.Fatalf("expected ErrEmployeeExternalIDConflict, got: %v", err)
	}
	if created {
		t.Fatalf("external_id conflict must not create new record")
	}
}

func TestResolveOrProvisionJITEmployeeKeepsSCIMSnapshotAttributes(t *testing.T) {
	svc := NewService()

	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"scim_sync",
		"qa",
		[]EmployeeSyncInput{
			{
				ExternalID: "scim-jkt-3001",
				Email:      "jit.priority.scim@sudirman.co",
				FullName:   "SCIM Canonical User",
				Department: "Finance",
				JobTitle:   "Analyst",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("seed scim employee should succeed: %v", err)
	}

	employee, created, err := svc.ResolveOrProvisionJITEmployee(
		"tenant_demo_jakarta",
		"jit.priority.scim@sudirman.co",
		"scim-jkt-3001",
		"OIDC Display Name Override",
		"Facility",
		"Engineer",
		"Factory",
	)
	if err != nil {
		t.Fatalf("jit resolve should succeed: %v", err)
	}
	if created {
		t.Fatalf("expected existing scim employee to be reused")
	}
	if employee.Source != "scim_sync" {
		t.Fatalf("expected source to remain scim_sync, got %s", employee.Source)
	}
	if employee.FullName != "SCIM Canonical User" {
		t.Fatalf("expected full_name to keep scim snapshot, got %s", employee.FullName)
	}
	if employee.Department != "Finance" || employee.JobTitle != "Analyst" || employee.Location != "Jakarta" {
		t.Fatalf(
			"expected profile to keep scim snapshot, got department=%s job_title=%s location=%s",
			employee.Department,
			employee.JobTitle,
			employee.Location,
		)
	}
	if employee.AccessRole != "resident" {
		t.Fatalf("expected access role to stay resident from scim profile, got %s", employee.AccessRole)
	}
}

func TestResolveOrProvisionJITEmployeeFillsEmptySCIMSnapshotAttributes(t *testing.T) {
	svc := NewService()

	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"hris_import",
		"qa",
		[]EmployeeSyncInput{
			{
				ExternalID: "hris-jkt-4001",
				Email:      "jit.priority.fill@sudirman.co",
				FullName:   "",
				Department: "",
				JobTitle:   "",
				Location:   "",
				Status:     "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("seed hris employee should succeed: %v", err)
	}

	employee, created, err := svc.ResolveOrProvisionJITEmployee(
		"tenant_demo_jakarta",
		"jit.priority.fill@sudirman.co",
		"hris-jkt-4001",
		"OIDC Filled Name",
		"Facility",
		"Engineer",
		"Jakarta",
	)
	if err != nil {
		t.Fatalf("jit resolve should succeed: %v", err)
	}
	if created {
		t.Fatalf("expected existing hris employee to be reused")
	}
	if employee.Source != "hris_import" {
		t.Fatalf("expected source to remain hris_import, got %s", employee.Source)
	}
	if employee.FullName != "OIDC Filled Name" {
		t.Fatalf("expected empty full_name to be filled, got %s", employee.FullName)
	}
	if employee.Department != "Facility" || employee.JobTitle != "Engineer" || employee.Location != "Jakarta" {
		t.Fatalf(
			"expected empty profile to be filled, got department=%s job_title=%s location=%s",
			employee.Department,
			employee.JobTitle,
			employee.Location,
		)
	}
	if employee.AccessRole != "building_admin" {
		t.Fatalf("expected recalculated role building_admin after fill, got %s", employee.AccessRole)
	}
}

func TestResolveOrProvisionJITEmployeeKeepsSCIMSnapshotDeepAttributes(t *testing.T) {
	svc := NewService()

	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"scim_sync",
		"qa",
		[]EmployeeSyncInput{
			{
				ExternalID:        "scim-jkt-5001",
				Email:             "jit.priority.deep@sudirman.co",
				FullName:          "SCIM Deep Canonical",
				Department:        "Finance",
				JobTitle:          "Analyst",
				Location:          "Jakarta",
				Phone:             "+62811111111",
				ManagerExternalID: "mgr-scim-001",
				EmploymentStatus:  "active",
				Status:            "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("seed scim employee should succeed: %v", err)
	}

	employee, created, err := svc.ResolveOrProvisionJITEmployeeWithProfile(
		"tenant_demo_jakarta",
		"jit.priority.deep@sudirman.co",
		"scim-jkt-5001",
		JITProvisionProfile{
			FullName:          "OIDC Override Name",
			Department:        "Facility",
			JobTitle:          "Engineer",
			Location:          "Factory",
			Phone:             "+62899999999",
			ManagerExternalID: "mgr-oidc-override",
			EmploymentStatus:  "active",
		},
	)
	if err != nil {
		t.Fatalf("jit resolve should succeed: %v", err)
	}
	if created {
		t.Fatalf("expected existing scim employee to be reused")
	}
	if employee.Phone != "+62811111111" {
		t.Fatalf("expected phone to keep scim snapshot, got %s", employee.Phone)
	}
	if employee.ManagerExternalID != "mgr-scim-001" {
		t.Fatalf("expected manager_external_id to keep scim snapshot, got %s", employee.ManagerExternalID)
	}
}

func TestResolveOrProvisionJITEmployeeFillsEmptySCIMSnapshotDeepAttributes(t *testing.T) {
	svc := NewService()

	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"hris_import",
		"qa",
		[]EmployeeSyncInput{
			{
				ExternalID:        "hris-jkt-6001",
				Email:             "jit.priority.deep.fill@sudirman.co",
				FullName:          "HRIS Deep Empty",
				Department:        "",
				JobTitle:          "",
				Location:          "",
				Phone:             "",
				ManagerExternalID: "",
				EmploymentStatus:  "",
				Status:            "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("seed hris employee should succeed: %v", err)
	}

	employee, created, err := svc.ResolveOrProvisionJITEmployeeWithProfile(
		"tenant_demo_jakarta",
		"jit.priority.deep.fill@sudirman.co",
		"hris-jkt-6001",
		JITProvisionProfile{
			FullName:          "OIDC Filled Deep",
			Department:        "Facility",
			JobTitle:          "Engineer",
			Location:          "Jakarta",
			Phone:             "+628123456789",
			ManagerExternalID: "mgr-jit-6001",
			EmploymentStatus:  "active",
		},
	)
	if err != nil {
		t.Fatalf("jit resolve should succeed: %v", err)
	}
	if created {
		t.Fatalf("expected existing hris employee to be reused")
	}
	if employee.Phone != "+628123456789" {
		t.Fatalf("expected empty phone to be filled, got %s", employee.Phone)
	}
	if employee.ManagerExternalID != "mgr-jit-6001" {
		t.Fatalf("expected empty manager_external_id to be filled, got %s", employee.ManagerExternalID)
	}
	if employee.EmploymentStatus != "active" {
		t.Fatalf("expected employment_status active, got %s", employee.EmploymentStatus)
	}
}

func TestHasActiveJITEmployeeIdentity(t *testing.T) {
	svc := NewService()
	email := "jit.identity.lookup@sudirman.co"
	externalID := "hris-jkt-identity-001"

	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"hris_import",
		"qa",
		[]EmployeeSyncInput{
			{
				ExternalID: externalID,
				Email:      email,
				FullName:   "Identity Lookup User",
				Department: "IT",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("seed employee should succeed: %v", err)
	}

	matched, matchErr := svc.HasActiveJITEmployeeIdentity("tenant_demo_jakarta", email, externalID)
	if matchErr != nil {
		t.Fatalf("expected active employee identity match success: %v", matchErr)
	}
	if !matched {
		t.Fatalf("expected matched=true")
	}
}

func TestHasActiveJITEmployeeIdentityReturnsConflict(t *testing.T) {
	svc := NewService()
	email := "jit.identity.conflict@sudirman.co"

	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		[]EmployeeSyncInput{
			{
				ExternalID: "sub-jit-identity-conflict-a",
				Email:      email,
				FullName:   "Identity Conflict User",
				Department: "IT",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("seed employee should succeed: %v", err)
	}

	_, matchErr := svc.HasActiveJITEmployeeIdentity("tenant_demo_jakarta", email, "sub-jit-identity-conflict-b")
	if !errors.Is(matchErr, ErrEmployeeExternalIDConflict) {
		t.Fatalf("expected ErrEmployeeExternalIDConflict, got: %v", matchErr)
	}
}

func TestHasActiveJITEmployeeIdentityReturnsFalseWhenNotFound(t *testing.T) {
	svc := NewService()

	matched, err := svc.HasActiveJITEmployeeIdentity(
		"tenant_demo_jakarta",
		"jit.identity.missing@sudirman.co",
		"sub-jit-identity-missing-001",
	)
	if err != nil {
		t.Fatalf("expected missing lookup to return nil err, got: %v", err)
	}
	if matched {
		t.Fatalf("expected matched=false for missing identity")
	}
}

func TestNormalizeEmployeePhone(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "+62 811-1111-1111", want: "+6281111111111"},
		{input: "006281222222222", want: "+6281222222222"},
		{input: "6281333333333", want: "+6281333333333"},
		{input: "0812-1234-5678", want: "0812-1234-5678"},
	}
	for i := range tests {
		got := normalizeEmployeePhone(tests[i].input)
		if got != tests[i].want {
			t.Fatalf("input=%q expected=%q got=%q", tests[i].input, tests[i].want, got)
		}
	}
}
