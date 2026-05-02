package wallet

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApplePassProviderDefaultConfig(t *testing.T) {
	p := NewApplePassProvider(ApplePassConfig{})
	if p.TeamID != "MISTYISLET_DEV" {
		t.Fatalf("expected default TeamID, got %q", p.TeamID)
	}
	if p.PassTypeID != "pass.com.mistyislet.access" {
		t.Fatalf("expected default PassTypeID, got %q", p.PassTypeID)
	}
	if p.OrganizationID != "mistyislet" {
		t.Fatalf("expected default OrganizationID, got %q", p.OrganizationID)
	}
	if p.WebServiceURL != "https://api.mistyislet.com/v1/passes" {
		t.Fatalf("expected default WebServiceURL, got %q", p.WebServiceURL)
	}
}

func TestApplePassProviderCustomConfig(t *testing.T) {
	p := NewApplePassProvider(ApplePassConfig{
		TeamID:     "CUSTOM_TEAM",
		PassTypeID: "pass.com.custom",
	})
	if p.TeamID != "CUSTOM_TEAM" {
		t.Fatalf("expected custom TeamID, got %q", p.TeamID)
	}
	if p.PassTypeID != "pass.com.custom" {
		t.Fatalf("expected custom PassTypeID, got %q", p.PassTypeID)
	}
}

func TestApplePassProviderIssuePassBundleStructure(t *testing.T) {
	p := NewApplePassProvider(ApplePassConfig{})
	bundle, err := p.IssuePass("tenant1", "Alice", "alice@example.com", "pass-001")
	if err != nil {
		t.Fatalf("IssuePass: %v", err)
	}

	if len(bundle.SerialNumber) != 24 {
		t.Fatalf("serial should be 24 hex chars (12 bytes), got %d: %q", len(bundle.SerialNumber), bundle.SerialNumber)
	}
	if len(bundle.AuthToken) != 32 {
		t.Fatalf("auth token should be 32 hex chars (16 bytes), got %d", len(bundle.AuthToken))
	}
	if !bundle.MockProvider {
		t.Fatal("expected MockProvider true")
	}
	if bundle.BundleSize <= 0 {
		t.Fatal("expected positive BundleSize")
	}
	if bundle.CreatedAt == "" {
		t.Fatal("expected CreatedAt")
	}
}

func TestApplePassProviderIssuePassJSON(t *testing.T) {
	p := NewApplePassProvider(ApplePassConfig{})
	bundle, err := p.IssuePass("tenant1", "Bob", "bob@test.com", "pass-002")
	if err != nil {
		t.Fatalf("IssuePass: %v", err)
	}

	var passData map[string]any
	if err := json.Unmarshal([]byte(bundle.PassJSON), &passData); err != nil {
		t.Fatalf("pass.json is not valid JSON: %v", err)
	}
	if passData["passTypeIdentifier"] != "pass.com.mistyislet.access" {
		t.Fatalf("unexpected passTypeIdentifier: %v", passData["passTypeIdentifier"])
	}
	if passData["teamIdentifier"] != "MISTYISLET_DEV" {
		t.Fatalf("unexpected teamIdentifier: %v", passData["teamIdentifier"])
	}
	if passData["serialNumber"] != bundle.SerialNumber {
		t.Fatal("passJSON serialNumber mismatch")
	}
}

func TestApplePassProviderIssuePassManifest(t *testing.T) {
	p := NewApplePassProvider(ApplePassConfig{})
	bundle, _ := p.IssuePass("t", "Name", "e@x.com", "p1")

	var manifest map[string]string
	if err := json.Unmarshal([]byte(bundle.ManifestJSON), &manifest); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if _, ok := manifest["pass.json"]; !ok {
		t.Fatal("manifest missing pass.json hash")
	}
	if len(manifest["pass.json"]) != 64 {
		t.Fatalf("pass.json hash should be 64 hex chars (SHA256), got %d", len(manifest["pass.json"]))
	}
}

func TestApplePassProviderIssuePassNfcPayload(t *testing.T) {
	p := NewApplePassProvider(ApplePassConfig{})
	bundle, _ := p.IssuePass("t", "Name", "e@x.com", "pass-nfc")

	if !strings.HasPrefix(bundle.NfcPayload, "mistyislet:pass-nfc:") {
		t.Fatalf("NFC payload should start with 'mistyislet:pass-nfc:', got %q", bundle.NfcPayload)
	}
}

func TestApplePassProviderIssuePassSignature(t *testing.T) {
	p := NewApplePassProvider(ApplePassConfig{})
	bundle, _ := p.IssuePass("t", "Name", "e@x.com", "p1")

	if !strings.HasPrefix(bundle.Signature, "MOCK_PKCS7_") {
		t.Fatalf("mock signature should start with MOCK_PKCS7_, got %q", bundle.Signature)
	}
}

func TestApplePassProviderRegisterDeviceFields(t *testing.T) {
	p := NewApplePassProvider(ApplePassConfig{})
	reg, err := p.RegisterDevice("serial-1", "dev-lib-1", "push-token-1")
	if err != nil {
		t.Fatalf("RegisterDevice: %v", err)
	}
	if reg.SerialNumber != "serial-1" {
		t.Fatalf("expected serial-1, got %q", reg.SerialNumber)
	}
	if reg.DeviceLibraryID != "dev-lib-1" {
		t.Fatalf("expected dev-lib-1, got %q", reg.DeviceLibraryID)
	}
	if reg.PushToken != "push-token-1" {
		t.Fatalf("expected push-token-1, got %q", reg.PushToken)
	}
	if reg.PassTypeID != "pass.com.mistyislet.access" {
		t.Fatalf("expected default PassTypeID, got %q", reg.PassTypeID)
	}
	if reg.RegisteredAt == "" {
		t.Fatal("expected RegisteredAt")
	}
}

func TestApplePassProviderNotifyPassUpdateNoop(t *testing.T) {
	p := NewApplePassProvider(ApplePassConfig{})
	if err := p.NotifyPassUpdate("token", "serial"); err != nil {
		t.Fatalf("NotifyPassUpdate should be no-op, got %v", err)
	}
}
