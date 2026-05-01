package state

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/alarm"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/event"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
	"github.com/mistypass/cloud/api/internal/modules/space"
	"github.com/mistypass/cloud/api/internal/modules/tenant"
	"github.com/mistypass/cloud/api/internal/modules/wallet"
	"github.com/mistypass/cloud/api/internal/state/sqlcgen"

	"github.com/lib/pq"
	"github.com/sqlc-dev/pqtype"
)

const (
	defaultDriverName   = "postgres"
	defaultQueryTimeout = 5 * time.Second
	defaultReplayLimit  = 100
	maxReplayLimit      = 500
	defaultMaxOpenConns = 25
	defaultMaxIdleConns = 10
	defaultConnMaxIdle  = 5 * time.Minute
	defaultConnLifetime = 30 * time.Minute

	changeTypeSnapshotSaved = "snapshot_saved"

	stateKeyTenant     = "module_tenant"
	stateKeySpace      = "module_space"
	stateKeyAccess     = "module_access"
	stateKeyGateway    = "module_gateway"
	stateKeyEnterprise = "module_enterprise"
	stateKeyEvent      = "module_event"
	stateKeyAlarm       = "module_alarm"
	stateKeyAudit       = "module_audit"
	stateKeyWallet      = "module_wallet"
	stateKeyAlertPolicy = "module_alert_policy"
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
	stateKeyAlertPolicy,
}

var allowedProjectionDeleteTables = map[string]struct{}{
	"mistypass_tenants":                          {},
	"mistypass_buildings":                        {},
	"mistypass_floors":                           {},
	"mistypass_areas":                            {},
	"mistypass_doors":                            {},
	"mistypass_door_groups":                      {},
	"mistypass_access_users":                     {},
	"mistypass_access_user_groups":               {},
	"mistypass_access_policies":                  {},
	"mistypass_temporary_access":                 {},
	"mistypass_visitor_passes":                   {},
	"mistypass_gateways":                         {},
	"mistypass_gateway_devices":                  {},
	"mistypass_gateway_serial_inventory":         {},
	"mistypass_enterprise_domain_mappings":       {},
	"mistypass_enterprise_hris_connectors":       {},
	"mistypass_enterprise_hris_webhook_receipts": {},
	"mistypass_enterprise_idp_configs":           {},
	"mistypass_enterprise_employees":             {},
	"mistypass_enterprise_sync_jobs":             {},
	"mistypass_access_events":                    {},
	"mistypass_device_events":                    {},
	"mistypass_alarms":                           {},
	"mistypass_audit_logs":                       {},
	"mistypass_wallet_configs":                   {},
	"mistypass_wallet_templates":                 {},
	"mistypass_wallet_passes":                    {},
	"mistypass_wallet_jobs":                      {},
	"mistypass_wallet_audit_logs":                {},
	"mistypass_alert_policies":                   {},
	"mistypass_alert_notifications":              {},
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
	DomainMappings        []enterprise.DomainMapping        `json:"domain_mappings"`
	HRISConnectors        []enterprise.HRISConnector        `json:"hris_connectors,omitempty"`
	HRISWebhookReceipts   []enterprise.HRISWebhookReceipt   `json:"hris_webhook_receipts,omitempty"`
	HRISWebhookExecutions []enterprise.HRISWebhookExecution `json:"hris_webhook_executions,omitempty"`
	IDPConfigs            map[string]enterprise.IDPConfig   `json:"idp_configs"`
	Employees             []enterprise.EnterpriseEmployee   `json:"employees"`
	SyncJobs              []enterprise.SyncJob              `json:"sync_jobs"`
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
	queries           *sqlcgen.Queries
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
	db.SetMaxOpenConns(defaultMaxOpenConns)
	db.SetMaxIdleConns(defaultMaxIdleConns)
	db.SetConnMaxIdleTime(defaultConnMaxIdle)
	db.SetConnMaxLifetime(defaultConnLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	store := &PostgresStore{
		db:      db,
		queries: sqlcgen.New(db),
	}
	store.projectionApplier = store.applyProjection
	return store, nil
}

func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresStore) UpsertAuthUser(user auth.User, passwordHash []byte) error {
	if s == nil || s.db == nil {
		return errors.New("postgres store is not initialized")
	}
	nextUser, ok := normalizeAuthUser(user)
	if !ok {
		return errors.New("auth user id/email/role are required")
	}
	buildingIDsJSON, err := json.Marshal(nextUser.BuildingIDs)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	return s.queries.UpsertAuthUser(ctx, sqlcgen.UpsertAuthUserParams{
		ID:           nextUser.ID,
		Name:         nextUser.Name,
		Email:        nextUser.Email,
		Role:         nextUser.Role,
		TenantID:     nextUser.TenantID,
		BuildingIds:  buildingIDsJSON,
		Language:     nextUser.Language,
		PasswordHash: cloneBytes(passwordHash),
	})
}

func (s *PostgresStore) FindAuthUserByEmail(email string) (auth.User, []byte, bool, error) {
	if s == nil || s.db == nil {
		return auth.User{}, nil, false, errors.New("postgres store is not initialized")
	}
	nextEmail := normalizeAuthEmail(email)
	if nextEmail == "" {
		return auth.User{}, nil, false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	row, err := s.queries.GetAuthUserByEmail(ctx, nextEmail)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.User{}, nil, false, nil
	}
	if err != nil {
		return auth.User{}, nil, false, err
	}
	user := auth.User{
		ID:       row.ID,
		Name:     row.Name,
		Email:    row.Email,
		Role:     row.Role,
		TenantID: row.TenantID,
		Language: row.Language,
	}
	buildingIDsRaw := []byte(row.BuildingIds)
	user.BuildingIDs, err = decodeAuthBuildingIDs(buildingIDsRaw)
	if err != nil {
		return auth.User{}, nil, false, err
	}
	normalizedUser, ok := normalizeAuthUser(user)
	if !ok {
		return auth.User{}, nil, false, nil
	}
	return normalizedUser, cloneBytes(row.PasswordHash), true, nil
}

func (s *PostgresStore) FindAuthUserByID(userID string) (auth.User, []byte, bool, error) {
	if s == nil || s.db == nil {
		return auth.User{}, nil, false, errors.New("postgres store is not initialized")
	}
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return auth.User{}, nil, false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	row, err := s.queries.GetAuthUserByID(ctx, nextUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.User{}, nil, false, nil
	}
	if err != nil {
		return auth.User{}, nil, false, err
	}
	user := auth.User{
		ID:       row.ID,
		Name:     row.Name,
		Email:    row.Email,
		Role:     row.Role,
		TenantID: row.TenantID,
		Language: row.Language,
	}
	buildingIDsRaw := []byte(row.BuildingIds)
	user.BuildingIDs, err = decodeAuthBuildingIDs(buildingIDsRaw)
	if err != nil {
		return auth.User{}, nil, false, err
	}
	normalizedUser, ok := normalizeAuthUser(user)
	if !ok {
		return auth.User{}, nil, false, nil
	}
	return normalizedUser, cloneBytes(row.PasswordHash), true, nil
}

func (s *PostgresStore) UpsertAuthRefreshSession(sessionID, userID string, expiresAt time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("postgres store is not initialized")
	}
	nextSessionID := strings.TrimSpace(sessionID)
	nextUserID := strings.TrimSpace(userID)
	nextExpiresAt := expiresAt.UTC()
	if nextSessionID == "" || nextUserID == "" || nextExpiresAt.IsZero() {
		return errors.New("refresh session id/user id/expires_at are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	return s.queries.UpsertAuthRefreshSession(ctx, sqlcgen.UpsertAuthRefreshSessionParams{
		SessionID: nextSessionID,
		UserID:    nextUserID,
		ExpiresAt: nextExpiresAt,
	})
}

