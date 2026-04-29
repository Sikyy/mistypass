package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestReferenceAccessRightsImpactPreviewAndBulkReview(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-access-rights-review-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	createAssignmentBody := []byte(`{"role_assignment":{"tenant_id":"tenant_demo_jakarta","role_id":"role_place_admin","applies_to_type":"Place","applies_to_id":"building_demo_001","assignee_type":"User","assignee_id":"usr_1001","assignee_email":"andri.pratama@mistypass.local","valid_until":"2026-01-01T00:00:00Z"}}`)
	createAssignmentRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/role_assignments", token, createAssignmentBody)
	if createAssignmentRecorder.Code != http.StatusCreated {
		t.Fatalf("expected role assignment create status 201, got %d body=%s", createAssignmentRecorder.Code, createAssignmentRecorder.Body.String())
	}
	var createdAssignment struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createAssignmentRecorder.Body.Bytes(), &createdAssignment); err != nil {
		t.Fatalf("decode created assignment: %v", err)
	}

	createShareBody := []byte(`{"share":{"tenant_id":"tenant_demo_jakarta","email":"review.guest@example.test","grantee_name":"Review Guest","group_id":"ug_common_office_jkt","place_id":"building_demo_001","valid_until":"2026-01-01T00:00:00Z"}}`)
	createShareRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/shares", token, createShareBody)
	if createShareRecorder.Code != http.StatusCreated {
		t.Fatalf("expected share create status 201, got %d body=%s", createShareRecorder.Code, createShareRecorder.Body.String())
	}
	var createdShare struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createShareRecorder.Body.Bytes(), &createdShare); err != nil {
		t.Fatalf("decode created share: %v", err)
	}

	selectionBody := []byte(`{"tenant_id":"tenant_demo_jakarta","role_assignment_ids":["` + createdAssignment.ID + `"],"share_ids":["` + createdShare.ID + `"]}`)
	previewRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/access_rights/impact_preview", token, selectionBody)
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("expected access rights preview status 200, got %d body=%s", previewRecorder.Code, previewRecorder.Body.String())
	}
	var preview struct {
		SelectedCount    int `json:"selected_count"`
		NeedsReviewCount int `json:"needs_review_count"`
		AffectedUsers    int `json:"affected_users"`
		AffectedPlaces   int `json:"affected_places"`
		Items            []struct {
			ID          string `json:"id"`
			SourceType  string `json:"source_type"`
			Status      string `json:"status"`
			NeedsReview bool   `json:"needs_review"`
		} `json:"items"`
	}
	if err := json.Unmarshal(previewRecorder.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if preview.SelectedCount != 2 || preview.NeedsReviewCount != 2 || preview.AffectedUsers < 2 || preview.AffectedPlaces < 2 {
		t.Fatalf("unexpected preview counters: %#v body=%s", preview, previewRecorder.Body.String())
	}
	for _, item := range preview.Items {
		if item.Status != "expired" || !item.NeedsReview {
			t.Fatalf("expected expired item needing review, got %#v body=%s", item, previewRecorder.Body.String())
		}
	}

	reviewRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/access_rights/review", token, selectionBody)
	if reviewRecorder.Code != http.StatusOK {
		t.Fatalf("expected access rights review status 200, got %d body=%s", reviewRecorder.Code, reviewRecorder.Body.String())
	}
	if !strings.Contains(reviewRecorder.Body.String(), `"reviewed_count":2`) ||
		!strings.Contains(reviewRecorder.Body.String(), `"reviewed_by":"organization.admin@mistypass.local"`) {
		t.Fatalf("expected review response, body=%s", reviewRecorder.Body.String())
	}

	getAssignmentRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/role_assignments/"+createdAssignment.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getAssignmentRecorder.Code != http.StatusOK {
		t.Fatalf("expected role assignment detail status 200, got %d body=%s", getAssignmentRecorder.Code, getAssignmentRecorder.Body.String())
	}
	if !strings.Contains(getAssignmentRecorder.Body.String(), `"reviewed_by":"organization.admin@mistypass.local"`) ||
		!strings.Contains(getAssignmentRecorder.Body.String(), `"reviewed_at"`) {
		t.Fatalf("expected role assignment review metadata, body=%s", getAssignmentRecorder.Body.String())
	}
	getShareRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/shares/"+createdShare.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getShareRecorder.Code != http.StatusOK {
		t.Fatalf("expected share detail status 200, got %d body=%s", getShareRecorder.Code, getShareRecorder.Body.String())
	}
	if !strings.Contains(getShareRecorder.Body.String(), `"reviewed_by":"organization.admin@mistypass.local"`) ||
		!strings.Contains(getShareRecorder.Body.String(), `"reviewed_at"`) {
		t.Fatalf("expected share review metadata, body=%s", getShareRecorder.Body.String())
	}

	previewAfterReviewRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/access_rights/impact_preview", token, selectionBody)
	if previewAfterReviewRecorder.Code != http.StatusOK {
		t.Fatalf("expected reviewed preview status 200, got %d body=%s", previewAfterReviewRecorder.Code, previewAfterReviewRecorder.Body.String())
	}
	if !strings.Contains(previewAfterReviewRecorder.Body.String(), `"needs_review_count":0`) {
		t.Fatalf("expected reviewed items to leave needs-review queue, body=%s", previewAfterReviewRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_access_rights_reviewed", "reviewed_count=2", "role_assignment_ids="+createdAssignment.ID, "share_ids="+createdShare.ID)
}

func TestReferenceAccessRightsScheduleTemplatesAndShareValidFrom(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-access-rights-schedule-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	templatesRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/access_rights/schedule_templates?tenant_id=tenant_demo_jakarta", token, nil)
	if templatesRecorder.Code != http.StatusOK {
		t.Fatalf("expected schedule template status 200, got %d body=%s", templatesRecorder.Code, templatesRecorder.Body.String())
	}
	type scheduleTemplateResponseItem struct {
		ID           string   `json:"id"`
		Name         string   `json:"name"`
		ValidFrom    string   `json:"valid_from"`
		ValidUntil   string   `json:"valid_until"`
		DurationDays int      `json:"duration_days"`
		SourceTypes  []string `json:"source_types"`
	}
	var templatesResponse struct {
		Items []scheduleTemplateResponseItem `json:"items"`
	}
	if err := json.Unmarshal(templatesRecorder.Body.Bytes(), &templatesResponse); err != nil {
		t.Fatalf("decode templates: %v", err)
	}
	templatesByID := map[string]scheduleTemplateResponseItem{}
	for _, item := range templatesResponse.Items {
		templatesByID[item.ID] = item
	}
	for _, templateID := range []string{"immediate_7_days", "immediate_30_days", "starts_tomorrow_7_days"} {
		item, exists := templatesByID[templateID]
		if !exists || item.Name == "" || item.ValidUntil == "" || item.DurationDays == 0 || len(item.SourceTypes) == 0 {
			t.Fatalf("expected populated schedule template %q, got %#v body=%s", templateID, item, templatesRecorder.Body.String())
		}
		if _, err := time.Parse(time.RFC3339, item.ValidUntil); err != nil {
			t.Fatalf("expected template %q valid_until to be RFC3339, got %q: %v", templateID, item.ValidUntil, err)
		}
	}
	startsTomorrow := templatesByID["starts_tomorrow_7_days"]
	if startsTomorrow.ValidFrom == "" {
		t.Fatalf("expected starts_tomorrow_7_days valid_from, body=%s", templatesRecorder.Body.String())
	}
	startsAt, err := time.Parse(time.RFC3339, startsTomorrow.ValidFrom)
	if err != nil {
		t.Fatalf("expected starts_tomorrow_7_days valid_from to be RFC3339, got %q: %v", startsTomorrow.ValidFrom, err)
	}
	if !startsAt.After(time.Now().UTC()) {
		t.Fatalf("expected starts_tomorrow_7_days to start in the future, got %s", startsTomorrow.ValidFrom)
	}

	createShareBody := []byte(`{"share":{"tenant_id":"tenant_demo_jakarta","email":"scheduled.share@example.test","grantee_name":"Scheduled Guest","group_id":"ug_common_office_jkt","place_id":"building_demo_001","valid_from":"2099-01-01T00:00:00Z","valid_until":"2099-01-08T00:00:00Z"}}`)
	createShareRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/shares", token, createShareBody)
	if createShareRecorder.Code != http.StatusCreated {
		t.Fatalf("expected share create status 201, got %d body=%s", createShareRecorder.Code, createShareRecorder.Body.String())
	}
	var createdShare struct {
		ID        string `json:"id"`
		ValidFrom string `json:"valid_from"`
	}
	if err := json.Unmarshal(createShareRecorder.Body.Bytes(), &createdShare); err != nil {
		t.Fatalf("decode created share: %v", err)
	}
	if createdShare.ID == "" || createdShare.ValidFrom != "2099-01-01T00:00:00Z" {
		t.Fatalf("expected created share to persist valid_from, got %#v body=%s", createdShare, createShareRecorder.Body.String())
	}

	getShareRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/shares/"+createdShare.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getShareRecorder.Code != http.StatusOK {
		t.Fatalf("expected share detail status 200, got %d body=%s", getShareRecorder.Code, getShareRecorder.Body.String())
	}
	if !strings.Contains(getShareRecorder.Body.String(), `"valid_from":"2099-01-01T00:00:00Z"`) {
		t.Fatalf("expected share detail to include valid_from, body=%s", getShareRecorder.Body.String())
	}

	selectionBody := []byte(`{"tenant_id":"tenant_demo_jakarta","share_ids":["` + createdShare.ID + `"]}`)
	previewRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/access_rights/impact_preview", token, selectionBody)
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("expected access rights preview status 200, got %d body=%s", previewRecorder.Code, previewRecorder.Body.String())
	}
	var preview struct {
		Items []struct {
			ID         string `json:"id"`
			SourceType string `json:"source_type"`
			Status     string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(previewRecorder.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if len(preview.Items) != 1 || preview.Items[0].ID != createdShare.ID || preview.Items[0].SourceType != "share" || preview.Items[0].Status != "scheduled" {
		t.Fatalf("expected scheduled share preview item, got %#v body=%s", preview.Items, previewRecorder.Body.String())
	}
}

func TestReferenceAccessRightsBulkScheduleUpdate(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-access-rights-bulk-schedule-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	createAssignmentBody := []byte(`{"role_assignment":{"tenant_id":"tenant_demo_jakarta","role_id":"role_place_admin","applies_to_type":"Place","applies_to_id":"building_demo_001","assignee_type":"User","assignee_id":"usr_1001","assignee_email":"andri.pratama@mistypass.local","valid_until":"2026-01-01T00:00:00Z"}}`)
	createAssignmentRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/role_assignments", token, createAssignmentBody)
	if createAssignmentRecorder.Code != http.StatusCreated {
		t.Fatalf("expected role assignment create status 201, got %d body=%s", createAssignmentRecorder.Code, createAssignmentRecorder.Body.String())
	}
	var createdAssignment struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createAssignmentRecorder.Body.Bytes(), &createdAssignment); err != nil {
		t.Fatalf("decode created assignment: %v", err)
	}

	createShareBody := []byte(`{"share":{"tenant_id":"tenant_demo_jakarta","email":"bulk.schedule@example.test","grantee_name":"Bulk Schedule Guest","group_id":"ug_common_office_jkt","place_id":"building_demo_001","valid_until":"2026-01-01T00:00:00Z"}}`)
	createShareRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/shares", token, createShareBody)
	if createShareRecorder.Code != http.StatusCreated {
		t.Fatalf("expected share create status 201, got %d body=%s", createShareRecorder.Code, createShareRecorder.Body.String())
	}
	var createdShare struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createShareRecorder.Body.Bytes(), &createdShare); err != nil {
		t.Fatalf("decode created share: %v", err)
	}

	scheduleBody := []byte(`{"tenant_id":"tenant_demo_jakarta","role_assignment_ids":["` + createdAssignment.ID + `"],"share_ids":["` + createdShare.ID + `"],"valid_from":"2099-02-01T00:00:00Z","valid_until":"2099-02-08T00:00:00Z"}`)
	scheduleRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/access_rights/schedule", token, scheduleBody)
	if scheduleRecorder.Code != http.StatusOK {
		t.Fatalf("expected bulk schedule update status 200, got %d body=%s", scheduleRecorder.Code, scheduleRecorder.Body.String())
	}
	if !strings.Contains(scheduleRecorder.Body.String(), `"updated_count":2`) ||
		!strings.Contains(scheduleRecorder.Body.String(), `"valid_from":"2099-02-01T00:00:00Z"`) ||
		!strings.Contains(scheduleRecorder.Body.String(), `"valid_until":"2099-02-08T00:00:00Z"`) {
		t.Fatalf("expected bulk schedule response, body=%s", scheduleRecorder.Body.String())
	}

	getAssignmentRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/role_assignments/"+createdAssignment.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getAssignmentRecorder.Code != http.StatusOK {
		t.Fatalf("expected role assignment detail status 200, got %d body=%s", getAssignmentRecorder.Code, getAssignmentRecorder.Body.String())
	}
	if !strings.Contains(getAssignmentRecorder.Body.String(), `"valid_from":"2099-02-01T00:00:00Z"`) ||
		!strings.Contains(getAssignmentRecorder.Body.String(), `"valid_until":"2099-02-08T00:00:00Z"`) {
		t.Fatalf("expected role assignment schedule to update, body=%s", getAssignmentRecorder.Body.String())
	}
	getShareRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/shares/"+createdShare.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getShareRecorder.Code != http.StatusOK {
		t.Fatalf("expected share detail status 200, got %d body=%s", getShareRecorder.Code, getShareRecorder.Body.String())
	}
	if !strings.Contains(getShareRecorder.Body.String(), `"valid_from":"2099-02-01T00:00:00Z"`) ||
		!strings.Contains(getShareRecorder.Body.String(), `"valid_until":"2099-02-08T00:00:00Z"`) {
		t.Fatalf("expected share schedule to update, body=%s", getShareRecorder.Body.String())
	}

	selectionBody := []byte(`{"tenant_id":"tenant_demo_jakarta","role_assignment_ids":["` + createdAssignment.ID + `"],"share_ids":["` + createdShare.ID + `"]}`)
	previewRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/access_rights/impact_preview", token, selectionBody)
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("expected preview status 200, got %d body=%s", previewRecorder.Code, previewRecorder.Body.String())
	}
	var preview struct {
		Items []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(previewRecorder.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if len(preview.Items) != 2 {
		t.Fatalf("expected two preview items, got %#v body=%s", preview.Items, previewRecorder.Body.String())
	}
	for _, item := range preview.Items {
		if item.Status != "scheduled" {
			t.Fatalf("expected scheduled preview item, got %#v body=%s", item, previewRecorder.Body.String())
		}
	}
	assertReferenceAuditLog(t, router, token, "reference_access_rights_schedule_updated", "updated_count=2", "role_assignment_ids="+createdAssignment.ID, "share_ids="+createdShare.ID)
}
