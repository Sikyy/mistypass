package domain

import (
	"strings"
	"testing"
)

func TestDefaultIDGenerator_Format(t *testing.T) {
	id, err := DefaultIDGenerator()
	if err != nil {
		t.Fatalf("DefaultIDGenerator returned error: %v", err)
	}
	if !strings.HasPrefix(id, "tenant_") {
		t.Fatalf("expected prefix tenant_, got %q", id)
	}
	// 6 random bytes -> 12 hex chars + "tenant_" prefix = 19 chars total
	if len(id) != 19 {
		t.Fatalf("expected 19 chars (tenant_ + 12 hex), got %d: %q", len(id), id)
	}
}

func TestDefaultIDGenerator_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := DefaultIDGenerator()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if seen[id] {
			t.Fatalf("duplicate ID generated: %q", id)
		}
		seen[id] = true
	}
}

func TestNormalizeStatus_ValidStatuses(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"active", "active"},
		{"Active", "active"},
		{"ACTIVE", "active"},
		{"  active  ", "active"},
		{"suspended", "suspended"},
		{"Suspended", "suspended"},
		{"  SUSPENDED  ", "suspended"},
		{"inactive", "inactive"},
		{"Inactive", "inactive"},
	}
	for _, tc := range tests {
		got, err := NormalizeStatus(tc.input)
		if err != nil {
			t.Errorf("NormalizeStatus(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.expected {
			t.Errorf("NormalizeStatus(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestNormalizeStatus_Invalid(t *testing.T) {
	invalids := []string{"", "  ", "deleted", "pending", "enabled", "disabled"}
	for _, input := range invalids {
		_, err := NormalizeStatus(input)
		if err != ErrInvalidTenantStatus {
			t.Errorf("NormalizeStatus(%q) expected ErrInvalidTenantStatus, got %v", input, err)
		}
	}
}

func TestNormalizeType_ValidTypes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "company"},
		{"company", "company"},
		{"Company", "company"},
		{"COMPANY", "company"},
		{"  company  ", "company"},
		{"studio", "studio"},
		{"Studio", "studio"},
		{"government", "government"},
		{"factory", "factory"},
		{"public_facility", "public_facility"},
		{"PUBLIC_FACILITY", "public_facility"},
	}
	for _, tc := range tests {
		got, err := NormalizeType(tc.input)
		if err != nil {
			t.Errorf("NormalizeType(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.expected {
			t.Errorf("NormalizeType(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestNormalizeType_Invalid(t *testing.T) {
	invalids := []string{"hospital", "school", "residential", "enterprise"}
	for _, input := range invalids {
		_, err := NormalizeType(input)
		if err != ErrInvalidTenantType {
			t.Errorf("NormalizeType(%q) expected ErrInvalidTenantType, got %v", input, err)
		}
	}
}

func TestCloneTenants_ReturnsIndependentCopy(t *testing.T) {
	original := []Tenant{
		{ID: "t1", Name: "One", Status: "active"},
		{ID: "t2", Name: "Two", Status: "active"},
	}

	cloned := CloneTenants(original)

	if len(cloned) != len(original) {
		t.Fatalf("expected %d tenants, got %d", len(original), len(cloned))
	}

	// Mutate the clone and verify original is unaffected
	cloned[0].Name = "MUTATED"
	if original[0].Name == "MUTATED" {
		t.Fatal("mutating clone affected the original slice")
	}

	// Mutate original and verify clone is unaffected
	original[1].Status = "suspended"
	if cloned[1].Status == "suspended" {
		t.Fatal("mutating original affected the clone")
	}
}

func TestCloneTenants_EmptySlice(t *testing.T) {
	cloned := CloneTenants([]Tenant{})
	if cloned == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(cloned) != 0 {
		t.Fatalf("expected 0 tenants, got %d", len(cloned))
	}
}

func TestCloneTenants_NilSlice(t *testing.T) {
	cloned := CloneTenants(nil)
	if cloned == nil {
		t.Fatal("expected non-nil empty slice from nil input")
	}
	if len(cloned) != 0 {
		t.Fatalf("expected 0 tenants, got %d", len(cloned))
	}
}
