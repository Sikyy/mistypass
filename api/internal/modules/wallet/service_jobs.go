package wallet

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/wallet/alertdispatch"
)

func (s *Service) ListJobs(tenantID string) []IssueJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	items := make([]IssueJob, 0, len(s.jobs))
	for i := range s.jobs {
		if s.jobs[i].TenantID != nextTenantID {
			continue
		}
		items = append(items, s.jobs[i])
	}
	return items
}

func (s *Service) GetJob(tenantID, jobID string) (IssueJob, error) {
	nextJobID := strings.TrimSpace(jobID)
	if nextJobID == "" {
		return IssueJob{}, ErrJobNotFound
	}

	nextTenantID := normalizeTenantID(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.jobs {
		if s.jobs[i].ID == nextJobID {
			if s.jobs[i].TenantID != nextTenantID {
				return IssueJob{}, ErrJobNotFound
			}
			return s.jobs[i], nil
		}
	}

	return IssueJob{}, ErrJobNotFound
}

func (s *Service) RetryJob(tenantID, jobID, targetID, actor string) (IssueJob, error) {
	nextJobID := strings.TrimSpace(jobID)
	if nextJobID == "" {
		return IssueJob{}, ErrJobNotFound
	}

	nextTenantID := normalizeTenantID(tenantID)
	nextActor := normalizeActor(actor)
	overrideTargetID := strings.TrimSpace(targetID)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jobs {
		if s.jobs[i].ID != nextJobID {
			continue
		}
		if s.jobs[i].TenantID != nextTenantID {
			return IssueJob{}, ErrJobNotFound
		}
		if s.jobs[i].Status == "success" {
			return IssueJob{}, ErrJobRetryNotAllowed
		}

		templateID := strings.TrimSpace(s.jobs[i].TemplateID)
		if templateID == "" {
			return IssueJob{}, ErrTemplateIDRequired
		}

		template, found := findTemplateByID(s.templates, templateID)
		if !found || template.TenantID != nextTenantID {
			return IssueJob{}, ErrTemplateNotFound
		}
		if template.Status != "active" {
			return IssueJob{}, ErrTemplateInactive
		}

		retryTargetID := strings.TrimSpace(s.jobs[i].TargetID)
		if overrideTargetID != "" {
			retryTargetID = overrideTargetID
		}

		s.jobs[i].RetryCount++
		s.jobs[i].UpdatedAt = now
		s.jobs[i].TargetID = retryTargetID
		s.jobs[i].ErrorCode = ""
		s.jobs[i].ErrorMessage = ""

		if retryTargetID == "" {
			s.jobs[i].Status = "failed"
			s.jobs[i].ErrorCode = "target_id_required"
			s.jobs[i].ErrorMessage = ErrTargetIDRequired.Error()
			s.appendAuditLocked(nextTenantID, "wallet.job.retry", nextActor, s.jobs[i].ID, "failed")
			if err := s.persistLocked(); err != nil {
				return IssueJob{}, err
			}
			return s.jobs[i], nil
		}

		record, err := s.createPassRecord(
			nextTenantID,
			template,
			s.jobs[i].TargetType,
			retryTargetID,
			"", "",
			s.jobs[i].ExpiresAt,
			nextActor,
			now,
		)
		if err != nil {
			s.jobs[i].Status = "failed"
			s.jobs[i].ErrorCode = "issue_failed"
			s.jobs[i].ErrorMessage = err.Error()
			s.appendAuditLocked(nextTenantID, "wallet.job.retry", nextActor, s.jobs[i].ID, "failed")
			if persistErr := s.persistLocked(); persistErr != nil {
				return IssueJob{}, persistErr
			}
			return s.jobs[i], nil
		}

		s.passes = append([]PassInstance{record}, s.passes...)
		s.jobs[i].PassID = record.ID
		s.jobs[i].Status = "success"
		s.jobs[i].ErrorCode = ""
		s.jobs[i].ErrorMessage = ""

		s.appendAuditLocked(nextTenantID, "wallet.job.retry", nextActor, s.jobs[i].ID, "success")
		if err := s.persistLocked(); err != nil {
			return IssueJob{}, err
		}
		return s.jobs[i], nil
	}

	return IssueJob{}, ErrJobNotFound
}

func (s *Service) RequeueDLQJob(tenantID, jobID, targetID, actor string) (IssueJob, error) {
	nextJobID := strings.TrimSpace(jobID)
	if nextJobID == "" {
		return IssueJob{}, ErrJobNotFound
	}

	nextTenantID := normalizeTenantID(tenantID)
	nextActor := normalizeActor(actor)
	overrideTargetID := strings.TrimSpace(targetID)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jobs {
		if s.jobs[i].ID != nextJobID {
			continue
		}
		if s.jobs[i].TenantID != nextTenantID {
			return IssueJob{}, ErrJobNotFound
		}
		if s.jobs[i].Status != "dlq" {
			return IssueJob{}, ErrJobNotInDLQ
		}

		if overrideTargetID != "" {
			s.jobs[i].TargetID = overrideTargetID
		}
		if strings.TrimSpace(s.jobs[i].TargetID) == "" {
			return IssueJob{}, ErrTargetIDRequired
		}

		s.jobs[i].Status = "pending"
		s.jobs[i].RetryCount = 0
		s.jobs[i].ErrorCode = ""
		s.jobs[i].ErrorMessage = ""
		s.jobs[i].UpdatedAt = now

		s.appendAuditLocked(nextTenantID, "wallet.job.dlq_requeue", nextActor, s.jobs[i].ID, "success")
		if err := s.persistLocked(); err != nil {
			return IssueJob{}, err
		}
		return s.jobs[i], nil
	}

	return IssueJob{}, ErrJobNotFound
}

func (s *Service) RequeueDLQJobs(options JobDLQRequeueOptions) (JobDLQRequeueResult, error) {
	resolvedOptions, err := normalizeJobDLQRequeueOptions(options)
	if err != nil {
		return JobDLQRequeueResult{}, err
	}

	result := JobDLQRequeueResult{
		TenantID:  resolvedOptions.TenantID,
		Limit:     resolvedOptions.Limit,
		UpdatedAt: time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jobs {
		if result.Requeued >= resolvedOptions.Limit {
			break
		}
		if s.jobs[i].TenantID != resolvedOptions.TenantID {
			continue
		}
		if s.jobs[i].Status != "dlq" {
			continue
		}
		if resolvedOptions.ErrorCode != "" && strings.TrimSpace(s.jobs[i].ErrorCode) != resolvedOptions.ErrorCode {
			continue
		}

		targetID := strings.TrimSpace(s.jobs[i].TargetID)
		if resolvedOptions.TargetIDOverride != "" {
			targetID = resolvedOptions.TargetIDOverride
		}
		if targetID == "" {
			result.Skipped++
			result.ProcessedJobs = append(result.ProcessedJobs, s.jobs[i].ID)
			continue
		}

		s.jobs[i].TargetID = targetID
		s.jobs[i].Status = "pending"
		s.jobs[i].RetryCount = 0
		s.jobs[i].ErrorCode = ""
		s.jobs[i].ErrorMessage = ""
		s.jobs[i].UpdatedAt = time.Now().UTC()
		result.Requeued++
		result.ProcessedJobs = append(result.ProcessedJobs, s.jobs[i].ID)
		s.appendAuditLocked(s.jobs[i].TenantID, "wallet.job.dlq_requeue_batch", resolvedOptions.Actor, s.jobs[i].ID, "success")
	}

	result.RemainingDLQ = countDLQJobsByTenantLocked(s.jobs, resolvedOptions.TenantID)
	if err := s.persistLocked(); err != nil {
		return JobDLQRequeueResult{}, err
	}
	return result, nil
}

func (s *Service) CleanupDLQJobs(options JobDLQCleanupOptions) (JobDLQCleanupResult, error) {
	resolvedOptions, err := normalizeJobDLQCleanupOptions(options)
	if err != nil {
		return JobDLQCleanupResult{}, err
	}

	result := JobDLQCleanupResult{
		TenantID:  resolvedOptions.TenantID,
		Limit:     resolvedOptions.Limit,
		UpdatedAt: time.Now().UTC(),
	}

	cutoff := time.Now().UTC().Add(-resolvedOptions.OlderThan)

	s.mu.Lock()
	defer s.mu.Unlock()

	nextJobs := make([]IssueJob, 0, len(s.jobs))
	for i := range s.jobs {
		if result.Removed >= resolvedOptions.Limit {
			nextJobs = append(nextJobs, s.jobs[i])
			continue
		}
		if s.jobs[i].TenantID != resolvedOptions.TenantID {
			nextJobs = append(nextJobs, s.jobs[i])
			continue
		}
		if s.jobs[i].Status != "dlq" {
			nextJobs = append(nextJobs, s.jobs[i])
			continue
		}
		if resolvedOptions.ErrorCode != "" && strings.TrimSpace(s.jobs[i].ErrorCode) != resolvedOptions.ErrorCode {
			nextJobs = append(nextJobs, s.jobs[i])
			continue
		}
		if s.jobs[i].UpdatedAt.After(cutoff) {
			nextJobs = append(nextJobs, s.jobs[i])
			continue
		}

		result.Removed++
		result.ProcessedJobs = append(result.ProcessedJobs, s.jobs[i].ID)
		s.appendAuditLocked(s.jobs[i].TenantID, "wallet.job.dlq_cleanup", resolvedOptions.Actor, s.jobs[i].ID, "success")
	}

	s.jobs = nextJobs
	result.RemainingDLQ = countDLQJobsByTenantLocked(s.jobs, resolvedOptions.TenantID)
	s.appendDLQCleanupArchiveLocked(resolvedOptions, result)
	if err := s.persistLocked(); err != nil {
		return JobDLQCleanupResult{}, err
	}
	return result, nil
}

func (s *Service) GetJobSummary(tenantID string, maxRetry int) JobSummary {
	nextTenantID := normalizeTenantID(tenantID)
	if maxRetry <= 0 {
		maxRetry = 3
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return buildJobSummary(nextTenantID, maxRetry, s.jobs, time.Now().UTC())
}

func (s *Service) GetJobMetrics(tenantID string, window time.Duration, maxRetry, dlqAlertThreshold int) JobMetrics {
	nextTenantID := normalizeTenantID(tenantID)
	if maxRetry <= 0 {
		maxRetry = 3
	}
	if window < time.Second {
		window = 15 * time.Minute
	}
	if dlqAlertThreshold <= 0 {
		dlqAlertThreshold = 20
	}

	now := time.Now().UTC()
	since := now.Add(-window)

	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := buildJobSummary(nextTenantID, maxRetry, s.jobs, now)
	metrics := JobMetrics{
		TenantID:          nextTenantID,
		MaxRetry:          maxRetry,
		DLQAlertThreshold: dlqAlertThreshold,
		Summary:           summary,
		Window: JobMetricsWindow{
			WindowSeconds:      int64(window.Seconds()),
			Since:              since,
			Until:              now,
			ErrorCodeBreakdown: map[string]int{},
		},
		UpdatedAt: now,
	}

	dlqErrorCodeBreakdown := map[string]int{}
	for i := range s.jobs {
		if s.jobs[i].TenantID != nextTenantID {
			continue
		}
		if s.jobs[i].Status == "dlq" {
			errorCode := strings.TrimSpace(s.jobs[i].ErrorCode)
			if errorCode == "" {
				errorCode = "unknown"
			}
			dlqErrorCodeBreakdown[errorCode]++
		}

		updatedAt := s.jobs[i].UpdatedAt
		if updatedAt.Before(since) || updatedAt.After(now) {
			continue
		}
		metrics.Window.Updated++
		switch s.jobs[i].Status {
		case "pending":
			metrics.Window.Pending++
		case "processing":
			metrics.Window.Processing++
		case "success":
			metrics.Window.Success++
		case "failed":
			metrics.Window.Failed++
		case "dlq":
			metrics.Window.DLQ++
		}

		errorCode := strings.TrimSpace(s.jobs[i].ErrorCode)
		if errorCode != "" {
			metrics.Window.ErrorCodeBreakdown[errorCode]++
		}
	}

	for i := range s.jobs {
		if s.jobs[i].TenantID != nextTenantID {
			continue
		}
		createdAt := s.jobs[i].CreatedAt
		if createdAt.Before(since) || createdAt.After(now) {
			continue
		}
		metrics.Window.Created++
	}

	if len(metrics.Window.ErrorCodeBreakdown) == 0 {
		metrics.Window.ErrorCodeBreakdown = nil
	}

	metrics.Alerts = buildJobMetricsAlerts(dlqErrorCodeBreakdown, dlqAlertThreshold)
	return metrics
}

func (s *Service) GetJobMetricsTrend(
	tenantID string,
	window time.Duration,
	bucketCount, maxRetry, dlqAlertThreshold int,
) JobMetricsTrend {
	nextTenantID := normalizeTenantID(tenantID)
	if maxRetry <= 0 {
		maxRetry = 3
	}
	if window < time.Second {
		window = 15 * time.Minute
	}
	if dlqAlertThreshold <= 0 {
		dlqAlertThreshold = 20
	}
	if bucketCount <= 0 {
		bucketCount = 12
	}
	if bucketCount > 120 {
		bucketCount = 120
	}
	if maxBucketsBySeconds := int(window.Seconds()); maxBucketsBySeconds > 0 && bucketCount > maxBucketsBySeconds {
		bucketCount = maxBucketsBySeconds
	}
	if bucketCount <= 0 {
		bucketCount = 1
	}

	now := time.Now().UTC()
	since := now.Add(-window)
	windowNS := window.Nanoseconds()
	bucketSeconds := window.Seconds() / float64(bucketCount)

	s.mu.RLock()
	defer s.mu.RUnlock()

	trend := JobMetricsTrend{
		TenantID:          nextTenantID,
		MaxRetry:          maxRetry,
		DLQAlertThreshold: dlqAlertThreshold,
		WindowSeconds:     int64(window.Seconds()),
		BucketSeconds:     int64(bucketSeconds),
		BucketCount:       bucketCount,
		Since:             since,
		Until:             now,
		Summary:           buildJobSummary(nextTenantID, maxRetry, s.jobs, now),
		Buckets:           make([]JobMetricsTrendBucket, bucketCount),
		UpdatedAt:         now,
	}
	if trend.BucketSeconds < 1 {
		trend.BucketSeconds = 1
	}

	for i := range trend.Buckets {
		startNS := windowNS * int64(i) / int64(bucketCount)
		endNS := windowNS * int64(i+1) / int64(bucketCount)
		end := since.Add(time.Duration(endNS))
		if i == bucketCount-1 {
			end = now
		}
		trend.Buckets[i] = JobMetricsTrendBucket{
			Index:              i,
			Start:              since.Add(time.Duration(startNS)),
			End:                end,
			ErrorCodeBreakdown: map[string]int{},
		}
	}

	dlqErrorCodeBreakdown := map[string]int{}
	for i := range s.jobs {
		if s.jobs[i].TenantID != nextTenantID {
			continue
		}

		if s.jobs[i].Status == "dlq" {
			errorCode := strings.TrimSpace(s.jobs[i].ErrorCode)
			if errorCode == "" {
				errorCode = "unknown"
			}
			dlqErrorCodeBreakdown[errorCode]++
		}

		if idx, ok := resolveMetricsTrendBucketIndex(s.jobs[i].UpdatedAt, since, now, bucketCount, windowNS); ok {
			trend.Buckets[idx].Updated++
			switch s.jobs[i].Status {
			case "pending":
				trend.Buckets[idx].Pending++
			case "processing":
				trend.Buckets[idx].Processing++
			case "success":
				trend.Buckets[idx].Success++
			case "failed":
				trend.Buckets[idx].Failed++
			case "dlq":
				trend.Buckets[idx].DLQ++
			}
			errorCode := strings.TrimSpace(s.jobs[i].ErrorCode)
			if errorCode != "" {
				trend.Buckets[idx].ErrorCodeBreakdown[errorCode]++
			}
		}

		if idx, ok := resolveMetricsTrendBucketIndex(s.jobs[i].CreatedAt, since, now, bucketCount, windowNS); ok {
			trend.Buckets[idx].Created++
		}
	}

	for i := range trend.Buckets {
		if len(trend.Buckets[i].ErrorCodeBreakdown) == 0 {
			trend.Buckets[i].ErrorCodeBreakdown = nil
		}
	}
	trend.Alerts = buildJobMetricsAlerts(dlqErrorCodeBreakdown, dlqAlertThreshold)
	return trend
}

func resolveMetricsTrendBucketIndex(
	timestamp, since, until time.Time,
	bucketCount int,
	windowNS int64,
) (int, bool) {
	if timestamp.Before(since) || timestamp.After(until) || bucketCount <= 0 || windowNS <= 0 {
		return 0, false
	}
	offset := timestamp.Sub(since).Nanoseconds()
	index := int(offset * int64(bucketCount) / windowNS)
	if index < 0 {
		return 0, false
	}
	if index >= bucketCount {
		index = bucketCount - 1
	}
	return index, true
}

func buildJobSummary(tenantID string, maxRetry int, jobs []IssueJob, now time.Time) JobSummary {
	summary := JobSummary{
		TenantID:           tenantID,
		MaxRetry:           maxRetry,
		ErrorCodeBreakdown: map[string]int{},
		UpdatedAt:          now,
	}

	for i := range jobs {
		if jobs[i].TenantID != tenantID {
			continue
		}
		summary.Total++
		switch jobs[i].Status {
		case "pending":
			summary.Pending++
		case "processing":
			summary.Processing++
		case "success":
			summary.Success++
		case "failed":
			summary.Failed++
		case "dlq":
			summary.DLQ++
		}

		if jobs[i].Status == "failed" {
			if isAutoRetryableJob(jobs[i], maxRetry) {
				summary.RetryableFailed++
			} else {
				summary.NonRetryableFailed++
			}
		}
		if jobs[i].Status == "dlq" {
			summary.NonRetryableFailed++
		}

		errorCode := strings.TrimSpace(jobs[i].ErrorCode)
		if errorCode != "" {
			summary.ErrorCodeBreakdown[errorCode]++
		}
	}
	if len(summary.ErrorCodeBreakdown) == 0 {
		summary.ErrorCodeBreakdown = nil
	}
	return summary
}

func buildJobMetricsAlerts(dlqErrorCodeBreakdown map[string]int, threshold int) []JobMetricsAlert {
	items := make([]JobMetricsAlert, 0, len(dlqErrorCodeBreakdown))
	for errorCode, count := range dlqErrorCodeBreakdown {
		if count < threshold {
			continue
		}
		items = append(items, JobMetricsAlert{
			Type:      "dlq_error_code_threshold",
			ErrorCode: errorCode,
			Count:     count,
			Threshold: threshold,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].ErrorCode < items[j].ErrorCode
		}
		return items[i].Count > items[j].Count
	})
	return items
}

type jobProcessOutcome struct {
	jobID   string
	status  string
	retried bool
}

func (s *Service) ProcessIssueJobs(options JobProcessOptions) (JobProcessResult, error) {
	resolvedOptions, err := normalizeJobProcessOptions(options)
	if err != nil {
		return JobProcessResult{}, err
	}

	result := JobProcessResult{
		TenantID:    resolvedOptions.TenantID,
		Limit:       resolvedOptions.Limit,
		WorkerCount: resolvedOptions.WorkerCount,
		MaxRetry:    resolvedOptions.MaxRetry,
		StartedAt:   time.Now().UTC(),
	}

	s.mu.Lock()
	claimedJobIDs := s.claimProcessableJobsLocked(resolvedOptions.TenantID, resolvedOptions.Limit, resolvedOptions.MaxRetry)
	s.mu.Unlock()

	result.Claimed = len(claimedJobIDs)
	if len(claimedJobIDs) == 0 {
		result.PendingAfter = s.countProcessableJobs(resolvedOptions.TenantID, resolvedOptions.MaxRetry)
		result.CompletedAt = time.Now().UTC()
		return result, nil
	}

	workerCount := resolvedOptions.WorkerCount
	if workerCount > len(claimedJobIDs) {
		workerCount = len(claimedJobIDs)
	}

	jobCh := make(chan string, len(claimedJobIDs))
	outcomeCh := make(chan jobProcessOutcome, len(claimedJobIDs))

	for i := range claimedJobIDs {
		jobCh <- claimedJobIDs[i]
	}
	close(jobCh)

	var wg sync.WaitGroup
	wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done()
			for jobID := range jobCh {
				outcome := s.processClaimedIssueJob(jobID, resolvedOptions)
				outcomeCh <- outcome
			}
		}()
	}
	wg.Wait()
	close(outcomeCh)

	for outcome := range outcomeCh {
		result.ProcessedJobIDs = append(result.ProcessedJobIDs, outcome.jobID)
		if outcome.retried {
			result.Retried++
		}
		switch outcome.status {
		case "success":
			result.Succeeded++
		case "failed":
			result.Failed++
		case "dlq":
			result.DLQ++
		default:
			result.Skipped++
		}
	}

	result.PendingAfter = s.countProcessableJobs(resolvedOptions.TenantID, resolvedOptions.MaxRetry)
	result.CompletedAt = time.Now().UTC()
	return result, nil
}

