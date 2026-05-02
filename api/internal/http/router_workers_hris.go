package httpx

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/hris"
	"github.com/mistypass/cloud/api/internal/modules/wallet"
	"github.com/mistypass/cloud/api/internal/redistore"
)

type enterpriseJITApprovalExternalSyncWorkerResult struct {
	Processed             int
	Synced                int
	Failed                int
	SkippedByAttemptLimit int
	SkippedByCooldown     int
}

type enterpriseSyncWorkerAlertAutoRetryWorkerResult struct {
	Processed  int
	Retried    int
	Failed     int
	Skipped    int
	Suppressed int
}

type enterpriseHRISWebhookReceiptWorkerResult struct {
	Processed             int
	Synced                int
	Skipped               int
	Failed                int
	SkippedByInFlight     int
	SkippedByAttemptLimit int
	SkippedByCooldown     int
	LastConnectorID       string
	LastVendor            string
	LastRequestID         string
	LastEventType         string
}

type enterpriseHRISWebhookDLQWorkerResult struct {
	Processed             int
	Replayed              int
	Failed                int
	SkippedByInFlight     int
	SkippedByAttemptLimit int
	SkippedByCooldown     int
	LastConnectorID       string
	LastVendor            string
	LastRequestID         string
	LastEventType         string
	LastFailureStage      string
}

type enterpriseHRISPullWorkerResult struct {
	Processed             int
	Synced                int
	Failed                int
	ConsecutiveFailures   int
	FailureAgeSeconds     int
	SkippedByInFlight     int
	SkippedByAttemptLimit int
	SkippedByCooldown     int
	LastConnectorID       string
	LastVendor            string
	LastMode              string
}

type enterpriseHRISWebhookQueuedExecution struct {
	Execution  enterprise.HRISWebhookExecution
	QueueName  string
	QueueClaim *redistore.WorkerQueueClaim
}

func enterpriseHRISWebhookExecutionQueueName(kind string) string {
	switch strings.TrimSpace(kind) {
	case enterprise.HRISWebhookExecutionKindReceiptProcess:
		return enterpriseHRISWebhookReceiptExecutionQueue
	case enterprise.HRISWebhookExecutionKindDLQReplay:
		return enterpriseHRISWebhookDLQExecutionQueue
	default:
		return ""
	}
}

func enterpriseJITApprovalExternalSyncCandidate(item enterprise.JITProvisionApproval) bool {
	status := strings.TrimSpace(item.Status)
	if status != "approved" && status != "rejected" {
		return false
	}
	syncStatus := strings.TrimSpace(item.ExternalSyncStatus)
	if syncStatus == "" {
		syncStatus = "pending"
	}
	return syncStatus == "pending" || syncStatus == "failed"
}

func (s *server) listQueuedEnterpriseHRISWebhookExecutions(
	kind string,
	batchSize int,
	processingTimeout time.Duration,
	now time.Time,
) []enterpriseHRISWebhookQueuedExecution {
	if s == nil || s.enterpriseSvc == nil {
		return nil
	}
	if batchSize <= 0 {
		batchSize = 1
	}
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	items := make([]enterpriseHRISWebhookQueuedExecution, 0, batchSize)
	seen := make(map[string]struct{}, batchSize)
	if s.workerQueueStore != nil {
		queueName := enterpriseHRISWebhookExecutionQueueName(kind)
		if queueName != "" {
			claims, err := s.workerQueueStore.ClaimWorkerQueueBatch(queueName, batchSize, processingTimeout)
			if err != nil {
				s.loggerOrDefault().Error(
					"enterprise hris webhook execution queue claim failed",
					"kind", kind,
					"queue", queueName,
					"batch_size", batchSize,
					"visibility_timeout", processingTimeout,
					"err", err,
				)
			} else {
				for i := range claims {
					item, ok := s.getQueuedEnterpriseHRISWebhookExecutionCandidate(kind, claims[i].ItemID)
					if !ok {
						s.acknowledgeEnterpriseHRISWebhookExecutionQueueClaim(
							queueName,
							kind,
							claims[i],
						)
						continue
					}
					if _, exists := seen[item.ID]; exists {
						s.acknowledgeEnterpriseHRISWebhookExecutionQueueClaim(
							queueName,
							kind,
							claims[i],
						)
						continue
					}
					seen[item.ID] = struct{}{}
					claim := claims[i]
					items = append(items, enterpriseHRISWebhookQueuedExecution{
						Execution:  item,
						QueueName:  queueName,
						QueueClaim: &claim,
					})
					if len(items) >= batchSize {
						return items
					}
				}
			}
		}
	}

	fallbackItems := s.enterpriseSvc.ListIndexedClaimableHRISWebhookExecutions(
		kind,
		processingTimeout,
		now,
		batchSize,
	)
	fallbackItems = s.filterIndexedEnterpriseHRISWebhookExecutionFallbackItems(kind, fallbackItems)
	for i := range fallbackItems {
		if _, exists := seen[fallbackItems[i].ID]; exists {
			continue
		}
		seen[fallbackItems[i].ID] = struct{}{}
		items = append(items, enterpriseHRISWebhookQueuedExecution{Execution: fallbackItems[i]})
		if len(items) >= batchSize {
			break
		}
	}
	return items
}

func (s *server) filterIndexedEnterpriseHRISWebhookExecutionFallbackItems(
	kind string,
	items []enterprise.HRISWebhookExecution,
) []enterprise.HRISWebhookExecution {
	if s == nil || s.workerQueueStore == nil || len(items) == 0 {
		return items
	}
	queueName := enterpriseHRISWebhookExecutionQueueName(kind)
	if queueName == "" {
		return items
	}

	queuedIDs := make([]string, 0, len(items))
	for i := range items {
		if strings.TrimSpace(items[i].Status) != enterprise.HRISWebhookExecutionStatusQueued {
			continue
		}
		queuedIDs = append(queuedIDs, items[i].ID)
	}
	if len(queuedIDs) == 0 {
		return items
	}

	telemetry, err := s.workerQueueStore.DescribeWorkerQueue(queueName, queuedIDs)
	if err != nil {
		s.loggerOrDefault().Warn(
			"describe indexed enterprise hris webhook execution fallback items failed",
			"kind", kind,
			"queue", queueName,
			"err", err,
		)
		return items
	}

	filtered := make([]enterprise.HRISWebhookExecution, 0, len(items))
	for i := range items {
		item := items[i]
		if strings.TrimSpace(item.Status) != enterprise.HRISWebhookExecutionStatusQueued {
			filtered = append(filtered, item)
			continue
		}
		state, ok := telemetry.Items[item.ID]
		if !ok || strings.TrimSpace(state.State) == "" || strings.TrimSpace(state.State) == redistore.WorkerQueueStateMissing {
			filtered = append(filtered, item)
			continue
		}
	}
	return filtered
}

func (s *server) getQueuedEnterpriseHRISWebhookExecutionCandidate(
	kind string,
	executionID string,
) (enterprise.HRISWebhookExecution, bool) {
	item, ok := s.lookupQueuedEnterpriseHRISWebhookExecutionCandidate(kind, executionID)
	if ok {
		return item, true
	}
	if s == nil || s.enterpriseSvc == nil {
		return enterprise.HRISWebhookExecution{}, false
	}
	if err := s.enterpriseSvc.RefreshCoreState(); err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook execution shared state refresh after external queue miss failed",
			"kind", kind,
			"execution_id", executionID,
			"err", err,
		)
		return enterprise.HRISWebhookExecution{}, false
	}
	return s.lookupQueuedEnterpriseHRISWebhookExecutionCandidate(kind, executionID)
}

func (s *server) lookupQueuedEnterpriseHRISWebhookExecutionCandidate(
	kind string,
	executionID string,
) (enterprise.HRISWebhookExecution, bool) {
	if s == nil || s.enterpriseSvc == nil {
		return enterprise.HRISWebhookExecution{}, false
	}
	item, err := s.enterpriseSvc.GetHRISWebhookExecutionByID(executionID)
	if err != nil {
		return enterprise.HRISWebhookExecution{}, false
	}
	if strings.TrimSpace(item.Kind) != strings.TrimSpace(kind) {
		return enterprise.HRISWebhookExecution{}, false
	}
	if strings.TrimSpace(item.ExecutionMode) != enterpriseExecutionModeQueued {
		return enterprise.HRISWebhookExecution{}, false
	}
	switch strings.TrimSpace(item.Status) {
	case enterprise.HRISWebhookExecutionStatusQueued, enterprise.HRISWebhookExecutionStatusRunning:
	default:
		return enterprise.HRISWebhookExecution{}, false
	}
	if strings.TrimSpace(item.DispatchMode) != enterprise.HRISWebhookExecutionDispatchModeWorkerTick {
		return enterprise.HRISWebhookExecution{}, false
	}
	if strings.TrimSpace(item.TargetID) == "" {
		return enterprise.HRISWebhookExecution{}, false
	}
	return item, true
}

