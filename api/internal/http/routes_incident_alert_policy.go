package httpx

import (
	"strconv"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/access"
)

const stateKeyIncidentAlertPolicy = "module_incident_alert_policy"

// incidentAlertPolicyTriggers are the built-in incident policy trigger keys.
var incidentAlertPolicyTriggers = []string{"door_held_open", "hardware_outage", "role_assignment"}

func incidentAlertPolicyID(trigger, tenantID string) string {
	return "ap_incident_" + trigger + "_" + tenantID
}

func isIncidentAlertPolicyTrigger(trigger string) bool {
	for _, t := range incidentAlertPolicyTriggers {
		if t == trigger {
			return true
		}
	}
	return false
}

// incidentAlertPolicyDefaults returns the built-in incident policy catalog,
// all disabled by default (opt-in).
func incidentAlertPolicyDefaults(tenantID string) []referenceAlertPolicy {
	return []referenceAlertPolicy{
		{
			ID:              incidentAlertPolicyID("door_held_open", tenantID),
			TenantID:        tenantID,
			Name:            "Door Held Open",
			Description:     "Alerts when a door remains open beyond the configured threshold.",
			Category:        "incident",
			Trigger:         "door_held_open",
			Severity:        "high",
			Condition:       "event.type == 'lock.held_open' && event.duration_seconds > threshold",
			Status:          "inactive",
			Enabled:         false,
			Threshold:       300,
			WindowSeconds:   600,
			CooldownSeconds: 1800,
		},
		{
			ID:              incidentAlertPolicyID("hardware_outage", tenantID),
			TenantID:        tenantID,
			Name:            "Hardware Outage",
			Description:     "Alerts when controller or reader uptime falls below threshold.",
			Category:        "incident",
			Trigger:         "hardware_outage",
			Severity:        "critical",
			Condition:       "event.type == 'controller.offline' && event.downtime_minutes > threshold",
			Status:          "inactive",
			Enabled:         false,
			Threshold:       5,
			WindowSeconds:   300,
			CooldownSeconds: 3600,
		},
		{
			ID:              incidentAlertPolicyID("role_assignment", tenantID),
			TenantID:        tenantID,
			Name:            "Role Assignment",
			Description:     "Alerts when a role is assigned or an existing role assignment is changed.",
			Category:        "incident",
			Trigger:         "role_assignment",
			Severity:        "high",
			Condition:       "event.type == 'role_assignment.created' || event.type == 'role_assignment.updated'",
			Status:          "inactive",
			Enabled:         false,
			Threshold:       1,
			WindowSeconds:   0,
			CooldownSeconds: 0,
		},
	}
}

// mergedIncidentAlertPolicies returns the catalog with any stored per-tenant
// overrides applied.
func (s *server) mergedIncidentAlertPolicies(tenantID string) []referenceAlertPolicy {
	policies := incidentAlertPolicyDefaults(tenantID)
	s.incidentAlertPolicyMu.RLock()
	defer s.incidentAlertPolicyMu.RUnlock()
	for i := range policies {
		if override, ok := s.incidentAlertPolicyOverrides[policies[i].ID]; ok {
			policies[i] = override
		}
	}
	return policies
}

func (s *server) incidentAlertPolicyByID(tenantID, policyID string) (referenceAlertPolicy, bool) {
	for _, p := range s.mergedIncidentAlertPolicies(tenantID) {
		if p.ID == policyID {
			return p, true
		}
	}
	return referenceAlertPolicy{}, false
}

