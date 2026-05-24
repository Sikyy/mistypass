# MistyPass Starter Workspace

This repository now includes a minimal runnable scaffold for:

- `api`: Go-based backend API for auth, tenant, and gateway basics.
- `web-admin`: React + TypeScript admin UI using official `Tailwind + shadcn/ui registry`.

## 1. Project layout

```text
MistyPass/
  api/
    cmd/api/main.go
    internal/
  web-admin/
    src/
```

## 2. Run backend API

Requirements:

- Go 1.22+

Commands:

```bash
cd api
go run ./cmd/api
```

Backend hot-reload (no extra dependency):

```bash
cd api
./scripts/dev-hot.sh
```

Optional PostgreSQL state persistence:

```bash
cd api
DATABASE_URL='postgres://user:pass@localhost:5432/mistypass?sslmode=disable' go run ./cmd/api
```

Optional Redis / Dragonfly volatile store (session + revoked token + rate limit):

```bash
cd api
REDIS_ADDR='127.0.0.1:6379' REDIS_KEY_PREFIX='mistypass' go run ./cmd/api
```

Docker Compose one-command dev stack (API + PostgreSQL + Redis + EMQX):

```bash
docker compose up -d --build
docker compose logs -f api
```

The default Compose stack now binds service ports to `127.0.0.1`, disables demo users unless explicitly enabled, and replaces public/default infra credentials with local-only passwords. Override these via your shell or a repo-root `.env` file when needed:

- `POSTGRES_PASSWORD`
- `REDIS_PASSWORD`
- `EMQX_DASHBOARD_USERNAME`
- `EMQX_DASHBOARD_PASSWORD`
- `APP_ENV` (default in Compose: `development`; use `staging` for the Mac mini staging API)
- `GATEWAY_BOOTSTRAP_TOKEN`
- `ENABLE_DEMO_USERS` (default in Compose: `false`)
- `JWT_SECRET` and `HRIS_VAULT_MASTER_KEY` (recommended if you need stable auth tokens or HRIS secret decryption across container restarts)

Stop and clean all local data volumes:

```bash
docker compose down -v
```

Optional TLS termination (Caddy reverse proxy, production-ready entrypoint):

```bash
MISTYPASS_DOMAIN=api.example.com docker compose -f docker-compose.yml -f docker-compose.tls.yml up -d --build
```

Notes:

- Caddy terminates TLS on `:443` and reverse proxies to `api:8080`.
- For public domains, Caddy will auto-manage certificates.
- For local development, omit `MISTYPASS_DOMAIN` and Caddy will use `localhost`.

Default local ports:

- All host ports in the default Compose stack are bound to `127.0.0.1` only.
- API: `http://localhost:8080`
- PostgreSQL: `localhost:5432`
- PgBouncer: `localhost:6432` (API container defaults to `pgbouncer:5432`)
- Redis: `localhost:6379`
- EMQX MQTT: `localhost:1883`
- EMQX Dashboard: `http://localhost:18083` (`EMQX_DASHBOARD_USERNAME` / `EMQX_DASHBOARD_PASSWORD`)
- NATS: `localhost:4222` (monitoring: `http://localhost:8222`)

Optional env:

- `DATABASE_AUTO_MIGRATE` (default: `true`) auto-creates snapshot table `mistypass` plus projection tables `mistypass_*`, and auto-migrates legacy `app_state` rows if present.
- `MQTT_ENABLED` (default: `false`)
- `MQTT_BROKER_URL` (default: `tcp://localhost:1883`)
- `MQTT_TOPIC_PREFIX` (default: `mistypass`)
- `NATS_ENABLED` (default: `false`)
- `NATS_SERVER_URL` (default: `nats://localhost:4222`)
- `NATS_SUBJECT_PREFIX` (default: `mistypass`)
- `AUTH_ADMIN_MFA_REQUIRED` (default: `false`)
- `EXTERNAL_AUTH_ENABLED` (default: `false`)
- `EXTERNAL_AUTH_PROVIDER` (default: `generic_oidc`)
- `EXTERNAL_AUTH_USERINFO_URL` (required when external auth enabled)

Event table partitioning (pg_partman, monthly):

- The runtime schema now supports partition-friendly event upserts (`ON CONFLICT (id, at)`).
- To migrate existing `mistypass_access_events` / `mistypass_device_events` and enable pg_partman maintenance, run:

```bash
psql "$DATABASE_URL" -f deploy/postgres/event-partitioning-partman.sql
```

- For Docker Compose local stack:

```bash
cat deploy/postgres/event-partitioning-partman.sql | docker compose exec -T postgres psql -U postgres -d mistypass
```

Notes:

- `pg_partman` must be available in the PostgreSQL instance before running the script.
- The script configures monthly partitions with 6-month retention.

Default server:

- `http://localhost:8080`
- Health check: `GET /healthz`

