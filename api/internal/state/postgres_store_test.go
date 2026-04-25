package state

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPostgresStoreSaveDeduplicatesUnchangedSnapshot(t *testing.T) {
	store := newTestPostgresStore(t)
	key := fmt.Sprintf("test_state_save_dedup_%d", time.Now().UnixNano())
	cleanupTestStateKey(t, store, key)
	defer cleanupTestStateKey(t, store, key)

	payload := map[string]any{
		"name":  "dedup-save",
		"count": 1,
		"tags":  []string{"a", "b"},
	}

	if err := store.Save(key, payload); err != nil {
		t.Fatalf("save #1 failed: %v", err)
	}
	if err := store.Save(key, payload); err != nil {
		t.Fatalf("save #2 failed: %v", err)
	}

	count := queryChangeLogCount(t, store, key)
	if count != 1 {
		t.Fatalf("expected one change_log row after duplicate save, got %d", count)
	}
}

func TestNewPostgresStoreConfiguresConnectionPool(t *testing.T) {
	store := newTestPostgresStore(t)
	stats := store.db.Stats()
	if stats.MaxOpenConnections != defaultMaxOpenConns {
		t.Fatalf("max open conns mismatch: got %d want %d", stats.MaxOpenConnections, defaultMaxOpenConns)
	}
	if store.db == nil {
		t.Fatalf("expected db to be initialized")
	}
}

