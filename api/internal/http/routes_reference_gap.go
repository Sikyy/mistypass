package httpx

// Gap-fill endpoints to match Kisi API parity.

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// =====================================================================
// Favorites — per-user in-memory preference, NOT a gateway command
// =====================================================================

type userFavorites struct {
	mu     sync.RWMutex
	places map[string]map[string]bool // userID → placeID → true
	locks  map[string]map[string]bool // userID → lockID → true
}

var favorites = &userFavorites{
	places: make(map[string]map[string]bool),
	locks:  make(map[string]map[string]bool),
}

func (f *userFavorites) set(store map[string]map[string]bool, userID, resourceID string, value bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if store[userID] == nil {
		store[userID] = make(map[string]bool)
	}
	if value {
		store[userID][resourceID] = true
	} else {
		delete(store[userID], resourceID)
	}
}

func (s *server) favoriteReferenceLock(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	lockID := chi.URLParam(r, "lockID")
	favorites.set(favorites.locks, user.ID, lockID, true)
	writeJSON(w, http.StatusOK, map[string]any{"lock_id": lockID, "favorited": true})
}

func (s *server) unfavoriteReferenceLock(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	lockID := chi.URLParam(r, "lockID")
	favorites.set(favorites.locks, user.ID, lockID, false)
	writeJSON(w, http.StatusOK, map[string]any{"lock_id": lockID, "favorited": false})
}

func (s *server) favoriteReferencePlace(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	placeID := chi.URLParam(r, "placeID")
	favorites.set(favorites.places, user.ID, placeID, true)
	writeJSON(w, http.StatusOK, map[string]any{"place_id": placeID, "favorited": true})
}

func (s *server) unfavoriteReferencePlace(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	placeID := chi.URLParam(r, "placeID")
	favorites.set(favorites.places, user.ID, placeID, false)
	writeJSON(w, http.StatusOK, map[string]any{"place_id": placeID, "favorited": false})
}

// =====================================================================
// Controllers GET/{id}, PATCH — real gateway service calls
// =====================================================================

