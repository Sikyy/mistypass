package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/space"
	"github.com/mistypass/cloud/api/internal/modules/tenant"
)

func (s *server) listTenants(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.tenantSvc.List(),
	})
}

func (s *server) createTenant(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		HQRegion string `json:"hq_region"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := s.tenantSvc.Create(request.Name, request.Type, request.HQRegion)
	if err != nil {
		switch {
		case errors.Is(err, tenant.ErrTenantNameRequired), errors.Is(err, tenant.ErrInvalidTenantType):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, created.ID, "legacy_tenant_created", fmt.Sprintf("tenant_id=%s,name=%s,type=%s,status=%s", created.ID, created.Name, created.Type, created.Status), "tenant")
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) getTenantTopology(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, chi.URLParam(r, "tenantID"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	topology, err := s.spaceSvc.TopologyByTenant(tenantID)
	if err != nil {
		switch {
		case errors.Is(err, space.ErrTenantIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if buildingScope != nil {
		topology = filterTopologyByBuildingScope(topology, buildingScope)
	}

	writeJSON(w, http.StatusOK, topology)
}

func (s *server) updateTenantStatus(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	var request struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := s.tenantSvc.UpdateStatus(tenantID, request.Status)
	if err != nil {
		switch {
		case errors.Is(err, tenant.ErrTenantNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, tenant.ErrInvalidTenantStatus):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "legacy_tenant_status_updated", fmt.Sprintf("tenant_id=%s,name=%s,status=%s", updated.ID, updated.Name, updated.Status), "tenant")
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) listBuildings(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.spaceSvc.ListBuildings(tenantID)
	if buildingScope != nil {
		items = filterBuildingsByScope(items, buildingScope)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) createBuilding(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID string `json:"tenant_id"`
		Name     string `json:"name"`
		Address  string `json:"address"`
		Region   string `json:"region"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantHint := request.TenantID
	if strings.TrimSpace(tenantHint) == "" {
		tenantHint = r.URL.Query().Get("tenant_id")
	}
	tenantID, ok := s.resolveTenantID(w, r, tenantHint)
	if !ok {
		return
	}
	request.TenantID = tenantID

	created, err := s.spaceSvc.CreateBuilding(request.TenantID, request.Name, request.Address, request.Region)
	if err != nil {
		switch {
		case errors.Is(err, space.ErrTenantIDRequired), errors.Is(err, space.ErrBuildingNameRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "legacy_building_created", buildingAuditTarget(created), "space")
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) listFloors(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.spaceSvc.ListFloors(tenantID)
	if buildingScope != nil {
		items = filterFloorsByScope(items, buildingScope)
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" {
		items = filterFloorsByScope(items, map[string]struct{}{placeID: {}})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) createFloor(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID   string `json:"tenant_id"`
		BuildingID string `json:"building_id"`
		PlaceID    string `json:"place_id"`
		Name       string `json:"name"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	request.TenantID = tenantID
	if strings.TrimSpace(request.BuildingID) == "" {
		request.BuildingID = request.PlaceID
	}
	if !s.requireBuildingScope(w, buildingScope, request.BuildingID) {
		return
	}

	created, err := s.spaceSvc.CreateFloor(request.TenantID, request.BuildingID, request.Name)
	if err != nil {
		switch {
		case errors.Is(err, space.ErrTenantIDRequired),
			errors.Is(err, space.ErrBuildingIDRequired),
			errors.Is(err, space.ErrFloorNameRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, space.ErrTenantOwnershipMismatch):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, space.ErrBuildingNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "legacy_floor_created", fmt.Sprintf("floor_id=%s,building_id=%s,name=%s", created.ID, created.BuildingID, created.Name), "space")
	writeJSON(w, http.StatusCreated, created)
}

func buildingAuditTarget(building space.Building) string {
	return fmt.Sprintf(
		"building_id=%s,name=%s,region=%s,status=%s",
		building.ID,
		building.Name,
		building.Region,
		building.Status,
	)
}

func doorAuditTarget(door space.Door) string {
	return fmt.Sprintf(
		"door_id=%s,building_id=%s,floor_id=%s,area_id=%s,name=%s,gateway_id=%s,kind=%s,status=%s",
		door.ID,
		door.BuildingID,
		door.FloorID,
		door.AreaID,
		door.Name,
		door.GatewayID,
		door.Kind,
		door.Status,
	)
}

func doorGroupAuditTarget(group space.DoorGroup) string {
	return fmt.Sprintf(
		"door_group_id=%s,name=%s,door_ids=%s",
		group.ID,
		group.Name,
		strings.Join(group.DoorIDs, "|"),
	)
}

func (s *server) getFloor(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	record, err := s.spaceSvc.GetFloor(tenantID, chi.URLParam(r, "floorID"))
	if err != nil {
		handleSpaceMutationError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, record.BuildingID) {
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *server) updateFloor(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID   string `json:"tenant_id"`
		BuildingID string `json:"building_id"`
		PlaceID    string `json:"place_id"`
		Name       string `json:"name"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, firstNonEmptyString(request.TenantID, r.URL.Query().Get("tenant_id")))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	floorID := chi.URLParam(r, "floorID")
	current, err := s.spaceSvc.GetFloor(tenantID, floorID)
	if err != nil {
		handleSpaceMutationError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, current.BuildingID) {
		return
	}
	buildingID := firstNonEmptyString(request.PlaceID, request.BuildingID, current.BuildingID)
	if !s.requireBuildingScope(w, buildingScope, buildingID) {
		return
	}
	name := firstNonEmptyString(request.Name, current.Name)
	updated, err := s.spaceSvc.UpdateFloor(tenantID, floorID, buildingID, name)
	if err != nil {
		handleSpaceMutationError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "legacy_floor_updated", fmt.Sprintf("floor_id=%s,building_id=%s,name=%s", updated.ID, updated.BuildingID, updated.Name), "space")
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) deleteFloor(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	floorID := chi.URLParam(r, "floorID")
	current, err := s.spaceSvc.GetFloor(tenantID, floorID)
	if err != nil {
		handleSpaceMutationError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, current.BuildingID) {
		return
	}
	if err := s.spaceSvc.DeleteFloor(tenantID, floorID); err != nil {
		handleSpaceMutationError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "legacy_floor_deleted", fmt.Sprintf("floor_id=%s,building_id=%s,name=%s", current.ID, current.BuildingID, current.Name), "space")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) listAreas(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.spaceSvc.ListAreas(tenantID)
	if buildingScope != nil {
		items = filterAreasByScope(items, buildingScope)
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" {
		items = filterAreasByScope(items, map[string]struct{}{placeID: {}})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) createArea(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID   string `json:"tenant_id"`
		BuildingID string `json:"building_id"`
		FloorID    string `json:"floor_id"`
		Name       string `json:"name"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	request.TenantID = tenantID
	if !s.requireBuildingScope(w, buildingScope, request.BuildingID) {
		return
	}

	created, err := s.spaceSvc.CreateArea(request.TenantID, request.BuildingID, request.FloorID, request.Name)
	if err != nil {
		switch {
		case errors.Is(err, space.ErrTenantIDRequired),
			errors.Is(err, space.ErrBuildingIDRequired),
			errors.Is(err, space.ErrFloorIDRequired),
			errors.Is(err, space.ErrAreaNameRequired),
			errors.Is(err, space.ErrFloorBuildingMismatch):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, space.ErrTenantOwnershipMismatch):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, space.ErrBuildingNotFound), errors.Is(err, space.ErrFloorNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "legacy_area_created", fmt.Sprintf("area_id=%s,building_id=%s,floor_id=%s,name=%s", created.ID, created.BuildingID, created.FloorID, created.Name), "space")
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) getArea(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	record, err := s.spaceSvc.GetArea(tenantID, chi.URLParam(r, "areaID"))
	if err != nil {
		handleSpaceMutationError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, record.BuildingID) {
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *server) updateArea(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID   string `json:"tenant_id"`
		BuildingID string `json:"building_id"`
		PlaceID    string `json:"place_id"`
		FloorID    string `json:"floor_id"`
		Name       string `json:"name"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, firstNonEmptyString(request.TenantID, r.URL.Query().Get("tenant_id")))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	areaID := chi.URLParam(r, "areaID")
	current, err := s.spaceSvc.GetArea(tenantID, areaID)
	if err != nil {
		handleSpaceMutationError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, current.BuildingID) {
		return
	}
	buildingID := firstNonEmptyString(request.PlaceID, request.BuildingID, current.BuildingID)
	if !s.requireBuildingScope(w, buildingScope, buildingID) {
		return
	}
	floorID := firstNonEmptyString(request.FloorID, current.FloorID)
	name := firstNonEmptyString(request.Name, current.Name)
	updated, err := s.spaceSvc.UpdateArea(tenantID, areaID, buildingID, floorID, name)
	if err != nil {
		handleSpaceMutationError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "legacy_area_updated", fmt.Sprintf("area_id=%s,building_id=%s,floor_id=%s,name=%s", updated.ID, updated.BuildingID, updated.FloorID, updated.Name), "space")
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) listDoors(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.spaceSvc.ListDoors(tenantID)
	if buildingScope != nil {
		items = filterDoorsByScope(items, buildingScope)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) createDoor(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID   string `json:"tenant_id"`
		BuildingID string `json:"building_id"`
		FloorID    string `json:"floor_id"`
		AreaID     string `json:"area_id"`
		Name       string `json:"name"`
		GatewayID  string `json:"gateway_id"`
		Kind       string `json:"kind"`
		Status     string `json:"status"`
		Mode       string `json:"mode"`
		State      string `json:"state"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	request.TenantID = tenantID
	if !s.requireBuildingScope(w, buildingScope, request.BuildingID) {
		return
	}

	if request.Kind == "" {
		request.Kind = request.Mode
	}
	if request.Status == "" {
		request.Status = request.State
	}

	created, err := s.spaceSvc.CreateDoor(
		request.TenantID,
		request.BuildingID,
		request.FloorID,
		request.AreaID,
		request.Name,
		request.GatewayID,
		request.Kind,
		request.Status,
	)
	if err != nil {
		switch {
		case errors.Is(err, space.ErrTenantIDRequired),
			errors.Is(err, space.ErrBuildingIDRequired),
			errors.Is(err, space.ErrFloorIDRequired),
			errors.Is(err, space.ErrAreaIDRequired),
			errors.Is(err, space.ErrDoorNameRequired),
			errors.Is(err, space.ErrFloorBuildingMismatch),
			errors.Is(err, space.ErrAreaFloorMismatch),
			errors.Is(err, space.ErrInvalidDoorKind),
			errors.Is(err, space.ErrInvalidDoorStatus):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, space.ErrTenantOwnershipMismatch):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, space.ErrBuildingNotFound),
			errors.Is(err, space.ErrFloorNotFound),
			errors.Is(err, space.ErrAreaNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "legacy_door_created", doorAuditTarget(created), "space")
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) listDoorGroups(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.spaceSvc.ListDoorGroups(tenantID)
	if buildingScope != nil {
		items = filterDoorGroupsByScope(items, allowedDoorIDsByBuildingScope(s.spaceSvc.ListDoors(tenantID), buildingScope))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) createDoorGroup(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID string   `json:"tenant_id"`
		Name     string   `json:"name"`
		DoorIDs  []string `json:"door_ids"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	request.TenantID = tenantID
	if buildingScope != nil {
		allowedDoorIDs := allowedDoorIDsByBuildingScope(s.spaceSvc.ListDoors(tenantID), buildingScope)
		for i := range request.DoorIDs {
			doorID := strings.TrimSpace(request.DoorIDs[i])
			if doorID == "" {
				continue
			}
			if _, exists := allowedDoorIDs[doorID]; !exists {
				writeError(w, http.StatusForbidden, "building scope forbidden")
				return
			}
		}
	}

	created, err := s.spaceSvc.CreateDoorGroup(request.TenantID, request.Name, request.DoorIDs)
	if err != nil {
		switch {
		case errors.Is(err, space.ErrTenantIDRequired), errors.Is(err, space.ErrDoorGroupNameRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, space.ErrTenantOwnershipMismatch):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, space.ErrDoorNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "legacy_door_group_created", doorGroupAuditTarget(created), "space")
	writeJSON(w, http.StatusCreated, created)
}