func TestPostgresStoreSaveAppendsChangeWhenPayloadChanged(t *testing.T) {
	store := newTestPostgresStore(t)
	key := fmt.Sprintf("test_state_save_changed_%d", time.Now().UnixNano())
	cleanupTestStateKey(t, store, key)
	defer cleanupTestStateKey(t, store, key)

	first := map[string]any{
		"name":  "changed-save",
		"count": 1,
	}
	second := map[string]any{
		"name":  "changed-save",
		"count": 2,
	}

	if err := store.Save(key, first); err != nil {
		t.Fatalf("save #1 failed: %v", err)
	}
	if err := store.Save(key, second); err != nil {
		t.Fatalf("save #2 failed: %v", err)
	}

	count := queryChangeLogCount(t, store, key)
	if count != 2 {
		t.Fatalf("expected two change_log rows after payload update, got %d", count)
	}

	changes, err := store.ListStateChanges(key, 10)
	if err != nil {
		t.Fatalf("list state changes failed: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("expected two state changes, got %d", len(changes))
	}
	if strings.TrimSpace(changes[0].PayloadHash) == "" || strings.TrimSpace(changes[1].PayloadHash) == "" {
		t.Fatalf("payload hash should not be empty")
	}
	if changes[0].PayloadHash == changes[1].PayloadHash {
		t.Fatalf("expected different payload hash between changed snapshots")
	}
}

func TestPostgresStoreSaveRequiresStateKey(t *testing.T) {
	store := newTestPostgresStore(t)
	err := store.Save("   ", map[string]any{"ok": true})
	if err == nil {
		t.Fatalf("expected empty state key to fail")
	}
	if err != ErrStateKeyRequired {
		t.Fatalf("expected ErrStateKeyRequired, got %v", err)
	}
}

func TestPostgresStoreSaveAdvancesReplayCheckpoint(t *testing.T) {
	store := newTestPostgresStore(t)
	key := fmt.Sprintf("test_state_save_checkpoint_%d", time.Now().UnixNano())
	cleanupTestStateKey(t, store, key)
	defer cleanupTestStateKey(t, store, key)

	first := map[string]any{
		"version": 1,
		"value":   "first",
	}
	second := map[string]any{
		"version": 2,
		"value":   "second",
	}

	if err := store.Save(key, first); err != nil {
		t.Fatalf("save #1 failed: %v", err)
	}
	firstChangeID := queryLatestChangeID(t, store, key)
	firstCheckpoint, exists := queryReplayCheckpointLastChangeID(t, store, key)
	if !exists {
		t.Fatalf("expected replay checkpoint row after first save")
	}
	if firstCheckpoint != firstChangeID {
		t.Fatalf("checkpoint mismatch after first save: checkpoint=%d change_id=%d", firstCheckpoint, firstChangeID)
	}

	if err := store.Save(key, second); err != nil {
		t.Fatalf("save #2 failed: %v", err)
	}
	secondChangeID := queryLatestChangeID(t, store, key)
	secondCheckpoint, exists := queryReplayCheckpointLastChangeID(t, store, key)
	if !exists {
		t.Fatalf("expected replay checkpoint row after second save")
	}
	if secondCheckpoint != secondChangeID {
		t.Fatalf("checkpoint mismatch after second save: checkpoint=%d change_id=%d", secondCheckpoint, secondChangeID)
	}
	if secondCheckpoint <= firstCheckpoint {
		t.Fatalf("checkpoint should advance after second save, first=%d second=%d", firstCheckpoint, secondCheckpoint)
	}
}

func TestPostgresStoreReplayFromCheckpointRetryAndIdempotent(t *testing.T) {
	store := newTestPostgresStore(t)
	key := fmt.Sprintf("test_state_replay_retry_%d", time.Now().UnixNano())
	cleanupTestStateKey(t, store, key)
	defer cleanupTestStateKey(t, store, key)

	if err := store.Save(key, map[string]any{"version": 1, "value": "first"}); err != nil {
		t.Fatalf("save #1 failed: %v", err)
	}
	if err := store.Save(key, map[string]any{"version": 2, "value": "second"}); err != nil {
		t.Fatalf("save #2 failed: %v", err)
	}

	latestChangeID := queryLatestChangeID(t, store, key)
	if latestChangeID < 1 {
		t.Fatalf("expected change log entries for replay retry test")
	}
	setReplayCheckpointLastChangeID(t, store, key, 0)

	originalProject := store.projectionApplier
	failOnce := true
	store.projectionApplier = func(ctx context.Context, stateKey string, payload []byte) error {
		if stateKey == key && failOnce {
			failOnce = false
			return errors.New("forced replay projection failure")
		}
		return originalProject(ctx, stateKey, payload)
	}
	defer func() {
		store.projectionApplier = originalProject
	}()

	_, err := store.ReplayStateChangesFromCheckpoint(key, 10)
	if err == nil {
		t.Fatalf("expected replay from checkpoint to fail on injected projection error")
	}

	checkpointAfterFailure, exists := queryReplayCheckpointLastChangeID(t, store, key)
	if !exists {
		t.Fatalf("expected checkpoint row after failure")
	}
	if checkpointAfterFailure != 0 {
		t.Fatalf("checkpoint should not advance on failure, got %d", checkpointAfterFailure)
	}

	retryResult, err := store.ReplayStateChangesFromCheckpoint(key, 10)
	if err != nil {
		t.Fatalf("retry replay from checkpoint should succeed: %v", err)
	}
	if retryResult.Applied < 1 {
		t.Fatalf("retry replay should apply >=1 change, got %d", retryResult.Applied)
	}
	if retryResult.LastChangeID != latestChangeID {
		t.Fatalf("retry replay last_change_id mismatch, expected=%d got=%d", latestChangeID, retryResult.LastChangeID)
	}

	checkpointAfterRetry, exists := queryReplayCheckpointLastChangeID(t, store, key)
	if !exists {
		t.Fatalf("expected checkpoint row after retry")
	}
	if checkpointAfterRetry != latestChangeID {
		t.Fatalf("checkpoint should advance to latest change_id=%d, got %d", latestChangeID, checkpointAfterRetry)
	}

	idempotentResult, err := store.ReplayStateChangesFromCheckpoint(key, 10)
	if err != nil {
		t.Fatalf("idempotent replay run should succeed: %v", err)
	}
	if idempotentResult.Applied != 0 {
		t.Fatalf("idempotent replay second run expected applied=0, got %d", idempotentResult.Applied)
	}
	if idempotentResult.FromID != latestChangeID || idempotentResult.LastChangeID != latestChangeID {
		t.Fatalf(
			"idempotent replay ids mismatch: from_id=%d last_change_id=%d expected=%d",
			idempotentResult.FromID,
			idempotentResult.LastChangeID,
			latestChangeID,
		)
	}
}

func TestPostgresStoreReplayFromCheckpointConcurrentWorkers(t *testing.T) {
	store := newTestPostgresStore(t)
	key := fmt.Sprintf("test_state_replay_concurrent_%d", time.Now().UnixNano())
	cleanupTestStateKey(t, store, key)
	defer cleanupTestStateKey(t, store, key)

	for i := 1; i <= 4; i++ {
		payload := map[string]any{
			"version": i,
			"value":   fmt.Sprintf("v-%d", i),
		}
		if err := store.Save(key, payload); err != nil {
			t.Fatalf("seed save #%d failed: %v", i, err)
		}
	}

	latestChangeID := queryLatestChangeID(t, store, key)
	if latestChangeID < 1 {
		t.Fatalf("expected change log rows for concurrent replay test")
	}
	setReplayCheckpointLastChangeID(t, store, key, 0)

	const workers = 8
	startCh := make(chan struct{})
	resultCh := make(chan ReplayFromCheckpointResult, workers)
	errCh := make(chan error, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-startCh
			result, err := store.ReplayStateChangesFromCheckpoint(key, 500)
			if err != nil {
				errCh <- err
				return
			}
			resultCh <- result
		}()
	}

	close(startCh)
	wg.Wait()
	close(resultCh)
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent replay worker error: %v", err)
		}
	}

	appliedRuns := 0
	for result := range resultCh {
		if result.Applied > 0 {
			appliedRuns++
		}
	}
	if appliedRuns < 1 {
		t.Fatalf("expected at least one replay worker to apply changes")
	}

	checkpointLastID, exists := queryReplayCheckpointLastChangeID(t, store, key)
	if !exists {
		t.Fatalf("expected replay checkpoint row after concurrent workers")
	}
	if checkpointLastID != latestChangeID {
		t.Fatalf("checkpoint should converge to latest change id=%d, got %d", latestChangeID, checkpointLastID)
	}

	finalResult, err := store.ReplayStateChangesFromCheckpoint(key, 500)
	if err != nil {
		t.Fatalf("final idempotent replay failed: %v", err)
	}
	if finalResult.Applied != 0 {
		t.Fatalf("final idempotent replay expected applied=0, got %d", finalResult.Applied)
	}
	if finalResult.FromID != latestChangeID || finalResult.LastChangeID != latestChangeID {
		t.Fatalf(
			"final replay ids mismatch: from_id=%d last_change_id=%d expected=%d",
			finalResult.FromID,
			finalResult.LastChangeID,
			latestChangeID,
		)
	}
}

