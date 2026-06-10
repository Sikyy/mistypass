package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

// The demo group "Common Office Access" (ug_common_office_jkt) has geofence
// enabled at (-6.2088, 106.8456) radius 150m; resident.jakarta is a member whose
// access to door_jkt_001 is only via that group (no role assignment).

func geofenceUnlock(t *testing.T, router http.Handler, token, body string) (int, string) {
	t.Helper()
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/access/unlock", token, []byte(body))
	var resp struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return rec.Code, resp.Reason
}

func newGeofenceRouter(t *testing.T) (http.Handler, string) {
	t.Helper()
	router, _, err := NewRouter(config.Config{JWTSecret: "geofence-unlock-test", EnableDemoUsers: true}, nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	return router, referenceAPILogin(t, router, "resident.jakarta@mistypass.local")
}

func TestGeofenceUnlockInRangeAllows(t *testing.T) {
	router, token := newGeofenceRouter(t)
	code, reason := geofenceUnlock(t, router, token, `{"lock_id":"door_jkt_001","latitude":-6.2088,"longitude":106.8456}`)
	if code != http.StatusOK {
		t.Fatalf("expected 200 in range, got %d reason=%s", code, reason)
	}
}

func TestGeofenceUnlockOutOfRangeDenies(t *testing.T) {
	router, token := newGeofenceRouter(t)
	// ~12km away
	code, reason := geofenceUnlock(t, router, token, `{"lock_id":"door_jkt_001","latitude":-6.30,"longitude":106.95}`)
	if code != http.StatusForbidden || reason != "geofence_denied" {
		t.Fatalf("expected 403 geofence_denied out of range, got %d reason=%s", code, reason)
	}
}

func TestGeofenceUnlockMissingLocationDenies(t *testing.T) {
	router, token := newGeofenceRouter(t)
	code, reason := geofenceUnlock(t, router, token, `{"lock_id":"door_jkt_001"}`)
	if code != http.StatusForbidden || reason != "location_required" {
		t.Fatalf("expected 403 location_required without coords, got %d reason=%s", code, reason)
	}
}

func TestGeofenceUnlockDisabledGroupNeedsNoLocation(t *testing.T) {
	router, _, err := NewRouter(config.Config{JWTSecret: "geofence-disabled-test", EnableDemoUsers: true}, nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	adminToken := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// Disable geofence on the granting group.
	patch := []byte(`{"group":{"tenant_id":"tenant_demo_jakarta","geofence_restriction_enabled":false}}`)
	rec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/groups/ug_common_office_jkt", adminToken, patch)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected group update 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")
	code, reason := geofenceUnlock(t, router, token, `{"lock_id":"door_jkt_001"}`)
	if code != http.StatusOK {
		t.Fatalf("expected 200 with geofence disabled and no coords, got %d reason=%s", code, reason)
	}
}

func TestReferenceGroupGeofenceCenterRoundTrip(t *testing.T) {
	router, _, err := NewRouter(config.Config{JWTSecret: "geofence-roundtrip-test", EnableDemoUsers: true}, nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	patch := []byte(`{"group":{"tenant_id":"tenant_demo_jakarta","geofence_restriction_enabled":true,"geofence_restriction_radius":200,"geofence_restriction_latitude":-6.1751,"geofence_restriction_longitude":106.8650}}`)
	rec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/groups/ug_common_office_jkt", token, patch)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected group update 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	getRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/groups/ug_common_office_jkt?tenant_id=tenant_demo_jakarta", token, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected group get 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	var group struct {
		GeofenceLatitude  float64 `json:"geofence_restriction_latitude"`
		GeofenceLongitude float64 `json:"geofence_restriction_longitude"`
		GeofenceRadius    float64 `json:"geofence_restriction_radius"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &group); err != nil {
		t.Fatalf("decode group: %v body=%s", err, getRec.Body.String())
	}
	if group.GeofenceLatitude != -6.1751 || group.GeofenceLongitude != 106.8650 || group.GeofenceRadius != 200 {
		t.Fatalf("expected geofence center round-trip, got %+v", group)
	}
}
