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
	assertContains(t, output, `"client_ip":"203.0.113.9"`)
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