func normalizeJobDLQRequeueOptions(input JobDLQRequeueOptions) (JobDLQRequeueOptions, error) {
	output := JobDLQRequeueOptions{
		TenantID:         normalizeTenantID(input.TenantID),
		Limit:            input.Limit,
		ErrorCode:        strings.TrimSpace(input.ErrorCode),
		TargetIDOverride: strings.TrimSpace(input.TargetIDOverride),
		Actor:            normalizeActor(input.Actor),
	}

	if output.Limit <= 0 {
		output.Limit = 20
	}
	if output.Limit > 500 {
		return JobDLQRequeueOptions{}, ErrInvalidJobDLQOptions
	}

	return output, nil
}

func normalizeJobDLQCleanupOptions(input JobDLQCleanupOptions) (JobDLQCleanupOptions, error) {
	output := JobDLQCleanupOptions{
		TenantID:  normalizeTenantID(input.TenantID),
		Limit:     input.Limit,
		ErrorCode: strings.TrimSpace(input.ErrorCode),
		OlderThan: input.OlderThan,
		Actor:     normalizeActor(input.Actor),
	}

	if output.Limit <= 0 {
		output.Limit = 50
	}
	if output.Limit > 1000 {
		return JobDLQCleanupOptions{}, ErrInvalidJobDLQOptions
	}
	if output.OlderThan <= 0 {
		output.OlderThan = 24 * time.Hour
	}
	if output.OlderThan > 365*24*time.Hour {
		return JobDLQCleanupOptions{}, ErrInvalidJobDLQOptions
	}

	return output, nil
}

