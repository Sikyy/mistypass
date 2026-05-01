package httpx

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/auth"
)

const (
	uploadIDLength     = 16
	uploadSignatureLen = 64 // hex-encoded SHA256-HMAC
)

// requestSignedUploadURL generates a pre-signed URL for uploading a file.
func (s *server) requestSignedUploadURL(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	storageDir := strings.TrimSpace(s.cfg.UploadStorageDir)
	signingKey := strings.TrimSpace(s.cfg.UploadSigningKey)
	if storageDir == "" || signingKey == "" {
		writeError(w, http.StatusServiceUnavailable, "file uploads are not configured")
		return
	}

	var request struct {
		Purpose     string `json:"purpose"`      // "avatar", "credential_photo", "document"
		ContentType string `json:"content_type"`  // "image/png", "image/jpeg", "application/pdf"
		FileName    string `json:"file_name"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	purpose := strings.ToLower(strings.TrimSpace(request.Purpose))
	if purpose == "" {
		purpose = "general"
	}
	switch purpose {
	case "avatar", "credential_photo", "document", "general":
	default:
		writeError(w, http.StatusBadRequest, "invalid purpose")
		return
	}

	contentType := strings.ToLower(strings.TrimSpace(request.ContentType))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	uploadID, err := generateUploadID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate upload ID")
		return
	}

	ttl := s.cfg.UploadURLTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	expiresAt := time.Now().UTC().Add(ttl)
	signature := signUpload(signingKey, uploadID, user.ID, expiresAt)

	uploadURL := fmt.Sprintf("/api/v1/uploads/%s?sig=%s&uid=%s&expires=%d",
		uploadID, signature, user.ID, expiresAt.Unix())

	// Generate a separate download signature (longer TTL)
	downloadExpiry := time.Now().UTC().Add(24 * time.Hour)
	downloadSig := signDownload(signingKey, uploadID, downloadExpiry)
	downloadURL := fmt.Sprintf("/api/v1/uploads/%s?sig=%s&expires=%d",
		uploadID, downloadSig, downloadExpiry.Unix())

	s.appendAuditLog(r, user.TenantID, "upload_url_created",
		fmt.Sprintf("upload_id=%s,purpose=%s,user_id=%s", uploadID, purpose, user.ID), "upload")

	writeJSON(w, http.StatusOK, map[string]any{
		"upload_id":    uploadID,
		"upload_url":   uploadURL,
		"download_url": downloadURL,
		"method":       "PUT",
		"expires_at":   expiresAt,
		"max_size":     s.cfg.UploadMaxSizeBytes,
		"content_type": contentType,
	})
}

// uploadFile handles the actual file upload via the signed URL.
func (s *server) uploadFile(w http.ResponseWriter, r *http.Request) {
	storageDir := strings.TrimSpace(s.cfg.UploadStorageDir)
	signingKey := strings.TrimSpace(s.cfg.UploadSigningKey)
	if storageDir == "" || signingKey == "" {
		writeError(w, http.StatusServiceUnavailable, "file uploads are not configured")
		return
	}

	uploadID := chi.URLParam(r, "uploadID")
	sig := r.URL.Query().Get("sig")
	uid := r.URL.Query().Get("uid")
	expiresStr := r.URL.Query().Get("expires")

	if uploadID == "" || sig == "" || expiresStr == "" || uid == "" {
		writeError(w, http.StatusBadRequest, "missing upload parameters")
		return
	}

	expiresUnix, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid expires parameter")
		return
	}
	expiresAt := time.Unix(expiresUnix, 0).UTC()
	if time.Now().UTC().After(expiresAt) {
		writeError(w, http.StatusGone, "upload URL has expired")
		return
	}

	// Verify user-bound signature
	expected := signUpload(signingKey, uploadID, uid, expiresAt)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		writeError(w, http.StatusForbidden, "invalid upload signature")
		return
	}

	// Limit body size
	maxSize := s.cfg.UploadMaxSizeBytes
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	// Sanitize upload ID to prevent directory traversal
	cleanID := filepath.Base(uploadID)

	// Ensure storage directory exists
	uploadDir := filepath.Join(storageDir, cleanID[:2])
	if err := os.MkdirAll(uploadDir, 0750); err != nil {
		writeError(w, http.StatusInternalServerError, "storage error")
		return
	}

	filePath := filepath.Join(uploadDir, cleanID)
	f, err := os.Create(filePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage error")
		return
	}
	defer f.Close()

	written, err := io.Copy(f, r.Body)
	if err != nil {
		_ = os.Remove(filePath)
		writeError(w, http.StatusBadRequest, "upload failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"upload_id": uploadID,
		"size":      written,
		"status":    "uploaded",
	})
}

// downloadFile serves an uploaded file. Requires a valid download signature.
func (s *server) downloadFile(w http.ResponseWriter, r *http.Request) {
	storageDir := strings.TrimSpace(s.cfg.UploadStorageDir)
	signingKey := strings.TrimSpace(s.cfg.UploadSigningKey)
	if storageDir == "" || signingKey == "" {
		writeError(w, http.StatusServiceUnavailable, "file uploads are not configured")
		return
	}

	uploadID := chi.URLParam(r, "uploadID")
	if uploadID == "" || len(uploadID) < 3 {
		writeError(w, http.StatusBadRequest, "invalid upload ID")
		return
	}

	sig := r.URL.Query().Get("sig")
	expiresStr := r.URL.Query().Get("expires")
	if sig == "" || expiresStr == "" {
		writeError(w, http.StatusForbidden, "missing download signature")
		return
	}
	expiresUnix, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid expires parameter")
		return
	}
	expiresAt := time.Unix(expiresUnix, 0).UTC()
	if time.Now().UTC().After(expiresAt) {
		writeError(w, http.StatusGone, "download URL has expired")
		return
	}
	expected := signDownload(signingKey, uploadID, expiresAt)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		writeError(w, http.StatusForbidden, "invalid download signature")
		return
	}

	// Sanitize path to prevent directory traversal
	cleanID := filepath.Base(uploadID)
	filePath := filepath.Join(storageDir, cleanID[:2], cleanID)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	http.ServeFile(w, r, filePath)
}

// listUserUploads returns uploads for the authenticated user (placeholder — could be backed by DB later).
func (s *server) listUserUploads(w http.ResponseWriter, r *http.Request) {
	_, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	// Currently returns empty — would be backed by a file metadata table.
	writeJSON(w, http.StatusOK, map[string]any{
		"items": []any{},
	})
}

func generateUploadID() (string, error) {
	buf := make([]byte, uploadIDLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "upl_" + hex.EncodeToString(buf), nil
}

func signUpload(key, uploadID, userID string, expiresAt time.Time) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(uploadID + ":" + userID + ":" + strconv.FormatInt(expiresAt.Unix(), 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

func signDownload(key, uploadID string, expiresAt time.Time) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte("dl:" + uploadID + ":" + strconv.FormatInt(expiresAt.Unix(), 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// Ensure authenticatedUser import is used
var _ = auth.User{}
