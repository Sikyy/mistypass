package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/event"
)

func TestAlertPolicySchedulerDispatchesOnEventIngest(t *testing.T) {
	srv, handler := newTestServerWithHandler(t, config.Config{
		JWTSecret:       "alert-scheduler-test",
		EnableDemoUsers: true,
	})
	token := referenceAPILogin(t, handler, "organization.admin@mistypass.local")

	// create a custom policy that matches access denied events
	policyBody := []byte(`{"alert_policy":{"tenant_id":"tenant_demo_jakarta","name":"Denied Access Alert","category":"custom","trigger":"access_denied","severity":"high","condition_expression":"event.result == \"denied\"","status":"active","enabled":true,"threshold":1,"window_seconds":900,"cooldown_seconds":5,"channels":{"email":true},"receiver_groups":["security"]}}`)
	createRec := referenceAPIRequest(t, handler, http.MethodPost, "/api/v1/alert_policies", token, policyBody)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected policy create 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created policy: %v", err)
	}

	// ingest an access event that matches the condition (result=denied)
	_, _, err := srv.eventSvc.IngestAccessEvent(event.IngestAccessEventInput{
		TenantID:   "tenant_demo_jakarta",
		BuildingID: "building_demo_001",
		Type:       "card_tap",
		Actor:      "test_user",
		DoorID:     "door_test_001",
		GatewayID:  "gw_demo_001",
		Result:     "denied",
		At:         time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ingest event: %v", err)
	}

	// give scheduler goroutine time to process
	time.Sleep(300 * time.Millisecond)

	// query notifications
	notifRec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/alert_policies/notifications?tenant_id=tenant_demo_jakarta&policy_id="+created.ID, token, nil)
	if notifRec.Code != http.StatusOK {
		t.Fatalf("expected notifications 200, got %d body=%s", notifRec.Code, notifRec.Body.String())
	}
	var notifResult struct {
		Items []struct {
			ID         string `json:"id"`
			PolicyID   string `json:"policy_id"`
			PolicyName string `json:"policy_name"`
			Severity   string `json:"severity"`
			EventType  string `json:"event_type"`
			Status     string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(notifRec.Body.Bytes(), &notifResult); err != nil {
		t.Fatalf("decode notifications: %v", err)
	}
	if len(notifResult.Items) == 0 {
		t.Fatalf("expected at least 1 notification, got 0")
	}
	found := false
	for _, item := range notifResult.Items {
		if item.PolicyID == created.ID && item.Severity == "high" && item.EventType == "card_tap" {
			found = true
			if item.Status != "dispatched" {
				t.Errorf("expected status=dispatched, got %s", item.Status)
			}
			break
		}
	}
	if !found {
		t.Errorf("notification for policy %s not found in %d items", created.ID, len(notifResult.Items))
	}
}

func TestAlertPolicyNotificationsEndpointFilters(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "alert-notif-filter-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/alert_policies/notifications?tenant_id=tenant_demo_jakarta&severity=critical&limit=10", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "\"items\"") {
		t.Errorf("expected items array in response, got: %s", body[:min(100, len(body))])
	}
}

func TestAlertPolicyCooldownPreventsRepeatNotification(t *testing.T) {
	srv, handler := newTestServerWithHandler(t, config.Config{
		JWTSecret:       "alert-cooldown-test",
		EnableDemoUsers: true,
	})
	token := referenceAPILogin(t, handler, "organization.admin@mistypass.local")

	// create policy with 60s cooldown
	policyBody := []byte(`{"alert_policy":{"tenant_id":"tenant_demo_jakarta","name":"Cooldown Test","category":"custom","trigger":"cooldown_test","severity":"medium","condition_expression":"event.result == \"denied\"","status":"active","enabled":true,"threshold":1,"window_seconds":900,"cooldown_seconds":60,"channels":{"email":true},"receiver_groups":["ops"]}}`)
	createRec := referenceAPIRequest(t, handler, http.MethodPost, "/api/v1/alert_policies", token, policyBody)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(createRec.Body.Bytes(), &created)

	// ingest first event
	srv.eventSvc.IngestAccessEvent(event.IngestAccessEventInput{
		TenantID: "tenant_demo_jakarta", BuildingID: "building_demo_001",
		Type: "card_tap", Actor: "test", DoorID: "d1", GatewayID: "gw_demo_001",
		Result: "denied", At: time.Now().UTC(),
	})
	time.Sleep(300 * time.Millisecond)

	// count notifications
	rec1 := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/alert_policies/notifications?tenant_id=tenant_demo_jakarta&policy_id="+created.ID, token, nil)
	var r1 struct{ Items []any `json:"items"` }
	_ = json.Unmarshal(rec1.Body.Bytes(), &r1)
	count1 := len(r1.Items)

	// ingest second event immediately (should be suppressed by cooldown)
	srv.eventSvc.IngestAccessEvent(event.IngestAccessEventInput{
		TenantID: "tenant_demo_jakarta", BuildingID: "building_demo_001",
		Type: "card_tap", Actor: "test2", DoorID: "d2", GatewayID: "gw_demo_001",
		Result: "denied", At: time.Now().UTC(),
	})
	time.Sleep(300 * time.Millisecond)

	rec2 := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/alert_policies/notifications?tenant_id=tenant_demo_jakarta&policy_id="+created.ID, token, nil)
	var r2 struct{ Items []any `json:"items"` }
	_ = json.Unmarshal(rec2.Body.Bytes(), &r2)
	count2 := len(r2.Items)

	if count2 != count1 {
		t.Errorf("cooldown should prevent second notification: count1=%d count2=%d", count1, count2)
	}
}
