package wallet

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIssuePassBatchQueuedAndProcessIssueJobs(t *testing.T) {
	svc := NewService()

	jobs, err := svc.IssuePassBatchQueued(
		"tenant_demo_jakarta",
		"wpt_employee_demo",
		"user",
		[]string{"usr_queue_test_1", ""},
		"",
		"qa.queue",
	)
	if err != nil {
		t.Fatalf("issue pass batch queued failed: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	var pendingJobID string
	var failedTargetJobID string
	for i := range jobs {
		if jobs[i].Status == "pending" {
			pendingJobID = jobs[i].ID
		}
		if jobs[i].Status == "failed" && jobs[i].ErrorCode == "target_id_required" {
			failedTargetJobID = jobs[i].ID
		}
	}
	if pendingJobID == "" {
		t.Fatalf("expected one pending queued job")
	}
	if failedTargetJobID == "" {
		t.Fatalf("expected one failed target_id_required queued job")
	}

	result, err := svc.ProcessIssueJobs(JobProcessOptions{
		TenantID:    "tenant_demo_jakarta",
		Limit:       10,
		WorkerCount: 2,
		MaxRetry:    3,
		BaseBackoff: 1 * time.Millisecond,
		MaxBackoff:  10 * time.Millisecond,
		Actor:       "qa.worker",
	})
	if err != nil {
		t.Fatalf("process issue jobs failed: %v", err)
	}
	if result.Claimed != 1 {
		t.Fatalf("expected claimed=1 (only pending), got %d", result.Claimed)
	}
	if result.Succeeded != 1 {
		t.Fatalf("expected succeeded=1, got %d", result.Succeeded)
	}
	if result.Failed != 0 {
		t.Fatalf("expected failed=0, got %d", result.Failed)
	}

	processedPending, err := svc.GetJob("tenant_demo_jakarta", pendingJobID)
	if err != nil {
		t.Fatalf("get processed pending job failed: %v", err)
	}
	if processedPending.Status != "success" {
		t.Fatalf("expected pending job to become success, got %s", processedPending.Status)
	}
	if processedPending.PassID == "" {
		t.Fatalf("expected processed pending job pass_id to be set")
	}

	failedJob, err := svc.GetJob("tenant_demo_jakarta", failedTargetJobID)
	if err != nil {
		t.Fatalf("get failed target job failed: %v", err)
	}
	if failedJob.Status != "failed" {
		t.Fatalf("expected failed target job to stay failed, got %s", failedJob.Status)
	}
}

func TestProcessIssueJobsRetriesIssueFailedJob(t *testing.T) {
	svc := NewService()

	jobID, err := walletID("wjb_")
	if err != nil {
		t.Fatalf("generate job id failed: %v", err)
	}

	now := time.Now().UTC()
	svc.mu.Lock()
	svc.jobs = append([]IssueJob{
		{
			ID:           jobID,
			TenantID:     "tenant_demo_jakarta",
			Provider:     "google",
			BatchID:      "wbt_retry_test",
			TemplateID:   "wpt_employee_demo",
			TargetType:   "user",
			TargetID:     "usr_retry_issue_failed",
			Status:       "failed",
			RetryCount:   0,
			ErrorCode:    "issue_failed",
			ErrorMessage: "forced issue failure",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}, svc.jobs...)
	svc.mu.Unlock()

	result, err := svc.ProcessIssueJobs(JobProcessOptions{
		TenantID:    "tenant_demo_jakarta",
		Limit:       5,
		WorkerCount: 1,
		MaxRetry:    2,
		BaseBackoff: 1 * time.Millisecond,
		MaxBackoff:  2 * time.Millisecond,
		Actor:       "qa.worker",
	})
	if err != nil {
		t.Fatalf("process issue jobs failed: %v", err)
	}
	if result.Claimed < 1 {
		t.Fatalf("expected at least one claimed job, got %d", result.Claimed)
	}
	if result.Retried < 1 {
		t.Fatalf("expected retried>=1, got %d", result.Retried)
	}
	if result.Succeeded < 1 {
		t.Fatalf("expected succeeded>=1, got %d", result.Succeeded)
	}

	retriedJob, err := svc.GetJob("tenant_demo_jakarta", jobID)
	if err != nil {
		t.Fatalf("get retried job failed: %v", err)
	}
	if retriedJob.Status != "success" {
		t.Fatalf("expected retried job status=success, got %s", retriedJob.Status)
	}
	if retriedJob.RetryCount != 1 {
		t.Fatalf("expected retry_count=1 after auto retry, got %d", retriedJob.RetryCount)
	}
	if retriedJob.PassID == "" {
		t.Fatalf("expected retried job pass_id to be set")
	}
}

func TestProcessIssueJobsRejectsInvalidOptions(t *testing.T) {
	svc := NewService()
	_, err := svc.ProcessIssueJobs(JobProcessOptions{
		TenantID:    "tenant_demo_jakarta",
		Limit:       1,
		WorkerCount: 64,
	})
	if err == nil {
		t.Fatalf("expected invalid options to fail")
	}
	if err != ErrInvalidJobProcessOptions {
		t.Fatalf("expected ErrInvalidJobProcessOptions, got %v", err)
	}
}

func TestProcessIssueJobsMovesTemplateInactiveToDLQAndRequeue(t *testing.T) {
	svc := NewService()

	queuedJobs, err := svc.IssuePassBatchQueued(
		"tenant_demo_jakarta",
		"wpt_employee_demo",
		"user",
		[]string{"usr_dlq_flow"},
		"",
		"qa.queue",
	)
	if err != nil {
		t.Fatalf("issue pass batch queued failed: %v", err)
	}
	if len(queuedJobs) != 1 {
		t.Fatalf("expected 1 queued job, got %d", len(queuedJobs))
	}
	jobID := queuedJobs[0].ID

	if _, err := svc.UpdateTemplateStatus("tenant_demo_jakarta", "wpt_employee_demo", "inactive", "qa.template"); err != nil {
		t.Fatalf("set template inactive failed: %v", err)
	}

	firstProcess, err := svc.ProcessIssueJobs(JobProcessOptions{
		TenantID:    "tenant_demo_jakarta",
		Limit:       10,
		WorkerCount: 1,
		MaxRetry:    3,
		BaseBackoff: 1 * time.Millisecond,
		MaxBackoff:  2 * time.Millisecond,
		Actor:       "qa.worker",
	})
	if err != nil {
		t.Fatalf("process issue jobs failed: %v", err)
	}
	if firstProcess.Claimed != 1 {
		t.Fatalf("expected claimed=1, got %d", firstProcess.Claimed)
	}
	if firstProcess.DLQ != 1 {
		t.Fatalf("expected dlq=1, got %d", firstProcess.DLQ)
	}

	dlqJob, err := svc.GetJob("tenant_demo_jakarta", jobID)
	if err != nil {
		t.Fatalf("get dlq job failed: %v", err)
	}
	if dlqJob.Status != "dlq" {
		t.Fatalf("expected job status dlq, got %s", dlqJob.Status)
	}
	if dlqJob.ErrorCode != "template_inactive" {
		t.Fatalf("expected error_code=template_inactive, got %s", dlqJob.ErrorCode)
	}

	requeued, err := svc.RequeueDLQJob("tenant_demo_jakarta", jobID, "", "qa.requeue")
	if err != nil {
		t.Fatalf("requeue dlq job failed: %v", err)
	}
	if requeued.Status != "pending" {
		t.Fatalf("expected requeued status pending, got %s", requeued.Status)
	}

	if _, err := svc.UpdateTemplateStatus("tenant_demo_jakarta", "wpt_employee_demo", "active", "qa.template"); err != nil {
		t.Fatalf("set template active failed: %v", err)
	}

	secondProcess, err := svc.ProcessIssueJobs(JobProcessOptions{
		TenantID:    "tenant_demo_jakarta",
		Limit:       10,
		WorkerCount: 1,
		MaxRetry:    3,
		BaseBackoff: 1 * time.Millisecond,
		MaxBackoff:  2 * time.Millisecond,
		Actor:       "qa.worker",
	})
	if err != nil {
		t.Fatalf("second process issue jobs failed: %v", err)
	}
	if secondProcess.Succeeded != 1 {
		t.Fatalf("expected second process succeeded=1, got %d", secondProcess.Succeeded)
	}

	finalJob, err := svc.GetJob("tenant_demo_jakarta", jobID)
	if err != nil {
		t.Fatalf("get final job failed: %v", err)
	}
	if finalJob.Status != "success" {
		t.Fatalf("expected final status success, got %s", finalJob.Status)
	}
	if finalJob.PassID == "" {
		t.Fatalf("expected final pass_id to be set")
	}
}

func TestGetJobSummaryCountsAndBreakdown(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC()

	svc.mu.Lock()
	svc.jobs = append([]IssueJob{
		{ID: "wjb_sum_pending", TenantID: "tenant_demo_jakarta", Status: "pending", CreatedAt: now, UpdatedAt: now},
		{ID: "wjb_sum_processing", TenantID: "tenant_demo_jakarta", Status: "processing", CreatedAt: now, UpdatedAt: now},
		{ID: "wjb_sum_success", TenantID: "tenant_demo_jakarta", Status: "success", CreatedAt: now, UpdatedAt: now},
		{ID: "wjb_sum_retryable_failed", TenantID: "tenant_demo_jakarta", Status: "failed", RetryCount: 0, ErrorCode: "issue_failed", CreatedAt: now, UpdatedAt: now},
		{ID: "wjb_sum_non_retryable_failed", TenantID: "tenant_demo_jakarta", Status: "failed", RetryCount: 0, ErrorCode: "target_id_required", CreatedAt: now, UpdatedAt: now},
		{ID: "wjb_sum_dlq", TenantID: "tenant_demo_jakarta", Status: "dlq", RetryCount: 3, ErrorCode: "template_inactive", CreatedAt: now, UpdatedAt: now},
	}, svc.jobs...)
	svc.mu.Unlock()

	summary := svc.GetJobSummary("tenant_demo_jakarta", 3)
	if summary.Total < 6 {
		t.Fatalf("expected summary total >= 6, got %d", summary.Total)
	}
	if summary.Pending < 1 || summary.Processing < 1 || summary.Success < 1 {
		t.Fatalf("expected pending/processing/success to be counted: %+v", summary)
	}
	if summary.Failed < 2 {
		t.Fatalf("expected failed >=2, got %d", summary.Failed)
	}
	if summary.DLQ < 1 {
		t.Fatalf("expected dlq >=1, got %d", summary.DLQ)
	}
	if summary.RetryableFailed < 1 {
		t.Fatalf("expected retryable_failed >=1, got %d", summary.RetryableFailed)
	}
	if summary.NonRetryableFailed < 2 {
		t.Fatalf("expected non_retryable_failed >=2, got %d", summary.NonRetryableFailed)
	}
	if summary.ErrorCodeBreakdown["issue_failed"] < 1 {
		t.Fatalf("expected issue_failed in breakdown")
	}
	if summary.ErrorCodeBreakdown["template_inactive"] < 1 {
		t.Fatalf("expected template_inactive in breakdown")
	}
}

func TestRequeueDLQJobsBatch(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC()

	svc.mu.Lock()
	svc.jobs = append([]IssueJob{
		{ID: "wjb_dlq_batch_1", TenantID: "tenant_demo_jakarta", TemplateID: "wpt_employee_demo", TargetType: "user", TargetID: "usr_dlq_keep", Status: "dlq", ErrorCode: "template_inactive", CreatedAt: now, UpdatedAt: now},
		{ID: "wjb_dlq_batch_2", TenantID: "tenant_demo_jakarta", TemplateID: "wpt_employee_demo", TargetType: "user", TargetID: "", Status: "dlq", ErrorCode: "template_inactive", CreatedAt: now, UpdatedAt: now},
		{ID: "wjb_dlq_batch_3", TenantID: "tenant_demo_jakarta", TemplateID: "wpt_employee_demo", TargetType: "user", TargetID: "usr_other_error", Status: "dlq", ErrorCode: "target_id_required", CreatedAt: now, UpdatedAt: now},
	}, svc.jobs...)
	svc.mu.Unlock()

	result, err := svc.RequeueDLQJobs(JobDLQRequeueOptions{
		TenantID:  "tenant_demo_jakarta",
		Limit:     10,
		ErrorCode: "template_inactive",
		Actor:     "qa.batch",
	})
	if err != nil {
		t.Fatalf("requeue dlq jobs failed: %v", err)
	}
	if result.Requeued != 1 {
		t.Fatalf("expected requeued=1, got %d", result.Requeued)
	}
	if result.Skipped != 1 {
		t.Fatalf("expected skipped=1, got %d", result.Skipped)
	}
	if result.RemainingDLQ < 2 {
		t.Fatalf("expected remaining dlq >=2, got %d", result.RemainingDLQ)
	}

	job1, err := svc.GetJob("tenant_demo_jakarta", "wjb_dlq_batch_1")
	if err != nil {
		t.Fatalf("get requeued job failed: %v", err)
	}
	if job1.Status != "pending" {
		t.Fatalf("expected job1 status pending, got %s", job1.Status)
	}

	job2, err := svc.GetJob("tenant_demo_jakarta", "wjb_dlq_batch_2")
	if err != nil {
		t.Fatalf("get skipped dlq job failed: %v", err)
	}
	if job2.Status != "dlq" {
		t.Fatalf("expected job2 to remain dlq, got %s", job2.Status)
	}

	recovered, err := svc.RequeueDLQJobs(JobDLQRequeueOptions{
		TenantID:         "tenant_demo_jakarta",
		Limit:            1,
		ErrorCode:        "template_inactive",
		TargetIDOverride: "usr_dlq_override",
		Actor:            "qa.batch",
	})
	if err != nil {
		t.Fatalf("requeue dlq jobs with override failed: %v", err)
	}
	if recovered.Requeued != 1 {
		t.Fatalf("expected override requeued=1, got %d", recovered.Requeued)
	}

	job2After, err := svc.GetJob("tenant_demo_jakarta", "wjb_dlq_batch_2")
	if err != nil {
		t.Fatalf("get recovered job failed: %v", err)
	}
	if job2After.Status != "pending" {
		t.Fatalf("expected recovered job status pending, got %s", job2After.Status)
	}
	if job2After.TargetID != "usr_dlq_override" {
		t.Fatalf("expected recovered job target override, got %s", job2After.TargetID)
	}
}

func TestCleanupDLQJobs(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC()

	svc.mu.Lock()
	svc.jobs = append([]IssueJob{
		{ID: "wjb_dlq_old", TenantID: "tenant_demo_jakarta", Status: "dlq", ErrorCode: "template_inactive", CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now.Add(-48 * time.Hour)},
		{ID: "wjb_dlq_recent", TenantID: "tenant_demo_jakarta", Status: "dlq", ErrorCode: "template_inactive", CreatedAt: now.Add(-10 * time.Minute), UpdatedAt: now.Add(-10 * time.Minute)},
		{ID: "wjb_dlq_other", TenantID: "tenant_demo_jakarta", Status: "dlq", ErrorCode: "target_id_required", CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now.Add(-48 * time.Hour)},
		{ID: "wjb_failed_keep", TenantID: "tenant_demo_jakarta", Status: "failed", ErrorCode: "issue_failed", CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now.Add(-48 * time.Hour)},
	}, svc.jobs...)
	svc.mu.Unlock()

	result, err := svc.CleanupDLQJobs(JobDLQCleanupOptions{
		TenantID:  "tenant_demo_jakarta",
		Limit:     10,
		ErrorCode: "template_inactive",
		OlderThan: 1 * time.Hour,
		Actor:     "qa.cleanup",
	})
	if err != nil {
		t.Fatalf("cleanup dlq jobs failed: %v", err)
	}
	if result.Removed != 1 {
		t.Fatalf("expected removed=1, got %d", result.Removed)
	}
	if result.RemainingDLQ < 2 {
		t.Fatalf("expected remaining dlq >=2, got %d", result.RemainingDLQ)
	}

	if _, err := svc.GetJob("tenant_demo_jakarta", "wjb_dlq_old"); err == nil {
		t.Fatalf("expected old dlq job to be removed")
	}
	if _, err := svc.GetJob("tenant_demo_jakarta", "wjb_dlq_recent"); err != nil {
		t.Fatalf("expected recent dlq job to stay: %v", err)
	}
	if _, err := svc.GetJob("tenant_demo_jakarta", "wjb_dlq_other"); err != nil {
		t.Fatalf("expected other error-code dlq to stay: %v", err)
	}
}

func TestCleanupDLQJobsAppendsArchive(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC()

	svc.mu.Lock()
	svc.jobs = append([]IssueJob{
		{
			ID:        "wjb_archive_old",
			TenantID:  "tenant_demo_jakarta",
			Status:    "dlq",
			ErrorCode: "template_inactive",
			CreatedAt: now.Add(-48 * time.Hour),
			UpdatedAt: now.Add(-48 * time.Hour),
		},
		{
			ID:        "wjb_archive_recent",
			TenantID:  "tenant_demo_jakarta",
			Status:    "dlq",
			ErrorCode: "template_inactive",
			CreatedAt: now.Add(-10 * time.Minute),
			UpdatedAt: now.Add(-10 * time.Minute),
		},
	}, svc.jobs...)
	svc.mu.Unlock()

	result, err := svc.CleanupDLQJobs(JobDLQCleanupOptions{
		TenantID:  "tenant_demo_jakarta",
		Limit:     10,
		ErrorCode: "template_inactive",
		OlderThan: time.Hour,
		Actor:     "qa.cleanup.archive",
	})
	if err != nil {
		t.Fatalf("cleanup dlq jobs failed: %v", err)
	}
	if result.Removed != 1 {
		t.Fatalf("expected removed=1, got %d", result.Removed)
	}

	archives := svc.ListDLQCleanupArchives("tenant_demo_jakarta", 10)
	if len(archives) < 1 {
		t.Fatalf("expected at least one cleanup archive")
	}
	latest := archives[0]
	if latest.ID == "" {
		t.Fatalf("expected cleanup archive id to be set")
	}
	if latest.TenantID != "tenant_demo_jakarta" {
		t.Fatalf("cleanup archive tenant mismatch: %s", latest.TenantID)
	}
	if latest.Actor != "qa.cleanup.archive" {
		t.Fatalf("cleanup archive actor mismatch: %s", latest.Actor)
	}
	if latest.Limit != 10 {
		t.Fatalf("cleanup archive limit mismatch: %d", latest.Limit)
	}
	if latest.ErrorCode != "template_inactive" {
		t.Fatalf("cleanup archive error_code mismatch: %s", latest.ErrorCode)
	}
	if latest.OlderThanSeconds != int64(time.Hour.Seconds()) {
		t.Fatalf("cleanup archive older_than_seconds mismatch: %d", latest.OlderThanSeconds)
	}
	if latest.Removed != 1 {
		t.Fatalf("cleanup archive removed mismatch: %d", latest.Removed)
	}
	if latest.RemainingDLQ < 1 {
		t.Fatalf("cleanup archive remaining_dlq mismatch: %d", latest.RemainingDLQ)
	}
	if len(latest.ProcessedJobs) != 1 || latest.ProcessedJobs[0] != "wjb_archive_old" {
		t.Fatalf("cleanup archive processed jobs mismatch: %+v", latest.ProcessedJobs)
	}
}

func TestDLQOptionsValidation(t *testing.T) {
	svc := NewService()

	if _, err := svc.RequeueDLQJobs(JobDLQRequeueOptions{
		TenantID: "tenant_demo_jakarta",
		Limit:    9999,
	}); err != ErrInvalidJobDLQOptions {
		t.Fatalf("expected ErrInvalidJobDLQOptions for requeue limit, got %v", err)
	}

	if _, err := svc.CleanupDLQJobs(JobDLQCleanupOptions{
		TenantID:  "tenant_demo_jakarta",
		Limit:     10,
		OlderThan: 366 * 24 * time.Hour,
	}); err != ErrInvalidJobDLQOptions {
		t.Fatalf("expected ErrInvalidJobDLQOptions for cleanup older_than, got %v", err)
	}
}

func TestGetJobMetricsWindowAndThresholdAlerts(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC()

	svc.mu.Lock()
	svc.jobs = []IssueJob{
		{
			ID:        "wjb_metrics_dlq_1",
			TenantID:  "tenant_demo_jakarta",
			Status:    "dlq",
			ErrorCode: "template_inactive",
			CreatedAt: now.Add(-40 * time.Minute),
			UpdatedAt: now.Add(-2 * time.Minute),
		},
		{
			ID:        "wjb_metrics_dlq_2",
			TenantID:  "tenant_demo_jakarta",
			Status:    "dlq",
			ErrorCode: "template_inactive",
			CreatedAt: now.Add(-20 * time.Minute),
			UpdatedAt: now.Add(-90 * time.Second),
		},
		{
			ID:        "wjb_metrics_old_dlq",
			TenantID:  "tenant_demo_jakarta",
			Status:    "dlq",
			ErrorCode: "target_id_required",
			CreatedAt: now.Add(-3 * time.Hour),
			UpdatedAt: now.Add(-3 * time.Hour),
		},
		{
			ID:        "wjb_metrics_success",
			TenantID:  "tenant_demo_jakarta",
			Status:    "success",
			CreatedAt: now.Add(-5 * time.Minute),
			UpdatedAt: now.Add(-4 * time.Minute),
		},
	}
	svc.mu.Unlock()

	metrics := svc.GetJobMetrics(
		"tenant_demo_jakarta",
		15*time.Minute,
		3,
		2,
	)

	if metrics.Window.WindowSeconds != int64((15 * time.Minute).Seconds()) {
		t.Fatalf("window seconds mismatch: got %d", metrics.Window.WindowSeconds)
	}
	if metrics.Window.Updated != 3 {
		t.Fatalf("expected updated=3, got %d", metrics.Window.Updated)
	}
	if metrics.Window.Created != 1 {
		t.Fatalf("expected created=1, got %d", metrics.Window.Created)
	}
	if metrics.Window.DLQ != 2 {
		t.Fatalf("expected window dlq=2, got %d", metrics.Window.DLQ)
	}
	if metrics.Window.Success != 1 {
		t.Fatalf("expected window success=1, got %d", metrics.Window.Success)
	}
	if metrics.Window.ErrorCodeBreakdown["template_inactive"] != 2 {
		t.Fatalf("expected window template_inactive=2, got %d", metrics.Window.ErrorCodeBreakdown["template_inactive"])
	}
	if metrics.Summary.DLQ != 3 {
		t.Fatalf("expected total dlq=3, got %d", metrics.Summary.DLQ)
	}
	if len(metrics.Alerts) != 1 {
		t.Fatalf("expected exactly one alert, got %d", len(metrics.Alerts))
	}
	if metrics.Alerts[0].Type != "dlq_error_code_threshold" {
		t.Fatalf("unexpected alert type: %s", metrics.Alerts[0].Type)
	}
	if metrics.Alerts[0].ErrorCode != "template_inactive" {
		t.Fatalf("unexpected alert error code: %s", metrics.Alerts[0].ErrorCode)
	}
	if metrics.Alerts[0].Count != 2 {
		t.Fatalf("expected alert count=2, got %d", metrics.Alerts[0].Count)
	}
	if metrics.Alerts[0].Threshold != 2 {
		t.Fatalf("expected alert threshold=2, got %d", metrics.Alerts[0].Threshold)
	}
}

func TestGetJobMetricsDefaults(t *testing.T) {
	svc := NewService()
	metrics := svc.GetJobMetrics(
		"tenant_demo_jakarta",
		0,
		0,
		0,
	)

	if metrics.MaxRetry != 3 {
		t.Fatalf("expected default max_retry=3, got %d", metrics.MaxRetry)
	}
	if metrics.DLQAlertThreshold != 20 {
		t.Fatalf("expected default dlq_alert_threshold=20, got %d", metrics.DLQAlertThreshold)
	}
	if metrics.Window.WindowSeconds != int64((15 * time.Minute).Seconds()) {
		t.Fatalf("expected default window=900s, got %d", metrics.Window.WindowSeconds)
	}
}

func TestGetJobMetricsTrendWindowBucketsAndAlerts(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC()

	svc.mu.Lock()
	svc.jobs = []IssueJob{
		{
			ID:        "wjb_trend_success_1",
			TenantID:  "tenant_demo_jakarta",
			Status:    "success",
			CreatedAt: now.Add(-11 * time.Minute),
			UpdatedAt: now.Add(-10 * time.Minute),
		},
		{
			ID:        "wjb_trend_failed_1",
			TenantID:  "tenant_demo_jakarta",
			Status:    "failed",
			ErrorCode: "issue_failed",
			CreatedAt: now.Add(-8 * time.Minute),
			UpdatedAt: now.Add(-7 * time.Minute),
		},
		{
			ID:        "wjb_trend_dlq_1",
			TenantID:  "tenant_demo_jakarta",
			Status:    "dlq",
			ErrorCode: "template_inactive",
			CreatedAt: now.Add(-5 * time.Minute),
			UpdatedAt: now.Add(-4 * time.Minute),
		},
		{
			ID:        "wjb_trend_dlq_2",
			TenantID:  "tenant_demo_jakarta",
			Status:    "dlq",
			ErrorCode: "template_inactive",
			CreatedAt: now.Add(-3 * time.Minute),
			UpdatedAt: now.Add(-2 * time.Minute),
		},
		{
			ID:        "wjb_trend_old",
			TenantID:  "tenant_demo_jakarta",
			Status:    "success",
			CreatedAt: now.Add(-3 * time.Hour),
			UpdatedAt: now.Add(-3 * time.Hour),
		},
	}
	svc.mu.Unlock()

	trend := svc.GetJobMetricsTrend(
		"tenant_demo_jakarta",
		12*time.Minute,
		6,
		3,
		2,
	)
	if trend.WindowSeconds != int64((12 * time.Minute).Seconds()) {
		t.Fatalf("window seconds mismatch: got %d", trend.WindowSeconds)
	}
	if trend.BucketCount != 6 {
		t.Fatalf("bucket count mismatch: got %d", trend.BucketCount)
	}
	if trend.BucketSeconds != int64((12*time.Minute).Seconds())/6 {
		t.Fatalf("bucket seconds mismatch: got %d", trend.BucketSeconds)
	}
	if len(trend.Buckets) != 6 {
		t.Fatalf("bucket length mismatch: got %d", len(trend.Buckets))
	}
	updated := 0
	created := 0
	success := 0
	failed := 0
	dlq := 0
	for i := range trend.Buckets {
		if i > 0 && trend.Buckets[i-1].End.After(trend.Buckets[i].Start) {
			t.Fatalf("bucket window overlap at index=%d", i)
		}
		updated += trend.Buckets[i].Updated
		created += trend.Buckets[i].Created
		success += trend.Buckets[i].Success
		failed += trend.Buckets[i].Failed
		dlq += trend.Buckets[i].DLQ
	}
	if updated != 4 {
		t.Fatalf("expected updated=4 in trend buckets, got %d", updated)
	}
	if created != 4 {
		t.Fatalf("expected created=4 in trend buckets, got %d", created)
	}
	if success != 1 || failed != 1 || dlq != 2 {
		t.Fatalf("unexpected status totals in trend buckets: success=%d failed=%d dlq=%d", success, failed, dlq)
	}
	if len(trend.Alerts) != 1 || trend.Alerts[0].ErrorCode != "template_inactive" {
		t.Fatalf("unexpected trend alerts: %+v", trend.Alerts)
	}
}

func TestGetJobMetricsTrendDefaults(t *testing.T) {
	svc := NewService()
	trend := svc.GetJobMetricsTrend(
		"tenant_demo_jakarta",
		0,
		0,
		0,
		0,
	)
	if trend.MaxRetry != 3 {
		t.Fatalf("expected default max_retry=3, got %d", trend.MaxRetry)
	}
	if trend.DLQAlertThreshold != 20 {
		t.Fatalf("expected default dlq_alert_threshold=20, got %d", trend.DLQAlertThreshold)
	}
	if trend.WindowSeconds != int64((15 * time.Minute).Seconds()) {
		t.Fatalf("expected default window=900s, got %d", trend.WindowSeconds)
	}
	if trend.BucketCount != 12 {
		t.Fatalf("expected default bucket_count=12, got %d", trend.BucketCount)
	}
	if len(trend.Buckets) != 12 {
		t.Fatalf("expected 12 buckets, got %d", len(trend.Buckets))
	}
}

func TestGetJobAlertSubscriptionNotFound(t *testing.T) {
	svc := NewService()
	record, found := svc.GetJobAlertSubscription("tenant_demo_jakarta")
	if found {
		t.Fatalf("expected no alert subscription, got %+v", record)
	}
}

func TestUpsertJobAlertSubscription(t *testing.T) {
	svc := NewService()

	record, err := svc.UpsertJobAlertSubscription(JobAlertSubscriptionUpsertOptions{
		TenantID:          "tenant_demo_jakarta",
		Enabled:           true,
		DLQAlertThreshold: 3,
		Window:            10 * time.Minute,
		Cooldown:          2 * time.Minute,
		EmailEnabled:      true,
		WhatsAppEnabled:   false,
		ReceiverGroups:    []string{"security", "ops", "Security"},
		Actor:             "qa.alert.subscription",
	})
	if err != nil {
		t.Fatalf("upsert job alert subscription failed: %v", err)
	}
	if record.TenantID != "tenant_demo_jakarta" {
		t.Fatalf("unexpected tenant_id: %s", record.TenantID)
	}
	if !record.Enabled {
		t.Fatalf("expected enabled=true")
	}
	if record.DLQAlertThreshold != 3 {
		t.Fatalf("expected threshold=3, got %d", record.DLQAlertThreshold)
	}
	if record.WindowSeconds != int64((10 * time.Minute).Seconds()) {
		t.Fatalf("expected window_seconds=600, got %d", record.WindowSeconds)
	}
	if record.CooldownSeconds != int64((2 * time.Minute).Seconds()) {
		t.Fatalf("expected cooldown_seconds=120, got %d", record.CooldownSeconds)
	}
	if !record.Channels.Email || record.Channels.WhatsApp {
		t.Fatalf("unexpected channels: %+v", record.Channels)
	}
	if len(record.ReceiverGroups) != 2 || record.ReceiverGroups[0] != "security" || record.ReceiverGroups[1] != "ops" {
		t.Fatalf("unexpected receiver_groups: %+v", record.ReceiverGroups)
	}

	reloaded, found := svc.GetJobAlertSubscription("tenant_demo_jakarta")
	if !found {
		t.Fatalf("expected alert subscription to be found")
	}
	if reloaded.DLQAlertThreshold != 3 || reloaded.WindowSeconds != int64((10*time.Minute).Seconds()) {
		t.Fatalf("unexpected reloaded alert subscription: %+v", reloaded)
	}
	logs := svc.ListAuditLogs("tenant_demo_jakarta")
	if len(logs) < 1 || logs[0].Action != "wallet.job.alert_subscription.upsert" {
		t.Fatalf("expected wallet.job.alert_subscription.upsert audit log, got %+v", logs)
	}
}

func TestUpsertJobAlertSubscriptionValidation(t *testing.T) {
	svc := NewService()

	if _, err := svc.UpsertJobAlertSubscription(JobAlertSubscriptionUpsertOptions{
		TenantID:          "tenant_demo_jakarta",
		Enabled:           true,
		DLQAlertThreshold: 0,
		Window:            10 * time.Minute,
		Cooldown:          time.Minute,
		EmailEnabled:      true,
	}); err != ErrInvalidJobAlertSubscriptionOptions {
		t.Fatalf("expected ErrInvalidJobAlertSubscriptionOptions for threshold, got %v", err)
	}

	if _, err := svc.UpsertJobAlertSubscription(JobAlertSubscriptionUpsertOptions{
		TenantID:          "tenant_demo_jakarta",
		Enabled:           true,
		DLQAlertThreshold: 2,
		Window:            0,
		Cooldown:          time.Minute,
		EmailEnabled:      true,
	}); err != ErrInvalidJobAlertSubscriptionOptions {
		t.Fatalf("expected ErrInvalidJobAlertSubscriptionOptions for window, got %v", err)
	}

	if _, err := svc.UpsertJobAlertSubscription(JobAlertSubscriptionUpsertOptions{
		TenantID:          "tenant_demo_jakarta",
		Enabled:           true,
		DLQAlertThreshold: 2,
		Window:            10 * time.Minute,
		Cooldown:          time.Minute,
		EmailEnabled:      false,
		WhatsAppEnabled:   false,
	}); err != ErrInvalidJobAlertSubscriptionOptions {
		t.Fatalf("expected ErrInvalidJobAlertSubscriptionOptions for channels, got %v", err)
	}

	record, err := svc.UpsertJobAlertSubscription(JobAlertSubscriptionUpsertOptions{
		TenantID:          "tenant_demo_jakarta",
		Enabled:           false,
		DLQAlertThreshold: 2,
		Window:            10 * time.Minute,
		Cooldown:          time.Minute,
		EmailEnabled:      false,
		WhatsAppEnabled:   false,
		ReceiverGroups:    nil,
	})
	if err != nil {
		t.Fatalf("upsert disabled alert subscription failed: %v", err)
	}
	if len(record.ReceiverGroups) != 1 || record.ReceiverGroups[0] != "security" {
		t.Fatalf("expected default receiver_groups=[security], got %+v", record.ReceiverGroups)
	}
}

func TestDispatchJobMetricsAlertsAndCooldown(t *testing.T) {
	svc := NewService()

	subscription := JobAlertSubscription{
		TenantID:          "tenant_demo_jakarta",
		Enabled:           true,
		DLQAlertThreshold: 2,
		WindowSeconds:     int64((10 * time.Minute).Seconds()),
		CooldownSeconds:   int64((2 * time.Minute).Seconds()),
		Channels: JobAlertSubscriptionChannels{
			Email:    true,
			WhatsApp: false,
		},
		ReceiverGroups: []string{"security"},
	}
	alerts := []JobMetricsAlert{
		{
			Type:      "dlq_error_code_threshold",
			ErrorCode: "template_inactive",
			Count:     3,
			Threshold: 2,
		},
	}

	first, err := svc.DispatchJobMetricsAlerts(subscription, alerts, "qa.alert.dispatch")
	if err != nil {
		t.Fatalf("first dispatch failed: %v", err)
	}
	if first.TotalAlerts != 1 || first.Dispatched != 1 || first.Skipped != 0 {
		t.Fatalf("unexpected first dispatch result: %+v", first)
	}
	if len(first.Items) != 1 || first.Items[0].Status != "sent" {
		t.Fatalf("unexpected first dispatch items: %+v", first.Items)
	}

	second, err := svc.DispatchJobMetricsAlerts(subscription, alerts, "qa.alert.dispatch")
	if err != nil {
		t.Fatalf("second dispatch failed: %v", err)
	}
	if second.Dispatched != 0 || second.Skipped != 1 {
		t.Fatalf("expected second dispatch to be skipped by cooldown, got %+v", second)
	}
	if len(second.Items) != 1 || second.Items[0].Reason != "cooldown" {
		t.Fatalf("expected cooldown skip reason, got %+v", second.Items)
	}

	logs := svc.ListJobAlertNotifications("tenant_demo_jakarta", 10)
	if len(logs) < 2 {
		t.Fatalf("expected at least 2 alert notification logs, got %d", len(logs))
	}
	if logs[0].Status != "skipped" || logs[0].Reason != "cooldown" {
		t.Fatalf("unexpected latest notification log: %+v", logs[0])
	}
	if logs[1].Status != "sent" {
		t.Fatalf("unexpected previous notification log: %+v", logs[1])
	}

	limited := svc.ListJobAlertNotifications("tenant_demo_jakarta", 1)
	if len(limited) != 1 {
		t.Fatalf("expected limit=1 to return one row, got %d", len(limited))
	}
}

func TestDispatchJobMetricsAlertsTransientFailureAndRetry(t *testing.T) {
	svc := NewService()
	svc.SetJobAlertMockTransientFailCount(1)

	subscription := JobAlertSubscription{
		TenantID:          "tenant_demo_jakarta",
		Enabled:           true,
		DLQAlertThreshold: 1,
		WindowSeconds:     int64((10 * time.Minute).Seconds()),
		CooldownSeconds:   0,
		Channels: JobAlertSubscriptionChannels{
			Email:    true,
			WhatsApp: false,
		},
		ReceiverGroups: []string{"security"},
	}
	alerts := []JobMetricsAlert{
		{
			Type:      "dlq_error_code_threshold",
			ErrorCode: "template_inactive",
			Count:     3,
			Threshold: 1,
		},
	}

	first, err := svc.DispatchJobMetricsAlerts(subscription, alerts, "qa.alert.dispatch")
	if err != nil {
		t.Fatalf("dispatch with transient failure failed: %v", err)
	}
	if first.Dispatched != 0 || first.Skipped != 0 || first.Failed != 1 {
		t.Fatalf("unexpected dispatch counters: %+v", first)
	}
	if len(first.Items) != 1 {
		t.Fatalf("expected one dispatch item, got %d", len(first.Items))
	}
	failedItem := first.Items[0]
	if failedItem.Status != "failed" || failedItem.Reason != "provider_transient_error" {
		t.Fatalf("expected provider transient failed item, got %+v", failedItem)
	}
	if !failedItem.Retryable || failedItem.Attempt != 1 || failedItem.ProviderError == "" {
		t.Fatalf("expected retryable attempt=1 with provider_error, got %+v", failedItem)
	}

	retried, err := svc.RetryJobAlertNotification("tenant_demo_jakarta", failedItem.ID, "qa.alert.retry")
	if err != nil {
		t.Fatalf("retry failed notification failed: %v", err)
	}
	if retried.Status != "sent" || retried.Attempt != 2 {
		t.Fatalf("expected retry sent with attempt=2, got %+v", retried)
	}
	if retried.SourceNotificationID != failedItem.ID {
		t.Fatalf("expected source notification id=%s, got %s", failedItem.ID, retried.SourceNotificationID)
	}

	idempotentSkip, err := svc.RetryJobAlertNotification("tenant_demo_jakarta", failedItem.ID, "qa.alert.retry")
	if err != nil {
		t.Fatalf("retry failed notification second attempt failed: %v", err)
	}
	if idempotentSkip.Status != "skipped" || idempotentSkip.Reason != "idempotent_already_sent" {
		t.Fatalf("expected idempotent skip on second retry, got %+v", idempotentSkip)
	}

	logs := svc.ListJobAlertNotifications("tenant_demo_jakarta", 5)
	if len(logs) < 3 {
		t.Fatalf("expected at least 3 logs, got %d", len(logs))
	}
	if logs[0].Status != "skipped" || logs[0].Reason != "idempotent_already_sent" {
		t.Fatalf("unexpected latest log: %+v", logs[0])
	}
	if logs[1].Status != "sent" {
		t.Fatalf("unexpected second latest log: %+v", logs[1])
	}
	if logs[2].Status != "failed" || logs[2].Reason != "provider_transient_error" {
		t.Fatalf("unexpected third latest log: %+v", logs[2])
	}
}

func TestRetryJobAlertNotificationNotAllowed(t *testing.T) {
	svc := NewService()

	subscription := JobAlertSubscription{
		TenantID:          "tenant_demo_jakarta",
		Enabled:           false,
		DLQAlertThreshold: 1,
		WindowSeconds:     int64((10 * time.Minute).Seconds()),
		CooldownSeconds:   0,
		Channels: JobAlertSubscriptionChannels{
			Email:    true,
			WhatsApp: false,
		},
		ReceiverGroups: []string{"security"},
	}
	alerts := []JobMetricsAlert{
		{
			Type:      "dlq_error_code_threshold",
			ErrorCode: "template_inactive",
			Count:     2,
			Threshold: 1,
		},
	}

	result, err := svc.DispatchJobMetricsAlerts(subscription, alerts, "qa.alert.dispatch")
	if err != nil {
		t.Fatalf("dispatch skipped notification failed: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one notification, got %d", len(result.Items))
	}

	_, err = svc.RetryJobAlertNotification("tenant_demo_jakarta", result.Items[0].ID, "qa.alert.retry")
	if !errors.Is(err, ErrJobAlertRetryNotAllowed) {
		t.Fatalf("expected ErrJobAlertRetryNotAllowed, got %v", err)
	}

	_, err = svc.RetryJobAlertNotification("tenant_demo_jakarta", "wan_missing", "qa.alert.retry")
	if !errors.Is(err, ErrJobAlertNotificationNotFound) {
		t.Fatalf("expected ErrJobAlertNotificationNotFound, got %v", err)
	}
}

func TestSetJobAlertEmailDeliveryOptionsInvalidProvider(t *testing.T) {
	svc := NewService()
	err := svc.SetJobAlertEmailDeliveryOptions(JobAlertEmailDeliveryOptions{
		Provider:  "invalid-provider",
		EmailFrom: "alerts@mistypass.local",
	})
	if !errors.Is(err, ErrInvalidJobAlertEmailProvider) {
		t.Fatalf("expected ErrInvalidJobAlertEmailProvider, got %v", err)
	}
}

func TestSetJobAlertEmailDeliveryOptionsInvalidWhatsAppProvider(t *testing.T) {
	svc := NewService()
	err := svc.SetJobAlertEmailDeliveryOptions(JobAlertEmailDeliveryOptions{
		Provider:         "mock",
		EmailFrom:        "alerts@mistypass.local",
		WhatsAppProvider: "invalid-provider",
	})
	if !errors.Is(err, ErrInvalidJobAlertWhatsAppProvider) {
		t.Fatalf("expected ErrInvalidJobAlertWhatsAppProvider, got %v", err)
	}
}

func TestDispatchJobMetricsAlertsWithWhatsAppMockProvider(t *testing.T) {
	svc := NewService()
	if err := svc.SetJobAlertEmailDeliveryOptions(JobAlertEmailDeliveryOptions{
		Provider:              "mock",
		EmailFrom:             "alerts@mistypass.local",
		ReceiverMap:           map[string][]string{"security": {"security@example.com"}},
		WhatsAppProvider:      "mock",
		WhatsAppReceiverMap:   map[string][]string{"security": {"+62811111111"}},
		WhatsAppEndpoint:      "",
		WhatsAppAPIKey:        "",
		WhatsAppPhoneNumberID: "",
		WhatsAppTimeout:       3 * time.Second,
	}); err != nil {
		t.Fatalf("set whatsapp mock options failed: %v", err)
	}

	subscription := JobAlertSubscription{
		TenantID:          "tenant_demo_jakarta",
		Enabled:           true,
		DLQAlertThreshold: 1,
		WindowSeconds:     int64((10 * time.Minute).Seconds()),
		CooldownSeconds:   0,
		Channels: JobAlertSubscriptionChannels{
			Email:    false,
			WhatsApp: true,
		},
		ReceiverGroups: []string{"security"},
	}
	alerts := []JobMetricsAlert{
		{
			Type:      "dlq_error_code_threshold",
			ErrorCode: "template_inactive",
			Count:     3,
			Threshold: 1,
		},
	}

	result, err := svc.DispatchJobMetricsAlerts(subscription, alerts, "qa.alert.dispatch")
	if err != nil {
		t.Fatalf("dispatch with whatsapp mock failed: %v", err)
	}
	if result.Dispatched != 1 || result.Failed != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected dispatch result: %+v", result)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one dispatch item, got %d", len(result.Items))
	}
	item := result.Items[0]
	if item.Status != "sent" || item.Provider != "mock" {
		t.Fatalf("unexpected dispatch item: %+v", item)
	}
	if len(item.ChannelResults) != 1 {
		t.Fatalf("expected one channel result, got %+v", item.ChannelResults)
	}
	if item.ChannelResults[0].Channel != "whatsapp" ||
		item.ChannelResults[0].Status != "sent" ||
		item.ChannelResults[0].Provider != "mock" {
		t.Fatalf("unexpected whatsapp channel result: %+v", item.ChannelResults[0])
	}
	if len(item.ChannelResults[0].Receivers) != 1 || item.ChannelResults[0].Receivers[0] != "+62811111111" {
		t.Fatalf("unexpected whatsapp receivers: %+v", item.ChannelResults[0].Receivers)
	}
}

func TestDispatchJobMetricsAlertsWithEmailAndWhatsAppUnifiedReceipt(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer provider.Close()

	svc := NewService()
	if err := svc.SetJobAlertEmailDeliveryOptions(JobAlertEmailDeliveryOptions{
		Provider:              "resend",
		EmailFrom:             "alerts@mistypass.local",
		ReceiverMap:           map[string][]string{"security": {"security@example.com"}},
		ResendEndpoint:        provider.URL,
		ResendAPIKey:          "re_test_token",
		ResendTimeout:         3 * time.Second,
		WhatsAppProvider:      "mock",
		WhatsAppReceiverMap:   map[string][]string{"security": {"+62811111111"}},
		WhatsAppEndpoint:      "",
		WhatsAppAPIKey:        "",
		WhatsAppPhoneNumberID: "",
		WhatsAppTimeout:       3 * time.Second,
	}); err != nil {
		t.Fatalf("set mixed provider options failed: %v", err)
	}

	subscription := JobAlertSubscription{
		TenantID:          "tenant_demo_jakarta",
		Enabled:           true,
		DLQAlertThreshold: 1,
		WindowSeconds:     int64((10 * time.Minute).Seconds()),
		CooldownSeconds:   0,
		Channels: JobAlertSubscriptionChannels{
			Email:    true,
			WhatsApp: true,
		},
		ReceiverGroups: []string{"security"},
	}
	alerts := []JobMetricsAlert{
		{
			Type:      "dlq_error_code_threshold",
			ErrorCode: "template_inactive",
			Count:     3,
			Threshold: 1,
		},
	}

	result, err := svc.DispatchJobMetricsAlerts(subscription, alerts, "qa.alert.dispatch")
	if err != nil {
		t.Fatalf("dispatch with mixed channels failed: %v", err)
	}
	if result.Dispatched != 1 || result.Failed != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected dispatch result: %+v", result)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one dispatch item, got %d", len(result.Items))
	}
	item := result.Items[0]
	if item.Status != "sent" || item.Provider != "multi" {
		t.Fatalf("expected sent/multi item, got %+v", item)
	}
	if len(item.ChannelResults) != 2 {
		t.Fatalf("expected two channel results, got %+v", item.ChannelResults)
	}
}

