package httpx

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

func (s *server) listGateways(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.gatewaySvc.List(tenantID)
	if buildingScope != nil {
		items = filterGatewaysByScope(items, buildingScope)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) listGatewaySerialInventory(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	items, err := s.gatewaySvc.ListSerialInventory(
		tenantID,
		r.URL.Query().Get("product_type"),
		r.URL.Query().Get("status"),
	)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrSerialInventoryProductTypeInvalid),
			errors.Is(err, gateway.ErrSerialInventoryStatusInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) importGatewaySerialInventory(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID string `json:"tenant_id"`
		Items    []struct {
			SerialNumber string `json:"serial_number"`
			ProductType  string `json:"product_type"`
			BatchCode    string `json:"batch_code"`
			Source       string `json:"source"`
		} `json:"items"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	records := make([]gateway.SerialInventoryImportItem, 0, len(request.Items))
	for i := range request.Items {
		records = append(records, gateway.SerialInventoryImportItem{
			SerialNumber: request.Items[i].SerialNumber,
			ProductType:  request.Items[i].ProductType,
			BatchCode:    request.Items[i].BatchCode,
			Source:       request.Items[i].Source,
		})
	}

	items, err := s.gatewaySvc.ImportSerialInventory(tenantID, records)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrTenantIDRequired),
			errors.Is(err, gateway.ErrSerialInventoryRecordsRequired),
			errors.Is(err, gateway.ErrSerialInventoryProductTypeInvalid),
			errors.Is(err, gateway.ErrSerialNumberRequired),
			errors.Is(err, gateway.ErrSerialNumberInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, gateway.ErrSerialInventoryTenantMismatch),
			errors.Is(err, gateway.ErrSerialInventoryProductTypeMismatch):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"items": items,
	})
	s.appendAuditLog(r, tenantID, "serial_inventory_import", "count="+strconv.Itoa(len(items)), "web_admin")
}

func (s *server) importGatewaySerialInventoryCSV(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID   string `json:"tenant_id"`
		CSVContent string `json:"csv_content"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	records, err := parseSerialInventoryCSV(request.CSVContent)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrSerialInventoryRecordsRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusBadRequest, "invalid csv_content format")
		}
		return
	}

	items, err := s.gatewaySvc.ImportSerialInventory(tenantID, records)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrTenantIDRequired),
			errors.Is(err, gateway.ErrSerialInventoryRecordsRequired),
			errors.Is(err, gateway.ErrSerialInventoryProductTypeInvalid),
			errors.Is(err, gateway.ErrSerialNumberRequired),
			errors.Is(err, gateway.ErrSerialNumberInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, gateway.ErrSerialInventoryTenantMismatch),
			errors.Is(err, gateway.ErrSerialInventoryProductTypeMismatch):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"items": items,
	})
	s.appendAuditLog(r, tenantID, "serial_inventory_import_csv", "count="+strconv.Itoa(len(items)), "web_admin")
}

func (s *server) batchUpdateGatewaySerialInventoryStatus(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID          string   `json:"tenant_id"`
		Status            string   `json:"status"`
		SerialNumbers     []string `json:"serial_numbers"`
		ConsumedGatewayID string   `json:"consumed_gateway_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	items, err := s.gatewaySvc.BatchUpdateSerialInventoryStatus(
		tenantID,
		request.SerialNumbers,
		request.Status,
		request.ConsumedGatewayID,
	)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrTenantIDRequired),
			errors.Is(err, gateway.ErrSerialInventoryStatusInvalid),
			errors.Is(err, gateway.ErrSerialInventorySerialNumbersRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, gateway.ErrSerialInventoryNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrSerialInventoryTenantMismatch),
			errors.Is(err, gateway.ErrSerialInventoryStatusTransitionInvalid):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
	s.appendAuditLog(r, tenantID, "serial_inventory_status_batch_update", "count="+strconv.Itoa(len(items))+",status="+strings.TrimSpace(request.Status), "web_admin")
}

func (s *server) updateGatewaySerialInventoryStatus(w http.ResponseWriter, r *http.Request) {
	serialNumber := chi.URLParam(r, "serialNumber")
	var request struct {
		TenantID          string `json:"tenant_id"`
		Status            string `json:"status"`
		ConsumedGatewayID string `json:"consumed_gateway_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	item, err := s.gatewaySvc.UpdateSerialInventoryStatus(
		tenantID,
		serialNumber,
		request.Status,
		request.ConsumedGatewayID,
	)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrTenantIDRequired),
			errors.Is(err, gateway.ErrSerialNumberRequired),
			errors.Is(err, gateway.ErrSerialInventoryStatusInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, gateway.ErrSerialInventoryNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrSerialInventoryTenantMismatch),
			errors.Is(err, gateway.ErrSerialInventoryStatusTransitionInvalid):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, item)
	s.appendAuditLog(r, tenantID, "serial_inventory_status_update", item.SerialNumber+":"+item.Status, "web_admin")
}

