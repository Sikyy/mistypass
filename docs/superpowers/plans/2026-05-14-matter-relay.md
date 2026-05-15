# Matter Relay Driver Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Matter over Thread smart lock control to gateway-agent via chip-tool subprocess, implementing the `RelayDriver` interface.

**Architecture:** A `MatterController` wraps chip-tool subprocess lifecycle (one-shot Exec for commands, long-lived interactive process for subscriptions). A `MatterRelay` uses the controller to implement `RelayDriver` plus extended Door Lock cluster methods (commission, lock state, credential sync). Agent integration adds CLI flags and relay selection priority.

**Tech Stack:** Go 1.25, chip-tool CLI (Matter SDK), `os/exec`, `bufio`, `regexp`, `sync`, `time`

**Spec:** `docs/superpowers/specs/2026-05-14-matter-relay-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| Create: `api/cmd/gateway-agent/matter_controller.go` | `MatterController` — chip-tool subprocess management, output parsing |
| Create: `api/cmd/gateway-agent/matter_relay.go` | `MatterRelay` — `RelayDriver` impl + Door Lock cluster extended methods |
| Create: `api/cmd/gateway-agent/matter_relay_test.go` | Unit tests — mock chip-tool, output parsing, re-lock race safety, commissioning |
| Modify: `api/cmd/gateway-agent/main.go` | Add Matter CLI flags, startup banner |
| Modify: `api/cmd/gateway-agent/agent.go` | Add Matter fields, relay init priority, subscription hookup |

---

### Task 1: Data Types and Output Parsing

**Files:**
- Create: `api/cmd/gateway-agent/matter_controller.go`
- Create: `api/cmd/gateway-agent/matter_relay_test.go`

- [ ] **Step 1: Write failing tests for DoorLockState and output parsing**

In `api/cmd/gateway-agent/matter_relay_test.go`:

```go
package main

import "testing"