func TestDispatchJobMetricsAlertsWithResendProvider(t *testing.T) {
	var capturedAuth string
	var capturedFrom string
	var capturedTo []string
	var capturedSubject string

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		capturedAuth = r.Header.Get("Authorization")
		defer r.Body.Close()
		var body struct {
			From    string   `json:"from"`
			To      []string `json:"to"`
			Subject string   `json:"subject"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body failed: %v", err)
		}
		capturedFrom = body.From
		capturedTo = append([]string(nil), body.To...)
		capturedSubject = body.Subject
		w.WriteHeader(http.StatusAccepted)
	}))
	defer provider.Close()

	svc := NewService()
	if err := svc.SetJobAlertEmailDeliveryOptions(JobAlertEmailDeliveryOptions{
		Provider:       "resend",
		EmailFrom:      "alerts@mistypass.local",
		ReceiverMap:    map[string][]string{"security": {"security@example.com"}},
		ResendEndpoint: provider.URL,
		ResendAPIKey:   "re_test_token",
		ResendTimeout:  3 * time.Second,
	}); err != nil {
		t.Fatalf("set resend options failed: %v", err)
	}

	subscription := JobAlertSubscription{
		TenantID:          "tenant_demo_jakarta",
		Enabled:           true,
		DLQAlertThreshold: 1,
		WindowSeconds:     int64((10 * time.Minute).Seconds()),
		CooldownSeconds:   0,
		Channels: JobAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}
	alerts := []JobMetricsAlert{
		{
			Type:      "dlq_error_code_threshold",
			ErrorCode: "template_inactive",
			Count:     3,
			Threshold: 1,
		},
	}

	result, err := svc.DispatchJobMetricsAlerts(subscription, alerts, "qa.alert.dispatch")
	if err != nil {
		t.Fatalf("dispatch with resend failed: %v", err)
	}
	if result.Dispatched != 1 || result.Failed != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected dispatch result: %+v", result)
	}
	if len(result.Items) != 1 || result.Items[0].Provider != "resend" || result.Items[0].Status != "sent" {
		t.Fatalf("unexpected dispatch item: %+v", result.Items)
	}
	if capturedAuth != "Bearer re_test_token" {
		t.Fatalf("unexpected provider auth header: %s", capturedAuth)
	}
	if capturedFrom != "alerts@mistypass.local" {
		t.Fatalf("unexpected provider from: %s", capturedFrom)
	}
	if len(capturedTo) != 1 || capturedTo[0] != "security@example.com" {
		t.Fatalf("unexpected provider to: %+v", capturedTo)
	}
	if capturedSubject == "" {
		t.Fatalf("expected provider subject to be set")
	}
}

func TestDispatchJobMetricsAlertsWithResendProviderTransientFailure(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"temporarily unavailable"}`))
	}))
	defer provider.Close()

	svc := NewService()
	if err := svc.SetJobAlertEmailDeliveryOptions(JobAlertEmailDeliveryOptions{
		Provider:       "resend",
		EmailFrom:      "alerts@mistypass.local",
		ReceiverMap:    map[string][]string{"security": {"security@example.com"}},
		ResendEndpoint: provider.URL,
		ResendAPIKey:   "re_test_token",
		ResendTimeout:  3 * time.Second,
	}); err != nil {
		t.Fatalf("set resend options failed: %v", err)
	}

	subscription := JobAlertSubscription{
		TenantID:          "tenant_demo_jakarta",
		Enabled:           true,
		DLQAlertThreshold: 1,
		WindowSeconds:     int64((10 * time.Minute).Seconds()),
		CooldownSeconds:   0,
		Channels: JobAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}
	alerts := []JobMetricsAlert{
		{
			Type:      "dlq_error_code_threshold",
			ErrorCode: "template_inactive",
			Count:     3,
			Threshold: 1,
		},
	}

	result, err := svc.DispatchJobMetricsAlerts(subscription, alerts, "qa.alert.dispatch")
	if err != nil {
		t.Fatalf("dispatch with resend transient failure failed: %v", err)
	}
	if result.Dispatched != 0 || result.Failed != 1 || result.Skipped != 0 {
		t.Fatalf("unexpected transient failure dispatch result: %+v", result)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one dispatch item, got %d", len(result.Items))
	}
	item := result.Items[0]
	if item.Status != "failed" || item.Reason != "provider_transient_error" || !item.Retryable {
		t.Fatalf("unexpected transient failure item: %+v", item)
	}
}

