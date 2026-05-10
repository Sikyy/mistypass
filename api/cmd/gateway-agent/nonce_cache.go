package main

import (
	"encoding/hex"
	"sync"
	"time"
)

type nonceCacheEntry struct {
	key       string
	expiresAt time.Time
}

// NonceCache is an in-memory LRU cache with TTL for preventing nonce replay.
// Thread-safe. Memory: ~320KB at max 10,000 entries (32 bytes/nonce).
type NonceCache struct {
	mu      sync.Mutex
	entries map[string]time.Time
	order   []nonceCacheEntry
	maxSize int
	ttl     time.Duration
}

func NewNonceCache(maxSize int, ttl time.Duration) *NonceCache {
	return &NonceCache{
		entries: make(map[string]time.Time, maxSize),
		order:   make([]nonceCacheEntry, 0, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

func (c *NonceCache) Contains(nonce []byte) bool {
	key := hex.EncodeToString(nonce)
	c.mu.Lock()
	defer c.mu.Unlock()

	c.evictExpired()

	expiresAt, ok := c.entries[key]
	if !ok {
		return false
	}
	return time.Now().Before(expiresAt)
}

func (c *NonceCache) Add(nonce []byte) {
	key := hex.EncodeToString(nonce)
	c.mu.Lock()
	defer c.mu.Unlock()

	c.evictExpired()

	// Evict oldest if at capacity
	for len(c.entries) >= c.maxSize && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest.key)
	}

	expiresAt := time.Now().Add(c.ttl)
	c.entries[key] = expiresAt
	c.order = append(c.order, nonceCacheEntry{key: key, expiresAt: expiresAt})
}

func (c *NonceCache) evictExpired() {
	now := time.Now()
	cutoff := 0
	for cutoff < len(c.order) && now.After(c.order[cutoff].expiresAt) {
		delete(c.entries, c.order[cutoff].key)
		cutoff++
	}
	if cutoff > 0 {
		c.order = c.order[cutoff:]
	}
}
