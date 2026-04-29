package wallet

import (
	"strings"
	"testing"
)

func TestApplePassProviderIssuePass(t *testing.T) {
	provider := NewApplePassProvider(ApplePassConfig{
		TeamID:     "TEST_TEAM",
		PassTypeID: "pass.com.test.access",
	})

	bundle, err := provider.IssuePass("tenant_test", "Alice", "alice@test.local", "wps_test_001")
	if err != nil {
		t.Fatalf("issue pass: %v", err)
	}
	if bundle.SerialNumber == "" {
		t.Error("expected serial number")
	}
	if bundle.AuthToken == "" {
		t.Error("expected auth token")
	}
	if !strings.Contains(bundle.NfcPayload, "mistyislet:wps_test_001:") {
		t.Errorf("expected nfc payload with pass ID, got: %s", bundle.NfcPayload)
	}
	if bundle.SaveLink == "" {
		t.Error("expected save link")
	}
	if !bundle.MockProvider {
		t.Error("expected mock_provider=true")
	}

	// verify pass.json structure
	if !strings.Contains(bundle.PassJSON, "pass.com.test.access") {
		t.Error("pass.json should contain pass type ID")
	}
	if !strings.Contains(bundle.PassJSON, "Alice") {
		t.Error("pass.json should contain holder name")
	}
	if !strings.Contains(bundle.PassJSON, bundle.AuthToken) {
		t.Error("pass.json should contain auth token")
	}

	// manifest should reference pass.json
	if !strings.Contains(bundle.ManifestJSON, "pass.json") {
		t.Error("manifest should reference pass.json")
	}

	// signature should be mock
	if !strings.HasPrefix(bundle.Signature, "MOCK_PKCS7_") {
		t.Error("expected mock signature prefix")
	}
}

func TestApplePassProviderRegisterDevice(t *testing.T) {
	provider := NewApplePassProvider(ApplePassConfig{})

	reg, err := provider.RegisterDevice("serial_123", "device_lib_abc", "push_token_xyz")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if reg.SerialNumber != "serial_123" {
		t.Errorf("expected serial_123, got %s", reg.SerialNumber)
	}
	if reg.PushToken != "push_token_xyz" {
		t.Errorf("expected push_token_xyz, got %s", reg.PushToken)
	}
}

func TestGoogleWalletProviderIssuePassObject(t *testing.T) {
	provider := NewGoogleWalletProvider(GoogleWalletConfig{
		IssuerID: "test_issuer",
	})

	// create class first
	class, err := provider.CreatePassClass("tenant_test", "tpl_001", "Employee Access")
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	if !strings.Contains(class.ID, "test_issuer") {
		t.Errorf("class ID should contain issuer ID, got: %s", class.ID)
	}
	if !class.MockProvider {
		t.Error("expected mock_provider=true")
	}

	// issue pass object
	obj, err := provider.IssuePassObject("tenant_test", "wps_test_002", class.ID, "Bob", "bob@test.local")
	if err != nil {
		t.Fatalf("issue object: %v", err)
	}
	if obj.ObjectID == "" {
		t.Error("expected object ID")
	}
	if !strings.Contains(obj.NfcPayload, "mistyislet:wps_test_002:") {
		t.Errorf("expected nfc payload with pass ID, got: %s", obj.NfcPayload)
	}
	if obj.SaveLink == "" {
		t.Error("expected save link")
	}
	if !strings.HasPrefix(obj.SaveLink, "https://pay.google.com/gp/v/save/") {
		t.Errorf("expected Google Pay save link, got: %s", obj.SaveLink)
	}
	// verify JWT structure in save link
	jwt := strings.TrimPrefix(obj.SaveLink, "https://pay.google.com/gp/v/save/")
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Errorf("expected 3-part JWT, got %d parts", len(parts))
	}
	if !obj.MockProvider {
		t.Error("expected mock_provider=true")
	}
}

func TestGoogleWalletProviderUpdateState(t *testing.T) {
	provider := NewGoogleWalletProvider(GoogleWalletConfig{})

	result, err := provider.UpdatePassObjectState("obj_123", "INACTIVE")
	if err != nil {
		t.Fatalf("update state: %v", err)
	}
	if result.State != "INACTIVE" {
		t.Errorf("expected INACTIVE, got %s", result.State)
	}
}

func TestIssuePassUsesGoogleProvider(t *testing.T) {
	svc := NewService()

	pass, err := svc.IssuePass("tenant_demo_jakarta", "wpt_employee_demo", "user", "usr_1001", "", "test")
	if err != nil {
		t.Fatalf("issue pass: %v", err)
	}
	if pass.SaveLink == "" {
		t.Error("expected save link from google provider")
	}
	if !strings.HasPrefix(pass.SaveLink, "https://pay.google.com/gp/v/save/") {
		t.Errorf("expected Google Pay save link, got: %s", pass.SaveLink)
	}
	if pass.Token == "" {
		t.Error("expected NFC token from provider")
	}
	if !strings.Contains(pass.Token, "mistyislet:") {
		t.Errorf("expected mistyislet NFC payload, got: %s", pass.Token)
	}
}
