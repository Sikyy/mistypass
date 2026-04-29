package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestReferenceCustomAlertPolicyCRUD(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-custom-alert-policy-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	createBody := []byte(`{"alert_policy":{"tenant_id":"tenant_demo_jakarta","category":"custom","name":"After-hours access review","description":"Escalate denied unlocks outside business hours.","trigger":"access_denied_after_hours","severity":"critical","condition_expression":"event.type == 'access_denied' && event.hour >= 18","enabled":true,"threshold":2,"window_seconds":600,"cooldown_seconds":1200,"channels":{"email":true,"whatsapp":true},"receiver_groups":["security","ops"]}}`)
	createRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/alert_policies", token, createBody)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected custom alert policy create status 201, got %d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Category    string   `json:"category"`
		Trigger     string   `json:"trigger"`
		Severity    string   `json:"severity"`
		Condition   string   `json:"condition_expression"`
		Status      string   `json:"status"`
		Enabled     bool     `json:"enabled"`
		Threshold   int      `json:"threshold"`
		ReceiverIDs []string `json:"receiver_groups"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode custom alert policy create: %v", err)
	}
	if created.ID == "" || created.Category != "custom" || created.Trigger != "access_denied_after_hours" ||
		created.Severity != "critical" || created.Condition == "" || created.Status != "active" || !created.Enabled ||
		created.Threshold != 2 || len(created.ReceiverIDs) != 2 {
		t.Fatalf("unexpected created custom alert policy: %#v body=%s", created, createRecorder.Body.String())
	}

	listRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/alert_policies?tenant_id=tenant_demo_jakarta&category=custom", token, nil)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected custom alert policy list status 200, got %d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	if !strings.Contains(listRecorder.Body.String(), created.ID) || !strings.Contains(listRecorder.Body.String(), "condition_expression") {
		t.Fatalf("expected custom alert policy in filtered list, body=%s", listRecorder.Body.String())
	}

	previewBody := []byte(`{"tenant_id":"tenant_demo_jakarta","condition_expression":"event.type == 'access_denied' && event.hour >= 18","event":{"type":"access_denied","hour":20,"result":"denied"}}`)
	previewRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/alert_policies/condition_preview", token, previewBody)
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("expected custom alert policy condition preview status 200, got %d body=%s", previewRecorder.Code, previewRecorder.Body.String())
	}
	if !strings.Contains(previewRecorder.Body.String(), `"matched":true`) {
		t.Fatalf("expected custom alert policy condition preview match, body=%s", previewRecorder.Body.String())
	}

	savedPreviewBody := []byte(`{"tenant_id":"tenant_demo_jakarta","policy_id":"` + created.ID + `","event":{"type":"access_denied","hour":10,"result":"denied"}}`)
	savedPreviewRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/alert_policies/condition_preview", token, savedPreviewBody)
	if savedPreviewRecorder.Code != http.StatusOK {
		t.Fatalf("expected saved custom alert policy condition preview status 200, got %d body=%s", savedPreviewRecorder.Code, savedPreviewRecorder.Body.String())
	}
	if !strings.Contains(savedPreviewRecorder.Body.String(), `"matched":false`) {
		t.Fatalf("expected saved custom alert policy condition preview miss, body=%s", savedPreviewRecorder.Body.String())
	}

	invalidPreviewBody := []byte(`{"tenant_id":"tenant_demo_jakarta","condition_expression":"event.type =="}`)
	invalidPreviewRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/alert_policies/condition_preview", token, invalidPreviewBody)
	if invalidPreviewRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid custom alert policy condition preview status 400, got %d body=%s", invalidPreviewRecorder.Code, invalidPreviewRecorder.Body.String())
	}

	invalidCreateBody := []byte(`{"alert_policy":{"tenant_id":"tenant_demo_jakarta","category":"custom","trigger":"broken_condition","condition_expression":"event.type ==","enabled":true,"channels":{"email":true}}}`)
	invalidCreateRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/alert_policies", token, invalidCreateBody)
	if invalidCreateRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid custom alert policy create status 400, got %d body=%s", invalidCreateRecorder.Code, invalidCreateRecorder.Body.String())
	}

	evaluateBody := []byte(`{"tenant_id":"tenant_demo_jakarta","policy_ids":["` + created.ID + `"],"event":{"type":"access_denied","hour":21,"result":"denied","door_id":"door_jkt_001"}}`)
	evaluateRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/alert_policies/evaluate", token, evaluateBody)
	if evaluateRecorder.Code != http.StatusOK {
		t.Fatalf("expected custom alert policy event evaluate status 200, got %d body=%s", evaluateRecorder.Code, evaluateRecorder.Body.String())
	}
	if !strings.Contains(evaluateRecorder.Body.String(), `"matched_count":1`) ||
		!strings.Contains(evaluateRecorder.Body.String(), `"policy_id":"`+created.ID+`"`) ||
		!strings.Contains(evaluateRecorder.Body.String(), `"notification_summary":"email+whatsapp to security,ops"`) {
		t.Fatalf("expected custom alert policy event evaluation match, body=%s", evaluateRecorder.Body.String())
	}

	noMatchEvaluateBody := []byte(`{"tenant_id":"tenant_demo_jakarta","policy_ids":["` + created.ID + `"],"event":{"type":"access_denied","hour":8,"result":"denied","door_id":"door_jkt_001"}}`)
	noMatchEvaluateRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/alert_policies/evaluate", token, noMatchEvaluateBody)
	if noMatchEvaluateRecorder.Code != http.StatusOK {
		t.Fatalf("expected custom alert policy event evaluate no-match status 200, got %d body=%s", noMatchEvaluateRecorder.Code, noMatchEvaluateRecorder.Body.String())
	}
	if !strings.Contains(noMatchEvaluateRecorder.Body.String(), `"evaluated_count":1`) ||
		!strings.Contains(noMatchEvaluateRecorder.Body.String(), `"matched_count":0`) {
		t.Fatalf("expected custom alert policy event evaluation miss, body=%s", noMatchEvaluateRecorder.Body.String())
	}

	getRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/alert_policies/"+created.ID, token, nil)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected custom alert policy detail status 200, got %d body=%s", getRecorder.Code, getRecorder.Body.String())
	}
	if !strings.Contains(getRecorder.Body.String(), `"name":"After-hours access review"`) {
		t.Fatalf("expected custom alert policy detail, body=%s", getRecorder.Body.String())
	}

	updateBody := []byte(`{"alert_policy":{"tenant_id":"tenant_demo_jakarta","name":"After-hours denied access","severity":"high","condition_expression":"event.type == 'access_denied' && event.hour >= 20","threshold":3,"channels":{"email":true,"whatsapp":false},"receiver_groups":["security"]}}`)
	updateRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/alert_policies/"+created.ID, token, updateBody)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected custom alert policy update status 200, got %d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	if !strings.Contains(updateRecorder.Body.String(), `"name":"After-hours denied access"`) ||
		!strings.Contains(updateRecorder.Body.String(), `"severity":"high"`) ||
		!strings.Contains(updateRecorder.Body.String(), `"threshold":3`) ||
		!strings.Contains(updateRecorder.Body.String(), `"whatsapp":false`) {
		t.Fatalf("expected updated custom alert policy, body=%s", updateRecorder.Body.String())
	}

	deleteRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/alert_policies/"+created.ID, token, nil)
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected custom alert policy delete status 204, got %d body=%s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	getDisabledRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/alert_policies/"+created.ID, token, nil)
	if getDisabledRecorder.Code != http.StatusOK {
		t.Fatalf("expected disabled custom alert policy detail status 200, got %d body=%s", getDisabledRecorder.Code, getDisabledRecorder.Body.String())
	}
	if !strings.Contains(getDisabledRecorder.Body.String(), `"status":"inactive"`) || !strings.Contains(getDisabledRecorder.Body.String(), `"enabled":false`) {
		t.Fatalf("expected deleted custom alert policy to be disabled, body=%s", getDisabledRecorder.Body.String())
	}

	assertReferenceAuditLog(t, router, token, "reference_custom_alert_policy_created", "policy_id="+created.ID, "trigger=access_denied_after_hours", "severity=critical")
	assertReferenceAuditLog(t, router, token, "reference_custom_alert_policy_updated", "policy_id="+created.ID, "status=active")
	assertReferenceAuditLog(t, router, token, "reference_alert_policy_disabled", "policy_id="+created.ID, "category=custom")
	assertReferenceAuditLog(t, router, token, "reference_custom_alert_policy_event_evaluated", "evaluated=1", "matched=0")
}
