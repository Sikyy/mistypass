package main

import (
	"testing"
	"time"
)

func TestNonceCache_AddAndContains(t *testing.T) {
	cache := NewNonceCache(100, 30*time.Second)

	nonce := [32]byte{1, 2, 3}
	if cache.Contains(nonce[:]) {
		t.Fatal("should not contain unused nonce")
	}

	cache.Add(nonce[:])
	if !cache.Contains(nonce[:]) {
		t.Fatal("should contain added nonce")
	}
}

func TestNonceCache_Expiry(t *testing.T) {
	cache := NewNonceCache(100, 50*time.Millisecond) // short TTL for test

	nonce := [32]byte{1, 2, 3}
	cache.Add(nonce[:])

	if !cache.Contains(nonce[:]) {
		t.Fatal("should contain nonce immediately")
	}

	time.Sleep(60 * time.Millisecond)

	if cache.Contains(nonce[:]) {
		t.Fatal("should not contain expired nonce")
	}
}

func TestNonceCache_MaxSize(t *testing.T) {
	cache := NewNonceCache(2, 10*time.Second)

	n1 := [32]byte{1}
	n2 := [32]byte{2}
	n3 := [32]byte{3}

	cache.Add(n1[:])
	cache.Add(n2[:])
	cache.Add(n3[:]) // should evict n1

	if cache.Contains(n1[:]) {
		t.Fatal("n1 should have been evicted")
	}
	if !cache.Contains(n2[:]) {
		t.Fatal("n2 should still exist")
	}
	if !cache.Contains(n3[:]) {
		t.Fatal("n3 should exist")
	}
}

func TestNonceCache_DuplicateAdd(t *testing.T) {
	cache := NewNonceCache(3, 10*time.Second)

	n1 := [32]byte{1}
	n2 := [32]byte{2}

	cache.Add(n1[:])
	cache.Add(n1[:]) // duplicate — should be no-op
	cache.Add(n2[:])

	// Both should exist; order slice should have 2 entries, not 3
	if !cache.Contains(n1[:]) {
		t.Fatal("n1 should still exist")
	}
	if !cache.Contains(n2[:]) {
		t.Fatal("n2 should exist")
	}

	// Add a third distinct nonce — should NOT evict n1 (capacity=3, only 2 unique)
	n3 := [32]byte{3}
	cache.Add(n3[:])
	if !cache.Contains(n1[:]) {
		t.Fatal("n1 should still exist after n3 — only 3 unique entries for capacity 3")
	}
}
