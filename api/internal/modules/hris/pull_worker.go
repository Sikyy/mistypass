package hris

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/retrybackoff"
)

var ErrPullAdapterNotFound = errors.New("hris pull adapter not found")
var ErrPullConnectorIDRequired = errors.New("hris pull connector_id is required")
var ErrPullStateNotFound = errors.New("hris pull state not found")
var ErrPullFailureRequired = errors.New("hris pull failure is required")
var ErrPullStateConflict = errors.New("hris pull state conflict")

const pullStateKey = "module_hris_pull"
const maxPullStateCASRetries = 5

const (
	PullModeFull                     = "full"
	PullModeIncremental              = "incremental"
	PullStateClaimReasonAttemptLimit = "attempt_limit"
	PullStateClaimReasonCooldown     = "cooldown"
	PullStateClaimReasonInFlight     = "in_flight"
	PullStateClaimReasonNotQueueable = "not_queueable"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type PullInput struct {
	Connector         enterprise.HRISConnector
	CredentialValue   string
	LastSuccessAt     *time.Time
	LastFullSuccessAt *time.Time
	Mode              string
	HTTPClient        HTTPDoer
	Now               time.Time
}

type PullResult struct {
	TenantID    string                         `json:"tenant_id"`
	Source      string                         `json:"source"`
	Actor       string                         `json:"actor"`
	RequestID   string                         `json:"request_id"`
	Mode        string                         `json:"mode,omitempty"`
	ConnectorID string                         `json:"connector_id,omitempty"`
	Employees   []enterprise.EmployeeSyncInput `json:"employees,omitempty"`
	PulledAt    time.Time                      `json:"pulled_at"`
}

type PullAdapter interface {
	Vendor() string
	Pull(ctx context.Context, input PullInput) (PullResult, error)
}

type IncrementalPullSupporter interface {
	SupportsIncremental(input PullInput) bool
}

type PullRegistry struct {
	mu       sync.RWMutex
	adapters map[string]PullAdapter
}

type ConnectorPullState struct {
	TenantID            string     `json:"tenant_id"`
	ConnectorID         string     `json:"connector_id"`
	Vendor              string     `json:"vendor"`
	Status              string     `json:"status"`
	LastRequestID       string     `json:"last_request_id,omitempty"`
	LastMode            string     `json:"last_mode,omitempty"`
	LastStartedAt       *time.Time `json:"last_started_at,omitempty"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`
	LastFullSuccessAt   *time.Time `json:"last_full_success_at,omitempty"`
	LastFailureAt       *time.Time `json:"last_failure_at,omitempty"`
	LastError           string     `json:"last_error,omitempty"`
	ConsecutiveFailures int        `json:"consecutive_failures,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type pullStateSnapshot struct {
	States []ConnectorPullState `json:"states,omitempty"`
}

type PullStateService struct {
	mu         sync.RWMutex
	states     []ConnectorPullState
	stateStore StateStore
}

func NewPullRegistry(adapters ...PullAdapter) *PullRegistry {
	registry := &PullRegistry{
		adapters: make(map[string]PullAdapter, len(adapters)),
	}
	for i := range adapters {
		if adapters[i] == nil {
			continue
		}
		registry.Register(adapters[i])
	}
	return registry
}

func (r *PullRegistry) Register(adapter PullAdapter) {
	if r == nil || adapter == nil {
		return
	}
	vendor := NormalizeVendor(adapter.Vendor())
	if vendor == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.adapters == nil {
		r.adapters = make(map[string]PullAdapter)
	}
	r.adapters[vendor] = adapter
}

func (r *PullRegistry) Get(vendor string) (PullAdapter, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[NormalizeVendor(vendor)]
	return adapter, ok
}

func (r *PullRegistry) Pull(ctx context.Context, input PullInput) (PullResult, error) {
	adapter, ok := r.Get(input.Connector.Vendor)
	if !ok {
		return PullResult{}, ErrPullAdapterNotFound
	}
	return adapter.Pull(ctx, input)
}

func NormalizePullResult(result PullResult) PullResult {
	result.TenantID = strings.TrimSpace(result.TenantID)
	result.Source = strings.TrimSpace(result.Source)
	result.Actor = strings.TrimSpace(result.Actor)
	result.RequestID = strings.TrimSpace(result.RequestID)
	result.Mode = NormalizePullMode(result.Mode)
	result.ConnectorID = strings.TrimSpace(result.ConnectorID)
	if result.Source == "" {
		result.Source = "hris"
	}
	if result.Actor == "" {
		result.Actor = SyncActor
	}
	if result.Mode == "" {
		result.Mode = PullModeFull
	}
	if result.PulledAt.IsZero() {
		result.PulledAt = time.Now().UTC()
	}
	return result
}

func NormalizePullMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", PullModeFull:
		return PullModeFull
	case PullModeIncremental:
		return PullModeIncremental
	default:
		return PullModeFull
	}
}

func SupportsIncrementalPull(adapter PullAdapter, input PullInput) bool {
	supporter, ok := adapter.(IncrementalPullSupporter)
	if !ok {
		return false
	}
	return supporter.SupportsIncremental(input)
}

func NewPullStateService() *PullStateService {
	return &PullStateService{
		states: []ConnectorPullState{},
	}
}

func NewPullStateServiceWithStateStore(store StateStore) (*PullStateService, error) {
	svc := NewPullStateService()
	svc.stateStore = store
	if err := svc.restoreFromStateStore(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *PullStateService) ListStates(tenantID string) []ConnectorPullState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]ConnectorPullState, 0, len(s.states))
	for i := range s.states {
		if filterTenantID != "" && strings.TrimSpace(s.states[i].TenantID) != filterTenantID {
			continue
		}
		items = append(items, cloneConnectorPullState(s.states[i]))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ConnectorID > items[j].ConnectorID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items
}

func (s *PullStateService) GetState(connectorID string) (ConnectorPullState, error) {
	nextConnectorID := strings.TrimSpace(connectorID)
	if nextConnectorID == "" {
		return ConnectorPullState{}, ErrPullConnectorIDRequired
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.states {
		if strings.TrimSpace(s.states[i].ConnectorID) != nextConnectorID {
			continue
		}
		return cloneConnectorPullState(s.states[i]), nil
	}
	return ConnectorPullState{}, ErrPullStateNotFound
}

func (s *PullStateService) MarkStarted(tenantID, connectorID, vendor, mode string, startedAt time.Time) (ConnectorPullState, error) {
	return s.upsertState(tenantID, connectorID, vendor, func(item *ConnectorPullState, now time.Time) {
		item.Status = "running"
		item.LastMode = NormalizePullMode(mode)
		item.LastStartedAt = cloneTimePointer(&startedAt)
		if item.LastStartedAt == nil {
			item.LastStartedAt = &now
		}
		item.UpdatedAt = now
	})
}

func (s *PullStateService) ClaimStateForPull(
	tenantID, connectorID, vendor, mode string,
	maxAttempts int,
	retryCooldown time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) (ConnectorPullState, string, error) {
	return s.ClaimStateForPullWithBackoff(
		tenantID,
		connectorID,
		vendor,
		mode,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		processingTimeout,
		now,
	)
}

func (s *PullStateService) ClaimStateForPullWithBackoff(
	tenantID, connectorID, vendor, mode string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) (ConnectorPullState, string, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return ConnectorPullState{}, "", ErrTenantIDRequired
	}
	nextConnectorID := strings.TrimSpace(connectorID)
	if nextConnectorID == "" {
		return ConnectorPullState{}, "", ErrPullConnectorIDRequired
	}
	nextVendor := NormalizeVendor(vendor)
	nextMode := NormalizePullMode(mode)
	if nextMode == "" {
		nextMode = PullModeFull
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryCooldown, retryMaxBackoff = retrybackoff.Normalize(retryCooldown, retryMaxBackoff)
	if processingTimeout <= 0 {
		processingTimeout = 30 * time.Minute
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	claimed := ConnectorPullState{}
	skipReason := ""
	if err := s.mutateStateLocked(func() (bool, error) {
		for i := range s.states {
			if strings.TrimSpace(s.states[i].ConnectorID) != nextConnectorID {
				continue
			}
			currentTenantID := strings.TrimSpace(s.states[i].TenantID)
			if currentTenantID != "" && currentTenantID != nextTenantID {
				return false, ErrPullStateNotFound
			}

			status := strings.TrimSpace(s.states[i].Status)
			switch status {
			case "", "idle", "succeeded":
			case "failed":
				if maxAttempts > 0 && s.states[i].ConsecutiveFailures >= maxAttempts {
					claimed = cloneConnectorPullState(s.states[i])
					skipReason = PullStateClaimReasonAttemptLimit
					return false, nil
				}
				if retryDelay := retrybackoff.Exponential(
					s.states[i].ConsecutiveFailures,
					retryCooldown,
					retryMaxBackoff,
				); retryDelay > 0 && s.states[i].LastFailureAt != nil {
					if s.states[i].LastFailureAt.Add(retryDelay).After(now) {
						claimed = cloneConnectorPullState(s.states[i])
						skipReason = PullStateClaimReasonCooldown
						return false, nil
					}
				}
			case "running":
				if s.states[i].LastStartedAt != nil && s.states[i].LastStartedAt.Add(processingTimeout).After(now) {
					claimed = cloneConnectorPullState(s.states[i])
					skipReason = PullStateClaimReasonInFlight
					return false, nil
				}
			default:
				claimed = cloneConnectorPullState(s.states[i])
				skipReason = PullStateClaimReasonNotQueueable
				return false, nil
			}

			if nextVendor != "" {
				s.states[i].Vendor = nextVendor
			}
			s.states[i].TenantID = nextTenantID
			s.states[i].Status = "running"
			s.states[i].LastMode = nextMode
			s.states[i].LastStartedAt = &now
			s.states[i].UpdatedAt = now
			claimed = cloneConnectorPullState(s.states[i])
			skipReason = ""
			return true, nil
		}

		item := ConnectorPullState{
			TenantID:      nextTenantID,
			ConnectorID:   nextConnectorID,
			Vendor:        nextVendor,
			Status:        "running",
			LastMode:      nextMode,
			LastStartedAt: &now,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		s.states = append([]ConnectorPullState{item}, s.states...)
		claimed = cloneConnectorPullState(item)
		skipReason = ""
		return true, nil
	}); err != nil {
		return ConnectorPullState{}, "", err
	}
	if skipReason != "" {
		return claimed, skipReason, nil
	}
	return claimed, "", nil
}

func (s *PullStateService) MarkSucceeded(
	tenantID, connectorID, vendor string,
	mode string,
	requestID string,
	succeededAt time.Time,
) (ConnectorPullState, error) {
	nextRequestID := strings.TrimSpace(requestID)
	return s.upsertState(tenantID, connectorID, vendor, func(item *ConnectorPullState, now time.Time) {
		completedAt := succeededAt
		nextMode := NormalizePullMode(mode)
		if completedAt.IsZero() {
			completedAt = now
		}
		item.Status = "succeeded"
		item.LastRequestID = nextRequestID
		item.LastMode = nextMode
		item.LastSuccessAt = &completedAt
		if nextMode == PullModeFull {
			item.LastFullSuccessAt = &completedAt
		}
		item.LastFailureAt = nil
		item.LastError = ""
		item.ConsecutiveFailures = 0
		item.UpdatedAt = completedAt
	})
}

func (s *PullStateService) MarkFailed(
	tenantID, connectorID, vendor string,
	failedAt time.Time,
	failure error,
) (ConnectorPullState, error) {
	if failure == nil {
		return ConnectorPullState{}, ErrPullFailureRequired
	}
	return s.upsertState(tenantID, connectorID, vendor, func(item *ConnectorPullState, now time.Time) {
		completedAt := failedAt
		if completedAt.IsZero() {
			completedAt = now
		}
		item.Status = "failed"
		item.LastFailureAt = &completedAt
		item.LastError = strings.TrimSpace(failure.Error())
		item.ConsecutiveFailures++
		item.UpdatedAt = completedAt
	})
}

func (s *PullStateService) upsertState(
	tenantID, connectorID, vendor string,
	mutate func(item *ConnectorPullState, now time.Time),
) (ConnectorPullState, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return ConnectorPullState{}, ErrTenantIDRequired
	}
	nextConnectorID := strings.TrimSpace(connectorID)
	if nextConnectorID == "" {
		return ConnectorPullState{}, ErrPullConnectorIDRequired
	}
	nextVendor := NormalizeVendor(vendor)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	updated := ConnectorPullState{}
	if err := s.mutateStateLocked(func() (bool, error) {
		for i := range s.states {
			if strings.TrimSpace(s.states[i].ConnectorID) != nextConnectorID {
				continue
			}
			if nextVendor != "" {
				s.states[i].Vendor = nextVendor
			}
			if nextTenantID != "" {
				s.states[i].TenantID = nextTenantID
			}
			mutate(&s.states[i], now)
			updated = cloneConnectorPullState(s.states[i])
			return true, nil
		}

		item := ConnectorPullState{
			TenantID:    nextTenantID,
			ConnectorID: nextConnectorID,
			Vendor:      nextVendor,
			Status:      "idle",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		mutate(&item, now)
		s.states = append([]ConnectorPullState{item}, s.states...)
		updated = cloneConnectorPullState(item)
		return true, nil
	}); err != nil {
		return ConnectorPullState{}, err
	}
	return updated, nil
}

func (s *PullStateService) restoreFromStateStore() error {
	if s.stateStore == nil {
		return nil
	}
	var snapshot pullStateSnapshot
	ok, err := s.stateStore.Load(pullStateKey, &snapshot)
	if err != nil || !ok {
		return err
	}
	snapshot = normalizePullStateSnapshot(snapshot)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.states = cloneConnectorPullStates(snapshot.States)
	return nil
}

func (s *PullStateService) RefreshState() error {
	if s == nil || s.stateStore == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshStateLocked()
}

func (s *PullStateService) loadStateLocked() (pullStateSnapshot, bool, error) {
	if s.stateStore == nil {
		return pullStateSnapshot{}, false, nil
	}

	var snapshot pullStateSnapshot
	found, err := s.stateStore.Load(pullStateKey, &snapshot)
	if err != nil {
		return pullStateSnapshot{}, false, err
	}
	if !found {
		return pullStateSnapshot{}, false, nil
	}
	return normalizePullStateSnapshot(snapshot), true, nil
}

func (s *PullStateService) refreshStateLocked() error {
	snapshot, found, err := s.loadStateLocked()
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	s.states = cloneConnectorPullStates(snapshot.States)
	return nil
}

func (s *PullStateService) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(pullStateKey, pullStateSnapshot{
		States: cloneConnectorPullStates(s.states),
	})
}

func (s *PullStateService) mutateStateLocked(mutator func() (bool, error)) error {
	if s.stateStore == nil {
		_, err := mutator()
		return err
	}

	casStore, hasCAS := s.stateStore.(compareAndSwapStateStore)
	if !hasCAS {
		if err := s.refreshStateLocked(); err != nil {
			return err
		}
		changed, err := mutator()
		if err != nil || !changed {
			return err
		}
		return s.persistLocked()
	}

	baseSnapshot := pullStateSnapshot{
		States: cloneConnectorPullStates(s.states),
	}
	for attempt := 0; attempt < maxPullStateCASRetries; attempt++ {
		snapshot, found, err := s.loadStateLocked()
		if err != nil {
			return err
		}
		if found {
			s.states = cloneConnectorPullStates(snapshot.States)
		} else {
			s.states = cloneConnectorPullStates(baseSnapshot.States)
		}

		changed, err := mutator()
		if err != nil {
			if found {
				s.states = cloneConnectorPullStates(snapshot.States)
			} else {
				s.states = cloneConnectorPullStates(baseSnapshot.States)
			}
			return err
		}
		if !changed {
			if found {
				s.states = cloneConnectorPullStates(snapshot.States)
			} else {
				s.states = cloneConnectorPullStates(baseSnapshot.States)
			}
			return nil
		}

		persisted, err := casStore.CompareAndSwap(
			pullStateKey,
			found,
			snapshot,
			pullStateSnapshot{
				States: cloneConnectorPullStates(s.states),
			},
		)
		if err != nil {
			if found {
				s.states = cloneConnectorPullStates(snapshot.States)
			} else {
				s.states = cloneConnectorPullStates(baseSnapshot.States)
			}
			return err
		}
		if persisted {
			return nil
		}
	}
	if snapshot, found, err := s.loadStateLocked(); err == nil {
		if found {
			s.states = cloneConnectorPullStates(snapshot.States)
		} else {
			s.states = cloneConnectorPullStates(baseSnapshot.States)
		}
	} else {
		s.states = cloneConnectorPullStates(baseSnapshot.States)
	}
	return ErrPullStateConflict
}

func normalizePullStateSnapshot(snapshot pullStateSnapshot) pullStateSnapshot {
	snapshot.States = normalizePullStateSnapshotStates(snapshot.States)
	return snapshot
}

func normalizePullStateSnapshotStates(items []ConnectorPullState) []ConnectorPullState {
	if len(items) == 0 {
		return nil
	}
	output := make([]ConnectorPullState, 0, len(items))
	for i := range items {
		item := cloneConnectorPullState(items[i])
		if strings.TrimSpace(item.TenantID) == "" || strings.TrimSpace(item.ConnectorID) == "" {
			continue
		}
		output = append(output, item)
	}
	return output
}

func cloneConnectorPullState(input ConnectorPullState) ConnectorPullState {
	output := input
	output.LastStartedAt = cloneTimePointer(input.LastStartedAt)
	output.LastSuccessAt = cloneTimePointer(input.LastSuccessAt)
	output.LastFullSuccessAt = cloneTimePointer(input.LastFullSuccessAt)
	output.LastFailureAt = cloneTimePointer(input.LastFailureAt)
	return output
}

func cloneConnectorPullStates(items []ConnectorPullState) []ConnectorPullState {
	if len(items) == 0 {
		return nil
	}
	output := make([]ConnectorPullState, 0, len(items))
	for i := range items {
		output = append(output, cloneConnectorPullState(items[i]))
	}
	return output
}

func cloneTimePointer(input *time.Time) *time.Time {
	if input == nil {
		return nil
	}
	value := *input
	return &value
}
