package hris

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

type dlqMemoryStateStore struct {
	items              map[string][]byte
	compareAndSwapHook func(key string, expectedExists bool, expectedPayload []byte, nextPayload []byte)
}

func (s *dlqMemoryStateStore) Load(key string, dst any) (bool, error) {
	payload, ok := s.items[key]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(payload, dst); err != nil {
		return false, err
	}
	return true, nil
}

func (s *dlqMemoryStateStore) Save(key string, value any) error {
	if s.items == nil {
		s.items = make(map[string][]byte)
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.items[key] = payload
	return nil
}

func (s *dlqMemoryStateStore) CompareAndSwap(key string, expectedExists bool, expected any, next any) (bool, error) {
	if s.items == nil {
		s.items = make(map[string][]byte)
	}

	var expectedPayload []byte
	var err error
	if expectedExists {
		expectedPayload, err = json.Marshal(expected)
		if err != nil {
			return false, err
		}
	}

	nextPayload, err := json.Marshal(next)
	if err != nil {
		return false, err
	}

	if s.compareAndSwapHook != nil {
		hook := s.compareAndSwapHook
		s.compareAndSwapHook = nil
		hook(key, expectedExists, expectedPayload, nextPayload)
	}

	currentPayload, found := s.items[key]
	if found != expectedExists {
		return false, nil
	}
	if expectedExists {
		sameExpected, err := testDLQJSONPayloadEqual(currentPayload, expectedPayload)
		if err != nil {
			return false, err
		}
		if !sameExpected {
			return false, nil
		}
	}
	if found {
		sameNext, err := testDLQJSONPayloadEqual(currentPayload, nextPayload)
		if err != nil {
			return false, err
		}
		if sameNext {
			return true, nil
		}
	}
	s.items[key] = nextPayload
	return true, nil
}

func testDLQJSONPayloadEqual(left, right []byte) (bool, error) {
	var leftValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		return false, err
	}
	var rightValue any
	if err := json.Unmarshal(right, &rightValue); err != nil {
		return false, err
	}
	return reflect.DeepEqual(leftValue, rightValue), nil
}

func TestDLQServiceAppendListAndResolve(t *testing.T) {
	svc := NewDLQService()

	entry, err := svc.AppendFailure(DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   "hrc_talenta_jakarta",
		Vendor:        "talenta",
		ReceiptID:     "whr_001",
		RequestID:     "req_001",
		EventType:     "talenta.employee.detail.created",
		FailureStage:  "normalize",
		Error:         "invalid payload",
		RawPayloadRef: "hris_webhook_receipt:whr_001",
	})
	if err != nil {
		t.Fatalf("expected append failure success: %v", err)
	}
	if entry.ID == "" {
		t.Fatalf("expected non-empty dlq entry id")
	}

	items := svc.ListEntries("tenant_demo_jakarta", "hrc_talenta_jakarta", 10)
	if len(items) != 1 {
		t.Fatalf("expected one dlq entry, got %d", len(items))
	}
	if items[0].ReceiptID != "whr_001" {
		t.Fatalf("receipt_id mismatch: %s", items[0].ReceiptID)
	}

	updated, err := svc.AppendFailure(DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   "hrc_talenta_jakarta",
		Vendor:        "talenta",
		ReceiptID:     "whr_001",
		RequestID:     "req_001",
		EventType:     "talenta.employee.detail.created",
		FailureStage:  "normalize",
		Error:         "invalid employee email",
		RawPayloadRef: "hris_webhook_receipt:whr_001",
	})
	if err != nil {
		t.Fatalf("expected upsert failure success: %v", err)
	}
	if updated.ID != entry.ID {
		t.Fatalf("expected dlq upsert to keep same entry id, before=%s after=%s", entry.ID, updated.ID)
	}
	if updated.Error != "invalid employee email" {
		t.Fatalf("expected updated error, got %s", updated.Error)
	}

	resolved, err := svc.MarkResolved(entry.ID)
	if err != nil {
		t.Fatalf("expected mark resolved success: %v", err)
	}
	if resolved.Status != "resolved" {
		t.Fatalf("expected resolved status, got %s", resolved.Status)
	}
	if resolved.ReplayCount != 1 {
		t.Fatalf("expected replay_count=1, got %d", resolved.ReplayCount)
	}
}