// updateReferenceIncidentAlertPolicy applies a payload to a built-in incident
// policy, persisting the override.
func (s *server) updateReferenceIncidentAlertPolicy(trigger, tenantID string, payload referenceAlertPolicyPayload) (referenceAlertPolicy, error) {
	var base referenceAlertPolicy
	found := false
	for _, p := range s.mergedIncidentAlertPolicies(tenantID) {
		if p.Trigger == trigger {
			base = p
			found = true
			break
		}
	}
	if !found {
		return referenceAlertPolicy{}, errInvalidReferenceAlertPolicyPayload
	}

	enabled, err := resolveReferenceAlertPolicyEnabled(base.Enabled, payload)
	if err != nil {
		return referenceAlertPolicy{}, err
	}
	base.Enabled = enabled
	base.Status = referenceAlertPolicyStatus(enabled)
	if strings.TrimSpace(payload.Severity) != "" {
		base.Severity = strings.TrimSpace(payload.Severity)
	}
	if payload.Threshold != nil {
		base.Threshold = *payload.Threshold
	}
	if payload.WindowSeconds != nil {
		base.WindowSeconds = *payload.WindowSeconds
	}
	if payload.CooldownSeconds != nil {
		base.CooldownSeconds = *payload.CooldownSeconds
	}
	if payload.Channels != nil {
		if payload.Channels.Email != nil {
			base.Channels.Email = *payload.Channels.Email
		}
		if payload.Channels.WhatsApp != nil {
			base.Channels.WhatsApp = *payload.Channels.WhatsApp
		}
		if payload.Channels.Webhook != nil {
			base.Channels.Webhook = *payload.Channels.Webhook
		}
		if payload.Channels.Slack != nil {
			base.Channels.Slack = *payload.Channels.Slack
		}
	}
	if payload.ReceiverGroups != nil {
		base.ReceiverGroups = payload.ReceiverGroups
	}
	base.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	s.incidentAlertPolicyMu.Lock()
	s.incidentAlertPolicyOverrides[base.ID] = base
	s.persistIncidentAlertPoliciesLocked()
	s.incidentAlertPolicyMu.Unlock()
	return base, nil
}

// maybeDispatchRoleAssignmentAlert fires a notification for a role assignment
// change when the role_assignment incident policy is enabled for the tenant.
func (s *server) maybeDispatchRoleAssignmentAlert(tenantID string, assignment access.RoleAssignment, eventType string) {
	policy, ok := s.incidentAlertPolicyByID(tenantID, incidentAlertPolicyID("role_assignment", tenantID))
	if !ok || !policy.Enabled {
		return
	}
	now := time.Now().UTC()
	dispatched := now
	s.appendAlertNotification(alertNotification{
		ID:           "apn_rolea_" + assignment.ID + "_" + strconv.FormatInt(now.UnixMilli(), 10),
		TenantID:     tenantID,
		PolicyID:     policy.ID,
		PolicyName:   policy.Name,
		Severity:     policy.Severity,
		Trigger:      "role_assignment",
		EventType:    eventType,
		EventID:      assignment.ID,
		Condition:    policy.Condition,
		Status:       "dispatched",
		Channels:     policy.Channels,
		Attempts:     1,
		DispatchedAt: &dispatched,
		CreatedAt:    now,
	})
}

// --- Incident alert policy persistence ---

type incidentAlertPolicyStateSnapshot struct {
	Overrides []referenceAlertPolicy `json:"overrides"`
}

// persistIncidentAlertPoliciesLocked saves overrides; caller holds incidentAlertPolicyMu.
func (s *server) persistIncidentAlertPoliciesLocked() {
	if s.stateStore == nil {
		return
	}
	overrides := make([]referenceAlertPolicy, 0, len(s.incidentAlertPolicyOverrides))
	for _, p := range s.incidentAlertPolicyOverrides {
		overrides = append(overrides, p)
	}
	_ = s.stateStore.Save(stateKeyIncidentAlertPolicy, incidentAlertPolicyStateSnapshot{Overrides: overrides})
}

func (s *server) restoreIncidentAlertPoliciesFromState() {
	if s.stateStore == nil {
		return
	}
	var snapshot incidentAlertPolicyStateSnapshot
	found, err := s.stateStore.Load(stateKeyIncidentAlertPolicy, &snapshot)
	if err != nil || !found {
		return
	}
	s.incidentAlertPolicyMu.Lock()
	defer s.incidentAlertPolicyMu.Unlock()
	for _, p := range snapshot.Overrides {
		s.incidentAlertPolicyOverrides[p.ID] = p
	}
}