MVP endpoints (Chi Router):

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/external/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/me`
- `POST /api/v1/enterprise/tenant/resolve`
- `POST /api/v1/enterprise/auth/start`
- `POST /api/v1/enterprise/auth/exchange`
- `POST /api/v1/enterprise/auth/logout`
- `GET /api/v1/enterprise/auth/oidc/callback`
- `POST /api/v1/enterprise/auth/saml/callback`
- `POST /api/v1/enterprise/jit-provision-approvals/external-sync/callback`
- `GET /api/v1/auth/users/{userID}/building-scope`
- `PUT /api/v1/auth/users/{userID}/building-scope`
- `GET /api/v1/auth/mfa/admin/status`
- `POST /api/v1/auth/mfa/admin/setup`
- `POST /api/v1/auth/mfa/admin/enable`
- `POST /api/v1/auth/mfa/admin/disable`
- `POST /api/v1/app/auth/login`
- `POST /api/v1/app/auth/refresh`
- `GET /api/v1/app/me`
- `GET /api/v1/app/credentials`
- `GET /api/v1/app/access/doors`
- `GET /api/v1/app/access/ble-token`
- `GET /api/v1/app/access/logs`
- `POST /api/v1/app/visitor-passes`
- `POST /api/v1/gateway/register`
- `POST /api/v1/gateway/activate`
- `POST /api/v1/gateway/heartbeat`
- `POST /api/v1/gateway/status`
- `POST /api/v1/gateway/config/pull`
- `POST /api/v1/gateway/config/applied`
- `POST /api/v1/gateway/events/access`
- `POST /api/v1/gateway/events/device`
- `POST /api/v1/gateway/events/batch`
- `POST /api/v1/gateway/events/checkpoint`
- `GET /api/v1/tenants`
- `POST /api/v1/tenants`
- `PATCH /api/v1/tenants/{tenantID}/status`
- `GET /api/v1/tenants/{tenantID}/topology`
- `GET /api/v1/buildings`
- `POST /api/v1/buildings`
- `GET /api/v1/floors`
- `POST /api/v1/floors`
- `GET /api/v1/areas`
- `POST /api/v1/areas`
- `GET /api/v1/doors`
- `POST /api/v1/doors`
- `GET /api/v1/door-groups`
- `POST /api/v1/door-groups`
- `GET /api/v1/gateways`
- `PATCH /api/v1/gateways/{gatewayID}/status`
- `GET /api/v1/gateways/cert-revocations`
- `POST /api/v1/gateways/cert-revocations`
- `DELETE /api/v1/gateways/cert-revocations/{serialNumber}`
- `GET /api/v1/gateways/serial-inventory`
- `POST /api/v1/gateways/serial-inventory/import`
- `POST /api/v1/gateways/serial-inventory/import-csv`
- `PATCH /api/v1/gateways/serial-inventory/batch-status`
- `PATCH /api/v1/gateways/serial-inventory/{serialNumber}/status`
- `GET /api/v1/gateways/serial-inventory/export-csv`
- `POST /api/v1/gateways/register`
- `POST /api/v1/gateways/{gatewayID}/bind-door`
- `POST /api/v1/gateways/{gatewayID}/unbind-door`
- `POST /api/v1/gateways/{gatewayID}/devices`
- `POST /api/v1/gateways/{gatewayID}/devices/{deviceID}/rs485/telemetry`
- `POST /api/v1/gateways/{gatewayID}/devices/probe-legacy`
- `POST /api/v1/gateways/{gatewayID}/config/publish`
- `POST /api/v1/gateways/{gatewayID}/reboot`
- `POST /api/v1/gateways/{gatewayID}/ota/tasks`
- `GET /api/v1/gateways/{gatewayID}/ota/tasks`
- `PATCH /api/v1/gateways/{gatewayID}/ota/tasks/{taskID}/status`
- `GET /api/v1/gateways/{gatewayID}/events/checkpoint`
- `GET /api/v1/gateways/{gatewayID}/mqtt/bootstrap`
- `GET /api/v1/gateways/events/checkpoint/summary`
- `GET /api/v1/access-policies`
- `POST /api/v1/access-policies`
- `PATCH /api/v1/access-policies/{policyID}`
- `GET /api/v1/users`
- `POST /api/v1/users`
- `GET /api/v1/user-groups`
- `POST /api/v1/user-groups`
- `PATCH /api/v1/user-groups/{groupID}`
- `GET /api/v1/temporary-access`
- `POST /api/v1/temporary-access`
- `GET /api/v1/visitor-passes`
- `POST /api/v1/visitor-passes`
- `GET /api/v1/events/access`
- `GET /api/v1/events/device`
- `GET /api/v1/events/stream`
- `GET /api/v1/alarms`
- `GET /api/v1/alarms/stream`
- `PATCH /api/v1/alarms/{alarmID}/status`
- `GET /api/v1/audit-logs`
- `GET /api/v1/audit/webhook/config`
- `PUT /api/v1/audit/webhook/config`
- `GET /api/v1/audit/webhook/deliveries`
- `POST /api/v1/audit/webhook/dispatch`
- `GET /api/v1/state/change-log` (super_admin)
- `POST /api/v1/state/change-log/replay` (super_admin)
- `GET /api/v1/state/change-log/checkpoints` (super_admin)
- `POST /api/v1/state/change-log/replay/checkpoint` (super_admin)
- `GET /api/v1/wallet/google/config`
- `PUT /api/v1/wallet/google/config`
- `POST /api/v1/wallet/google/config/validate`
- `GET /api/v1/wallet/templates`
- `POST /api/v1/wallet/templates`
- `PATCH /api/v1/wallet/templates/{templateID}/status`
- `POST /api/v1/wallet/passes/issue`
- `POST /api/v1/wallet/passes/issue-batch`
- `GET /api/v1/wallet/passes`
- `GET /api/v1/wallet/passes/{passID}`
- `GET /api/v1/wallet/passes/{passID}/save-link`
- `PATCH /api/v1/wallet/passes/{passID}/suspend`
- `PATCH /api/v1/wallet/passes/{passID}/activate`
- `PATCH /api/v1/wallet/passes/{passID}/revoke`
- `GET /api/v1/wallet/jobs`
- `GET /api/v1/wallet/jobs/{jobID}`
- `POST /api/v1/wallet/jobs/{jobID}/retry`
- `GET /api/v1/wallet/audit-logs`
- `GET /api/v1/enterprise/domain-mappings`
- `POST /api/v1/enterprise/domain-mappings`
- `PATCH /api/v1/enterprise/domain-mappings/{mappingID}/status`
- `GET /api/v1/enterprise/idp-config`
- `PUT /api/v1/enterprise/idp-config`
- `POST /api/v1/enterprise/idp-config/validate`
- `GET /api/v1/enterprise/employees`
- `GET /api/v1/enterprise/jit-provision-approvals`
- `POST /api/v1/enterprise/jit-provision-approvals/{approvalID}/review`
- `GET /api/v1/enterprise/jit-provision-approvals/external-sync-pending`
- `POST /api/v1/enterprise/jit-provision-approvals/{approvalID}/external-sync`
- `POST /api/v1/enterprise/employees/sync`
- `POST /api/v1/enterprise/employees/sync/reconcile`
- `GET /api/v1/enterprise/sync-requests`
- `GET /api/v1/enterprise/sync-worker-alerts`
- `GET /api/v1/enterprise/sync-worker-alerts/summary`
- `POST /api/v1/enterprise/sync-requests/reconcile-pending`
- `GET /api/v1/enterprise/sync-jobs`

## 3. Run web admin

Requirements:

- Node.js 20+

Commands:

```bash
cd web-admin
npm install
npm run dev
```

Default UI URL:

- `http://localhost:5173`

