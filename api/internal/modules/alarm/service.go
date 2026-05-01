package alarm

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrAlarmNotFound = errors.New("alarm not found")
var ErrInvalidAlarmStatus = errors.New("invalid alarm status")
var ErrScheduleNotFound = errors.New("alarm schedule not found")
var ErrScheduleNameRequired = errors.New("alarm schedule name is required")
var ErrScheduleInvalidDayOfWeek = errors.New("alarm schedule day_of_week must be 0-6")
var ErrScheduleInvalidTime = errors.New("alarm schedule start_time/end_time must be HH:MM format")
var ErrScheduleTimezoneRequired = errors.New("alarm schedule timezone is required")

type Alarm struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	BuildingID string    `json:"building_id"`
	AreaID     string    `json:"area_id"`
	DoorID     string    `json:"door_id"`
	Type       string    `json:"type"`
	Severity   string    `json:"severity"`
	Location   string    `json:"location"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type AlarmSchedule struct {
	ID          string   `json:"id"`
	TenantID    string   `json:"tenant_id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	DaysOfWeek  []int    `json:"days_of_week"`
	StartTime   string   `json:"start_time"`
	EndTime     string   `json:"end_time"`
	Timezone    string   `json:"timezone"`
	AlarmTypes  []string `json:"alarm_types"`
	Enabled     bool     `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AlarmScheduleCalendarEntry struct {
	ScheduleID string   `json:"schedule_id"`
	Name       string   `json:"name"`
	DayOfWeek  int      `json:"day_of_week"`
	StartTime  string   `json:"start_time"`
	EndTime    string   `json:"end_time"`
	AlarmTypes []string `json:"alarm_types"`
}

type StateStore interface {
	Load(key string, dst any) (bool, error)
	Save(key string, value any) error
}

const stateKey = "module_alarm"

type stateSnapshot struct {
	Alarms    []Alarm         `json:"alarms"`
	Schedules []AlarmSchedule `json:"schedules,omitempty"`
}

type Service struct {
	mu          sync.RWMutex
	alarms      []Alarm
	schedules   []AlarmSchedule
	stateStore  StateStore
	subscribers map[uint64]chan struct{}
	nextSubID   uint64
}

func NewService() *Service {
	now := time.Now().UTC()
	return &Service{
		alarms: []Alarm{
			{
				ID:         "alm_9001",
				TenantID:   "tenant_demo_factory",
				BuildingID: "building_demo_003",
				AreaID:     "area_demo_003",
				DoorID:     "door_fct_029",
				Type:       "Forced Door Open",
				Severity:   "critical",
				Location:   "Serang Plant A / F1 / Packing Zone",
				Status:     "open",
				CreatedAt:  now.Add(-66 * time.Minute),
			},
			{
				ID:         "alm_9002",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_002",
				AreaID:     "area_demo_002",
				DoorID:     "door_jkt_014",
				Type:       "Gateway Heartbeat Timeout",
				Severity:   "high",
				Location:   "Kuningan Tower / L3 / Lobby",
				Status:     "investigating",
				CreatedAt:  now.Add(-83 * time.Minute),
			},
			{
				ID:         "alm_9003",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_001",
				AreaID:     "area_demo_001",
				DoorID:     "door_jkt_001",
				Type:       "Abnormal Command Retry",
				Severity:   "medium",
				Location:   "Sudirman Hub / L8 / Finance",
				Status:     "acknowledged",
				CreatedAt:  now.Add(-110 * time.Minute),
			},
		},
		subscribers: make(map[uint64]chan struct{}),
	}
}

func NewServiceWithStateStore(store StateStore) (*Service, error) {
	svc := NewService()
	svc.stateStore = store
	if err := svc.restoreFromStateStore(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *Service) List(tenantID string) []Alarm {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]Alarm, 0, len(s.alarms))
	for i := range s.alarms {
		if filterTenantID != "" && s.alarms[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.alarms[i])
	}
	return items
}

func (s *Service) SubscribeChanges() (<-chan struct{}, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subID := s.nextSubID
	s.nextSubID++
	ch := make(chan struct{}, 1)
	s.subscribers[subID] = ch

	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		target, exists := s.subscribers[subID]
		if !exists {
			return
		}
		delete(s.subscribers, subID)
		close(target)
	}
}

func (s *Service) UpdateStatus(tenantID, alarmID, status string) (Alarm, error) {
	nextStatus, err := normalizeStatus(status)
	if err != nil {
		return Alarm{}, err
	}

	targetID := strings.TrimSpace(alarmID)
	if targetID == "" {
		return Alarm{}, ErrAlarmNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.alarms {
		if s.alarms[i].ID != targetID {
			continue
		}
		if filterTenantID != "" && s.alarms[i].TenantID != filterTenantID {
			return Alarm{}, ErrAlarmNotFound
		}
		s.alarms[i].Status = nextStatus
		if err := s.persistLocked(); err != nil {
			return Alarm{}, err
		}
		s.notifySubscribersLocked()
		return s.alarms[i], nil
	}

	return Alarm{}, ErrAlarmNotFound
}

func (s *Service) restoreFromStateStore() error {
	if s.stateStore == nil {
		return nil
	}

	var snapshot stateSnapshot
	found, err := s.stateStore.Load(stateKey, &snapshot)
	if err != nil {
		return err
	}
	if !found {
		return s.stateStore.Save(stateKey, stateSnapshot{
			Alarms:    cloneAlarms(s.alarms),
			Schedules: cloneSchedules(s.schedules),
		})
	}

	s.mu.Lock()
	s.alarms = cloneAlarms(snapshot.Alarms)
	s.schedules = cloneSchedules(snapshot.Schedules)
	s.mu.Unlock()
	return nil
}

func (s *Service) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(stateKey, stateSnapshot{
		Alarms:    cloneAlarms(s.alarms),
		Schedules: cloneSchedules(s.schedules),
	})
}

func (s *Service) notifySubscribersLocked() {
	for _, subscriber := range s.subscribers {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
}

func cloneAlarms(items []Alarm) []Alarm {
	output := make([]Alarm, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneSchedules(items []AlarmSchedule) []AlarmSchedule {
	output := make([]AlarmSchedule, 0, len(items))
	for i := range items {
		s := items[i]
		s.DaysOfWeek = append([]int(nil), s.DaysOfWeek...)
		s.AlarmTypes = append([]string(nil), s.AlarmTypes...)
		output = append(output, s)
	}
	return output
}

func normalizeStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "open":
		return "open", nil
	case "acknowledged":
		return "acknowledged", nil
	case "investigating":
		return "investigating", nil
	case "mitigated":
		return "mitigated", nil
	case "escalated":
		return "escalated", nil
	case "resolved":
		return "resolved", nil
	case "false_positive":
		return "false_positive", nil
	default:
		return "", ErrInvalidAlarmStatus
	}
}

func alarmScheduleID() (string, error) {
	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "alarm_schedule_" + hex.EncodeToString(raw), nil
}

func validateSchedule(schedule AlarmSchedule) error {
	if strings.TrimSpace(schedule.Name) == "" {
		return ErrScheduleNameRequired
	}
	for _, d := range schedule.DaysOfWeek {
		if d < 0 || d > 6 {
			return ErrScheduleInvalidDayOfWeek
		}
	}
	if !isValidTimeHHMM(schedule.StartTime) || !isValidTimeHHMM(schedule.EndTime) {
		return ErrScheduleInvalidTime
	}
	if strings.TrimSpace(schedule.Timezone) == "" {
		return ErrScheduleTimezoneRequired
	}
	return nil
}

func isValidTimeHHMM(t string) bool {
	if len(t) != 5 || t[2] != ':' {
		return false
	}
	h := (t[0]-'0')*10 + (t[1] - '0')
	m := (t[3]-'0')*10 + (t[4] - '0')
	return h <= 23 && m <= 59
}

func (s *Service) CreateSchedule(tenantID string, schedule AlarmSchedule) (AlarmSchedule, error) {
	if strings.TrimSpace(tenantID) == "" {
		return AlarmSchedule{}, errors.New("tenant_id is required")
	}
	if err := validateSchedule(schedule); err != nil {
		return AlarmSchedule{}, err
	}

	id, err := alarmScheduleID()
	if err != nil {
		return AlarmSchedule{}, err
	}
	now := time.Now().UTC()
	record := AlarmSchedule{
		ID:          id,
		TenantID:    strings.TrimSpace(tenantID),
		Name:        strings.TrimSpace(schedule.Name),
		Description: strings.TrimSpace(schedule.Description),
		DaysOfWeek:  append([]int(nil), schedule.DaysOfWeek...),
		StartTime:   schedule.StartTime,
		EndTime:     schedule.EndTime,
		Timezone:    strings.TrimSpace(schedule.Timezone),
		AlarmTypes:  append([]string(nil), schedule.AlarmTypes...),
		Enabled:     schedule.Enabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.schedules = append([]AlarmSchedule{record}, s.schedules...)
	if err := s.persistLocked(); err != nil {
		return AlarmSchedule{}, err
	}
	return record, nil
}

func (s *Service) ListSchedules(tenantID string) []AlarmSchedule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]AlarmSchedule, 0, len(s.schedules))
	for i := range s.schedules {
		if filterTenantID != "" && s.schedules[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.schedules[i])
	}
	return items
}

func (s *Service) GetSchedule(tenantID, scheduleID string) (AlarmSchedule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	targetID := strings.TrimSpace(scheduleID)
	filterTenantID := strings.TrimSpace(tenantID)
	for i := range s.schedules {
		if s.schedules[i].ID != targetID {
			continue
		}
		if filterTenantID != "" && s.schedules[i].TenantID != filterTenantID {
			continue
		}
		return s.schedules[i], true
	}
	return AlarmSchedule{}, false
}

func (s *Service) UpdateSchedule(tenantID, scheduleID string, schedule AlarmSchedule) (AlarmSchedule, error) {
	targetID := strings.TrimSpace(scheduleID)
	if targetID == "" {
		return AlarmSchedule{}, ErrScheduleNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.schedules {
		if s.schedules[i].ID != targetID {
			continue
		}
		if filterTenantID != "" && s.schedules[i].TenantID != filterTenantID {
			return AlarmSchedule{}, ErrScheduleNotFound
		}

		if name := strings.TrimSpace(schedule.Name); name != "" {
			s.schedules[i].Name = name
		}
		if desc := strings.TrimSpace(schedule.Description); desc != "" {
			s.schedules[i].Description = desc
		}
		if len(schedule.DaysOfWeek) > 0 {
			for _, d := range schedule.DaysOfWeek {
				if d < 0 || d > 6 {
					return AlarmSchedule{}, ErrScheduleInvalidDayOfWeek
				}
			}
			s.schedules[i].DaysOfWeek = append([]int(nil), schedule.DaysOfWeek...)
		}
		if schedule.StartTime != "" {
			if !isValidTimeHHMM(schedule.StartTime) {
				return AlarmSchedule{}, ErrScheduleInvalidTime
			}
			s.schedules[i].StartTime = schedule.StartTime
		}
		if schedule.EndTime != "" {
			if !isValidTimeHHMM(schedule.EndTime) {
				return AlarmSchedule{}, ErrScheduleInvalidTime
			}
			s.schedules[i].EndTime = schedule.EndTime
		}
		if tz := strings.TrimSpace(schedule.Timezone); tz != "" {
			s.schedules[i].Timezone = tz
		}
		if len(schedule.AlarmTypes) > 0 {
			s.schedules[i].AlarmTypes = append([]string(nil), schedule.AlarmTypes...)
		}
		s.schedules[i].Enabled = schedule.Enabled
		s.schedules[i].UpdatedAt = time.Now().UTC()

		if err := s.persistLocked(); err != nil {
			return AlarmSchedule{}, err
		}
		return s.schedules[i], nil
	}

	return AlarmSchedule{}, ErrScheduleNotFound
}

func (s *Service) DeleteSchedule(tenantID, scheduleID string) error {
	targetID := strings.TrimSpace(scheduleID)
	if targetID == "" {
		return ErrScheduleNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.schedules {
		if s.schedules[i].ID != targetID {
			continue
		}
		if filterTenantID != "" && s.schedules[i].TenantID != filterTenantID {
			return ErrScheduleNotFound
		}
		s.schedules = append(s.schedules[:i], s.schedules[i+1:]...)
		return s.persistLocked()
	}

	return ErrScheduleNotFound
}

func (s *Service) GetWeeklyCalendar(tenantID string) []AlarmScheduleCalendarEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	var entries []AlarmScheduleCalendarEntry
	for i := range s.schedules {
		sched := s.schedules[i]
		if filterTenantID != "" && sched.TenantID != filterTenantID {
			continue
		}
		if !sched.Enabled {
			continue
		}
		for _, day := range sched.DaysOfWeek {
			entries = append(entries, AlarmScheduleCalendarEntry{
				ScheduleID: sched.ID,
				Name:       sched.Name,
				DayOfWeek:  day,
				StartTime:  sched.StartTime,
				EndTime:    sched.EndTime,
				AlarmTypes: append([]string(nil), sched.AlarmTypes...),
			})
		}
	}
	return entries
}
