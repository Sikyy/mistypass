package enterprise

import (
	"sort"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/retrybackoff"
)

func (s *Service) ListHRISConnectors(tenantID string) []HRISConnector {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]HRISConnector, 0, len(s.hrisConnectors))
	for i := range s.hrisConnectors {
		if filterTenantID != "" && strings.TrimSpace(s.hrisConnectors[i].TenantID) != filterTenantID {
			continue
		}
		items = append(items, cloneHRISConnector(s.hrisConnectors[i]))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items
}

func (s *Service) GetHRISConnector(tenantID, connectorID string) (HRISConnector, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISConnector{}, ErrTenantIDRequired
	}
	nextConnectorID := strings.TrimSpace(connectorID)
	if nextConnectorID == "" {
		return HRISConnector{}, ErrHRISConnectorNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.hrisConnectors {
		item := s.hrisConnectors[i]
		if strings.TrimSpace(item.ID) != nextConnectorID {
			continue
		}
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			return HRISConnector{}, ErrHRISConnectorNotFound
		}
		return cloneHRISConnector(item), nil
	}
	return HRISConnector{}, ErrHRISConnectorNotFound
}

func (s *Service) GetHRISConnectorByID(connectorID string) (HRISConnector, error) {
	nextConnectorID := strings.TrimSpace(connectorID)
	if nextConnectorID == "" {
		return HRISConnector{}, ErrHRISConnectorNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.hrisConnectors {
		item := s.hrisConnectors[i]
		if strings.TrimSpace(item.ID) != nextConnectorID {
			continue
		}
		return cloneHRISConnector(item), nil
	}
	return HRISConnector{}, ErrHRISConnectorNotFound
}

func (s *Service) CreateHRISConnector(
	tenantID string,
	vendor string,
	status string,
	syncStrategy string,
	credentialRef string,
	webhookSecretRef string,
	updatedBy string,
) (HRISConnector, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISConnector{}, ErrTenantIDRequired
	}
	nextVendor, err := normalizeHRISConnectorVendor(vendor)
	if err != nil {
		return HRISConnector{}, err
	}
	nextStatus, err := normalizeHRISConnectorStatus(status)
	if err != nil {
		return HRISConnector{}, err
	}
	nextSyncStrategy, err := normalizeHRISConnectorSyncStrategy(syncStrategy)
	if err != nil {
		return HRISConnector{}, err
	}
	nextCredentialRef := strings.TrimSpace(credentialRef)
	nextWebhookSecretRef := strings.TrimSpace(webhookSecretRef)
	nextUpdatedBy := strings.TrimSpace(updatedBy)
	if nextUpdatedBy == "" {
		nextUpdatedBy = "system"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.hrisConnectors {
		item := s.hrisConnectors[i]
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		if strings.TrimSpace(item.Vendor) == nextVendor {
			return HRISConnector{}, ErrHRISConnectorAlreadyExists
		}
	}

	connectorID, err := randomID("hrc_")
	if err != nil {
		return HRISConnector{}, err
	}
	now := time.Now().UTC()
	record := HRISConnector{
		ID:               connectorID,
		TenantID:         nextTenantID,
		Vendor:           nextVendor,
		Status:           nextStatus,
		SyncStrategy:     nextSyncStrategy,
		CredentialRef:    nextCredentialRef,
		WebhookSecretRef: nextWebhookSecretRef,
		UpdatedBy:        nextUpdatedBy,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.hrisConnectors = append([]HRISConnector{record}, s.hrisConnectors...)
	if err := s.persistLocked(); err != nil {
		return HRISConnector{}, err
	}
	return cloneHRISConnector(record), nil
}

func (s *Service) UpdateHRISConnector(
	tenantID string,
	connectorID string,
	status string,
	syncStrategy string,
	credentialRef string,
	webhookSecretRef string,
	updatedBy string,
) (HRISConnector, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISConnector{}, ErrTenantIDRequired
	}
	nextConnectorID := strings.TrimSpace(connectorID)
	if nextConnectorID == "" {
		return HRISConnector{}, ErrHRISConnectorNotFound
	}

	nextStatus := ""
	if strings.TrimSpace(status) != "" {
		var err error
		nextStatus, err = normalizeHRISConnectorStatus(status)
		if err != nil {
			return HRISConnector{}, err
		}
	}
	nextSyncStrategy := ""
	if strings.TrimSpace(syncStrategy) != "" {
		var err error
		nextSyncStrategy, err = normalizeHRISConnectorSyncStrategy(syncStrategy)
		if err != nil {
			return HRISConnector{}, err
		}
	}
	nextCredentialRef := strings.TrimSpace(credentialRef)
	nextWebhookSecretRef := strings.TrimSpace(webhookSecretRef)
	nextUpdatedBy := strings.TrimSpace(updatedBy)
	if nextUpdatedBy == "" {
		nextUpdatedBy = "system"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.hrisConnectors {
		item := s.hrisConnectors[i]
		if strings.TrimSpace(item.ID) != nextConnectorID {
			continue
		}
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			return HRISConnector{}, ErrHRISConnectorNotFound
		}
		if nextStatus != "" {
			item.Status = nextStatus
		}
		if nextSyncStrategy != "" {
			item.SyncStrategy = nextSyncStrategy
		}
		if nextCredentialRef != "" {
			item.CredentialRef = nextCredentialRef
		}
		if nextWebhookSecretRef != "" {
			item.WebhookSecretRef = nextWebhookSecretRef
		}
		item.UpdatedBy = nextUpdatedBy
		item.UpdatedAt = time.Now().UTC()
		s.hrisConnectors[i] = item
		if err := s.persistLocked(); err != nil {
			return HRISConnector{}, err
		}
		return cloneHRISConnector(item), nil
	}

	return HRISConnector{}, ErrHRISConnectorNotFound
}

func (s *Service) MarkHRISConnectorSynced(
	tenantID string,
	connectorID string,
	syncedAt time.Time,
) (HRISConnector, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISConnector{}, ErrTenantIDRequired
	}
	nextConnectorID := strings.TrimSpace(connectorID)
	if nextConnectorID == "" {
		return HRISConnector{}, ErrHRISConnectorNotFound
	}
	now := syncedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.hrisConnectors {
		item := s.hrisConnectors[i]
		if strings.TrimSpace(item.ID) != nextConnectorID {
			continue
		}
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			return HRISConnector{}, ErrHRISConnectorNotFound
		}
		item.LastSyncAt = &now
		item.UpdatedAt = now
		s.hrisConnectors[i] = item
		if err := s.persistLocked(); err != nil {
			return HRISConnector{}, err
		}
		return cloneHRISConnector(item), nil
	}

	return HRISConnector{}, ErrHRISConnectorNotFound
}

func (s *Service) ListHRISWebhookReceipts(tenantID, connectorID string, limit int) []HRISWebhookReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := listHRISWebhookReceipts(s.hrisWebhookReceipts, tenantID, connectorID)
	if limit <= 0 || limit > maxWebhookReceiptLimit {
		limit = maxWebhookReceiptLimit
	}
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) ListAllHRISWebhookReceipts(tenantID, connectorID string) []HRISWebhookReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return listHRISWebhookReceipts(s.hrisWebhookReceipts, tenantID, connectorID)
}

