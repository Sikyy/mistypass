package hris

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestPullStateServiceClaimStateForPull(t *testing.T) {
	svc := NewPullStateService()
	now := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)

	if _, err := svc.MarkSucceeded(
		"tenant_demo_jakarta",
		"connector_succeeded",
		"talenta",
		PullModeFull,
		"pull-req-001",
		now.Add(-2*time.Hour),
	); err != nil {
		t.Fatalf("mark succeeded seed should succeed: %v", err)
	}
	if _, err := svc.MarkFailed(
		"tenant_demo_jakarta",
		"connector_cooldown",
		"talenta",
		now.Add(-5*time.Minute),
		errors.New("429 throttled"),
	); err != nil {
		t.Fatalf("mark failed cooldown seed should succeed: %v", err)
	}
	if _, err := svc.MarkFailed(
		"tenant_demo_jakarta",
		"connector_attempt_limit",
		"talenta",
		now.Add(-2*time.Hour),
		errors.New("invalid credential"),
	); err != nil {
		t.Fatalf("mark failed attempt limit seed should succeed: %v", err)
	}
	if _, err := svc.MarkFailed(
		"tenant_demo_jakarta",
		"connector_attempt_limit",
		"talenta",
		now.Add(-90*time.Minute),
		errors.New("still invalid"),
	); err != nil {
		t.Fatalf("mark failed second attempt limit seed should succeed: %v", err)
	}
	if _, err := svc.MarkStarted(
		"tenant_demo_jakarta",
		"connector_running_fresh",
		"talenta",
		PullModeIncremental,
		now.Add(-2*time.Minute),
	); err != nil {
		t.Fatalf("mark started fresh seed should succeed: %v", err)
	}
	if _, err := svc.MarkStarted(
		"tenant_demo_jakarta",
		"connector_running_stale",
		"talenta",
		PullModeIncremental,
		now.Add(-40*time.Minute),
	); err != nil {
		t.Fatalf("mark started stale seed should succeed: %v", err)
	}

	claimedNew, reason, err := svc.ClaimStateForPull(
		"tenant_demo_jakarta",
		"connector_new",
		"talenta",
		PullModeFull,
		5,
		30*time.Minute,
		30*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("claim new connector should succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected no skip reason for new connector, got %s", reason)
	}
	if claimedNew.Status != "running" || claimedNew.LastStartedAt == nil || !claimedNew.LastStartedAt.Equal(now) {
		t.Fatalf("unexpected claimed new state: %+v", claimedNew)
	}

	claimedSucceeded, reason, err := svc.ClaimStateForPull(
		"tenant_demo_jakarta",
		"connector_succeeded",
		"talenta",
		PullModeIncremental,
		5,
		30*time.Minute,
		30*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("claim succeeded connector should succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected no skip reason for succeeded connector, got %s", reason)
	}
	if claimedSucceeded.Status != "running" || claimedSucceeded.LastMode != PullModeIncremental {
		t.Fatalf("unexpected claimed succeeded state: %+v", claimedSucceeded)
	}

	cooldownState, reason, err := svc.ClaimStateForPull(
		"tenant_demo_jakarta",
		"connector_cooldown",
		"talenta",
		PullModeIncremental,
		5,
		30*time.Minute,
		30*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("claim cooldown connector should not error: %v", err)
	}
	if reason != PullStateClaimReasonCooldown {
		t.Fatalf("expected cooldown skip reason, got %s", reason)
	}
	if cooldownState.Status != "failed" || cooldownState.ConsecutiveFailures != 1 {
		t.Fatalf("unexpected cooldown state: %+v", cooldownState)
	}

	attemptLimitState, reason, err := svc.ClaimStateForPull(
		"tenant_demo_jakarta",
		"connector_attempt_limit",
		"talenta",
		PullModeIncremental,
		2,
		0,
		30*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("claim attempt-limit connector should not error: %v", err)
	}
	if reason != PullStateClaimReasonAttemptLimit {
		t.Fatalf("expected attempt limit skip reason, got %s", reason)
	}
	if attemptLimitState.Status != "failed" || attemptLimitState.ConsecutiveFailures != 2 {
		t.Fatalf("unexpected attempt-limit state: %+v", attemptLimitState)
	}

	inFlightState, reason, err := svc.ClaimStateForPull(
		"tenant_demo_jakarta",
		"connector_running_fresh",
		"talenta",
		PullModeIncremental,
		5,
		0,
		30*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("claim fresh running connector should not error: %v", err)
	}
	if reason != PullStateClaimReasonInFlight {
		t.Fatalf("expected in-flight skip reason, got %s", reason)
	}
	if inFlightState.Status != "running" {
		t.Fatalf("unexpected in-flight state: %+v", inFlightState)
	}

	staleState, reason, err := svc.ClaimStateForPull(
		"tenant_demo_jakarta",
		"connector_running_stale",
		"talenta",
		PullModeFull,
		5,
		0,
		30*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("claim stale running connector should succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected stale running connector to be reclaimed, got %s", reason)
	}
	if staleState.Status != "running" || staleState.LastStartedAt == nil || !staleState.LastStartedAt.Equal(now) {
		t.Fatalf("unexpected stale reclaimed state: %+v", staleState)
	}
}

