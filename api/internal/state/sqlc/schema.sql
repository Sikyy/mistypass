create table if not exists mistypass (
  state_key text primary key,
  payload jsonb not null,
  updated_at timestamptz not null default now()
);

create table if not exists mistypass_change_log (
  id bigserial primary key,
  state_key text not null,
  change_type text not null,
  payload_hash text not null,
  payload jsonb not null,
  created_at timestamptz not null default now()
);

create table if not exists mistypass_change_replay_checkpoints (
  state_key text primary key,
  last_change_id bigint not null,
  updated_at timestamptz not null default now()
);

create table if not exists mistypass_gateway_device_tokens (
  gateway_id text primary key,
  token_hash text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists mistypass_gateways (
  id text primary key,
  tenant_id text not null,
  serial_number text not null,
  building_id text,
  device_capacity int not null,
  status text not null,
  last_seen_at timestamptz not null,
  devices jsonb,
  bound_door_ids jsonb,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_gateway_devices (
  id text primary key,
  gateway_id text not null,
  tenant_id text not null,
  serial_number text not null,
  kind text not null,
  source text not null,
  protocol text,
  rs485_config jsonb,
  rs485_health jsonb,
  status text not null,
  last_seen_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_gateway_serial_inventory (
  id text primary key,
  tenant_id text not null,
  serial_number text not null,
  product_type text not null,
  status text not null,
  batch_code text,
  source text,
  consumed_gateway_id text,
  consumed_at timestamptz,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_tenants (
  id text primary key,
  name text not null,
  tenant_type text not null,
  hq_region text not null,
  status text not null,
  created_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_buildings (
  id text primary key,
  tenant_id text not null,
  name text not null,
  address text,
  region text,
  created_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_floors (
  id text primary key,
  tenant_id text not null,
  building_id text not null,
  name text not null,
  created_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_areas (
  id text primary key,
  tenant_id text not null,
  building_id text not null,
  floor_id text not null,
  name text not null,
  created_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_doors (
  id text primary key,
  tenant_id text not null,
  building_id text not null,
  floor_id text not null,
  area_id text not null,
  name text not null,
  gateway_id text,
  kind text not null,
  status text not null,
  created_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_door_groups (
  id text primary key,
  tenant_id text not null,
  name text not null,
  door_ids jsonb,
  created_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_auth_users (
  id text primary key,
  name text not null default '',
  email text not null,
  role text not null,
  tenant_id text not null default '',
  building_ids jsonb not null default '[]'::jsonb,
  language text not null default '',
  password_hash bytea,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create unique index if not exists mistypass_auth_users_email_idx on mistypass_auth_users(email);

create table if not exists mistypass_auth_refresh_sessions (
  session_id text primary key,
  user_id text not null,
  expires_at timestamptz not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists mistypass_auth_revoked_access_tokens (
  token_id text primary key,
  expires_at timestamptz not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists mistypass_auth_admin_mfa_states (
  user_id text primary key,
  secret text not null default '',
  pending_secret text not null default '',
  enabled boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists mistypass_auth_webauthn_credentials (
  id text primary key,
  user_id text not null,
  public_key bytea not null,
  attestation_type text not null default 'none',
  aaguid text not null default '',
  sign_count int not null default 0,
  display_name text not null default 'Passkey',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index if not exists mistypass_auth_webauthn_credentials_user_idx on mistypass_auth_webauthn_credentials(user_id);

create table if not exists mistypass_enterprise_domain_mappings (
  id text primary key,
  tenant_id text not null,
  domain text not null,
  status text not null,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_enterprise_idp_configs (
  id text primary key,
  tenant_id text not null,
  provider text not null,
  issuer_url text not null,
  client_id text not null,
  status text not null,
  sync_mode text not null,
  scopes jsonb,
  updated_by text,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_enterprise_employees (
  id text primary key,
  tenant_id text not null,
  external_id text,
  email text not null,
  full_name text not null,
  department text,
  job_title text,
  location text,
  access_role text,
  building_id text,
  group_ids jsonb,
  status text,
  source text,
  last_synced_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_enterprise_sync_jobs (
  id text primary key,
  tenant_id text not null,
  source text,
  status text,
  total int,
  created int,
  updated int,
  deactivated int,
  rejected int,
  actor text,
  started_at timestamptz not null,
  ended_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_access_users (
  id text primary key,
  tenant_id text not null,
  building_id text,
  name text not null,
  email text not null,
  role text not null,
  status text not null,
  group_ids jsonb,
  created_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_access_user_groups (
  id text primary key,
  tenant_id text not null,
  building_id text,
  name text not null,
  description text,
  members jsonb,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_access_policies (
  id text primary key,
  tenant_id text not null,
  name text not null,
  scope_type text not null,
  building_id text,
  area_id text,
  door_id text,
  schedule text,
  members int not null,
  status text not null,
  updated_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_temporary_access (
  id text primary key,
  tenant_id text not null,
  scope_type text not null,
  building_id text,
  area_id text,
  door_id text,
  delivery_method text not null,
  grantee_name text not null,
  grantee_email text not null,
  grantee_phone text not null,
  valid_until text not null,
  authorized_by_email text,
  authorized_by_role text,
  authorized_at timestamptz not null,
  created_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_visitor_passes (
  id text primary key,
  tenant_id text not null,
  building_id text,
  host text not null,
  visitor text not null,
  delivery_method text not null,
  expires_at text not null,
  created_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_access_events (
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
);
create unique index if not exists mistypass_access_events_id_at_uidx on mistypass_access_events(id, at);
create index if not exists mistypass_access_events_id_idx on mistypass_access_events(id);
create index if not exists mistypass_access_events_tenant_idx on mistypass_access_events(tenant_id);
create index if not exists mistypass_access_events_at_idx on mistypass_access_events(at desc);

create table if not exists mistypass_device_events (
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
);
create unique index if not exists mistypass_device_events_id_at_uidx on mistypass_device_events(id, at);
create index if not exists mistypass_device_events_id_idx on mistypass_device_events(id);
create index if not exists mistypass_device_events_tenant_idx on mistypass_device_events(tenant_id);
create index if not exists mistypass_device_events_at_idx on mistypass_device_events(at desc);

create table if not exists mistypass_alarms (
  id text primary key,
  tenant_id text not null,
  building_id text,
  area_id text,
  door_id text,
  alarm_type text,
  severity text,
  location text,
  status text,
  created_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_audit_logs (
  id text primary key,
  tenant_id text not null,
  actor text,
  role text,
  action text,
  target text,
  source text,
  at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_wallet_configs (
  id text primary key,
  tenant_id text not null,
  provider text not null,
  issuer_id text not null,
  service_account_email text not null,
  key_ref text not null,
  status text not null,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_wallet_templates (
  id text primary key,
  tenant_id text not null,
  provider text not null,
  pass_type text not null,
  class_id text not null,
  name text not null,
  status text not null,
  style_config jsonb,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_wallet_passes (
  id text primary key,
  tenant_id text not null,
  provider text not null,
  template_id text not null,
  target_type text not null,
  target_id text not null,
  object_id text not null,
  status text not null,
  save_link text not null,
  expires_at text,
  issued_at timestamptz not null,
  activated_at timestamptz,
  revoked_at timestamptz,
  created_by text,
  updated_by text,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_wallet_jobs (
  id text primary key,
  tenant_id text not null,
  provider text,
  batch_id text,
  template_id text,
  target_type text,
  target_id text,
  expires_at text,
  pass_id text,
  status text,
  retry_count int,
  error_code text,
  error_message text,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_wallet_audit_logs (
  id text primary key,
  tenant_id text not null,
  action text,
  actor text,
  target_id text,
  result text,
  at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);

create table if not exists mistypass_alert_policies (
  id text primary key,
  tenant_id text not null,
  name text not null,
  category text not null default 'custom',
  trigger_type text not null,
  severity text not null default 'medium',
  status text not null default 'active',
  enabled boolean not null default true,
  channels jsonb not null default '{}'::jsonb,
  raw jsonb not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  synced_at timestamptz not null default now()
);
create index if not exists mistypass_alert_policies_tenant_idx on mistypass_alert_policies(tenant_id);

create table if not exists mistypass_alert_notifications (
  id text primary key,
  tenant_id text not null,
  policy_id text not null,
  severity text not null,
  status text not null default 'pending',
  channels jsonb not null default '{}'::jsonb,
  attempts int not null default 0,
  last_error text,
  dispatched_at timestamptz,
  created_at timestamptz not null default now()
);
create index if not exists mistypass_alert_notifications_tenant_idx on mistypass_alert_notifications(tenant_id);
