package access

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

func (s *Service) ListHolidayCalendars(tenantID string) []HolidayCalendar {
	filterTenantID := strings.TrimSpace(tenantID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]HolidayCalendar, 0, len(s.holidayCalendars))
	for i := range s.holidayCalendars {
		if filterTenantID != "" && s.holidayCalendars[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.holidayCalendars[i])
	}
	return items
}

func (s *Service) GetHolidayCalendar(tenantID, calendarID string) (HolidayCalendar, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextID := strings.TrimSpace(calendarID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.holidayCalendars {
		if s.holidayCalendars[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.holidayCalendars[i].TenantID != filterTenantID {
			continue
		}
		return s.holidayCalendars[i], nil
	}
	return HolidayCalendar{}, ErrHolidayCalendarNotFound
}

func (s *Service) CreateHolidayCalendar(tenantID, name, country string, entries []HolidayEntry) (HolidayCalendar, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HolidayCalendar{}, ErrTenantIDRequired
	}
	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return HolidayCalendar{}, ErrHolidayCalendarNameRequired
	}
	idBytes := make([]byte, 6)
	rand.Read(idBytes)
	now := time.Now().UTC()
	cal := HolidayCalendar{
		ID:        "hcal_" + hex.EncodeToString(idBytes),
		TenantID:  nextTenantID,
		Name:      nextName,
		Country:   strings.TrimSpace(country),
		Entries:   normalizeHolidayEntries(entries),
		UpdatedAt: now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.holidayCalendars = append(s.holidayCalendars, cal)
	_ = s.persistLocked()
	return cal, nil
}

func (s *Service) UpdateHolidayCalendar(tenantID, calendarID, name, country string, entries []HolidayEntry) (HolidayCalendar, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	nextID := strings.TrimSpace(calendarID)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.holidayCalendars {
		if s.holidayCalendars[i].ID != nextID {
			continue
		}
		if nextTenantID != "" && s.holidayCalendars[i].TenantID != nextTenantID {
			continue
		}
		if n := strings.TrimSpace(name); n != "" {
			s.holidayCalendars[i].Name = n
		}
		if c := strings.TrimSpace(country); c != "" {
			s.holidayCalendars[i].Country = c
		}
		if entries != nil {
			s.holidayCalendars[i].Entries = normalizeHolidayEntries(entries)
		}
		s.holidayCalendars[i].UpdatedAt = time.Now().UTC()
		_ = s.persistLocked()
		return s.holidayCalendars[i], nil
	}
	return HolidayCalendar{}, ErrHolidayCalendarNotFound
}

func (s *Service) DeleteHolidayCalendar(tenantID, calendarID string) error {
	nextTenantID := strings.TrimSpace(tenantID)
	nextID := strings.TrimSpace(calendarID)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.holidayCalendars {
		if s.holidayCalendars[i].ID != nextID {
			continue
		}
		if nextTenantID != "" && s.holidayCalendars[i].TenantID != nextTenantID {
			continue
		}
		s.holidayCalendars = append(s.holidayCalendars[:i], s.holidayCalendars[i+1:]...)
		_ = s.persistLocked()
		return nil
	}
	return ErrHolidayCalendarNotFound
}

func normalizeHolidayEntries(entries []HolidayEntry) []HolidayEntry {
	if entries == nil {
		return nil
	}
	result := make([]HolidayEntry, 0, len(entries))
	for _, e := range entries {
		date := strings.TrimSpace(e.Date)
		if date == "" {
			continue
		}
		result = append(result, HolidayEntry{
			Date:        date,
			Name:        strings.TrimSpace(e.Name),
			Description: strings.TrimSpace(e.Description),
		})
	}
	return result
}

// --- Schedule CRUD ---

func (s *Service) ListSchedules(tenantID string) []Schedule {
	filterTenantID := strings.TrimSpace(tenantID)
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]Schedule, 0, len(s.schedules))
	for i := range s.schedules {
		if filterTenantID != "" && s.schedules[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.schedules[i])
	}
	return items
}

