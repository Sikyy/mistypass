package httpx

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestAllowLoginAttemptRateLimit(t *testing.T) {
	s := &server{
		loginRateLimitBuckets: map[string]loginRateLimitBucket{},
	}
	now := time.Date(2026, 4, 17, 8, 0, 0, 0, time.UTC)

	for i := 0; i < loginRateLimitMaxAttempts; i++ {
		allowed, retryAfter := s.allowLoginAttempt("10.20.30.40", now)
		if !allowed {
			t.Fatalf("expected attempt %d to be allowed, retryAfter=%s", i+1, retryAfter)
		}
	}

	allowed, retryAfter := s.allowLoginAttempt("10.20.30.40", now.Add(5*time.Second))
	if allowed {
		t.Fatalf("expected attempt exceeding limit to be denied")
	}
	if retryAfter <= 0 {
		t.Fatalf("expected positive retryAfter when denied")
	}
}

func TestAllowLoginAttemptResetsAfterWindow(t *testing.T) {
	s := &server{
		loginRateLimitBuckets: map[string]loginRateLimitBucket{},
	}
	now := time.Date(2026, 4, 17, 8, 0, 0, 0, time.UTC)

	for i := 0; i < loginRateLimitMaxAttempts; i++ {
		allowed, _ := s.allowLoginAttempt("10.20.30.41", now)
		if !allowed {
			t.Fatalf("expected attempt %d to be allowed", i+1)
		}
	}

	allowed, _ := s.allowLoginAttempt("10.20.30.41", now.Add(loginRateLimitWindow+time.Second))
	if !allowed {
		t.Fatalf("expected limit window to reset")
	}
}

func TestRequestClientIPPrefersForwardedHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/auth/login", nil)
	req.RemoteAddr = "127.0.0.1:8080"
	req.Header.Set("X-Forwarded-For", "203.0.113.8, 10.0.0.8")
	req.Header.Set("X-Real-IP", "198.51.100.8")

	if got := requestClientIP(req); got != "203.0.113.8" {
		t.Fatalf("expected forwarded ip, got %s", got)
	}
}
