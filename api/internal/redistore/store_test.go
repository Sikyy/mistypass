package redistore

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func newTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	store, err := New(Options{Addr: mr.Addr(), KeyPrefix: "test"})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, mr
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

func TestNew_Valid(t *testing.T) {
	store, _ := newTestStore(t)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestNew_MissingAddr(t *testing.T) {
	_, err := New(Options{Addr: ""})
	if err == nil {
		t.Fatal("expected error for missing addr")
	}
}

func TestNew_DefaultKeyPrefix(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	store, err := New(Options{Addr: mr.Addr()})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if store.keyPrefix != "mistypass" {
		t.Fatalf("expected default key prefix 'mistypass', got %q", store.keyPrefix)
	}
}

// ---------------------------------------------------------------------------
// Auth Refresh Sessions — Upsert
// ---------------------------------------------------------------------------

func TestUpsertAuthRefreshSession_Valid(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.UpsertAuthRefreshSession("sess-1", "user-1", time.Now().Add(10*time.Minute))
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
}

func TestUpsertAuthRefreshSession_EmptySessionID(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.UpsertAuthRefreshSession("", "user-1", time.Now().Add(10*time.Minute))
	if err == nil {
		t.Fatal("expected error for empty sessionID")
	}
}

func TestUpsertAuthRefreshSession_EmptyUserID(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.UpsertAuthRefreshSession("sess-1", "", time.Now().Add(10*time.Minute))
	if err == nil {
		t.Fatal("expected error for empty userID")
	}
}

func TestUpsertAuthRefreshSession_ExpiredTTL(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.UpsertAuthRefreshSession("sess-1", "user-1", time.Now().Add(-1*time.Minute))
	if err == nil {
		t.Fatal("expected error for expired TTL")
	}
}

// ---------------------------------------------------------------------------
// Auth Refresh Sessions — Find
// ---------------------------------------------------------------------------

func TestFindAuthRefreshSession_Found(t *testing.T) {
	store, _ := newTestStore(t)
	expires := time.Now().Add(10 * time.Minute).UTC().Truncate(time.Second)
	if err := store.UpsertAuthRefreshSession("sess-1", "user-1", expires); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	userID, expiresAt, found, err := store.FindAuthRefreshSession("sess-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if !found {
		t.Fatal("expected session to be found")
	}
	if userID != "user-1" {
		t.Fatalf("expected userID 'user-1', got %q", userID)
	}
	if expiresAt.Unix() != expires.Unix() {
		t.Fatalf("expected expiresAt %v, got %v", expires, expiresAt)
	}
}

func TestFindAuthRefreshSession_NotFound(t *testing.T) {
	store, _ := newTestStore(t)
	_, _, found, err := store.FindAuthRefreshSession("nonexistent")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found {
		t.Fatal("expected session not to be found")
	}
}

func TestFindAuthRefreshSession_EmptyID(t *testing.T) {
	store, _ := newTestStore(t)
	_, _, found, err := store.FindAuthRefreshSession("")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found {
		t.Fatal("expected empty ID to return not found")
	}
}

func TestFindAuthRefreshSession_Expired(t *testing.T) {
	store, mr := newTestStore(t)
	if err := store.UpsertAuthRefreshSession("sess-exp", "user-1", time.Now().Add(2*time.Second)); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Fast-forward miniredis so the key expires
	mr.FastForward(5 * time.Second)

	_, _, found, err := store.FindAuthRefreshSession("sess-exp")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found {
		t.Fatal("expected expired session not to be found")
	}
}

// ---------------------------------------------------------------------------
// Auth Refresh Sessions — Delete
// ---------------------------------------------------------------------------

func TestDeleteAuthRefreshSession_Existing(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.UpsertAuthRefreshSession("sess-del", "user-1", time.Now().Add(10*time.Minute)); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if err := store.DeleteAuthRefreshSession("sess-del"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, _, found, err := store.FindAuthRefreshSession("sess-del")
	if err != nil {
		t.Fatalf("find after delete: %v", err)
	}
	if found {
		t.Fatal("session should be gone after delete")
	}
}

func TestDeleteAuthRefreshSession_NonExisting(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.DeleteAuthRefreshSession("nonexistent")
	if err != nil {
		t.Fatalf("delete non-existing should not error: %v", err)
	}
}

func TestDeleteAuthRefreshSession_EmptyID(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.DeleteAuthRefreshSession("")
	if err != nil {
		t.Fatalf("delete empty should not error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Auth Refresh Sessions — Delete by UserID
// ---------------------------------------------------------------------------

func TestDeleteAuthRefreshSessionsByUserID_MultipleSessions(t *testing.T) {
	store, _ := newTestStore(t)
	expires := time.Now().Add(10 * time.Minute)
	for _, sid := range []string{"s1", "s2", "s3"} {
		if err := store.UpsertAuthRefreshSession(sid, "user-bulk", expires); err != nil {
			t.Fatalf("upsert %s: %v", sid, err)
		}
	}

	deleted, err := store.DeleteAuthRefreshSessionsByUserID("user-bulk")
	if err != nil {
		t.Fatalf("delete by userID: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("expected 3 deleted, got %d", deleted)
	}

	for _, sid := range []string{"s1", "s2", "s3"} {
		_, _, found, _ := store.FindAuthRefreshSession(sid)
		if found {
			t.Fatalf("session %s should be deleted", sid)
		}
	}
}

func TestDeleteAuthRefreshSessionsByUserID_EmptyUserID(t *testing.T) {
	store, _ := newTestStore(t)
	deleted, err := store.DeleteAuthRefreshSessionsByUserID("")
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected 0 deleted, got %d", deleted)
	}
}

func TestDeleteAuthRefreshSessionsByUserID_NoSessions(t *testing.T) {
	store, _ := newTestStore(t)
	deleted, err := store.DeleteAuthRefreshSessionsByUserID("ghost-user")
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected 0 deleted, got %d", deleted)
	}
}

// ---------------------------------------------------------------------------
// Token Revocation — Upsert
// ---------------------------------------------------------------------------

func TestUpsertAuthRevokedAccessToken_Valid(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.UpsertAuthRevokedAccessToken("tok-1", time.Now().Add(5*time.Minute))
	if err != nil {
		t.Fatalf("upsert revoked: %v", err)
	}
}

func TestUpsertAuthRevokedAccessToken_EmptyTokenID(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.UpsertAuthRevokedAccessToken("", time.Now().Add(5*time.Minute))
	if err == nil {
		t.Fatal("expected error for empty tokenID")
	}
}

func TestUpsertAuthRevokedAccessToken_Expired(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.UpsertAuthRevokedAccessToken("tok-exp", time.Now().Add(-1*time.Minute))
	if err == nil {
		t.Fatal("expected error for already-expired token")
	}
}

// ---------------------------------------------------------------------------
// Token Revocation — IsRevoked
// ---------------------------------------------------------------------------

func TestIsAuthAccessTokenRevoked_Revoked(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.UpsertAuthRevokedAccessToken("tok-rev", time.Now().Add(5*time.Minute)); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	revoked, err := store.IsAuthAccessTokenRevoked("tok-rev", time.Now())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !revoked {
		t.Fatal("expected token to be revoked")
	}
}

func TestIsAuthAccessTokenRevoked_NotRevoked(t *testing.T) {
	store, _ := newTestStore(t)
	revoked, err := store.IsAuthAccessTokenRevoked("tok-clean", time.Now())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if revoked {
		t.Fatal("expected token not to be revoked")
	}
}

func TestIsAuthAccessTokenRevoked_EmptyTokenID(t *testing.T) {
	store, _ := newTestStore(t)
	revoked, err := store.IsAuthAccessTokenRevoked("", time.Now())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if revoked {
		t.Fatal("empty tokenID should not be revoked")
	}
}

func TestIsAuthAccessTokenRevoked_AfterExpiry(t *testing.T) {
	store, mr := newTestStore(t)
	if err := store.UpsertAuthRevokedAccessToken("tok-ttl", time.Now().Add(2*time.Second)); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	mr.FastForward(5 * time.Second)

	revoked, err := store.IsAuthAccessTokenRevoked("tok-ttl", time.Now())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if revoked {
		t.Fatal("token should no longer be revoked after TTL expires")
	}
}

// ---------------------------------------------------------------------------
// Rate Limiting
// ---------------------------------------------------------------------------

func TestAllowRateLimit_UnderLimit(t *testing.T) {
	store, _ := newTestStore(t)
	allowed, retryAfter, err := store.AllowRateLimit("login", "user@example.com", time.Now(), 1*time.Minute, 5)
	if err != nil {
		t.Fatalf("allow: %v", err)
	}
	if !allowed {
		t.Fatal("first attempt should be allowed")
	}
	if retryAfter != 0 {
		t.Fatalf("expected zero retryAfter, got %v", retryAfter)
	}
}

func TestAllowRateLimit_AtLimit(t *testing.T) {
	store, _ := newTestStore(t)
	now := time.Now()
	maxAttempts := 3
	for i := 0; i < maxAttempts; i++ {
		allowed, _, err := store.AllowRateLimit("login", "user@test.com", now, 1*time.Minute, maxAttempts)
		if err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
		if !allowed {
			t.Fatalf("attempt %d should be allowed (max=%d)", i+1, maxAttempts)
		}
	}
}

func TestAllowRateLimit_OverLimit(t *testing.T) {
	store, _ := newTestStore(t)
	now := time.Now()
	maxAttempts := 2

	for i := 0; i < maxAttempts; i++ {
		_, _, err := store.AllowRateLimit("login", "flood@test.com", now, 1*time.Minute, maxAttempts)
		if err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}

	allowed, retryAfter, err := store.AllowRateLimit("login", "flood@test.com", now, 1*time.Minute, maxAttempts)
	if err != nil {
		t.Fatalf("over-limit call: %v", err)
	}
	if allowed {
		t.Fatal("should be denied over limit")
	}
	if retryAfter <= 0 {
		t.Fatal("expected positive retryAfter when denied")
	}
}

func TestAllowRateLimit_EmptyScope(t *testing.T) {
	store, _ := newTestStore(t)
	_, _, err := store.AllowRateLimit("", "key", time.Now(), 1*time.Minute, 5)
	if err == nil {
		t.Fatal("expected error for empty scope")
	}
}

func TestAllowRateLimit_EmptyKey(t *testing.T) {
	store, _ := newTestStore(t)
	_, _, err := store.AllowRateLimit("scope", "", time.Now(), 1*time.Minute, 5)
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestAllowRateLimit_InvalidWindow(t *testing.T) {
	store, _ := newTestStore(t)
	_, _, err := store.AllowRateLimit("scope", "key", time.Now(), 0, 5)
	if err == nil {
		t.Fatal("expected error for zero window")
	}

	_, _, err = store.AllowRateLimit("scope", "key", time.Now(), -1*time.Second, 5)
	if err == nil {
		t.Fatal("expected error for negative window")
	}
}

func TestAllowRateLimit_InvalidMaxAttempts(t *testing.T) {
	store, _ := newTestStore(t)
	_, _, err := store.AllowRateLimit("scope", "key", time.Now(), 1*time.Minute, 0)
	if err == nil {
		t.Fatal("expected error for zero maxAttempts")
	}

	_, _, err = store.AllowRateLimit("scope", "key", time.Now(), 1*time.Minute, -1)
	if err == nil {
		t.Fatal("expected error for negative maxAttempts")
	}
}

// ---------------------------------------------------------------------------
// Leasing — TryAcquire
// ---------------------------------------------------------------------------

func TestTryAcquireLease_Acquire(t *testing.T) {
	store, _ := newTestStore(t)
	acquired, err := store.TryAcquireLease("job-1", "worker-a", 30*time.Second)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if !acquired {
		t.Fatal("first acquire should succeed")
	}
}

func TestTryAcquireLease_Conflict(t *testing.T) {
	store, _ := newTestStore(t)
	if _, err := store.TryAcquireLease("job-1", "worker-a", 30*time.Second); err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	acquired, err := store.TryAcquireLease("job-1", "worker-b", 30*time.Second)
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	if acquired {
		t.Fatal("second acquire should fail (conflict)")
	}
}

func TestTryAcquireLease_EmptyKey(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.TryAcquireLease("", "token", 30*time.Second)
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestTryAcquireLease_EmptyToken(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.TryAcquireLease("key", "", 30*time.Second)
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestTryAcquireLease_InvalidTTL(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.TryAcquireLease("key", "token", 0)
	if err == nil {
		t.Fatal("expected error for zero TTL")
	}

	_, err = store.TryAcquireLease("key", "token", -1*time.Second)
	if err == nil {
		t.Fatal("expected error for negative TTL")
	}
}

func TestTryAcquireLease_ReacquireAfterExpiry(t *testing.T) {
	store, mr := newTestStore(t)
	if _, err := store.TryAcquireLease("job-ttl", "worker-a", 2*time.Second); err != nil {
		t.Fatalf("acquire: %v", err)
	}

	mr.FastForward(5 * time.Second)

	acquired, err := store.TryAcquireLease("job-ttl", "worker-b", 30*time.Second)
	if err != nil {
		t.Fatalf("reacquire: %v", err)
	}
	if !acquired {
		t.Fatal("should reacquire after TTL expiry")
	}
}

// ---------------------------------------------------------------------------
// Leasing — Release
// ---------------------------------------------------------------------------

func TestReleaseLease_OwnerReleases(t *testing.T) {
	store, _ := newTestStore(t)
	if _, err := store.TryAcquireLease("job-rel", "worker-a", 30*time.Second); err != nil {
		t.Fatalf("acquire: %v", err)
	}

	if err := store.ReleaseLease("job-rel", "worker-a"); err != nil {
		t.Fatalf("release: %v", err)
	}

	// Another worker should now be able to acquire
	acquired, err := store.TryAcquireLease("job-rel", "worker-b", 30*time.Second)
	if err != nil {
		t.Fatalf("reacquire: %v", err)
	}
	if !acquired {
		t.Fatal("should acquire after owner release")
	}
}

func TestReleaseLease_NonOwnerCannotRelease(t *testing.T) {
	store, _ := newTestStore(t)
	if _, err := store.TryAcquireLease("job-no", "worker-a", 30*time.Second); err != nil {
		t.Fatalf("acquire: %v", err)
	}

	// Non-owner release should not error but should not remove the lease
	if err := store.ReleaseLease("job-no", "worker-b"); err != nil {
		t.Fatalf("release by non-owner: %v", err)
	}

	// Original owner's lease should still be held
	acquired, err := store.TryAcquireLease("job-no", "worker-c", 30*time.Second)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if acquired {
		t.Fatal("non-owner release should not have freed the lease")
	}
}

func TestReleaseLease_EmptyParams(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.ReleaseLease("", "token")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	err = store.ReleaseLease("key", "")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

// ---------------------------------------------------------------------------
// Worker Queue — Enqueue
// ---------------------------------------------------------------------------

func TestEnqueueWorkerQueue_Basic(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.EnqueueWorkerQueue("emails", "item-1")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
}

func TestEnqueueWorkerQueue_EmptyQueueName(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.EnqueueWorkerQueue("", "item-1")
	if err == nil {
		t.Fatal("expected error for empty queue name")
	}
}

func TestEnqueueWorkerQueue_EmptyItemID(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.EnqueueWorkerQueue("emails", "")
	if err == nil {
		t.Fatal("expected error for empty item ID")
	}
}

func TestEnqueueWorkerQueue_Duplicate(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.EnqueueWorkerQueue("emails", "dup-1"); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	// Second enqueue of same item should not error (idempotent)
	if err := store.EnqueueWorkerQueue("emails", "dup-1"); err != nil {
		t.Fatalf("duplicate enqueue should not error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Worker Queue — Dequeue
// ---------------------------------------------------------------------------

func TestDequeueWorkerQueueBatch_Basic(t *testing.T) {
	store, _ := newTestStore(t)
	for _, id := range []string{"a", "b", "c"} {
		if err := store.EnqueueWorkerQueue("q", id); err != nil {
			t.Fatalf("enqueue %s: %v", id, err)
		}
	}

	items, err := store.DequeueWorkerQueueBatch("q", 2)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0] != "a" || items[1] != "b" {
		t.Fatalf("expected [a b], got %v", items)
	}

	// Remaining item
	items, err = store.DequeueWorkerQueueBatch("q", 10)
	if err != nil {
		t.Fatalf("dequeue rest: %v", err)
	}
	if len(items) != 1 || items[0] != "c" {
		t.Fatalf("expected [c], got %v", items)
	}
}

func TestDequeueWorkerQueueBatch_EmptyQueue(t *testing.T) {
	store, _ := newTestStore(t)
	items, err := store.DequeueWorkerQueueBatch("empty-q", 5)
	if err != nil {
		t.Fatalf("dequeue empty: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestDequeueWorkerQueueBatch_EmptyQueueName(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.DequeueWorkerQueueBatch("", 5)
	if err == nil {
		t.Fatal("expected error for empty queue name")
	}
}

func TestDequeueWorkerQueueBatch_InvalidBatchSize(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.DequeueWorkerQueueBatch("q", 0)
	if err == nil {
		t.Fatal("expected error for zero batch size")
	}
	_, err = store.DequeueWorkerQueueBatch("q", -1)
	if err == nil {
		t.Fatal("expected error for negative batch size")
	}
}

// ---------------------------------------------------------------------------
// Worker Queue — Claim
// ---------------------------------------------------------------------------

func TestClaimWorkerQueueBatch_Basic(t *testing.T) {
	store, _ := newTestStore(t)
	for _, id := range []string{"x", "y"} {
		if err := store.EnqueueWorkerQueue("cq", id); err != nil {
			t.Fatalf("enqueue %s: %v", id, err)
		}
	}

	claims, err := store.ClaimWorkerQueueBatch("cq", 5, 30*time.Second)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claims) != 2 {
		t.Fatalf("expected 2 claims, got %d", len(claims))
	}
	for _, c := range claims {
		if c.ItemID == "" {
			t.Fatal("claim itemID should not be empty")
		}
		if c.ClaimToken == "" {
			t.Fatal("claim token should not be empty")
		}
	}
}

func TestClaimWorkerQueueBatch_EmptyQueue(t *testing.T) {
	store, _ := newTestStore(t)
	claims, err := store.ClaimWorkerQueueBatch("empty-cq", 5, 30*time.Second)
	if err != nil {
		t.Fatalf("claim empty: %v", err)
	}
	if len(claims) != 0 {
		t.Fatalf("expected 0 claims, got %d", len(claims))
	}
}

func TestClaimWorkerQueueBatch_EmptyQueueName(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.ClaimWorkerQueueBatch("", 5, 30*time.Second)
	if err == nil {
		t.Fatal("expected error for empty queue name")
	}
}

func TestClaimWorkerQueueBatch_InvalidBatchSize(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.ClaimWorkerQueueBatch("cq", 0, 30*time.Second)
	if err == nil {
		t.Fatal("expected error for zero batch size")
	}
}

func TestClaimWorkerQueueBatch_InvalidVisibilityTimeout(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.ClaimWorkerQueueBatch("cq", 5, 0)
	if err == nil {
		t.Fatal("expected error for zero visibility timeout")
	}
	_, err = store.ClaimWorkerQueueBatch("cq", 5, -1*time.Second)
	if err == nil {
		t.Fatal("expected error for negative visibility timeout")
	}
}

// ---------------------------------------------------------------------------
// Worker Queue — Ack
// ---------------------------------------------------------------------------

func TestAckWorkerQueue_CorrectToken(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.EnqueueWorkerQueue("aq", "item-ack"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	claims, err := store.ClaimWorkerQueueBatch("aq", 1, 30*time.Second)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claim: err=%v len=%d", err, len(claims))
	}

	acked, err := store.AckWorkerQueue("aq", claims[0].ItemID, claims[0].ClaimToken)
	if err != nil {
		t.Fatalf("ack: %v", err)
	}
	if !acked {
		t.Fatal("expected ack to succeed")
	}
}

func TestAckWorkerQueue_WrongToken(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.EnqueueWorkerQueue("aq2", "item-wrong"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	claims, err := store.ClaimWorkerQueueBatch("aq2", 1, 30*time.Second)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claim: err=%v len=%d", err, len(claims))
	}

	acked, err := store.AckWorkerQueue("aq2", claims[0].ItemID, "wrong-token")
	if err != nil {
		t.Fatalf("ack: %v", err)
	}
	if acked {
		t.Fatal("ack with wrong token should return false")
	}
}

func TestAckWorkerQueue_EmptyParams(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.AckWorkerQueue("", "item", "token")
	if err == nil {
		t.Fatal("expected error for empty queue name")
	}
	_, err = store.AckWorkerQueue("q", "", "token")
	if err == nil {
		t.Fatal("expected error for empty item ID")
	}
	_, err = store.AckWorkerQueue("q", "item", "")
	if err == nil {
		t.Fatal("expected error for empty claim token")
	}
}

// ---------------------------------------------------------------------------
// Worker Queue — Requeue
// ---------------------------------------------------------------------------

func TestRequeueWorkerQueue_CorrectToken(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.EnqueueWorkerQueue("rq", "item-req"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	claims, err := store.ClaimWorkerQueueBatch("rq", 1, 30*time.Second)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claim: err=%v len=%d", err, len(claims))
	}

	requeued, err := store.RequeueWorkerQueue("rq", claims[0].ItemID, claims[0].ClaimToken)
	if err != nil {
		t.Fatalf("requeue: %v", err)
	}
	if !requeued {
		t.Fatal("expected requeue to succeed")
	}

	// Item should now be back in the queue and claimable again
	claims2, err := store.ClaimWorkerQueueBatch("rq", 1, 30*time.Second)
	if err != nil {
		t.Fatalf("re-claim: %v", err)
	}
	if len(claims2) != 1 {
		t.Fatalf("expected 1 re-claimed item, got %d", len(claims2))
	}
	if claims2[0].ItemID != "item-req" {
		t.Fatalf("expected item-req, got %s", claims2[0].ItemID)
	}
}

func TestRequeueWorkerQueue_WrongToken(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.EnqueueWorkerQueue("rq2", "item-rq"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	claims, err := store.ClaimWorkerQueueBatch("rq2", 1, 30*time.Second)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claim: err=%v len=%d", err, len(claims))
	}

	requeued, err := store.RequeueWorkerQueue("rq2", claims[0].ItemID, "bad-token")
	if err != nil {
		t.Fatalf("requeue: %v", err)
	}
	if requeued {
		t.Fatal("requeue with wrong token should return false")
	}
}

func TestRequeueWorkerQueue_EmptyParams(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.RequeueWorkerQueue("", "item", "token")
	if err == nil {
		t.Fatal("expected error for empty queue name")
	}
	_, err = store.RequeueWorkerQueue("q", "", "token")
	if err == nil {
		t.Fatal("expected error for empty item ID")
	}
	_, err = store.RequeueWorkerQueue("q", "item", "")
	if err == nil {
		t.Fatal("expected error for empty claim token")
	}
}

// ---------------------------------------------------------------------------
// Worker Queue — Describe
// ---------------------------------------------------------------------------

func TestDescribeWorkerQueue_QueuedItems(t *testing.T) {
	store, _ := newTestStore(t)
	for _, id := range []string{"d1", "d2"} {
		if err := store.EnqueueWorkerQueue("dq", id); err != nil {
			t.Fatalf("enqueue %s: %v", id, err)
		}
	}

	tel, err := store.DescribeWorkerQueue("dq", []string{"d1", "d2", "d3"})
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	if tel.PendingCount != 2 {
		t.Fatalf("expected 2 pending, got %d", tel.PendingCount)
	}
	if tel.ClaimedCount != 0 {
		t.Fatalf("expected 0 claimed, got %d", tel.ClaimedCount)
	}
	if state, ok := tel.Items["d1"]; !ok || state.State != WorkerQueueStateQueued {
		t.Fatalf("d1 should be queued, got %+v", tel.Items["d1"])
	}
	if state, ok := tel.Items["d3"]; !ok || state.State != WorkerQueueStateMissing {
		t.Fatalf("d3 should be missing, got %+v", state)
	}
}

func TestDescribeWorkerQueue_ClaimedItems(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.EnqueueWorkerQueue("dqc", "c1"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	claims, err := store.ClaimWorkerQueueBatch("dqc", 1, 30*time.Second)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claim: err=%v len=%d", err, len(claims))
	}

	tel, err := store.DescribeWorkerQueue("dqc", []string{"c1"})
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	if tel.ClaimedCount != 1 {
		t.Fatalf("expected 1 claimed, got %d", tel.ClaimedCount)
	}
	state, ok := tel.Items["c1"]
	if !ok || state.State != WorkerQueueStateClaimed {
		t.Fatalf("c1 should be claimed, got %+v", state)
	}
	if state.VisibilityDeadlineAt == nil {
		t.Fatal("claimed item should have a visibility deadline")
	}
}

func TestDescribeWorkerQueue_EmptyQueueName(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.DescribeWorkerQueue("", nil)
	if err == nil {
		t.Fatal("expected error for empty queue name")
	}
}

func TestDescribeWorkerQueue_NoItemIDs(t *testing.T) {
	store, _ := newTestStore(t)
	tel, err := store.DescribeWorkerQueue("dq-empty", nil)
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	if tel.PendingCount != 0 {
		t.Fatalf("expected 0 pending, got %d", tel.PendingCount)
	}
	if len(tel.Items) != 0 {
		t.Fatalf("expected empty items map, got %d entries", len(tel.Items))
	}
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func TestClose_NilStore(t *testing.T) {
	var s *Store
	if err := s.Close(); err != nil {
		t.Fatalf("close nil store should not error: %v", err)
	}
}

func TestClose_ValidStore(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}