func (s *server) refreshEnterpriseHRISWebhookReceiptWorkerState() error {
	if s == nil {
		return nil
	}
	if s.enterpriseSvc != nil {
		if err := s.enterpriseSvc.RefreshCoreState(); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) refreshEnterpriseHRISWebhookDLQWorkerState() error {
	if s == nil {
		return nil
	}
	if s.enterpriseSvc != nil {
		if err := s.enterpriseSvc.RefreshCoreState(); err != nil {
			return err
		}
	}
	if s.hrisDLQSvc != nil {
		if err := s.hrisDLQSvc.RefreshState(); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) refreshEnterpriseHRISWebhookExecutionTargetSharedState(
	kind string,
	tenantID string,
	executionID string,
	targetID string,
) {
	if s == nil {
		return
	}

	var err error
	switch strings.TrimSpace(kind) {
	case enterprise.HRISWebhookExecutionKindReceiptProcess:
		err = s.refreshEnterpriseHRISWebhookReceiptWorkerState()
	case enterprise.HRISWebhookExecutionKindDLQReplay:
		err = s.refreshEnterpriseHRISWebhookDLQWorkerState()
	default:
		return
	}
	if err != nil {
		s.loggerOrDefault().Warn(
			"enterprise hris webhook execution target shared state refresh failed",
			"kind", kind,
			"tenant_id", tenantID,
			"execution_id", executionID,
			"target_id", targetID,
			"err", err,
		)
	}
}

func (s *server) refreshEnterpriseHRISPullWorkerState() error {
	if s == nil {
		return nil
	}
	if s.enterpriseSvc != nil {
		if err := s.enterpriseSvc.RefreshCoreState(); err != nil {
			return err
		}
	}
	if s.hrisPullStateSvc != nil {
		if err := s.hrisPullStateSvc.RefreshState(); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) runQueuedEnterpriseHRISWebhookReceiptExecutions(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
) int {
	if batchSize <= 0 || s == nil || s.enterpriseSvc == nil {
		return 0
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	now := time.Now().UTC()
	items := s.listQueuedEnterpriseHRISWebhookExecutions(
		enterprise.HRISWebhookExecutionKindReceiptProcess,
		batchSize,
		processingTimeout,
		now,
	)
	processed := 0
	for i := range items {
		queuedItem := items[i]
		execution := queuedItem.Execution
		originalExecutionStatus := strings.TrimSpace(execution.Status)
		claimed, claimReason, err := s.enterpriseSvc.ClaimHRISWebhookExecution(
			execution.TenantID,
			execution.ID,
			processingTimeout,
			now,
		)
		if err != nil {
			s.loggerOrDefault().Error(
				"enterprise hris webhook receipt queued execution claim failed",
				"tenant_id", execution.TenantID,
				"execution_id", execution.ID,
				"receipt_id", execution.TargetID,
				"err", err,
			)
			continue
		}
		if claimReason != "" {
			s.handleQueuedEnterpriseHRISWebhookExecutionClaimSkip(queuedItem, claimed, claimReason)
			continue
		}
		execution = claimed
		s.refreshEnterpriseHRISWebhookExecutionTargetSharedState(
			execution.Kind,
			execution.TenantID,
			execution.ID,
			execution.TargetID,
		)

		receipt, err := s.enterpriseSvc.GetHRISWebhookReceipt(execution.TenantID, execution.TargetID)
		if err != nil {
			_, _ = s.enterpriseSvc.AcknowledgeHRISWebhookExecution(execution.TenantID, execution.ID, "", err)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		}

		requeued, err := s.requeueEnterpriseHRISWebhookReceiptExecutionForFreshTarget(
			queuedItem,
			execution,
			receipt,
			originalExecutionStatus,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			processingTimeout,
			now,
		)
		if err != nil {
			_, _ = s.enterpriseSvc.AcknowledgeHRISWebhookExecution(execution.TenantID, execution.ID, strings.TrimSpace(receipt.Status), err)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		}
		if requeued {
			processed++
			continue
		}
		switch strings.TrimSpace(receipt.Status) {
		case "processed", "skipped":
			s.completeHRISWebhookReceiptExecution(receipt, execution.ID, nil)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		case "failed", "dlq":
			s.completeHRISWebhookReceiptExecution(
				receipt,
				execution.ID,
				errors.New(firstNonEmptyString(receipt.LastError, receipt.Status)),
			)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		}

		recordDLQ := receipt.AttemptCount >= maxAttempts
		s.completeHRISWebhookReceiptExecution(
			receipt,
			execution.ID,
			s.processEnterpriseHRISWebhookReceipt(nil, receipt, recordDLQ),
		)
		s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
		processed++
	}
	return processed
}

func (s *server) runQueuedEnterpriseHRISWebhookDLQExecutions(batchSize int) int {
	return s.runQueuedEnterpriseHRISWebhookDLQExecutionsWithRetryBackoffAndProcessingTimeout(
		batchSize,
		1,
		0,
		0,
		5*time.Minute,
	)
}

func (s *server) runQueuedEnterpriseHRISWebhookDLQExecutionsWithRetryBackoffAndProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
) int {
	if batchSize <= 0 || s == nil || s.enterpriseSvc == nil || s.hrisDLQSvc == nil {
		return 0
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	now := time.Now().UTC()
	items := s.listQueuedEnterpriseHRISWebhookExecutions(
		enterprise.HRISWebhookExecutionKindDLQReplay,
		batchSize,
		processingTimeout,
		now,
	)
	processed := 0
	for i := range items {
		queuedItem := items[i]
		execution := queuedItem.Execution
		originalExecutionStatus := strings.TrimSpace(execution.Status)
		claimed, claimReason, err := s.enterpriseSvc.ClaimHRISWebhookExecution(
			execution.TenantID,
			execution.ID,
			processingTimeout,
			now,
		)
		if err != nil {
			s.loggerOrDefault().Error(
				"enterprise hris webhook dlq queued execution claim failed",
				"tenant_id", execution.TenantID,
				"execution_id", execution.ID,
				"entry_id", execution.TargetID,
				"err", err,
			)
			continue
		}
		if claimReason != "" {
			s.handleQueuedEnterpriseHRISWebhookExecutionClaimSkip(queuedItem, claimed, claimReason)
			continue
		}
		execution = claimed
		s.refreshEnterpriseHRISWebhookExecutionTargetSharedState(
			execution.Kind,
			execution.TenantID,
			execution.ID,
			execution.TargetID,
		)

		entry, err := s.hrisDLQSvc.GetEntry(execution.TargetID)
		if err != nil {
			_, _ = s.enterpriseSvc.AcknowledgeHRISWebhookExecution(execution.TenantID, execution.ID, "", err)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		}

		requeued, err := s.requeueEnterpriseHRISWebhookDLQExecutionForFreshTarget(
			queuedItem,
			execution,
			entry,
			originalExecutionStatus,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			processingTimeout,
			now,
		)
		if err != nil {
			_, _ = s.enterpriseSvc.AcknowledgeHRISWebhookExecution(execution.TenantID, execution.ID, strings.TrimSpace(entry.Status), err)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		}
		if requeued {
			processed++
			continue
		}
		switch strings.TrimSpace(entry.Status) {
		case "resolved":
			s.completeHRISWebhookDLQExecutionSuccess(execution.TenantID, entry, execution.ID)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		case "replaying":
			// Continue below and let the current worker replay the claimed entry.
		default:
			s.completeHRISWebhookDLQExecution(
				execution.TenantID,
				entry,
				execution.ID,
				fmt.Errorf("queued dlq execution target is no longer replaying: %s", entry.Status),
			)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		}

		updated, err := s.replayEnterpriseHRISWebhookDLQClaimedEntry(
			nil,
			execution.TenantID,
			entry,
			firstNonEmptyString(execution.AuditSource, "enterprise_sync_worker"),
		)
		if err != nil {
			s.completeHRISWebhookDLQExecution(execution.TenantID, entry, execution.ID, err)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		}
		s.completeHRISWebhookDLQExecutionSuccess(execution.TenantID, updated, execution.ID)
		s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
		processed++
	}
	return processed
}

func (s *server) requeueEnterpriseHRISWebhookReceiptExecutionForFreshTarget(
	item enterpriseHRISWebhookQueuedExecution,
	execution enterprise.HRISWebhookExecution,
	receipt enterprise.HRISWebhookReceipt,
	originalExecutionStatus string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) (bool, error) {
	if s == nil || s.enterpriseSvc == nil {
		return false, nil
	}
	if originalExecutionStatus != enterprise.HRISWebhookExecutionStatusRunning {
		return false, nil
	}
	runtime := describeHRISWebhookReceiptQueueState(
		receipt,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
	)
	if runtime.State != enterprise.HRISWebhookReceiptClaimReasonInFlight || runtime.ProcessingDeadlineAt == nil {
		return false, nil
	}
	requeued, err := s.enterpriseSvc.RequeueHRISWebhookExecution(
		execution.TenantID,
		execution.ID,
		strings.TrimSpace(receipt.Status),
		*runtime.ProcessingDeadlineAt,
		nil,
	)
	if err != nil {
		return false, err
	}
	s.requeueQueuedEnterpriseHRISWebhookExecution(item, requeued)
	return true, nil
}

func (s *server) reenqueueEnterpriseHRISWebhookExecution(
	execution enterprise.HRISWebhookExecution,
) {
	if s == nil {
		return
	}
	if strings.TrimSpace(execution.Status) != enterprise.HRISWebhookExecutionStatusQueued {
		return
	}
	if strings.TrimSpace(execution.ExecutionMode) != enterpriseExecutionModeQueued {
		return
	}
	if strings.TrimSpace(execution.DispatchMode) != enterprise.HRISWebhookExecutionDispatchModeWorkerTick {
		return
	}
	queueName := enterpriseHRISWebhookExecutionQueueName(execution.Kind)
	if queueName == "" {
		return
	}
	s.enqueueEnterpriseHRISWebhookExecution(
		queueName,
		execution.ID,
		execution.TenantID,
		execution.Kind,
	)
}

func (s *server) reenqueueEnterpriseHRISWebhookExecutionOnCooldown(
	execution enterprise.HRISWebhookExecution,
	claimReason string,
) {
	if s == nil || strings.TrimSpace(claimReason) != enterprise.HRISWebhookExecutionClaimReasonCooldown {
		return
	}
	s.reenqueueEnterpriseHRISWebhookExecution(execution)
}

func (s *server) acknowledgeEnterpriseHRISWebhookExecutionQueueClaim(
	queueName string,
	kind string,
	claim redistore.WorkerQueueClaim,
) bool {
	if s == nil || s.workerQueueStore == nil {
		return false
	}
	nextQueueName := strings.TrimSpace(queueName)
	nextExecutionID := strings.TrimSpace(claim.ItemID)
	nextClaimToken := strings.TrimSpace(claim.ClaimToken)
	if nextQueueName == "" || nextExecutionID == "" || nextClaimToken == "" {
		return false
	}
	applied, err := s.workerQueueStore.AckWorkerQueue(nextQueueName, nextExecutionID, nextClaimToken)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook execution queue ack failed",
			"kind", kind,
			"queue", nextQueueName,
			"execution_id", nextExecutionID,
			"err", err,
		)
		return false
	}
	if applied {
		return true
	}
	queueState, visibilityDeadlineAt := s.describeEnterpriseHRISWebhookExecutionQueueItem(nextQueueName, nextExecutionID)
	s.loggerOrDefault().Info(
		"enterprise hris webhook execution queue ack ignored stale claim",
		"kind", kind,
		"queue", nextQueueName,
		"execution_id", nextExecutionID,
		"queue_state", queueState,
		"visibility_deadline_at", visibilityDeadlineAt,
	)
	return false
}

func (s *server) acknowledgeQueuedEnterpriseHRISWebhookExecution(
	item enterpriseHRISWebhookQueuedExecution,
) bool {
	if item.QueueClaim == nil {
		return false
	}
	return s.acknowledgeEnterpriseHRISWebhookExecutionQueueClaim(
		item.QueueName,
		item.Execution.Kind,
		*item.QueueClaim,
	)
}

func (s *server) requeueEnterpriseHRISWebhookExecutionQueueClaim(
	queueName string,
	kind string,
	claim redistore.WorkerQueueClaim,
) bool {
	if s == nil || s.workerQueueStore == nil {
		return false
	}
	nextQueueName := strings.TrimSpace(queueName)
	nextExecutionID := strings.TrimSpace(claim.ItemID)
	nextClaimToken := strings.TrimSpace(claim.ClaimToken)
	if nextQueueName == "" || nextExecutionID == "" || nextClaimToken == "" {
		return false
	}
	applied, err := s.workerQueueStore.RequeueWorkerQueue(nextQueueName, nextExecutionID, nextClaimToken)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook execution queue requeue failed",
			"kind", kind,
			"queue", nextQueueName,
			"execution_id", nextExecutionID,
			"err", err,
		)
		return false
	}
	if applied {
		return true
	}
	queueState, visibilityDeadlineAt := s.describeEnterpriseHRISWebhookExecutionQueueItem(nextQueueName, nextExecutionID)
	s.loggerOrDefault().Warn(
		"enterprise hris webhook execution queue requeue missed active claim; falling back to enqueue",
		"kind", kind,
		"queue", nextQueueName,
		"execution_id", nextExecutionID,
		"queue_state", queueState,
		"visibility_deadline_at", visibilityDeadlineAt,
	)
	return false
}

func (s *server) requeueQueuedEnterpriseHRISWebhookExecution(
	item enterpriseHRISWebhookQueuedExecution,
	execution enterprise.HRISWebhookExecution,
) {
	if item.QueueClaim != nil && s.requeueEnterpriseHRISWebhookExecutionQueueClaim(
		item.QueueName,
		firstNonEmptyString(item.Execution.Kind, execution.Kind),
		*item.QueueClaim,
	) {
		return
	}
	s.reenqueueEnterpriseHRISWebhookExecution(execution)
}

func (s *server) handleQueuedEnterpriseHRISWebhookExecutionClaimSkip(
	item enterpriseHRISWebhookQueuedExecution,
	execution enterprise.HRISWebhookExecution,
	claimReason string,
) {
	if strings.TrimSpace(claimReason) == enterprise.HRISWebhookExecutionClaimReasonCooldown {
		s.requeueQueuedEnterpriseHRISWebhookExecution(item, execution)
		return
	}
	s.acknowledgeQueuedEnterpriseHRISWebhookExecution(item)
}