Admin routes:

- `/dashboard`
- `/tenants`
- `/spaces`
- `/access`
- `/gateways`
- `/events`
- `/alarms`

Optional API base URL:

```bash
VITE_API_BASE_URL=http://localhost:8080 npm run dev
```

## 4. Login behavior

The backend now issues signed access/refresh tokens and enforces role/tenant scope from token claims.
Docker Compose now starts with demo users disabled by default; only set `ENABLE_DEMO_USERS=true` for local smoke tests that need seeded accounts.

Suggested test credentials (`ENABLE_DEMO_USERS=true` only):

- `superadmin@mistypass.local` / `admin123` (`super_admin`, cross-tenant)
- `tenant.admin@sudirman.co` / `admin123` (`tenant_admin`, `tenant_demo_jakarta`)
- `building.admin.sudirman@mistypass.local` / `admin123` (`building_admin`, `tenant_demo_jakarta`, `building_demo_001`)
- `ops.jkt.01@mistypass.local` / `admin123` (`operator`, `tenant_demo_jakarta`)
- `tenant.admin@factory.local` / `admin123` (`tenant_admin`, `tenant_demo_factory`)
- `resident.jakarta@mistypass.local` / `admin123` (`resident`, `tenant_demo_jakarta`, App login)

Auth/session env vars:

