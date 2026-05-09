package httpx

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/camera"
)

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/places/{placeId}/doors/{doorId}/lockdown — Per-door lockdown
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appPlaceEnableDoorLockdown(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	doorID := chi.URLParam(r, "doorId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}
	door, err := s.spaceSvc.GetDoor(tenantID, doorID)
	if err != nil || door.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "door not found in this place")
		return
	}

	lockdownKey := "door_lockdown:" + tenantID + ":" + doorID
	lockdownData := map[string]any{
		"door_id":    doorID,
		"place_id":   placeID,
		"enabled":    true,
		"enabled_by": user.Email,
		"enabled_at": time.Now().UTC().Format(time.RFC3339),
	}
	if s.stateStore != nil {
		_ = s.stateStore.Save(lockdownKey, lockdownData)
	} else {
		s.memStoreMu.Lock()
		s.memStore[lockdownKey] = lockdownData
		s.memStoreMu.Unlock()
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "lockdown_enabled"})
}

// DELETE /api/v1/app/places/{placeId}/doors/{doorId}/lockdown

func (s *server) appPlaceDisableDoorLockdown(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	doorID := chi.URLParam(r, "doorId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	lockdownKey := "door_lockdown:" + tenantID + ":" + doorID
	lockdownOffData := map[string]any{
		"door_id":  doorID,
		"place_id": placeID,
		"enabled":  false,
	}
	if s.stateStore != nil {
		_ = s.stateStore.Save(lockdownKey, lockdownOffData)
	} else {
		s.memStoreMu.Lock()
		s.memStore[lockdownKey] = lockdownOffData
		s.memStoreMu.Unlock()
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "lockdown_disabled"})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/doors/{doorId}/restrictions
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appPlaceDoorRestrictions(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	doorID := chi.URLParam(r, "doorId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}
	door, err := s.spaceSvc.GetDoor(tenantID, doorID)
	if err != nil || door.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "door not found in this place")
		return
	}

	restrictionKey := "door_restrictions:" + tenantID + ":" + doorID
	var restrictions []map[string]any
	found := false
	if s.stateStore != nil {
		found, _ = s.stateStore.Load(restrictionKey, &restrictions)
	}
	if !found {
		restrictions = []map[string]any{}
	}

	writeJSON(w, http.StatusOK, restrictions)
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/doors/{doorId}/schedules
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appPlaceDoorSchedules(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	doorID := chi.URLParam(r, "doorId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	schedulesKey := "schedules:" + tenantID + ":" + placeID
	var allSchedules []map[string]any
	schedFound := false
	if s.stateStore != nil {
		schedFound, _ = s.stateStore.Load(schedulesKey, &allSchedules)
	}
	if !schedFound {
		allSchedules = []map[string]any{}
	}

	doorSchedules := make([]map[string]any, 0)
	for _, sched := range allSchedules {
		if did, ok := sched["door_id"].(string); ok && did == doorID {
			doorSchedules = append(doorSchedules, sched)
		}
	}

	writeJSON(w, http.StatusOK, doorSchedules)
}

// ─────────────────────────────────────────────────────────────────────────────
// PATCH /api/v1/app/places/{placeId}/doors/{doorId} — Rename door
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appPlaceRenameDoor(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	doorID := chi.URLParam(r, "doorId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	door, err := s.spaceSvc.GetDoor(tenantID, doorID)
	if err != nil || door.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "door not found in this place")
		return
	}

	renameKey := "door_rename:" + tenantID + ":" + doorID
	renameData := map[string]any{
		"door_id":    doorID,
		"name":       name,
		"renamed_by": user.Email,
		"renamed_at": time.Now().UTC().Format(time.RFC3339),
	}
	if s.stateStore != nil {
		_ = s.stateStore.Save(renameKey, renameData)
	} else {
		s.memStoreMu.Lock()
		s.memStore[renameKey] = renameData
		s.memStoreMu.Unlock()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":     doorID,
		"name":   name,
		"status": door.Status,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// PATCH /api/v1/app/gateways/{gatewayId} — Rename gateway
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appRenameGateway(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	gatewayID := chi.URLParam(r, "gatewayId")
	tenantID := user.TenantID

	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	renameKey := "gateway_rename:" + tenantID + ":" + gatewayID
	gwRenameData := map[string]any{
		"gateway_id": gatewayID,
		"name":       name,
	}
	if s.stateStore != nil {
		_ = s.stateStore.Save(renameKey, gwRenameData)
	} else {
		s.memStoreMu.Lock()
		s.memStore[renameKey] = gwRenameData
		s.memStoreMu.Unlock()
	}

	writeJSON(w, http.StatusOK, map[string]any{"name": name})
}

// ─────────────────────────────────────────────────────────────────────────────
// PATCH /api/v1/app/cameras/{cameraId} — Rename camera
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appRenameCamera(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	cameraID := chi.URLParam(r, "cameraId")
	tenantID := user.TenantID

	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if _, err := s.cameraSvc.Get(tenantID, cameraID); err != nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	updated, err := s.cameraSvc.Update(tenantID, cameraID, camera.CameraUpdateRequest{Name: &name})
	if err != nil {
		writeInternalError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, updated)
}
