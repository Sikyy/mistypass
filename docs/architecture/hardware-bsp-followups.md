# Hardware and BSP Security Follow-Ups

<!-- BLOCKED_EXTERNAL -->

Updated: 2026-05-11

This document holds follow-up work that is not pure backend or cloud software. Keep hardware root of trust, BSP, physical topology, reader wiring, and enclosure requirements here so the software security status stays accurate.

## Scope

In scope:

- Secure boot and bootloader trust chain.
- Signed OTA verification anchored in hardware-backed trust.
- Factory key injection and certificate provisioning process.
- Reader/controller physical separation and relay placement.
- Tamper detection, enclosure, power-loss behavior, and field service controls.
- OSDP Secure Channel and downstream reader key management.

Out of scope:

- Cloud API request authentication, mTLS listener, WebSocket channel, event replay, and credential sync. These are tracked in `docs/architecture/gateway-security-software-status.md`.

## Follow-Up Board

| Item | Target outcome | Dependency | Priority |
| --- | --- | --- | --- |
| Secure Boot on target SoC | Boot only signed bootloader/kernel/rootfs | i.MX93 HABv4 or final SoC BSP | Production blocker |
| Signed OTA with rollback protection | Firmware update cannot install unsigned or downgraded images | Secure Boot, boot slots, monotonic version storage | Production blocker |
| Factory key injection | Device private keys are generated or injected without developer access to production secrets | Manufacturing process, HSM or secure element decision | Production blocker |
| Gateway client cert storage | Client certificate private key is protected at rest | Secure element, TPM, or SoC secure storage | High |
| Reader/controller default separation | Door-side reader compromise does not expose relay control | Hardware SKU and installation standard | High for high-security sites |
| Relay placement inside secure side | Relay and strike wiring remain behind controlled area | Installation standard and enclosure design | High |
| Tamper input and enclosure sensor | Gateway records and reports enclosure open/tamper events | GPIO design, sensor BOM, event mapping | Medium |
| OSDP Secure Channel provisioning | Downstream reader links use managed secure channel keys | Reader model support, key ceremony | Medium |
| Power-loss and UPS behavior | Door state and event queue behavior are deterministic during outage | Power design, UPS option, field test | Medium |
| Field recovery mode | Recovery cannot bypass secure boot or leak production credentials | BSP recovery flow, factory reset policy | Medium |

## Product Decisions Needed

- Decide whether the default production topology is single gateway with relay or separated reader plus secure-side controller.
- Decide if high-security deployments require OSDP Secure Channel by default.
- Decide final SoC and BSP baseline before committing secure boot implementation details.
- Decide whether production keys live in SoC secure storage, discrete secure element, TPM, or manufacturing HSM flow.
- Decide OTA slot layout, rollback policy, and field recovery path.

## Acceptance Criteria

Before claiming production hardware security parity with mature access-control systems, the device program should show:

- Verified secure boot chain from immutable ROM or equivalent hardware root to application.
- Firmware update package signature verification and downgrade prevention.
- Documented key injection process with separation between manufacturing, cloud CA, and field service.
- Test evidence that relay control is not reachable from the insecure side in the separated topology.
- Tamper and power-loss test logs uploaded through the existing gateway event/audit path.

## Relationship to Software Work

The backend can already issue gateway client certificates, authenticate device requests, enforce HTTP replay nonce headers, push realtime gateway messages, and ingest offline events. Hardware work should reuse those contracts instead of introducing a parallel cloud protocol unless a measured field constraint requires it.
