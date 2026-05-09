package httpx

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/credential"
)

// ═══════════════════════════════════════════════════════════════════════════════
// SCHEDULES
// ═══════════════════════════════════════════════════════════════════════════════

// GET /api/v1/app/places/{placeId}/schedules
func (s *server) appAdminListSchedules(w http.ResponseWriter, r *http.Request) {
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

	items := s.accessSvc.ListSchedules(tenantID)
	if items == nil {
		items = []access.Schedule{}
	}

	mapped := make([]map[string]any, 0, len(items))
	for _, sched := range items {
		startTime, endTime, daysOfWeek := "", "", []int{}
		if len(sched.TimeWindows) > 0 {
			tw := sched.TimeWindows[0]
			startTime = tw.StartTime
			endTime = tw.EndTime
			daysOfWeek = parseDayOfWeekSet(tw.DayOfWeekSet)
		}
		mapped = append(mapped, map[string]any{
			"id":            sched.ID,
			"name":          sched.Name,
			"description":   sched.Description,
			"schedule_type": "unlock",
			"start_time":    startTime,
			"end_time":      endTime,
			"days_of_week":  daysOfWeek,
			"is_enabled":    true,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": mapped,
		"total": len(mapped),
	})
}

// POST /api/v1/app/places/{placeId}/schedules
func (s *server) appAdminCreateSchedule(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Name              string              `json:"name"`
		Description       string              `json:"description"`
		ValidFrom         string              `json:"valid_from"`
		ValidUntil        string              `json:"valid_until"`
		TimeWindows       []access.TimeWindow `json:"time_windows"`
		ExceptionDates    []string            `json:"exception_dates"`
		HolidayCalendarID string              `json:"holiday_calendar_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	schedule, err := s.accessSvc.CreateSchedule(
		tenantID, req.Name, req.Description,
		req.ValidFrom, req.ValidUntil, req.HolidayCalendarID,
		req.TimeWindows, req.ExceptionDates,
	)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrTenantIDRequired),
			errors.Is(err, access.ErrScheduleNameRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeInternalError(w, r, err)
		}
		return
	}

	writeJSON(w, http.StatusCreated, schedule)
}

// PUT /api/v1/app/places/{placeId}/schedules/{scheduleId}
func (s *server) appAdminUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	scheduleID := chi.URLParam(r, "scheduleId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(scheduleID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and schedule_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	var req struct {
		Name              string              `json:"name"`
		Description       string              `json:"description"`
		ValidFrom         string              `json:"valid_from"`
		ValidUntil        string              `json:"valid_until"`
		TimeWindows       []access.TimeWindow `json:"time_windows"`
		ExceptionDates    []string            `json:"exception_dates"`
		HolidayCalendarID string              `json:"holiday_calendar_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	schedule, err := s.accessSvc.UpdateSchedule(
		tenantID, scheduleID,
		req.Name, req.Description,
		req.ValidFrom, req.ValidUntil, req.HolidayCalendarID,
		req.TimeWindows, req.ExceptionDates,
	)
	if err != nil {
		if errors.Is(err, access.ErrScheduleNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeInternalError(w, r, err)
		}
		return
	}

	writeJSON(w, http.StatusOK, schedule)
}

