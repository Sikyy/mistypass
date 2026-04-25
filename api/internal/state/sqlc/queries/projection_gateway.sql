-- name: UpsertProjectionGateway :exec
insert into mistypass_gateways (id, tenant_id, serial_number, building_id, device_capacity, status, last_seen_at, devices, bound_door_ids, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    serial_number = excluded.serial_number,
    building_id = excluded.building_id,
    device_capacity = excluded.device_capacity,
    status = excluded.status,
    last_seen_at = excluded.last_seen_at,
    devices = excluded.devices,
    bound_door_ids = excluded.bound_door_ids,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionGatewayDevice :exec
insert into mistypass_gateway_devices (id, gateway_id, tenant_id, serial_number, kind, source, protocol, rs485_config, rs485_health, status, last_seen_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
on conflict (id) do update
set gateway_id = excluded.gateway_id,
    tenant_id = excluded.tenant_id,
    serial_number = excluded.serial_number,
    kind = excluded.kind,
    source = excluded.source,
    protocol = excluded.protocol,
    rs485_config = excluded.rs485_config,
    rs485_health = excluded.rs485_health,
    status = excluded.status,
    last_seen_at = excluded.last_seen_at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionGatewaySerialInventory :exec
insert into mistypass_gateway_serial_inventory (
  id, tenant_id, serial_number, product_type, status, batch_code, source, consumed_gateway_id, consumed_at, created_at, updated_at, raw, synced_at
)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    serial_number = excluded.serial_number,
    product_type = excluded.product_type,
    status = excluded.status,
    batch_code = excluded.batch_code,
    source = excluded.source,
    consumed_gateway_id = excluded.consumed_gateway_id,
    consumed_at = excluded.consumed_at,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at,
    raw = excluded.raw,
    synced_at = now();
