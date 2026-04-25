package event

import (
	"testing"
	"time"
)

func TestIngestAccessEventIdempotency(t *testing.T) {
	svc := NewService()
	before := len(svc.ListAccessEvents("tenant_demo_jakarta"))

	first, deduped, err := svc.IngestAccessEvent(IngestAccessEventInput{
		IdempotencyKey: "rq-access-001",
		TenantID:       "tenant_demo_jakarta",
		BuildingID:     "building_demo_001",
		AreaID:         "area_demo_001",
		Type:           "access_granted",
		Actor:          "qa.user",
		DoorID:         "door_jkt_001",
		GatewayID:      "gw_demo_001",
		Result:         "success",
	})
	if err != nil {
		t.Fatalf("ingest access event #1 error: %v", err)
	}
	if deduped {
		t.Fatalf("expected first event not deduplicated")
	}
	if first.ID == "" {
		t.Fatalf("expected generated event id")
	}

	second, deduped, err := svc.IngestAccessEvent(IngestAccessEventInput{
		IdempotencyKey: "rq-access-001",
		TenantID:       "tenant_demo_jakarta",
		BuildingID:     "building_demo_001",
		Type:           "access_granted",
		Actor:          "qa.user.changed",
		DoorID:         "door_jkt_001",
		GatewayID:      "gw_demo_001",
		Result:         "success",
	})
	if err != nil {
		t.Fatalf("ingest access event #2 error: %v", err)
	}
	if !deduped {
		t.Fatalf("expected second event to be deduplicated")
	}
	if second.ID != first.ID {
		t.Fatalf("expected duplicated event id to match first, got first=%s second=%s", first.ID, second.ID)
	}

	after := len(svc.ListAccessEvents("tenant_demo_jakarta"))
	if after != before+1 {
		t.Fatalf("expected list size +1 after idempotent replay, before=%d after=%d", before, after)
	}
}

func TestIngestDeviceEventIdempotencyByEventID(t *testing.T) {
	svc := NewService()
	before := len(svc.ListDeviceEvents("tenant_demo_jakarta"))

	first, deduped, err := svc.IngestDeviceEvent(IngestDeviceEventInput{
		ID:         "gwed-fixed-001",
		TenantID:   "tenant_demo_jakarta",
		BuildingID: "building_demo_001",
		Type:       "gateway_event",
		GatewayID:  "gw_demo_001",
		Detail:     "timeout burst",
		Result:     "warning",
	})
	if err != nil {
		t.Fatalf("ingest device event #1 error: %v", err)
	}
	if deduped {
		t.Fatalf("expected first event not deduplicated")
	}

	second, deduped, err := svc.IngestDeviceEvent(IngestDeviceEventInput{
		ID:         "gwed-fixed-001",
		TenantID:   "tenant_demo_jakarta",
		BuildingID: "building_demo_001",
		Type:       "gateway_event",
		GatewayID:  "gw_demo_001",
		Detail:     "timeout burst retry",
		Result:     "warning",
	})
	if err != nil {
		t.Fatalf("ingest device event #2 error: %v", err)
	}
	if !deduped {
		t.Fatalf("expected second event to be deduplicated")
	}
	if second.ID != first.ID {
		t.Fatalf("expected duplicate to return same id, got first=%s second=%s", first.ID, second.ID)
	}

	after := len(svc.ListDeviceEvents("tenant_demo_jakarta"))
	if after != before+1 {
		t.Fatalf("expected list size +1 after duplicate replay, before=%d after=%d", before, after)
	}
}

func TestIngestAccessEventValidation(t *testing.T) {
	svc := NewService()

	_, _, err := svc.IngestAccessEvent(IngestAccessEventInput{
		TenantID:  "tenant_demo_jakarta",
		GatewayID: "gw_demo_001",
	})
	if err == nil {
		t.Fatalf("expected missing type to fail")
	}
	if err != ErrAccessEventTypeRequired {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCountEventsByGateway(t *testing.T) {
	svc := NewService()

	_, _, err := svc.IngestAccessEvent(IngestAccessEventInput{
		ID:        "gwea-count-1",
		TenantID:  "tenant_demo_jakarta",
		Type:      "access_granted",
		GatewayID: "gw_demo_001",
	})
	if err != nil {
		t.Fatalf("ingest access for count error: %v", err)
	}
	_, _, err = svc.IngestDeviceEvent(IngestDeviceEventInput{
		ID:        "gwed-count-1",
		TenantID:  "tenant_demo_jakarta",
		Type:      "gateway_event",
		GatewayID: "gw_demo_001",
	})
	if err != nil {
		t.Fatalf("ingest device for count error: %v", err)
	}

	accessCount, deviceCount := svc.CountEventsByGateway("tenant_demo_jakarta", "gw_demo_001")
	if accessCount < 1 || deviceCount < 1 {
		t.Fatalf("expected at least one access/device event, got access=%d device=%d", accessCount, deviceCount)
	}
}

func TestSubscribeChangesNotifiesOnMutation(t *testing.T) {
	svc := NewService()
	changeCh, unsubscribe := svc.SubscribeChanges()
	defer unsubscribe()

	if _, _, err := svc.IngestAccessEvent(IngestAccessEventInput{
		ID:        "gwea-subscribe-1",
		TenantID:  "tenant_demo_jakarta",
		Type:      "access_granted",
		GatewayID: "gw_demo_001",
	}); err != nil {
		t.Fatalf("ingest access event error: %v", err)
	}

	select {
	case <-changeCh:
	case <-time.After(time.Second):
		t.Fatalf("expected access event mutation to notify subscribers")
	}
}

func TestSubscribeChangesSkipsDeduplicatedMutation(t *testing.T) {
	svc := NewService()
	changeCh, unsubscribe := svc.SubscribeChanges()
	defer unsubscribe()

	if _, _, err := svc.IngestDeviceEvent(IngestDeviceEventInput{
		ID:        "gwed-subscribe-1",
		TenantID:  "tenant_demo_jakarta",
		Type:      "gateway_event",
		GatewayID: "gw_demo_001",
	}); err != nil {
		t.Fatalf("ingest device event error: %v", err)
	}

	select {
	case <-changeCh:
	case <-time.After(time.Second):
		t.Fatalf("expected initial device event mutation to notify subscribers")
	}

	if _, deduped, err := svc.IngestDeviceEvent(IngestDeviceEventInput{
		ID:        "gwed-subscribe-1",
		TenantID:  "tenant_demo_jakarta",
		Type:      "gateway_event",
		GatewayID: "gw_demo_001",
	}); err != nil {
		t.Fatalf("ingest duplicate device event error: %v", err)
	} else if !deduped {
		t.Fatalf("expected second device event to be deduplicated")
	}

	select {
	case <-changeCh:
		t.Fatalf("expected deduplicated device event not to notify subscribers")
	case <-time.After(100 * time.Millisecond):
	}
}