func (s *server) exportGatewaySerialInventoryCSV(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	items, err := s.gatewaySvc.ListSerialInventory(
		tenantID,
		r.URL.Query().Get("product_type"),
		r.URL.Query().Get("status"),
	)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrSerialInventoryProductTypeInvalid),
			errors.Is(err, gateway.ErrSerialInventoryStatusInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"gateway-serial-inventory.csv\"")
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{
		"serial_number",
		"product_type",
		"status",
		"tenant_id",
		"batch_code",
		"source",
		"consumed_gateway_id",
		"consumed_at",
		"updated_at",
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range items {
		consumedAt := ""
		if items[i].ConsumedAt != nil {
			consumedAt = items[i].ConsumedAt.UTC().Format(time.RFC3339)
		}
		if err := writer.Write([]string{
			items[i].SerialNumber,
			items[i].ProductType,
			items[i].Status,
			items[i].TenantID,
			items[i].BatchCode,
			items[i].Source,
			items[i].ConsumedGatewayID,
			consumedAt,
			items[i].UpdatedAt.UTC().Format(time.RFC3339),
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func (s *server) registerGateway(w http.ResponseWriter, r *http.Request) {
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
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	request.TenantID = tenantID
	if buildingScope != nil && strings.TrimSpace(request.BuildingID) == "" {
		writeError(w, http.StatusBadRequest, "building_id is required for building_admin")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, request.BuildingID) {
		return
	}

	created, err := s.gatewaySvc.Register(request.SerialNumber, request.TenantID, request.BuildingID, request.DeviceCapacity)
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

	writeJSON(w, http.StatusCreated, created)
}

func (s *server) bindGatewayDoor(w http.ResponseWriter, r *http.Request) {
	gatewayID := chi.URLParam(r, "gatewayID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	var request struct {
		DoorID string `json:"door_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if buildingScope != nil {
		gw, exists := s.findGatewayByTenant(tenantID, gatewayID)
		if !exists {
			writeError(w, http.StatusNotFound, "gateway not found")
			return
		}
		if !s.requireBuildingScope(w, buildingScope, gw.BuildingID) {
			return
		}

		doorID := strings.TrimSpace(request.DoorID)
		if doorID != "" {
			allowedDoorIDs := allowedDoorIDsByBuildingScope(s.spaceSvc.ListDoors(tenantID), buildingScope)
			if _, exists := allowedDoorIDs[doorID]; !exists {
				writeError(w, http.StatusForbidden, "building scope forbidden")
				return
			}
		}
	}

	updated, err := s.gatewaySvc.BindDoor(tenantID, gatewayID, request.DoorID)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayIDRequired), errors.Is(err, gateway.ErrDoorIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *server) unbindGatewayDoor(w http.ResponseWriter, r *http.Request) {
	gatewayID := chi.URLParam(r, "gatewayID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	var request struct {
		DoorID string `json:"door_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if buildingScope != nil {
		gw, exists := s.findGatewayByTenant(tenantID, gatewayID)
		if !exists {
			writeError(w, http.StatusNotFound, "gateway not found")
			return
		}
		if !s.requireBuildingScope(w, buildingScope, gw.BuildingID) {
			return
		}

		doorID := strings.TrimSpace(request.DoorID)
		if doorID != "" {
			allowedDoorIDs := allowedDoorIDsByBuildingScope(s.spaceSvc.ListDoors(tenantID), buildingScope)
			if _, exists := allowedDoorIDs[doorID]; !exists {
				writeError(w, http.StatusForbidden, "building scope forbidden")
				return
			}
		}
	}

	updated, err := s.gatewaySvc.UnbindDoor(tenantID, gatewayID, request.DoorID)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayIDRequired), errors.Is(err, gateway.ErrDoorIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *server) registerGatewayDevice(w http.ResponseWriter, r *http.Request) {
	gatewayID := chi.URLParam(r, "gatewayID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	var request struct {
		SerialNumber string               `json:"serial_number"`
		Kind         string               `json:"kind"`
		Source       string               `json:"source"`
		Protocol     string               `json:"protocol"`
		RS485Config  *gateway.RS485Config `json:"rs485_config"`
		Status       string               `json:"status"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if buildingScope != nil {
		gw, exists := s.findGatewayByTenant(tenantID, gatewayID)
		if !exists {
			writeError(w, http.StatusNotFound, "gateway not found")
			return
		}
		if !s.requireBuildingScope(w, buildingScope, gw.BuildingID) {
			return
		}
	}

	updated, err := s.gatewaySvc.RegisterDevice(
		tenantID,
		gatewayID,
		request.SerialNumber,
		request.Kind,
		request.Source,
		request.Status,
		request.Protocol,
		request.RS485Config,
	)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayIDRequired),
			errors.Is(err, gateway.ErrGatewayDeviceSerialRequired),
			errors.Is(err, gateway.ErrGatewayDeviceCapacityExceeded),
			errors.Is(err, gateway.ErrGatewayDeviceKindInvalid),
			errors.Is(err, gateway.ErrGatewayDeviceSourceInvalid),
			errors.Is(err, gateway.ErrGatewayDeviceProtocolInvalid),
			errors.Is(err, gateway.ErrGatewayDeviceRS485ConfigInvalid),
			errors.Is(err, gateway.ErrGatewayDeviceRS485ConfigProtocolMismatch),
			errors.Is(err, gateway.ErrGatewayDeviceStatusInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, gateway.ErrGatewayDeviceSerialConflict),
			errors.Is(err, gateway.ErrGatewayDeviceSerialNotProvisioned),
			errors.Is(err, gateway.ErrGatewayDeviceSerialNotAvailable),
			errors.Is(err, gateway.ErrGatewayDeviceSerialProductTypeMismatch),
			errors.Is(err, gateway.ErrSerialNumberTenantMismatch):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *server) reportGatewayDeviceRS485Telemetry(w http.ResponseWriter, r *http.Request) {
	gatewayID := chi.URLParam(r, "gatewayID")
	deviceID := chi.URLParam(r, "deviceID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if buildingScope != nil {
		gw, exists := s.findGatewayByTenant(tenantID, gatewayID)
		if !exists {
			writeError(w, http.StatusNotFound, "gateway not found")
			return
		}
		if !s.requireBuildingScope(w, buildingScope, gw.BuildingID) {
			return
		}
	}

	var request gateway.RS485TelemetryReport
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	device, alerted, err := s.gatewaySvc.ReportRS485Telemetry(tenantID, gatewayID, deviceID, request)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound),
			errors.Is(err, gateway.ErrGatewayDeviceNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayIDRequired),
			errors.Is(err, gateway.ErrGatewayDeviceIDRequired),
			errors.Is(err, gateway.ErrGatewayDeviceRS485TelemetryInvalid),
			errors.Is(err, gateway.ErrGatewayDeviceRS485TelemetryProtocolMismatch):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	telemetrySummary := s.gatewaySvc.BuildDeviceTelemetrySummary(device)
	if alerted && device.RS485Health != nil {
		auditTenantID := strings.TrimSpace(tenantID)
		if auditTenantID == "" {
			if gw, exists := s.findGatewayByTenant("", gatewayID); exists {
				auditTenantID = strings.TrimSpace(gw.TenantID)
			}
		}
		target := fmt.Sprintf(
			"gateway=%s device=%s protocol=%s retry=%d timeout=%d collision=%d consecutive_timeout=%d alert_level=%s line_quality=%s governance_action=%s reason_codes=%s last_error=%s",
			strings.TrimSpace(gatewayID),
			strings.TrimSpace(device.ID),
			telemetrySummary.Protocol,
			device.RS485Health.RetryCount,
			device.RS485Health.TimeoutCount,
			device.RS485Health.CollisionCount,
			device.RS485Health.ConsecutiveTimeouts,
			telemetrySummary.AlertLevel,
			telemetrySummary.LineQuality,
			telemetrySummary.GovernanceAction,
			strings.Join(telemetrySummary.ReasonCodes, ","),
			strings.TrimSpace(device.RS485Health.LastError),
		)
		s.appendAuditLog(r, auditTenantID, "gateway_rs485_health_alert", target, "gateway_rs485")
		s.appendAuditLog(r, auditTenantID, "gateway_protocol_health_alert", target, "gateway_protocol_health")
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"gateway_id": gatewayID,
		"device_id":  device.ID,
		"device":     device,
		"alerted":    alerted,
		"telemetry":  telemetrySummary,
	})
}

func (s *server) probeGatewayLegacyDevices(w http.ResponseWriter, r *http.Request) {
	gatewayID := chi.URLParam(r, "gatewayID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if buildingScope != nil {
		gw, exists := s.findGatewayByTenant(tenantID, gatewayID)
		if !exists {
			writeError(w, http.StatusNotFound, "gateway not found")
			return
		}
		if !s.requireBuildingScope(w, buildingScope, gw.BuildingID) {
			return
		}
	}

	plan, err := s.gatewaySvc.ProbeLegacyDevicePlan(tenantID, gatewayID)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":      plan.Items,
		"governance": plan.Governance,
	})
}

func (s *server) publishGatewayConfig(w http.ResponseWriter, r *http.Request) {
	gatewayID := chi.URLParam(r, "gatewayID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	var request struct {
		Version string `json:"version"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if buildingScope != nil {
		gw, exists := s.findGatewayByTenant(tenantID, gatewayID)
		if !exists {
			writeError(w, http.StatusNotFound, "gateway not found")
			return
		}
		if !s.requireBuildingScope(w, buildingScope, gw.BuildingID) {
			return
		}
	}

	ack, err := s.gatewaySvc.PublishConfig(tenantID, gatewayID, request.Version)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayIDRequired), errors.Is(err, gateway.ErrConfigVersionRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusAccepted, ack)
}

func (s *server) rebootGateway(w http.ResponseWriter, r *http.Request) {
	gatewayID := chi.URLParam(r, "gatewayID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if buildingScope != nil {
		gw, exists := s.findGatewayByTenant(tenantID, gatewayID)
		if !exists {
			writeError(w, http.StatusNotFound, "gateway not found")
			return
		}
		if !s.requireBuildingScope(w, buildingScope, gw.BuildingID) {
			return
		}
	}
	ack, err := s.gatewaySvc.Reboot(tenantID, gatewayID)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusAccepted, ack)
}