func TestDLQServiceMarkReplayFailed(t *testing.T) {
	svc := NewDLQService()

	entry, err := svc.AppendFailure(DeadLetterFailureInput{
		TenantID:     "tenant_demo_jakarta",
		ConnectorID:  "hrc_gadjian_jakarta",
		Vendor:       "gadjian",
		ReceiptID:    "whr_002",
		FailureStage: "sync",
		Error:        "apply access failed",
	})
	if err != nil {
		t.Fatalf("expected append failure success: %v", err)
	}

	failed, err := svc.MarkReplayFailed(entry.ID, errors.New("replay still failed"))
	if err != nil {
		t.Fatalf("expected mark replay failed success: %v", err)
	}
	if failed.Status != "dlq" {
		t.Fatalf("expected dlq status, got %s", failed.Status)
	}
	if failed.ReplayCount != 1 {
		t.Fatalf("expected replay_count=1, got %d", failed.ReplayCount)
	}
	if failed.Error != "replay still failed" {
		t.Fatalf("unexpected replay failure error: %s", failed.Error)
	}
}

func TestDLQServiceAppendFailureMergesCompareAndSwapConflict(t *testing.T) {
	store := &dlqMemoryStateStore{}
	svc, err := NewDLQServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected dlq service with state store to initialize: %v", err)
	}

	competingEntry := DeadLetterEntry{
		ID:           "hdlq_competing_001",
		TenantID:     "tenant_demo_jakarta",
		ConnectorID:  "connector-talenta-conflict",
		Vendor:       "talenta",
		ReceiptID:    "whr_competing_001",
		RequestID:    "talenta-dlq-competing-001",
		FailureStage: "merge",
		Error:        "competing dlq entry",
		Status:       "dlq",
		CreatedAt:    time.Date(2026, 4, 24, 3, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 4, 24, 3, 0, 0, 0, time.UTC),
	}
	store.compareAndSwapHook = func(key string, expectedExists bool, _ []byte, _ []byte) {
		if key != dlqStateKey || expectedExists {
			return
		}
		payload, err := json.Marshal(deadLetterSnapshot{
			Entries: []DeadLetterEntry{competingEntry},
		})
		if err != nil {
			t.Fatalf("expected competing dlq snapshot to encode: %v", err)
		}
		store.items[key] = payload
	}

	entry, err := svc.AppendFailure(DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   "connector-talenta-conflict",
		Vendor:        "talenta",
		ReceiptID:     "whr_new_001",
		RequestID:     "talenta-dlq-new-001",
		EventType:     "talenta.employee.detail.updated",
		FailureStage:  "normalize",
		Error:         "new dlq entry",
		RawPayloadRef: "hris_webhook_receipt:whr_new_001",
	})
	if err != nil {
		t.Fatalf("expected append failure to succeed after CAS conflict: %v", err)
	}

	restored, err := NewDLQServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored dlq service with state store to initialize: %v", err)
	}
	items := restored.ListEntries("tenant_demo_jakarta", "connector-talenta-conflict", 10)
	if len(items) != 2 {
		t.Fatalf("expected both dlq entries to survive CAS retry, got %+v", items)
	}

	foundCreated := false
	foundCompeting := false
	for i := range items {
		switch items[i].ID {
		case entry.ID:
			foundCreated = true
		case competingEntry.ID:
			foundCompeting = true
		}
	}
	if !foundCreated || !foundCompeting {
		t.Fatalf("expected created and competing dlq entries to be preserved, got %+v", items)
	}
}

