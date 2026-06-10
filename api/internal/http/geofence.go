package httpx

import (
	"math"

	"github.com/mistypass/cloud/api/internal/modules/access"
)

const defaultGeofenceRadiusMeters = 150.0

// geofenceDistanceMeters returns the great-circle distance between two
// coordinates in meters (haversine).
func geofenceDistanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusM = 6371000.0
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := rad(lat2 - lat1)
	dLon := rad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// evaluateGeofenceAccess decides whether geofence restrictions permit the unlock.
// grantingGroups are the groups that both bind the user and contain the lock;
// roleGranted is true when a role-assignment path also grants access (role paths
// carry no geofence). Returns (allowed, reason); reason is "" when allowed,
// otherwise "location_required" or "geofence_denied".
//
// OR-of-paths: access is allowed if any granting path is satisfied. A role path
// or a non-geofenced granting group always satisfies; a geofenced group is
// satisfied only when the request location is within its radius. Location is
// required only when every granting path is a geofenced group.
func evaluateGeofenceAccess(grantingGroups []access.UserGroup, roleGranted bool, hasLocation bool, lat, lon float64) (bool, string) {
	if roleGranted {
		return true, ""
	}

	geofenced := make([]access.UserGroup, 0, len(grantingGroups))
	for _, g := range grantingGroups {
		if g.GeofenceRestrictionEnabled {
			geofenced = append(geofenced, g)
			continue
		}
		// A granting group without geofence is an unrestricted path in.
		return true, ""
	}
	if len(geofenced) == 0 {
		// No granting groups recorded (e.g. access came from elsewhere) — do not
		// add a geofence denial on top of the prior access decision.
		return true, ""
	}
	if !hasLocation {
		return false, "location_required"
	}
	for _, g := range geofenced {
		radius := g.GeofenceRestrictionRadius
		if radius <= 0 {
			radius = defaultGeofenceRadiusMeters
		}
		if geofenceDistanceMeters(lat, lon, g.GeofenceRestrictionLatitude, g.GeofenceRestrictionLongitude) <= radius {
			return true, ""
		}
	}
	return false, "geofence_denied"
}

// grantingUserGroupsForLock returns the UserGroups (with restrictions) whose IDs
// both bind the user and whose door group contains the lock.
func (s *server) grantingUserGroupsForLock(tenantID string, userGroupIDs []string, lockID string) []access.UserGroup {
	if len(userGroupIDs) == 0 {
		return nil
	}
	userSet := make(map[string]struct{}, len(userGroupIDs))
	for _, id := range userGroupIDs {
		userSet[id] = struct{}{}
	}
	var out []access.UserGroup
	for _, dg := range s.spaceSvc.ListDoorGroups(tenantID) {
		if _, bound := userSet[dg.ID]; !bound {
			continue
		}
		contains := false
		for _, doorID := range dg.DoorIDs {
			if doorID == lockID {
				contains = true
				break
			}
		}
		if !contains {
			continue
		}
		if ug, err := s.accessSvc.GetUserGroup(tenantID, dg.ID); err == nil {
			out = append(out, ug)
		}
	}
	return out
}
