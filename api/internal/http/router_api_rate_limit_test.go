package httpx

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestAllowGlobalAPIAttemptRateLimit(t *testing.T) {
	s := &server{
		apiRateLimitBuckets: map[string]loginRateLimitBucket{},
	}
	now := time.Date(2026, 4, 17, 8, 0, 0, 0, time.UTC)

	for i := 0; i < apiRateLimitMaxRequests; i++ {
		allowed, retryAfter := s.allowGlobalAPIAttempt("10.20.30.40", now)
		if !allowed {
			t.Fatalf("expected attempt %d to be allowed, retryAfter=%s", i+1, retryAfter)
		}
	}

	allowed, retryAfter := s.allowGlobalAPIAttempt("10.20.30.40", now.Add(5*time.Second))
	if allowed {
		t.Fatalf("expected attempt exceeding limit to be denied")
	}
	if retryAfter <= 0 {
		t.Fatalf("expected positive retryAfter when denied")
	}
}

func TestAllowGlobalAPIAttemptResetsAfterWindow(t *testing.T) {
	s := &server{
		apiRateLimitBuckets: map[string]loginRateLimitBucket{},
	}
	now := time.Date(2026, 4, 17, 8, 0, 0, 0, time.UTC)

	for i := 0; i < apiRateLimitMaxRequests; i++ {
		allowed, _ := s.allowGlobalAPIAttempt("10.20.30.41", now)
		if !allowed {
			t.Fatalf("expected attempt %d to be allowed", i+1)
		}
	}

	allowed, _ := s.allowGlobalAPIAttempt("10.20.30.41", now.Add(apiRateLimitWindow+time.Second))
	if !allowed {
		t.Fatalf("expected limit window to reset")
	}
}

func TestWithGlobalAPIRateLimitReturns429(t *testing.T) {
	s := &server{
		apiRateLimitBuckets: map[string]loginRateLimitBucket{},
	}

	handler := s.withGlobalAPIRateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for i := 0; i < apiRateLimitMaxRequests; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.8")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected status %d for request %d, got %d", http.StatusNoContent, i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.8")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header")
	}
}

func TestWithEnterprisePublicRateLimitReturns429(t *testing.T) {
	s := &server{
		enterprisePublicRateBuckets: map[string]loginRateLimitBucket{},
	}

	handler := s.withEnterprisePublicRateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for i := 0; i < enterprisePublicRateLimitMaxRequests; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/start", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.10")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected status %d for request %d, got %d", http.StatusNoContent, i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/start", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header")
	}
}

func TestWithEnterpriseWebhookRateLimitReturns429(t *testing.T) {
	s := &server{
		enterpriseWebhookRateBuckets: map[string]loginRateLimitBucket{},
	}

	handler := s.withEnterpriseWebhookRateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	for i := 0; i < enterpriseWebhookRateLimitMaxRequests; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/hris-webhook/connector-talenta", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.11")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected status %d for request %d, got %d", http.StatusAccepted, i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/hris-webhook/connector-talenta", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.11")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header")
	}
}

func TestCompactGlobalAPIRateBucketsCapsMemoryUsage(t *testing.T) {
	s := &server{
		apiRateLimitBuckets: map[string]loginRateLimitBucket{},
	}
	now := time.Date(2026, 4, 24, 8, 0, 0, 0, time.UTC)

	for i := 0; i < apiRateLimitBucketMaxKeys+111; i++ {
		s.apiRateLimitBuckets["api_"+strconv.Itoa(i)] = loginRateLimitBucket{
			WindowStart: now,
			Attempts:    1,
		}
	}

	s.compactGlobalAPIRateBuckets(now)

	if got := len(s.apiRateLimitBuckets); got > apiRateLimitBucketMaxKeys/2 {
		t.Fatalf("expected compacted api buckets <= %d, got %d", apiRateLimitBucketMaxKeys/2, got)
	}
}

func TestCompactLoginRateBucketsCapsMemoryUsage(t *testing.T) {
	s := &server{
		loginRateLimitBuckets: map[string]loginRateLimitBucket{},
	}
	now := time.Date(2026, 4, 24, 8, 0, 0, 0, time.UTC)

	for i := 0; i < loginRateLimitBucketMaxKeys+111; i++ {
		s.loginRateLimitBuckets["login_"+strconv.Itoa(i)] = loginRateLimitBucket{
			WindowStart: now,
			Attempts:    1,
		}
	}

	s.compactLoginRateBuckets(now)

	if got := len(s.loginRateLimitBuckets); got > loginRateLimitBucketMaxKeys/2 {
		t.Fatalf("expected compacted login buckets <= %d, got %d", loginRateLimitBucketMaxKeys/2, got)
	}
}
