# Matter over Thread Relay Driver for Gateway-Agent

**Date:** 2026-05-14
**Status:** Approved
**Scope:** Add Matter Door Lock cluster integration to gateway-agent via chip-tool subprocess

---

## Goal

Enable MistyPass gateway-agent to control Matter-compatible smart locks (Aqara U200/U300/U400, etc.) over Thread, using the official `chip-tool` CLI as the Matter controller. This extends the existing `RelayDriver` interface with a new `MatterRelay` backend.

## Architecture

```
gateway-agent (Go)
├── MatterRelay          implements RelayDriver
│   ├── Unlock()         chip-tool doorlock unlock-door
│   ├── Lock()           chip-tool doorlock lock-door
│   └── Close()          stops subscription process
├── MatterController     manages chip-tool lifecycle
│   ├── Exec()           one-shot commands (commission, unlock, credential sync)
│   ├── Interactive      long-lived process (status subscription)
│   ├── Commission()     automatic pairing
│   └── SyncCredentials  PIN/user sync to lock
└── agent.go
    └── relay = NewMatterRelay(...)
```

The gateway-agent connects to Matter locks via an existing Thread Border Router (Apple TV, HomePod, or standalone OTBR) on the local network. The gateway itself does not act as a TBR in this iteration.

## New Files

| File | Responsibility |
|------|---------------|
| `cmd/gateway-agent/matter_relay.go` | `MatterRelay` struct implementing `RelayDriver` + extended Door Lock cluster methods |
| `cmd/gateway-agent/matter_controller.go` | `MatterController` — chip-tool subprocess management, output parsing, interactive session |
| `cmd/gateway-agent/matter_relay_test.go` | Unit tests with mock chip-tool output |

## Command-Line Flags

Following existing flag patterns in `main.go`:

```
-relay-matter <node-id>           Enable Matter relay with target node ID (uint64)
-matter-endpoint <int>            Door Lock cluster endpoint (default: 1)
-matter-storage <dir>             chip-tool fabric credential directory
                                  (default: /var/lib/mistypass/matter/)
-matter-setup-code <string>       Setup code for commissioning (e.g. MT:Y3.13OTB00KA0648G00)
                                  Used on first start; ignored after successful commission
-matter-chip-tool <path>          chip-tool binary path (default: searches PATH)
-matter-timed-timeout <ms>        timedInteractionTimeoutMs for Door Lock cluster commands
                                  (default: 10000; increase for high-latency Thread networks)
```

## Driver Priority

In `agent.go`, relay selection order becomes:

```
GPIO > OSDP > RS-485 > Matter > DryRun
```

```go
case a.relayMatterNodeID > 0:
    ctrl, err := NewMatterController(a.matterChipToolPath, a.matterStorageDir, a.logger)
    if err != nil { return err }
    a.relay, err = NewMatterRelay(ctrl, a.relayMatterNodeID, a.matterEndpoint, a.logger)
```

## MatterController

Encapsulates all chip-tool subprocess interaction.

```go
type MatterController struct {
    binary     string
    storageDir string
    logger     *slog.Logger

    // Interactive session for subscriptions
    mu         sync.Mutex
    session    *exec.Cmd
    stdin      io.WriteCloser
    scanner    *bufio.Scanner
}
```

### Exec — One-Shot Commands

```go
func (c *MatterController) Exec(ctx context.Context, args ...string) (string, error)
```

Runs `chip-tool <args...> --storage-directory <dir>` as a subprocess. Returns stdout. Timeout from context. Parses exit code and stderr for errors.

### Interactive Session — Subscribe Only

```go
func (c *MatterController) StartSubscription(ctx context.Context, subscribeCmd string) (*bufio.Scanner, error)
func (c *MatterController) StopSubscription() error
```

Manages a long-lived `chip-tool interactive start --storage-directory <dir>` process dedicated exclusively to subscriptions. On startup, a single `doorlock subscribe` command is sent via stdin; the returned scanner streams lock-state change events from stdout.

