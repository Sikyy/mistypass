-- Event table partitioning migration (pg_partman + monthly range partitions)
--
-- Usage:
--   psql "$DATABASE_URL" -f deploy/postgres/event-partitioning-partman.sql
--
-- Notes:
-- 1) Requires pg_partman to be available in the PostgreSQL instance.
-- 2) Existing non-partitioned mistypass_*_events tables are migrated in-place.
-- 3) Data retention is configured to keep the latest 6 months of partitions.

create schema if not exists partman;
create extension if not exists pg_partman with schema partman;

-- Convert/create mistypass_access_events as partitioned table.
do $$
declare
  relkind "char";
begin
  select c.relkind
    into relkind
  from pg_class c
  join pg_namespace n on n.oid = c.relnamespace
  where n.nspname = 'public'
    and c.relname = 'mistypass_access_events';

  if relkind is null then
    create table mistypass_access_events (
      id text not null,
      tenant_id text not null,
      building_id text,
      area_id text,
      event_type text,
      actor text,
      door_id text,
      gateway_id text,
      result text,
      at timestamptz not null,
      raw jsonb not null,
      synced_at timestamptz not null default now(),
      primary key (id, at)
    ) partition by range (at);
  elsif relkind = 'r' then
    alter table mistypass_access_events rename to mistypass_access_events_legacy;

    create table mistypass_access_events (
      id text not null,
      tenant_id text not null,
      building_id text,
      area_id text,
      event_type text,
      actor text,
      door_id text,
      gateway_id text,
      result text,
      at timestamptz not null,
      raw jsonb not null,
      synced_at timestamptz not null default now(),
      primary key (id, at)
    ) partition by range (at);

    insert into mistypass_access_events (id, tenant_id, building_id, area_id, event_type, actor, door_id, gateway_id, result, at, raw, synced_at)
    select id, tenant_id, building_id, area_id, event_type, actor, door_id, gateway_id, result, at, raw, synced_at
    from mistypass_access_events_legacy
    on conflict (id, at) do update
    set tenant_id = excluded.tenant_id,
        building_id = excluded.building_id,
        area_id = excluded.area_id,
        event_type = excluded.event_type,
        actor = excluded.actor,
        door_id = excluded.door_id,
        gateway_id = excluded.gateway_id,
        result = excluded.result,
        raw = excluded.raw,
        synced_at = excluded.synced_at;

    drop table mistypass_access_events_legacy;
  elsif relkind <> 'p' then
    raise exception 'unsupported relkind % for mistypass_access_events', relkind;
  end if;

  execute 'create table if not exists mistypass_access_events_default partition of mistypass_access_events default';
end $$;

create unique index if not exists mistypass_access_events_id_at_uidx on mistypass_access_events(id, at);
create index if not exists mistypass_access_events_id_idx on mistypass_access_events(id);
create index if not exists mistypass_access_events_tenant_idx on mistypass_access_events(tenant_id);
create index if not exists mistypass_access_events_at_idx on mistypass_access_events(at desc);

-- Convert/create mistypass_device_events as partitioned table.
do $$
declare
  relkind "char";
begin
  select c.relkind
    into relkind
  from pg_class c
  join pg_namespace n on n.oid = c.relnamespace
  where n.nspname = 'public'
    and c.relname = 'mistypass_device_events';

  if relkind is null then
    create table mistypass_device_events (
      id text not null,
      tenant_id text not null,
      building_id text,
      event_type text,
      gateway_id text,
      detail text,
      result text,
      at timestamptz not null,
      raw jsonb not null,
      synced_at timestamptz not null default now(),
      primary key (id, at)
    ) partition by range (at);
  elsif relkind = 'r' then
    alter table mistypass_device_events rename to mistypass_device_events_legacy;

    create table mistypass_device_events (
      id text not null,
      tenant_id text not null,
      building_id text,
      event_type text,
      gateway_id text,
      detail text,
      result text,
      at timestamptz not null,
      raw jsonb not null,
      synced_at timestamptz not null default now(),
      primary key (id, at)
    ) partition by range (at);

    insert into mistypass_device_events (id, tenant_id, building_id, event_type, gateway_id, detail, result, at, raw, synced_at)
    select id, tenant_id, building_id, event_type, gateway_id, detail, result, at, raw, synced_at
    from mistypass_device_events_legacy
    on conflict (id, at) do update
    set tenant_id = excluded.tenant_id,
        building_id = excluded.building_id,
        event_type = excluded.event_type,
        gateway_id = excluded.gateway_id,
        detail = excluded.detail,
        result = excluded.result,
        raw = excluded.raw,
        synced_at = excluded.synced_at;

    drop table mistypass_device_events_legacy;
  elsif relkind <> 'p' then
    raise exception 'unsupported relkind % for mistypass_device_events', relkind;
  end if;

  execute 'create table if not exists mistypass_device_events_default partition of mistypass_device_events default';
end $$;

create unique index if not exists mistypass_device_events_id_at_uidx on mistypass_device_events(id, at);
create index if not exists mistypass_device_events_id_idx on mistypass_device_events(id);
create index if not exists mistypass_device_events_tenant_idx on mistypass_device_events(tenant_id);
create index if not exists mistypass_device_events_at_idx on mistypass_device_events(at desc);

-- Register partition maintenance with pg_partman.
do $$
begin
  if not exists (
    select 1
    from partman.part_config
    where parent_table = 'public.mistypass_access_events'
  ) then
    perform partman.create_parent(
      p_parent_table := 'public.mistypass_access_events',
      p_control := 'at',
      p_type := 'native',
      p_interval := 'monthly',
      p_premake := 6
    );
  end if;

  if not exists (
    select 1
    from partman.part_config
    where parent_table = 'public.mistypass_device_events'
  ) then
    perform partman.create_parent(
      p_parent_table := 'public.mistypass_device_events',
      p_control := 'at',
      p_type := 'native',
      p_interval := 'monthly',
      p_premake := 6
    );
  end if;
end $$;

update partman.part_config
set retention = '6 months',
    retention_keep_table = false
where parent_table in (
  'public.mistypass_access_events',
  'public.mistypass_device_events'
);

call partman.run_maintenance_proc();
