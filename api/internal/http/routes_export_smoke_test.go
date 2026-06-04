package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

// newStubGotenberg returns an httptest server that mimics Gotenberg's
// /forms/chromium/convert/html endpoint by returning fixed PDF bytes, so the
// export PDF path can be exercised end-to-end without a live Gotenberg.
func newStubGotenberg(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("%PDF-1.4\n%stub-pdf\n%%EOF\n"))
	}))
}

// TestExportReportTypeVocabularyUnified guards the report-export contract: both
// /api/v1/reports/export and /api/v1/analytics/export must accept the analytics
// report-type vocabulary (access_summary, door_activity, alarm_metrics) for both
// PDF and CSV, returning real bytes instead of a 400 "unknown report type".
func TestExportReportTypeVocabularyUnified(t *testing.T) {
	stub := newStubGotenberg(t)
	defer stub.Close()

	router, cleanup, err := NewRouter(config.Config{
		JWTSecret:       "export-smoke-test-secret",
		EnableDemoUsers: true,
		GotenbergURL:    stub.URL,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	const (
		start = "2025-01-01T00:00:00Z"
		end   = "2027-01-01T00:00:00Z"
	)

	// The analytics vocabulary the bug report exercised against both endpoints.
	analyticsTypes := []string{"access_summary", "door_activity", "alarm_metrics"}
	endpoints := []string{"/api/v1/reports/export", "/api/v1/analytics/export"}

	for _, endpoint := range endpoints {
		for _, reportType := range analyticsTypes {
			base := endpoint + "?tenant_id=tenant_demo_jakarta&type=" + reportType + "&start=" + start + "&end=" + end

			// PDF
			pdfRec := referenceAPIRequest(t, router, http.MethodGet, base+"&format=pdf", token, nil)
			if pdfRec.Code != http.StatusOK {
				t.Fatalf("PDF %s type=%s: expected 200, got %d body=%s", endpoint, reportType, pdfRec.Code, pdfRec.Body.String())
			}
			if ct := pdfRec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/pdf") {
				t.Fatalf("PDF %s type=%s: expected application/pdf, got %q", endpoint, reportType, ct)
			}
			if !strings.HasPrefix(pdfRec.Body.String(), "%PDF") {
				t.Fatalf("PDF %s type=%s: expected PDF bytes, got %q", endpoint, reportType, pdfRec.Body.String())
			}

			// CSV
			csvRec := referenceAPIRequest(t, router, http.MethodGet, base+"&format=csv", token, nil)
			if csvRec.Code != http.StatusOK {
				t.Fatalf("CSV %s type=%s: expected 200, got %d body=%s", endpoint, reportType, csvRec.Code, csvRec.Body.String())
			}
			if ct := csvRec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
				t.Fatalf("CSV %s type=%s: expected text/csv, got %q", endpoint, reportType, ct)
			}
			if body := csvRec.Body.String(); strings.Contains(body, "not_implemented") || strings.TrimSpace(body) == "" {
				t.Fatalf("CSV %s type=%s: expected populated CSV, got %q", endpoint, reportType, body)
			}
		}
	}

	// /api/v1/reports/export must keep accepting its native pdfgen vocabulary.
	nativeTypes := []string{"weekly_analytics", "events", "unlock_stats", "user_presence", "incidents", "hardware"}
	for _, reportType := range nativeTypes {
		path := "/api/v1/reports/export?tenant_id=tenant_demo_jakarta&type=" + reportType + "&format=pdf&start=" + start + "&end=" + end
		rec := referenceAPIRequest(t, router, http.MethodGet, path, token, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("native PDF type=%s: expected 200, got %d body=%s", reportType, rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/pdf") {
			t.Fatalf("native PDF type=%s: expected application/pdf, got %q", reportType, ct)
		}
	}

	// A genuinely unknown type must still be rejected with 400.
	bad := referenceAPIRequest(t, router, http.MethodGet,
		"/api/v1/reports/export?tenant_id=tenant_demo_jakarta&type=not_a_report&format=pdf&start="+start+"&end="+end, token, nil)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("unknown type: expected 400, got %d body=%s", bad.Code, bad.Body.String())
	}
}