func TestPostgresStoreReplayFromCheckpointConcurrentMultiStateKeys(t *testing.T) {
	store := newTestPostgresStore(t)
	keys := []string{
		fmt.Sprintf("test_state_replay_multi_key_a_%d", time.Now().UnixNano()),
		fmt.Sprintf("test_state_replay_multi_key_b_%d", time.Now().UnixNano()),
	}
	for i := range keys {
		cleanupTestStateKey(t, store, keys[i])
	}
	defer func() {
		for i := range keys {
			cleanupTestStateKey(t, store, keys[i])
		}
	}()

	latestByKey := make(map[string]int64, len(keys))
	for _, key := range keys {
		for version := 1; version <= 3; version++ {
			payload := map[string]any{
				"state_key": key,
				"version":   version,
				"value":     fmt.Sprintf("%s-v-%d", key, version),
			}
			if err := store.Save(key, payload); err != nil {
				t.Fatalf("seed save failed for key=%s version=%d: %v", key, version, err)
			}
		}
		latestByKey[key] = queryLatestChangeID(t, store, key)
		setReplayCheckpointLastChangeID(t, store, key, 0)
	}

	type replayWorkerResult struct {
		key    string
		result ReplayFromCheckpointResult
	}

	const workersPerKey = 4
	startCh := make(chan struct{})
	resultCh := make(chan replayWorkerResult, len(keys)*workersPerKey)
	errCh := make(chan error, len(keys)*workersPerKey)

	var wg sync.WaitGroup
	wg.Add(len(keys) * workersPerKey)
	for _, key := range keys {
		currentKey := key
		for i := 0; i < workersPerKey; i++ {
			go func() {
				defer wg.Done()
				<-startCh
				result, err := store.ReplayStateChangesFromCheckpoint(currentKey, 500)
				if err != nil {
					errCh <- err
					return
				}
				resultCh <- replayWorkerResult{
					key:    currentKey,
					result: result,
				}
			}()
		}
	}

	close(startCh)
	wg.Wait()
	close(resultCh)
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent multi-key replay worker error: %v", err)
		}
	}

	appliedRunsByKey := make(map[string]int, len(keys))
	for workerResult := range resultCh {
		if workerResult.result.Applied > 0 {
			appliedRunsByKey[workerResult.key]++
		}
	}

	for _, key := range keys {
		if appliedRunsByKey[key] < 1 {
			t.Fatalf("expected at least one replay worker to apply changes for key=%s", key)
		}

		checkpointLastID, exists := queryReplayCheckpointLastChangeID(t, store, key)
		if !exists {
			t.Fatalf("expected replay checkpoint row for key=%s", key)
		}
		if checkpointLastID != latestByKey[key] {
			t.Fatalf(
				"checkpoint should converge to latest change id for key=%s: expected=%d got=%d",
				key,
				latestByKey[key],
				checkpointLastID,
			)
		}

		finalResult, err := store.ReplayStateChangesFromCheckpoint(key, 500)
		if err != nil {
			t.Fatalf("final idempotent replay failed for key=%s: %v", key, err)
		}
		if finalResult.Applied != 0 {
			t.Fatalf("final idempotent replay expected applied=0 for key=%s, got %d", key, finalResult.Applied)
		}
		if finalResult.FromID != latestByKey[key] || finalResult.LastChangeID != latestByKey[key] {
			t.Fatalf(
				"final replay ids mismatch for key=%s: from_id=%d last_change_id=%d expected=%d",
				key,
				finalResult.FromID,
				finalResult.LastChangeID,
				latestByKey[key],
			)
		}
	}
}

