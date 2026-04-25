-- name: UpsertProjectionBuilding :exec
insert into mistypass_buildings (id, tenant_id, name, address, region, created_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    name = excluded.name,
    address = excluded.address,
    region = excluded.region,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionFloor :exec
insert into mistypass_floors (id, tenant_id, building_id, name, created_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    building_id = excluded.building_id,
    name = excluded.name,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionArea :exec
insert into mistypass_areas (id, tenant_id, building_id, floor_id, name, created_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    building_id = excluded.building_id,
    floor_id = excluded.floor_id,
    name = excluded.name,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionDoor :exec
insert into mistypass_doors (id, tenant_id, building_id, floor_id, area_id, name, gateway_id, kind, status, created_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    building_id = excluded.building_id,
    floor_id = excluded.floor_id,
    area_id = excluded.area_id,
    name = excluded.name,
    gateway_id = excluded.gateway_id,
    kind = excluded.kind,
    status = excluded.status,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionDoorGroup :exec
insert into mistypass_door_groups (id, tenant_id, name, door_ids, created_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    name = excluded.name,
    door_ids = excluded.door_ids,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now();
