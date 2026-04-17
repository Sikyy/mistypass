package space

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrTenantIDRequired = errors.New("tenant_id is required")
var ErrBuildingIDRequired = errors.New("building_id is required")
var ErrBuildingNameRequired = errors.New("building name is required")
var ErrBuildingNotFound = errors.New("building not found")

var ErrFloorIDRequired = errors.New("floor_id is required")
var ErrFloorNameRequired = errors.New("floor name is required")
var ErrFloorNotFound = errors.New("floor not found")

var ErrAreaIDRequired = errors.New("area_id is required")
var ErrAreaNameRequired = errors.New("area name is required")
var ErrAreaNotFound = errors.New("area not found")

var ErrDoorNameRequired = errors.New("door name is required")
var ErrDoorGroupNameRequired = errors.New("door group name is required")
var ErrDoorNotFound = errors.New("door not found")
var ErrFloorBuildingMismatch = errors.New("floor does not belong to the building")
var ErrAreaFloorMismatch = errors.New("area does not belong to the floor")
var ErrTenantOwnershipMismatch = errors.New("tenant ownership mismatch in topology")
var ErrInvalidDoorKind = errors.New("invalid door kind")
var ErrInvalidDoorStatus = errors.New("invalid door status")

type Building struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Region    string    `json:"region,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Floor struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	BuildingID string    `json:"building_id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
}

