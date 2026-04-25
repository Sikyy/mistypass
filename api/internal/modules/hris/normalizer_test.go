package hris_test

import (
	"errors"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/hris"
	"github.com/mistypass/cloud/api/internal/modules/hris/talenta"
)

func TestRegistryNormalizeWebhook(t *testing.T) {
	registry := hris.NewRegistry(talenta.NewNormalizer())

	result, err := registry.NormalizeWebhook(enterprise.HRISWebhookReceipt{
		ID:          "whr_001",
		TenantID:    "tenant_demo_jakarta",
		ConnectorID: "hrc_001",
		Vendor:      "talenta",
		EventType:   talenta.EventEmployeeDetailCreated,
		RequestID:   "mekari-evt-001",
		RawPayload: `{
			"event_type":"talenta.employee.detail.created",
			"employee":{
				"employment":{"employee_id":"EMP-001","organization_name":"IT Division","job_position":"Engineer","branch":"Jakarta HQ","status":"active"},
				"personal":{"first_name":"Arief","last_name":"Putra","email":"arief.putra@sudirman.co"}
			}
		}`,
	})
	if err != nil {
		t.Fatalf("expected normalize webhook to succeed: %v", err)
	}
	if result.Source != "hris_talenta" {
		t.Fatalf("expected source hris_talenta, got %s", result.Source)
	}
	if result.RequestID != "mekari-evt-001" {
		t.Fatalf("expected request_id mekari-evt-001, got %s", result.RequestID)
	}
	if result.RawPayloadRef != "hris_webhook_receipt:whr_001" {
		t.Fatalf("raw_payload_ref mismatch: %s", result.RawPayloadRef)
	}
	if len(result.Employees) != 1 {
		t.Fatalf("expected one employee, got %d", len(result.Employees))
	}
}

func TestRegistryNormalizeWebhookNotFound(t *testing.T) {
	registry := hris.NewRegistry()

	_, err := registry.NormalizeWebhook(enterprise.HRISWebhookReceipt{Vendor: "unknown"})
	if !errors.Is(err, hris.ErrNormalizerNotFound) {
		t.Fatalf("expected ErrNormalizerNotFound, got %v", err)
	}
}

func TestStableRequestIDFallback(t *testing.T) {
	receipt := enterprise.HRISWebhookReceipt{
		TenantID:  "tenant_demo_jakarta",
		Vendor:    "talenta",
		EventType: talenta.EventEmployeeTransferApproved,
	}

	requestID := hris.StableRequestID(receipt, "EMP-001", "2026-04-22T10:20:00Z")
	expected := "talenta:tenant_demo_jakarta:talenta.employee.transfer.approved:emp-001:2026-04-22t10:20:00z"
	if requestID != expected {
		t.Fatalf("expected stable request_id %s, got %s", expected, requestID)
	}
}
