package alarm

import (
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrAlarmNotFound = errors.New("alarm not found")
var ErrInvalidAlarmStatus = errors.New("invalid alarm status")

type Alarm struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	BuildingID string    `json:"building_id"`
	AreaID     string    `json:"area_id"`
	DoorID     string    `json:"door_id"`
	Type       string    `json:"type"`
	Severity   string    `json:"severity"`
	Location   string    `json:"location"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type StateStore interface {
	Load(key string, dst any) (bool, error)
	Save(key string, value any) error
}

const stateKey = "module_alarm"

type stateSnapshot struct {
	Alarms []Alarm `json:"alarms"`
}

type Service struct {
	mu          sync.RWMutex
	alarms      []Alarm
	stateStore  StateStore
	subscribers map[uint64]chan struct{}
	nextSubID   uint64
}

func NewService() *Service {
	now := time.Now().UTC()
	return &Service{
		alarms: []Alarm{
			{
				ID:         "alm_9001",
				TenantID:   "tenant_demo_factory",
				BuildingID: "building_demo_003",
				AreaID:     "area_demo_003",
				DoorID:     "door_fct_029",
				Type:       "Forced Door Open",
				Severity:   "critical",
				Location:   "Serang Plant A / F1 / Packing Zone",
				Status:     "open",
				CreatedAt:  now.Add(-66 * time.Minute),
			},
			{
				ID:         "alm_9002",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_002",
				AreaID:     "area_demo_002",
				DoorID:     "door_jkt_014",
				Type:       "Gateway Heartbeat Timeout",
				Severity:   "high",
				Location:   "Kuningan Tower / L3 / Lobby",
				Status:     "investigating",
				CreatedAt:  now.Add(-83 * time.Minute),
			},
			{
				ID:         "alm_9003",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_001",
				AreaID:     "area_demo_001",
				DoorID:     "door_jkt_001",
				Type:       "Abnormal Command Retry",
				Severity:   "medium",
				Location:   "Sudirman Hub / L8 / Finance",
				Status:     "acknowledged",
				CreatedAt:  now.Add(-110 * time.Minute),
			},
		},
		subscribers: make(map[uint64]chan struct{}),
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

func (s *Service) List(tenantID string) []Alarm {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]Alarm, 0, len(s.alarms))
	for i := range s.alarms {
		if filterTenantID != "" && s.alarms[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.alarms[i])
	}
	return items
}

func (s *Service) SubscribeChanges() (<-chan struct{}, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subID := s.nextSubID
	s.nextSubID++
	ch := make(chan struct{}, 1)
	s.subscribers[subID] = ch

	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		target, exists := s.subscribers[subID]
		if !exists {
			return
		}
		delete(s.subscribers, subID)
		close(target)
	}
}

func (s *Service) UpdateStatus(tenantID, alarmID, status string) (Alarm, error) {
	nextStatus, err := normalizeStatus(status)
	if err != nil {
		return Alarm{}, err
	}

	targetID := strings.TrimSpace(alarmID)
	if targetID == "" {
		return Alarm{}, ErrAlarmNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.alarms {
		if s.alarms[i].ID != targetID {
			continue
		}
		if filterTenantID != "" && s.alarms[i].TenantID != filterTenantID {
			return Alarm{}, ErrAlarmNotFound
		}
		s.alarms[i].Status = nextStatus
		if err := s.persistLocked(); err != nil {
			return Alarm{}, err
		}
		s.notifySubscribersLocked()
		return s.alarms[i], nil
	}

	return Alarm{}, ErrAlarmNotFound
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
			Alarms: cloneAlarms(s.alarms),
		})
	}

	s.mu.Lock()
	s.alarms = cloneAlarms(snapshot.Alarms)
	s.mu.Unlock()
	return nil
}

func (s *Service) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(stateKey, stateSnapshot{
		Alarms: cloneAlarms(s.alarms),
	})
}

func (s *Service) notifySubscribersLocked() {
	for _, subscriber := range s.subscribers {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
}

func cloneAlarms(items []Alarm) []Alarm {
	output := make([]Alarm, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func normalizeStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "open":
		return "open", nil
	case "acknowledged":
		return "acknowledged", nil
	case "investigating":
		return "investigating", nil
	case "mitigated":
		return "mitigated", nil
	case "escalated":
		return "escalated", nil
	case "resolved":
		return "resolved", nil
	case "false_positive":
		return "false_positive", nil
	default:
		return "", ErrInvalidAlarmStatus
	}
}