func TestDispatchJobMetricsAlertsWithResendProviderReceiversNotConfigured(t *testing.T) {
	svc := NewService()
	if err := svc.SetJobAlertEmailDeliveryOptions(JobAlertEmailDeliveryOptions{
		Provider:       "resend",
		EmailFrom:      "alerts@mistypass.local",
		ReceiverMap:    map[string][]string{"ops": {"ops@example.com"}},
		ResendEndpoint: "http://127.0.0.1:19091/emails",
		ResendAPIKey:   "re_test_token",
		ResendTimeout:  3 * time.Second,
	}); err != nil {
		t.Fatalf("set resend options failed: %v", err)
	}

	subscription := JobAlertSubscription{
		TenantID:          "tenant_demo_jakarta",
		Enabled:           true,
		DLQAlertThreshold: 1,
		WindowSeconds:     int64((10 * time.Minute).Seconds()),
		CooldownSeconds:   0,
		Channels: JobAlertSubscriptionChannels{
			Email: true,
		},
		ReceiverGroups: []string{"security"},
	}
	alerts := []JobMetricsAlert{
		{
			Type:      "dlq_error_code_threshold",
			ErrorCode: "template_inactive",
			Count:     3,
			Threshold: 1,
		},
	}

	result, err := svc.DispatchJobMetricsAlerts(subscription, alerts, "qa.alert.dispatch")
	if err != nil {
		t.Fatalf("dispatch with missing receivers failed: %v", err)
	}
	if result.Failed != 1 || result.Dispatched != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected dispatch counters: %+v", result)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one dispatch item, got %d", len(result.Items))
	}
	item := result.Items[0]
	if item.Status != "failed" || item.Reason != "email_receivers_not_configured" || item.Retryable {
		t.Fatalf("unexpected missing receivers item: %+v", item)
	}
}

