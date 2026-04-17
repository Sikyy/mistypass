package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/event"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

func TestGatewayBootstrapEventsCheckpointQueueIsolation(t *testing.T) {
	s := &server{
		gatewaySvc: gateway.NewService(),
		eventSvc:   event.NewService(),
		auditSvc:   audit.NewService(),
	}
	s.setGatewayDeviceToken("gw_demo_001", "dev-token-ckpt")

	if _, err := s.gatewaySvc.AddQueueIngestTotal("tenant_demo_jakarta", "gw_demo_001", "default", 10); err != nil {
		t.Fatalf("seed default queue ingest total error: %v", err)
	}
	if _, err := s.gatewaySvc.AddQueueIngestTotal("tenant_demo_jakarta", "gw_demo_001", "priority", 10); err != nil {
		t.Fatalf("seed priority queue ingest total error: %v", err)
	}

	code, body := callGatewayCheckpoint(t, s, "dev-token-ckpt", map[string]any{
		"gateway_id":      "gw_demo_001",
		"tenant_id":       "tenant_demo_jakarta",
		"queue":           "default",
		"checkpoint_id":   "default-seq-2",
		"last_request_id": "rq-default-2",
		"acked_count":     2,
		"last_occurred_at": time.Now().UTC().
			Format(time.RFC3339),
	})
	if code != http.StatusOK {
		t.Fatalf("expected default checkpoint 200, got %d body=%v", code, body)
	}
	if queue, _ := body["queue"].(string); queue != "default" {
		t.Fatalf("expected default checkpoint queue, got %q body=%v", queue, body)
	}

	code, body = callGatewayCheckpoint(t, s, "dev-token-ckpt", map[string]any{
		"gateway_id":      "gw_demo_001",
		"tenant_id":       "tenant_demo_jakarta",
		"queue":           "priority",
		"checkpoint_id":   "priority-seq-1",
		"last_request_id": "rq-priority-1",
		"acked_count":     1,
		"last_occurred_at": time.Now().UTC().
			Format(time.RFC3339),
	})
	if code != http.StatusOK {
		t.Fatalf("expected priority checkpoint 200, got %d body=%v", code, body)
	}

	code, body = callGatewayCheckpoint(t, s, "dev-token-ckpt", map[string]any{
		"gateway_id":      "gw_demo_001",
		"tenant_id":       "tenant_demo_jakarta",
		"queue":           "default",
		"checkpoint_id":   "default-seq-1-regress",
		"last_request_id": "rq-default-1-regress",
		"acked_count":     1,
		"last_occurred_at": time.Now().UTC().
			Format(time.RFC3339),
	})
	if code != http.StatusConflict {
		t.Fatalf("expected default regression 409, got %d body=%v", code, body)
	}
	if nextAction, _ := body["next_action"].(string); nextAction != "retry_with_non_regressing_acked_count" {
		t.Fatalf("expected non-regressing next_action, got %q body=%v", nextAction, body)
	}
	checkpoint, ok := body["checkpoint"].(map[string]any)
	if !ok {
		t.Fatalf("expected checkpoint payload in conflict body=%v", body)
	}
	if queue, _ := checkpoint["queue"].(string); queue != "default" {
		t.Fatalf("expected conflict checkpoint queue=default, got %q body=%v", queue, body)
	}
	if acked, ok := checkpoint["acked_count"].(float64); !ok || int(acked) != 2 {
		t.Fatalf("expected conflict checkpoint acked_count=2, got %v body=%v", checkpoint["acked_count"], body)
	}

	code, body = callGatewayCheckpoint(t, s, "dev-token-ckpt", map[string]any{
		"gateway_id":      "gw_demo_001",
		"tenant_id":       "tenant_demo_jakarta",
		"queue":           "priority",
		"checkpoint_id":   "priority-seq-2",
		"last_request_id": "rq-priority-2",
		"acked_count":     2,
		"last_occurred_at": time.Now().UTC().
			Format(time.RFC3339),
	})
	if code != http.StatusOK {
		t.Fatalf("expected priority queue keep progressing after default conflict, got %d body=%v", code, body)
	}
	if queue, _ := body["queue"].(string); queue != "priority" {
		t.Fatalf("expected response queue priority, got %q body=%v", queue, body)
	}
}

func TestGatewayBootstrapEventsCheckpointDefaultQueueFallbackUpperBound(t *testing.T) {
	s := &server{
		gatewaySvc: gateway.NewService(),
		eventSvc:   event.NewService(),
		auditSvc:   audit.NewService(),
	}
	s.setGatewayDeviceToken("gw_demo_001", "dev-token-fallback")

	_, deduplicated, err := s.eventSvc.IngestAccessEvent(event.IngestAccessEventInput{
		ID:        "evt_fb_default_001",
		TenantID:  "tenant_demo_jakarta",
		GatewayID: "gw_demo_001",
		Type:      "access_granted",
		At:        time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("seed fallback access event error: %v", err)
	}
	if deduplicated {
		t.Fatalf("expected fallback seed event to be newly ingested")
	}

	code, body := callGatewayCheckpoint(t, s, "dev-token-fallback", map[string]any{
		"gateway_id":      "gw_demo_001",
		"tenant_id":       "tenant_demo_jakarta",
		"queue":           "default",
		"checkpoint_id":   "default-fallback-seq-2",
		"last_request_id": "rq-default-fallback-2",
		"acked_count":     2,
		"last_occurred_at": time.Now().UTC().
			Format(time.RFC3339),
	})
	if code != http.StatusConflict {
		t.Fatalf("expected default fallback upper-bound conflict 409, got %d body=%v", code, body)
	}
	if source, _ := body["server_total_source"].(string); source != "event_rows_fallback" {
		t.Fatalf("expected fallback source event_rows_fallback, got %q body=%v", source, body)
	}
	if total, ok := body["server_event_total"].(float64); !ok || int(total) != 1 {
		t.Fatalf("expected fallback server_event_total=1, got %v body=%v", body["server_event_total"], body)
	}
	if nextAction, _ := body["next_action"].(string); nextAction != "retry_with_server_event_total" {
		t.Fatalf("expected fallback next_action retry_with_server_event_total, got %q body=%v", nextAction, body)
	}
}

func callGatewayCheckpoint(
	t *testing.T,
	s *server,
	deviceToken string,
	payload map[string]any,
) (int, map[string]any) {
	t.Helper()

	requestBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request error: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/gateway/events/checkpoint",
		bytes.NewReader(requestBytes),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Device-Token", deviceToken)

	recorder := httptest.NewRecorder()
	s.gatewayBootstrapEventsCheckpoint(recorder, request)

	if recorder.Body.Len() == 0 {
		return recorder.Code, map[string]any{}
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body error: %v body=%s", err, recorder.Body.String())
	}
	return recorder.Code, body
}
