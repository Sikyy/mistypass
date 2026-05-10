package httpx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/credential"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
	"github.com/mistypass/cloud/api/internal/modules/space"
)

func TestGatewayCredentialSyncRequiresDeviceIdentityAndScopesCredentials(t *testing.T) {
	s := newGatewayDeviceRequestTestServer()
	s.setGatewayDeviceToken("gw_demo_001", "dev-token-sync")

	code, body := callGatewayCredentialSync(t, s, "dev-token-sync", "", map[string]string{
		"X-Gateway-ID": "gw_demo_001",
		"X-Tenant-ID":  "tenant_demo_jakarta",
	})
	if code != http.StatusOK {
		t.Fatalf("expected credential sync 200, got %d body=%v", code, body)
	}
	if body.GatewayID != "gw_demo_001" || body.TenantID != "tenant_demo_jakarta" {
		t.Fatalf("unexpected gateway identity in response: %+v", body)
	}
	if len(body.BoundDoorIDs) == 0 {
		t.Fatalf("expected bound door ids in response")
	}
	if body.CredentialCount != len(body.Credentials) {
		t.Fatalf("credential_count mismatch: %+v", body)
	}
	if _, ok := findGatewayCredentialSyncByUser(body.Credentials, "usr_1001"); !ok {
		t.Fatalf("expected jakarta user credential in response: %+v", body.Credentials)
	}
	if _, ok := findGatewayCredentialSyncByUser(body.Credentials, "usr_1002"); ok {
		t.Fatalf("expected factory tenant credential to be excluded: %+v", body.Credentials)
	}
	for _, entry := range body.Credentials {
		for _, lockID := range entry.LockIDs {
			if !stringSliceContains(body.BoundDoorIDs, lockID) {
				t.Fatalf("credential %s contains non-gateway lock %s", entry.UserID, lockID)
			}
		}
	}

	code, _ = callGatewayCredentialSync(t, s, "", "", map[string]string{
		"X-Gateway-ID": "gw_demo_001",
		"X-Tenant-ID":  "tenant_demo_jakarta",
	})
	if code != http.StatusUnauthorized {
		t.Fatalf("expected missing device token to fail, got %d", code)
	}

	code, _ = callGatewayCredentialSync(t, s, "dev-token-sync", "", nil)
	if code != http.StatusBadRequest {
		t.Fatalf("expected missing gateway identity to fail, got %d", code)
	}
}

func TestGatewayCredentialSyncSinceVersionExcludesSeenEntries(t *testing.T) {
	s := newGatewayDeviceRequestTestServer()
	s.setGatewayDeviceToken("gw_demo_001", "dev-token-sync")

	code, body := callGatewayCredentialSync(t, s, "dev-token-sync", "", map[string]string{
		"X-Gateway-ID": "gw_demo_001",
		"X-Tenant-ID":  "tenant_demo_jakarta",
	})
	if code != http.StatusOK {
		t.Fatalf("expected initial credential sync 200, got %d body=%v", code, body)
	}
	entry, ok := findGatewayCredentialSyncByUser(body.Credentials, "usr_1001")
	if !ok {
		t.Fatalf("expected usr_1001 credential in initial response: %+v", body.Credentials)
	}

	code, body = callGatewayCredentialSync(t, s, "dev-token-sync", "?since_version="+fmt.Sprint(entry.SyncVersion), map[string]string{
		"X-Gateway-ID": "gw_demo_001",
		"X-Tenant-ID":  "tenant_demo_jakarta",
	})
	if code != http.StatusOK {
		t.Fatalf("expected incremental credential sync 200, got %d body=%v", code, body)
	}
	if _, ok := findGatewayCredentialSyncByUser(body.Credentials, "usr_1001"); ok {
		t.Fatalf("expected since_version to exclude usr_1001, got %+v", body.Credentials)
	}
}

