package httpx

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/event"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
	"github.com/mistypass/cloud/api/internal/modules/space"
)

const gatewayEventsBatchMaxItems = 200

const (
	gatewayConfigAuthzCacheTTLSeconds          = 300
	gatewayConfigAuthzCacheMaxStaleSeconds     = 900
	gatewayConfigAuthzCacheRefreshRetrySeconds = 30
)

func (s *server) gatewayBootstrapConfigPull(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID      string `json:"gateway_id"`
		TenantID       string `json:"tenant_id"`
		CurrentVersion string `json:"current_version"`
		AuthzVersion   string `json:"authz_cache_version"`
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
	if !s.authorizeGatewayHTTPDeviceRequest(w, r, record.ID) {
		return
	}

	snapshot, err := s.gatewaySvc.PullConfig(request.TenantID, request.GatewayID)
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

	currentVersion := strings.TrimSpace(request.CurrentVersion)
	desiredVersion := strings.TrimSpace(snapshot.DesiredVersion)
	fetchedAt := time.Now().UTC()
	authzCache := s.buildGatewayConfigAuthzCache(request.TenantID, snapshot.GatewayID, snapshot.BoundDoorIDs, fetchedAt)
	reportedAuthzVersion := strings.TrimSpace(request.AuthzVersion)
	authzCache.VersionReported = reportedAuthzVersion
	authzCache.Status = gatewayConfigAuthzResolveStatus(
		authzCache.StatusCodes,
		reportedAuthzVersion,
		authzCache.Version,
		authzCache.Policy.RollbackVersion,
	)

	// Include pending OTA tasks so the gateway can discover firmware updates.
	var pendingOTA []gateway.GatewayOTATask
	if allOTA, otaErr := s.gatewaySvc.ListOTATasks(request.TenantID, request.GatewayID); otaErr == nil {
		for _, task := range allOTA {
			if task.Status == "queued" || task.Status == "dispatching" {
				pendingOTA = append(pendingOTA, task)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"gateway_id":        snapshot.GatewayID,
		"tenant_id":         snapshot.TenantID,
		"current_version":   currentVersion,
		"desired_version":   desiredVersion,
		"should_apply":      desiredVersion != "" && desiredVersion != currentVersion,
		"desired_at":        snapshot.DesiredUpdatedAt,
		"applied_version":   snapshot.AppliedVersion,
		"applied_at":        snapshot.AppliedAt,
		"bound_door_ids":    snapshot.BoundDoorIDs,
		"devices":           snapshot.Devices,
		"authz_cache":       authzCache,
		"pending_ota_tasks": pendingOTA,
		"fetched_at":        fetchedAt,
	})
}

func (s *server) gatewayBootstrapConfigApplied(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID         string `json:"gateway_id"`
		TenantID          string `json:"tenant_id"`
		Version           string `json:"version"`
		AuthzCacheVersion string `json:"authz_cache_version"`
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
	if !s.authorizeGatewayHTTPDeviceRequest(w, r, record.ID) {
		return
	}

	snapshot, err := s.gatewaySvc.MarkConfigApplied(request.TenantID, request.GatewayID, request.Version)
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

	ackedAt := time.Now().UTC()
	expectedAuthzCache := s.buildGatewayConfigAuthzCache(snapshot.TenantID, snapshot.GatewayID, snapshot.BoundDoorIDs, ackedAt)
	reportedAuthzCacheVersion := strings.TrimSpace(request.AuthzCacheVersion)
	authzCacheVersionMatch := reportedAuthzCacheVersion == "" || reportedAuthzCacheVersion == expectedAuthzCache.Version
	authzCacheStatus := gatewayConfigAuthzResolveStatus(
		expectedAuthzCache.StatusCodes,
		reportedAuthzCacheVersion,
		expectedAuthzCache.Version,
		expectedAuthzCache.Policy.RollbackVersion,
	)
	if reportedAuthzCacheVersion != "" && authzCacheVersionMatch {
		s.setGatewayAuthzCacheAckVersion(snapshot.GatewayID, reportedAuthzCacheVersion)
	}
	if reportedAuthzCacheVersion != "" && !authzCacheVersionMatch {
		s.appendAuditLog(
			r,
			snapshot.TenantID,
			"gateway_config_authz_cache_version_drift",
			fmt.Sprintf(
				"gateway=%s applied=%s reported=%s expected=%s",
				snapshot.GatewayID,
				strings.TrimSpace(snapshot.AppliedVersion),
				reportedAuthzCacheVersion,
				expectedAuthzCache.Version,
			),
			"gateway_config",
		)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"gateway_id":      snapshot.GatewayID,
		"tenant_id":       snapshot.TenantID,
		"desired_version": snapshot.DesiredVersion,
		"applied_version": snapshot.AppliedVersion,
		"in_sync": strings.TrimSpace(snapshot.DesiredVersion) == "" ||
			strings.TrimSpace(snapshot.DesiredVersion) == strings.TrimSpace(snapshot.AppliedVersion),
		"desired_at": snapshot.DesiredUpdatedAt,
		"applied_at": snapshot.AppliedAt,
		"authz_cache": map[string]any{
			"version_reported": reportedAuthzCacheVersion,
			"version_expected": expectedAuthzCache.Version,
			"version_match":    authzCacheVersionMatch,
			"status":           authzCacheStatus,
			"generated_at":     expectedAuthzCache.GeneratedAt,
			"expires_at":       expectedAuthzCache.ExpiresAt,
			"ttl_seconds":      expectedAuthzCache.TTLSeconds,
			"policy":           expectedAuthzCache.Policy,
			"status_codes":     expectedAuthzCache.StatusCodes,
		},
		"acked_at": ackedAt,
	})
}