func normalizeJobProcessOptions(input JobProcessOptions) (JobProcessOptions, error) {
	output := JobProcessOptions{
		TenantID:    normalizeTenantID(input.TenantID),
		Limit:       input.Limit,
		WorkerCount: input.WorkerCount,
		MaxRetry:    input.MaxRetry,
		BaseBackoff: input.BaseBackoff,
		MaxBackoff:  input.MaxBackoff,
		Actor:       normalizeActor(input.Actor),
	}

	if output.Limit <= 0 {
		output.Limit = 20
	}
	if output.Limit > 500 {
		return JobProcessOptions{}, ErrInvalidJobProcessOptions
	}

	if output.WorkerCount <= 0 {
		output.WorkerCount = 2
	}
	if output.WorkerCount > 32 {
		return JobProcessOptions{}, ErrInvalidJobProcessOptions
	}

	if output.MaxRetry < 0 {
		return JobProcessOptions{}, ErrInvalidJobProcessOptions
	}
	if output.MaxRetry == 0 {
		output.MaxRetry = 3
	}
	if output.MaxRetry > 20 {
		return JobProcessOptions{}, ErrInvalidJobProcessOptions
	}

	if output.BaseBackoff < 0 || output.MaxBackoff < 0 {
		return JobProcessOptions{}, ErrInvalidJobProcessOptions
	}
	if output.BaseBackoff == 0 {
		output.BaseBackoff = 200 * time.Millisecond
	}
	if output.MaxBackoff == 0 {
		output.MaxBackoff = 5 * time.Second
	}
	if output.MaxBackoff < output.BaseBackoff {
		return JobProcessOptions{}, ErrInvalidJobProcessOptions
	}

	return output, nil
}

