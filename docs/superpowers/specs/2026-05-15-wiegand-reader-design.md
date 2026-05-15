# Wiegand Input Reader for Gateway-Agent

**Date:** 2026-05-15
**Status:** Approved
**Scope:** Add Wiegand 26/34-bit input reader via GPIO sysfs on Orange Pi Zero3

---

## Goal

Enable gateway-agent to receive card credentials from Wiegand-output readers (ZKTeco ProID10 13.56MHz, etc.) via GPIO edge detection, decode 26/34-bit frames, and feed card data into the existing access control flow (`HandleCredentialPresented`). Combined with the existing GPIO relay, this completes the card-swipe-to-door-unlock chain.

## Architecture

```
ZKTeco ProID10 (13.56MHz Wiegand reader)
  D0 ──→ GPIO Pin (input, falling edge)
  D1 ──→ GPIO Pin (input, falling edge)
  GND ──→ GND
  +12V ←── External 12V power supply

gateway-agent (Go)
├── WiegandReader         card input via GPIO
│   ├── Start()           export GPIO, set edge=falling, launch epoll loop
│   ├── Stop()            close epoll, unexport GPIO
│   └── readLoop()        epoll D0/D1 → collect bits → 50ms timeout → decode frame
│                         → HandleCredentialPresented("wiegand_26", "FC:CardNum", lockID)
├── Agent
│   └── HandleCredentialPresented()  → VerifyCredential() → relay.Unlock()
└── GPIORelay             door unlock (existing)
```

The Wiegand reader is a pure input device. It does NOT modify the relay, agent, or access decision logic. It follows the same Start/Stop + callback pattern as PCSCReader, NFCHCEReader, and BLEReader.

## New Files

| File | Responsibility |
|------|---------------|
| `cmd/gateway-agent/wiegand_reader.go` | `WiegandReader` — GPIO sysfs init, epoll edge detection loop, Wiegand 26/34-bit frame decoding, parity validation |
| `cmd/gateway-agent/wiegand_reader_test.go` | Unit tests — frame decoding, parity check, bit collection timeout, format detection |

## Modified Files

| File | Change |
|------|--------|
| `cmd/gateway-agent/main.go` | Add 3 CLI flags, initialize WiegandReader, startup banner |

## Command-Line Flags

```
-wiegand-lock-id <string>   Lock ID for Wiegand reader (e.g. door_factory_001). Empty = disabled.
-wiegand-d0-gpio <int>      GPIO pin number for Wiegand D0 signal. -1 = disabled.
-wiegand-d1-gpio <int>      GPIO pin number for Wiegand D1 signal. -1 = disabled.
```

All three must be provided to enable the Wiegand reader. If `-wiegand-lock-id` is set but either GPIO pin is -1, log a warning and skip initialization.

## Wiegand Protocol

### Signal Characteristics

- D0 and D1 are normally HIGH (pulled up)
- A 0-bit is signaled by D0 going LOW for ~50μs
- A 1-bit is signaled by D1 going LOW for ~50μs
- Inter-bit gap: ~1ms (D0 and D1 never fire simultaneously)
- Frame end: no edge detected for 50ms after the last bit

### 26-bit Format

```
Bit:  0  | 1-8      | 9-24             | 25
      PE | FC (8b)  | CardNum (16b)    | PO
```

- Bit 0: Even parity over bits 1-12
- Bits 1-8: Facility Code (0-255)
- Bits 9-24: Card Number (0-65535)
- Bit 25: Odd parity over bits 13-24

### 34-bit Format

```
Bit:  0  | 1-16      | 17-32            | 33
      PE | FC (16b)  | CardNum (16b)    | PO
```

- Bit 0: Even parity over bits 1-16
- Bits 1-16: Facility Code (0-65535)
- Bits 17-32: Card Number (0-65535)
- Bit 33: Odd parity over bits 17-32

### Frame Detection

The reader auto-detects 26-bit vs 34-bit based on the number of bits collected when the 50ms timeout fires. Any other bit count is logged as an error and discarded.

## WiegandReader

```go
type WiegandReader struct {
    d0Pin, d1Pin int
    lockID       string
    onCredential func(credType, credData, lockID string)
    logger       *slog.Logger
    stopCh       chan struct{}
}
```

### GPIO Setup (sysfs)

For each pin (D0, D1):

1. Export: write pin number to `/sys/class/gpio/export`
2. Direction: write `"in"` to `/sys/class/gpio/gpioN/direction`
3. Edge: write `"falling"` to `/sys/class/gpio/gpioN/edge`
4. Open `/sys/class/gpio/gpioN/value` for epoll monitoring

### readLoop (epoll state machine)

```
State: IDLE (waiting for first bit)
  ↓ D0 falling edge → bits = append(bits, 0), state = COLLECTING
  ↓ D1 falling edge → bits = append(bits, 1), state = COLLECTING

State: COLLECTING (accumulating bits)
  ↓ D0 falling edge → bits = append(bits, 0), reset 50ms timer
  ↓ D1 falling edge → bits = append(bits, 1), reset 50ms timer
  ↓ 50ms timeout → decodeFrame(bits), state = IDLE
```

The epoll loop uses `syscall.EpollWait` with a 50ms timeout. On each wake:
- If EPOLLPRI event on D0 fd → read + seek to consume the event, append 0-bit
- If EPOLLPRI event on D1 fd → read + seek to consume the event, append 1-bit
- If timeout (0 events returned) and bits are non-empty → decode frame

