package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// MatterRelay controls a Matter Door Lock via chip-tool subprocess.
// Implements RelayDriver for integration with the gateway agent.
type MatterRelay struct {
	ctrl     *MatterController
	nodeID   uint64
	endpoint int
	logger   *slog.Logger

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

// HasFabric checks if the chip-tool storage directory contains existing fabric data.
func (m *MatterRelay) HasFabric() bool {
	// chip-tool stores fabric credentials as JSON files in the storage directory.
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
