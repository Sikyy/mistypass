# Roaming-Test Link Test Plan - 2026-05-25

> 能力状态：CONTRACT_READY
> Scope: Roaming-Test first-door chain test, from Cloud API contract to real relay, reader, lock, camera, and mobile smoke.

This plan turns the previous hardware notes into one executable sequence. It
keeps completed contract/dry-run evidence separate from physical tests that
still need the Roaming-Test bench.

## References

- [Hardware integration guide](../hardware-integration-guide.md)
- [Edge MVP validation report](../testing/artifacts/edge-mvp-validation-20260417-033943-5527.md)
- [MVP device validation plan](mvp-device-validation-plan.md)
- [MVP device validation runbook](mvp-device-validation-runbook.md)
- [Hardware Bench W0 Freeze](hardware-bench-w0-freeze-2026-05-24.md)
- [Roaming-Test hardware onboarding](roaming-test-hardware-onboarding-2026-05-25.md)
- [Wiegand reader design](../superpowers/specs/2026-05-15-wiegand-reader-design.md)

## Target Chain

```text
Mobile / card / admin unlock
  -> MistyPass Cloud API
  -> gateway config/authz cache
  -> Orange Pi gateway-agent
  -> relay output
  -> EM lock power cut/release
  -> event upload/checkpoint
  -> Admin/mobile/report evidence
```

## Roaming-Test Bench Inventory

| Item | Current value | Status |
| --- | --- | --- |
| Tenant | `tenant_demo_jakarta` | ready |
| Building | `Roaming Building` | to create/record generated ID |
| Floor | `Roaming F1` | to create/record generated ID |
| Area | `Roaming Entry` | to create/record generated ID |
| Door | `Roaming-Test` | to create/record generated ID |
| Gateway | Orange Pi, provisional serial `MP-GW-W0-20260524-001` | serial/MAC to confirm |
| Relay module | 2-channel opto-isolated relay shown in photo, `NO/COM/NC`, 5V supply, 3.3V control | usable candidate; active level to verify |
| Reader | ZKTeco `PROID10BM 13.56MHz` | to wire and measure D0/D1 |
| Camera | Hikvision `DS-2CD1023G2-LIU-LIUF` | LAN/cloud binding data needed |
| Lock | Type B EM Lock 600 LBS, 12VDC 400mA | do not connect before relay lamp/buzzer pass |
| Lock power | 12V 3A / 36W switching PSU | enough for one lock; keep SBC power separate |

## Completed Baseline

| Area | Evidence | Result | Remaining gap |
| --- | --- | --- | --- |
| Cloud gateway contract | `docs/testing/artifacts/edge-mvp-validation-20260417-033943-5527.md` | 7 scripts PASS, fail_count 0 | Still needs Roaming-Test resource IDs and physical output evidence. |
| Gateway serial/protocol API | `curl-gateway-serial-protocol-20260417-033943-5527.log` | PASS | Use real/provisional Roaming serial and record generated gateway ID. |
| Legacy Wiegand POC contract | `curl-gateway-legacy-wiegand-poc-20260417-033943-5527.log` | PASS | Real PROID10BM GPIO frame capture still pending. |
| Door I/O event contract | `curl-gateway-door-io-loop-20260417-033943-5527.log` | PASS | Real relay/REX/tamper/door-contact inputs still pending. |
| Event idempotency/checkpoint/retry | Edge validation artifact logs | PASS | Repeat after Roaming gateway sends real events. |
| WalletMate II dry-run reader test | `docs/hardware-integration-guide.md` live test record | PASS with DryRunRelay | It proved PC/SC UID flow, not physical relay/lock. |
| Camera provider code path | `docs/hardware-integration-guide.md` Hikvision section | ISAPI path documented/implemented | Real camera IP/credentials or Hik-Connect binding still pending. |
| Mobile staging API smoke | `docs/testing/mobile-staging-manual-walkthrough-2026-05-24.md` | Android/iOS core staging paths tested | Unlock should wait until safe Roaming door output path is verified. |

## Test Phases

