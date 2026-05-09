package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestAllowedCORSOriginMatchesConfiguredOriginList(t *testing.T) {
	configured := "http://localhost:5173, http://127.0.0.1:5173"

	if got := allowedCORSOrigin(configured, "http://127.0.0.1:5173", "development"); got != "http://127.0.0.1:5173" {
		t.Fatalf("expected 127.0.0.1 origin, got %q", got)
	}
	if got := allowedCORSOrigin(configured, "http://localhost:5173", "development"); got != "http://localhost:5173" {
		t.Fatalf("expected localhost origin, got %q", got)
	}
	if got := allowedCORSOrigin(configured, "http://localhost:5174", "development"); got != "" {
		t.Fatalf("expected unconfigured origin to be rejected, got %q", got)
	}
	if got := allowedCORSOrigin(configured, "", "development"); got != "http://localhost:5173" {
		t.Fatalf("expected first configured origin as fallback, got %q", got)
	}

	// wildcard accepted in development
	if got := allowedCORSOrigin("*", "http://evil.com", "development"); got != "*" {
		t.Fatalf("expected wildcard in development, got %q", got)
	}
	// wildcard rejected in production
	if got := allowedCORSOrigin("*", "http://evil.com", "production"); got != "" {
		t.Fatalf("expected wildcard rejected in production, got %q", got)
	}
}

func TestRouterCORSReflectsAllowedRequestOrigin(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:  "cors-test-secret",
		CORSOrigin: "http://localhost:5173, http://127.0.0.1:5173",
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:5173" {
		t.Fatalf("expected reflected allowed origin, got %q", got)
	}
	if got := rec.Header().Values("Vary"); !containsHeaderValue(got, "Origin") {
		t.Fatalf("expected Vary Origin, got %q", strings.Join(got, ", "))
	}
	if got := rec.Header().Values("Access-Control-Expose-Headers"); !containsHeaderValue(got, "X-Collection-Range") {
		t.Fatalf("expected exposed collection range header, got %q", strings.Join(got, ", "))
	}
	if got := rec.Header().Values("Access-Control-Expose-Headers"); !containsHeaderValue(got, "Deprecation") ||
		!containsHeaderValue(got, "Link") ||
		!containsHeaderValue(got, "X-MistyPass-Replacement") {
		t.Fatalf("expected exposed deprecation headers, got %q", strings.Join(got, ", "))
	}
}

func containsHeaderValue(values []string, expected string) bool {
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if strings.TrimSpace(part) == expected {
				return true
			}
		}
	}
	return false
}
