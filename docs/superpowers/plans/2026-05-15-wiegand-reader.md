# Wiegand Input Reader Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Wiegand 26/34-bit input reader to gateway-agent via GPIO sysfs edge detection, enabling card-swipe-to-door-unlock with ZKTeco ProID10 reader.

**Architecture:** A `WiegandReader` monitors two GPIO pins (D0/D1) via Linux sysfs epoll for falling edges. Bits are collected into a frame buffer; a 50ms silence timeout triggers frame decoding (26 or 34-bit with parity validation). Decoded facility code + card number are passed to `HandleCredentialPresented` which reuses the existing access rule matching and relay unlock flow.

**Tech Stack:** Go 1.25, Linux sysfs GPIO (`/sys/class/gpio/`), `syscall` (epoll), `os`

**Spec:** `docs/superpowers/specs/2026-05-15-wiegand-reader-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| Create: `api/cmd/gateway-agent/wiegand_reader.go` | `WiegandReader` struct, GPIO sysfs helpers, epoll loop, Wiegand 26/34-bit decode, parity check |
| Create: `api/cmd/gateway-agent/wiegand_reader_test.go` | Unit tests for parity, decode, frame detection (no real GPIO) |
| Modify: `api/cmd/gateway-agent/main.go` | Add 3 CLI flags, initialize WiegandReader, startup banner, shutdown |

---

### Task 1: Parity Helpers and Wiegand Frame Decoding

**Files:**
- Create: `api/cmd/gateway-agent/wiegand_reader.go`
- Create: `api/cmd/gateway-agent/wiegand_reader_test.go`

- [ ] **Step 1: Write failing tests for parity and decode functions**

In `api/cmd/gateway-agent/wiegand_reader_test.go`:

```go
package main

import "testing"

func TestCheckEvenParity(t *testing.T) {
	tests := []struct {
		name string
		bits []byte
		want bool
	}{
		{"all zeros", []byte{0, 0, 0, 0}, true},       // 0 ones = even
		{"one 1", []byte{1, 0, 0, 0}, false},           // 1 one = odd
		{"two 1s", []byte{1, 1, 0, 0}, true},           // 2 ones = even
		{"three 1s", []byte{1, 1, 1, 0}, false},        // 3 ones = odd
		{"all ones", []byte{1, 1, 1, 1}, true},          // 4 ones = even
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkEvenParity(tt.bits); got != tt.want {
				t.Errorf("checkEvenParity(%v) = %v, want %v", tt.bits, got, tt.want)
			}
		})
	}
}

func TestCheckOddParity(t *testing.T) {
	tests := []struct {
		name string
		bits []byte
		want bool
	}{
		{"all zeros", []byte{0, 0, 0, 0}, false},      // 0 ones = even, not odd
		{"one 1", []byte{1, 0, 0, 0}, true},            // 1 one = odd
		{"two 1s", []byte{1, 1, 0, 0}, false},          // 2 ones = even
		{"three 1s", []byte{1, 1, 1, 0}, true},         // 3 ones = odd
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkOddParity(tt.bits); got != tt.want {
				t.Errorf("checkOddParity(%v) = %v, want %v", tt.bits, got, tt.want)
			}
		})
	}
}

func TestDecodeWiegand26(t *testing.T) {
	// Build a valid 26-bit frame: PE(even over 1-12) | FC 8bit | Card 16bit | PO(odd over 13-24)
	// FC=100 (01100100), Card=12345 (0011000000111001)
	// Bits 1-12:  0 1 1 0 0 1 0 0 | 0 0 1 1  -> 5 ones -> odd -> PE=1
	// Bits 13-24: 0 0 0 0 0 0 1 1 1 0 0 1    -> 4 ones -> even -> PO=1
	frame := []byte{
		1,                                     // bit 0: even parity
		0, 1, 1, 0, 0, 1, 0, 0,               // bits 1-8: FC=100
		0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 1, 1, 1, 0, 0, 1, // bits 9-24: Card=12345
		1,                                     // bit 25: odd parity
	}
	fc, card, err := decodeWiegand26(frame)
	if err != nil {
		t.Fatalf("decodeWiegand26() error: %v", err)
	}
	if fc != 100 {
		t.Errorf("facility code = %d, want 100", fc)
	}
	if card != 12345 {
		t.Errorf("card number = %d, want 12345", card)
	}
}