func (s *server) describeEnterpriseHRISWebhookExecutionQueueItem(
	queueName string,
	executionID string,
) (string, string) {
	if s == nil || s.workerQueueStore == nil {
		return "", ""
	}
	nextQueueName := strings.TrimSpace(queueName)
	nextExecutionID := strings.TrimSpace(executionID)
	if nextQueueName == "" || nextExecutionID == "" {
		return "", ""
	}
	telemetry, err := s.workerQueueStore.DescribeWorkerQueue(nextQueueName, []string{nextExecutionID})
	if err != nil {
		s.loggerOrDefault().Warn(
			"describe enterprise hris webhook execution queue item failed",
			"queue", nextQueueName,
			"execution_id", nextExecutionID,
			"err", err,
		)
		return "", ""
	}
	item, ok := telemetry.Items[nextExecutionID]
	if !ok {
		return "", ""
	}
	visibilityDeadlineAt := ""
	if item.VisibilityDeadlineAt != nil {
		visibilityDeadlineAt = item.VisibilityDeadlineAt.UTC().Format(time.RFC3339)
	}
	return strings.TrimSpace(item.State), visibilityDeadlineAt
}

func (s *server) requeueEnterpriseHRISWebhookDLQExecutionForFreshTarget(
	item enterpriseHRISWebhookQueuedExecution,
	execution enterprise.HRISWebhookExecution,
	entry hris.DeadLetterEntry,
	originalExecutionStatus string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) (bool, error) {
	if s == nil || s.enterpriseSvc == nil {
		return false, nil
	}
	if originalExecutionStatus != enterprise.HRISWebhookExecutionStatusRunning {
		return false, nil
	}
	runtime := describeHRISWebhookDLQReplayState(
		entry,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
	)
	if runtime.State != hris.DLQEntryClaimReasonInFlight || runtime.ProcessingDeadlineAt == nil {
		return false, nil
	}
	requeued, err := s.enterpriseSvc.RequeueHRISWebhookExecution(
		execution.TenantID,
		execution.ID,
		strings.TrimSpace(entry.Status),
		*runtime.ProcessingDeadlineAt,
		nil,
	)
	if err != nil {
		return false, err
	}
	s.requeueQueuedEnterpriseHRISWebhookExecution(item, requeued)
	return true, nil
}

func (s *server) startEnterpriseJITApprovalExternalSyncWorker() {
	if !s.cfg.EnterpriseJITApprovalExternalSyncWorkerEnabled {
		return
	}
	interval := s.cfg.EnterpriseJITApprovalExternalSyncWorkerInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	batchSize := s.cfg.EnterpriseJITApprovalExternalSyncWorkerBatchSize
	if batchSize <= 0 {
		batchSize = 1
	}
	maxAttempts := s.cfg.EnterpriseJITApprovalExternalSyncWorkerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryCooldown := s.cfg.EnterpriseJITApprovalExternalSyncWorkerRetryCooldown
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	alertFailureThreshold := s.cfg.EnterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold
	if alertFailureThreshold <= 0 {
		alertFailureThreshold = 1
	}
	forceError := s.cfg.EnterpriseJITApprovalExternalSyncWorkerForceError
	forceErrorTenantID := strings.TrimSpace(s.cfg.EnterpriseJITApprovalExternalSyncWorkerForceErrorTenantID)

	s.loggerOrDefault().Info(
		"enterprise jit approval external sync worker enabled",
		"interval", interval,
		"batch_size", batchSize,
		"max_attempts", maxAttempts,
		"retry_cooldown", retryCooldown,
		"alert_threshold", alertFailureThreshold,
		"force_error", forceError,
		"force_error_tenant_id", forceErrorTenantID,
	)

	go func() {
		s.runEnterpriseJITApprovalExternalSyncWorkerTick(
			batchSize,
			maxAttempts,
			retryCooldown,
			alertFailureThreshold,
			forceError,
			forceErrorTenantID,
		)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.runEnterpriseJITApprovalExternalSyncWorkerTick(
				batchSize,
				maxAttempts,
				retryCooldown,
				alertFailureThreshold,
				forceError,
				forceErrorTenantID,
			)
		}
	}()
}

func (s *server) startEnterpriseHRISWebhookReceiptWorker() {
	if !s.cfg.EnterpriseHRISWebhookReceiptWorkerEnabled {
		return
	}
	interval := s.cfg.EnterpriseHRISWebhookReceiptWorkerInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	batchSize := s.cfg.EnterpriseHRISWebhookReceiptWorkerBatchSize
	if batchSize <= 0 {
		batchSize = 1
	}
	maxAttempts := s.cfg.EnterpriseHRISWebhookReceiptWorkerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryCooldown := s.cfg.EnterpriseHRISWebhookReceiptWorkerRetryCooldown
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	retryMaxBackoff := s.cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	processingTimeout := s.cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	alertFailureThreshold := s.cfg.EnterpriseHRISWebhookReceiptWorkerAlertFailureThreshold
	if alertFailureThreshold <= 0 {
		alertFailureThreshold = 1
	}
	lockTTL := s.cfg.EnterpriseHRISWebhookReceiptWorkerLockTTL
	if lockTTL <= 0 {
		lockTTL = 10 * time.Minute
	}

	s.loggerOrDefault().Info(
		"enterprise hris webhook receipt worker enabled",
		"interval", interval,
		"batch_size", batchSize,
		"max_attempts", maxAttempts,
		"retry_cooldown", retryCooldown,
		"retry_max_backoff", retryMaxBackoff,
		"processing_timeout", processingTimeout,
		"alert_threshold", alertFailureThreshold,
		"lock_ttl", lockTTL,
		"lease_enabled", s.workerLeaseStore != nil,
	)
	if s.workerLeaseStore == nil {
		s.loggerOrDefault().Warn(
			"enterprise hris webhook receipt worker running without redis lease; duplicate receipt processing remains possible in multi-instance deployments",
		)
	}

	go func() {
		s.runEnterpriseHRISWebhookReceiptWorkerTickWithLeaseAndRetryBackoff(
			batchSize,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			processingTimeout,
			alertFailureThreshold,
			lockTTL,
		)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.runEnterpriseHRISWebhookReceiptWorkerTickWithLeaseAndRetryBackoff(
					batchSize,
					maxAttempts,
					retryCooldown,
					retryMaxBackoff,
					processingTimeout,
					alertFailureThreshold,
					lockTTL,
				)
			case <-s.hrisWebhookReceiptWorkerWake:
				s.runEnterpriseHRISWebhookReceiptWorkerTickWithLeaseAndRetryBackoff(
					batchSize,
					maxAttempts,
					retryCooldown,
					retryMaxBackoff,
					processingTimeout,
					alertFailureThreshold,
					lockTTL,
				)
			case task := <-s.hrisWebhookReceiptWorkerQueue:
				s.processQueuedEnterpriseHRISWebhookReceipt(task.Receipt, task.RecordDLQ, task.ExecutionID)
			}
		}
	}()
}

func (s *server) startEnterpriseHRISWebhookDLQWorker() {
	if !s.cfg.EnterpriseHRISWebhookDLQWorkerEnabled {
		return
	}
	interval := s.cfg.EnterpriseHRISWebhookDLQWorkerInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	batchSize := s.cfg.EnterpriseHRISWebhookDLQWorkerBatchSize
	if batchSize <= 0 {
		batchSize = 1
	}
	maxAttempts := s.cfg.EnterpriseHRISWebhookDLQWorkerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryCooldown := s.cfg.EnterpriseHRISWebhookDLQWorkerRetryCooldown
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	retryMaxBackoff := s.cfg.EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	processingTimeout := s.cfg.EnterpriseHRISWebhookDLQWorkerProcessingTimeout
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	alertFailureThreshold := s.cfg.EnterpriseHRISWebhookDLQWorkerAlertFailureThreshold
	if alertFailureThreshold <= 0 {
		alertFailureThreshold = 1
	}
	lockTTL := s.cfg.EnterpriseHRISWebhookDLQWorkerLockTTL
	if lockTTL <= 0 {
		lockTTL = 10 * time.Minute
	}

	s.loggerOrDefault().Info(
		"enterprise hris webhook dlq worker enabled",
		"interval", interval,
		"batch_size", batchSize,
		"max_attempts", maxAttempts,
		"retry_cooldown", retryCooldown,
		"retry_max_backoff", retryMaxBackoff,
		"processing_timeout", processingTimeout,
		"alert_threshold", alertFailureThreshold,
		"lock_ttl", lockTTL,
		"lease_enabled", s.workerLeaseStore != nil,
	)
	if s.workerLeaseStore == nil {
		s.loggerOrDefault().Warn(
			"enterprise hris webhook dlq worker running without redis lease; duplicate dlq replays remain possible in multi-instance deployments",
		)
	}

	go func() {
		s.runEnterpriseHRISWebhookDLQWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
			batchSize,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			processingTimeout,
			alertFailureThreshold,
			lockTTL,
		)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.runEnterpriseHRISWebhookDLQWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
					batchSize,
					maxAttempts,
					retryCooldown,
					retryMaxBackoff,
					processingTimeout,
					alertFailureThreshold,
					lockTTL,
				)
			case <-s.hrisWebhookDLQWorkerWake:
				s.runEnterpriseHRISWebhookDLQWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
					batchSize,
					maxAttempts,
					retryCooldown,
					retryMaxBackoff,
					processingTimeout,
					alertFailureThreshold,
					lockTTL,
				)
			case task := <-s.hrisWebhookDLQWorkerQueue:
				s.replayQueuedEnterpriseHRISWebhookDLQEntry(task.TenantID, task.Entry, task.AuditSource, task.ExecutionID)
			}
		}
	}()
}

func (s *server) startEnterpriseHRISPullWorker() {
	if !s.cfg.EnterpriseHRISPullWorkerEnabled {
		return
	}
	interval := s.cfg.EnterpriseHRISPullWorkerInterval
	if interval <= 0 {
		interval = time.Hour
	}
	batchSize := s.cfg.EnterpriseHRISPullWorkerBatchSize
	if batchSize <= 0 {
		batchSize = 1
	}
	maxAttempts := s.cfg.EnterpriseHRISPullWorkerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryCooldown := s.cfg.EnterpriseHRISPullWorkerRetryCooldown
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	retryMaxBackoff := s.cfg.EnterpriseHRISPullWorkerRetryMaxBackoff
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	processingTimeout := s.cfg.EnterpriseHRISPullWorkerProcessingTimeout
	if processingTimeout <= 0 {
		processingTimeout = 30 * time.Minute
	}
	reconcileInterval := s.cfg.EnterpriseHRISPullWorkerReconcileInterval
	if reconcileInterval <= 0 {
		reconcileInterval = 24 * time.Hour
	}
	alertFailureThreshold := s.cfg.EnterpriseHRISPullWorkerAlertFailureThreshold
	if alertFailureThreshold <= 0 {
		alertFailureThreshold = 1
	}
	lockTTL := s.cfg.EnterpriseHRISPullWorkerLockTTL
	if lockTTL <= 0 {
		lockTTL = 10 * time.Minute
	}

	s.loggerOrDefault().Info(
		"enterprise hris pull worker enabled",
		"interval", interval,
		"batch_size", batchSize,
		"max_attempts", maxAttempts,
		"retry_cooldown", retryCooldown,
		"retry_max_backoff", retryMaxBackoff,
		"processing_timeout", processingTimeout,
		"reconcile_interval", reconcileInterval,
		"alert_threshold", alertFailureThreshold,
		"lock_ttl", lockTTL,
		"lease_enabled", s.workerLeaseStore != nil,
	)
	if s.workerLeaseStore == nil {
		s.loggerOrDefault().Warn(
			"enterprise hris pull worker running without redis lease; duplicate pull ticks remain possible in multi-instance deployments",
		)
	}

	go func() {
		s.runEnterpriseHRISPullWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
			batchSize,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			reconcileInterval,
			processingTimeout,
			alertFailureThreshold,
			lockTTL,
		)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.runEnterpriseHRISPullWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
				batchSize,
				maxAttempts,
				retryCooldown,
				retryMaxBackoff,
				reconcileInterval,
				processingTimeout,
				alertFailureThreshold,
				lockTTL,
			)
		}
	}()
}

