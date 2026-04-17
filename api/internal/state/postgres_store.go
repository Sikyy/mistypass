package state

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/alarm"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/event"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
	"github.com/mistypass/cloud/api/internal/modules/space"
	"github.com/mistypass/cloud/api/internal/modules/tenant"
	"github.com/mistypass/cloud/api/internal/modules/wallet"

	"github.com/lib/pq"
)

const (
	defaultDriverName   = "postgres"
	defaultQueryTimeout = 5 * time.Second
	defaultReplayLimit  = 100
	maxReplayLimit      = 500

	changeTypeSnapshotSaved = "snapshot_saved"

	stateKeyTenant     = "module_tenant"
	stateKeySpace      = "module_space"
	stateKeyAccess     = "module_access"
	stateKeyGateway    = "module_gateway"
	stateKeyEnterprise = "module_enterprise"
	stateKeyEvent      = "module_event"
	stateKeyAlarm      = "module_alarm"
	stateKeyAudit      = "module_audit"
	stateKeyWallet     = "module_wallet"
)

var ErrStateKeyRequired = errors.New("state_key is required")
var ErrInvalidProjectionTable = errors.New("invalid projection table")

var projectionKeys = []string{
	stateKeyTenant,
	stateKeySpace,
	stateKeyAccess,
	stateKeyGateway,
	stateKeyEnterprise,
	stateKeyEvent,
	stateKeyAlarm,
	stateKeyAudit,
	stateKeyWallet,
}

var allowedProjectionDeleteTables = map[string]struct{}{
	"mistypass_tenants":                    {},
	"mistypass_buildings":                  {},
	"mistypass_floors":                     {},
	"mistypass_areas":                      {},
	"mistypass_doors":                      {},
	"mistypass_door_groups":                {},
	"mistypass_access_users":               {},
	"mistypass_access_user_groups":         {},
	"mistypass_access_policies":            {},
	"mistypass_temporary_access":           {},
	"mistypass_visitor_passes":             {},
	"mistypass_gateways":                   {},
	"mistypass_gateway_devices":            {},
	"mistypass_gateway_serial_inventory":   {},
	"mistypass_enterprise_domain_mappings": {},
	"mistypass_enterprise_idp_configs":     {},
	"mistypass_enterprise_employees":       {},
	"mistypass_enterprise_sync_jobs":       {},
	"mistypass_access_events":              {},
	"mistypass_device_events":              {},
	"mistypass_alarms":                     {},
	"mistypass_audit_logs":                 {},
	"mistypass_wallet_configs":             {},
	"mistypass_wallet_templates":           {},
	"mistypass_wallet_passes":              {},
	"mistypass_wallet_jobs":                {},
	"mistypass_wallet_audit_logs":          {},
}

type tenantStateSnapshot struct {
	Tenants []tenant.Tenant `json:"tenants"`
}

type spaceStateSnapshot struct {
	Buildings  []space.Building  `json:"buildings"`
	Floors     []space.Floor     `json:"floors"`
	Areas      []space.Area      `json:"areas"`
	Doors      []space.Door      `json:"doors"`
	DoorGroups []space.DoorGroup `json:"door_groups"`
}

type accessStateSnapshot struct {
	Users           []access.AccessUser      `json:"users"`
	UserGroups      []access.UserGroup       `json:"user_groups"`
	Policies        []access.Policy          `json:"policies"`
	TemporaryAccess []access.TemporaryAccess `json:"temporary_access"`
	VisitorPasses   []access.VisitorPass     `json:"visitor_passes"`
}

type gatewayStateSnapshot struct {
	Gateways        []gateway.Gateway             `json:"gateways"`
	SerialInventory []gateway.SerialInventoryItem `json:"serial_inventory,omitempty"`
}

type enterpriseStateSnapshot struct {
	DomainMappings []enterprise.DomainMapping      `json:"domain_mappings"`
	IDPConfigs     map[string]enterprise.IDPConfig `json:"idp_configs"`
	Employees      []enterprise.EnterpriseEmployee `json:"employees"`
	SyncJobs       []enterprise.SyncJob            `json:"sync_jobs"`
}

type eventStateSnapshot struct {
	AccessEvents []event.AccessEvent `json:"access_events"`
	DeviceEvents []event.DeviceEvent `json:"device_events"`
}

type alarmStateSnapshot struct {
	Alarms []alarm.Alarm `json:"alarms"`
}

type auditStateSnapshot struct {
	Logs []audit.Log `json:"logs"`
}

type walletStateSnapshot struct {
	Config    *wallet.GoogleConfig  `json:"config"`
	Templates []wallet.PassTemplate `json:"templates"`
	Passes    []wallet.PassInstance `json:"passes"`
	Jobs      []wallet.IssueJob     `json:"jobs"`
	AuditLogs []wallet.AuditLog     `json:"audit_logs"`
}

type PostgresStore struct {
	db                *sql.DB
	projectionApplier func(ctx context.Context, key string, payload []byte) error
}

type StateChangeRecord struct {
	ID          int64           `json:"id"`
	StateKey    string          `json:"state_key"`
	ChangeType  string          `json:"change_type"`
	PayloadHash string          `json:"payload_hash"`
	Payload     json.RawMessage `json:"payload"`
	CreatedAt   time.Time       `json:"created_at"`
}

type ReplayStateChangesResult struct {
	Applied      int   `json:"applied"`
	LastChangeID int64 `json:"last_change_id"`
}