- `APP_ENV` (`development` | `production`, default: `development`)
- `ENABLE_DEMO_USERS` (default: `false`; only enabled when explicitly set to `true`)
- `JWT_SECRET` (required when `APP_ENV=production`; non-production uses a CSPRNG-generated ephemeral in-memory secret if omitted)
- `HRIS_VAULT_MASTER_KEY` (required when `APP_ENV=production`; recommended in non-production if HRIS secrets must remain decryptable after restart)
- `JWT_ISSUER` (default: `mistypass-api`)
- `JWT_ACCESS_TTL` (seconds or duration, default: `1h`)
- `JWT_REFRESH_TTL` (seconds or duration, default: `168h`)
- `AUTH_ADMIN_MFA_REQUIRED` (default: `false`; when `true`, `super_admin`/`tenant_admin` login requires enrolled TOTP MFA)
- `EXTERNAL_AUTH_ENABLED` (default: `false`; enables `/api/v1/auth/external/login`)
- `EXTERNAL_AUTH_PROVIDER` (default: `generic_oidc`; supports `generic_oidc|casdoor|ory_kratos`)
- `EXTERNAL_AUTH_USERINFO_URL` (required when external auth enabled; userinfo/whoami endpoint)
- `GATEWAY_BOOTSTRAP_TOKEN` (required in production; used by device-side `POST /api/v1/gateway/register` bootstrap authentication)
- `GATEWAY_REQUIRE_REQUEST_NONCE` (default: `false`; set `true` after all gateway agents send `X-Request-Nonce` + `X-Request-Timestamp` on gateway HTTP device requests)
- `GATEWAY_WS_MAX_SESSION_TTL` (duration, default: `6h`; server closes gateway WebSocket sessions at expiry and agents reconnect)
- `GATEWAY_MTLS_ADDR` (optional; when set, starts a gateway-only HTTPS listener that requires verified client certificates, e.g. `:9443`)
- `GATEWAY_MTLS_SERVER_CERT_PEM` / `GATEWAY_MTLS_SERVER_KEY_PEM` (required when `GATEWAY_MTLS_ADDR` is set; server certificate/key for the gateway mTLS listener)
- `GATEWAY_CA_CERT_PEM` / `GATEWAY_CA_KEY_PEM` (required when `GATEWAY_MTLS_ADDR` is set; CA used to sign and verify gateway client certificates)
- `GATEWAY_MTLS_CERT_LIFETIME` (duration, default: `24h`, allowed range: `1h` to `72h`; registration/renewal responses include `cert_expires_at` and `cert_renew_after`)
- `GATEWAY_MTLS_REVOKED_SERIALS` (comma-separated certificate serial denylist for deployment-level emergency mTLS revocation; runtime serial blocks are managed by `POST /api/v1/gateways/cert-revocations`)
- `EXTERNAL_AUTH_TIMEOUT` (duration, default: `8s`)
- `EXTERNAL_AUTH_DEFAULT_ROLE` (default: `resident`; fallback role when provider payload has no role)
- `REDIS_ADDR` (optional; when configured, auth refresh session, revoked access token blacklist, and API/login rate-limit counters migrate to Redis-compatible backend)
- `REDIS_PASSWORD` (optional)
- `REDIS_DB` (default: `0`)
- `REDIS_KEY_PREFIX` (default: `mistypass`)
- `REDIS_DIAL_TIMEOUT` (duration, default: `3s`)
- `REDIS_READ_TIMEOUT` (duration, default: `3s`)
- `REDIS_WRITE_TIMEOUT` (duration, default: `3s`)
- `MQTT_ENABLED` (default: `false`; enables gateway MQTT bootstrap contract endpoint)
- `MQTT_BROKER_URL` (default: `tcp://localhost:1883`)
- `MQTT_TOPIC_PREFIX` (default: `mistypass`; tenant/gateway topic namespace root)
- `NATS_ENABLED` (default: `false`; enables internal event bus publishing)
- `NATS_SERVER_URL` (default: `nats://localhost:4222`)
- `NATS_SUBJECT_PREFIX` (default: `mistypass`; internal bus subject namespace root)

Wallet remote validation env vars (optional):

- `WALLET_GOOGLE_REMOTE_VALIDATE` (default: `false`)
- `WALLET_GOOGLE_REMOTE_TIMEOUT` (duration, default: `8s`)
- When remote validation is enabled, `key_ref` in `POST /api/v1/wallet/google/config/validate` supports:
  - `env://<ENV_VAR_NAME>`
  - `secret://env/<ENV_VAR_NAME>`
  - `file://<path-to-service-account-json>`
- Note: real Google Wallet issuance APIs may still be blocked by enterprise LEI onboarding status.

Wallet alert dispatch provider env vars (optional):

Shared email provider env vars (optional):

- `MAIL_PROVIDER` (`resend` | `cloudflare`; report email defaults to `resend` for backward compatibility)
- `CLOUDFLARE_ACCOUNT_ID` (required for default Cloudflare Email Service endpoint)
- `CLOUDFLARE_EMAIL_API_TOKEN` (required when using Cloudflare Email Service)
- `CLOUDFLARE_EMAIL_ENDPOINT` (default: `https://api.cloudflare.com/client/v4/accounts/{account_id}/email/sending/send`)
- `CLOUDFLARE_EMAIL_TIMEOUT` (duration, default: `5s`)
- `USER_INVITATION_EMAIL_PROVIDER` (`queue` | `mock` | `resend` | `cloudflare`, default: `queue`)
- `USER_INVITATION_EMAIL_FROM` (default: `no-reply@mistypass.local`)
- `REPORT_EMAIL_FROM` (report schedule sender; falls back to `USER_INVITATION_EMAIL_FROM` for backward compatibility)

Wallet alert dispatch provider env vars (optional):

- `WALLET_ALERT_EMAIL_PROVIDER` (`mock` | `resend` | `cloudflare`, default: `mock`)
- `WALLET_ALERT_EMAIL_FROM` (default: `no-reply@mistypass.local`)
- `WALLET_ALERT_EMAIL_RECEIVER_MAP` (group-to-email mapping, format: `security=sec@example.com,sec2@example.com;ops=ops@example.com`)
- `WALLET_ALERT_RESEND_ENDPOINT` (default: `https://api.resend.com/emails`)
- `WALLET_ALERT_RESEND_API_KEY` (required when provider is `resend`)
- `WALLET_ALERT_RESEND_TIMEOUT` (duration, default: `5s`)
- `WALLET_ALERT_WHATSAPP_PROVIDER` (`mock` | `meta`, default: `mock`)
- `WALLET_ALERT_WHATSAPP_RECEIVER_MAP` (group-to-phone mapping, format: `security=+62811111111,+62811111112;ops=+62822222222`)
- `WALLET_ALERT_WHATSAPP_ENDPOINT` (default: `https://graph.facebook.com/v22.0` when provider is `meta`)
- `WALLET_ALERT_WHATSAPP_API_KEY` (required when `WALLET_ALERT_WHATSAPP_PROVIDER=meta`)
- `WALLET_ALERT_WHATSAPP_PHONE_NUMBER_ID` (required when `WALLET_ALERT_WHATSAPP_PROVIDER=meta`)
- `WALLET_ALERT_WHATSAPP_TIMEOUT` (duration, default: `5s`)
- Current plan: until Meta enterprise API approval is completed, keep `WALLET_ALERT_WHATSAPP_PROVIDER=mock`; production notifications use email via `cloudflare`.
- Backward compatibility: `WALLET_ALERT_EMAIL_PROVIDER=spaceemail` and `WALLET_ALERT_SPACEEMAIL_*` env vars will be mapped to `resend`.
- `GET /api/v1/wallet/jobs/alert-notifications` and `POST /api/v1/wallet/jobs/alerts/dispatch` now return `channel_results` for unified per-channel delivery receipts.
- `WALLET_ALERT_DISPATCH_MOCK_TRANSIENT_FAIL_COUNT` (int, default: `0`, testing only)

