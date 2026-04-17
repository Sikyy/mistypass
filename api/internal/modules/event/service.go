package event

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrTenantIDRequired = errors.New("tenant_id is required")
var ErrGatewayIDRequired = errors.New("gateway_id is required")
var ErrAccessEventTypeRequired = errors.New("access event type is required")
var ErrDeviceEventTypeRequired = errors.New("device event type is required")

type AccessEvent struct {
	ID             string    `json:"id"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	TenantID       string    `json:"tenant_id"`
	BuildingID     string    `json:"building_id"`
	AreaID         string    `json:"area_id"`
	Type           string    `json:"type"`
	Actor          string    `json:"actor"`
	DoorID         string    `json:"door_id"`
	GatewayID      string    `json:"gateway_id"`
	Result         string    `json:"result"`
	At             time.Time `json:"at"`
}

type DeviceEvent struct {
	ID             string    `json:"id"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	TenantID       string    `json:"tenant_id"`
	BuildingID     string    `json:"building_id"`
	Type           string    `json:"type"`
	GatewayID      string    `json:"gateway_id"`
	Detail         string    `json:"detail"`
	Result         string    `json:"result"`
	At             time.Time `json:"at"`
}

type IngestAccessEventInput struct {
	ID             string
	IdempotencyKey string
	TenantID       string
	BuildingID     string
	AreaID         string
	Type           string
	Actor          string
	DoorID         string
	GatewayID      string
	Result         string
	At             time.Time
}

type IngestDeviceEventInput struct {
	ID             string
	IdempotencyKey string
	TenantID       string
	BuildingID     string
	Type           string
	GatewayID      string
	Detail         string
	Result         string
	At             time.Time
}

type StateStore interface {
	Load(key string, dst any) (bool, error)
	Save(key string, value any) error
}

const stateKey = "module_event"

type stateSnapshot struct {
	AccessEvents []AccessEvent `json:"access_events"`
	DeviceEvents []DeviceEvent `json:"device_events"`
}

type Service struct {
	mu           sync.RWMutex
	accessEvents []AccessEvent
	deviceEvents []DeviceEvent
	stateStore   StateStore
}

func NewService() *Service {
	now := time.Now().UTC()
	return &Service{
		accessEvents: []AccessEvent{
			{
				ID:         "evt_1001",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_001",
				AreaID:     "area_demo_001",
				Type:       "access_granted",
				Actor:      "indra.saputra",
				DoorID:     "door_jkt_001",
				GatewayID:  "MP-GW-JKT-0001",
				Result:     "success",
				At:         now.Add(-5 * time.Minute),
			},
			{
				ID:         "evt_1002",
				TenantID:   "tenant_demo_factory",
				BuildingID: "building_demo_003",
				AreaID:     "area_demo_003",
				Type:       "access_denied",
				Actor:      "unknown_device",
				DoorID:     "door_fct_029",
				GatewayID:  "MP-GW-BTN-0098",
				Result:     "denied",
				At:         now.Add(-9 * time.Minute),
			},
			{
				ID:         "evt_1004",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_002",
				AreaID:     "area_demo_002",
				Type:       "access_granted",
				Actor:      "dina.wijaya",
				DoorID:     "door_jkt_014",
				GatewayID:  "MP-GW-JKT-0042",
				Result:     "success",
				At:         now.Add(-16 * time.Minute),
			},
		},
		deviceEvents: []DeviceEvent{
			{
				ID:         "evt_1003",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_002",
				Type:       "gateway_event",
				GatewayID:  "MP-GW-JKT-0042",
				Detail:     "heartbeat jitter above threshold",
				Result:     "warning",
				At:         now.Add(-12 * time.Minute),
			},
		},
	}
}

func NewServiceWithStateStore(store StateStore) (*Service, error) {
	svc := NewService()
	svc.stateStore = store
	if err := svc.restoreFromStateStore(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *Service) ListAccessEvents(tenantID string) []AccessEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]AccessEvent, 0, len(s.accessEvents))
	for i := range s.accessEvents {
		if filterTenantID != "" && s.accessEvents[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.accessEvents[i])
	}
	return items
}

