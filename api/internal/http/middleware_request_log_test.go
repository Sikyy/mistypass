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

func TestWithRequestLogRedactsSignedDownloadQueryParams(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	s := &server{logger: logger}

	handler := s.withRequestLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const secretSig = "a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/uploads/upl_abc123?sig="+secretSig+"&expires=1893456000", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	output := buf.String()
	if strings.Contains(output, secretSig) {
		t.Fatalf("log output leaked raw upload signature, output=%s", output)
	}
	assertContains(t, output, `"path":"/api/v1/uploads/upl_abc123"`)
	assertContains(t, output, "sig=REDACTED")
	assertContains(t, output, "expires=REDACTED")
}

func TestWithRequestLogRedactsSignedUploadQueryParams(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	s := &server{logger: logger}

	handler := s.withRequestLog(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	const secretSig = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	req := httptest.NewRequest(http.MethodPut, "/api/v1/uploads/upl_def456?sig="+secretSig+"&uid=user-123&expires=1893456000", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	output := buf.String()
	if strings.Contains(output, secretSig) {
		t.Fatalf("log output leaked raw upload signature, output=%s", output)
	}
	if strings.Contains(output, "user-123") {
		t.Fatalf("log output leaked raw uid, output=%s", output)
	}
	assertContains(t, output, "sig=REDACTED")
	assertContains(t, output, "uid=REDACTED")
	assertContains(t, output, "expires=REDACTED")
}

func TestWithRequestLogRedactsPercentEncodedSensitiveKeys(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	s := &server{logger: logger}

	handler := s.withRequestLog(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	const secretSig = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	// %73ig decodes to "sig" and %65xpires to "expires": the uploads handlers read
	// params via r.URL.Query(), which percent-decodes key names, so these spellings
	// are honored as live capabilities and must be redacted just the same.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/uploads/upl_ghi789?%73ig="+secretSig+"&%65xpires=1893456000&uid=user-456", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	output := buf.String()
	if strings.Contains(output, secretSig) {
		t.Fatalf("log output leaked percent-encoded signature, output=%s", output)
	}
	if strings.Contains(output, "user-456") {
		t.Fatalf("log output leaked raw uid, output=%s", output)
	}
	assertContains(t, output, "uid=REDACTED")
}

func TestWithRequestLogDoesNotRedactNonUploadQuery(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	s := &server{logger: logger}

	handler := s.withRequestLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// A "sig" query param outside the uploads endpoints must be logged verbatim.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health?sig=keepme&region=ap", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertContains(t, buf.String(), `"query":"sig=keepme&region=ap"`)
}

func assertContains(t *testing.T, content, expected string) {
	t.Helper()
	if !strings.Contains(content, expected) {
		t.Fatalf("expected output to contain %q, output=%s", expected, content)
	}
}
