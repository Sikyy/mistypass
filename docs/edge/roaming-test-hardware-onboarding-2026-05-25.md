# Roaming-Test Hardware Onboarding - 2026-05-25

> 能力状态：CONTRACT_READY
> W0 status: ready for topology/gateway registration and 2-channel opto-isolated relay module active-level verification; lock-body testing waits for lamp/buzzer relay evidence.

This runbook captures the first real Roaming bench door. It extends the W0
bench freeze without changing the original Jakarta demo baseline.

The executable chain test sequence is tracked in
[Roaming-Test link test plan](roaming-test-link-test-plan-2026-05-25.md).

## Resource Plan

| Resource | Value | Notes |
| --- | --- | --- |
| Tenant | `tenant_demo_jakarta` | Staging/demo tenant for the first physical bench run. |
| Building display name | `Roaming Building` | Create in Admin/API; keep generated ID in evidence. |
| Floor display name | `Roaming F1` | Use a simple floor so the door can be mounted under topology. |
| Area display name | `Roaming Entry` | Create under `Roaming Building` / `Roaming F1`. |
| Door display name | `Roaming-Test` | First real bench door point. |
| Gateway serial | `MP-GW-W0-20260524-001` | Provisional W0 label until the Orange Pi serial/MAC is recorded. |
| Gateway ID | API generated | Returned by `POST /api/v1/gateway/register`. |
| Reader label | `reader_roaming_proid10bm_001` | ZKTeco PROID10BM 13.56MHz Wiegand reader. |
| Camera label | `camera_roaming_ds2cd1023g2_001` | Hikvision DS-2CD1023G2-LIU-LIUF. |
| Relay label | `relay_roaming_test_001` | 2-channel opto-isolated relay module confirmed; active level still to verify. |

## Hardware Inventory

| Component | Current detail | W0 action |
| --- | --- | --- |
| Edge controller | Orange Pi, exact serial unknown | Use provisional serial first; record `/proc/cpuinfo` serial or network MAC when available. |
| Reader | ZKTeco PROID10BM 13.56MHz | Wire as Wiegand 26/34 after measuring D0/D1 idle voltage. |
| Camera | Hikvision DS-2CD1023G2-LIU-LIUF | Register after LAN IP/credentials or Hik-Connect serial/verification code are known. |
| Lock | EM Lock 600 LBS, type B, 12VDC 400mA | Do not connect in W0; first validate the 2-channel opto-isolated relay module with lamp/buzzer. |
| Lock status wires | `NO`, `NC`, `COM` on type B lock | Treat as lock/bond/status feedback, not as the gateway-controlled relay. |
| Lock power | 12V 3A switching PSU, 220VAC input, 12VDC output, 36W max | Enough for one 400mA maglock and reader power, but keep SBC power separate. |
| Relay module | 2-channel opto-isolated relay module, 3.3V control, 5V supply | Has `NO/COM/NC`; no new relay purchase needed unless active-level testing fails. |

## Registration Flow

1. Create topology: `Roaming Building` -> `Roaming F1` -> `Roaming Entry` -> `Roaming-Test`.
2. Import the provisional gateway serial into serial inventory.
3. Bootstrap register the gateway with `X-Bootstrap-Token`.
4. Bind the generated gateway ID to the generated `Roaming-Test` door ID.
5. Publish gateway config, then have the gateway pull/apply it with its device token.
6. Only after 2-channel opto-isolated relay module lamp/buzzer output passes, move to lock-body wiring.

Example API order:

```zsh
API_BASE_URL=https://staging-api.mistyislet.com
TENANT_ID=tenant_demo_jakarta
GW_SERIAL=MP-GW-W0-20260524-001

# 1. Login and keep access_token as AT.
# 2. POST /api/v1/buildings, /api/v1/floors, /api/v1/areas, /api/v1/doors.
# 3. POST /api/v1/gateways/serial-inventory/import.
# 4. POST /api/v1/gateway/register with X-Bootstrap-Token.
# 5. POST /api/v1/gateways/{gateway_id}/bind-door?tenant_id=$TENANT_ID.
# 6. POST /api/v1/gateways/{gateway_id}/config/publish?tenant_id=$TENANT_ID.
```

Keep the generated IDs in the evidence table:

| Field | Value |
| --- | --- |
| `building_id` |  |
| `floor_id` |  |
| `area_id` |  |
| `door_id` |  |
| `gateway_id` |  |
| `device_token` stored on gateway | yes/no |
| `camera_id` |  |

## Wiring Sequence

### Wiegand reader

Use the existing Orange Pi Zero 3 Wiegand baseline:

| PROID10BM wire | Orange Pi path |
| --- | --- |
| `D0` green | PC9 / GPIO 73 with 10k pull-up to 3.3V |
| `D1` white | PC10 / GPIO 74 with 10k pull-up to 3.3V |
| `GND` black | Orange Pi GND |
| `+12V` red | External 12V supply |

Before connecting to GPIO, measure D0/D1 idle voltage. It must be at or below
3.3V. If the reader has internal 5V pull-ups, add a divider or level shifter.

### Relay and lock

The 2-channel opto-isolated relay module is the gateway-controlled switch. The `NO/NC/COM` wires on the
type B maglock should not be treated as a substitute relay for the Orange Pi.

W0 lamp/buzzer test:

| 2-channel relay module terminal | Connect to |
| --- | --- |
| `IN` | Orange Pi GPIO selected for the 2-channel opto-isolated relay module |
| `VCC` | 3.3V or 5V, matching the relay module spec |
| `GND` | Orange Pi GND |
| `COM` / `NO` | Low-voltage lamp or buzzer test circuit |

W1 fail-safe maglock test, after W0 passes:

| Power path | Connect to |
| --- | --- |
| 12V PSU `+` | 2-channel opto-isolated relay module `COM` |
| 2-channel opto-isolated relay module `NC` | Lock `V+` |
| Lock `V-` | 12V PSU `-` |

Idle relay module means `NC` stays closed and the maglock remains powered/locked.
Unlock energizes the 2-channel opto-isolated relay module, opens `NC`, cuts lock power, and releases the door.

## Camera Registration Notes

The camera can be recorded now by model, but API registration needs one of these:

- LAN mode: camera IP/host, port, username, password.
- Hik-Connect/ISC mode: cloud serial, verification code, channel count, and cloud
  account/provider configuration.

Do not block the first door 2-channel opto-isolated relay module run on camera cloud playback. Bind the camera
to `Roaming-Test` after the door/gateway path is stable.

## Required Next Data

| Needed | Why |
| --- | --- |
| Actual Orange Pi serial or MAC | Replace provisional `MP-GW-W0-20260524-001` before pilot evidence is finalized. |
| 2-channel opto-isolated relay module active level | Confirm whether the board is active-low or active-high before connecting the lock. |
| D0/D1 measured idle voltage | Protect Orange Pi GPIO. |
| Camera LAN IP or Hik-Connect serial/verification code | Register and test Hikvision camera. |
| Lock status contact behavior | Decide whether type B `NO/NC/COM` should feed door/lock status input. |
