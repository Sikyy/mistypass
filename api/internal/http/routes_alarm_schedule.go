package httpx

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/alarm"
)

func (s *server) createAlarmSchedule(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID    string   `json:"tenant_id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		DaysOfWeek  []int    `json:"days_of_week"`
		StartTime   string   `json:"start_time"`
		EndTime     string   `json:"end_time"`
		Timezone    string   `json:"timezone"`
		AlarmTypes  []string `json:"alarm_types"`
		Enabled     bool     `json:"enabled"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	schedule, err := s.alarmSvc.CreateSchedule(tenantID, alarm.AlarmSchedule{
		Name:        request.Name,
		Description: request.Description,
		DaysOfWeek:  request.DaysOfWeek,
		StartTime:   request.StartTime,
		EndTime:     request.EndTime,
		Timezone:    request.Timezone,
		AlarmTypes:  request.AlarmTypes,
		Enabled:     request.Enabled,
	})
	if err != nil {
		switch {
		case errors.Is(err, alarm.ErrScheduleNameRequired),
			errors.Is(err, alarm.ErrScheduleInvalidDayOfWeek),
			errors.Is(err, alarm.ErrScheduleInvalidTime),
			errors.Is(err, alarm.ErrScheduleTimezoneRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "alarm_schedule_created",
		fmt.Sprintf("schedule_id=%s,name=%s", schedule.ID, schedule.Name), "alarm")
	writeJSON(w, http.StatusCreated, schedule)
}

func (s *server) listAlarmSchedules(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	items := s.alarmSvc.ListSchedules(tenantID)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) getAlarmSchedule(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	scheduleID := chi.URLParam(r, "scheduleID")
	schedule, found := s.alarmSvc.GetSchedule(tenantID, scheduleID)
	if !found {
		writeError(w, http.StatusNotFound, "alarm schedule not found")
		return
	}
	writeJSON(w, http.StatusOK, schedule)
}

func (s *server) updateAlarmSchedule(w http.ResponseWriter, r *http.Request) {
	scheduleID := chi.URLParam(r, "scheduleID")
	var request struct {
		TenantID    string   `json:"tenant_id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		DaysOfWeek  []int    `json:"days_of_week"`
		StartTime   string   `json:"start_time"`
		EndTime     string   `json:"end_time"`
		Timezone    string   `json:"timezone"`
		AlarmTypes  []string `json:"alarm_types"`
		Enabled     bool     `json:"enabled"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	schedule, err := s.alarmSvc.UpdateSchedule(tenantID, scheduleID, alarm.AlarmSchedule{
		Name:        request.Name,
		Description: request.Description,
		DaysOfWeek:  request.DaysOfWeek,
		StartTime:   request.StartTime,
		EndTime:     request.EndTime,
		Timezone:    request.Timezone,
		AlarmTypes:  request.AlarmTypes,
		Enabled:     request.Enabled,
	})
	if err != nil {
		switch {
		case errors.Is(err, alarm.ErrScheduleNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, alarm.ErrScheduleInvalidDayOfWeek),
			errors.Is(err, alarm.ErrScheduleInvalidTime):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "alarm_schedule_updated",
		fmt.Sprintf("schedule_id=%s,name=%s", schedule.ID, schedule.Name), "alarm")
	writeJSON(w, http.StatusOK, schedule)
}

func (s *server) deleteAlarmSchedule(w http.ResponseWriter, r *http.Request) {
	scheduleID := chi.URLParam(r, "scheduleID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	if err := s.alarmSvc.DeleteSchedule(tenantID, scheduleID); err != nil {
		if errors.Is(err, alarm.ErrScheduleNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "alarm_schedule_deleted",
		fmt.Sprintf("schedule_id=%s", scheduleID), "alarm")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) getAlarmScheduleCalendar(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	entries := s.alarmSvc.GetWeeklyCalendar(tenantID)
	if entries == nil {
		entries = []alarm.AlarmScheduleCalendarEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": entries})
}
