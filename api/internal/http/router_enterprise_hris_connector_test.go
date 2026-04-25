package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/hris"
)

func TestCreateEnterpriseHRISConnector(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
		auditSvc:      audit.NewService(),
	}

	body := map[string]any{
		"tenant_id":          "tenant_demo_jakarta",
		"vendor":             "talenta",
		"status":             "active",
		"sync_strategy":      "hybrid",
		"credential_ref":     "vault://tenant_demo_jakarta/hris/talenta/client_id",
		"webhook_secret_ref": "vault://tenant_demo_jakarta/hris/talenta/webhook_secret",
		"updated_by":         "tenant.admin@sudirman.co",
	}
	requestBytes, _ := json.Marshal(body)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/hris-connectors", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.createEnterpriseHRISConnector(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.HRISConnector
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid connector json: %v body=%s", err, recorder.Body.String())
	}
	if payload.ID == "" {
		t.Fatalf("expected non-empty connector id")
	}
	if payload.Vendor != "talenta" {
		t.Fatalf("expected vendor talenta, got %s", payload.Vendor)
	}
	if payload.SyncStrategy != "hybrid" {
		t.Fatalf("expected sync_strategy hybrid, got %s", payload.SyncStrategy)
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_connector_created", "enterprise_sync", 10)
	if len(logs) == 0 {
		t.Fatalf("expected connector created audit log")
	}
}

func TestCreateEnterpriseHRISConnectorDuplicateVendorConflict(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
	}

	_, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"hybrid",
		"vault://tenant_demo_jakarta/hris/talenta/client_id",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("seed connector create should succeed: %v", err)
	}

	body := map[string]any{
		"tenant_id":     "tenant_demo_jakarta",
		"vendor":        "talenta",
		"status":        "active",
		"sync_strategy": "pull",
	}
	requestBytes, _ := json.Marshal(body)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/hris-connectors", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.createEnterpriseHRISConnector(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestListAndUpdateEnterpriseHRISConnector(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
		auditSvc:      audit.NewService(),
	}

	created, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"gadjian",
		"active",
		"hybrid",
		"vault://tenant_demo_jakarta/hris/gadjian/api_key",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("seed connector create should succeed: %v", err)
	}

	listRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-connectors?tenant_id=tenant_demo_jakarta",
		nil,
	)
	listRequest = withAuthUser(listRequest, auth.User{Role: "super_admin"})
	listRecorder := httptest.NewRecorder()

	s.listEnterpriseHRISConnectors(listRecorder, listRequest)

	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from list, got %d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var listPayload struct {
		Items []enterprise.HRISConnector `json:"items"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("expected valid connector list json: %v body=%s", err, listRecorder.Body.String())
	}
	if len(listPayload.Items) != 1 || listPayload.Items[0].ID != created.ID {
		t.Fatalf("unexpected connector list payload: %+v", listPayload.Items)
	}

	updateBody := map[string]any{
		"tenant_id":          "tenant_demo_jakarta",
		"status":             "inactive",
		"sync_strategy":      "pull",
		"credential_ref":     "vault://tenant_demo_jakarta/hris/gadjian/api_key_v2",
		"webhook_secret_ref": "vault://tenant_demo_jakarta/hris/gadjian/webhook_secret",
		"updated_by":         "ops@sudirman.co",
	}
	updateBytes, _ := json.Marshal(updateBody)
	updateRequest := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/enterprise/hris-connectors/"+created.ID,
		bytes.NewReader(updateBytes),
	)
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest = withAuthUser(updateRequest, auth.User{Role: "super_admin"})
	updateRequest = withURLParam(updateRequest, "connectorID", created.ID)
	updateRecorder := httptest.NewRecorder()

	s.updateEnterpriseHRISConnector(updateRecorder, updateRequest)

	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from update, got %d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updated enterprise.HRISConnector
	if err := json.Unmarshal(updateRecorder.Body.Bytes(), &updated); err != nil {
		t.Fatalf("expected valid updated connector json: %v body=%s", err, updateRecorder.Body.String())
	}
	if updated.Status != "inactive" {
		t.Fatalf("expected inactive status, got %s", updated.Status)
	}
	if updated.SyncStrategy != "pull" {
		t.Fatalf("expected pull sync_strategy, got %s", updated.SyncStrategy)
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_connector_updated", "enterprise_sync", 10)
	if len(logs) == 0 {
		t.Fatalf("expected connector updated audit log")
	}
}

func TestCreateEnterpriseHRISConnectorStoresInlineSecretsInVault(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
		auditSvc:      audit.NewService(),
		hrisVaultSvc:  hris.NewVaultService("vault-master-key-001"),
	}

	body := map[string]any{
		"tenant_id":            "tenant_demo_jakarta",
		"vendor":               "talenta",
		"status":               "active",
		"sync_strategy":        "webhook",
		"credential_value":     "talenta-client-id-001",
		"webhook_secret_value": "talenta-webhook-secret-001",
		"updated_by":           "tenant.admin@sudirman.co",
	}
	requestBytes, _ := json.Marshal(body)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/hris-connectors", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.createEnterpriseHRISConnector(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload enterprise.HRISConnector
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid connector json: %v body=%s", err, recorder.Body.String())
	}
	if payload.CredentialRef != "vault://tenant_demo_jakarta/hris/talenta/credential" {
		t.Fatalf("unexpected credential_ref: %s", payload.CredentialRef)
	}
	if payload.WebhookSecretRef != "vault://tenant_demo_jakarta/hris/talenta/webhook_secret" {
		t.Fatalf("unexpected webhook_secret_ref: %s", payload.WebhookSecretRef)
	}

	resolvedCredential, err := s.hrisVaultSvc.ResolveSecretRef(payload.CredentialRef)
	if err != nil {
		t.Fatalf("expected credential ref to resolve: %v", err)
	}
	if resolvedCredential.Value != "talenta-client-id-001" {
		t.Fatalf("unexpected credential secret value: %s", resolvedCredential.Value)
	}

	resolvedWebhookSecret, err := s.hrisVaultSvc.ResolveSecretRef(payload.WebhookSecretRef)
	if err != nil {
		t.Fatalf("expected webhook secret ref to resolve: %v", err)
	}
	if resolvedWebhookSecret.Value != "talenta-webhook-secret-001" {
		t.Fatalf("unexpected webhook secret value: %s", resolvedWebhookSecret.Value)
	}
}
