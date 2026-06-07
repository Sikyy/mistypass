package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"
)

var (
	ErrRolloutNotFound         = errors.New("gateway rollout not found")
	ErrRolloutFirmwareRequired = errors.New("rollout firmware_id is required")
	ErrRolloutTargetEmpty      = errors.New("rollout target resolves to no gateways")
	ErrRolloutPhasesInvalid    = errors.New("rollout phases must be non-empty, strictly increasing percentages ending at 100 (each 1-100)")
	ErrRolloutThresholdInvalid = errors.New("rollout failure_threshold_pct must be 0-100")
	ErrRolloutStateConflict    = errors.New("rollout action not allowed in current state")
)

const (
	rolloutStatePending          = "pending"
	rolloutStateActive           = "active"
	rolloutStateAwaitingApproval = "awaiting_approval"
	rolloutStatePaused           = "paused"
	rolloutStateCompleted        = "completed"
	rolloutStateFailed           = "failed"

	// applied when a rollout's FailureThresholdPct is 0 at creation
	defaultRolloutFailureThresholdPct = 20
	// a cohort gateway with no terminal task past this window counts as failed
	rolloutStallWindow = time.Hour
)

// RolloutTarget selects which gateways a rollout covers.
type RolloutTarget struct {
	Kind       string   `json:"kind"` // "all" | "building" | "gateways"
	BuildingID string   `json:"building_id,omitempty"`
	GatewayIDs []string `json:"gateway_ids,omitempty"`
}

// RolloutPhase is one cumulative-coverage wave.
type RolloutPhase struct {
	Percentage       int  `json:"percentage"`        // cumulative 1-100, strictly increasing, last == 100
	RequiresApproval bool `json:"requires_approval"` // pause for manual approval before entering this phase
}

