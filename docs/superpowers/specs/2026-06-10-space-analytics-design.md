# Space Analytics (Occupancy + Retention) — Design

> Date: 2026-06-10
> Status: approved
> Source: docs/kisi-gap-analysis.md §2.4 — Kisi 2025-26 shipped Daily Occupancy,
> Visual Analytics, and User Retention reports. This closes the backend data gap
> with two new analytics endpoints; the "visual" rendering is frontend work
> layered on this data (out of scope here, like the existing topology viz).

## 1. Goal

Two new tenant-scoped analytics endpoints over existing access-event data,
mirroring the established `/analytics/*` handler pattern (tenant + building scope
+ start/end window):

- `GET /api/v1/analytics/occupancy` — daily occupancy: unique people present per
  day, peak day, plus live current-present count.
- `GET /api/v1/analytics/retention` — user retention/engagement per bucket
  (day or week): active / new / returning users and a retention rate.

Roles mirror the other analytics endpoints: `super_admin, tenant_admin, operator,
building_admin`. Building scope and `building_id` filter behave as in
`getAccessSummary`.

## 2. Definitions

A "present/active" signal is an access event whose `Result` is `success` or
`accepted` (case-insensitive — same rule as `getAccessSummary`) and whose `Actor`
is non-empty. Time window is `[start, end)`. Days/buckets are UTC.

- **Occupancy (per UTC day):** `unique_users` = distinct actors with ≥1 active
  event that day; `total_entries` = count of active events that day. Response
  also returns `peak_date`/`peak_unique_users`, `total_unique_users` over the
  window, and `current_present` (live, from presences with empty `exited_at`).
- **Retention (per bucket; bucket = `day` | `week`, default `week`):**
  - `active_users` = distinct actors active in the bucket.
  - `new_users` = actors whose first active bucket in the window is this bucket.
  - `returning_users` = actors active this bucket who were also active in some
    earlier bucket in the window.
  - `retention_rate` = `returning_users / prev_bucket_active_users` (0 when the
    previous bucket had no active users; first bucket rate is 0).

  Weeks are ISO-ish: bucket key = the UTC date of the Monday on/before the event
  (so a week bucket start is deterministic).

## 3. Architecture

- `internal/http/analytics_space.go` — pure aggregation over `[]event.AccessEvent`:
  - `computeOccupancy(events, start, end, buildingID) (days []occupancyDay, peakDate string, peakUnique int, totalUnique int)`
  - `computeRetention(events, start, end, buildingID, bucket) []retentionBucket`
  - plus small helpers (`isActiveAccessEvent`, `weekBucketKey`).
- Handlers `getOccupancyAnalytics`, `getUserRetentionAnalytics` in
  `routes_analytics.go` (mirror `getAccessSummary`: resolve tenant, building
  scope, start/end; `ListAccessEvents` then `filterAccessEventsByScope`; call the
  pure func; occupancy also reads `accessSvc.ListPresences` for `current_present`).
- Routes registered next to the other `/analytics/*` GETs.

No new service methods; reuses `eventSvc.ListAccessEvents` and
`accessSvc.ListPresences`.

## 4. Testing (TDD)

Pure (unit, synthetic events — deterministic):
- `computeOccupancy`: two actors on day A, one on day B → day A unique 2 / day B
  unique 1; peak = day A; total_unique = 2 (overlapping actor counted once);
  denied/empty-actor events excluded; building_id filter respected; window edges
  `[start,end)` respected.
- `computeRetention` (week): actor active in week 1 only = new in w1, not
  returning; actor active w1 and w2 = returning in w2; rate = returning/prev
  active; bucket=day path bucketed by day.

Integration (smoke, demo data):
- `GET /analytics/occupancy?start&end` (jakarta, window around now) → 200, a day
  with `unique_users >= 1`, `current_present` present (>=0).
- `GET /analytics/retention?start&end&bucket=day` → 200, ≥1 bucket with
  `active_users >= 1`.
- both require start/end (400 without), and are role-gated (401 without bearer).

## 5. Out of scope / future
PDF/CSV export of these reports (wire into `/analytics/export` later); frontend
space-usage visualization; per-area heatmaps; cohort-matrix retention.
