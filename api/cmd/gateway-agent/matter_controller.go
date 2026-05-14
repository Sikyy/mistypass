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
	"time"
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
	reStatus    = regexp.MustCompile(`status\s*=\s*0x([0-9a-fA-F]+)`)
	reLockState = regexp.MustCompile(`LockState\s*=\s*(\d+)`)
	reVersion   = regexp.MustCompile(`Version:\s*([\d.]+)`)
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
	_ = c.session.Wait() // expected error from Kill()

	c.session = nil
	c.stdin = nil
	c.scanner = nil
	return nil
}