func (s *server) runEnterpriseJITApprovalExternalSyncWorkerTick(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	alertFailureThreshold int,
	forceError bool,
	forceErrorTenantID string,
) {
	now := time.Now().UTC()
	allApprovals := s.enterpriseSvc.ListJITProvisionApprovals("", "", 0)
	tenantIDs := pendingJITApprovalExternalSyncTenantIDs(allApprovals, maxAttempts, retryCooldown, now)
	if len(tenantIDs) == 0 {
		return
	}

	for i := range tenantIDs {
		tenantID := tenantIDs[i]
		items := s.enterpriseSvc.ListJITProvisionApprovals(tenantID, "", 0)
		result := enterpriseJITApprovalExternalSyncWorkerResult{}
		for j := range items {
			if result.Processed >= batchSize {
				break
			}
			item := items[j]
			if !enterpriseJITApprovalExternalSyncCandidate(item) {
				continue
			}
			syncStatus := strings.TrimSpace(item.ExternalSyncStatus)
			if syncStatus == "failed" {
				if maxAttempts > 0 && item.ExternalSyncAttemptCount >= maxAttempts {
					result.SkippedByAttemptLimit++
					continue
				}
				if retryCooldown > 0 && item.ExternalSyncUpdatedAt != nil {
					retryReadyAt := item.ExternalSyncUpdatedAt.Add(retryCooldown)
					if retryReadyAt.After(now) {
						result.SkippedByCooldown++
						continue
					}
				}
			}

			nextStatus := "synced"
			nextRef := fmt.Sprintf("worker-auto-sync:%d", now.UnixNano())
			nextErr := ""
			if forceError {
				if forceErrorTenantID == "" || tenantID == forceErrorTenantID {
					nextStatus = "failed"
					nextRef = "worker-force-error"
					nextErr = "forced enterprise jit approval external sync worker failure"
				}
			}
			updated, err := s.enterpriseSvc.UpdateJITProvisionApprovalExternalSync(
				tenantID,
				item.ID,
				nextStatus,
				nextRef,
				nextErr,
			)
			if err != nil {
				result.Failed++
				result.Processed++
				s.loggerOrDefault().Error(
					"enterprise jit approval external sync worker failed",
					"tenant_id", tenantID,
					"approval_id", item.ID,
					"err", err,
				)
				continue
			}
			if strings.TrimSpace(updated.ExternalSyncStatus) == "synced" {
				result.Synced++
			} else {
				result.Failed++
			}
			result.Processed++
		}

		if result.Processed == 0 {
			if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 {
				s.loggerOrDefault().Info(
					"enterprise jit approval external sync worker skipped",
					"tenant_id", tenantID,
					"processed", 0,
					"skipped_attempt_limit", result.SkippedByAttemptLimit,
					"skipped_cooldown", result.SkippedByCooldown,
				)
			}
			continue
		}
		if result.Failed >= alertFailureThreshold {
			s.loggerOrDefault().Warn(
				"enterprise jit approval external sync worker alert",
				"tenant_id", tenantID,
				"failed", result.Failed,
				"threshold", alertFailureThreshold,
			)
			s.appendEnterpriseJITApprovalExternalSyncWorkerAlertAudit(tenantID, result, alertFailureThreshold)
		}
		s.loggerOrDefault().Info(
			"enterprise jit approval external sync worker finished",
			"tenant_id", tenantID,
			"processed", result.Processed,
			"synced", result.Synced,
			"failed", result.Failed,
			"skipped_attempt_limit", result.SkippedByAttemptLimit,
			"skipped_cooldown", result.SkippedByCooldown,
		)
	}
}

func (s *server) appendEnterpriseJITApprovalExternalSyncWorkerAlertAudit(
	tenantID string,
	result enterpriseJITApprovalExternalSyncWorkerResult,
	alertFailureThreshold int,
) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}
	if result.Failed < alertFailureThreshold {
		return
	}
	target := strings.TrimSpace(
		fmt.Sprintf(
			"failed=%d threshold=%d processed=%d synced=%d skipped_attempt_limit=%d skipped_cooldown=%d",
			result.Failed,
			alertFailureThreshold,
			result.Processed,
			result.Synced,
			result.SkippedByAttemptLimit,
			result.SkippedByCooldown,
		),
	)
	_, _ = s.auditSvc.Append(
		nextTenantID,
		"enterprise_sync_worker",
		"system",
		"enterprise_jit_approval_external_sync_worker_alert",
		target,
		"enterprise_sync_worker",
	)
}

func (s *server) runEnterpriseSyncReconcileWorkerTick(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	alertFailureThreshold int,
	forceError bool,
	forceErrorTenantID string,
) {
	allRecords := s.enterpriseSvc.ListSyncRequestRecords("", 0)
	tenantIDs := pendingSyncRequestTenantIDs(allRecords)
	if len(tenantIDs) == 0 {
		return
	}

	for i := range tenantIDs {
		tenantID := tenantIDs[i]
		result, err := s.enterpriseSvc.ReconcilePendingSyncRequestsWithPolicy(
			tenantID,
			batchSize,
			maxAttempts,
			retryCooldown,
			func(items []enterprise.EnterpriseEmployee) (int, int, int, error) {
				if forceError {
					if forceErrorTenantID == "" || tenantID == forceErrorTenantID {
						return 0, 0, 0, errors.New("forced enterprise sync reconcile worker apply failure")
					}
				}
				accessInputs := enterpriseEmployeesToAccessBatchInputs(items)
				return s.accessSvc.UpsertUsersByEmail(tenantID, accessInputs)
			},
		)
		if err != nil {
			s.loggerOrDefault().Error(
				"enterprise sync reconcile worker failed",
				"tenant_id", tenantID,
				"err", err,
			)
			continue
		}
		if result.Processed == 0 {
			if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 {
				s.loggerOrDefault().Info(
					"enterprise sync reconcile worker skipped",
					"tenant_id", tenantID,
					"processed", 0,
					"skipped_attempt_limit", result.SkippedByAttemptLimit,
					"skipped_cooldown", result.SkippedByCooldown,
				)
				s.appendEnterpriseSyncWorkerAlertAudit(tenantID, result, alertFailureThreshold)
			}
			continue
		}
		if result.Failed >= alertFailureThreshold {
			s.loggerOrDefault().Warn(
				"enterprise sync reconcile worker alert",
				"tenant_id", tenantID,
				"failed", result.Failed,
				"threshold", alertFailureThreshold,
			)
			s.appendEnterpriseSyncWorkerAlertAudit(tenantID, result, alertFailureThreshold)
		}
		s.loggerOrDefault().Info(
			"enterprise sync reconcile worker finished",
			"tenant_id", tenantID,
			"processed", result.Processed,
			"applied", result.Applied,
			"failed", result.Failed,
			"skipped_attempt_limit", result.SkippedByAttemptLimit,
			"skipped_cooldown", result.SkippedByCooldown,
		)
	}
}

func (s *server) runEnterpriseSyncWorkerAlertAutoRetryWorkerTick(
	batchSize int,
	maxAttempts int,
	baseBackoff time.Duration,
	maxBackoff time.Duration,
) {
	if batchSize <= 0 {
		batchSize = 1
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if baseBackoff <= 0 {
		baseBackoff = 5 * time.Minute
	}
	if maxBackoff <= 0 {
		maxBackoff = time.Hour
	}
	if maxBackoff < baseBackoff {
		maxBackoff = baseBackoff
	}
	if s.enterpriseSvc == nil || s.walletSvc == nil {
		return
	}

	now := time.Now().UTC()
	allNotifications := s.enterpriseSvc.ListSyncWorkerAlertNotificationsWithOptions(
		enterprise.SyncWorkerAlertNotificationListOptions{
			Limit: 0,
		},
	)
	tenantIDs := pendingSyncWorkerAlertAutoRetryTenantIDs(allNotifications, now)
	if len(tenantIDs) == 0 {
		return
	}

	for i := range tenantIDs {
		tenantID := tenantIDs[i]
		result, err := s.enterpriseSvc.AutoRetrySyncWorkerAlertNotifications(enterprise.SyncWorkerAlertNotificationAutoRetryInput{
			TenantID:    tenantID,
			Limit:       batchSize,
			MaxAttempts: maxAttempts,
			BaseBackoff: baseBackoff,
			MaxBackoff:  maxBackoff,
			RetriedAt:   now,
			Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
				delivery := s.walletSvc.DispatchAlert(wallet.AlertDeliveryInput{
					TenantID:       input.TenantID,
					Channels:       input.Channels,
					ReceiverGroups: input.ReceiverGroups,
					IdempotencyKey: input.IdempotencyKey,
					EmailSubject:   input.EmailSubject,
					EmailText:      input.EmailText,
					WhatsAppText:   input.WhatsAppText,
				})
				return enterprise.SyncWorkerAlertDeliveryResult{
					Status:         delivery.Status,
					Reason:         delivery.Reason,
					Provider:       delivery.Provider,
					ProviderError:  delivery.ProviderError,
					Retryable:      delivery.Retryable,
					ChannelResults: delivery.ChannelResults,
				}
			},
		})
		if err != nil {
			s.loggerOrDefault().Error(
				"enterprise sync worker alert auto retry worker failed",
				"tenant_id", tenantID,
				"err", err,
			)
			continue
		}
		if result.TotalNotifications == 0 {
			continue
		}
		workerResult := enterpriseSyncWorkerAlertAutoRetryWorkerResult{
			Processed:  result.TotalNotifications,
			Retried:    result.Retried,
			Failed:     result.Failed,
			Skipped:    result.Skipped,
			Suppressed: result.Suppressed,
		}
		s.loggerOrDefault().Info(
			"enterprise sync worker alert auto retry worker finished",
			"tenant_id", tenantID,
			"processed", workerResult.Processed,
			"retried", workerResult.Retried,
			"failed", workerResult.Failed,
			"skipped", workerResult.Skipped,
			"suppressed", workerResult.Suppressed,
		)
		s.appendEnterpriseSyncWorkerAlertAutoRetryWorkerAudit(tenantID, workerResult)
	}
}

func (s *server) runEnterpriseSyncWorkerAlertAutoRetryWorkerTickWithLease(
	batchSize int,
	maxAttempts int,
	baseBackoff time.Duration,
	maxBackoff time.Duration,
	lockTTL time.Duration,
) {
	if s.workerLeaseStore == nil || lockTTL <= 0 {
		s.runEnterpriseSyncWorkerAlertAutoRetryWorkerTick(batchSize, maxAttempts, baseBackoff, maxBackoff)
		return
	}

	token, err := randomHexID(16)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise sync worker alert auto retry worker lease token generation failed",
			"err", err,
		)
		return
	}
	acquired, err := s.workerLeaseStore.TryAcquireLease(enterpriseSyncWorkerAlertAutoRetryLeaseKey, token, lockTTL)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise sync worker alert auto retry worker lease acquire failed",
			"err", err,
		)
		return
	}
	if !acquired {
		s.loggerOrDefault().Info(
			"enterprise sync worker alert auto retry worker lease unavailable; skipping tick",
			"lease_key", enterpriseSyncWorkerAlertAutoRetryLeaseKey,
		)
		return
	}
	defer func() {
		if err := s.workerLeaseStore.ReleaseLease(enterpriseSyncWorkerAlertAutoRetryLeaseKey, token); err != nil {
			s.loggerOrDefault().Error(
				"enterprise sync worker alert auto retry worker lease release failed",
				"lease_key", enterpriseSyncWorkerAlertAutoRetryLeaseKey,
				"err", err,
			)
		}
	}()

	s.runEnterpriseSyncWorkerAlertAutoRetryWorkerTick(batchSize, maxAttempts, baseBackoff, maxBackoff)
}