func TestPullStateServiceClaimStateForPullWithBackoffAppliesExponentialCooldown(t *testing.T) {
	svc := NewPullStateService()
	now := time.Now().UTC()
	lastFailureAt := now.Add(-6 * time.Minute)

	svc.states = []ConnectorPullState{
		{
			TenantID:            "tenant_demo_jakarta",
			ConnectorID:         "connector_backoff",
			Vendor:              "talenta",
			Status:              "failed",
			LastFailureAt:       &lastFailureAt,
			ConsecutiveFailures: 2,
			CreatedAt:           now.Add(-30 * time.Minute),
			UpdatedAt:           lastFailureAt,
		},
	}

	cooldownState, reason, err := svc.ClaimStateForPullWithBackoff(
		"tenant_demo_jakarta",
		"connector_backoff",
		"talenta",
		PullModeIncremental,
		5,
		5*time.Minute,
		20*time.Minute,
		30*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("claim exponential cooldown connector should not error: %v", err)
	}
	if reason != PullStateClaimReasonCooldown {
		t.Fatalf("expected cooldown skip reason, got %s", reason)
	}
	if cooldownState.Status != "failed" || cooldownState.ConsecutiveFailures != 2 {
		t.Fatalf("unexpected cooldown state: %+v", cooldownState)
	}

	claimedState, reason, err := svc.ClaimStateForPullWithBackoff(
		"tenant_demo_jakarta",
		"connector_backoff",
		"talenta",
		PullModeIncremental,
		5,
		5*time.Minute,
		20*time.Minute,
		30*time.Minute,
		now.Add(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("claim connector after exponential cooldown should succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected no skip reason after exponential cooldown expiry, got %s", reason)
	}
	if claimedState.Status != "running" || claimedState.LastStartedAt == nil || !claimedState.LastStartedAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("unexpected claimed state after exponential cooldown expiry: %+v", claimedState)
	}
}