func (s *Service) claimProcessableJobsLocked(tenantID string, limit, maxRetry int) []string {
	jobIDs := make([]string, 0, limit)
	now := time.Now().UTC()
	for i := range s.jobs {
		if len(jobIDs) >= limit {
			break
		}
		if s.jobs[i].TenantID != tenantID {
			continue
		}
		if s.jobs[i].Status == "pending" {
			s.jobs[i].Status = "processing"
			s.jobs[i].UpdatedAt = now
			s.jobs[i].ErrorCode = ""
			s.jobs[i].ErrorMessage = ""
			jobIDs = append(jobIDs, s.jobs[i].ID)
			continue
		}
		if !isAutoRetryableJob(s.jobs[i], maxRetry) {
			continue
		}
		s.jobs[i].RetryCount++
		s.jobs[i].Status = "processing"
		s.jobs[i].UpdatedAt = now
		s.jobs[i].ErrorCode = ""
		s.jobs[i].ErrorMessage = ""
		jobIDs = append(jobIDs, s.jobs[i].ID)
	}
	return jobIDs
}

func (s *Service) countProcessableJobs(tenantID string, maxRetry int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for i := range s.jobs {
		if s.jobs[i].TenantID != tenantID {
			continue
		}
		if s.jobs[i].Status == "pending" || isAutoRetryableJob(s.jobs[i], maxRetry) {
			count++
		}
	}
	return count
}

func isAutoRetryableJob(job IssueJob, maxRetry int) bool {
	return job.Status == "failed" &&
		strings.TrimSpace(job.ErrorCode) == "issue_failed" &&
		job.RetryCount < maxRetry
}