**Concurrency rule:** The interactive session is subscribe-only. All other commands (unlock, lock, commission, set-user, set-credential) go through `Exec()`. This avoids stdout interleaving — the interactive session's stdout contains only subscription event data, never mixed with command responses.

### Output Parsing

chip-tool writes structured log lines to stdout. Key patterns:

| Pattern | Meaning |
|---------|---------|
| `CHIP:DMG: status = 0x0` | Command succeeded |
| `CHIP:DMG: status = 0x01` | FAILURE |
| `CHIP:DMG: status = 0x7e` | UNSUPPORTED_COMMAND |
| `LockState = N` | Lock state attribute (1=Locked, 2=Unlocked, 3=Unlatched) |
| `CHIP:TOO: Endpoint: N Cluster: 0x0000_0101` | Door Lock cluster response |

Parsing uses regex on stdout lines; unrecognized output is logged at debug level.

**Version compatibility:** On startup, `MatterController` runs `chip-tool version` and logs the result. Supported range: chip-tool **1.4.x – 1.5.x** (Matter 1.4 / 1.5 SDK). If the version is outside this range, log a warning but do not block — output patterns may still work. The regex patterns above are validated against these versions.

## MatterRelay

```go
type MatterRelay struct {
    ctrl       *MatterController
    nodeID     uint64
    endpoint   int
    logger     *slog.Logger

    // Re-lock scheduling: prevents goroutine races on rapid Unlock() calls.
    // Each Unlock() cancels any pending re-lock timer before scheduling a new one.
    relockMu    sync.Mutex
    relockTimer *time.Timer

    onStateChange func(state DoorLockState)
    cancelSub     context.CancelFunc
}
```

### RelayDriver Interface

```go
func (m *MatterRelay) Unlock(duration time.Duration) error
```

1. Executes: `doorlock unlock-door <nodeID> <endpoint> --timedInteractionTimeoutMs <configured>`
2. Parses stdout for `status = 0x0`
3. Re-lock scheduling (race-safe):
   ```go
   m.relockMu.Lock()
   if m.relockTimer != nil {
       m.relockTimer.Stop() // cancel any previous pending re-lock
   }
   m.relockTimer = time.AfterFunc(duration, func() { m.Lock() })
   m.relockMu.Unlock()
   ```
   This ensures rapid successive Unlock() calls never produce competing Lock() goroutines.

```go
func (m *MatterRelay) Close() error
```

1. Cancels any active subscription
2. Calls `ctrl.StopInteractive()`

### Extended Methods

```go
func (m *MatterRelay) Lock() error
```
Executes: `doorlock lock-door <nodeID> <endpoint> --timedInteractionTimeoutMs 10000`

```go
func (m *MatterRelay) GetLockState() (DoorLockState, error)
```
Executes: `doorlock read lock-state <nodeID> <endpoint>`
Parses `LockState = N` from output.

```go
func (m *MatterRelay) Commission(setupCode string) error
```
Executes: `pairing onnetwork <nodeID> <setupCode>`
For Thread devices already on network via TBR.

```go
func (m *MatterRelay) SubscribeLockState(ctx context.Context, callback func(DoorLockState)) error
```
Via interactive session: `doorlock subscribe lock-state <minInterval> <maxInterval> <nodeID> <endpoint>`
Parses `LockState` changes from stdout stream, invokes callback.

```go
func (m *MatterRelay) SetUser(user MatterUser) error
func (m *MatterRelay) SetCredential(cred MatterCredential) error
func (m *MatterRelay) DeleteUser(userIndex uint16) error
```
Maps to `doorlock set-user`, `doorlock set-credential`, `doorlock clear-user` chip-tool commands. All use `--timedInteractionTimeoutMs 10000` (Door Lock cluster security requirement).

## Data Types

