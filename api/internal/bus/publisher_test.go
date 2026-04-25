package bus

import (
	"context"
	"errors"
	"testing"
)

func TestNewPublisherDisabledReturnsNoop(t *testing.T) {
	publisher, err := NewPublisher(false, "", "")
	if err != nil {
		t.Fatalf("expected disabled publisher init success: %v", err)
	}
	if publisher.Enabled() {
		t.Fatalf("expected disabled publisher")
	}
	if err := publisher.PublishJSON(context.Background(), "audit.log.appended", map[string]any{"ok": true}, nil); err != nil {
		t.Fatalf("expected noop publish success: %v", err)
	}
}

func TestQualifiedSubject(t *testing.T) {
	if got := qualifiedSubject("mistypass", "audit.log.appended"); got != "mistypass.audit.log.appended" {
		t.Fatalf("unexpected qualified subject: %s", got)
	}
	if got := qualifiedSubject("  ", " audit.webhook.dispatched "); got != "mistypass.audit.webhook.dispatched" {
		t.Fatalf("unexpected default prefix subject: %s", got)
	}
}

func TestPublishJSONRejectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	publisher := &natsPublisher{}
	err := publisher.PublishJSON(ctx, "audit.log.appended", map[string]any{"ok": true}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestPublishJSONRequiresInitializedConnection(t *testing.T) {
	publisher := &natsPublisher{}
	err := publisher.PublishJSON(context.Background(), "audit.log.appended", map[string]any{"ok": true}, nil)
	if err == nil || err.Error() != "nats publisher connection is not initialized" {
		t.Fatalf("expected uninitialized connection error, got %v", err)
	}
}