func TestDLQServiceMarkReplayFailedMergesCompareAndSwapConflict(t *testing.T) {
	store := &dlqMemoryStateStore{}
	svc, err := NewDLQServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected dlq service with state store to initialize: %v", err)
	}

	entry, err := svc.AppendFailure(DeadLetterFailureInput{
		TenantID:     "tenant_demo_jakarta",
		ConnectorID:  "connector-talenta-conflict",
		Vendor:       "talenta",
		ReceiptID:    "whr_fail_conflict_001",
		RequestID:    "talenta-dlq-fail-conflict-001",
		FailureStage: "sync",
		Error:        "initial dlq error",
	})
	if err != nil {
		t.Fatalf("expected append failure success: %v", err)
	}

	competingEntry := DeadLetterEntry{
		ID:           "hdlq_competing_002",
		TenantID:     "tenant_demo_jakarta",
		ConnectorID:  "connector-talenta-conflict",
		Vendor:       "talenta",
		ReceiptID:    "whr_competing_002",
		RequestID:    "talenta-dlq-competing-002",
		FailureStage: "merge",
		Error:        "competing merge failure",
		Status:       "dlq",
		CreatedAt:    time.Date(2026, 4, 24, 3, 5, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 4, 24, 3, 5, 0, 0, time.UTC),
	}
	store.compareAndSwapHook = func(key string, expectedExists bool, expectedPayload []byte, _ []byte) {
		if key != dlqStateKey || !expectedExists {
			return
		}
		var snapshot deadLetterSnapshot
		if err := json.Unmarshal(expectedPayload, &snapshot); err != nil {
			t.Fatalf("expected current dlq snapshot to decode: %v", err)
		}
		snapshot.Entries = append([]DeadLetterEntry{competingEntry}, snapshot.Entries...)
		payload, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatalf("expected competing dlq snapshot to encode: %v", err)
		}
		store.items[key] = payload
	}

	failed, err := svc.MarkReplayFailed(entry.ID, errors.New("replay still failed after conflict"))
	if err != nil {
		t.Fatalf("expected replay failed update to succeed after CAS conflict: %v", err)
	}
	if failed.Error != "replay still failed after conflict" {
		t.Fatalf("unexpected failed dlq entry after CAS retry: %+v", failed)
	}

	restored, err := NewDLQServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored dlq service with state store to initialize: %v", err)
	}
	items := restored.ListEntries("tenant_demo_jakarta", "connector-talenta-conflict", 10)
	if len(items) != 2 {
		t.Fatalf("expected both dlq entries to survive CAS retry, got %+v", items)
	}

	foundFailed := false
	foundCompeting := false
	for i := range items {
		switch items[i].ID {
		case entry.ID:
			foundFailed = items[i].Error == "replay still failed after conflict"
		case competingEntry.ID:
			foundCompeting = true
		}
	}
	if !foundFailed || !foundCompeting {
		t.Fatalf("expected failed and competing dlq entries to be preserved, got %+v", items)
	}
}

func TestDLQServiceClaimEntryForReplay(t *testing.T) {
	svc := NewDLQService()
	now := time.Now().UTC()
	oldReplayAt := now.Add(-10 * time.Minute)
	freshReplayAt := now.Add(-time.Minute)

	svc.entries = []DeadLetterEntry{
		{
			ID:        "hdlq_dlq",
			TenantID:  "tenant_demo_jakarta",
			Status:    "dlq",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:           "hdlq_cooldown",
			TenantID:     "tenant_demo_jakarta",
			Status:       "dlq",
			ReplayCount:  1,
			LastReplayAt: &freshReplayAt,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "hdlq_replaying_fresh",
			TenantID:     "tenant_demo_jakarta",
			Status:       "replaying",
			ReplayCount:  1,
			LastReplayAt: &freshReplayAt,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "hdlq_replaying_stale",
			TenantID:     "tenant_demo_jakarta",
			Status:       "replaying",
			ReplayCount:  1,
			LastReplayAt: &oldReplayAt,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:         "hdlq_resolved",
			TenantID:   "tenant_demo_jakarta",
			Status:     "resolved",
			CreatedAt:  now,
			UpdatedAt:  now,
			ResolvedAt: &now,
		},
	}

	claimed, reason, err := svc.ClaimEntryForReplay("hdlq_dlq", 5, 5*time.Minute, 5*time.Minute, now)
	if err != nil {
		t.Fatalf("expected dlq claim success: %v", err)
	}
	if reason != "" || claimed.Status != "replaying" || claimed.ReplayCount != 1 || claimed.LastReplayAt == nil {
		t.Fatalf("unexpected dlq claim result reason=%q item=%+v", reason, claimed)
	}

	cooldown, reason, err := svc.ClaimEntryForReplay("hdlq_cooldown", 5, 5*time.Minute, 5*time.Minute, now)
	if err != nil {
		t.Fatalf("expected cooldown claim check success: %v", err)
	}
	if reason != DLQEntryClaimReasonCooldown || cooldown.ReplayCount != 1 {
		t.Fatalf("unexpected cooldown claim result reason=%q item=%+v", reason, cooldown)
	}

	inFlight, reason, err := svc.ClaimEntryForReplay("hdlq_replaying_fresh", 5, 5*time.Minute, 5*time.Minute, now)
	if err != nil {
		t.Fatalf("expected in-flight claim check success: %v", err)
	}
	if reason != DLQEntryClaimReasonInFlight || inFlight.ReplayCount != 1 {
		t.Fatalf("unexpected in-flight claim result reason=%q item=%+v", reason, inFlight)
	}

	stale, reason, err := svc.ClaimEntryForReplay("hdlq_replaying_stale", 5, 5*time.Minute, 5*time.Minute, now)
	if err != nil {
		t.Fatalf("expected stale replaying claim success: %v", err)
	}
	if reason != "" || stale.Status != "replaying" || stale.ReplayCount != 2 || stale.LastReplayAt == nil {
		t.Fatalf("unexpected stale replaying claim result reason=%q item=%+v", reason, stale)
	}

	resolved, reason, err := svc.ClaimEntryForReplay("hdlq_resolved", 5, 5*time.Minute, 5*time.Minute, now)
	if err != nil {
		t.Fatalf("expected resolved claim check success: %v", err)
	}
	if reason != DLQEntryClaimReasonNotReplayable || resolved.Status != "resolved" {
		t.Fatalf("unexpected resolved claim result reason=%q item=%+v", reason, resolved)
	}
}