```go
type DoorLockState uint8

const (
    DoorLockStateNotFullyLocked DoorLockState = 0
    DoorLockStateLocked         DoorLockState = 1
    DoorLockStateUnlocked       DoorLockState = 2
    DoorLockStateUnlatched      DoorLockState = 3
    DoorLockStateUndefined      DoorLockState = 0xFF
)

type MatterUser struct {
    UserIndex  uint16
    UserName   string
    UniqueID   uint32
    UserStatus uint8  // 1=OccupiedEnabled, 3=OccupiedDisabled
    UserType   uint8  // 0=UnrestrictedUser, 1=YearDayScheduleUser, ...
}

type MatterCredential struct {
    UserIndex       uint16
    CredentialIndex uint16
    CredentialType  uint8  // 1=PIN, 2=RFID, 3=Fingerprint, 4=FingerVein
    CredentialData  []byte // PIN as UTF-8 bytes, RFID/biometric as raw bytes
}
```

## Auto-Commissioning Flow

On Agent start, if `-relay-matter` is set:

1. Check if `storageDir` contains existing fabric data (chip-tool persists after first commission)
2. If fabric exists → skip commissioning, proceed to normal operation
3. If no fabric + `-matter-setup-code` provided:
   a. Run `chip-tool pairing onnetwork <nodeID> <setupCode>`
   b. On success → log "Matter device commissioned" + save state
   c. On failure → **fatal error, refuse to start** (do not silently degrade to DryRun — a failed commission indicates a real environment problem that requires operator attention)
4. If no fabric + no setup code → **fatal error, refuse to start** (misconfiguration)

Both failure modes are fatal and consistent: if `-relay-matter` is set, the agent requires a working Matter connection. Silent degradation to DryRun would mask deployment issues.

## Status Subscription Integration

In `agent.go`, after relay initialization:

```go
if mr, ok := a.relay.(*MatterRelay); ok {
    go mr.SubscribeLockState(ctx, func(state DoorLockState) {
        a.enqueueEvent(Event{
            Type:      "lock_state_changed",
            LockID:    a.lockID,
            GatewayID: a.gatewayID,
            Data:      map[string]any{"state": state.String(), "source": "matter"},
        })
    })
}
```

## Credential Sync Integration

Cloud API pushes user/credential updates via WebSocket. Agent calls:

```go
if mr, ok := a.relay.(*MatterRelay); ok {
    for _, user := range updatedUsers {
        mr.SetUser(MatterUser{...})
        if user.PIN != "" {
            mr.SetCredential(MatterCredential{
                CredentialType: 1, // PIN
                CredentialData: user.PIN,
                ...
            })
        }
    }
}
```

## Error Handling

| Error | Behavior |
|-------|----------|
| chip-tool binary not found | Fatal on startup — log path tried, suggest install |
| chip-tool version unsupported | Log warning, continue (best-effort) |
| Commission fails | Fatal on startup — environment problem requires operator attention |
| Unlock command timeout (>configured ms) | Return error, log for alerting |
| Interactive session crashes | Auto-restart with backoff (1s, 2s, 4s, max 30s) |
| Node unreachable | Return error, agent logs access_denied event |
| CASE session failure | Retry once; if persistent, log fabric corruption warning |

## Testing

- **Unit tests**: Mock chip-tool binary (shell script that returns canned stdout) to test output parsing, error handling, state machine
- **Integration tests**: Requires real chip-tool + Matter test device (or CHIP virtual device `chip-all-clusters-app`)
- **CI**: Unit tests only; integration tests manual with hardware

## Dependencies

- `chip-tool` binary — must be pre-installed on gateway hardware
  - Build from [connectedhomeip](https://github.com/project-chip/connectedhomeip)
  - Or install via package manager if available for target platform
- Thread Border Router on the same network (external, not managed by gateway-agent)
- No new Go module dependencies

## Future Iterations

1. **Self-hosted TBR**: Add `-matter-otbr` flag to run OpenThread Border Router alongside gateway-agent
2. **BLE commissioning**: Add `pairing ble-thread` for devices not yet on Thread network
3. **Multi-lock support**: Support multiple Matter locks per gateway (multiple node IDs)
4. **Matter event subscriptions**: Beyond lock-state — door position sensor, tamper alerts, battery level
