package alertdispatch

import "testing"

func TestPlanReady(t *testing.T) {
	planned := Plan(PlanInput{
		Subscription: Subscription{
			TenantID:       "tenant_a",
			Enabled:        true,
			Channels:       []string{" email ", "whatsapp", "Email"},
			ReceiverGroups: []string{"security", "Security", "ops"},
		},
		Alert: Alert{
			Type:      "dlq_error_code_threshold",
			ErrorCode: "template_inactive",
			Count:     3,
			Threshold: 1,
		},
		InCooldown: false,
	})

	if planned.Status != "ready" {
		t.Fatalf("expected ready, got %q", planned.Status)
	}
	if planned.Reason != "" {
		t.Fatalf("expected empty reason, got %q", planned.Reason)
	}
	if planned.TenantID != "tenant_a" {
		t.Fatalf("unexpected tenant id: %q", planned.TenantID)
	}
	if len(planned.Channels) != 2 || planned.Channels[0] != "email" || planned.Channels[1] != "whatsapp" {
		t.Fatalf("unexpected channels: %#v", planned.Channels)
	}
	if len(planned.ReceiverGroups) != 2 || planned.ReceiverGroups[0] != "security" || planned.ReceiverGroups[1] != "ops" {
		t.Fatalf("unexpected receiver groups: %#v", planned.ReceiverGroups)
	}
	if planned.IdempotencyKey == "" {
		t.Fatalf("expected idempotency key")
	}
}

func TestPlanSubscriptionDisabled(t *testing.T) {
	planned := Plan(PlanInput{
		Subscription: Subscription{
			Enabled:  false,
			Channels: []string{"email"},
		},
		Alert: Alert{
			ErrorCode: "template_inactive",
			Threshold: 1,
		},
	})

	if planned.Status != "skipped" {
		t.Fatalf("expected skipped, got %q", planned.Status)
	}
	if planned.Reason != "subscription_disabled" {
		t.Fatalf("unexpected reason: %q", planned.Reason)
	}
}

func TestPlanChannelDisabled(t *testing.T) {
	planned := Plan(PlanInput{
		Subscription: Subscription{
			Enabled: true,
		},
		Alert: Alert{
			ErrorCode: "template_inactive",
			Threshold: 1,
		},
	})

	if planned.Status != "skipped" {
		t.Fatalf("expected skipped, got %q", planned.Status)
	}
	if planned.Reason != "channel_disabled" {
		t.Fatalf("unexpected reason: %q", planned.Reason)
	}
}

func TestPlanCooldown(t *testing.T) {
	planned := Plan(PlanInput{
		Subscription: Subscription{
			Enabled:  true,
			Channels: []string{"email"},
		},
		Alert: Alert{
			ErrorCode: "template_inactive",
			Threshold: 1,
		},
		InCooldown: true,
	})

	if planned.Status != "skipped" {
		t.Fatalf("expected skipped, got %q", planned.Status)
	}
	if planned.Reason != "cooldown" {
		t.Fatalf("unexpected reason: %q", planned.Reason)
	}
}

func TestBuildNotificationIdempotencyKeyStable(t *testing.T) {
	a := BuildNotificationIdempotencyKey("tenant_a", "type_a", "code_a", 2)
	b := BuildNotificationIdempotencyKey("tenant_a", "type_a", "code_a", 2)
	c := BuildNotificationIdempotencyKey("tenant_a", "type_a", "code_b", 2)

	if a == "" || b == "" || c == "" {
		t.Fatalf("expected non-empty idempotency keys")
	}
	if a != b {
		t.Fatalf("expected stable idempotency key, got %q vs %q", a, b)
	}
	if a == c {
		t.Fatalf("expected different key for different error code")
	}
}