func (s *server) gatewayBootstrapAccessEvent(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID      string `json:"gateway_id"`
		TenantID       string `json:"tenant_id"`
		EventID        string `json:"event_id"`
		RequestID      string `json:"request_id"`
		BuildingID     string `json:"building_id"`
		AreaID         string `json:"area_id"`
		Type           string `json:"type"`
		Actor          string `json:"actor"`
		DoorID         string `json:"door_id"`
		Result         string `json:"result"`
		OccurredAt     string `json:"occurred_at"`
		IdempotencyKey string `json:"idempotency_key"`
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
	if !s.authorizeGatewayHTTPDeviceRequest(w, r, record.ID) {
		return
	}

	idempotencyKey := strings.TrimSpace(request.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(request.RequestID)
	}
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(request.EventID)
	}

	eventType := strings.TrimSpace(request.Type)
	if eventType == "" {
		eventType = "access_event"
	}
	result := strings.TrimSpace(request.Result)
	if result == "" {
		result = "accepted"
	}
	buildingID := strings.TrimSpace(request.BuildingID)
	if buildingID == "" {
		buildingID = strings.TrimSpace(record.BuildingID)
	}

	occurredAt := time.Now().UTC()
	if raw := strings.TrimSpace(request.OccurredAt); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "occurred_at must be RFC3339")
			return
		}
		occurredAt = parsed.UTC()
	}

	saved, deduped, err := s.eventSvc.IngestAccessEvent(event.IngestAccessEventInput{
		ID:             request.EventID,
		IdempotencyKey: idempotencyKey,
		TenantID:       record.TenantID,
		BuildingID:     buildingID,
		AreaID:         request.AreaID,
		Type:           eventType,
		Actor:          request.Actor,
		DoorID:         request.DoorID,
		GatewayID:      record.ID,
		Result:         result,
		At:             occurredAt,
	})
	if err != nil {
		switch {
		case errors.Is(err, event.ErrTenantIDRequired),
			errors.Is(err, event.ErrGatewayIDRequired),
			errors.Is(err, event.ErrAccessEventTypeRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	var queueProgress gateway.GatewayQueueIngestTotal
	if deduped {
		queueProgress, err = s.gatewaySvc.GetQueueIngestTotal(record.TenantID, record.ID, "default")
		if err != nil {
			if errors.Is(err, gateway.ErrGatewayQueueIngestTotalNotFound) {
				accessTotal, deviceTotal := s.eventSvc.CountEventsByGateway(record.TenantID, record.ID)
				queueProgress = gateway.GatewayQueueIngestTotal{
					GatewayID:     record.ID,
					Queue:         "default",
					IngestedTotal: accessTotal + deviceTotal,
					UpdatedAt:     time.Now().UTC(),
				}
			} else {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	} else {
		queueProgress, err = s.gatewaySvc.AddQueueIngestTotal(record.TenantID, record.ID, "default", 1)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	s.appendAuditLog(
		r,
		record.TenantID,
		gatewayAccessAuditAction(saved.Result),
		gatewayAccessEventAuditTarget(
			record.ID,
			"default",
			saved.ID,
			saved.Type,
			saved.Result,
			saved.DoorID,
			saved.Actor,
			saved.IdempotencyKey,
			deduped,
			saved.At,
		),
		"gateway_access_event",
	)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"event_id":        saved.ID,
		"status":          "accepted",
		"deduplicated":    deduped,
		"idempotency_key": saved.IdempotencyKey,
		"queue_progress": map[string]any{
			"queue":          queueProgress.Queue,
			"ingested_total": queueProgress.IngestedTotal,
			"updated_at":     queueProgress.UpdatedAt,
		},
		"received_at": time.Now().UTC(),
		"occurred_at": saved.At,
	})
}

func (s *server) gatewayBootstrapDeviceEvent(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID      string `json:"gateway_id"`
		TenantID       string `json:"tenant_id"`
		EventID        string `json:"event_id"`
		RequestID      string `json:"request_id"`
		BuildingID     string `json:"building_id"`
		Type           string `json:"type"`
		Detail         string `json:"detail"`
		Result         string `json:"result"`
		OccurredAt     string `json:"occurred_at"`
		IdempotencyKey string `json:"idempotency_key"`
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
	if !s.authorizeGatewayHTTPDeviceRequest(w, r, record.ID) {
		return
	}

	idempotencyKey := strings.TrimSpace(request.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(request.RequestID)
	}
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(request.EventID)
	}

	eventType := strings.TrimSpace(request.Type)
	if eventType == "" {
		eventType = "gateway_event"
	}
	result := strings.TrimSpace(request.Result)
	if result == "" {
		result = "accepted"
	}
	buildingID := strings.TrimSpace(request.BuildingID)
	if buildingID == "" {
		buildingID = strings.TrimSpace(record.BuildingID)
	}

	occurredAt := time.Now().UTC()
	if raw := strings.TrimSpace(request.OccurredAt); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "occurred_at must be RFC3339")
			return
		}
		occurredAt = parsed.UTC()
	}

	saved, deduped, err := s.eventSvc.IngestDeviceEvent(event.IngestDeviceEventInput{
		ID:             request.EventID,
		IdempotencyKey: idempotencyKey,
		TenantID:       record.TenantID,
		BuildingID:     buildingID,
		Type:           eventType,
		GatewayID:      record.ID,
		Detail:         request.Detail,
		Result:         result,
		At:             occurredAt,
	})
	if err != nil {
		switch {
		case errors.Is(err, event.ErrTenantIDRequired),
			errors.Is(err, event.ErrGatewayIDRequired),
			errors.Is(err, event.ErrDeviceEventTypeRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	var queueProgress gateway.GatewayQueueIngestTotal
	if deduped {
		queueProgress, err = s.gatewaySvc.GetQueueIngestTotal(record.TenantID, record.ID, "default")
		if err != nil {
			if errors.Is(err, gateway.ErrGatewayQueueIngestTotalNotFound) {
				accessTotal, deviceTotal := s.eventSvc.CountEventsByGateway(record.TenantID, record.ID)
				queueProgress = gateway.GatewayQueueIngestTotal{
					GatewayID:     record.ID,
					Queue:         "default",
					IngestedTotal: accessTotal + deviceTotal,
					UpdatedAt:     time.Now().UTC(),
				}
			} else {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	} else {
		queueProgress, err = s.gatewaySvc.AddQueueIngestTotal(record.TenantID, record.ID, "default", 1)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	s.appendAuditLog(
		r,
		record.TenantID,
		gatewayDeviceAuditAction(saved.Type),
		gatewayDeviceEventAuditTarget(
			record.ID,
			"default",
			saved.ID,
			saved.Type,
			saved.Result,
			saved.Detail,
			saved.IdempotencyKey,
			deduped,
			saved.At,
		),
		"gateway_device_event",
	)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"event_id":        saved.ID,
		"status":          "accepted",
		"deduplicated":    deduped,
		"idempotency_key": saved.IdempotencyKey,
		"queue_progress": map[string]any{
			"queue":          queueProgress.Queue,
			"ingested_total": queueProgress.IngestedTotal,
			"updated_at":     queueProgress.UpdatedAt,
		},
		"received_at": time.Now().UTC(),
		"occurred_at": saved.At,
	})
}

func (s *server) gatewayBootstrapEventsBatch(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID    string `json:"gateway_id"`
		TenantID     string `json:"tenant_id"`
		Queue        string `json:"queue"`
		AccessEvents []struct {
			EventID        string `json:"event_id"`
			RequestID      string `json:"request_id"`
			IdempotencyKey string `json:"idempotency_key"`
			BuildingID     string `json:"building_id"`
			AreaID         string `json:"area_id"`
			Type           string `json:"type"`
			Actor          string `json:"actor"`
			DoorID         string `json:"door_id"`
			Result         string `json:"result"`
			OccurredAt     string `json:"occurred_at"`
		} `json:"access_events"`
		DeviceEvents []struct {
			EventID        string `json:"event_id"`
			RequestID      string `json:"request_id"`
			IdempotencyKey string `json:"idempotency_key"`
			BuildingID     string `json:"building_id"`
			Type           string `json:"type"`
			Detail         string `json:"detail"`
			Result         string `json:"result"`
			OccurredAt     string `json:"occurred_at"`
		} `json:"device_events"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(request.AccessEvents) == 0 && len(request.DeviceEvents) == 0 {
		writeError(w, http.StatusBadRequest, "batch events are required")
		return
	}
	totalItems := len(request.AccessEvents) + len(request.DeviceEvents)
	if totalItems > gatewayEventsBatchMaxItems {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("batch size exceeded: max %d", gatewayEventsBatchMaxItems))
		return
	}

	record, ok := s.findGatewayByTenant(request.TenantID, request.GatewayID)
	if !ok {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if !s.authorizeGatewayHTTPDeviceRequest(w, r, record.ID) {
		return
	}
	queue := normalizeGatewayCheckpointQueue(request.Queue)
	receivedAt := time.Now().UTC()

	accessCreated := 0
	accessDeduplicated := 0
	accessFailed := 0
	accessRetryable := 0
	accessIDs := make([]string, 0, len(request.AccessEvents))
	accessResults := make([]map[string]any, 0, len(request.AccessEvents))
	retryAccessEvents := make([]map[string]any, 0, len(request.AccessEvents))
	for i := range request.AccessEvents {
		occurredAt, err := parseGatewayOccurredAt(request.AccessEvents[i].OccurredAt)
		if err != nil {
			accessFailed++
			accessResults = append(accessResults, map[string]any{
				"index":     i,
				"event_id":  strings.TrimSpace(request.AccessEvents[i].EventID),
				"status":    "failed",
				"error":     "occurred_at must be RFC3339",
				"retryable": false,
			})
			continue
		}
		if err := s.gatewayBatchForcedRetryableError(request.AccessEvents[i].EventID); err != nil {
			retryable := isGatewayBatchFailureRetryable(err)
			accessFailed++
			accessResults = append(accessResults, map[string]any{
				"index":     i,
				"event_id":  strings.TrimSpace(request.AccessEvents[i].EventID),
				"status":    "failed",
				"error":     err.Error(),
				"retryable": retryable,
			})
			if retryable {
				accessRetryable++
				retryAccessEvents = append(retryAccessEvents, map[string]any{
					"event_id":        request.AccessEvents[i].EventID,
					"request_id":      request.AccessEvents[i].RequestID,
					"idempotency_key": request.AccessEvents[i].IdempotencyKey,
					"building_id":     request.AccessEvents[i].BuildingID,
					"area_id":         request.AccessEvents[i].AreaID,
					"type":            request.AccessEvents[i].Type,
					"actor":           request.AccessEvents[i].Actor,
					"door_id":         request.AccessEvents[i].DoorID,
					"result":          request.AccessEvents[i].Result,
					"occurred_at":     request.AccessEvents[i].OccurredAt,
				})
			}
			continue
		}
		idempotencyKey := strings.TrimSpace(request.AccessEvents[i].IdempotencyKey)
		if idempotencyKey == "" {
			idempotencyKey = strings.TrimSpace(request.AccessEvents[i].RequestID)
		}
		if idempotencyKey == "" {
			idempotencyKey = strings.TrimSpace(request.AccessEvents[i].EventID)
		}
		eventType := strings.TrimSpace(request.AccessEvents[i].Type)
		if eventType == "" {
			eventType = "access_event"
		}
		result := strings.TrimSpace(request.AccessEvents[i].Result)
		if result == "" {
			result = "accepted"
		}
		buildingID := strings.TrimSpace(request.AccessEvents[i].BuildingID)
		if buildingID == "" {
			buildingID = strings.TrimSpace(record.BuildingID)
		}

		saved, deduped, err := s.eventSvc.IngestAccessEvent(event.IngestAccessEventInput{
			ID:             request.AccessEvents[i].EventID,
			IdempotencyKey: idempotencyKey,
			TenantID:       record.TenantID,
			BuildingID:     buildingID,
			AreaID:         request.AccessEvents[i].AreaID,
			Type:           eventType,
			Actor:          request.AccessEvents[i].Actor,
			DoorID:         request.AccessEvents[i].DoorID,
			GatewayID:      record.ID,
			Result:         result,
			At:             occurredAt,
		})
		if err != nil {
			retryable := isGatewayBatchFailureRetryable(err)
			accessFailed++
			accessResults = append(accessResults, map[string]any{
				"index":     i,
				"event_id":  strings.TrimSpace(request.AccessEvents[i].EventID),
				"status":    "failed",
				"error":     err.Error(),
				"retryable": retryable,
			})
			if retryable {
				accessRetryable++
				retryAccessEvents = append(retryAccessEvents, map[string]any{
					"event_id":        request.AccessEvents[i].EventID,
					"request_id":      request.AccessEvents[i].RequestID,
					"idempotency_key": request.AccessEvents[i].IdempotencyKey,
					"building_id":     request.AccessEvents[i].BuildingID,
					"area_id":         request.AccessEvents[i].AreaID,
					"type":            request.AccessEvents[i].Type,
					"actor":           request.AccessEvents[i].Actor,
					"door_id":         request.AccessEvents[i].DoorID,
					"result":          request.AccessEvents[i].Result,
					"occurred_at":     request.AccessEvents[i].OccurredAt,
				})
			}
			continue
		}
		if deduped {
			accessDeduplicated++
		} else {
			accessCreated++
		}
		accessIDs = append(accessIDs, saved.ID)
		accessResults = append(accessResults, map[string]any{
			"index":           i,
			"event_id":        saved.ID,
			"status":          "accepted",
			"deduplicated":    deduped,
			"idempotency_key": saved.IdempotencyKey,
		})
		s.appendAuditLog(
			r,
			record.TenantID,
			gatewayAccessAuditAction(saved.Result),
			gatewayAccessEventAuditTarget(
				record.ID,
				queue,
				saved.ID,
				saved.Type,
				saved.Result,
				saved.DoorID,
				saved.Actor,
				saved.IdempotencyKey,
				deduped,
				saved.At,
			),
			"gateway_access_event_batch",
		)
	}

	deviceCreated := 0
	deviceDeduplicated := 0
	deviceFailed := 0
	deviceRetryable := 0
	deviceIDs := make([]string, 0, len(request.DeviceEvents))
	deviceResults := make([]map[string]any, 0, len(request.DeviceEvents))
	retryDeviceEvents := make([]map[string]any, 0, len(request.DeviceEvents))
	for i := range request.DeviceEvents {
		occurredAt, err := parseGatewayOccurredAt(request.DeviceEvents[i].OccurredAt)
		if err != nil {
			deviceFailed++
			deviceResults = append(deviceResults, map[string]any{
				"index":     i,
				"event_id":  strings.TrimSpace(request.DeviceEvents[i].EventID),
				"status":    "failed",
				"error":     "occurred_at must be RFC3339",
				"retryable": false,
			})
			continue
		}
		if err := s.gatewayBatchForcedRetryableError(request.DeviceEvents[i].EventID); err != nil {
			retryable := isGatewayBatchFailureRetryable(err)
			deviceFailed++
			deviceResults = append(deviceResults, map[string]any{
				"index":     i,
				"event_id":  strings.TrimSpace(request.DeviceEvents[i].EventID),
				"status":    "failed",
				"error":     err.Error(),
				"retryable": retryable,
			})
			if retryable {
				deviceRetryable++
				retryDeviceEvents = append(retryDeviceEvents, map[string]any{
					"event_id":        request.DeviceEvents[i].EventID,
					"request_id":      request.DeviceEvents[i].RequestID,
					"idempotency_key": request.DeviceEvents[i].IdempotencyKey,
					"building_id":     request.DeviceEvents[i].BuildingID,
					"type":            request.DeviceEvents[i].Type,
					"detail":          request.DeviceEvents[i].Detail,
					"result":          request.DeviceEvents[i].Result,
					"occurred_at":     request.DeviceEvents[i].OccurredAt,
				})
			}
			continue
		}
		idempotencyKey := strings.TrimSpace(request.DeviceEvents[i].IdempotencyKey)
		if idempotencyKey == "" {
			idempotencyKey = strings.TrimSpace(request.DeviceEvents[i].RequestID)
		}
		if idempotencyKey == "" {
			idempotencyKey = strings.TrimSpace(request.DeviceEvents[i].EventID)
		}
		eventType := strings.TrimSpace(request.DeviceEvents[i].Type)
		if eventType == "" {
			eventType = "gateway_event"
		}
		result := strings.TrimSpace(request.DeviceEvents[i].Result)
		if result == "" {
			result = "accepted"
		}
		buildingID := strings.TrimSpace(request.DeviceEvents[i].BuildingID)
		if buildingID == "" {
			buildingID = strings.TrimSpace(record.BuildingID)
		}

		saved, deduped, err := s.eventSvc.IngestDeviceEvent(event.IngestDeviceEventInput{
			ID:             request.DeviceEvents[i].EventID,
			IdempotencyKey: idempotencyKey,
			TenantID:       record.TenantID,
			BuildingID:     buildingID,
			Type:           eventType,
			GatewayID:      record.ID,
			Detail:         request.DeviceEvents[i].Detail,
			Result:         result,
			At:             occurredAt,
		})
		if err != nil {
			retryable := isGatewayBatchFailureRetryable(err)
			deviceFailed++
			deviceResults = append(deviceResults, map[string]any{
				"index":     i,
				"event_id":  strings.TrimSpace(request.DeviceEvents[i].EventID),
				"status":    "failed",
				"error":     err.Error(),
				"retryable": retryable,
			})
			if retryable {
				deviceRetryable++
				retryDeviceEvents = append(retryDeviceEvents, map[string]any{
					"event_id":        request.DeviceEvents[i].EventID,
					"request_id":      request.DeviceEvents[i].RequestID,
					"idempotency_key": request.DeviceEvents[i].IdempotencyKey,
					"building_id":     request.DeviceEvents[i].BuildingID,
					"type":            request.DeviceEvents[i].Type,
					"detail":          request.DeviceEvents[i].Detail,
					"result":          request.DeviceEvents[i].Result,
					"occurred_at":     request.DeviceEvents[i].OccurredAt,
				})
			}
			continue
		}
		if deduped {
			deviceDeduplicated++
		} else {
			deviceCreated++
		}
		deviceIDs = append(deviceIDs, saved.ID)
		deviceResults = append(deviceResults, map[string]any{
			"index":           i,
			"event_id":        saved.ID,
			"status":          "accepted",
			"deduplicated":    deduped,
			"idempotency_key": saved.IdempotencyKey,
		})
		s.appendAuditLog(
			r,
			record.TenantID,
			gatewayDeviceAuditAction(saved.Type),
			gatewayDeviceEventAuditTarget(
				record.ID,
				queue,
				saved.ID,
				saved.Type,
				saved.Result,
				saved.Detail,
				saved.IdempotencyKey,
				deduped,
				saved.At,
			),
			"gateway_device_event_batch",
		)
	}

	totalFailed := accessFailed + deviceFailed
	totalRetryableFailed := accessRetryable + deviceRetryable
	acceptedTotal := totalItems - totalFailed
	createdTotal := accessCreated + deviceCreated
	queueIngestedTotal := 0
	if createdTotal > 0 {
		progress, err := s.gatewaySvc.AddQueueIngestTotal(record.TenantID, record.ID, queue, createdTotal)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		queueIngestedTotal = progress.IngestedTotal
	} else {
		progress, err := s.gatewaySvc.GetQueueIngestTotal(record.TenantID, record.ID, queue)
		if err == nil {
			queueIngestedTotal = progress.IngestedTotal
		} else if errors.Is(err, gateway.ErrGatewayQueueIngestTotalNotFound) && queue == "default" {
			accessTotal, deviceTotal := s.eventSvc.CountEventsByGateway(record.TenantID, record.ID)
			queueIngestedTotal = accessTotal + deviceTotal
		}
	}
	suggestedCheckpointID := gatewayBatchSuggestedCheckpointID(record.ID, queue, receivedAt)
	queueStatusCode := gatewayBatchQueueStatusCode(totalFailed, totalRetryableFailed)
	nextAction := gatewayBatchNextAction(totalFailed, totalRetryableFailed)
	status := "accepted"
	if totalFailed > 0 {
		status = "partial"
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":      status,
		"gateway_id":  record.ID,
		"tenant_id":   record.TenantID,
		"received_at": receivedAt,
		"access": map[string]any{
			"received":     len(request.AccessEvents),
			"created":      accessCreated,
			"deduplicated": accessDeduplicated,
			"failed":       accessFailed,
			"event_ids":    accessIDs,
			"results":      accessResults,
		},
		"device": map[string]any{
			"received":     len(request.DeviceEvents),
			"created":      deviceCreated,
			"deduplicated": deviceDeduplicated,
			"failed":       deviceFailed,
			"event_ids":    deviceIDs,
			"results":      deviceResults,
		},
		"totals": map[string]any{
			"received":             totalItems,
			"created":              accessCreated + deviceCreated,
			"deduplicated":         accessDeduplicated + deviceDeduplicated,
			"failed":               totalFailed,
			"retryable_failed":     totalRetryableFailed,
			"non_retryable_failed": totalFailed - totalRetryableFailed,
		},
		"retry_subset": map[string]any{
			"gateway_id":    record.ID,
			"tenant_id":     record.TenantID,
			"queue":         queue,
			"access_events": retryAccessEvents,
			"device_events": retryDeviceEvents,
		},
		"queue_hint": map[string]any{
			"queue":                 queue,
			"checkpoint_id":         suggestedCheckpointID,
			"acked_increment":       acceptedTotal,
			"server_ingested_total": queueIngestedTotal,
			"status_code":           queueStatusCode,
			"next_action":           nextAction,
			"retry_subset_present":  totalRetryableFailed > 0,
		},
	})
}

func (s *server) gatewayBootstrapEventsCheckpoint(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID      string `json:"gateway_id"`
		TenantID       string `json:"tenant_id"`
		Queue          string `json:"queue"`
		CheckpointID   string `json:"checkpoint_id"`
		LastRequestID  string `json:"last_request_id"`
		AckedCount     int    `json:"acked_count"`
		LastOccurredAt string `json:"last_occurred_at"`
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
	if !s.authorizeGatewayHTTPDeviceRequest(w, r, record.ID) {
		return
	}

	queue := strings.TrimSpace(request.Queue)
	if queue == "" {
		queue = "default"
	}
	if request.AckedCount >= 0 {
		serverEventTotal := -1
		serverEventTotalSource := ""
		progress, err := s.gatewaySvc.GetQueueIngestTotal(request.TenantID, request.GatewayID, queue)
		if err == nil {
			serverEventTotal = progress.IngestedTotal
			serverEventTotalSource = "queue_ingest_total"
		}
		if serverEventTotal < 0 && queue == "default" {
			accessTotal, deviceTotal := s.eventSvc.CountEventsByGateway(record.TenantID, record.ID)
			serverEventTotal = accessTotal + deviceTotal
			serverEventTotalSource = "event_rows_fallback"
		}
		if serverEventTotal >= 0 && request.AckedCount > serverEventTotal {
			response := map[string]any{
				"error":               "event checkpoint acked_count exceeds server event total",
				"next_action":         "retry_with_server_event_total",
				"server_event_total":  serverEventTotal,
				"server_total_source": serverEventTotalSource,
				"queue":               queue,
			}
			latest, err := s.gatewaySvc.GetEventCheckpoint(request.TenantID, request.GatewayID, queue)
			if err == nil {
				response["checkpoint"] = map[string]any{
					"gateway_id":       latest.GatewayID,
					"tenant_id":        record.TenantID,
					"queue":            latest.Queue,
					"checkpoint_id":    latest.CheckpointID,
					"last_request_id":  latest.LastRequestID,
					"acked_count":      latest.AckedCount,
					"last_occurred_at": latest.LastOccurredAt,
					"updated_at":       latest.UpdatedAt,
				}
			}
			writeJSON(w, http.StatusConflict, response)
			return
		}
	}
	var lastOccurredAt *time.Time
	if raw := strings.TrimSpace(request.LastOccurredAt); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "last_occurred_at must be RFC3339")
			return
		}
		ts := parsed.UTC()
		lastOccurredAt = &ts
	}

	checkpoint, err := s.gatewaySvc.UpsertEventCheckpoint(
		request.TenantID,
		request.GatewayID,
		queue,
		request.CheckpointID,
		request.LastRequestID,
		request.AckedCount,
		lastOccurredAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayEventCheckpointAckedCountRegression):
			latest, latestErr := s.gatewaySvc.GetEventCheckpoint(request.TenantID, request.GatewayID, queue)
			if latestErr == nil {
				writeJSON(w, http.StatusConflict, map[string]any{
					"error":       err.Error(),
					"next_action": "retry_with_non_regressing_acked_count",
					"checkpoint": map[string]any{
						"gateway_id":       latest.GatewayID,
						"tenant_id":        record.TenantID,
						"queue":            latest.Queue,
						"checkpoint_id":    latest.CheckpointID,
						"last_request_id":  latest.LastRequestID,
						"acked_count":      latest.AckedCount,
						"last_occurred_at": latest.LastOccurredAt,
						"updated_at":       latest.UpdatedAt,
					},
				})
				return
			}
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, gateway.ErrGatewayIDRequired),
			errors.Is(err, gateway.ErrGatewayEventCheckpointQueueRequired),
			errors.Is(err, gateway.ErrGatewayEventCheckpointRequired),
			errors.Is(err, gateway.ErrGatewayEventCheckpointAckedCountInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	target := fmt.Sprintf(
		"gateway=%s queue=%s checkpoint=%s acked=%d last_request=%s",
		checkpoint.GatewayID,
		checkpoint.Queue,
		checkpoint.CheckpointID,
		checkpoint.AckedCount,
		checkpoint.LastRequestID,
	)
	s.appendAuditLog(r, record.TenantID, "gateway_event_checkpoint_reported", target, "gateway_event_checkpoint")

	writeJSON(w, http.StatusOK, map[string]any{
		"gateway_id":       checkpoint.GatewayID,
		"tenant_id":        record.TenantID,
		"queue":            checkpoint.Queue,
		"checkpoint_id":    checkpoint.CheckpointID,
		"last_request_id":  checkpoint.LastRequestID,
		"acked_count":      checkpoint.AckedCount,
		"last_occurred_at": checkpoint.LastOccurredAt,
		"updated_at":       checkpoint.UpdatedAt,
	})
}

func (s *server) listGatewayEventCheckpoints(w http.ResponseWriter, r *http.Request) {
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

	items, err := s.gatewaySvc.ListEventCheckpoints(tenantID, gatewayID)
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

func (s *server) listGatewayEventCheckpointSummary(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	gatewayID := strings.TrimSpace(r.URL.Query().Get("gateway_id"))
	queue := strings.TrimSpace(r.URL.Query().Get("queue"))

	limit := 100
	limitInput := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitInput != "" {
		parsedLimit, err := strconv.Atoi(limitInput)
		if err != nil || parsedLimit < 0 {
			writeError(w, http.StatusBadRequest, "limit must be an integer >= 0")
			return
		}
		limit = parsedLimit
	}
	trendWindowMinutes := 60
	trendWindowInput := strings.TrimSpace(r.URL.Query().Get("trend_window_minutes"))
	if trendWindowInput != "" {
		parsedWindow, err := strconv.Atoi(trendWindowInput)
		if err != nil || parsedWindow <= 0 {
			writeError(w, http.StatusBadRequest, "trend_window_minutes must be an integer > 0")
			return
		}
		trendWindowMinutes = parsedWindow
	}

	gateways := s.gatewaySvc.List(tenantID)
	gatewayByID := make(map[string]gateway.Gateway, len(gateways))
	allowedGatewayIDs := make(map[string]struct{}, len(gateways))
	for i := range gateways {
		gatewayByID[gateways[i].ID] = gateways[i]
		if buildingScope != nil {
			if _, exists := buildingScope[gateways[i].BuildingID]; !exists {
				continue
			}
		}
		allowedGatewayIDs[gateways[i].ID] = struct{}{}
	}
	if gatewayID != "" {
		if _, exists := gatewayByID[gatewayID]; !exists {
			writeError(w, http.StatusNotFound, "gateway not found")
			return
		}
		if buildingScope != nil {
			if _, allowed := allowedGatewayIDs[gatewayID]; !allowed {
				writeError(w, http.StatusForbidden, "building scope forbidden")
				return
			}
		}
	}

	checkpoints, err := s.gatewaySvc.ListEventCheckpointsByTenant(tenantID, gatewayID, queue)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	trendWindowUntil := time.Now().UTC()
	trendWindowSince := trendWindowUntil.Add(-time.Duration(trendWindowMinutes) * time.Minute)
	checkpointAuditLogs := []audit.Log{}
	if s.auditSvc != nil {
		checkpointAuditLogs = s.auditSvc.ListFiltered(
			tenantID,
			"gateway_event_checkpoint_reported",
			"gateway_event_checkpoint",
			0,
		)
		checkpointAuditLogs = filterAuditLogsByTimeRange(checkpointAuditLogs, &trendWindowSince, &trendWindowUntil)
	}
	queueTrends, trendSummary := buildGatewayCheckpointWindowTrends(
		checkpointAuditLogs,
		allowedGatewayIDs,
		gatewayID,
		queue,
	)

	items := make([]map[string]any, 0, len(checkpoints))
	totalLag := 0
	totalAcked := 0
	totalEvents := 0
	for i := range checkpoints {
		if _, allowed := allowedGatewayIDs[checkpoints[i].GatewayID]; !allowed {
			continue
		}
		gw, exists := gatewayByID[checkpoints[i].GatewayID]
		if !exists {
			continue
		}
		queueKey := gatewayCheckpointTrendKey(checkpoints[i].GatewayID, checkpoints[i].Queue)
		itemTrend := queueTrends[queueKey]
		if itemTrend.Direction == "" {
			itemTrend.Direction = "flat"
		}
		accessCount, deviceCount := s.eventSvc.CountEventsByGateway(tenantID, checkpoints[i].GatewayID)
		eventTotal := accessCount + deviceCount
		lag := eventTotal - checkpoints[i].AckedCount
		if lag < 0 {
			lag = 0
		}

		totalLag += lag
		totalAcked += checkpoints[i].AckedCount
		totalEvents += eventTotal

		items = append(items, map[string]any{
			"gateway_id":         checkpoints[i].GatewayID,
			"tenant_id":          gw.TenantID,
			"building_id":        gw.BuildingID,
			"queue":              checkpoints[i].Queue,
			"checkpoint_id":      checkpoints[i].CheckpointID,
			"last_request_id":    checkpoints[i].LastRequestID,
			"acked_count":        checkpoints[i].AckedCount,
			"event_total":        eventTotal,
			"access_event_total": accessCount,
			"device_event_total": deviceCount,
			"lag_count":          lag,
			"last_occurred_at":   checkpoints[i].LastOccurredAt,
			"updated_at":         checkpoints[i].UpdatedAt,
			"time_window_trend": map[string]any{
				"report_total":    itemTrend.ReportTotal,
				"acked_delta":     itemTrend.AckedDelta,
				"direction":       itemTrend.Direction,
				"first_report_at": itemTrend.FirstReportAt,
				"last_report_at":  itemTrend.LastReportAt,
			},
		})
	}

	sort.Slice(items, func(i, j int) bool {
		lagI, _ := items[i]["lag_count"].(int)
		lagJ, _ := items[j]["lag_count"].(int)
		if lagI != lagJ {
			return lagI > lagJ
		}
		atI, _ := items[i]["updated_at"].(time.Time)
		atJ, _ := items[j]["updated_at"].(time.Time)
		return atI.After(atJ)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"totals": map[string]any{
			"queues":      len(items),
			"event_total": totalEvents,
			"acked_total": totalAcked,
			"lag_total":   totalLag,
		},
		"time_window_trend": map[string]any{
			"window_minutes":    trendWindowMinutes,
			"since":             trendWindowSince,
			"until":             trendWindowUntil,
			"report_total":      trendSummary.ReportTotal,
			"gateway_total":     trendSummary.GatewayTotal,
			"queue_total":       trendSummary.QueueTotal,
			"acked_delta_total": trendSummary.AckedDeltaTotal,
			"direction":         trendSummary.Direction,
			"last_report_at":    trendSummary.LastReportAt,
		},
	})
}

func parseGatewayOccurredAt(raw string) (time.Time, error) {
	next := strings.TrimSpace(raw)
	if next == "" {
		return time.Now().UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, next)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func gatewayBatchSuggestedCheckpointID(gatewayID, queue string, receivedAt time.Time) string {
	nextGatewayID := strings.TrimSpace(gatewayID)
	if nextGatewayID == "" {
		nextGatewayID = "gateway"
	}
	nextQueue := normalizeGatewayCheckpointQueue(queue)
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	return fmt.Sprintf("%s-%s-%d", nextGatewayID, nextQueue, receivedAt.UnixMilli())
}

func gatewayBatchQueueStatusCode(totalFailed, totalRetryableFailed int) string {
	if totalRetryableFailed > 0 {
		return "QUEUE_RETRY_SUBSET_REQUIRED"
	}
	if totalFailed > 0 {
		return "QUEUE_PARTIAL_NON_RETRYABLE"
	}
	return "QUEUE_READY_TO_CHECKPOINT"
}

func gatewayBatchNextAction(totalFailed, totalRetryableFailed int) string {
	if totalRetryableFailed > 0 {
		return "replay_retry_subset_then_report_checkpoint"
	}
	if totalFailed > 0 {
		return "report_checkpoint_with_non_retryable_failures"
	}
	return "report_checkpoint"
}

func gatewayAccessAuditAction(result string) string {
	next := strings.ToLower(strings.TrimSpace(result))
	switch next {
	case "accepted", "success", "granted", "allow":
		return "gateway_access_grant_recorded"
	case "denied", "rejected", "forbidden", "deny":
		return "gateway_access_deny_recorded"
	default:
		return "gateway_access_event_recorded"
	}
}

func gatewayDeviceAuditAction(eventType string) string {
	next := strings.ToLower(strings.TrimSpace(eventType))
	switch {
	case strings.Contains(next, "tamper"):
		return "gateway_tamper_event_recorded"
	case strings.Contains(next, "timeout"):
		return "gateway_door_timeout_recorded"
	case strings.Contains(next, "rex"):
		return "gateway_rex_event_recorded"
	default:
		return "gateway_device_event_recorded"
	}
}

func gatewayAccessEventAuditTarget(
	gatewayID,
	queue,
	eventID,
	eventType,
	result,
	doorID,
	actor,
	idempotencyKey string,
	deduplicated bool,
	occurredAt time.Time,
) string {
	return strings.TrimSpace(
		fmt.Sprintf(
			"gateway=%s queue=%s event=%s type=%s result=%s door=%s actor=%s deduplicated=%t idempotency_key=%s occurred_at=%s",
			strings.TrimSpace(gatewayID),
			normalizeGatewayCheckpointQueue(queue),
			strings.TrimSpace(eventID),
			strings.TrimSpace(eventType),
			strings.TrimSpace(result),
			strings.TrimSpace(doorID),
			strings.TrimSpace(actor),
			deduplicated,
			strings.TrimSpace(idempotencyKey),
			occurredAt.UTC().Format(time.RFC3339),
		),
	)
}

func gatewayDeviceEventAuditTarget(
	gatewayID,
	queue,
	eventID,
	eventType,
	result,
	detail,
	idempotencyKey string,
	deduplicated bool,
	occurredAt time.Time,
) string {
	return strings.TrimSpace(
		fmt.Sprintf(
			"gateway=%s queue=%s event=%s type=%s result=%s detail=%s deduplicated=%t idempotency_key=%s occurred_at=%s",
			strings.TrimSpace(gatewayID),
			normalizeGatewayCheckpointQueue(queue),
			strings.TrimSpace(eventID),
			strings.TrimSpace(eventType),
			strings.TrimSpace(result),
			strings.TrimSpace(detail),
			deduplicated,
			strings.TrimSpace(idempotencyKey),
			occurredAt.UTC().Format(time.RFC3339),
		),
	)
}

func isGatewayBatchFailureRetryable(err error) bool {
	if err == nil {
		return false
	}
	return !errors.Is(err, event.ErrTenantIDRequired) &&
		!errors.Is(err, event.ErrGatewayIDRequired) &&
		!errors.Is(err, event.ErrAccessEventTypeRequired) &&
		!errors.Is(err, event.ErrDeviceEventTypeRequired)
}

func (s *server) gatewayBatchForcedRetryableError(eventID string) error {
	if !s.cfg.GatewayEventsBatchForceRetryableError {
		return nil
	}
	nextEventID := strings.TrimSpace(eventID)
	if nextEventID == "" {
		return nil
	}
	prefix := strings.TrimSpace(s.cfg.GatewayEventsBatchForceRetryablePrefix)
	if prefix == "" {
		prefix = "force-retry-"
	}
	if !strings.HasPrefix(nextEventID, prefix) {
		return nil
	}

	s.gatewayBatchFailureMu.Lock()
	defer s.gatewayBatchFailureMu.Unlock()
	if _, exists := s.gatewayBatchFailureSeen[nextEventID]; exists {
		return nil
	}
	s.gatewayBatchFailureSeen[nextEventID] = struct{}{}
	return errors.New("forced retryable batch failure for testing")
}

type gatewayConfigAuthzScope struct {
	BuildingIDs []string `json:"building_ids,omitempty"`
	AreaIDs     []string `json:"area_ids,omitempty"`
	DoorIDs     []string `json:"door_ids,omitempty"`
}

type gatewayConfigAuthzCacheCounts struct {
	Doors           int `json:"doors"`
	Policies        int `json:"policies"`
	TemporaryAccess int `json:"temporary_access"`
	VisitorPasses   int `json:"visitor_passes"`
	Users           int `json:"users"`
	UserGroups      int `json:"user_groups"`
	AccessRules     int `json:"access_rules"`
}

type gatewayConfigAuthzCachePolicy struct {
	FallbackMode        string    `json:"fallback_mode"`
	NoCacheBehavior     string    `json:"no_cache_behavior"`
	MaxStaleSeconds     int       `json:"max_stale_seconds"`
	StaleUntil          time.Time `json:"stale_until"`
	RefreshRetrySeconds int       `json:"refresh_retry_seconds"`
	RollbackVersion     string    `json:"rollback_version"`
}

type gatewayConfigAuthzStatusCodes struct {
	Fresh   string `json:"fresh"`
	Stale   string `json:"stale"`
	Missing string `json:"missing"`
	Drift   string `json:"drift"`
}

type gatewayConfigAccessRule struct {
	CredentialType string              `json:"credential_type"`
	CredentialData string              `json:"credential_data"`
	UserID         string              `json:"user_id"`
	UserEmail      string              `json:"user_email"`
	LockIDs        []string            `json:"lock_ids"`
	TimeWindows    []access.TimeWindow `json:"time_windows,omitempty"`
	ExceptionDates []string            `json:"exception_dates,omitempty"`
	ValidFrom      string              `json:"valid_from,omitempty"`
	ValidUntil     string              `json:"valid_until,omitempty"`
}

type gatewayConfigAuthzCache struct {
	Version         string                        `json:"version"`
	VersionReported string                        `json:"version_reported,omitempty"`
	Status          string                        `json:"status,omitempty"`
	GeneratedAt     time.Time                     `json:"generated_at"`
	ExpiresAt       time.Time                     `json:"expires_at"`
	TTLSeconds      int                           `json:"ttl_seconds"`
	Scope           gatewayConfigAuthzScope       `json:"scope"`
	Policy          gatewayConfigAuthzCachePolicy `json:"policy"`
	StatusCodes     gatewayConfigAuthzStatusCodes `json:"status_codes"`
	Counts          gatewayConfigAuthzCacheCounts `json:"counts"`
	Doors           []space.Door                  `json:"doors,omitempty"`
	Policies        []access.Policy               `json:"policies,omitempty"`
	TemporaryAccess []access.TemporaryAccess      `json:"temporary_access,omitempty"`
	VisitorPasses   []access.VisitorPass          `json:"visitor_passes,omitempty"`
	Users           []access.AccessUser           `json:"users,omitempty"`
	UserGroups      []access.UserGroup            `json:"user_groups,omitempty"`
	AccessRules     []gatewayConfigAccessRule     `json:"access_rules,omitempty"`
	LockdownLocks   []string                      `json:"lockdown_locks,omitempty"`
}

func (s *server) buildGatewayConfigAuthzCache(tenantID, gatewayID string, boundDoorIDs []string, generatedAt time.Time) gatewayConfigAuthzCache {
	scope, doorIDSet, buildingIDSet, areaIDSet, doors := s.gatewayConfigAuthzScopeFromBoundDoors(tenantID, boundDoorIDs)
	hasBoundDoors := len(scope.DoorIDs) > 0
	generatedAtUTC := generatedAt.UTC()
	rollbackVersion := s.gatewayAuthzCacheAckVersion(gatewayID)

	userGroups := s.gatewayConfigAuthzUserGroups(tenantID, hasBoundDoors, buildingIDSet)
	allowedUserGroupIDs := make(map[string]struct{}, len(userGroups))
	for i := range userGroups {
		allowedUserGroupIDs[userGroups[i].ID] = struct{}{}
	}

	cache := gatewayConfigAuthzCache{
		GeneratedAt: generatedAtUTC,
		ExpiresAt:   generatedAtUTC.Add(time.Duration(gatewayConfigAuthzCacheTTLSeconds) * time.Second),
		TTLSeconds:  gatewayConfigAuthzCacheTTLSeconds,
		Scope:       scope,
		Policy: gatewayConfigAuthzCachePolicy{
			FallbackMode:        "use_last_acknowledged",
			NoCacheBehavior:     "deny_all",
			MaxStaleSeconds:     gatewayConfigAuthzCacheMaxStaleSeconds,
			StaleUntil:          generatedAtUTC.Add(time.Duration(gatewayConfigAuthzCacheMaxStaleSeconds) * time.Second),
			RefreshRetrySeconds: gatewayConfigAuthzCacheRefreshRetrySeconds,
			RollbackVersion:     rollbackVersion,
		},
		StatusCodes: gatewayConfigAuthzStatusCodes{
			Fresh:   "AUTHZ_CACHE_FRESH",
			Stale:   "AUTHZ_CACHE_STALE",
			Missing: "AUTHZ_CACHE_MISSING",
			Drift:   "AUTHZ_CACHE_DRIFT",
		},
		Doors:    doors,
		Policies: s.gatewayConfigAuthzPolicies(tenantID, hasBoundDoors, doorIDSet, buildingIDSet, areaIDSet),
		TemporaryAccess: s.gatewayConfigAuthzTemporaryAccess(
			tenantID,
			hasBoundDoors,
			doorIDSet,
			buildingIDSet,
			areaIDSet,
		),
		VisitorPasses: s.gatewayConfigAuthzVisitorPasses(tenantID, hasBoundDoors, buildingIDSet),
		Users:         s.gatewayConfigAuthzUsers(tenantID, hasBoundDoors, buildingIDSet, allowedUserGroupIDs),
		UserGroups:    userGroups,
	}
	cache.AccessRules = s.buildGatewayAccessRules(tenantID, boundDoorIDs, cache.Users, cache.UserGroups)
	cache.Counts = gatewayConfigAuthzCacheCounts{
		Doors:           len(cache.Doors),
		Policies:        len(cache.Policies),
		TemporaryAccess: len(cache.TemporaryAccess),
		VisitorPasses:   len(cache.VisitorPasses),
		Users:           len(cache.Users),
		UserGroups:      len(cache.UserGroups),
		AccessRules:     len(cache.AccessRules),
	}
	cache.Version = gatewayConfigAuthzCacheVersion(cache)
	return cache
}

func (s *server) buildGatewayAccessRules(
	tenantID string,
	boundDoorIDs []string,
	users []access.AccessUser,
	userGroups []access.UserGroup,
) []gatewayConfigAccessRule {
	if s.walletSvc == nil || len(boundDoorIDs) == 0 {
		return nil
	}

	userLockAccess := s.buildGatewayUserLockAccessForUsers(tenantID, boundDoorIDs, users, userGroups)

	// Map credentials → users → access rules.
	// All credentials expire in 72h (MaxOfflineDuration). When the gateway is
	// online, it receives fresh rules via WebSocket/config-pull well before expiry.
	// This bounds the revocation delay if the gateway goes offline: a revoked user's
	// credential becomes invalid after at most 72h even without a push.
	credentialValidUntil := time.Now().UTC().Add(72 * time.Hour).Format(time.RFC3339)
	rules := make([]gatewayConfigAccessRule, 0)

	// Physical card inventory → NFC UIDs
	for _, item := range s.walletSvc.ListPhysicalCardInventory(tenantID, "") {
		if item.UID == "" || item.AssignedPassID == "" {
			continue
		}
		pass, err := s.walletSvc.GetPass(tenantID, item.AssignedPassID)
		if err != nil || pass.TargetType != "user" || pass.Status != "active" {
			continue
		}
		lockIDs, exists := userLockAccess[pass.TargetID]
		if !exists {
			continue
		}
		user := findUserByID(users, pass.TargetID)
		rules = append(rules, gatewayConfigAccessRule{
			CredentialType: "nfc_uid",
			CredentialData: item.UID,
			UserID:         pass.TargetID,
			UserEmail:      userEmail(user),
			LockIDs:        lockIDs,
			ValidUntil:     credentialValidUntil,
		})
		if item.CardNumber != "" {
			rules = append(rules, gatewayConfigAccessRule{
				CredentialType: "card_number",
				CredentialData: item.CardNumber,
				UserID:         pass.TargetID,
				UserEmail:      userEmail(user),
				LockIDs:        lockIDs,
				ValidUntil:     credentialValidUntil,
			})
		}
	}

	// Passes with UID (cards registered directly)
	for _, pass := range s.walletSvc.ListPasses(tenantID) {
		if pass.UID == "" || pass.TargetType != "user" || pass.Status != "active" {
			continue
		}
		lockIDs, exists := userLockAccess[pass.TargetID]
		if !exists {
			continue
		}
		// Skip if already covered by physical card inventory
		alreadyCovered := false
		for _, r := range rules {
			if r.CredentialType == "nfc_uid" && r.CredentialData == pass.UID {
				alreadyCovered = true
				break
			}
		}
		if alreadyCovered {
			continue
		}
		user := findUserByID(users, pass.TargetID)
		rules = append(rules, gatewayConfigAccessRule{
			CredentialType: "nfc_uid",
			CredentialData: pass.UID,
			UserID:         pass.TargetID,
			UserEmail:      userEmail(user),
			LockIDs:        lockIDs,
			ValidUntil:     credentialValidUntil,
		})
	}

	// Mobile BLE credentials (public keys for local signature verification)
	if s.credentialSvc != nil {
		mobileCredSyncs := s.credentialSvc.BuildGatewayCredentialSync(tenantID, boundDoorIDs, userLockAccess)
		for _, mc := range mobileCredSyncs {
			rules = append(rules, gatewayConfigAccessRule{
				CredentialType: "ble_signature",
				CredentialData: mc.PublicKeyPEM,
				UserID:         mc.UserID,
				UserEmail:      mc.UserEmail,
				LockIDs:        mc.LockIDs,
				ValidUntil:     credentialValidUntil,
			})
		}
	}

	return rules
}

func (s *server) buildGatewayUserLockAccess(tenantID string, boundDoorIDs []string) map[string][]string {
	if s.accessSvc == nil || s.spaceSvc == nil || len(boundDoorIDs) == 0 {
		return nil
	}
	scope, _, buildingIDSet, _, _ := s.gatewayConfigAuthzScopeFromBoundDoors(tenantID, boundDoorIDs)
	if len(scope.DoorIDs) == 0 {
		return nil
	}
	userGroups := s.gatewayConfigAuthzUserGroups(tenantID, true, buildingIDSet)
	allowedUserGroupIDs := make(map[string]struct{}, len(userGroups))
	for i := range userGroups {
		allowedUserGroupIDs[userGroups[i].ID] = struct{}{}
	}
	users := s.gatewayConfigAuthzUsers(tenantID, true, buildingIDSet, allowedUserGroupIDs)
	return s.buildGatewayUserLockAccessForUsers(tenantID, boundDoorIDs, users, userGroups)
}

func (s *server) buildGatewayUserLockAccessForUsers(
	tenantID string,
	boundDoorIDs []string,
	users []access.AccessUser,
	userGroups []access.UserGroup,
) map[string][]string {
	if s.accessSvc == nil || s.spaceSvc == nil || len(boundDoorIDs) == 0 || len(users) == 0 {
		return nil
	}

	boundDoorSet := make(map[string]struct{}, len(boundDoorIDs))
	for _, id := range boundDoorIDs {
		if nextID := strings.TrimSpace(id); nextID != "" {
			boundDoorSet[nextID] = struct{}{}
		}
	}
	if len(boundDoorSet) == 0 {
		return nil
	}

	userLockAccess := make(map[string][]string)
	doorGroups := s.spaceSvc.ListDoorGroups(tenantID)
	allDoors := s.spaceSvc.ListDoors(tenantID)
	roleAssignments := s.accessSvc.ListRoleAssignments(tenantID)

	for _, user := range users {
		lockIDs := make(map[string]struct{})

		for _, ug := range userGroups {
			if !containsString(user.GroupIDs, ug.ID) {
				continue
			}
			for _, dg := range doorGroups {
				for _, doorID := range dg.DoorIDs {
					if _, bound := boundDoorSet[doorID]; bound {
						lockIDs[doorID] = struct{}{}
					}
				}
			}
			for _, door := range allDoors {
				if _, bound := boundDoorSet[door.ID]; !bound {
					continue
				}
				if door.BuildingID == ug.BuildingID || door.BuildingID == ug.PlaceID {
					lockIDs[door.ID] = struct{}{}
				}
			}
		}

		for _, ra := range roleAssignments {
			if ra.AssigneeType != "User" || ra.AssigneeID != user.ID {
				continue
			}
			for _, door := range allDoors {
				if _, bound := boundDoorSet[door.ID]; !bound {
					continue
				}
				if ra.AppliesToType == "Organization" || (ra.AppliesToType == "Place" && ra.AppliesToID == door.BuildingID) {
					lockIDs[door.ID] = struct{}{}
				}
			}
		}

		if len(lockIDs) > 0 {
			userLockAccess[user.ID] = sortedSetKeys(lockIDs)
		}
	}
	return userLockAccess
}

func findUserByID(users []access.AccessUser, id string) *access.AccessUser {
	for i := range users {
		if users[i].ID == id {
			return &users[i]
		}
	}
	return nil
}

func userEmail(user *access.AccessUser) string {
	if user == nil {
		return ""
	}
	return user.Email
}

func gatewayConfigAuthzResolveStatus(
	codes gatewayConfigAuthzStatusCodes,
	reportedVersion,
	expectedVersion,
	rollbackVersion string,
) string {
	reported := strings.TrimSpace(reportedVersion)
	expected := strings.TrimSpace(expectedVersion)
	rollback := strings.TrimSpace(rollbackVersion)
	if reported == "" {
		return codes.Missing
	}
	if expected != "" && reported == expected {
		return codes.Fresh
	}
	if rollback != "" && reported == rollback {
		return codes.Stale
	}
	return codes.Drift
}

func (s *server) gatewayConfigAuthzScopeFromBoundDoors(
	tenantID string,
	boundDoorIDs []string,
) (gatewayConfigAuthzScope, map[string]struct{}, map[string]struct{}, map[string]struct{}, []space.Door) {
	doorIDs := sortedUniqueTrimmedIDs(boundDoorIDs)
	doorIDSet := make(map[string]struct{}, len(doorIDs))
	for i := range doorIDs {
		doorIDSet[doorIDs[i]] = struct{}{}
	}
	buildingIDSet := map[string]struct{}{}
	areaIDSet := map[string]struct{}{}

	scope := gatewayConfigAuthzScope{
		DoorIDs: doorIDs,
	}
	if len(doorIDSet) == 0 || s.spaceSvc == nil {
		return scope, doorIDSet, buildingIDSet, areaIDSet, nil
	}

	doors := make([]space.Door, 0, len(doorIDSet))
	allDoors := s.spaceSvc.ListDoors(tenantID)
	for i := range allDoors {
		doorID := strings.TrimSpace(allDoors[i].ID)
		if _, exists := doorIDSet[doorID]; !exists {
			continue
		}
		doors = append(doors, allDoors[i])
		if buildingID := strings.TrimSpace(allDoors[i].BuildingID); buildingID != "" {
			buildingIDSet[buildingID] = struct{}{}
		}
		if areaID := strings.TrimSpace(allDoors[i].AreaID); areaID != "" {
			areaIDSet[areaID] = struct{}{}
		}
	}
	sort.Slice(doors, func(i, j int) bool {
		return doors[i].ID < doors[j].ID
	})

	scope.BuildingIDs = sortedSetKeys(buildingIDSet)
	scope.AreaIDs = sortedSetKeys(areaIDSet)
	return scope, doorIDSet, buildingIDSet, areaIDSet, doors
}

func (s *server) gatewayConfigAuthzPolicies(
	tenantID string,
	hasBoundDoors bool,
	doorIDSet, buildingIDSet, areaIDSet map[string]struct{},
) []access.Policy {
	if !hasBoundDoors || s.accessSvc == nil {
		return nil
	}
	items := make([]access.Policy, 0)
	policies := s.accessSvc.ListPolicies(tenantID)
	for i := range policies {
		if strings.ToLower(strings.TrimSpace(policies[i].Status)) != "active" {
			continue
		}
		if !gatewayConfigAuthzScopeMatch(
			policies[i].ScopeType,
			policies[i].BuildingID,
			policies[i].AreaID,
			policies[i].DoorID,
			hasBoundDoors,
			doorIDSet,
			buildingIDSet,
			areaIDSet,
		) {
			continue
		}
		items = append(items, policies[i])
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID == items[j].ID {
			return items[i].Name < items[j].Name
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func (s *server) gatewayConfigAuthzTemporaryAccess(
	tenantID string,
	hasBoundDoors bool,
	doorIDSet, buildingIDSet, areaIDSet map[string]struct{},
) []access.TemporaryAccess {
	if !hasBoundDoors || s.accessSvc == nil {
		return nil
	}
	items := make([]access.TemporaryAccess, 0)
	records := s.accessSvc.ListTemporaryAccess(tenantID)
	for i := range records {
		if !gatewayConfigAuthzScopeMatch(
			records[i].ScopeType,
			records[i].BuildingID,
			records[i].AreaID,
			records[i].DoorID,
			hasBoundDoors,
			doorIDSet,
			buildingIDSet,
			areaIDSet,
		) {
			continue
		}
		items = append(items, records[i])
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID == items[j].ID {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func (s *server) gatewayConfigAuthzVisitorPasses(
	tenantID string,
	hasBoundDoors bool,
	buildingIDSet map[string]struct{},
) []access.VisitorPass {
	if !hasBoundDoors || s.accessSvc == nil {
		return nil
	}
	items := make([]access.VisitorPass, 0)
	records := s.accessSvc.ListVisitorPasses(tenantID)
	for i := range records {
		buildingID := strings.TrimSpace(records[i].BuildingID)
		if buildingID != "" {
			if _, exists := buildingIDSet[buildingID]; !exists {
				continue
			}
		}
		items = append(items, records[i])
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID == items[j].ID {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func (s *server) gatewayConfigAuthzUsers(
	tenantID string,
	hasBoundDoors bool,
	buildingIDSet, allowedUserGroupIDs map[string]struct{},
) []access.AccessUser {
	if !hasBoundDoors || s.accessSvc == nil {
		return nil
	}
	items := make([]access.AccessUser, 0)
	users := s.accessSvc.ListUsers(tenantID)
	for i := range users {
		if strings.ToLower(strings.TrimSpace(users[i].Status)) != "active" {
			continue
		}
		buildingID := strings.TrimSpace(users[i].BuildingID)
		if buildingID != "" {
			if _, exists := buildingIDSet[buildingID]; !exists {
				continue
			}
		}
		record := users[i]
		record.GroupIDs = filterAndSortUserGroupIDs(users[i].GroupIDs, allowedUserGroupIDs)
		items = append(items, record)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID == items[j].ID {
			return items[i].Email < items[j].Email
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func (s *server) gatewayConfigAuthzUserGroups(
	tenantID string,
	hasBoundDoors bool,
	buildingIDSet map[string]struct{},
) []access.UserGroup {
	if !hasBoundDoors || s.accessSvc == nil {
		return nil
	}
	items := make([]access.UserGroup, 0)
	groups := s.accessSvc.ListUserGroups(tenantID)
	for i := range groups {
		buildingID := strings.TrimSpace(groups[i].BuildingID)
		if buildingID != "" {
			if _, exists := buildingIDSet[buildingID]; !exists {
				continue
			}
		}
		record := groups[i]
		record.Members = sortedUniqueTrimmedIDs(groups[i].Members)
		items = append(items, record)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID == items[j].ID {
			return items[i].Name < items[j].Name
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func gatewayConfigAuthzScopeMatch(
	scopeType, buildingID, areaID, doorID string,
	hasBoundDoors bool,
	doorIDSet, buildingIDSet, areaIDSet map[string]struct{},
) bool {
	switch strings.ToLower(strings.TrimSpace(scopeType)) {
	case "", "all":
		return hasBoundDoors
	case "building":
		_, exists := buildingIDSet[strings.TrimSpace(buildingID)]
		return exists
	case "area":
		_, exists := areaIDSet[strings.TrimSpace(areaID)]
		return exists
	case "door":
		_, exists := doorIDSet[strings.TrimSpace(doorID)]
		return exists
	default:
		return false
	}
}

func gatewayConfigAuthzCacheVersion(cache gatewayConfigAuthzCache) string {
	payload := struct {
		TTLSeconds      int                           `json:"ttl_seconds"`
		Scope           gatewayConfigAuthzScope       `json:"scope"`
		Counts          gatewayConfigAuthzCacheCounts `json:"counts"`
		Doors           []space.Door                  `json:"doors,omitempty"`
		Policies        []access.Policy               `json:"policies,omitempty"`
		TemporaryAccess []access.TemporaryAccess      `json:"temporary_access,omitempty"`
		VisitorPasses   []access.VisitorPass          `json:"visitor_passes,omitempty"`
		Users           []access.AccessUser           `json:"users,omitempty"`
		UserGroups      []access.UserGroup            `json:"user_groups,omitempty"`
	}{
		TTLSeconds:      cache.TTLSeconds,
		Scope:           cache.Scope,
		Counts:          cache.Counts,
		Doors:           cache.Doors,
		Policies:        cache.Policies,
		TemporaryAccess: cache.TemporaryAccess,
		VisitorPasses:   cache.VisitorPasses,
		Users:           cache.Users,
		UserGroups:      cache.UserGroups,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "authz-unavailable"
	}
	sum := sha256.Sum256(raw)
	return "authz-" + hex.EncodeToString(sum[:])
}

func filterAndSortUserGroupIDs(values []string, allowed map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for i := range values {
		next := strings.TrimSpace(values[i])
		if next == "" {
			continue
		}
		if len(allowed) > 0 {
			if _, exists := allowed[next]; !exists {
				continue
			}
		}
		if _, exists := seen[next]; exists {
			continue
		}
		seen[next] = struct{}{}
		items = append(items, next)
	}
	if len(items) == 0 {
		return nil
	}
	sort.Strings(items)
	return items
}

func sortedSetKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	items := make([]string, 0, len(values))
	for value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		items = append(items, value)
	}
	if len(items) == 0 {
		return nil
	}
	sort.Strings(items)
	return items
}

func sortedUniqueTrimmedIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for i := range values {
		next := strings.TrimSpace(values[i])
		if next == "" {
			continue
		}
		if _, exists := seen[next]; exists {
			continue
		}
		seen[next] = struct{}{}
		items = append(items, next)
	}
	if len(items) == 0 {
		return nil
	}
	sort.Strings(items)
	return items
}

type gatewayCheckpointAuditMetrics struct {
	GatewayID    string
	Queue        string
	CheckpointID string
	AckedCount   int
	LastRequest  string
}

type gatewayCheckpointQueueTrend struct {
	ReportTotal   int
	AckedDelta    int
	Direction     string
	FirstReportAt *time.Time
	LastReportAt  *time.Time
}

type gatewayCheckpointWindowTrendSummary struct {
	ReportTotal     int
	GatewayTotal    int
	QueueTotal      int
	AckedDeltaTotal int
	Direction       string
	LastReportAt    *time.Time
}

func buildGatewayCheckpointWindowTrends(
	logs []audit.Log,
	allowedGatewayIDs map[string]struct{},
	filterGatewayID,
	filterQueue string,
) (map[string]gatewayCheckpointQueueTrend, gatewayCheckpointWindowTrendSummary) {
	type accumulator struct {
		reportTotal int
		firstAt     time.Time
		lastAt      time.Time
		firstAcked  int
		lastAcked   int
	}

	filteredGatewayID := strings.TrimSpace(filterGatewayID)
	filteredQueue := normalizeGatewayCheckpointQueue(filterQueue)
	filterByQueue := strings.TrimSpace(filterQueue) != ""
	accumulators := make(map[string]accumulator)
	gatewayIDs := make(map[string]struct{})

	for i := range logs {
		metrics := parseGatewayCheckpointAuditMetrics(logs[i].Target)
		if metrics.GatewayID == "" {
			continue
		}
		if allowedGatewayIDs != nil {
			if _, allowed := allowedGatewayIDs[metrics.GatewayID]; !allowed {
				continue
			}
		}
		if filteredGatewayID != "" && metrics.GatewayID != filteredGatewayID {
			continue
		}
		if filterByQueue && metrics.Queue != filteredQueue {
			continue
		}

		gatewayIDs[metrics.GatewayID] = struct{}{}
		key := gatewayCheckpointTrendKey(metrics.GatewayID, metrics.Queue)
		entry, exists := accumulators[key]
		if !exists {
			accumulators[key] = accumulator{
				reportTotal: 1,
				firstAt:     logs[i].At,
				lastAt:      logs[i].At,
				firstAcked:  metrics.AckedCount,
				lastAcked:   metrics.AckedCount,
			}
			continue
		}

		entry.reportTotal++
		if logs[i].At.Before(entry.firstAt) {
			entry.firstAt = logs[i].At
			entry.firstAcked = metrics.AckedCount
		}
		if logs[i].At.After(entry.lastAt) || logs[i].At.Equal(entry.lastAt) {
			entry.lastAt = logs[i].At
			entry.lastAcked = metrics.AckedCount
		}
		accumulators[key] = entry
	}

	items := make(map[string]gatewayCheckpointQueueTrend, len(accumulators))
	summary := gatewayCheckpointWindowTrendSummary{
		GatewayTotal: len(gatewayIDs),
	}
	for key, entry := range accumulators {
		ackedDelta := entry.lastAcked - entry.firstAcked
		first := entry.firstAt
		last := entry.lastAt
		items[key] = gatewayCheckpointQueueTrend{
			ReportTotal:   entry.reportTotal,
			AckedDelta:    ackedDelta,
			Direction:     checkpointTrendDirection(ackedDelta),
			FirstReportAt: &first,
			LastReportAt:  &last,
		}
		summary.ReportTotal += entry.reportTotal
		summary.AckedDeltaTotal += ackedDelta
		if summary.LastReportAt == nil || last.After(*summary.LastReportAt) {
			copyLast := last
			summary.LastReportAt = &copyLast
		}
	}
	summary.QueueTotal = len(items)
	summary.Direction = checkpointTrendDirection(summary.AckedDeltaTotal)
	return items, summary
}

func parseGatewayCheckpointAuditMetrics(rawTarget string) gatewayCheckpointAuditMetrics {
	values := parseAuditTargetKeyValues(rawTarget)
	return gatewayCheckpointAuditMetrics{
		GatewayID:    strings.TrimSpace(values["gateway"]),
		Queue:        normalizeGatewayCheckpointQueue(values["queue"]),
		CheckpointID: strings.TrimSpace(values["checkpoint"]),
		AckedCount:   parseIntOrZero(values["acked"]),
		LastRequest:  strings.TrimSpace(values["last_request"]),
	}
}

func parseAuditTargetKeyValues(rawTarget string) map[string]string {
	parts := strings.Fields(strings.TrimSpace(rawTarget))
	values := make(map[string]string, len(parts))
	for i := range parts {
		pair := strings.SplitN(parts[i], "=", 2)
		if len(pair) != 2 {
			continue
		}
		key := strings.TrimSpace(pair[0])
		value := strings.TrimSpace(pair[1])
		if key == "" || value == "" {
			continue
		}
		values[key] = value
	}
	return values
}

func gatewayCheckpointTrendKey(gatewayID, queue string) string {
	return strings.TrimSpace(gatewayID) + "|" + normalizeGatewayCheckpointQueue(queue)
}

func normalizeGatewayCheckpointQueue(queue string) string {
	next := strings.TrimSpace(queue)
	if next == "" {
		return "default"
	}
	return next
}

func checkpointTrendDirection(delta int) string {
	if delta > 0 {
		return "up"
	}
	if delta < 0 {
		return "down"
	}
	return "flat"
}

func (s *server) getAuthUserBuildingScope(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	target, err := s.authService.GetUserByID(userID)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if !s.canManageAuthUserBuildingScope(w, r, target) {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":      target.ID,
		"email":        target.Email,
		"role":         target.Role,
		"tenant_id":    target.TenantID,
		"building_ids": target.BuildingIDs,
	})
}

func (s *server) updateAuthUserBuildingScope(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	target, err := s.authService.GetUserByID(userID)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if !s.canManageAuthUserBuildingScope(w, r, target) {
		return
	}

	var request struct {
		BuildingIDs []string `json:"building_ids"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := s.authService.UpdateUserBuildingScope(userID, request.BuildingIDs)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, auth.ErrUserRoleUnsupported):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":      updated.ID,
		"email":        updated.Email,
		"role":         updated.Role,
		"tenant_id":    updated.TenantID,
		"building_ids": updated.BuildingIDs,
	})
}

