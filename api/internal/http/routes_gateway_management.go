package httpx

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"sort"
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

func (s *server) updateGatewayStatus(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	gatewayID := chi.URLParam(r, "gatewayID")
	var request struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	record, found := s.findGatewayByTenant(tenantID, gatewayID)
	if !found {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !s.requireBuildingScope(w, buildingScope, record.BuildingID) {
		return
	}

	item, err := s.gatewaySvc.UpdateGatewayStatus(tenantID, gatewayID, request.Status)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayIDRequired),
			errors.Is(err, gateway.ErrGatewayStatusRequired),
			errors.Is(err, gateway.ErrGatewayStatusInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, "gateway_status_updated", item.ID+":"+item.Status, "gateway")
	if gatewayCertificateStatusRevoked(item.Status) {
		s.closeGatewayWebSocketSessions(item.ID, "gateway disabled; reconnect denied")
		s.publishGatewayWebSocketDisconnect(item.TenantID, item.ID, "")
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *server) listGatewayCertificateRevocations(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	items := []gateway.GatewayCertificateRevocation{}
	if s.gatewaySvc != nil {
		runtimeItems := s.gatewaySvc.ListCertificateRevocations(tenantID)
		items = append(items, runtimeItems...)
	}
	s.syncGatewayCertificateRevocationMetrics()

	user, _ := authenticatedUser(r)
	if strings.EqualFold(strings.TrimSpace(user.Role), "super_admin") {
		seen := make(map[string]struct{}, len(items))
		for i := range items {
			seen[items[i].SerialNumber] = struct{}{}
		}
		for i := range s.cfg.GatewayMTLSRevokedSerials {
			serial := gateway.NormalizeCertificateSerial(s.cfg.GatewayMTLSRevokedSerials[i])
			if serial == "" {
				continue
			}
			if _, exists := seen[serial]; exists {
				continue
			}
			items = append(items, gateway.GatewayCertificateRevocation{
				ID:           "gw_cert_env_" + serial,
				SerialNumber: serial,
				Source:       "environment",
			})
			seen[serial] = struct{}{}
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		leftTime := items[i].RevokedAt
		rightTime := items[j].RevokedAt
		if leftTime != nil && rightTime != nil && !leftTime.Equal(*rightTime) {
			return leftTime.After(*rightTime)
		}
		if leftTime != nil && rightTime == nil {
			return true
		}
		if leftTime == nil && rightTime != nil {
			return false
		}
		return items[i].SerialNumber < items[j].SerialNumber
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) revokeGatewayCertificateSerial(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID     string `json:"tenant_id"`
		GatewayID    string `json:"gateway_id"`
		SerialNumber string `json:"serial_number"`
		Reason       string `json:"reason"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, firstNonEmptyString(request.TenantID, r.URL.Query().Get("tenant_id")))
	if !ok {
		return
	}
	if strings.TrimSpace(request.GatewayID) != "" {
		record, found := s.findGatewayByTenant(tenantID, request.GatewayID)
		if !found {
			writeError(w, http.StatusNotFound, "gateway not found")
			return
		}
		buildingScope, ok := s.buildingScopeForRequest(w, r)
		if !ok {
			return
		}
		if !s.requireBuildingScope(w, buildingScope, record.BuildingID) {
			return
		}
	}
	if strings.TrimSpace(request.GatewayID) == "" && !requestCanRevokeGlobalGatewayCertificateSerial(r) {
		writeError(w, http.StatusBadRequest, "gateway_id is required for tenant-scoped certificate revocation")
		return
	}

	item, err := s.gatewaySvc.RevokeCertificateSerial(tenantID, request.GatewayID, request.SerialNumber, request.Reason, gatewayCertificateRevocationActor(r))
	if err != nil {
		handleGatewayCertificateRevocationError(w, err)
		return
	}
	s.syncGatewayCertificateRevocationMetrics()
	if strings.TrimSpace(item.GatewayID) != "" {
		s.closeGatewayWebSocketSessions(item.GatewayID, "certificate serial revoked; reconnect denied")
	}
	s.closeGatewayWebSocketCertificateSerial(item.SerialNumber, "certificate serial revoked; reconnect denied")
	s.publishGatewayWebSocketDisconnect(item.TenantID, item.GatewayID, item.SerialNumber)

	auditTenantID := item.TenantID
	if auditTenantID == "" {
		auditTenantID = tenantID
	}
	s.appendAuditLog(r, auditTenantID, "gateway_mtls_cert_serial_revoked", fmt.Sprintf("serial=%s,gateway=%s,source=%s", item.SerialNumber, item.GatewayID, item.Source), "gateway")
	writeJSON(w, http.StatusOK, item)
}

func (s *server) restoreGatewayCertificateSerial(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID string `json:"tenant_id"`
	}
	_ = decodeJSON(r, &request)

	tenantID, ok := s.resolveTenantID(w, r, firstNonEmptyString(request.TenantID, r.URL.Query().Get("tenant_id")))
	if !ok {
		return
	}

	serial := chi.URLParam(r, "serialNumber")
	if gatewaySerialConfiguredInEnvironment(s.cfg.GatewayMTLSRevokedSerials, serial) {
		writeError(w, http.StatusConflict, "certificate serial is configured in GATEWAY_MTLS_REVOKED_SERIALS")
		return
	}

	item, err := s.gatewaySvc.RestoreCertificateSerial(tenantID, serial)
	if err != nil {
		handleGatewayCertificateRevocationError(w, err)
		return
	}
	s.syncGatewayCertificateRevocationMetrics()

	auditTenantID := item.TenantID
	if auditTenantID == "" {
		auditTenantID = tenantID
	}
	s.appendAuditLog(r, auditTenantID, "gateway_mtls_cert_serial_restored", fmt.Sprintf("serial=%s,gateway=%s", item.SerialNumber, item.GatewayID), "gateway")
	writeJSON(w, http.StatusOK, item)
}

func gatewaySerialConfiguredInEnvironment(items []string, serialNumber string) bool {
	serial := gateway.NormalizeCertificateSerial(serialNumber)
	if serial == "" {
		return false
	}
	for i := range items {
		if gateway.NormalizeCertificateSerial(items[i]) == serial {
			return true
		}
	}
	return false
}

func (s *server) syncGatewayCertificateRevocationMetrics() {
	runtimeCount := 0
	envSerials := []string(nil)
	if s != nil && s.gatewaySvc != nil {
		runtimeCount = len(s.gatewaySvc.ListCertificateRevocations(""))
	}
	if s != nil {
		envSerials = s.cfg.GatewayMTLSRevokedSerials
	}
	setGatewayCertificateRevocationMetric("runtime", runtimeCount)
	setGatewayCertificateRevocationMetric("environment", countConfiguredGatewayCertificateSerials(envSerials))
}

func countConfiguredGatewayCertificateSerials(items []string) int {
	seen := map[string]struct{}{}
	for i := range items {
		serial := gateway.NormalizeCertificateSerial(items[i])
		if serial == "" {
			continue
		}
		seen[serial] = struct{}{}
	}
	return len(seen)
}

func gatewayCertificateRevocationActor(r *http.Request) string {
	if r == nil {
		return "system"
	}
	user, ok := authenticatedUser(r)
	if !ok {
		return "system"
	}
	if strings.TrimSpace(user.Email) != "" {
		return strings.TrimSpace(user.Email)
	}
	return strings.TrimSpace(user.ID)
}

func requestCanRevokeGlobalGatewayCertificateSerial(r *http.Request) bool {
	user, ok := authenticatedUser(r)
	if !ok {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(user.Role), "super_admin")
}

func handleGatewayCertificateRevocationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gateway.ErrGatewayNotFound),
		errors.Is(err, gateway.ErrGatewayCertificateSerialNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, gateway.ErrGatewayCertificateSerialConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, gateway.ErrGatewayCertificateSerialRequired),
		errors.Is(err, gateway.ErrGatewayCertificateSerialInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
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
	s.appendAuditLog(r, tenantID, "gateway_registered", fmt.Sprintf("gateway_id=%s,place_id=%s,serial_number=%s", created.ID, created.BuildingID, created.SerialNumber), "gateway")
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
	s.appendAuditLog(r, tenantID, "gateway_door_bound", fmt.Sprintf("gateway_id=%s,door_id=%s,place_id=%s", updated.ID, strings.TrimSpace(request.DoorID), updated.BuildingID), "gateway")
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
	s.appendAuditLog(r, tenantID, "gateway_door_unbound", fmt.Sprintf("gateway_id=%s,door_id=%s,place_id=%s", updated.ID, strings.TrimSpace(request.DoorID), updated.BuildingID), "gateway")
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
	if device, exists := gatewayDeviceBySerial(updated, request.SerialNumber); exists {
		s.appendAuditLog(r, tenantID, "gateway_device_registered", fmt.Sprintf("gateway_id=%s,device_id=%s,serial_number=%s,kind=%s,protocol=%s,status=%s", updated.ID, device.ID, device.SerialNumber, device.Kind, device.Protocol, device.Status), "gateway")
	}
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
	s.appendAuditLog(r, tenantID, "gateway_config_published", fmt.Sprintf("gateway_id=%s,version=%s,task_id=%s", ack.GatewayID, strings.TrimSpace(request.Version), ack.TaskID), "gateway")
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
	s.appendAuditLog(r, tenantID, "gateway_reboot_queued", fmt.Sprintf("gateway_id=%s,task_id=%s", ack.GatewayID, ack.TaskID), "gateway")
}

func (s *server) getGatewayMQTTBootstrap(w http.ResponseWriter, r *http.Request) {
	gatewayID := chi.URLParam(r, "gatewayID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !s.cfg.MQTTEnabled {
		writeError(w, http.StatusServiceUnavailable, "mqtt integration is not enabled")
		return
	}

	gw, exists := s.findGatewayByTenant(tenantID, gatewayID)
	if !exists {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, gw.BuildingID) {
		return
	}

	topicPrefix := strings.Trim(strings.TrimSpace(s.cfg.MQTTTopicPrefix), "/")
	if topicPrefix == "" {
		topicPrefix = "mistypass"
	}
	tenantTopicSegment := mqttTopicSegment(gw.TenantID)
	gatewayTopicSegment := mqttTopicSegment(gw.ID)
	baseTopic := fmt.Sprintf("%s/tenants/%s/gateways/%s", topicPrefix, tenantTopicSegment, gatewayTopicSegment)

	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":      true,
		"tenant_id":    gw.TenantID,
		"gateway_id":   gw.ID,
		"broker_url":   s.cfg.MQTTBrokerURL,
		"client_id":    fmt.Sprintf("gw-%s", gatewayTopicSegment),
		"username":     gw.ID,
		"auth_mode":    "gateway_device_token",
		"topic_prefix": topicPrefix,
		"topics": map[string]any{
			"subscribe": map[string]string{
				"config":  baseTopic + "/commands/config",
				"reboot":  baseTopic + "/commands/reboot",
				"control": baseTopic + "/commands/control",
			},
			"publish": map[string]string{
				"heartbeat":  baseTopic + "/events/heartbeat",
				"access":     baseTopic + "/events/access",
				"device":     baseTopic + "/events/device",
				"checkpoint": baseTopic + "/events/checkpoint",
				"status":     baseTopic + "/status",
			},
		},
		"qos": map[string]int{
			"commands": 1,
			"events":   1,
			"status":   0,
		},
	})
}

func (s *server) createGatewayOTATask(w http.ResponseWriter, r *http.Request) {
	gatewayID := chi.URLParam(r, "gatewayID")
	var request struct {
		TenantID          string `json:"tenant_id"`
		FirmwareVersion   string `json:"firmware_version"`
		FirmwareURL       string `json:"firmware_url"`
		FirmwareSHA256    string `json:"firmware_sha256"`
		FirmwareSignature string `json:"firmware_signature"`
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
	gw, exists := s.findGatewayByTenant(tenantID, gatewayID)
	if !exists {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, gw.BuildingID) {
		return
	}

	task, err := s.gatewaySvc.CreateOTATask(
		tenantID,
		gatewayID,
		request.FirmwareVersion,
		request.FirmwareURL,
		request.FirmwareSHA256,
		request.FirmwareSignature,
		requestActor(r),
	)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayIDRequired),
			errors.Is(err, gateway.ErrGatewayOTAFirmwareVersionRequired),
			errors.Is(err, gateway.ErrGatewayOTAFirmwareURLRequired),
			errors.Is(err, gateway.ErrGatewayOTAFirmwareSHA256Invalid),
			errors.Is(err, gateway.ErrGatewayOTAFirmwareSignatureInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, task)
	s.appendAuditLog(
		r,
		task.TenantID,
		"gateway_ota_task_created",
		fmt.Sprintf("gateway=%s,task=%s,firmware=%s,status=%s", task.GatewayID, task.ID, task.FirmwareVersion, task.Status),
		"gateway_ota",
	)
}

func (s *server) listGatewayOTATasks(w http.ResponseWriter, r *http.Request) {
	gatewayID := chi.URLParam(r, "gatewayID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	gw, exists := s.findGatewayByTenant(tenantID, gatewayID)
	if !exists {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, gw.BuildingID) {
		return
	}

	items, err := s.gatewaySvc.ListOTATasks(tenantID, gatewayID)
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
		"items": items,
	})
}

func (s *server) updateGatewayOTATaskStatus(w http.ResponseWriter, r *http.Request) {
	gatewayID := chi.URLParam(r, "gatewayID")
	taskID := chi.URLParam(r, "taskID")
	var request struct {
		TenantID     string `json:"tenant_id"`
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
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
	gw, exists := s.findGatewayByTenant(tenantID, gatewayID)
	if !exists {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, gw.BuildingID) {
		return
	}

	item, err := s.gatewaySvc.UpdateOTATaskStatus(
		tenantID,
		gatewayID,
		taskID,
		request.Status,
		request.ErrorMessage,
		requestActor(r),
	)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound),
			errors.Is(err, gateway.ErrGatewayOTATaskNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayIDRequired),
			errors.Is(err, gateway.ErrGatewayOTATaskIDRequired),
			errors.Is(err, gateway.ErrGatewayOTATaskStatusInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, item)
	s.appendAuditLog(
		r,
		item.TenantID,
		"gateway_ota_task_status_updated",
		fmt.Sprintf("gateway=%s,task=%s,status=%s,error=%s", item.GatewayID, item.ID, item.Status, strings.TrimSpace(item.ErrorMessage)),
		"gateway_ota",
	)
}

func requestActor(r *http.Request) string {
	user, ok := authenticatedUser(r)
	if !ok {
		return "system"
	}
	if nextEmail := strings.TrimSpace(user.Email); nextEmail != "" {
		return nextEmail
	}
	if nextUserID := strings.TrimSpace(user.ID); nextUserID != "" {
		return nextUserID
	}
	return "system"
}

func mqttTopicSegment(raw string) string {
	next := strings.ToLower(strings.TrimSpace(raw))
	if next == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(next))
	lastDash := false
	for _, ch := range next {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			b.WriteRune(ch)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "unknown"
	}
	return result
}
