package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/hris"
)

func TestUpsertAndListEnterpriseHRISSecrets(t *testing.T) {
	s := &server{
		hrisVaultSvc: hris.NewVaultService("vault-master-key-001"),
		auditSvc:     audit.NewService(),
	}

	upsertBody, _ := json.Marshal(map[string]any{
		"tenant_id":  "tenant_demo_jakarta",
		"name":       "hris/talenta/webhook_secret",
		"kind":       "webhook_secret",
		"value":      "talenta-webhook-secret-001",
		"updated_by": "tenant.admin@sudirman.co",
	})
	upsertRequest := httptest.NewRequest(http.MethodPut, "/api/v1/enterprise/hris-secrets", bytes.NewReader(upsertBody))
	upsertRequest.Header.Set("Content-Type", "application/json")
	upsertRequest = withAuthUser(upsertRequest, auth.User{Role: "super_admin"})
	upsertRecorder := httptest.NewRecorder()

	s.upsertEnterpriseHRISSecret(upsertRecorder, upsertRequest)

	if upsertRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", upsertRecorder.Code, upsertRecorder.Body.String())
	}

	var upsertPayload struct {
		Item hris.SecretMetadata `json:"item"`
	}
	if err := json.Unmarshal(upsertRecorder.Body.Bytes(), &upsertPayload); err != nil {
		t.Fatalf("expected valid upsert payload: %v body=%s", err, upsertRecorder.Body.String())
	}
	if upsertPayload.Item.Ref != "vault://tenant_demo_jakarta/hris/talenta/webhook_secret" {
		t.Fatalf("unexpected secret ref: %s", upsertPayload.Item.Ref)
	}

	listRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-secrets?tenant_id=tenant_demo_jakarta",
		nil,
	)
	listRequest = withAuthUser(listRequest, auth.User{Role: "super_admin"})
	listRecorder := httptest.NewRecorder()

	s.listEnterpriseHRISSecrets(listRecorder, listRequest)

	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from list, got %d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var listPayload struct {
		Items []hris.SecretMetadata `json:"items"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("expected valid list payload: %v body=%s", err, listRecorder.Body.String())
	}
	if len(listPayload.Items) != 1 || listPayload.Items[0].Ref != upsertPayload.Item.Ref {
		t.Fatalf("unexpected listed secrets: %+v", listPayload.Items)
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_secret_upserted", "enterprise_sync", 10)
	if len(logs) == 0 {
		t.Fatalf("expected hris secret upsert audit log")
	}
}

func TestListEnterpriseHRISSecretsSupportsRefLookupWithoutTenantIDForSuperAdmin(t *testing.T) {
	s := &server{
		hrisVaultSvc: hris.NewVaultService("vault-master-key-001"),
		auditSvc:     audit.NewService(),
	}

	metadata, err := s.hrisVaultSvc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/credential",
		"connector_credential",
		`{"client_id":"talenta-client-001","client_secret":"talenta-secret-001"}`,
		"tenant.admin@sudirman.co",
	)
	if err != nil {
		t.Fatalf("expected secret upsert success: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-secrets?ref="+url.QueryEscape(metadata.Ref),
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISSecrets(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from ref lookup without tenant_id, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("talenta-secret-001")) {
		t.Fatalf("expected secret plaintext to stay hidden from metadata route, body=%s", recorder.Body.String())
	}

	var payload struct {
		Item hris.SecretMetadata `json:"item"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid metadata payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.Item.Ref != metadata.Ref || payload.Item.TenantID != "tenant_demo_jakarta" {
		t.Fatalf("unexpected metadata payload: %+v", payload.Item)
	}
}

func TestListEnterpriseHRISSecretsRefLookupRespectsTenantScope(t *testing.T) {
	s := &server{
		hrisVaultSvc: hris.NewVaultService("vault-master-key-001"),
		auditSvc:     audit.NewService(),
	}

	metadata, err := s.hrisVaultSvc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/webhook_secret",
		"webhook_secret",
		"talenta-webhook-secret-001",
		"tenant.admin@sudirman.co",
	)
	if err != nil {
		t.Fatalf("expected secret upsert success: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-secrets?ref="+url.QueryEscape(metadata.Ref),
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "tenant_admin", TenantID: "tenant_other"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISSecrets(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected tenant-scoped ref lookup to be hidden as 404, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
