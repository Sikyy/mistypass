package httpx

import (
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/access"
)

func TestGeofenceDistanceMeters(t *testing.T) {
	if d := geofenceDistanceMeters(-6.2088, 106.8456, -6.2088, 106.8456); d > 1 {
		t.Fatalf("expected ~0m for identical points, got %.2f", d)
	}
	// ~1.11km per 0.01 degree of latitude near the equator.
	d := geofenceDistanceMeters(-6.2088, 106.8456, -6.2188, 106.8456)
	if d < 1000 || d > 1200 {
		t.Fatalf("expected ~1.1km for 0.01deg lat, got %.2f", d)
	}
	far := geofenceDistanceMeters(-6.2088, 106.8456, 1.3521, 103.8198) // Jakarta -> Singapore
	if far < 800_000 {
		t.Fatalf("expected >800km Jakarta->Singapore, got %.2f", far)
	}
}

func geofencedGroup(lat, lon, radius float64) access.UserGroup {
	return access.UserGroup{
		GeofenceRestrictionEnabled:   true,
		GeofenceRestrictionRadius:    radius,
		GeofenceRestrictionLatitude:  lat,
		GeofenceRestrictionLongitude: lon,
	}
}

func TestEvaluateGeofenceAccess(t *testing.T) {
	center := geofencedGroup(-6.2088, 106.8456, 150)

	// role assignment path bypasses geofence
	if ok, reason := evaluateGeofenceAccess([]access.UserGroup{center}, true, false, 0, 0); !ok || reason != "" {
		t.Fatalf("role path should bypass geofence, got ok=%v reason=%s", ok, reason)
	}
	// non-geofenced granting group satisfies without location
	if ok, _ := evaluateGeofenceAccess([]access.UserGroup{{GeofenceRestrictionEnabled: false}}, false, false, 0, 0); !ok {
		t.Fatalf("non-geofenced group should satisfy")
	}
	// geofenced-only, location in range -> allow
	if ok, reason := evaluateGeofenceAccess([]access.UserGroup{center}, false, true, -6.2088, 106.8456); !ok || reason != "" {
		t.Fatalf("in-range should allow, got ok=%v reason=%s", ok, reason)
	}
	// geofenced-only, location out of range -> geofence_denied
	if ok, reason := evaluateGeofenceAccess([]access.UserGroup{center}, false, true, -6.30, 106.95); ok || reason != "geofence_denied" {
		t.Fatalf("out-of-range should deny geofence_denied, got ok=%v reason=%s", ok, reason)
	}
	// geofenced-only, no location -> location_required
	if ok, reason := evaluateGeofenceAccess([]access.UserGroup{center}, false, false, 0, 0); ok || reason != "location_required" {
		t.Fatalf("missing location should deny location_required, got ok=%v reason=%s", ok, reason)
	}
	// mixed: one geofenced (out of range) + one non-geofenced -> allow (OR of paths)
	mixed := []access.UserGroup{center, {GeofenceRestrictionEnabled: false}}
	if ok, _ := evaluateGeofenceAccess(mixed, false, false, 0, 0); !ok {
		t.Fatalf("a non-geofenced granting group should satisfy even alongside a geofenced one")
	}
}
