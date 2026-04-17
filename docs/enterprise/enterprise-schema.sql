-- Enterprise onboarding and identity integration schema (draft)
-- Target: PostgreSQL 15+

create table if not exists enterprise_domain_mapping (
  id text primary key,
  tenant_id text not null,
  domain text not null,
  status text not null default 'active',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint enterprise_domain_mapping_status_ck check (status in ('active', 'inactive'))
);

create unique index if not exists enterprise_domain_mapping_domain_uq
  on enterprise_domain_mapping (lower(domain));

create index if not exists enterprise_domain_mapping_tenant_idx
  on enterprise_domain_mapping (tenant_id, status);

create table if not exists enterprise_idp_config (
  id text primary key,
  tenant_id text not null,
  provider text not null,
  issuer_url text not null,
  client_id text not null,
  client_secret_ref text,
  auth_url text,
  token_url text,
  jwks_url text,
  user_info_url text,
  scopes text[] not null default '{}',
  status text not null default 'active',
  sync_mode text not null default 'jit',
  updated_by text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint enterprise_idp_provider_ck check (provider in ('oidc', 'saml')),
  constraint enterprise_idp_status_ck check (status in ('active', 'inactive')),
  constraint enterprise_idp_sync_mode_ck check (sync_mode in ('jit', 'manual', 'scheduled'))
);

create unique index if not exists enterprise_idp_config_tenant_uq
  on enterprise_idp_config (tenant_id);

create table if not exists enterprise_employee (
  id text primary key,
  tenant_id text not null,
  external_id text,
  email text not null,
  full_name text not null,
  department text,
  job_title text,
  location text,
  access_role text not null,
  building_id text,
  group_ids text[] not null default '{}',
  status text not null default 'active',
  source text not null,
  last_synced_at timestamptz not null default now(),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint enterprise_employee_status_ck check (status in ('active', 'inactive'))
);

create unique index if not exists enterprise_employee_tenant_email_uq
  on enterprise_employee (tenant_id, lower(email));

create index if not exists enterprise_employee_tenant_external_idx
  on enterprise_employee (tenant_id, external_id);

create index if not exists enterprise_employee_tenant_status_idx
  on enterprise_employee (tenant_id, status);

create table if not exists enterprise_identity_link (
  id text primary key,
  tenant_id text not null,
  provider text not null,
  external_subject text not null,
  employee_id text,
  user_id text,
  email text,
  status text not null default 'active',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint enterprise_identity_link_provider_ck check (provider in ('oidc', 'saml')),
  constraint enterprise_identity_link_status_ck check (status in ('active', 'inactive'))
);

create unique index if not exists enterprise_identity_link_subject_uq
  on enterprise_identity_link (tenant_id, provider, external_subject);

create table if not exists enterprise_sync_job (
  id text primary key,
  tenant_id text not null,
  source text not null,
  status text not null,
  total int not null default 0,
  created_count int not null default 0,
  updated_count int not null default 0,
  deactivated_count int not null default 0,
  rejected_count int not null default 0,
  actor text not null,
  started_at timestamptz not null,
  ended_at timestamptz not null,
  created_at timestamptz not null default now(),
  constraint enterprise_sync_job_status_ck check (status in ('queued', 'running', 'completed', 'failed'))
);

create index if not exists enterprise_sync_job_tenant_started_idx
  on enterprise_sync_job (tenant_id, started_at desc);