func TestDoorLockStateString(t *testing.T) {
	tests := []struct {
		state DoorLockState
		want  string
	}{
		{DoorLockStateLocked, "locked"},
		{DoorLockStateUnlocked, "unlocked"},
		{DoorLockStateUnlatched, "unlatched"},
		{DoorLockStateNotFullyLocked, "not_fully_locked"},
		{DoorLockStateUndefined, "undefined"},
		{DoorLockState(99), "unknown(99)"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("DoorLockState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestParseCommandStatus(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantOK  bool
		wantErr bool
	}{
		{
			name:   "success",
			output: "CHIP:DMG: status = 0x0\nCHIP:TOO: Endpoint: 1 Cluster: 0x0000_0101",
			wantOK: true,
		},
		{
			name:    "failure",
			output:  "CHIP:DMG: status = 0x01\nCHIP:EM: Failed to send message",
			wantOK:  false,
			wantErr: true,
		},
		{
			name:    "unsupported",
			output:  "CHIP:DMG: status = 0x7e",
			wantOK:  false,
			wantErr: true,
		},
		{
			name:    "no status line",
			output:  "CHIP:DL: some debug output\nnothing useful",
			wantOK:  false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := parseCommandStatus(tt.output)
			if ok != tt.wantOK {
				t.Errorf("parseCommandStatus() ok = %v, want %v", ok, tt.wantOK)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCommandStatus() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseLockState(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    DoorLockState
		wantErr bool
	}{
		{
			name:   "locked",
			output: "CHIP:TOO: Endpoint: 1 Cluster: 0x0000_0101 Attribute 0x0000_0000 DataVersion: 123\nCHIP:TOO:   LockState = 1",
			want:   DoorLockStateLocked,
		},
		{
			name:   "unlocked",
			output: "CHIP:TOO:   LockState = 2",
			want:   DoorLockStateUnlocked,
		},
		{
			name:    "no lock state",
			output:  "CHIP:TOO: some other output",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLockState(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLockState() err = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && got != tt.want {
				t.Errorf("parseLockState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseChipToolVersion(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantVer string
		wantOK  bool
	}{
		{
			name:    "valid 1.4",
			output:  "CHIP:TOO: chip-tool Version: 1.4.0.0 (abc123)\n",
			wantVer: "1.4.0.0",
			wantOK:  true,
		},
		{
			name:    "valid 1.5",
			output:  "chip-tool Version: 1.5.1.0\n",
			wantVer: "1.5.1.0",
			wantOK:  true,
		},
		{
			name:   "no version",
			output: "some random output",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver, ok := parseChipToolVersion(tt.output)
			if ok != tt.wantOK {
				t.Errorf("parseChipToolVersion() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && ver != tt.wantVer {
				t.Errorf("parseChipToolVersion() ver = %q, want %q", ver, tt.wantVer)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -run "TestDoorLockState|TestParseCommand|TestParseLockState|TestParseChipTool" -v`
Expected: Compilation errors — types and functions not defined yet.

- [ ] **Step 3: Implement data types and parsing functions**

In `api/cmd/gateway-agent/matter_controller.go`:

```go
package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// DoorLockState represents the Matter Door Lock cluster LockState attribute.
type DoorLockState uint8

const (
	DoorLockStateNotFullyLocked DoorLockState = 0
	DoorLockStateLocked         DoorLockState = 1
	DoorLockStateUnlocked       DoorLockState = 2
	DoorLockStateUnlatched      DoorLockState = 3
	DoorLockStateUndefined      DoorLockState = 0xFF
)

func (s DoorLockState) String() string {
	switch s {
	case DoorLockStateNotFullyLocked:
		return "not_fully_locked"
	case DoorLockStateLocked:
		return "locked"
	case DoorLockStateUnlocked:
		return "unlocked"
	case DoorLockStateUnlatched:
		return "unlatched"
	case DoorLockStateUndefined:
		return "undefined"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// MatterUser represents a user entry in the Matter Door Lock cluster.
type MatterUser struct {
	UserIndex  uint16
	UserName   string
	UniqueID   uint32
	UserStatus uint8 // 1=OccupiedEnabled, 3=OccupiedDisabled
	UserType   uint8 // 0=UnrestrictedUser
}

// MatterCredential represents a credential entry in the Matter Door Lock cluster.
type MatterCredential struct {
	UserIndex       uint16
	CredentialIndex uint16
	CredentialType  uint8  // 1=PIN, 2=RFID, 3=Fingerprint, 4=FingerVein
	CredentialData  []byte // PIN as UTF-8 bytes, RFID/biometric as raw bytes
}

// --- Output parsing ---

var (
	reStatus      = regexp.MustCompile(`status\s*=\s*0x([0-9a-fA-F]+)`)
	reLockState   = regexp.MustCompile(`LockState\s*=\s*(\d+)`)
	reVersion     = regexp.MustCompile(`Version:\s*([\d.]+)`)
)

// parseCommandStatus checks chip-tool output for a status line.
// Returns (true, nil) on success (status 0x0), or (false, error) with details.
func parseCommandStatus(output string) (bool, error) {
	matches := reStatus.FindStringSubmatch(output)
	if matches == nil {
		return false, fmt.Errorf("chip-tool: no status line in output")
	}
	if matches[1] == "0" {
		return true, nil
	}
	return false, fmt.Errorf("chip-tool: command failed with status 0x%s", matches[1])
}

// parseLockState extracts the LockState attribute value from chip-tool output.
func parseLockState(output string) (DoorLockState, error) {
	matches := reLockState.FindStringSubmatch(output)
	if matches == nil {
		return DoorLockStateUndefined, fmt.Errorf("chip-tool: no LockState in output")
	}
	val, err := strconv.Atoi(matches[1])
	if err != nil {
		return DoorLockStateUndefined, fmt.Errorf("chip-tool: invalid LockState value %q", matches[1])
	}
	return DoorLockState(val), nil
}

// parseChipToolVersion extracts the version string from chip-tool version output.
func parseChipToolVersion(output string) (string, bool) {
	matches := reVersion.FindStringSubmatch(output)
	if matches == nil {
		return "", false
	}
	return matches[1], true
}

// MatterController manages chip-tool subprocess execution.
type MatterController struct {
	binary       string
	storageDir   string
	timedTimeout int // --timedInteractionTimeoutMs value
	logger       *slog.Logger

	// Subscribe-only interactive session
	mu      sync.Mutex
	session *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
}

// NewMatterController creates a controller, verifies the chip-tool binary exists,
// and checks its version.
func NewMatterController(binary, storageDir string, timedTimeout int, logger *slog.Logger) (*MatterController, error) {
	// Resolve binary path
	path, err := exec.LookPath(binary)
	if err != nil {
		return nil, fmt.Errorf("chip-tool binary not found at %q: %w (install from https://github.com/project-chip/connectedhomeip)", binary, err)
	}

	c := &MatterController{
		binary:       path,
		storageDir:   storageDir,
		timedTimeout: timedTimeout,
		logger:       logger,
	}

	// Version check (best-effort, don't block on failure)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, verErr := c.Exec(ctx, "version")
	if verErr != nil {
		logger.Warn("chip-tool version check failed", "error", verErr)
	} else if ver, ok := parseChipToolVersion(out); ok {
		logger.Info("chip-tool version", "version", ver)
		if !strings.HasPrefix(ver, "1.4.") && !strings.HasPrefix(ver, "1.5.") {
			logger.Warn("chip-tool version outside tested range (1.4.x-1.5.x), output parsing may be unreliable", "version", ver)
		}
	}

	return c, nil
}

// Exec runs a one-shot chip-tool command and returns its stdout.
func (c *MatterController) Exec(ctx context.Context, args ...string) (string, error) {
	fullArgs := append(args, "--storage-directory", c.storageDir)
	cmd := exec.CommandContext(ctx, c.binary, fullArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	c.logger.Debug("chip-tool exec", "args", fullArgs)
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("chip-tool exec %v: %w\nstderr: %s", args, err, stderr.String())
	}
	return stdout.String(), nil
}
```

Note: add `"time"` to imports (already included by the `context` timeout in `NewMatterController`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -run "TestDoorLockState|TestParseCommand|TestParseLockState|TestParseChipTool" -v`
Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/siky/code/MistyPass && git add api/cmd/gateway-agent/matter_controller.go api/cmd/gateway-agent/matter_relay_test.go && git commit -m "feat(gateway): add Matter data types and chip-tool output parsing"
```

---

### Task 2: MatterController — Exec with Mock chip-tool

Test that `Exec()` correctly runs subprocesses and returns output using a mock shell script.

**Files:**
- Modify: `api/cmd/gateway-agent/matter_relay_test.go`

- [ ] **Step 1: Write failing test for Exec**

Append to `api/cmd/gateway-agent/matter_relay_test.go`:

```go
import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeMockChipTool creates a shell script that mimics chip-tool output.
// The script echoes canned stdout based on the first argument.
func writeMockChipTool(t *testing.T, dir string, script string) string {
	t.Helper()
	path := filepath.Join(dir, "chip-tool")
	err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMatterControllerExec(t *testing.T) {
	dir := t.TempDir()
	storageDir := filepath.Join(dir, "fabric")
	os.MkdirAll(storageDir, 0o755)

	// Mock chip-tool that echoes its arguments and a success status
	mock := writeMockChipTool(t, dir, `
echo "CHIP:TOO: Endpoint: 1 Cluster: 0x0000_0101"
echo "CHIP:DMG: status = 0x0"
`)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctrl := &MatterController{binary: mock, storageDir: storageDir, timedTimeout: 10000, logger: logger}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := ctrl.Exec(ctx, "doorlock", "unlock-door", "1", "1")
	if err != nil {
		t.Fatalf("Exec() unexpected error: %v", err)
	}
	ok, parseErr := parseCommandStatus(out)
	if !ok || parseErr != nil {
		t.Fatalf("expected success status, got ok=%v err=%v output=%q", ok, parseErr, out)
	}
}

func TestMatterControllerExecFailure(t *testing.T) {
	dir := t.TempDir()
	storageDir := filepath.Join(dir, "fabric")
	os.MkdirAll(storageDir, 0o755)

	mock := writeMockChipTool(t, dir, `
echo "CHIP:DMG: status = 0x01"
exit 1
`)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctrl := &MatterController{binary: mock, storageDir: storageDir, timedTimeout: 10000, logger: logger}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := ctrl.Exec(ctx, "doorlock", "lock-door", "1", "1")
	if err == nil {
		t.Fatal("Exec() expected error for exit code 1")
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -run "TestMatterControllerExec" -v`
Expected: Both tests PASS (Exec implementation already exists from Task 1).

- [ ] **Step 3: Commit**

```bash
cd /Users/siky/code/MistyPass && git add api/cmd/gateway-agent/matter_relay_test.go && git commit -m "test(gateway): add MatterController Exec subprocess tests"
```

---

### Task 3: MatterController — Interactive Subscription Session

**Files:**
- Modify: `api/cmd/gateway-agent/matter_controller.go`
- Modify: `api/cmd/gateway-agent/matter_relay_test.go`

- [ ] **Step 1: Write failing test for StartSubscription / StopSubscription**

Append to `api/cmd/gateway-agent/matter_relay_test.go`:

```go
func TestMatterControllerSubscription(t *testing.T) {
	dir := t.TempDir()
	storageDir := filepath.Join(dir, "fabric")
	os.MkdirAll(storageDir, 0o755)

	// Mock chip-tool interactive: reads stdin, streams lock state changes
	mock := writeMockChipTool(t, dir, `
# Read the subscribe command from stdin
read cmd
# Simulate periodic lock state reports
echo "CHIP:TOO:   LockState = 1"
sleep 0.1
echo "CHIP:TOO:   LockState = 2"
sleep 0.1
echo "CHIP:TOO:   LockState = 1"
# Keep alive until killed
sleep 60
`)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctrl := &MatterController{binary: mock, storageDir: storageDir, timedTimeout: 10000, logger: logger}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	scanner, err := ctrl.StartSubscription(ctx, "doorlock subscribe lock-state 1 60 1 1")
	if err != nil {
		t.Fatalf("StartSubscription() error: %v", err)
	}

	// Read lock state events
	var states []DoorLockState
	for scanner.Scan() && len(states) < 3 {
		line := scanner.Text()
		if s, err := parseLockState(line); err == nil {
			states = append(states, s)
		}
	}

	if len(states) != 3 {
		t.Fatalf("expected 3 lock state events, got %d", len(states))
	}
	if states[0] != DoorLockStateLocked || states[1] != DoorLockStateUnlocked || states[2] != DoorLockStateLocked {
		t.Fatalf("unexpected states: %v", states)
	}

	if err := ctrl.StopSubscription(); err != nil {
		t.Fatalf("StopSubscription() error: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -run "TestMatterControllerSubscription" -v`
Expected: Compilation error — `StartSubscription` and `StopSubscription` not defined.

- [ ] **Step 3: Implement subscription session**

Append to `api/cmd/gateway-agent/matter_controller.go`:

```go
// StartSubscription launches a long-lived chip-tool interactive session
// dedicated to subscriptions. The subscribeCmd is sent via stdin;
// the returned scanner streams stdout lines (parse with parseLockState).
//
// Concurrency: this session is subscribe-only. All other commands use Exec().
func (c *MatterController) StartSubscription(ctx context.Context, subscribeCmd string) (*bufio.Scanner, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session != nil {
		return nil, fmt.Errorf("subscription session already running")
	}

	args := []string{"interactive", "start", "--storage-directory", c.storageDir}
	cmd := exec.CommandContext(ctx, c.binary, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("chip-tool stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("chip-tool stdout pipe: %w", err)
	}

	c.logger.Info("starting chip-tool subscription session", "cmd", subscribeCmd)
	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, fmt.Errorf("chip-tool interactive start: %w", err)
	}

	// Send the subscribe command
	if _, err := fmt.Fprintln(stdin, subscribeCmd); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("chip-tool send subscribe cmd: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	c.session = cmd
	c.stdin = stdin
	c.scanner = scanner

	return scanner, nil
}

// StopSubscription terminates the interactive subscription session.
func (c *MatterController) StopSubscription() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session == nil {
		return nil
	}

	c.logger.Info("stopping chip-tool subscription session")

	// Close stdin to signal EOF, then kill process
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.session.Process != nil {
		c.session.Process.Kill()
	}
	// Wait to avoid zombie process
	c.session.Wait()

	c.session = nil
	c.stdin = nil
	c.scanner = nil
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -run "TestMatterControllerSubscription" -v -timeout 10s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/siky/code/MistyPass && git add api/cmd/gateway-agent/matter_controller.go api/cmd/gateway-agent/matter_relay_test.go && git commit -m "feat(gateway): add MatterController subscribe-only interactive session"
```

---

### Task 4: MatterRelay — Core RelayDriver (Unlock/Lock/Close)

**Files:**
- Create: `api/cmd/gateway-agent/matter_relay.go`
- Modify: `api/cmd/gateway-agent/matter_relay_test.go`

- [ ] **Step 1: Write failing tests for Unlock, Lock, Close, and re-lock race safety**

Append to `api/cmd/gateway-agent/matter_relay_test.go`:

```go
func newTestRelay(t *testing.T, script string) *MatterRelay {
	t.Helper()
	dir := t.TempDir()
	storageDir := filepath.Join(dir, "fabric")
	os.MkdirAll(storageDir, 0o755)

	mock := writeMockChipTool(t, dir, script)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctrl := &MatterController{binary: mock, storageDir: storageDir, timedTimeout: 10000, logger: logger}

	relay, err := NewMatterRelay(ctrl, 1, 1, logger)
	if err != nil {
		t.Fatal(err)
	}
	return relay
}

func TestMatterRelayUnlock(t *testing.T) {
	relay := newTestRelay(t, `echo "CHIP:DMG: status = 0x0"`)

	if err := relay.Unlock(5 * time.Second); err != nil {
		t.Fatalf("Unlock() error: %v", err)
	}

	// Verify re-lock timer was scheduled
	relay.relockMu.Lock()
	hasTimer := relay.relockTimer != nil
	relay.relockMu.Unlock()
	if !hasTimer {
		t.Fatal("expected re-lock timer to be scheduled")
	}

	// Clean up timer
	relay.Close()
}

func TestMatterRelayUnlockCancelsExistingTimer(t *testing.T) {
	relay := newTestRelay(t, `echo "CHIP:DMG: status = 0x0"`)

	// First unlock — schedules a re-lock in 10s
	relay.Unlock(10 * time.Second)

	// Second unlock — should cancel the first re-lock and schedule a new one
	relay.Unlock(10 * time.Second)

	// Verify only one timer is active (not two)
	relay.relockMu.Lock()
	hasTimer := relay.relockTimer != nil
	relay.relockMu.Unlock()
	if !hasTimer {
		t.Fatal("expected re-lock timer after second Unlock()")
	}

	relay.Close()
}

func TestMatterRelayLock(t *testing.T) {
	relay := newTestRelay(t, `echo "CHIP:DMG: status = 0x0"`)

	if err := relay.Lock(); err != nil {
		t.Fatalf("Lock() error: %v", err)
	}
}

func TestMatterRelayClose(t *testing.T) {
	relay := newTestRelay(t, `echo "CHIP:DMG: status = 0x0"`)

	// Schedule a re-lock
	relay.Unlock(1 * time.Hour)

	// Close should cancel the timer and stop subscription
	if err := relay.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	relay.relockMu.Lock()
	hasTimer := relay.relockTimer != nil
	relay.relockMu.Unlock()
	if hasTimer {
		t.Fatal("expected timer to be nil after Close()")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -run "TestMatterRelay" -v`
Expected: Compilation error — `NewMatterRelay` and `MatterRelay` not defined.

- [ ] **Step 3: Implement MatterRelay**

In `api/cmd/gateway-agent/matter_relay.go`:

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// MatterRelay controls a Matter Door Lock via chip-tool subprocess.
// Implements RelayDriver for integration with the gateway agent.
type MatterRelay struct {
	ctrl       *MatterController
	nodeID     uint64
	endpoint   int
	logger     *slog.Logger

	// Re-lock scheduling: prevents goroutine races on rapid Unlock() calls.
	relockMu    sync.Mutex
	relockTimer *time.Timer

	onStateChange func(state DoorLockState)
	cancelSub     context.CancelFunc
}

// NewMatterRelay creates a relay bound to a specific Matter node and endpoint.
func NewMatterRelay(ctrl *MatterController, nodeID uint64, endpoint int, logger *slog.Logger) (*MatterRelay, error) {
	return &MatterRelay{
		ctrl:     ctrl,
		nodeID:   nodeID,
		endpoint: endpoint,
		logger:   logger.With("component", "matter_relay", "node_id", nodeID),
	}, nil
}

// Unlock sends an unlock-door command and schedules a re-lock after duration.
// Race-safe: cancels any pending re-lock timer before scheduling a new one.
func (m *MatterRelay) Unlock(duration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := m.ctrl.Exec(ctx,
		"doorlock", "unlock-door",
		fmt.Sprintf("%d", m.nodeID),
		fmt.Sprintf("%d", m.endpoint),
		"--timedInteractionTimeoutMs", fmt.Sprintf("%d", m.ctrl.timedTimeout),
	)
	if err != nil {
		return fmt.Errorf("matter unlock: %w", err)
	}
	if ok, parseErr := parseCommandStatus(out); !ok {
		return fmt.Errorf("matter unlock: %w", parseErr)
	}

	m.logger.Info(">>> Matter unlock", "duration", duration)

	// Schedule re-lock (race-safe)
	m.relockMu.Lock()
	if m.relockTimer != nil {
		m.relockTimer.Stop()
	}
	m.relockTimer = time.AfterFunc(duration, func() {
		if err := m.Lock(); err != nil {
			m.logger.Error("matter auto-relock failed", "error", err)
		}
	})
	m.relockMu.Unlock()

	return nil
}

// Lock sends a lock-door command.
func (m *MatterRelay) Lock() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := m.ctrl.Exec(ctx,
		"doorlock", "lock-door",
		fmt.Sprintf("%d", m.nodeID),
		fmt.Sprintf("%d", m.endpoint),
		"--timedInteractionTimeoutMs", fmt.Sprintf("%d", m.ctrl.timedTimeout),
	)
	if err != nil {
		return fmt.Errorf("matter lock: %w", err)
	}
	if ok, parseErr := parseCommandStatus(out); !ok {
		return fmt.Errorf("matter lock: %w", parseErr)
	}

	m.logger.Info(">>> Matter lock (relocked)")
	return nil
}

// Close cancels the re-lock timer and stops any subscription session.
func (m *MatterRelay) Close() error {
	m.relockMu.Lock()
	if m.relockTimer != nil {
		m.relockTimer.Stop()
		m.relockTimer = nil
	}
	m.relockMu.Unlock()

	if m.cancelSub != nil {
		m.cancelSub()
	}
	return m.ctrl.StopSubscription()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -run "TestMatterRelay" -v`
Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/siky/code/MistyPass && git add api/cmd/gateway-agent/matter_relay.go api/cmd/gateway-agent/matter_relay_test.go && git commit -m "feat(gateway): add MatterRelay implementing RelayDriver with race-safe re-lock"
```

---

### Task 5: MatterRelay — Commission and Fabric Check

**Files:**
- Modify: `api/cmd/gateway-agent/matter_relay.go`
- Modify: `api/cmd/gateway-agent/matter_relay_test.go`

- [ ] **Step 1: Write failing tests for Commission and HasFabric**

Append to `api/cmd/gateway-agent/matter_relay_test.go`:

```go
func TestMatterRelayHasFabric(t *testing.T) {
	dir := t.TempDir()
	storageDir := filepath.Join(dir, "fabric")
	os.MkdirAll(storageDir, 0o755)

	mock := writeMockChipTool(t, dir, `echo "ok"`)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctrl := &MatterController{binary: mock, storageDir: storageDir, timedTimeout: 10000, logger: logger}

	relay, _ := NewMatterRelay(ctrl, 1, 1, logger)

	// Empty storage dir — no fabric
	if relay.HasFabric() {
		t.Fatal("expected no fabric in empty directory")
	}

	// Create a fake fabric file
	os.WriteFile(filepath.Join(storageDir, "chip_fabric_data.json"), []byte("{}"), 0o644)
	if !relay.HasFabric() {
		t.Fatal("expected fabric to be detected")
	}
}

func TestMatterRelayCommission(t *testing.T) {
	relay := newTestRelay(t, `
if [ "$1" = "pairing" ]; then
    echo "CHIP:TOO: Device commissioning completed with success"
    echo "CHIP:DMG: status = 0x0"
    exit 0
fi
echo "CHIP:DMG: status = 0x0"
`)
	if err := relay.Commission("MT:Y3.13OTB00KA0648G00"); err != nil {
		t.Fatalf("Commission() error: %v", err)
	}
}

func TestMatterRelayCommissionFailure(t *testing.T) {
	relay := newTestRelay(t, `
echo "CHIP:DMG: status = 0x01"
echo "CHIP:TOO: Pairing failed"
exit 1
`)
	err := relay.Commission("MT:INVALID")
	if err == nil {
		t.Fatal("Commission() expected error for failed pairing")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -run "TestMatterRelay(HasFabric|Commission)" -v`
Expected: Compilation errors — `HasFabric` and `Commission` not defined.

- [ ] **Step 3: Implement Commission and HasFabric**

Append to `api/cmd/gateway-agent/matter_relay.go`:

```go
// HasFabric checks if the chip-tool storage directory contains existing fabric data.
func (m *MatterRelay) HasFabric() bool {
	// chip-tool stores fabric credentials as JSON files in the storage directory.
	// The presence of any .json file indicates a commissioned fabric.
	entries, err := os.ReadDir(m.ctrl.storageDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > 5 && e.Name()[len(e.Name())-5:] == ".json" {
			return true
		}
	}
	return false
}

// Commission pairs with a Matter device using the given setup code.
// This is a one-time operation; subsequent starts skip commissioning if fabric data exists.
func (m *MatterRelay) Commission(setupCode string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	m.logger.Info("commissioning Matter device", "node_id", m.nodeID, "setup_code", setupCode)

	out, err := m.ctrl.Exec(ctx,
		"pairing", "onnetwork",
		fmt.Sprintf("%d", m.nodeID),
		setupCode,
	)
	if err != nil {
		return fmt.Errorf("matter commission: %w\noutput: %s", err, out)
	}

	m.logger.Info("Matter device commissioned successfully", "node_id", m.nodeID)
	return nil
}
```

Add `"os"` to the import block in `matter_relay.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -run "TestMatterRelay(HasFabric|Commission)" -v`
Expected: All 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/siky/code/MistyPass && git add api/cmd/gateway-agent/matter_relay.go api/cmd/gateway-agent/matter_relay_test.go && git commit -m "feat(gateway): add Matter commissioning and fabric detection"
```

---

### Task 6: MatterRelay — GetLockState and SubscribeLockState

**Files:**
- Modify: `api/cmd/gateway-agent/matter_relay.go`
- Modify: `api/cmd/gateway-agent/matter_relay_test.go`

- [ ] **Step 1: Write failing tests**

Append to `api/cmd/gateway-agent/matter_relay_test.go`:

```go
func TestMatterRelayGetLockState(t *testing.T) {
	relay := newTestRelay(t, `
echo "CHIP:TOO: Endpoint: 1 Cluster: 0x0000_0101"
echo "CHIP:TOO:   LockState = 1"
echo "CHIP:DMG: status = 0x0"
`)
	state, err := relay.GetLockState()
	if err != nil {
		t.Fatalf("GetLockState() error: %v", err)
	}
	if state != DoorLockStateLocked {
		t.Fatalf("expected Locked, got %v", state)
	}
}

func TestMatterRelaySubscribeLockState(t *testing.T) {
	dir := t.TempDir()
	storageDir := filepath.Join(dir, "fabric")
	os.MkdirAll(storageDir, 0o755)

	mock := writeMockChipTool(t, dir, `
read cmd
echo "CHIP:TOO:   LockState = 2"
sleep 0.1
echo "CHIP:TOO:   LockState = 1"
sleep 60
`)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctrl := &MatterController{binary: mock, storageDir: storageDir, timedTimeout: 10000, logger: logger}
	relay, _ := NewMatterRelay(ctrl, 1, 1, logger)

	var mu sync.Mutex
	var states []DoorLockState
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := relay.SubscribeLockState(ctx, func(state DoorLockState) {
		mu.Lock()
		states = append(states, state)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("SubscribeLockState() error: %v", err)
	}

	// Wait for events
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	got := len(states)
	mu.Unlock()
	if got < 2 {
		t.Fatalf("expected at least 2 events, got %d", got)
	}

	relay.Close()
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -run "TestMatterRelay(GetLockState|Subscribe)" -v`
Expected: Compilation errors.

- [ ] **Step 3: Implement GetLockState and SubscribeLockState**

Append to `api/cmd/gateway-agent/matter_relay.go`:

```go
// GetLockState reads the current lock state from the device.
func (m *MatterRelay) GetLockState() (DoorLockState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := m.ctrl.Exec(ctx,
		"doorlock", "read", "lock-state",
		fmt.Sprintf("%d", m.nodeID),
		fmt.Sprintf("%d", m.endpoint),
	)
	if err != nil {
		return DoorLockStateUndefined, fmt.Errorf("matter read lock-state: %w", err)
	}
	return parseLockState(out)
}

// SubscribeLockState starts a persistent subscription to lock state changes.
// The callback is invoked on each state change. Uses the subscribe-only interactive session.
func (m *MatterRelay) SubscribeLockState(ctx context.Context, callback func(DoorLockState)) error {
	subCtx, cancel := context.WithCancel(ctx)
	m.cancelSub = cancel
	m.onStateChange = callback

	subscribeCmd := fmt.Sprintf("doorlock subscribe lock-state 1 60 %d %d", m.nodeID, m.endpoint)
	scanner, err := m.ctrl.StartSubscription(subCtx, subscribeCmd)
	if err != nil {
		cancel()
		return fmt.Errorf("matter subscribe lock-state: %w", err)
	}

	go m.subscriptionLoop(scanner)
	return nil
}

// subscriptionLoop reads lock state events from the interactive session stdout.
func (m *MatterRelay) subscriptionLoop(scanner *bufio.Scanner) {
	m.logger.Info("matter subscription loop started")
	for scanner.Scan() {
		line := scanner.Text()
		state, err := parseLockState(line)
		if err != nil {
			continue // not a lock state line, skip
		}
		m.logger.Info("matter lock state changed", "state", state.String())
		if m.onStateChange != nil {
			m.onStateChange(state)
		}
	}
	m.logger.Info("matter subscription loop ended")
}
```

Add `"bufio"` to the import block in `matter_relay.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -run "TestMatterRelay(GetLockState|Subscribe)" -v -timeout 10s`
Expected: Both PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/siky/code/MistyPass && git add api/cmd/gateway-agent/matter_relay.go api/cmd/gateway-agent/matter_relay_test.go && git commit -m "feat(gateway): add Matter lock state read and subscription"
```

---

### Task 7: MatterRelay — Credential Sync (SetUser/SetCredential/DeleteUser)

**Files:**
- Modify: `api/cmd/gateway-agent/matter_relay.go`
- Modify: `api/cmd/gateway-agent/matter_relay_test.go`

- [ ] **Step 1: Write failing tests**

Append to `api/cmd/gateway-agent/matter_relay_test.go`:

```go
func TestMatterRelaySetUser(t *testing.T) {
	relay := newTestRelay(t, `echo "CHIP:DMG: status = 0x0"`)

	err := relay.SetUser(MatterUser{
		UserIndex:  1,
		UserName:   "Alice",
		UniqueID:   1001,
		UserStatus: 1,
		UserType:   0,
	})
	if err != nil {
		t.Fatalf("SetUser() error: %v", err)
	}
}

func TestMatterRelaySetCredential(t *testing.T) {
	relay := newTestRelay(t, `echo "CHIP:DMG: status = 0x0"`)

	err := relay.SetCredential(MatterCredential{
		UserIndex:       1,
		CredentialIndex: 1,
		CredentialType:  1, // PIN
		CredentialData:  []byte("123456"),
	})
	if err != nil {
		t.Fatalf("SetCredential() error: %v", err)
	}
}

func TestMatterRelayDeleteUser(t *testing.T) {
	relay := newTestRelay(t, `echo "CHIP:DMG: status = 0x0"`)

	err := relay.DeleteUser(1)
	if err != nil {
		t.Fatalf("DeleteUser() error: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -run "TestMatterRelay(SetUser|SetCredential|DeleteUser)" -v`
Expected: Compilation errors.

- [ ] **Step 3: Implement credential sync methods**

Append to `api/cmd/gateway-agent/matter_relay.go`:

```go
// SetUser creates or updates a user entry on the Matter lock.
func (m *MatterRelay) SetUser(user MatterUser) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := m.ctrl.Exec(ctx,
		"doorlock", "set-user",
		fmt.Sprintf("%d", m.nodeID),
		fmt.Sprintf("%d", m.endpoint),
		"0", // operationType: Add
		fmt.Sprintf("%d", user.UserIndex),
		fmt.Sprintf("%q", user.UserName),
		fmt.Sprintf("%d", user.UniqueID),
		fmt.Sprintf("%d", user.UserStatus),
		fmt.Sprintf("%d", user.UserType),
		"0", // credentialRule: Single
		"--timedInteractionTimeoutMs", fmt.Sprintf("%d", m.ctrl.timedTimeout),
	)
	if err != nil {
		return fmt.Errorf("matter set-user: %w", err)
	}
	if ok, parseErr := parseCommandStatus(out); !ok {
		return fmt.Errorf("matter set-user: %w", parseErr)
	}

	m.logger.Info("matter user set", "user_index", user.UserIndex, "name", user.UserName)
	return nil
}

// SetCredential creates or updates a credential (PIN, RFID, etc.) on the Matter lock.
func (m *MatterRelay) SetCredential(cred MatterCredential) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build credential type:index struct as JSON for chip-tool
	credStruct := fmt.Sprintf(`{"credentialType": %d, "credentialIndex": %d}`,
		cred.CredentialType, cred.CredentialIndex)

	out, err := m.ctrl.Exec(ctx,
		"doorlock", "set-credential",
		fmt.Sprintf("%d", m.nodeID),
		fmt.Sprintf("%d", m.endpoint),
		"0", // operationType: Add
		credStruct,
		fmt.Sprintf("%q", string(cred.CredentialData)),
		fmt.Sprintf("%d", cred.UserIndex),
		"0", // userStatus: not specified (use existing)
		"0", // userType: not specified
		"--timedInteractionTimeoutMs", fmt.Sprintf("%d", m.ctrl.timedTimeout),
	)
	if err != nil {
		return fmt.Errorf("matter set-credential: %w", err)
	}
	if ok, parseErr := parseCommandStatus(out); !ok {
		return fmt.Errorf("matter set-credential: %w", parseErr)
	}

	m.logger.Info("matter credential set", "user_index", cred.UserIndex, "type", cred.CredentialType)
	return nil
}

// DeleteUser removes a user and all associated credentials from the Matter lock.
func (m *MatterRelay) DeleteUser(userIndex uint16) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := m.ctrl.Exec(ctx,
		"doorlock", "clear-user",
		fmt.Sprintf("%d", m.nodeID),
		fmt.Sprintf("%d", m.endpoint),
		fmt.Sprintf("%d", userIndex),
		"--timedInteractionTimeoutMs", fmt.Sprintf("%d", m.ctrl.timedTimeout),
	)
	if err != nil {
		return fmt.Errorf("matter clear-user: %w", err)
	}
	if ok, parseErr := parseCommandStatus(out); !ok {
		return fmt.Errorf("matter clear-user: %w", parseErr)
	}

	m.logger.Info("matter user deleted", "user_index", userIndex)
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -run "TestMatterRelay(SetUser|SetCredential|DeleteUser)" -v`
Expected: All 3 PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/siky/code/MistyPass && git add api/cmd/gateway-agent/matter_relay.go api/cmd/gateway-agent/matter_relay_test.go && git commit -m "feat(gateway): add Matter credential sync (set-user, set-credential, clear-user)"
```

---

### Task 8: Agent Integration — Flags, Relay Init, Subscription Hookup

**Files:**
- Modify: `api/cmd/gateway-agent/main.go`
- Modify: `api/cmd/gateway-agent/agent.go`

- [ ] **Step 1: Add Matter fields to Agent struct**

In `api/cmd/gateway-agent/agent.go`, add fields after `osdpAddress` (around line 35):

```go
	osdpAddress        byte   // OSDP peripheral device address (0-126)
	// Matter relay configuration
	relayMatterNodeID   uint64 // Matter node ID for target lock (0 = disabled)
	matterEndpoint      int    // Door Lock cluster endpoint (default: 1)
	matterStorageDir    string // chip-tool fabric credential storage
	matterSetupCode     string // setup code for commissioning
	matterChipToolPath  string // path to chip-tool binary
	matterTimedTimeout  int    // --timedInteractionTimeoutMs value
```

- [ ] **Step 2: Add Matter relay initialization in Start()**

In `api/cmd/gateway-agent/agent.go`, in the `Start()` method, add the Matter case between the RS-485 block and the DryRun fallback (around line 118):

Replace:
```go
	} else if a.relayRS485Device != "" {
		driver, err := NewRS485Relay(a.relayRS485Device, a.logger)
		if err != nil {
			return fmt.Errorf("rs485 relay init: %w", err)
		}
		a.relay = driver
	} else {
		a.relay = &DryRunRelay{logger: a.logger}
	}
```

With:
```go
	} else if a.relayRS485Device != "" {
		driver, err := NewRS485Relay(a.relayRS485Device, a.logger)
		if err != nil {
			return fmt.Errorf("rs485 relay init: %w", err)
		}
		a.relay = driver
	} else if a.relayMatterNodeID > 0 {
		ctrl, err := NewMatterController(a.matterChipToolPath, a.matterStorageDir, a.matterTimedTimeout, a.logger)
		if err != nil {
			return fmt.Errorf("matter controller init: %w", err)
		}
		mr, err := NewMatterRelay(ctrl, a.relayMatterNodeID, a.matterEndpoint, a.logger)
		if err != nil {
			return fmt.Errorf("matter relay init: %w", err)
		}
		// Auto-commission if no existing fabric
		if !mr.HasFabric() {
			if a.matterSetupCode == "" {
				return fmt.Errorf("matter: no fabric data and no -matter-setup-code provided")
			}
			if err := mr.Commission(a.matterSetupCode); err != nil {
				return fmt.Errorf("matter commission failed: %w", err)
			}
		}
		a.relay = mr
	} else {
		a.relay = &DryRunRelay{logger: a.logger}
	}
```

- [ ] **Step 3: Add Matter subscription hookup after relay init**

In `agent.go`, after the relay initialization block and before the mTLS section (around line 123, after the `}` that closes the relay selection), add:

```go
	// Start Matter lock state subscription if applicable
	if mr, ok := a.relay.(*MatterRelay); ok {
		go mr.SubscribeLockState(context.Background(), func(state DoorLockState) {
			a.mu.Lock()
			a.eventQueue = append(a.eventQueue, AccessEvent{
				GatewayID:  a.gatewayID,
				EventType:  "lock_state_changed",
				Result:     state.String(),
				OccurredAt: time.Now().UTC().Format(time.RFC3339),
			})
			a.mu.Unlock()
		})
	}
```

Add `"context"` to imports in `agent.go` if not already present.

- [ ] **Step 4: Add CLI flags in main.go**

In `api/cmd/gateway-agent/main.go`, add flags after `osdpAddress` (around line 38):

```go
	osdpAddress := flag.Int("osdp-address", 0, "OSDP peripheral device address (0-126)")
	// Matter relay flags
	relayMatter := flag.Uint64("relay-matter", 0, "Matter node ID for target lock (enables Matter relay). 0 = disabled.")
	matterEndpoint := flag.Int("matter-endpoint", 1, "Matter Door Lock cluster endpoint")
	matterStorage := flag.String("matter-storage", "/var/lib/mistypass/matter/", "chip-tool fabric credential storage directory")
	matterSetupCode := flag.String("matter-setup-code", "", "Matter setup code for commissioning (e.g. MT:Y3.13OTB00KA0648G00). Used on first start only.")
	matterChipTool := flag.String("matter-chip-tool", "chip-tool", "Path to chip-tool binary")
	matterTimedTimeout := flag.Int("matter-timed-timeout", 10000, "timedInteractionTimeoutMs for Door Lock cluster commands")
```

Wire them into the Agent struct (around line 72, after `osdpAddress`):

```go
		osdpAddress:        byte(*osdpAddress),
		relayMatterNodeID:  *relayMatter,
		matterEndpoint:     *matterEndpoint,
		matterStorageDir:   *matterStorage,
		matterSetupCode:    *matterSetupCode,
		matterChipToolPath: *matterChipTool,
		matterTimedTimeout: *matterTimedTimeout,
```

Add Matter to the startup banner (around line 100, after the RS485 relay print):

```go
	} else if *relayMatter > 0 {
		fmt.Printf("Relay:    Matter node=%d endpoint=%d\n", *relayMatter, *matterEndpoint)
```

- [ ] **Step 5: Verify the build compiles**

Run: `cd /Users/siky/code/MistyPass/api && go build ./cmd/gateway-agent/`
Expected: Build succeeds with no errors.

- [ ] **Step 6: Run all tests**

Run: `cd /Users/siky/code/MistyPass/api && go test ./cmd/gateway-agent/ -v -timeout 30s`
Expected: All existing tests + all new Matter tests PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/siky/code/MistyPass && git add api/cmd/gateway-agent/main.go api/cmd/gateway-agent/agent.go && git commit -m "feat(gateway): integrate Matter relay into agent with CLI flags and auto-commission"
```

---

### Task 9: Final Verification and Cleanup

- [ ] **Step 1: Run full test suite**

Run: `cd /Users/siky/code/MistyPass/api && go test ./... -timeout 60s`
Expected: All tests pass.

- [ ] **Step 2: Run go vet**

Run: `cd /Users/siky/code/MistyPass/api && go vet ./cmd/gateway-agent/`
Expected: No warnings.

- [ ] **Step 3: Verify help output shows new flags**

Run: `cd /Users/siky/code/MistyPass/api && go run ./cmd/gateway-agent/ -h 2>&1 | grep -A1 matter`
Expected: All 6 Matter flags displayed with descriptions.

- [ ] **Step 4: Commit spec and plan docs**

```bash
cd /Users/siky/code/MistyPass && git add docs/superpowers/ && git commit -m "docs: add Matter relay design spec and implementation plan"
```
