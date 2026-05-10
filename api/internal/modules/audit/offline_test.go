package audit

import (
	"context"
	"testing"
	"time"
)

func TestIngestOfflineBatch_AcceptsValidEntries(t *testing.T) {
	svc := NewService()
	initialCount := len(svc.List(""))

	entries := []OfflineAuditEntry{
		{
			EventID:   "evt_001",
			UserID:    "usr_1001",
			LockID:    "lock_front_door",
			Method:    "ble",
			Result:    "granted",
			Reason:    "valid credential",
			GatewayID: 1,
			Timestamp: time.Now().Add(-10 * time.Minute).Unix(),
			IsOffline: true,
		},
		{
			EventID:   "evt_002",
			UserID:    "usr_1002",
			LockID:    "lock_back_door",
			Method:    "nfc_hce",
			Result:    "denied",
			Reason:    "credential expired",
			GatewayID: 1,
			Timestamp: time.Now().Add(-5 * time.Minute).Unix(),
			IsOffline: true,
		},
	}

	accepted, err := svc.IngestOfflineBatch(context.Background(), entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accepted != 2 {
		t.Fatalf("expected 2 accepted, got %d", accepted)
	}

	allLogs := svc.List("")
	if len(allLogs) != initialCount+2 {
		t.Fatalf("expected %d total logs, got %d", initialCount+2, len(allLogs))
	}
}

func TestIngestOfflineBatchForGatewayUsesContextAndDeduplicatesEventID(t *testing.T) {
	svc := NewService()
	entries := []OfflineAuditEntry{
		{
			EventID:   "evt_context_001",
			UserID:    "usr_1001",
			LockID:    "door_jkt_001",
			Method:    "ble",
			Result:    "granted",
			GatewayID: 1,
			Timestamp: time.Now().Unix(),
			IsOffline: true,
		},
		{
			EventID:   "evt_context_001",
			UserID:    "usr_1001",
			LockID:    "door_jkt_001",
			Method:    "ble",
			Result:    "granted",
			GatewayID: 1,
			Timestamp: time.Now().Unix(),
			IsOffline: true,
		},
	}

	accepted, err := svc.IngestOfflineBatchForGateway(context.Background(), OfflineBatchContext{
		TenantID:  "tenant_demo_jakarta",
		GatewayID: "gw_demo_001",
	}, entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accepted != 1 {
		t.Fatalf("expected duplicate event_id to be accepted once, got %d", accepted)
	}

	logs := svc.List("tenant_demo_jakarta")
	if len(logs) == 0 {
		t.Fatalf("expected tenant-scoped offline audit log")
	}
	if logs[0].Target != "door_jkt_001 event_id=evt_context_001 gateway_id=gw_demo_001" {
		t.Fatalf("expected event and gateway context in target, got %q", logs[0].Target)
	}
}

func TestIngestOfflineBatch_SkipsInvalidEntries(t *testing.T) {
	svc := NewService()

	entries := []OfflineAuditEntry{
		{
			// Missing EventID — invalid
			UserID:    "usr_1001",
			LockID:    "lock_front_door",
			Method:    "ble",
			Result:    "granted",
			Timestamp: time.Now().Unix(),
		},
		{
			EventID:   "evt_valid",
			UserID:    "usr_1001",
			LockID:    "lock_front_door",
			Method:    "ble",
			Result:    "granted",
			Reason:    "valid",
			GatewayID: 1,
			Timestamp: time.Now().Unix(),
			IsOffline: true,
		},
		{
			// Invalid method
			EventID:   "evt_bad_method",
			UserID:    "usr_1001",
			LockID:    "lock_front_door",
			Method:    "wifi",
			Result:    "granted",
			Timestamp: time.Now().Unix(),
		},
	}

	accepted, err := svc.IngestOfflineBatch(context.Background(), entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accepted != 1 {
		t.Fatalf("expected 1 accepted (2 invalid skipped), got %d", accepted)
	}
}

func TestIngestOfflineBatch_EmptyBatch(t *testing.T) {
	svc := NewService()

	accepted, err := svc.IngestOfflineBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accepted != 0 {
		t.Fatalf("expected 0 accepted for empty batch, got %d", accepted)
	}
}

func TestIngestOfflineBatch_CancelledContext(t *testing.T) {
	svc := NewService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	entries := []OfflineAuditEntry{
		{
			EventID:   "evt_001",
			UserID:    "usr_1001",
			LockID:    "lock_front_door",
			Method:    "ble",
			Result:    "granted",
			Reason:    "valid",
			GatewayID: 1,
			Timestamp: time.Now().Unix(),
			IsOffline: true,
		},
	}

	_, err := svc.IngestOfflineBatch(ctx, entries)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestValidateOfflineEntry(t *testing.T) {
	tests := []struct {
		name    string
		entry   OfflineAuditEntry
		wantErr bool
	}{
		{
			name: "valid ble granted",
			entry: OfflineAuditEntry{
				EventID: "evt_001", UserID: "usr_1", LockID: "lock_1",
				Method: "ble", Result: "granted", Timestamp: 1700000000,
			},
			wantErr: false,
		},
		{
			name: "valid nfc_hce denied",
			entry: OfflineAuditEntry{
				EventID: "evt_002", UserID: "usr_2", LockID: "lock_2",
				Method: "nfc_hce", Result: "denied", Timestamp: 1700000000,
			},
			wantErr: false,
		},
		{
			name: "missing event_id",
			entry: OfflineAuditEntry{
				UserID: "usr_1", LockID: "lock_1",
				Method: "ble", Result: "granted", Timestamp: 1700000000,
			},
			wantErr: true,
		},
		{
			name: "missing user_id",
			entry: OfflineAuditEntry{
				EventID: "evt_001", LockID: "lock_1",
				Method: "ble", Result: "granted", Timestamp: 1700000000,
			},
			wantErr: true,
		},
		{
			name: "missing lock_id",
			entry: OfflineAuditEntry{
				EventID: "evt_001", UserID: "usr_1",
				Method: "ble", Result: "granted", Timestamp: 1700000000,
			},
			wantErr: true,
		},
		{
			name: "invalid method",
			entry: OfflineAuditEntry{
				EventID: "evt_001", UserID: "usr_1", LockID: "lock_1",
				Method: "wifi", Result: "granted", Timestamp: 1700000000,
			},
			wantErr: true,
		},
		{
			name: "invalid result",
			entry: OfflineAuditEntry{
				EventID: "evt_001", UserID: "usr_1", LockID: "lock_1",
				Method: "ble", Result: "unknown", Timestamp: 1700000000,
			},
			wantErr: true,
		},
		{
			name: "zero timestamp",
			entry: OfflineAuditEntry{
				EventID: "evt_001", UserID: "usr_1", LockID: "lock_1",
				Method: "ble", Result: "granted", Timestamp: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOfflineEntry(tt.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateOfflineEntry() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAppendWithTimestamp(t *testing.T) {
	svc := NewService()
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	record, err := svc.AppendWithTimestamp(
		"tenant_demo_jakarta",
		"usr_1001",
		"system",
		"offline_access_granted",
		"lock_front_door",
		"gateway_offline_ble",
		ts,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record.At != ts {
		t.Fatalf("expected timestamp %v, got %v", ts, record.At)
	}
	if record.Action != "offline_access_granted" {
		t.Fatalf("expected action 'offline_access_granted', got %s", record.Action)
	}
	if record.Source != "gateway_offline_ble" {
		t.Fatalf("expected source 'gateway_offline_ble', got %s", record.Source)
	}
}
