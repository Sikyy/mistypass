# Gateway Request Nonce Rollout Runbook

Updated: 2026-05-11

This runbook stages `GATEWAY_REQUIRE_REQUEST_NONCE=true` without breaking older gateway agents. It covers only software rollout. Hardware boot, OTA signing, and reader/controller physical controls are tracked separately.

## Preconditions

- All production API replicas run the build that supports Redis-backed gateway nonce replay guards.
- `REDIS_ADDR` points to the production Redis/Dragonfly HA endpoint on every API replica.
- Gateway agents send both headers on every gateway HTTP device request:
  - `X-Request-Nonce`
  - `X-Request-Timestamp` in RFC3339 UTC format
- Gateway WebSocket handshakes use mTLS or header tokens. `?token=` is no longer accepted.
- Prometheus scrapes API `/metrics` and loads `deploy/prometheus/gateway-security-alerts.yml`.

## Phase 0: Observe Compatibility

Keep:

```env
GATEWAY_REQUIRE_REQUEST_NONCE=false
```

Check for 24 hours:

- `mistypass_gateway_request_nonce_validation_total{result="accepted_redis"}` increases on active gateway traffic.
- `mistypass_gateway_request_nonce_validation_total{result="accepted_memory"}` stays zero in multi-instance production.
- `mistypass_gateway_request_nonce_validation_total{result=~"partial|invalid_timestamp|timestamp_window|duplicate|store_error"}` stays near zero.
- No gateway fleet segment is still missing nonce headers in access/audit/event/config calls.

Pause rollout if Redis store errors occur or if any known gateway agent version cannot send nonce headers.

## Phase 1: Canary Required Mode

Enable `GATEWAY_REQUIRE_REQUEST_NONCE=true` for one API replica behind the load balancer.

Use a narrow traffic slice if the load balancer supports weighted routing. Otherwise place one low-risk tenant or test building behind the canary replica.

Watch for at least 2 hours:

- HTTP 401/400/409 spikes on `/api/v1/gateway/*`.
- `mistypass_gateway_request_nonce_validation_total{result="missing"}`.
- `mistypass_gateway_request_nonce_validation_total{result=~"partial|invalid_timestamp|timestamp_window|duplicate"}`.
- Gateway offline or stale authz cache reports.

Rollback canary by setting `GATEWAY_REQUIRE_REQUEST_NONCE=false` on the canary replica and restarting only that replica.

## Phase 2: Fleet Enablement

Enable:

```env
GATEWAY_REQUIRE_REQUEST_NONCE=true
```

on all API replicas.

Confirm:

- No sustained `missing` nonce results for 30 minutes.
- No sustained `store_error`; Redis errors should page immediately because required mode depends on Redis for cross-instance replay resistance.
- `accepted_redis` continues increasing on all gateway traffic.
- `duplicate` results are isolated. Sustained duplicates indicate agent retry bugs or replay attempts.

## Rollback

Use rollback only for client compatibility or Redis outage.

1. Set `GATEWAY_REQUIRE_REQUEST_NONCE=false` on all API replicas.
2. Restart API replicas gradually.
3. Keep Redis configured. Optional nonce headers still use Redis and still protect upgraded agents.
4. Create an incident note with the dominant metric result, affected gateway versions, and tenant/building scope.

## Operational Notes

- Multi-instance production must not run required mode without Redis. In this build Redis store errors return `503` instead of falling back to process memory.
- `duplicate` is expected for true replay or for agents that retry the same signed request body with the same nonce. Agents should generate a fresh nonce for each new HTTP request attempt.
- `timestamp_window` usually means device clock drift. Field ops should inspect NTP or RTC drift before disabling nonce enforcement.
- During API deploys, keep `GATEWAY_WS_MAX_SESSION_TTL` unchanged so WebSocket reconnect churn does not mask nonce rollout failures.