func listHRISWebhookReceipts(items []HRISWebhookReceipt, tenantID, connectorID string) []HRISWebhookReceipt {
	filterTenantID := strings.TrimSpace(tenantID)
	filterConnectorID := strings.TrimSpace(connectorID)
	result := make([]HRISWebhookReceipt, 0, len(items))
	for i := range items {
		item := items[i]
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		if filterConnectorID != "" && strings.TrimSpace(item.ConnectorID) != filterConnectorID {
			continue
		}
		result = append(result, cloneHRISWebhookReceipt(item))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ReceivedAt.Equal(result[j].ReceivedAt) {
			return result[i].ID > result[j].ID
		}
		return result[i].ReceivedAt.After(result[j].ReceivedAt)
	})
	return result
}

func (s *Service) GetHRISWebhookReceipt(tenantID, receiptID string) (HRISWebhookReceipt, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISWebhookReceipt{}, ErrTenantIDRequired
	}
	nextReceiptID := strings.TrimSpace(receiptID)
	if nextReceiptID == "" {
		return HRISWebhookReceipt{}, ErrHRISWebhookReceiptNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.hrisWebhookReceipts {
		item := s.hrisWebhookReceipts[i]
		if strings.TrimSpace(item.ID) != nextReceiptID {
			continue
		}
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			return HRISWebhookReceipt{}, ErrHRISWebhookReceiptNotFound
		}
		return cloneHRISWebhookReceipt(item), nil
	}
	return HRISWebhookReceipt{}, ErrHRISWebhookReceiptNotFound
}

func findHRISWebhookReceiptByIDLocked(
	items []HRISWebhookReceipt,
	receiptID string,
) (HRISWebhookReceipt, bool) {
	nextReceiptID := strings.TrimSpace(receiptID)
	if nextReceiptID == "" {
		return HRISWebhookReceipt{}, false
	}
	for i := range items {
		if strings.TrimSpace(items[i].ID) != nextReceiptID {
			continue
		}
		return items[i], true
	}
	return HRISWebhookReceipt{}, false
}

func (s *Service) ReceiveHRISWebhookReceipt(connectorID string, input HRISWebhookReceiptInput) (HRISWebhookReceipt, error) {
	nextConnectorID := strings.TrimSpace(connectorID)
	if nextConnectorID == "" {
		return HRISWebhookReceipt{}, ErrHRISConnectorNotFound
	}

	nextEventType := strings.TrimSpace(input.EventType)
	nextRequestID := strings.TrimSpace(input.RequestID)
	nextContentType := strings.TrimSpace(input.ContentType)
	nextSourceIP := strings.TrimSpace(input.SourceIP)
	nextHeaders := make(map[string]string, len(input.Headers))
	for key, value := range input.Headers {
		nextKey := strings.ToLower(strings.TrimSpace(key))
		if nextKey == "" {
			continue
		}
		nextHeaders[nextKey] = strings.TrimSpace(value)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	connectorIndex := findHRISConnectorIndexLocked(s.hrisConnectors, nextConnectorID)
	if connectorIndex < 0 {
		return HRISWebhookReceipt{}, ErrHRISConnectorNotFound
	}
	connector := s.hrisConnectors[connectorIndex]
	if strings.TrimSpace(connector.Status) != "active" {
		return HRISWebhookReceipt{}, ErrHRISConnectorInactive
	}

	receiptID, err := randomID("whr_")
	if err != nil {
		return HRISWebhookReceipt{}, err
	}
	now := time.Now().UTC()
	record := HRISWebhookReceipt{
		ID:          receiptID,
		EventType:   nextEventType,
		RequestID:   nextRequestID,
		ContentType: nextContentType,
		Headers:     nextHeaders,
		RawPayload:  input.RawPayload,
		SourceIP:    nextSourceIP,
		Status:      "received",
		ReceivedAt:  now,
	}
	if err := s.mutateHRISWebhookStateLocked(func() (bool, error) {
		connectorIndex := findHRISConnectorIndexLocked(s.hrisConnectors, nextConnectorID)
		if connectorIndex < 0 {
			return false, ErrHRISConnectorNotFound
		}
		currentConnector := s.hrisConnectors[connectorIndex]
		if strings.TrimSpace(currentConnector.Status) != "active" {
			return false, ErrHRISConnectorInactive
		}

		record.TenantID = currentConnector.TenantID
		record.ConnectorID = currentConnector.ID
		record.Vendor = currentConnector.Vendor
		s.hrisWebhookReceipts = append([]HRISWebhookReceipt{record}, s.hrisWebhookReceipts...)
		s.upsertHRISWebhookReceiptDueIndexLocked(record.ID, record.ReceivedAt)
		return true, nil
	}); err != nil {
		return HRISWebhookReceipt{}, err
	}
	return cloneHRISWebhookReceipt(record), nil
}

func (s *Service) ListPendingHRISWebhookReceipts(tenantID string, limit int) []HRISWebhookReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]HRISWebhookReceipt, 0, len(s.hrisWebhookReceipts))
	for i := range s.hrisWebhookReceipts {
		item := s.hrisWebhookReceipts[i]
		if strings.TrimSpace(item.Status) != "received" {
			continue
		}
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		items = append(items, cloneHRISWebhookReceipt(item))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ReceivedAt.Equal(items[j].ReceivedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].ReceivedAt.After(items[j].ReceivedAt)
	})
	if limit <= 0 || limit > maxWebhookReceiptLimit {
		limit = maxWebhookReceiptLimit
	}
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) ListRetryableHRISWebhookReceipts(tenantID string, limit int) []HRISWebhookReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]HRISWebhookReceipt, 0, len(s.hrisWebhookReceipts))
	for i := range s.hrisWebhookReceipts {
		item := s.hrisWebhookReceipts[i]
		status := strings.TrimSpace(item.Status)
		if status != "received" && status != "failed" {
			continue
		}
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		items = append(items, cloneHRISWebhookReceipt(item))
	}
	sort.Slice(items, func(i, j int) bool {
		leftAttemptAt := items[i].ReceivedAt
		if items[i].LastAttemptAt != nil {
			leftAttemptAt = *items[i].LastAttemptAt
		}
		rightAttemptAt := items[j].ReceivedAt
		if items[j].LastAttemptAt != nil {
			rightAttemptAt = *items[j].LastAttemptAt
		}
		if leftAttemptAt.Equal(rightAttemptAt) {
			return items[i].ID > items[j].ID
		}
		return leftAttemptAt.Before(rightAttemptAt)
	})
	if limit <= 0 || limit > maxWebhookReceiptLimit {
		limit = maxWebhookReceiptLimit
	}
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) ListQueueableHRISWebhookReceipts(tenantID string, limit int) []HRISWebhookReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]HRISWebhookReceipt, 0, len(s.hrisWebhookReceipts))
	for i := range s.hrisWebhookReceipts {
		item := s.hrisWebhookReceipts[i]
		status := strings.TrimSpace(item.Status)
		if status != "received" && status != "failed" && status != "processing" {
			continue
		}
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		items = append(items, cloneHRISWebhookReceipt(item))
	}
	sort.Slice(items, func(i, j int) bool {
		leftAttemptAt := items[i].ReceivedAt
		if items[i].LastAttemptAt != nil {
			leftAttemptAt = *items[i].LastAttemptAt
		}
		rightAttemptAt := items[j].ReceivedAt
		if items[j].LastAttemptAt != nil {
			rightAttemptAt = *items[j].LastAttemptAt
		}
		if leftAttemptAt.Equal(rightAttemptAt) {
			return items[i].ID > items[j].ID
		}
		return leftAttemptAt.Before(rightAttemptAt)
	})
	if limit <= 0 || limit > maxWebhookReceiptLimit {
		limit = maxWebhookReceiptLimit
	}
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) ListClaimableHRISWebhookReceiptsWithBackoff(
	tenantID string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookReceipt {
	maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now = normalizeHRISWebhookReceiptClaimParams(
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
	)

	s.mu.RLock()
	defer s.mu.RUnlock()

	return listClaimableHRISWebhookReceiptsWithBackoffLocked(
		s.hrisWebhookReceipts,
		tenantID,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
		limit,
	)
}

func (s *Service) ListDueHRISWebhookReceiptsWithBackoff(
	tenantID string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookReceipt {
	maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now = normalizeHRISWebhookReceiptClaimParams(
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
	)

	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]HRISWebhookReceipt, 0, limit)
	seen := make(map[string]struct{}, len(s.dueReceiptIDs))
	for i := range s.dueReceiptIDs {
		entry := s.dueReceiptIDs[i]
		if !entry.DueAt.IsZero() && entry.DueAt.After(now) {
			break
		}

		item, ok := findHRISWebhookReceiptByIDLocked(s.hrisWebhookReceipts, entry.ReceiptID)
		if !ok {
			continue
		}
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		if hrisWebhookReceiptClaimReason(item, maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now) != "" {
			continue
		}
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		items = append(items, cloneHRISWebhookReceipt(item))
		if limit > 0 && len(items) >= limit {
			return items
		}
	}

	fallbackItems := listClaimableHRISWebhookReceiptsWithBackoffLocked(
		s.hrisWebhookReceipts,
		tenantID,
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

func listClaimableHRISWebhookReceiptsWithBackoffLocked(
	allReceipts []HRISWebhookReceipt,
	tenantID string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookReceipt {
	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]HRISWebhookReceipt, 0, len(allReceipts))
	for i := range allReceipts {
		item := allReceipts[i]
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		if hrisWebhookReceiptClaimReason(item, maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now) != "" {
			continue
		}
		items = append(items, cloneHRISWebhookReceipt(item))
	}
	sort.Slice(items, func(i, j int) bool {
		leftAttemptAt := items[i].ReceivedAt
		if items[i].LastAttemptAt != nil {
			leftAttemptAt = *items[i].LastAttemptAt
		}
		rightAttemptAt := items[j].ReceivedAt
		if items[j].LastAttemptAt != nil {
			rightAttemptAt = *items[j].LastAttemptAt
		}
		if leftAttemptAt.Equal(rightAttemptAt) {
			return items[i].ID > items[j].ID
		}
		return leftAttemptAt.Before(rightAttemptAt)
	})
	if limit <= 0 || limit > maxWebhookReceiptLimit {
		limit = maxWebhookReceiptLimit
	}
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) ClaimHRISWebhookReceiptForProcessing(
	tenantID string,
	receiptID string,
	maxAttempts int,
	retryCooldown time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) (HRISWebhookReceipt, string, error) {
	return s.ClaimHRISWebhookReceiptForProcessingWithBackoff(
		tenantID,
		receiptID,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		processingTimeout,
		now,
	)
}

func (s *Service) ClaimHRISWebhookReceiptForProcessingWithBackoff(
	tenantID string,
	receiptID string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) (HRISWebhookReceipt, string, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISWebhookReceipt{}, "", ErrTenantIDRequired
	}
	nextReceiptID := strings.TrimSpace(receiptID)
	if nextReceiptID == "" {
		return HRISWebhookReceipt{}, "", ErrHRISWebhookReceiptNotFound
	}
	maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now = normalizeHRISWebhookReceiptClaimParams(
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
	)

	s.mu.Lock()
	defer s.mu.Unlock()

	var claimed HRISWebhookReceipt
	claimReason := ""
	if err := s.mutateHRISWebhookStateLocked(func() (bool, error) {
		for i := range s.hrisWebhookReceipts {
			if strings.TrimSpace(s.hrisWebhookReceipts[i].ID) != nextReceiptID {
				continue
			}
			if strings.TrimSpace(s.hrisWebhookReceipts[i].TenantID) != nextTenantID {
				return false, ErrHRISWebhookReceiptNotFound
			}

			if reason := hrisWebhookReceiptClaimReason(
				s.hrisWebhookReceipts[i],
				maxAttempts,
				retryCooldown,
				retryMaxBackoff,
				processingTimeout,
				now,
			); reason != "" {
				claimed = cloneHRISWebhookReceipt(s.hrisWebhookReceipts[i])
				claimReason = reason
				return false, nil
			}

			s.hrisWebhookReceipts[i].Status = "processing"
			s.hrisWebhookReceipts[i].LastError = ""
			s.hrisWebhookReceipts[i].ProcessedAt = nil
			s.hrisWebhookReceipts[i].AttemptCount++
			s.hrisWebhookReceipts[i].LastAttemptAt = &now
			s.upsertHRISWebhookReceiptDueIndexLocked(
				s.hrisWebhookReceipts[i].ID,
				hrisWebhookReceiptProcessingDueAt(now, processingTimeout),
			)
			claimed = cloneHRISWebhookReceipt(s.hrisWebhookReceipts[i])
			claimReason = ""
			return true, nil
		}
		return false, ErrHRISWebhookReceiptNotFound
	}); err != nil {
		return HRISWebhookReceipt{}, "", err
	}
	if claimReason != "" {
		return claimed, claimReason, nil
	}
	if claimed.ID == "" {
		return HRISWebhookReceipt{}, "", ErrHRISWebhookReceiptNotFound
	}
	return claimed, "", nil
}

