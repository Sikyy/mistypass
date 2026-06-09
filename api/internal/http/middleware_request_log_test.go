package httpx

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithRequestLogRecordsStructuredFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	s := &server{logger: logger}

	handler := s.withRequestLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants?region=ap", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.Header.Set("User-Agent", "mistypass-test")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	output := buf.String()
	assertContains(t, output, `"msg":"http request completed"`)
	assertContains(t, output, `"method":"POST"`)
	assertContains(t, output, `"path":"/api/v1/tenants"`)
	assertContains(t, output, `"query":"region=ap"`)
	assertContains(t, output, `"status":201`)
	assertContains(t, output, `"request_id":"-"`)
	assertContains(t, output, `"client_ip":"192.0.2.1"`)
	assertContains(t, output, `"user_agent":"mistypass-test"`)
	assertContains(t, output, `"duration_ms":`)
}

func TestWithRequestLogDefaultsStatusWhenHandlerDoesNotWrite(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	s := &server{logger: logger}

	handler := s.withRequestLog(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	assertContains(t, buf.String(), `"status":200`)
}

func assertContains(t *testing.T, content, expected string) {
	t.Helper()
	if !strings.Contains(content, expected) {
		t.Fatalf("expected output to contain %q, output=%s", expected, content)
	}
}

func TestWithRequestLogRedactsUploadSignedURLParams(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	s := &server{logger: logger}

	handler := s.withRequestLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	// A signed blob-download URL (same shape OTA firmware downloads use).
	req := httptest.NewRequest(http.MethodGet, "/api/v1/uploads/upl_abc?sig=deadbeefcafe&expires=1717999999&uid=user_42", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	output := buf.String()
	for _, secret := range []string{"deadbeefcafe", "1717999999", "user_42"} {
		if strings.Contains(output, secret) {
			t.Fatalf("signed-URL credential %q leaked into request log: %s", secret, output)
		}
	}
	assertContains(t, output, "REDACTED")
}

func TestRedactSensitiveQuery(t *testing.T) {
	got := redactSensitiveQuery("/api/v1/uploads/upl_abc123", "sig=deadbeefcafe&expires=1717999999&uid=user_42")
	for _, secret := range []string{"deadbeefcafe", "1717999999", "user_42"} {
		if strings.Contains(got, secret) {
			t.Fatalf("sensitive value %q leaked in redacted query: %q", secret, got)
		}
	}
	if !strings.Contains(got, "sig=REDACTED") || !strings.Contains(got, "expires=REDACTED") || !strings.Contains(got, "uid=REDACTED") {
		t.Fatalf("expected REDACTED markers for sig/uid/expires, got %q", got)
	}

	// Non-uploads path: untouched.
	if got := redactSensitiveQuery("/api/v1/gateways", "tenant_id=t1&foo=bar"); got != "tenant_id=t1&foo=bar" {
		t.Fatalf("non-uploads query must be untouched, got %q", got)
	}
	// /uploads without sensitive params: unchanged (no re-encode).
	if got := redactSensitiveQuery("/api/v1/uploads/upl_abc", "foo=bar"); got != "foo=bar" {
		t.Fatalf("uploads query without sensitive params must be untouched, got %q", got)
	}
	// Empty query stays empty.
	if got := redactSensitiveQuery("/api/v1/uploads/upl_abc", ""); got != "" {
		t.Fatalf("empty query must stay empty, got %q", got)
	}
	// Malformed query on a sensitive path: fail closed.
	if got := redactSensitiveQuery("/api/v1/uploads/upl_abc", "sig=%zz&bad"); got != "REDACTED" {
		t.Fatalf("malformed sensitive query must fail closed to REDACTED, got %q", got)
	}
}
