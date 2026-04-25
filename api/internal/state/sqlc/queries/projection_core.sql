-- name: UpsertProjectionTenant :exec
insert into mistypass_tenants (id, name, tenant_type, hq_region, status, created_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, now())
on conflict (id) do update
set name = excluded.name,
    tenant_type = excluded.tenant_type,
    hq_region = excluded.hq_region,
    status = excluded.status,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionAccessEvent :exec
insert into mistypass_access_events (id, tenant_id, building_id, area_id, event_type, actor, door_id, gateway_id, result, at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())
on conflict (id, at) do update
set tenant_id = excluded.tenant_id,
    building_id = excluded.building_id,
    area_id = excluded.area_id,
    event_type = excluded.event_type,
    actor = excluded.actor,
    door_id = excluded.door_id,
    gateway_id = excluded.gateway_id,
    result = excluded.result,
    at = excluded.at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionDeviceEvent :exec
insert into mistypass_device_events (id, tenant_id, building_id, event_type, gateway_id, detail, result, at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
on conflict (id, at) do update
set tenant_id = excluded.tenant_id,
    building_id = excluded.building_id,
    event_type = excluded.event_type,
    gateway_id = excluded.gateway_id,
    detail = excluded.detail,
    result = excluded.result,
    at = excluded.at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionAlarm :exec
insert into mistypass_alarms (id, tenant_id, building_id, area_id, door_id, alarm_type, severity, location, status, created_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    building_id = excluded.building_id,
    area_id = excluded.area_id,
    door_id = excluded.door_id,
    alarm_type = excluded.alarm_type,
    severity = excluded.severity,
    location = excluded.location,
    status = excluded.status,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionAuditLog :exec
insert into mistypass_audit_logs (id, tenant_id, actor, role, action, target, source, at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    actor = excluded.actor,
    role = excluded.role,
    action = excluded.action,
    target = excluded.target,
    source = excluded.source,
    at = excluded.at,
    raw = excluded.raw,
    synced_at = now();
