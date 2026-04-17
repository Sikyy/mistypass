package enterprise

import "testing"

func TestGetEmployeeByEmailActiveFound(t *testing.T) {
	svc := NewService()

	employee, err := svc.GetEmployeeByEmail("tenant_demo_jakarta", "ARIEF.PUTRA@sudirman.co")
	if err != nil {
		t.Fatalf("expected employee lookup to succeed: %v", err)
	}
	if employee.ID != "emp_001" {
		t.Fatalf("unexpected employee id: got=%s want=%s", employee.ID, "emp_001")
	}
	if employee.Status != "active" {
		t.Fatalf("expected active employee, got status=%s", employee.Status)
	}
}

func TestGetEmployeeByEmailNotFound(t *testing.T) {
	svc := NewService()

	_, err := svc.GetEmployeeByEmail("tenant_demo_jakarta", "missing.user@sudirman.co")
	if err == nil {
		t.Fatalf("expected missing employee lookup to fail")
	}
	if err != ErrEmployeeNotFound {
		t.Fatalf("unexpected error: %v", err)
	}
}
