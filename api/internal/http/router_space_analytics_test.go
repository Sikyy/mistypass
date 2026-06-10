package httpx

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/config"
)

func spaceAnalyticsWindow() (string, string) {
	// Demo access events are seeded relative to now; a +/- 2-day window captures them.
	// Use fixed offsets from a base; tests don't depend on wall clock for assertions
	// beyond "demo events fall inside".
	start := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	end := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)
	return start, end
}

func TestOccupancyAnalyticsEndpoint(t *testing.T) {
	router, _, err := NewRouter(config.Config{JWTSecret: "occupancy-test", EnableDemoUsers: true}, nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	start, end := spaceAnalyticsWindow()

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/analytics/occupancy?tenant_id=tenant_demo_jakarta&start="+start+"&end="+end, token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Days []struct {
			Date        string `json:"date"`
			UniqueUsers int    `json:"unique_users"`
		} `json:"days"`
		TotalUniqueUsers int `json:"total_unique_users"`
		CurrentPresent   int `json:"current_present"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if resp.TotalUniqueUsers < 1 {
		t.Fatalf("expected >=1 unique user from demo events, got %+v", resp)
	}

	// missing window -> 400
	bad := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/analytics/occupancy?tenant_id=tenant_demo_jakarta", token, nil)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without window, got %d", bad.Code)
	}
	// no bearer -> 401
	noauth := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/analytics/occupancy?tenant_id=tenant_demo_jakarta&start="+start+"&end="+end, "", nil)
	if noauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without bearer, got %d", noauth.Code)
	}
}

func TestRetentionAnalyticsEndpoint(t *testing.T) {
	router, _, err := NewRouter(config.Config{JWTSecret: "retention-test", EnableDemoUsers: true}, nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	start, end := spaceAnalyticsWindow()

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/analytics/retention?tenant_id=tenant_demo_jakarta&start="+start+"&end="+end+"&bucket=day", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Bucket  string `json:"bucket"`
		Buckets []struct {
			ActiveUsers int `json:"active_users"`
		} `json:"buckets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if resp.Bucket != "day" || len(resp.Buckets) < 1 || resp.Buckets[0].ActiveUsers < 1 {
		t.Fatalf("expected >=1 daily bucket with active users, got %+v", resp)
	}

	bad := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/analytics/retention?tenant_id=tenant_demo_jakarta&start="+start+"&end="+end+"&bucket=month", token, nil)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid bucket, got %d", bad.Code)
	}
}
