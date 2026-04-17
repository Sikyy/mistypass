package alertdispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	defaultTenantID      = "tenant_demo_jakarta"
	defaultAlertType     = "dlq_error_code_threshold"
	defaultAlertError    = "unknown"
	defaultReceiverGroup = "security"
)

type Alert struct {
	Type      string
	ErrorCode string
	Count     int
	Threshold int
}

type Subscription struct {
	TenantID       string
	Enabled        bool
	Channels       []string
	ReceiverGroups []string
}

type PlanInput struct {
	Subscription Subscription
	Alert        Alert
	InCooldown   bool
}

type PlannedNotification struct {
	TenantID       string
	Type           string
	ErrorCode      string
	Count          int
	Threshold      int
	Channels       []string
	ReceiverGroups []string
	IdempotencyKey string
	Status         string
	Reason         string
}

func Plan(input PlanInput) PlannedNotification {
	tenantID := NormalizeTenantID(input.Subscription.TenantID)
	alertType := NormalizeAlertType(input.Alert.Type)
	errorCode := NormalizeErrorCode(input.Alert.ErrorCode)
	channels := normalizeChannels(input.Subscription.Channels)
	receiverGroups := NormalizeReceiverGroups(input.Subscription.ReceiverGroups)
	if len(receiverGroups) == 0 {
		receiverGroups = []string{defaultReceiverGroup}
	}

	planned := PlannedNotification{
		TenantID:       tenantID,
		Type:           alertType,
		ErrorCode:      errorCode,
		Count:          input.Alert.Count,
		Threshold:      input.Alert.Threshold,
		Channels:       channels,
		ReceiverGroups: receiverGroups,
		IdempotencyKey: BuildNotificationIdempotencyKey(
			tenantID,
			alertType,
			errorCode,
			input.Alert.Threshold,
		),
		Status: "ready",
	}

	if !input.Subscription.Enabled {
		planned.Status = "skipped"
		planned.Reason = "subscription_disabled"
		return planned
	}
	if len(channels) == 0 {
		planned.Status = "skipped"
		planned.Reason = "channel_disabled"
		return planned
	}
	if input.InCooldown {
		planned.Status = "skipped"
		planned.Reason = "cooldown"
		return planned
	}
	return planned
}

func NormalizeTenantID(value string) string {
	next := strings.TrimSpace(value)
	if next == "" {
		return defaultTenantID
	}
	return next
}

func NormalizeAlertType(value string) string {
	next := strings.TrimSpace(value)
	if next == "" {
		return defaultAlertType
	}
	return next
}

func NormalizeErrorCode(value string) string {
	next := strings.TrimSpace(value)
	if next == "" {
		return defaultAlertError
	}
	return next
}

func NormalizeReceiverGroups(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	output := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for i := range values {
		next := strings.TrimSpace(values[i])
		if next == "" {
			continue
		}
		key := strings.ToLower(next)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		output = append(output, next)
	}
	if len(output) == 0 {
		return nil
	}
	return output
}

func BuildNotificationIdempotencyKey(
	tenantID,
	alertType,
	errorCode string,
	threshold int,
) string {
	raw := fmt.Sprintf(
		"%s|%s|%s|%d",
		NormalizeTenantID(tenantID),
		NormalizeAlertType(alertType),
		NormalizeErrorCode(errorCode),
		threshold,
	)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:12])
}

func normalizeChannels(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	output := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for i := range values {
		next := strings.ToLower(strings.TrimSpace(values[i]))
		if next == "" {
			continue
		}
		if _, exists := seen[next]; exists {
			continue
		}
		seen[next] = struct{}{}
		output = append(output, next)
	}
	if len(output) == 0 {
		return nil
	}
	return output
}
