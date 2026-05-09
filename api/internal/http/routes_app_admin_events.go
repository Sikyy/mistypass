package httpx

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/events — Paginated event list (filterable)
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminListEvents(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	if strings.TrimSpace(placeID) == "" {
		writeError(w, http.StatusBadRequest, "place id is required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	// Query filter params
	q := r.URL.Query()
	filterUserID := strings.TrimSpace(q.Get("user_id"))
	filterObjectType := strings.TrimSpace(q.Get("object_type"))
	filterObjectAction := strings.TrimSpace(q.Get("object_action"))
	filterObjectID := strings.TrimSpace(q.Get("object_id"))

	// Collect access events for this building
	accessEvents := s.eventSvc.ListAccessEvents(tenantID)
	deviceEvents := s.eventSvc.ListDeviceEvents(tenantID)

	type unifiedEvent struct {
		ID         string    `json:"id"`
		PlaceID    string    `json:"place_id"`
		Timestamp  time.Time `json:"timestamp"`
		Actor      string    `json:"actor"`
		Action     string    `json:"action"`
		ObjectName string    `json:"object_name"`
		ObjectType string    `json:"object_type"`
		ObjectID   string    `json:"object_id"`
		Result     string    `json:"result"`
	}

	var all []unifiedEvent

	for _, evt := range accessEvents {
		if evt.BuildingID != placeID {
			continue
		}
		if filterUserID != "" && evt.Actor != filterUserID {
			continue
		}
		if filterObjectType != "" && filterObjectType != "door" {
			continue
		}
		if filterObjectAction != "" && evt.Type != filterObjectAction {
			continue
		}
		if filterObjectID != "" && evt.DoorID != filterObjectID {
			continue
		}
		all = append(all, unifiedEvent{
			ID:         evt.ID,
			PlaceID:    placeID,
			Timestamp:  evt.At,
			Actor:      evt.Actor,
			Action:     evt.Type,
			ObjectName: evt.DoorID,
			ObjectType: "door",
			ObjectID:   evt.DoorID,
			Result:     evt.Result,
		})
	}

	for _, evt := range deviceEvents {
		if evt.BuildingID != placeID {
			continue
		}
		if filterUserID != "" {
			continue // device events have no user actor
		}
		if filterObjectType != "" && filterObjectType != "gateway" {
			continue
		}
		if filterObjectAction != "" && evt.Type != filterObjectAction {
			continue
		}
		if filterObjectID != "" && evt.GatewayID != filterObjectID {
			continue
		}
		all = append(all, unifiedEvent{
			ID:         evt.ID,
			PlaceID:    placeID,
			Timestamp:  evt.At,
			Actor:      "system",
			Action:     evt.Type,
			ObjectName: evt.GatewayID,
			ObjectType: "gateway",
			ObjectID:   evt.GatewayID,
			Result:     evt.Result,
		})
	}

	// Sort newest first
	sort.Slice(all, func(i, j int) bool { return all[i].Timestamp.After(all[j].Timestamp) })

	total := len(all)
	offset, limit := parsePagination(r, total)

	end := offset + limit
	if end > total {
		end = total
	}
	page := all
	if offset < total {
		page = all[offset:end]
	} else {
		page = nil
	}

	items := make([]map[string]any, 0, len(page))
	for _, evt := range page {
		items = append(items, map[string]any{
			"id":          evt.ID,
			"place_id":    evt.PlaceID,
			"timestamp":   evt.Timestamp.Format(time.RFC3339),
			"actor":       evt.Actor,
			"action":      evt.Action,
			"object_name": evt.ObjectName,
			"object_type": evt.ObjectType,
			"object_id":   evt.ObjectID,
			"result":      evt.Result,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"pagination": map[string]any{
			"offset": offset,
			"limit":  limit,
			"total":  total,
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/events/{eventId} — Event details
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminGetEvent(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	eventID := chi.URLParam(r, "eventId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(eventID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and event_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	// Search access events
	for _, evt := range s.eventSvc.ListAccessEvents(tenantID) {
		if evt.ID == eventID && evt.BuildingID == placeID {
			writeJSON(w, http.StatusOK, map[string]any{
				"id":          evt.ID,
				"place_id":    placeID,
				"timestamp":   evt.At.Format(time.RFC3339),
				"actor":       evt.Actor,
				"action":      evt.Type,
				"object_name": evt.DoorID,
				"object_type": "door",
				"object_id":   evt.DoorID,
				"area_id":     evt.AreaID,
				"gateway_id":  evt.GatewayID,
				"result":      evt.Result,
			})
			return
		}
	}

	// Search device events
	for _, evt := range s.eventSvc.ListDeviceEvents(tenantID) {
		if evt.ID == eventID && evt.BuildingID == placeID {
			writeJSON(w, http.StatusOK, map[string]any{
				"id":          evt.ID,
				"place_id":    placeID,
				"timestamp":   evt.At.Format(time.RFC3339),
				"actor":       "system",
				"action":      evt.Type,
				"object_name": evt.GatewayID,
				"object_type": "gateway",
				"object_id":   evt.GatewayID,
				"detail":      evt.Detail,
				"result":      evt.Result,
			})
			return
		}
	}

	writeError(w, http.StatusNotFound, "event not found")
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/events/{eventId}/related — Related events timeline
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminGetRelatedEvents(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	eventID := chi.URLParam(r, "eventId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(eventID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and event_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	// Find the source event to determine its actor and time window
	var sourceActor string
	var sourceDoorID string
	var sourceTime time.Time
	found := false

	for _, evt := range s.eventSvc.ListAccessEvents(tenantID) {
		if evt.ID == eventID && evt.BuildingID == placeID {
			sourceActor = evt.Actor
			sourceDoorID = evt.DoorID
			sourceTime = evt.At
			found = true
			break
		}
	}

	if !found {
		// Check device events — relate by gateway
		for _, evt := range s.eventSvc.ListDeviceEvents(tenantID) {
			if evt.ID == eventID && evt.BuildingID == placeID {
				sourceActor = evt.GatewayID
				sourceTime = evt.At
				found = true
				break
			}
		}
	}

	if !found {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}

	// Collect related events: same actor or same door, within +-30 minutes
	window := 30 * time.Minute
	items := make([]map[string]any, 0)

	for _, evt := range s.eventSvc.ListAccessEvents(tenantID) {
		if evt.BuildingID != placeID || evt.ID == eventID {
			continue
		}
		withinWindow := evt.At.After(sourceTime.Add(-window)) && evt.At.Before(sourceTime.Add(window))
		if !withinWindow {
			continue
		}
		if evt.Actor == sourceActor || evt.DoorID == sourceDoorID {
			items = append(items, map[string]any{
				"id":          evt.ID,
				"place_id":    placeID,
				"timestamp":   evt.At.Format(time.RFC3339),
				"actor":       evt.Actor,
				"action":      evt.Type,
				"object_name": evt.DoorID,
				"object_type": "door",
				"result":      evt.Result,
				"relation":    relatedEventRelation(evt.Actor, sourceActor, evt.DoorID, sourceDoorID),
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"event_id": eventID,
		"items":    items,
	})
}

// relatedEventRelation describes why two events are related.
func relatedEventRelation(actor, sourceActor, doorID, sourceDoorID string) string {
	if actor == sourceActor && doorID == sourceDoorID {
		return "same_actor_same_door"
	}
	if actor == sourceActor {
		return "same_actor"
	}
	return "same_door"
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/events/{eventId}/media — Camera snapshots
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminGetEventMedia(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	eventID := chi.URLParam(r, "eventId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(eventID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and event_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	snapshots := s.cameraSvc.ListSnapshotsByEventID(tenantID, eventID)
	items := make([]map[string]any, 0, len(snapshots))
	for _, snap := range snapshots {
		items = append(items, map[string]any{
			"id":              snap.ID,
			"camera_id":       snap.CameraID,
			"event_id":        snap.EventID,
			"event_type":      snap.EventType,
			"content_type":    snap.ContentType,
			"size_bytes":      snap.SizeBytes,
			"captured_at":     snap.CapturedAt.Format(time.RFC3339),
			"retention_until": snap.RetentionUntil.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"event_id": eventID,
		"items":    items,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/incidents — Paginated incident list (filterable)
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminListIncidents(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	if strings.TrimSpace(placeID) == "" {
		writeError(w, http.StatusBadRequest, "place id is required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	// Filter params
	q := r.URL.Query()
	filterState := strings.TrimSpace(q.Get("state"))
	filterType := strings.TrimSpace(q.Get("type"))
	filterStatus := strings.TrimSpace(q.Get("status"))
	filterSeverity := strings.TrimSpace(q.Get("severity"))
	filterSubjectType := strings.TrimSpace(q.Get("subject_type"))

	// Derive incidents from denied access events and device warning/error events.
	// Group denied events by door to form access_denied incidents.
	// Treat device events with warning/error results as device_alert incidents.
	accessEvents := s.eventSvc.ListAccessEvents(tenantID)
	deviceEvents := s.eventSvc.ListDeviceEvents(tenantID)

	type incident struct {
		ID          string
		PlaceID     string
		Type        string
		State       string
		Status      string
		Severity    string
		SubjectType string
		SubjectID   string
		Description string
		CreatedAt   time.Time
		Count       int
	}

	// Group denied access events by door as incidents
	doorDenied := make(map[string][]time.Time)
	doorActors := make(map[string]string)
	for _, evt := range accessEvents {
		if evt.BuildingID != placeID {
			continue
		}
		if evt.Result != "denied" {
			continue
		}
		doorDenied[evt.DoorID] = append(doorDenied[evt.DoorID], evt.At)
		doorActors[evt.DoorID] = evt.Actor
	}

	var incidents []incident

	for doorID, times := range doorDenied {
		sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
		inc := incident{
			ID:          "inc_deny_" + doorID,
			PlaceID:     placeID,
			Type:        "access_denied",
			State:       "open",
			Status:      "active",
			Severity:    "medium",
			SubjectType: "door",
			SubjectID:   doorID,
			Description: "Access denied at " + doorID + " by " + doorActors[doorID],
			CreatedAt:   times[0],
			Count:       len(times),
		}
		incidents = append(incidents, inc)
	}

	// Device alert incidents
	for _, evt := range deviceEvents {
		if evt.BuildingID != placeID {
			continue
		}
		if evt.Result != "warning" && evt.Result != "error" {
			continue
		}
		sev := "low"
		if evt.Result == "error" {
			sev = "high"
		}
		inc := incident{
			ID:          "inc_dev_" + evt.ID,
			PlaceID:     placeID,
			Type:        "device_alert",
			State:       "open",
			Status:      "active",
			Severity:    sev,
			SubjectType: "gateway",
			SubjectID:   evt.GatewayID,
			Description: evt.Detail,
			CreatedAt:   evt.At,
			Count:       1,
		}
		incidents = append(incidents, inc)
	}

	// Apply filters
	filtered := make([]incident, 0, len(incidents))
	for _, inc := range incidents {
		if filterState != "" && inc.State != filterState {
			continue
		}
		if filterType != "" && inc.Type != filterType {
			continue
		}
		if filterStatus != "" && inc.Status != filterStatus {
			continue
		}
		if filterSeverity != "" && inc.Severity != filterSeverity {
			continue
		}
		if filterSubjectType != "" && inc.SubjectType != filterSubjectType {
			continue
		}
		filtered = append(filtered, inc)
	}

	// Sort newest first
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].CreatedAt.After(filtered[j].CreatedAt) })

	total := len(filtered)
	offset, limit := parsePagination(r, total)

	end := offset + limit
	if end > total {
		end = total
	}
	page := filtered
	if offset < total {
		page = filtered[offset:end]
	} else {
		page = nil
	}

	items := make([]map[string]any, 0, len(page))
	for _, inc := range page {
		items = append(items, map[string]any{
			"id":           inc.ID,
			"place_id":     inc.PlaceID,
			"type":         inc.Type,
			"state":        inc.State,
			"status":       inc.Status,
			"severity":     inc.Severity,
			"subject_type": inc.SubjectType,
			"subject_id":   inc.SubjectID,
			"description":  inc.Description,
			"created_at":   inc.CreatedAt.Format(time.RFC3339),
			"count":        inc.Count,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"pagination": map[string]any{
			"offset": offset,
			"limit":  limit,
			"total":  total,
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/incidents/{incidentId} — Incident details
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminGetIncident(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	incidentID := chi.URLParam(r, "incidentId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(incidentID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and incident_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	// Check for door-based denied incident: inc_deny_{doorID}
	if strings.HasPrefix(incidentID, "inc_deny_") {
		doorID := strings.TrimPrefix(incidentID, "inc_deny_")
		accessEvents := s.eventSvc.ListAccessEvents(tenantID)
		var deniedEvents []map[string]any
		var firstAt time.Time
		var lastActor string

		for _, evt := range accessEvents {
			if evt.BuildingID != placeID || evt.DoorID != doorID || evt.Result != "denied" {
				continue
			}
			if firstAt.IsZero() || evt.At.Before(firstAt) {
				firstAt = evt.At
			}
			lastActor = evt.Actor
			deniedEvents = append(deniedEvents, map[string]any{
				"event_id":  evt.ID,
				"actor":     evt.Actor,
				"timestamp": evt.At.Format(time.RFC3339),
			})
		}

		if len(deniedEvents) > 0 {
			writeJSON(w, http.StatusOK, map[string]any{
				"id":           incidentID,
				"place_id":     placeID,
				"type":         "access_denied",
				"state":        "open",
				"status":       "active",
				"severity":     "medium",
				"subject_type": "door",
				"subject_id":   doorID,
				"description":  "Access denied at " + doorID + " by " + lastActor,
				"created_at":   firstAt.Format(time.RFC3339),
				"count":        len(deniedEvents),
				"events":       deniedEvents,
			})
			return
		}
	}

	// Check for device-based incident: inc_dev_{eventID}
	if strings.HasPrefix(incidentID, "inc_dev_") {
		srcEventID := strings.TrimPrefix(incidentID, "inc_dev_")
		for _, evt := range s.eventSvc.ListDeviceEvents(tenantID) {
			if evt.ID == srcEventID && evt.BuildingID == placeID {
				sev := "low"
				if evt.Result == "error" {
					sev = "high"
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"id":           incidentID,
					"place_id":     placeID,
					"type":         "device_alert",
					"state":        "open",
					"status":       "active",
					"severity":     sev,
					"subject_type": "gateway",
					"subject_id":   evt.GatewayID,
					"description":  evt.Detail,
					"created_at":   evt.At.Format(time.RFC3339),
					"count":        1,
				})
				return
			}
		}
	}

	writeError(w, http.StatusNotFound, "incident not found")
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/incidents/{incidentId}/occurrences — Occurrence list
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminGetOccurrences(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	incidentID := chi.URLParam(r, "incidentId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(incidentID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and incident_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	items := make([]map[string]any, 0)

	// Door-based denied incident occurrences
	if strings.HasPrefix(incidentID, "inc_deny_") {
		doorID := strings.TrimPrefix(incidentID, "inc_deny_")
		for _, evt := range s.eventSvc.ListAccessEvents(tenantID) {
			if evt.BuildingID != placeID || evt.DoorID != doorID || evt.Result != "denied" {
				continue
			}
			items = append(items, map[string]any{
				"event_id":    evt.ID,
				"actor":       evt.Actor,
				"door_id":     evt.DoorID,
				"gateway_id":  evt.GatewayID,
				"result":      evt.Result,
				"occurred_at": evt.At.Format(time.RFC3339),
			})
		}
	}

	// Device alert incident occurrence (single event)
	if strings.HasPrefix(incidentID, "inc_dev_") {
		srcEventID := strings.TrimPrefix(incidentID, "inc_dev_")
		for _, evt := range s.eventSvc.ListDeviceEvents(tenantID) {
			if evt.ID == srcEventID && evt.BuildingID == placeID {
				items = append(items, map[string]any{
					"event_id":    evt.ID,
					"gateway_id":  evt.GatewayID,
					"detail":      evt.Detail,
					"result":      evt.Result,
					"occurred_at": evt.At.Format(time.RFC3339),
				})
				break
			}
		}
	}

	if len(items) == 0 {
		writeError(w, http.StatusNotFound, "incident not found")
		return
	}

	// Sort newest first
	sort.Slice(items, func(i, j int) bool {
		ti, _ := items[i]["occurred_at"].(string)
		tj, _ := items[j]["occurred_at"].(string)
		return ti > tj
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"incident_id": incidentID,
		"items":       items,
		"pagination": map[string]any{
			"offset": 0,
			"limit":  len(items),
			"total":  len(items),
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/activity — User presence / recent activity
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminGetUserActivity(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	if strings.TrimSpace(placeID) == "" {
		writeError(w, http.StatusBadRequest, "place id is required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	// Derive user presence from recent access_granted events in this place.
	// Each unique actor with a granted event is considered "present" or "recently active".
	accessEvents := s.eventSvc.ListAccessEvents(tenantID)

	type presenceEntry struct {
		Actor    string
		LastSeen time.Time
		DoorID   string
		EventID  string
	}

	presenceMap := make(map[string]*presenceEntry)
	for _, evt := range accessEvents {
		if evt.BuildingID != placeID {
			continue
		}
		if evt.Result != "success" {
			continue
		}
		existing, ok := presenceMap[evt.Actor]
		if !ok || evt.At.After(existing.LastSeen) {
			presenceMap[evt.Actor] = &presenceEntry{
				Actor:    evt.Actor,
				LastSeen: evt.At,
				DoorID:   evt.DoorID,
				EventID:  evt.ID,
			}
		}
	}

	// Convert to sorted list (most recent first)
	entries := make([]*presenceEntry, 0, len(presenceMap))
	for _, entry := range presenceMap {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].LastSeen.After(entries[j].LastSeen) })

	total := len(entries)
	offset, limit := parsePagination(r, total)

	end := offset + limit
	if end > total {
		end = total
	}
	page := entries
	if offset < total {
		page = entries[offset:end]
	} else {
		page = nil
	}

	items := make([]map[string]any, 0, len(page))
	for _, entry := range page {
		items = append(items, map[string]any{
			"user_id":   entry.Actor,
			"place_id":  placeID,
			"last_seen": entry.LastSeen.Format(time.RFC3339),
			"last_door": entry.DoorID,
			"event_id":  entry.EventID,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"pagination": map[string]any{
			"offset": offset,
			"limit":  limit,
			"total":  total,
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/activity/{eventId} — Presence event detail
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAdminGetPresenceEvent(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	eventID := chi.URLParam(r, "eventId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(eventID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and event_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	// Find the specific access event used as a presence record
	for _, evt := range s.eventSvc.ListAccessEvents(tenantID) {
		if evt.ID == eventID && evt.BuildingID == placeID {
			writeJSON(w, http.StatusOK, map[string]any{
				"id":         evt.ID,
				"place_id":   placeID,
				"user_id":    evt.Actor,
				"door_id":    evt.DoorID,
				"area_id":    evt.AreaID,
				"gateway_id": evt.GatewayID,
				"action":     evt.Type,
				"result":     evt.Result,
				"timestamp":  evt.At.Format(time.RFC3339),
			})
			return
		}
	}

	writeError(w, http.StatusNotFound, "presence event not found")
}
