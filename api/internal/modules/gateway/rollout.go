package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
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

	defaultRolloutFailureThresholdPct = 20
	rolloutStallWindow                = time.Hour
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
