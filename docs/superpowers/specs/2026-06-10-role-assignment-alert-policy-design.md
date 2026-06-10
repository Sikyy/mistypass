# Role-Assignment Alert Policy — Design

> Date: 2026-06-10
> Status: approved (route A)
> Source: docs/kisi-gap-analysis.md §2.2 — adds the Role Assignment incident policy (Kisi 2025-11) and, as a framework fix, makes the existing built-in incident policies actually toggleable/firing.

## 1. Goal

When a sensitive permission change happens — a role assignment is created or
updated — emit an alert notification, if the tenant has enabled the Role
Assignment incident policy. Default disabled (opt-in, avoids alert noise).

This requires fixing a latent framework gap: the existing built-in incident
policies (`Door Held Open`, `Hardware Outage`) are display-only catalog entries —
their `ap_incident_*` IDs are not recognized by the alert-policy resolver, so they
can never be fetched, toggled, or fired. We make built-in incident policies
first-class: toggleable via the existing `/alert_policies` CRUD and persisted.

Non-goals: per-role sensitivity filtering (fire on all role assignments when
enabled), cross-event cooldown/storm dampening (each change is a distinct
security event; the notification ring already bounds volume).

## 2. Architecture

### 2.1 Override store
- `server.incidentAlertPolicyMu sync.RWMutex`
- `server.incidentAlertPolicyOverrides map[string]referenceAlertPolicy` (key = policy ID)
- Persisted under new state key `module_incident_alert_policy` (mirror the custom
  policy persist/restore). Restored on startup next to `restoreAlertPoliciesFromState`.

### 2.2 Catalog with merge
`referenceIncidentAlertPolicies(tenantID)` returns three defaults
(`door_held_open`, `hardware_outage`, `role_assignment`), each replaced by its
stored override when present. Defaults are disabled. New entry:
- ID `ap_incident_role_assignment_<tenant>`, Name "Role Assignment",
  Category "incident", Trigger "role_assignment", Severity "high",
  Condition `event.type == 'role_assignment.created' || event.type == 'role_assignment.updated'`,
  Enabled false, Threshold 1, Window 0, Cooldown 0.

### 2.3 Resolution / CRUD wiring
- `referenceAlertPolicyIDParts`: recognize `ap_incident_<trigger>_<tenant>` for the
  known triggers → return kind `incident_<trigger>`, tenant.
- `referenceAlertPolicyByKind`: `incident_<trigger>` → return the merged catalog policy.
- `updateReferenceAlertPolicy` switch: `incident_<trigger>` →
  `updateReferenceIncidentAlertPolicy(trigger, tenant, payload)` — start from the
  catalog default, apply payload (enabled/channels/threshold/cooldown/receiver groups),
  store override, persist, return effective policy. DELETE reuses the existing
  "update with enabled=false" path.

### 2.4 Firing
`maybeDispatchRoleAssignmentAlert(tenantID, assignment, eventType)`:
- Load the role_assignment incident policy (merged). If not enabled → return.
- Append an `alertNotification`: PolicyID/PolicyName from policy, Severity from
  policy, Trigger "role_assignment", EventType (`role_assignment.created` /
  `.updated`), EventID = assignment.ID, Channels from policy, Status "dispatched",
  Attempts 1, DispatchedAt now.
Called after the audit log in `createReferenceRoleAssignment` and
`updateReferenceRoleAssignment`.

## 3. Testing (TDD)
- List: `/alert_policies` includes the role_assignment incident policy (disabled),
  alongside door_held_open / hardware_outage.
- Toggle: PATCH `ap_incident_role_assignment_<tenant>` enabled=true → GET reflects
  enabled=true, status active; a fresh router with the same stateStore restores it.
- Fire-on-enable: enable policy, create a role assignment → a notification with
  trigger role_assignment + event_id appears in `/alert_policies/notifications`.
- No-fire-when-disabled: default (disabled) → creating a role assignment produces
  no role_assignment notification.
- Update fires: enabled + update a role assignment → notification with
  event_type role_assignment.updated.
- Framework retrofit smoke: PATCH `ap_incident_door_held_open_<tenant>` enabled=true
  → GET reflects it (proves built-ins are now toggleable, not just role_assignment).

## 4. Out of scope / future
Per-role sensitivity filters, condition-expression evaluation for incident
policies (they fire from their explicit hook, not the generic condition engine),
wiring door_held_open / hardware_outage to live device events.
