# Gateway Security Software Status

<!-- PROD_READY -->

Updated: 2026-05-11

This document is the current software-layer status for MistyPass gateway to cloud communication. Hardware root of trust, secure boot, enclosure tamper, and reader/controller physical separation are tracked separately in `docs/architecture/hardware-bsp-followups.md`.

## Current Software Baseline

| Area | Current status | Notes |
| --- | --- | --- |
| Device bootstrap | Implemented | `POST /api/v1/gateway/register` requires `GATEWAY_BOOTSTRAP_TOKEN`, consumes serial inventory, and returns a gateway-scoped device token. |
| Gateway mTLS | Implemented | Optional gateway-only TLS 1.3 listener via `GATEWAY_MTLS_ADDR`; verified client certificate must bind `CN=<gateway_id>` and `O=<tenant_id>`. |
| Client certificate issuance | Implemented | Registration and renewal accept gateway CSR and sign with the configured gateway CA. |
| Device token fallback | Implemented | HTTP gateway paths accept device tokens for compatibility; WebSocket accepts mTLS or header tokens only; dedicated mTLS listener rejects token fallback when client cert is required. |
| Persistent cloud channel | Implemented | `GET /api/v1/gateway/ws` authenticates with mTLS, `Authorization: Bearer`, or `X-Device-Token`; `?token=` is rejected. Sessions expire by `GATEWAY_WS_MAX_SESSION_TTL` and agents reconnect. |
| HTTP replay protection | Implemented, opt-in required mode | Gateway agent sends `X-Request-Nonce` + `X-Request-Timestamp`; server validates timestamp window and gateway-scoped nonce uniqueness. Redis is used for cross-instance replay prevention when `REDIS_ADDR` is configured. Set `GATEWAY_REQUIRE_REQUEST_NONCE=true` after old agents are upgraded. |
| Config and authz cache | Implemented | `config/pull` returns bound-door scoped config and `authz_cache` with version, TTL, stale policy, rollback version, and status codes. |
| Config acknowledgement | Implemented | `config/applied` accepts `authz_cache_version`, detects drift, and records audit when reported and expected versions differ. |
| Credential sync | Implemented | `/gateway/credentials/sync` authenticates gateway identity and returns credentials scoped to the gateway's bound doors. |
| Offline event replay | Implemented | `/gateway/events/batch` supports idempotency, partial success, retry subsets, and queue hints. |
| Replay checkpoint | Implemented | `/gateway/events/checkpoint` enforces monotonic `acked_count` and rejects impossible counts above server-ingested totals. |
| Offline audit upload | Implemented | `/gateway/audit/batch` authenticates gateway identity and writes logs under server-side tenant/gateway context. |
| Gateway revocation operations | Implemented | `PATCH /api/v1/gateways/{gatewayID}/status` supports `disabled`/`revoked`; `GET/POST/DELETE /api/v1/gateways/cert-revocations` manages persisted runtime mTLS serial denylist entries. Admin UI exposes status changes and serial restore/revoke operations with audit logs. |
| Revocation push | Implemented at cloud channel layer | WebSocket registry can send gateway messages; NATS fanout delivers authz cache pushes across API replicas when `NATS_ENABLED=true` and skips the origin instance to avoid duplicate local frames; edge-side enforcement still depends on local cache refresh and online push delivery. |

## Production Configuration Guidance

- Enable gateway mTLS in production with `GATEWAY_MTLS_ADDR`, server certificate/key, and gateway CA certificate/key.
- Keep `GATEWAY_BOOTSTRAP_TOKEN` out of device runtime after provisioning; use it only for initial registration and activation.
- Enable `GATEWAY_REQUIRE_REQUEST_NONCE=true` once deployed gateway agents are confirmed to send nonce headers on all HTTP device requests.
- Configure `REDIS_ADDR` before running multiple API replicas so nonce replay protection is shared across instances.
- Follow `docs/architecture/gateway-nonce-rollout-runbook.md` for staged nonce enforcement.
- Enable `NATS_ENABLED=true` in multi-instance API deployments so WebSocket config push fanout can reach the replica that owns each gateway connection.
- Use `Authorization: Bearer <device_token>` / `X-Device-Token` headers or mTLS for WebSocket handshakes; `?token=` is no longer accepted.
- Keep `GATEWAY_WS_MAX_SESSION_TTL` at the default `6h` unless field reconnect behavior requires a shorter rotation window.
- Keep gateway client certs short-lived with `GATEWAY_MTLS_CERT_LIFETIME` (default `24h`, max `72h`); use `PATCH /api/v1/gateways/{gatewayID}/status` with `revoked`/`disabled` for identity-level shutdown.
- Use `POST /api/v1/gateways/cert-revocations` or the admin UI gateway security panel for runtime mTLS certificate serial revocation; use `GATEWAY_MTLS_REVOKED_SERIALS` only for deployment-level emergency serial blocks that should not be restorable from the UI/API.
- Treat `GATEWAY_CA_KEY_PEM` as production secret material and store it in the deployment secret manager, not in source files.

## Security Metrics

Prometheus `/metrics` now includes:

- `mistypass_gateway_request_nonce_validation_total{result=...}` for accepted Redis/memory paths and nonce failure classes.
- `mistypass_gateway_mtls_auth_total{result=...}` for accepted mTLS, revoked serials, subject/tenant mismatches, gateway revocation, and token fallback use.
- `mistypass_gateway_mtls_cert_revocations_total{source=...}` for current runtime and environment certificate serial revocation counts.
- `mistypass_gateway_websocket_auth_total{mode=...}` for mTLS/header-token modes and rejected query tokens.
- `mistypass_gateway_websocket_sessions_total{event=...}` for connected, disconnected, and expired WebSocket sessions.
- `mistypass_gateway_websocket_push_fanout_total{result=...}` for NATS fanout publish and local-delivery results.
- `mistypass_gateway_authz_cache_reports_total{status=...}` for `AUTHZ_CACHE_FRESH|STALE|MISSING|DRIFT` reports.

Alert rules live in `deploy/prometheus/gateway-security-alerts.yml`.

## Remaining Software Follow-Ups

No software-layer follow-up from the previous gateway security list is still open. Further work should now be driven by production rollout evidence, for example tightening default nonce enforcement after agent adoption, reducing token fallback exposure in mTLS-only environments, and tuning alert thresholds from fleet telemetry.

## Architecture Position

MistyPass is currently an edge-first design: gateways can keep operating from local authorization cache during cloud outages, then replay events and audit logs after reconnect. The software layer now has the main communication controls expected for that model: authenticated devices, optional mTLS, persistent outbound channel, scoped config/credential sync, replay-safe event upload, and configurable HTTP nonce enforcement.

This does not make the physical device tamper-resistant by itself. Hardware trust, secure boot, signed OTA enforcement at bootloader level, and default reader/controller separation remain separate product and BSP work items.