| Phase | Test | Preconditions | Steps | Pass criteria |
| --- | --- | --- | --- | --- |
| T0 | Safety photo and label capture | All hardware visible on bench | Photograph Orange Pi, relay, reader, lock PSU, lock, camera labels, and wiring before power-on. | Photos stored; model/serial/MAC table filled. |
| T1 | Relay module electrical sanity | Relay not connected to lock | Power relay module, connect Orange Pi GND, choose a GPIO not used by Wiegand, measure idle and trigger states with multimeter. | Startup default is safe/off; trigger state and active level are known. |
| T2 | Relay lamp/buzzer pulse | T1 pass | Connect relay output to lamp/buzzer or multimeter continuity only. Run gateway-agent with `-relay-gpio <relay_gpio>` and trigger a test unlock. | Relay/load pulses for `unlock-duration`, then returns safe/off. |
| T3 | Roaming topology create | Staging API healthy | Create `Roaming Building`, `Roaming F1`, `Roaming Entry`, `Roaming-Test`. | Generated IDs recorded in evidence table. |
| T4 | Gateway inventory/register/bind | T3 pass, bootstrap token available | Import `MP-GW-W0-20260524-001`, register gateway, bind to Roaming-Test, publish config, pull/apply config. | Gateway ID/device token recorded; config pull shows Roaming-Test bound door. |
| T5 | Gateway-agent online smoke | T4 pass | Start agent against staging with token file, relay GPIO, and short poll interval. | Heartbeat/config pull succeeds; Admin/API shows gateway online or recent status. |
| T6 | Wiegand voltage and frame test | T1/T4 pass, reader powered by external 12V | Measure D0/D1 idle voltage, wire `D0=GPIO73`, `D1=GPIO74` only if <=3.3V, then present a fixed-UID card. | Facility/card number or raw Wiegand frame is recorded without GPIO overvoltage. |
| T7 | Credential allow/deny | T6 pass, fixed test card available | Register/activate one allowed credential, then test allowed card, unknown card, revoked/expired card. | Allow triggers relay pulse; deny does not move relay and logs denial reason. |
| T8 | Lock-body fail-safe test | T2/T7 pass | Wire `12V+ -> COM`, `NC -> lock V+`, `lock V- -> 12V-`. Trigger one unlock while physically holding door safe. | Idle state locks; unlock cuts power and releases; relock restores power. |
| T9 | 30-cycle single-door run | T8 pass | Run 30 remote/card unlocks with event upload enabled. | 30/30 successful relay actions; no duplicate events; no stuck relay. |
| T10 | Offline/reconnect recovery | T9 pass | Disconnect network for 2 minutes, perform allowed/denied tests, reconnect. | Local decision works from cache; queued events upload and checkpoint after reconnect. |
| T11 | Hikvision bind/snapshot | Door path stable | Register camera using LAN IP/credentials or Hik-Connect data and bind to Roaming-Test. | Snapshot/video-link/cloud-recordings path returns expected real or empty state. |
| T12 | Mobile smoke | T8 or T9 pass | Use Android/iOS staging app with safe Roaming-Test door selected. | Door appears; one unlock works; event appears; iOS pass list remains platform-filtered. |
| T13 | Report/evidence closeout | T9/T11/T12 pass | Export hardware/report evidence and attach logs/photos. | Evidence table complete and W1 entry criteria satisfied. |

## GPIO Allocation

Do not reuse the old `relay-gpio 73` example when the ZKTeco reader is attached.
For Roaming-Test:

| Function | GPIO | Notes |
| --- | --- | --- |
| Wiegand D0 | `73` | Planned PC9 input with 10k pull-up to 3.3V. |
| Wiegand D1 | `74` | Planned PC10 input with 10k pull-up to 3.3V. |
| Relay IN | TBD | Must be a separate output GPIO; verify Orange Pi header/sysfs number. |

## Relay Candidate Checks

The relay board in the provided photo appears suitable only if these checks pass:

| Check | Expected |
| --- | --- |
| Output terminals | `NO`, `COM`, `NC` present for each relay channel. |
| Module supply | `5V+` / `5V-`, current at least 100mA. |
| Control input | 3.3V control input works from Orange Pi GPIO. |
| Ground reference | Orange Pi GND and relay module `GND`/`5V-` share reference. |
| Active level | Known before connecting lock. Current gateway-agent GPIO relay is active-low by default. |
| Load rating | At least above 12VDC 0.4A; photo indicates DC30V 10A resistive. |

If the board is active-high, either add an inverter/driver or update the
gateway-agent to support an explicit active-high relay mode before lock-body
testing.

## Evidence Table

| Evidence | Value/path | Result | Notes |
| --- | --- | --- | --- |
| Bench photo |  |  | Include all powered devices before wiring lock. |
| Relay module photo |  |  | Must show terminal labels and jumper/trigger labels. |
| Orange Pi serial/MAC |  |  | Replace provisional serial if available. |
| Relay GPIO |  |  | Must not be 73 or 74 when Wiegand is connected. |
| Relay active level |  |  | active-low / active-high. |
| T1 multimeter readings |  |  | GPIO idle/trigger, relay COM-NC/COM-NO state. |
| T2 lamp/buzzer video |  |  | Before lock-body wiring. |
| Roaming generated IDs |  |  | `building_id`, `floor_id`, `area_id`, `door_id`, `gateway_id`. |
| Config pull/apply log |  |  | Include `authz_cache_version` and bound door. |
| Wiegand voltage photo |  |  | D0/D1 idle <=3.3V. |
| Wiegand card read log |  |  | Facility/card number or raw frame. |
| Credential allow/deny log |  |  | allowed, unknown, revoked/expired. |
| Lock-body video |  |  | Show idle locked, unlock release, relock. |
| 30-cycle run log |  |  | 30/30 required for W1. |
| Offline recovery log |  |  | Queue replay and checkpoint after reconnect. |
| Camera bind/snapshot log |  |  | LAN or Hik-Connect mode. |
| Mobile smoke notes |  |  | Android/iOS app version, user, door, result. |

## Stop Conditions

- Any GPIO, Wiegand D0/D1, or relay input line measures above 3.3V before a
  level shifter/divider is added.
- Relay default state would unlock or energize the wrong path at boot.
- Relay active level is unknown.
- The lock is wired before lamp/buzzer pulse evidence is captured.
- A failed unlock leaves relay or lock power stuck in the unsafe state.
- Camera, report, or mobile demo data is mistaken for Roaming-Test physical evidence.

## Immediate Next Actions

1. Confirm relay channel terminal labels and active level with no lock attached.
2. Pick and document a relay GPIO that does not conflict with Wiegand `73/74`.
3. Create Roaming topology/resources and record generated IDs.
4. Register/bind the gateway using provisional serial `MP-GW-W0-20260524-001`.
5. Run relay lamp/buzzer pulse before connecting the EM lock.
