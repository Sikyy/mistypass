package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

func TestGatewayCertificateRevocationHandlers(t *testing.T) {
	gatewaySvc := gateway.NewService()
	s := &server{
		gatewaySvc: gatewaySvc,
		auditSvc:   audit.NewService(),
	}

	body := []byte(`{"tenant_id":"tenant_demo_jakarta","gateway_id":"gw_demo_001","serial_number":"00:AB:C1:23","reason":"incident"}`)
	revokeReq := httptest.NewRequest(http.MethodPost, "/api/v1/gateways/cert-revocations", bytes.NewReader(body))
	revokeReq = withAuthUser(revokeReq, auth.User{ID: "usr_1", Email: "ops@example.com", Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})
	revokeRec := httptest.NewRecorder()
	s.revokeGatewayCertificateSerial(revokeRec, revokeReq)
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("expected revoke status 200, got %d body=%s", revokeRec.Code, revokeRec.Body.String())
	}
	if !gatewaySvc.IsCertificateSerialRevoked("ab:c1:23") {
		t.Fatalf("expected runtime certificate serial revocation")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/gateways/cert-revocations?tenant_id=tenant_demo_jakarta", nil)
	listReq = withAuthUser(listReq, auth.User{Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})
	listRec := httptest.NewRecorder()
	s.listGatewayCertificateRevocations(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	var listResponse struct {
		Items []gateway.GatewayCertificateRevocation `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResponse.Items) != 1 || listResponse.Items[0].SerialNumber != "abc123" {
		t.Fatalf("unexpected list response: %+v", listResponse.Items)
	}

	restoreReq := httptest.NewRequest(http.MethodDelete, "/api/v1/gateways/cert-revocations/abc123", bytes.NewReader([]byte(`{"tenant_id":"tenant_demo_jakarta"}`)))
	restoreReq = withAuthUser(restoreReq, auth.User{Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})
	restoreReq = withGatewayCertificateRevocationURLParam(restoreReq, "abc123")
	restoreRec := httptest.NewRecorder()
	s.restoreGatewayCertificateSerial(restoreRec, restoreReq)
	if restoreRec.Code != http.StatusOK {
		t.Fatalf("expected restore status 200, got %d body=%s", restoreRec.Code, restoreRec.Body.String())
	}
	if gatewaySvc.IsCertificateSerialRevoked("abc123") {
		t.Fatalf("expected certificate serial restore")
	}
}

func TestGatewayCertificateRevocationRestoreRejectsEnvironmentSerial(t *testing.T) {
	s := &server{
		cfg:        configWithRevokedGatewaySerial("AB:C1:23"),
		gatewaySvc: gateway.NewService(),
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/gateways/cert-revocations/abc123", bytes.NewReader([]byte(`{"tenant_id":"tenant_demo_jakarta"}`)))
	req = withAuthUser(req, auth.User{Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})
	req = withGatewayCertificateRevocationURLParam(req, "abc123")
	rec := httptest.NewRecorder()
	s.restoreGatewayCertificateSerial(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected conflict for environment serial, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func configWithRevokedGatewaySerial(serial string) config.Config {
	return config.Config{GatewayMTLSRevokedSerials: []string{serial}}
}

func withGatewayCertificateRevocationURLParam(request *http.Request, serialNumber string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("serialNumber", serialNumber)
	ctx := context.WithValue(request.Context(), chi.RouteCtxKey, routeCtx)
	return request.WithContext(ctx)
}
