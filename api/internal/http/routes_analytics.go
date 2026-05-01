package httpx

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// GET /api/v1/analytics/access-summary
// ---------------------------------------------------------------------------

type accessSummaryByDoor struct {
	DoorID   string `json:"door_id"`
	DoorName string `json:"door_name"`
	Count    int    `json:"count"`
}

type accessSummaryByDay struct {
	Date    string `json:"date"`
	Granted int    `json:"granted"`
	Denied  int    `json:"denied"`
}

type accessSummaryResponse struct {
	Period      map[string]string     `json:"period"`
	TotalEvents int                   `json:"total_events"`
	ByResult    map[string]int        `json:"by_result"`
	ByDoor      []accessSummaryByDoor `json:"by_door"`
	ByDay       []accessSummaryByDay  `json:"by_day"`
	PeakHour    int                   `json:"peak_hour"`
}

func (s *server) getAccessSummary(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}

	startStr := strings.TrimSpace(r.URL.Query().Get("start"))
	endStr := strings.TrimSpace(r.URL.Query().Get("end"))
	if startStr == "" || endStr == "" {
		writeError(w, http.StatusBadRequest, "start and end query parameters are required (RFC3339)")
		return
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start parameter: must be RFC3339")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end parameter: must be RFC3339")
		return
	}

	filterBuildingID := strings.TrimSpace(r.URL.Query().Get("building_id"))

	items := s.eventSvc.ListAccessEvents(tenantID)
	if buildingScope != nil {
		items = filterAccessEventsByScope(items, buildingScope)
	}

	byResult := map[string]int{"granted": 0, "denied": 0}
	doorCounts := map[string]int{}
	dayCounts := map[string]*accessSummaryByDay{}
	hourCounts := [24]int{}

	for i := range items {
		ev := items[i]
		if ev.At.Before(start) || !ev.At.Before(end) {
			continue
		}
		if filterBuildingID != "" && ev.BuildingID != filterBuildingID {
			continue
		}

		switch {
		case strings.EqualFold(ev.Result, "success"), strings.EqualFold(ev.Result, "accepted"):
			byResult["granted"]++
		default:
			byResult["denied"]++
		}

		doorCounts[ev.DoorID]++

		dayKey := ev.At.UTC().Format("2006-01-02")
		dayEntry, exists := dayCounts[dayKey]
		if !exists {
			dayEntry = &accessSummaryByDay{Date: dayKey}
			dayCounts[dayKey] = dayEntry
		}
		switch {
		case strings.EqualFold(ev.Result, "success"), strings.EqualFold(ev.Result, "accepted"):
			dayEntry.Granted++
		default:
			dayEntry.Denied++
		}

		hourCounts[ev.At.UTC().Hour()]++
	}

	totalEvents := byResult["granted"] + byResult["denied"]

	// Resolve door names via space service.
	byDoor := make([]accessSummaryByDoor, 0, len(doorCounts))
	for doorID, count := range doorCounts {
		doorName := doorID
		if s.spaceSvc != nil {
			if door, err := s.spaceSvc.GetDoor(tenantID, doorID); err == nil {
				doorName = door.Name
			}
		}
		byDoor = append(byDoor, accessSummaryByDoor{
			DoorID:   doorID,
			DoorName: doorName,
			Count:    count,
		})
	}

	byDay := make([]accessSummaryByDay, 0, len(dayCounts))
	for _, entry := range dayCounts {
		byDay = append(byDay, *entry)
	}

	peakHour := 0
	peakCount := 0
	for h := 0; h < 24; h++ {
		if hourCounts[h] > peakCount {
			peakCount = hourCounts[h]
			peakHour = h
		}
	}

	writeJSON(w, http.StatusOK, accessSummaryResponse{
		Period:      map[string]string{"start": start.UTC().Format(time.RFC3339), "end": end.UTC().Format(time.RFC3339)},
		TotalEvents: totalEvents,
		ByResult:    byResult,
		ByDoor:      byDoor,
		ByDay:       byDay,
		PeakHour:    peakHour,
	})
}

// ---------------------------------------------------------------------------
// GET /api/v1/analytics/door-activity
// ---------------------------------------------------------------------------

type doorActivityEntry struct {
	DoorID             string `json:"door_id"`
	TotalAccess        int    `json:"total_access"`
	UniqueUsers        int    `json:"unique_users"`
	HourlyDistribution [24]int `json:"hourly_distribution"`
}

type doorActivityResponse struct {
	Doors []doorActivityEntry `json:"doors"`
}