func (s *Service) ListDeviceEvents(tenantID string) []DeviceEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]DeviceEvent, 0, len(s.deviceEvents))
	for i := range s.deviceEvents {
		if filterTenantID != "" && s.deviceEvents[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.deviceEvents[i])
	}
	return items
}

func (s *Service) CountEventsByGateway(tenantID, gatewayID string) (int, int) {
	filterTenantID := strings.TrimSpace(tenantID)
	filterGatewayID := strings.TrimSpace(gatewayID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	accessCount := 0
	deviceCount := 0
	for i := range s.accessEvents {
		if filterTenantID != "" && s.accessEvents[i].TenantID != filterTenantID {
			continue
		}
		if filterGatewayID != "" && s.accessEvents[i].GatewayID != filterGatewayID {
			continue
		}
		accessCount++
	}
	for i := range s.deviceEvents {
		if filterTenantID != "" && s.deviceEvents[i].TenantID != filterTenantID {
			continue
		}
		if filterGatewayID != "" && s.deviceEvents[i].GatewayID != filterGatewayID {
			continue
		}
		deviceCount++
	}
	return accessCount, deviceCount
}

func (s *Service) IngestAccessEvent(input IngestAccessEventInput) (AccessEvent, bool, error) {
	next := IngestAccessEventInput{
		ID:             strings.TrimSpace(input.ID),
		IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
		TenantID:       strings.TrimSpace(input.TenantID),
		BuildingID:     strings.TrimSpace(input.BuildingID),
		AreaID:         strings.TrimSpace(input.AreaID),
		Type:           strings.TrimSpace(input.Type),
		Actor:          strings.TrimSpace(input.Actor),
		DoorID:         strings.TrimSpace(input.DoorID),
		GatewayID:      strings.TrimSpace(input.GatewayID),
		Result:         strings.TrimSpace(input.Result),
		At:             input.At.UTC(),
	}
	if next.TenantID == "" {
		return AccessEvent{}, false, ErrTenantIDRequired
	}
	if next.GatewayID == "" {
		return AccessEvent{}, false, ErrGatewayIDRequired
	}
	if next.Type == "" {
		return AccessEvent{}, false, ErrAccessEventTypeRequired
	}
	if next.At.IsZero() {
		next.At = time.Now().UTC()
	}
	if next.Result == "" {
		next.Result = "accepted"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := findAccessEventByIdempotencyLocked(s.accessEvents, next.TenantID, next.GatewayID, next.IdempotencyKey); ok {
		return existing, true, nil
	}
	if existing, ok := findAccessEventByIDLocked(s.accessEvents, next.TenantID, next.GatewayID, next.ID); ok {
		return existing, true, nil
	}
	if next.ID == "" {
		eventID, err := nextEventID("gwea")
		if err != nil {
			return AccessEvent{}, false, err
		}
		next.ID = eventID
	}

	record := AccessEvent{
		ID:             next.ID,
		IdempotencyKey: next.IdempotencyKey,
		TenantID:       next.TenantID,
		BuildingID:     next.BuildingID,
		AreaID:         next.AreaID,
		Type:           next.Type,
		Actor:          next.Actor,
		DoorID:         next.DoorID,
		GatewayID:      next.GatewayID,
		Result:         next.Result,
		At:             next.At,
	}
	s.accessEvents = append([]AccessEvent{record}, s.accessEvents...)
	if err := s.persistLocked(); err != nil {
		return AccessEvent{}, false, err
	}
	return record, false, nil
}

func (s *Service) IngestDeviceEvent(input IngestDeviceEventInput) (DeviceEvent, bool, error) {
	next := IngestDeviceEventInput{
		ID:             strings.TrimSpace(input.ID),
		IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
		TenantID:       strings.TrimSpace(input.TenantID),
		BuildingID:     strings.TrimSpace(input.BuildingID),
		Type:           strings.TrimSpace(input.Type),
		GatewayID:      strings.TrimSpace(input.GatewayID),
		Detail:         strings.TrimSpace(input.Detail),
		Result:         strings.TrimSpace(input.Result),
		At:             input.At.UTC(),
	}
	if next.TenantID == "" {
		return DeviceEvent{}, false, ErrTenantIDRequired
	}
	if next.GatewayID == "" {
		return DeviceEvent{}, false, ErrGatewayIDRequired
	}
	if next.Type == "" {
		return DeviceEvent{}, false, ErrDeviceEventTypeRequired
	}
	if next.At.IsZero() {
		next.At = time.Now().UTC()
	}
	if next.Result == "" {
		next.Result = "accepted"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := findDeviceEventByIdempotencyLocked(s.deviceEvents, next.TenantID, next.GatewayID, next.IdempotencyKey); ok {
		return existing, true, nil
	}
	if existing, ok := findDeviceEventByIDLocked(s.deviceEvents, next.TenantID, next.GatewayID, next.ID); ok {
		return existing, true, nil
	}
	if next.ID == "" {
		eventID, err := nextEventID("gwed")
		if err != nil {
			return DeviceEvent{}, false, err
		}
		next.ID = eventID
	}

	record := DeviceEvent{
		ID:             next.ID,
		IdempotencyKey: next.IdempotencyKey,
		TenantID:       next.TenantID,
		BuildingID:     next.BuildingID,
		Type:           next.Type,
		GatewayID:      next.GatewayID,
		Detail:         next.Detail,
		Result:         next.Result,
		At:             next.At,
	}
	s.deviceEvents = append([]DeviceEvent{record}, s.deviceEvents...)
	if err := s.persistLocked(); err != nil {
		return DeviceEvent{}, false, err
	}
	return record, false, nil
}

func (s *Service) restoreFromStateStore() error {
	if s.stateStore == nil {
		return nil
	}

	var snapshot stateSnapshot
	found, err := s.stateStore.Load(stateKey, &snapshot)
	if err != nil {
		return err
	}
	if !found {
		return s.stateStore.Save(stateKey, stateSnapshot{
			AccessEvents: cloneAccessEvents(s.accessEvents),
			DeviceEvents: cloneDeviceEvents(s.deviceEvents),
		})
	}

	s.mu.Lock()
	s.accessEvents = cloneAccessEvents(snapshot.AccessEvents)
	s.deviceEvents = cloneDeviceEvents(snapshot.DeviceEvents)
	s.mu.Unlock()
	return nil
}

func (s *Service) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(stateKey, stateSnapshot{
		AccessEvents: cloneAccessEvents(s.accessEvents),
		DeviceEvents: cloneDeviceEvents(s.deviceEvents),
	})
}

func cloneAccessEvents(items []AccessEvent) []AccessEvent {
	output := make([]AccessEvent, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneDeviceEvents(items []DeviceEvent) []DeviceEvent {
	output := make([]DeviceEvent, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func findAccessEventByIdempotencyLocked(events []AccessEvent, tenantID, gatewayID, key string) (AccessEvent, bool) {
	if key == "" {
		return AccessEvent{}, false
	}
	for i := range events {
		if events[i].TenantID == tenantID && events[i].GatewayID == gatewayID && events[i].IdempotencyKey == key {
			return events[i], true
		}
	}
	return AccessEvent{}, false
}

func findAccessEventByIDLocked(events []AccessEvent, tenantID, gatewayID, id string) (AccessEvent, bool) {
	if id == "" {
		return AccessEvent{}, false
	}
	for i := range events {
		if events[i].TenantID == tenantID && events[i].GatewayID == gatewayID && events[i].ID == id {
			return events[i], true
		}
	}
	return AccessEvent{}, false
}

func findDeviceEventByIdempotencyLocked(events []DeviceEvent, tenantID, gatewayID, key string) (DeviceEvent, bool) {
	if key == "" {
		return DeviceEvent{}, false
	}
	for i := range events {
		if events[i].TenantID == tenantID && events[i].GatewayID == gatewayID && events[i].IdempotencyKey == key {
			return events[i], true
		}
	}
	return DeviceEvent{}, false
}

func findDeviceEventByIDLocked(events []DeviceEvent, tenantID, gatewayID, id string) (DeviceEvent, bool) {
	if id == "" {
		return DeviceEvent{}, false
	}
	for i := range events {
		if events[i].TenantID == tenantID && events[i].GatewayID == gatewayID && events[i].ID == id {
			return events[i], true
		}
	}
	return DeviceEvent{}, false
}

func nextEventID(prefix string) (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return strings.TrimSpace(prefix) + "_" + hex.EncodeToString(raw), nil
}
