package audit

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

type Log struct {
	TenantID string    `json:"tenant_id"`
	ID       string    `json:"id"`
	Actor    string    `json:"actor"`
	Role     string    `json:"role"`
	Action   string    `json:"action"`
	Target   string    `json:"target"`
	Source   string    `json:"source"`
	At       time.Time `json:"at"`
}

type StateStore interface {
	Load(key string, dst any) (bool, error)
	Save(key string, value any) error
}

const stateKey = "module_audit"

type stateSnapshot struct {
	Logs              []Log                    `json:"logs"`
	WebhookConfigs    map[string]WebhookConfig `json:"webhook_configs,omitempty"`
	WebhookDeliveries []WebhookDelivery        `json:"webhook_deliveries,omitempty"`
}

type Service struct {
	mu                sync.RWMutex
	logs              []Log
	webhookConfigs    map[string]WebhookConfig
	webhookDeliveries []WebhookDelivery
	stateStore        StateStore
}

func NewService() *Service {
	now := time.Now().UTC()
	return &Service{
		logs: []Log{
			{
				TenantID: "tenant_demo_jakarta",
				ID:       "aud_3001",
				Actor:    "superadmin@mistypass.local",
				Role:     "super_admin",
				Action:   "tenant_update",
				Target:   "tenant_demo_jakarta",
				Source:   "web_admin",
				At:       now.Add(-6 * time.Minute),
			},
			{
				TenantID: "tenant_demo_jakarta",
				ID:       "aud_3002",
				Actor:    "ops.jkt.01@mistypass.local",
				Role:     "operator",
				Action:   "gateway_reboot",
				Target:   "gw_demo_001",
				Source:   "web_admin",
				At:       now.Add(-41 * time.Minute),
			},
			{
				TenantID: "tenant_demo_jakarta",
				ID:       "aud_3003",
				Actor:    "tenant.admin@sudirman.co",
				Role:     "tenant_admin",
				Action:   "visitor_issue",
				Target:   "vst_2201",
				Source:   "web_admin",
				At:       now.Add(-84 * time.Minute),
			},
			{
				TenantID: "tenant_demo_factory",
				ID:       "aud_3004",
				Actor:    "gateway_agent",
				Role:     "system",
				Action:   "policy_publish",
				Target:   "plc_1003",
				Source:   "mqtt_bridge",
				At:       now.Add(-112 * time.Minute),
			},
		},
		webhookConfigs:    map[string]WebhookConfig{},
		webhookDeliveries: []WebhookDelivery{},
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

func (s *Service) List(tenantID string) []Log {
	return s.ListFiltered(tenantID, "", "", 0)
}

func (s *Service) ListFiltered(tenantID, action, source string, limit int) []Log {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	filterAction := strings.TrimSpace(action)
	filterSource := strings.TrimSpace(source)
	items := make([]Log, 0, len(s.logs))
	for i := range s.logs {
		if filterTenantID != "" && s.logs[i].TenantID != filterTenantID {
			continue
		}
		if filterAction != "" && s.logs[i].Action != filterAction {
			continue
		}
		if filterSource != "" && s.logs[i].Source != filterSource {
			continue
		}
		items = append(items, s.logs[i])
	}
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) Append(tenantID, actor, role, action, target, source string) (Log, error) {
	record := Log{
		TenantID: strings.TrimSpace(tenantID),
		Actor:    strings.TrimSpace(actor),
		Role:     strings.TrimSpace(role),
		Action:   strings.TrimSpace(action),
		Target:   strings.TrimSpace(target),
		Source:   strings.TrimSpace(source),
		At:       time.Now().UTC(),
	}
	if record.Actor == "" {
		record.Actor = "system"
	}
	if record.Role == "" {
		record.Role = "system"
	}
	if record.Source == "" {
		record.Source = "api"
	}

	id, err := auditLogID()
	if err != nil {
		return Log{}, err
	}
	record.ID = id

	s.mu.Lock()
	defer s.mu.Unlock()

	s.logs = append([]Log{record}, s.logs...)
	if err := s.persistLocked(); err != nil {
		return Log{}, err
	}
	return record, nil
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
			Logs:              cloneLogs(s.logs),
			WebhookConfigs:    cloneWebhookConfigs(s.webhookConfigs),
			WebhookDeliveries: cloneWebhookDeliveries(s.webhookDeliveries),
		})
	}

	s.mu.Lock()
	s.logs = cloneLogs(snapshot.Logs)
	s.webhookConfigs = cloneWebhookConfigs(snapshot.WebhookConfigs)
	s.webhookDeliveries = cloneWebhookDeliveries(snapshot.WebhookDeliveries)
	s.mu.Unlock()
	return nil
}

func (s *Service) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(stateKey, stateSnapshot{
		Logs:              cloneLogs(s.logs),
		WebhookConfigs:    cloneWebhookConfigs(s.webhookConfigs),
		WebhookDeliveries: cloneWebhookDeliveries(s.webhookDeliveries),
	})
}

func cloneLogs(items []Log) []Log {
	output := make([]Log, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneWebhookConfigs(items map[string]WebhookConfig) map[string]WebhookConfig {
	if len(items) == 0 {
		return map[string]WebhookConfig{}
	}
	output := make(map[string]WebhookConfig, len(items))
	for tenantID, config := range items {
		cloned := config
		cloned.Actions = append([]string(nil), config.Actions...)
		output[tenantID] = cloned
	}
	return output
}

func cloneWebhookDeliveries(items []WebhookDelivery) []WebhookDelivery {
	output := make([]WebhookDelivery, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func auditLogID() (string, error) {
	raw := make([]byte, 5)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "aud_" + hex.EncodeToString(raw), nil
}
