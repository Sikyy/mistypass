package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

func TestGatewayOTATaskLifecycle(t *testing.T) {
	s := &server{
		gatewaySvc: gateway.NewService(),
		auditSvc:   audit.NewService(),
	}

	createReqBody := map[string]any{
		"tenant_id":          "tenant_demo_jakarta",
		"firmware_version":   "v2.4.1",
		"firmware_url":       "https://cdn.example.com/firmware/gw_demo_001/v2.4.1.bin",
		"firmware_sha256":    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"firmware_signature": strings.Repeat("b", 128),
	}
	createReqBytes, err := json.Marshal(createReqBody)
	if err != nil {
		t.Fatalf("marshal create request failed: %v", err)
	}
	createReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/gateways/gw_demo_001/ota/tasks",
		bytes.NewReader(createReqBytes),
	)
	createReq = withGatewayMQTTURLParam(createReq, "gatewayID", "gw_demo_001")
	createReq = withGatewayMQTTUser(createReq, auth.User{
		ID:       "u1",
		Email:    "tenant-admin@example.com",
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
	})
	createRec := httptest.NewRecorder()
	s.createGatewayOTATask(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, createRec.Code, createRec.Body.String())
	}

	var created gateway.GatewayOTATask
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if created.ID == "" || created.GatewayID != "gw_demo_001" || created.Status != "queued" {
		t.Fatalf("unexpected created ota task: %+v", created)
	}

	listReq := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/gateways/gw_demo_001/ota/tasks?tenant_id=tenant_demo_jakarta",
		nil,
	)
	listReq = withGatewayMQTTURLParam(listReq, "gatewayID", "gw_demo_001")
	listReq = withGatewayMQTTUser(listReq, auth.User{
		ID:       "u1",
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
	})
	listRec := httptest.NewRecorder()
	s.listGatewayOTATasks(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, listRec.Code, listRec.Body.String())
	}
	var listPayload struct {
		Items []gateway.GatewayOTATask `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if len(listPayload.Items) == 0 || listPayload.Items[0].ID != created.ID {
		t.Fatalf("expected created task in list, got %+v", listPayload.Items)
	}

	updateReqBody := map[string]any{
		"tenant_id":     "tenant_demo_jakarta",
		"status":        "failed",
		"error_message": "device offline during maintenance window",
	}
	updateReqBytes, err := json.Marshal(updateReqBody)
	if err != nil {
		t.Fatalf("marshal update request failed: %v", err)
	}
	updateReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/gateways/gw_demo_001/ota/tasks/"+created.ID+"/status",
		bytes.NewReader(updateReqBytes),
	)
	updateReq = withGatewayOTAURLParams(updateReq, "gw_demo_001", created.ID)
	updateReq = withGatewayMQTTUser(updateReq, auth.User{
		ID:       "u2",
		Email:    "ops-admin@example.com",
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
	})
	updateRec := httptest.NewRecorder()
	s.updateGatewayOTATaskStatus(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, updateRec.Code, updateRec.Body.String())
	}

	var updated gateway.GatewayOTATask
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update response failed: %v", err)
	}
	if updated.Status != "failed" {
		t.Fatalf("expected failed status, got %+v", updated)
	}
	if updated.ErrorMessage == "" {
		t.Fatalf("expected error_message for failed status, got %+v", updated)
	}
}

func TestCreateGatewayOTATaskInvalidSHA256(t *testing.T) {
	s := &server{gatewaySvc: gateway.NewService()}

	requestBody := map[string]any{
		"tenant_id":        "tenant_demo_jakarta",
		"firmware_version": "v2.4.2",
		"firmware_url":     "https://cdn.example.com/firmware/gw_demo_001/v2.4.2.bin",
		"firmware_sha256":  "not-a-sha256",
	}
	requestBytes, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateways/gw_demo_001/ota/tasks", bytes.NewReader(requestBytes))
	req = withGatewayMQTTURLParam(req, "gatewayID", "gw_demo_001")
	req = withGatewayMQTTUser(req, auth.User{
		ID:       "u1",
		Role:     "tenant_admin",
		TenantID: "tenant_demo_jakarta",
	})

	rec := httptest.NewRecorder()
	s.createGatewayOTATask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestGatewayBootstrapOTAReport(t *testing.T) {
	gatewaySvc := gateway.NewService()
	s := &server{
		gatewaySvc:          gatewaySvc,
		auditSvc:            audit.NewService(),
		gatewayDeviceTokens: map[string]string{"gw_demo_001": "gw_test_token_001"},
	}

	// Create an OTA task via admin API
	createReqBody, _ := json.Marshal(map[string]any{
		"tenant_id":          "tenant_demo_jakarta",
		"firmware_version":   "v3.0.0",
		"firmware_url":       "https://cdn.example.com/firmware/v3.0.0.bin",
		"firmware_sha256":    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"firmware_signature": strings.Repeat("b", 128),
	})
	createReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(createReqBody))
	createReq = withGatewayMQTTURLParam(createReq, "gatewayID", "gw_demo_001")
	createReq = withGatewayMQTTUser(createReq, auth.User{ID: "u1", Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})
	createRec := httptest.NewRecorder()
	s.createGatewayOTATask(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create OTA task failed: %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created gateway.GatewayOTATask
	_ = json.Unmarshal(createRec.Body.Bytes(), &created)

	// Gateway reports OTA success via device token
	reportBody, _ := json.Marshal(map[string]any{
		"gateway_id": "gw_demo_001",
		"tenant_id":  "tenant_demo_jakarta",
		"task_id":    created.ID,
		"status":     "succeeded",
	})
	reportReq := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/ota/report", bytes.NewReader(reportBody))
	reportReq.Header.Set("Content-Type", "application/json")
	reportReq.Header.Set("Authorization", "Bearer gw_test_token_001")
	reportRec := httptest.NewRecorder()
	s.gatewayBootstrapOTAReport(reportRec, reportReq)
	if reportRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", reportRec.Code, reportRec.Body.String())
	}

	var reportPayload struct {
		TaskID    string `json:"task_id"`
		GatewayID string `json:"gateway_id"`
		Status    string `json:"status"`
	}
	_ = json.Unmarshal(reportRec.Body.Bytes(), &reportPayload)
	if reportPayload.Status != "succeeded" {
		t.Fatalf("expected succeeded, got %s", reportPayload.Status)
	}

	// Report with wrong token → rejected
	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/ota/report",
		bytes.NewReader(reportBody))
	badReq.Header.Set("Content-Type", "application/json")
	badReq.Header.Set("Authorization", "Bearer gw_wrong_token")
	badRec := httptest.NewRecorder()
	s.gatewayBootstrapOTAReport(badRec, badReq)
	if badRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad token, got %d", badRec.Code)
	}
}

func withGatewayOTAURLParams(request *http.Request, gatewayID, taskID string) *http.Request {
	routeCtx := chi.NewRouteContext()
	if gatewayID != "" {
		routeCtx.URLParams.Add("gatewayID", gatewayID)
	}
	if taskID != "" {
		routeCtx.URLParams.Add("taskID", taskID)
	}
	ctx := context.WithValue(request.Context(), chi.RouteCtxKey, routeCtx)
	return request.WithContext(ctx)
}

func TestGatewayFirmwareSummaryEndpoint(t *testing.T) {
	svc := gateway.NewService()
	if err := svc.RecordFirmwareVersion("tenant_demo_jakarta", "gw_demo_001", "1.4.0"); err != nil {
		t.Fatal(err)
	}
	s := &server{gatewaySvc: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/gateways/firmware-summary?tenant_id=tenant_demo_jakarta", nil)
	req = withGatewayMQTTUser(req, auth.User{ID: "u1", Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})
	rec := httptest.NewRecorder()
	s.gatewayFirmwareSummary(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var sum gateway.GatewayFirmwareSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &sum); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if sum.Reported < 1 {
		t.Fatalf("expected reported>=1, got %+v", sum)
	}
}
