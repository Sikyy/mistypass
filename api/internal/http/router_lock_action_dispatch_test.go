package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestLockActionDispatchAuditAndStatus(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "lock-dispatch-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// use existing demo door (from demo data)
	// list locks to find an existing one
	listRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/locks?tenant_id=tenant_demo_jakarta&limit=1", token, nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listRec.Code)
	}
	var listResult struct {
		Items []struct{ ID string `json:"id"` } `json:"items"`
	}
	_ = json.Unmarshal(listRec.Body.Bytes(), &listResult)
	if len(listResult.Items) == 0 {
		t.Skip("no demo locks available")
	}
	door := listResult.Items[0]

	// unlock without bound gateway — should return "accepted" (no NATS dispatch)
	unlockRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/locks/"+door.ID+"/unlock?tenant_id=tenant_demo_jakarta", token, nil)
	if unlockRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", unlockRec.Code, unlockRec.Body.String())
	}
	var unlockResult struct {
		Status string `json:"status"`
		Action string `json:"action"`
		LockID string `json:"lock_id"`
	}
	_ = json.Unmarshal(unlockRec.Body.Bytes(), &unlockResult)
	if unlockResult.Status != "accepted" {
		t.Errorf("expected status=accepted (no gateway bound), got %s", unlockResult.Status)
	}
	if unlockResult.Action != "unlock" {
		t.Errorf("expected action=unlock, got %s", unlockResult.Action)
	}

	// verify audit log was created
	assertReferenceAuditLog(t, router, token, "lock_action_unlock", "lock_id="+door.ID)
}

func TestPlaceActionDispatchAudit(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "place-dispatch-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// lockdown a demo place
	lockdownRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/places/building_demo_001/lock_down?tenant_id=tenant_demo_jakarta", token, nil)
	if lockdownRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", lockdownRec.Code, lockdownRec.Body.String())
	}
	var result struct {
		Action    string `json:"action"`
		Status    string `json:"status"`
		LockCount int    `json:"lock_count"`
	}
	_ = json.Unmarshal(lockdownRec.Body.Bytes(), &result)
	if result.Action != "lock_down" {
		t.Errorf("expected action=lock_down, got %s", result.Action)
	}
	if result.LockCount < 0 {
		t.Errorf("expected non-negative lock count")
	}

	// audit
	auditRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/audit-logs?tenant_id=tenant_demo_jakarta&action=place_action_lock_down&limit=1", token, nil)
	if auditRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from audit, got %d", auditRec.Code)
	}
	if !strings.Contains(auditRec.Body.String(), "place_action_lock_down") {
		t.Errorf("expected place_action_lock_down audit log")
	}
}
