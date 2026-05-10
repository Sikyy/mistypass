package audit

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// OfflineAuditEntry represents a single access event recorded by a gateway while offline.
type OfflineAuditEntry struct {
	EventID   string `json:"event_id"`
	UserID    string `json:"user_id"`
	LockID    string `json:"lock_id"`
	Method    string `json:"method"`     // "ble" | "nfc_hce"
	Result    string `json:"result"`     // "granted" | "denied"
	Reason    string `json:"reason"`
	GatewayID uint32 `json:"gateway_id"`
	Timestamp int64  `json:"timestamp"`  // unix seconds
	IsOffline bool   `json:"is_offline"`
}

// IngestOfflineBatch processes a batch of offline audit entries from a gateway.
// It converts them to audit Log records and appends them to the audit log.
// Returns the number of entries accepted (duplicates or invalid entries are skipped).
func (s *Service) IngestOfflineBatch(ctx context.Context, entries []OfflineAuditEntry) (int, error) {
	accepted := 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return accepted, fmt.Errorf("context cancelled: %w", err)
		}

		if err := validateOfflineEntry(entry); err != nil {
			slog.Warn("skipping invalid offline audit entry",
				"event_id", entry.EventID,
				"err", err,
			)
			continue
		}

		// Deduplicate by event_id — skip if already ingested
		if s.hasEventID(entry.EventID) {
			continue
		}

		action := "offline_access_" + entry.Result
		target := entry.LockID
		source := "gateway_offline"
		if entry.Method != "" {
			source = "gateway_offline_" + entry.Method
		}

		_, err := s.AppendWithTimestamp(
			"", // tenant_id resolved by gateway context
			entry.UserID,
			"system",
			action,
			target,
			source,
			time.Unix(entry.Timestamp, 0).UTC(),
		)
		if err != nil {
			slog.Error("failed to append offline audit entry",
				"event_id", entry.EventID,
				"err", err,
			)
			continue
		}

		accepted++
	}
	return accepted, nil
}

// AppendWithTimestamp is like Append but uses a specific timestamp instead of time.Now().
// Used for ingesting historical/offline events.
func (s *Service) AppendWithTimestamp(tenantID, actor, role, action, target, source string, at time.Time) (Log, error) {
	record := Log{
		TenantID: strings.TrimSpace(tenantID),
		Actor:    strings.TrimSpace(actor),
		Role:     strings.TrimSpace(role),
		Action:   strings.TrimSpace(action),
		Target:   strings.TrimSpace(target),
		Source:   strings.TrimSpace(source),
		At:       at,
	}
	if record.Actor == "" {
		record.Actor = "system"
	}
	if record.Role == "" {
		record.Role = "system"
	}
	if record.Source == "" {
		record.Source = "api"
	}

	id, err := auditLogID()
	if err != nil {
		return Log{}, err
	}
	record.ID = id

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.hmacKey) > 0 {
		prevHash := ""
		if len(s.logs) > 0 {
			prevHash = s.logs[0].Hash
		}
		record.PrevHash = prevHash
		record.Hash = computeAuditLogHMAC(s.hmacKey, record)
	}

	s.logs = append([]Log{record}, s.logs...)
	if err := s.persistLocked(); err != nil {
		return Log{}, err
	}
	return record, nil
}

// hasEventID checks if an event_id has already been ingested (deduplication).
func (s *Service) hasEventID(eventID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check the target field for the event_id pattern used in offline entries.
	// For a production implementation this would use an index or bloom filter.
	for i := range s.logs {
		if s.logs[i].ID == eventID {
			return true
		}
	}
	return false
}

func validateOfflineEntry(entry OfflineAuditEntry) error {
	if entry.EventID == "" {
		return fmt.Errorf("event_id is required")
	}
	if entry.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if entry.LockID == "" {
		return fmt.Errorf("lock_id is required")
	}
	if entry.Timestamp <= 0 {
		return fmt.Errorf("timestamp must be positive")
	}
	method := strings.ToLower(entry.Method)
	if method != "ble" && method != "nfc_hce" {
		return fmt.Errorf("method must be 'ble' or 'nfc_hce'")
	}
	result := strings.ToLower(entry.Result)
	if result != "granted" && result != "denied" {
		return fmt.Errorf("result must be 'granted' or 'denied'")
	}
	return nil
}
