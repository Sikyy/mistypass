-- name: UpsertProjectionAccessUser :exec
insert into mistypass_access_users (id, tenant_id, building_id, name, email, role, status, group_ids, created_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    building_id = excluded.building_id,
    name = excluded.name,
    email = excluded.email,
    role = excluded.role,
    status = excluded.status,
    group_ids = excluded.group_ids,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionAccessUserGroup :exec
insert into mistypass_access_user_groups (id, tenant_id, building_id, name, description, members, created_at, updated_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    building_id = excluded.building_id,
    name = excluded.name,
    description = excluded.description,
    members = excluded.members,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionAccessPolicy :exec
insert into mistypass_access_policies (id, tenant_id, name, scope_type, building_id, area_id, door_id, schedule, members, status, updated_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    name = excluded.name,
    scope_type = excluded.scope_type,
    building_id = excluded.building_id,
    area_id = excluded.area_id,
    door_id = excluded.door_id,
    schedule = excluded.schedule,
    members = excluded.members,
    status = excluded.status,
    updated_at = excluded.updated_at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionTemporaryAccess :exec
insert into mistypass_temporary_access (id, tenant_id, scope_type, building_id, area_id, door_id, delivery_method, grantee_name, grantee_email, grantee_phone, valid_until, authorized_by_email, authorized_by_role, authorized_at, created_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    scope_type = excluded.scope_type,
    building_id = excluded.building_id,
    area_id = excluded.area_id,
    door_id = excluded.door_id,
    delivery_method = excluded.delivery_method,
    grantee_name = excluded.grantee_name,
    grantee_email = excluded.grantee_email,
    grantee_phone = excluded.grantee_phone,
    valid_until = excluded.valid_until,
    authorized_by_email = excluded.authorized_by_email,
    authorized_by_role = excluded.authorized_by_role,
    authorized_at = excluded.authorized_at,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionVisitorPass :exec
insert into mistypass_visitor_passes (id, tenant_id, building_id, host, visitor, delivery_method, expires_at, created_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    building_id = excluded.building_id,
    host = excluded.host,
    visitor = excluded.visitor,
    delivery_method = excluded.delivery_method,
    expires_at = excluded.expires_at,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now();