func newTestPostgresStore(t *testing.T) *PostgresStore {
	t.Helper()

	dsns := []string{
		strings.TrimSpace(os.Getenv("DATABASE_URL")),
		"postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable",
		"postgres://siky@localhost:5432/postgres?sslmode=disable",
	}

	var lastErr error
	for i := range dsns {
		dsn := strings.TrimSpace(dsns[i])
		if dsn == "" {
			continue
		}
		store, err := NewPostgresStore(dsn)
		if err != nil {
			lastErr = err
			continue
		}
		if err := store.EnsureSchema(); err != nil {
			_ = store.Close()
			t.Fatalf("ensure schema failed: %v", err)
		}
		t.Cleanup(func() {
			_ = store.Close()
		})
		return store
	}

	if lastErr != nil {
		t.Skipf("postgres unavailable for integration test: %v", lastErr)
	}
	t.Skip("postgres unavailable for integration test")
	return nil
}

func cleanupTestStateKey(t *testing.T, store *PostgresStore, key string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := store.db.ExecContext(ctx, `delete from mistypass_change_replay_checkpoints where state_key = $1`, key); err != nil {
		t.Fatalf("cleanup replay checkpoint failed: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `delete from mistypass_change_log where state_key = $1`, key); err != nil {
		t.Fatalf("cleanup change log failed: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `delete from mistypass where state_key = $1`, key); err != nil {
		t.Fatalf("cleanup state row failed: %v", err)
	}
}

func queryChangeLogCount(t *testing.T, store *PostgresStore, key string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	if err := store.db.QueryRowContext(
		ctx,
		`select count(*) from mistypass_change_log where state_key = $1`,
		key,
	).Scan(&count); err != nil {
		t.Fatalf("count change log failed: %v", err)
	}
	return count
}

func queryLatestChangeID(t *testing.T, store *PostgresStore, key string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int64
	if err := store.db.QueryRowContext(
		ctx,
		`select coalesce(max(id), 0) from mistypass_change_log where state_key = $1`,
		key,
	).Scan(&id); err != nil {
		t.Fatalf("query latest change id failed: %v", err)
	}
	return id
}

func queryReplayCheckpointLastChangeID(t *testing.T, store *PostgresStore, key string) (int64, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int64
	err := store.db.QueryRowContext(
		ctx,
		`select last_change_id from mistypass_change_replay_checkpoints where state_key = $1`,
		key,
	).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false
		}
		t.Fatalf("query replay checkpoint failed: %v", err)
	}
	return id, true
}

func setReplayCheckpointLastChangeID(t *testing.T, store *PostgresStore, key string, lastChangeID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := store.db.ExecContext(
		ctx,
		`insert into mistypass_change_replay_checkpoints (state_key, last_change_id, updated_at)
values ($1, $2, now())
on conflict (state_key) do update
set last_change_id = $2,
    updated_at = now()`,
		key,
		lastChangeID,
	); err != nil {
		t.Fatalf("set replay checkpoint failed: %v", err)
	}
}