// DELETE /api/v1/app/places/{placeId}/schedules/{scheduleId}
func (s *server) appAdminDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	scheduleID := chi.URLParam(r, "scheduleId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(scheduleID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and schedule_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	if err := s.accessSvc.DeleteSchedule(tenantID, scheduleID); err != nil {
		if errors.Is(err, access.ErrScheduleNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeInternalError(w, r, err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ═══════════════════════════════════════════════════════════════════════════════
// HOLIDAY REGIONS (preset countries and holidays for the mobile app)
// ═══════════════════════════════════════════════════════════════════════════════

// GET /api/v1/app/places/{placeId}/holiday-regions
func (s *server) appAdminListHolidayRegions(w http.ResponseWriter, r *http.Request) {
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

	// Return preset countries (reuse existing presetCountries from holiday_calendar_presets)
	writeJSON(w, http.StatusOK, map[string]any{
		"items": presetCountries,
		"total": len(presetCountries),
	})
}

// GET /api/v1/app/places/{placeId}/holiday-regions/{regionId}/holidays
func (s *server) appAdminListHolidays(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	regionID := chi.URLParam(r, "regionId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(regionID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and region_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	yearStr := r.URL.Query().Get("year")
	year := 2026
	if yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil && y >= 2024 && y <= 2030 {
			year = y
		}
	}

	entries, countryName := holidayPresetEntries(strings.ToUpper(regionID), year)
	if entries == nil {
		writeError(w, http.StatusNotFound, "no holidays available for region: "+regionID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"region_id":    strings.ToUpper(regionID),
		"country_name": countryName,
		"year":         year,
		"items":        entries,
		"total":        len(entries),
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// ZONES (areas within a building/place)
// ═══════════════════════════════════════════════════════════════════════════════

// GET /api/v1/app/places/{placeId}/zones
func (s *server) appAdminListZones(w http.ResponseWriter, r *http.Request) {
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

	// Zones map to Areas in the backend. Filter by building (place).
	allAreas := s.spaceSvc.ListAreas(tenantID)
	allDoors := s.spaceSvc.ListDoors(tenantID)

	// Count doors per area for this place
	doorCountByArea := make(map[string]int)
	for _, door := range allDoors {
		if door.BuildingID == placeID {
			doorCountByArea[door.AreaID]++
		}
	}

	items := make([]map[string]any, 0)
	for _, area := range allAreas {
		if area.BuildingID != placeID {
			continue
		}
		items = append(items, map[string]any{
			"id":          area.ID,
			"name":        area.Name,
			"place_id":    placeID,
			"description": "",
			"status":      "active",
			"floor_id":    area.FloorID,
			"door_count":  doorCountByArea[area.ID],
			"created_at":  area.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": len(items),
	})
}

// GET /api/v1/app/places/{placeId}/zones/{zoneId}
func (s *server) appAdminGetZone(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	zoneID := chi.URLParam(r, "zoneId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(zoneID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and zone_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	area, err := s.spaceSvc.GetArea(tenantID, zoneID)
	if err != nil {
		writeError(w, http.StatusNotFound, "zone not found")
		return
	}

	if area.BuildingID != placeID {
		writeError(w, http.StatusNotFound, "zone not found in this place")
		return
	}

	// Collect doors in this zone
	allDoors := s.spaceSvc.ListDoors(tenantID)
	doors := make([]map[string]any, 0)
	for _, door := range allDoors {
		if door.BuildingID == placeID && door.AreaID == zoneID {
			doors = append(doors, map[string]any{
				"id":     door.ID,
				"name":   door.Name,
				"kind":   door.Kind,
				"status": door.Status,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         area.ID,
		"name":       area.Name,
		"place_id":   placeID,
		"floor_id":   area.FloorID,
		"doors":      doors,
		"door_count": len(doors),
		"created_at": area.CreatedAt.Format(time.RFC3339),
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// CARDS (physical card inventory for a place)
// ═══════════════════════════════════════════════════════════════════════════════

// GET /api/v1/app/places/{placeId}/cards
func (s *server) appAdminListCards(w http.ResponseWriter, r *http.Request) {
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

	statusFilter := r.URL.Query().Get("status")

	// Return user-bound NFC cards from wallet passes (credential_kind=physical_card)
	allPasses := s.walletSvc.ListPasses(tenantID)
	mapped := make([]map[string]any, 0)
	for _, p := range allPasses {
		if p.CredentialKind != "physical_card" {
			continue
		}
		if statusFilter != "" && p.Status != statusFilter {
			continue
		}

		userName := ""
		userEmail := ""
		if p.TargetID != "" {
			if u, err := s.authService.GetUserByID(p.TargetID); err == nil {
				userName = u.Name
				userEmail = u.Email
			}
		}
		deviceName := p.DeviceName
		if deviceName == "" {
			deviceName = "NFC Card"
		}

		item := map[string]any{
			"id":          p.ID,
			"card_uid":    p.UID,
			"card_number": p.CardNumber,
			"status":      p.Status,
			"type":        "physical_card",
			"user_id":     p.TargetID,
			"user_name":   userName,
			"user_email":  userEmail,
			"device_name": deviceName,
			"issued_at":   p.IssuedAt.Format(time.RFC3339),
		}
		if p.ExpiresAt != "" {
			item["expires_at"] = p.ExpiresAt
		}
		mapped = append(mapped, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": mapped,
		"total": len(mapped),
	})
}

// POST /api/v1/app/places/{placeId}/cards/assign
func (s *server) appAdminAssignCard(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		CardNumber string `json:"card_number"`
		UID        string `json:"uid"`
		VendorID   string `json:"vendor_id"`
		Status     string `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	item, err := s.walletSvc.CreatePhysicalCardInventoryItem(
		tenantID, req.CardNumber, req.UID, req.VendorID, req.Status, user.Email,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, item)
}

// DELETE /api/v1/app/places/{placeId}/cards/{cardUid}
func (s *server) appAdminUnassignCard(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	cardUID := chi.URLParam(r, "cardUid")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(cardUID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and card_uid are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	// Find card by UID, then set status to scrapped
	items := s.walletSvc.ListPhysicalCardInventory(tenantID, "")
	var found bool
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.UID), strings.TrimSpace(cardUID)) ||
			strings.EqualFold(strings.TrimSpace(item.CardNumber), strings.TrimSpace(cardUID)) {
			_, err := s.walletSvc.UpdatePhysicalCardInventoryStatus(
				tenantID, item.ID, "scrapped", "unassigned via mobile admin", user.Email,
			)
			if err != nil {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			found = true
			break
		}
	}

	if !found {
		writeError(w, http.StatusNotFound, "card not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/app/places/{placeId}/cards/{cardUid}/status
func (s *server) appAdminGetCardStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	cardUID := chi.URLParam(r, "cardUid")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(cardUID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and card_uid are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	items := s.walletSvc.ListPhysicalCardInventory(tenantID, "")
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.UID), strings.TrimSpace(cardUID)) ||
			strings.EqualFold(strings.TrimSpace(item.CardNumber), strings.TrimSpace(cardUID)) {
			writeJSON(w, http.StatusOK, map[string]any{
				"card_uid":    cardUID,
				"card_number": item.CardNumber,
				"uid":         item.UID,
				"status":      item.Status,
				"vendor_id":   item.VendorID,
				"vendor_name": item.VendorName,
				"source":      item.Source,
				"created_at":  item.CreatedAt.Format(time.RFC3339),
				"updated_at":  item.UpdatedAt.Format(time.RFC3339),
			})
			return
		}
	}

	writeError(w, http.StatusNotFound, "card not found")
}

// POST /api/v1/app/places/{placeId}/cards/manual-token
func (s *server) appAdminManualCardToken(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		UID        string `json:"uid"`
		CardNumber string `json:"card_number"`
		ReaderID   string `json:"reader_id"`
		VendorID   string `json:"vendor_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	item, err := s.walletSvc.ScanPhysicalCardInventory(
		tenantID, req.UID, req.CardNumber, req.ReaderID, req.VendorID, user.Email,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, item)
}

// ═══════════════════════════════════════════════════════════════════════════════
// DIGITAL CREDENTIALS (mobile credentials for a place)
// ═══════════════════════════════════════════════════════════════════════════════

// GET /api/v1/app/places/{placeId}/credentials
func (s *server) appAdminListCredentials(w http.ResponseWriter, r *http.Request) {
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

	userIDFilter := r.URL.Query().Get("user_id")
	statusFilter := r.URL.Query().Get("status")

	var creds []credential.MobileCredential
	if userIDFilter != "" {
		creds = s.credentialSvc.ListUserCredentials(tenantID, userIDFilter)
	} else {
		creds = s.credentialSvc.ListActiveCredentials(tenantID)
	}

	if statusFilter != "" && statusFilter != "active" {
		filtered := make([]credential.MobileCredential, 0, len(creds))
		for _, c := range creds {
			if c.Status == statusFilter {
				filtered = append(filtered, c)
			}
		}
		creds = filtered
	}

	mapped := make([]map[string]any, 0, len(creds))
	for _, c := range creds {
		userName := ""
		if c.UserEmail != "" {
			if u, err := s.authService.FindUserByEmail(c.UserEmail); err == nil {
				userName = u.Name
			}
		}
		mapped = append(mapped, map[string]any{
			"id":              c.ID,
			"type":            "mobile_" + c.Platform,
			"user_email":      c.UserEmail,
			"user_name":       userName,
			"recipient_email": c.UserEmail,
			"platform":        c.Platform,
			"device_model":    c.DeviceModel,
			"status":          c.Status,
			"issued_at":       c.IssuedAt.Format(time.RFC3339),
			"expires_at":      c.ExpiresAt.Format(time.RFC3339),
			"usage_count":     0,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": mapped,
		"total": len(mapped),
	})
}

// POST /api/v1/app/places/{placeId}/credentials
func (s *server) appAdminCreateCredential(w http.ResponseWriter, r *http.Request) {
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

	var req credential.RegisterDeviceInput
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.TenantID = tenantID

	cred, err := s.credentialSvc.RegisterDevice(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, cred)
}

// GET /api/v1/app/places/{placeId}/credentials/{credentialId}
func (s *server) appAdminGetCredential(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	credentialID := chi.URLParam(r, "credentialId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(credentialID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and credential_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	cred, err := s.credentialSvc.GetCredential(tenantID, credentialID)
	if err != nil {
		writeError(w, http.StatusNotFound, "credential not found")
		return
	}

	writeJSON(w, http.StatusOK, cred)
}

// GET /api/v1/app/places/{placeId}/credentials/search?q=
func (s *server) appAdminSearchCredentials(w http.ResponseWriter, r *http.Request) {
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

	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	creds := s.credentialSvc.ListActiveCredentials(tenantID)
	if creds == nil {
		creds = []credential.MobileCredential{}
	}

	if query != "" {
		filtered := make([]credential.MobileCredential, 0, len(creds))
		for _, c := range creds {
			if strings.Contains(strings.ToLower(c.UserEmail), query) ||
				strings.Contains(strings.ToLower(c.UserID), query) ||
				strings.Contains(strings.ToLower(c.DeviceModel), query) ||
				strings.Contains(strings.ToLower(c.Platform), query) {
				filtered = append(filtered, c)
			}
		}
		creds = filtered
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": creds,
		"total": len(creds),
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// TEAMS (groups in the backend = teams in the mobile app)
// ═══════════════════════════════════════════════════════════════════════════════

// GET /api/v1/app/places/{placeId}/teams
func (s *server) appAdminListTeams(w http.ResponseWriter, r *http.Request) {
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

	allTeams := s.accessSvc.ListTeams(tenantID)
	allMemberships := s.accessSvc.ListTeamMemberships(tenantID)

	// Build member count per team
	memberCountByTeam := make(map[string]int)
	for _, m := range allMemberships {
		memberCountByTeam[m.TeamID]++
	}

	// Filter teams by place
	items := make([]map[string]any, 0)
	for _, team := range allTeams {
		if team.PlaceID != placeID {
			continue
		}
		items = append(items, map[string]any{
			"id":           team.ID,
			"name":         team.Name,
			"description":  team.Description,
			"scope":        team.Scope,
			"place_id":     team.PlaceID,
			"source":       team.Source,
			"member_count": memberCountByTeam[team.ID],
			"created_at":   team.CreatedAt.Format(time.RFC3339),
			"updated_at":   team.UpdatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": len(items),
	})
}

// POST /api/v1/app/places/{placeId}/teams
func (s *server) appAdminCreateTeam(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Source      string `json:"source"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "Manual"
	}

	team, err := s.accessSvc.CreateTeam(tenantID, req.Name, "place", placeID, req.Description, source)
	if err != nil {
		if errors.Is(err, access.ErrTeamNameRequired) {
			writeError(w, http.StatusBadRequest, err.Error())
		} else {
			writeInternalError(w, r, err)
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":          team.ID,
		"name":        team.Name,
		"description": team.Description,
		"scope":       team.Scope,
		"place_id":    team.PlaceID,
		"source":      team.Source,
		"created_at":  team.CreatedAt.Format(time.RFC3339),
		"updated_at":  team.UpdatedAt.Format(time.RFC3339),
	})
}

// GET /api/v1/app/places/{placeId}/teams/{teamId}
func (s *server) appAdminGetTeam(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	teamID := chi.URLParam(r, "teamId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(teamID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and team_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	team, err := s.accessSvc.GetTeam(tenantID, teamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "team not found")
		return
	}

	if team.PlaceID != placeID {
		writeError(w, http.StatusNotFound, "team not found in this place")
		return
	}

	// Collect members
	allMemberships := s.accessSvc.ListTeamMemberships(tenantID)
	members := make([]map[string]any, 0)
	for _, m := range allMemberships {
		if m.TeamID != teamID {
			continue
		}
		members = append(members, map[string]any{
			"id":           m.ID,
			"member_type":  m.MemberType,
			"member_id":    m.MemberID,
			"member_email": m.MemberEmail,
			"member_name":  m.MemberName,
			"source":       m.Source,
			"created_at":   m.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":           team.ID,
		"name":         team.Name,
		"description":  team.Description,
		"scope":        team.Scope,
		"place_id":     team.PlaceID,
		"source":       team.Source,
		"members":      members,
		"member_count": len(members),
		"created_at":   team.CreatedAt.Format(time.RFC3339),
		"updated_at":   team.UpdatedAt.Format(time.RFC3339),
	})
}

// PUT /api/v1/app/places/{placeId}/teams/{teamId}
func (s *server) appAdminUpdateTeam(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	teamID := chi.URLParam(r, "teamId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(teamID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and team_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	// Verify team belongs to this place
	existing, err := s.accessSvc.GetTeam(tenantID, teamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "team not found")
		return
	}
	if existing.PlaceID != placeID {
		writeError(w, http.StatusNotFound, "team not found in this place")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Source      string `json:"source"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = existing.Name
	}
	description := req.Description
	if description == "" {
		description = existing.Description
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = existing.Source
	}

	team, err := s.accessSvc.UpdateTeam(tenantID, teamID, name, existing.Scope, placeID, description, source)
	if err != nil {
		if errors.Is(err, access.ErrTeamNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeInternalError(w, r, err)
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":          team.ID,
		"name":        team.Name,
		"description": team.Description,
		"scope":       team.Scope,
		"place_id":    team.PlaceID,
		"source":      team.Source,
		"created_at":  team.CreatedAt.Format(time.RFC3339),
		"updated_at":  team.UpdatedAt.Format(time.RFC3339),
	})
}

// DELETE /api/v1/app/places/{placeId}/teams/{teamId}
func (s *server) appAdminDeleteTeam(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	teamID := chi.URLParam(r, "teamId")
	if strings.TrimSpace(placeID) == "" || strings.TrimSpace(teamID) == "" {
		writeError(w, http.StatusBadRequest, "place_id and team_id are required")
		return
	}

	tenantID := user.TenantID

	_, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	// Verify team belongs to this place
	existing, err := s.accessSvc.GetTeam(tenantID, teamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "team not found")
		return
	}
	if existing.PlaceID != placeID {
		writeError(w, http.StatusNotFound, "team not found in this place")
		return
	}

	if err := s.accessSvc.DeleteTeam(tenantID, teamID); err != nil {
		if errors.Is(err, access.ErrTeamNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeInternalError(w, r, err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseDayOfWeekSet(s string) []int {
	dayMap := map[string]int{"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6}
	var result []int
	for _, part := range strings.Split(s, ",") {
		if v, ok := dayMap[strings.ToLower(strings.TrimSpace(part))]; ok {
			result = append(result, v)
		}
	}
	return result
}

// ═══════════════════════════════════════════════════════════════════════════════
// CREDENTIAL REVOKE (admin, place-scoped)
// ═══════════════════════════════════════════════════════════════════════════════

// POST /api/v1/app/places/{placeId}/credentials/{credentialId}/revoke
func (s *server) appAdminRevokeCredential(w http.ResponseWriter, r *http.Request) {
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

	credentialID := chi.URLParam(r, "credentialId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	if err := s.credentialSvc.RevokeCredential(tenantID, credentialID); err != nil {
		writeError(w, http.StatusNotFound, "credential not found")
		return
	}

	s.auditSvc.Append(tenantID, user.Email, user.Role, "mobile_credential_revoked", credentialID, "mobile_admin")
	w.WriteHeader(http.StatusNoContent)
}