func TestDispatchJobMetricsAlertsSubscriptionDisabled(t *testing.T) {
	svc := NewService()

	subscription := JobAlertSubscription{
		TenantID:          "tenant_demo_jakarta",
		Enabled:           false,
		DLQAlertThreshold: 2,
		WindowSeconds:     int64((10 * time.Minute).Seconds()),
		CooldownSeconds:   int64((2 * time.Minute).Seconds()),
		Channels: JobAlertSubscriptionChannels{
			Email:    true,
			WhatsApp: false,
		},
		ReceiverGroups: []string{"security"},
	}
	alerts := []JobMetricsAlert{
		{
			Type:      "dlq_error_code_threshold",
			ErrorCode: "template_inactive",
			Count:     2,
			Threshold: 2,
		},
	}

	result, err := svc.DispatchJobMetricsAlerts(subscription, alerts, "qa.alert.dispatch")
	if err != nil {
		t.Fatalf("dispatch with disabled subscription failed: %v", err)
	}
	if result.Dispatched != 0 || result.Skipped != 1 {
		t.Fatalf("expected skipped dispatch, got %+v", result)
	}
	if len(result.Items) != 1 || result.Items[0].Reason != "subscription_disabled" {
		t.Fatalf("expected subscription_disabled reason, got %+v", result.Items)
	}
}

