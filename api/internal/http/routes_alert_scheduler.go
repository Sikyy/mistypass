package httpx

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type alertNotification struct {
	ID            string                       `json:"id"`
	TenantID      string                       `json:"tenant_id"`
	PolicyID      string                       `json:"policy_id"`
	PolicyName    string                       `json:"policy_name"`
	Severity      string                       `json:"severity"`
	Trigger       string                       `json:"trigger"`
	EventType     string                       `json:"event_type,omitempty"`
	EventID       string                       `json:"event_id,omitempty"`
	Condition     string                       `json:"condition_expression,omitempty"`
	Status        string                       `json:"status"`
	Channels      referenceAlertPolicyChannels `json:"channels"`
	Attempts      int                          `json:"attempts"`
	LastError     string                       `json:"last_error,omitempty"`
	CooldownKey   string                       `json:"cooldown_key,omitempty"`
	DispatchedAt  *time.Time                   `json:"dispatched_at,omitempty"`
	CreatedAt     time.Time                    `json:"created_at"`
}

const maxAlertNotifications = 5000

func (s *server) startAlertPolicyEventScheduler() {
	if s.eventSvc == nil {
		return
	}
	changeCh, _ := s.eventSvc.SubscribeChanges()

	go func() {
		var lastAccessCount, lastDeviceCount int
		for range changeCh {
			s.evaluateAlertPoliciesOnEventChange(&lastAccessCount, &lastDeviceCount)
		}
	}()
}

func (s *server) evaluateAlertPoliciesOnEventChange(lastAccessCount, lastDeviceCount *int) {
	s.customAlertPolicyMu.RLock()
	policies := make([]referenceAlertPolicy, 0, len(s.customAlertPolicies))
	for _, p := range s.customAlertPolicies {
		if p.Enabled && strings.EqualFold(p.Status, "active") {
			policies = append(policies, p)
		}
	}
	s.customAlertPolicyMu.RUnlock()

	if len(policies) == 0 {
		return
	}

	// get current events to find new ones
	accessEvents := s.eventSvc.ListAccessEvents("")
	deviceEvents := s.eventSvc.ListDeviceEvents("")
	currentAccessCount := len(accessEvents)
	currentDeviceCount := len(deviceEvents)

	newAccessCount := currentAccessCount - *lastAccessCount
	newDeviceCount := currentDeviceCount - *lastDeviceCount
	if newAccessCount <= 0 && newDeviceCount <= 0 {
		*lastAccessCount = currentAccessCount
		*lastDeviceCount = currentDeviceCount
		return
	}

	// evaluate new events against policies
	var newEvents []map[string]any

	if newAccessCount > 0 && newAccessCount <= currentAccessCount {
		for i := 0; i < newAccessCount && i < len(accessEvents); i++ {
			ev := accessEvents[i]
			newEvents = append(newEvents, map[string]any{
				"id":          ev.ID,
				"tenant_id":   ev.TenantID,
				"building_id": ev.BuildingID,
				"area_id":     ev.AreaID,
				"type":        ev.Type,
				"actor":       ev.Actor,
				"door_id":     ev.DoorID,
				"gateway_id":  ev.GatewayID,
				"result":      ev.Result,
				"at":          ev.At.Format(time.RFC3339),
				"object_type": "Lock",
				"hour":        ev.At.Hour(),
			})
		}
	}

	if newDeviceCount > 0 && newDeviceCount <= currentDeviceCount {
		for i := 0; i < newDeviceCount && i < len(deviceEvents); i++ {
			ev := deviceEvents[i]
			newEvents = append(newEvents, map[string]any{
				"id":          ev.ID,
				"tenant_id":   ev.TenantID,
				"building_id": ev.BuildingID,
				"type":        ev.Type,
				"gateway_id":  ev.GatewayID,
				"detail":      ev.Detail,
				"result":      ev.Result,
				"at":          ev.At.Format(time.RFC3339),
				"object_type": "Device",
				"hour":        ev.At.Hour(),
			})
		}
	}

	*lastAccessCount = currentAccessCount
	*lastDeviceCount = currentDeviceCount

	if len(newEvents) == 0 {
		return
	}

	now := time.Now().UTC()
	for _, policy := range policies {
		for _, event := range newEvents {
			eventTenantID, _ := event["tenant_id"].(string)
			if policy.TenantID != "" && eventTenantID != "" && policy.TenantID != eventTenantID {
				continue
			}

			matched, err := evaluateReferenceAlertPolicyCondition(policy.Condition, event)
			if err != nil || !matched {
				continue
			}

			cooldownKey := fmt.Sprintf("%s:%s:%s", policy.TenantID, policy.ID, policy.Trigger)
			if s.isAlertCooldownActive(cooldownKey, policy.CooldownSeconds, now) {
				continue
			}

			eventID, _ := event["id"].(string)
			eventType, _ := event["type"].(string)
			notifID := fmt.Sprintf("apn_%s_%d", policy.ID[len(policy.ID)-min(8, len(policy.ID)):], now.UnixMilli())

			notification := alertNotification{
				ID:           notifID,
				TenantID:     policy.TenantID,
				PolicyID:     policy.ID,
				PolicyName:   policy.Name,
				Severity:     policy.Severity,
				Trigger:      policy.Trigger,
				EventType:    eventType,
				EventID:      eventID,
				Condition:    policy.Condition,
				Status:       "dispatched",
				Channels:     policy.Channels,
				Attempts:     1,
				CooldownKey:  cooldownKey,
				CreatedAt:    now,
			}
			dispatched := now
			notification.DispatchedAt = &dispatched

			s.appendAlertNotification(notification)
			s.recordAlertCooldown(cooldownKey, now)
		}
	}
}

func (s *server) isAlertCooldownActive(key string, cooldownSeconds int64, now time.Time) bool {
	if cooldownSeconds <= 0 {
		return false
	}
	s.alertCooldownMu.RLock()
	defer s.alertCooldownMu.RUnlock()
	last, exists := s.alertCooldowns[key]
	if !exists {
		return false
	}
	return now.Sub(last).Seconds() < float64(cooldownSeconds)
}

func (s *server) recordAlertCooldown(key string, at time.Time) {
	s.alertCooldownMu.Lock()
	defer s.alertCooldownMu.Unlock()
	if s.alertCooldowns == nil {
		s.alertCooldowns = map[string]time.Time{}
	}
	s.alertCooldowns[key] = at
}

func (s *server) appendAlertNotification(n alertNotification) {
	s.alertNotificationMu.Lock()
	defer s.alertNotificationMu.Unlock()
	s.alertNotifications = append([]alertNotification{n}, s.alertNotifications...)
	if len(s.alertNotifications) > maxAlertNotifications {
		s.alertNotifications = s.alertNotifications[:maxAlertNotifications]
	}
}

func (s *server) listAlertNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be an integer > 0")
			return
		}
		if parsed > 500 {
			parsed = 500
		}
		limit = parsed
	}

	policyID := strings.TrimSpace(r.URL.Query().Get("policy_id"))
	severity := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("severity")))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))

	s.alertNotificationMu.RLock()
	items := make([]alertNotification, 0, limit)
	for i := range s.alertNotifications {
		if len(items) >= limit {
			break
		}
		n := s.alertNotifications[i]
		if tenantID != "" && n.TenantID != tenantID {
			continue
		}
		if policyID != "" && n.PolicyID != policyID {
			continue
		}
		if severity != "" && !strings.EqualFold(n.Severity, severity) {
			continue
		}
		if status != "" && !strings.EqualFold(n.Status, status) {
			continue
		}
		items = append(items, n)
	}
	s.alertNotificationMu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