func (s *server) runEnterpriseHRISWebhookReceiptWorkerTickWithLease(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	s.runEnterpriseHRISWebhookReceiptWorkerTickWithLeaseAndRetryBackoff(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		processingTimeout,
		alertFailureThreshold,
		lockTTL,
	)
}

func (s *server) runEnterpriseHRISWebhookReceiptWorkerTickWithLeaseAndRetryBackoff(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	if s.workerLeaseStore == nil || lockTTL <= 0 {
		s.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(
			batchSize,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			processingTimeout,
			alertFailureThreshold,
		)
		return
	}

	token, err := randomHexID(16)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook receipt worker lease token generation failed",
			"err", err,
		)
		return
	}
	acquired, err := s.workerLeaseStore.TryAcquireLease(enterpriseHRISWebhookReceiptLeaseKey, token, lockTTL)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook receipt worker lease acquire failed",
			"err", err,
		)
		return
	}
	if !acquired {
		s.loggerOrDefault().Info(
			"enterprise hris webhook receipt worker lease unavailable; skipping tick",
			"lease_key", enterpriseHRISWebhookReceiptLeaseKey,
		)
		return
	}
	defer func() {
		if err := s.workerLeaseStore.ReleaseLease(enterpriseHRISWebhookReceiptLeaseKey, token); err != nil {
			s.loggerOrDefault().Error(
				"enterprise hris webhook receipt worker lease release failed",
				"lease_key", enterpriseHRISWebhookReceiptLeaseKey,
				"err", err,
			)
		}
	}()

	s.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISWebhookReceiptWorkerTick(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
) {
	s.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		processingTimeout,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
) {
	if batchSize <= 0 {
		batchSize = 1
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	if alertFailureThreshold <= 0 {
		alertFailureThreshold = 1
	}
	if s.enterpriseSvc == nil {
		return
	}
	if err := s.refreshEnterpriseHRISWebhookReceiptWorkerState(); err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook receipt worker state refresh failed",
			"err", err,
		)
		return
	}
	processedQueuedExecutions := s.runQueuedEnterpriseHRISWebhookReceiptExecutions(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
	)
	if processedQueuedExecutions >= batchSize {
		return
	}
	batchSize -= processedQueuedExecutions
	now := time.Now().UTC()
	allReceipts := s.enterpriseSvc.ListDueHRISWebhookReceiptsWithBackoff(
		"",
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
		0,
	)
	tenantIDs := pendingHRISWebhookReceiptTenantIDs(allReceipts)
	if len(tenantIDs) == 0 {
		return
	}
	receiptsByTenant := groupHRISWebhookReceiptsByTenant(allReceipts)

	for i := range tenantIDs {
		tenantID := tenantIDs[i]
		items := receiptsByTenant[tenantID]
		result := enterpriseHRISWebhookReceiptWorkerResult{}
		for j := range items {
			if result.Processed >= batchSize {
				break
			}
			item := items[j]
			status := strings.TrimSpace(item.Status)
			if status != "received" && status != "failed" && status != "processing" {
				continue
			}

			claimed, skipReason, err := s.enterpriseSvc.ClaimHRISWebhookReceiptForProcessingWithBackoff(
				tenantID,
				item.ID,
				maxAttempts,
				retryCooldown,
				retryMaxBackoff,
				processingTimeout,
				now,
			)
			if err != nil {
				s.loggerOrDefault().Error(
					"enterprise hris webhook receipt worker claim failed",
					"tenant_id", tenantID,
					"receipt_id", item.ID,
					"err", err,
				)
				continue
			}
			switch skipReason {
			case "":
			case enterprise.HRISWebhookReceiptClaimReasonAttemptLimit:
				result.SkippedByAttemptLimit++
				continue
			case enterprise.HRISWebhookReceiptClaimReasonCooldown:
				result.SkippedByCooldown++
				continue
			case enterprise.HRISWebhookReceiptClaimReasonInFlight:
				result.SkippedByInFlight++
				continue
			default:
				continue
			}

			recordDLQ := claimed.AttemptCount >= maxAttempts
			if err := s.processEnterpriseHRISWebhookReceipt(nil, claimed, recordDLQ); err != nil {
				result.Processed++
				result.Failed++
				result.LastConnectorID = strings.TrimSpace(claimed.ConnectorID)
				result.LastVendor = strings.TrimSpace(claimed.Vendor)
				result.LastRequestID = strings.TrimSpace(claimed.RequestID)
				result.LastEventType = strings.TrimSpace(claimed.EventType)
				s.loggerOrDefault().Error(
					"enterprise hris webhook receipt worker failed",
					"tenant_id", tenantID,
					"receipt_id", claimed.ID,
					"err", err,
				)
				continue
			}

			updated, err := s.enterpriseSvc.GetHRISWebhookReceipt(tenantID, claimed.ID)
			if err == nil && strings.TrimSpace(updated.Status) == "skipped" {
				result.Skipped++
			} else {
				result.Synced++
			}
			result.Processed++
		}

		if result.Processed == 0 {
			if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 || result.SkippedByInFlight > 0 {
				s.loggerOrDefault().Info(
					"enterprise hris webhook receipt worker skipped",
					"tenant_id", tenantID,
					"processed", 0,
					"skipped_in_flight", result.SkippedByInFlight,
					"skipped_attempt_limit", result.SkippedByAttemptLimit,
					"skipped_cooldown", result.SkippedByCooldown,
				)
				if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 {
					s.appendEnterpriseHRISWebhookReceiptWorkerAlertAudit(tenantID, result, alertFailureThreshold)
				}
			}
			continue
		}
		if result.Failed >= alertFailureThreshold {
			s.loggerOrDefault().Warn(
				"enterprise hris webhook receipt worker alert",
				"tenant_id", tenantID,
				"failed", result.Failed,
				"threshold", alertFailureThreshold,
			)
			s.appendEnterpriseHRISWebhookReceiptWorkerAlertAudit(tenantID, result, alertFailureThreshold)
		}
		s.loggerOrDefault().Info(
			"enterprise hris webhook receipt worker finished",
			"tenant_id", tenantID,
			"processed", result.Processed,
			"synced", result.Synced,
			"skipped", result.Skipped,
			"failed", result.Failed,
			"skipped_in_flight", result.SkippedByInFlight,
			"skipped_attempt_limit", result.SkippedByAttemptLimit,
			"skipped_cooldown", result.SkippedByCooldown,
		)
	}
}

func (s *server) runEnterpriseHRISWebhookDLQWorkerTickWithLease(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	s.runEnterpriseHRISWebhookDLQWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		0,
		alertFailureThreshold,
		lockTTL,
	)
}

func (s *server) runEnterpriseHRISWebhookDLQWorkerTickWithLeaseAndProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	s.runEnterpriseHRISWebhookDLQWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		processingTimeout,
		alertFailureThreshold,
		lockTTL,
	)
}

func (s *server) runEnterpriseHRISWebhookDLQWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	if s.workerLeaseStore == nil || lockTTL <= 0 {
		s.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(
			batchSize,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			processingTimeout,
			alertFailureThreshold,
		)
		return
	}

	token, err := randomHexID(16)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook dlq worker lease token generation failed",
			"err", err,
		)
		return
	}
	acquired, err := s.workerLeaseStore.TryAcquireLease(enterpriseHRISWebhookDLQLeaseKey, token, lockTTL)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook dlq worker lease acquire failed",
			"err", err,
		)
		return
	}
	if !acquired {
		s.loggerOrDefault().Info(
			"enterprise hris webhook dlq worker lease unavailable; skipping tick",
			"lease_key", enterpriseHRISWebhookDLQLeaseKey,
		)
		return
	}
	defer func() {
		if err := s.workerLeaseStore.ReleaseLease(enterpriseHRISWebhookDLQLeaseKey, token); err != nil {
			s.loggerOrDefault().Error(
				"enterprise hris webhook dlq worker lease release failed",
				"lease_key", enterpriseHRISWebhookDLQLeaseKey,
				"err", err,
			)
		}
	}()

	s.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISWebhookDLQWorkerTick(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	alertFailureThreshold int,
) {
	s.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		0,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISWebhookDLQWorkerTickWithProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
) {
	s.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		processingTimeout,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
) {
	if s.hrisDLQSvc == nil {
		return
	}
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	if err := s.refreshEnterpriseHRISWebhookDLQWorkerState(); err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook dlq worker state refresh failed",
			"err", err,
		)
		return
	}
	processedQueuedExecutions := s.runQueuedEnterpriseHRISWebhookDLQExecutionsWithRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
	)
	if processedQueuedExecutions >= batchSize {
		return
	}
	batchSize -= processedQueuedExecutions
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	now := time.Now().UTC()
	allEntries := s.hrisDLQSvc.ListDueEntriesForReplayWithBackoff(
		"",
		"",
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
		0,
	)
	tenantIDs := pendingHRISWebhookDLQTenantIDs(allEntries, maxAttempts, retryCooldown, now)
	if len(tenantIDs) == 0 {
		return
	}
	entriesByTenant := groupHRISWebhookDLQEntriesByTenant(allEntries)

	for i := range tenantIDs {
		tenantID := tenantIDs[i]
		items := entriesByTenant[tenantID]
		result := enterpriseHRISWebhookDLQWorkerResult{}
		for j := range items {
			if result.Processed >= batchSize {
				break
			}
			item := items[j]
			status := strings.TrimSpace(item.Status)
			if status != "dlq" && status != "replaying" {
				continue
			}
			claimed, skipReason, err := s.hrisDLQSvc.ClaimEntryForReplayWithBackoff(
				item.ID,
				maxAttempts,
				retryCooldown,
				retryMaxBackoff,
				processingTimeout,
				now,
			)
			if err != nil {
				s.loggerOrDefault().Error(
					"enterprise hris webhook dlq worker claim failed",
					"tenant_id", tenantID,
					"entry_id", item.ID,
					"err", err,
				)
				continue
			}
			switch skipReason {
			case "":
			case hris.DLQEntryClaimReasonAttemptLimit:
				result.SkippedByAttemptLimit++
				continue
			case hris.DLQEntryClaimReasonCooldown:
				result.SkippedByCooldown++
				continue
			case hris.DLQEntryClaimReasonInFlight:
				result.SkippedByInFlight++
				continue
			default:
				continue
			}

			if _, err := s.replayEnterpriseHRISWebhookDLQClaimedEntry(nil, tenantID, claimed, "enterprise_sync_worker"); err != nil {
				result.Failed++
				result.Processed++
				result.LastConnectorID = strings.TrimSpace(claimed.ConnectorID)
				result.LastVendor = strings.TrimSpace(claimed.Vendor)
				result.LastRequestID = strings.TrimSpace(claimed.RequestID)
				result.LastEventType = strings.TrimSpace(claimed.EventType)
				result.LastFailureStage = strings.TrimSpace(claimed.FailureStage)
				s.loggerOrDefault().Error(
					"enterprise hris webhook dlq worker replay failed",
					"tenant_id", tenantID,
					"entry_id", claimed.ID,
					"err", err,
				)
				continue
			}
			result.Replayed++
			result.Processed++
		}

		if result.Processed == 0 {
			if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 || result.SkippedByInFlight > 0 {
				s.loggerOrDefault().Info(
					"enterprise hris webhook dlq worker skipped",
					"tenant_id", tenantID,
					"processed", 0,
					"skipped_in_flight", result.SkippedByInFlight,
					"skipped_attempt_limit", result.SkippedByAttemptLimit,
					"skipped_cooldown", result.SkippedByCooldown,
				)
				if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 {
					s.appendEnterpriseHRISWebhookDLQWorkerAlertAudit(tenantID, result, alertFailureThreshold)
				}
			}
			continue
		}
		if result.Failed >= alertFailureThreshold {
			s.loggerOrDefault().Warn(
				"enterprise hris webhook dlq worker alert",
				"tenant_id", tenantID,
				"failed", result.Failed,
				"threshold", alertFailureThreshold,
			)
			s.appendEnterpriseHRISWebhookDLQWorkerAlertAudit(tenantID, result, alertFailureThreshold)
		}
		s.loggerOrDefault().Info(
			"enterprise hris webhook dlq worker finished",
			"tenant_id", tenantID,
			"processed", result.Processed,
			"replayed", result.Replayed,
			"failed", result.Failed,
			"skipped_in_flight", result.SkippedByInFlight,
			"skipped_attempt_limit", result.SkippedByAttemptLimit,
			"skipped_cooldown", result.SkippedByCooldown,
		)
	}
}

