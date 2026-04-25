package enterprise

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchJWKSUsesCacheWithinTTL(t *testing.T) {
	clearOIDCJWKSCache()
	previousNow := oidcJWKSNow
	previousTTL := oidcJWKSCacheTTL
	t.Cleanup(func() {
		oidcJWKSNow = previousNow
		oidcJWKSCacheTTL = previousTTL
		clearOIDCJWKSCache()
	})

	current := time.Date(2026, time.April, 18, 6, 0, 0, 0, time.UTC)
	oidcJWKSNow = func() time.Time { return current }
	oidcJWKSCacheTTL = time.Hour

	payload := mustBuildTestJWKS(t, "kid-cache")
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	keysA, err := fetchJWKS(server.URL)
	if err != nil {
		t.Fatalf("fetchJWKS first call returned error: %v", err)
	}
	if len(keysA) != 1 {
		t.Fatalf("fetchJWKS first call key count = %d, want 1", len(keysA))
	}

	keysA["kid-cache"].E = 3

	keysB, err := fetchJWKS(server.URL)
	if err != nil {
		t.Fatalf("fetchJWKS second call returned error: %v", err)
	}
	if len(keysB) != 1 {
		t.Fatalf("fetchJWKS second call key count = %d, want 1", len(keysB))
	}
	if keysB["kid-cache"].E == 3 {
		t.Fatalf("fetchJWKS cache returned a mutated key copy")
	}
	if hits.Load() != 1 {
		t.Fatalf("JWKS endpoint hit count = %d, want 1", hits.Load())
	}
}

func TestFetchJWKSRefetchesAfterCacheExpiry(t *testing.T) {
	clearOIDCJWKSCache()
	previousNow := oidcJWKSNow
	previousTTL := oidcJWKSCacheTTL
	previousMaxEntries := oidcJWKSCacheMaxEntries
	t.Cleanup(func() {
		oidcJWKSNow = previousNow
		oidcJWKSCacheTTL = previousTTL
		oidcJWKSCacheMaxEntries = previousMaxEntries
		clearOIDCJWKSCache()
	})

	current := time.Date(2026, time.April, 18, 6, 30, 0, 0, time.UTC)
	oidcJWKSNow = func() time.Time { return current }
	oidcJWKSCacheTTL = time.Hour

	payload := mustBuildTestJWKS(t, "kid-expiry")
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	if _, err := fetchJWKS(server.URL); err != nil {
		t.Fatalf("fetchJWKS first call returned error: %v", err)
	}

	current = current.Add(2 * time.Hour)

	if _, err := fetchJWKS(server.URL); err != nil {
		t.Fatalf("fetchJWKS second call after expiry returned error: %v", err)
	}

	if hits.Load() != 2 {
		t.Fatalf("JWKS endpoint hit count = %d, want 2 after expiry", hits.Load())
	}
}

func TestFetchJWKSEvictsOldestWhenCacheLimitReached(t *testing.T) {
	clearOIDCJWKSCache()
	previousNow := oidcJWKSNow
	previousTTL := oidcJWKSCacheTTL
	previousMaxEntries := oidcJWKSCacheMaxEntries
	t.Cleanup(func() {
		oidcJWKSNow = previousNow
		oidcJWKSCacheTTL = previousTTL
		oidcJWKSCacheMaxEntries = previousMaxEntries
		clearOIDCJWKSCache()
	})

	current := time.Date(2026, time.April, 18, 7, 0, 0, 0, time.UTC)
	oidcJWKSNow = func() time.Time { return current }
	oidcJWKSCacheTTL = time.Hour
	oidcJWKSCacheMaxEntries = 2

	var hits atomic.Int32
	newJWKSHandler := func(kid string) *httptest.Server {
		payload := mustBuildTestJWKS(t, kid)
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			_, _ = w.Write(payload)
		}))
	}

	serverA := newJWKSHandler("kid-a")
	defer serverA.Close()
	serverB := newJWKSHandler("kid-b")
	defer serverB.Close()
	serverC := newJWKSHandler("kid-c")
	defer serverC.Close()

	if _, err := fetchJWKS(serverA.URL); err != nil {
		t.Fatalf("fetchJWKS serverA returned error: %v", err)
	}
	current = current.Add(time.Minute)
	if _, err := fetchJWKS(serverB.URL); err != nil {
		t.Fatalf("fetchJWKS serverB returned error: %v", err)
	}
	current = current.Add(time.Minute)
	if _, err := fetchJWKS(serverC.URL); err != nil {
		t.Fatalf("fetchJWKS serverC returned error: %v", err)
	}

	cacheSize := 0
	oidcJWKSCache.mu.RLock()
	cacheSize = len(oidcJWKSCache.entries)
	if cacheSize != 2 {
		oidcJWKSCache.mu.RUnlock()
		t.Fatalf("jwks cache size = %d, want 2", cacheSize)
	}
	if _, exists := oidcJWKSCache.entries[serverA.URL]; exists {
		oidcJWKSCache.mu.RUnlock()
		t.Fatalf("expected oldest cache entry to be evicted")
	}
	oidcJWKSCache.mu.RUnlock()
}

func TestReadJWKSBytesRejectsOversizedPayload(t *testing.T) {
	previousMaxBytes := oidcJWKSPayloadMaxBytes
	t.Cleanup(func() {
		oidcJWKSPayloadMaxBytes = previousMaxBytes
	})
	oidcJWKSPayloadMaxBytes = 64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, oidcJWKSPayloadMaxBytes+1))
	}))
	defer server.Close()

	if _, err := readJWKSBytes(server.URL); err == nil {
		t.Fatalf("expected oversized jwks payload to be rejected")
	}
}

func mustBuildTestJWKS(t *testing.T, kid string) []byte {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	doc := jwksDocument{
		Keys: []jwkKey{
			{
				Kid: kid,
				Kty: "RSA",
				Alg: "RS256",
				Use: "sig",
				N:   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
				E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes()),
			},
		},
	}

	payload, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal jwks payload: %v", err)
	}
	return payload
}
