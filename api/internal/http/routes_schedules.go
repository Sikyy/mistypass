package httpx

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
)

func (s *server) listSchedules(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	items := s.accessSvc.ListSchedules(tenantID)
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) getSchedule(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	schedule, err := s.accessSvc.GetSchedule(tenantID, chi.URLParam(r, "scheduleID"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, schedule)
}

func (s *server) createSchedule(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID          string              `json:"tenant_id"`
		Name              string              `json:"name"`
		Description       string              `json:"description"`
		ValidFrom         string              `json:"valid_from"`
		ValidUntil        string              `json:"valid_until"`
		TimeWindows       []access.TimeWindow `json:"time_windows"`
		ExceptionDates    []string            `json:"exception_dates"`
		HolidayCalendarID string              `json:"holiday_calendar_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	schedule, err := s.accessSvc.CreateSchedule(
		tenantID, request.Name, request.Description,
		request.ValidFrom, request.ValidUntil, request.HolidayCalendarID,
		request.TimeWindows, request.ExceptionDates,
	)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrTenantIDRequired),
			errors.Is(err, access.ErrScheduleNameRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	s.appendAuditLog(r, tenantID, "schedule_created",
		fmt.Sprintf("schedule_id=%s,name=%s", schedule.ID, schedule.Name), "access")
	writeJSON(w, http.StatusCreated, schedule)
}

func (s *server) updateSchedule(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	var request struct {
		Name              string              `json:"name"`
		Description       string              `json:"description"`
		ValidFrom         string              `json:"valid_from"`
		ValidUntil        string              `json:"valid_until"`
		TimeWindows       []access.TimeWindow `json:"time_windows"`
		ExceptionDates    []string            `json:"exception_dates"`
		HolidayCalendarID string              `json:"holiday_calendar_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	schedule, err := s.accessSvc.UpdateSchedule(
		tenantID, chi.URLParam(r, "scheduleID"),
		request.Name, request.Description,
		request.ValidFrom, request.ValidUntil, request.HolidayCalendarID,
		request.TimeWindows, request.ExceptionDates,
	)
	if err != nil {
		if errors.Is(err, access.ErrScheduleNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	s.appendAuditLog(r, tenantID, "schedule_updated",
		fmt.Sprintf("schedule_id=%s,name=%s", schedule.ID, schedule.Name), "access")
	writeJSON(w, http.StatusOK, schedule)
}

func (s *server) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	scheduleID := chi.URLParam(r, "scheduleID")
	if err := s.accessSvc.DeleteSchedule(tenantID, scheduleID); err != nil {
		if errors.Is(err, access.ErrScheduleNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	s.appendAuditLog(r, tenantID, "schedule_deleted",
		fmt.Sprintf("schedule_id=%s", scheduleID), "access")
	w.WriteHeader(http.StatusNoContent)
}