func (s *server) runEnterpriseHRISPullWorkerTick(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	reconcileInterval time.Duration,
	alertFailureThreshold int,
) {
	s.runEnterpriseHRISPullWorkerTickWithRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		reconcileInterval,
		0,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISPullWorkerTickWithLease(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	reconcileInterval time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	s.runEnterpriseHRISPullWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		reconcileInterval,
		0,
		alertFailureThreshold,
		lockTTL,
	)
}

func (s *server) runEnterpriseHRISPullWorkerTickWithLeaseAndProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	reconcileInterval time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	s.runEnterpriseHRISPullWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		reconcileInterval,
		processingTimeout,
		alertFailureThreshold,
		lockTTL,
	)
}

func (s *server) runEnterpriseHRISPullWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	reconcileInterval time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	if s.workerLeaseStore == nil || lockTTL <= 0 {
		s.runEnterpriseHRISPullWorkerTickWithRetryBackoffAndProcessingTimeout(
			batchSize,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			reconcileInterval,
			processingTimeout,
			alertFailureThreshold,
		)
		return
	}

	token, err := randomHexID(16)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris pull worker lease token generation failed",
			"err", err,
		)
		return
	}
	acquired, err := s.workerLeaseStore.TryAcquireLease(enterpriseHRISPullLeaseKey, token, lockTTL)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris pull worker lease acquire failed",
			"err", err,
		)
		return
	}
	if !acquired {
		s.loggerOrDefault().Info(
			"enterprise hris pull worker lease unavailable; skipping tick",
			"lease_key", enterpriseHRISPullLeaseKey,
		)
		return
	}
	defer func() {
		if err := s.workerLeaseStore.ReleaseLease(enterpriseHRISPullLeaseKey, token); err != nil {
			s.loggerOrDefault().Error(
				"enterprise hris pull worker lease release failed",
				"lease_key", enterpriseHRISPullLeaseKey,
				"err", err,
			)
		}
	}()

	s.runEnterpriseHRISPullWorkerTickWithRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		reconcileInterval,
		processingTimeout,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISPullWorkerTickWithProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	reconcileInterval time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
) {
	s.runEnterpriseHRISPullWorkerTickWithRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		reconcileInterval,
		processingTimeout,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISPullWorkerTickWithRetryBackoffAndProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	reconcileInterval time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
) {
	if s.enterpriseSvc == nil || s.hrisPullRegistry == nil || s.hrisPullStateSvc == nil {
		return
	}
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	if processingTimeout <= 0 {
		processingTimeout = 30 * time.Minute
	}
	if err := s.refreshEnterpriseHRISPullWorkerState(); err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris pull worker shared state refresh failed",
			"err", err,
		)
		return
	}

	now := time.Now().UTC()
	connectors := s.enterpriseSvc.ListHRISConnectors("")
	if len(connectors) == 0 {
		return
	}
	sort.Slice(connectors, func(i, j int) bool {
		leftTenantID := strings.TrimSpace(connectors[i].TenantID)
		rightTenantID := strings.TrimSpace(connectors[j].TenantID)
		if leftTenantID != rightTenantID {
			return leftTenantID < rightTenantID
		}
		leftVendor := strings.TrimSpace(connectors[i].Vendor)
		rightVendor := strings.TrimSpace(connectors[j].Vendor)
		if leftVendor != rightVendor {
			return leftVendor < rightVendor
		}
		return strings.TrimSpace(connectors[i].ID) < strings.TrimSpace(connectors[j].ID)
	})

	stateMap := make(map[string]hris.ConnectorPullState)
	for _, item := range s.hrisPullStateSvc.ListStates("") {
		stateMap[strings.TrimSpace(item.ConnectorID)] = item
	}

	resultsByTenant := make(map[string]*enterpriseHRISPullWorkerResult)
	processedCount := 0
	for i := range connectors {
		connector := connectors[i]
		if !enterpriseHRISPullConnectorCandidate(connector) {
			continue
		}
		adapter, ok := s.hrisPullRegistry.Get(connector.Vendor)
		if !ok {
			continue
		}

		tenantID := strings.TrimSpace(connector.TenantID)
		if tenantID == "" {
			continue
		}
		result := resultsByTenant[tenantID]
		if result == nil {
			result = &enterpriseHRISPullWorkerResult{}
			resultsByTenant[tenantID] = result
		}

		state := stateMap[strings.TrimSpace(connector.ID)]
		mode := enterpriseHRISPullMode(connector, state, now, reconcileInterval)
		pullInput := hris.PullInput{
			Connector:         connector,
			LastSuccessAt:     state.LastSuccessAt,
			LastFullSuccessAt: enterpriseHRISLastFullSuccessAt(connector, state),
			Mode:              mode,
			Now:               now,
		}
		if mode == "" {
			continue
		}

		if processedCount >= batchSize {
			break
		}

		claimedState, skipReason, err := s.hrisPullStateSvc.ClaimStateForPullWithBackoff(
			tenantID,
			connector.ID,
			connector.Vendor,
			mode,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			processingTimeout,
			now,
		)
		if err != nil {
			s.loggerOrDefault().Error(
				"enterprise hris pull worker claim failed",
				"tenant_id", tenantID,
				"connector_id", connector.ID,
				"err", err,
			)
			continue
		}
		switch skipReason {
		case "":
			state = claimedState
		case hris.PullStateClaimReasonAttemptLimit:
			result.SkippedByAttemptLimit++
			updateEnterpriseHRISPullWorkerStatefulMetrics(result, connector, claimedState, mode, now)
			continue
		case hris.PullStateClaimReasonCooldown:
			result.SkippedByCooldown++
			updateEnterpriseHRISPullWorkerStatefulMetrics(result, connector, claimedState, mode, now)
			continue
		case hris.PullStateClaimReasonInFlight:
			result.SkippedByInFlight++
			updateEnterpriseHRISPullWorkerStatefulMetrics(result, connector, claimedState, mode, now)
			continue
		default:
			continue
		}

		credentialValue := ""
		if strings.TrimSpace(connector.CredentialRef) != "" {
			if s.hrisVaultSvc == nil {
				result.Processed++
				result.Failed++
				result.LastConnectorID = strings.TrimSpace(connector.ID)
				result.LastVendor = strings.TrimSpace(connector.Vendor)
				result.LastMode = strings.TrimSpace(mode)
				processedCount++
				updatedState, _ := s.hrisPullStateSvc.MarkFailed(tenantID, connector.ID, connector.Vendor, now, errors.New("hris credential vault unavailable"))
				updateEnterpriseHRISPullWorkerStatefulMetrics(result, connector, updatedState, mode, now)
				s.loggerOrDefault().Error(
					"enterprise hris pull worker credential vault unavailable",
					"tenant_id", tenantID,
					"connector_id", connector.ID,
				)
				continue
			}
			resolvedCredential, err := s.hrisVaultSvc.ResolveSecretRef(connector.CredentialRef)
			if err != nil {
				result.Processed++
				result.Failed++
				result.LastConnectorID = strings.TrimSpace(connector.ID)
				result.LastVendor = strings.TrimSpace(connector.Vendor)
				result.LastMode = strings.TrimSpace(mode)
				processedCount++
				updatedState, _ := s.hrisPullStateSvc.MarkFailed(tenantID, connector.ID, connector.Vendor, now, err)
				updateEnterpriseHRISPullWorkerStatefulMetrics(result, connector, updatedState, mode, now)
				s.loggerOrDefault().Error(
					"enterprise hris pull worker credential resolution failed",
					"tenant_id", tenantID,
					"connector_id", connector.ID,
					"err", err,
				)
				continue
			}
			credentialValue = resolvedCredential.Value
		}

		pullInput.CredentialValue = credentialValue
		if mode == hris.PullModeIncremental && !hris.SupportsIncrementalPull(adapter, pullInput) {
			mode = hris.PullModeFull
			pullInput.Mode = mode
			claimedState, err := s.hrisPullStateSvc.MarkStarted(tenantID, connector.ID, connector.Vendor, mode, now)
			if err != nil {
				result.Processed++
				result.Failed++
				result.LastConnectorID = strings.TrimSpace(connector.ID)
				result.LastVendor = strings.TrimSpace(connector.Vendor)
				result.LastMode = strings.TrimSpace(mode)
				processedCount++
				s.loggerOrDefault().Error(
					"enterprise hris pull worker fallback to full claim update failed",
					"tenant_id", tenantID,
					"connector_id", connector.ID,
					"err", err,
				)
				continue
			}
			state = claimedState
		}

		if err := s.processEnterpriseHRISPullConnector(connector, credentialValue, state, mode, now); err != nil {
			updatedState, _ := s.hrisPullStateSvc.MarkFailed(tenantID, connector.ID, connector.Vendor, now, err)
			result.Processed++
			result.Failed++
			updateEnterpriseHRISPullWorkerStatefulMetrics(result, connector, updatedState, mode, now)
			processedCount++
			s.loggerOrDefault().Error(
				"enterprise hris pull worker failed",
				"tenant_id", tenantID,
				"connector_id", connector.ID,
				"vendor", connector.Vendor,
				"err", err,
			)
			continue
		}

		result.Processed++
		result.Synced++
		processedCount++
	}

	for tenantID, result := range resultsByTenant {
		if result.Processed == 0 {
			if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 || result.SkippedByInFlight > 0 {
				s.loggerOrDefault().Info(
					"enterprise hris pull worker skipped",
					"tenant_id", tenantID,
					"processed", 0,
					"skipped_in_flight", result.SkippedByInFlight,
					"skipped_attempt_limit", result.SkippedByAttemptLimit,
					"skipped_cooldown", result.SkippedByCooldown,
				)
				if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 {
					s.appendEnterpriseHRISPullWorkerAlertAudit(tenantID, *result, alertFailureThreshold)
				}
			}
			continue
		}
		if shouldAppendEnterpriseHRISPullWorkerAlertAudit(*result, alertFailureThreshold) {
			s.loggerOrDefault().Warn(
				"enterprise hris pull worker alert",
				"tenant_id", tenantID,
				"failed", result.Failed,
				"threshold", alertFailureThreshold,
				"consecutive_failures", result.ConsecutiveFailures,
				"failure_age_seconds", result.FailureAgeSeconds,
			)
			s.appendEnterpriseHRISPullWorkerAlertAudit(tenantID, *result, alertFailureThreshold)
		}
		s.loggerOrDefault().Info(
			"enterprise hris pull worker finished",
			"tenant_id", tenantID,
			"processed", result.Processed,
			"synced", result.Synced,
			"failed", result.Failed,
			"skipped_in_flight", result.SkippedByInFlight,
			"skipped_attempt_limit", result.SkippedByAttemptLimit,
			"skipped_cooldown", result.SkippedByCooldown,
		)
	}
}