func (s *Service) processClaimedIssueJob(jobID string, options JobProcessOptions) jobProcessOutcome {
	s.mu.RLock()
	jobIndex := -1
	for i := range s.jobs {
		if s.jobs[i].ID == jobID {
			jobIndex = i
			break
		}
	}
	if jobIndex < 0 {
		s.mu.RUnlock()
		return jobProcessOutcome{jobID: jobID, status: "skipped"}
	}
	job := s.jobs[jobIndex]
	s.mu.RUnlock()

	if job.Status != "processing" {
		return jobProcessOutcome{jobID: jobID, status: "skipped"}
	}

	retried := job.RetryCount > 0
	if retried {
		sleepDuration := exponentialBackoff(job.RetryCount, options.BaseBackoff, options.MaxBackoff)
		if sleepDuration > 0 {
			time.Sleep(sleepDuration)
		}
	}

	if strings.TrimSpace(job.TargetID) == "" {
		status := s.setJobFailure(jobID, "target_id_required", ErrTargetIDRequired.Error(), options.Actor, false, options.MaxRetry)
		return jobProcessOutcome{jobID: jobID, status: status, retried: retried}
	}

	s.mu.RLock()
	template, found := findTemplateByID(s.templates, strings.TrimSpace(job.TemplateID))
	s.mu.RUnlock()
	if !found || template.TenantID != options.TenantID {
		status := s.setJobFailure(jobID, "template_not_found", ErrTemplateNotFound.Error(), options.Actor, false, options.MaxRetry)
		return jobProcessOutcome{jobID: jobID, status: status, retried: retried}
	}
	if template.Status != "active" {
		status := s.setJobFailure(jobID, "template_inactive", ErrTemplateInactive.Error(), options.Actor, false, options.MaxRetry)
		return jobProcessOutcome{jobID: jobID, status: status, retried: retried}
	}

	record, err := s.createPassRecord(
		options.TenantID,
		template,
		job.TargetType,
		job.TargetID,
		"", "",
		job.ExpiresAt,
		options.Actor,
		time.Now().UTC(),
	)
	if err != nil {
		status := s.setJobFailure(jobID, "issue_failed", err.Error(), options.Actor, true, options.MaxRetry)
		return jobProcessOutcome{jobID: jobID, status: status, retried: retried}
	}

	if updateErr := s.setJobSuccess(jobID, record, options.Actor); updateErr != nil {
		return jobProcessOutcome{jobID: jobID, status: "failed", retried: retried}
	}

	return jobProcessOutcome{jobID: jobID, status: "success", retried: retried}
}

func (s *Service) setJobFailure(jobID, errorCode, errorMessage, actor string, retryable bool, maxRetry int) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jobs {
		if s.jobs[i].ID != jobID {
			continue
		}
		s.jobs[i].Status = resolveJobFailureStatus(s.jobs[i], strings.TrimSpace(errorCode), retryable, maxRetry)
		s.jobs[i].ErrorCode = strings.TrimSpace(errorCode)
		s.jobs[i].ErrorMessage = strings.TrimSpace(errorMessage)
		s.jobs[i].UpdatedAt = time.Now().UTC()
		auditResult := "failed"
		if s.jobs[i].Status == "dlq" {
			auditResult = "dlq"
		}
		s.appendAuditLocked(s.jobs[i].TenantID, "wallet.job.process", normalizeActor(actor), s.jobs[i].ID, auditResult)
		_ = s.persistLocked()
		return s.jobs[i].Status
	}
	return "skipped"
}

func resolveJobFailureStatus(job IssueJob, errorCode string, retryable bool, maxRetry int) string {
	if !retryable {
		return "dlq"
	}
	if strings.TrimSpace(errorCode) != "issue_failed" {
		return "dlq"
	}
	if maxRetry <= 0 {
		maxRetry = 3
	}
	if job.RetryCount >= maxRetry {
		return "dlq"
	}
	return "failed"
}

func countDLQJobsByTenantLocked(items []IssueJob, tenantID string) int {
	count := 0
	for i := range items {
		if items[i].TenantID != tenantID {
			continue
		}
		if items[i].Status == "dlq" {
			count++
		}
	}
	return count
}

func (s *Service) setJobSuccess(jobID string, record PassInstance, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jobs {
		if s.jobs[i].ID != jobID {
			continue
		}
		if s.jobs[i].Status != "processing" {
			return nil
		}
		s.passes = append([]PassInstance{record}, s.passes...)
		s.jobs[i].PassID = record.ID
		s.jobs[i].Status = "success"
		s.jobs[i].ErrorCode = ""
		s.jobs[i].ErrorMessage = ""
		s.jobs[i].UpdatedAt = time.Now().UTC()
		s.appendAuditLocked(s.jobs[i].TenantID, "wallet.job.process", normalizeActor(actor), s.jobs[i].ID, "success")
		return s.persistLocked()
	}
	return ErrJobNotFound
}

func exponentialBackoff(retryCount int, base, max time.Duration) time.Duration {
	if retryCount <= 0 || base <= 0 {
		return 0
	}
	delay := base
	for i := 1; i < retryCount; i++ {
		delay *= 2
		if delay >= max {
			return max
		}
	}
	if delay > max {
		return max
	}
	return delay
}

func (s *Service) ListAuditLogs(tenantID string) []AuditLog {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	items := make([]AuditLog, 0, len(s.auditLogs))
	for i := range s.auditLogs {
		if s.auditLogs[i].TenantID != nextTenantID {
			continue
		}
		items = append(items, s.auditLogs[i])
	}
	return items
}

func (s *Service) ListDLQCleanupArchives(tenantID string, limit int) []JobDLQCleanupArchive {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	if limit <= 0 {
		limit = 20
	}
	if limit > 500 {
		limit = 500
	}

	items := make([]JobDLQCleanupArchive, 0, limit)
	for i := range s.dlqCleanupArchives {
		if s.dlqCleanupArchives[i].TenantID != nextTenantID {
			continue
		}
		items = append(items, cloneDLQCleanupArchive(s.dlqCleanupArchives[i]))
		if len(items) >= limit {
			break
		}
	}
	return items
}

func (s *Service) GetJobAlertSubscription(tenantID string) (JobAlertSubscription, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	for i := range s.jobAlertSubscriptions {
		if s.jobAlertSubscriptions[i].TenantID != nextTenantID {
			continue
		}
		return cloneJobAlertSubscription(s.jobAlertSubscriptions[i]), true
	}
	return JobAlertSubscription{}, false
}