type ReplayCheckpoint struct {
	StateKey     string    `json:"state_key"`
	LastChangeID int64     `json:"last_change_id"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ReplayFromCheckpointResult struct {
	StateKey     string           `json:"state_key"`
	FromID       int64            `json:"from_id"`
	Applied      int              `json:"applied"`
	LastChangeID int64            `json:"last_change_id"`
	Checkpoint   ReplayCheckpoint `json:"checkpoint"`
}

func NewPostgresStore(databaseURL string) (*PostgresStore, error) {
	db, err := sql.Open(defaultDriverName, databaseURL)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	store := &PostgresStore{db: db}
	store.projectionApplier = store.applyProjection
	return store, nil
}

func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresStore) EnsureSchema() error {
	if s == nil || s.db == nil {
		return errors.New("postgres store is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `
create table if not exists mistypass (
  state_key text primary key,
  payload jsonb not null,
  updated_at timestamptz not null default now()
);

do $$
begin
  if to_regclass('public.app_state') is not null then
    insert into mistypass (state_key, payload, updated_at)
    select state_key, payload, updated_at
    from app_state
    on conflict (state_key) do nothing;
  end if;
end $$;

create index if not exists mistypass_updated_at_idx
  on mistypass(updated_at desc);

create table if not exists mistypass_change_log (
  id bigserial primary key,
  state_key text not null,
  change_type text not null,
  payload_hash text not null,
  payload jsonb not null,
  created_at timestamptz not null default now()
);
create index if not exists mistypass_change_log_state_key_created_idx
  on mistypass_change_log(state_key, created_at desc);
create index if not exists mistypass_change_log_created_idx
  on mistypass_change_log(created_at desc);

create table if not exists mistypass_change_replay_checkpoints (
  state_key text primary key,
  last_change_id bigint not null default 0,
  updated_at timestamptz not null default now()
);
create index if not exists mistypass_change_replay_checkpoints_updated_idx
  on mistypass_change_replay_checkpoints(updated_at desc);
`)
	if err != nil {
		return err
	}

	if err := s.ensureProjectionTables(ctx); err != nil {
		return err
	}
	return s.syncProjectionTables(ctx)
}

func (s *PostgresStore) ensureProjectionTables(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
create table if not exists mistypass_tenants (
  id text primary key,
  name text not null,
  tenant_type text not null,
  hq_region text,
  status text not null,
  created_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);
create index if not exists mistypass_tenants_status_idx on mistypass_tenants(status);

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
create index if not exists mistypass_buildings_tenant_idx on mistypass_buildings(tenant_id);

create table if not exists mistypass_floors (
  id text primary key,
  tenant_id text not null,
  building_id text not null,
  name text not null,
  created_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);
create index if not exists mistypass_floors_tenant_idx on mistypass_floors(tenant_id);
create index if not exists mistypass_floors_building_idx on mistypass_floors(building_id);

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
create index if not exists mistypass_areas_tenant_idx on mistypass_areas(tenant_id);
create index if not exists mistypass_areas_building_idx on mistypass_areas(building_id);
create index if not exists mistypass_areas_floor_idx on mistypass_areas(floor_id);

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
create index if not exists mistypass_doors_tenant_idx on mistypass_doors(tenant_id);
create index if not exists mistypass_doors_building_idx on mistypass_doors(building_id);
create index if not exists mistypass_doors_area_idx on mistypass_doors(area_id);

create table if not exists mistypass_door_groups (
  id text primary key,
  tenant_id text not null,
  name text not null,
  door_ids jsonb,
  created_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);
create index if not exists mistypass_door_groups_tenant_idx on mistypass_door_groups(tenant_id);

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
create index if not exists mistypass_access_users_tenant_idx on mistypass_access_users(tenant_id);
create index if not exists mistypass_access_users_email_idx on mistypass_access_users(lower(email));

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
create index if not exists mistypass_access_user_groups_tenant_idx on mistypass_access_user_groups(tenant_id);

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
create index if not exists mistypass_access_policies_tenant_idx on mistypass_access_policies(tenant_id);

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
create index if not exists mistypass_temporary_access_tenant_idx on mistypass_temporary_access(tenant_id);

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
create index if not exists mistypass_visitor_passes_tenant_idx on mistypass_visitor_passes(tenant_id);

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
create index if not exists mistypass_gateways_tenant_idx on mistypass_gateways(tenant_id);

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
create index if not exists mistypass_gateway_devices_gateway_idx on mistypass_gateway_devices(gateway_id);
create index if not exists mistypass_gateway_devices_tenant_idx on mistypass_gateway_devices(tenant_id);

alter table if exists mistypass_gateway_devices add column if not exists protocol text;
alter table if exists mistypass_gateway_devices add column if not exists rs485_config jsonb;
alter table if exists mistypass_gateway_devices add column if not exists rs485_health jsonb;

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
create unique index if not exists mistypass_gateway_serial_inventory_serial_idx on mistypass_gateway_serial_inventory(serial_number);
create index if not exists mistypass_gateway_serial_inventory_tenant_idx on mistypass_gateway_serial_inventory(tenant_id);
create index if not exists mistypass_gateway_serial_inventory_status_idx on mistypass_gateway_serial_inventory(status);

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
create index if not exists mistypass_enterprise_domain_tenant_idx on mistypass_enterprise_domain_mappings(tenant_id);

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
create index if not exists mistypass_enterprise_idp_tenant_idx on mistypass_enterprise_idp_configs(tenant_id);

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
create index if not exists mistypass_enterprise_employees_tenant_idx on mistypass_enterprise_employees(tenant_id);
create index if not exists mistypass_enterprise_employees_email_idx on mistypass_enterprise_employees(lower(email));

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
create index if not exists mistypass_enterprise_sync_jobs_tenant_idx on mistypass_enterprise_sync_jobs(tenant_id);

create table if not exists mistypass_access_events (
  id text primary key,
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
  synced_at timestamptz not null default now()
);
create index if not exists mistypass_access_events_tenant_idx on mistypass_access_events(tenant_id);

create table if not exists mistypass_device_events (
  id text primary key,
  tenant_id text not null,
  building_id text,
  event_type text,
  gateway_id text,
  detail text,
  result text,
  at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);
create index if not exists mistypass_device_events_tenant_idx on mistypass_device_events(tenant_id);

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
create index if not exists mistypass_alarms_tenant_idx on mistypass_alarms(tenant_id);

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
create index if not exists mistypass_audit_logs_tenant_idx on mistypass_audit_logs(tenant_id);

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
create index if not exists mistypass_wallet_configs_tenant_idx on mistypass_wallet_configs(tenant_id);

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
create index if not exists mistypass_wallet_templates_tenant_idx on mistypass_wallet_templates(tenant_id);

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
create index if not exists mistypass_wallet_passes_tenant_idx on mistypass_wallet_passes(tenant_id);

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
create index if not exists mistypass_wallet_jobs_tenant_idx on mistypass_wallet_jobs(tenant_id);

alter table if exists mistypass_wallet_jobs add column if not exists template_id text;
alter table if exists mistypass_wallet_jobs add column if not exists expires_at text;

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
create index if not exists mistypass_wallet_audit_logs_tenant_idx on mistypass_wallet_audit_logs(tenant_id);
`)
	return err
}

