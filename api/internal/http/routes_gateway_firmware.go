package httpx

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

const firmwareMaxUploadBytes = 256 << 20 // 256 MiB ceiling

// uploadGatewayFirmware stores a signed firmware artifact + registry record.
// The binary lands in the shared signed-blob store, served later via /uploads/{id}.
func (s *server) uploadGatewayFirmware(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	storageDir := strings.TrimSpace(s.cfg.UploadStorageDir)
	// signingKey isn't used to sign here, but it's required end-to-end: without it,
	// config/pull can't mint a signed download URL, so uploaded firmware would be unservable.
	signingKey := strings.TrimSpace(s.cfg.UploadSigningKey)
	if storageDir == "" || signingKey == "" {
		writeError(w, http.StatusServiceUnavailable, "file uploads are not configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, firmwareMaxUploadBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	declaredSHA := strings.ToLower(strings.TrimSpace(r.FormValue("sha256")))

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing firmware file")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read firmware")
		return
	}
	sum := sha256.Sum256(data)
	gotSHA := hex.EncodeToString(sum[:])
	if declaredSHA == "" || gotSHA != declaredSHA {
		writeError(w, http.StatusBadRequest, "sha256 does not match uploaded bytes")
		return
	}

	fw, err := s.gatewaySvc.CreateFirmware(gateway.CreateFirmwareInput{
		TenantID:   tenantID,
		Version:    strings.TrimSpace(r.FormValue("version")),
		Channel:    strings.TrimSpace(r.FormValue("channel")),
		SHA256:     gotSHA,
		Signature:  strings.ToLower(strings.TrimSpace(r.FormValue("signature"))),
		SizeBytes:  int64(len(data)),
		UploadedBy: requestActor(r),
	})
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayFirmwareVersionRequired),
			errors.Is(err, gateway.ErrGatewayFirmwareSHA256Invalid),
			errors.Is(err, gateway.ErrGatewayFirmwareSignatureInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	// Store the blob at storageDir/{id[:2]}/{id} — the same layout downloadFile serves.
	cleanID := filepath.Base(fw.ID)
	dir := filepath.Join(storageDir, cleanID[:2])
	if err := os.MkdirAll(dir, 0o750); err != nil {
		_ = s.gatewaySvc.DeleteFirmware(fw.ID)
		writeError(w, http.StatusInternalServerError, "storage error")
		return
	}
	if err := os.WriteFile(filepath.Join(dir, cleanID), data, 0o600); err != nil { // #nosec G304 -- cleanID is the minted firmware id (fw_<hex>), not user-controlled
		_ = s.gatewaySvc.DeleteFirmware(fw.ID)
		writeError(w, http.StatusInternalServerError, "storage error")
		return
	}

	writeJSON(w, http.StatusCreated, fw)
}

func (s *server) listGatewayFirmware(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	items := s.gatewaySvc.ListFirmware(tenantID, strings.TrimSpace(r.URL.Query().Get("channel")))
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
