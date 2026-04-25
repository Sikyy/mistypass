package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		"tenant_id":        "tenant_demo_jakarta",
		"firmware_version": "v2.4.1",
		"firmware_url":     "https://cdn.example.com/firmware/gw_demo_001/v2.4.1.bin",
		"firmware_sha256":  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
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