func normalizeHRISWebhookReceiptClaimParams(
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) (int, time.Duration, time.Duration, time.Duration, time.Time) {
	if maxAttempts <= 0 {
		maxAttempts = 1
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

func hrisWebhookReceiptClaimReason(
	item HRISWebhookReceipt,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) string {
	status := strings.TrimSpace(item.Status)
	switch status {
	case "received":
		return ""
	case "failed":
		if maxAttempts > 0 && item.AttemptCount >= maxAttempts {
			return HRISWebhookReceiptClaimReasonAttemptLimit
		}
		if retryDelay := retrybackoff.Exponential(
			item.AttemptCount,
			retryCooldown,
			retryMaxBackoff,
		); retryDelay > 0 && item.LastAttemptAt != nil {
			if item.LastAttemptAt.Add(retryDelay).After(now) {
				return HRISWebhookReceiptClaimReasonCooldown
			}
		}
		return ""
	case "processing":
		if item.LastAttemptAt != nil && item.LastAttemptAt.Add(processingTimeout).After(now) {
			return HRISWebhookReceiptClaimReasonInFlight
		}
		return ""
	default:
		return HRISWebhookReceiptClaimReasonNotQueueable
	}
}

func isQueueableHRISWebhookReceiptStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "received", "failed", "processing":
		return true
	default:
		return false
	}
}

func hrisWebhookReceiptDueIndexHeuristic(item HRISWebhookReceipt) time.Time {
	if item.LastAttemptAt != nil && !item.LastAttemptAt.IsZero() {
		return item.LastAttemptAt.UTC()
	}
	return item.ReceivedAt.UTC()
}

func hrisWebhookReceiptProcessingDueAt(now time.Time, processingTimeout time.Duration) time.Time {
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

func hrisWebhookReceiptFailureDueAt(
	item HRISWebhookReceipt,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	now time.Time,
) time.Time {
	base := now
	if item.LastAttemptAt != nil && !item.LastAttemptAt.IsZero() {
		base = item.LastAttemptAt.UTC()
	} else if now.IsZero() {
		base = time.Now().UTC()
	} else {
		base = now.UTC()
	}
	return base.Add(retrybackoff.Exponential(item.AttemptCount, retryCooldown, retryMaxBackoff))
}

func sortHRISWebhookReceiptDueIndexEntries(items []hrisWebhookReceiptDueIndexEntry) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].DueAt.Equal(items[j].DueAt) {
			return items[i].ReceiptID < items[j].ReceiptID
		}
		return items[i].DueAt.Before(items[j].DueAt)
	})
}

