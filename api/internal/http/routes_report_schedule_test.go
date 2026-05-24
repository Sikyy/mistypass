package httpx

import (
	"strings"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/pdfgen"
)

func TestNormalizeReportScheduleReportType(t *testing.T) {
	tests := map[string]string{
		"weekly_analytics": "weekly_analytics",
		"access_summary":   "weekly_analytics",
		"events":           "events",
		"visitor_log":      "events",
		"unlock_stats":     "unlock_stats",
		"door_usage":       "unlock_stats",
		"alarm_history":    "incidents",
		"alarm_metrics":    "incidents",
		"hardware":         "hardware",
	}

	for input, expected := range tests {
		got, ok := normalizeReportScheduleReportType(input)
		if !ok {
			t.Fatalf("expected %q to normalize", input)
		}
		if got != expected {
			t.Fatalf("normalize %q = %q, want %q", input, got, expected)
		}
	}

	if _, ok := normalizeReportScheduleReportType("unknown"); ok {
		t.Fatal("expected unknown report type to be rejected")
	}
}

func TestBuildReportEmailHTMLUsesMistyisletBrand(t *testing.T) {
	meta := pdfgen.ReportMeta{
		TenantName:  "Test Tenant",
		PlaceName:   "Jakarta HQ",
		PeriodStart: time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC),
		GeneratedAt: time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC),
	}

	body := buildReportEmailHTML("Weekly ops", "weekly_analytics", meta, "weekly_analytics.pdf")
	for _, expected := range []string{"Mistyislet", "Weekly ops", "Weekly Analytics", "Jakarta HQ", "weekly_analytics.pdf"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected email body to contain %q, body=%s", expected, body)
		}
	}
	if strings.Contains(body, "Please find your scheduled report attached") {
		t.Fatal("expected placeholder email copy to be removed")
	}
}
