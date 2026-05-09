package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/alarm"
)

func (s *server) appListAlarms(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	items := s.alarmSvc.List(user.TenantID)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) appUpdateAlarmStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	alarmID := chi.URLParam(r, "alarmID")
	var request struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := s.alarmSvc.UpdateStatus(user.TenantID, alarmID, request.Status)
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
	s.appendAuditLog(r, user.TenantID, "alarm_status_updated",
		fmt.Sprintf("alarm_id=%s,status=%s,user=%s", updated.ID, updated.Status, user.Email), "alarm")
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) appStreamAlarms(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
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
	tenantID := user.TenantID
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
		revision, latestAt, openCount := alarmsStreamRevision(items)
		if revision == lastRevision {
			return true
		}
		lastRevision = revision
		return send("update", map[string]any{
			"total":      len(items),
			"open_total": openCount,
			"latest_at":  latestAt,
			"revision":   revision,
			"items":      items,
		})
	}

	if !send("hello", map[string]any{"stream": "alarms", "connected": true}) {
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

func (s *server) appListAlarmSchedules(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	items := s.alarmSvc.ListSchedules(user.TenantID)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) appGetAlarmCalendar(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	entries := s.alarmSvc.GetWeeklyCalendar(user.TenantID)
	writeJSON(w, http.StatusOK, map[string]any{"items": entries})
}