type Area struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	BuildingID string    `json:"building_id"`
	FloorID    string    `json:"floor_id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
}

type Door struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	BuildingID string    `json:"building_id"`
	FloorID    string    `json:"floor_id"`
	AreaID     string    `json:"area_id"`
	Name       string    `json:"name"`
	GatewayID  string    `json:"gateway_id"`
	Kind       string    `json:"kind"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type DoorGroup struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	DoorIDs   []string  `json:"door_ids,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type TenantTopology struct {
	TenantID  string     `json:"tenant_id"`
	Buildings []Building `json:"buildings"`
	Floors    []Floor    `json:"floors"`
	Areas     []Area     `json:"areas"`
	Doors     []Door     `json:"doors"`
}

type StateStore interface {
	Load(key string, dst any) (bool, error)
	Save(key string, value any) error
}

const stateKey = "module_space"

type stateSnapshot struct {
	Buildings  []Building  `json:"buildings"`
	Floors     []Floor     `json:"floors"`
	Areas      []Area      `json:"areas"`
	Doors      []Door      `json:"doors"`
	DoorGroups []DoorGroup `json:"door_groups"`
}

type Service struct {
	mu         sync.RWMutex
	buildings  []Building
	floors     []Floor
	areas      []Area
	doors      []Door
	doorGroups []DoorGroup
	stateStore StateStore
}

func NewService() *Service {
	now := time.Now().UTC()
	return &Service{
		buildings: []Building{
			{
				ID:        "building_demo_001",
				TenantID:  "tenant_demo_jakarta",
				Name:      "Sudirman Hub",
				Address:   "Jl. Jend. Sudirman No. 23, Jakarta",
				Region:    "ID-JK",
				CreatedAt: now,
			},
			{
				ID:        "building_demo_002",
				TenantID:  "tenant_demo_jakarta",
				Name:      "Kuningan Tower",
				Address:   "Jl. HR Rasuna Said Kav. 12, Jakarta",
				Region:    "ID-JK",
				CreatedAt: now,
			},
			{
				ID:        "building_demo_003",
				TenantID:  "tenant_demo_factory",
				Name:      "Serang Plant A",
				Address:   "Jl. Industri Raya Blok A, Serang",
				Region:    "ID-BT",
				CreatedAt: now,
			},
		},
		floors: []Floor{
			{
				ID:         "floor_demo_001",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_001",
				Name:       "L8",
				CreatedAt:  now,
			},
			{
				ID:         "floor_demo_002",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_002",
				Name:       "L3",
				CreatedAt:  now,
			},
			{
				ID:         "floor_demo_003",
				TenantID:   "tenant_demo_factory",
				BuildingID: "building_demo_003",
				Name:       "F1",
				CreatedAt:  now,
			},
		},
		areas: []Area{
			{
				ID:         "area_demo_001",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_001",
				FloorID:    "floor_demo_001",
				Name:       "Finance",
				CreatedAt:  now,
			},
			{
				ID:         "area_demo_002",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_002",
				FloorID:    "floor_demo_002",
				Name:       "Lobby",
				CreatedAt:  now,
			},
			{
				ID:         "area_demo_003",
				TenantID:   "tenant_demo_factory",
				BuildingID: "building_demo_003",
				FloorID:    "floor_demo_003",
				Name:       "Packing Zone",
				CreatedAt:  now,
			},
		},
		doors: []Door{
			{
				ID:         "door_jkt_001",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_001",
				FloorID:    "floor_demo_001",
				AreaID:     "area_demo_001",
				Name:       "A-23 East Wing",
				GatewayID:  "MP-GW-JKT-0001",
				Kind:       "office",
				Status:     "online",
				CreatedAt:  now,
			},
			{
				ID:         "door_jkt_014",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_002",
				FloorID:    "floor_demo_002",
				AreaID:     "area_demo_002",
				Name:       "Turnstile Bank 2",
				GatewayID:  "MP-GW-JKT-0042",
				Kind:       "turnstile",
				Status:     "online",
				CreatedAt:  now,
			},
			{
				ID:         "door_fct_029",
				TenantID:   "tenant_demo_factory",
				BuildingID: "building_demo_003",
				FloorID:    "floor_demo_003",
				AreaID:     "area_demo_003",
				Name:       "Cargo Elevator E1",
				GatewayID:  "MP-GW-BTN-0098",
				Kind:       "elevator",
				Status:     "offline",
				CreatedAt:  now,
			},
		},
		doorGroups: []DoorGroup{
			{
				ID:        "dg_1001",
				TenantID:  "tenant_demo_jakarta",
				Name:      "Lobby Turnstiles",
				DoorIDs:   []string{"door_jkt_014"},
				CreatedAt: now,
			},
			{
				ID:        "dg_1002",
				TenantID:  "tenant_demo_factory",
				Name:      "Cargo Access",
				DoorIDs:   []string{"door_fct_029"},
				CreatedAt: now,
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

func (s *Service) ListBuildings(tenantID string) []Building {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		items := make([]Building, len(s.buildings))
		copy(items, s.buildings)
		return items
	}

	items := make([]Building, 0, len(s.buildings))
	for i := range s.buildings {
		if s.buildings[i].TenantID == nextTenantID {
			items = append(items, s.buildings[i])
		}
	}
	return items
}

func (s *Service) CreateBuilding(tenantID, name, address, region string) (Building, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return Building{}, ErrTenantIDRequired
	}

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return Building{}, ErrBuildingNameRequired
	}

	id, err := entityID("building_")
	if err != nil {
		return Building{}, err
	}

	record := Building{
		ID:        id,
		TenantID:  nextTenantID,
		Name:      nextName,
		Address:   strings.TrimSpace(address),
		Region:    strings.TrimSpace(region),
		CreatedAt: time.Now().UTC(),
	}

	s.mu.Lock()
	s.buildings = append([]Building{record}, s.buildings...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return Building{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) ListFloors(tenantID string) []Floor {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		items := make([]Floor, len(s.floors))
		copy(items, s.floors)
		return items
	}

	items := make([]Floor, 0, len(s.floors))
	for i := range s.floors {
		if s.floors[i].TenantID == nextTenantID {
			items = append(items, s.floors[i])
		}
	}
	return items
}

func (s *Service) CreateFloor(tenantID, buildingID, name string) (Floor, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return Floor{}, ErrTenantIDRequired
	}
	nextBuildingID := strings.TrimSpace(buildingID)
	if nextBuildingID == "" {
		return Floor{}, ErrBuildingIDRequired
	}
	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return Floor{}, ErrFloorNameRequired
	}

	s.mu.RLock()
	building, buildingExists := s.findBuildingLocked(nextBuildingID)
	s.mu.RUnlock()
	if !buildingExists {
		return Floor{}, ErrBuildingNotFound
	}
	if building.TenantID != nextTenantID {
		return Floor{}, ErrTenantOwnershipMismatch
	}

	id, err := entityID("floor_")
	if err != nil {
		return Floor{}, err
	}

	record := Floor{
		ID:         id,
		TenantID:   nextTenantID,
		BuildingID: nextBuildingID,
		Name:       nextName,
		CreatedAt:  time.Now().UTC(),
	}

	s.mu.Lock()
	s.floors = append([]Floor{record}, s.floors...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return Floor{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) ListAreas(tenantID string) []Area {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		items := make([]Area, len(s.areas))
		copy(items, s.areas)
		return items
	}

	items := make([]Area, 0, len(s.areas))
	for i := range s.areas {
		if s.areas[i].TenantID == nextTenantID {
			items = append(items, s.areas[i])
		}
	}
	return items
}

func (s *Service) CreateArea(tenantID, buildingID, floorID, name string) (Area, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return Area{}, ErrTenantIDRequired
	}
	nextBuildingID := strings.TrimSpace(buildingID)
	if nextBuildingID == "" {
		return Area{}, ErrBuildingIDRequired
	}
	nextFloorID := strings.TrimSpace(floorID)
	if nextFloorID == "" {
		return Area{}, ErrFloorIDRequired
	}
	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return Area{}, ErrAreaNameRequired
	}

	s.mu.RLock()
	building, buildingExists := s.findBuildingLocked(nextBuildingID)
	floor, floorExists := s.findFloorLocked(nextFloorID)
	s.mu.RUnlock()

	if !buildingExists {
		return Area{}, ErrBuildingNotFound
	}
	if !floorExists {
		return Area{}, ErrFloorNotFound
	}
	if building.TenantID != nextTenantID || floor.TenantID != nextTenantID {
		return Area{}, ErrTenantOwnershipMismatch
	}
	if floor.BuildingID != nextBuildingID {
		return Area{}, ErrFloorBuildingMismatch
	}

	id, err := entityID("area_")
	if err != nil {
		return Area{}, err
	}

	record := Area{
		ID:         id,
		TenantID:   nextTenantID,
		BuildingID: nextBuildingID,
		FloorID:    nextFloorID,
		Name:       nextName,
		CreatedAt:  time.Now().UTC(),
	}

	s.mu.Lock()
	s.areas = append([]Area{record}, s.areas...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return Area{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) ListDoors(tenantID string) []Door {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		items := make([]Door, len(s.doors))
		copy(items, s.doors)
		return items
	}

	items := make([]Door, 0, len(s.doors))
	for i := range s.doors {
		if s.doors[i].TenantID == nextTenantID {
			items = append(items, s.doors[i])
		}
	}
	return items
}

func (s *Service) ListDoorGroups(tenantID string) []DoorGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		items := make([]DoorGroup, len(s.doorGroups))
		copy(items, s.doorGroups)
		return items
	}

	items := make([]DoorGroup, 0, len(s.doorGroups))
	for i := range s.doorGroups {
		if s.doorGroups[i].TenantID == nextTenantID {
			items = append(items, s.doorGroups[i])
		}
	}
	return items
}

func (s *Service) CreateDoorGroup(tenantID, name string, doorIDs []string) (DoorGroup, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return DoorGroup{}, ErrTenantIDRequired
	}

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return DoorGroup{}, ErrDoorGroupNameRequired
	}

	nextDoorIDs := make([]string, 0, len(doorIDs))
	seen := make(map[string]struct{}, len(doorIDs))

	s.mu.RLock()
	for _, raw := range doorIDs {
		nextDoorID := strings.TrimSpace(raw)
		if nextDoorID == "" {
			continue
		}
		if _, exists := seen[nextDoorID]; exists {
			continue
		}
		door, found := s.findDoorLocked(nextDoorID)
		if !found {
			s.mu.RUnlock()
			return DoorGroup{}, ErrDoorNotFound
		}
		if door.TenantID != nextTenantID {
			s.mu.RUnlock()
			return DoorGroup{}, ErrTenantOwnershipMismatch
		}
		seen[nextDoorID] = struct{}{}
		nextDoorIDs = append(nextDoorIDs, nextDoorID)
	}
	s.mu.RUnlock()

	id, err := entityID("dg_")
	if err != nil {
		return DoorGroup{}, err
	}

	record := DoorGroup{
		ID:        id,
		TenantID:  nextTenantID,
		Name:      nextName,
		DoorIDs:   nextDoorIDs,
		CreatedAt: time.Now().UTC(),
	}

	s.mu.Lock()
	s.doorGroups = append([]DoorGroup{record}, s.doorGroups...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return DoorGroup{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) CreateDoor(
	tenantID, buildingID, floorID, areaID, name, gatewayID, kind, status string,
) (Door, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return Door{}, ErrTenantIDRequired
	}
	nextBuildingID := strings.TrimSpace(buildingID)
	if nextBuildingID == "" {
		return Door{}, ErrBuildingIDRequired
	}
	nextFloorID := strings.TrimSpace(floorID)
	if nextFloorID == "" {
		return Door{}, ErrFloorIDRequired
	}
	nextAreaID := strings.TrimSpace(areaID)
	if nextAreaID == "" {
		return Door{}, ErrAreaIDRequired
	}
	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return Door{}, ErrDoorNameRequired
	}

	nextKind, err := normalizeDoorKind(kind)
	if err != nil {
		return Door{}, err
	}
	nextStatus, err := normalizeDoorStatus(status)
	if err != nil {
		return Door{}, err
	}

	s.mu.RLock()
	building, buildingExists := s.findBuildingLocked(nextBuildingID)
	floor, floorExists := s.findFloorLocked(nextFloorID)
	area, areaExists := s.findAreaLocked(nextAreaID)
	s.mu.RUnlock()

	if !buildingExists {
		return Door{}, ErrBuildingNotFound
	}
	if !floorExists {
		return Door{}, ErrFloorNotFound
	}
	if !areaExists {
		return Door{}, ErrAreaNotFound
	}
	if building.TenantID != nextTenantID || floor.TenantID != nextTenantID || area.TenantID != nextTenantID {
		return Door{}, ErrTenantOwnershipMismatch
	}
	if floor.BuildingID != nextBuildingID {
		return Door{}, ErrFloorBuildingMismatch
	}
	if area.FloorID != nextFloorID || area.BuildingID != nextBuildingID {
		return Door{}, ErrAreaFloorMismatch
	}

	id, err := entityID("door_")
	if err != nil {
		return Door{}, err
	}

	record := Door{
		ID:         id,
		TenantID:   nextTenantID,
		BuildingID: nextBuildingID,
		FloorID:    nextFloorID,
		AreaID:     nextAreaID,
		Name:       nextName,
		GatewayID:  strings.TrimSpace(gatewayID),
		Kind:       nextKind,
		Status:     nextStatus,
		CreatedAt:  time.Now().UTC(),
	}

	s.mu.Lock()
	s.doors = append([]Door{record}, s.doors...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return Door{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) TopologyByTenant(tenantID string) (TenantTopology, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return TenantTopology{}, ErrTenantIDRequired
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	topology := TenantTopology{
		TenantID:  nextTenantID,
		Buildings: []Building{},
		Floors:    []Floor{},
		Areas:     []Area{},
		Doors:     []Door{},
	}

	for i := range s.buildings {
		if s.buildings[i].TenantID == nextTenantID {
			topology.Buildings = append(topology.Buildings, s.buildings[i])
		}
	}
	for i := range s.floors {
		if s.floors[i].TenantID == nextTenantID {
			topology.Floors = append(topology.Floors, s.floors[i])
		}
	}
	for i := range s.areas {
		if s.areas[i].TenantID == nextTenantID {
			topology.Areas = append(topology.Areas, s.areas[i])
		}
	}
	for i := range s.doors {
		if s.doors[i].TenantID == nextTenantID {
			topology.Doors = append(topology.Doors, s.doors[i])
		}
	}

	return topology, nil
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
			Buildings:  cloneBuildings(s.buildings),
			Floors:     cloneFloors(s.floors),
			Areas:      cloneAreas(s.areas),
			Doors:      cloneDoors(s.doors),
			DoorGroups: cloneDoorGroups(s.doorGroups),
		})
	}

	s.mu.Lock()
	s.buildings = cloneBuildings(snapshot.Buildings)
	s.floors = cloneFloors(snapshot.Floors)
	s.areas = cloneAreas(snapshot.Areas)
	s.doors = cloneDoors(snapshot.Doors)
	s.doorGroups = cloneDoorGroups(snapshot.DoorGroups)
	s.mu.Unlock()
	return nil
}

func (s *Service) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(stateKey, stateSnapshot{
		Buildings:  cloneBuildings(s.buildings),
		Floors:     cloneFloors(s.floors),
		Areas:      cloneAreas(s.areas),
		Doors:      cloneDoors(s.doors),
		DoorGroups: cloneDoorGroups(s.doorGroups),
	})
}

func cloneBuildings(items []Building) []Building {
	output := make([]Building, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneFloors(items []Floor) []Floor {
	output := make([]Floor, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneAreas(items []Area) []Area {
	output := make([]Area, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneDoors(items []Door) []Door {
	output := make([]Door, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneDoorGroups(items []DoorGroup) []DoorGroup {
	output := make([]DoorGroup, 0, len(items))
	for i := range items {
		record := items[i]
		record.DoorIDs = append([]string(nil), items[i].DoorIDs...)
		output = append(output, record)
	}
	return output
}

func entityID(prefix string) (string, error) {
	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(raw), nil
}

func (s *Service) findBuildingLocked(buildingID string) (Building, bool) {
	for i := range s.buildings {
		if s.buildings[i].ID == buildingID {
			return s.buildings[i], true
		}
	}
	return Building{}, false
}

func (s *Service) findFloorLocked(floorID string) (Floor, bool) {
	for i := range s.floors {
		if s.floors[i].ID == floorID {
			return s.floors[i], true
		}
	}
	return Floor{}, false
}

func (s *Service) findAreaLocked(areaID string) (Area, bool) {
	for i := range s.areas {
		if s.areas[i].ID == areaID {
			return s.areas[i], true
		}
	}
	return Area{}, false
}

func (s *Service) findDoorLocked(doorID string) (Door, bool) {
	for i := range s.doors {
		if s.doors[i].ID == doorID {
			return s.doors[i], true
		}
	}
	return Door{}, false
}

func normalizeDoorKind(kind string) (string, error) {
	nextKind := strings.ToLower(strings.TrimSpace(kind))
	if nextKind == "" {
		return "office", nil
	}

	switch nextKind {
	case "office", "turnstile", "server-room", "elevator", "parking-gate", "emergency-exit":
		return nextKind, nil
	default:
		return "", ErrInvalidDoorKind
	}
}

func normalizeDoorStatus(status string) (string, error) {
	nextStatus := strings.ToLower(strings.TrimSpace(status))
	if nextStatus == "" {
		return "offline", nil
	}

	switch nextStatus {
	case "online", "offline":
		return nextStatus, nil
	default:
		return "", ErrInvalidDoorStatus
	}
}