func (s *Service) upsertHRISWebhookReceiptDueIndexLocked(receiptID string, dueAt time.Time) {
	nextReceiptID := strings.TrimSpace(receiptID)
	if nextReceiptID == "" {
		return
	}
	nextDueAt := dueAt
	if nextDueAt.IsZero() {
		nextDueAt = time.Now().UTC()
	} else {
		nextDueAt = nextDueAt.UTC()
	}
	for i := range s.dueReceiptIDs {
		if strings.TrimSpace(s.dueReceiptIDs[i].ReceiptID) != nextReceiptID {
			continue
		}
		s.dueReceiptIDs[i].DueAt = nextDueAt
		sortHRISWebhookReceiptDueIndexEntries(s.dueReceiptIDs)
		return
	}
	s.dueReceiptIDs = append(s.dueReceiptIDs, hrisWebhookReceiptDueIndexEntry{
		ReceiptID: nextReceiptID,
		DueAt:     nextDueAt,
	})
	sortHRISWebhookReceiptDueIndexEntries(s.dueReceiptIDs)
}

func (s *Service) removeHRISWebhookReceiptDueIndexLocked(receiptID string) {
	nextReceiptID := strings.TrimSpace(receiptID)
	if nextReceiptID == "" || len(s.dueReceiptIDs) == 0 {
		return
	}
	filtered := s.dueReceiptIDs[:0]
	for i := range s.dueReceiptIDs {
		if strings.TrimSpace(s.dueReceiptIDs[i].ReceiptID) == nextReceiptID {
			continue
		}
		filtered = append(filtered, s.dueReceiptIDs[i])
	}
	s.dueReceiptIDs = filtered
}

func (s *Service) normalizeHRISWebhookReceiptDueIndexLocked() {
	if len(s.hrisWebhookReceipts) == 0 {
		s.dueReceiptIDs = nil
		return
	}

	existing := make(map[string]hrisWebhookReceiptDueIndexEntry, len(s.dueReceiptIDs))
	for i := range s.dueReceiptIDs {
		entry := s.dueReceiptIDs[i]
		nextReceiptID := strings.TrimSpace(entry.ReceiptID)
		if nextReceiptID == "" {
			continue
		}
		if entry.DueAt.IsZero() {
			continue
		}
		entry.ReceiptID = nextReceiptID
		entry.DueAt = entry.DueAt.UTC()
		existing[nextReceiptID] = entry
	}

	normalized := make([]hrisWebhookReceiptDueIndexEntry, 0, len(s.hrisWebhookReceipts))
	for i := range s.hrisWebhookReceipts {
		item := s.hrisWebhookReceipts[i]
		if !isQueueableHRISWebhookReceiptStatus(item.Status) {
			continue
		}
		nextReceiptID := strings.TrimSpace(item.ID)
		if nextReceiptID == "" {
			continue
		}
		entry, ok := existing[nextReceiptID]
		if !ok {
			entry = hrisWebhookReceiptDueIndexEntry{
				ReceiptID: nextReceiptID,
				DueAt:     hrisWebhookReceiptDueIndexHeuristic(item),
			}
		}
		normalized = append(normalized, entry)
	}
	sortHRISWebhookReceiptDueIndexEntries(normalized)
	s.dueReceiptIDs = normalized
}

func buildHRISWebhookReceiptDueIndexEntries(
	items []HRISWebhookReceipt,
) []hrisWebhookReceiptDueIndexEntry {
	result := make([]hrisWebhookReceiptDueIndexEntry, 0, len(items))
	for i := range items {
		if !isQueueableHRISWebhookReceiptStatus(items[i].Status) {
			continue
		}
		nextReceiptID := strings.TrimSpace(items[i].ID)
		if nextReceiptID == "" {
			continue
		}
		result = append(result, hrisWebhookReceiptDueIndexEntry{
			ReceiptID: nextReceiptID,
			DueAt:     hrisWebhookReceiptDueIndexHeuristic(items[i]),
		})
	}
	sortHRISWebhookReceiptDueIndexEntries(result)
	return result
}

func (s *Service) MarkHRISWebhookReceiptStarted(tenantID, receiptID string) (HRISWebhookReceipt, error) {
	now := time.Now().UTC()
	return s.updateHRISWebhookReceiptStatus(
		tenantID,
		receiptID,
		"processing",
		"",
		nil,
		func(item *HRISWebhookReceipt) {
			item.AttemptCount++
			item.LastAttemptAt = &now
		},
		func(item HRISWebhookReceipt) {
			s.upsertHRISWebhookReceiptDueIndexLocked(item.ID, now)
		},
	)
}

func (s *Service) MarkHRISWebhookReceiptProcessed(tenantID, receiptID string) (HRISWebhookReceipt, error) {
	now := time.Now().UTC()
	return s.updateHRISWebhookReceiptStatus(
		tenantID,
		receiptID,
		"processed",
		"",
		&now,
		nil,
		func(item HRISWebhookReceipt) {
			s.removeHRISWebhookReceiptDueIndexLocked(item.ID)
		},
	)
}

func (s *Service) MarkHRISWebhookReceiptSkipped(tenantID, receiptID, reason string) (HRISWebhookReceipt, error) {
	now := time.Now().UTC()
	return s.updateHRISWebhookReceiptStatus(
		tenantID,
		receiptID,
		"skipped",
		reason,
		&now,
		nil,
		func(item HRISWebhookReceipt) {
			s.removeHRISWebhookReceiptDueIndexLocked(item.ID)
		},
	)
}

func (s *Service) MarkHRISWebhookReceiptFailed(tenantID, receiptID string, failure error) (HRISWebhookReceipt, error) {
	return s.MarkHRISWebhookReceiptFailedWithBackoff(tenantID, receiptID, failure, 0, 0)
}

func (s *Service) MarkHRISWebhookReceiptFailedWithBackoff(
	tenantID, receiptID string,
	failure error,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
) (HRISWebhookReceipt, error) {
	now := time.Now().UTC()
	message := ""
	if failure != nil {
		message = failure.Error()
	}
	return s.updateHRISWebhookReceiptStatus(
		tenantID,
		receiptID,
		"failed",
		message,
		&now,
		nil,
		func(item HRISWebhookReceipt) {
			s.upsertHRISWebhookReceiptDueIndexLocked(
				item.ID,
				hrisWebhookReceiptFailureDueAt(item, retryCooldown, retryMaxBackoff, now),
			)
		},
	)
}

func (s *Service) MarkHRISWebhookReceiptDLQ(tenantID, receiptID string, failure error) (HRISWebhookReceipt, error) {
	now := time.Now().UTC()
	message := ""
	if failure != nil {
		message = failure.Error()
	}
	return s.updateHRISWebhookReceiptStatus(
		tenantID,
		receiptID,
		"dlq",
		message,
		&now,
		nil,
		func(item HRISWebhookReceipt) {
			s.removeHRISWebhookReceiptDueIndexLocked(item.ID)
		},
	)
}

