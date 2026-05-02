package httpx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/camera"
)

func (s *server) listCameras(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	items := s.cameraSvc.List(tenantID)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) createCamera(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Camera camera.CameraCreateRequest `json:"camera"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.Camera.TenantID)
	if !ok {
		return
	}
	request.Camera.TenantID = tenantID
	cam, err := s.cameraSvc.Create(request.Camera)
	if err != nil {
		handleCameraError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "camera_created",
		fmt.Sprintf("camera_id=%s,name=%s,provider=%s,host=%s", cam.ID, cam.Name, cam.Provider, cam.Host), "camera")
	writeJSON(w, http.StatusCreated, cam)
}

func (s *server) getCamera(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	cam, err := s.cameraSvc.Get(tenantID, chi.URLParam(r, "cameraID"))
	if err != nil {
		handleCameraError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cam)
}

func (s *server) updateCamera(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Camera camera.CameraUpdateRequest `json:"camera"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	cameraID := chi.URLParam(r, "cameraID")
	cam, err := s.cameraSvc.Update(tenantID, cameraID, request.Camera)
	if err != nil {
		handleCameraError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "camera_updated",
		fmt.Sprintf("camera_id=%s,name=%s", cam.ID, cam.Name), "camera")
	writeJSON(w, http.StatusOK, cam)
}

func (s *server) deleteCamera(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	cameraID := chi.URLParam(r, "cameraID")
	if err := s.cameraSvc.Delete(tenantID, cameraID); err != nil {
		handleCameraError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "camera_deleted",
		fmt.Sprintf("camera_id=%s", cameraID), "camera")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) fetchVideoLink(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	url, err := s.cameraSvc.GetVideoLink(ctx, tenantID, chi.URLParam(r, "cameraID"))
	if err != nil {
		handleCameraError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": url})
}

func (s *server) testCameraConnection(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	cameraID := chi.URLParam(r, "cameraID")
	if err := s.cameraSvc.TestConnection(ctx, tenantID, cameraID); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"camera_id": cameraID,
			"status":    "error",
			"message":   err.Error(),
			"tested_at": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"camera_id": cameraID,
		"status":    "ok",
		"message":   "connection successful",
		"tested_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *server) captureSnapshot(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	cameraID := chi.URLParam(r, "cameraID")
	snap, err := s.cameraSvc.CaptureSnapshot(ctx, tenantID, cameraID, "", "manual")
	if err != nil {
		handleCameraError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "camera_snapshot_captured",
		fmt.Sprintf("camera_id=%s,snapshot_id=%s,event_type=manual", cameraID, snap.ID), "camera")
	writeJSON(w, http.StatusOK, snap)
}

func (s *server) listCameraSnapshots(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	items := s.cameraSvc.ListSnapshots(tenantID, chi.URLParam(r, "cameraID"))
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) discoverCameras(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Provider string `json:"provider"`
		Subnet   string `json:"subnet"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	if provider == "" {
		provider = "onvif"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	discovered, err := s.cameraSvc.DiscoverCameras(ctx, provider, strings.TrimSpace(request.Subnet))
	if err != nil {
		handleCameraError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": discovered})
}

func handleCameraError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, camera.ErrCameraNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, camera.ErrCameraNameRequired),
		errors.Is(err, camera.ErrCameraHostRequired),
		errors.Is(err, camera.ErrInvalidProvider):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, camera.ErrProviderNotFound):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
