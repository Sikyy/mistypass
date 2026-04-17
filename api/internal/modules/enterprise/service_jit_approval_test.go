package enterprise

import (
	"errors"
	"testing"
)

func TestJITProvisionApprovalUpsertAndReviewFlow(t *testing.T) {
	svc := NewService()
	email := "jit.approval.flow@sudirman.co"
	externalID := "sub-jit-approval-flow-001"

	first, err := svc.UpsertJITProvisionApprovalRequest(
		"tenant_demo_jakarta",
		email,
		externalID,
		"oidc",
		"active",
	)
	if err != nil {
		t.Fatalf("expected upsert approval request success: %v", err)
	}
	if first.Status != "pending" {
		t.Fatalf("expected pending status, got %s", first.Status)
	}

	second, err := svc.UpsertJITProvisionApprovalRequest(
		"tenant_demo_jakarta",
		email,
		externalID,
		"oidc",
		"active",
	)
	if err != nil {
		t.Fatalf("expected second upsert approval request success: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected pending request deduplicated by identity, got %s vs %s", second.ID, first.ID)
	}

	approvedBefore, err := svc.HasApprovedJITProvisionApproval("tenant_demo_jakarta", email, externalID)
	if err != nil {
		t.Fatalf("expected has approved check success: %v", err)
	}
	if approvedBefore {
		t.Fatalf("expected approvedBefore=false")
	}

	reviewed, err := svc.ReviewJITProvisionApproval(
		"tenant_demo_jakarta",
		first.ID,
		"approved",
		"tenant.admin@sudirman.co",
		"manual approve for onboarding",
	)
	if err != nil {
		t.Fatalf("expected review approval success: %v", err)
	}
	if reviewed.Status != "approved" {
		t.Fatalf("expected reviewed status approved, got %s", reviewed.Status)
	}
	if reviewed.ReviewedBy != "tenant.admin@sudirman.co" {
		t.Fatalf("unexpected reviewed_by: %s", reviewed.ReviewedBy)
	}
	if reviewed.ReviewedAt == nil {
		t.Fatalf("expected reviewed_at set")
	}

	approvedAfter, err := svc.HasApprovedJITProvisionApproval("tenant_demo_jakarta", email, externalID)
	if err != nil {
		t.Fatalf("expected has approved check success after review: %v", err)
	}
	if !approvedAfter {
		t.Fatalf("expected approvedAfter=true")
	}

	approvedItems := svc.ListJITProvisionApprovals("tenant_demo_jakarta", "approved", 10)
	if len(approvedItems) == 0 {
		t.Fatalf("expected at least one approved item")
	}
	if approvedItems[0].ID != first.ID {
		t.Fatalf("expected reviewed item at first position, got %s", approvedItems[0].ID)
	}
}

func TestReviewJITProvisionApprovalRejectsInvalidDecision(t *testing.T) {
	svc := NewService()
	item, err := svc.UpsertJITProvisionApprovalRequest(
		"tenant_demo_jakarta",
		"jit.approval.invalid@sudirman.co",
		"sub-jit-approval-invalid-001",
		"oidc",
		"active",
	)
	if err != nil {
		t.Fatalf("expected upsert approval request success: %v", err)
	}

	_, reviewErr := svc.ReviewJITProvisionApproval(
		"tenant_demo_jakarta",
		item.ID,
		"pending",
		"tenant.admin@sudirman.co",
		"",
	)
	if !errors.Is(reviewErr, ErrInvalidJITProvisionApprovalDecision) {
		t.Fatalf("expected ErrInvalidJITProvisionApprovalDecision, got %v", reviewErr)
	}
}

func TestJITProvisionApprovalExternalSyncFlow(t *testing.T) {
	svc := NewService()
	item, err := svc.UpsertJITProvisionApprovalRequest(
		"tenant_demo_jakarta",
		"jit.approval.sync@sudirman.co",
		"sub-jit-approval-sync-001",
		"oidc",
		"active",
	)
	if err != nil {
		t.Fatalf("expected upsert approval request success: %v", err)
	}

	_, err = svc.ReviewJITProvisionApproval(
		"tenant_demo_jakarta",
		item.ID,
		"approved",
		"tenant.admin@sudirman.co",
		"approved for upstream sync",
	)
	if err != nil {
		t.Fatalf("expected review approval success: %v", err)
	}

	pending := svc.ListPendingJITProvisionApprovalExternalSync("tenant_demo_jakarta", 10)
	if len(pending) == 0 {
		t.Fatalf("expected pending external sync item")
	}
	if pending[0].ExternalSyncStatus != "pending" {
		t.Fatalf("expected external_sync_status pending, got %s", pending[0].ExternalSyncStatus)
	}

	failed, err := svc.UpdateJITProvisionApprovalExternalSync(
		"tenant_demo_jakarta",
		item.ID,
		"failed",
		"hris-sync-job-001",
		"upstream timeout",
	)
	if err != nil {
		t.Fatalf("expected external sync failed update success: %v", err)
	}
	if failed.ExternalSyncStatus != "failed" {
		t.Fatalf("expected failed external sync status, got %s", failed.ExternalSyncStatus)
	}
	if failed.ExternalSyncAttemptCount != 1 {
		t.Fatalf("expected attempt_count=1, got %d", failed.ExternalSyncAttemptCount)
	}
	if failed.ExternalSyncLastError != "upstream timeout" {
		t.Fatalf("unexpected last error: %s", failed.ExternalSyncLastError)
	}

	synced, err := svc.UpdateJITProvisionApprovalExternalSync(
		"tenant_demo_jakarta",
		item.ID,
		"synced",
		"hris-sync-job-002",
		"",
	)
	if err != nil {
		t.Fatalf("expected external sync synced update success: %v", err)
	}
	if synced.ExternalSyncStatus != "synced" {
		t.Fatalf("expected synced status, got %s", synced.ExternalSyncStatus)
	}
	if synced.ExternalSyncLastError != "" {
		t.Fatalf("expected last error cleared, got %s", synced.ExternalSyncLastError)
	}

	pendingAfter := svc.ListPendingJITProvisionApprovalExternalSync("tenant_demo_jakarta", 10)
	if len(pendingAfter) != 0 {
		t.Fatalf("expected no pending external sync after synced update, got %d", len(pendingAfter))
	}
}

func TestJITProvisionApprovalExternalSyncRejectsInvalidStatus(t *testing.T) {
	svc := NewService()
	item, err := svc.UpsertJITProvisionApprovalRequest(
		"tenant_demo_jakarta",
		"jit.approval.sync.invalid@sudirman.co",
		"sub-jit-approval-sync-invalid-001",
		"oidc",
		"active",
	)
	if err != nil {
		t.Fatalf("expected upsert approval request success: %v", err)
	}
	_, err = svc.ReviewJITProvisionApproval(
		"tenant_demo_jakarta",
		item.ID,
		"approved",
		"tenant.admin@sudirman.co",
		"",
	)
	if err != nil {
		t.Fatalf("expected review approval success: %v", err)
	}

	_, syncErr := svc.UpdateJITProvisionApprovalExternalSync(
		"tenant_demo_jakarta",
		item.ID,
		"pending",
		"",
		"",
	)
	if !errors.Is(syncErr, ErrInvalidJITProvisionApprovalExternalSyncStatus) {
		t.Fatalf("expected ErrInvalidJITProvisionApprovalExternalSyncStatus, got %v", syncErr)
	}
}