func (s *Service) RestoreHRISWebhookReceipt(snapshot HRISWebhookReceipt) (HRISWebhookReceipt, error) {
	nextTenantID := strings.TrimSpace(snapshot.TenantID)
	if nextTenantID == "" {
		return HRISWebhookReceipt{}, ErrTenantIDRequired
	}
	nextReceiptID := strings.TrimSpace(snapshot.ID)
	if nextReceiptID == "" {
		return HRISWebhookReceipt{}, ErrHRISWebhookReceiptNotFound
	}
	restoredSnapshot := cloneHRISWebhookReceipt(snapshot)

	s.mu.Lock()
	defer s.mu.Unlock()

	var restored HRISWebhookReceipt
	if err := s.mutateHRISWebhookStateLocked(func() (bool, error) {
		for i := range s.hrisWebhookReceipts {
			if strings.TrimSpace(s.hrisWebhookReceipts[i].ID) != nextReceiptID {
				continue
			}
			if strings.TrimSpace(s.hrisWebhookReceipts[i].TenantID) != nextTenantID {
				return false, ErrHRISWebhookReceiptNotFound
			}
			s.hrisWebhookReceipts[i] = cloneHRISWebhookReceipt(restoredSnapshot)
			restored = cloneHRISWebhookReceipt(s.hrisWebhookReceipts[i])
			return true, nil
		}
		return false, ErrHRISWebhookReceiptNotFound
	}); err != nil {
		return HRISWebhookReceipt{}, err
	}
	return restored, nil
}

func (s *Service) updateHRISWebhookReceiptStatus(
	tenantID string,
	receiptID string,
	status string,
	lastError string,
	processedAt *time.Time,
	mutate func(item *HRISWebhookReceipt),
	adjustDueIndex func(item HRISWebhookReceipt),
) (HRISWebhookReceipt, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISWebhookReceipt{}, ErrTenantIDRequired
	}
	nextReceiptID := strings.TrimSpace(receiptID)
	if nextReceiptID == "" {
		return HRISWebhookReceipt{}, ErrHRISWebhookReceiptNotFound
	}
	nextStatus := strings.ToLower(strings.TrimSpace(status))
	if nextStatus == "" {
		nextStatus = "received"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var updated HRISWebhookReceipt
	if err := s.mutateHRISWebhookStateLocked(func() (bool, error) {
		for i := range s.hrisWebhookReceipts {
			if strings.TrimSpace(s.hrisWebhookReceipts[i].ID) != nextReceiptID {
				continue
			}
			if strings.TrimSpace(s.hrisWebhookReceipts[i].TenantID) != nextTenantID {
				return false, ErrHRISWebhookReceiptNotFound
			}
			s.hrisWebhookReceipts[i].Status = nextStatus
			s.hrisWebhookReceipts[i].LastError = strings.TrimSpace(lastError)
			if processedAt == nil {
				s.hrisWebhookReceipts[i].ProcessedAt = nil
			} else {
				nextProcessedAt := processedAt.UTC()
				s.hrisWebhookReceipts[i].ProcessedAt = &nextProcessedAt
			}
			if mutate != nil {
				mutate(&s.hrisWebhookReceipts[i])
			}
			if adjustDueIndex != nil {
				adjustDueIndex(s.hrisWebhookReceipts[i])
			}
			updated = cloneHRISWebhookReceipt(s.hrisWebhookReceipts[i])
			return true, nil
		}
		return false, ErrHRISWebhookReceiptNotFound
	}); err != nil {
		return HRISWebhookReceipt{}, err
	}
	return updated, nil
}