func (s *server) processEnterpriseHRISPullConnector(
	connector enterprise.HRISConnector,
	credentialValue string,
	state hris.ConnectorPullState,
	mode string,
	now time.Time,
) error {
	if strings.TrimSpace(connector.CredentialRef) == "" {
		return errors.New("hris connector credential_ref is required for pull sync")
	}
	if strings.TrimSpace(credentialValue) == "" {
		return errors.New("hris connector credential value is required for pull sync")
	}

	timeout := firstNonZeroDuration(s.cfg.ExternalAuthTimeout, 15*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pullResult, err := s.hrisPullRegistry.Pull(ctx, hris.PullInput{
		Connector:         connector,
		CredentialValue:   credentialValue,
		LastSuccessAt:     state.LastSuccessAt,
		LastFullSuccessAt: enterpriseHRISLastFullSuccessAt(connector, state),
		Mode:              mode,
		HTTPClient:        s.hrisHTTPClient,
		Now:               now,
	})
	if err != nil {
		return err
	}

	source := strings.TrimSpace(pullResult.Source)
	if source == "" {
		source = hris.SyncSourceForVendor(connector.Vendor)
	}
	requestID := strings.TrimSpace(pullResult.RequestID)
	if requestID == "" {
		requestID = hris.NormalizeVendor(connector.Vendor) + ":" + strings.TrimSpace(connector.ID) + ":pull:" + now.UTC().Format("20060102t150405z")
	}
	pullMode := hris.NormalizePullMode(pullResult.Mode)
	if pullMode == "" {
		pullMode = hris.NormalizePullMode(mode)
	}

	inputs := buildEnterpriseHRISPullReconcileInputs(
		s.enterpriseSvc.ListEmployees(connector.TenantID),
		source,
		pullResult.Employees,
		pullMode == hris.PullModeFull,
	)
	if len(inputs) > 0 {
		_, _, _, _, err = s.enterpriseSvc.SyncEmployeesWithAccessUpsertMetadata(
			connector.TenantID,
			source,
			hris.SyncActor,
			requestID,
			connector.ID,
			enterpriseHRISPullRawPayloadRef(requestID),
			inputs,
			func(items []enterprise.EnterpriseEmployee) (int, int, int, error) {
				accessInputs := enterpriseEmployeesToAccessBatchInputs(items)
				return s.accessSvc.UpsertUsersByEmail(connector.TenantID, accessInputs)
			},
		)
		if err != nil {
			return err
		}
	}

	if _, err := s.enterpriseSvc.MarkHRISConnectorSynced(connector.TenantID, connector.ID, now); err != nil {
		return err
	}
	if _, err := s.hrisPullStateSvc.MarkSucceeded(connector.TenantID, connector.ID, connector.Vendor, pullMode, requestID, now); err != nil {
		return err
	}
	return nil
}

func enterpriseHRISPullConnectorCandidate(connector enterprise.HRISConnector) bool {
	if strings.TrimSpace(connector.Status) != "active" {
		return false
	}
	switch strings.TrimSpace(connector.SyncStrategy) {
	case "pull", "hybrid", "":
		return true
	default:
		return false
	}
}

func enterpriseHRISPullMode(
	connector enterprise.HRISConnector,
	state hris.ConnectorPullState,
	now time.Time,
	reconcileInterval time.Duration,
) string {
	lastSuccessAt := enterpriseHRISLastSuccessAt(connector, state)
	if lastSuccessAt == nil {
		return hris.PullModeFull
	}
	if enterpriseHRISFullReconcileDue(connector, state, now, reconcileInterval) {
		return hris.PullModeFull
	}
	return hris.PullModeIncremental
}

func enterpriseHRISLastSuccessAt(
	connector enterprise.HRISConnector,
	state hris.ConnectorPullState,
) *time.Time {
	lastSuccessAt := cloneTimePointerLocal(connector.LastSyncAt)
	if state.LastSuccessAt != nil {
		if lastSuccessAt == nil || state.LastSuccessAt.After(*lastSuccessAt) {
			lastSuccessAt = cloneTimePointerLocal(state.LastSuccessAt)
		}
	}
	return lastSuccessAt
}

func enterpriseHRISLastFullSuccessAt(
	connector enterprise.HRISConnector,
	state hris.ConnectorPullState,
) *time.Time {
	if state.LastFullSuccessAt != nil {
		return cloneTimePointerLocal(state.LastFullSuccessAt)
	}
	if state.LastSuccessAt == nil && connector.LastSyncAt != nil {
		return cloneTimePointerLocal(connector.LastSyncAt)
	}
	return nil
}

func enterpriseHRISFullReconcileDue(
	connector enterprise.HRISConnector,
	state hris.ConnectorPullState,
	now time.Time,
	reconcileInterval time.Duration,
) bool {
	lastFullSuccessAt := enterpriseHRISLastFullSuccessAt(connector, state)
	if lastFullSuccessAt == nil {
		return true
	}
	if reconcileInterval <= 0 {
		reconcileInterval = 24 * time.Hour
	}
	return !lastFullSuccessAt.Add(reconcileInterval).After(now)
}

func buildEnterpriseHRISPullReconcileInputs(
	existing []enterprise.EnterpriseEmployee,
	source string,
	pulled []enterprise.EmployeeSyncInput,
	fullSnapshot bool,
) []enterprise.EmployeeSyncInput {
	output := make([]enterprise.EmployeeSyncInput, 0, len(pulled))
	seenExternalIDs := make(map[string]struct{}, len(pulled))
	seenEmails := make(map[string]struct{}, len(pulled))
	for i := range pulled {
		item := pulled[i]
		if nextExternalID := strings.TrimSpace(item.ExternalID); nextExternalID != "" {
			seenExternalIDs[nextExternalID] = struct{}{}
		}
		if nextEmail := strings.ToLower(strings.TrimSpace(item.Email)); nextEmail != "" {
			seenEmails[nextEmail] = struct{}{}
		}
		output = append(output, item)
	}
	if !fullSnapshot {
		return output
	}

	nextSource := strings.TrimSpace(source)
	for i := range existing {
		item := existing[i]
		if strings.TrimSpace(item.Source) != nextSource {
			continue
		}
		externalID := strings.TrimSpace(item.ExternalID)
		if externalID != "" {
			if _, ok := seenExternalIDs[externalID]; ok {
				continue
			}
		} else {
			email := strings.ToLower(strings.TrimSpace(item.Email))
			if email == "" {
				continue
			}
			if _, ok := seenEmails[email]; ok {
				continue
			}
		}
		if strings.TrimSpace(item.Status) == "inactive" && enterprise.EmploymentStatusBlocksSession(item.EmploymentStatus) {
			continue
		}

		output = append(output, enterprise.EmployeeSyncInput{
			ExternalID:        item.ExternalID,
			EmployeeNumber:    item.EmployeeNumber,
			Email:             item.Email,
			FullName:          item.FullName,
			Department:        item.Department,
			JobTitle:          item.JobTitle,
			Location:          item.Location,
			Phone:             item.Phone,
			ManagerExternalID: item.ManagerExternalID,
			EmploymentStatus:  "inactive",
			JoinDate:          item.JoinDate,
			ResignDate:        firstNonEmptyString(item.ResignDate, time.Now().UTC().Format("2006-01-02")),
			ShiftCode:         item.ShiftCode,
			ScheduleWindow:    item.ScheduleWindow,
			LeaveStatus:       item.LeaveStatus,
			CostCenter:        item.CostCenter,
			PhotoURL:          item.PhotoURL,
			Status:            "inactive",
		})
	}
	return output
}

func enterpriseHRISPullRawPayloadRef(requestID string) string {
	nextRequestID := strings.TrimSpace(requestID)
	if nextRequestID == "" {
		return ""
	}
	return "hris_pull_run:" + nextRequestID
}

func cloneTimePointerLocal(input *time.Time) *time.Time {
	if input == nil {
		return nil
	}
	value := *input
	return &value
}

func shouldAppendEnterpriseWorkerAlertAudit(
	processed int,
	failed int,
	alertFailureThreshold int,
	skippedByAttemptLimit int,
	skippedByCooldown int,
) bool {
	if failed >= alertFailureThreshold {
		return true
	}
	return processed == 0 && (skippedByAttemptLimit > 0 || skippedByCooldown > 0)
}

func enterpriseHRISPullFailureAgeSeconds(now time.Time, lastFailureAt *time.Time) int {
	if lastFailureAt == nil {
		return 0
	}
	if now.Before(*lastFailureAt) {
		return 0
	}
	return int(now.Sub(*lastFailureAt).Seconds())
}

func updateEnterpriseHRISPullWorkerStatefulMetrics(
	result *enterpriseHRISPullWorkerResult,
	connector enterprise.HRISConnector,
	state hris.ConnectorPullState,
	mode string,
	now time.Time,
) {
	if result == nil {
		return
	}
	consecutiveFailures := state.ConsecutiveFailures
	failureAgeSeconds := enterpriseHRISPullFailureAgeSeconds(now, state.LastFailureAt)
	if consecutiveFailures <= 0 && failureAgeSeconds <= 0 {
		return
	}
	if consecutiveFailures < result.ConsecutiveFailures {
		return
	}
	if consecutiveFailures == result.ConsecutiveFailures && failureAgeSeconds <= result.FailureAgeSeconds {
		return
	}
	result.ConsecutiveFailures = consecutiveFailures
	result.FailureAgeSeconds = failureAgeSeconds
	result.LastConnectorID = strings.TrimSpace(connector.ID)
	result.LastVendor = strings.TrimSpace(connector.Vendor)
	result.LastMode = strings.TrimSpace(mode)
}

func shouldAppendEnterpriseHRISPullWorkerAlertAudit(
	result enterpriseHRISPullWorkerResult,
	alertFailureThreshold int,
) bool {
	if shouldAppendEnterpriseWorkerAlertAudit(
		result.Processed,
		result.Failed,
		alertFailureThreshold,
		result.SkippedByAttemptLimit,
		result.SkippedByCooldown,
	) {
		return true
	}
	threshold := alertFailureThreshold
	if threshold <= 0 {
		threshold = 1
	}
	return result.ConsecutiveFailures >= threshold
}

func (s *server) appendEnterpriseSyncWorkerAlertAudit(
	tenantID string,
	result enterprise.BatchPendingSyncReconcileResult,
	alertFailureThreshold int,
) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}
	if !shouldAppendEnterpriseWorkerAlertAudit(
		result.Processed,
		result.Failed,
		alertFailureThreshold,
		result.SkippedByAttemptLimit,
		result.SkippedByCooldown,
	) {
		return
	}
	target := strings.TrimSpace(
		fmt.Sprintf(
			"failed=%d threshold=%d processed=%d applied=%d skipped_attempt_limit=%d skipped_cooldown=%d",
			result.Failed,
			alertFailureThreshold,
			result.Processed,
			result.Applied,
			result.SkippedByAttemptLimit,
			result.SkippedByCooldown,
		),
	)
	_, _ = s.auditSvc.Append(
		nextTenantID,
		"enterprise_sync_worker",
		"system",
		"enterprise_sync_reconcile_worker_alert",
		target,
		"enterprise_sync_worker",
	)
}

