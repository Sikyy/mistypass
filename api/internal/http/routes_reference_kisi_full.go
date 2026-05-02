package httpx

// Full Kisi parity endpoints: Elevators, ElevatorStops, GroupElevatorStops,
// GroupTerminals, Presences, CSVCardImports, first_to_arrive/last_to_leave,
// users sign_up, users password change.

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/auth"
)

// =====================================================================
// Elevators CRUD
// =====================================================================

func (s *server) listElevators(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	items := s.accessSvc.ListElevators(tenantID)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) createElevator(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID    string `json:"tenant_id"`
		PlaceID     string `json:"place_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, req.TenantID)
	if !ok {
		return
	}
	elevator, err := s.accessSvc.CreateElevator(tenantID, req.PlaceID, req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.appendAuditLog(r, tenantID, "elevator_created", fmt.Sprintf("id=%s,name=%s", elevator.ID, elevator.Name), "access")
	writeJSON(w, http.StatusCreated, elevator)
}

func (s *server) getElevator(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	elevator, err := s.accessSvc.GetElevator(tenantID, chi.URLParam(r, "elevatorID"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, elevator)
}

func (s *server) updateElevator(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	elevator, err := s.accessSvc.UpdateElevator(tenantID, chi.URLParam(r, "elevatorID"), req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.appendAuditLog(r, tenantID, "elevator_updated", fmt.Sprintf("id=%s", elevator.ID), "access")
	writeJSON(w, http.StatusOK, elevator)
}

func (s *server) deleteElevator(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if err := s.accessSvc.DeleteElevator(tenantID, chi.URLParam(r, "elevatorID")); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.appendAuditLog(r, tenantID, "elevator_deleted", fmt.Sprintf("id=%s", chi.URLParam(r, "elevatorID")), "access")
	w.WriteHeader(http.StatusNoContent)
}

// =====================================================================
// Elevator Stops CRUD + lock_down / cancel_lockdown
// =====================================================================

func (s *server) listElevatorStops(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	items := s.accessSvc.ListElevatorStops(tenantID, r.URL.Query().Get("elevator_id"))
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) createElevatorStop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID   string `json:"tenant_id"`
		ElevatorID string `json:"elevator_id"`
		FloorID    string `json:"floor_id"`
		Name       string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, req.TenantID)
	if !ok {
		return
	}
	stop, err := s.accessSvc.CreateElevatorStop(tenantID, req.ElevatorID, req.FloorID, req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.appendAuditLog(r, tenantID, "elevator_stop_created", fmt.Sprintf("id=%s", stop.ID), "access")
	writeJSON(w, http.StatusCreated, stop)
}

func (s *server) getElevatorStop(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	stop, err := s.accessSvc.GetElevatorStop(tenantID, chi.URLParam(r, "elevatorStopID"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stop)
}

func (s *server) updateElevatorStop(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	var req struct {
		Name *string `json:"name"`
	}
	_ = decodeJSON(r, &req)
	stop, err := s.accessSvc.UpdateElevatorStop(tenantID, chi.URLParam(r, "elevatorStopID"), req.Name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stop)
}

func (s *server) deleteElevatorStop(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if err := s.accessSvc.DeleteElevatorStop(tenantID, chi.URLParam(r, "elevatorStopID")); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) lockDownElevatorStop(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	stop, err := s.accessSvc.SetElevatorStopStatus(tenantID, chi.URLParam(r, "elevatorStopID"), "locked_down")
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.appendAuditLog(r, tenantID, "elevator_stop_locked_down", fmt.Sprintf("id=%s", stop.ID), "access")
	writeJSON(w, http.StatusOK, stop)
}

func (s *server) cancelElevatorStopLockdown(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	stop, err := s.accessSvc.SetElevatorStopStatus(tenantID, chi.URLParam(r, "elevatorStopID"), "active")
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.appendAuditLog(r, tenantID, "elevator_stop_lockdown_cancelled", fmt.Sprintf("id=%s", stop.ID), "access")
	writeJSON(w, http.StatusOK, stop)
}

// =====================================================================
// Group Elevator Stops CRUD
// =====================================================================

func (s *server) listGroupElevatorStops(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	items := s.accessSvc.ListGroupElevatorStops(tenantID, r.URL.Query().Get("group_id"))
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) createGroupElevatorStop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID       string `json:"tenant_id"`
		GroupID        string `json:"group_id"`
		ElevatorStopID string `json:"elevator_stop_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, req.TenantID)
	if !ok {
		return
	}
	item, err := s.accessSvc.CreateGroupElevatorStop(tenantID, req.GroupID, req.ElevatorStopID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *server) getGroupElevatorStop(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	targetID := chi.URLParam(r, "groupElevatorStopID")
	items := s.accessSvc.ListGroupElevatorStops(tenantID, "")
	for i := range items {
		if items[i].ID == targetID {
			writeJSON(w, http.StatusOK, items[i])
			return
		}
	}
	writeError(w, http.StatusNotFound, "group elevator stop not found")
}

func (s *server) deleteGroupElevatorStop(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if err := s.accessSvc.DeleteGroupElevatorStop(tenantID, chi.URLParam(r, "groupElevatorStopID")); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// =====================================================================
// Group Terminals CRUD
// =====================================================================

func (s *server) listGroupTerminals(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	items := s.accessSvc.ListGroupTerminals(tenantID, r.URL.Query().Get("group_id"))
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) createGroupTerminal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID   string `json:"tenant_id"`
		GroupID    string `json:"group_id"`
		TerminalID string `json:"terminal_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, req.TenantID)
	if !ok {
		return
	}
	item, err := s.accessSvc.CreateGroupTerminal(tenantID, req.GroupID, req.TerminalID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *server) getGroupTerminal(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	targetID := chi.URLParam(r, "groupTerminalID")
	items := s.accessSvc.ListGroupTerminals(tenantID, "")
	for i := range items {
		if items[i].ID == targetID {
			writeJSON(w, http.StatusOK, items[i])
			return
		}
	}
	writeError(w, http.StatusNotFound, "group terminal not found")
}

func (s *server) deleteGroupTerminal(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if err := s.accessSvc.DeleteGroupTerminal(tenantID, chi.URLParam(r, "groupTerminalID")); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// =====================================================================
// Presences
// =====================================================================

func (s *server) listPresences(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	items := s.accessSvc.ListPresences(tenantID, r.URL.Query().Get("place_id"))
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// =====================================================================
// CSV Card Imports
// =====================================================================

func (s *server) listCSVCardImports(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	items := s.accessSvc.ListCSVCardImports(tenantID)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) createCSVCardImport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID string `json:"tenant_id"`
		FileName string `json:"file_name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, req.TenantID)
	if !ok {
		return
	}
	item, err := s.accessSvc.CreateCSVCardImport(tenantID, req.FileName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.appendAuditLog(r, tenantID, "csv_card_import_created", fmt.Sprintf("id=%s", item.ID), "access")
	writeJSON(w, http.StatusCreated, item)
}

func (s *server) getCSVCardImport(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	item, err := s.accessSvc.GetCSVCardImport(tenantID, chi.URLParam(r, "importID"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// =====================================================================
// Locks first_to_arrive / last_to_leave
// =====================================================================

func (s *server) firstToArriveReferenceLock(w http.ResponseWriter, r *http.Request) {
	s.writeReferenceLockAction(w, r, "first_to_arrive")
}

func (s *server) lastToLeaveReferenceLock(w http.ResponseWriter, r *http.Request) {
	s.writeReferenceLockAction(w, r, "last_to_leave")
}

// =====================================================================
// Users sign_up + password change
// =====================================================================

func (s *server) userSignUp(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.SelfRegistrationEnabled {
		writeError(w, http.StatusForbidden, "self-registration is disabled")
		return
	}

	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.Password)
	name := strings.TrimSpace(req.Name)
	if email == "" || password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}
	if err := auth.ValidatePasswordPolicy(password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := s.authService.CreateUser(email, name, password)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.appendAuditLog(r, "", "user_signed_up", fmt.Sprintf("user_id=%s,email=%s", user.ID, user.Email), "auth")
	writeJSON(w, http.StatusCreated, user)
}

func (s *server) changeUserPassword(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.authService.ChangePassword(user.ID, req.CurrentPassword, req.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.appendAuditLog(r, user.TenantID, "user_password_changed", fmt.Sprintf("user_id=%s", user.ID), "auth")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ensure imports are used
var _ = access.Elevator{}
var _ = auth.ErrPasswordTooWeak
