# Gateway Security Software Status

Updated: 2026-05-11

This document is the current software-layer status for MistyPass gateway to cloud communication. Hardware root of trust, secure boot, enclosure tamper, and reader/controller physical separation are tracked separately in `docs/architecture/hardware-bsp-followups.md`.

## Current Software Baseline

| Area | Current status | Notes |
| --- | --- | --- |
| Device bootstrap | Implemented | `POST /api/v1/gateway/register` requires `GATEWAY_BOOTSTRAP_TOKEN`, consumes serial inventory, and returns a gateway-scoped device token. |
| Gateway mTLS | Implemented | Optional gateway-only TLS 1.3 listener via `GATEWAY_MTLS_ADDR`; verified client certificate must bind `CN=<gateway_id>` and `O=<tenant_id>`. |
| Client certificate issuance | Implemented | Registration and renewal accept gateway CSR and sign with the configured gateway CA. |
| Device token fallback | Implemented | HTTP and WebSocket gateway paths still accept device tokens for compatibility; dedicated mTLS listener rejects token fallback when client cert is required. |
| Persistent cloud channel | Implemented | `GET /api/v1/gateway/ws` authenticates with `Authorization: Bearer` / `X-Device-Token`; legacy query token remains only for compatibility. |
| HTTP replay protection | Implemented, opt-in required mode | Gateway agent sends `X-Request-Nonce` + `X-Request-Timestamp`; server validates timestamp window and gateway-scoped nonce uniqueness. Set `GATEWAY_REQUIRE_REQUEST_NONCE=true` after old agents are upgraded. |
| Config and authz cache | Implemented | `config/pull` returns bound-door scoped config and `authz_cache` with version, TTL, stale policy, rollback version, and status codes. |
| Config acknowledgement | Implemented | `config/applied` accepts `authz_cache_version`, detects drift, and records audit when reported and expected versions differ. |
| Credential sync | Implemented | `/gateway/credentials/sync` authenticates gateway identity and returns credentials scoped to the gateway's bound doors. |
| Offline event replay | Implemented | `/gateway/events/batch` supports idempotency, partial success, retry subsets, and queue hints. |
| Replay checkpoint | Implemented | `/gateway/events/checkpoint` enforces monotonic `acked_count` and rejects impossible counts above server-ingested totals. |
| Offline audit upload | Implemented | `/gateway/audit/batch` authenticates gateway identity and writes logs under server-side tenant/gateway context. |
| Revocation push | Implemented at cloud channel layer | WebSocket registry can send gateway messages; edge-side enforcement still depends on local cache refresh and online push delivery. |

## Production Configuration Guidance

- Enable gateway mTLS in production with `GATEWAY_MTLS_ADDR`, server certificate/key, and gateway CA certificate/key.
- Keep `GATEWAY_BOOTSTRAP_TOKEN` out of device runtime after provisioning; use it only for initial registration and activation.
- Enable `GATEWAY_REQUIRE_REQUEST_NONCE=true` once deployed gateway agents are confirmed to send nonce headers on all HTTP device requests.
- Prefer `Authorization: Bearer <device_token>` or `X-Device-Token` headers for WebSocket handshakes; do not rely on legacy `?token=`.
- Treat `GATEWAY_CA_KEY_PEM` as production secret material and store it in the deployment secret manager, not in source files.

## Remaining Software Follow-Ups

| Follow-up | Impact | Suggested priority |
| --- | --- | --- |
| Persist request nonce cache in Redis for horizontally scaled API nodes | Prevents replay across multiple API replicas | High before multi-node production |
| Remove legacy WebSocket `?token=` fallback after gateway agent rollout | Reduces token exposure in logs and proxies | Medium |
| Add explicit WebSocket session expiry and reconnect requirement | Bounds long-lived token/cert sessions | Medium |
| Add certificate revocation list or short-lived cert rotation policy | Improves response to compromised gateway certs | Medium |
| Add deployment runbook for `GATEWAY_REQUIRE_REQUEST_NONCE` staged rollout | Reduces field upgrade risk | Medium |
| Add metrics for nonce failures, mTLS fallback use, WS auth mode, and stale authz cache reports | Improves security operations visibility | Medium |

## Architecture Position

MistyPass is currently an edge-first design: gateways can keep operating from local authorization cache during cloud outages, then replay events and audit logs after reconnect. The software layer now has the main communication controls expected for that model: authenticated devices, optional mTLS, persistent outbound channel, scoped config/credential sync, replay-safe event upload, and configurable HTTP nonce enforcement.

This does not make the physical device tamper-resistant by itself. Hardware trust, secure boot, signed OTA enforcement at bootloader level, and default reader/controller separation remain separate product and BSP work items.
