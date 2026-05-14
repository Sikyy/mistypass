package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

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

// writeMockChipTool creates a shell script that mimics chip-tool output.
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

func TestMatterControllerSubscription(t *testing.T) {
	dir := t.TempDir()
	storageDir := filepath.Join(dir, "fabric")
	os.MkdirAll(storageDir, 0o755)

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

	var states []DoorLockState
	for scanner.Scan() {
		line := scanner.Text()
		if s, err := parseLockState(line); err == nil {
			states = append(states, s)
		}
		if len(states) >= 3 {
			break
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

// newTestRelay creates a MatterRelay with a mock chip-tool for testing.
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

	relay.mu.Lock()
	hasTimer := relay.relockTimer != nil
	relay.mu.Unlock()
	if !hasTimer {
		t.Fatal("expected re-lock timer to be scheduled")
	}

	relay.Close()
}

func TestMatterRelayUnlockCancelsExistingTimer(t *testing.T) {
	relay := newTestRelay(t, `echo "CHIP:DMG: status = 0x0"`)

	relay.Unlock(10 * time.Second)
	relay.Unlock(10 * time.Second)

	relay.mu.Lock()
	hasTimer := relay.relockTimer != nil
	relay.mu.Unlock()
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

	relay.Unlock(1 * time.Hour)

	if err := relay.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	relay.mu.Lock()
	hasTimer := relay.relockTimer != nil
	relay.mu.Unlock()
	if hasTimer {
		t.Fatal("expected timer to be nil after Close()")
	}
}

func TestMatterRelayHasFabric(t *testing.T) {
	dir := t.TempDir()
	storageDir := filepath.Join(dir, "fabric")
	os.MkdirAll(storageDir, 0o755)

	mock := writeMockChipTool(t, dir, `echo "ok"`)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctrl := &MatterController{binary: mock, storageDir: storageDir, timedTimeout: 10000, logger: logger}

	relay, _ := NewMatterRelay(ctrl, 1, 1, logger)

	if relay.HasFabric() {
		t.Fatal("expected no fabric in empty directory")
	}

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

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	got := len(states)
	mu.Unlock()
	if got < 2 {
		t.Fatalf("expected at least 2 events, got %d", got)
	}

	relay.Close()
}

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
		CredentialType:  1,
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