func TestDecodeWiegand26_ParityError(t *testing.T) {
	// Same frame but flip the even parity bit
	frame := []byte{
		0,                                     // bit 0: WRONG parity (should be 1)
		0, 1, 1, 0, 0, 1, 0, 0,
		0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 1, 1, 1, 0, 0, 1,
		1,
	}
	_, _, err := decodeWiegand26(frame)
	if err == nil {
		t.Fatal("decodeWiegand26() expected parity error")
	}
}

func TestDecodeWiegand26_WrongLength(t *testing.T) {
	frame := make([]byte, 20) // not 26
	_, _, err := decodeWiegand26(frame)
	if err == nil {
		t.Fatal("decodeWiegand26() expected length error")
	}
}

func TestDecodeWiegand34(t *testing.T) {
	// Build a valid 34-bit frame: PE(even over 1-16) | FC 16bit | Card 16bit | PO(odd over 17-32)
	// FC=200 (0000000011001000), Card=54321 (1101010000110001)
	// Bits 1-16: 0 0 0 0 0 0 0 0 1 1 0 0 1 0 0 0 -> 4 ones -> even -> PE=0
	// Bits 17-32: 1 1 0 1 0 1 0 0 0 0 1 1 0 0 0 1 -> 7 ones -> odd -> PO=0
	frame := []byte{
		0,                                                     // bit 0: even parity
		0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 0, 0, 1, 0, 0, 0,     // bits 1-16: FC=200
		1, 1, 0, 1, 0, 1, 0, 0, 0, 0, 1, 1, 0, 0, 0, 1,     // bits 17-32: Card=54321
		0,                                                     // bit 33: odd parity
	}
	fc, card, err := decodeWiegand34(frame)
	if err != nil {
		t.Fatalf("decodeWiegand34() error: %v", err)
	}
	if fc != 200 {
		t.Errorf("facility code = %d, want 200", fc)
	}
	if card != 54321 {
		t.Errorf("card number = %d, want 54321", card)
	}
}

func TestDecodeWiegand34_ParityError(t *testing.T) {
	frame := []byte{
		1,                                                     // bit 0: WRONG parity (should be 0)
		0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 0, 0, 1, 0, 0, 0,
		1, 1, 0, 1, 0, 1, 0, 0, 0, 0, 1, 1, 0, 0, 0, 1,
		0,
	}
	_, _, err := decodeWiegand34(frame)
	if err == nil {
		t.Fatal("decodeWiegand34() expected parity error")
	}
}

