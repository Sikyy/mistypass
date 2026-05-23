package pdfgen

import (
	"strings"
	"testing"
	"time"
)

func TestRenderHTML_WeeklyAnalytics(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	meta := ReportMeta{
		TenantName:  "Test Corp",
		PlaceName:   "HQ",
		PeriodStart: time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC),
		GeneratedAt: time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC),
	}
	data := WeeklyAnalyticsData{
		DailyUsage: []DailyUsageRow{
			{Date: "2026-05-17", Unlocks: 50, UniqueUsers: 10, Occupancy: 80},
		},
		UnlocksByType: map[string]int{"BLE": 30, "PIN": 20},
		TopDoors:      []DoorRanking{{Door: "Main Entrance", Unlocks: 45}},
	}
	html, err := r.RenderHTML("weekly_analytics", meta, data)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	output := string(html)
	if !strings.Contains(output, "Test Corp") {
		t.Error("expected tenant name in output")
	}
	if !strings.Contains(output, "HQ") {
		t.Error("expected place name in output")
	}
	if !strings.Contains(output, "chart.js") || !strings.Contains(output, "Chart") {
		t.Error("expected Chart.js in output")
	}
	if !strings.Contains(output, "reportData") {
		t.Error("expected reportData JSON injection")
	}
}

func TestRenderHTML_InvalidType(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	meta := ReportMeta{
		TenantName: "Test", PlaceName: "HQ",
		PeriodStart: time.Now(), PeriodEnd: time.Now(), GeneratedAt: time.Now(),
	}
	_, err = r.RenderHTML("nonexistent_type", meta, nil)
	if err == nil {
		t.Error("expected error for invalid report type")
	}
}

func TestAllTemplatesParse(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	types := []string{
		"weekly_analytics", "events", "unlock_stats",
		"user_presence", "incidents", "hardware",
	}
	for _, rt := range types {
		if _, exists := r.templates[rt]; !exists {
			t.Errorf("template %q not loaded", rt)
		}
	}
}