func (s *Service) CreateHRISWebhookExecution(input HRISWebhookExecutionInput) (HRISWebhookExecution, error) {
	nextTenantID := strings.TrimSpace(input.TenantID)
	if nextTenantID == "" {
		return HRISWebhookExecution{}, ErrTenantIDRequired
	}
	nextKind := normalizeHRISWebhookExecutionKind(input.Kind)
	if nextKind == "" {
		return HRISWebhookExecution{}, ErrInvalidHRISWebhookExecutionKind
	}
	nextTargetID := strings.TrimSpace(input.TargetID)
	if nextTargetID == "" {
		return HRISWebhookExecution{}, ErrHRISWebhookExecutionNotFound
	}
	nextDispatchMode := normalizeHRISWebhookExecutionDispatchMode(input.DispatchMode)
	if nextDispatchMode == "" {
		return HRISWebhookExecution{}, ErrInvalidHRISWebhookExecutionDispatchMode
	}

	executionID, err := randomID("hwe_")
	if err != nil {
		return HRISWebhookExecution{}, err
	}
	now := time.Now().UTC()
	record := HRISWebhookExecution{
		ID:                      executionID,
		TenantID:                nextTenantID,
		Kind:                    nextKind,
		TargetID:                nextTargetID,
		ReceiptID:               strings.TrimSpace(input.ReceiptID),
		ConnectorID:             strings.TrimSpace(input.ConnectorID),
		Vendor:                  strings.TrimSpace(input.Vendor),
		RequestID:               strings.TrimSpace(input.RequestID),
		EventType:               strings.TrimSpace(input.EventType),
		FailureStage:            strings.TrimSpace(input.FailureStage),
		AuditSource:             strings.TrimSpace(input.AuditSource),
		ExecutionMode:           strings.TrimSpace(input.ExecutionMode),
		DispatchMode:            nextDispatchMode,
		Status:                  HRISWebhookExecutionStatusQueued,
		TargetStatus:            strings.TrimSpace(input.TargetStatus),
		RequestedBy:             strings.TrimSpace(input.RequestedBy),
		ReplaySourceExecutionID: strings.TrimSpace(input.ReplaySourceExecutionID),
		ReplayRequireWorker:     cloneOptionalBool(input.ReplayRequireWorker),
		QueuedAt:                now,
		UpdatedAt:               now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.mutateHRISWebhookStateLocked(func() (bool, error) {
		if strings.TrimSpace(record.ReplaySourceExecutionID) != "" {
			if existing, ok := findActiveHRISWebhookReplayExecutionLocked(
				s.hrisWebhookExecutions,
				record.TenantID,
				record.ReplaySourceExecutionID,
			); ok {
				return false, &HRISWebhookExecutionReplayConflictError{
					ExistingExecution: cloneHRISWebhookExecution(existing),
				}
			}
		}
		s.hrisWebhookExecutions = append([]HRISWebhookExecution{record}, s.hrisWebhookExecutions...)
		if len(s.hrisWebhookExecutions) > maxWebhookExecutionLimit {
			s.hrisWebhookExecutions = s.hrisWebhookExecutions[:maxWebhookExecutionLimit]
		}
		return true, nil
	}); err != nil {
		return HRISWebhookExecution{}, err
	}
	return cloneHRISWebhookExecution(record), nil
}

func (s *Service) ListHRISWebhookExecutions(tenantID string, limit int) []HRISWebhookExecution {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := listHRISWebhookExecutions(s.hrisWebhookExecutions, tenantID)
	if limit <= 0 || limit >= len(items) {
		return items
	}
	return items[:limit]
}

func (s *Service) ListAllHRISWebhookExecutions(tenantID string) []HRISWebhookExecution {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return listHRISWebhookExecutions(s.hrisWebhookExecutions, tenantID)
}

func (s *Service) FindActiveHRISWebhookReplayExecution(
	tenantID string,
	sourceExecutionID string,
) (HRISWebhookExecution, bool) {
	nextTenantID := strings.TrimSpace(tenantID)
	nextSourceExecutionID := strings.TrimSpace(sourceExecutionID)
	if nextTenantID == "" || nextSourceExecutionID == "" {
		return HRISWebhookExecution{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := findActiveHRISWebhookReplayExecutionLocked(
		s.hrisWebhookExecutions,
		nextTenantID,
		nextSourceExecutionID,
	)
	if !ok {
		return HRISWebhookExecution{}, false
	}
	return cloneHRISWebhookExecution(item), true
}

func (s *Service) ListQueuedHRISWebhookExecutions(kind string, limit int) []HRISWebhookExecution {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.listQueuedHRISWebhookExecutionsLocked(kind, limit)
}

func (s *Service) ListClaimableHRISWebhookExecutions(
	kind string,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookExecution {
	processingTimeout, now = normalizeHRISWebhookExecutionClaimParams(processingTimeout, now)

	s.mu.RLock()
	defer s.mu.RUnlock()

	return listClaimableHRISWebhookExecutionsLocked(
		s.hrisWebhookExecutions,
		kind,
		processingTimeout,
		now,
		limit,
	)
}

func (s *Service) ListIndexedClaimableHRISWebhookExecutions(
	kind string,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookExecution {
	processingTimeout, now = normalizeHRISWebhookExecutionClaimParams(processingTimeout, now)

	s.mu.RLock()
	defer s.mu.RUnlock()

	return listIndexedClaimableHRISWebhookExecutionsLocked(
		s.hrisWebhookExecutions,
		s.listQueuedHRISWebhookExecutionsLocked(kind, limit),
		kind,
		processingTimeout,
		now,
		limit,
	)
}

func listHRISWebhookExecutions(items []HRISWebhookExecution, tenantID string) []HRISWebhookExecution {
	filterTenantID := strings.TrimSpace(tenantID)
	result := make([]HRISWebhookExecution, 0, len(items))
	for i := range items {
		item := items[i]
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		result = append(result, cloneHRISWebhookExecution(item))
	}
	return result
}

func (s *Service) listQueuedHRISWebhookExecutionsLocked(kind string, limit int) []HRISWebhookExecution {
	var ids []string
	switch normalizeHRISWebhookExecutionKind(kind) {
	case HRISWebhookExecutionKindReceiptProcess:
		ids = s.queuedReceiptExecutionIDs
	case HRISWebhookExecutionKindDLQReplay:
		ids = s.queuedDLQReplayExecutionIDs
	default:
		return nil
	}
	if len(ids) == 0 {
		return nil
	}

	items := make([]HRISWebhookExecution, 0, len(ids))
	for i := range ids {
		item, ok := findHRISWebhookExecutionByIDLocked(s.hrisWebhookExecutions, ids[i])
		if !ok || !isQueuedHRISWebhookExecutionCandidate(item) {
			continue
		}
		items = append(items, cloneHRISWebhookExecution(item))
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items
}

func findHRISWebhookExecutionByIDLocked(
	items []HRISWebhookExecution,
	executionID string,
) (HRISWebhookExecution, bool) {
	nextExecutionID := strings.TrimSpace(executionID)
	if nextExecutionID == "" {
		return HRISWebhookExecution{}, false
	}
	for i := range items {
		if strings.TrimSpace(items[i].ID) != nextExecutionID {
			continue
		}
		return items[i], true
	}
	return HRISWebhookExecution{}, false
}

func isQueuedHRISWebhookExecutionCandidate(item HRISWebhookExecution) bool {
	if !isWorkerManagedHRISWebhookExecutionCandidate(item) {
		return false
	}
	return strings.TrimSpace(item.Status) == HRISWebhookExecutionStatusQueued
}

func isWorkerManagedHRISWebhookExecutionCandidate(item HRISWebhookExecution) bool {
	switch normalizeHRISWebhookExecutionKind(item.Kind) {
	case HRISWebhookExecutionKindReceiptProcess, HRISWebhookExecutionKindDLQReplay:
	default:
		return false
	}
	if strings.TrimSpace(item.ExecutionMode) != "queued" {
		return false
	}
	if strings.TrimSpace(item.DispatchMode) != HRISWebhookExecutionDispatchModeWorkerTick {
		return false
	}
	return strings.TrimSpace(item.TargetID) != ""
}

func normalizeHRISWebhookExecutionClaimParams(
	processingTimeout time.Duration,
	now time.Time,
) (time.Duration, time.Time) {
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	return processingTimeout, now
}

func hrisWebhookExecutionClaimReason(
	item HRISWebhookExecution,
	processingTimeout time.Duration,
	now time.Time,
) string {
	if !isWorkerManagedHRISWebhookExecutionCandidate(item) {
		return HRISWebhookExecutionClaimReasonNotQueueable
	}
	switch strings.TrimSpace(item.Status) {
	case HRISWebhookExecutionStatusQueued:
		if !item.QueuedAt.IsZero() && item.QueuedAt.UTC().After(now) {
			return HRISWebhookExecutionClaimReasonCooldown
		}
		return ""
	case HRISWebhookExecutionStatusRunning:
		if item.StartedAt != nil && item.StartedAt.Add(processingTimeout).After(now) {
			return HRISWebhookExecutionClaimReasonInFlight
		}
		return ""
	default:
		return HRISWebhookExecutionClaimReasonNotQueueable
	}
}

func hrisWebhookExecutionClaimSortTime(item HRISWebhookExecution) time.Time {
	if strings.TrimSpace(item.Status) == HRISWebhookExecutionStatusQueued && !item.QueuedAt.IsZero() {
		return item.QueuedAt.UTC()
	}
	if item.StartedAt != nil && !item.StartedAt.IsZero() {
		return item.StartedAt.UTC()
	}
	return item.QueuedAt.UTC()
}

func listClaimableHRISWebhookExecutionsLocked(
	items []HRISWebhookExecution,
	kind string,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookExecution {
	nextKind := normalizeHRISWebhookExecutionKind(kind)
	if nextKind == "" {
		return nil
	}

	filtered := make([]HRISWebhookExecution, 0, len(items))
	for i := range items {
		item := items[i]
		if normalizeHRISWebhookExecutionKind(item.Kind) != nextKind {
			continue
		}
		if hrisWebhookExecutionClaimReason(item, processingTimeout, now) != "" {
			continue
		}
		filtered = append(filtered, cloneHRISWebhookExecution(item))
	}
	sort.Slice(filtered, func(i, j int) bool {
		leftAt := hrisWebhookExecutionClaimSortTime(filtered[i])
		rightAt := hrisWebhookExecutionClaimSortTime(filtered[j])
		if leftAt.Equal(rightAt) {
			return filtered[i].ID < filtered[j].ID
		}
		return leftAt.Before(rightAt)
	})
	if limit > 0 && len(filtered) > limit {
		return filtered[:limit]
	}
	return filtered
}

func listIndexedClaimableHRISWebhookExecutionsLocked(
	allItems []HRISWebhookExecution,
	indexedQueued []HRISWebhookExecution,
	kind string,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookExecution {
	dueQueued := listDueQueuedHRISWebhookExecutionsFromIndex(indexedQueued, now, limit)
	staleRunning := listStaleRunningHRISWebhookExecutionsLocked(
		allItems,
		kind,
		processingTimeout,
		now,
		limit,
	)
	if len(dueQueued) == 0 {
		if limit > 0 && len(staleRunning) > limit {
			return staleRunning[:limit]
		}
		return staleRunning
	}
	if len(staleRunning) == 0 {
		if limit > 0 && len(dueQueued) > limit {
			return dueQueued[:limit]
		}
		return dueQueued
	}

	merged := make([]HRISWebhookExecution, 0, len(dueQueued)+len(staleRunning))
	leftIndex := 0
	rightIndex := 0
	for leftIndex < len(dueQueued) && rightIndex < len(staleRunning) {
		leftAt := hrisWebhookExecutionClaimSortTime(dueQueued[leftIndex])
		rightAt := hrisWebhookExecutionClaimSortTime(staleRunning[rightIndex])
		if leftAt.Before(rightAt) || (leftAt.Equal(rightAt) && dueQueued[leftIndex].ID < staleRunning[rightIndex].ID) {
			merged = append(merged, dueQueued[leftIndex])
			leftIndex++
		} else {
			merged = append(merged, staleRunning[rightIndex])
			rightIndex++
		}
		if limit > 0 && len(merged) >= limit {
			return merged[:limit]
		}
	}
	for leftIndex < len(dueQueued) {
		merged = append(merged, dueQueued[leftIndex])
		leftIndex++
		if limit > 0 && len(merged) >= limit {
			return merged[:limit]
		}
	}
	for rightIndex < len(staleRunning) {
		merged = append(merged, staleRunning[rightIndex])
		rightIndex++
		if limit > 0 && len(merged) >= limit {
			return merged[:limit]
		}
	}
	return merged
}

func listDueQueuedHRISWebhookExecutionsFromIndex(
	indexedQueued []HRISWebhookExecution,
	now time.Time,
	limit int,
) []HRISWebhookExecution {
	if len(indexedQueued) == 0 {
		return nil
	}
	items := make([]HRISWebhookExecution, 0, len(indexedQueued))
	for i := range indexedQueued {
		if !indexedQueued[i].QueuedAt.IsZero() && indexedQueued[i].QueuedAt.UTC().After(now) {
			break
		}
		items = append(items, cloneHRISWebhookExecution(indexedQueued[i]))
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items
}

func listStaleRunningHRISWebhookExecutionsLocked(
	items []HRISWebhookExecution,
	kind string,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookExecution {
	nextKind := normalizeHRISWebhookExecutionKind(kind)
	if nextKind == "" {
		return nil
	}

	filtered := make([]HRISWebhookExecution, 0, len(items))
	for i := range items {
		item := items[i]
		if normalizeHRISWebhookExecutionKind(item.Kind) != nextKind {
			continue
		}
		if strings.TrimSpace(item.Status) != HRISWebhookExecutionStatusRunning {
			continue
		}
		if hrisWebhookExecutionClaimReason(item, processingTimeout, now) != "" {
			continue
		}
		filtered = append(filtered, cloneHRISWebhookExecution(item))
	}
	sort.Slice(filtered, func(i, j int) bool {
		leftAt := hrisWebhookExecutionClaimSortTime(filtered[i])
		rightAt := hrisWebhookExecutionClaimSortTime(filtered[j])
		if leftAt.Equal(rightAt) {
			return filtered[i].ID < filtered[j].ID
		}
		return leftAt.Before(rightAt)
	})
	if limit > 0 && len(filtered) > limit {
		return filtered[:limit]
	}
	return filtered
}

func buildQueuedHRISWebhookExecutionIDs(
	items []HRISWebhookExecution,
	kind string,
) []string {
	nextKind := normalizeHRISWebhookExecutionKind(kind)
	filtered := make([]HRISWebhookExecution, 0, len(items))
	for i := range items {
		item := items[i]
		if normalizeHRISWebhookExecutionKind(item.Kind) != nextKind {
			continue
		}
		if !isQueuedHRISWebhookExecutionCandidate(item) {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].QueuedAt.Equal(filtered[j].QueuedAt) {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].QueuedAt.Before(filtered[j].QueuedAt)
	})

	result := make([]string, 0, len(filtered))
	for i := range filtered {
		if strings.TrimSpace(filtered[i].ID) == "" {
			continue
		}
		result = append(result, filtered[i].ID)
	}
	return result
}

func (s *Service) GetHRISWebhookExecution(tenantID, executionID string) (HRISWebhookExecution, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISWebhookExecution{}, ErrTenantIDRequired
	}
	nextExecutionID := strings.TrimSpace(executionID)
	if nextExecutionID == "" {
		return HRISWebhookExecution{}, ErrHRISWebhookExecutionNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.hrisWebhookExecutions {
		item := s.hrisWebhookExecutions[i]
		if strings.TrimSpace(item.ID) != nextExecutionID {
			continue
		}
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			return HRISWebhookExecution{}, ErrHRISWebhookExecutionNotFound
		}
		return cloneHRISWebhookExecution(item), nil
	}
	return HRISWebhookExecution{}, ErrHRISWebhookExecutionNotFound
}

func (s *Service) GetHRISWebhookExecutionByID(executionID string) (HRISWebhookExecution, error) {
	nextExecutionID := strings.TrimSpace(executionID)
	if nextExecutionID == "" {
		return HRISWebhookExecution{}, ErrHRISWebhookExecutionNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := findHRISWebhookExecutionByIDLocked(s.hrisWebhookExecutions, nextExecutionID)
	if !ok {
		return HRISWebhookExecution{}, ErrHRISWebhookExecutionNotFound
	}
	return cloneHRISWebhookExecution(item), nil
}

func (s *Service) UpdateHRISWebhookExecutionDispatchMode(tenantID, executionID, dispatchMode string) (HRISWebhookExecution, error) {
	nextDispatchMode := normalizeHRISWebhookExecutionDispatchMode(dispatchMode)
	if nextDispatchMode == "" {
		return HRISWebhookExecution{}, ErrInvalidHRISWebhookExecutionDispatchMode
	}
	return s.updateHRISWebhookExecution(
		tenantID,
		executionID,
		func(item *HRISWebhookExecution) {
			item.DispatchMode = nextDispatchMode
		},
	)
}

func (s *Service) MarkHRISWebhookExecutionRunning(tenantID, executionID string) (HRISWebhookExecution, error) {
	now := time.Now().UTC()
	return s.updateHRISWebhookExecution(
		tenantID,
		executionID,
		func(item *HRISWebhookExecution) {
			item.Status = HRISWebhookExecutionStatusRunning
			item.StartedAt = &now
			item.FinishedAt = nil
			item.LastError = ""
			item.AttemptCount++
		},
	)
}

func (s *Service) ClaimHRISWebhookExecution(
	tenantID string,
	executionID string,
	processingTimeout time.Duration,
	now time.Time,
) (HRISWebhookExecution, string, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISWebhookExecution{}, "", ErrTenantIDRequired
	}
	nextExecutionID := strings.TrimSpace(executionID)
	if nextExecutionID == "" {
		return HRISWebhookExecution{}, "", ErrHRISWebhookExecutionNotFound
	}
	processingTimeout, now = normalizeHRISWebhookExecutionClaimParams(processingTimeout, now)

	s.mu.Lock()
	defer s.mu.Unlock()

	var claimed HRISWebhookExecution
	claimReason := ""
	if err := s.mutateHRISWebhookStateLocked(func() (bool, error) {
		for i := range s.hrisWebhookExecutions {
			if strings.TrimSpace(s.hrisWebhookExecutions[i].ID) != nextExecutionID {
				continue
			}
			if strings.TrimSpace(s.hrisWebhookExecutions[i].TenantID) != nextTenantID {
				return false, ErrHRISWebhookExecutionNotFound
			}
			if reason := hrisWebhookExecutionClaimReason(s.hrisWebhookExecutions[i], processingTimeout, now); reason != "" {
				claimed = cloneHRISWebhookExecution(s.hrisWebhookExecutions[i])
				claimReason = reason
				return false, nil
			}
			s.hrisWebhookExecutions[i].Status = HRISWebhookExecutionStatusRunning
			s.hrisWebhookExecutions[i].StartedAt = &now
			s.hrisWebhookExecutions[i].FinishedAt = nil
			s.hrisWebhookExecutions[i].LastError = ""
			s.hrisWebhookExecutions[i].AttemptCount++
			s.hrisWebhookExecutions[i].UpdatedAt = now
			claimed = cloneHRISWebhookExecution(s.hrisWebhookExecutions[i])
			claimReason = ""
			return true, nil
		}
		return false, ErrHRISWebhookExecutionNotFound
	}); err != nil {
		return HRISWebhookExecution{}, "", err
	}
	if claimReason != "" {
		return claimed, claimReason, nil
	}
	if claimed.ID == "" {
		return HRISWebhookExecution{}, "", ErrHRISWebhookExecutionNotFound
	}
	return claimed, "", nil
}

func (s *Service) RequeueHRISWebhookExecution(
	tenantID string,
	executionID string,
	targetStatus string,
	retryAt time.Time,
	failure error,
) (HRISWebhookExecution, error) {
	now := time.Now().UTC()
	nextRetryAt := now
	if !retryAt.IsZero() {
		nextRetryAt = retryAt.UTC()
	}
	message := ""
	if failure != nil {
		message = failure.Error()
	}
	nextTargetStatus := strings.TrimSpace(targetStatus)
	return s.updateHRISWebhookExecution(
		tenantID,
		executionID,
		func(item *HRISWebhookExecution) {
			item.Status = HRISWebhookExecutionStatusQueued
			item.TargetStatus = nextTargetStatus
			item.LastError = message
			item.RequeueCount++
			item.QueuedAt = nextRetryAt
			item.StartedAt = nil
			item.FinishedAt = nil
		},
	)
}

func (s *Service) AcknowledgeHRISWebhookExecution(
	tenantID string,
	executionID string,
	targetStatus string,
	failure error,
) (HRISWebhookExecution, error) {
	if failure != nil {
		return s.MarkHRISWebhookExecutionFailed(tenantID, executionID, targetStatus, failure)
	}
	return s.MarkHRISWebhookExecutionSucceeded(tenantID, executionID, targetStatus)
}

func (s *Service) MarkHRISWebhookExecutionSucceeded(tenantID, executionID, targetStatus string) (HRISWebhookExecution, error) {
	now := time.Now().UTC()
	nextTargetStatus := strings.TrimSpace(targetStatus)
	return s.updateHRISWebhookExecution(
		tenantID,
		executionID,
		func(item *HRISWebhookExecution) {
			item.Status = HRISWebhookExecutionStatusSucceeded
			item.TargetStatus = nextTargetStatus
			item.LastError = ""
			if item.StartedAt == nil {
				item.StartedAt = &now
			}
			item.FinishedAt = &now
		},
	)
}

func (s *Service) MarkHRISWebhookExecutionFailed(
	tenantID string,
	executionID string,
	targetStatus string,
	failure error,
) (HRISWebhookExecution, error) {
	now := time.Now().UTC()
	message := ""
	if failure != nil {
		message = failure.Error()
	}
	nextTargetStatus := strings.TrimSpace(targetStatus)
	return s.updateHRISWebhookExecution(
		tenantID,
		executionID,
		func(item *HRISWebhookExecution) {
			item.Status = HRISWebhookExecutionStatusFailed
			item.TargetStatus = nextTargetStatus
			item.LastError = message
			if item.StartedAt == nil {
				item.StartedAt = &now
			}
			item.FinishedAt = &now
		},
	)
}

func (s *Service) updateHRISWebhookExecution(
	tenantID string,
	executionID string,
	mutate func(item *HRISWebhookExecution),
) (HRISWebhookExecution, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISWebhookExecution{}, ErrTenantIDRequired
	}
	nextExecutionID := strings.TrimSpace(executionID)
	if nextExecutionID == "" {
		return HRISWebhookExecution{}, ErrHRISWebhookExecutionNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var updated HRISWebhookExecution
	if err := s.mutateHRISWebhookStateLocked(func() (bool, error) {
		for i := range s.hrisWebhookExecutions {
			if strings.TrimSpace(s.hrisWebhookExecutions[i].ID) != nextExecutionID {
				continue
			}
			if strings.TrimSpace(s.hrisWebhookExecutions[i].TenantID) != nextTenantID {
				return false, ErrHRISWebhookExecutionNotFound
			}
			if mutate != nil {
				mutate(&s.hrisWebhookExecutions[i])
			}
			s.hrisWebhookExecutions[i].UpdatedAt = time.Now().UTC()
			updated = cloneHRISWebhookExecution(s.hrisWebhookExecutions[i])
			return true, nil
		}
		return false, ErrHRISWebhookExecutionNotFound
	}); err != nil {
		return HRISWebhookExecution{}, err
	}
	return updated, nil
}

func normalizeHRISWebhookExecutionKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case HRISWebhookExecutionKindReceiptProcess:
		return HRISWebhookExecutionKindReceiptProcess
	case HRISWebhookExecutionKindDLQReplay:
		return HRISWebhookExecutionKindDLQReplay
	default:
		return ""
	}
}

func normalizeHRISWebhookExecutionDispatchMode(dispatchMode string) string {
	switch strings.ToLower(strings.TrimSpace(dispatchMode)) {
	case HRISWebhookExecutionDispatchModeWorkerTick:
		return HRISWebhookExecutionDispatchModeWorkerTick
	case HRISWebhookExecutionDispatchModeWorkerTaskChannel:
		return HRISWebhookExecutionDispatchModeWorkerTaskChannel
	case HRISWebhookExecutionDispatchModeGoroutineFallback:
		return HRISWebhookExecutionDispatchModeGoroutineFallback
	default:
		return ""
	}
}

func findActiveHRISWebhookReplayExecutionLocked(
	items []HRISWebhookExecution,
	tenantID string,
	sourceExecutionID string,
) (HRISWebhookExecution, bool) {
	nextTenantID := strings.TrimSpace(tenantID)
	nextSourceExecutionID := strings.TrimSpace(sourceExecutionID)
	if nextTenantID == "" || nextSourceExecutionID == "" {
		return HRISWebhookExecution{}, false
	}
	for i := range items {
		item := items[i]
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		if strings.TrimSpace(item.ReplaySourceExecutionID) != nextSourceExecutionID {
			continue
		}
		switch strings.TrimSpace(item.Status) {
		case HRISWebhookExecutionStatusQueued, HRISWebhookExecutionStatusRunning:
			return item, true
		}
	}
	return HRISWebhookExecution{}, false
}

func (s *Service) ResolveTenantByEmail(email string) (TenantResolution, error) {
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return TenantResolution{}, ErrEmailRequired
	}

	domain, err := emailDomain(nextEmail)
	if err != nil {
		return TenantResolution{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	bestTenantID := ""
	bestDomain := ""
	for i := range s.domainMappings {
		if s.domainMappings[i].Status != "active" {
			continue
		}
		mappedDomain := s.domainMappings[i].Domain
		if !domainMatches(domain, mappedDomain) {
			continue
		}
		if len(mappedDomain) <= len(bestDomain) {
			continue
		}
		bestDomain = mappedDomain
		bestTenantID = s.domainMappings[i].TenantID
	}

	if bestTenantID != "" {
		return TenantResolution{
			Email:    nextEmail,
			Domain:   domain,
			TenantID: bestTenantID,
			Matched:  true,
		}, nil
	}

	return TenantResolution{}, ErrDomainNotMapped
}