// GatewayRollout is a phased firmware rollout over a set of gateways.
type GatewayRollout struct {
	ID                  string         `json:"id"`
	TenantID            string         `json:"tenant_id"`
	FirmwareID          string         `json:"firmware_id"`
	FirmwareVersion     string         `json:"firmware_version"`
	Target              RolloutTarget  `json:"target"`
	Phases              []RolloutPhase `json:"phases"`
	FailureThresholdPct int            `json:"failure_threshold_pct"`
	State               string         `json:"state"`
	CurrentPhase        int            `json:"current_phase"`
	PhaseStartedAt      time.Time      `json:"phase_started_at"`
	CreatedBy           string         `json:"created_by,omitempty"`
	UpdatedBy           string         `json:"updated_by,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

func rolloutRecordID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "rollout_" + hex.EncodeToString(b), nil
}

// validateRolloutPhases requires non-empty, each in 1..100, strictly increasing, last == 100.
func validateRolloutPhases(phases []RolloutPhase) bool {
	if len(phases) == 0 {
		return false
	}
	prev := 0
	for _, p := range phases {
		if p.Percentage < 1 || p.Percentage > 100 || p.Percentage <= prev {
			return false
		}
		prev = p.Percentage
	}
	return prev == 100
}

// cohortForPhase returns the gateways newly covered by phaseIdx (cumulative slicing).
func cohortForPhase(all []Gateway, phases []RolloutPhase, phaseIdx int) []Gateway {
	n := len(all)
	cum := func(i int) int {
		if i < 0 {
			return 0
		}
		c := (phases[i].Percentage*n + 99) / 100 // ceil(pct*n/100)
		if c > n {
			c = n
		}
		return c
	}
	start, end := cum(phaseIdx-1), cum(phaseIdx)
	if start > end {
		start = end
	}
	return all[start:end]
}

// rolloutTargetGatewaysLocked resolves a target to a tenant-scoped, ID-sorted gateway set.
// Caller holds s.mu.
func (s *Service) rolloutTargetGatewaysLocked(tenantID string, target RolloutTarget) []Gateway {
	wantIDs := map[string]struct{}{}
	for _, id := range target.GatewayIDs {
		wantIDs[id] = struct{}{}
	}
	var out []Gateway
	for i := range s.gateways {
		if s.gateways[i].TenantID != tenantID {
			continue
		}
		switch target.Kind {
		case "all":
			out = append(out, s.gateways[i])
		case "building":
			if s.gateways[i].BuildingID == target.BuildingID {
				out = append(out, s.gateways[i])
			}
		case "gateways":
			if _, ok := wantIDs[s.gateways[i].ID]; ok {
				out = append(out, s.gateways[i])
			}
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

func cloneGatewayRollouts(in []GatewayRollout) []GatewayRollout {
	if len(in) == 0 {
		return nil
	}
	out := make([]GatewayRollout, len(in))
	copy(out, in)
	return out
}

// CreateRolloutInput carries a new rollout request.
type CreateRolloutInput struct {
	TenantID            string
	FirmwareID          string
	Target              RolloutTarget
	Phases              []RolloutPhase
	FailureThresholdPct int
	CreatedBy           string
}

// CreateRollout validates, persists, and immediately starts phase 0.
func (s *Service) CreateRollout(in CreateRolloutInput) (GatewayRollout, error) {
	tenantID := strings.TrimSpace(in.TenantID)
	fwID := strings.TrimSpace(in.FirmwareID)
	if fwID == "" {
		return GatewayRollout{}, ErrRolloutFirmwareRequired
	}
	if !validateRolloutPhases(in.Phases) {
		return GatewayRollout{}, ErrRolloutPhasesInvalid
	}
	threshold := in.FailureThresholdPct
	if threshold == 0 {
		threshold = defaultRolloutFailureThresholdPct
	}
	if threshold < 0 || threshold > 100 {
		return GatewayRollout{}, ErrRolloutThresholdInvalid
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	fw, ok := s.findFirmwareLocked(fwID, tenantID)
	if !ok {
		return GatewayRollout{}, ErrGatewayFirmwareNotFound
	}
	all := s.rolloutTargetGatewaysLocked(tenantID, in.Target)
	if len(all) == 0 {
		return GatewayRollout{}, ErrRolloutTargetEmpty
	}

	id, err := rolloutRecordID()
	if err != nil {
		return GatewayRollout{}, err
	}
	now := time.Now().UTC()
	r := GatewayRollout{
		ID:                  id,
		TenantID:            tenantID,
		FirmwareID:          fwID,
		FirmwareVersion:     fw.Version,
		Target:              in.Target,
		Phases:              in.Phases,
		FailureThresholdPct: threshold,
		State:               rolloutStatePending,
		CurrentPhase:        0,
		CreatedBy:           strings.TrimSpace(in.CreatedBy),
		UpdatedBy:           strings.TrimSpace(in.CreatedBy),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	s.rollouts = append([]GatewayRollout{r}, s.rollouts...)
	s.startRolloutPhaseLocked(&s.rollouts[0], 0, all, now)
	if err := s.persistLocked(); err != nil {
		return GatewayRollout{}, err
	}
	return s.rollouts[0], nil
}

// startRolloutPhaseLocked creates OTA tasks for the phase cohort and marks the rollout active.
// Caller holds s.mu.
func (s *Service) startRolloutPhaseLocked(r *GatewayRollout, phase int, all []Gateway, now time.Time) {
	fw, ok := s.findFirmwareLocked(r.FirmwareID, r.TenantID)
	if !ok {
		r.State = rolloutStateFailed
		r.UpdatedAt = now
		return
	}
	for _, gw := range cohortForPhase(all, r.Phases, phase) {
		s.appendRolloutOTATaskLocked(gw, fw, r.ID, phase, now)
	}
	r.CurrentPhase = phase
	r.PhaseStartedAt = now
	r.State = rolloutStateActive
	r.UpdatedAt = now
}

// appendRolloutOTATaskLocked builds a queued OTA task attributed to a rollout phase.
// Firmware sha/sig come from the (already-validated) registry record. Caller holds s.mu.
func (s *Service) appendRolloutOTATaskLocked(gw Gateway, fw GatewayFirmware, rolloutID string, phase int, now time.Time) {
	taskID, err := otaTaskID()
	if err != nil {
		return
	}
	s.otaTasks = append([]GatewayOTATask{{
		ID:                taskID,
		GatewayID:         gw.ID,
		TenantID:          gw.TenantID,
		FirmwareVersion:   fw.Version,
		FirmwareSHA256:    fw.SHA256,
		FirmwareSignature: fw.Signature,
		FirmwareID:        fw.ID,
		Status:            gatewayOTATaskStatusQueued,
		RequestedBy:       "rollout:" + rolloutID,
		UpdatedBy:         "rollout:" + rolloutID,
		RolloutID:         rolloutID,
		RolloutPhase:      phase,
		CreatedAt:         now,
		UpdatedAt:         now,
	}}, s.otaTasks...)
}

// latestRolloutTaskLocked finds the newest OTA task for a rollout phase + gateway. Caller holds s.mu.
func (s *Service) latestRolloutTaskLocked(rolloutID string, phase int, gatewayID string) (GatewayOTATask, bool) {
	for i := range s.otaTasks {
		if s.otaTasks[i].RolloutID == rolloutID && s.otaTasks[i].RolloutPhase == phase && s.otaTasks[i].GatewayID == gatewayID {
			return s.otaTasks[i], true
		}
	}
	return GatewayOTATask{}, false
}

// evaluateRolloutPhaseLocked reports whether the current phase is terminal and its failure rate (%).
// A cohort gateway with no terminal task counts as failed once the stall window elapses.
// Empty cohort → terminal with 0% failure. Caller holds s.mu.
func (s *Service) evaluateRolloutPhaseLocked(r *GatewayRollout, all []Gateway, now time.Time) (bool, int) {
	cohort := cohortForPhase(all, r.Phases, r.CurrentPhase)
	total := len(cohort)
	if total == 0 {
		return true, 0
	}
	stalled := now.Sub(r.PhaseStartedAt) > rolloutStallWindow
	// Only succeeded/failed (and stall) are terminal; queued/dispatching keep the phase in-flight.
	failed, terminal := 0, 0
	for _, gw := range cohort {
		task, ok := s.latestRolloutTaskLocked(r.ID, r.CurrentPhase, gw.ID)
		switch {
		case ok && task.Status == gatewayOTATaskStatusSucceeded:
			terminal++
		case ok && task.Status == gatewayOTATaskStatusFailed:
			terminal++
			failed++
		case stalled:
			terminal++
			failed++
		}
	}
	if terminal < total {
		return false, 0
	}
	// Ceiling: a failure gate should err toward pausing, so round the rate up.
	return true, (failed*100 + total - 1) / total
}

// advanceRolloutLocked drives an active rollout forward as far as it can.
// Returns true if it changed the rollout's state/phase. Caller holds s.mu.
func (s *Service) advanceRolloutLocked(rolloutIdx int) bool {
	r := &s.rollouts[rolloutIdx]
	now := time.Now().UTC()
	all := s.rolloutTargetGatewaysLocked(r.TenantID, r.Target) // target set is immutable under the lock
	changed := false
	for r.State == rolloutStateActive {
		terminal, failureRate := s.evaluateRolloutPhaseLocked(r, all, now)
		if !terminal {
			break
		}
		changed = true
		r.UpdatedAt = now
		if failureRate >= r.FailureThresholdPct {
			r.State = rolloutStatePaused
			break
		}
		if r.CurrentPhase >= len(r.Phases)-1 {
			r.State = rolloutStateCompleted
			break
		}
		next := r.CurrentPhase + 1
		if r.Phases[next].RequiresApproval {
			r.State = rolloutStateAwaitingApproval
			break
		}
		s.startRolloutPhaseLocked(r, next, all, now) // creates next cohort; loop re-evaluates (empty cohort auto-skips)
	}
	return changed
}

// GetRollout evaluates the rollout (catches stall timeouts) then returns it, tenant-scoped.
func (s *Service) GetRollout(tenantID, id string) (GatewayRollout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.findRolloutIndexLocked(strings.TrimSpace(id), strings.TrimSpace(tenantID))
	if i < 0 {
		return GatewayRollout{}, ErrRolloutNotFound
	}
	if s.advanceRolloutLocked(i) {
		if err := s.persistLocked(); err != nil {
			return GatewayRollout{}, err
		}
	}
	return s.rollouts[i], nil
}

// ListRollouts returns a tenant's rollouts, newest first.
func (s *Service) ListRollouts(tenantID string) []GatewayRollout {
	ft := strings.TrimSpace(tenantID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]GatewayRollout, 0, len(s.rollouts))
	for i := range s.rollouts {
		if ft == "" || s.rollouts[i].TenantID == ft {
			out = append(out, s.rollouts[i])
		}
	}
	return out
}

// findRolloutIndexLocked returns the index of a rollout (optionally tenant-filtered), or -1.
func (s *Service) findRolloutIndexLocked(id, tenantID string) int {
	for i := range s.rollouts {
		if s.rollouts[i].ID == id && (tenantID == "" || s.rollouts[i].TenantID == tenantID) {
			return i
		}
	}
	return -1
}

// PauseRollout halts an active rollout. Only valid from "active".
func (s *Service) PauseRollout(tenantID, id, actor string) (GatewayRollout, error) {
	return s.transitionRolloutIdx(tenantID, id, func(ri int, now time.Time) error {
		s.advanceRolloutLocked(ri) // refresh state first (a finished/stalled phase may have completed/paused it)
		r := &s.rollouts[ri]
		if r.State != rolloutStateActive {
			return ErrRolloutStateConflict
		}
		r.State = rolloutStatePaused
		r.UpdatedBy = actor
		r.UpdatedAt = now
		return nil
	})
}

// ResumeRollout reactivates a paused rollout. If the current phase is already terminal
// (e.g. it was paused by the failure gate), resume forces past it — an explicit operator override.
func (s *Service) ResumeRollout(tenantID, id, actor string) (GatewayRollout, error) {
	return s.transitionRolloutIdx(tenantID, id, func(ri int, now time.Time) error {
		r := &s.rollouts[ri]
		if r.State != rolloutStatePaused {
			return ErrRolloutStateConflict
		}
		r.State = rolloutStateActive
		r.UpdatedBy = actor
		r.UpdatedAt = now
		all := s.rolloutTargetGatewaysLocked(r.TenantID, r.Target)
		terminal, _ := s.evaluateRolloutPhaseLocked(r, all, now)
		if !terminal {
			s.advanceRolloutLocked(ri) // re-evaluate with a fresh clock; may detect a stall and pause
			return nil
		}
		// Terminal: override the failure gate and move past the current phase.
		if r.CurrentPhase >= len(r.Phases)-1 {
			r.State = rolloutStateCompleted
			return nil
		}
		next := r.CurrentPhase + 1
		if r.Phases[next].RequiresApproval {
			r.State = rolloutStateAwaitingApproval
			return nil
		}
		s.startRolloutPhaseLocked(r, next, all, now)
		s.advanceRolloutLocked(ri)
		return nil
	})
}

// ApproveRollout advances an awaiting-approval rollout into its next phase.
func (s *Service) ApproveRollout(tenantID, id, actor string) (GatewayRollout, error) {
	return s.transitionRolloutIdx(tenantID, id, func(ri int, now time.Time) error {
		r := &s.rollouts[ri]
		if r.State != rolloutStateAwaitingApproval {
			return ErrRolloutStateConflict
		}
		r.UpdatedBy = actor
		all := s.rolloutTargetGatewaysLocked(r.TenantID, r.Target)
		s.startRolloutPhaseLocked(r, r.CurrentPhase+1, all, now)
		s.advanceRolloutLocked(ri)
		return nil
	})
}

// AbortRollout fails a non-terminal rollout; no further tasks are created.
func (s *Service) AbortRollout(tenantID, id, actor string) (GatewayRollout, error) {
	return s.transitionRolloutIdx(tenantID, id, func(ri int, now time.Time) error {
		s.advanceRolloutLocked(ri) // refresh state first; a just-completed rollout can't be aborted
		r := &s.rollouts[ri]
		if r.State == rolloutStateCompleted || r.State == rolloutStateFailed {
			return ErrRolloutStateConflict
		}
		r.State = rolloutStateFailed
		r.UpdatedBy = actor
		r.UpdatedAt = now
		return nil
	})
}

// transitionRolloutIdx runs fn against a tenant-scoped rollout (by index) under the write
// lock, then persists. fn assumes s.mu is held.
func (s *Service) transitionRolloutIdx(tenantID, id string, fn func(int, time.Time) error) (GatewayRollout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ri := s.findRolloutIndexLocked(strings.TrimSpace(id), strings.TrimSpace(tenantID))
	if ri < 0 {
		return GatewayRollout{}, ErrRolloutNotFound
	}
	if err := fn(ri, time.Now().UTC()); err != nil {
		return GatewayRollout{}, err
	}
	if err := s.persistLocked(); err != nil {
		return GatewayRollout{}, err
	}
	return s.rollouts[ri], nil
}

// RolloutGatewayStatus is one gateway's progress within a rollout.
type RolloutGatewayStatus struct {
	GatewayID              string `json:"gateway_id"`
	Phase                  int    `json:"phase"`      // -1 if not yet in any created cohort
	OTAStatus              string `json:"ota_status"` // queued|dispatching|succeeded|failed|timed_out|pending
	CurrentFirmwareVersion string `json:"current_firmware_version,omitempty"`
}

// RolloutGatewayProgress returns per-gateway progress for a rollout's full target set.
func (s *Service) RolloutGatewayProgress(tenantID, id string) ([]RolloutGatewayStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ri := s.findRolloutIndexLocked(strings.TrimSpace(id), strings.TrimSpace(tenantID))
	if ri < 0 {
		return nil, ErrRolloutNotFound
	}
	r := s.rollouts[ri]
	all := s.rolloutTargetGatewaysLocked(r.TenantID, r.Target)
	now := time.Now().UTC()
	out := make([]RolloutGatewayStatus, 0, len(all))
	for _, gw := range all {
		st := RolloutGatewayStatus{GatewayID: gw.ID, Phase: -1, OTAStatus: "pending", CurrentFirmwareVersion: gw.CurrentFirmwareVersion}
		for i := range s.otaTasks {
			if s.otaTasks[i].RolloutID == r.ID && s.otaTasks[i].GatewayID == gw.ID {
				st.Phase = s.otaTasks[i].RolloutPhase
				st.OTAStatus = s.otaTasks[i].Status
				if s.otaTasks[i].Status != gatewayOTATaskStatusSucceeded &&
					s.otaTasks[i].Status != gatewayOTATaskStatusFailed &&
					st.Phase == r.CurrentPhase &&
					now.Sub(r.PhaseStartedAt) > rolloutStallWindow {
					st.OTAStatus = "timed_out"
				}
				break
			}
		}
		out = append(out, st)
	}
	return out, nil
}