func (s *PostgresStore) syncProjectionTables(ctx context.Context) error {
	for _, key := range projectionKeys {
		payload, found, err := s.loadRawPayload(ctx, key)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		if err := s.projectStatePayload(ctx, key, payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) loadRawPayload(ctx context.Context, key string) ([]byte, bool, error) {
	var raw []byte
	err := s.db.QueryRowContext(ctx, `select payload from mistypass where state_key = $1`, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if len(raw) == 0 {
		return nil, false, nil
	}
	return raw, true, nil
}

func (s *PostgresStore) Load(key string, dst any) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("postgres store is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()

	var raw []byte
	err := s.db.QueryRowContext(
		ctx,
		`select payload from mistypass where state_key = $1`,
		key,
	).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(raw) == 0 {
		return false, nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return false, fmt.Errorf("decode state %q: %w", key, err)
	}
	return true, nil
}

func (s *PostgresStore) Save(key string, value any) error {
	if s == nil || s.db == nil {
		return errors.New("postgres store is not initialized")
	}

	nextKey := strings.TrimSpace(key)
	if nextKey == "" {
		return ErrStateKeyRequired
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode state %q: %w", nextKey, err)
	}
	payloadHash := statePayloadHash(payload)

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	currentPayload, found, err := s.loadCurrentPayloadForUpdate(ctx, tx, nextKey)
	if err != nil {
		return fmt.Errorf("load state %q: %w", nextKey, err)
	}
	if found {
		samePayload, err := jsonPayloadEqual(currentPayload, payload)
		if err != nil {
			return fmt.Errorf("compare state payload %q: %w", nextKey, err)
		}
		if samePayload {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				return err
			}
			tx = nil
			return nil
		}
	}

	_, err = tx.ExecContext(
		ctx,
		`insert into mistypass (state_key, payload, updated_at)
values ($1, $2::jsonb, now())
on conflict (state_key) do update
set payload = excluded.payload,
    updated_at = now()`,
		nextKey,
		string(payload),
	)
	if err != nil {
		return fmt.Errorf("save state %q: %w", nextKey, err)
	}

	changeID, err := s.appendStateChangeTx(ctx, tx, nextKey, payloadHash, payload)
	if err != nil {
		return fmt.Errorf("append state change %q: %w", nextKey, err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	tx = nil

	checkpointLastID := int64(0)
	for batch := 0; batch < 4; batch++ {
		replayResult, replayErr := s.ReplayStateChangesFromCheckpoint(nextKey, maxReplayLimit)
		if replayErr != nil {
			return fmt.Errorf("project state %q change_id=%d via replay: %w", nextKey, changeID, replayErr)
		}
		checkpointLastID = replayResult.LastChangeID
		if checkpointLastID >= changeID || replayResult.Applied == 0 {
			break
		}
	}
	return nil
}

func (s *PostgresStore) loadCurrentPayloadForUpdate(ctx context.Context, tx *sql.Tx, key string) ([]byte, bool, error) {
	var raw []byte
	err := tx.QueryRowContext(
		ctx,
		`select payload from mistypass where state_key = $1 for update`,
		key,
	).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if len(raw) == 0 {
		return nil, false, nil
	}
	return append([]byte(nil), raw...), true, nil
}

func (s *PostgresStore) projectStatePayload(ctx context.Context, key string, payload []byte) error {
	if s != nil && s.projectionApplier != nil {
		return s.projectionApplier(ctx, key, payload)
	}
	return s.applyProjection(ctx, key, payload)
}

func statePayloadHash(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func jsonPayloadEqual(left, right []byte) (bool, error) {
	var leftValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		return false, err
	}
	var rightValue any
	if err := json.Unmarshal(right, &rightValue); err != nil {
		return false, err
	}
	return reflect.DeepEqual(leftValue, rightValue), nil
}

func deleteProjectionRowsNotInIDs(ctx context.Context, tx *sql.Tx, table string, ids []string) error {
	if _, allowed := allowedProjectionDeleteTables[table]; !allowed {
		return fmt.Errorf("%w: %s", ErrInvalidProjectionTable, table)
	}
	if len(ids) == 0 {
		_, err := tx.ExecContext(ctx, fmt.Sprintf("delete from %s", table))
		return err
	}
	_, err := tx.ExecContext(
		ctx,
		fmt.Sprintf("delete from %s where id <> all($1)", table),
		pq.Array(ids),
	)
	return err
}

func (s *PostgresStore) applyProjection(ctx context.Context, key string, payload []byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	switch key {
	case stateKeyTenant:
		var snapshot tenantStateSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		tenantIDs := make([]string, 0, len(snapshot.Tenants))
		for i := range snapshot.Tenants {
			tenantIDs = append(tenantIDs, snapshot.Tenants[i].ID)
			raw, err := json.Marshal(snapshot.Tenants[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_tenants (id, name, tenant_type, hq_region, status, created_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7::jsonb,now())
on conflict (id) do update
set name = excluded.name,
    tenant_type = excluded.tenant_type,
    hq_region = excluded.hq_region,
    status = excluded.status,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.Tenants[i].ID,
				snapshot.Tenants[i].Name,
				snapshot.Tenants[i].Type,
				snapshot.Tenants[i].HQRegion,
				snapshot.Tenants[i].Status,
				snapshot.Tenants[i].CreatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_tenants", tenantIDs); err != nil {
			return err
		}
	case stateKeySpace:
		var snapshot spaceStateSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		buildingIDs := make([]string, 0, len(snapshot.Buildings))
		floorIDs := make([]string, 0, len(snapshot.Floors))
		areaIDs := make([]string, 0, len(snapshot.Areas))
		doorIDs := make([]string, 0, len(snapshot.Doors))
		doorGroupIDs := make([]string, 0, len(snapshot.DoorGroups))
		for i := range snapshot.Buildings {
			buildingIDs = append(buildingIDs, snapshot.Buildings[i].ID)
			raw, err := json.Marshal(snapshot.Buildings[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_buildings (id, tenant_id, name, address, region, created_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    name = excluded.name,
    address = excluded.address,
    region = excluded.region,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.Buildings[i].ID,
				snapshot.Buildings[i].TenantID,
				snapshot.Buildings[i].Name,
				snapshot.Buildings[i].Address,
				snapshot.Buildings[i].Region,
				snapshot.Buildings[i].CreatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		for i := range snapshot.Floors {
			floorIDs = append(floorIDs, snapshot.Floors[i].ID)
			raw, err := json.Marshal(snapshot.Floors[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_floors (id, tenant_id, building_id, name, created_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    building_id = excluded.building_id,
    name = excluded.name,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.Floors[i].ID,
				snapshot.Floors[i].TenantID,
				snapshot.Floors[i].BuildingID,
				snapshot.Floors[i].Name,
				snapshot.Floors[i].CreatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		for i := range snapshot.Areas {
			areaIDs = append(areaIDs, snapshot.Areas[i].ID)
			raw, err := json.Marshal(snapshot.Areas[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_areas (id, tenant_id, building_id, floor_id, name, created_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    building_id = excluded.building_id,
    floor_id = excluded.floor_id,
    name = excluded.name,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.Areas[i].ID,
				snapshot.Areas[i].TenantID,
				snapshot.Areas[i].BuildingID,
				snapshot.Areas[i].FloorID,
				snapshot.Areas[i].Name,
				snapshot.Areas[i].CreatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		for i := range snapshot.Doors {
			doorIDs = append(doorIDs, snapshot.Doors[i].ID)
			raw, err := json.Marshal(snapshot.Doors[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_doors (id, tenant_id, building_id, floor_id, area_id, name, gateway_id, kind, status, created_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,now())
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
    synced_at = now()`,
				snapshot.Doors[i].ID,
				snapshot.Doors[i].TenantID,
				snapshot.Doors[i].BuildingID,
				snapshot.Doors[i].FloorID,
				snapshot.Doors[i].AreaID,
				snapshot.Doors[i].Name,
				snapshot.Doors[i].GatewayID,
				snapshot.Doors[i].Kind,
				snapshot.Doors[i].Status,
				snapshot.Doors[i].CreatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		for i := range snapshot.DoorGroups {
			doorGroupIDs = append(doorGroupIDs, snapshot.DoorGroups[i].ID)
			raw, err := json.Marshal(snapshot.DoorGroups[i])
			if err != nil {
				return err
			}
			doorIDs, err := json.Marshal(snapshot.DoorGroups[i].DoorIDs)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_door_groups (id, tenant_id, name, door_ids, created_at, raw, synced_at)
values ($1,$2,$3,$4::jsonb,$5,$6::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    name = excluded.name,
    door_ids = excluded.door_ids,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.DoorGroups[i].ID,
				snapshot.DoorGroups[i].TenantID,
				snapshot.DoorGroups[i].Name,
				string(doorIDs),
				snapshot.DoorGroups[i].CreatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_buildings", buildingIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_floors", floorIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_areas", areaIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_doors", doorIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_door_groups", doorGroupIDs); err != nil {
			return err
		}
	case stateKeyAccess:
		var snapshot accessStateSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		userIDs := make([]string, 0, len(snapshot.Users))
		userGroupIDs := make([]string, 0, len(snapshot.UserGroups))
		policyIDs := make([]string, 0, len(snapshot.Policies))
		temporaryAccessIDs := make([]string, 0, len(snapshot.TemporaryAccess))
		visitorPassIDs := make([]string, 0, len(snapshot.VisitorPasses))
		for i := range snapshot.Users {
			userIDs = append(userIDs, snapshot.Users[i].ID)
			raw, err := json.Marshal(snapshot.Users[i])
			if err != nil {
				return err
			}
			groupIDs, err := json.Marshal(snapshot.Users[i].GroupIDs)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_access_users (id, tenant_id, building_id, name, email, role, status, group_ids, created_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10::jsonb,now())
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
    synced_at = now()`,
				snapshot.Users[i].ID,
				snapshot.Users[i].TenantID,
				snapshot.Users[i].BuildingID,
				snapshot.Users[i].Name,
				snapshot.Users[i].Email,
				snapshot.Users[i].Role,
				snapshot.Users[i].Status,
				string(groupIDs),
				snapshot.Users[i].CreatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		for i := range snapshot.UserGroups {
			userGroupIDs = append(userGroupIDs, snapshot.UserGroups[i].ID)
			raw, err := json.Marshal(snapshot.UserGroups[i])
			if err != nil {
				return err
			}
			members, err := json.Marshal(snapshot.UserGroups[i].Members)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_access_user_groups (id, tenant_id, building_id, name, description, members, created_at, updated_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    building_id = excluded.building_id,
    name = excluded.name,
    description = excluded.description,
    members = excluded.members,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.UserGroups[i].ID,
				snapshot.UserGroups[i].TenantID,
				snapshot.UserGroups[i].BuildingID,
				snapshot.UserGroups[i].Name,
				snapshot.UserGroups[i].Description,
				string(members),
				snapshot.UserGroups[i].CreatedAt,
				snapshot.UserGroups[i].UpdatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		for i := range snapshot.Policies {
			policyIDs = append(policyIDs, snapshot.Policies[i].ID)
			raw, err := json.Marshal(snapshot.Policies[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_access_policies (id, tenant_id, name, scope_type, building_id, area_id, door_id, schedule, members, status, updated_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,now())
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
    synced_at = now()`,
				snapshot.Policies[i].ID,
				snapshot.Policies[i].TenantID,
				snapshot.Policies[i].Name,
				snapshot.Policies[i].ScopeType,
				snapshot.Policies[i].BuildingID,
				snapshot.Policies[i].AreaID,
				snapshot.Policies[i].DoorID,
				snapshot.Policies[i].Schedule,
				snapshot.Policies[i].Members,
				snapshot.Policies[i].Status,
				snapshot.Policies[i].UpdatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		for i := range snapshot.TemporaryAccess {
			temporaryAccessIDs = append(temporaryAccessIDs, snapshot.TemporaryAccess[i].ID)
			raw, err := json.Marshal(snapshot.TemporaryAccess[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_temporary_access (id, tenant_id, scope_type, building_id, area_id, door_id, delivery_method, grantee_name, grantee_email, grantee_phone, valid_until, authorized_by_email, authorized_by_role, authorized_at, created_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16::jsonb,now())
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
    synced_at = now()`,
				snapshot.TemporaryAccess[i].ID,
				snapshot.TemporaryAccess[i].TenantID,
				snapshot.TemporaryAccess[i].ScopeType,
				snapshot.TemporaryAccess[i].BuildingID,
				snapshot.TemporaryAccess[i].AreaID,
				snapshot.TemporaryAccess[i].DoorID,
				snapshot.TemporaryAccess[i].DeliveryMethod,
				snapshot.TemporaryAccess[i].GranteeName,
				snapshot.TemporaryAccess[i].GranteeEmail,
				snapshot.TemporaryAccess[i].GranteePhone,
				snapshot.TemporaryAccess[i].ValidUntil,
				snapshot.TemporaryAccess[i].AuthorizedByEmail,
				snapshot.TemporaryAccess[i].AuthorizedByRole,
				snapshot.TemporaryAccess[i].AuthorizedAt,
				snapshot.TemporaryAccess[i].CreatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		for i := range snapshot.VisitorPasses {
			visitorPassIDs = append(visitorPassIDs, snapshot.VisitorPasses[i].ID)
			raw, err := json.Marshal(snapshot.VisitorPasses[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_visitor_passes (id, tenant_id, building_id, host, visitor, delivery_method, expires_at, created_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    building_id = excluded.building_id,
    host = excluded.host,
    visitor = excluded.visitor,
    delivery_method = excluded.delivery_method,
    expires_at = excluded.expires_at,
    created_at = excluded.created_at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.VisitorPasses[i].ID,
				snapshot.VisitorPasses[i].TenantID,
				snapshot.VisitorPasses[i].BuildingID,
				snapshot.VisitorPasses[i].Host,
				snapshot.VisitorPasses[i].Visitor,
				snapshot.VisitorPasses[i].DeliveryMethod,
				snapshot.VisitorPasses[i].ExpiresAt,
				snapshot.VisitorPasses[i].CreatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_access_users", userIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_access_user_groups", userGroupIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_access_policies", policyIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_temporary_access", temporaryAccessIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_visitor_passes", visitorPassIDs); err != nil {
			return err
		}
	case stateKeyGateway:
		var snapshot gatewayStateSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		gatewayIDs := make([]string, 0, len(snapshot.Gateways))
		deviceIDs := make([]string, 0, len(snapshot.Gateways)*2)
		serialInventoryIDs := make([]string, 0, len(snapshot.SerialInventory))
		for i := range snapshot.Gateways {
			gatewayIDs = append(gatewayIDs, snapshot.Gateways[i].ID)
			raw, err := json.Marshal(snapshot.Gateways[i])
			if err != nil {
				return err
			}
			devices, err := json.Marshal(snapshot.Gateways[i].Devices)
			if err != nil {
				return err
			}
			boundDoorIDs, err := json.Marshal(snapshot.Gateways[i].BoundDoorIDs)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_gateways (id, tenant_id, serial_number, building_id, device_capacity, status, last_seen_at, devices, bound_door_ids, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10::jsonb,now())
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
    synced_at = now()`,
				snapshot.Gateways[i].ID,
				snapshot.Gateways[i].TenantID,
				snapshot.Gateways[i].SerialNumber,
				snapshot.Gateways[i].BuildingID,
				snapshot.Gateways[i].DeviceCapacity,
				snapshot.Gateways[i].Status,
				snapshot.Gateways[i].LastSeenAt,
				string(devices),
				string(boundDoorIDs),
				string(raw),
			); err != nil {
				return err
			}
			for d := range snapshot.Gateways[i].Devices {
				deviceIDs = append(deviceIDs, snapshot.Gateways[i].Devices[d].ID)
				deviceRaw, err := json.Marshal(snapshot.Gateways[i].Devices[d])
				if err != nil {
					return err
				}
				rs485Config := "null"
				if snapshot.Gateways[i].Devices[d].RS485Config != nil {
					rs485Raw, err := json.Marshal(snapshot.Gateways[i].Devices[d].RS485Config)
					if err != nil {
						return err
					}
					rs485Config = string(rs485Raw)
				}
				rs485Health := "null"
				if snapshot.Gateways[i].Devices[d].RS485Health != nil {
					rs485HealthRaw, err := json.Marshal(snapshot.Gateways[i].Devices[d].RS485Health)
					if err != nil {
						return err
					}
					rs485Health = string(rs485HealthRaw)
				}
				if _, err := tx.ExecContext(ctx, `
		insert into mistypass_gateway_devices (id, gateway_id, tenant_id, serial_number, kind, source, protocol, rs485_config, rs485_health, status, last_seen_at, raw, synced_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10,$11,$12::jsonb,now())
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
		    synced_at = now()`,
					snapshot.Gateways[i].Devices[d].ID,
					snapshot.Gateways[i].ID,
					snapshot.Gateways[i].TenantID,
					snapshot.Gateways[i].Devices[d].SerialNumber,
					snapshot.Gateways[i].Devices[d].Kind,
					snapshot.Gateways[i].Devices[d].Source,
					snapshot.Gateways[i].Devices[d].Protocol,
					rs485Config,
					rs485Health,
					snapshot.Gateways[i].Devices[d].Status,
					snapshot.Gateways[i].Devices[d].LastSeenAt,
					string(deviceRaw),
				); err != nil {
					return err
				}
			}
		}
		for i := range snapshot.SerialInventory {
			serialInventoryIDs = append(serialInventoryIDs, snapshot.SerialInventory[i].ID)
			raw, err := json.Marshal(snapshot.SerialInventory[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_gateway_serial_inventory (
	id, tenant_id, serial_number, product_type, status, batch_code, source, consumed_gateway_id, consumed_at, created_at, updated_at, raw, synced_at
)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,now())
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
    synced_at = now()`,
				snapshot.SerialInventory[i].ID,
				snapshot.SerialInventory[i].TenantID,
				snapshot.SerialInventory[i].SerialNumber,
				snapshot.SerialInventory[i].ProductType,
				snapshot.SerialInventory[i].Status,
				snapshot.SerialInventory[i].BatchCode,
				snapshot.SerialInventory[i].Source,
				snapshot.SerialInventory[i].ConsumedGatewayID,
				snapshot.SerialInventory[i].ConsumedAt,
				snapshot.SerialInventory[i].CreatedAt,
				snapshot.SerialInventory[i].UpdatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_gateways", gatewayIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_gateway_devices", deviceIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_gateway_serial_inventory", serialInventoryIDs); err != nil {
			return err
		}
	case stateKeyEnterprise:
		var snapshot enterpriseStateSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		domainMappingIDs := make([]string, 0, len(snapshot.DomainMappings))
		idpConfigIDs := make([]string, 0, len(snapshot.IDPConfigs))
		employeeIDs := make([]string, 0, len(snapshot.Employees))
		syncJobIDs := make([]string, 0, len(snapshot.SyncJobs))
		for i := range snapshot.DomainMappings {
			domainMappingIDs = append(domainMappingIDs, snapshot.DomainMappings[i].ID)
			raw, err := json.Marshal(snapshot.DomainMappings[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_enterprise_domain_mappings (id, tenant_id, domain, status, created_at, updated_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    domain = excluded.domain,
    status = excluded.status,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.DomainMappings[i].ID,
				snapshot.DomainMappings[i].TenantID,
				snapshot.DomainMappings[i].Domain,
				snapshot.DomainMappings[i].Status,
				snapshot.DomainMappings[i].CreatedAt,
				snapshot.DomainMappings[i].UpdatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		for _, config := range snapshot.IDPConfigs {
			idpConfigIDs = append(idpConfigIDs, config.ID)
			raw, err := json.Marshal(config)
			if err != nil {
				return err
			}
			scopes, err := json.Marshal(config.Scopes)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_enterprise_idp_configs (id, tenant_id, provider, issuer_url, client_id, status, sync_mode, scopes, updated_by, created_at, updated_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11,$12::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    provider = excluded.provider,
    issuer_url = excluded.issuer_url,
    client_id = excluded.client_id,
    status = excluded.status,
    sync_mode = excluded.sync_mode,
    scopes = excluded.scopes,
    updated_by = excluded.updated_by,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at,
    raw = excluded.raw,
    synced_at = now()`,
				config.ID,
				config.TenantID,
				config.Provider,
				config.IssuerURL,
				config.ClientID,
				config.Status,
				config.SyncMode,
				string(scopes),
				config.UpdatedBy,
				config.CreatedAt,
				config.UpdatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		for i := range snapshot.Employees {
			employeeIDs = append(employeeIDs, snapshot.Employees[i].ID)
			raw, err := json.Marshal(snapshot.Employees[i])
			if err != nil {
				return err
			}
			groupIDs, err := json.Marshal(snapshot.Employees[i].GroupIDs)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_enterprise_employees (id, tenant_id, external_id, email, full_name, department, job_title, location, access_role, building_id, group_ids, status, source, last_synced_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12,$13,$14,$15::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    external_id = excluded.external_id,
    email = excluded.email,
    full_name = excluded.full_name,
    department = excluded.department,
    job_title = excluded.job_title,
    location = excluded.location,
    access_role = excluded.access_role,
    building_id = excluded.building_id,
    group_ids = excluded.group_ids,
    status = excluded.status,
    source = excluded.source,
    last_synced_at = excluded.last_synced_at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.Employees[i].ID,
				snapshot.Employees[i].TenantID,
				snapshot.Employees[i].ExternalID,
				snapshot.Employees[i].Email,
				snapshot.Employees[i].FullName,
				snapshot.Employees[i].Department,
				snapshot.Employees[i].JobTitle,
				snapshot.Employees[i].Location,
				snapshot.Employees[i].AccessRole,
				snapshot.Employees[i].BuildingID,
				string(groupIDs),
				snapshot.Employees[i].Status,
				snapshot.Employees[i].Source,
				snapshot.Employees[i].LastSyncedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		for i := range snapshot.SyncJobs {
			syncJobIDs = append(syncJobIDs, snapshot.SyncJobs[i].ID)
			raw, err := json.Marshal(snapshot.SyncJobs[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_enterprise_sync_jobs (id, tenant_id, source, status, total, created, updated, deactivated, rejected, actor, started_at, ended_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    source = excluded.source,
    status = excluded.status,
    total = excluded.total,
    created = excluded.created,
    updated = excluded.updated,
    deactivated = excluded.deactivated,
    rejected = excluded.rejected,
    actor = excluded.actor,
    started_at = excluded.started_at,
    ended_at = excluded.ended_at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.SyncJobs[i].ID,
				snapshot.SyncJobs[i].TenantID,
				snapshot.SyncJobs[i].Source,
				snapshot.SyncJobs[i].Status,
				snapshot.SyncJobs[i].Total,
				snapshot.SyncJobs[i].Created,
				snapshot.SyncJobs[i].Updated,
				snapshot.SyncJobs[i].Deactivated,
				snapshot.SyncJobs[i].Rejected,
				snapshot.SyncJobs[i].Actor,
				snapshot.SyncJobs[i].StartedAt,
				snapshot.SyncJobs[i].EndedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_enterprise_domain_mappings", domainMappingIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_enterprise_idp_configs", idpConfigIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_enterprise_employees", employeeIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_enterprise_sync_jobs", syncJobIDs); err != nil {
			return err
		}
	case stateKeyEvent:
		var snapshot eventStateSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		accessEventIDs := make([]string, 0, len(snapshot.AccessEvents))
		deviceEventIDs := make([]string, 0, len(snapshot.DeviceEvents))
		for i := range snapshot.AccessEvents {
			accessEventIDs = append(accessEventIDs, snapshot.AccessEvents[i].ID)
			raw, err := json.Marshal(snapshot.AccessEvents[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_access_events (id, tenant_id, building_id, area_id, event_type, actor, door_id, gateway_id, result, at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,now())
on conflict (id) do update
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
    synced_at = now()`,
				snapshot.AccessEvents[i].ID,
				snapshot.AccessEvents[i].TenantID,
				snapshot.AccessEvents[i].BuildingID,
				snapshot.AccessEvents[i].AreaID,
				snapshot.AccessEvents[i].Type,
				snapshot.AccessEvents[i].Actor,
				snapshot.AccessEvents[i].DoorID,
				snapshot.AccessEvents[i].GatewayID,
				snapshot.AccessEvents[i].Result,
				snapshot.AccessEvents[i].At,
				string(raw),
			); err != nil {
				return err
			}
		}
		for i := range snapshot.DeviceEvents {
			deviceEventIDs = append(deviceEventIDs, snapshot.DeviceEvents[i].ID)
			raw, err := json.Marshal(snapshot.DeviceEvents[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_device_events (id, tenant_id, building_id, event_type, gateway_id, detail, result, at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    building_id = excluded.building_id,
    event_type = excluded.event_type,
    gateway_id = excluded.gateway_id,
    detail = excluded.detail,
    result = excluded.result,
    at = excluded.at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.DeviceEvents[i].ID,
				snapshot.DeviceEvents[i].TenantID,
				snapshot.DeviceEvents[i].BuildingID,
				snapshot.DeviceEvents[i].Type,
				snapshot.DeviceEvents[i].GatewayID,
				snapshot.DeviceEvents[i].Detail,
				snapshot.DeviceEvents[i].Result,
				snapshot.DeviceEvents[i].At,
				string(raw),
			); err != nil {
				return err
			}
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_access_events", accessEventIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_device_events", deviceEventIDs); err != nil {
			return err
		}
	case stateKeyAlarm:
		var snapshot alarmStateSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		alarmIDs := make([]string, 0, len(snapshot.Alarms))
		for i := range snapshot.Alarms {
			alarmIDs = append(alarmIDs, snapshot.Alarms[i].ID)
			raw, err := json.Marshal(snapshot.Alarms[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_alarms (id, tenant_id, building_id, area_id, door_id, alarm_type, severity, location, status, created_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,now())
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
    synced_at = now()`,
				snapshot.Alarms[i].ID,
				snapshot.Alarms[i].TenantID,
				snapshot.Alarms[i].BuildingID,
				snapshot.Alarms[i].AreaID,
				snapshot.Alarms[i].DoorID,
				snapshot.Alarms[i].Type,
				snapshot.Alarms[i].Severity,
				snapshot.Alarms[i].Location,
				snapshot.Alarms[i].Status,
				snapshot.Alarms[i].CreatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_alarms", alarmIDs); err != nil {
			return err
		}
	case stateKeyAudit:
		var snapshot auditStateSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		auditLogIDs := make([]string, 0, len(snapshot.Logs))
		for i := range snapshot.Logs {
			auditLogIDs = append(auditLogIDs, snapshot.Logs[i].ID)
			raw, err := json.Marshal(snapshot.Logs[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_audit_logs (id, tenant_id, actor, role, action, target, source, at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    actor = excluded.actor,
    role = excluded.role,
    action = excluded.action,
    target = excluded.target,
    source = excluded.source,
    at = excluded.at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.Logs[i].ID,
				snapshot.Logs[i].TenantID,
				snapshot.Logs[i].Actor,
				snapshot.Logs[i].Role,
				snapshot.Logs[i].Action,
				snapshot.Logs[i].Target,
				snapshot.Logs[i].Source,
				snapshot.Logs[i].At,
				string(raw),
			); err != nil {
				return err
			}
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_audit_logs", auditLogIDs); err != nil {
			return err
		}
	case stateKeyWallet:
		var snapshot walletStateSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		configIDs := make([]string, 0, 1)
		templateIDs := make([]string, 0, len(snapshot.Templates))
		passIDs := make([]string, 0, len(snapshot.Passes))
		jobIDs := make([]string, 0, len(snapshot.Jobs))
		auditLogIDs := make([]string, 0, len(snapshot.AuditLogs))
		if snapshot.Config != nil {
			configIDs = append(configIDs, snapshot.Config.ID)
			raw, err := json.Marshal(snapshot.Config)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_wallet_configs (id, tenant_id, provider, issuer_id, service_account_email, key_ref, status, created_at, updated_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    provider = excluded.provider,
    issuer_id = excluded.issuer_id,
    service_account_email = excluded.service_account_email,
    key_ref = excluded.key_ref,
    status = excluded.status,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.Config.ID,
				snapshot.Config.TenantID,
				snapshot.Config.Provider,
				snapshot.Config.IssuerID,
				snapshot.Config.ServiceAccountEmail,
				snapshot.Config.KeyRef,
				snapshot.Config.Status,
				snapshot.Config.CreatedAt,
				snapshot.Config.UpdatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		for i := range snapshot.Templates {
			templateIDs = append(templateIDs, snapshot.Templates[i].ID)
			raw, err := json.Marshal(snapshot.Templates[i])
			if err != nil {
				return err
			}
			styleConfig, err := json.Marshal(snapshot.Templates[i].StyleConfig)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_wallet_templates (id, tenant_id, provider, pass_type, class_id, name, status, style_config, created_at, updated_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    provider = excluded.provider,
    pass_type = excluded.pass_type,
    class_id = excluded.class_id,
    name = excluded.name,
    status = excluded.status,
    style_config = excluded.style_config,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.Templates[i].ID,
				snapshot.Templates[i].TenantID,
				snapshot.Templates[i].Provider,
				snapshot.Templates[i].PassType,
				snapshot.Templates[i].ClassID,
				snapshot.Templates[i].Name,
				snapshot.Templates[i].Status,
				string(styleConfig),
				snapshot.Templates[i].CreatedAt,
				snapshot.Templates[i].UpdatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		for i := range snapshot.Passes {
			passIDs = append(passIDs, snapshot.Passes[i].ID)
			raw, err := json.Marshal(snapshot.Passes[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_wallet_passes (id, tenant_id, provider, template_id, target_type, target_id, object_id, status, save_link, expires_at, issued_at, activated_at, revoked_at, created_by, updated_by, created_at, updated_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    provider = excluded.provider,
    template_id = excluded.template_id,
    target_type = excluded.target_type,
    target_id = excluded.target_id,
    object_id = excluded.object_id,
    status = excluded.status,
    save_link = excluded.save_link,
    expires_at = excluded.expires_at,
    issued_at = excluded.issued_at,
    activated_at = excluded.activated_at,
    revoked_at = excluded.revoked_at,
    created_by = excluded.created_by,
    updated_by = excluded.updated_by,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.Passes[i].ID,
				snapshot.Passes[i].TenantID,
				snapshot.Passes[i].Provider,
				snapshot.Passes[i].TemplateID,
				snapshot.Passes[i].TargetType,
				snapshot.Passes[i].TargetID,
				snapshot.Passes[i].ObjectID,
				snapshot.Passes[i].Status,
				snapshot.Passes[i].SaveLink,
				snapshot.Passes[i].ExpiresAt,
				snapshot.Passes[i].IssuedAt,
				snapshot.Passes[i].ActivatedAt,
				snapshot.Passes[i].RevokedAt,
				snapshot.Passes[i].CreatedBy,
				snapshot.Passes[i].UpdatedBy,
				snapshot.Passes[i].CreatedAt,
				snapshot.Passes[i].UpdatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		for i := range snapshot.Jobs {
			jobIDs = append(jobIDs, snapshot.Jobs[i].ID)
			raw, err := json.Marshal(snapshot.Jobs[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_wallet_jobs (id, tenant_id, provider, batch_id, template_id, target_type, target_id, expires_at, pass_id, status, retry_count, error_code, error_message, created_at, updated_at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    provider = excluded.provider,
    batch_id = excluded.batch_id,
    template_id = excluded.template_id,
    target_type = excluded.target_type,
    target_id = excluded.target_id,
    expires_at = excluded.expires_at,
    pass_id = excluded.pass_id,
    status = excluded.status,
    retry_count = excluded.retry_count,
    error_code = excluded.error_code,
    error_message = excluded.error_message,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.Jobs[i].ID,
				snapshot.Jobs[i].TenantID,
				snapshot.Jobs[i].Provider,
				snapshot.Jobs[i].BatchID,
				snapshot.Jobs[i].TemplateID,
				snapshot.Jobs[i].TargetType,
				snapshot.Jobs[i].TargetID,
				snapshot.Jobs[i].ExpiresAt,
				snapshot.Jobs[i].PassID,
				snapshot.Jobs[i].Status,
				snapshot.Jobs[i].RetryCount,
				snapshot.Jobs[i].ErrorCode,
				snapshot.Jobs[i].ErrorMessage,
				snapshot.Jobs[i].CreatedAt,
				snapshot.Jobs[i].UpdatedAt,
				string(raw),
			); err != nil {
				return err
			}
		}
		for i := range snapshot.AuditLogs {
			auditLogIDs = append(auditLogIDs, snapshot.AuditLogs[i].ID)
			raw, err := json.Marshal(snapshot.AuditLogs[i])
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
insert into mistypass_wallet_audit_logs (id, tenant_id, action, actor, target_id, result, at, raw, synced_at)
values ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    action = excluded.action,
    actor = excluded.actor,
    target_id = excluded.target_id,
    result = excluded.result,
    at = excluded.at,
    raw = excluded.raw,
    synced_at = now()`,
				snapshot.AuditLogs[i].ID,
				snapshot.AuditLogs[i].TenantID,
				snapshot.AuditLogs[i].Action,
				snapshot.AuditLogs[i].Actor,
				snapshot.AuditLogs[i].TargetID,
				snapshot.AuditLogs[i].Result,
				snapshot.AuditLogs[i].At,
				string(raw),
			); err != nil {
				return err
			}
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_wallet_configs", configIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_wallet_templates", templateIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_wallet_passes", passIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_wallet_jobs", jobIDs); err != nil {
			return err
		}
		if err := deleteProjectionRowsNotInIDs(ctx, tx, "mistypass_wallet_audit_logs", auditLogIDs); err != nil {
			return err
		}
	default:
		// No projection for unknown keys.
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	tx = nil
	return nil
}

func (s *PostgresStore) appendStateChangeTx(
	ctx context.Context,
	tx *sql.Tx,
	key string,
	payloadHash string,
	payload []byte,
) (int64, error) {
	nextKey := strings.TrimSpace(key)
	nextHash := strings.TrimSpace(payloadHash)
	if nextKey == "" || nextHash == "" || len(payload) == 0 {
		return 0, nil
	}
	var changeID int64
	err := tx.QueryRowContext(
		ctx,
		`insert into mistypass_change_log (state_key, change_type, payload_hash, payload, created_at)
	values ($1,$2,$3,$4::jsonb,now())
	returning id`,
		nextKey,
		changeTypeSnapshotSaved,
		nextHash,
		string(payload),
	).Scan(&changeID)
	if err != nil {
		return 0, err
	}
	return changeID, nil
}

func (s *PostgresStore) ListStateChanges(stateKey string, limit int) ([]StateChangeRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("postgres store is not initialized")
	}
	nextLimit := normalizeReplayLimit(limit)
	nextKey := strings.TrimSpace(stateKey)

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()

	var rows *sql.Rows
	var err error
	if nextKey == "" {
		rows, err = s.db.QueryContext(
			ctx,
			`select id, state_key, change_type, payload_hash, payload, created_at
from mistypass_change_log
order by id desc
limit $1`,
			nextLimit,
		)
	} else {
		rows, err = s.db.QueryContext(
			ctx,
			`select id, state_key, change_type, payload_hash, payload, created_at
from mistypass_change_log
where state_key = $1
order by id desc
limit $2`,
			nextKey,
			nextLimit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]StateChangeRecord, 0, nextLimit)
	for rows.Next() {
		var record StateChangeRecord
		var payload []byte
		if err := rows.Scan(
			&record.ID,
			&record.StateKey,
			&record.ChangeType,
			&record.PayloadHash,
			&payload,
			&record.CreatedAt,
		); err != nil {
			return nil, err
		}
		record.Payload = append([]byte(nil), payload...)
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PostgresStore) ReplayStateChanges(stateKey string, fromID int64, limit int) (ReplayStateChangesResult, error) {
	if s == nil || s.db == nil {
		return ReplayStateChangesResult{}, errors.New("postgres store is not initialized")
	}
	nextKey := strings.TrimSpace(stateKey)
	if nextKey == "" {
		return ReplayStateChangesResult{}, ErrStateKeyRequired
	}
	nextFromID := fromID
	if nextFromID < 0 {
		nextFromID = 0
	}
	nextLimit := normalizeReplayLimit(limit)

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(
		ctx,
		`select id, payload
from mistypass_change_log
where state_key = $1 and id > $2
order by id asc
limit $3`,
		nextKey,
		nextFromID,
		nextLimit,
	)
	if err != nil {
		return ReplayStateChangesResult{}, err
	}
	defer rows.Close()

	result := ReplayStateChangesResult{}
	for rows.Next() {
		var changeID int64
		var payload []byte
		if err := rows.Scan(&changeID, &payload); err != nil {
			return ReplayStateChangesResult{}, err
		}
		if err := s.projectStatePayload(ctx, nextKey, payload); err != nil {
			return ReplayStateChangesResult{}, fmt.Errorf("replay projection failed at change_id=%d: %w", changeID, err)
		}
		result.Applied++
		result.LastChangeID = changeID
	}
	if err := rows.Err(); err != nil {
		return ReplayStateChangesResult{}, err
	}
	return result, nil
}

func (s *PostgresStore) ListReplayCheckpoints(stateKey string, limit int) ([]ReplayCheckpoint, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("postgres store is not initialized")
	}

	nextLimit := normalizeReplayLimit(limit)
	nextKey := strings.TrimSpace(stateKey)

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()

	var rows *sql.Rows
	var err error
	if nextKey == "" {
		rows, err = s.db.QueryContext(
			ctx,
			`select state_key, last_change_id, updated_at
from mistypass_change_replay_checkpoints
order by updated_at desc, state_key asc
limit $1`,
			nextLimit,
		)
	} else {
		rows, err = s.db.QueryContext(
			ctx,
			`select state_key, last_change_id, updated_at
from mistypass_change_replay_checkpoints
where state_key = $1
order by updated_at desc
limit $2`,
			nextKey,
			nextLimit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ReplayCheckpoint, 0, nextLimit)
	for rows.Next() {
		var item ReplayCheckpoint
		if err := rows.Scan(&item.StateKey, &item.LastChangeID, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PostgresStore) ReplayStateChangesFromCheckpoint(stateKey string, limit int) (ReplayFromCheckpointResult, error) {
	if s == nil || s.db == nil {
		return ReplayFromCheckpointResult{}, errors.New("postgres store is not initialized")
	}

	nextKey := strings.TrimSpace(stateKey)
	if nextKey == "" {
		return ReplayFromCheckpointResult{}, ErrStateKeyRequired
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()

	checkpoint, exists, err := s.getReplayCheckpoint(ctx, nextKey)
	if err != nil {
		return ReplayFromCheckpointResult{}, err
	}

	fromID := int64(0)
	if exists {
		fromID = checkpoint.LastChangeID
	}

	replayResult, err := s.ReplayStateChanges(nextKey, fromID, limit)
	if err != nil {
		return ReplayFromCheckpointResult{}, err
	}

	nextLastChangeID := fromID
	if replayResult.Applied > 0 {
		nextLastChangeID = replayResult.LastChangeID
	}

	savedCheckpoint, err := s.upsertReplayCheckpoint(ctx, nextKey, nextLastChangeID)
	if err != nil {
		return ReplayFromCheckpointResult{}, err
	}

	return ReplayFromCheckpointResult{
		StateKey:     nextKey,
		FromID:       fromID,
		Applied:      replayResult.Applied,
		LastChangeID: nextLastChangeID,
		Checkpoint:   savedCheckpoint,
	}, nil
}

func (s *PostgresStore) getReplayCheckpoint(ctx context.Context, stateKey string) (ReplayCheckpoint, bool, error) {
	var item ReplayCheckpoint
	err := s.db.QueryRowContext(
		ctx,
		`select state_key, last_change_id, updated_at
from mistypass_change_replay_checkpoints
where state_key = $1`,
		stateKey,
	).Scan(&item.StateKey, &item.LastChangeID, &item.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReplayCheckpoint{}, false, nil
		}
		return ReplayCheckpoint{}, false, err
	}
	return item, true, nil
}

func (s *PostgresStore) upsertReplayCheckpoint(ctx context.Context, stateKey string, lastChangeID int64) (ReplayCheckpoint, error) {
	nextLastChangeID := lastChangeID
	if nextLastChangeID < 0 {
		nextLastChangeID = 0
	}

	var item ReplayCheckpoint
	err := s.db.QueryRowContext(
		ctx,
		`insert into mistypass_change_replay_checkpoints (state_key, last_change_id, updated_at)
values ($1, $2, now())
on conflict (state_key) do update
set last_change_id = greatest(mistypass_change_replay_checkpoints.last_change_id, excluded.last_change_id),
    updated_at = now()
returning state_key, last_change_id, updated_at`,
		stateKey,
		nextLastChangeID,
	).Scan(&item.StateKey, &item.LastChangeID, &item.UpdatedAt)
	if err != nil {
		return ReplayCheckpoint{}, err
	}
	return item, nil
}

func normalizeReplayLimit(limit int) int {
	if limit <= 0 {
		return defaultReplayLimit
	}
	if limit > maxReplayLimit {
		return maxReplayLimit
	}
	return limit
}