func TestDLQServiceListClaimableEntriesForReplayWithBackoffFiltersCooldownAttemptLimitAndInFlight(t *testing.T) {
	svc := NewDLQService()
	now := time.Now().UTC()
	oldReplayAt := now.Add(-20 * time.Minute)
	freshReplayAt := now.Add(-time.Minute)

	svc.entries = []DeadLetterEntry{
		{
			ID:          "hdlq_ready",
			TenantID:    "tenant_demo_jakarta",
			ConnectorID: "hrc_talenta_jakarta",
			Status:      "dlq",
			CreatedAt:   now.Add(-10 * time.Minute),
			UpdatedAt:   now.Add(-10 * time.Minute),
		},
		{
			ID:           "hdlq_stale",
			TenantID:     "tenant_demo_jakarta",
			ConnectorID:  "hrc_talenta_jakarta",
			Status:       "dlq",
			ReplayCount:  1,
			LastReplayAt: &oldReplayAt,
			CreatedAt:    now.Add(-9 * time.Minute),
			UpdatedAt:    now.Add(-9 * time.Minute),
		},
		{
			ID:           "hdlq_cooldown",
			TenantID:     "tenant_demo_jakarta",
			ConnectorID:  "hrc_talenta_jakarta",
			Status:       "dlq",
			ReplayCount:  1,
			LastReplayAt: &freshReplayAt,
			CreatedAt:    now.Add(-8 * time.Minute),
			UpdatedAt:    now.Add(-8 * time.Minute),
		},
		{
			ID:           "hdlq_limit",
			TenantID:     "tenant_demo_jakarta",
			ConnectorID:  "hrc_talenta_jakarta",
			Status:       "dlq",
			ReplayCount:  3,
			LastReplayAt: &oldReplayAt,
			CreatedAt:    now.Add(-7 * time.Minute),
			UpdatedAt:    now.Add(-7 * time.Minute),
		},
		{
			ID:           "hdlq_replaying_fresh",
			TenantID:     "tenant_demo_jakarta",
			ConnectorID:  "hrc_talenta_jakarta",
			Status:       "replaying",
			ReplayCount:  1,
			LastReplayAt: &freshReplayAt,
			CreatedAt:    now.Add(-6 * time.Minute),
			UpdatedAt:    now.Add(-6 * time.Minute),
		},
		{
			ID:           "hdlq_replaying_stale",
			TenantID:     "tenant_demo_jakarta",
			ConnectorID:  "hrc_talenta_jakarta",
			Status:       "replaying",
			ReplayCount:  1,
			LastReplayAt: &oldReplayAt,
			CreatedAt:    now.Add(-5 * time.Minute),
			UpdatedAt:    now.Add(-5 * time.Minute),
		},
		{
			ID:          "hdlq_other_connector",
			TenantID:    "tenant_demo_jakarta",
			ConnectorID: "hrc_other",
			Status:      "dlq",
			CreatedAt:   now.Add(-4 * time.Minute),
			UpdatedAt:   now.Add(-4 * time.Minute),
		},
	}

	items := svc.ListClaimableEntriesForReplayWithBackoff(
		"tenant_demo_jakarta",
		"hrc_talenta_jakarta",
		3,
		5*time.Minute,
		15*time.Minute,
		5*time.Minute,
		now,
		10,
	)
	if len(items) != 3 {
		t.Fatalf("expected 3 claimable entries, got %d", len(items))
	}
	if items[0].ID != "hdlq_replaying_stale" || items[1].ID != "hdlq_stale" || items[2].ID != "hdlq_ready" {
		t.Fatalf("unexpected claimable dlq entry order: %+v", items)
	}
	if items[0].LastReplayAt == nil {
		t.Fatalf("expected stale replaying entry to preserve last_replay_at")
	}
	if items[0].LastReplayAt == &oldReplayAt {
		t.Fatalf("expected claimable dlq last_replay_at to be cloned")
	}
}

