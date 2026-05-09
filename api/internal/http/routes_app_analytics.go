package httpx

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/analytics/summary?days=30
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	days := 30
	if d, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && d > 0 && d <= 365 {
		days = d
	}

	// Compute analytics from access events
	since := time.Now().UTC().AddDate(0, 0, -days)
	allEvents := s.eventSvc.ListAccessEvents(tenantID)
	// Filter to time range and place
	events := make([]struct{ Actor, DoorID, Result, EventType, BuildingID string; At time.Time }, 0)
	for _, e := range allEvents {
		if e.At.Before(since) || e.BuildingID != placeID {
			continue
		}
		events = append(events, struct{ Actor, DoorID, Result, EventType, BuildingID string; At time.Time }{
			e.Actor, e.DoorID, e.Result, e.Type, e.BuildingID, e.At,
		})
	}

	totalUnlocks := 0
	failedAttempts := 0
	uniqueUsers := make(map[string]struct{})
	doorUsage := make(map[string]int)
	methodBreakdown := make(map[string]int)
	dailyMap := make(map[string][3]int) // date -> [unlocks, unique, failed]

	for _, e := range events {
		day := e.At.Format("2006-01-02")
		stats := dailyMap[day]

		if e.Result == "granted" || e.Result == "success" {
			totalUnlocks++
			stats[0]++
		} else {
			failedAttempts++
			stats[2]++
		}
		uniqueUsers[e.Actor] = struct{}{}
		stats[1] = len(uniqueUsers) // approximate
		dailyMap[day] = stats

		doorUsage[e.DoorID]++
		methodBreakdown[e.EventType]++
	}

	avgDaily := 0.0
	if days > 0 {
		avgDaily = float64(totalUnlocks) / float64(days)
	}

	// Top doors
	type doorCount struct {
		ID    string
		Count int
	}
	topDoors := make([]map[string]any, 0)
	allDoors := s.spaceSvc.ListDoors(tenantID)
	doorNames := make(map[string]string)
	for _, d := range allDoors {
		doorNames[d.ID] = d.Name
	}
	for doorID, count := range doorUsage {
		topDoors = append(topDoors, map[string]any{
			"id":    doorID,
			"name":  doorNames[doorID],
			"count": count,
		})
	}

	// Method breakdown
	methods := make([]map[string]any, 0, len(methodBreakdown))
	for method, count := range methodBreakdown {
		methods = append(methods, map[string]any{
			"method": method,
			"count":  count,
		})
	}

	// Daily trend
	daily := make([]map[string]any, 0, len(dailyMap))
	for date, stats := range dailyMap {
		daily = append(daily, map[string]any{
			"date":         date,
			"unlocks":      stats[0],
			"unique_users": stats[1],
			"failed":       stats[2],
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total_unlocks":      totalUnlocks,
		"unique_users":       len(uniqueUsers),
		"failed_attempts":    failedAttempts,
		"avg_daily_unlocks":  avgDaily,
		"period_days":        days,
		"top_doors":          topDoors,
		"unlocks_by_method":  methods,
		"daily_trend":        daily,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/analytics/presence?days=30
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appUserPresence(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	days := 30
	if d, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && d > 0 {
		days = d
	}

	since := time.Now().UTC().AddDate(0, 0, -days)
	allEventsP := s.eventSvc.ListAccessEvents(tenantID)

	type presenceData struct {
		Email        string
		Name         string
		FirstUnlock  time.Time
		LastUnlock   time.Time
		DaysPresent  map[string]struct{}
		TotalUnlocks int
	}
	byUser := make(map[string]*presenceData)

	allUsers := s.accessSvc.ListUsers(tenantID)
	userInfo := make(map[string]struct{ Name, Email string })
	for _, u := range allUsers {
		userInfo[u.ID] = struct{ Name, Email string }{u.Name, u.Email}
	}

	for _, e := range allEventsP {
		if e.At.Before(since) || e.BuildingID != placeID || (e.Result != "granted" && e.Result != "success") {
			continue
		}
		pd, exists := byUser[e.Actor]
		if !exists {
			info := userInfo[e.Actor]
			pd = &presenceData{
				Email:       info.Email,
				Name:        info.Name,
				FirstUnlock: e.At,
				DaysPresent: make(map[string]struct{}),
			}
			byUser[e.Actor] = pd
		}
		if e.At.Before(pd.FirstUnlock) {
			pd.FirstUnlock = e.At
		}
		if e.At.After(pd.LastUnlock) {
			pd.LastUnlock = e.At
		}
		pd.DaysPresent[e.At.Format("2006-01-02")] = struct{}{}
		pd.TotalUnlocks++
	}

	items := make([]map[string]any, 0, len(byUser))
	for userID, pd := range byUser {
		items = append(items, map[string]any{
			"id":            userID,
			"user_name":     pd.Name,
			"email":         pd.Email,
			"first_unlock":  pd.FirstUnlock.Format(time.RFC3339),
			"last_unlock":   pd.LastUnlock.Format(time.RFC3339),
			"days_present":  len(pd.DaysPresent),
			"total_unlocks": pd.TotalUnlocks,
		})
	}

	writeJSON(w, http.StatusOK, items)
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/places/{placeId}/reports/export
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appExportReport(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	var req struct {
		Type   string `json:"type"`
		From   string `json:"from"`
		To     string `json:"to"`
		Format string `json:"format"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Format == "" {
		req.Format = "csv"
	}

	// Generate a presigned URL placeholder for the export
	exportURL := "/api/v1/uploads/report-" + placeID + "-" + time.Now().UTC().Format("20060102T150405") + "." + req.Format
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)

	writeJSON(w, http.StatusOK, map[string]any{
		"url":        exportURL,
		"expires_at": expiresAt,
		"format":     req.Format,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/analytics/failed-attempts?days=30
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appAnalyticsFailedAttempts(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	days := 30
	if d, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && d > 0 && d <= 365 {
		days = d
	}

	since := time.Now().UTC().AddDate(0, 0, -days)
	allEvents := s.eventSvc.ListAccessEvents(tenantID)

	allDoors := s.spaceSvc.ListDoors(tenantID)
	doorNames := make(map[string]string)
	for _, d := range allDoors {
		doorNames[d.ID] = d.Name
	}

	allUsers := s.accessSvc.ListUsers(tenantID)
	userNames := make(map[string]string)
	for _, u := range allUsers {
		userNames[u.ID] = u.Name
	}

	type failedItem struct {
		ID        string `json:"id"`
		UserName  string `json:"user_name"`
		DoorName  string `json:"door_name"`
		Method    string `json:"method"`
		Reason    string `json:"reason"`
		Timestamp string `json:"timestamp"`
	}

	items := make([]failedItem, 0)
	for _, e := range allEvents {
		if e.At.Before(since) || e.BuildingID != placeID {
			continue
		}
		if e.Result == "granted" || e.Result == "success" {
			continue
		}
		items = append(items, failedItem{
			ID:        e.ID,
			UserName:  userNames[e.Actor],
			DoorName:  doorNames[e.DoorID],
			Method:    e.Type,
			Reason:    e.Result,
			Timestamp: e.At.Format(time.RFC3339),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Timestamp > items[j].Timestamp
	})

	if len(items) > 50 {
		items = items[:50]
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