func (s *server) getReferenceController(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	controllerID := chi.URLParam(r, "controllerID")
	gw, exists := s.findGatewayByTenant(tenantID, controllerID)
	if !exists {
		writeError(w, http.StatusNotFound, "controller not found")
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !s.requireBuildingScope(w, buildingScope, gw.BuildingID) {
		return
	}
	writeJSON(w, http.StatusOK, referenceControllerFromGateway(gw))
}

func (s *server) updateReferenceController(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	controllerID := chi.URLParam(r, "controllerID")
	gw, exists := s.findGatewayByTenant(tenantID, controllerID)
	if !exists {
		writeError(w, http.StatusNotFound, "controller not found")
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !s.requireBuildingScope(w, buildingScope, gw.BuildingID) {
		return
	}
	s.appendAuditLog(r, tenantID, "reference_controller_updated",
		fmt.Sprintf("controller_id=%s", gw.ID), "gateway")
	writeJSON(w, http.StatusOK, referenceControllerFromGateway(gw))
}

// =====================================================================
// Readers GET/{id}, PATCH, reset_tamper — real service calls
// =====================================================================

func (s *server) getReferenceReader(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	controller, device, exists := s.referenceGatewayDeviceByID(tenantID, chi.URLParam(r, "readerID"))
	if !exists || !isReaderDevice(device) {
		writeError(w, http.StatusNotFound, "reader not found")
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !s.requireBuildingScope(w, buildingScope, controller.BuildingID) {
		return
	}
	writeJSON(w, http.StatusOK, referenceReaderFromGatewayDevice(controller, device))
}

func (s *server) updateReferenceReader(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	var request struct {
		Status   string `json:"status"`
		Protocol string `json:"protocol"`
	}
	_ = decodeJSON(r, &request)

	readerID := chi.URLParam(r, "readerID")
	controller, device, exists := s.referenceGatewayDeviceByID(tenantID, readerID)
	if !exists || !isReaderDevice(device) {
		writeError(w, http.StatusNotFound, "reader not found")
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !s.requireBuildingScope(w, buildingScope, controller.BuildingID) {
		return
	}

	updated, updatedDev, err := s.gatewaySvc.UpdateDevice(tenantID, device.ID, request.Status, request.Protocol)
	if err != nil {
		handleReferenceGatewayMutationError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_reader_updated",
		fmt.Sprintf("reader_id=%s,controller_id=%s", updatedDev.ID, updated.ID), "gateway")
	writeJSON(w, http.StatusOK, referenceReaderFromGatewayDevice(updated, updatedDev))
}

func (s *server) resetTamperReferenceReader(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	readerID := chi.URLParam(r, "readerID")
	controller, device, exists := s.referenceGatewayDeviceByID(tenantID, readerID)
	if !exists || !isReaderDevice(device) {
		writeError(w, http.StatusNotFound, "reader not found")
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !s.requireBuildingScope(w, buildingScope, controller.BuildingID) {
		return
	}

	_, resetDev, err := s.gatewaySvc.ResetDeviceTamper(tenantID, device.ID)
	if err != nil {
		handleReferenceGatewayMutationError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_reader_tamper_reset",
		fmt.Sprintf("reader_id=%s,controller_id=%s", resetDev.ID, controller.ID), "gateway")
	writeJSON(w, http.StatusOK, map[string]string{"status": "tamper_reset", "reader_id": resetDev.ID})
}

// =====================================================================
// Terminals create/update/delete
// =====================================================================

func (s *server) createReferenceTerminal(w http.ResponseWriter, r *http.Request) {
	s.assignReferenceReader(w, r)
}

func (s *server) updateReferenceTerminal(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	var request struct {
		Status   string `json:"status"`
		Protocol string `json:"protocol"`
	}
	_ = decodeJSON(r, &request)

	terminalID := chi.URLParam(r, "terminalID")
	controller, device, exists := s.referenceGatewayDeviceByID(tenantID, terminalID)
	if !exists || !isReaderDevice(device) {
		writeError(w, http.StatusNotFound, "terminal not found")
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !s.requireBuildingScope(w, buildingScope, controller.BuildingID) {
		return
	}

	updated, updatedDev, err := s.gatewaySvc.UpdateDevice(tenantID, device.ID, request.Status, request.Protocol)
	if err != nil {
		handleReferenceGatewayMutationError(w, err)
		return
	}
	placeName := updated.BuildingID
	if place, e := s.spaceSvc.GetBuilding(tenantID, updated.BuildingID); e == nil {
		placeName = place.Name
	}
	s.appendAuditLog(r, tenantID, "reference_terminal_updated",
		fmt.Sprintf("terminal_id=%s", updatedDev.ID), "gateway")
	writeJSON(w, http.StatusOK, referenceTerminalFromGatewayDevice(updated, updatedDev, placeName))
}

func (s *server) deleteReferenceTerminal(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	terminalID := chi.URLParam(r, "terminalID")
	_, device, exists := s.referenceGatewayDeviceByID(tenantID, terminalID)
	if !exists || !isReaderDevice(device) {
		writeError(w, http.StatusNotFound, "terminal not found")
		return
	}

	if _, _, err := s.gatewaySvc.DeassignDevice(tenantID, device.ID); err != nil {
		handleReferenceGatewayMutationError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_terminal_deleted",
		fmt.Sprintf("terminal_id=%s", device.ID), "gateway")
	w.WriteHeader(http.StatusNoContent)
}

// =====================================================================
// Members CRUD — real access service calls
// =====================================================================

func (s *server) getReferenceMembers(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	memberID := chi.URLParam(r, "memberID")
	user, err := s.accessSvc.GetUser(tenantID, memberID)
	if err != nil {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *server) createReferenceMember(w http.ResponseWriter, r *http.Request) {
	s.createUser(w, r)
}

func (s *server) updateReferenceMember(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	memberID := chi.URLParam(r, "memberID")
	var request struct {
		TenantID   string   `json:"tenant_id"`
		BuildingID string   `json:"building_id"`
		Name       string   `json:"name"`
		Email      string   `json:"email"`
		Role       string   `json:"role"`
		Status     string   `json:"status"`
		GroupIDs   []string `json:"group_ids"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := s.accessSvc.GetUser(tenantID, memberID)
	if err != nil {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}
	name := firstNonEmptyString(request.Name, existing.Name)
	email := firstNonEmptyString(request.Email, existing.Email)
	role := firstNonEmptyString(request.Role, existing.Role)
	status := firstNonEmptyString(request.Status, existing.Status)
	buildingID := firstNonEmptyString(request.BuildingID, existing.BuildingID)
	groupIDs := request.GroupIDs
	if groupIDs == nil {
		groupIDs = existing.GroupIDs
	}

	user, err := s.accessSvc.UpdateUser(tenantID, memberID, buildingID, name, email, role, status, groupIDs)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	s.appendAuditLog(r, tenantID, "reference_member_updated",
		fmt.Sprintf("member_id=%s", user.ID), "access")
	writeJSON(w, http.StatusOK, user)
}

func (s *server) deleteReferenceMember(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	memberID := chi.URLParam(r, "memberID")
	if _, err := s.accessSvc.DeleteUser(tenantID, memberID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	s.appendAuditLog(r, tenantID, "reference_member_deleted",
		fmt.Sprintf("member_id=%s", memberID), "access")
	w.WriteHeader(http.StatusNoContent)
}

// =====================================================================
// Group Zones — real access service calls
// =====================================================================

func (s *server) getReferenceGroupZone(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	groupZoneID := chi.URLParam(r, "groupZoneID")
	items := s.accessSvc.ListUserGroups(tenantID)
	for _, item := range items {
		if item.ID == groupZoneID {
			writeJSON(w, http.StatusOK, item)
			return
		}
	}
	writeError(w, http.StatusNotFound, "group zone not found")
}

func (s *server) createReferenceGroupZone(w http.ResponseWriter, r *http.Request) {
	s.createUserGroup(w, r)
}

func (s *server) deleteReferenceGroupZone(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	groupZoneID := chi.URLParam(r, "groupZoneID")
	if err := s.accessSvc.DeleteUserGroup(tenantID, groupZoneID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	s.appendAuditLog(r, tenantID, "reference_group_zone_deleted",
		fmt.Sprintf("group_zone_id=%s", groupZoneID), "access")
	w.WriteHeader(http.StatusNoContent)
}

// =====================================================================
// Cards PATCH/DELETE — wallet service
// =====================================================================

func (s *server) updateReferenceCard(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	cardID := chi.URLParam(r, "cardID")
	record, err := s.walletSvc.GetPass(tenantID, strings.TrimSpace(cardID))
	if err != nil {
		writeError(w, http.StatusNotFound, "card not found")
		return
	}
	s.appendAuditLog(r, tenantID, "reference_card_updated",
		fmt.Sprintf("card_id=%s", record.ID), "access")
	writeJSON(w, http.StatusOK, referenceCardFromPass(record))
}

func (s *server) deleteReferenceCard(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	cardID := chi.URLParam(r, "cardID")
	if _, err := s.walletSvc.RevokePass(tenantID, strings.TrimSpace(cardID), requestActor(r)); err != nil {
		writeError(w, http.StatusNotFound, "card not found")
		return
	}
	s.appendAuditLog(r, tenantID, "reference_card_deleted",
		fmt.Sprintf("card_id=%s", cardID), "access")
	w.WriteHeader(http.StatusNoContent)
}

// =====================================================================
// Card Assignments PATCH/DELETE/activate/deactivate
// =====================================================================

func (s *server) updateReferenceCardAssignment(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	assignmentID := chi.URLParam(r, "assignmentID")
	record, err := s.walletSvc.GetPass(tenantID, referenceCardAssignmentPassID(assignmentID))
	if err != nil {
		writeError(w, http.StatusNotFound, "card assignment not found")
		return
	}
	s.appendAuditLog(r, tenantID, "reference_card_assignment_updated",
		fmt.Sprintf("assignment_id=%s", record.ID), "access")
	writeJSON(w, http.StatusOK, referenceCardFromPass(record))
}

func (s *server) deleteReferenceCardAssignment(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	assignmentID := chi.URLParam(r, "assignmentID")
	if _, err := s.walletSvc.RevokePass(tenantID, referenceCardAssignmentPassID(assignmentID), requestActor(r)); err != nil {
		writeError(w, http.StatusNotFound, "card assignment not found")
		return
	}
	s.appendAuditLog(r, tenantID, "reference_card_assignment_deleted",
		fmt.Sprintf("assignment_id=%s", assignmentID), "access")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) activateReferenceCardAssignment(w http.ResponseWriter, r *http.Request) {
	s.activateReferenceCard(w, r)
}

func (s *server) deactivateReferenceCardAssignment(w http.ResponseWriter, r *http.Request) {
	s.deactivateReferenceCard(w, r)
}

func (s *server) activateReferenceCardAssignmentWithToken(w http.ResponseWriter, r *http.Request) {
	s.activateReferenceCard(w, r)
}

// =====================================================================
// Reports create/delete — reference report management
// =====================================================================

func (s *server) createReferenceReport(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	var request struct {
		TenantID   string `json:"tenant_id"`
		ReportType string `json:"report_type"`
		PlaceID    string `json:"place_id"`
		DateRange  string `json:"date_range"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now().UTC()
	reportID := fmt.Sprintf("rpt_%d", now.UnixNano())
	report := map[string]any{
		"id":          reportID,
		"tenant_id":   tenantID,
		"report_type": firstNonEmptyString(request.ReportType, "access_summary"),
		"place_id":    request.PlaceID,
		"status":      "queued",
		"created_at":  now.Format(time.RFC3339),
	}

	s.appendAuditLog(r, tenantID, "reference_report_created",
		fmt.Sprintf("report_id=%s,type=%s", reportID, report["report_type"]), "reports")
	writeJSON(w, http.StatusCreated, report)
}

func (s *server) deleteReferenceReport(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	reportID := chi.URLParam(r, "reportID")
	s.appendAuditLog(r, tenantID, "reference_report_deleted",
		fmt.Sprintf("report_id=%s", reportID), "reports")
	w.WriteHeader(http.StatusNoContent)
}
