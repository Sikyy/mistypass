package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrSerialNumberRequired = errors.New("serial_number is required")
var ErrSerialNumberInvalid = errors.New("serial_number format is invalid")
var ErrGatewaySerialAlreadyRegistered = errors.New("gateway serial_number already registered")
var ErrSerialNumberNotProvisioned = errors.New("serial_number is not provisioned")
var ErrSerialNumberAlreadyConsumed = errors.New("serial_number already consumed")
var ErrSerialNumberNotAvailable = errors.New("serial_number is not available")
var ErrSerialNumberTenantMismatch = errors.New("serial_number tenant mismatch")
var ErrSerialNumberProductTypeMismatch = errors.New("serial_number product_type mismatch")
var ErrTenantIDRequired = errors.New("tenant_id is required")
var ErrGatewayNotFound = errors.New("gateway not found")
var ErrGatewayIDRequired = errors.New("gateway_id is required")
var ErrGatewayStatusRequired = errors.New("gateway status is required")
var ErrGatewayStatusInvalid = errors.New("invalid gateway status")
var ErrDoorIDRequired = errors.New("door_id is required")
var ErrConfigVersionRequired = errors.New("config version is required")
var ErrInvalidDeviceCapacity = errors.New("device_capacity must be 4 or 8")
var ErrGatewayDeviceSerialRequired = errors.New("device serial_number is required")
var ErrGatewayDeviceIDRequired = errors.New("gateway device_id is required")
var ErrGatewayDeviceSerialConflict = errors.New("device serial_number already registered on another gateway")
var ErrGatewayDeviceCapacityExceeded = errors.New("gateway device capacity exceeded")
var ErrGatewayDeviceNotFound = errors.New("gateway device not found")
var ErrGatewayDeviceKindInvalid = errors.New("invalid gateway device kind")
var ErrGatewayDeviceSourceInvalid = errors.New("invalid gateway device source")
var ErrGatewayDeviceStatusInvalid = errors.New("invalid gateway device status")
var ErrGatewayDeviceProtocolInvalid = errors.New("invalid gateway device protocol")
var ErrGatewayDeviceRS485ConfigInvalid = errors.New("invalid gateway device rs485_config")
var ErrGatewayDeviceRS485ConfigProtocolMismatch = errors.New("rs485_config requires protocol=rs485")
var ErrGatewayDeviceRS485TelemetryInvalid = errors.New("invalid gateway device rs485 telemetry")
var ErrGatewayDeviceRS485TelemetryProtocolMismatch = errors.New("rs485 telemetry requires protocol=rs485")
var ErrGatewayDeviceSerialNotProvisioned = errors.New("device serial_number is not provisioned")
var ErrGatewayDeviceSerialNotAvailable = errors.New("device serial_number is not available")
var ErrGatewayDeviceSerialProductTypeMismatch = errors.New("device serial_number product_type mismatch")
var ErrSerialInventoryRecordsRequired = errors.New("serial inventory records are required")
var ErrSerialInventoryProductTypeInvalid = errors.New("invalid serial inventory product_type")
var ErrSerialInventoryStatusInvalid = errors.New("invalid serial inventory status")
var ErrSerialInventoryTenantMismatch = errors.New("serial inventory tenant mismatch")
var ErrSerialInventoryProductTypeMismatch = errors.New("serial inventory product_type mismatch")
var ErrSerialInventoryNotFound = errors.New("serial inventory not found")
var ErrSerialInventoryStatusTransitionInvalid = errors.New("serial inventory status transition invalid")
var ErrSerialInventorySerialNumbersRequired = errors.New("serial inventory serial_numbers are required")
var ErrGatewayEventCheckpointRequired = errors.New("event checkpoint is required")
var ErrGatewayEventCheckpointQueueRequired = errors.New("event checkpoint queue is required")
var ErrGatewayEventCheckpointNotFound = errors.New("event checkpoint not found")
var ErrGatewayEventCheckpointAckedCountInvalid = errors.New("event checkpoint acked_count is invalid")
var ErrGatewayEventCheckpointAckedCountRegression = errors.New("event checkpoint acked_count regression")
var ErrGatewayQueueIngestTotalNotFound = errors.New("gateway queue ingest total not found")
var ErrGatewayQueueIngestDeltaInvalid = errors.New("gateway queue ingest delta is invalid")
var ErrGatewayOTATaskIDRequired = errors.New("gateway ota task_id is required")
var ErrGatewayOTAFirmwareVersionRequired = errors.New("gateway ota firmware_version is required")
var ErrGatewayOTAFirmwareURLRequired = errors.New("gateway ota firmware_url is required")
var ErrGatewayOTAFirmwareSHA256Required = errors.New("gateway ota firmware_sha256 is required")
var ErrGatewayOTAFirmwareSHA256Invalid = errors.New("gateway ota firmware_sha256 is invalid")
var ErrGatewayOTAFirmwareSignatureRequired = errors.New("gateway ota firmware_signature is required")
var ErrGatewayOTAFirmwareSignatureInvalid = errors.New("gateway ota firmware_signature is invalid (expected hex-encoded Ed25519 signature)")
var ErrGatewayOTATaskStatusInvalid = errors.New("gateway ota task status is invalid")
var ErrGatewayOTATaskNotFound = errors.New("gateway ota task not found")
var ErrGatewayCertificateSerialRequired = errors.New("gateway certificate serial_number is required")
var ErrGatewayCertificateSerialInvalid = errors.New("gateway certificate serial_number is invalid")
var ErrGatewayCertificateSerialNotFound = errors.New("gateway certificate serial_number revocation not found")
var ErrGatewayCertificateSerialConflict = errors.New("gateway certificate serial_number already revoked for another tenant")

const defaultDeviceCapacity = 4

const (
	rs485AlertConsecutiveTimeoutThreshold = 3
	rs485AlertCollisionThreshold          = 5
	rs485CriticalConsecutiveTimeouts      = 6
	rs485CriticalCollisions               = 10
)

const (
	serialInventoryProductGateway    = "gateway"
	serialInventoryProductReader     = "reader"
	serialInventoryProductController = "controller"
	serialInventoryProductRelay      = "relay"
	serialInventoryProductSensor     = "sensor"
)

const (
	serialInventoryStatusAvailable = "available"
	serialInventoryStatusConsumed  = "consumed"
	serialInventoryStatusFrozen    = "frozen"
	serialInventoryStatusScrapped  = "scrapped"
)

const (
	gatewayOTATaskStatusQueued      = "queued"
	gatewayOTATaskStatusDispatching = "dispatching"
	gatewayOTATaskStatusSucceeded   = "succeeded"
	gatewayOTATaskStatusFailed      = "failed"
	gatewayOTATaskStatusCanceled    = "canceled"
)

type GatewayDevice struct {
	ID           string       `json:"id"`
	GatewayID    string       `json:"gateway_id"`
	SerialNumber string       `json:"serial_number"`
	Kind         string       `json:"kind"`
	Source       string       `json:"source"`
	Protocol     string       `json:"protocol"`
	RS485Config  *RS485Config `json:"rs485_config,omitempty"`
	RS485Health  *RS485Health `json:"rs485_health,omitempty"`
	Status       string       `json:"status"`
	LastSeenAt   time.Time    `json:"last_seen_at"`
}

type RS485Config struct {
	BaudRate      int    `json:"baud_rate"`
	Parity        string `json:"parity"`
	StopBits      int    `json:"stop_bits"`
	DeviceAddress int    `json:"device_address"`
	TimeoutMS     int    `json:"timeout_ms"`
}

type RS485Health struct {
	RetryCount          int       `json:"retry_count"`
	TimeoutCount        int       `json:"timeout_count"`
	CollisionCount      int       `json:"collision_count"`
	ConsecutiveTimeouts int       `json:"consecutive_timeouts"`
	LastError           string    `json:"last_error,omitempty"`
	LastReportAt        time.Time `json:"last_report_at"`
}

type RS485TelemetryReport struct {
	Retries                  int    `json:"retries"`
	Timeouts                 int    `json:"timeouts"`
	Collisions               int    `json:"collisions"`
	LastError                string `json:"last_error"`
	ResetConsecutiveTimeouts bool   `json:"reset_consecutive_timeouts"`
}

type ProtocolLineQualityPolicy struct {
	WarningConsecutiveTimeouts  int `json:"warning_consecutive_timeouts"`
	WarningCollisions           int `json:"warning_collisions"`
	CriticalConsecutiveTimeouts int `json:"critical_consecutive_timeouts"`
	CriticalCollisions          int `json:"critical_collisions"`
}

type DeviceTelemetrySummary struct {
	Protocol         string                    `json:"protocol"`
	AlertLevel       string                    `json:"alert_level"`
	LineQuality      string                    `json:"line_quality"`
	GovernanceAction string                    `json:"governance_action"`
	Degraded         bool                      `json:"degraded"`
	ReasonCodes      []string                  `json:"reason_codes,omitempty"`
	LinePolicy       ProtocolLineQualityPolicy `json:"line_policy"`
	LastReportAt     *time.Time                `json:"last_report_at,omitempty"`
}

type LegacyProbeGovernance struct {
	LegacyProtocol      string                    `json:"legacy_protocol"`
	UpgradeProtocol     string                    `json:"upgrade_protocol"`
	OfflineFallback     string                    `json:"offline_fallback"`
	DegradedLineAction  string                    `json:"degraded_line_action"`
	RecommendedTopology string                    `json:"recommended_topology"`
	LinePolicy          ProtocolLineQualityPolicy `json:"line_policy"`
}

type LegacyProbeResult struct {
	Items      []string              `json:"items"`
	Governance LegacyProbeGovernance `json:"governance"`
}

type Gateway struct {
	ID             string          `json:"id"`
	TenantID       string          `json:"tenant_id"`
	SerialNumber   string          `json:"serial_number"`
	BuildingID     string          `json:"building_id"`
	DeviceCapacity int             `json:"device_capacity"`
	Devices        []GatewayDevice `json:"devices,omitempty"`
	Status         string          `json:"status"`
	LastSeenAt     time.Time       `json:"last_seen_at"`
	BoundDoorIDs   []string        `json:"bound_door_ids,omitempty"`

	CurrentFirmwareVersion string    `json:"current_firmware_version,omitempty"`
	FirmwareReportedAt     time.Time `json:"firmware_reported_at,omitempty"`
}