func TestGatewayAuditBatchRequiresDeviceIdentityAndUsesGatewayContext(t *testing.T) {
	s := newGatewayDeviceRequestTestServer()
	s.setGatewayDeviceToken("gw_demo_001", "dev-token-audit")

	payload := map[string]any{
		"entries": []map[string]any{
			{
				"event_id":   "evt_gateway_audit_001",
				"user_id":    "usr_1001",
				"lock_id":    "door_jkt_001",
				"method":     "ble",
				"result":     "granted",
				"reason":     "offline cache",
				"gateway_id": 1,
				"timestamp":  time.Now().Add(-time.Minute).Unix(),
				"is_offline": true,
			},
		},
	}
	code, body := callGatewayAuditBatch(t, s, "dev-token-audit", payload, map[string]string{
		"X-Gateway-ID": "gw_demo_001",
		"X-Tenant-ID":  "tenant_demo_jakarta",
	})
	if code != http.StatusOK {
		t.Fatalf("expected audit batch 200, got %d body=%v", code, body)
	}
	if body["gateway_id"] != "gw_demo_001" || body["tenant_id"] != "tenant_demo_jakarta" {
		t.Fatalf("unexpected gateway identity in response: %v", body)
	}
	if accepted, _ := body["accepted"].(float64); int(accepted) != 1 {
		t.Fatalf("expected accepted=1, got %v", body)
	}

	var found bool
	for _, log := range s.auditSvc.List("tenant_demo_jakarta") {
		if log.Action == "offline_access_granted" && log.Actor == "usr_1001" && log.Target == "door_jkt_001 event_id=evt_gateway_audit_001 gateway_id=gw_demo_001" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected offline audit log under authenticated tenant/gateway context")
	}

	code, _ = callGatewayAuditBatch(t, s, "", payload, map[string]string{
		"X-Gateway-ID": "gw_demo_001",
		"X-Tenant-ID":  "tenant_demo_jakarta",
	})
	if code != http.StatusUnauthorized {
		t.Fatalf("expected missing device token to fail, got %d", code)
	}
}

func newGatewayDeviceRequestTestServer() *server {
	return &server{
		gatewaySvc:          gateway.NewService(),
		spaceSvc:            space.NewService(),
		accessSvc:           access.NewService(),
		credentialSvc:       credential.NewService(),
		auditSvc:            audit.NewService(),
		gatewayDeviceTokens: map[string]string{},
	}
}

type gatewayCredentialSyncResponse struct {
	GatewayID       string                             `json:"gateway_id"`
	TenantID        string                             `json:"tenant_id"`
	BoundDoorIDs    []string                           `json:"bound_door_ids"`
	Credentials     []credential.GatewayCredentialSync `json:"credentials"`
	CredentialCount int                                `json:"credential_count"`
}

func callGatewayCredentialSync(t *testing.T, s *server, deviceToken, rawQuery string, headers map[string]string) (int, gatewayCredentialSyncResponse) {
	t.Helper()

	target := "/api/v1/gateway/credentials/sync" + rawQuery
	request := httptest.NewRequest(http.MethodGet, target, nil)
	if deviceToken != "" {
		request.Header.Set("X-Device-Token", deviceToken)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	s.gatewayCredentialSync(recorder, request)

	var body gatewayCredentialSyncResponse
	if recorder.Body.Len() > 0 {
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode credential sync response: %v body=%s", err, recorder.Body.String())
		}
	}
	return recorder.Code, body
}

func callGatewayAuditBatch(t *testing.T, s *server, deviceToken string, payload map[string]any, headers map[string]string) (int, map[string]any) {
	t.Helper()

	requestBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal audit batch request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/audit/batch", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	if deviceToken != "" {
		request.Header.Set("X-Device-Token", deviceToken)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	s.gatewayAuditBatch(recorder, request)

	if recorder.Body.Len() == 0 {
		return recorder.Code, map[string]any{}
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode audit batch response: %v body=%s", err, recorder.Body.String())
	}
	return recorder.Code, body
}

func findGatewayCredentialSyncByUser(items []credential.GatewayCredentialSync, userID string) (credential.GatewayCredentialSync, bool) {
	for _, item := range items {
		if item.UserID == userID {
			return item, true
		}
	}
	return credential.GatewayCredentialSync{}, false
}

func stringSliceContains(items []string, expected string) bool {
	for _, item := range items {
		if item == expected {
			return true
		}
	}
	return false
}