func (s *server) getDoorActivity(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}

	filterDoorID := strings.TrimSpace(r.URL.Query().Get("door_id"))
	filterBuildingID := strings.TrimSpace(r.URL.Query().Get("building_id"))

	if filterDoorID == "" && filterBuildingID == "" {
		writeError(w, http.StatusBadRequest, "door_id or building_id query parameter is required")
		return
	}

	daysStr := strings.TrimSpace(r.URL.Query().Get("days"))
	days := 7
	if daysStr != "" {
		parsed, err := strconv.Atoi(daysStr)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "days must be a positive integer")
			return
		}
		days = parsed
	}
	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)

	items := s.eventSvc.ListAccessEvents(tenantID)
	if buildingScope != nil {
		items = filterAccessEventsByScope(items, buildingScope)
	}

	type doorAgg struct {
		totalAccess        int
		uniqueUsers        map[string]struct{}
		hourlyDistribution [24]int
	}
	doorMap := map[string]*doorAgg{}

	for i := range items {
		ev := items[i]
		if ev.At.Before(cutoff) {
			continue
		}
		if filterDoorID != "" && ev.DoorID != filterDoorID {
			continue
		}
		if filterBuildingID != "" && ev.BuildingID != filterBuildingID {
			continue
		}

		agg, exists := doorMap[ev.DoorID]
		if !exists {
			agg = &doorAgg{uniqueUsers: map[string]struct{}{}}
			doorMap[ev.DoorID] = agg
		}
		agg.totalAccess++
		if ev.Actor != "" {
			agg.uniqueUsers[ev.Actor] = struct{}{}
		}
		agg.hourlyDistribution[ev.At.UTC().Hour()]++
	}

	doors := make([]doorActivityEntry, 0, len(doorMap))
	for doorID, agg := range doorMap {
		doors = append(doors, doorActivityEntry{
			DoorID:             doorID,
			TotalAccess:        agg.totalAccess,
			UniqueUsers:        len(agg.uniqueUsers),
			HourlyDistribution: agg.hourlyDistribution,
		})
	}

	writeJSON(w, http.StatusOK, doorActivityResponse{
		Doors: doors,
	})
}

// ---------------------------------------------------------------------------
// GET /api/v1/analytics/alarm-metrics
// ---------------------------------------------------------------------------

type alarmMetricsResponse struct {
	Total                   int            `json:"total"`
	BySeverity              map[string]int `json:"by_severity"`
	ByStatus                map[string]int `json:"by_status"`
	MeanResolutionMinutes   float64        `json:"mean_resolution_minutes"`
}

func (s *server) getAlarmMetrics(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}

	startStr := strings.TrimSpace(r.URL.Query().Get("start"))
	endStr := strings.TrimSpace(r.URL.Query().Get("end"))
	if startStr == "" || endStr == "" {
		writeError(w, http.StatusBadRequest, "start and end query parameters are required (RFC3339)")
		return
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start parameter: must be RFC3339")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end parameter: must be RFC3339")
		return
	}

	items := s.alarmSvc.List(tenantID)
	if buildingScope != nil {
		items = filterAlarmsByScope(items, buildingScope)
	}

	bySeverity := map[string]int{}
	byStatus := map[string]int{}
	resolvedCount := 0
	var totalResolutionMinutes float64

	now := time.Now().UTC()

	for i := range items {
		alm := items[i]
		if alm.CreatedAt.Before(start) || !alm.CreatedAt.Before(end) {
			continue
		}

		severity := strings.ToLower(strings.TrimSpace(alm.Severity))
		if severity == "" {
			severity = "unknown"
		}
		bySeverity[severity]++

		status := strings.ToLower(strings.TrimSpace(alm.Status))
		if status == "" {
			status = "unknown"
		}
		byStatus[status]++

		if status == "resolved" || status == "false_positive" {
			resolvedCount++
			totalResolutionMinutes += now.Sub(alm.CreatedAt).Minutes()
		}
	}

	total := 0
	for _, count := range bySeverity {
		total += count
	}

	meanResolutionMinutes := float64(0)
	if resolvedCount > 0 {
		meanResolutionMinutes = totalResolutionMinutes / float64(resolvedCount)
	}

	writeJSON(w, http.StatusOK, alarmMetricsResponse{
		Total:                 total,
		BySeverity:            bySeverity,
		ByStatus:              byStatus,
		MeanResolutionMinutes: meanResolutionMinutes,
	})
}
