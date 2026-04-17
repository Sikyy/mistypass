package httpx

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mistypass/cloud/api/internal/state"
)

func (s *server) listStateChangeLog(w http.ResponseWriter, r *http.Request) {
	if s.stateChangeReader == nil {
		writeError(w, http.StatusNotImplemented, "state change log is unavailable")
		return
	}

	limit := 100
	limitInput := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitInput != "" {
		parsedLimit, err := strconv.Atoi(limitInput)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be an integer")
			return
		}
		limit = parsedLimit
	}

	items, err := s.stateChangeReader.ListStateChanges(r.URL.Query().Get("state_key"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) replayStateChangeLog(w http.ResponseWriter, r *http.Request) {
	if s.stateChangeReplayer == nil {
		writeError(w, http.StatusNotImplemented, "state change replay is unavailable")
		return
	}

	var request struct {
		StateKey string `json:"state_key"`
		FromID   int64  `json:"from_id"`
		Limit    int    `json:"limit"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := s.stateChangeReplayer.ReplayStateChanges(request.StateKey, request.FromID, request.Limit)
	if err != nil {
		switch {
		case errors.Is(err, state.ErrStateKeyRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"state_key":      strings.TrimSpace(request.StateKey),
		"from_id":        request.FromID,
		"applied":        result.Applied,
		"last_change_id": result.LastChangeID,
	})
}

func (s *server) listStateChangeLogCheckpoints(w http.ResponseWriter, r *http.Request) {
	if s.stateChangeCheckpointReader == nil {
		writeError(w, http.StatusNotImplemented, "state change replay checkpoint is unavailable")
		return
	}

	limit := 100
	limitInput := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitInput != "" {
		parsedLimit, err := strconv.Atoi(limitInput)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be an integer")
			return
		}
		limit = parsedLimit
	}

	items, err := s.stateChangeCheckpointReader.ListReplayCheckpoints(r.URL.Query().Get("state_key"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) replayStateChangeLogFromCheckpoint(w http.ResponseWriter, r *http.Request) {
	if s.stateChangeCheckpointReplayer == nil {
		writeError(w, http.StatusNotImplemented, "state change replay checkpoint is unavailable")
		return
	}

	var request struct {
		StateKey string `json:"state_key"`
		Limit    int    `json:"limit"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := s.stateChangeCheckpointReplayer.ReplayStateChangesFromCheckpoint(request.StateKey, request.Limit)
	if err != nil {
		switch {
		case errors.Is(err, state.ErrStateKeyRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"state_key":      result.StateKey,
		"from_id":        result.FromID,
		"applied":        result.Applied,
		"last_change_id": result.LastChangeID,
		"checkpoint":     result.Checkpoint,
	})
}
