package gateway

import (
	"strings"
	"testing"
)

func TestCreateAndGetFirmware(t *testing.T) {
	svc := NewService()
	fw, err := svc.CreateFirmware(CreateFirmwareInput{
		TenantID:  "tenant_demo_jakarta",
		Version:   "1.4.0",
		Channel:   "stable",
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Signature: strings.Repeat("b", 128),
		SizeBytes: 123,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if fw.ID == "" || fw.Version != "1.4.0" || fw.Channel != "stable" {
		t.Fatalf("unexpected fw: %+v", fw)
	}
	got, err := svc.GetFirmware("tenant_demo_jakarta", fw.ID)
	if err != nil || got.ID != fw.ID {
		t.Fatalf("get: %v %+v", err, got)
	}
	if _, err := svc.GetFirmware("tenant_other", fw.ID); err != ErrGatewayFirmwareNotFound {
		t.Fatalf("cross-tenant get should fail, got %v", err)
	}
	byID, err := svc.GetFirmwareByID(fw.ID)
	if err != nil || byID.ID != fw.ID {
		t.Fatalf("get-by-id: %v %+v", err, byID)
	}
}

func TestCreateFirmwareValidation(t *testing.T) {
	svc := NewService()
	if _, err := svc.CreateFirmware(CreateFirmwareInput{TenantID: "t", Version: ""}); err != ErrGatewayFirmwareVersionRequired {
		t.Fatalf("want version-required, got %v", err)
	}
	if _, err := svc.CreateFirmware(CreateFirmwareInput{TenantID: "t", Version: "1.0.0", SHA256: "short"}); err != ErrGatewayFirmwareSHA256Invalid {
		t.Fatalf("want sha-invalid, got %v", err)
	}
	if _, err := svc.CreateFirmware(CreateFirmwareInput{TenantID: "t", Version: "1.0.0", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Signature: "bad"}); err != ErrGatewayFirmwareSignatureInvalid {
		t.Fatalf("want sig-invalid, got %v", err)
	}
}

func TestListFirmwareByChannel(t *testing.T) {
	svc := NewService()
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sig := strings.Repeat("b", 128)
	_, _ = svc.CreateFirmware(CreateFirmwareInput{TenantID: "t1", Version: "1.0.0", Channel: "stable", SHA256: sha, Signature: sig})
	_, _ = svc.CreateFirmware(CreateFirmwareInput{TenantID: "t1", Version: "1.1.0", Channel: "beta", SHA256: sha, Signature: sig})
	if got := svc.ListFirmware("t1", ""); len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got := svc.ListFirmware("t1", "beta"); len(got) != 1 || got[0].Channel != "beta" {
		t.Fatalf("want 1 beta, got %+v", got)
	}
	if got := svc.ListFirmware("t2", ""); len(got) != 0 {
		t.Fatalf("tenant isolation broken, got %d", len(got))
	}
}
