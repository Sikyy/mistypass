# MistyPass Gateway Communication Protocol

> Protocol reference for communication between the MistyPass cloud API and gateway devices.
>
> Version: 1.0 | Last updated: 2026-04-30

---

## Table of Contents

1. [Overview](#overview)
2. [Authentication](#authentication)
3. [Bootstrap Flow](#bootstrap-flow)
4. [Configuration Sync](#configuration-sync)
5. [Credential Verification](#credential-verification)
6. [Event Reporting](#event-reporting)
7. [NATS Real-Time Messaging](#nats-real-time-messaging)
8. [OTA Firmware Updates](#ota-firmware-updates)
9. [Offline Operation](#offline-operation)
10. [Sequence Diagrams](#sequence-diagrams)

---

## Overview

MistyPass gateways are edge devices deployed in buildings to control physical door access. Each gateway communicates with the cloud API over HTTPS (REST) for provisioning, configuration, and event reporting, and over NATS for real-time commands and credential verification.

**Transport summary:**

| Channel | Purpose | Direction |
|---------|---------|-----------|
| HTTPS REST | Bootstrap, config sync, events, OTA | Gateway -> Cloud |
| NATS | Commands, live credential verify | Bidirectional |

All REST endpoints are prefixed with `/api/v1/gateway/`.

---

## Authentication

Three authentication mechanisms are used depending on the lifecycle stage of the gateway.

| Mechanism | Header | Format | Used For |
|-----------|--------|--------|----------|
| Bootstrap token | `X-Bootstrap-Token` | Opaque token | Initial registration (`/register`) only |
| Device token | `Authorization` | `Bearer gw_xxx` | All subsequent REST requests |
| Request nonce | Body field `nonce` | Unique per-request string | Anti-replay on `config/pull` and `events/batch` |

**Token lifecycle:**

1. A bootstrap token is provisioned out-of-band (e.g., flashed during manufacturing or provided by an installer).
2. On successful registration, the cloud issues a device token (`gw_xxx`).
3. The device token is used for all further communication. The bootstrap token must not be reused after registration.

---

## Bootstrap Flow

The bootstrap flow provisions a new gateway and transitions it from factory state to active.

### 1. Register

```
POST /api/v1/gateway/register
```

**Authentication:** `X-Bootstrap-Token`

**Request body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `serial_number` | string | Yes | Hardware serial number |
| `tenant_id` | string | Yes | Tenant this gateway belongs to |
| `building_id` | string | Yes | Building where the gateway is installed |
| `device_capacity` | object | Yes | Hardware capabilities (door count, reader types, etc.) |

**Response (201 Created):**

| Field | Type | Description |
|-------|------|-------------|
| `gateway_id` | string | Assigned gateway identifier |
| `device_token` | string | Bearer token for all subsequent requests (`gw_xxx`) |

### 2. Activate

```
POST /api/v1/gateway/activate
```

**Authentication:** `Bearer gw_xxx`

**Request body:** Empty or `{}`.

**Response (200 OK):**

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | `"active"` |

### 3. Heartbeat

```
POST /api/v1/gateway/heartbeat
```

**Authentication:** `Bearer gw_xxx`

Periodic health check sent by the gateway at a regular interval. The cloud uses heartbeats to detect offline gateways.

**Request body:** Device-defined health payload (uptime, memory, firmware version, etc.).

**Response (200 OK):** Acknowledged.

### 4. Status

```
POST /api/v1/gateway/status
```

**Authentication:** `Bearer gw_xxx`

Query or report the current operational status of the gateway.

**Response (200 OK):** Current status object.

---

## Configuration Sync

Configuration sync is a pull-based model. The gateway periodically requests its desired configuration from the cloud and confirms when it has been applied.

### Pull Configuration

```
POST /api/v1/gateway/config/pull
```

**Authentication:** `Bearer gw_xxx`

**Anti-replay:** Request must include a unique `nonce`.

**Response (200 OK):**

| Field | Type | Description |
|-------|------|-------------|
| `desired_version` | string | Version hash the cloud wants the gateway to run |
| `applied_version` | string | Version the cloud last saw the gateway confirm |
| `should_apply` | boolean | `true` if `desired_version != applied_version` |
| `bound_door_ids` | string[] | Door IDs this gateway controls |
| `devices` | object[] | Connected device descriptors (readers, locks, REX) |
| `authz_cache` | object | Authorization cache (see below) |
| `pending_ota_tasks` | object[] | OTA updates awaiting this gateway (see [OTA](#ota-firmware-updates)) |

#### Authorization Cache (`authz_cache`)

The `authz_cache` object contains everything the gateway needs to make offline access decisions.

| Field | Type | Description |
|-------|------|-------------|
| `access_rules` | object[] | Credential-to-user-to-lock mappings |
| `policies` | object[] | Access policies (schedules, overrides) |
| `users` | object[] | User records referenced by access rules |
| `user_groups` | object[] | Group memberships |
| `time_windows` | object[] | Named time windows used by policies |

#### Cache TTL and Staleness Policy

| Parameter | Value | Description |
|-----------|-------|-------------|
| Fresh TTL | 300 s | Cache is considered fresh |
| Max stale TTL | 900 s | Cache is usable but stale |
| Retry interval | 30 s | Retry interval when pull fails |
| Fallback mode | `use_last_acknowledged` | Use the last successfully applied config |
| No-cache behavior | `deny_all` | If no cache exists at all, deny every access attempt |

#### Version Hash

The `desired_version` field is a content-addressable hash of the full configuration. The gateway compares it against its local version to determine whether an update is needed, avoiding unnecessary processing of unchanged configs.

### Confirm Applied

```
POST /api/v1/gateway/config/applied
```

**Authentication:** `Bearer gw_xxx`

**Request body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `applied_version` | string | Yes | The version hash the gateway has successfully applied |

**Response (200 OK):** Acknowledged.

The cloud updates its record of the gateway's applied version. This closes the sync loop.

---

## Credential Verification

### Online Verification

```
POST /api/v1/gateway/verify-credential
```

**Authentication:** `Bearer gw_xxx`

Used when the gateway is online and wants the cloud to make the access decision (or to supplement local cache decisions).

**Request body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `credential_type` | string | Yes | One of: `nfc_uid`, `ble_token`, `qr_code`, `pin` |
| `credential_data` | string | Yes | The raw credential value |

**Response (200 OK):**

| Field | Type | Description |
|-------|------|-------------|
| `decision` | string | `"allow"` or `"deny"` |
| `auto_unlock` | boolean | Whether the gateway should immediately actuate the lock |

---

## Event Reporting

Gateways report access events, device events, and sync checkpoints back to the cloud.

### Single Access Event

```
POST /api/v1/gateway/events/access
```

**Authentication:** `Bearer gw_xxx`

Reports a single access event (badge tap, PIN entry, etc.).

**Request body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `event_type` | string | Yes | `"granted"` or `"denied"` |
| `credential_type` | string | Yes | `nfc_uid`, `ble_token`, `qr_code`, `pin` |
| `credential_data` | string | Yes | The credential value presented |
| `lock_id` | string | Yes | Which lock was involved |
| `occurred_at` | string | Yes | ISO 8601 timestamp |

### Single Device Event

```
POST /api/v1/gateway/events/device
```

**Authentication:** `Bearer gw_xxx`

Reports a device-level event.

**Request body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `event_type` | string | Yes | e.g., `"tamper"`, `"rex"` (request-to-exit) |
| `device_id` | string | Yes | Device that generated the event |
| `occurred_at` | string | Yes | ISO 8601 timestamp |

### Batch Events

```
POST /api/v1/gateway/events/batch
```

**Authentication:** `Bearer gw_xxx`

**Anti-replay:** Request must include a unique `nonce`.

Sends multiple events in a single request. Used primarily when syncing queued events after an offline period.

**Request body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `events` | object[] | Yes | Array of access and/or device events |
| `nonce` | string | Yes | Anti-replay nonce |

**Response (200 OK):**

| Field | Type | Description |
|-------|------|-------------|
| `results` | object[] | Per-event acceptance status |
| `retry_subset` | string[] | Event IDs that should be retried (transient failures) |

The gateway must retry only the events listed in `retry_subset`. Events not in `retry_subset` were accepted or permanently rejected.

### Checkpoint

```
POST /api/v1/gateway/events/checkpoint
```

**Authentication:** `Bearer gw_xxx`

Tracks event sync progress so the gateway and cloud agree on what has been delivered.

**Request body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `acked_count` | integer | Yes | Total number of events acknowledged so far |
| `last_occurred_at` | string | Yes | Timestamp of the most recent acknowledged event |

---

## NATS Real-Time Messaging

NATS subjects are scoped per gateway using the pattern `mistypass.gateway.{gateway_id}.*`.

### Subjects

| Subject | Direction | Payload Type |
|---------|-----------|--------------|
| `mistypass.gateway.{gw_id}.command` | Cloud -> Gateway | `GatewayCommand` |
| `mistypass.gateway.{gw_id}.event` | Gateway -> Cloud | `GatewayEvent` |
| `mistypass.gateway.{gw_id}.verify` | Gateway -> Cloud | `CredentialVerifyRequest` |
| `mistypass.gateway.{gw_id}.verify_result` | Cloud -> Gateway | `CredentialVerifyResponse` |

### GatewayCommand

Sent by the cloud to instruct the gateway to perform an action.

| Field | Type | Description |
|-------|------|-------------|
| `request_id` | string | Unique request identifier for correlation |
| `command` | string | `"unlock"`, `"lock_down"`, or `"cancel_lockdown"` |
| `lock_id` | string | Target lock |
| `place_id` | string | Place context |
| `tenant_id` | string | Tenant context |
| `issued_by` | string | User or system that issued the command |
| `issued_at` | string | ISO 8601 timestamp |

### GatewayEvent

Sent by the gateway to report command results and access activity in real time.

| Field | Type | Description |
|-------|------|-------------|
| `request_id` | string | Correlates with the originating command (if any) |
| `event_type` | string | `"command_ack"`, `"access_granted"`, `"access_denied"`, `"heartbeat"` |
| `command` | string | The command being acknowledged (for `command_ack`) |
| `status` | string | `"success"`, `"failed"`, or `"timeout"` |
| `error` | string | Error description (if `status` is not `success`) |

### CredentialVerifyRequest

Sent by the gateway to request a real-time access decision from the cloud.

| Field | Type | Description |
|-------|------|-------------|
| `request_id` | string | Unique identifier; used to match the response |
| `reader_id` | string | Reader that captured the credential |
| `lock_id` | string | Lock associated with the reader |
| `credential_type` | string | `nfc_uid`, `ble_token`, `qr_code`, `pin` |
| `credential_data` | string | Raw credential value |

### CredentialVerifyResponse

Cloud reply on the `verify_result` subject.

| Field | Type | Description |
|-------|------|-------------|
| `request_id` | string | Matches the originating request |
| `decision` | string | `"allow"` or `"deny"` |
| `reason` | string | Human-readable explanation |
| `user_id` | string | Resolved user (if `allow`) |

---

## OTA Firmware Updates

### Lifecycle

OTA updates follow a state machine:

```
queued --> dispatching --> succeeded
                      \-> failed
                      \-> canceled
```

### Flow

1. An administrator creates an OTA task through the management API, specifying the target firmware.
2. The gateway discovers the pending task via the `pending_ota_tasks` field in `config/pull`.
3. The gateway downloads, verifies, and installs the firmware.
4. The gateway reports the outcome.

### OTA Task Fields (delivered via `config/pull`)

| Field | Type | Description |
|-------|------|-------------|
| `firmware_version` | string | Target firmware version |
| `firmware_url` | string | URL to download the firmware binary |
| `firmware_sha256` | string | SHA-256 hash of the firmware binary |
| `firmware_signature` | string | Ed25519 signature (hex-encoded) |

The gateway must:

1. Download the binary from `firmware_url`.
2. Verify the SHA-256 hash matches `firmware_sha256`.
3. Verify the Ed25519 signature in `firmware_signature` against a trusted public key.
4. Only then apply the update.

### Report OTA Result

```
POST /api/v1/gateway/ota/report
```

**Authentication:** `Bearer gw_xxx`

**Request body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `firmware_version` | string | Yes | The version that was attempted |
| `status` | string | Yes | `"succeeded"` or `"failed"` |
| `error` | string | No | Error details if `status` is `"failed"` |

---

## Offline Operation

Gateways are designed to operate autonomously when the cloud is unreachable.

### Authorization Cache Behavior

```
[Cloud reachable]
    |
    v
Pull config/pull --> cache authz_cache locally
    |
    +--> Cache age < 300s  --> FRESH: use with full confidence
    +--> Cache age < 900s  --> STALE: use, but keep retrying pull
    +--> Cache age >= 900s --> EXPIRED: fall back to last acknowledged version
    +--> No cache at all   --> DENY ALL access attempts
```

| Scenario | Behavior |
|----------|----------|
| Cache is fresh (< 300 s) | Normal operation using cached rules |
| Cache is stale (300-900 s) | Continue using cache; retry pull every 30 s |
| Cache expired (> 900 s) | Use last acknowledged config version (`fallback_mode: use_last_acknowledged`) |
| No cache exists | Deny all access (`no_cache_behavior: deny_all`) |

### Event Queuing

When offline, the gateway queues events locally. On reconnection:

1. Send queued events via `POST /api/v1/gateway/events/batch`.
2. Retry any events returned in `retry_subset`.
3. Confirm sync progress via `POST /api/v1/gateway/events/checkpoint`.

---

## Sequence Diagrams

### Gateway Bootstrap

```
Gateway                           Cloud API
   |                                  |
   |-- POST /register -------------->|  (X-Bootstrap-Token)
   |<-------- gateway_id + token ----|
   |                                  |
   |-- POST /activate -------------->|  (Bearer gw_xxx)
   |<------------- status=active ----|
   |                                  |
   |-- POST /config/pull ----------->|
   |<---------- config + authz ------| 
   |                                  |
   |-- POST /config/applied -------->|
   |<-------------- ack -------------|
   |                                  |
   |== OPERATIONAL ===================|
```

### Access Decision (Online)

```
User        Reader       Gateway                Cloud API
 |             |            |                       |
 |--tap/scan-->|            |                       |
 |             |--credential-->|                    |
 |             |            |-- POST /verify ------>|
 |             |            |<--- allow/deny -------|
 |             |            |                       |
 |             |<--unlock/deny-|                    |
 |             |            |-- POST events/access->|
 |<--door----->|            |                       |
```

### Access Decision (Offline, Cached)

```
User        Reader       Gateway
 |             |            |
 |--tap/scan-->|            |
 |             |--credential-->|
 |             |            |-- lookup authz_cache (local)
 |             |<--unlock/deny-|
 |             |            |-- queue event locally
 |<--door----->|            |
```

### Real-Time Command via NATS

```
Admin         Cloud API            NATS             Gateway
  |               |                  |                  |
  |--unlock cmd-->|                  |                  |
  |               |--GatewayCommand->|                  |
  |               |                  |--GatewayCommand->|
  |               |                  |                  |--actuate lock
  |               |                  |<--GatewayEvent---|
  |               |<--GatewayEvent---|                  |
  |<---result-----|                  |                  |
```

### OTA Update

```
Admin         Cloud API            Gateway
  |               |                   |
  |--create OTA-->|                   |
  |               |                   |
  |               |   (next config/pull cycle)
  |               |<-- config/pull ---|
  |               |--- pending_ota -->|
  |               |                   |-- download firmware
  |               |                   |-- verify SHA-256
  |               |                   |-- verify Ed25519 sig
  |               |                   |-- apply update
  |               |                   |-- reboot
  |               |<-- ota/report ----|
  |<--status------|                   |
```

### Offline Reconnection

```
Gateway                           Cloud API
   |                                  |
   |  (offline period: events queued) |
   |                                  |
   |-- POST /heartbeat ------------->|  (reconnected)
   |<-------------- ack -------------|
   |                                  |
   |-- POST /events/batch ---------->|  (queued events + nonce)
   |<--- results + retry_subset -----|
   |                                  |
   |-- POST /events/batch ---------->|  (retry_subset only)
   |<--- results --------------------|
   |                                  |
   |-- POST /events/checkpoint ----->|  (acked_count, last_occurred_at)
   |<-------------- ack -------------|
   |                                  |
   |-- POST /config/pull ----------->|  (refresh cache)
   |<---------- config + authz ------|
```