Enterprise sync reconcile worker env vars (optional):

- `ENTERPRISE_SYNC_RECONCILE_WORKER_ENABLED` (default: `false`)
- `ENTERPRISE_SYNC_RECONCILE_WORKER_INTERVAL` (duration, default: `30s`)
- `ENTERPRISE_SYNC_RECONCILE_WORKER_BATCH_SIZE` (int, default: `20`)
- `ENTERPRISE_SYNC_RECONCILE_WORKER_MAX_ATTEMPTS` (int, default: `5`)
- `ENTERPRISE_SYNC_RECONCILE_WORKER_RETRY_COOLDOWN` (duration, default: `30s`)
- `ENTERPRISE_SYNC_RECONCILE_WORKER_ALERT_FAILURE_THRESHOLD` (int, default: `3`)
- `ENTERPRISE_SYNC_RECONCILE_WORKER_FORCE_ERROR` (bool, default: `false`, testing only)
- `ENTERPRISE_SYNC_RECONCILE_WORKER_FORCE_ERROR_TENANT_ID` (string, optional, testing only)
- `GATEWAY_EVENTS_BATCH_FORCE_RETRYABLE_ERROR` (bool, default: `false`, testing only)
- `GATEWAY_EVENTS_BATCH_FORCE_RETRYABLE_PREFIX` (string, default: `force-retry-`, testing only)

Enterprise JIT auth env vars (optional):

- `ENTERPRISE_JIT_PROVISION_APPROVAL_REQUIRED` (bool, default: `false`; when `true`, `sync_mode=jit` only allows known directory employees to sign in, and blocks auto-provision with `403`)

Enterprise JIT approval external sync worker env vars (optional):

- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_ENABLED` (bool, default: `false`)
- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_INTERVAL` (duration, default: `30s`)
- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_BATCH_SIZE` (int, default: `20`)
- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_MAX_ATTEMPTS` (int, default: `5`)
- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_RETRY_COOLDOWN` (duration, default: `30s`)
- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_ALERT_FAILURE_THRESHOLD` (int, default: `3`)
- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_FORCE_ERROR` (bool, default: `false`, testing only)
- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_FORCE_ERROR_TENANT_ID` (string, optional, testing only)
- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_CALLBACK_TOKEN` (string, required to enable public callback endpoint)

Legacy credential (old docs/examples):

- `admin@mistypass.local` / `admin123`

Gateway bootstrap auth:

- `POST /api/v1/gateway/register` requires `X-Bootstrap-Token: <bootstrap_token>` (or `Authorization: Bearer <bootstrap_token>`) and then returns `device_token`
- `bootstrap_token` comes from server-side `GATEWAY_BOOTSTRAP_TOKEN`
- Other `/api/v1/gateway/*` device endpoints require `X-Device-Token: <device_token>`
- `Authorization: Bearer <device_token>` is also accepted
- Gateway HTTP device requests support replay protection with `X-Request-Nonce` + RFC3339 `X-Request-Timestamp`; when `GATEWAY_REQUIRE_REQUEST_NONCE=true`, missing nonce headers are rejected.
- Gateway device requests should identify the device with `X-Gateway-ID` and `X-Tenant-ID`; `/credentials/sync` and `/audit/batch` also accept `gateway_id` / `tenant_id` query parameters for compatibility.
- When `GATEWAY_MTLS_ADDR` is configured, `/api/v1/gateway/*` is also served on a dedicated TLS 1.3 listener that requires a gateway client certificate signed by `GATEWAY_CA_CERT_PEM`; the client certificate subject must bind `CN=<gateway_id>` and `O=<tenant_id>`.
- Admins can set `PATCH /api/v1/gateways/{gatewayID}/status` to `disabled` or `revoked` to block both mTLS and token fallback for that gateway identity.
- Admins can use `GET/POST/DELETE /api/v1/gateways/cert-revocations` to list, add, and restore persisted runtime mTLS certificate serial revocations. Serial values are normalized, audited, and enforced during gateway mTLS authentication; serials configured in `GATEWAY_MTLS_REVOKED_SERIALS` remain deployment-managed and cannot be restored via API.
- `GET /api/v1/gateway/ws` authenticates new gateway agents with mTLS or `Authorization: Bearer <device_token>` / `X-Device-Token`; `?token=` is rejected.
- Gateway WebSocket sessions expire after `GATEWAY_WS_MAX_SESSION_TTL`; the server sends a close frame with `session expired; reconnect`, and current agents reconnect with exponential backoff.
- When `NATS_ENABLED=true`, gateway WebSocket authz cache pushes are fanned out on `mistypass.gateway.ws.push` so multi-instance API deployments can deliver a push from the replica that owns the target gateway connection. Each push carries an origin instance id so the publisher does not duplicate its own local delivery.
- `POST /api/v1/gateway/config/pull` returns desired config version + bound doors/devices, and now includes `authz_cache` (scoped by bound doors) with `version/generated_at/expires_at/ttl_seconds/scope/counts`, plus `policy` (`fallback_mode/no_cache_behavior/max_stale_seconds/stale_until/rollback_version`) and `status_codes` (`AUTHZ_CACHE_*`) for edge-side expiry/rollback handling. Request may carry optional `authz_cache_version`; response returns `authz_cache.status` (`AUTHZ_CACHE_FRESH|STALE|MISSING|DRIFT`).
- `POST /api/v1/gateway/config/applied` accepts optional `authz_cache_version` and returns `authz_cache.version_match` + `authz_cache.status`; mismatch writes audit `gateway_config_authz_cache_version_drift` for drift tracing, match updates next pull `policy.rollback_version`.
- `POST /api/v1/gateway/events/access` and `POST /api/v1/gateway/events/device` now persist into event module and support replay dedup by `idempotency_key` (or fallback `request_id`/`event_id`).
- Single-event ingest responses include `queue_progress` (`queue/ingested_total/updated_at`) for default queue watermark coordination.
- `POST /api/v1/gateway/events/batch` supports offline queue flush in one request and returns per-type `created/deduplicated` counters.
- `POST /api/v1/gateway/events/batch` supports partial success mode: invalid items are returned in `results` with `status=failed`; each failed row includes `retryable`, and `retry_subset` returns the directly replayable failed subset (same payload shape as batch request, now with `queue`).
- `POST /api/v1/gateway/events/batch` now returns `queue_hint` (`queue/checkpoint_id/acked_increment/server_ingested_total/status_code/next_action`) so gateway can decide whether to replay `retry_subset` or report checkpoint directly. `server_ingested_total` tracks created records (pure deduplicated replays do not increase it).
- `POST /api/v1/gateway/events/checkpoint` lets gateway report queue watermark (`queue/checkpoint_id/acked_count`) for replay progress tracking; `acked_count` must be monotonic per `gateway_id + queue` (regression returns HTTP `409`).
  - On `409`, response includes latest server-side checkpoint snapshot and `next_action=retry_with_non_regressing_acked_count` for direct gateway recovery.
  - If `acked_count` exceeds server-side queue ingest total, returns `409` with `next_action=retry_with_server_event_total` and `server_event_total` (with `server_total_source`; `default` queue has `event_rows_fallback` for legacy snapshots).
- `GET /api/v1/gateways/{gatewayID}/events/checkpoint` lets admin query latest gateway queue watermark records.
- `GET /api/v1/gateways/events/checkpoint/summary` provides queue-level lag view (`event_total/acked_total/lag_total`) and `time_window_trend` (computed from recent `gateway_event_checkpoint_reported` audit logs; supports `trend_window_minutes`).
- OTA firmware task APIs are available on `/api/v1/gateways/{gatewayID}/ota/tasks`: create task (`POST` with `firmware_version/firmware_url/firmware_sha256`), list tasks (`GET`), and status updates (`PATCH .../{taskID}/status` with `queued|dispatching|succeeded|failed|canceled`).
- Replayed events return the same `event_id` with `deduplicated=true`, preventing duplicate rows in `/api/v1/events/access|device`.
- Optional `occurred_at` must be RFC3339 when provided.
- For MistyPass-produced hardware, serials should be imported first via `POST /api/v1/gateways/serial-inventory/import`; registration/attach will consume serial status.
- Bulk onboarding/export is supported by `POST /api/v1/gateways/serial-inventory/import-csv` and `GET /api/v1/gateways/serial-inventory/export-csv`.
- Serial inventory lifecycle supports `available`, `frozen`, `scrapped` via `PATCH /api/v1/gateways/serial-inventory/{serialNumber}/status`.
- Batch lifecycle updates are supported by `PATCH /api/v1/gateways/serial-inventory/batch-status` (`serial_numbers[]` + target `status`).
- Gateway device protocol defaults: `legacy_*` / `legacy_integration` -> `wiegand_26`, non-legacy -> `osdp_v2`; explicit protocols support `wiegand_26`, `wiegand_34`, `osdp_v2`, `rs485`, `ble`.
- For `protocol=rs485`, `POST /api/v1/gateways/{gatewayID}/devices` accepts optional `rs485_config` (`baud_rate`, `parity`, `stop_bits`, `device_address`, `timeout_ms`); sending this config with non-`rs485` protocols is rejected.
- `POST /api/v1/gateways/{gatewayID}/devices/{deviceID}/rs485/telemetry` supports runtime reliability counters (`retries/timeouts/collisions`, optional `last_error`); alerts are audited once thresholds are reached (`consecutive_timeouts>=3` or `collision_count>=5`).

Building admin scope management:

- `building_admin` scope is carried in JWT claim `building_ids`
- `GET/PUT /api/v1/auth/users/{userID}/building-scope` can read/update scope
- `building_admin` can only read/write resources within allowed `building_ids`
- `users`, `user-groups`, `visitor-passes` write APIs now support `building_id`

Admin MFA and external auth:

- Admin MFA endpoints (`/api/v1/auth/mfa/admin/*`) support TOTP enrollment (`setup`), activation (`enable` with code), status query, and disable.
- When `AUTH_ADMIN_MFA_REQUIRED=true`, `super_admin` / `tenant_admin` login requires enrolled and valid `mfa_code`.
- External auth login (`POST /api/v1/auth/external/login`) validates provider access token via configured userinfo endpoint, then issues local MistyPass JWT session via trusted identity bridge.

Enterprise auth exchange behavior:

- `POST /api/v1/enterprise/auth/exchange` now supports both `oidc` and `saml` provider
- `POST /api/v1/enterprise/auth/start` returns provider-specific login entry (`authorize_url` for OIDC / `sso_url` for SAML) with short-lived `state_token`.
- `GET /api/v1/enterprise/auth/oidc/callback` and `POST /api/v1/enterprise/auth/saml/callback` consume one-time `state_token` and return unified session payload.
- OIDC callback supports `code` exchange path (`code -> id_token`) against configured `token_url` (or `issuer_url/oauth2/token` fallback).
- OIDC validates token signature + `iss` + `aud` + `email`
- SAML validates signed assertion (X509) + issuer/audience/recipient/time window + `email`
- SAML config fields: `saml_acs_url`, `saml_x509_cert`
- When `idp_config.sync_mode=jit` and local auth user is missing:
  - If employee profile exists and is `active`, issue trusted session from enterprise profile.
  - If directory record is missing, auto-provision `source=jit_provision` employee profile (domain-gated) then issue trusted session.
  - If matched employee is `inactive`, reject session issue.
- In `sync_mode=jit`, callback/exchange also blocks by identity claim status: `employment_status/status in {inactive, terminated, disabled, suspended, deprovisioned}` or `active=false` rejects session issue before JIT provision.
- In `sync_mode=jit`, enterprise login now re-syncs local trusted user role/building scope from enterprise employee profile on each successful callback/exchange.
- In `sync_mode=jit`, if matched employee `source` indicates directory sync (`scim`/`hris`), callback claims no longer overwrite existing `full_name/department/job_title/location`; claims only fill empty fields.
- In `sync_mode=jit`, deep attributes now include `phone`, `manager_external_id`, and normalized `employment_status`; these fields follow the same SCIM/HRIS snapshot-priority rule (only fill empty fields for directory-sourced employees).
- In `sync_mode=jit`, when session issue is blocked as inactive, backend performs first-stage deprovision action: revoke all refresh sessions for the matched email and append audit event `enterprise_jit_deprovision_applied`.
- In `sync_mode=jit`, when inactive deprovision is triggered, backend also performs second-stage local downgrade: trusted user is reduced to least privilege (`role=resident`, empty building scope), and audit target includes `downgraded_local/old_role/new_role`.
- In `sync_mode=jit`, if `ENTERPRISE_JIT_PROVISION_APPROVAL_REQUIRED=true`, unknown directory users are blocked before auto-provision (`ErrJITProvisionApprovalRequired`, `403`) and audited as `enterprise_jit_approval_required`.
- Under approval-required mode, blocked requests are persisted as `pending` JIT approval records; once reviewed as `approved`, the same identity is allowed to continue JIT auto-provision on next callback/exchange.
- callback/exchange session issue errors now use differentiated status codes: `inactive -> 403`, `external_id conflict -> 409`, input validation failures -> `400`, unexpected internal failures -> `500`.
- Exchange response now includes `sync_mode` and `jit_applied` to indicate whether JIT fallback path was used.
- `POST /api/v1/enterprise/auth/logout` supports enterprise session revocation with `access_token` and/or `refresh_token` and returns revocation summary.

Enterprise employee sync behavior:

- `POST /api/v1/enterprise/employees/sync` supports optional `request_id` for idempotent retries.
- Same `tenant_id + request_id` returns the same sync job and cached `access_sync` counters.
- If enterprise sync was persisted but access sync was not yet applied, retry with same `request_id` continues access apply and then caches the final result.
- `POST /api/v1/enterprise/employees/sync/reconcile` allows explicit replay by `tenant_id + request_id` without resending employee list.
- `GET /api/v1/enterprise/sync-requests` can inspect reconciliation audit fields (`access_applied`, `access_attempt_count`, `last_access_error`, `last_access_attempt_at`).
- `POST /api/v1/enterprise/sync-requests/reconcile-pending` runs batch compensation for pending (`access_applied=false`) requests with `limit`.
- `GET /api/v1/enterprise/jit-provision-approvals` lists persisted JIT approval records (`pending/approved/rejected`) by tenant.
- `POST /api/v1/enterprise/jit-provision-approvals/{approvalID}/review` updates approval decision (`approved/rejected`) with reviewer audit trace.
- `GET /api/v1/enterprise/jit-provision-approvals/external-sync-pending` lists reviewed approvals waiting for upstream sync (`external_sync_status=pending`).
- `POST /api/v1/enterprise/jit-provision-approvals/{approvalID}/external-sync` records upstream sync result (`synced/failed`) with `external_sync_ref` and retry/error trace.
- `POST /api/v1/enterprise/jit-provision-approvals/external-sync/callback` allows upstream HRIS/SCIM callback push (`tenant_id/approval_id/status/...`) with callback token auth (`X-Enterprise-Callback-Token` or `Authorization: Bearer ...`).
- JIT external-sync worker retries `pending/failed` approvals by policy gate (`max_attempts`, `retry_cooldown`) and appends alert audit `enterprise_jit_approval_external_sync_worker_alert` once failures in a tick reach threshold.
- Worker compensation applies policy gates (`max_attempts`, `retry_cooldown`) and logs alerts once failures in a tick reach `alert_failure_threshold`.
- Worker alert is persisted to audit logs (`action=enterprise_sync_reconcile_worker_alert`), queryable via `GET /api/v1/audit-logs?tenant_id=...&action=enterprise_sync_reconcile_worker_alert&source=enterprise_sync_worker&limit=50`.
- `GET /api/v1/enterprise/sync-worker-alerts` returns structured alert metrics (`failed`, `threshold`, `processed`, `applied`, `skipped_*`) from worker audit events; supports optional `since`/`until` (RFC3339).
- `GET /api/v1/enterprise/sync-worker-alerts/summary` returns tenant-level aggregation (`count`, `first_seen_at`, `last_seen_at`, latest metrics); supports optional `since`/`until` (RFC3339).

PostgreSQL state change log behavior:

- Every successful module snapshot `Save` writes one change event into `mistypass_change_log`.
- `GET /api/v1/state/change-log` can inspect recent state changes by `state_key`.
- `POST /api/v1/state/change-log/replay` re-applies projection updates from change log events (`state_key + from_id + limit`).
- `GET /api/v1/state/change-log/checkpoints` can inspect replay checkpoints (`state_key + last_change_id + updated_at`).
- `POST /api/v1/state/change-log/replay/checkpoint` replays from the stored checkpoint and advances checkpoint after success (`state_key + limit`).

Audit webhook PoC behavior:

- `PUT /api/v1/audit/webhook/config` upserts tenant-level webhook settings (`enabled/endpoint/actions[]`).
- `GET /api/v1/audit/webhook/config` reads current tenant webhook config.
- `POST /api/v1/audit/webhook/dispatch` manually dispatches one audit event (`audit_log_id` or latest by filter).
- `GET /api/v1/audit/webhook/deliveries` lists recent delivery records with `status/http_status/error`.
- When `NATS_ENABLED=true`, audit events are fanned out to NATS subjects:
  - `mistypass.audit.log.appended`
  - `mistypass.audit.webhook.dispatched`
  - `mistypass.audit.webhook.dispatch.failed`
- Gateway WebSocket push fanout also uses NATS subject `mistypass.gateway.ws.push` for cross-replica delivery.
- Gateway security metrics include nonce validation, mTLS/token fallback auth results, current mTLS serial revocation gauges, WebSocket auth/session lifecycle, cross-replica fanout, and authz cache reports. Prometheus alert examples live in `deploy/prometheus/gateway-security-alerts.yml`.
- Regression script: `docs/testing/curl-audit-webhook-fanout.zsh` covers disabled conflict, unreachable endpoint failure record, and action-filter conflict.

## 5. Notes

- By default data is in-memory and resets when the API restarts.
- If `DATABASE_URL` is configured, current phase persists `tenant`/`space`/`access`/`gateway`/`enterprise`/`event`/`alarm`/`audit`/`wallet` module state into PostgreSQL snapshot table `mistypass` and syncs it into `mistypass_*` projection tables.
- If `REDIS_ADDR` is configured, auth refresh session state, revoked access token blacklist, IP rate-limit counters, and gateway request nonce replay guards use Redis/Dragonfly for multi-instance consistency.
- This is a delivery-focused MVP scaffold intended for rapid extension.

## 6. Wallet planning

- Google Wallet priority issuance plan:
  - `docs/wallet/google-wallet-issuance-plan.md`
- Development status and roadmap split:
  - `docs/development-status-roadmap.md`
- C-zone sprint plan:
  - `docs/sprints/c-zone-sprint-plan.md`
- Indonesia enterprise onboarding design:
  - `docs/enterprise/indonesia-enterprise-domain-idp-design.md`
- Gateway serial/protocol + Misty Access plan:
  - `docs/architecture/gateway-serial-protocol-mobile-plan.md`
- Gateway software security status:
  - `docs/architecture/gateway-security-software-status.md`
- Hardware/BSP security follow-ups:
  - `docs/architecture/hardware-bsp-followups.md`
- Completed features + tech stack map:
  - `docs/architecture/completed-features-tech-stack.md`
- Enterprise design spec:
  - `docs/enterprise/enterprise-design-spec.md`
- Enterprise callback orchestration draft:
  - `docs/enterprise/oidc-saml-callback-orchestration-draft.md`
- Enterprise schema draft:
  - `docs/enterprise/enterprise-schema.sql`
- Admin UI test and API map:
  - `docs/testing/admin-ui-test-and-api-map.md`
