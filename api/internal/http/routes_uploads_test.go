package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestSignedUploadURLFlow(t *testing.T) {
	tmpDir := t.TempDir()
	router, err := NewRouter(config.Config{
		JWTSecret:        "upload-test",
		EnableDemoUsers:  true,
		UploadStorageDir: tmpDir,
		UploadSigningKey: "test-signing-key-abc123",
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// Request signed URL
	reqBody := []byte(`{"purpose":"avatar","content_type":"image/png","file_name":"photo.png"}`)
	reqRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/uploads/signed-url", token, reqBody)
	if reqRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", reqRec.Code, reqRec.Body.String())
	}

	var signedResp struct {
		UploadID    string `json:"upload_id"`
		UploadURL   string `json:"upload_url"`
		DownloadURL string `json:"download_url"`
		Method      string `json:"method"`
		MaxSize     int64  `json:"max_size"`
	}
	_ = json.Unmarshal(reqRec.Body.Bytes(), &signedResp)
	if signedResp.UploadID == "" {
		t.Fatalf("expected upload_id")
	}
	if !strings.HasPrefix(signedResp.UploadURL, "/api/v1/uploads/") {
		t.Fatalf("expected upload_url path, got %s", signedResp.UploadURL)
	}
	if signedResp.Method != "PUT" {
		t.Fatalf("expected PUT method, got %s", signedResp.Method)
	}

	// Upload file using signed URL
	fileContent := []byte("fake-png-content-for-testing")
	uploadRec := referenceAPIRequest(t, router, http.MethodPut, "/api/v1"+signedResp.UploadURL[len("/api/v1"):], "", fileContent)
	if uploadRec.Code != http.StatusOK {
		t.Fatalf("expected upload 200, got %d body=%s", uploadRec.Code, uploadRec.Body.String())
	}

	var uploadResp struct {
		UploadID string `json:"upload_id"`
		Size     int64  `json:"size"`
		Status   string `json:"status"`
	}
	_ = json.Unmarshal(uploadRec.Body.Bytes(), &uploadResp)
	if uploadResp.Size != int64(len(fileContent)) {
		t.Fatalf("expected size %d, got %d", len(fileContent), uploadResp.Size)
	}

	// Download file using signed download URL
	if signedResp.DownloadURL == "" {
		t.Fatalf("expected download_url in response")
	}
	downloadRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1"+signedResp.DownloadURL[len("/api/v1"):], "", nil)
	if downloadRec.Code != http.StatusOK {
		t.Fatalf("expected download 200, got %d body=%s", downloadRec.Code, downloadRec.Body.String())
	}
	if !bytes.Equal(downloadRec.Body.Bytes(), fileContent) {
		t.Fatalf("downloaded content mismatch")
	}
}

func TestSignedUploadURLNotConfigured(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "upload-noconfig-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	reqBody := []byte(`{"purpose":"avatar"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/uploads/signed-url", token, reqBody)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when not configured, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUploadExpiredSignature(t *testing.T) {
	tmpDir := t.TempDir()
	router, err := NewRouter(config.Config{
		JWTSecret:        "upload-expired-test",
		EnableDemoUsers:  true,
		UploadStorageDir: tmpDir,
		UploadSigningKey: "test-key",
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	// Try uploading with an expired timestamp
	rec := referenceAPIRequest(t, router, http.MethodPut, "/api/v1/uploads/upl_fake123?sig=badsig&uid=test&expires=1000000000", "", []byte("data"))
	if rec.Code != http.StatusGone {
		t.Fatalf("expected 410 for expired URL, got %d body=%s", rec.Code, rec.Body.String())
	}
}
