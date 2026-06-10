package httpx

import (
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/event"
)

func TestComputeOccupancy(t *testing.T) {
	dayA := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	dayB := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	events := []event.AccessEvent{
		{Actor: "a", Result: "success", BuildingID: "b1", At: dayA.Add(10 * time.Hour)},
		{Actor: "b", Result: "success", BuildingID: "b1", At: dayA.Add(11 * time.Hour)},
		{Actor: "a", Result: "success", BuildingID: "b1", At: dayB.Add(10 * time.Hour)},
		{Actor: "c", Result: "denied", BuildingID: "b1", At: dayA.Add(12 * time.Hour)},
		{Actor: "", Result: "success", BuildingID: "b1", At: dayA.Add(13 * time.Hour)},
		{Actor: "d", Result: "success", BuildingID: "b2", At: dayA.Add(9 * time.Hour)},
	}
	days, peakDate, peakUnique, totalUnique := computeOccupancy(events, dayA, dayB.Add(24*time.Hour), "b1")

	if len(days) != 2 {
		t.Fatalf("expected 2 days, got %d (%+v)", len(days), days)
	}
	if days[0].Date != "2026-06-01" || days[0].UniqueUsers != 2 || days[0].TotalEntries != 2 {
		t.Fatalf("unexpected day A: %+v", days[0])
	}
	if days[1].Date != "2026-06-02" || days[1].UniqueUsers != 1 {
		t.Fatalf("unexpected day B: %+v", days[1])
	}
	if peakDate != "2026-06-01" || peakUnique != 2 {
		t.Fatalf("expected peak day A=2, got %s=%d", peakDate, peakUnique)
	}
	if totalUnique != 2 {
		t.Fatalf("expected 2 total unique users (a,b), got %d", totalUnique)
	}
}

func TestComputeRetentionWeekly(t *testing.T) {
	w1 := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	w2 := w1.Add(7 * 24 * time.Hour)
	events := []event.AccessEvent{
		{Actor: "a", Result: "success", At: w1},
		{Actor: "b", Result: "success", At: w1},
		{Actor: "a", Result: "success", At: w2},
		{Actor: "c", Result: "success", At: w2},
	}
	buckets := computeRetention(events, w1.Add(-time.Hour), w2.Add(24*time.Hour), "", "week")
	if len(buckets) != 2 {
		t.Fatalf("expected 2 weekly buckets, got %d (%+v)", len(buckets), buckets)
	}
	if buckets[0].ActiveUsers != 2 || buckets[0].NewUsers != 2 || buckets[0].ReturningUsers != 0 {
		t.Fatalf("unexpected week1: %+v", buckets[0])
	}
	if buckets[1].ActiveUsers != 2 || buckets[1].NewUsers != 1 || buckets[1].ReturningUsers != 1 {
		t.Fatalf("unexpected week2: %+v", buckets[1])
	}
	if buckets[1].RetentionRate < 0.49 || buckets[1].RetentionRate > 0.51 {
		t.Fatalf("expected week2 retention ~0.5, got %f", buckets[1].RetentionRate)
	}
}

func TestComputeRetentionDailyBuckets(t *testing.T) {
	d1 := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	events := []event.AccessEvent{
		{Actor: "a", Result: "success", At: d1},
		{Actor: "a", Result: "success", At: d2},
	}
	buckets := computeRetention(events, d1.Add(-time.Hour), d2.Add(time.Hour), "", "day")
	if len(buckets) != 2 {
		t.Fatalf("expected 2 daily buckets, got %d (%+v)", len(buckets), buckets)
	}
	if buckets[1].ReturningUsers != 1 {
		t.Fatalf("expected actor a returning on day 2, got %+v", buckets[1])
	}
}
