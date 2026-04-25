package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/alarm"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/event"
)

func (s *server) listUsers(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.accessSvc.ListUsers(tenantID)
	if buildingScope != nil {
		items = filterUsersByScope(items, buildingScope)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) createUser(w http.ResponseWriter, r *http.Request) {
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
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	request.TenantID = tenantID
	if buildingScope != nil && strings.TrimSpace(request.BuildingID) == "" {
		writeError(w, http.StatusBadRequest, "building_id is required for building_admin")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, request.BuildingID) {
		return
	}

	created, err := s.accessSvc.CreateUser(
		request.TenantID,
		request.BuildingID,
		request.Name,
		request.Email,
		request.Role,
		request.Status,
		request.GroupIDs,
	)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrTenantIDRequired),
			errors.Is(err, access.ErrUserNameRequired),
			errors.Is(err, access.ErrUserEmailRequired),
			errors.Is(err, access.ErrInvalidUserStatus):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

func (s *server) listUserGroups(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.accessSvc.ListUserGroups(tenantID)
	if buildingScope != nil {
		items = filterUserGroupsByScope(items, buildingScope)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) createUserGroup(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID    string   `json:"tenant_id"`
		BuildingID  string   `json:"building_id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Members     []string `json:"members"`
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
	if buildingScope != nil && strings.TrimSpace(request.BuildingID) == "" {
		writeError(w, http.StatusBadRequest, "building_id is required for building_admin")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, request.BuildingID) {
		return
	}

	created, err := s.accessSvc.CreateUserGroup(request.TenantID, request.BuildingID, request.Name, request.Description, request.Members)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrTenantIDRequired), errors.Is(err, access.ErrUserGroupNameRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

func (s *server) updateUserGroup(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	var request struct {
		BuildingID  string   `json:"building_id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Members     []string `json:"members"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if buildingScope != nil && strings.TrimSpace(request.BuildingID) == "" {
		writeError(w, http.StatusBadRequest, "building_id is required for building_admin")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, request.BuildingID) {
		return
	}

	updated, err := s.accessSvc.UpdateUserGroup(tenantID, groupID, request.BuildingID, request.Name, request.Description, request.Members)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrUserGroupNameRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, access.ErrUserGroupNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *server) listAccessPolicies(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.accessSvc.ListPolicies(tenantID)
	if buildingScope != nil {
		items = filterPoliciesByScope(items, buildingScope)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) createAccessPolicy(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID   string `json:"tenant_id"`
		Name       string `json:"name"`
		ScopeType  string `json:"scope_type"`
		BuildingID string `json:"building_id"`
		AreaID     string `json:"area_id"`
		DoorID     string `json:"door_id"`
		Schedule   string `json:"schedule"`
		Members    int    `json:"members"`
		Status     string `json:"status"`
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
	if buildingScope != nil && strings.TrimSpace(request.BuildingID) == "" {
		writeError(w, http.StatusBadRequest, "building_id is required for building_admin")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, request.BuildingID) {
		return
	}

	created, err := s.accessSvc.CreatePolicy(
		request.TenantID,
		request.Name,
		request.ScopeType,
		request.BuildingID,
		request.AreaID,
		request.DoorID,
		request.Schedule,
		request.Members,
		request.Status,
	)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrTenantIDRequired),
			errors.Is(err, access.ErrPolicyNameRequired),
			errors.Is(err, access.ErrInvalidScopeType),
			errors.Is(err, access.ErrInvalidPolicyStatus):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

func (s *server) updateAccessPolicy(w http.ResponseWriter, r *http.Request) {
	policyID := chi.URLParam(r, "policyID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	var request struct {
		Name       string `json:"name"`
		ScopeType  string `json:"scope_type"`
		BuildingID string `json:"building_id"`
		AreaID     string `json:"area_id"`
		DoorID     string `json:"door_id"`
		Schedule   string `json:"schedule"`
		Members    int    `json:"members"`
		Status     string `json:"status"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if buildingScope != nil && strings.TrimSpace(request.BuildingID) == "" {
		writeError(w, http.StatusBadRequest, "building_id is required for building_admin")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, request.BuildingID) {
		return
	}

	updated, err := s.accessSvc.UpdatePolicy(
		tenantID,
		policyID,
		request.Name,
		request.ScopeType,
		request.BuildingID,
		request.AreaID,
		request.DoorID,
		request.Schedule,
		request.Members,
		request.Status,
	)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrPolicyNameRequired),
			errors.Is(err, access.ErrInvalidScopeType),
			errors.Is(err, access.ErrInvalidPolicyStatus):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, access.ErrPolicyNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *server) listTemporaryAccess(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.accessSvc.ListTemporaryAccess(tenantID)
	if buildingScope != nil {
		items = filterTemporaryAccessByScope(items, buildingScope)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) createTemporaryAccess(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID       string `json:"tenant_id"`
		ScopeType      string `json:"scope_type"`
		BuildingID     string `json:"building_id"`
		AreaID         string `json:"area_id"`
		DoorID         string `json:"door_id"`
		DeliveryMethod string `json:"delivery_method"`
		GranteeName    string `json:"grantee_name"`
		GranteeGender  string `json:"grantee_gender"`
		GranteePhone   string `json:"grantee_phone"`
		GranteeEmail   string `json:"grantee_email"`
		MobileModel    string `json:"mobile_model"`
		PassType       string `json:"pass_type"`
		ValidUntil     string `json:"valid_until"`
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
	if buildingScope != nil && strings.TrimSpace(request.BuildingID) == "" {
		writeError(w, http.StatusBadRequest, "building_id is required for building_admin")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, request.BuildingID) {
		return
	}

	if request.ScopeType == "" {
		request.ScopeType = "door"
	}
	if request.DoorID != "" && request.ScopeType == "all" {
		request.ScopeType = "door"
	}
	authorizedByID := "system"
	authorizedByEmail := ""
	authorizedByRole := "system"
	if user, ok := authenticatedUser(r); ok {
		authorizedByID = strings.TrimSpace(user.ID)
		authorizedByEmail = strings.TrimSpace(user.Email)
		authorizedByRole = strings.TrimSpace(user.Role)
	}

	created, err := s.accessSvc.CreateTemporaryAccess(
		request.TenantID,
		request.ScopeType,
		request.BuildingID,
		request.AreaID,
		request.DoorID,
		request.DeliveryMethod,
		request.GranteeName,
		request.GranteeGender,
		request.GranteePhone,
		request.GranteeEmail,
		request.MobileModel,
		request.PassType,
		request.ValidUntil,
		authorizedByID,
		authorizedByEmail,
		authorizedByRole,
	)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrTenantIDRequired),
			errors.Is(err, access.ErrInvalidScopeType),
			errors.Is(err, access.ErrDeliveryMethodInvalid),
			errors.Is(err, access.ErrGranteeNameRequired),
			errors.Is(err, access.ErrGranteePhoneRequired),
			errors.Is(err, access.ErrGranteeEmailRequired),
			errors.Is(err, access.ErrValidUntilRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

func (s *server) listVisitorPasses(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.accessSvc.ListVisitorPasses(tenantID)
	if buildingScope != nil {
		items = filterVisitorPassesByScope(items, buildingScope)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) createVisitorPass(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID       string `json:"tenant_id"`
		BuildingID     string `json:"building_id"`
		Host           string `json:"host"`
		Visitor        string `json:"visitor"`
		DeliveryMethod string `json:"delivery_method"`
		ExpiresAt      string `json:"expires_at"`
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
	if buildingScope != nil && strings.TrimSpace(request.BuildingID) == "" {
		writeError(w, http.StatusBadRequest, "building_id is required for building_admin")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, request.BuildingID) {
		return
	}

	created, err := s.accessSvc.CreateVisitorPass(
		request.TenantID,
		request.BuildingID,
		request.Host,
		request.Visitor,
		request.DeliveryMethod,
		request.ExpiresAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrTenantIDRequired),
			errors.Is(err, access.ErrHostRequired),
			errors.Is(err, access.ErrVisitorRequired),
			errors.Is(err, access.ErrDeliveryMethodInvalid),
			errors.Is(err, access.ErrExpiresAtRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

func (s *server) listAccessEvents(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.eventSvc.ListAccessEvents(tenantID)
	if buildingScope != nil {
		items = filterAccessEventsByScope(items, buildingScope)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) listDeviceEvents(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.eventSvc.ListDeviceEvents(tenantID)
	if buildingScope != nil {
		items = filterDeviceEventsByScope(items, buildingScope)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) streamEvents(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ctx := r.Context()
	lastRevision := ""

	send := func(eventName string, payload any) bool {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, encoded); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	sendUpdate := func() bool {
		accessItems := s.eventSvc.ListAccessEvents(tenantID)
		deviceItems := s.eventSvc.ListDeviceEvents(tenantID)
		if buildingScope != nil {
			accessItems = filterAccessEventsByScope(accessItems, buildingScope)
			deviceItems = filterDeviceEventsByScope(deviceItems, buildingScope)
		}

		revision, latestAt := eventsStreamRevision(accessItems, deviceItems)
		if revision == lastRevision {
			return true
		}
		lastRevision = revision
		return send("update", map[string]any{
			"tenant_id":    tenantID,
			"access_total": len(accessItems),
			"device_total": len(deviceItems),
			"latest_at":    latestAt,
			"revision":     revision,
			"updated_at":   time.Now().UTC().Format(time.RFC3339Nano),
		})
	}

	if !send("hello", map[string]any{
		"tenant_id":    tenantID,
		"stream":       "events",
		"connected":    true,
		"connected_at": time.Now().UTC().Format(time.RFC3339Nano),
	}) {
		return
	}
	changeCh, unsubscribe := s.eventSvc.SubscribeChanges()
	defer unsubscribe()
	if !sendUpdate() {
		return
	}

	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-changeCh:
			if !sendUpdate() {
				return
			}
		case <-heartbeatTicker.C:
			if !send("heartbeat", map[string]any{
				"stream": "events",
				"at":     time.Now().UTC().Format(time.RFC3339Nano),
			}) {
				return
			}
		}
	}
}

func eventsStreamRevision(accessItems []event.AccessEvent, deviceItems []event.DeviceEvent) (string, string) {
	latestUnix := int64(0)
	latestAt := ""

	for i := range accessItems {
		nextUnix := accessItems[i].At.UTC().UnixNano()
		if nextUnix >= latestUnix {
			latestUnix = nextUnix
			latestAt = accessItems[i].At.UTC().Format(time.RFC3339Nano)
		}
	}
	for i := range deviceItems {
		nextUnix := deviceItems[i].At.UTC().UnixNano()
		if nextUnix >= latestUnix {
			latestUnix = nextUnix
			latestAt = deviceItems[i].At.UTC().Format(time.RFC3339Nano)
		}
	}

	revision := fmt.Sprintf("%d:%d:%d", len(accessItems), len(deviceItems), latestUnix)
	return revision, latestAt
}

func (s *server) listAlarms(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.alarmSvc.List(tenantID)
	if buildingScope != nil {
		items = filterAlarmsByScope(items, buildingScope)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) streamAlarms(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ctx := r.Context()
	lastRevision := ""

	send := func(eventName string, payload any) bool {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, encoded); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	sendUpdate := func() bool {
		items := s.alarmSvc.List(tenantID)
		if buildingScope != nil {
			items = filterAlarmsByScope(items, buildingScope)
		}
		revision, latestAt, openCount := alarmsStreamRevision(items)
		if revision == lastRevision {
			return true
		}
		lastRevision = revision
		return send("update", map[string]any{
			"tenant_id":  tenantID,
			"total":      len(items),
			"open_total": openCount,
			"latest_at":  latestAt,
			"revision":   revision,
			"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
		})
	}

	if !send("hello", map[string]any{
		"tenant_id":    tenantID,
		"stream":       "alarms",
		"connected":    true,
		"connected_at": time.Now().UTC().Format(time.RFC3339Nano),
	}) {
		return
	}
	changeCh, unsubscribe := s.alarmSvc.SubscribeChanges()
	defer unsubscribe()
	if !sendUpdate() {
		return
	}

	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-changeCh:
			if !sendUpdate() {
				return
			}
		case <-heartbeatTicker.C:
			if !send("heartbeat", map[string]any{
				"stream": "alarms",
				"at":     time.Now().UTC().Format(time.RFC3339Nano),
			}) {
				return
			}
		}
	}
}

func alarmsStreamRevision(items []alarm.Alarm) (string, string, int) {
	latestUnix := int64(0)
	latestAt := ""
	openCount := 0
	for i := range items {
		if strings.EqualFold(strings.TrimSpace(items[i].Status), "open") {
			openCount++
		}
		nextUnix := items[i].CreatedAt.UTC().UnixNano()
		if nextUnix >= latestUnix {
			latestUnix = nextUnix
			latestAt = items[i].CreatedAt.UTC().Format(time.RFC3339Nano)
		}
	}
	revision := fmt.Sprintf("%d:%d:%d", len(items), openCount, latestUnix)
	return revision, latestAt, openCount
}

func (s *server) updateAlarmStatus(w http.ResponseWriter, r *http.Request) {
	alarmID := chi.URLParam(r, "alarmID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	var request struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if buildingScope != nil {
		alarms := s.alarmSvc.List(tenantID)
		targetID := strings.TrimSpace(alarmID)
		found := false
		for i := range alarms {
			if alarms[i].ID != targetID {
				continue
			}
			found = true
			if !s.requireBuildingScope(w, buildingScope, alarms[i].BuildingID) {
				return
			}
			break
		}
		if !found {
			writeError(w, http.StatusNotFound, alarm.ErrAlarmNotFound.Error())
			return
		}
	}

	updated, err := s.alarmSvc.UpdateStatus(tenantID, alarmID, request.Status)
	if err != nil {
		switch {
		case errors.Is(err, alarm.ErrAlarmNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, alarm.ErrInvalidAlarmStatus):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *server) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	limit := 0
	limitInput := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitInput != "" {
		parsedLimit, err := strconv.Atoi(limitInput)
		if err != nil || parsedLimit < 0 {
			writeError(w, http.StatusBadRequest, "limit must be an integer >= 0")
			return
		}
		limit = parsedLimit
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.auditSvc.ListFiltered(tenantID, action, source, limit),
	})
}

func (s *server) getAuditWebhookConfig(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	config, err := s.auditSvc.GetWebhookConfig(tenantID)
	if err != nil {
		switch {
		case errors.Is(err, audit.ErrWebhookTenantIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, audit.ErrWebhookConfigNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	config.SigningSecret = ""
	writeJSON(w, http.StatusOK, config)
}

func (s *server) upsertAuditWebhookConfig(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID      string   `json:"tenant_id"`
		Enabled       bool     `json:"enabled"`
		Endpoint      string   `json:"endpoint"`
		Actions       []string `json:"actions"`
		SigningSecret string   `json:"signing_secret"`
		UpdatedBy     string   `json:"updated_by"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	config, err := s.auditSvc.UpsertWebhookConfig(
		tenantID,
		request.Enabled,
		request.Endpoint,
		request.Actions,
		request.SigningSecret,
		request.UpdatedBy,
	)
	if err != nil {
		switch {
		case errors.Is(err, audit.ErrWebhookTenantIDRequired),
			errors.Is(err, audit.ErrWebhookEndpointRequired),
			errors.Is(err, audit.ErrInvalidWebhookEndpoint):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "audit_webhook_config_upsert", config.Endpoint, "web_admin")
	config.SigningSecret = ""
	writeJSON(w, http.StatusOK, config)
}

func (s *server) listAuditWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	limit := 0
	limitInput := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitInput != "" {
		parsedLimit, err := strconv.Atoi(limitInput)
		if err != nil || parsedLimit < 0 {
			writeError(w, http.StatusBadRequest, "limit must be an integer >= 0")
			return
		}
		limit = parsedLimit
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.auditSvc.ListWebhookDeliveries(tenantID, limit),
	})
}

func (s *server) dispatchAuditWebhook(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID   string `json:"tenant_id"`
		AuditLogID string `json:"audit_log_id"`
		Action     string `json:"action"`
		Source     string `json:"source"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	var targetLog audit.Log
	var err error
	auditLogID := strings.TrimSpace(request.AuditLogID)
	if auditLogID != "" {
		targetLog, err = s.auditSvc.FindLogByID(tenantID, auditLogID)
		if err != nil {
			switch {
			case errors.Is(err, audit.ErrWebhookTenantIDRequired):
				writeError(w, http.StatusBadRequest, err.Error())
			case errors.Is(err, audit.ErrAuditLogNotFound):
				writeError(w, http.StatusNotFound, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
	} else {
		items := s.auditSvc.ListFiltered(
			tenantID,
			strings.TrimSpace(request.Action),
			strings.TrimSpace(request.Source),
			1,
		)
		if len(items) == 0 {
			writeError(w, http.StatusNotFound, audit.ErrAuditLogNotFound.Error())
			return
		}
		targetLog = items[0]
	}

	delivery, dispatchErr := s.auditSvc.DispatchWebhookForLog(r.Context(), tenantID, targetLog, nil)
	if dispatchErr != nil {
		s.publishInternalEvent(
			r.Context(),
			"audit.webhook.dispatch.failed",
			map[string]any{
				"tenant_id":    tenantID,
				"audit_log_id": targetLog.ID,
				"action":       targetLog.Action,
				"delivery":     delivery,
				"error":        dispatchErr.Error(),
			},
			map[string]string{
				"tenant_id": tenantID,
				"status":    "failed",
			},
		)
		switch {
		case errors.Is(dispatchErr, audit.ErrWebhookTenantIDRequired),
			errors.Is(dispatchErr, audit.ErrWebhookEndpointRequired),
			errors.Is(dispatchErr, audit.ErrInvalidWebhookEndpoint):
			writeError(w, http.StatusBadRequest, dispatchErr.Error())
		case errors.Is(dispatchErr, audit.ErrWebhookConfigNotFound), errors.Is(dispatchErr, audit.ErrAuditLogNotFound):
			writeError(w, http.StatusNotFound, dispatchErr.Error())
		case errors.Is(dispatchErr, audit.ErrWebhookDisabled), errors.Is(dispatchErr, audit.ErrWebhookActionFiltered):
			writeError(w, http.StatusConflict, dispatchErr.Error())
		default:
			writeJSON(w, http.StatusBadGateway, map[string]any{
				"error":    dispatchErr.Error(),
				"delivery": delivery,
			})
		}
		return
	}

	s.appendAuditLog(
		r,
		tenantID,
		"audit_webhook_dispatched",
		"event_id="+targetLog.ID+",delivery_id="+delivery.ID,
		"audit_webhook",
	)
	s.publishInternalEvent(
		r.Context(),
		"audit.webhook.dispatched",
		map[string]any{
			"tenant_id":    tenantID,
			"audit_log_id": targetLog.ID,
			"action":       targetLog.Action,
			"delivery":     delivery,
		},
		map[string]string{
			"tenant_id": tenantID,
			"status":    delivery.Status,
		},
	)
	writeJSON(w, http.StatusOK, map[string]any{
		"delivery": delivery,
		"event":    targetLog,
	})
}