type SerialInventoryItem struct {
	ID                string     `json:"id"`
	TenantID          string     `json:"tenant_id"`
	SerialNumber      string     `json:"serial_number"`
	ProductType       string     `json:"product_type"`
	Status            string     `json:"status"`
	BatchCode         string     `json:"batch_code,omitempty"`
	Source            string     `json:"source,omitempty"`
	ConsumedGatewayID string     `json:"consumed_gateway_id,omitempty"`
	ConsumedAt        *time.Time `json:"consumed_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type SerialInventoryImportItem struct {
	SerialNumber string `json:"serial_number"`
	ProductType  string `json:"product_type"`
	BatchCode    string `json:"batch_code"`
	Source       string `json:"source"`
}

type CommandAck struct {
	TaskID    string    `json:"task_id"`
	GatewayID string    `json:"gateway_id"`
	Command   string    `json:"command"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type GatewayConfigState struct {
	GatewayID        string     `json:"gateway_id"`
	DesiredVersion   string     `json:"desired_version,omitempty"`
	DesiredUpdatedAt *time.Time `json:"desired_updated_at,omitempty"`
	AppliedVersion   string     `json:"applied_version,omitempty"`
	AppliedAt        *time.Time `json:"applied_at,omitempty"`
}

type GatewayConfigSnapshot struct {
	GatewayID        string          `json:"gateway_id"`
	TenantID         string          `json:"tenant_id"`
	DesiredVersion   string          `json:"desired_version,omitempty"`
	DesiredUpdatedAt *time.Time      `json:"desired_updated_at,omitempty"`
	AppliedVersion   string          `json:"applied_version,omitempty"`
	AppliedAt        *time.Time      `json:"applied_at,omitempty"`
	BoundDoorIDs     []string        `json:"bound_door_ids,omitempty"`
	Devices          []GatewayDevice `json:"devices,omitempty"`
}

type GatewayEventCheckpoint struct {
	GatewayID      string     `json:"gateway_id"`
	Queue          string     `json:"queue"`
	CheckpointID   string     `json:"checkpoint_id"`
	LastRequestID  string     `json:"last_request_id,omitempty"`
	AckedCount     int        `json:"acked_count"`
	LastOccurredAt *time.Time `json:"last_occurred_at,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type GatewayQueueIngestTotal struct {
	GatewayID     string    `json:"gateway_id"`
	Queue         string    `json:"queue"`
	IngestedTotal int       `json:"ingested_total"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type GatewayOTATask struct {
	ID                string    `json:"id"`
	GatewayID         string    `json:"gateway_id"`
	TenantID          string    `json:"tenant_id"`
	FirmwareVersion   string    `json:"firmware_version"`
	FirmwareURL       string    `json:"firmware_url"`
	FirmwareSHA256    string    `json:"firmware_sha256,omitempty"`
	FirmwareSignature string    `json:"firmware_signature,omitempty"` // Ed25519 signature of firmware binary (hex-encoded)
	Status            string    `json:"status"`
	ErrorMessage      string    `json:"error_message,omitempty"`
	RequestedBy       string    `json:"requested_by,omitempty"`
	UpdatedBy         string    `json:"updated_by,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type StateStore interface {
	Load(key string, dst any) (bool, error)
	Save(key string, value any) error
}

const stateKey = "module_gateway"

type stateSnapshot struct {
	Gateways               []Gateway                      `json:"gateways"`
	SerialInventory        []SerialInventoryItem          `json:"serial_inventory,omitempty"`
	CertificateRevocations []GatewayCertificateRevocation `json:"certificate_revocations,omitempty"`
	ConfigStates           []GatewayConfigState           `json:"config_states,omitempty"`
	EventCheckpoints       []GatewayEventCheckpoint       `json:"event_checkpoints,omitempty"`
	QueueIngestTotals      []GatewayQueueIngestTotal      `json:"queue_ingest_totals,omitempty"`
	OTATasks               []GatewayOTATask               `json:"ota_tasks,omitempty"`
	Firmwares              []GatewayFirmware              `json:"firmwares,omitempty"`
}

type Service struct {
	mu                     sync.RWMutex
	gateways               []Gateway
	serialInventory        []SerialInventoryItem
	certificateRevocations []GatewayCertificateRevocation
	configStates           []GatewayConfigState
	eventCheckpoints       []GatewayEventCheckpoint
	queueIngestTotals      []GatewayQueueIngestTotal
	otaTasks               []GatewayOTATask
	firmwares              []GatewayFirmware
	stateStore             StateStore
}

func NewService() *Service {
	now := time.Now().UTC()
	gateways := []Gateway{
		{
			ID:             "gw_demo_001",
			TenantID:       "tenant_demo_jakarta",
			SerialNumber:   "MP-GW-JKT-0001",
			BuildingID:     "building_demo_001",
			DeviceCapacity: 8,
			BoundDoorIDs:   []string{"door_jkt_001", "door_jkt_002", "door_jkt_003", "door_jkt_004", "door_jkt_005", "door_jkt_006", "door_jkt_014"},
			Devices: []GatewayDevice{
				{
					ID:           "gdv_demo_001",
					GatewayID:    "gw_demo_001",
					SerialNumber: "RD-JKT-001",
					Kind:         "reader",
					Source:       "mistypass_procured",
					Protocol:     "osdp_v2",
					Status:       "online",
					LastSeenAt:   now,
				},
				{
					ID:           "gdv_demo_002",
					GatewayID:    "gw_demo_001",
					SerialNumber: "RD-JKT-002",
					Kind:         "legacy_reader",
					Source:       "legacy_integration",
					Protocol:     "wiegand_26",
					Status:       "offline",
					LastSeenAt:   now.Add(-8 * time.Minute),
				},
			},
			Status:     "online",
			LastSeenAt: now,
		},
		{
			ID:             "gw_demo_002",
			TenantID:       "tenant_demo_factory",
			SerialNumber:   "MP-GW-BTN-0001",
			BuildingID:     "building_demo_003",
			DeviceCapacity: 4,
			Devices: []GatewayDevice{
				{
					ID:           "gdv_demo_003",
					GatewayID:    "gw_demo_002",
					SerialNumber: "RD-BTN-100",
					Kind:         "reader",
					Source:       "mistypass_procured",
					Protocol:     "osdp_v2",
					Status:       "offline",
					LastSeenAt:   now.Add(-15 * time.Minute),
				},
			},
			Status:     "offline",
			LastSeenAt: now.Add(-12 * time.Minute),
		},
	}

	return &Service{
		gateways:               gateways,
		serialInventory:        serialInventoryFromGateways(gateways),
		certificateRevocations: []GatewayCertificateRevocation{},
		configStates:           []GatewayConfigState{},
		eventCheckpoints:       []GatewayEventCheckpoint{},
		queueIngestTotals:      []GatewayQueueIngestTotal{},
		otaTasks:               []GatewayOTATask{},
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

func (s *Service) List(tenantID string) []Gateway {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]Gateway, 0, len(s.gateways))
	for i := range s.gateways {
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.gateways[i])
	}
	return items
}

func (s *Service) FindGatewayByID(tenantID, gatewayID string) (Gateway, bool) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextGatewayID := strings.TrimSpace(gatewayID)
	if nextGatewayID == "" {
		return Gateway{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.gateways {
		if s.gateways[i].ID != nextGatewayID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			continue
		}
		return s.gateways[i], true
	}
	return Gateway{}, false
}

func (s *Service) FindGatewayByDoorID(tenantID, doorID string) (Gateway, bool) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextDoorID := strings.TrimSpace(doorID)
	if nextDoorID == "" {
		return Gateway{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.gateways {
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			continue
		}
		for _, boundDoorID := range s.gateways[i].BoundDoorIDs {
			if boundDoorID == nextDoorID {
				return s.gateways[i], true
			}
		}
	}
	return Gateway{}, false
}

func (s *Service) UpdateGatewayStatus(tenantID, gatewayID, status string) (Gateway, error) {
	nextGatewayID := strings.TrimSpace(gatewayID)
	if nextGatewayID == "" {
		return Gateway{}, ErrGatewayIDRequired
	}
	nextStatus, err := normalizeGatewayStatus(status)
	if err != nil {
		return Gateway{}, err
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.gateways {
		if s.gateways[i].ID != nextGatewayID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return Gateway{}, ErrGatewayNotFound
		}
		s.gateways[i].Status = nextStatus
		s.gateways[i].LastSeenAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return Gateway{}, err
		}
		return cloneGateway(s.gateways[i]), nil
	}

	return Gateway{}, ErrGatewayNotFound
}

// RecordFirmwareVersion stores the running firmware version a gateway reported.
// An empty version is ignored so an older agent that doesn't report can't clobber it.
func (s *Service) RecordFirmwareVersion(tenantID, gatewayID, version string) error {
	nextGatewayID := strings.TrimSpace(gatewayID)
	if nextGatewayID == "" {
		return ErrGatewayIDRequired
	}
	nextVersion := strings.TrimSpace(version)
	if nextVersion == "" {
		return nil
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.gateways {
		if s.gateways[i].ID != nextGatewayID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return ErrGatewayNotFound
		}
		s.gateways[i].CurrentFirmwareVersion = nextVersion
		s.gateways[i].FirmwareReportedAt = time.Now().UTC()
		return s.persistLocked()
	}
	return ErrGatewayNotFound
}

// GatewayFirmwareVersionCount is one firmware version and how many gateways run it.
type GatewayFirmwareVersionCount struct {
	Version string `json:"version"`
	Count   int    `json:"count"`
}

// GatewayFirmwareSummary is the firmware-version distribution across a tenant's gateways.
type GatewayFirmwareSummary struct {
	Total    int                           `json:"total"`
	Reported int                           `json:"reported"`
	Versions []GatewayFirmwareVersionCount `json:"versions"`
}

// FirmwareSummary returns the firmware-version distribution for a tenant's gateways,
// sorted by count desc (version asc as a stable tiebreak).
func (s *Service) FirmwareSummary(tenantID string) GatewayFirmwareSummary {
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := map[string]int{}
	summary := GatewayFirmwareSummary{}
	for i := range s.gateways {
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			continue
		}
		summary.Total++
		v := strings.TrimSpace(s.gateways[i].CurrentFirmwareVersion)
		if v == "" {
			continue
		}
		summary.Reported++
		counts[v]++
	}
	for v, c := range counts {
		summary.Versions = append(summary.Versions, GatewayFirmwareVersionCount{Version: v, Count: c})
	}
	sort.Slice(summary.Versions, func(a, b int) bool {
		if summary.Versions[a].Count != summary.Versions[b].Count {
			return summary.Versions[a].Count > summary.Versions[b].Count
		}
		return summary.Versions[a].Version < summary.Versions[b].Version
	})
	return summary
}

func (s *Service) ListSerialInventory(tenantID, productType, status string) ([]SerialInventoryItem, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	filterProductType := ""
	filterStatus := ""
	var err error

	if strings.TrimSpace(productType) != "" {
		filterProductType, err = normalizeSerialInventoryProductType(productType)
		if err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(status) != "" {
		filterStatus, err = normalizeSerialInventoryStatus(status)
		if err != nil {
			return nil, err
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]SerialInventoryItem, 0, len(s.serialInventory))
	for i := range s.serialInventory {
		if filterTenantID != "" && s.serialInventory[i].TenantID != filterTenantID {
			continue
		}
		if filterProductType != "" && s.serialInventory[i].ProductType != filterProductType {
			continue
		}
		if filterStatus != "" && s.serialInventory[i].Status != filterStatus {
			continue
		}
		items = append(items, s.serialInventory[i])
	}
	return items, nil
}

func (s *Service) ImportSerialInventory(tenantID string, records []SerialInventoryImportItem) ([]SerialInventoryItem, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return nil, ErrTenantIDRequired
	}
	if len(records) == 0 {
		return nil, ErrSerialInventoryRecordsRequired
	}

	now := time.Now().UTC()
	createdOrUpdated := make([]SerialInventoryItem, 0, len(records))

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range records {
		productType, err := normalizeSerialInventoryProductType(records[i].ProductType)
		if err != nil {
			return nil, err
		}

		serial, err := normalizeInventorySerial(productType, records[i].SerialNumber)
		if err != nil {
			return nil, err
		}

		existingIdx := s.findSerialInventoryIndexLocked(serial)
		if existingIdx >= 0 {
			if s.serialInventory[existingIdx].TenantID != nextTenantID {
				return nil, ErrSerialInventoryTenantMismatch
			}
			if s.serialInventory[existingIdx].ProductType != productType {
				return nil, ErrSerialInventoryProductTypeMismatch
			}
			if s.serialInventory[existingIdx].Status == serialInventoryStatusAvailable {
				s.serialInventory[existingIdx].BatchCode = strings.TrimSpace(records[i].BatchCode)
				s.serialInventory[existingIdx].Source = strings.TrimSpace(records[i].Source)
				if s.serialInventory[existingIdx].Source == "" {
					s.serialInventory[existingIdx].Source = "factory"
				}
				s.serialInventory[existingIdx].UpdatedAt = now
			}
			createdOrUpdated = append(createdOrUpdated, s.serialInventory[existingIdx])
			continue
		}

		id, err := serialInventoryID()
		if err != nil {
			return nil, err
		}

		record := SerialInventoryItem{
			ID:           id,
			TenantID:     nextTenantID,
			SerialNumber: serial,
			ProductType:  productType,
			Status:       serialInventoryStatusAvailable,
			BatchCode:    strings.TrimSpace(records[i].BatchCode),
			Source:       strings.TrimSpace(records[i].Source),
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if record.Source == "" {
			record.Source = "factory"
		}
		s.serialInventory = append([]SerialInventoryItem{record}, s.serialInventory...)
		createdOrUpdated = append(createdOrUpdated, record)
	}

	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return createdOrUpdated, nil
}

func (s *Service) UpdateSerialInventoryStatus(tenantID, serialNumber, status, consumedGatewayID string) (SerialInventoryItem, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return SerialInventoryItem{}, ErrTenantIDRequired
	}
	normalizedSerial := strings.ToUpper(strings.TrimSpace(serialNumber))
	if normalizedSerial == "" {
		return SerialInventoryItem{}, ErrSerialNumberRequired
	}
	nextStatus, err := normalizeSerialInventoryStatus(status)
	if err != nil {
		return SerialInventoryItem{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.findSerialInventoryIndexLocked(normalizedSerial)
	if idx < 0 {
		return SerialInventoryItem{}, ErrSerialInventoryNotFound
	}
	if s.serialInventory[idx].TenantID != nextTenantID {
		return SerialInventoryItem{}, ErrSerialInventoryTenantMismatch
	}

	current := s.serialInventory[idx]
	if current.Status == nextStatus {
		return current, nil
	}

	now := time.Now().UTC()
	if err := applySerialInventoryStatusTransition(
		&s.serialInventory[idx],
		nextStatus,
		consumedGatewayID,
		now,
	); err != nil {
		return SerialInventoryItem{}, err
	}

	if err := s.persistLocked(); err != nil {
		return SerialInventoryItem{}, err
	}
	return s.serialInventory[idx], nil
}

func (s *Service) BatchUpdateSerialInventoryStatus(tenantID string, serialNumbers []string, status, consumedGatewayID string) ([]SerialInventoryItem, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return nil, ErrTenantIDRequired
	}
	nextStatus, err := normalizeSerialInventoryStatus(status)
	if err != nil {
		return nil, err
	}

	normalizedSerials := make([]string, 0, len(serialNumbers))
	seen := make(map[string]struct{}, len(serialNumbers))
	for i := range serialNumbers {
		serial := strings.ToUpper(strings.TrimSpace(serialNumbers[i]))
		if serial == "" {
			continue
		}
		if _, exists := seen[serial]; exists {
			continue
		}
		seen[serial] = struct{}{}
		normalizedSerials = append(normalizedSerials, serial)
	}
	if len(normalizedSerials) == 0 {
		return nil, ErrSerialInventorySerialNumbersRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	indexes := make([]int, 0, len(normalizedSerials))
	for i := range normalizedSerials {
		idx := s.findSerialInventoryIndexLocked(normalizedSerials[i])
		if idx < 0 {
			return nil, ErrSerialInventoryNotFound
		}
		if s.serialInventory[idx].TenantID != nextTenantID {
			return nil, ErrSerialInventoryTenantMismatch
		}
		indexes = append(indexes, idx)
	}

	now := time.Now().UTC()
	for i := range indexes {
		if err := applySerialInventoryStatusTransition(
			&s.serialInventory[indexes[i]],
			nextStatus,
			consumedGatewayID,
			now,
		); err != nil {
			return nil, err
		}
	}

	if err := s.persistLocked(); err != nil {
		return nil, err
	}

	updated := make([]SerialInventoryItem, 0, len(indexes))
	for i := range indexes {
		updated = append(updated, s.serialInventory[indexes[i]])
	}
	return updated, nil
}

func (s *Service) Register(serialNumber, tenantID, buildingID string, deviceCapacity int) (Gateway, error) {
	if strings.TrimSpace(serialNumber) == "" {
		return Gateway{}, ErrSerialNumberRequired
	}
	serial, err := normalizeGatewaySerial(serialNumber)
	if err != nil {
		return Gateway{}, err
	}
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return Gateway{}, ErrTenantIDRequired
	}
	nextCapacity, err := normalizeDeviceCapacity(deviceCapacity)
	if err != nil {
		return Gateway{}, err
	}

	id, err := gatewayID()
	if err != nil {
		return Gateway{}, err
	}

	record := Gateway{
		ID:             id,
		TenantID:       nextTenantID,
		SerialNumber:   serial,
		BuildingID:     strings.TrimSpace(buildingID),
		DeviceCapacity: nextCapacity,
		Status:         "online",
		LastSeenAt:     time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.gateways {
		if strings.EqualFold(s.gateways[i].SerialNumber, serial) {
			return Gateway{}, ErrGatewaySerialAlreadyRegistered
		}
	}
	if err := s.consumeGatewaySerialLocked(serial, nextTenantID, id); err != nil {
		return Gateway{}, err
	}
	s.gateways = append(s.gateways, record)
	if err := s.persistLocked(); err != nil {
		return Gateway{}, err
	}

	return record, nil
}

func (s *Service) Deassign(tenantID, gatewayID string) (Gateway, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return Gateway{}, ErrGatewayIDRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return Gateway{}, ErrGatewayNotFound
		}

		removed := cloneGateway(s.gateways[i])
		now := time.Now().UTC()
		s.releaseSerialInventoryLocked(removed.SerialNumber, removed.TenantID, now)
		for d := range removed.Devices {
			if removed.Devices[d].Source == "mistypass_procured" {
				s.releaseSerialInventoryLocked(removed.Devices[d].SerialNumber, removed.TenantID, now)
			}
		}

		s.gateways = append(s.gateways[:i], s.gateways[i+1:]...)
		s.removeGatewayRuntimeStateLocked(gwID)
		if err := s.persistLocked(); err != nil {
			return Gateway{}, err
		}
		return removed, nil
	}

	return Gateway{}, ErrGatewayNotFound
}

func (s *Service) BindDoor(tenantID, gatewayID, doorID string) (Gateway, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return Gateway{}, ErrGatewayIDRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)

	nextDoorID := strings.TrimSpace(doorID)
	if nextDoorID == "" {
		return Gateway{}, ErrDoorIDRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return Gateway{}, ErrGatewayNotFound
		}

		for _, existing := range s.gateways[i].BoundDoorIDs {
			if existing == nextDoorID {
				return s.gateways[i], nil
			}
		}

		s.gateways[i].BoundDoorIDs = append(s.gateways[i].BoundDoorIDs, nextDoorID)
		s.gateways[i].LastSeenAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return Gateway{}, err
		}
		return s.gateways[i], nil
	}

	return Gateway{}, ErrGatewayNotFound
}

func (s *Service) UnbindDoor(tenantID, gatewayID, doorID string) (Gateway, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return Gateway{}, ErrGatewayIDRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)

	nextDoorID := strings.TrimSpace(doorID)
	if nextDoorID == "" {
		return Gateway{}, ErrDoorIDRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return Gateway{}, ErrGatewayNotFound
		}

		nextBound := make([]string, 0, len(s.gateways[i].BoundDoorIDs))
		for _, existing := range s.gateways[i].BoundDoorIDs {
			if existing == nextDoorID {
				continue
			}
			nextBound = append(nextBound, existing)
		}
		s.gateways[i].BoundDoorIDs = nextBound
		s.gateways[i].LastSeenAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return Gateway{}, err
		}
		return s.gateways[i], nil
	}

	return Gateway{}, ErrGatewayNotFound
}

func (s *Service) RegisterDevice(
	tenantID,
	gatewayID,
	serialNumber,
	kind,
	source,
	status,
	protocol string,
	rs485Config *RS485Config,
) (Gateway, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return Gateway{}, ErrGatewayIDRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)

	serial := strings.TrimSpace(serialNumber)
	if serial == "" {
		return Gateway{}, ErrGatewayDeviceSerialRequired
	}

	nextKind, err := normalizeDeviceKind(kind)
	if err != nil {
		return Gateway{}, err
	}
	nextSource, err := normalizeDeviceSource(source)
	if err != nil {
		return Gateway{}, err
	}
	nextStatus, err := normalizeDeviceStatus(status)
	if err != nil {
		return Gateway{}, err
	}
	nextProtocol, err := normalizeDeviceProtocol(nextKind, nextSource, protocol)
	if err != nil {
		return Gateway{}, err
	}
	nextRS485Config, err := normalizeRS485Config(nextProtocol, rs485Config)
	if err != nil {
		return Gateway{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return Gateway{}, ErrGatewayNotFound
		}
		for g := range s.gateways {
			for d := range s.gateways[g].Devices {
				if !strings.EqualFold(s.gateways[g].Devices[d].SerialNumber, serial) {
					continue
				}
				if s.gateways[g].ID != gwID {
					return Gateway{}, ErrGatewayDeviceSerialConflict
				}
			}
		}

		gatewayTenantID := s.gateways[i].TenantID
		for d := range s.gateways[i].Devices {
			if strings.EqualFold(s.gateways[i].Devices[d].SerialNumber, serial) {
				if err := s.consumeDeviceSerialLocked(serial, gatewayTenantID, gwID, nextKind, nextSource); err != nil {
					return Gateway{}, err
				}
				s.gateways[i].Devices[d].Status = nextStatus
				s.gateways[i].Devices[d].Protocol = nextProtocol
				s.gateways[i].Devices[d].RS485Config = nextRS485Config
				s.gateways[i].Devices[d].LastSeenAt = time.Now().UTC()
				if err := s.persistLocked(); err != nil {
					return Gateway{}, err
				}
				return s.gateways[i], nil
			}
		}

		if len(s.gateways[i].Devices) >= s.gateways[i].DeviceCapacity {
			return Gateway{}, ErrGatewayDeviceCapacityExceeded
		}
		if err := s.consumeDeviceSerialLocked(serial, gatewayTenantID, gwID, nextKind, nextSource); err != nil {
			return Gateway{}, err
		}

		deviceID, err := gatewayDeviceID()
		if err != nil {
			return Gateway{}, err
		}
		record := GatewayDevice{
			ID:           deviceID,
			GatewayID:    gwID,
			SerialNumber: serial,
			Kind:         nextKind,
			Source:       nextSource,
			Protocol:     nextProtocol,
			RS485Config:  nextRS485Config,
			Status:       nextStatus,
			LastSeenAt:   time.Now().UTC(),
		}
		s.gateways[i].Devices = append([]GatewayDevice{record}, s.gateways[i].Devices...)
		s.gateways[i].LastSeenAt = time.Now().UTC()
		if nextStatus == "online" {
			s.gateways[i].Status = "online"
		}
		if err := s.persistLocked(); err != nil {
			return Gateway{}, err
		}
		return s.gateways[i], nil
	}

	return Gateway{}, ErrGatewayNotFound
}

func (s *Service) DeassignDevice(tenantID, deviceID string) (Gateway, GatewayDevice, error) {
	devID := strings.TrimSpace(deviceID)
	devID = strings.TrimPrefix(devID, "terminal_")
	if devID == "" {
		return Gateway{}, GatewayDevice{}, ErrGatewayDeviceIDRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.gateways {
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			continue
		}
		for d := range s.gateways[i].Devices {
			if s.gateways[i].Devices[d].ID != devID {
				continue
			}

			removed := s.gateways[i].Devices[d]
			if removed.Source == "mistypass_procured" {
				s.releaseSerialInventoryLocked(removed.SerialNumber, s.gateways[i].TenantID, time.Now().UTC())
			}
			s.gateways[i].Devices = append(s.gateways[i].Devices[:d], s.gateways[i].Devices[d+1:]...)
			s.gateways[i].LastSeenAt = time.Now().UTC()
			if len(s.gateways[i].Devices) == 0 {
				s.gateways[i].Status = "offline"
			}
			if err := s.persistLocked(); err != nil {
				return Gateway{}, GatewayDevice{}, err
			}
			return s.gateways[i], removed, nil
		}
	}

	return Gateway{}, GatewayDevice{}, ErrGatewayDeviceNotFound
}

func (s *Service) UpdateDevice(tenantID, deviceID, status, protocol string) (Gateway, GatewayDevice, error) {
	devID := strings.TrimSpace(deviceID)
	if devID == "" {
		return Gateway{}, GatewayDevice{}, ErrGatewayDeviceIDRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.gateways {
		if tenantID != "" && s.gateways[i].TenantID != tenantID {
			continue
		}
		for d := range s.gateways[i].Devices {
			if s.gateways[i].Devices[d].ID != devID {
				continue
			}
			if nextStatus := strings.TrimSpace(status); nextStatus != "" {
				validatedStatus, err := normalizeDeviceStatus(nextStatus)
				if err != nil {
					return Gateway{}, GatewayDevice{}, err
				}
				s.gateways[i].Devices[d].Status = validatedStatus
			}
			if nextProtocol := strings.TrimSpace(protocol); nextProtocol != "" {
				validatedProtocol, err := normalizeDeviceProtocol(s.gateways[i].Devices[d].Kind, s.gateways[i].Devices[d].Source, nextProtocol)
				if err != nil {
					return Gateway{}, GatewayDevice{}, err
				}
				s.gateways[i].Devices[d].Protocol = validatedProtocol
			}
			s.gateways[i].Devices[d].LastSeenAt = time.Now().UTC()
			if err := s.persistLocked(); err != nil {
				return Gateway{}, GatewayDevice{}, err
			}
			return s.gateways[i], s.gateways[i].Devices[d], nil
		}
	}
	return Gateway{}, GatewayDevice{}, ErrGatewayDeviceNotFound
}

func (s *Service) ResetDeviceTamper(tenantID, deviceID string) (Gateway, GatewayDevice, error) {
	devID := strings.TrimSpace(deviceID)
	if devID == "" {
		return Gateway{}, GatewayDevice{}, ErrGatewayDeviceIDRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.gateways {
		if tenantID != "" && s.gateways[i].TenantID != tenantID {
			continue
		}
		for d := range s.gateways[i].Devices {
			if s.gateways[i].Devices[d].ID != devID {
				continue
			}
			s.gateways[i].Devices[d].RS485Health = nil
			s.gateways[i].Devices[d].LastSeenAt = time.Now().UTC()
			if err := s.persistLocked(); err != nil {
				return Gateway{}, GatewayDevice{}, err
			}
			return s.gateways[i], s.gateways[i].Devices[d], nil
		}
	}
	return Gateway{}, GatewayDevice{}, ErrGatewayDeviceNotFound
}

func (s *Service) ReportRS485Telemetry(
	tenantID,
	gatewayID,
	deviceID string,
	report RS485TelemetryReport,
) (GatewayDevice, bool, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return GatewayDevice{}, false, ErrGatewayIDRequired
	}
	devID := strings.TrimSpace(deviceID)
	if devID == "" {
		return GatewayDevice{}, false, ErrGatewayDeviceIDRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)

	nextReport, err := normalizeRS485TelemetryReport(report)
	if err != nil {
		return GatewayDevice{}, false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return GatewayDevice{}, false, ErrGatewayNotFound
		}

		for d := range s.gateways[i].Devices {
			if s.gateways[i].Devices[d].ID != devID {
				continue
			}
			if s.gateways[i].Devices[d].Protocol != "rs485" {
				return GatewayDevice{}, false, ErrGatewayDeviceRS485TelemetryProtocolMismatch
			}

			now := time.Now().UTC()
			if s.gateways[i].Devices[d].RS485Health == nil {
				s.gateways[i].Devices[d].RS485Health = &RS485Health{}
			}
			health := s.gateways[i].Devices[d].RS485Health
			health.RetryCount += nextReport.Retries
			health.TimeoutCount += nextReport.Timeouts
			health.CollisionCount += nextReport.Collisions
			if nextReport.ResetConsecutiveTimeouts {
				health.ConsecutiveTimeouts = 0
			} else if nextReport.Timeouts > 0 {
				health.ConsecutiveTimeouts += nextReport.Timeouts
			} else {
				health.ConsecutiveTimeouts = 0
			}
			if nextReport.LastError != "" {
				health.LastError = nextReport.LastError
			}
			health.LastReportAt = now

			s.gateways[i].Devices[d].LastSeenAt = now
			s.gateways[i].LastSeenAt = now
			if s.gateways[i].Status != "online" {
				s.gateways[i].Status = "online"
			}

			if err := s.persistLocked(); err != nil {
				return GatewayDevice{}, false, err
			}

			alerted := health.ConsecutiveTimeouts >= rs485AlertConsecutiveTimeoutThreshold ||
				health.CollisionCount >= rs485AlertCollisionThreshold
			return s.gateways[i].Devices[d], alerted, nil
		}
		return GatewayDevice{}, false, ErrGatewayDeviceNotFound
	}

	return GatewayDevice{}, false, ErrGatewayNotFound
}

func (s *Service) BuildDeviceTelemetrySummary(device GatewayDevice) DeviceTelemetrySummary {
	return summarizeDeviceTelemetry(device)
}

func (s *Service) ProbeLegacyDeviceSerials(tenantID, gatewayID string) ([]string, error) {
	result, err := s.ProbeLegacyDevicePlan(tenantID, gatewayID)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (s *Service) ProbeLegacyDevicePlan(tenantID, gatewayID string) (LegacyProbeResult, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return LegacyProbeResult{}, ErrGatewayIDRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return LegacyProbeResult{}, ErrGatewayNotFound
		}

		remain := s.gateways[i].DeviceCapacity - len(s.gateways[i].Devices)
		if remain <= 0 {
			return LegacyProbeResult{
				Items:      []string{},
				Governance: defaultLegacyProbeGovernance(),
			}, nil
		}
		if remain > 3 {
			remain = 3
		}

		serials := make([]string, 0, remain)
		for idx := 0; idx < remain; idx++ {
			serials = append(serials, strings.ToUpper(fmt.Sprintf("LEG-%s-%02d", tail(gwID, 6), idx+1)))
		}
		return LegacyProbeResult{
			Items:      serials,
			Governance: defaultLegacyProbeGovernance(),
		}, nil
	}

	return LegacyProbeResult{}, ErrGatewayNotFound
}

func (s *Service) PublishConfig(tenantID, gatewayID, version string) (CommandAck, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return CommandAck{}, ErrGatewayIDRequired
	}
	nextVersion := strings.TrimSpace(version)
	if nextVersion == "" {
		return CommandAck{}, ErrConfigVersionRequired
	}
	taskID, err := commandTaskID()
	if err != nil {
		return CommandAck{}, err
	}
	filterTenantID := strings.TrimSpace(tenantID)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return CommandAck{}, ErrGatewayNotFound
		}
		found = true
		s.gateways[i].LastSeenAt = now
		break
	}
	if !found {
		return CommandAck{}, ErrGatewayNotFound
	}

	cfg := s.ensureConfigStateLocked(gwID)
	cfg.DesiredVersion = nextVersion
	cfg.DesiredUpdatedAt = timePointer(now)
	if err := s.persistLocked(); err != nil {
		return CommandAck{}, err
	}

	return CommandAck{
		TaskID:    taskID,
		GatewayID: gwID,
		Command:   "update_config",
		Status:    "queued",
		CreatedAt: now,
	}, nil
}

func (s *Service) PullConfig(tenantID, gatewayID string) (GatewayConfigSnapshot, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return GatewayConfigSnapshot{}, ErrGatewayIDRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return GatewayConfigSnapshot{}, ErrGatewayNotFound
		}

		snapshot := GatewayConfigSnapshot{
			GatewayID:    s.gateways[i].ID,
			TenantID:     s.gateways[i].TenantID,
			BoundDoorIDs: append([]string(nil), s.gateways[i].BoundDoorIDs...),
			Devices:      cloneGatewayDevices(s.gateways[i].Devices),
		}
		if idx := s.findConfigStateIndexLocked(gwID); idx >= 0 {
			snapshot.DesiredVersion = s.configStates[idx].DesiredVersion
			snapshot.DesiredUpdatedAt = cloneTimePointer(s.configStates[idx].DesiredUpdatedAt)
			snapshot.AppliedVersion = s.configStates[idx].AppliedVersion
			snapshot.AppliedAt = cloneTimePointer(s.configStates[idx].AppliedAt)
		}
		return snapshot, nil
	}

	return GatewayConfigSnapshot{}, ErrGatewayNotFound
}

func (s *Service) MarkConfigApplied(tenantID, gatewayID, version string) (GatewayConfigSnapshot, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return GatewayConfigSnapshot{}, ErrGatewayIDRequired
	}
	nextVersion := strings.TrimSpace(version)
	if nextVersion == "" {
		return GatewayConfigSnapshot{}, ErrConfigVersionRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return GatewayConfigSnapshot{}, ErrGatewayNotFound
		}

		s.gateways[i].Status = "online"
		s.gateways[i].LastSeenAt = now

		cfg := s.ensureConfigStateLocked(gwID)
		cfg.AppliedVersion = nextVersion
		cfg.AppliedAt = timePointer(now)
		if cfg.DesiredVersion == "" {
			cfg.DesiredVersion = nextVersion
			cfg.DesiredUpdatedAt = timePointer(now)
		}

		if err := s.persistLocked(); err != nil {
			return GatewayConfigSnapshot{}, err
		}

		return GatewayConfigSnapshot{
			GatewayID:        s.gateways[i].ID,
			TenantID:         s.gateways[i].TenantID,
			DesiredVersion:   cfg.DesiredVersion,
			DesiredUpdatedAt: cloneTimePointer(cfg.DesiredUpdatedAt),
			AppliedVersion:   cfg.AppliedVersion,
			AppliedAt:        cloneTimePointer(cfg.AppliedAt),
			BoundDoorIDs:     append([]string(nil), s.gateways[i].BoundDoorIDs...),
			Devices:          cloneGatewayDevices(s.gateways[i].Devices),
		}, nil
	}

	return GatewayConfigSnapshot{}, ErrGatewayNotFound
}

func (s *Service) UpsertEventCheckpoint(
	tenantID,
	gatewayID,
	queue,
	checkpointID,
	lastRequestID string,
	ackedCount int,
	lastOccurredAt *time.Time,
) (GatewayEventCheckpoint, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return GatewayEventCheckpoint{}, ErrGatewayIDRequired
	}
	nextQueue := strings.TrimSpace(queue)
	if nextQueue == "" {
		return GatewayEventCheckpoint{}, ErrGatewayEventCheckpointQueueRequired
	}
	nextCheckpointID := strings.TrimSpace(checkpointID)
	if nextCheckpointID == "" {
		return GatewayEventCheckpoint{}, ErrGatewayEventCheckpointRequired
	}
	if ackedCount < 0 {
		return GatewayEventCheckpoint{}, ErrGatewayEventCheckpointAckedCountInvalid
	}

	filterTenantID := strings.TrimSpace(tenantID)
	now := time.Now().UTC()
	var normalizedLastOccurredAt *time.Time
	if lastOccurredAt != nil && !lastOccurredAt.IsZero() {
		ts := lastOccurredAt.UTC()
		normalizedLastOccurredAt = &ts
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	gatewayExists := false
	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return GatewayEventCheckpoint{}, ErrGatewayNotFound
		}
		gatewayExists = true
		s.gateways[i].LastSeenAt = now
		break
	}
	if !gatewayExists {
		return GatewayEventCheckpoint{}, ErrGatewayNotFound
	}

	idx := s.findEventCheckpointIndexLocked(gwID, nextQueue)
	if idx < 0 {
		s.eventCheckpoints = append(s.eventCheckpoints, GatewayEventCheckpoint{
			GatewayID:      gwID,
			Queue:          nextQueue,
			CheckpointID:   nextCheckpointID,
			LastRequestID:  strings.TrimSpace(lastRequestID),
			AckedCount:     ackedCount,
			LastOccurredAt: cloneTimePointer(normalizedLastOccurredAt),
			UpdatedAt:      now,
		})
		idx = len(s.eventCheckpoints) - 1
	} else {
		if ackedCount < s.eventCheckpoints[idx].AckedCount {
			return GatewayEventCheckpoint{}, ErrGatewayEventCheckpointAckedCountRegression
		}
		s.eventCheckpoints[idx].CheckpointID = nextCheckpointID
		s.eventCheckpoints[idx].LastRequestID = strings.TrimSpace(lastRequestID)
		s.eventCheckpoints[idx].AckedCount = ackedCount
		s.eventCheckpoints[idx].LastOccurredAt = cloneTimePointer(normalizedLastOccurredAt)
		s.eventCheckpoints[idx].UpdatedAt = now
	}

	if err := s.persistLocked(); err != nil {
		return GatewayEventCheckpoint{}, err
	}
	return s.eventCheckpoints[idx], nil
}

func (s *Service) ListEventCheckpoints(tenantID, gatewayID string) ([]GatewayEventCheckpoint, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return nil, ErrGatewayIDRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	gatewayExists := false
	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return nil, ErrGatewayNotFound
		}
		gatewayExists = true
		break
	}
	if !gatewayExists {
		return nil, ErrGatewayNotFound
	}

	items := make([]GatewayEventCheckpoint, 0, len(s.eventCheckpoints))
	for i := range s.eventCheckpoints {
		if s.eventCheckpoints[i].GatewayID != gwID {
			continue
		}
		record := s.eventCheckpoints[i]
		record.LastOccurredAt = cloneTimePointer(s.eventCheckpoints[i].LastOccurredAt)
		items = append(items, record)
	}
	return items, nil
}

func (s *Service) ListEventCheckpointsByTenant(tenantID, gatewayID, queue string) ([]GatewayEventCheckpoint, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	filterGatewayID := strings.TrimSpace(gatewayID)
	filterQueue := strings.TrimSpace(queue)

	s.mu.RLock()
	defer s.mu.RUnlock()

	allowedGatewayTenant := make(map[string]string, len(s.gateways))
	anyGatewayMatchID := false
	for i := range s.gateways {
		if filterGatewayID != "" && s.gateways[i].ID == filterGatewayID {
			anyGatewayMatchID = true
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			continue
		}
		if filterGatewayID != "" && s.gateways[i].ID != filterGatewayID {
			continue
		}
		allowedGatewayTenant[s.gateways[i].ID] = s.gateways[i].TenantID
	}
	if filterGatewayID != "" {
		if !anyGatewayMatchID || len(allowedGatewayTenant) == 0 {
			return nil, ErrGatewayNotFound
		}
	}

	items := make([]GatewayEventCheckpoint, 0, len(s.eventCheckpoints))
	for i := range s.eventCheckpoints {
		_, ok := allowedGatewayTenant[s.eventCheckpoints[i].GatewayID]
		if !ok {
			if filterGatewayID == "" && filterTenantID == "" {
				// include all existing gateway checkpoints when no explicit filter.
				found := false
				for g := range s.gateways {
					if s.gateways[g].ID == s.eventCheckpoints[i].GatewayID {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			} else {
				continue
			}
		}
		if filterQueue != "" && s.eventCheckpoints[i].Queue != filterQueue {
			continue
		}
		record := s.eventCheckpoints[i]
		record.LastOccurredAt = cloneTimePointer(s.eventCheckpoints[i].LastOccurredAt)
		if strings.TrimSpace(record.GatewayID) == "" {
			continue
		}
		if strings.TrimSpace(record.Queue) == "" {
			record.Queue = "default"
		}
		items = append(items, record)
	}
	return items, nil
}

func (s *Service) GetEventCheckpoint(tenantID, gatewayID, queue string) (GatewayEventCheckpoint, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return GatewayEventCheckpoint{}, ErrGatewayIDRequired
	}
	nextQueue := strings.TrimSpace(queue)
	if nextQueue == "" {
		return GatewayEventCheckpoint{}, ErrGatewayEventCheckpointQueueRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	gatewayExists := false
	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return GatewayEventCheckpoint{}, ErrGatewayNotFound
		}
		gatewayExists = true
		break
	}
	if !gatewayExists {
		return GatewayEventCheckpoint{}, ErrGatewayNotFound
	}

	idx := s.findEventCheckpointIndexLocked(gwID, nextQueue)
	if idx < 0 {
		return GatewayEventCheckpoint{}, ErrGatewayEventCheckpointNotFound
	}
	record := s.eventCheckpoints[idx]
	record.LastOccurredAt = cloneTimePointer(s.eventCheckpoints[idx].LastOccurredAt)
	return record, nil
}

func (s *Service) AddQueueIngestTotal(tenantID, gatewayID, queue string, delta int) (GatewayQueueIngestTotal, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return GatewayQueueIngestTotal{}, ErrGatewayIDRequired
	}
	nextQueue := strings.TrimSpace(queue)
	if nextQueue == "" {
		return GatewayQueueIngestTotal{}, ErrGatewayEventCheckpointQueueRequired
	}
	if delta <= 0 {
		return GatewayQueueIngestTotal{}, ErrGatewayQueueIngestDeltaInvalid
	}
	filterTenantID := strings.TrimSpace(tenantID)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	gatewayExists := false
	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return GatewayQueueIngestTotal{}, ErrGatewayNotFound
		}
		gatewayExists = true
		s.gateways[i].LastSeenAt = now
		break
	}
	if !gatewayExists {
		return GatewayQueueIngestTotal{}, ErrGatewayNotFound
	}

	idx := s.findQueueIngestTotalIndexLocked(gwID, nextQueue)
	if idx < 0 {
		s.queueIngestTotals = append(s.queueIngestTotals, GatewayQueueIngestTotal{
			GatewayID:     gwID,
			Queue:         nextQueue,
			IngestedTotal: delta,
			UpdatedAt:     now,
		})
		idx = len(s.queueIngestTotals) - 1
	} else {
		s.queueIngestTotals[idx].IngestedTotal += delta
		s.queueIngestTotals[idx].UpdatedAt = now
	}

	if err := s.persistLocked(); err != nil {
		return GatewayQueueIngestTotal{}, err
	}
	return s.queueIngestTotals[idx], nil
}

func (s *Service) GetQueueIngestTotal(tenantID, gatewayID, queue string) (GatewayQueueIngestTotal, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return GatewayQueueIngestTotal{}, ErrGatewayIDRequired
	}
	nextQueue := strings.TrimSpace(queue)
	if nextQueue == "" {
		return GatewayQueueIngestTotal{}, ErrGatewayEventCheckpointQueueRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	gatewayExists := false
	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return GatewayQueueIngestTotal{}, ErrGatewayNotFound
		}
		gatewayExists = true
		break
	}
	if !gatewayExists {
		return GatewayQueueIngestTotal{}, ErrGatewayNotFound
	}

	idx := s.findQueueIngestTotalIndexLocked(gwID, nextQueue)
	if idx < 0 {
		return GatewayQueueIngestTotal{}, ErrGatewayQueueIngestTotalNotFound
	}
	return s.queueIngestTotals[idx], nil
}

func (s *Service) Reboot(tenantID, gatewayID string) (CommandAck, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return CommandAck{}, ErrGatewayIDRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)

	taskID, err := commandTaskID()
	if err != nil {
		return CommandAck{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return CommandAck{}, ErrGatewayNotFound
		}

		s.gateways[i].Status = "online"
		s.gateways[i].LastSeenAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return CommandAck{}, err
		}

		return CommandAck{
			TaskID:    taskID,
			GatewayID: gwID,
			Command:   "reboot",
			Status:    "queued",
			CreatedAt: time.Now().UTC(),
		}, nil
	}

	return CommandAck{}, ErrGatewayNotFound
}

func (s *Service) CreateOTATask(
	tenantID,
	gatewayID,
	firmwareVersion,
	firmwareURL,
	firmwareSHA256,
	firmwareSignature,
	requestedBy string,
) (GatewayOTATask, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return GatewayOTATask{}, ErrGatewayIDRequired
	}
	nextVersion := strings.TrimSpace(firmwareVersion)
	if nextVersion == "" {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareVersionRequired
	}
	nextURL := strings.TrimSpace(firmwareURL)
	if nextURL == "" {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareURLRequired
	}
	nextSHA256 := strings.ToLower(strings.TrimSpace(firmwareSHA256))
	if nextSHA256 == "" {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareSHA256Required
	}
	if !isValidSHA256Hex(nextSHA256) {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareSHA256Invalid
	}
	nextSignature := strings.ToLower(strings.TrimSpace(firmwareSignature))
	if nextSignature == "" {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareSignatureRequired
	}
	if !isValidEd25519SignatureHex(nextSignature) {
		return GatewayOTATask{}, ErrGatewayOTAFirmwareSignatureInvalid
	}
	taskID, err := otaTaskID()
	if err != nil {
		return GatewayOTATask{}, err
	}

	filterTenantID := strings.TrimSpace(tenantID)
	now := time.Now().UTC()
	nextRequestedBy := strings.TrimSpace(requestedBy)

	s.mu.Lock()
	defer s.mu.Unlock()

	taskTenantID := ""
	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return GatewayOTATask{}, ErrGatewayNotFound
		}
		taskTenantID = s.gateways[i].TenantID
		s.gateways[i].LastSeenAt = now
		break
	}
	if taskTenantID == "" {
		return GatewayOTATask{}, ErrGatewayNotFound
	}

	task := GatewayOTATask{
		ID:                taskID,
		GatewayID:         gwID,
		TenantID:          taskTenantID,
		FirmwareVersion:   nextVersion,
		FirmwareURL:       nextURL,
		FirmwareSHA256:    nextSHA256,
		FirmwareSignature: nextSignature,
		Status:            gatewayOTATaskStatusQueued,
		RequestedBy:       nextRequestedBy,
		UpdatedBy:         nextRequestedBy,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	s.otaTasks = append([]GatewayOTATask{task}, s.otaTasks...)
	if err := s.persistLocked(); err != nil {
		return GatewayOTATask{}, err
	}
	return task, nil
}

func (s *Service) ListOTATasks(tenantID, gatewayID string) ([]GatewayOTATask, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return nil, ErrGatewayIDRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	gatewayExists := false
	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return nil, ErrGatewayNotFound
		}
		gatewayExists = true
		break
	}
	if !gatewayExists {
		return nil, ErrGatewayNotFound
	}

	items := make([]GatewayOTATask, 0, len(s.otaTasks))
	for i := range s.otaTasks {
		if s.otaTasks[i].GatewayID != gwID {
			continue
		}
		items = append(items, s.otaTasks[i])
	}
	return items, nil
}

func (s *Service) UpdateOTATaskStatus(
	tenantID,
	gatewayID,
	taskID,
	status,
	errorMessage,
	updatedBy string,
) (GatewayOTATask, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return GatewayOTATask{}, ErrGatewayIDRequired
	}
	nextTaskID := strings.TrimSpace(taskID)
	if nextTaskID == "" {
		return GatewayOTATask{}, ErrGatewayOTATaskIDRequired
	}
	nextStatus, err := normalizeGatewayOTATaskStatus(status)
	if err != nil {
		return GatewayOTATask{}, err
	}
	nextErrorMessage := strings.TrimSpace(errorMessage)
	if nextStatus != gatewayOTATaskStatusFailed {
		nextErrorMessage = ""
	}
	nextUpdatedBy := strings.TrimSpace(updatedBy)
	filterTenantID := strings.TrimSpace(tenantID)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	gatewayExists := false
	for i := range s.gateways {
		if s.gateways[i].ID != gwID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return GatewayOTATask{}, ErrGatewayNotFound
		}
		s.gateways[i].LastSeenAt = now
		gatewayExists = true
		break
	}
	if !gatewayExists {
		return GatewayOTATask{}, ErrGatewayNotFound
	}

	idx := s.findOTATaskIndexLocked(nextTaskID, gwID)
	if idx < 0 {
		return GatewayOTATask{}, ErrGatewayOTATaskNotFound
	}
	if filterTenantID != "" && s.otaTasks[idx].TenantID != filterTenantID {
		return GatewayOTATask{}, ErrGatewayOTATaskNotFound
	}

	s.otaTasks[idx].Status = nextStatus
	s.otaTasks[idx].ErrorMessage = nextErrorMessage
	if nextUpdatedBy != "" {
		s.otaTasks[idx].UpdatedBy = nextUpdatedBy
	}
	s.otaTasks[idx].UpdatedAt = now

	if err := s.persistLocked(); err != nil {
		return GatewayOTATask{}, err
	}
	return s.otaTasks[idx], nil
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
			Gateways:               cloneGateways(s.gateways),
			SerialInventory:        cloneSerialInventory(s.serialInventory),
			CertificateRevocations: cloneGatewayCertificateRevocations(s.certificateRevocations),
			ConfigStates:           cloneGatewayConfigStates(s.configStates),
			EventCheckpoints:       cloneGatewayEventCheckpoints(s.eventCheckpoints),
			QueueIngestTotals:      cloneGatewayQueueIngestTotals(s.queueIngestTotals),
			OTATasks:               cloneGatewayOTATasks(s.otaTasks),
			Firmwares:              cloneGatewayFirmwares(s.firmwares),
		})
	}
	shouldBackfillInventory := false
	if len(snapshot.SerialInventory) == 0 {
		snapshot.SerialInventory = serialInventoryFromGateways(snapshot.Gateways)
		shouldBackfillInventory = true
	}

	s.mu.Lock()
	s.gateways = cloneGateways(snapshot.Gateways)
	s.serialInventory = cloneSerialInventory(snapshot.SerialInventory)
	s.certificateRevocations = cloneGatewayCertificateRevocations(snapshot.CertificateRevocations)
	s.configStates = cloneGatewayConfigStates(snapshot.ConfigStates)
	s.eventCheckpoints = cloneGatewayEventCheckpoints(snapshot.EventCheckpoints)
	s.queueIngestTotals = cloneGatewayQueueIngestTotals(snapshot.QueueIngestTotals)
	s.otaTasks = cloneGatewayOTATasks(snapshot.OTATasks)
	s.firmwares = cloneGatewayFirmwares(snapshot.Firmwares)
	s.mu.Unlock()

	if shouldBackfillInventory {
		return s.stateStore.Save(stateKey, stateSnapshot{
			Gateways:               cloneGateways(snapshot.Gateways),
			SerialInventory:        cloneSerialInventory(snapshot.SerialInventory),
			CertificateRevocations: cloneGatewayCertificateRevocations(snapshot.CertificateRevocations),
			ConfigStates:           cloneGatewayConfigStates(snapshot.ConfigStates),
			EventCheckpoints:       cloneGatewayEventCheckpoints(snapshot.EventCheckpoints),
			QueueIngestTotals:      cloneGatewayQueueIngestTotals(snapshot.QueueIngestTotals),
			OTATasks:               cloneGatewayOTATasks(snapshot.OTATasks),
			Firmwares:              cloneGatewayFirmwares(snapshot.Firmwares),
		})
	}
	return nil
}

func (s *Service) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(stateKey, stateSnapshot{
		Gateways:               cloneGateways(s.gateways),
		SerialInventory:        cloneSerialInventory(s.serialInventory),
		CertificateRevocations: cloneGatewayCertificateRevocations(s.certificateRevocations),
		ConfigStates:           cloneGatewayConfigStates(s.configStates),
		EventCheckpoints:       cloneGatewayEventCheckpoints(s.eventCheckpoints),
		QueueIngestTotals:      cloneGatewayQueueIngestTotals(s.queueIngestTotals),
		OTATasks:               cloneGatewayOTATasks(s.otaTasks),
		Firmwares:              cloneGatewayFirmwares(s.firmwares),
	})
}

func cloneGateways(items []Gateway) []Gateway {
	output := make([]Gateway, 0, len(items))
	for i := range items {
		output = append(output, cloneGateway(items[i]))
	}
	return output
}

func cloneGateway(item Gateway) Gateway {
	record := item
	record.BoundDoorIDs = append([]string(nil), item.BoundDoorIDs...)
	record.Devices = cloneGatewayDevices(item.Devices)
	return record
}

func cloneGatewayDevices(items []GatewayDevice) []GatewayDevice {
	if len(items) == 0 {
		return nil
	}
	output := make([]GatewayDevice, 0, len(items))
	for i := range items {
		device := items[i]
		if items[i].RS485Config != nil {
			cfg := *items[i].RS485Config
			device.RS485Config = &cfg
		}
		if items[i].RS485Health != nil {
			health := *items[i].RS485Health
			device.RS485Health = &health
		}
		output = append(output, device)
	}
	return output
}

func cloneSerialInventory(items []SerialInventoryItem) []SerialInventoryItem {
	output := make([]SerialInventoryItem, 0, len(items))
	for i := range items {
		record := items[i]
		if items[i].ConsumedAt != nil {
			value := *items[i].ConsumedAt
			record.ConsumedAt = &value
		} else {
			record.ConsumedAt = nil
		}
		output = append(output, record)
	}
	return output
}

func cloneGatewayConfigStates(items []GatewayConfigState) []GatewayConfigState {
	output := make([]GatewayConfigState, 0, len(items))
	for i := range items {
		record := items[i]
		record.DesiredUpdatedAt = cloneTimePointer(items[i].DesiredUpdatedAt)
		record.AppliedAt = cloneTimePointer(items[i].AppliedAt)
		output = append(output, record)
	}
	return output
}

func cloneGatewayEventCheckpoints(items []GatewayEventCheckpoint) []GatewayEventCheckpoint {
	output := make([]GatewayEventCheckpoint, 0, len(items))
	for i := range items {
		record := items[i]
		record.LastOccurredAt = cloneTimePointer(items[i].LastOccurredAt)
		output = append(output, record)
	}
	return output
}

func cloneGatewayQueueIngestTotals(items []GatewayQueueIngestTotal) []GatewayQueueIngestTotal {
	if len(items) == 0 {
		return nil
	}
	output := make([]GatewayQueueIngestTotal, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneGatewayOTATasks(items []GatewayOTATask) []GatewayOTATask {
	if len(items) == 0 {
		return nil
	}
	output := make([]GatewayOTATask, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func serialInventoryFromGateways(gateways []Gateway) []SerialInventoryItem {
	now := time.Now().UTC()
	items := make([]SerialInventoryItem, 0, len(gateways)*3)
	seen := make(map[string]struct{}, len(gateways)*3)

	appendConsumed := func(tenantID, serialNumber, productType, gatewayID string, consumedAt time.Time) {
		serial := strings.ToUpper(strings.TrimSpace(serialNumber))
		if serial == "" {
			return
		}
		if _, exists := seen[serial]; exists {
			return
		}
		seen[serial] = struct{}{}
		ts := consumedAt
		if ts.IsZero() {
			ts = now
		}
		items = append(items, SerialInventoryItem{
			ID:                serialInventorySeedID(serial, gatewayID),
			TenantID:          strings.TrimSpace(tenantID),
			SerialNumber:      serial,
			ProductType:       productType,
			Status:            serialInventoryStatusConsumed,
			BatchCode:         "legacy-seed",
			Source:            "seed",
			ConsumedGatewayID: strings.TrimSpace(gatewayID),
			ConsumedAt:        timePointer(ts),
			CreatedAt:         ts,
			UpdatedAt:         ts,
		})
	}

	for i := range gateways {
		appendConsumed(
			gateways[i].TenantID,
			gateways[i].SerialNumber,
			serialInventoryProductGateway,
			gateways[i].ID,
			gateways[i].LastSeenAt,
		)
		for d := range gateways[i].Devices {
			if gateways[i].Devices[d].Source != "mistypass_procured" {
				continue
			}
			appendConsumed(
				gateways[i].TenantID,
				gateways[i].Devices[d].SerialNumber,
				inventoryProductTypeByDeviceKind(gateways[i].Devices[d].Kind),
				gateways[i].ID,
				gateways[i].Devices[d].LastSeenAt,
			)
		}
	}

	return items
}

func serialInventorySeedID(serialNumber, gatewayID string) string {
	base := strings.ToLower(strings.TrimSpace(serialNumber + "-" + gatewayID))
	if base == "" {
		return "siv_seed_empty"
	}
	encoded := hex.EncodeToString([]byte(base))
	if len(encoded) > 42 {
		encoded = encoded[:42]
	}
	return "siv_seed_" + encoded
}

func gatewayID() (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "gw_" + hex.EncodeToString(raw), nil
}

func commandTaskID() (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "task_" + hex.EncodeToString(raw), nil
}

func otaTaskID() (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "ota_" + hex.EncodeToString(raw), nil
}

func gatewayDeviceID() (string, error) {
	raw := make([]byte, 5)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "gdv_" + hex.EncodeToString(raw), nil
}

func serialInventoryID() (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "siv_" + hex.EncodeToString(raw), nil
}

func timePointer(value time.Time) *time.Time {
	v := value
	return &v
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	next := *value
	return &next
}

func normalizeDeviceCapacity(capacity int) (int, error) {
	switch capacity {
	case 0:
		return defaultDeviceCapacity, nil
	case 4, 8:
		return capacity, nil
	default:
		return 0, ErrInvalidDeviceCapacity
	}
}

func normalizeDeviceKind(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "reader":
		return "reader", nil
	case "door_controller":
		return "door_controller", nil
	case "relay":
		return "relay", nil
	case "sensor":
		return "sensor", nil
	case "legacy_reader":
		return "legacy_reader", nil
	case "legacy_controller":
		return "legacy_controller", nil
	default:
		return "", ErrGatewayDeviceKindInvalid
	}
}

func normalizeDeviceSource(source string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", "mistypass_procured":
		return "mistypass_procured", nil
	case "legacy_integration":
		return "legacy_integration", nil
	default:
		return "", ErrGatewayDeviceSourceInvalid
	}
}

func normalizeDeviceStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "online":
		return "online", nil
	case "offline":
		return "offline", nil
	default:
		return "", ErrGatewayDeviceStatusInvalid
	}
}

func normalizeSerialInventoryProductType(productType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(productType)) {
	case serialInventoryProductGateway:
		return serialInventoryProductGateway, nil
	case serialInventoryProductReader:
		return serialInventoryProductReader, nil
	case serialInventoryProductController:
		return serialInventoryProductController, nil
	case serialInventoryProductRelay:
		return serialInventoryProductRelay, nil
	case serialInventoryProductSensor:
		return serialInventoryProductSensor, nil
	default:
		return "", ErrSerialInventoryProductTypeInvalid
	}
}

func normalizeGatewayStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "":
		return "", ErrGatewayStatusRequired
	case "online":
		return "online", nil
	case "offline":
		return "offline", nil
	case "disabled":
		return "disabled", nil
	case "revoked":
		return "revoked", nil
	default:
		return "", ErrGatewayStatusInvalid
	}
}

func normalizeSerialInventoryStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case serialInventoryStatusAvailable:
		return serialInventoryStatusAvailable, nil
	case serialInventoryStatusConsumed:
		return serialInventoryStatusConsumed, nil
	case serialInventoryStatusFrozen:
		return serialInventoryStatusFrozen, nil
	case serialInventoryStatusScrapped:
		return serialInventoryStatusScrapped, nil
	default:
		return "", ErrSerialInventoryStatusInvalid
	}
}

func normalizeDeviceProtocol(kind, source, protocol string) (string, error) {
	nextProtocol := strings.ToLower(strings.TrimSpace(protocol))
	if nextProtocol == "" {
		if source == "legacy_integration" || kind == "legacy_reader" || kind == "legacy_controller" {
			return "wiegand_26", nil
		}
		return "osdp_v2", nil
	}

	switch nextProtocol {
	case "wiegand_26", "wiegand_34", "osdp_v2", "ble", "rs485":
		return nextProtocol, nil
	default:
		return "", ErrGatewayDeviceProtocolInvalid
	}
}

func normalizeRS485Config(protocol string, cfg *RS485Config) (*RS485Config, error) {
	if protocol != "rs485" {
		if cfg == nil {
			return nil, nil
		}
		return nil, ErrGatewayDeviceRS485ConfigProtocolMismatch
	}

	next := RS485Config{
		BaudRate:      9600,
		Parity:        "none",
		StopBits:      1,
		DeviceAddress: 1,
		TimeoutMS:     800,
	}
	if cfg != nil {
		if cfg.BaudRate != 0 {
			next.BaudRate = cfg.BaudRate
		}
		normalizedParity := strings.ToLower(strings.TrimSpace(cfg.Parity))
		if normalizedParity != "" {
			next.Parity = normalizedParity
		}
		if cfg.StopBits != 0 {
			next.StopBits = cfg.StopBits
		}
		if cfg.DeviceAddress != 0 {
			next.DeviceAddress = cfg.DeviceAddress
		}
		if cfg.TimeoutMS != 0 {
			next.TimeoutMS = cfg.TimeoutMS
		}
	}

	if !isSupportedRS485BaudRate(next.BaudRate) {
		return nil, fmt.Errorf("%w: baud_rate", ErrGatewayDeviceRS485ConfigInvalid)
	}
	switch next.Parity {
	case "none", "even", "odd":
	default:
		return nil, fmt.Errorf("%w: parity", ErrGatewayDeviceRS485ConfigInvalid)
	}
	if next.StopBits != 1 && next.StopBits != 2 {
		return nil, fmt.Errorf("%w: stop_bits", ErrGatewayDeviceRS485ConfigInvalid)
	}
	if next.DeviceAddress < 1 || next.DeviceAddress > 247 {
		return nil, fmt.Errorf("%w: device_address", ErrGatewayDeviceRS485ConfigInvalid)
	}
	if next.TimeoutMS < 100 || next.TimeoutMS > 5000 {
		return nil, fmt.Errorf("%w: timeout_ms", ErrGatewayDeviceRS485ConfigInvalid)
	}
	return &next, nil
}

func normalizeRS485TelemetryReport(report RS485TelemetryReport) (RS485TelemetryReport, error) {
	next := RS485TelemetryReport{
		Retries:                  report.Retries,
		Timeouts:                 report.Timeouts,
		Collisions:               report.Collisions,
		LastError:                strings.TrimSpace(report.LastError),
		ResetConsecutiveTimeouts: report.ResetConsecutiveTimeouts,
	}
	if next.Retries < 0 || next.Timeouts < 0 || next.Collisions < 0 {
		return RS485TelemetryReport{}, ErrGatewayDeviceRS485TelemetryInvalid
	}
	if next.Retries == 0 && next.Timeouts == 0 && next.Collisions == 0 && next.LastError == "" && !next.ResetConsecutiveTimeouts {
		return RS485TelemetryReport{}, ErrGatewayDeviceRS485TelemetryInvalid
	}
	return next, nil
}

func defaultProtocolLineQualityPolicy() ProtocolLineQualityPolicy {
	return ProtocolLineQualityPolicy{
		WarningConsecutiveTimeouts:  rs485AlertConsecutiveTimeoutThreshold,
		WarningCollisions:           rs485AlertCollisionThreshold,
		CriticalConsecutiveTimeouts: rs485CriticalConsecutiveTimeouts,
		CriticalCollisions:          rs485CriticalCollisions,
	}
}

func defaultLegacyProbeGovernance() LegacyProbeGovernance {
	return LegacyProbeGovernance{
		LegacyProtocol:      "wiegand_26",
		UpgradeProtocol:     "osdp_v2",
		OfflineFallback:     "local_cache_then_cloud_sync",
		DegradedLineAction:  "degrade_to_signed_offline_cache_and_schedule_line_check",
		RecommendedTopology: "gateway_inside_reader_outside_short_wiegand_then_upgrade_rs485_or_osdp",
		LinePolicy:          defaultProtocolLineQualityPolicy(),
	}
}

func summarizeDeviceTelemetry(device GatewayDevice) DeviceTelemetrySummary {
	policy := defaultProtocolLineQualityPolicy()
	summary := DeviceTelemetrySummary{
		Protocol:         strings.TrimSpace(device.Protocol),
		AlertLevel:       "none",
		LineQuality:      "healthy",
		GovernanceAction: "keep_observing",
		Degraded:         false,
		LinePolicy:       policy,
	}

	if summary.Protocol == "" {
		summary.Protocol = "unknown"
	}
	if summary.Protocol != "rs485" {
		summary.GovernanceAction = "follow_protocol_default"
		return summary
	}
	if device.RS485Health == nil {
		summary.GovernanceAction = "report_telemetry"
		return summary
	}

	if !device.RS485Health.LastReportAt.IsZero() {
		reportAt := device.RS485Health.LastReportAt.UTC()
		summary.LastReportAt = &reportAt
	}

	reasonCodes := make([]string, 0, 2)
	if device.RS485Health.ConsecutiveTimeouts >= policy.WarningConsecutiveTimeouts {
		reasonCodes = append(reasonCodes, "consecutive_timeouts")
	}
	if device.RS485Health.CollisionCount >= policy.WarningCollisions {
		reasonCodes = append(reasonCodes, "collision_spike")
	}

	critical := device.RS485Health.ConsecutiveTimeouts >= policy.CriticalConsecutiveTimeouts ||
		device.RS485Health.CollisionCount >= policy.CriticalCollisions
	warning := len(reasonCodes) > 0

	if critical {
		summary.AlertLevel = "critical"
		summary.LineQuality = "critical"
		summary.GovernanceAction = "switch_to_offline_cache_and_schedule_line_repair"
		summary.Degraded = true
	} else if warning {
		summary.AlertLevel = "warning"
		summary.LineQuality = "degraded"
		summary.GovernanceAction = "inspect_line_and_prepare_osdp_upgrade"
		summary.Degraded = true
	}
	if len(reasonCodes) > 0 {
		summary.ReasonCodes = reasonCodes
	}

	return summary
}

func isSupportedRS485BaudRate(baudRate int) bool {
	switch baudRate {
	case 1200, 2400, 4800, 9600, 19200, 38400, 57600, 115200:
		return true
	default:
		return false
	}
}

func normalizeGatewaySerial(serialNumber string) (string, error) {
	serial := strings.ToUpper(strings.TrimSpace(serialNumber))
	if serial == "" {
		return "", ErrSerialNumberRequired
	}
	if len(serial) < len("MP-GW-")+3 || len(serial) > 64 {
		return "", ErrSerialNumberInvalid
	}
	if !strings.HasPrefix(serial, "MP-GW-") {
		return "", ErrSerialNumberInvalid
	}
	for _, r := range serial {
		if r == '-' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		return "", ErrSerialNumberInvalid
	}
	return serial, nil
}

func normalizeInventorySerial(productType, serialNumber string) (string, error) {
	if productType == serialInventoryProductGateway {
		return normalizeGatewaySerial(serialNumber)
	}

	serial := strings.ToUpper(strings.TrimSpace(serialNumber))
	if serial == "" {
		return "", ErrSerialNumberRequired
	}
	if len(serial) < 3 || len(serial) > 64 {
		return "", ErrSerialNumberInvalid
	}
	for _, r := range serial {
		if r == '-' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		return "", ErrSerialNumberInvalid
	}
	return serial, nil
}

func inventoryProductTypeByDeviceKind(kind string) string {
	switch kind {
	case "reader", "legacy_reader":
		return serialInventoryProductReader
	case "door_controller", "legacy_controller":
		return serialInventoryProductController
	case "relay":
		return serialInventoryProductRelay
	case "sensor":
		return serialInventoryProductSensor
	default:
		return serialInventoryProductReader
	}
}

func (s *Service) findSerialInventoryIndexLocked(serialNumber string) int {
	for i := range s.serialInventory {
		if strings.EqualFold(s.serialInventory[i].SerialNumber, serialNumber) {
			return i
		}
	}
	return -1
}

func (s *Service) releaseSerialInventoryLocked(serialNumber, tenantID string, now time.Time) {
	idx := s.findSerialInventoryIndexLocked(serialNumber)
	if idx < 0 || s.serialInventory[idx].TenantID != tenantID {
		return
	}
	_ = applySerialInventoryStatusTransition(&s.serialInventory[idx], serialInventoryStatusAvailable, "", now)
}

func (s *Service) removeGatewayRuntimeStateLocked(gatewayID string) {
	nextGatewayID := strings.TrimSpace(gatewayID)
	if nextGatewayID == "" {
		return
	}

	configStates := make([]GatewayConfigState, 0, len(s.configStates))
	for i := range s.configStates {
		if s.configStates[i].GatewayID != nextGatewayID {
			configStates = append(configStates, s.configStates[i])
		}
	}
	s.configStates = configStates

	eventCheckpoints := make([]GatewayEventCheckpoint, 0, len(s.eventCheckpoints))
	for i := range s.eventCheckpoints {
		if s.eventCheckpoints[i].GatewayID != nextGatewayID {
			eventCheckpoints = append(eventCheckpoints, s.eventCheckpoints[i])
		}
	}
	s.eventCheckpoints = eventCheckpoints

	queueTotals := make([]GatewayQueueIngestTotal, 0, len(s.queueIngestTotals))
	for i := range s.queueIngestTotals {
		if s.queueIngestTotals[i].GatewayID != nextGatewayID {
			queueTotals = append(queueTotals, s.queueIngestTotals[i])
		}
	}
	s.queueIngestTotals = queueTotals

	otaTasks := make([]GatewayOTATask, 0, len(s.otaTasks))
	for i := range s.otaTasks {
		if s.otaTasks[i].GatewayID != nextGatewayID {
			otaTasks = append(otaTasks, s.otaTasks[i])
		}
	}
	s.otaTasks = otaTasks
}

func (s *Service) findConfigStateIndexLocked(gatewayID string) int {
	nextGatewayID := strings.TrimSpace(gatewayID)
	if nextGatewayID == "" {
		return -1
	}
	for i := range s.configStates {
		if s.configStates[i].GatewayID == nextGatewayID {
			return i
		}
	}
	return -1
}

func (s *Service) ensureConfigStateLocked(gatewayID string) *GatewayConfigState {
	nextGatewayID := strings.TrimSpace(gatewayID)
	if idx := s.findConfigStateIndexLocked(nextGatewayID); idx >= 0 {
		return &s.configStates[idx]
	}
	s.configStates = append(s.configStates, GatewayConfigState{
		GatewayID: nextGatewayID,
	})
	return &s.configStates[len(s.configStates)-1]
}

func (s *Service) findEventCheckpointIndexLocked(gatewayID, queue string) int {
	nextGatewayID := strings.TrimSpace(gatewayID)
	nextQueue := strings.TrimSpace(queue)
	if nextGatewayID == "" || nextQueue == "" {
		return -1
	}
	for i := range s.eventCheckpoints {
		if s.eventCheckpoints[i].GatewayID == nextGatewayID && s.eventCheckpoints[i].Queue == nextQueue {
			return i
		}
	}
	return -1
}

func (s *Service) findQueueIngestTotalIndexLocked(gatewayID, queue string) int {
	nextGatewayID := strings.TrimSpace(gatewayID)
	nextQueue := strings.TrimSpace(queue)
	if nextGatewayID == "" || nextQueue == "" {
		return -1
	}
	for i := range s.queueIngestTotals {
		if s.queueIngestTotals[i].GatewayID == nextGatewayID && s.queueIngestTotals[i].Queue == nextQueue {
			return i
		}
	}
	return -1
}

func (s *Service) findOTATaskIndexLocked(taskID, gatewayID string) int {
	nextTaskID := strings.TrimSpace(taskID)
	nextGatewayID := strings.TrimSpace(gatewayID)
	if nextTaskID == "" || nextGatewayID == "" {
		return -1
	}
	for i := range s.otaTasks {
		if s.otaTasks[i].ID == nextTaskID && s.otaTasks[i].GatewayID == nextGatewayID {
			return i
		}
	}
	return -1
}

func normalizeGatewayOTATaskStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case gatewayOTATaskStatusQueued:
		return gatewayOTATaskStatusQueued, nil
	case gatewayOTATaskStatusDispatching:
		return gatewayOTATaskStatusDispatching, nil
	case gatewayOTATaskStatusSucceeded:
		return gatewayOTATaskStatusSucceeded, nil
	case gatewayOTATaskStatusFailed:
		return gatewayOTATaskStatusFailed, nil
	case gatewayOTATaskStatusCanceled:
		return gatewayOTATaskStatusCanceled, nil
	default:
		return "", ErrGatewayOTATaskStatusInvalid
	}
}

func isValidSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// isValidEd25519SignatureHex validates a hex-encoded Ed25519 signature (64 bytes = 128 hex chars).
func isValidEd25519SignatureHex(value string) bool {
	if len(value) != 128 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func (s *Service) consumeGatewaySerialLocked(serialNumber, tenantID, gatewayID string) error {
	idx := s.findSerialInventoryIndexLocked(serialNumber)
	if idx < 0 {
		return ErrSerialNumberNotProvisioned
	}
	if s.serialInventory[idx].TenantID != tenantID {
		return ErrSerialNumberTenantMismatch
	}
	if s.serialInventory[idx].ProductType != serialInventoryProductGateway {
		return ErrSerialNumberProductTypeMismatch
	}
	now := time.Now().UTC()
	switch s.serialInventory[idx].Status {
	case serialInventoryStatusAvailable:
		s.serialInventory[idx].Status = serialInventoryStatusConsumed
		s.serialInventory[idx].ConsumedGatewayID = gatewayID
		s.serialInventory[idx].ConsumedAt = timePointer(now)
		s.serialInventory[idx].UpdatedAt = now
		return nil
	case serialInventoryStatusConsumed:
		return ErrSerialNumberAlreadyConsumed
	case serialInventoryStatusFrozen, serialInventoryStatusScrapped:
		return ErrSerialNumberNotAvailable
	default:
		return ErrSerialNumberNotAvailable
	}
}

func (s *Service) consumeDeviceSerialLocked(serialNumber, tenantID, gatewayID, kind, source string) error {
	if source != "mistypass_procured" {
		return nil
	}

	idx := s.findSerialInventoryIndexLocked(serialNumber)
	if idx < 0 {
		return ErrGatewayDeviceSerialNotProvisioned
	}
	if s.serialInventory[idx].TenantID != tenantID {
		return ErrSerialNumberTenantMismatch
	}

	expectedType := inventoryProductTypeByDeviceKind(kind)
	if s.serialInventory[idx].ProductType != expectedType {
		return ErrGatewayDeviceSerialProductTypeMismatch
	}

	now := time.Now().UTC()
	switch s.serialInventory[idx].Status {
	case serialInventoryStatusAvailable:
		s.serialInventory[idx].Status = serialInventoryStatusConsumed
		s.serialInventory[idx].ConsumedGatewayID = gatewayID
		s.serialInventory[idx].ConsumedAt = timePointer(now)
		s.serialInventory[idx].UpdatedAt = now
		return nil
	case serialInventoryStatusConsumed:
		if s.serialInventory[idx].ConsumedGatewayID != "" && s.serialInventory[idx].ConsumedGatewayID != gatewayID {
			return ErrGatewayDeviceSerialConflict
		}
		s.serialInventory[idx].UpdatedAt = now
		if s.serialInventory[idx].ConsumedGatewayID == "" {
			s.serialInventory[idx].ConsumedGatewayID = gatewayID
		}
		if s.serialInventory[idx].ConsumedAt == nil {
			s.serialInventory[idx].ConsumedAt = timePointer(now)
		}
		return nil
	case serialInventoryStatusFrozen, serialInventoryStatusScrapped:
		return ErrGatewayDeviceSerialNotAvailable
	default:
		return ErrGatewayDeviceSerialNotAvailable
	}
}

func applySerialInventoryStatusTransition(item *SerialInventoryItem, nextStatus, consumedGatewayID string, now time.Time) error {
	currentStatus := strings.TrimSpace(item.Status)
	if currentStatus == nextStatus {
		item.UpdatedAt = now
		return nil
	}

	switch nextStatus {
	case serialInventoryStatusAvailable:
		if currentStatus == serialInventoryStatusScrapped {
			return ErrSerialInventoryStatusTransitionInvalid
		}
		item.Status = serialInventoryStatusAvailable
		item.ConsumedGatewayID = ""
		item.ConsumedAt = nil
	case serialInventoryStatusConsumed:
		if currentStatus == serialInventoryStatusScrapped {
			return ErrSerialInventoryStatusTransitionInvalid
		}
		item.Status = serialInventoryStatusConsumed
		if strings.TrimSpace(consumedGatewayID) != "" {
			item.ConsumedGatewayID = strings.TrimSpace(consumedGatewayID)
		}
		if item.ConsumedAt == nil {
			item.ConsumedAt = timePointer(now)
		}
	case serialInventoryStatusFrozen:
		if currentStatus == serialInventoryStatusScrapped {
			return ErrSerialInventoryStatusTransitionInvalid
		}
		item.Status = serialInventoryStatusFrozen
	case serialInventoryStatusScrapped:
		item.Status = serialInventoryStatusScrapped
		item.ConsumedGatewayID = ""
		item.ConsumedAt = nil
	default:
		return ErrSerialInventoryStatusInvalid
	}

	item.UpdatedAt = now
	return nil
}

func tail(value string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(value) <= n {
		return value
	}
	return value[len(value)-n:]
}

func (s *Service) existsGateway(tenantID, gatewayID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	filterTenantID := strings.TrimSpace(tenantID)

	for i := range s.gateways {
		if s.gateways[i].ID == gatewayID {
			if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
				return false
			}
			return true
		}
	}

	return false
}