func TestDispatchJobMetricsAlertsNoChannel(t *testing.T) {
	svc := NewService()

	subscription := JobAlertSubscription{
		TenantID:          "tenant_demo_jakarta",
		Enabled:           true,
		DLQAlertThreshold: 2,
		WindowSeconds:     int64((10 * time.Minute).Seconds()),
		CooldownSeconds:   int64((2 * time.Minute).Seconds()),
		Channels: JobAlertSubscriptionChannels{
			Email:    false,
			WhatsApp: false,
		},
		ReceiverGroups: []string{"security"},
	}
	alerts := []JobMetricsAlert{
		{
			Type:      "dlq_error_code_threshold",
			ErrorCode: "template_inactive",
			Count:     2,
			Threshold: 2,
		},
	}

	result, err := svc.DispatchJobMetricsAlerts(subscription, alerts, "qa.alert.dispatch")
	if err != nil {
		t.Fatalf("dispatch with no channel failed: %v", err)
	}
	if result.Dispatched != 0 || result.Skipped != 1 {
		t.Fatalf("expected skipped dispatch, got %+v", result)
	}
	if len(result.Items) != 1 || result.Items[0].Reason != "channel_disabled" {
		t.Fatalf("expected channel_disabled reason, got %+v", result.Items)
	}
}