func (s *server) updateAuthUserPasswordAuth(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	var request struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := s.authService.UpdateUserPasswordAuth(userID, request.Enabled)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	actor, _ := authenticatedUser(r)
	s.appendAuditLog(r, updated.TenantID, "auth_user_password_auth_updated",
		fmt.Sprintf("user_id=%s,enabled=%v,by=%s", updated.ID, request.Enabled, actor.Email), "auth")
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":               updated.ID,
		"email":                 updated.Email,
		"password_auth_enabled": updated.PasswordAuthEnabled,
	})
}

func (s *server) canManageAuthUserBuildingScope(w http.ResponseWriter, r *http.Request, target auth.User) bool {
	actor, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return false
	}

	actorRole := strings.ToLower(strings.TrimSpace(actor.Role))
	if actorRole == "super_admin" {
		return true
	}
	if actorRole != "tenant_admin" {
		writeError(w, http.StatusForbidden, "forbidden")
		return false
	}

	actorTenantID := strings.TrimSpace(actor.TenantID)
	targetTenantID := strings.TrimSpace(target.TenantID)
	if actorTenantID == "" || targetTenantID == "" || actorTenantID != targetTenantID {
		writeError(w, http.StatusForbidden, "tenant scope forbidden")
		return false
	}

	return true
}
