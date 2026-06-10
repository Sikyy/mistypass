# Geofence Server-Side Enforcement — Design

> Date: 2026-06-10
> Status: approved (strict model)
> Source: docs/kisi-gap-analysis.md §2.2 — "GPS geofence … 服务端强制缺失". Today the
> UserGroup geofence restriction (enabled + radius) is config-only; the unlock
> path never enforces it (Android does client-side geofencing only).

## 1. Goal

When a user's access to a door is granted (only) through geofenced group(s),
the mobile unlock must include the user's coordinates and they must fall within
the granting group's geofence radius. Otherwise the unlock is denied.

Strict / secure-by-default model (chosen): location required when a geofenced
group is the access path; missing coords → deny `location_required`; outside
radius → deny `geofence_denied`.

## 2. Data model

Add a geofence center to the `UserGroup` restriction (cohesive with the existing
`GeofenceRestrictionEnabled` + `GeofenceRestrictionRadius`; no change to the
space/Building model):
- `access.UserGroup`: `GeofenceRestrictionLatitude float64`, `GeofenceRestrictionLongitude float64`.
- `access.UserGroupInput`: `GeofenceRestrictionLatitude *float64`, `GeofenceRestrictionLongitude *float64`.
- Apply in create defaults + update (service_users.go), mirroring `GeofenceRestrictionRadius`.
- Reference API group create/update payload + response: add the two fields next
  to the existing geofence enabled/radius plumbing (routes_reference_api.go).

Radius is interpreted in meters (existing default 150 → 150 m). Distance via
haversine.

## 3. Enforcement

New pure helper `geofenceDistanceMeters(lat1, lon1, lat2, lon2 float64) float64`
(haversine) in a small `geofence.go`, plus `geofenceSatisfied(group, lat, lon, hasLoc) (ok bool, reason string)`.

Access evaluation (OR-of-paths): a user may reach a lock through multiple
granting groups and/or a role assignment. Allow the unlock if **any** path is
satisfied:
- a role-assignment path → always satisfied (role assignments carry no geofence);
- a granting group with geofence disabled → satisfied;
- a granting group with geofence enabled → satisfied only if the request carries
  coords within `radius` of the group's center.

If at least one granting group exists but **every** granting path is a geofenced
group and none is satisfied:
- no coords in request → deny `location_required`;
- coords present but all out of range → deny `geofence_denied`.

To evaluate this, the access check must surface the granting groups (not just a
name). Add `grantingUserGroupsForLock(tenantID, userGroupIDs, lockID) []access.UserGroup`
that returns the UserGroups (with restrictions) whose IDs both bind the user and
contain the lock (reuses the DoorGroup match in `checkUserLockAccess`, then loads
each matched group's UserGroup restriction via `accessSvc.GetUserGroup`). The
existing `checkUserLockAccess` is unchanged (still used elsewhere / by the gateway
verify path); geofence is layered in the app unlock handlers.

## 4. Request contract

`appUnlockDoor` (POST /app/access/unlock) and `appPlaceUnlockDoor`
(POST /app/places/{placeId}/doors/{doorId}/unlock) request bodies gain optional
`latitude *float64`, `longitude *float64`. QR-unlock is out of scope (token/guest
path, no group).

Enforcement runs only after access is granted, and only when role-assignment
access is absent and the granting groups include a geofenced one.

## 5. Testing (TDD)

- `geofenceDistanceMeters`: ~0 for identical points; known city-block distance
  within tolerance; large for far points.
- Unlock allowed when in radius (coords within center+radius of granting group).
- Unlock denied `geofence_denied` when coords outside radius.
- Unlock denied `location_required` when geofenced-only access and no coords.
- Unlock allowed when geofence disabled on the granting group (no coords needed).
- Unlock allowed via role assignment even if a geofenced group also matches and
  no coords (OR-of-paths: role path satisfies).
- Reference group create/update round-trips the new lat/lng fields.

Tests configure a demo group's geofence via PATCH `/groups/{id}` (or seed) then
exercise `/app/access/unlock` with in/out/missing coords using the demo gateway
(no live gateway needed — handler returns decision before/independent of dispatch
when denied; allow path returns 200 with decision allow as today).

## 6. Out of scope / future
QR-unlock geofencing; primary-device / managed-device enforcement (separate gap
items); per-door (vs per-group) geofence centers; accuracy/altitude handling.