func TestDecodeWiegandFrame(t *testing.T) {
	tests := []struct {
		name     string
		bitCount int
		wantType string
		wantErr  bool
	}{
		{"26 bits", 26, "wiegand_26", false},
		{"34 bits", 34, "wiegand_34", false},
		{"20 bits", 20, "", true},
		{"0 bits", 0, "", true},
		{"40 bits", 40, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, _, _, err := decodeWiegandFrame(make([]byte, tt.bitCount))
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeWiegandFrame(%d bits) err = %v, wantErr %v", tt.bitCount, err, tt.wantErr)
			}
			if err == nil && gotType != tt.wantType {
				t.Errorf("decodeWiegandFrame(%d bits) type = %q, want %q", tt.bitCount, gotType, tt.wantType)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -run "TestCheckEvenParity|TestCheckOddParity|TestDecodeWiegand" -v`
Expected: Compilation errors — functions not defined.

- [ ] **Step 3: Implement parity and decode functions**

In `api/cmd/gateway-agent/wiegand_reader.go`:

```go
package main

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"syscall"
	"time"
)

// WiegandReader receives card credentials from a Wiegand-output reader
// (e.g. ZKTeco ProID10) via GPIO edge detection on D0/D1 pins.
// Decodes 26-bit and 34-bit Wiegand frames and passes card data
// to the onCredential callback for access control.
type WiegandReader struct {
	d0Pin, d1Pin int
	lockID       string
	onCredential func(credType, credData, lockID string)
	logger       *slog.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewWiegandReader creates a reader bound to two GPIO pins and a lock ID.
func NewWiegandReader(d0Pin, d1Pin int, lockID string, onCredential func(string, string, string), logger *slog.Logger) *WiegandReader {
	return &WiegandReader{
		d0Pin:        d0Pin,
		d1Pin:        d1Pin,
		lockID:       lockID,
		onCredential: onCredential,
		logger:       logger.With("component", "wiegand_reader"),
	}
}

// --- Parity helpers ---

// checkEvenParity returns true if the number of 1-bits in bits is even.
func checkEvenParity(bits []byte) bool {
	count := 0
	for _, b := range bits {
		if b != 0 {
			count++
		}
	}
	return count%2 == 0
}

// checkOddParity returns true if the number of 1-bits in bits is odd.
func checkOddParity(bits []byte) bool {
	return !checkEvenParity(bits)
}

// --- Wiegand frame decoding ---

// bitsToUint converts a slice of 0/1 bytes to an unsigned integer (MSB first).
func bitsToUint(bits []byte) uint32 {
	var val uint32
	for _, b := range bits {
		val = (val << 1) | uint32(b&1)
	}
	return val
}

// decodeWiegand26 decodes a 26-bit Wiegand frame.
// Format: PE(1) | FC(8) | Card(16) | PO(1)
// PE = even parity over bits 1-12, PO = odd parity over bits 13-24.
func decodeWiegand26(bits []byte) (facilityCode uint16, cardNumber uint32, err error) {
	if len(bits) != 26 {
		return 0, 0, fmt.Errorf("wiegand26: expected 26 bits, got %d", len(bits))
	}
	// Even parity: bit 0 covers bits 1-12
	if !checkEvenParity(bits[0:13]) {
		return 0, 0, fmt.Errorf("wiegand26: even parity check failed")
	}
	// Odd parity: bit 25 covers bits 13-24 (last 13 bits: 13-25)
	if !checkOddParity(bits[13:26]) {
		return 0, 0, fmt.Errorf("wiegand26: odd parity check failed")
	}
	facilityCode = uint16(bitsToUint(bits[1:9]))
	cardNumber = bitsToUint(bits[9:25])
	return facilityCode, cardNumber, nil
}

// decodeWiegand34 decodes a 34-bit Wiegand frame.
// Format: PE(1) | FC(16) | Card(16) | PO(1)
// PE = even parity over bits 1-16, PO = odd parity over bits 17-32.
func decodeWiegand34(bits []byte) (facilityCode uint16, cardNumber uint32, err error) {
	if len(bits) != 34 {
		return 0, 0, fmt.Errorf("wiegand34: expected 34 bits, got %d", len(bits))
	}
	// Even parity: bit 0 covers bits 1-16
	if !checkEvenParity(bits[0:17]) {
		return 0, 0, fmt.Errorf("wiegand34: even parity check failed")
	}
	// Odd parity: bit 33 covers bits 17-32 (last 17 bits: 17-33)
	if !checkOddParity(bits[17:34]) {
		return 0, 0, fmt.Errorf("wiegand34: odd parity check failed")
	}
	facilityCode = uint16(bitsToUint(bits[1:17]))
	cardNumber = bitsToUint(bits[17:33])
	return facilityCode, cardNumber, nil
}

// decodeWiegandFrame auto-detects 26/34-bit format and decodes.
func decodeWiegandFrame(bits []byte) (credType string, facilityCode uint16, cardNumber uint32, err error) {
	switch len(bits) {
	case 26:
		fc, cn, err := decodeWiegand26(bits)
		return "wiegand_26", fc, cn, err
	case 34:
		fc, cn, err := decodeWiegand34(bits)
		return "wiegand_34", fc, cn, err
	default:
		return "", 0, 0, fmt.Errorf("wiegand: unsupported frame length %d bits (expected 26 or 34)", len(bits))
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -run "TestCheckEvenParity|TestCheckOddParity|TestDecodeWiegand" -v`
Expected: All 7 tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/siky/code/MistyPass && git add api/cmd/gateway-agent/wiegand_reader.go api/cmd/gateway-agent/wiegand_reader_test.go && git commit -m "feat(gateway): add Wiegand 26/34-bit frame decoding with parity validation"
```

---

### Task 2: GPIO sysfs Helpers and WiegandReader Start/Stop

**Files:**
- Modify: `api/cmd/gateway-agent/wiegand_reader.go`

- [ ] **Step 1: Implement GPIO sysfs helpers and Start/Stop**

Append to `api/cmd/gateway-agent/wiegand_reader.go`:

```go
// --- GPIO sysfs helpers ---

func gpioExport(pin int) error {
	return os.WriteFile("/sys/class/gpio/export", []byte(fmt.Sprintf("%d", pin)), 0o600)
}

func gpioUnexport(pin int) error {
	return os.WriteFile("/sys/class/gpio/unexport", []byte(fmt.Sprintf("%d", pin)), 0o600)
}

func gpioSetDirection(pin int, direction string) error {
	return os.WriteFile(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", pin), []byte(direction), 0o600)
}

func gpioSetEdge(pin int, edge string) error {
	return os.WriteFile(fmt.Sprintf("/sys/class/gpio/gpio%d/edge", pin), []byte(edge), 0o600)
}

func gpioOpenValue(pin int) (*os.File, error) {
	return os.Open(fmt.Sprintf("/sys/class/gpio/gpio%d/value", pin))
}

// initGPIOPin exports a pin, sets it as input with falling edge detection.
func initGPIOPin(pin int) error {
	if err := gpioExport(pin); err != nil {
		// May already be exported, try to continue
	}
	if err := gpioSetDirection(pin, "in"); err != nil {
		return fmt.Errorf("gpio%d set direction: %w", pin, err)
	}
	if err := gpioSetEdge(pin, "falling"); err != nil {
		return fmt.Errorf("gpio%d set edge: %w", pin, err)
	}
	return nil
}

// Start initializes GPIO pins and launches the epoll read loop.
func (w *WiegandReader) Start() error {
	// Initialize D0
	if err := initGPIOPin(w.d0Pin); err != nil {
		gpioUnexport(w.d0Pin)
		return fmt.Errorf("wiegand D0 init: %w", err)
	}

	// Initialize D1 (rollback D0 on failure)
	if err := initGPIOPin(w.d1Pin); err != nil {
		gpioUnexport(w.d0Pin)
		gpioUnexport(w.d1Pin)
		return fmt.Errorf("wiegand D1 init: %w", err)
	}

	// Open value files for epoll
	d0File, err := gpioOpenValue(w.d0Pin)
	if err != nil {
		w.cleanup()
		return fmt.Errorf("wiegand D0 open: %w", err)
	}
	d1File, err := gpioOpenValue(w.d1Pin)
	if err != nil {
		d0File.Close()
		w.cleanup()
		return fmt.Errorf("wiegand D1 open: %w", err)
	}

	w.stopCh = make(chan struct{})
	w.wg.Add(1)
	go w.readLoop(d0File, d1File)

	w.logger.Info("wiegand reader started",
		"d0_pin", w.d0Pin, "d1_pin", w.d1Pin, "lock_id", w.lockID)
	return nil
}

// Stop signals the read loop to exit and unexports GPIO pins.
func (w *WiegandReader) Stop() {
	if w.stopCh != nil {
		close(w.stopCh)
		w.wg.Wait()
	}
	w.cleanup()
	w.logger.Info("wiegand reader stopped")
}

func (w *WiegandReader) cleanup() {
	gpioUnexport(w.d0Pin)
	gpioUnexport(w.d1Pin)
}
```

- [ ] **Step 2: Verify build compiles**

Run: `cd /Users/siky/code/MistyPass/api && go build ./cmd/gateway-agent/`
Expected: Build succeeds. (The `readLoop` method is not yet defined — add a stub.)

Add a temporary stub at the bottom of `wiegand_reader.go`:

```go
// readLoop is the epoll-based GPIO edge detection loop.
// Implemented in Task 3.
func (w *WiegandReader) readLoop(d0File, d1File *os.File) {
	defer w.wg.Done()
	defer d0File.Close()
	defer d1File.Close()
	<-w.stopCh
}
```

- [ ] **Step 3: Commit**

```bash
cd /Users/siky/code/MistyPass && git add api/cmd/gateway-agent/wiegand_reader.go && git commit -m "feat(gateway): add Wiegand GPIO sysfs helpers and reader Start/Stop lifecycle"
```

---

### Task 3: epoll Read Loop with Frame Collection

**Files:**
- Modify: `api/cmd/gateway-agent/wiegand_reader.go`

- [ ] **Step 1: Replace readLoop stub with epoll implementation**

Replace the `readLoop` stub with the full implementation:

```go
// readLoop is the epoll-based GPIO edge detection loop.
// It monitors D0/D1 for falling edges, collects bits into a frame buffer,
// and decodes the frame after 50ms of silence.
func (w *WiegandReader) readLoop(d0File, d1File *os.File) {
	defer w.wg.Done()
	defer d0File.Close()
	defer d1File.Close()

	d0Fd := int(d0File.Fd())
	d1Fd := int(d1File.Fd())

	// Create epoll instance
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		w.logger.Error("wiegand epoll create failed", "error", err)
		return
	}
	defer syscall.Close(epfd)

	// Register D0 and D1 for priority events (EPOLLPRI = sysfs GPIO edge)
	for _, fd := range []int{d0Fd, d1Fd} {
		event := syscall.EpollEvent{
			Events: syscall.EPOLLPRI | syscall.EPOLLERR,
			Fd:     int32(fd),
		}
		if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, &event); err != nil {
			w.logger.Error("wiegand epoll add failed", "fd", fd, "error", err)
			return
		}
		// Initial read to clear any pending event
		buf := make([]byte, 1)
		syscall.Read(fd, buf)
		syscall.Seek(fd, 0, 0)
	}

	var bits []byte
	events := make([]syscall.EpollEvent, 2)
	debounceUntil := time.Time{} // zero = no debounce active

	w.logger.Info("wiegand epoll loop started")

	for {
		// Check for stop signal (non-blocking)
		select {
		case <-w.stopCh:
			return
		default:
		}

		// Wait for edge events with 50ms timeout
		n, err := syscall.EpollWait(epfd, events, 50)
		if err != nil {
			if err == syscall.EINTR {
				continue // interrupted by signal, retry
			}
			w.logger.Error("wiegand epoll wait error", "error", err)
			time.Sleep(1 * time.Second) // backoff before retry
			continue
		}

		now := time.Now()

		// Process edge events
		for i := 0; i < n; i++ {
			fd := int(events[i].Fd)
			// Consume the event: read + seek back to 0
			buf := make([]byte, 1)
			syscall.Read(fd, buf)
			syscall.Seek(fd, 0, 0)

			// Skip if debouncing
			if now.Before(debounceUntil) {
				continue
			}

			switch fd {
			case d0Fd:
				bits = append(bits, 0)
			case d1Fd:
				bits = append(bits, 1)
			}
		}

		// Frame timeout: 50ms with no new bits and buffer non-empty
		if n == 0 && len(bits) > 0 {
			// Skip if debouncing
			if now.Before(debounceUntil) {
				bits = bits[:0]
				continue
			}

			credType, fc, card, err := decodeWiegandFrame(bits)
			if err != nil {
				w.logger.Warn("wiegand frame decode error",
					"error", err, "bit_count", len(bits))
			} else {
				credData := fmt.Sprintf("%d:%d", fc, card)
				w.logger.Info("wiegand card detected",
					"type", credType, "fc", fc, "card", card, "lock_id", w.lockID)
				w.onCredential(credType, credData, w.lockID)

				// Debounce: ignore edges for 2 seconds
				debounceUntil = time.Now().Add(2 * time.Second)
			}
			bits = bits[:0]
		}
	}
}
```

- [ ] **Step 2: Verify build compiles**

Run: `cd /Users/siky/code/MistyPass/api && go build ./cmd/gateway-agent/`
Expected: Build succeeds.

- [ ] **Step 3: Run all existing tests to ensure no breakage**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -count=1 -timeout 30s`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
cd /Users/siky/code/MistyPass && git add api/cmd/gateway-agent/wiegand_reader.go && git commit -m "feat(gateway): add Wiegand epoll read loop with frame collection and debounce"
```

---

### Task 4: main.go Integration — Flags, Init, Banner, Shutdown

**Files:**
- Modify: `api/cmd/gateway-agent/main.go`

- [ ] **Step 1: Add Wiegand CLI flags**

In `api/cmd/gateway-agent/main.go`, after the `bleListenAddr` flag (around line 47), add:

```go
	wiegandLockID := flag.String("wiegand-lock-id", "", "Lock ID for Wiegand reader (e.g. door_factory_001). Empty = disabled.")
	wiegandD0 := flag.Int("wiegand-d0-gpio", -1, "GPIO pin number for Wiegand D0 signal")
	wiegandD1 := flag.Int("wiegand-d1-gpio", -1, "GPIO pin number for Wiegand D1 signal")
```

- [ ] **Step 2: Add Wiegand reader initialization**

After the BLE reader block (after the closing `}` of `if *bleLockID != ""`, around line 144), add:

```go
	// Start Wiegand reader if lock ID and GPIO pins are configured
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
				fmt.Printf("Wiegand: D0=gpio%d D1=gpio%d → %s\n", *wiegandD0, *wiegandD1, *wiegandLockID)
			}
		}
	}
```

- [ ] **Step 3: Add Wiegand shutdown**

In the shutdown section (after `bleReader.Stop()`, around line 165), add:

```go
	if wiegandReader != nil {
		wiegandReader.Stop()
	}
```

- [ ] **Step 4: Verify build compiles**

Run: `cd /Users/siky/code/MistyPass/api && go build ./cmd/gateway-agent/`
Expected: Build succeeds.

- [ ] **Step 5: Verify flags appear in help**

Run: `cd /Users/siky/code/MistyPass/api && go run ./cmd/gateway-agent/ -h 2>&1 | grep -A1 wiegand`
Expected: All 3 Wiegand flags displayed.

- [ ] **Step 6: Run all tests**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -count=1 -timeout 30s`
Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
cd /Users/siky/code/MistyPass && git add api/cmd/gateway-agent/main.go && git commit -m "feat(gateway): integrate Wiegand reader with CLI flags and startup/shutdown"
```

---

### Task 5: Final Verification

- [ ] **Step 1: Run full test suite**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -v -count=1 -timeout 30s`
Expected: All tests pass including new Wiegand decode tests.

- [ ] **Step 2: Run go vet**

Run: `cd /Users/siky/code/MistyPass/api && go vet ./cmd/gateway-agent/`
Expected: No warnings.

- [ ] **Step 3: Verify flag help output**

Run: `cd /Users/siky/code/MistyPass/api && go run ./cmd/gateway-agent/ -h 2>&1 | grep wiegand`
Expected: 3 Wiegand flags listed.

- [ ] **Step 4: Commit spec and plan docs**

```bash
cd /Users/siky/code/MistyPass && git add docs/superpowers/ && git commit -m "docs: add Wiegand reader design spec and implementation plan"
```
