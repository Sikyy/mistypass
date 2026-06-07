package httpx

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

func (s *server) createGatewayRollout(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	var req struct {
		FirmwareID          string                 `json:"firmware_id"`
		Target              gateway.RolloutTarget  `json:"target"`
		Phases              []gateway.RolloutPhase `json:"phases"`
		FailureThresholdPct int                    `json:"failure_threshold_pct"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rollout, err := s.gatewaySvc.CreateRollout(gateway.CreateRolloutInput{
		TenantID:            tenantID,
		FirmwareID:          req.FirmwareID,
		Target:              req.Target,
		Phases:              req.Phases,
		FailureThresholdPct: req.FailureThresholdPct,
		CreatedBy:           requestActor(r),
	})
	if err != nil {
		writeRolloutError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, rollout)
}

func (s *server) listGatewayRollouts(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.gatewaySvc.ListRollouts(tenantID)})
}

func (s *server) getGatewayRollout(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	rollout, progress, err := s.gatewaySvc.GetRolloutDetail(tenantID, chi.URLParam(r, "rolloutID"))
	if err != nil {
		writeRolloutError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rollout": rollout, "gateways": progress})
}

func (s *server) rolloutAction(action func(tenantID, id, actor string) (gateway.GatewayRollout, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
		if !ok {
			return
		}
		rollout, err := action(tenantID, chi.URLParam(r, "rolloutID"), requestActor(r))
		if err != nil {
			writeRolloutError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, rollout)
	}
}

func (s *server) approveGatewayRollout(w http.ResponseWriter, r *http.Request) {
	s.rolloutAction(s.gatewaySvc.ApproveRollout)(w, r)
}
func (s *server) pauseGatewayRollout(w http.ResponseWriter, r *http.Request) {
	s.rolloutAction(s.gatewaySvc.PauseRollout)(w, r)
}
func (s *server) resumeGatewayRollout(w http.ResponseWriter, r *http.Request) {
	s.rolloutAction(s.gatewaySvc.ResumeRollout)(w, r)
}
func (s *server) abortGatewayRollout(w http.ResponseWriter, r *http.Request) {
	s.rolloutAction(s.gatewaySvc.AbortRollout)(w, r)
}

func writeRolloutError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, gateway.ErrRolloutNotFound),
		errors.Is(err, gateway.ErrGatewayFirmwareNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, gateway.ErrRolloutStateConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, gateway.ErrRolloutFirmwareRequired),
		errors.Is(err, gateway.ErrRolloutTargetEmpty),
		errors.Is(err, gateway.ErrRolloutPhasesInvalid),
		errors.Is(err, gateway.ErrRolloutThresholdInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeInternalError(w, r, err)
	}
}
