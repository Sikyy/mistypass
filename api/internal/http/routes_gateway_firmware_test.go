package httpx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

func firmwareUploadRequest(t *testing.T, version, channel, sha, sig string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("version", version)
	_ = mw.WriteField("channel", channel)
	_ = mw.WriteField("sha256", sha)
	_ = mw.WriteField("signature", sig)
	fw, _ := mw.CreateFormFile("file", "gateway-agent")
	_, _ = fw.Write(data)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateways/firmware?tenant_id=tenant_demo_jakarta", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return withGatewayMQTTUser(req, auth.User{ID: "u1", Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})
}

func TestUploadGatewayFirmware(t *testing.T) {
	dir := t.TempDir()
	s := &server{
		gatewaySvc: gateway.NewService(),
		cfg:        config.Config{UploadStorageDir: dir, UploadSigningKey: "k"},
	}
	data := []byte("fake firmware bytes")
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	sig := strings.Repeat("b", 128)

	rec := httptest.NewRecorder()
	s.uploadGatewayFirmware(rec, firmwareUploadRequest(t, "1.4.0", "stable", sha, sig, data))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var fw gateway.GatewayFirmware
	_ = json.Unmarshal(rec.Body.Bytes(), &fw)
	if fw.ID == "" || fw.Version != "1.4.0" || fw.SHA256 != sha {
		t.Fatalf("unexpected fw: %+v", fw)
	}
	blobPath := filepath.Join(dir, fw.ID[:2], fw.ID)
	if _, err := os.Stat(blobPath); err != nil {
		t.Fatalf("blob not written to disk: %v", err)
	}

	// sha mismatch → 400
	badRec := httptest.NewRecorder()
	s.uploadGatewayFirmware(badRec, firmwareUploadRequest(t, "1.4.1", "", strings.Repeat("a", 64), sig, data))
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("sha mismatch expected 400, got %d", badRec.Code)
	}
}

func TestListGatewayFirmware(t *testing.T) {
	svc := gateway.NewService()
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sig := strings.Repeat("b", 128)
	_, _ = svc.CreateFirmware(gateway.CreateFirmwareInput{TenantID: "tenant_demo_jakarta", Version: "1.4.0", Channel: "stable", SHA256: sha, Signature: sig})
	s := &server{gatewaySvc: svc, cfg: config.Config{UploadStorageDir: t.TempDir(), UploadSigningKey: "k"}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/gateways/firmware?tenant_id=tenant_demo_jakarta", nil)
	req = withGatewayMQTTUser(req, auth.User{ID: "u1", Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})
	rec := httptest.NewRecorder()
	s.listGatewayFirmware(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Items []gateway.GatewayFirmware `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if len(payload.Items) != 1 || payload.Items[0].Version != "1.4.0" {
		t.Fatalf("unexpected items: %+v", payload.Items)
	}
}

func withUploadIDParam(r *http.Request, id string) *http.Request {
	rc := chi.NewRouteContext()
	rc.URLParams.Add("uploadID", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
}

func TestFirmwareSignedURLServesViaDownloadFile(t *testing.T) {
	dir := t.TempDir()
	s := &server{
		gatewaySvc:          gateway.NewService(),
		gatewayDeviceTokens: map[string]string{"gw_demo_001": "gw_test_token_001"},
		cfg:                 config.Config{UploadStorageDir: dir, UploadSigningKey: "k"},
	}
	data := []byte("real firmware payload")
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	upRec := httptest.NewRecorder()
	s.uploadGatewayFirmware(upRec, firmwareUploadRequest(t, "1.4.0", "", sha, strings.Repeat("b", 128), data))
	if upRec.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", upRec.Code, upRec.Body.String())
	}
	var fw gateway.GatewayFirmware
	_ = json.Unmarshal(upRec.Body.Bytes(), &fw)
	if _, err := s.gatewaySvc.CreateOTATask("tenant_demo_jakarta", "gw_demo_001", "", "", "", "", fw.ID, "a"); err != nil {
		t.Fatal(err)
	}
	pullBody, _ := json.Marshal(map[string]any{"gateway_id": "gw_demo_001", "tenant_id": "tenant_demo_jakarta"})
	pullReq := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/config/pull", bytes.NewReader(pullBody))
	pullReq.Header.Set("Authorization", "Bearer gw_test_token_001")
	pullRec := httptest.NewRecorder()
	s.gatewayBootstrapConfigPull(pullRec, pullReq)
	var resp struct {
		PendingOTATasks []struct {
			FirmwareURL string `json:"firmware_url"`
		} `json:"pending_ota_tasks"`
	}
	_ = json.Unmarshal(pullRec.Body.Bytes(), &resp)
	if len(resp.PendingOTATasks) == 0 {
		t.Fatal("no pending tasks")
	}
	u := resp.PendingOTATasks[0].FirmwareURL
	q := u[strings.Index(u, "?"):]
	dlReq := httptest.NewRequest(http.MethodGet, "/api/v1/uploads/"+fw.ID+q, nil)
	dlReq = withUploadIDParam(dlReq, fw.ID)
	dlRec := httptest.NewRecorder()
	s.downloadFile(dlRec, dlReq)
	if dlRec.Code != http.StatusOK {
		t.Fatalf("download expected 200, got %d body=%s", dlRec.Code, dlRec.Body.String())
	}
	if !bytes.Equal(dlRec.Body.Bytes(), data) {
		t.Fatal("served bytes mismatch")
	}
}
