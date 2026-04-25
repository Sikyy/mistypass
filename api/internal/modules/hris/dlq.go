package hris

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mistypass/cloud/api/internal/retrybackoff"
)

var ErrDLQTenantIDRequired = errors.New("hris dlq tenant_id is required")
var ErrDLQFailureStageRequired = errors.New("hris dlq failure_stage is required")
var ErrDLQErrorRequired = errors.New("hris dlq error is required")
var ErrDLQEntryNotFound = errors.New("hris dlq entry not found")
var ErrDLQEntryReplayInFlight = errors.New("hris dlq entry replay is already in progress")
var ErrDLQEntryReplayNotAllowed = errors.New("hris dlq entry cannot be replayed")
var ErrDLQStateConflict = errors.New("hris dlq state conflict")

const dlqStateKey = "module_hris_dlq"

const maxDLQStateCASRetries = 5

const (
	DLQEntryClaimReasonAttemptLimit  = "attempt_limit"
	DLQEntryClaimReasonCooldown      = "cooldown"
	DLQEntryClaimReasonInFlight      = "in_flight"
	DLQEntryClaimReasonNotReplayable = "not_replayable"
)

type DeadLetterEntry struct {
	ID            string     `json:"id"`
	TenantID      string     `json:"tenant_id"`
	ConnectorID   string     `json:"connector_id,omitempty"`
	Vendor        string     `json:"vendor,omitempty"`
	ReceiptID     string     `json:"receipt_id,omitempty"`
	RequestID     string     `json:"request_id,omitempty"`
	EventType     string     `json:"event_type,omitempty"`
	FailureStage  string     `json:"failure_stage"`
	Error         string     `json:"error"`
	RawPayloadRef string     `json:"raw_payload_ref,omitempty"`
	Status        string     `json:"status"`
	ReplayCount   int        `json:"replay_count,omitempty"`
	LastReplayAt  *time.Time `json:"last_replay_at,omitempty"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type DeadLetterFailureInput struct {
	TenantID      string
	ConnectorID   string
	Vendor        string
	ReceiptID     string
	RequestID     string
	EventType     string
	FailureStage  string
	Error         string
	RawPayloadRef string
}

type deadLetterDueIndexEntry struct {
	EntryID string    `json:"entry_id"`
	DueAt   time.Time `json:"due_at"`
}

type deadLetterSnapshot struct {
	Entries     []DeadLetterEntry         `json:"entries,omitempty"`
	DueEntryIDs []deadLetterDueIndexEntry `json:"due_entry_ids,omitempty"`
}

type compareAndSwapStateStore interface {
	CompareAndSwap(key string, expectedExists bool, expected any, next any) (bool, error)
}

type DLQService struct {
	mu          sync.RWMutex
	entries     []DeadLetterEntry
	dueEntryIDs []deadLetterDueIndexEntry
	stateStore  StateStore
}

func NewDLQService() *DLQService {
	return &DLQService{
		entries:     []DeadLetterEntry{},
		dueEntryIDs: []deadLetterDueIndexEntry{},
	}
}

func NewDLQServiceWithStateStore(store StateStore) (*DLQService, error) {
	svc := NewDLQService()
	svc.stateStore = store
	if err := svc.restoreFromStateStore(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *DLQService) AppendFailure(input DeadLetterFailureInput) (DeadLetterEntry, error) {
	nextTenantID := strings.TrimSpace(input.TenantID)
	if nextTenantID == "" {
		return DeadLetterEntry{}, ErrDLQTenantIDRequired
	}
	nextFailureStage := strings.TrimSpace(input.FailureStage)
	if nextFailureStage == "" {
		return DeadLetterEntry{}, ErrDLQFailureStageRequired
	}
	nextError := strings.TrimSpace(input.Error)
	if nextError == "" {
		return DeadLetterEntry{}, ErrDLQErrorRequired
	}

	now := time.Now().UTC()
	nextReceiptID := strings.TrimSpace(input.ReceiptID)

	s.mu.Lock()
	defer s.mu.Unlock()

	entryID, err := randomDLQID()
	if err != nil {
		return DeadLetterEntry{}, err
	}
	entry := DeadLetterEntry{
		ID:            entryID,
		TenantID:      nextTenantID,
		ConnectorID:   strings.TrimSpace(input.ConnectorID),
		Vendor:        strings.TrimSpace(input.Vendor),
		ReceiptID:     nextReceiptID,
		RequestID:     strings.TrimSpace(input.RequestID),
		EventType:     strings.TrimSpace(input.EventType),
		FailureStage:  nextFailureStage,
		Error:         nextError,
		RawPayloadRef: strings.TrimSpace(input.RawPayloadRef),
		Status:        "dlq",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.mutateStateLocked(func() (bool, error) {
		if nextReceiptID != "" {
			for i := range s.entries {
				if strings.TrimSpace(s.entries[i].ReceiptID) != nextReceiptID {
					continue
				}
				if strings.TrimSpace(s.entries[i].FailureStage) != nextFailureStage {
					continue
				}
				if strings.TrimSpace(s.entries[i].Status) != "dlq" {
					continue
				}
				s.entries[i].ConnectorID = strings.TrimSpace(input.ConnectorID)
				s.entries[i].Vendor = strings.TrimSpace(input.Vendor)
				s.entries[i].RequestID = strings.TrimSpace(input.RequestID)
				s.entries[i].EventType = strings.TrimSpace(input.EventType)
				s.entries[i].Error = nextError
				s.entries[i].RawPayloadRef = strings.TrimSpace(input.RawPayloadRef)
				s.entries[i].UpdatedAt = now
				entry = cloneDeadLetterEntry(s.entries[i])
				return true, nil
			}
		}

		s.entries = append([]DeadLetterEntry{entry}, s.entries...)
		s.upsertDLQDueIndexLocked(entry.ID, entry.CreatedAt)
		return true, nil
	}); err != nil {
		return DeadLetterEntry{}, err
	}
	return cloneDeadLetterEntry(entry), nil
}

func (s *DLQService) ListEntries(tenantID, connectorID string, limit int) []DeadLetterEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	filterConnectorID := strings.TrimSpace(connectorID)
	items := make([]DeadLetterEntry, 0, len(s.entries))
	for i := range s.entries {
		item := s.entries[i]
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		if filterConnectorID != "" && strings.TrimSpace(item.ConnectorID) != filterConnectorID {
			continue
		}
		items = append(items, cloneDeadLetterEntry(item))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *DLQService) ListClaimableEntriesForReplayWithBackoff(
	tenantID, connectorID string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []DeadLetterEntry {
	maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now = normalizeDLQReplayClaimParams(
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
	)

	s.mu.RLock()
	defer s.mu.RUnlock()

	return listClaimableDLQEntriesForReplayWithBackoffLocked(
		s.entries,
		tenantID,
		connectorID,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
		limit,
	)
}

func (s *DLQService) ListDueEntriesForReplayWithBackoff(
	tenantID, connectorID string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []DeadLetterEntry {
	maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now = normalizeDLQReplayClaimParams(
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
	)

	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	filterConnectorID := strings.TrimSpace(connectorID)
	items := make([]DeadLetterEntry, 0, limit)
	seen := make(map[string]struct{}, len(s.dueEntryIDs))
	for i := range s.dueEntryIDs {
		entry := s.dueEntryIDs[i]
		if !entry.DueAt.IsZero() && entry.DueAt.After(now) {
			break
		}

		item, ok := findDLQEntryByIDLocked(s.entries, entry.EntryID)
		if !ok {
			continue
		}
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		if filterConnectorID != "" && strings.TrimSpace(item.ConnectorID) != filterConnectorID {
			continue
		}
		if dlqReplayClaimReason(item, maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now) != "" {
			continue
		}
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		items = append(items, cloneDeadLetterEntry(item))
		if limit > 0 && len(items) >= limit {
			return items
		}
	}

	fallbackItems := listClaimableDLQEntriesForReplayWithBackoffLocked(
		s.entries,
		tenantID,
		connectorID,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
		0,
	)
	for i := range fallbackItems {
		if _, exists := seen[fallbackItems[i].ID]; exists {
			continue
		}
		seen[fallbackItems[i].ID] = struct{}{}
		items = append(items, fallbackItems[i])
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items
}

func listClaimableDLQEntriesForReplayWithBackoffLocked(
	allEntries []DeadLetterEntry,
	tenantID, connectorID string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []DeadLetterEntry {
	filterTenantID := strings.TrimSpace(tenantID)
	filterConnectorID := strings.TrimSpace(connectorID)
	items := make([]DeadLetterEntry, 0, len(allEntries))
	for i := range allEntries {
		item := allEntries[i]
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		if filterConnectorID != "" && strings.TrimSpace(item.ConnectorID) != filterConnectorID {
			continue
		}
		if dlqReplayClaimReason(item, maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now) != "" {
			continue
		}
		items = append(items, cloneDeadLetterEntry(item))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *DLQService) GetEntry(entryID string) (DeadLetterEntry, error) {
	nextEntryID := strings.TrimSpace(entryID)
	if nextEntryID == "" {
		return DeadLetterEntry{}, ErrDLQEntryNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.entries {
		if strings.TrimSpace(s.entries[i].ID) != nextEntryID {
			continue
		}
		return cloneDeadLetterEntry(s.entries[i]), nil
	}
	return DeadLetterEntry{}, ErrDLQEntryNotFound
}

func findDLQEntryByIDLocked(items []DeadLetterEntry, entryID string) (DeadLetterEntry, bool) {
	nextEntryID := strings.TrimSpace(entryID)
	if nextEntryID == "" {
		return DeadLetterEntry{}, false
	}
	for i := range items {
		if strings.TrimSpace(items[i].ID) != nextEntryID {
			continue
		}
		return items[i], true
	}
	return DeadLetterEntry{}, false
}

func (s *DLQService) ClaimEntryForReplay(
	entryID string,
	maxAttempts int,
	retryCooldown time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) (DeadLetterEntry, string, error) {
	return s.ClaimEntryForReplayWithBackoff(
		entryID,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		processingTimeout,
		now,
	)
}

func (s *DLQService) ClaimEntryForReplayWithBackoff(
	entryID string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) (DeadLetterEntry, string, error) {
	nextEntryID := strings.TrimSpace(entryID)
	if nextEntryID == "" {
		return DeadLetterEntry{}, "", ErrDLQEntryNotFound
	}
	maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now = normalizeDLQReplayClaimParams(
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
	)

	s.mu.Lock()
	defer s.mu.Unlock()

	var claimed DeadLetterEntry
	claimReason := ""
	if err := s.mutateStateLocked(func() (bool, error) {
		for i := range s.entries {
			if strings.TrimSpace(s.entries[i].ID) != nextEntryID {
				continue
			}

			if reason := dlqReplayClaimReason(
				s.entries[i],
				maxAttempts,
				retryCooldown,
				retryMaxBackoff,
				processingTimeout,
				now,
			); reason != "" {
				claimed = cloneDeadLetterEntry(s.entries[i])
				claimReason = reason
				return false, nil
			}

			s.entries[i].ReplayCount++
			s.entries[i].Status = "replaying"
			s.entries[i].ResolvedAt = nil
			s.entries[i].UpdatedAt = now
			s.entries[i].LastReplayAt = &now
			s.upsertDLQDueIndexLocked(
				s.entries[i].ID,
				dlqReplayProcessingDueAt(now, processingTimeout),
			)
			claimed = cloneDeadLetterEntry(s.entries[i])
			claimReason = ""
			return true, nil
		}
		return false, ErrDLQEntryNotFound
	}); err != nil {
		return DeadLetterEntry{}, "", err
	}
	if claimReason != "" {
		return claimed, claimReason, nil
	}
	if claimed.ID == "" {
		return DeadLetterEntry{}, "", ErrDLQEntryNotFound
	}
	return claimed, "", nil
}

func normalizeDLQReplayClaimParams(
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) (int, time.Duration, time.Duration, time.Duration, time.Time) {
	if maxAttempts < 0 {
		maxAttempts = 0
	}
	retryCooldown, retryMaxBackoff = retrybackoff.Normalize(retryCooldown, retryMaxBackoff)
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	return maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now
}

func dlqReplayClaimReason(
	item DeadLetterEntry,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) string {
	status := strings.TrimSpace(item.Status)
	switch status {
	case "dlq":
		if maxAttempts > 0 && item.ReplayCount >= maxAttempts {
			return DLQEntryClaimReasonAttemptLimit
		}
		if retryDelay := retrybackoff.Exponential(
			item.ReplayCount,
			retryCooldown,
			retryMaxBackoff,
		); retryDelay > 0 && item.LastReplayAt != nil {
			if item.LastReplayAt.Add(retryDelay).After(now) {
				return DLQEntryClaimReasonCooldown
			}
		}
		return ""
	case "replaying":
		if item.LastReplayAt != nil && item.LastReplayAt.Add(processingTimeout).After(now) {
			return DLQEntryClaimReasonInFlight
		}
		return ""
	default:
		return DLQEntryClaimReasonNotReplayable
	}
}

func isDLQDueIndexCandidate(status string) bool {
	switch strings.TrimSpace(status) {
	case "dlq", "replaying":
		return true
	default:
		return false
	}
}

func dlqReplayDueIndexHeuristic(item DeadLetterEntry) time.Time {
	if item.LastReplayAt != nil && !item.LastReplayAt.IsZero() {
		return item.LastReplayAt.UTC()
	}
	return item.CreatedAt.UTC()
}

func dlqReplayProcessingDueAt(now time.Time, processingTimeout time.Duration) time.Time {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if processingTimeout <= 0 {
		return now
	}
	return now.Add(processingTimeout)
}

func dlqReplayFailureDueAt(
	item DeadLetterEntry,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	now time.Time,
) time.Time {
	base := now
	if item.LastReplayAt != nil && !item.LastReplayAt.IsZero() {
		base = item.LastReplayAt.UTC()
	} else if now.IsZero() {
		base = time.Now().UTC()
	} else {
		base = now.UTC()
	}
	return base.Add(retrybackoff.Exponential(item.ReplayCount, retryCooldown, retryMaxBackoff))
}

func sortDLQDueIndexEntries(items []deadLetterDueIndexEntry) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].DueAt.Equal(items[j].DueAt) {
			return items[i].EntryID < items[j].EntryID
		}
		return items[i].DueAt.Before(items[j].DueAt)
	})
}

func (s *DLQService) upsertDLQDueIndexLocked(entryID string, dueAt time.Time) {
	nextEntryID := strings.TrimSpace(entryID)
	if nextEntryID == "" {
		return
	}
	nextDueAt := dueAt
	if nextDueAt.IsZero() {
		nextDueAt = time.Now().UTC()
	} else {
		nextDueAt = nextDueAt.UTC()
	}
	for i := range s.dueEntryIDs {
		if strings.TrimSpace(s.dueEntryIDs[i].EntryID) != nextEntryID {
			continue
		}
		s.dueEntryIDs[i].DueAt = nextDueAt
		sortDLQDueIndexEntries(s.dueEntryIDs)
		return
	}
	s.dueEntryIDs = append(s.dueEntryIDs, deadLetterDueIndexEntry{
		EntryID: nextEntryID,
		DueAt:   nextDueAt,
	})
	sortDLQDueIndexEntries(s.dueEntryIDs)
}

func (s *DLQService) removeDLQDueIndexLocked(entryID string) {
	nextEntryID := strings.TrimSpace(entryID)
	if nextEntryID == "" || len(s.dueEntryIDs) == 0 {
		return
	}
	filtered := s.dueEntryIDs[:0]
	for i := range s.dueEntryIDs {
		if strings.TrimSpace(s.dueEntryIDs[i].EntryID) == nextEntryID {
			continue
		}
		filtered = append(filtered, s.dueEntryIDs[i])
	}
	s.dueEntryIDs = filtered
}

func (s *DLQService) normalizeDLQDueIndexLocked() {
	if len(s.entries) == 0 {
		s.dueEntryIDs = nil
		return
	}

	existing := make(map[string]deadLetterDueIndexEntry, len(s.dueEntryIDs))
	for i := range s.dueEntryIDs {
		entry := s.dueEntryIDs[i]
		nextEntryID := strings.TrimSpace(entry.EntryID)
		if nextEntryID == "" || entry.DueAt.IsZero() {
			continue
		}
		entry.EntryID = nextEntryID
		entry.DueAt = entry.DueAt.UTC()
		existing[nextEntryID] = entry
	}

	normalized := make([]deadLetterDueIndexEntry, 0, len(s.entries))
	for i := range s.entries {
		item := s.entries[i]
		if !isDLQDueIndexCandidate(item.Status) {
			continue
		}
		nextEntryID := strings.TrimSpace(item.ID)
		if nextEntryID == "" {
			continue
		}
		entry, ok := existing[nextEntryID]
		if !ok {
			entry = deadLetterDueIndexEntry{
				EntryID: nextEntryID,
				DueAt:   dlqReplayDueIndexHeuristic(item),
			}
		}
		normalized = append(normalized, entry)
	}
	sortDLQDueIndexEntries(normalized)
	s.dueEntryIDs = normalized
}

func buildDLQDueIndexEntries(items []DeadLetterEntry) []deadLetterDueIndexEntry {
	result := make([]deadLetterDueIndexEntry, 0, len(items))
	for i := range items {
		if !isDLQDueIndexCandidate(items[i].Status) {
			continue
		}
		nextEntryID := strings.TrimSpace(items[i].ID)
		if nextEntryID == "" {
			continue
		}
		result = append(result, deadLetterDueIndexEntry{
			EntryID: nextEntryID,
			DueAt:   dlqReplayDueIndexHeuristic(items[i]),
		})
	}
	sortDLQDueIndexEntries(result)
	return result
}

func (s *DLQService) MarkReplayFailed(entryID string, failure error) (DeadLetterEntry, error) {
	return s.MarkReplayFailedWithBackoff(entryID, failure, 0, 0)
}

func (s *DLQService) MarkReplayFailedWithBackoff(
	entryID string,
	failure error,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
) (DeadLetterEntry, error) {
	nextEntryID := strings.TrimSpace(entryID)
	if nextEntryID == "" {
		return DeadLetterEntry{}, ErrDLQEntryNotFound
	}
	if failure == nil {
		return DeadLetterEntry{}, ErrDLQErrorRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var updated DeadLetterEntry
	if err := s.mutateStateLocked(func() (bool, error) {
		for i := range s.entries {
			if strings.TrimSpace(s.entries[i].ID) != nextEntryID {
				continue
			}
			now := time.Now().UTC()
			if strings.TrimSpace(s.entries[i].Status) != "replaying" || s.entries[i].LastReplayAt == nil {
				s.entries[i].ReplayCount++
				s.entries[i].LastReplayAt = &now
			}
			s.entries[i].UpdatedAt = now
			s.entries[i].ResolvedAt = nil
			s.entries[i].Status = "dlq"
			s.entries[i].Error = strings.TrimSpace(failure.Error())
			s.upsertDLQDueIndexLocked(
				s.entries[i].ID,
				dlqReplayFailureDueAt(s.entries[i], retryCooldown, retryMaxBackoff, now),
			)
			updated = cloneDeadLetterEntry(s.entries[i])
			return true, nil
		}
		return false, ErrDLQEntryNotFound
	}); err != nil {
		return DeadLetterEntry{}, err
	}
	return updated, nil
}

func (s *DLQService) MarkResolved(entryID string) (DeadLetterEntry, error) {
	nextEntryID := strings.TrimSpace(entryID)
	if nextEntryID == "" {
		return DeadLetterEntry{}, ErrDLQEntryNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var resolved DeadLetterEntry
	if err := s.mutateStateLocked(func() (bool, error) {
		for i := range s.entries {
			if strings.TrimSpace(s.entries[i].ID) != nextEntryID {
				continue
			}
			now := time.Now().UTC()
			if strings.TrimSpace(s.entries[i].Status) != "replaying" || s.entries[i].LastReplayAt == nil {
				s.entries[i].ReplayCount++
				s.entries[i].LastReplayAt = &now
			}
			s.entries[i].ResolvedAt = &now
			s.entries[i].UpdatedAt = now
			s.entries[i].Status = "resolved"
			s.removeDLQDueIndexLocked(s.entries[i].ID)
			resolved = cloneDeadLetterEntry(s.entries[i])
			return true, nil
		}
		return false, ErrDLQEntryNotFound
	}); err != nil {
		return DeadLetterEntry{}, err
	}
	return resolved, nil
}

func (s *DLQService) RestoreEntry(snapshot DeadLetterEntry) (DeadLetterEntry, error) {
	nextEntryID := strings.TrimSpace(snapshot.ID)
	if nextEntryID == "" {
		return DeadLetterEntry{}, ErrDLQEntryNotFound
	}
	restoredSnapshot := cloneDeadLetterEntry(snapshot)

	s.mu.Lock()
	defer s.mu.Unlock()

	var restored DeadLetterEntry
	if err := s.mutateStateLocked(func() (bool, error) {
		for i := range s.entries {
			if strings.TrimSpace(s.entries[i].ID) != nextEntryID {
				continue
			}
			s.entries[i] = cloneDeadLetterEntry(restoredSnapshot)
			restored = cloneDeadLetterEntry(s.entries[i])
			return true, nil
		}
		return false, ErrDLQEntryNotFound
	}); err != nil {
		return DeadLetterEntry{}, err
	}
	return restored, nil
}

func (s *DLQService) restoreFromStateStore() error {
	if s.stateStore == nil {
		return nil
	}
	var snapshot deadLetterSnapshot
	ok, err := s.stateStore.Load(dlqStateKey, &snapshot)
	if err != nil || !ok {
		return err
	}
	s.entries = cloneDeadLetterEntries(snapshot.Entries)
	s.dueEntryIDs = cloneDLQDueIndexEntries(snapshot.DueEntryIDs)
	s.normalizeDLQDueIndexLocked()
	return nil
}

func (s *DLQService) RefreshState() error {
	if s == nil || s.stateStore == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshStateLocked()
}

func (s *DLQService) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(dlqStateKey, deadLetterSnapshot{
		Entries:     cloneDeadLetterEntries(s.entries),
		DueEntryIDs: cloneDLQDueIndexEntries(s.dueEntryIDs),
	})
}

func (s *DLQService) loadStateLocked() (deadLetterSnapshot, bool, error) {
	if s.stateStore == nil {
		return deadLetterSnapshot{}, false, nil
	}

	var snapshot deadLetterSnapshot
	found, err := s.stateStore.Load(dlqStateKey, &snapshot)
	if err != nil {
		return deadLetterSnapshot{}, false, err
	}
	if !found {
		return deadLetterSnapshot{}, false, nil
	}
	return snapshot, true, nil
}

func (s *DLQService) refreshStateLocked() error {
	snapshot, found, err := s.loadStateLocked()
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	s.entries = cloneDeadLetterEntries(snapshot.Entries)
	s.dueEntryIDs = cloneDLQDueIndexEntries(snapshot.DueEntryIDs)
	s.normalizeDLQDueIndexLocked()
	return nil
}

func (s *DLQService) mutateStateLocked(mutator func() (bool, error)) error {
	if s.stateStore == nil {
		changed, err := mutator()
		if err != nil {
			return err
		}
		if changed {
			s.normalizeDLQDueIndexLocked()
		}
		return nil
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
		s.normalizeDLQDueIndexLocked()
		return s.persistLocked()
	}

	baseSnapshot := deadLetterSnapshot{
		Entries:     cloneDeadLetterEntries(s.entries),
		DueEntryIDs: cloneDLQDueIndexEntries(s.dueEntryIDs),
	}
	for attempt := 0; attempt < maxDLQStateCASRetries; attempt++ {
		snapshot, found, err := s.loadStateLocked()
		if err != nil {
			return err
		}
		if found {
			s.entries = cloneDeadLetterEntries(snapshot.Entries)
			s.dueEntryIDs = cloneDLQDueIndexEntries(snapshot.DueEntryIDs)
		} else {
			s.entries = cloneDeadLetterEntries(baseSnapshot.Entries)
			s.dueEntryIDs = cloneDLQDueIndexEntries(baseSnapshot.DueEntryIDs)
		}

		changed, err := mutator()
		if err != nil {
			if found {
				s.entries = cloneDeadLetterEntries(snapshot.Entries)
				s.dueEntryIDs = cloneDLQDueIndexEntries(snapshot.DueEntryIDs)
			} else {
				s.entries = cloneDeadLetterEntries(baseSnapshot.Entries)
				s.dueEntryIDs = cloneDLQDueIndexEntries(baseSnapshot.DueEntryIDs)
			}
			return err
		}
		if !changed {
			if found {
				s.entries = cloneDeadLetterEntries(snapshot.Entries)
				s.dueEntryIDs = cloneDLQDueIndexEntries(snapshot.DueEntryIDs)
			} else {
				s.entries = cloneDeadLetterEntries(baseSnapshot.Entries)
				s.dueEntryIDs = cloneDLQDueIndexEntries(baseSnapshot.DueEntryIDs)
			}
			return err
		}
		s.normalizeDLQDueIndexLocked()

		persisted, err := casStore.CompareAndSwap(
			dlqStateKey,
			found,
			snapshot,
			deadLetterSnapshot{
				Entries:     cloneDeadLetterEntries(s.entries),
				DueEntryIDs: cloneDLQDueIndexEntries(s.dueEntryIDs),
			},
		)
		if err != nil {
			if found {
				s.entries = cloneDeadLetterEntries(snapshot.Entries)
				s.dueEntryIDs = cloneDLQDueIndexEntries(snapshot.DueEntryIDs)
			} else {
				s.entries = cloneDeadLetterEntries(baseSnapshot.Entries)
				s.dueEntryIDs = cloneDLQDueIndexEntries(baseSnapshot.DueEntryIDs)
			}
			return err
		}
		if persisted {
			return nil
		}
	}
	if snapshot, found, err := s.loadStateLocked(); err == nil {
		if found {
			s.entries = cloneDeadLetterEntries(snapshot.Entries)
			s.dueEntryIDs = cloneDLQDueIndexEntries(snapshot.DueEntryIDs)
		} else {
			s.entries = cloneDeadLetterEntries(baseSnapshot.Entries)
			s.dueEntryIDs = cloneDLQDueIndexEntries(baseSnapshot.DueEntryIDs)
		}
	} else {
		s.entries = cloneDeadLetterEntries(baseSnapshot.Entries)
		s.dueEntryIDs = cloneDLQDueIndexEntries(baseSnapshot.DueEntryIDs)
	}
	return ErrDLQStateConflict
}

func cloneDeadLetterEntry(input DeadLetterEntry) DeadLetterEntry {
	output := input
	if input.LastReplayAt != nil {
		value := *input.LastReplayAt
		output.LastReplayAt = &value
	}
	if input.ResolvedAt != nil {
		value := *input.ResolvedAt
		output.ResolvedAt = &value
	}
	return output
}

func cloneDeadLetterEntries(items []DeadLetterEntry) []DeadLetterEntry {
	output := make([]DeadLetterEntry, 0, len(items))
	for i := range items {
		output = append(output, cloneDeadLetterEntry(items[i]))
	}
	return output
}

func cloneDLQDueIndexEntries(items []deadLetterDueIndexEntry) []deadLetterDueIndexEntry {
	output := make([]deadLetterDueIndexEntry, 0, len(items))
	for i := range items {
		item := items[i]
		item.EntryID = strings.TrimSpace(item.EntryID)
		if item.EntryID == "" {
			continue
		}
		item.DueAt = item.DueAt.UTC()
		output = append(output, item)
	}
	return output
}

func randomDLQID() (string, error) {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return "hdlq_" + hex.EncodeToString(data[:]), nil
}