func TestDLQServiceClaimEntryForReplayWithBackoffAppliesExponentialCooldown(t *testing.T) {
	svc := NewDLQService()
	now := time.Now().UTC()
	lastReplayAt := now.Add(-6 * time.Minute)

	svc.entries = []DeadLetterEntry{
		{
			ID:           "hdlq_backoff",
			TenantID:     "tenant_demo_jakarta",
			Status:       "dlq",
			ReplayCount:  2,
			LastReplayAt: &lastReplayAt,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	cooldown, reason, err := svc.ClaimEntryForReplayWithBackoff(
		"hdlq_backoff",
		5,
		5*time.Minute,
		20*time.Minute,
		5*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("expected exponential cooldown check success: %v", err)
	}
	if reason != DLQEntryClaimReasonCooldown {
		t.Fatalf("expected cooldown skip reason, got %q", reason)
	}
	if cooldown.ReplayCount != 2 || cooldown.Status != "dlq" {
		t.Fatalf("unexpected cooldown claim result: %+v", cooldown)
	}

	claimed, reason, err := svc.ClaimEntryForReplayWithBackoff(
		"hdlq_backoff",
		5,
		5*time.Minute,
		20*time.Minute,
		5*time.Minute,
		now.Add(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("expected claim after exponential cooldown success: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected empty skip reason after cooldown expiry, got %q", reason)
	}
	if claimed.Status != "replaying" || claimed.ReplayCount != 3 || claimed.LastReplayAt == nil {
		t.Fatalf("unexpected claimed entry after cooldown expiry: %+v", claimed)
	}
}

func TestDLQServiceMarkReplayOutcomeAfterClaim(t *testing.T) {
	svc := NewDLQService()
	entry, err := svc.AppendFailure(DeadLetterFailureInput{
		TenantID:     "tenant_demo_jakarta",
		ConnectorID:  "hrc_gadjian_jakarta",
		Vendor:       "gadjian",
		ReceiptID:    "whr_claim_001",
		FailureStage: "sync",
		Error:        "apply access failed",
	})
	if err != nil {
		t.Fatalf("expected append failure success: %v", err)
	}

	claimed, reason, err := svc.ClaimEntryForReplay(entry.ID, 5, 0, 5*time.Minute, time.Now().UTC())
	if err != nil {
		t.Fatalf("expected claim success: %v", err)
	}
	if reason != "" || claimed.ReplayCount != 1 || claimed.Status != "replaying" {
		t.Fatalf("unexpected claim result reason=%q item=%+v", reason, claimed)
	}

	failed, err := svc.MarkReplayFailed(entry.ID, errors.New("replay still failed"))
	if err != nil {
		t.Fatalf("expected mark replay failed success after claim: %v", err)
	}
	if failed.Status != "dlq" || failed.ReplayCount != 1 {
		t.Fatalf("unexpected claimed replay failure result: %+v", failed)
	}

	claimed, reason, err = svc.ClaimEntryForReplay(entry.ID, 5, 0, 5*time.Minute, time.Now().UTC().Add(10*time.Minute))
	if err != nil {
		t.Fatalf("expected second claim success: %v", err)
	}
	if reason != "" || claimed.ReplayCount != 2 || claimed.Status != "replaying" {
		t.Fatalf("unexpected second claim result reason=%q item=%+v", reason, claimed)
	}

	resolved, err := svc.MarkResolved(entry.ID)
	if err != nil {
		t.Fatalf("expected mark resolved success after claim: %v", err)
	}
	if resolved.Status != "resolved" || resolved.ReplayCount != 2 {
		t.Fatalf("unexpected claimed resolved result: %+v", resolved)
	}
}

func TestDLQServiceDueIndexPersistsAndListsDueItemsWithFallback(t *testing.T) {
	store := &dlqMemoryStateStore{}
	svc, err := NewDLQServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected dlq service with state store to initialize: %v", err)
	}

	now := time.Date(2026, 4, 24, 7, 0, 0, 0, time.UTC)
	staleReplayAt := now.Add(-20 * time.Minute)
	freshReplayAt := now.Add(-1 * time.Minute)

	svc.mu.Lock()
	svc.entries = []DeadLetterEntry{
		{
			ID:          "hdlq_ready",
			TenantID:    "tenant_demo_jakarta",
			ConnectorID: "connector-talenta",
			Vendor:      "talenta",
			Status:      "dlq",
			CreatedAt:   now.Add(-25 * time.Minute),
			UpdatedAt:   now.Add(-25 * time.Minute),
		},
		{
			ID:           "hdlq_cooldown",
			TenantID:     "tenant_demo_jakarta",
			ConnectorID:  "connector-talenta",
			Vendor:       "talenta",
			Status:       "dlq",
			ReplayCount:  1,
			LastReplayAt: &freshReplayAt,
			CreatedAt:    now.Add(-24 * time.Minute),
			UpdatedAt:    now.Add(-24 * time.Minute),
		},
		{
			ID:           "hdlq_stale",
			TenantID:     "tenant_demo_jakarta",
			ConnectorID:  "connector-talenta",
			Vendor:       "talenta",
			Status:       "replaying",
			ReplayCount:  1,
			LastReplayAt: &staleReplayAt,
			CreatedAt:    now.Add(-23 * time.Minute),
			UpdatedAt:    now.Add(-23 * time.Minute),
		},
	}
	svc.dueEntryIDs = []deadLetterDueIndexEntry{
		{EntryID: "hdlq_stale", DueAt: staleReplayAt.Add(10 * time.Minute)},
		{EntryID: "hdlq_ready", DueAt: now.Add(-25 * time.Minute)},
		{EntryID: "hdlq_cooldown", DueAt: freshReplayAt.Add(5 * time.Minute)},
	}
	svc.normalizeDLQDueIndexLocked()
	if err := svc.persistLocked(); err != nil {
		svc.mu.Unlock()
		t.Fatalf("expected dlq runtime state to persist: %v", err)
	}
	svc.mu.Unlock()

	var snapshot deadLetterSnapshot
	if err := json.Unmarshal(store.items[dlqStateKey], &snapshot); err != nil {
		t.Fatalf("expected dlq runtime snapshot to decode: %v", err)
	}
	if len(snapshot.DueEntryIDs) != 3 {
		t.Fatalf("expected 3 persisted due dlq ids, got %+v", snapshot.DueEntryIDs)
	}
	if snapshot.DueEntryIDs[0].EntryID != "hdlq_ready" ||
		snapshot.DueEntryIDs[1].EntryID != "hdlq_stale" ||
		snapshot.DueEntryIDs[2].EntryID != "hdlq_cooldown" {
		t.Fatalf("unexpected persisted due dlq ordering: %+v", snapshot.DueEntryIDs)
	}

	restored, err := NewDLQServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored dlq service with state store to initialize: %v", err)
	}

	items := restored.ListDueEntriesForReplayWithBackoff(
		"tenant_demo_jakarta",
		"connector-talenta",
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		now,
		10,
	)
	if len(items) != 2 {
		t.Fatalf("expected 2 due dlq entries, got %+v", items)
	}
	if items[0].ID != "hdlq_ready" || items[1].ID != "hdlq_stale" {
		t.Fatalf("unexpected due dlq ordering: %+v", items)
	}

	restored.mu.Lock()
	restored.dueEntryIDs = nil
	restored.mu.Unlock()

	fallbackItems := restored.ListDueEntriesForReplayWithBackoff(
		"tenant_demo_jakarta",
		"connector-talenta",
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		now,
		10,
	)
	if len(fallbackItems) != 2 {
		t.Fatalf("expected fallback due dlq entries, got %+v", fallbackItems)
	}
	if fallbackItems[0].ID != "hdlq_stale" || fallbackItems[1].ID != "hdlq_ready" {
		t.Fatalf("unexpected fallback due dlq ordering: %+v", fallbackItems)
	}
}