func (s *PostgresStore) FindAuthRefreshSession(sessionID string) (string, time.Time, bool, error) {
	if s == nil || s.db == nil {
		return "", time.Time{}, false, errors.New("postgres store is not initialized")
	}
	nextSessionID := strings.TrimSpace(sessionID)
	if nextSessionID == "" {
		return "", time.Time{}, false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	row, err := s.queries.GetAuthRefreshSession(ctx, nextSessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", time.Time{}, false, nil
	}
	if err != nil {
		return "", time.Time{}, false, err
	}
	return strings.TrimSpace(row.UserID), row.ExpiresAt.UTC(), true, nil
}

func (s *PostgresStore) DeleteAuthRefreshSession(sessionID string) error {
	if s == nil || s.db == nil {
		return errors.New("postgres store is not initialized")
	}
	nextSessionID := strings.TrimSpace(sessionID)
	if nextSessionID == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	return s.queries.DeleteAuthRefreshSession(ctx, nextSessionID)
}

func (s *PostgresStore) DeleteAuthRefreshSessionsByUserID(userID string) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("postgres store is not initialized")
	}
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	affected, err := s.queries.DeleteAuthRefreshSessionsByUserID(ctx, nextUserID)
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

func (s *PostgresStore) UpsertAuthRevokedAccessToken(tokenID string, expiresAt time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("postgres store is not initialized")
	}
	nextTokenID := strings.TrimSpace(tokenID)
	nextExpiresAt := expiresAt.UTC()
	if nextTokenID == "" || nextExpiresAt.IsZero() {
		return errors.New("revoked access token id/expires_at are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	return s.queries.UpsertAuthRevokedAccessToken(ctx, sqlcgen.UpsertAuthRevokedAccessTokenParams{
		TokenID:   nextTokenID,
		ExpiresAt: nextExpiresAt,
	})
}

func (s *PostgresStore) IsAuthAccessTokenRevoked(tokenID string, now time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("postgres store is not initialized")
	}
	nextTokenID := strings.TrimSpace(tokenID)
	if nextTokenID == "" {
		return false, nil
	}
	nextNow := now.UTC()

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	expiresAt, err := s.queries.GetAuthRevokedAccessTokenExpiresAt(ctx, nextTokenID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !expiresAt.After(nextNow) {
		_ = s.queries.DeleteExpiredAuthRevokedAccessToken(ctx, sqlcgen.DeleteExpiredAuthRevokedAccessTokenParams{
			TokenID:   nextTokenID,
			ExpiresAt: nextNow,
		})
		return false, nil
	}
	return true, nil
}

func (s *PostgresStore) UpsertAuthAdminMFAState(userID string, state auth.AdminMFAPersistenceState) error {
	if s == nil || s.db == nil {
		return errors.New("postgres store is not initialized")
	}
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return errors.New("auth admin mfa user_id is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	_, err := s.db.ExecContext(
		ctx,
		`insert into mistypass_auth_admin_mfa_states (user_id, secret, pending_secret, enabled, updated_at, created_at)
values ($1, $2, $3, $4, $5, now())
on conflict (user_id) do update
set secret = excluded.secret,
    pending_secret = excluded.pending_secret,
    enabled = excluded.enabled,
    updated_at = excluded.updated_at`,
		nextUserID,
		strings.TrimSpace(state.Secret),
		strings.TrimSpace(state.PendingSecret),
		state.Enabled,
		state.UpdatedAt.UTC(),
	)
	return err
}

func (s *PostgresStore) FindAuthAdminMFAState(userID string) (auth.AdminMFAPersistenceState, bool, error) {
	if s == nil || s.db == nil {
		return auth.AdminMFAPersistenceState{}, false, errors.New("postgres store is not initialized")
	}
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return auth.AdminMFAPersistenceState{}, false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()

	row := s.db.QueryRowContext(
		ctx,
		`select secret, pending_secret, enabled, updated_at
from mistypass_auth_admin_mfa_states
where user_id = $1`,
		nextUserID,
	)
	var state auth.AdminMFAPersistenceState
	err := row.Scan(&state.Secret, &state.PendingSecret, &state.Enabled, &state.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.AdminMFAPersistenceState{}, false, nil
	}
	if err != nil {
		return auth.AdminMFAPersistenceState{}, false, err
	}
	state.Secret = strings.TrimSpace(state.Secret)
	state.PendingSecret = strings.TrimSpace(state.PendingSecret)
	state.UpdatedAt = state.UpdatedAt.UTC()
	return state, true, nil
}