func (s *Service) GetSchedule(tenantID, scheduleID string) (Schedule, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextID := strings.TrimSpace(scheduleID)
	if nextID == "" {
		return Schedule{}, ErrScheduleNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.schedules {
		if s.schedules[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.schedules[i].TenantID != filterTenantID {
			continue
		}
		return s.schedules[i], nil
	}
	return Schedule{}, ErrScheduleNotFound
}

func (s *Service) CreateSchedule(
	tenantID, name, description, validFrom, validUntil, holidayCalendarID string,
	timeWindows []TimeWindow, exceptionDates []string,
) (Schedule, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return Schedule{}, ErrTenantIDRequired
	}
	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return Schedule{}, ErrScheduleNameRequired
	}

	id, err := accessID("sched_")
	if err != nil {
		return Schedule{}, err
	}
	now := time.Now().UTC()

	schedule := Schedule{
		ID:                id,
		TenantID:          nextTenantID,
		Name:              nextName,
		Description:       strings.TrimSpace(description),
		ValidFrom:         strings.TrimSpace(validFrom),
		ValidUntil:        strings.TrimSpace(validUntil),
		TimeWindows:       timeWindows,
		ExceptionDates:    normalizeExceptionDates(exceptionDates),
		HolidayCalendarID: strings.TrimSpace(holidayCalendarID),
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.schedules = append([]Schedule{schedule}, s.schedules...)
	if err := s.persistLocked(); err != nil {
		return Schedule{}, err
	}
	return schedule, nil
}

func (s *Service) UpdateSchedule(
	tenantID, scheduleID, name, description, validFrom, validUntil, holidayCalendarID string,
	timeWindows []TimeWindow, exceptionDates []string,
) (Schedule, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextID := strings.TrimSpace(scheduleID)
	if nextID == "" {
		return Schedule{}, ErrScheduleNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.schedules {
		if s.schedules[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.schedules[i].TenantID != filterTenantID {
			continue
		}
		if n := strings.TrimSpace(name); n != "" {
			s.schedules[i].Name = n
		}
		if d := strings.TrimSpace(description); d != "" {
			s.schedules[i].Description = d
		}
		if vf := strings.TrimSpace(validFrom); vf != "" {
			s.schedules[i].ValidFrom = vf
		}
		if vu := strings.TrimSpace(validUntil); vu != "" {
			s.schedules[i].ValidUntil = vu
		}
		if timeWindows != nil {
			s.schedules[i].TimeWindows = timeWindows
		}
		if exceptionDates != nil {
			s.schedules[i].ExceptionDates = normalizeExceptionDates(exceptionDates)
		}
		if hcID := strings.TrimSpace(holidayCalendarID); hcID != "" {
			s.schedules[i].HolidayCalendarID = hcID
		}
		s.schedules[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return Schedule{}, err
		}
		return s.schedules[i], nil
	}
	return Schedule{}, ErrScheduleNotFound
}

func (s *Service) DeleteSchedule(tenantID, scheduleID string) error {
	filterTenantID := strings.TrimSpace(tenantID)
	nextID := strings.TrimSpace(scheduleID)
	if nextID == "" {
		return ErrScheduleNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.schedules {
		if s.schedules[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.schedules[i].TenantID != filterTenantID {
			continue
		}
		s.schedules = append(s.schedules[:i], s.schedules[i+1:]...)
		if err := s.persistLocked(); err != nil {
			return err
		}
		return nil
	}
	return ErrScheduleNotFound
}

func normalizeExceptionDates(dates []string) []string {
	if dates == nil {
		return nil
	}
	result := make([]string, 0, len(dates))
	for _, d := range dates {
		trimmed := strings.TrimSpace(d)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// --- Organization Settings ---

func defaultOrganizationSettings(tenantID string) OrganizationSettings {
	return OrganizationSettings{
		TenantID:              tenantID,
		Name:                  "Mistyislet",
		PrimaryDomain:         "mistypass.local",
		Timezone:              "Asia/Jakarta",
		SupportEmail:          "support@mistypass.local",
		EmailNotifications:    true,
		PushNotifications:     true,
		WeeklyReports:         false,
		EnforceMFA:            false,
		PasswordPolicy:        "standard",
		SessionTimeoutMinutes: 480,
		Status:                "active",
		UpdatedAt:             time.Now().UTC(),
	}
}

func (s *Service) GetOrganizationSettings(tenantID string) OrganizationSettings {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return defaultOrganizationSettings(nextTenantID)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if settings, exists := s.organizationSettings[nextTenantID]; exists {
		return settings
	}
	return defaultOrganizationSettings(nextTenantID)
}

func (s *Service) UpdateOrganizationSettings(
	tenantID string,
	name, primaryDomain, timezone, supportEmail *string,
	emailNotifications, pushNotifications, weeklyReports, enforceMFA, webAuthnEnabled *bool,
	passwordPolicy *string,
	sessionTimeoutMinutes *int,
) (OrganizationSettings, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return OrganizationSettings{}, ErrTenantIDRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	settings, exists := s.organizationSettings[nextTenantID]
	if !exists {
		settings = defaultOrganizationSettings(nextTenantID)
	}

	if name != nil {
		settings.Name = strings.TrimSpace(*name)
	}
	if primaryDomain != nil {
		settings.PrimaryDomain = strings.TrimSpace(*primaryDomain)
	}
	if timezone != nil {
		settings.Timezone = strings.TrimSpace(*timezone)
	}
	if supportEmail != nil {
		settings.SupportEmail = strings.TrimSpace(*supportEmail)
	}
	if emailNotifications != nil {
		settings.EmailNotifications = *emailNotifications
	}
	if pushNotifications != nil {
		settings.PushNotifications = *pushNotifications
	}
	if weeklyReports != nil {
		settings.WeeklyReports = *weeklyReports
	}
	if enforceMFA != nil {
		settings.EnforceMFA = *enforceMFA
	}
	if webAuthnEnabled != nil {
		settings.WebAuthnEnabled = *webAuthnEnabled
	}
	if passwordPolicy != nil {
		p := strings.TrimSpace(*passwordPolicy)
		if p == "standard" || p == "strict" {
			settings.PasswordPolicy = p
		}
	}
	if sessionTimeoutMinutes != nil && *sessionTimeoutMinutes > 0 {
		settings.SessionTimeoutMinutes = *sessionTimeoutMinutes
	}
	settings.UpdatedAt = time.Now().UTC()
	s.organizationSettings[nextTenantID] = settings

	if err := s.persistLocked(); err != nil {
		return OrganizationSettings{}, err
	}
	return settings, nil
}

func (s *Service) RotateWebhookSecret(tenantID string) (string, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return "", ErrTenantIDRequired
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	newKey := hex.EncodeToString(secret)

	s.mu.Lock()
	defer s.mu.Unlock()

	settings, exists := s.organizationSettings[nextTenantID]
	if !exists {
		settings = defaultOrganizationSettings(nextTenantID)
	}

	settings.WebhookSigningKey = newKey
	settings.WebhookRotatedAt = time.Now().UTC()
	settings.UpdatedAt = time.Now().UTC()
	s.organizationSettings[nextTenantID] = settings

	if err := s.persistLocked(); err != nil {
		return "", err
	}
	return newKey, nil
}

func (s *Service) DisableOrganization(tenantID string) error {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return ErrTenantIDRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	settings, exists := s.organizationSettings[nextTenantID]
	if !exists {
		settings = defaultOrganizationSettings(nextTenantID)
	}

	settings.Status = "disabled"
	settings.UpdatedAt = time.Now().UTC()
	s.organizationSettings[nextTenantID] = settings

	if err := s.persistLocked(); err != nil {
		return err
	}
	return nil
}

func (s *Service) TransferOrganization(sourceTenantID, targetTenantID string) error {
	source := strings.TrimSpace(sourceTenantID)
	target := strings.TrimSpace(targetTenantID)
	if source == "" || target == "" {
		return ErrTenantIDRequired
	}
	if source == target {
		return errors.New("source and target tenant must be different")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Reassign all users from source to target tenant
	for i := range s.users {
		if s.users[i].TenantID == source {
			s.users[i].TenantID = target
		}
	}

	// Reassign all user groups
	for i := range s.userGroups {
		if s.userGroups[i].TenantID == source {
			s.userGroups[i].TenantID = target
		}
	}

	// Reassign all access policies
	for i := range s.policies {
		if s.policies[i].TenantID == source {
			s.policies[i].TenantID = target
		}
	}

	// Reassign invitation deliveries
	for i := range s.userInvitationDeliveries {
		if s.userInvitationDeliveries[i].TenantID == source {
			s.userInvitationDeliveries[i].TenantID = target
		}
	}

	// Reassign role assignments
	for i := range s.roleAssignments {
		if s.roleAssignments[i].TenantID == source {
			s.roleAssignments[i].TenantID = target
		}
	}

	// Move organization settings: mark source as transferred, copy to target if needed
	if sourceSettings, exists := s.organizationSettings[source]; exists {
		sourceSettings.Status = "transferred"
		sourceSettings.UpdatedAt = time.Now().UTC()
		s.organizationSettings[source] = sourceSettings
	}

	if err := s.persistLocked(); err != nil {
		return err
	}
	return nil
}

// --- Schedule Evaluation ---

type ScheduleEvaluation struct {
	IsActive         bool         `json:"is_active"`
	Reason           string       `json:"reason"`
	ValidFrom        string       `json:"valid_from,omitempty"`
	ValidUntil       string       `json:"valid_until,omitempty"`
	TimeWindows      []TimeWindow `json:"time_windows,omitempty"`
	ExceptionDates   []string     `json:"exception_dates,omitempty"`
	HolidayCalendar  string       `json:"holiday_calendar,omitempty"`
	EvaluatedAt      string       `json:"evaluated_at"`
	NextActiveWindow string       `json:"next_active_window,omitempty"`
}

func EvaluateSchedule(now time.Time, validFrom, validUntil string, timeWindows []TimeWindow, exceptionDates []string, holidays []HolidayEntry) ScheduleEvaluation {
	eval := ScheduleEvaluation{
		EvaluatedAt: now.UTC().Format(time.RFC3339),
		ValidFrom:   validFrom,
		ValidUntil:  validUntil,
		TimeWindows: timeWindows,
	}

	// check date range
	if validFrom != "" {
		from, err := time.Parse(time.RFC3339, validFrom)
		if err == nil && now.Before(from) {
			eval.Reason = "not_yet_valid"
			return eval
		}
	}
	if validUntil != "" {
		until, err := time.Parse(time.RFC3339, validUntil)
		if err == nil && now.After(until) {
			eval.Reason = "expired"
			return eval
		}
	}

	// check exception dates
	todayStr := now.Format("2006-01-02")
	for _, d := range exceptionDates {
		if strings.TrimSpace(d) == todayStr {
			eval.Reason = "exception_date"
			return eval
		}
	}

	// check holiday calendar
	for _, h := range holidays {
		if strings.TrimSpace(h.Date) == todayStr {
			eval.Reason = "holiday:" + h.Name
			return eval
		}
	}

	// check time windows
	if len(timeWindows) > 0 {
		inWindow := false
		for _, tw := range timeWindows {
			if isInTimeWindow(now, tw) {
				inWindow = true
				break
			}
		}
		if !inWindow {
			eval.Reason = "outside_time_window"
			return eval
		}
	}

	eval.IsActive = true
	eval.Reason = "active"
	return eval
}

func isInTimeWindow(now time.Time, tw TimeWindow) bool {
	// check day of week
	if !isDayInSet(now.Weekday(), tw.DayOfWeekSet) {
		return false
	}
	// check time range
	startTime := parseHHMM(tw.StartTime)
	endTime := parseHHMM(tw.EndTime)
	if startTime < 0 || endTime < 0 {
		return true // invalid time range means no restriction
	}
	currentMinutes := now.Hour()*60 + now.Minute()
	return currentMinutes >= startTime && currentMinutes < endTime
}

func isDayInSet(weekday time.Weekday, daySet string) bool {
	normalized := strings.ToLower(strings.TrimSpace(daySet))
	if normalized == "" || normalized == "all" || normalized == "everyday" {
		return true
	}
	if normalized == "weekday" || normalized == "weekdays" {
		return weekday >= time.Monday && weekday <= time.Friday
	}
	if normalized == "weekend" || normalized == "weekends" {
		return weekday == time.Saturday || weekday == time.Sunday
	}
	// parse comma-separated ISO days: 1=Mon, 7=Sun
	isoDay := int(weekday)
	if isoDay == 0 {
		isoDay = 7 // Sunday
	}
	for _, part := range strings.Split(normalized, ",") {
		d := strings.TrimSpace(part)
		if d == "" {
			continue
		}
		n := 0
		for _, ch := range d {
			if ch >= '0' && ch <= '9' {
				n = n*10 + int(ch-'0')
			}
		}
		if n == isoDay {
			return true
		}
	}
	return false
}

func parseHHMM(s string) int {
	parts := strings.SplitN(strings.TrimSpace(s), ":", 2)
	if len(parts) != 2 {
		return -1
	}
	h, hErr := strings.CutPrefix(parts[0], "0")
	if hErr {
		// trimmed leading zero
	}
	_ = h
	hour := 0
	for _, ch := range strings.TrimSpace(parts[0]) {
		if ch >= '0' && ch <= '9' {
			hour = hour*10 + int(ch-'0')
		}
	}
	minute := 0
	for _, ch := range strings.TrimSpace(parts[1]) {
		if ch >= '0' && ch <= '9' {
			minute = minute*10 + int(ch-'0')
		}
	}
	if hour > 23 || minute > 59 {
		return -1
	}
	return hour*60 + minute
}

// --- Bookable Spaces & Bookings CRUD ---

