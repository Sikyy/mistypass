package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
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
		CSRPEM         string `json:"csr_pem,omitempty"` // optional CSR for mTLS client cert
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

	response := map[string]any{
		"gateway_id":    record.ID,
		"tenant_id":     record.TenantID,
		"status":        "registered",
		"device_token":  deviceToken,
		"registered_at": time.Now().UTC(),
	}

	// If CSR was provided and mTLS CA is available, sign it
	if csrPEM := request.CSRPEM; csrPEM != "" && s.gatewayDeviceCA != nil {
		certPEM, err := s.gatewayDeviceCA.SignCSR([]byte(csrPEM))
		if err != nil {
			s.logger.Warn("mTLS: CSR signing failed during registration", "error", err, "gateway_id", record.ID)
		} else {
			response["cert_pem"] = string(certPEM)
			response["ca_cert_pem"] = string(s.gatewayDeviceCA.CACertPEM)
			s.logger.Info("mTLS: client cert issued during registration", "gateway_id", record.ID)
		}
	}

	writeJSON(w, http.StatusCreated, response)
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
	if !s.authorizeGatewayBootstrapToken(w, r) {
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

// gatewayBootstrapOTAReport allows a gateway to report OTA execution results
// using its device token (no admin Bearer token required).
func (s *server) gatewayBootstrapOTAReport(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID    string `json:"gateway_id"`
		TenantID     string `json:"tenant_id"`
		TaskID       string `json:"task_id"`
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
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

	nextStatus := strings.ToLower(strings.TrimSpace(request.Status))
	switch nextStatus {
	case "dispatching", "succeeded", "failed":
	default:
		writeError(w, http.StatusBadRequest, "status must be dispatching, succeeded, or failed")
		return
	}

	task, err := s.gatewaySvc.UpdateOTATaskStatus(
		request.TenantID,
		request.GatewayID,
		request.TaskID,
		nextStatus,
		strings.TrimSpace(request.ErrorMessage),
		"gateway:"+record.ID,
	)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound),
			errors.Is(err, gateway.ErrGatewayOTATaskNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayOTATaskStatusInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, task.TenantID, "gateway_ota_task_status_reported",
		fmt.Sprintf("gateway=%s,task=%s,status=%s", record.ID, task.ID, task.Status), "gateway_ota")

	writeJSON(w, http.StatusOK, map[string]any{
		"task_id":    task.ID,
		"gateway_id": task.GatewayID,
		"status":     task.Status,
		"updated_at": task.UpdatedAt,
	})
}

// gatewayBootstrapCertRenew issues a new mTLS client certificate.
// POST /api/v1/gateway/cert/renew
// The gateway must authenticate with its existing device token (or current cert).
func (s *server) gatewayBootstrapCertRenew(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID string `json:"gateway_id"`
		TenantID  string `json:"tenant_id"`
		CSRPEM    string `json:"csr_pem"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if request.CSRPEM == "" {
		writeError(w, http.StatusBadRequest, "csr_pem is required")
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

	if s.gatewayDeviceCA == nil {
		writeError(w, http.StatusServiceUnavailable, "mTLS CA not configured")
		return
	}

	certPEM, err := s.gatewayDeviceCA.SignCSR([]byte(request.CSRPEM))
	if err != nil {
		s.logger.Warn("mTLS: CSR signing failed during renewal", "error", err, "gateway_id", record.ID)
		writeError(w, http.StatusBadRequest, "CSR signing failed: "+err.Error())
		return
	}

	s.logger.Info("mTLS: client cert renewed", "gateway_id", record.ID)
	writeJSON(w, http.StatusOK, map[string]any{
		"cert_pem":    string(certPEM),
		"ca_cert_pem": string(s.gatewayDeviceCA.CACertPEM),
	})
}
