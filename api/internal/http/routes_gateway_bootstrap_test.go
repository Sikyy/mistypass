package httpx

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

func TestGatewayBootstrapRegisterRequiresConfiguredBootstrapToken(t *testing.T) {
	s := &server{
		gatewaySvc: gateway.NewService(),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/register", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()

	s.gatewayBootstrapRegister(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	}
}

func TestGatewayBootstrapRegisterRejectsInvalidBootstrapToken(t *testing.T) {
	s := &server{
		cfg:        config.Config{GatewayBootstrapToken: "bootstrap-token-001"},
		gatewaySvc: gateway.NewService(),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/register", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-Bootstrap-Token", "wrong-token")
	rec := httptest.NewRecorder()

	s.gatewayBootstrapRegister(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
}

func TestGatewayBootstrapRegisterReturnsDeviceTokenWhenAuthorized(t *testing.T) {
	gatewaySvc := gateway.NewService()
	if _, err := gatewaySvc.ImportSerialInventory("tenant_demo_jakarta", []gateway.SerialInventoryImportItem{
		{
			SerialNumber: "MP-GW-EDGE-001",
			ProductType:  "gateway",
		},
	}); err != nil {
		t.Fatalf("import serial inventory failed: %v", err)
	}

	s := &server{
		cfg:        config.Config{GatewayBootstrapToken: "bootstrap-token-001"},
		gatewaySvc: gatewaySvc,
	}

	requestBody, err := json.Marshal(map[string]any{
		"serial_number":   "MP-GW-EDGE-001",
		"tenant_id":       "tenant_demo_jakarta",
		"building_id":     "building_demo_001",
		"device_capacity": 4,
	})
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/register", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bootstrap-Token", "bootstrap-token-001")
	rec := httptest.NewRecorder()

	s.gatewayBootstrapRegister(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var payload struct {
		GatewayID   string `json:"gateway_id"`
		TenantID    string `json:"tenant_id"`
		DeviceToken string `json:"device_token"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if payload.GatewayID == "" {
		t.Fatalf("expected gateway_id in response")
	}
	if payload.TenantID != "tenant_demo_jakarta" {
		t.Fatalf("unexpected tenant_id: %s", payload.TenantID)
	}
	if payload.Status != "registered" {
		t.Fatalf("unexpected status: %s", payload.Status)
	}
	if payload.DeviceToken == "" {
		t.Fatalf("expected device_token in response")
	}
}

func TestGatewayBootstrapRegisterRejectsMismatchedCSRSubject(t *testing.T) {
	gatewaySvc := gateway.NewService()
	if _, err := gatewaySvc.ImportSerialInventory("tenant_demo_jakarta", []gateway.SerialInventoryImportItem{
		{
			SerialNumber: "MP-GW-EDGE-CSR",
			ProductType:  "gateway",
		},
	}); err != nil {
		t.Fatalf("import serial inventory failed: %v", err)
	}
	deviceCA, err := gateway.NewDeviceCA()
	if err != nil {
		t.Fatalf("NewDeviceCA failed: %v", err)
	}
	csrPEM := testGatewayCSR(t, "gw_attacker", "tenant_demo_jakarta")

	s := &server{
		cfg:             config.Config{GatewayBootstrapToken: "bootstrap-token-001"},
		gatewaySvc:      gatewaySvc,
		gatewayDeviceCA: deviceCA,
	}

	requestBody, err := json.Marshal(map[string]any{
		"serial_number":   "MP-GW-EDGE-CSR",
		"tenant_id":       "tenant_demo_jakarta",
		"building_id":     "building_demo_001",
		"device_capacity": 4,
		"csr_pem":         string(csrPEM),
	})
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/register", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bootstrap-Token", "bootstrap-token-001")
	rec := httptest.NewRecorder()

	s.gatewayBootstrapRegister(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func testGatewayCSR(t *testing.T, commonName, organization string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{organization},
		},
	}, key)
	if err != nil {
		t.Fatalf("create CSR failed: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
}

func TestGatewayConfigPullRecordsFirmwareVersion(t *testing.T) {
	svc := gateway.NewService()
	s := &server{
		gatewaySvc:          svc,
		gatewayDeviceTokens: map[string]string{"gw_demo_001": "gw_test_token_001"},
	}
	body, _ := json.Marshal(map[string]any{
		"gateway_id":       "gw_demo_001",
		"tenant_id":        "tenant_demo_jakarta",
		"firmware_version": "1.4.0",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/config/pull", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer gw_test_token_001")
	rec := httptest.NewRecorder()
	s.gatewayBootstrapConfigPull(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("config/pull expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	recorded := false
	for _, g := range svc.List("tenant_demo_jakarta") {
		if g.ID == "gw_demo_001" && g.CurrentFirmwareVersion == "1.4.0" {
			recorded = true
		}
	}
	if !recorded {
		t.Fatal("firmware version not recorded from config/pull")
	}
}
