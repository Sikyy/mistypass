package wallet

import (
	"strings"
	"testing"
)

func TestGoogleWalletProviderDefaultConfig(t *testing.T) {
	p := NewGoogleWalletProvider(GoogleWalletConfig{})
	if p.IssuerID != "mistyislet_dev_issuer" {
		t.Fatalf("expected default IssuerID, got %q", p.IssuerID)
	}
	if p.ServiceAccountEmail != "wallet@mistyislet-dev.iam.gserviceaccount.com" {
		t.Fatalf("expected default ServiceAccountEmail, got %q", p.ServiceAccountEmail)
	}
	if p.ClassPrefix != "mistyislet.access" {
		t.Fatalf("expected default ClassPrefix, got %q", p.ClassPrefix)
	}
}

func TestGoogleWalletProviderCustomConfig(t *testing.T) {
	p := NewGoogleWalletProvider(GoogleWalletConfig{
		IssuerID:    "custom_issuer",
		ClassPrefix: "custom.prefix",
	})
	if p.IssuerID != "custom_issuer" {
		t.Fatalf("expected custom IssuerID, got %q", p.IssuerID)
	}
	if p.ClassPrefix != "custom.prefix" {
		t.Fatalf("expected custom ClassPrefix, got %q", p.ClassPrefix)
	}
}

func TestGoogleWalletProviderCreatePassClass(t *testing.T) {
	p := NewGoogleWalletProvider(GoogleWalletConfig{})
	cls, err := p.CreatePassClass("tenant1", "tmpl-001", "Access Template")
	if err != nil {
		t.Fatalf("CreatePassClass: %v", err)
	}

	expectedID := "mistyislet_dev_issuer.mistyislet.access.class.tmpl-001"
	if cls.ID != expectedID {
		t.Fatalf("expected class ID %q, got %q", expectedID, cls.ID)
	}
	if cls.ClassTemplateID != "tmpl-001" {
		t.Fatalf("expected template ID tmpl-001, got %q", cls.ClassTemplateID)
	}
	if cls.ReviewStatus != "UNDER_REVIEW" {
		t.Fatalf("expected UNDER_REVIEW, got %q", cls.ReviewStatus)
	}
	if !cls.NfcEnabled {
		t.Fatal("expected NfcEnabled true")
	}
	if !cls.MockProvider {
		t.Fatal("expected MockProvider true")
	}
	if cls.CreatedAt == "" {
		t.Fatal("expected CreatedAt")
	}
}

func TestGoogleWalletProviderIssuePassObjectFields(t *testing.T) {
	p := NewGoogleWalletProvider(GoogleWalletConfig{})
	obj, err := p.IssuePassObject("tenant1", "pass-001", "class-001", "Alice", "alice@test.com")
	if err != nil {
		t.Fatalf("IssuePassObject: %v", err)
	}

	expectedObjectID := "mistyislet_dev_issuer.mistyislet.access.pass-001"
	if obj.ObjectID != expectedObjectID {
		t.Fatalf("expected objectID %q, got %q", expectedObjectID, obj.ObjectID)
	}
	if obj.ClassID != "class-001" {
		t.Fatalf("expected classID class-001, got %q", obj.ClassID)
	}
	if obj.HolderName != "Alice" {
		t.Fatalf("expected holder Alice, got %q", obj.HolderName)
	}
	if obj.HolderEmail != "alice@test.com" {
		t.Fatalf("expected email alice@test.com, got %q", obj.HolderEmail)
	}
	if obj.State != "ACTIVE" {
		t.Fatalf("expected ACTIVE state, got %q", obj.State)
	}
	if !obj.MockProvider {
		t.Fatal("expected MockProvider true")
	}
}

func TestGoogleWalletProviderIssuePassObjectSaveLink(t *testing.T) {
	p := NewGoogleWalletProvider(GoogleWalletConfig{})
	obj, _ := p.IssuePassObject("t", "p1", "c1", "Name", "e@x.com")

	if !strings.HasPrefix(obj.SaveLink, "https://pay.google.com/gp/v/save/") {
		t.Fatalf("save link should start with Google Wallet URL, got %q", obj.SaveLink)
	}
	if obj.SaveLinkJWT == "" {
		t.Fatal("expected non-empty SaveLinkJWT")
	}
}

func TestGoogleWalletProviderIssuePassObjectJWTStructure(t *testing.T) {
	p := NewGoogleWalletProvider(GoogleWalletConfig{})
	obj, _ := p.IssuePassObject("t", "p1", "c1", "Name", "e@x.com")

	parts := strings.Split(obj.SaveLinkJWT, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT should have 3 dot-separated parts, got %d", len(parts))
	}
	for i, part := range parts {
		if part == "" {
			t.Fatalf("JWT part %d is empty", i)
		}
	}
}

func TestGoogleWalletProviderIssuePassObjectNfcPayload(t *testing.T) {
	p := NewGoogleWalletProvider(GoogleWalletConfig{})
	obj, _ := p.IssuePassObject("t", "pass-nfc", "c1", "Name", "e@x.com")

	if !strings.HasPrefix(obj.NfcPayload, "mistyislet:pass-nfc:") {
		t.Fatalf("NFC payload should start with 'mistyislet:pass-nfc:', got %q", obj.NfcPayload)
	}
}

func TestGoogleWalletProviderIssuePassObjectBarcode(t *testing.T) {
	p := NewGoogleWalletProvider(GoogleWalletConfig{})
	obj, _ := p.IssuePassObject("t", "p1", "c1", "Name", "e@x.com")

	if len(obj.Barcode) != 12 {
		t.Fatalf("barcode should be 12 hex chars (6 bytes), got %d: %q", len(obj.Barcode), obj.Barcode)
	}
}

func TestGoogleWalletProviderUpdatePassObjectState(t *testing.T) {
	p := NewGoogleWalletProvider(GoogleWalletConfig{})
	result, err := p.UpdatePassObjectState("obj-001", " inactive ")
	if err != nil {
		t.Fatalf("UpdatePassObjectState: %v", err)
	}
	if result.ObjectID != "obj-001" {
		t.Fatalf("expected obj-001, got %q", result.ObjectID)
	}
	if result.State != "INACTIVE" {
		t.Fatalf("expected INACTIVE (uppercased), got %q", result.State)
	}
	if result.UpdatedAt == "" {
		t.Fatal("expected UpdatedAt")
	}
}

func TestMockJWTSignStructure(t *testing.T) {
	jwt := mockJWTSign([]byte(`{"test":"data"}`))
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("mock JWT should have 3 parts, got %d", len(parts))
	}
}