func (s *Service) UpsertJobAlertSubscription(
	input JobAlertSubscriptionUpsertOptions,
) (JobAlertSubscription, error) {
	resolved, err := resolveJobAlertSubscriptionUpsertOptions(input)
	if err != nil {
		return JobAlertSubscription{}, err
	}

	now := time.Now().UTC()
	record := JobAlertSubscription{
		TenantID:          resolved.TenantID,
		Enabled:           resolved.Enabled,
		DLQAlertThreshold: resolved.DLQAlertThreshold,
		WindowSeconds:     int64(resolved.Window.Seconds()),
		CooldownSeconds:   int64(resolved.Cooldown.Seconds()),
		Channels: JobAlertSubscriptionChannels{
			Email:    resolved.EmailEnabled,
			WhatsApp: resolved.WhatsAppEnabled,
		},
		ReceiverGroups: normalizeReceiverGroups(resolved.ReceiverGroups),
		UpdatedAt:      now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	upserted := false
	for i := range s.jobAlertSubscriptions {
		if s.jobAlertSubscriptions[i].TenantID != resolved.TenantID {
			continue
		}
		s.jobAlertSubscriptions[i] = cloneJobAlertSubscription(record)
		upserted = true
		break
	}
	if !upserted {
		s.jobAlertSubscriptions = append(
			[]JobAlertSubscription{cloneJobAlertSubscription(record)},
			s.jobAlertSubscriptions...,
		)
	}
	s.appendAuditLocked(
		resolved.TenantID,
		"wallet.job.alert_subscription.upsert",
		normalizeActor(resolved.Actor),
		resolved.TenantID,
		"success",
	)
	if err := s.persistLocked(); err != nil {
		return JobAlertSubscription{}, err
	}
	return cloneJobAlertSubscription(record), nil
}

func (s *Service) ListJobAlertNotifications(tenantID string, limit int) []JobAlertNotification {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	items := make([]JobAlertNotification, 0, limit)
	for i := range s.jobAlertNotifications {
		if s.jobAlertNotifications[i].TenantID != nextTenantID {
			continue
		}
		items = append(items, cloneJobAlertNotification(s.jobAlertNotifications[i]))
		if len(items) >= limit {
			break
		}
	}
	return items
}

func (s *Service) DispatchJobMetricsAlerts(
	subscription JobAlertSubscription,
	alerts []JobMetricsAlert,
	actor string,
) (JobAlertDispatchResult, error) {
	now := time.Now().UTC()
	nextTenantID := normalizeTenantID(subscription.TenantID)
	cooldown := time.Duration(subscription.CooldownSeconds) * time.Second
	if cooldown < 0 {
		cooldown = 0
	}
	nextActor := normalizeActor(actor)
	channels := resolveAlertChannels(subscription.Channels)
	receiverGroups := normalizeReceiverGroups(subscription.ReceiverGroups)
	if len(receiverGroups) == 0 {
		receiverGroups = []string{"security"}
	}

	result := JobAlertDispatchResult{
		TenantID:    nextTenantID,
		TotalAlerts: len(alerts),
		UpdatedAt:   now,
	}
	if len(alerts) == 0 {
		return result, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result.Items = make([]JobAlertNotification, 0, len(alerts))
	for i := range alerts {
		errorCode := alertdispatch.NormalizeErrorCode(alerts[i].ErrorCode)
		planned := alertdispatch.Plan(alertdispatch.PlanInput{
			Subscription: alertdispatch.Subscription{
				TenantID:       nextTenantID,
				Enabled:        subscription.Enabled,
				Channels:       channels,
				ReceiverGroups: receiverGroups,
			},
			Alert: alertdispatch.Alert{
				Type:      alerts[i].Type,
				ErrorCode: alerts[i].ErrorCode,
				Count:     alerts[i].Count,
				Threshold: alerts[i].Threshold,
			},
			InCooldown: s.isJobAlertInCooldownLocked(nextTenantID, errorCode, cooldown, now),
		})

		record := JobAlertNotification{
			TenantID:       planned.TenantID,
			Type:           planned.Type,
			ErrorCode:      planned.ErrorCode,
			Count:          planned.Count,
			Threshold:      planned.Threshold,
			Channels:       append([]string(nil), planned.Channels...),
			ReceiverGroups: append([]string(nil), planned.ReceiverGroups...),
			Status:         planned.Status,
			Reason:         planned.Reason,
			IdempotencyKey: planned.IdempotencyKey,
			Provider:       s.jobAlertEmailProvider,
			TriggeredAt:    now,
		}
		if record.Status == "ready" {
			attempt := s.nextJobAlertAttemptLocked(nextTenantID, planned.IdempotencyKey)
			record.Attempt = attempt
			status, reason, provider, providerError, retryable, channelResults := s.dispatchJobAlertWithProviderLocked(record, attempt)
			record.Status = status
			record.Reason = reason
			record.Provider = provider
			record.ProviderError = providerError
			record.Retryable = retryable
			record.ChannelResults = cloneJobAlertChannelResults(channelResults)
		} else {
			record.ChannelResults = buildStaticJobAlertChannelResults(record.Channels, record.Status, record.Reason)
		}

		id, err := walletID("wan_")
		if err != nil {
			id = fmt.Sprintf("wan_fallback_%d", time.Now().UnixNano())
		}
		record.ID = id

		if record.Status == "sent" {
			result.Dispatched++
			s.upsertJobAlertCooldownLocked(nextTenantID, errorCode, now)
			s.appendAuditLocked(nextTenantID, "wallet.job.alert.dispatch", nextActor, errorCode, "sent")
		} else if record.Status == "failed" {
			result.Failed++
			auditResult := "failed"
			if record.Reason != "" {
				auditResult = "failed:" + record.Reason
			}
			s.appendAuditLocked(nextTenantID, "wallet.job.alert.dispatch", nextActor, errorCode, auditResult)
		} else {
			result.Skipped++
			auditResult := "skipped"
			if record.Reason != "" {
				auditResult = "skipped:" + record.Reason
			}
			s.appendAuditLocked(nextTenantID, "wallet.job.alert.dispatch", nextActor, errorCode, auditResult)
		}

		result.Items = append(result.Items, cloneJobAlertNotification(record))
	}

	s.jobAlertNotifications = append(cloneJobAlertNotifications(result.Items), s.jobAlertNotifications...)
	if len(s.jobAlertNotifications) > 5000 {
		s.jobAlertNotifications = s.jobAlertNotifications[:5000]
	}

	if err := s.persistLocked(); err != nil {
		return JobAlertDispatchResult{}, err
	}
	return result, nil
}

func (s *Service) RetryJobAlertNotification(tenantID, notificationID, actor string) (JobAlertNotification, error) {
	nextTenantID := normalizeTenantID(tenantID)
	nextNotificationID := strings.TrimSpace(notificationID)
	if nextNotificationID == "" {
		return JobAlertNotification{}, ErrJobAlertNotificationNotFound
	}
	nextActor := normalizeActor(actor)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	var source JobAlertNotification
	found := false
	for i := range s.jobAlertNotifications {
		if s.jobAlertNotifications[i].TenantID != nextTenantID {
			continue
		}
		if s.jobAlertNotifications[i].ID != nextNotificationID {
			continue
		}
		source = cloneJobAlertNotification(s.jobAlertNotifications[i])
		found = true
		break
	}
	if !found {
		return JobAlertNotification{}, ErrJobAlertNotificationNotFound
	}
	if source.Status != "failed" || !source.Retryable {
		return JobAlertNotification{}, ErrJobAlertRetryNotAllowed
	}

	idempotencyKey := strings.TrimSpace(source.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = buildJobAlertNotificationIdempotencyKey(
			nextTenantID,
			strings.TrimSpace(source.Type),
			strings.TrimSpace(source.ErrorCode),
			source.Threshold,
		)
	}

	record := cloneJobAlertNotification(source)
	id, err := walletID("wan_")
	if err != nil {
		id = fmt.Sprintf("wan_fallback_%d", time.Now().UnixNano())
	}
	record.ID = id
	record.IdempotencyKey = idempotencyKey
	record.SourceNotificationID = source.ID
	record.TriggeredAt = now
	record.Provider = s.jobAlertEmailProvider
	record.ProviderError = ""
	record.Reason = ""
	record.Retryable = false

	if s.hasSentJobAlertByIdempotencyLocked(nextTenantID, idempotencyKey) {
		record.Status = "skipped"
		record.Reason = "idempotent_already_sent"
		record.ChannelResults = buildStaticJobAlertChannelResults(record.Channels, record.Status, record.Reason)
		auditResult := "skipped:" + record.Reason
		s.appendAuditLocked(nextTenantID, "wallet.job.alert.retry", nextActor, source.ID, auditResult)
	} else {
		attempt := s.nextJobAlertAttemptLocked(nextTenantID, idempotencyKey)
		record.Attempt = attempt
		status, reason, provider, providerError, retryable, channelResults := s.dispatchJobAlertWithProviderLocked(record, attempt)
		record.Status = status
		record.Reason = reason
		record.Provider = provider
		record.ProviderError = providerError
		record.Retryable = retryable
		record.ChannelResults = cloneJobAlertChannelResults(channelResults)
		if record.Status == "sent" {
			if record.Reason == "" {
				record.ProviderError = ""
			}
			s.upsertJobAlertCooldownLocked(nextTenantID, record.ErrorCode, now)
			s.appendAuditLocked(nextTenantID, "wallet.job.alert.retry", nextActor, source.ID, "sent")
		} else {
			auditResult := "failed"
			if record.Reason != "" {
				auditResult = "failed:" + record.Reason
			}
			s.appendAuditLocked(nextTenantID, "wallet.job.alert.retry", nextActor, source.ID, auditResult)
		}
	}

	s.jobAlertNotifications = append([]JobAlertNotification{cloneJobAlertNotification(record)}, s.jobAlertNotifications...)
	if len(s.jobAlertNotifications) > 5000 {
		s.jobAlertNotifications = s.jobAlertNotifications[:5000]
	}

	if err := s.persistLocked(); err != nil {
		return JobAlertNotification{}, err
	}
	return cloneJobAlertNotification(record), nil
}

func (s *Service) nextJobAlertAttemptLocked(tenantID, idempotencyKey string) int {
	nextAttempt := 1
	for i := range s.jobAlertNotifications {
		if s.jobAlertNotifications[i].TenantID != tenantID {
			continue
		}
		if s.jobAlertNotifications[i].IdempotencyKey != idempotencyKey {
			continue
		}
		if s.jobAlertNotifications[i].Attempt >= nextAttempt {
			nextAttempt = s.jobAlertNotifications[i].Attempt + 1
		}
	}
	return nextAttempt
}

func (s *Service) hasSentJobAlertByIdempotencyLocked(tenantID, idempotencyKey string) bool {
	if idempotencyKey == "" {
		return false
	}
	for i := range s.jobAlertNotifications {
		if s.jobAlertNotifications[i].TenantID != tenantID {
			continue
		}
		if s.jobAlertNotifications[i].IdempotencyKey != idempotencyKey {
			continue
		}
		if s.jobAlertNotifications[i].Status == "sent" {
			return true
		}
	}
	return false
}

func (s *Service) dispatchJobAlertWithProviderLocked(
	record JobAlertNotification,
	attempt int,
) (status, reason, provider, providerError string, retryable bool, channelResults []JobAlertChannelResult) {
	if attempt < 1 {
		attempt = 1
	}
	normalizedChannels := normalizeDispatchChannels(record.Channels)
	if len(normalizedChannels) == 0 {
		return "skipped", "channel_disabled", "", "", false, nil
	}

	channelResults = make([]JobAlertChannelResult, 0, len(normalizedChannels))
	for i := range normalizedChannels {
		switch normalizedChannels[i] {
		case "email":
			channelResults = append(channelResults, s.dispatchJobAlertEmailChannelLocked(record, attempt))
		case "whatsapp":
			channelResults = append(channelResults, s.dispatchJobAlertWhatsAppChannelLocked(record))
		default:
			channelResults = append(channelResults, JobAlertChannelResult{
				Channel: normalizedChannels[i],
				Status:  "skipped",
				Reason:  "channel_disabled",
			})
		}
	}

	return summarizeJobAlertChannelResults(channelResults)
}

func (s *Service) dispatchJobAlertEmailChannelLocked(record JobAlertNotification, attempt int) JobAlertChannelResult {
	nextProvider := s.jobAlertEmailProvider
	if nextProvider == "" {
		nextProvider = "mock"
	}

	recipients := s.resolveAlertEmailReceiversLocked(record.ReceiverGroups)
	if len(recipients) == 0 {
		return JobAlertChannelResult{
			Channel:       "email",
			Status:        "failed",
			Reason:        "email_receivers_not_configured",
			Provider:      nextProvider,
			ProviderError: "email receivers not configured",
			Retryable:     false,
		}
	}

	if nextProvider == "resend" || nextProvider == "spaceemail" {
		nextProvider = "resend"
		if s.jobAlertEmailSender == nil {
			return JobAlertChannelResult{
				Channel:       "email",
				Status:        "failed",
				Reason:        "provider_not_configured",
				Provider:      nextProvider,
				ProviderError: "resend sender is not configured",
				Retryable:     false,
				Receivers:     recipients,
			}
		}
		subject, text := buildJobAlertEmailMessage(record)
		sendResult, err := s.jobAlertEmailSender.Send(
			context.Background(),
			AlertEmailSendInput{
				TenantID:       record.TenantID,
				To:             recipients,
				IdempotencyKey: record.IdempotencyKey,
				Subject:        subject,
				Text:           text,
			},
		)
		if err != nil {
			channelRetryable := isJobAlertProviderRetryable(err)
			reason := "provider_error"
			if channelRetryable {
				reason = "provider_transient_error"
			}
			return JobAlertChannelResult{
				Channel:       "email",
				Status:        "failed",
				Reason:        reason,
				Provider:      nextProvider,
				ProviderError: strings.TrimSpace(err.Error()),
				Retryable:     channelRetryable,
				Receivers:     recipients,
			}
		}
		return JobAlertChannelResult{
			Channel:                "email",
			Status:                 "sent",
			Provider:               nextProvider,
			ProviderDeliveryID:     strings.TrimSpace(sendResult.ProviderDeliveryID),
			ProviderDeliveryStatus: strings.TrimSpace(sendResult.ProviderDeliveryStatus),
			Retryable:              false,
			Receivers:              recipients,
		}
	}

	if s.jobAlertMockTransientFailCount > 0 &&
		!s.hasSentJobAlertByIdempotencyLocked(record.TenantID, record.IdempotencyKey) &&
		attempt <= s.jobAlertMockTransientFailCount {
		return JobAlertChannelResult{
			Channel:       "email",
			Status:        "failed",
			Reason:        "provider_transient_error",
			Provider:      "mock",
			ProviderError: "provider_unavailable",
			Retryable:     true,
			Receivers:     recipients,
		}
	}
	return JobAlertChannelResult{
		Channel:   "email",
		Status:    "sent",
		Provider:  "mock",
		Retryable: false,
		Receivers: recipients,
	}
}

func (s *Service) dispatchJobAlertWhatsAppChannelLocked(record JobAlertNotification) JobAlertChannelResult {
	nextProvider := s.jobAlertWhatsAppProvider
	if nextProvider == "" {
		nextProvider = "mock"
	}
	receivers := s.resolveAlertWhatsAppReceiversLocked(record.ReceiverGroups)
	if len(receivers) == 0 {
		return JobAlertChannelResult{
			Channel:       "whatsapp",
			Status:        "failed",
			Reason:        "whatsapp_receivers_not_configured",
			Provider:      nextProvider,
			ProviderError: "whatsapp receivers not configured",
			Retryable:     false,
		}
	}

	if nextProvider == "mock" {
		if s.jobAlertWhatsAppSender == nil {
			return JobAlertChannelResult{
				Channel:       "whatsapp",
				Status:        "failed",
				Reason:        "provider_not_configured",
				Provider:      "mock",
				ProviderError: "mock whatsapp sender is not configured",
				Retryable:     false,
				Receivers:     receivers,
			}
		}
		sendResult, err := s.jobAlertWhatsAppSender.Send(
			context.Background(),
			AlertWhatsAppSendInput{
				TenantID:       record.TenantID,
				To:             receivers,
				IdempotencyKey: record.IdempotencyKey,
				Text:           buildJobAlertWhatsAppMessage(record),
			},
		)
		if err != nil {
			channelRetryable := isJobAlertProviderRetryable(err)
			reason := "provider_error"
			if channelRetryable {
				reason = "provider_transient_error"
			}
			return JobAlertChannelResult{
				Channel:       "whatsapp",
				Status:        "failed",
				Reason:        reason,
				Provider:      "mock",
				ProviderError: strings.TrimSpace(err.Error()),
				Retryable:     channelRetryable,
				Receivers:     receivers,
			}
		}
		return JobAlertChannelResult{
			Channel:                "whatsapp",
			Status:                 "sent",
			Provider:               "mock",
			ProviderDeliveryID:     strings.TrimSpace(sendResult.ProviderDeliveryID),
			ProviderDeliveryStatus: strings.TrimSpace(sendResult.ProviderDeliveryStatus),
			Retryable:              false,
			Receivers:              receivers,
		}
	}

	if nextProvider == "meta" {
		if s.jobAlertWhatsAppSender == nil {
			return JobAlertChannelResult{
				Channel:       "whatsapp",
				Status:        "failed",
				Reason:        "provider_not_configured",
				Provider:      "meta",
				ProviderError: "meta whatsapp sender is not configured",
				Retryable:     false,
				Receivers:     receivers,
			}
		}
		sendResult, err := s.jobAlertWhatsAppSender.Send(
			context.Background(),
			AlertWhatsAppSendInput{
				TenantID:       record.TenantID,
				To:             receivers,
				IdempotencyKey: record.IdempotencyKey,
				Text:           buildJobAlertWhatsAppMessage(record),
			},
		)
		if err != nil {
			channelRetryable := isJobAlertProviderRetryable(err)
			reason := "provider_error"
			if channelRetryable {
				reason = "provider_transient_error"
			}
			return JobAlertChannelResult{
				Channel:       "whatsapp",
				Status:        "failed",
				Reason:        reason,
				Provider:      "meta",
				ProviderError: strings.TrimSpace(err.Error()),
				Retryable:     channelRetryable,
				Receivers:     receivers,
			}
		}
		return JobAlertChannelResult{
			Channel:                "whatsapp",
			Status:                 "sent",
			Provider:               "meta",
			ProviderDeliveryID:     strings.TrimSpace(sendResult.ProviderDeliveryID),
			ProviderDeliveryStatus: strings.TrimSpace(sendResult.ProviderDeliveryStatus),
			Retryable:              false,
			Receivers:              receivers,
		}
	}

	return JobAlertChannelResult{
		Channel:   "whatsapp",
		Status:    "failed",
		Reason:    "provider_not_supported",
		Provider:  nextProvider,
		Retryable: false,
		Receivers: receivers,
	}
}

func normalizeDispatchChannels(channels []string) []string {
	if len(channels) == 0 {
		return nil
	}
	output := make([]string, 0, len(channels))
	seen := map[string]struct{}{}
	for i := range channels {
		next := strings.ToLower(strings.TrimSpace(channels[i]))
		if next == "" {
			continue
		}
		if _, exists := seen[next]; exists {
			continue
		}
		seen[next] = struct{}{}
		output = append(output, next)
	}
	if len(output) == 0 {
		return nil
	}
	return output
}

func buildStaticJobAlertChannelResults(channels []string, status, reason string) []JobAlertChannelResult {
	normalized := normalizeDispatchChannels(channels)
	if len(normalized) == 0 {
		return nil
	}
	output := make([]JobAlertChannelResult, 0, len(normalized))
	for i := range normalized {
		output = append(output, JobAlertChannelResult{
			Channel: normalized[i],
			Status:  status,
			Reason:  reason,
		})
	}
	return output
}

func summarizeJobAlertChannelResults(
	channelResults []JobAlertChannelResult,
) (string, string, string, string, bool, []JobAlertChannelResult) {
	if len(channelResults) == 0 {
		return "skipped", "channel_disabled", "", "", false, channelResults
	}

	anySent := false
	anyFailed := false
	retryable := false
	firstFailed := JobAlertChannelResult{}
	firstSkipped := JobAlertChannelResult{}
	for i := range channelResults {
		switch channelResults[i].Status {
		case "sent":
			anySent = true
		case "failed":
			if !anyFailed {
				firstFailed = channelResults[i]
			}
			anyFailed = true
			if channelResults[i].Retryable {
				retryable = true
			}
		case "skipped":
			if firstSkipped.Channel == "" {
				firstSkipped = channelResults[i]
			}
		}
	}

	if anySent {
		provider := summarizeProvider(channelResults)
		if anyFailed {
			return "sent", "partial_channel_failure", provider, firstFailed.ProviderError, retryable, channelResults
		}
		return "sent", "", provider, "", false, channelResults
	}

	if anyFailed {
		return "failed", firstFailed.Reason, firstFailed.Provider, firstFailed.ProviderError, retryable, channelResults
	}

	if firstSkipped.Channel != "" {
		return "skipped", firstSkipped.Reason, firstSkipped.Provider, "", false, channelResults
	}
	return "skipped", "channel_disabled", "", "", false, channelResults
}

func summarizeProvider(channelResults []JobAlertChannelResult) string {
	providers := map[string]struct{}{}
	for i := range channelResults {
		if channelResults[i].Status != "sent" {
			continue
		}
		next := strings.TrimSpace(channelResults[i].Provider)
		if next == "" {
			continue
		}
		providers[next] = struct{}{}
	}
	if len(providers) == 0 {
		return ""
	}
	if len(providers) == 1 {
		for key := range providers {
			return key
		}
	}
	return "multi"
}

func isJobAlertProviderRetryable(err error) bool {
	if err == nil {
		return false
	}
	var httpErr AlertEmailHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Retryable()
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "temporar") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused")
}