func (s *server) appendEnterpriseSyncWorkerAlertAutoRetryWorkerAudit(
	tenantID string,
	result enterpriseSyncWorkerAlertAutoRetryWorkerResult,
) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}
	target := strings.TrimSpace(
		fmt.Sprintf(
			"processed=%d retried=%d failed=%d skipped=%d suppressed=%d",
			result.Processed,
			result.Retried,
			result.Failed,
			result.Skipped,
			result.Suppressed,
		),
	)
	_, _ = s.auditSvc.Append(
		nextTenantID,
		"enterprise_sync_worker",
		"system",
		"enterprise_sync_worker_alert_auto_retry_worker_completed",
		target,
		"enterprise_sync_worker",
	)
}

func (s *server) appendEnterpriseHRISWebhookReceiptWorkerAlertAudit(
	tenantID string,
	result enterpriseHRISWebhookReceiptWorkerResult,
	alertFailureThreshold int,
) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}
	if !shouldAppendEnterpriseWorkerAlertAudit(
		result.Processed,
		result.Failed,
		alertFailureThreshold,
		result.SkippedByAttemptLimit,
		result.SkippedByCooldown,
	) {
		return
	}
	targetParts := []string{
		fmt.Sprintf("failed=%d", result.Failed),
		fmt.Sprintf("threshold=%d", alertFailureThreshold),
		fmt.Sprintf("processed=%d", result.Processed),
		fmt.Sprintf("synced=%d", result.Synced),
		fmt.Sprintf("skipped=%d", result.Skipped),
		fmt.Sprintf("skipped_in_flight=%d", result.SkippedByInFlight),
		fmt.Sprintf("skipped_attempt_limit=%d", result.SkippedByAttemptLimit),
		fmt.Sprintf("skipped_cooldown=%d", result.SkippedByCooldown),
	}
	if nextConnectorID := strings.TrimSpace(result.LastConnectorID); nextConnectorID != "" {
		targetParts = append(targetParts, "connector_id="+nextConnectorID)
	}
	if nextVendor := strings.TrimSpace(result.LastVendor); nextVendor != "" {
		targetParts = append(targetParts, "vendor="+nextVendor)
	}
	if nextRequestID := strings.TrimSpace(result.LastRequestID); nextRequestID != "" {
		targetParts = append(targetParts, "request_id="+nextRequestID)
	}
	if nextEventType := strings.TrimSpace(result.LastEventType); nextEventType != "" {
		targetParts = append(targetParts, "event_type="+nextEventType)
	}
	target := strings.Join(targetParts, " ")
	_, _ = s.auditSvc.Append(
		nextTenantID,
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_webhook_receipt_worker_alert",
		target,
		"enterprise_sync_worker",
	)
}

func (s *server) appendEnterpriseHRISWebhookDLQWorkerAlertAudit(
	tenantID string,
	result enterpriseHRISWebhookDLQWorkerResult,
	alertFailureThreshold int,
) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}
	if !shouldAppendEnterpriseWorkerAlertAudit(
		result.Processed,
		result.Failed,
		alertFailureThreshold,
		result.SkippedByAttemptLimit,
		result.SkippedByCooldown,
	) {
		return
	}
	targetParts := []string{
		fmt.Sprintf("failed=%d", result.Failed),
		fmt.Sprintf("threshold=%d", alertFailureThreshold),
		fmt.Sprintf("processed=%d", result.Processed),
		fmt.Sprintf("replayed=%d", result.Replayed),
		fmt.Sprintf("skipped_in_flight=%d", result.SkippedByInFlight),
		fmt.Sprintf("skipped_attempt_limit=%d", result.SkippedByAttemptLimit),
		fmt.Sprintf("skipped_cooldown=%d", result.SkippedByCooldown),
	}
	if nextConnectorID := strings.TrimSpace(result.LastConnectorID); nextConnectorID != "" {
		targetParts = append(targetParts, "connector_id="+nextConnectorID)
	}
	if nextVendor := strings.TrimSpace(result.LastVendor); nextVendor != "" {
		targetParts = append(targetParts, "vendor="+nextVendor)
	}
	if nextRequestID := strings.TrimSpace(result.LastRequestID); nextRequestID != "" {
		targetParts = append(targetParts, "request_id="+nextRequestID)
	}
	if nextEventType := strings.TrimSpace(result.LastEventType); nextEventType != "" {
		targetParts = append(targetParts, "event_type="+nextEventType)
	}
	if nextFailureStage := strings.TrimSpace(result.LastFailureStage); nextFailureStage != "" {
		targetParts = append(targetParts, "failure_stage="+nextFailureStage)
	}
	target := strings.Join(targetParts, " ")
	_, _ = s.auditSvc.Append(
		nextTenantID,
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_webhook_dlq_worker_alert",
		target,
		"enterprise_sync_worker",
	)
}

func (s *server) appendEnterpriseHRISPullWorkerAlertAudit(
	tenantID string,
	result enterpriseHRISPullWorkerResult,
	alertFailureThreshold int,
) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}
	if !shouldAppendEnterpriseHRISPullWorkerAlertAudit(result, alertFailureThreshold) {
		return
	}
	targetParts := []string{
		fmt.Sprintf("failed=%d", result.Failed),
		fmt.Sprintf("threshold=%d", alertFailureThreshold),
		fmt.Sprintf("processed=%d", result.Processed),
		fmt.Sprintf("synced=%d", result.Synced),
		fmt.Sprintf("consecutive_failures=%d", result.ConsecutiveFailures),
		fmt.Sprintf("failure_age_seconds=%d", result.FailureAgeSeconds),
		fmt.Sprintf("skipped_in_flight=%d", result.SkippedByInFlight),
		fmt.Sprintf("skipped_attempt_limit=%d", result.SkippedByAttemptLimit),
		fmt.Sprintf("skipped_cooldown=%d", result.SkippedByCooldown),
	}
	if nextConnectorID := strings.TrimSpace(result.LastConnectorID); nextConnectorID != "" {
		targetParts = append(targetParts, "connector_id="+nextConnectorID)
	}
	if nextVendor := strings.TrimSpace(result.LastVendor); nextVendor != "" {
		targetParts = append(targetParts, "vendor="+nextVendor)
	}
	if nextMode := strings.TrimSpace(result.LastMode); nextMode != "" {
		targetParts = append(targetParts, "mode="+nextMode)
	}
	target := strings.Join(targetParts, " ")
	_, _ = s.auditSvc.Append(
		nextTenantID,
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_pull_worker_alert",
		target,
		"enterprise_sync_worker",
	)
}

func (s *server) appendEnterpriseHRISWebhookProcessingAlertAudit(
	tenantID string,
	connectorID string,
	vendor string,
	eventType string,
	requestID string,
	failureStage string,
) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}
	target := strings.TrimSpace(
		fmt.Sprintf(
			"failed=1 threshold=1 processed=1 applied=0 connector_id=%s vendor=%s event_type=%s request_id=%s failure_stage=%s",
			strings.TrimSpace(connectorID),
			strings.TrimSpace(vendor),
			strings.TrimSpace(eventType),
			strings.TrimSpace(requestID),
			strings.TrimSpace(failureStage),
		),
	)
	_, _ = s.auditSvc.Append(
		nextTenantID,
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_webhook_processing_alert",
		target,
		"enterprise_sync_worker",
	)
}

func pendingSyncWorkerAlertAutoRetryTenantIDs(
	items []enterprise.SyncWorkerAlertNotification,
	now time.Time,
) []string {
	set := make(map[string]struct{})
	for i := range items {
		if items[i].Status != "failed" || !items[i].Retryable || items[i].NextRetryAt == nil {
			continue
		}
		if items[i].NextRetryAt.After(now) {
			continue
		}
		tenantID := strings.TrimSpace(items[i].TenantID)
		if tenantID == "" {
			continue
		}
		set[tenantID] = struct{}{}
	}
	tenantIDs := make([]string, 0, len(set))
	for tenantID := range set {
		tenantIDs = append(tenantIDs, tenantID)
	}
	sort.Strings(tenantIDs)
	return tenantIDs
}

func pendingSyncRequestTenantIDs(records []enterprise.SyncRequestRecord) []string {
	set := make(map[string]struct{})
	for i := range records {
		if records[i].AccessApplied {
			continue
		}
		nextTenantID := strings.TrimSpace(records[i].TenantID)
		if nextTenantID == "" {
			continue
		}
		set[nextTenantID] = struct{}{}
	}

	items := make([]string, 0, len(set))
	for tenantID := range set {
		items = append(items, tenantID)
	}
	sort.Strings(items)
	return items
}

func pendingHRISWebhookReceiptTenantIDs(items []enterprise.HRISWebhookReceipt) []string {
	set := make(map[string]struct{})
	for i := range items {
		status := strings.TrimSpace(items[i].Status)
		if status != "received" && status != "failed" && status != "processing" {
			continue
		}
		nextTenantID := strings.TrimSpace(items[i].TenantID)
		if nextTenantID == "" {
			continue
		}
		set[nextTenantID] = struct{}{}
	}

	output := make([]string, 0, len(set))
	for tenantID := range set {
		output = append(output, tenantID)
	}
	sort.Strings(output)
	return output
}

func groupHRISWebhookReceiptsByTenant(items []enterprise.HRISWebhookReceipt) map[string][]enterprise.HRISWebhookReceipt {
	grouped := make(map[string][]enterprise.HRISWebhookReceipt)
	for i := range items {
		tenantID := strings.TrimSpace(items[i].TenantID)
		if tenantID == "" {
			continue
		}
		grouped[tenantID] = append(grouped[tenantID], items[i])
	}
	return grouped
}

func pendingHRISWebhookDLQTenantIDs(
	entries []hris.DeadLetterEntry,
	_ int,
	_ time.Duration,
	_ time.Time,
) []string {
	set := make(map[string]struct{})
	for i := range entries {
		status := strings.TrimSpace(entries[i].Status)
		if status != "dlq" && status != "replaying" {
			continue
		}
		// Keep cooldown/attempt-limit and in-flight replaying entries in scope so skip-only ticks
		// can still emit the expected worker logs/audit.
		nextTenantID := strings.TrimSpace(entries[i].TenantID)
		if nextTenantID == "" {
			continue
		}
		set[nextTenantID] = struct{}{}
	}

	items := make([]string, 0, len(set))
	for tenantID := range set {
		items = append(items, tenantID)
	}
	sort.Strings(items)
	return items
}

func groupHRISWebhookDLQEntriesByTenant(entries []hris.DeadLetterEntry) map[string][]hris.DeadLetterEntry {
	grouped := make(map[string][]hris.DeadLetterEntry)
	for i := range entries {
		tenantID := strings.TrimSpace(entries[i].TenantID)
		if tenantID == "" {
			continue
		}
		grouped[tenantID] = append(grouped[tenantID], entries[i])
	}
	return grouped
}

func pendingJITApprovalExternalSyncTenantIDs(
	records []enterprise.JITProvisionApproval,
	maxAttempts int,
	retryCooldown time.Duration,
	now time.Time,
) []string {
	set := make(map[string]struct{})
	for i := range records {
		item := records[i]
		if !enterpriseJITApprovalExternalSyncCandidate(item) {
			continue
		}
		syncStatus := strings.TrimSpace(item.ExternalSyncStatus)
		if syncStatus == "failed" {
			if maxAttempts > 0 && item.ExternalSyncAttemptCount >= maxAttempts {
				continue
			}
			if retryCooldown > 0 && item.ExternalSyncUpdatedAt != nil {
				retryReadyAt := item.ExternalSyncUpdatedAt.Add(retryCooldown)
				if retryReadyAt.After(now) {
					continue
				}
			}
		}
		nextTenantID := strings.TrimSpace(item.TenantID)
		if nextTenantID == "" {
			continue
		}
		set[nextTenantID] = struct{}{}
	}

	items := make([]string, 0, len(set))
	for tenantID := range set {
		items = append(items, tenantID)
	}
	sort.Strings(items)
	return items
}

