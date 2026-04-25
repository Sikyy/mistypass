package httpx

import (
	"errors"
	"net/http"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

func (s *server) gatewayBootstrapRegister(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeGatewayBootstrapToken(w, r) {
		return
	}

	var request struct {
		SerialNumber   string `json:"serial_number"`
		TenantID       string `json:"tenant_id"`
		BuildingID     string `json:"building_id"`
		DeviceCapacity int    `json:"device_capacity"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	record, err := s.gatewaySvc.Register(request.SerialNumber, request.TenantID, request.BuildingID, request.DeviceCapacity)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrSerialNumberRequired),
			errors.Is(err, gateway.ErrSerialNumberInvalid),
			errors.Is(err, gateway.ErrTenantIDRequired),
			errors.Is(err, gateway.ErrInvalidDeviceCapacity):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, gateway.ErrGatewaySerialAlreadyRegistered),
			errors.Is(err, gateway.ErrSerialNumberNotProvisioned),
			errors.Is(err, gateway.ErrSerialNumberTenantMismatch),
			errors.Is(err, gateway.ErrSerialNumberProductTypeMismatch),
			errors.Is(err, gateway.ErrSerialNumberNotAvailable),
			errors.Is(err, gateway.ErrSerialNumberAlreadyConsumed):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	deviceTokenRaw, err := randomHexID(16)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	deviceToken := "gw_" + deviceTokenRaw
	s.setGatewayDeviceToken(record.ID, deviceToken)

	writeJSON(w, http.StatusCreated, map[string]any{
		"gateway_id":    record.ID,
		"tenant_id":     record.TenantID,
		"status":        "registered",
		"device_token":  deviceToken,
		"registered_at": time.Now().UTC(),
	})
}

func (s *server) gatewayBootstrapActivate(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID string `json:"gateway_id"`
		TenantID  string `json:"tenant_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	record, ok := s.findGatewayByTenant(request.TenantID, request.GatewayID)
	if !ok {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if !s.authorizeGatewayDeviceToken(w, r, record.ID) {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"gateway_id":   record.ID,
		"tenant_id":    record.TenantID,
		"status":       "active",
		"activated_at": time.Now().UTC(),
	})
}

func (s *server) gatewayBootstrapHeartbeat(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID string `json:"gateway_id"`
		TenantID  string `json:"tenant_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	record, ok := s.findGatewayByTenant(request.TenantID, request.GatewayID)
	if !ok {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if !s.authorizeGatewayDeviceToken(w, r, record.ID) {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"gateway_id":  record.ID,
		"tenant_id":   record.TenantID,
		"status":      "ok",
		"received_at": time.Now().UTC(),
	})
}

func (s *server) gatewayBootstrapStatus(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID string `json:"gateway_id"`
		TenantID  string `json:"tenant_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	record, ok := s.findGatewayByTenant(request.TenantID, request.GatewayID)
	if !ok {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if !s.authorizeGatewayDeviceToken(w, r, record.ID) {
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"gateway_id":  record.ID,
		"tenant_id":   record.TenantID,
		"accepted_at": time.Now().UTC(),
	})
}