func (s *PostgresStore) UpsertWebAuthnCredential(cred auth.WebAuthnCredential) error {
	if s == nil || s.db == nil {
		return errors.New("postgres store is not initialized")
	}
	if strings.TrimSpace(cred.ID) == "" || strings.TrimSpace(cred.UserID) == "" {
		return errors.New("webauthn credential id and user_id are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	_, err := s.db.ExecContext(
		ctx,
		`insert into mistypass_auth_webauthn_credentials (id, user_id, public_key, attestation_type, aaguid, sign_count, display_name, created_at, updated_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, now())
on conflict (id) do update
set sign_count = excluded.sign_count,
    display_name = excluded.display_name,
    updated_at = now()`,
		cred.ID, cred.UserID, cred.PublicKey, cred.AttestationType, cred.AAGUID, cred.SignCount, cred.DisplayName, cred.CreatedAt.UTC(),
	)
	return err
}

func (s *PostgresStore) FindWebAuthnCredentialsByUserID(userID string) ([]auth.WebAuthnCredential, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("postgres store is not initialized")
	}
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(
		ctx,
		`select id, user_id, public_key, attestation_type, aaguid, sign_count, display_name, created_at
from mistypass_auth_webauthn_credentials
where user_id = $1
order by created_at asc`,
		nextUserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creds []auth.WebAuthnCredential
	for rows.Next() {
		var c auth.WebAuthnCredential
		if err := rows.Scan(&c.ID, &c.UserID, &c.PublicKey, &c.AttestationType, &c.AAGUID, &c.SignCount, &c.DisplayName, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.CreatedAt = c.CreatedAt.UTC()
		creds = append(creds, c)
	}
	return creds, rows.Err()
}

func (s *PostgresStore) DeleteWebAuthnCredential(credentialID string) error {
	if s == nil || s.db == nil {
		return errors.New("postgres store is not initialized")
	}
	nextID := strings.TrimSpace(credentialID)
	if nextID == "" {
		return errors.New("webauthn credential id is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `delete from mistypass_auth_webauthn_credentials where id = $1`, nextID)
	return err
}

func (s *PostgresStore) UpsertGatewayDeviceToken(gatewayID, deviceToken string) error {
	if s == nil || s.db == nil {
		return errors.New("postgres store is not initialized")
	}
	nextGatewayID := strings.TrimSpace(gatewayID)
	nextToken := strings.TrimSpace(deviceToken)
	if nextGatewayID == "" || nextToken == "" {
		return errors.New("gateway_id and device_token are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	return s.queries.UpsertGatewayDeviceToken(ctx, sqlcgen.UpsertGatewayDeviceTokenParams{
		GatewayID: nextGatewayID,
		TokenHash: gatewayDeviceTokenHash(nextToken),
	})
}

func (s *PostgresStore) VerifyGatewayDeviceToken(gatewayID, providedToken string) (bool, bool, error) {
	if s == nil || s.db == nil {
		return false, false, errors.New("postgres store is not initialized")
	}
	nextGatewayID := strings.TrimSpace(gatewayID)
	nextProvidedToken := strings.TrimSpace(providedToken)
	if nextGatewayID == "" || nextProvidedToken == "" {
		return false, false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	storedHash, err := s.queries.GetGatewayDeviceTokenHash(ctx, nextGatewayID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	computed := gatewayDeviceTokenHash(nextProvidedToken)
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(storedHash)), []byte(computed)) == 1 {
		return true, true, nil
	}
	return true, false, nil
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

create table if not exists mistypass_gateway_device_tokens (
  gateway_id text primary key,
  token_hash text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index if not exists mistypass_gateway_device_tokens_updated_idx on mistypass_gateway_device_tokens(updated_at desc);

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
alter table if exists mistypass_auth_users add column if not exists name text not null default '';
alter table if exists mistypass_auth_users add column if not exists language text not null default '';
create unique index if not exists mistypass_auth_users_email_idx on mistypass_auth_users(lower(email));
create index if not exists mistypass_auth_users_updated_idx on mistypass_auth_users(updated_at desc);

create table if not exists mistypass_auth_refresh_sessions (
  session_id text primary key,
  user_id text not null,
  expires_at timestamptz not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index if not exists mistypass_auth_refresh_sessions_user_idx on mistypass_auth_refresh_sessions(user_id);
create index if not exists mistypass_auth_refresh_sessions_expires_idx on mistypass_auth_refresh_sessions(expires_at);

create table if not exists mistypass_auth_revoked_access_tokens (
  token_id text primary key,
  expires_at timestamptz not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index if not exists mistypass_auth_revoked_access_tokens_expires_idx on mistypass_auth_revoked_access_tokens(expires_at);

create table if not exists mistypass_auth_admin_mfa_states (
  user_id text primary key,
  secret text not null default '',
  pending_secret text not null default '',
  enabled boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index if not exists mistypass_auth_admin_mfa_states_updated_idx on mistypass_auth_admin_mfa_states(updated_at desc);

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
create index if not exists mistypass_enterprise_domain_tenant_idx on mistypass_enterprise_domain_mappings(tenant_id);

create table if not exists mistypass_enterprise_hris_connectors (
  id text primary key,
  tenant_id text not null,
  vendor text not null,
  status text not null,
  sync_strategy text not null,
  credential_ref text,
  webhook_secret_ref text,
  last_sync_at timestamptz,
  updated_by text,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);
create index if not exists mistypass_enterprise_hris_connectors_tenant_idx on mistypass_enterprise_hris_connectors(tenant_id);
create unique index if not exists mistypass_enterprise_hris_connectors_tenant_vendor_uidx on mistypass_enterprise_hris_connectors(tenant_id, vendor);

create table if not exists mistypass_enterprise_hris_webhook_receipts (
  id text primary key,
  tenant_id text not null,
  connector_id text not null,
  vendor text not null,
  event_type text,
  request_id text,
  content_type text,
  headers jsonb,
  raw_payload text not null default '',
  source_ip text,
  status text not null,
  attempt_count integer not null default 0,
  last_error text,
  received_at timestamptz not null,
  last_attempt_at timestamptz,
  processed_at timestamptz,
  raw jsonb not null,
  synced_at timestamptz not null default now()
);
alter table if exists mistypass_enterprise_hris_webhook_receipts add column if not exists attempt_count integer not null default 0;
alter table if exists mistypass_enterprise_hris_webhook_receipts add column if not exists last_attempt_at timestamptz;
create index if not exists mistypass_enterprise_hris_webhook_receipts_tenant_idx on mistypass_enterprise_hris_webhook_receipts(tenant_id);
create index if not exists mistypass_enterprise_hris_webhook_receipts_connector_idx on mistypass_enterprise_hris_webhook_receipts(connector_id, received_at desc);

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

func gatewayDeviceTokenHash(deviceToken string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(deviceToken)))
	return hex.EncodeToString(sum[:])
}

func normalizeAuthUser(user auth.User) (auth.User, bool) {
	nextUser := auth.User{
		ID:                  strings.TrimSpace(user.ID),
		Name:                strings.TrimSpace(user.Name),
		Email:               normalizeAuthEmail(user.Email),
		Role:                strings.ToLower(strings.TrimSpace(user.Role)),
		TenantID:            strings.TrimSpace(user.TenantID),
		BuildingIDs:         normalizeAuthIDs(user.BuildingIDs),
		Language:            normalizeAuthLanguage(user.Language),
		PasswordAuthEnabled: user.PasswordAuthEnabled,
	}
	if nextUser.ID == "" || nextUser.Email == "" || nextUser.Role == "" {
		return auth.User{}, false
	}
	return nextUser, true
}

func normalizeAuthEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizeAuthLanguage(language string) string {
	nextLanguage := strings.TrimSpace(language)
	switch nextLanguage {
	case "en-US", "id-ID", "zh-CN":
		return nextLanguage
	default:
		return ""
	}
}

func normalizeAuthIDs(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for i := range values {
		value := strings.TrimSpace(values[i])
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) == 0 {
		return []string{}
	}
	return result
}

func decodeAuthBuildingIDs(raw []byte) ([]string, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return []string{}, nil
	}
	values := []string{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return normalizeAuthIDs(values), nil
}

func cloneBytes(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
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
	raw, err := s.queries.GetStatePayload(ctx, key)
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

	raw, err := s.queries.GetStatePayload(ctx, key)
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
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				slog.Default().Error(
					"state tx rollback failed",
					"op", "save",
					"key", nextKey,
					"err", err,
				)
			}
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

	if err := s.queries.WithTx(tx).UpsertStatePayload(ctx, sqlcgen.UpsertStatePayloadParams{
		StateKey: nextKey,
		Payload:  payload,
	}); err != nil {
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

func (s *PostgresStore) CompareAndSwap(key string, expectedExists bool, expected any, next any) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("postgres store is not initialized")
	}

	nextKey := strings.TrimSpace(key)
	if nextKey == "" {
		return false, ErrStateKeyRequired
	}

	var expectedPayload []byte
	var err error
	if expectedExists {
		expectedPayload, err = json.Marshal(expected)
		if err != nil {
			return false, fmt.Errorf("encode expected state %q: %w", nextKey, err)
		}
	}

	nextPayload, err := json.Marshal(next)
	if err != nil {
		return false, fmt.Errorf("encode next state %q: %w", nextKey, err)
	}
	payloadHash := statePayloadHash(nextPayload)

	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() {
		if tx != nil {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				slog.Default().Error(
					"state tx rollback failed",
					"op", "compare_and_swap",
					"key", nextKey,
					"err", err,
				)
			}
		}
	}()

	currentPayload, found, err := s.loadCurrentPayloadForUpdate(ctx, tx, nextKey)
	if err != nil {
		return false, fmt.Errorf("load state %q: %w", nextKey, err)
	}
	if found != expectedExists {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			return false, err
		}
		tx = nil
		return false, nil
	}
	if expectedExists {
		sameExpected, err := jsonPayloadEqual(currentPayload, expectedPayload)
		if err != nil {
			return false, fmt.Errorf("compare expected state payload %q: %w", nextKey, err)
		}
		if !sameExpected {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				return false, err
			}
			tx = nil
			return false, nil
		}
	}
	if found {
		sameNext, err := jsonPayloadEqual(currentPayload, nextPayload)
		if err != nil {
			return false, fmt.Errorf("compare next state payload %q: %w", nextKey, err)
		}
		if sameNext {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				return false, err
			}
			tx = nil
			return true, nil
		}
	}

	if err := s.queries.WithTx(tx).UpsertStatePayload(ctx, sqlcgen.UpsertStatePayloadParams{
		StateKey: nextKey,
		Payload:  nextPayload,
	}); err != nil {
		return false, fmt.Errorf("save state %q: %w", nextKey, err)
	}

	changeID, err := s.appendStateChangeTx(ctx, tx, nextKey, payloadHash, nextPayload)
	if err != nil {
		return false, fmt.Errorf("append state change %q: %w", nextKey, err)
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	tx = nil

	checkpointLastID := int64(0)
	for batch := 0; batch < 4; batch++ {
		replayResult, replayErr := s.ReplayStateChangesFromCheckpoint(nextKey, maxReplayLimit)
		if replayErr != nil {
			return false, fmt.Errorf("project state %q change_id=%d via replay: %w", nextKey, changeID, replayErr)
		}
		checkpointLastID = replayResult.LastChangeID
		if checkpointLastID >= changeID || replayResult.Applied == 0 {
			break
		}
	}
	return true, nil
}

func (s *PostgresStore) loadCurrentPayloadForUpdate(ctx context.Context, tx *sql.Tx, key string) ([]byte, bool, error) {
	raw, err := s.queries.WithTx(tx).GetStatePayloadForUpdate(ctx, key)
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

// deleteProjectionRowsNotInIDs deletes rows not in the given ID set.
// SAFETY: table name is validated against allowedProjectionDeleteTables (fixed set of known table names)
// before being interpolated into SQL. This is safe from SQL injection.
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

type projectionDeleteSet struct {
	table string
	ids   []string
}

type projectionArgsBuilder[T any] func(item T) (id string, args []any, err error)

func upsertProjectionRows[T any](
	ctx context.Context,
	tx *sql.Tx,
	items []T,
	query string,
	build projectionArgsBuilder[T],
) ([]string, error) {
	ids := make([]string, 0, len(items))
	for i := range items {
		id, args, err := build(items[i])
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

func deleteProjectionRows(ctx context.Context, tx *sql.Tx, sets ...projectionDeleteSet) error {
	for i := range sets {
		if err := deleteProjectionRowsNotInIDs(ctx, tx, sets[i].table, sets[i].ids); err != nil {
			return err
		}
	}
	return nil
}

func marshalProjectionJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func marshalProjectionNullableJSON(value any) (string, error) {
	if value == nil {
		return "null", nil
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface:
		if rv.IsNil() {
			return "null", nil
		}
	}
	return marshalProjectionJSON(value)
}

func sqlText(value string) sql.NullString {
	return sql.NullString{
		String: value,
		Valid:  true,
	}
}

func sqlRawJSON(value string) pqtype.NullRawMessage {
	return pqtype.NullRawMessage{
		RawMessage: []byte(value),
		Valid:      true,
	}
}

func sqlTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{
		Time:  *value,
		Valid: true,
	}
}

func sqlInt32(value int) sql.NullInt32 {
	return sql.NullInt32{
		Int32: int32(value),
		Valid: true,
	}
}

func upsertProjectionEnterpriseHRISConnectorTx(
	ctx context.Context,
	tx *sql.Tx,
	item enterprise.HRISConnector,
) error {
	raw, err := marshalProjectionJSON(item)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`insert into mistypass_enterprise_hris_connectors (
			id,
			tenant_id,
			vendor,
			status,
			sync_strategy,
			credential_ref,
			webhook_secret_ref,
			last_sync_at,
			updated_by,
			created_at,
			updated_at,
			raw,
			synced_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
		on conflict (id) do update
		set tenant_id = excluded.tenant_id,
		    vendor = excluded.vendor,
		    status = excluded.status,
		    sync_strategy = excluded.sync_strategy,
		    credential_ref = excluded.credential_ref,
		    webhook_secret_ref = excluded.webhook_secret_ref,
		    last_sync_at = excluded.last_sync_at,
		    updated_by = excluded.updated_by,
		    created_at = excluded.created_at,
		    updated_at = excluded.updated_at,
		    raw = excluded.raw,
		    synced_at = now()`,
		item.ID,
		item.TenantID,
		item.Vendor,
		item.Status,
		item.SyncStrategy,
		sqlText(item.CredentialRef),
		sqlText(item.WebhookSecretRef),
		sqlTime(item.LastSyncAt),
		sqlText(item.UpdatedBy),
		item.CreatedAt,
		item.UpdatedAt,
		[]byte(raw),
	)
	return err
}

func upsertProjectionEnterpriseHRISWebhookReceiptTx(
	ctx context.Context,
	tx *sql.Tx,
	item enterprise.HRISWebhookReceipt,
) error {
	raw, err := marshalProjectionJSON(item)
	if err != nil {
		return err
	}
	headers, err := marshalProjectionNullableJSON(item.Headers)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`insert into mistypass_enterprise_hris_webhook_receipts (
			id,
			tenant_id,
			connector_id,
			vendor,
			event_type,
			request_id,
			content_type,
			headers,
			raw_payload,
			source_ip,
			status,
			attempt_count,
			last_error,
			received_at,
			last_attempt_at,
			processed_at,
			raw,
			synced_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, now())
		on conflict (id) do update
		set tenant_id = excluded.tenant_id,
		    connector_id = excluded.connector_id,
		    vendor = excluded.vendor,
		    event_type = excluded.event_type,
		    request_id = excluded.request_id,
		    content_type = excluded.content_type,
		    headers = excluded.headers,
		    raw_payload = excluded.raw_payload,
		    source_ip = excluded.source_ip,
		    status = excluded.status,
		    attempt_count = excluded.attempt_count,
		    last_error = excluded.last_error,
		    received_at = excluded.received_at,
		    last_attempt_at = excluded.last_attempt_at,
		    processed_at = excluded.processed_at,
		    raw = excluded.raw,
		    synced_at = now()`,
		item.ID,
		item.TenantID,
		item.ConnectorID,
		item.Vendor,
		sqlText(item.EventType),
		sqlText(item.RequestID),
		sqlText(item.ContentType),
		sqlRawJSON(headers),
		item.RawPayload,
		sqlText(item.SourceIP),
		item.Status,
		item.AttemptCount,
		sqlText(item.LastError),
		item.ReceivedAt,
		sqlTime(item.LastAttemptAt),
		sqlTime(item.ProcessedAt),
		[]byte(raw),
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
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				slog.Default().Error(
					"state tx rollback failed",
					"op", "apply_projection",
					"key", key,
					"err", err,
				)
			}
		}
	}()
	qtx := s.queries.WithTx(tx)

	switch key {
	case stateKeyTenant:
		var snapshot tenantStateSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		tenantIDs := make([]string, 0, len(snapshot.Tenants))
		for i := range snapshot.Tenants {
			next := snapshot.Tenants[i]
			tenantIDs = append(tenantIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionTenant(ctx, sqlcgen.UpsertProjectionTenantParams{
				ID:         next.ID,
				Name:       next.Name,
				TenantType: next.Type,
				HqRegion:   next.HQRegion,
				Status:     next.Status,
				CreatedAt:  next.CreatedAt,
				Raw:        []byte(raw),
			}); err != nil {
				return err
			}
		}
		if err := deleteProjectionRows(ctx, tx, projectionDeleteSet{
			table: "mistypass_tenants",
			ids:   tenantIDs,
		}); err != nil {
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
			next := snapshot.Buildings[i]
			buildingIDs = append(buildingIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionBuilding(ctx, sqlcgen.UpsertProjectionBuildingParams{
				ID:        next.ID,
				TenantID:  next.TenantID,
				Name:      next.Name,
				Address:   sqlText(next.Address),
				Region:    sqlText(next.Region),
				CreatedAt: next.CreatedAt,
				Raw:       []byte(raw),
			}); err != nil {
				return err
			}
		}
		for i := range snapshot.Floors {
			next := snapshot.Floors[i]
			floorIDs = append(floorIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionFloor(ctx, sqlcgen.UpsertProjectionFloorParams{
				ID:         next.ID,
				TenantID:   next.TenantID,
				BuildingID: next.BuildingID,
				Name:       next.Name,
				CreatedAt:  next.CreatedAt,
				Raw:        []byte(raw),
			}); err != nil {
				return err
			}
		}
		for i := range snapshot.Areas {
			next := snapshot.Areas[i]
			areaIDs = append(areaIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionArea(ctx, sqlcgen.UpsertProjectionAreaParams{
				ID:         next.ID,
				TenantID:   next.TenantID,
				BuildingID: next.BuildingID,
				FloorID:    next.FloorID,
				Name:       next.Name,
				CreatedAt:  next.CreatedAt,
				Raw:        []byte(raw),
			}); err != nil {
				return err
			}
		}
		for i := range snapshot.Doors {
			next := snapshot.Doors[i]
			doorIDs = append(doorIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionDoor(ctx, sqlcgen.UpsertProjectionDoorParams{
				ID:         next.ID,
				TenantID:   next.TenantID,
				BuildingID: next.BuildingID,
				FloorID:    next.FloorID,
				AreaID:     next.AreaID,
				Name:       next.Name,
				GatewayID:  sqlText(next.GatewayID),
				Kind:       next.Kind,
				Status:     next.Status,
				CreatedAt:  next.CreatedAt,
				Raw:        []byte(raw),
			}); err != nil {
				return err
			}
		}
		for i := range snapshot.DoorGroups {
			next := snapshot.DoorGroups[i]
			doorGroupIDs = append(doorGroupIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			doorIDs, err := marshalProjectionJSON(next.DoorIDs)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionDoorGroup(ctx, sqlcgen.UpsertProjectionDoorGroupParams{
				ID:        next.ID,
				TenantID:  next.TenantID,
				Name:      next.Name,
				DoorIds:   sqlRawJSON(doorIDs),
				CreatedAt: next.CreatedAt,
				Raw:       []byte(raw),
			}); err != nil {
				return err
			}
		}
		if err := deleteProjectionRows(
			ctx,
			tx,
			projectionDeleteSet{table: "mistypass_buildings", ids: buildingIDs},
			projectionDeleteSet{table: "mistypass_floors", ids: floorIDs},
			projectionDeleteSet{table: "mistypass_areas", ids: areaIDs},
			projectionDeleteSet{table: "mistypass_doors", ids: doorIDs},
			projectionDeleteSet{table: "mistypass_door_groups", ids: doorGroupIDs},
		); err != nil {
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
			next := snapshot.Users[i]
			userIDs = append(userIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			groupIDs, err := marshalProjectionJSON(next.GroupIDs)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionAccessUser(ctx, sqlcgen.UpsertProjectionAccessUserParams{
				ID:         next.ID,
				TenantID:   next.TenantID,
				BuildingID: sqlText(next.BuildingID),
				Name:       next.Name,
				Email:      next.Email,
				Role:       next.Role,
				Status:     next.Status,
				GroupIds:   sqlRawJSON(groupIDs),
				CreatedAt:  next.CreatedAt,
				Raw:        []byte(raw),
			}); err != nil {
				return err
			}
		}
		for i := range snapshot.UserGroups {
			next := snapshot.UserGroups[i]
			userGroupIDs = append(userGroupIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			members, err := marshalProjectionJSON(next.Members)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionAccessUserGroup(ctx, sqlcgen.UpsertProjectionAccessUserGroupParams{
				ID:          next.ID,
				TenantID:    next.TenantID,
				BuildingID:  sqlText(next.BuildingID),
				Name:        next.Name,
				Description: sqlText(next.Description),
				Members:     sqlRawJSON(members),
				CreatedAt:   next.CreatedAt,
				UpdatedAt:   next.UpdatedAt,
				Raw:         []byte(raw),
			}); err != nil {
				return err
			}
		}
		for i := range snapshot.Policies {
			next := snapshot.Policies[i]
			policyIDs = append(policyIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionAccessPolicy(ctx, sqlcgen.UpsertProjectionAccessPolicyParams{
				ID:         next.ID,
				TenantID:   next.TenantID,
				Name:       next.Name,
				ScopeType:  next.ScopeType,
				BuildingID: sqlText(next.BuildingID),
				AreaID:     sqlText(next.AreaID),
				DoorID:     sqlText(next.DoorID),
				Schedule:   sqlText(next.Schedule),
				Members:    int32(next.Members),
				Status:     next.Status,
				UpdatedAt:  next.UpdatedAt,
				Raw:        []byte(raw),
			}); err != nil {
				return err
			}
		}
		for i := range snapshot.TemporaryAccess {
			next := snapshot.TemporaryAccess[i]
			temporaryAccessIDs = append(temporaryAccessIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionTemporaryAccess(ctx, sqlcgen.UpsertProjectionTemporaryAccessParams{
				ID:                next.ID,
				TenantID:          next.TenantID,
				ScopeType:         next.ScopeType,
				BuildingID:        sqlText(next.BuildingID),
				AreaID:            sqlText(next.AreaID),
				DoorID:            sqlText(next.DoorID),
				DeliveryMethod:    next.DeliveryMethod,
				GranteeName:       next.GranteeName,
				GranteeEmail:      next.GranteeEmail,
				GranteePhone:      next.GranteePhone,
				ValidUntil:        next.ValidUntil,
				AuthorizedByEmail: sqlText(next.AuthorizedByEmail),
				AuthorizedByRole:  sqlText(next.AuthorizedByRole),
				AuthorizedAt:      next.AuthorizedAt,
				CreatedAt:         next.CreatedAt,
				Raw:               []byte(raw),
			}); err != nil {
				return err
			}
		}
		for i := range snapshot.VisitorPasses {
			next := snapshot.VisitorPasses[i]
			visitorPassIDs = append(visitorPassIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionVisitorPass(ctx, sqlcgen.UpsertProjectionVisitorPassParams{
				ID:             next.ID,
				TenantID:       next.TenantID,
				BuildingID:     sqlText(next.BuildingID),
				Host:           next.Host,
				Visitor:        next.Visitor,
				DeliveryMethod: next.DeliveryMethod,
				ExpiresAt:      next.ExpiresAt,
				CreatedAt:      next.CreatedAt,
				Raw:            []byte(raw),
			}); err != nil {
				return err
			}
		}
		if err := deleteProjectionRows(
			ctx,
			tx,
			projectionDeleteSet{table: "mistypass_access_users", ids: userIDs},
			projectionDeleteSet{table: "mistypass_access_user_groups", ids: userGroupIDs},
			projectionDeleteSet{table: "mistypass_access_policies", ids: policyIDs},
			projectionDeleteSet{table: "mistypass_temporary_access", ids: temporaryAccessIDs},
			projectionDeleteSet{table: "mistypass_visitor_passes", ids: visitorPassIDs},
		); err != nil {
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
			if err := qtx.UpsertProjectionGateway(ctx, sqlcgen.UpsertProjectionGatewayParams{
				ID:             snapshot.Gateways[i].ID,
				TenantID:       snapshot.Gateways[i].TenantID,
				SerialNumber:   snapshot.Gateways[i].SerialNumber,
				BuildingID:     sqlText(snapshot.Gateways[i].BuildingID),
				DeviceCapacity: int32(snapshot.Gateways[i].DeviceCapacity),
				Status:         snapshot.Gateways[i].Status,
				LastSeenAt:     snapshot.Gateways[i].LastSeenAt,
				Devices:        sqlRawJSON(string(devices)),
				BoundDoorIds:   sqlRawJSON(string(boundDoorIDs)),
				Raw:            []byte(raw),
			}); err != nil {
				return err
			}
			for d := range snapshot.Gateways[i].Devices {
				deviceIDs = append(deviceIDs, snapshot.Gateways[i].Devices[d].ID)
				deviceRaw, err := marshalProjectionJSON(snapshot.Gateways[i].Devices[d])
				if err != nil {
					return err
				}
				rs485Config, err := marshalProjectionNullableJSON(snapshot.Gateways[i].Devices[d].RS485Config)
				if err != nil {
					return err
				}
				rs485Health, err := marshalProjectionNullableJSON(snapshot.Gateways[i].Devices[d].RS485Health)
				if err != nil {
					return err
				}
				if err := qtx.UpsertProjectionGatewayDevice(ctx, sqlcgen.UpsertProjectionGatewayDeviceParams{
					ID:           snapshot.Gateways[i].Devices[d].ID,
					GatewayID:    snapshot.Gateways[i].ID,
					TenantID:     snapshot.Gateways[i].TenantID,
					SerialNumber: snapshot.Gateways[i].Devices[d].SerialNumber,
					Kind:         snapshot.Gateways[i].Devices[d].Kind,
					Source:       snapshot.Gateways[i].Devices[d].Source,
					Protocol:     sqlText(snapshot.Gateways[i].Devices[d].Protocol),
					Rs485Config:  sqlRawJSON(rs485Config),
					Rs485Health:  sqlRawJSON(rs485Health),
					Status:       snapshot.Gateways[i].Devices[d].Status,
					LastSeenAt:   snapshot.Gateways[i].Devices[d].LastSeenAt,
					Raw:          []byte(deviceRaw),
				}); err != nil {
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
			if err := qtx.UpsertProjectionGatewaySerialInventory(ctx, sqlcgen.UpsertProjectionGatewaySerialInventoryParams{
				ID:                snapshot.SerialInventory[i].ID,
				TenantID:          snapshot.SerialInventory[i].TenantID,
				SerialNumber:      snapshot.SerialInventory[i].SerialNumber,
				ProductType:       snapshot.SerialInventory[i].ProductType,
				Status:            snapshot.SerialInventory[i].Status,
				BatchCode:         sqlText(snapshot.SerialInventory[i].BatchCode),
				Source:            sqlText(snapshot.SerialInventory[i].Source),
				ConsumedGatewayID: sqlText(snapshot.SerialInventory[i].ConsumedGatewayID),
				ConsumedAt:        sqlTime(snapshot.SerialInventory[i].ConsumedAt),
				CreatedAt:         snapshot.SerialInventory[i].CreatedAt,
				UpdatedAt:         snapshot.SerialInventory[i].UpdatedAt,
				Raw:               []byte(raw),
			}); err != nil {
				return err
			}
		}
		if err := deleteProjectionRows(
			ctx,
			tx,
			projectionDeleteSet{table: "mistypass_gateways", ids: gatewayIDs},
			projectionDeleteSet{table: "mistypass_gateway_devices", ids: deviceIDs},
			projectionDeleteSet{table: "mistypass_gateway_serial_inventory", ids: serialInventoryIDs},
		); err != nil {
			return err
		}
	case stateKeyEnterprise:
		var snapshot enterpriseStateSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		domainMappingIDs := make([]string, 0, len(snapshot.DomainMappings))
		hrisConnectorIDs := make([]string, 0, len(snapshot.HRISConnectors))
		hrisWebhookReceiptIDs := make([]string, 0, len(snapshot.HRISWebhookReceipts))
		idpConfigIDs := make([]string, 0, len(snapshot.IDPConfigs))
		employeeIDs := make([]string, 0, len(snapshot.Employees))
		syncJobIDs := make([]string, 0, len(snapshot.SyncJobs))
		for i := range snapshot.DomainMappings {
			next := snapshot.DomainMappings[i]
			domainMappingIDs = append(domainMappingIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionEnterpriseDomainMapping(ctx, sqlcgen.UpsertProjectionEnterpriseDomainMappingParams{
				ID:        next.ID,
				TenantID:  next.TenantID,
				Domain:    next.Domain,
				Status:    next.Status,
				CreatedAt: next.CreatedAt,
				UpdatedAt: next.UpdatedAt,
				Raw:       []byte(raw),
			}); err != nil {
				return err
			}
		}
		for i := range snapshot.HRISConnectors {
			next := snapshot.HRISConnectors[i]
			hrisConnectorIDs = append(hrisConnectorIDs, next.ID)
			if err := upsertProjectionEnterpriseHRISConnectorTx(ctx, tx, next); err != nil {
				return err
			}
		}
		for i := range snapshot.HRISWebhookReceipts {
			next := snapshot.HRISWebhookReceipts[i]
			hrisWebhookReceiptIDs = append(hrisWebhookReceiptIDs, next.ID)
			if err := upsertProjectionEnterpriseHRISWebhookReceiptTx(ctx, tx, next); err != nil {
				return err
			}
		}
		for _, config := range snapshot.IDPConfigs {
			idpConfigIDs = append(idpConfigIDs, config.ID)
			raw, err := marshalProjectionJSON(config)
			if err != nil {
				return err
			}
			scopes, err := marshalProjectionJSON(config.Scopes)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionEnterpriseIDPConfig(ctx, sqlcgen.UpsertProjectionEnterpriseIDPConfigParams{
				ID:        config.ID,
				TenantID:  config.TenantID,
				Provider:  config.Provider,
				IssuerUrl: config.IssuerURL,
				ClientID:  config.ClientID,
				Status:    config.Status,
				SyncMode:  config.SyncMode,
				Scopes:    sqlRawJSON(scopes),
				UpdatedBy: sqlText(config.UpdatedBy),
				CreatedAt: config.CreatedAt,
				UpdatedAt: config.UpdatedAt,
				Raw:       []byte(raw),
			}); err != nil {
				return err
			}
		}
		for i := range snapshot.Employees {
			next := snapshot.Employees[i]
			employeeIDs = append(employeeIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			groupIDs, err := marshalProjectionJSON(next.GroupIDs)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionEnterpriseEmployee(ctx, sqlcgen.UpsertProjectionEnterpriseEmployeeParams{
				ID:           next.ID,
				TenantID:     next.TenantID,
				ExternalID:   sqlText(next.ExternalID),
				Email:        next.Email,
				FullName:     next.FullName,
				Department:   sqlText(next.Department),
				JobTitle:     sqlText(next.JobTitle),
				Location:     sqlText(next.Location),
				AccessRole:   sqlText(next.AccessRole),
				BuildingID:   sqlText(next.BuildingID),
				GroupIds:     sqlRawJSON(groupIDs),
				Status:       sqlText(next.Status),
				Source:       sqlText(next.Source),
				LastSyncedAt: next.LastSyncedAt,
				Raw:          []byte(raw),
			}); err != nil {
				return err
			}
		}
		for i := range snapshot.SyncJobs {
			next := snapshot.SyncJobs[i]
			syncJobIDs = append(syncJobIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionEnterpriseSyncJob(ctx, sqlcgen.UpsertProjectionEnterpriseSyncJobParams{
				ID:          next.ID,
				TenantID:    next.TenantID,
				Source:      sqlText(next.Source),
				Status:      sqlText(next.Status),
				Total:       sqlInt32(next.Total),
				Created:     sqlInt32(next.Created),
				Updated:     sqlInt32(next.Updated),
				Deactivated: sqlInt32(next.Deactivated),
				Rejected:    sqlInt32(next.Rejected),
				Actor:       sqlText(next.Actor),
				StartedAt:   next.StartedAt,
				EndedAt:     next.EndedAt,
				Raw:         []byte(raw),
			}); err != nil {
				return err
			}
		}
		if err := deleteProjectionRows(
			ctx,
			tx,
			projectionDeleteSet{table: "mistypass_enterprise_domain_mappings", ids: domainMappingIDs},
			projectionDeleteSet{table: "mistypass_enterprise_hris_connectors", ids: hrisConnectorIDs},
			projectionDeleteSet{table: "mistypass_enterprise_hris_webhook_receipts", ids: hrisWebhookReceiptIDs},
			projectionDeleteSet{table: "mistypass_enterprise_idp_configs", ids: idpConfigIDs},
			projectionDeleteSet{table: "mistypass_enterprise_employees", ids: employeeIDs},
			projectionDeleteSet{table: "mistypass_enterprise_sync_jobs", ids: syncJobIDs},
		); err != nil {
			return err
		}
	case stateKeyEvent:
		var snapshot eventStateSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		accessEventIDs := make([]string, 0, len(snapshot.AccessEvents))
		for i := range snapshot.AccessEvents {
			next := snapshot.AccessEvents[i]
			accessEventIDs = append(accessEventIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionAccessEvent(ctx, sqlcgen.UpsertProjectionAccessEventParams{
				ID:         next.ID,
				TenantID:   next.TenantID,
				BuildingID: sqlText(next.BuildingID),
				AreaID:     sqlText(next.AreaID),
				EventType:  sqlText(next.Type),
				Actor:      sqlText(next.Actor),
				DoorID:     sqlText(next.DoorID),
				GatewayID:  sqlText(next.GatewayID),
				Result:     sqlText(next.Result),
				At:         next.At,
				Raw:        []byte(raw),
			}); err != nil {
				return err
			}
		}
		deviceEventIDs := make([]string, 0, len(snapshot.DeviceEvents))
		for i := range snapshot.DeviceEvents {
			next := snapshot.DeviceEvents[i]
			deviceEventIDs = append(deviceEventIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionDeviceEvent(ctx, sqlcgen.UpsertProjectionDeviceEventParams{
				ID:         next.ID,
				TenantID:   next.TenantID,
				BuildingID: sqlText(next.BuildingID),
				EventType:  sqlText(next.Type),
				GatewayID:  sqlText(next.GatewayID),
				Detail:     sqlText(next.Detail),
				Result:     sqlText(next.Result),
				At:         next.At,
				Raw:        []byte(raw),
			}); err != nil {
				return err
			}
		}
		if err := deleteProjectionRows(
			ctx,
			tx,
			projectionDeleteSet{table: "mistypass_access_events", ids: accessEventIDs},
			projectionDeleteSet{table: "mistypass_device_events", ids: deviceEventIDs},
		); err != nil {
			return err
		}
	case stateKeyAlarm:
		var snapshot alarmStateSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		alarmIDs := make([]string, 0, len(snapshot.Alarms))
		for i := range snapshot.Alarms {
			next := snapshot.Alarms[i]
			alarmIDs = append(alarmIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionAlarm(ctx, sqlcgen.UpsertProjectionAlarmParams{
				ID:         next.ID,
				TenantID:   next.TenantID,
				BuildingID: sqlText(next.BuildingID),
				AreaID:     sqlText(next.AreaID),
				DoorID:     sqlText(next.DoorID),
				AlarmType:  sqlText(next.Type),
				Severity:   sqlText(next.Severity),
				Location:   sqlText(next.Location),
				Status:     sqlText(next.Status),
				CreatedAt:  next.CreatedAt,
				Raw:        []byte(raw),
			}); err != nil {
				return err
			}
		}
		if err := deleteProjectionRows(ctx, tx, projectionDeleteSet{
			table: "mistypass_alarms",
			ids:   alarmIDs,
		}); err != nil {
			return err
		}
	case stateKeyAudit:
		var snapshot auditStateSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		auditLogIDs := make([]string, 0, len(snapshot.Logs))
		for i := range snapshot.Logs {
			next := snapshot.Logs[i]
			auditLogIDs = append(auditLogIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionAuditLog(ctx, sqlcgen.UpsertProjectionAuditLogParams{
				ID:       next.ID,
				TenantID: next.TenantID,
				Actor:    sqlText(next.Actor),
				Role:     sqlText(next.Role),
				Action:   sqlText(next.Action),
				Target:   sqlText(next.Target),
				Source:   sqlText(next.Source),
				At:       next.At,
				Raw:      []byte(raw),
			}); err != nil {
				return err
			}
		}
		if err := deleteProjectionRows(ctx, tx, projectionDeleteSet{
			table: "mistypass_audit_logs",
			ids:   auditLogIDs,
		}); err != nil {
			return err
		}
	case stateKeyWallet:
		var snapshot walletStateSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		configIDs := make([]string, 0, 1)
		var (
			templateIDs []string
			passIDs     []string
			jobIDs      []string
			auditLogIDs []string
		)
		if snapshot.Config != nil {
			configIDs = append(configIDs, snapshot.Config.ID)
			raw, err := marshalProjectionJSON(snapshot.Config)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionWalletConfig(ctx, sqlcgen.UpsertProjectionWalletConfigParams{
				ID:                  snapshot.Config.ID,
				TenantID:            snapshot.Config.TenantID,
				Provider:            snapshot.Config.Provider,
				IssuerID:            snapshot.Config.IssuerID,
				ServiceAccountEmail: snapshot.Config.ServiceAccountEmail,
				KeyRef:              snapshot.Config.KeyRef,
				Status:              snapshot.Config.Status,
				CreatedAt:           snapshot.Config.CreatedAt,
				UpdatedAt:           snapshot.Config.UpdatedAt,
				Raw:                 []byte(raw),
			}); err != nil {
				return err
			}
		}
		templateIDs = make([]string, 0, len(snapshot.Templates))
		for i := range snapshot.Templates {
			next := snapshot.Templates[i]
			templateIDs = append(templateIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			styleConfig, err := marshalProjectionJSON(next.StyleConfig)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionWalletTemplate(ctx, sqlcgen.UpsertProjectionWalletTemplateParams{
				ID:          next.ID,
				TenantID:    next.TenantID,
				Provider:    next.Provider,
				PassType:    next.PassType,
				ClassID:     next.ClassID,
				Name:        next.Name,
				Status:      next.Status,
				StyleConfig: sqlRawJSON(styleConfig),
				CreatedAt:   next.CreatedAt,
				UpdatedAt:   next.UpdatedAt,
				Raw:         []byte(raw),
			}); err != nil {
				return err
			}
		}
		passIDs = make([]string, 0, len(snapshot.Passes))
		for i := range snapshot.Passes {
			next := snapshot.Passes[i]
			passIDs = append(passIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionWalletPass(ctx, sqlcgen.UpsertProjectionWalletPassParams{
				ID:          next.ID,
				TenantID:    next.TenantID,
				Provider:    next.Provider,
				TemplateID:  next.TemplateID,
				TargetType:  next.TargetType,
				TargetID:    next.TargetID,
				ObjectID:    next.ObjectID,
				Status:      next.Status,
				SaveLink:    next.SaveLink,
				ExpiresAt:   sqlText(next.ExpiresAt),
				IssuedAt:    next.IssuedAt,
				ActivatedAt: sqlTime(next.ActivatedAt),
				RevokedAt:   sqlTime(next.RevokedAt),
				CreatedBy:   sqlText(next.CreatedBy),
				UpdatedBy:   sqlText(next.UpdatedBy),
				CreatedAt:   next.CreatedAt,
				UpdatedAt:   next.UpdatedAt,
				Raw:         []byte(raw),
			}); err != nil {
				return err
			}
		}
		jobIDs = make([]string, 0, len(snapshot.Jobs))
		for i := range snapshot.Jobs {
			next := snapshot.Jobs[i]
			jobIDs = append(jobIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionWalletJob(ctx, sqlcgen.UpsertProjectionWalletJobParams{
				ID:           next.ID,
				TenantID:     next.TenantID,
				Provider:     sqlText(next.Provider),
				BatchID:      sqlText(next.BatchID),
				TemplateID:   sqlText(next.TemplateID),
				TargetType:   sqlText(next.TargetType),
				TargetID:     sqlText(next.TargetID),
				ExpiresAt:    sqlText(next.ExpiresAt),
				PassID:       sqlText(next.PassID),
				Status:       sqlText(next.Status),
				RetryCount:   sqlInt32(next.RetryCount),
				ErrorCode:    sqlText(next.ErrorCode),
				ErrorMessage: sqlText(next.ErrorMessage),
				CreatedAt:    next.CreatedAt,
				UpdatedAt:    next.UpdatedAt,
				Raw:          []byte(raw),
			}); err != nil {
				return err
			}
		}
		auditLogIDs = make([]string, 0, len(snapshot.AuditLogs))
		for i := range snapshot.AuditLogs {
			next := snapshot.AuditLogs[i]
			auditLogIDs = append(auditLogIDs, next.ID)
			raw, err := marshalProjectionJSON(next)
			if err != nil {
				return err
			}
			if err := qtx.UpsertProjectionWalletAuditLog(ctx, sqlcgen.UpsertProjectionWalletAuditLogParams{
				ID:       next.ID,
				TenantID: next.TenantID,
				Action:   sqlText(next.Action),
				Actor:    sqlText(next.Actor),
				TargetID: sqlText(next.TargetID),
				Result:   sqlText(next.Result),
				At:       next.At,
				Raw:      []byte(raw),
			}); err != nil {
				return err
			}
		}
		if err := deleteProjectionRows(
			ctx,
			tx,
			projectionDeleteSet{table: "mistypass_wallet_configs", ids: configIDs},
			projectionDeleteSet{table: "mistypass_wallet_templates", ids: templateIDs},
			projectionDeleteSet{table: "mistypass_wallet_passes", ids: passIDs},
			projectionDeleteSet{table: "mistypass_wallet_jobs", ids: jobIDs},
			projectionDeleteSet{table: "mistypass_wallet_audit_logs", ids: auditLogIDs},
		); err != nil {
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
	changeID, err := s.queries.WithTx(tx).InsertStateChange(ctx, sqlcgen.InsertStateChangeParams{
		StateKey:    nextKey,
		ChangeType:  changeTypeSnapshotSaved,
		PayloadHash: nextHash,
		Payload:     payload,
	})
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

	var (
		records []sqlcgen.MistypassChangeLog
		err     error
	)
	if nextKey == "" {
		records, err = s.queries.ListStateChangesAll(ctx, int32(nextLimit))
	} else {
		records, err = s.queries.ListStateChangesByKey(ctx, sqlcgen.ListStateChangesByKeyParams{
			StateKey: nextKey,
			Limit:    int32(nextLimit),
		})
	}
	if err != nil {
		return nil, err
	}

	items := make([]StateChangeRecord, 0, len(records))
	for i := range records {
		record := StateChangeRecord{
			ID:          records[i].ID,
			StateKey:    records[i].StateKey,
			ChangeType:  records[i].ChangeType,
			PayloadHash: records[i].PayloadHash,
			Payload:     append([]byte(nil), records[i].Payload...),
			CreatedAt:   records[i].CreatedAt,
		}
		items = append(items, record)
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

	rows, err := s.queries.ListReplayPayloadsByKeyFromID(ctx, sqlcgen.ListReplayPayloadsByKeyFromIDParams{
		StateKey: nextKey,
		ID:       nextFromID,
		Limit:    int32(nextLimit),
	})
	if err != nil {
		return ReplayStateChangesResult{}, err
	}

	result := ReplayStateChangesResult{}
	for i := range rows {
		if err := s.projectStatePayload(ctx, nextKey, rows[i].Payload); err != nil {
			return ReplayStateChangesResult{}, fmt.Errorf("replay projection failed at change_id=%d: %w", rows[i].ID, err)
		}
		result.Applied++
		result.LastChangeID = rows[i].ID
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

	var (
		rows []sqlcgen.MistypassChangeReplayCheckpoint
		err  error
	)
	if nextKey == "" {
		rows, err = s.queries.ListReplayCheckpointsAll(ctx, int32(nextLimit))
	} else {
		rows, err = s.queries.ListReplayCheckpointsByKey(ctx, sqlcgen.ListReplayCheckpointsByKeyParams{
			StateKey: nextKey,
			Limit:    int32(nextLimit),
		})
	}
	if err != nil {
		return nil, err
	}

	items := make([]ReplayCheckpoint, 0, len(rows))
	for i := range rows {
		items = append(items, ReplayCheckpoint{
			StateKey:     rows[i].StateKey,
			LastChangeID: rows[i].LastChangeID,
			UpdatedAt:    rows[i].UpdatedAt,
		})
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
	row, err := s.queries.GetReplayCheckpointByKey(ctx, stateKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReplayCheckpoint{}, false, nil
		}
		return ReplayCheckpoint{}, false, err
	}
	return ReplayCheckpoint{
		StateKey:     row.StateKey,
		LastChangeID: row.LastChangeID,
		UpdatedAt:    row.UpdatedAt,
	}, true, nil
}

func (s *PostgresStore) upsertReplayCheckpoint(ctx context.Context, stateKey string, lastChangeID int64) (ReplayCheckpoint, error) {
	nextLastChangeID := lastChangeID
	if nextLastChangeID < 0 {
		nextLastChangeID = 0
	}

	row, err := s.queries.UpsertReplayCheckpoint(ctx, sqlcgen.UpsertReplayCheckpointParams{
		StateKey:     stateKey,
		LastChangeID: nextLastChangeID,
	})
	if err != nil {
		return ReplayCheckpoint{}, err
	}
	return ReplayCheckpoint{
		StateKey:     row.StateKey,
		LastChangeID: row.LastChangeID,
		UpdatedAt:    row.UpdatedAt,
	}, nil
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
