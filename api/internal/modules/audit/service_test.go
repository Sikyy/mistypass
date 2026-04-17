package audit

import "testing"

func TestListFilteredByActionSourceAndLimit(t *testing.T) {
	svc := NewService()

	_, err := svc.Append(
		"tenant_demo_jakarta",
		"enterprise_sync_worker",
		"system",
		"enterprise_sync_reconcile_worker_alert",
		"failed=3 threshold=3",
		"enterprise_sync_worker",
	)
	if err != nil {
		t.Fatalf("append worker alert audit should succeed: %v", err)
	}
	_, err = svc.Append(
		"tenant_demo_jakarta",
		"ops.jkt.01@mistypass.local",
		"operator",
		"gateway_reboot",
		"gw_demo_001",
		"web_admin",
	)
	if err != nil {
		t.Fatalf("append gateway reboot audit should succeed: %v", err)
	}

	filtered := svc.ListFiltered(
		"tenant_demo_jakarta",
		"enterprise_sync_reconcile_worker_alert",
		"enterprise_sync_worker",
		0,
	)
	if len(filtered) != 1 {
		t.Fatalf("expected one filtered worker alert log, got %d", len(filtered))
	}
	if filtered[0].Action != "enterprise_sync_reconcile_worker_alert" {
		t.Fatalf("unexpected filtered action: %s", filtered[0].Action)
	}
	if filtered[0].Source != "enterprise_sync_worker" {
		t.Fatalf("unexpected filtered source: %s", filtered[0].Source)
	}

	limited := svc.ListFiltered("tenant_demo_jakarta", "", "", 1)
	if len(limited) != 1 {
		t.Fatalf("expected limit=1 to return one item, got %d", len(limited))
	}
}
