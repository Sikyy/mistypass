package alarm

import (
	"testing"
	"time"
)

func TestSubscribeChangesNotifiesOnStatusUpdate(t *testing.T) {
	svc := NewService()
	changeCh, unsubscribe := svc.SubscribeChanges()
	defer unsubscribe()

	if _, err := svc.UpdateStatus("tenant_demo_jakarta", "alm_9002", "resolved"); err != nil {
		t.Fatalf("update alarm status error: %v", err)
	}

	select {
	case <-changeCh:
	case <-time.After(time.Second):
		t.Fatalf("expected alarm status mutation to notify subscribers")
	}
}

func TestSubscribeChangesSkipsInvalidUpdate(t *testing.T) {
	svc := NewService()
	changeCh, unsubscribe := svc.SubscribeChanges()
	defer unsubscribe()

	if _, err := svc.UpdateStatus("tenant_demo_jakarta", "alm_9002", "invalid_status"); err == nil {
		t.Fatalf("expected invalid alarm status to fail")
	}

	select {
	case <-changeCh:
		t.Fatalf("expected invalid alarm update not to notify subscribers")
	case <-time.After(100 * time.Millisecond):
	}
}