func TestPullStateServiceClaimStateForPullWithBackoffMergesCompareAndSwapConflict(t *testing.T) {
	store := &dlqMemoryStateStore{}
	svc, err := NewPullStateServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected pull state service with state store to initialize: %v", err)
	}

	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	competingState := ConnectorPullState{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   "connector_competing",
		Vendor:        "talenta",
		Status:        "succeeded",
		LastMode:      PullModeFull,
		CreatedAt:     now.Add(-time.Hour),
		UpdatedAt:     now.Add(-time.Hour),
		LastSuccessAt: cloneTimePointer(&now),
	}
	store.compareAndSwapHook = func(key string, expectedExists bool, _ []byte, _ []byte) {
		if key != pullStateKey || expectedExists {
			return
		}
		payload, err := json.Marshal(pullStateSnapshot{
			States: []ConnectorPullState{competingState},
		})
		if err != nil {
			t.Fatalf("expected competing pull state snapshot to encode: %v", err)
		}
		store.items[key] = payload
	}

	claimed, reason, err := svc.ClaimStateForPullWithBackoff(
		"tenant_demo_jakarta",
		"connector_new",
		"talenta",
		PullModeIncremental,
		5,
		0,
		0,
		30*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("expected pull claim to succeed after CAS conflict: %v", err)
	}
	if reason != "" || claimed.Status != "running" || claimed.LastMode != PullModeIncremental {
		t.Fatalf("unexpected claimed pull state after CAS retry: reason=%q state=%+v", reason, claimed)
	}

	restored, err := NewPullStateServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored pull state service with state store to initialize: %v", err)
	}
	items := restored.ListStates("tenant_demo_jakarta")
	if len(items) != 2 {
		t.Fatalf("expected both pull states to survive CAS retry, got %+v", items)
	}

	foundClaimed := false
	foundCompeting := false
	for i := range items {
		switch items[i].ConnectorID {
		case "connector_new":
			foundClaimed = items[i].Status == "running"
		case competingState.ConnectorID:
			foundCompeting = items[i].Status == "succeeded"
		}
	}
	if !foundClaimed || !foundCompeting {
		t.Fatalf("expected claimed and competing pull states to be preserved, got %+v", items)
	}
}

func TestPullStateServiceMarkFailedMergesCompareAndSwapConflict(t *testing.T) {
	store := &dlqMemoryStateStore{}
	svc, err := NewPullStateServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected pull state service with state store to initialize: %v", err)
	}

	startedAt := time.Date(2026, 4, 24, 11, 0, 0, 0, time.UTC)
	if _, err := svc.MarkStarted(
		"tenant_demo_jakarta",
		"connector_primary",
		"talenta",
		PullModeIncremental,
		startedAt,
	); err != nil {
		t.Fatalf("expected mark started seed to succeed: %v", err)
	}

	competingState := ConnectorPullState{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   "connector_competing",
		Vendor:        "talenta",
		Status:        "succeeded",
		LastMode:      PullModeFull,
		CreatedAt:     startedAt.Add(-time.Hour),
		UpdatedAt:     startedAt.Add(-time.Hour),
		LastSuccessAt: cloneTimePointer(&startedAt),
	}
	store.compareAndSwapHook = func(key string, expectedExists bool, expectedPayload []byte, _ []byte) {
		if key != pullStateKey || !expectedExists {
			return
		}
		var snapshot pullStateSnapshot
		if err := json.Unmarshal(expectedPayload, &snapshot); err != nil {
			t.Fatalf("expected current pull state snapshot to decode: %v", err)
		}
		snapshot.States = append([]ConnectorPullState{competingState}, snapshot.States...)
		payload, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatalf("expected competing pull state snapshot to encode: %v", err)
		}
		store.items[key] = payload
	}

	failedAt := startedAt.Add(5 * time.Minute)
	failed, err := svc.MarkFailed(
		"tenant_demo_jakarta",
		"connector_primary",
		"talenta",
		failedAt,
		errors.New("429 throttled after conflict"),
	)
	if err != nil {
		t.Fatalf("expected mark failed to succeed after CAS conflict: %v", err)
	}
	if failed.Status != "failed" || failed.LastError != "429 throttled after conflict" || failed.ConsecutiveFailures != 1 {
		t.Fatalf("unexpected failed pull state after CAS retry: %+v", failed)
	}

	restored, err := NewPullStateServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected restored pull state service with state store to initialize: %v", err)
	}
	items := restored.ListStates("tenant_demo_jakarta")
	if len(items) != 2 {
		t.Fatalf("expected both pull states to survive CAS retry, got %+v", items)
	}

	foundFailed := false
	foundCompeting := false
	for i := range items {
		switch items[i].ConnectorID {
		case "connector_primary":
			foundFailed = items[i].Status == "failed" &&
				items[i].LastError == "429 throttled after conflict" &&
				items[i].ConsecutiveFailures == 1
		case competingState.ConnectorID:
			foundCompeting = items[i].Status == "succeeded"
		}
	}
	if !foundFailed || !foundCompeting {
		t.Fatalf("expected failed and competing pull states to be preserved, got %+v", items)
	}
}