### Frame Decoding

```go
func decodeWiegand26(bits []byte) (facilityCode uint16, cardNumber uint32, err error)
func decodeWiegand34(bits []byte) (facilityCode uint16, cardNumber uint32, err error)
func checkEvenParity(bits []byte) bool
func checkOddParity(bits []byte) bool
```

After successful decoding:

```go
credType := "wiegand_26" // or "wiegand_34"
credData := fmt.Sprintf("%d:%d", facilityCode, cardNumber)
w.logger.Info("wiegand card detected", "type", credType, "fc", facilityCode, "card", cardNumber)
w.onCredential(credType, credData, w.lockID)
```

### Debounce

After a successful frame decode, ignore all edges for 2 seconds to prevent double-reads from the same card tap. This matches the PCSCReader's 3-second debounce window.

## Credential Data Format

Wiegand credentials are stored in access rules as `"FC:CardNum"`:

- `credential_type`: `"wiegand_26"` or `"wiegand_34"`
- `credential_data`: `"100:12345"` (facility code : card number, decimal)

The existing `VerifyCredential` in agent.go matches these by exact string comparison against `AccessRule.CredentialType` and `AccessRule.CredentialData`. No changes needed to the access decision logic.

### First-Time Card Enrollment

On first use, the operator swipes a card. The gateway-agent logs the decoded FC:CardNum. The operator then creates an access rule in the Cloud with that credential data. Subsequent swipes match the rule and unlock.

## main.go Integration

After the BLE reader initialization block:

```go
var wiegandReader *WiegandReader
if *wiegandLockID != "" {
    if *wiegandD0 < 0 || *wiegandD1 < 0 {
        logger.Warn("wiegand-lock-id set but D0/D1 GPIO pins not configured, skipping")
    } else {
        wiegandReader = NewWiegandReader(*wiegandD0, *wiegandD1, *wiegandLockID,
            agent.HandleCredentialPresented, logger)
        if err := wiegandReader.Start(); err != nil {
            logger.Warn("Wiegand reader failed to start", "error", err)
        } else {
            fmt.Printf("Wiegand: GPIO D0=%d D1=%d → %s\n", *wiegandD0, *wiegandD1, *wiegandLockID)
        }
    }
}
```

Shutdown:
```go
if wiegandReader != nil {
    wiegandReader.Stop()
}
```

Startup banner adds:
```
Wiegand: D0=gpio73 D1=gpio74 → door_factory_001
```

## Error Handling

| Error | Behavior |
|-------|----------|
| GPIO export fails | Log error, unexport any already-exported pins (rollback), skip Wiegand reader (non-fatal) |
| GPIO edge/direction set fails | Log error, unexport all pins (rollback), skip Wiegand reader (non-fatal) |
| Unexpected bit count (not 26 or 34) | Log warning with raw bit count, discard frame |
| Parity check fails | Log warning with raw bits, discard frame |
| epoll error | Log error, attempt re-init after 1s backoff |

## Testing

Unit tests cover decoding logic only (no real GPIO needed):

- `TestDecodeWiegand26` — valid frame → correct FC + CardNum
- `TestDecodeWiegand26_ParityError` — bad parity → error
- `TestDecodeWiegand34` — valid 34-bit frame → correct FC + CardNum
- `TestDecodeWiegand34_ParityError` — bad parity → error
- `TestCheckEvenParity` / `TestCheckOddParity` — parity helpers
- `TestFrameDetection` — 26 bits → wiegand_26, 34 bits → wiegand_34, other → error

Integration testing requires real hardware (ProID10 + Orange Pi + DESFire EV2/EV3 or Mifare Classic card).

## Hardware Wiring (Orange Pi Zero3)

```
ProID10          Orange Pi Zero3
───────          ──────────────
D0  (green)  →   10kΩ pull-up to 3.3V  →  PC9  (GPIO 73)
D1  (white)  →   10kΩ pull-up to 3.3V  →  PC10 (GPIO 74)
GND (black)  →   GND
+12V (red)   ←   External 12V DC power supply

Relay Module     Orange Pi Zero3
────────────     ──────────────
IN1          ←   GPIO pin (configured via -relay-gpio)
VCC          ←   3.3V or 5V (match relay module spec)
GND          →   GND
```

### Voltage and Pull-up Notes

**D0/D1 pull-up resistors are required.** sysfs GPIO cannot configure internal pull-ups. Add external 10kΩ resistors from each D0/D1 line to the Orange Pi's 3.3V pin. This ensures idle-HIGH state and prevents noise-triggered false edges.

**Voltage safety:** Standard Wiegand D0/D1 are open-collector outputs — the reader only pulls LOW, it does not drive HIGH. With 3.3V external pull-ups, signal levels are 0–3.3V, safe for the H618 GPIO. **Before first power-on, verify with a multimeter** that D0/D1 idle voltage is ≤3.3V. If the ProID10 has internal 5V pull-ups (non-standard), add a voltage divider (10kΩ series + 20kΩ to GND) or a bidirectional level shifter.

**12V power:** ProID10 requires 12V DC. Do NOT power it from Orange Pi's 5V/3.3V pins.

### sysfs GPIO Compatibility Note

This implementation uses Linux sysfs GPIO (`/sys/class/gpio/`), which is supported on Orange Pi Zero3 (H618, kernel 6.1). If a future kernel removes sysfs GPIO support, the implementation should migrate to the character device API (`/dev/gpiochipN` + `ioctl`) or a Go library like `periph.io`.
