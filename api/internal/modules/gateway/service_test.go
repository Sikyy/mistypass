package gateway

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type gatewayMemoryStateStore struct {
	items map[string][]byte
}

func (s *gatewayMemoryStateStore) Load(key string, dst any) (bool, error) {
	payload, ok := s.items[key]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(payload, dst); err != nil {
		return false, err
	}
	return true, nil
}

func (s *gatewayMemoryStateStore) Save(key string, value any) error {
	if s.items == nil {
		s.items = map[string][]byte{}
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.items[key] = payload
	return nil
}

func TestNormalizeDeviceProtocolDefault(t *testing.T) {
	got, err := normalizeDeviceProtocol("legacy_reader", "legacy_integration", "")
	if err != nil {
		t.Fatalf("legacy default protocol error: %v", err)
	}
	if got != "wiegand_26" {
		t.Fatalf("legacy default protocol mismatch: got %s", got)
	}

	got, err = normalizeDeviceProtocol("reader", "mistypass_procured", "")
	if err != nil {
		t.Fatalf("modern default protocol error: %v", err)
	}
	if got != "osdp_v2" {
		t.Fatalf("modern default protocol mismatch: got %s", got)
	}
}

func TestNormalizeDeviceProtocolExplicit(t *testing.T) {
	cases := map[string]string{
		"wiegand_26": "wiegand_26",
		"wiegand_34": "wiegand_34",
		"osdp_v2":    "osdp_v2",
		"ble":        "ble",
		"rs485":      "rs485",
		"RS485":      "rs485",
	}

	for input, expected := range cases {
		got, err := normalizeDeviceProtocol("reader", "mistypass_procured", input)
		if err != nil {
			t.Fatalf("protocol %s should be valid, err=%v", input, err)
		}
		if got != expected {
			t.Fatalf("protocol %s normalized mismatch: got=%s expected=%s", input, got, expected)
		}
	}
}

func TestNormalizeDeviceProtocolInvalid(t *testing.T) {
	_, err := normalizeDeviceProtocol("reader", "mistypass_procured", "uart")
	if err == nil {
		t.Fatalf("expected invalid protocol error")
	}
	if err != ErrGatewayDeviceProtocolInvalid {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeRS485ConfigDefaults(t *testing.T) {
	got, err := normalizeRS485Config("rs485", nil)
	if err != nil {
		t.Fatalf("normalize rs485 default error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected default rs485 config")
	}
	if got.BaudRate != 9600 || got.Parity != "none" || got.StopBits != 1 || got.DeviceAddress != 1 || got.TimeoutMS != 800 {
		t.Fatalf("unexpected default rs485 config: %+v", got)
	}
}

func TestNormalizeRS485ConfigValidation(t *testing.T) {
	_, err := normalizeRS485Config("rs485", &RS485Config{BaudRate: 12345})
	if err == nil {
		t.Fatalf("expected invalid rs485 baud rate")
	}
	if !errors.Is(err, ErrGatewayDeviceRS485ConfigInvalid) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeRS485ConfigProtocolMismatch(t *testing.T) {
	_, err := normalizeRS485Config("ble", &RS485Config{BaudRate: 9600})
	if err == nil {
		t.Fatalf("expected protocol mismatch error")
	}
	if err != ErrGatewayDeviceRS485ConfigProtocolMismatch {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportRS485Telemetry(t *testing.T) {
	svc := NewService()
	_, err := svc.ImportSerialInventory("tenant_demo_jakarta", []SerialInventoryImportItem{
		{
			SerialNumber: "RD-QA-RS485-TELE-001",
			ProductType:  "reader",
		},
	})
	if err != nil {
		t.Fatalf("import serial inventory error: %v", err)
	}

	updatedGateway, err := svc.RegisterDevice(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"RD-QA-RS485-TELE-001",
		"reader",
		"mistypass_procured",
		"online",
		"rs485",
		&RS485Config{BaudRate: 19200, Parity: "even", StopBits: 1, DeviceAddress: 10, TimeoutMS: 1200},
	)
	if err != nil {
		t.Fatalf("register rs485 device error: %v", err)
	}
	deviceID := updatedGateway.Devices[0].ID

	device, alerted, err := svc.ReportRS485Telemetry(
		"tenant_demo_jakarta",
		"gw_demo_001",
		deviceID,
		RS485TelemetryReport{
			Retries:    2,
			Timeouts:   2,
			Collisions: 1,
			LastError:  "crc_error",
		},
	)
	if err != nil {
		t.Fatalf("report rs485 telemetry #1 error: %v", err)
	}
	if alerted {
		t.Fatalf("expected first telemetry report to be non-alert")
	}
	if device.RS485Health == nil {
		t.Fatalf("expected rs485 health state to exist")
	}
	if device.RS485Health.RetryCount != 2 ||
		device.RS485Health.TimeoutCount != 2 ||
		device.RS485Health.CollisionCount != 1 ||
		device.RS485Health.ConsecutiveTimeouts != 2 {
		t.Fatalf("unexpected rs485 health #1: %+v", device.RS485Health)
	}

	device, alerted, err = svc.ReportRS485Telemetry(
		"tenant_demo_jakarta",
		"gw_demo_001",
		deviceID,
		RS485TelemetryReport{
			Timeouts:  1,
			LastError: "line_timeout",
		},
	)
	if err != nil {
		t.Fatalf("report rs485 telemetry #2 error: %v", err)
	}
	if !alerted {
		t.Fatalf("expected second telemetry report to trigger alert")
	}
	if device.RS485Health.ConsecutiveTimeouts != 3 {
		t.Fatalf("unexpected consecutive timeout count: %d", device.RS485Health.ConsecutiveTimeouts)
	}
}

func TestBuildDeviceTelemetrySummary(t *testing.T) {
	summary := summarizeDeviceTelemetry(GatewayDevice{
		Protocol: "rs485",
		RS485Health: &RS485Health{
			TimeoutCount:        7,
			CollisionCount:      2,
			ConsecutiveTimeouts: 4,
			LastError:           "line_timeout",
			LastReportAt:        time.Now().UTC(),
		},
	})
	if summary.AlertLevel != "warning" {
		t.Fatalf("expected warning alert level, got %s", summary.AlertLevel)
	}
	if summary.LineQuality != "degraded" {
		t.Fatalf("expected degraded line quality, got %s", summary.LineQuality)
	}
	if !summary.Degraded {
		t.Fatalf("expected degraded flag")
	}
	if len(summary.ReasonCodes) == 0 || summary.ReasonCodes[0] != "consecutive_timeouts" {
		t.Fatalf("expected timeout reason code, got %+v", summary.ReasonCodes)
	}

	critical := summarizeDeviceTelemetry(GatewayDevice{
		Protocol: "rs485",
		RS485Health: &RS485Health{
			CollisionCount:      12,
			ConsecutiveTimeouts: 1,
			LastReportAt:        time.Now().UTC(),
		},
	})
	if critical.AlertLevel != "critical" {
		t.Fatalf("expected critical alert level, got %s", critical.AlertLevel)
	}
	if critical.LineQuality != "critical" {
		t.Fatalf("expected critical line quality, got %s", critical.LineQuality)
	}
	if critical.GovernanceAction == "" {
		t.Fatalf("expected governance action")
	}

	nonRS485 := summarizeDeviceTelemetry(GatewayDevice{Protocol: "osdp_v2"})
	if nonRS485.AlertLevel != "none" {
		t.Fatalf("expected non-rs485 alert level none, got %s", nonRS485.AlertLevel)
	}
	if nonRS485.GovernanceAction != "follow_protocol_default" {
		t.Fatalf("unexpected governance action for non-rs485: %s", nonRS485.GovernanceAction)
	}
}

func TestProbeLegacyDevicePlan(t *testing.T) {
	svc := NewService()
	result, err := svc.ProbeLegacyDevicePlan("tenant_demo_jakarta", "gw_demo_001")
	if err != nil {
		t.Fatalf("probe legacy plan error: %v", err)
	}
	if len(result.Items) == 0 {
		t.Fatalf("expected at least one suggested legacy serial")
	}
	if result.Governance.LegacyProtocol != "wiegand_26" {
		t.Fatalf("unexpected legacy protocol: %s", result.Governance.LegacyProtocol)
	}
	if result.Governance.UpgradeProtocol != "osdp_v2" {
		t.Fatalf("unexpected upgrade protocol: %s", result.Governance.UpgradeProtocol)
	}
	if result.Governance.LinePolicy.WarningConsecutiveTimeouts <= 0 {
		t.Fatalf("unexpected line policy: %+v", result.Governance.LinePolicy)
	}
}

func TestReportRS485TelemetryProtocolMismatch(t *testing.T) {
	svc := NewService()
	device, alerted, err := svc.ReportRS485Telemetry(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"gdv_demo_001",
		RS485TelemetryReport{
			Retries: 1,
		},
	)
	if err == nil {
		t.Fatalf("expected protocol mismatch error, got device=%+v alerted=%t", device, alerted)
	}
	if err != ErrGatewayDeviceRS485TelemetryProtocolMismatch {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportRS485TelemetryInvalidPayload(t *testing.T) {
	svc := NewService()
	_, _, err := svc.ReportRS485Telemetry(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"gdv_demo_001",
		RS485TelemetryReport{},
	)
	if err == nil {
		t.Fatalf("expected invalid telemetry payload error")
	}
	if err != ErrGatewayDeviceRS485TelemetryInvalid {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPublishPullAndApplyConfig(t *testing.T) {
	svc := NewService()

	ack, err := svc.PublishConfig("tenant_demo_jakarta", "gw_demo_001", "cfg-20260413-a")
	if err != nil {
		t.Fatalf("publish config error: %v", err)
	}
	if ack.Command != "update_config" || ack.Status != "queued" {
		t.Fatalf("unexpected publish ack: %+v", ack)
	}

	pulled, err := svc.PullConfig("tenant_demo_jakarta", "gw_demo_001")
	if err != nil {
		t.Fatalf("pull config error: %v", err)
	}
	if pulled.DesiredVersion != "cfg-20260413-a" {
		t.Fatalf("unexpected desired_version after publish: %s", pulled.DesiredVersion)
	}
	if pulled.DesiredUpdatedAt == nil {
		t.Fatalf("expected desired_updated_at to be set")
	}
	if pulled.AppliedVersion != "" || pulled.AppliedAt != nil {
		t.Fatalf("expected applied version to be empty before ack, got version=%s applied_at=%v", pulled.AppliedVersion, pulled.AppliedAt)
	}

	applied, err := svc.MarkConfigApplied("tenant_demo_jakarta", "gw_demo_001", "cfg-20260413-a")
	if err != nil {
		t.Fatalf("mark config applied error: %v", err)
	}
	if applied.AppliedVersion != "cfg-20260413-a" {
		t.Fatalf("unexpected applied_version: %s", applied.AppliedVersion)
	}
	if applied.AppliedAt == nil {
		t.Fatalf("expected applied_at to be set")
	}

	pulled, err = svc.PullConfig("tenant_demo_jakarta", "gw_demo_001")
	if err != nil {
		t.Fatalf("pull config #2 error: %v", err)
	}
	if pulled.DesiredVersion != "cfg-20260413-a" || pulled.AppliedVersion != "cfg-20260413-a" {
		t.Fatalf("expected desired/applied to match after ack, got desired=%s applied=%s", pulled.DesiredVersion, pulled.AppliedVersion)
	}
}

func TestUpdateGatewayStatus(t *testing.T) {
	svc := NewService()
	item, err := svc.UpdateGatewayStatus("tenant_demo_jakarta", "gw_demo_001", "revoked")
	if err != nil {
		t.Fatalf("update gateway status: %v", err)
	}
	if item.Status != "revoked" {
		t.Fatalf("expected revoked status, got %s", item.Status)
	}

	found, ok := svc.FindGatewayByID("tenant_demo_jakarta", "gw_demo_001")
	if !ok {
		t.Fatalf("expected gateway lookup success")
	}
	if found.Status != "revoked" {
		t.Fatalf("expected stored revoked status, got %s", found.Status)
	}

	if _, err := svc.UpdateGatewayStatus("tenant_demo_jakarta", "gw_demo_001", "bad-status"); !errors.Is(err, ErrGatewayStatusInvalid) {
		t.Fatalf("expected invalid status error, got %v", err)
	}
	if _, err := svc.UpdateGatewayStatus("tenant_demo_jakarta", "gw_demo_001", ""); !errors.Is(err, ErrGatewayStatusRequired) {
		t.Fatalf("expected required status error, got %v", err)
	}
	if _, err := svc.UpdateGatewayStatus("tenant_demo_jakarta", "missing", "revoked"); !errors.Is(err, ErrGatewayNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestCertificateSerialRevocationLifecycle(t *testing.T) {
	svc := NewService()

	item, err := svc.RevokeCertificateSerial("tenant_demo_jakarta", "gw_demo_001", "00:AB:C1:23", "key exposed", "security@example.com")
	if err != nil {
		t.Fatalf("revoke cert serial: %v", err)
	}
	if item.SerialNumber != "abc123" {
		t.Fatalf("unexpected normalized serial: %s", item.SerialNumber)
	}
	if item.TenantID != "tenant_demo_jakarta" || item.GatewayID != "gw_demo_001" {
		t.Fatalf("unexpected revocation scope: %+v", item)
	}
	if item.Source != certificateRevocationSourceRuntime {
		t.Fatalf("unexpected revocation source: %s", item.Source)
	}
	if !svc.IsCertificateSerialRevoked("ab:c1:23") {
		t.Fatalf("expected certificate serial to be revoked")
	}

	items := svc.ListCertificateRevocations("tenant_demo_jakarta")
	if len(items) != 1 {
		t.Fatalf("expected one revocation, got %d", len(items))
	}

	removed, err := svc.RestoreCertificateSerial("tenant_demo_jakarta", "ABC123")
	if err != nil {
		t.Fatalf("restore cert serial: %v", err)
	}
	if removed.SerialNumber != "abc123" {
		t.Fatalf("unexpected restored serial: %s", removed.SerialNumber)
	}
	if svc.IsCertificateSerialRevoked("abc123") {
		t.Fatalf("expected certificate serial to be restored")
	}
}

func TestCertificateSerialRevocationPersistsToStateStore(t *testing.T) {
	store := &gatewayMemoryStateStore{}
	first, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("new first service: %v", err)
	}
	if _, err := first.RevokeCertificateSerial("tenant_demo_jakarta", "gw_demo_001", "0A:0B", "rotation", "ops@example.com"); err != nil {
		t.Fatalf("revoke serial: %v", err)
	}

	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("new restored service: %v", err)
	}
	if !restored.IsCertificateSerialRevoked("0a0b") {
		t.Fatalf("expected restored service to load certificate revocation")
	}
	items := restored.ListCertificateRevocations("tenant_demo_jakarta")
	if len(items) != 1 || items[0].SerialNumber != "a0b" {
		t.Fatalf("unexpected restored revocations: %+v", items)
	}
}

func TestMarkConfigAppliedValidation(t *testing.T) {
	svc := NewService()

	_, err := svc.MarkConfigApplied("tenant_demo_jakarta", "gw_demo_001", "")
	if err == nil {
		t.Fatalf("expected missing version to fail")
	}
	if err != ErrConfigVersionRequired {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpsertAndListEventCheckpoint(t *testing.T) {
	svc := NewService()
	occurredAt := time.Now().UTC().Add(-2 * time.Minute)

	first, err := svc.UpsertEventCheckpoint(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"default",
		"seq-10",
		"rq-10",
		10,
		&occurredAt,
	)
	if err != nil {
		t.Fatalf("upsert checkpoint #1 error: %v", err)
	}
	if first.CheckpointID != "seq-10" || first.AckedCount != 10 {
		t.Fatalf("unexpected checkpoint #1: %+v", first)
	}

	second, err := svc.UpsertEventCheckpoint(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"default",
		"seq-11",
		"rq-11",
		11,
		nil,
	)
	if err != nil {
		t.Fatalf("upsert checkpoint #2 error: %v", err)
	}
	if second.CheckpointID != "seq-11" || second.AckedCount != 11 {
		t.Fatalf("unexpected checkpoint #2: %+v", second)
	}

	items, err := svc.ListEventCheckpoints("tenant_demo_jakarta", "gw_demo_001")
	if err != nil {
		t.Fatalf("list checkpoints error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one checkpoint item, got %d", len(items))
	}
	if items[0].CheckpointID != "seq-11" || items[0].LastRequestID != "rq-11" {
		t.Fatalf("unexpected checkpoint list item: %+v", items[0])
	}
}

func TestUpsertEventCheckpointValidation(t *testing.T) {
	svc := NewService()

	_, err := svc.UpsertEventCheckpoint(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"default",
		"",
		"",
		0,
		nil,
	)
	if err == nil {
		t.Fatalf("expected empty checkpoint to fail")
	}
	if err != ErrGatewayEventCheckpointRequired {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.UpsertEventCheckpoint(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"default",
		"seq-1",
		"",
		-1,
		nil,
	)
	if err == nil {
		t.Fatalf("expected negative acked_count to fail")
	}
	if err != ErrGatewayEventCheckpointAckedCountInvalid {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpsertEventCheckpointAckedCountRegression(t *testing.T) {
	svc := NewService()

	first, err := svc.UpsertEventCheckpoint(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"default",
		"seq-5",
		"rq-5",
		5,
		nil,
	)
	if err != nil {
		t.Fatalf("upsert checkpoint #1 error: %v", err)
	}
	if first.AckedCount != 5 {
		t.Fatalf("unexpected first acked_count: %d", first.AckedCount)
	}

	_, err = svc.UpsertEventCheckpoint(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"default",
		"seq-4",
		"rq-4",
		4,
		nil,
	)
	if err == nil {
		t.Fatalf("expected acked_count regression to fail")
	}
	if err != ErrGatewayEventCheckpointAckedCountRegression {
		t.Fatalf("unexpected error: %v", err)
	}

	items, err := svc.ListEventCheckpoints("tenant_demo_jakarta", "gw_demo_001")
	if err != nil {
		t.Fatalf("list checkpoints error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one checkpoint item, got %d", len(items))
	}
	if items[0].CheckpointID != "seq-5" || items[0].AckedCount != 5 {
		t.Fatalf("checkpoint should remain unchanged after regression reject: %+v", items[0])
	}
}

func TestListEventCheckpointsByTenant(t *testing.T) {
	svc := NewService()

	_, err := svc.UpsertEventCheckpoint(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"default",
		"seq-jkt-1",
		"rq-jkt-1",
		1,
		nil,
	)
	if err != nil {
		t.Fatalf("upsert jakarta checkpoint error: %v", err)
	}
	_, err = svc.UpsertEventCheckpoint(
		"tenant_demo_factory",
		"gw_demo_002",
		"default",
		"seq-fct-1",
		"rq-fct-1",
		1,
		nil,
	)
	if err != nil {
		t.Fatalf("upsert factory checkpoint error: %v", err)
	}

	jakartaItems, err := svc.ListEventCheckpointsByTenant("tenant_demo_jakarta", "", "")
	if err != nil {
		t.Fatalf("list jakarta checkpoints error: %v", err)
	}
	if len(jakartaItems) != 1 || jakartaItems[0].GatewayID != "gw_demo_001" {
		t.Fatalf("unexpected jakarta checkpoint items: %+v", jakartaItems)
	}

	factoryItems, err := svc.ListEventCheckpointsByTenant("tenant_demo_factory", "", "")
	if err != nil {
		t.Fatalf("list factory checkpoints error: %v", err)
	}
	if len(factoryItems) != 1 || factoryItems[0].GatewayID != "gw_demo_002" {
		t.Fatalf("unexpected factory checkpoint items: %+v", factoryItems)
	}

	_, err = svc.ListEventCheckpointsByTenant("tenant_demo_jakarta", "gw_demo_002", "")
	if err == nil {
		t.Fatalf("expected cross-tenant gateway filter to fail")
	}
	if err != ErrGatewayNotFound {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetEventCheckpoint(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC().Add(-time.Minute)

	_, err := svc.UpsertEventCheckpoint(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"default",
		"seq-12",
		"rq-12",
		12,
		&now,
	)
	if err != nil {
		t.Fatalf("upsert checkpoint error: %v", err)
	}

	record, err := svc.GetEventCheckpoint("tenant_demo_jakarta", "gw_demo_001", "default")
	if err != nil {
		t.Fatalf("get checkpoint error: %v", err)
	}
	if record.CheckpointID != "seq-12" || record.AckedCount != 12 || record.LastRequestID != "rq-12" {
		t.Fatalf("unexpected checkpoint record: %+v", record)
	}

	_, err = svc.GetEventCheckpoint("tenant_demo_jakarta", "gw_demo_001", "priority")
	if err == nil {
		t.Fatalf("expected missing queue checkpoint to fail")
	}
	if err != ErrGatewayEventCheckpointNotFound {
		t.Fatalf("unexpected missing checkpoint error: %v", err)
	}
}

func TestAddAndGetQueueIngestTotal(t *testing.T) {
	svc := NewService()

	total1, err := svc.AddQueueIngestTotal("tenant_demo_jakarta", "gw_demo_001", "default", 2)
	if err != nil {
		t.Fatalf("add queue ingest total #1 error: %v", err)
	}
	if total1.IngestedTotal != 2 {
		t.Fatalf("unexpected ingest total #1: %+v", total1)
	}

	total2, err := svc.AddQueueIngestTotal("tenant_demo_jakarta", "gw_demo_001", "default", 3)
	if err != nil {
		t.Fatalf("add queue ingest total #2 error: %v", err)
	}
	if total2.IngestedTotal != 5 {
		t.Fatalf("unexpected ingest total #2: %+v", total2)
	}

	got, err := svc.GetQueueIngestTotal("tenant_demo_jakarta", "gw_demo_001", "default")
	if err != nil {
		t.Fatalf("get queue ingest total error: %v", err)
	}
	if got.IngestedTotal != 5 {
		t.Fatalf("queue ingest total mismatch: %+v", got)
	}

	_, err = svc.GetQueueIngestTotal("tenant_demo_jakarta", "gw_demo_001", "priority")
	if err == nil {
		t.Fatalf("expected missing queue ingest total to fail")
	}
	if err != ErrGatewayQueueIngestTotalNotFound {
		t.Fatalf("unexpected missing queue ingest total error: %v", err)
	}

	_, err = svc.AddQueueIngestTotal("tenant_demo_jakarta", "gw_demo_001", "default", 0)
	if err == nil {
		t.Fatalf("expected zero delta to fail")
	}
	if err != ErrGatewayQueueIngestDeltaInvalid {
		t.Fatalf("unexpected queue ingest delta error: %v", err)
	}
}

func TestCreateListAndUpdateOTATask(t *testing.T) {
	svc := NewService()

	created, err := svc.CreateOTATask(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"v2.4.1",
		"https://cdn.example.com/firmware/gw_demo_001/v2.4.1.bin",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		strings.Repeat("b", 128), // firmware_signature (now mandatory; format-only check)
		"tenant-admin@example.com",
	)
	if err != nil {
		t.Fatalf("create ota task error: %v", err)
	}
	if created.ID == "" || created.Status != gatewayOTATaskStatusQueued {
		t.Fatalf("unexpected created ota task: %+v", created)
	}

	items, err := svc.ListOTATasks("tenant_demo_jakarta", "gw_demo_001")
	if err != nil {
		t.Fatalf("list ota tasks error: %v", err)
	}
	if len(items) == 0 || items[0].ID != created.ID {
		t.Fatalf("expected created task in list, got %+v", items)
	}

	updated, err := svc.UpdateOTATaskStatus(
		"tenant_demo_jakarta",
		"gw_demo_001",
		created.ID,
		"failed",
		"gateway offline",
		"ops@example.com",
	)
	if err != nil {
		t.Fatalf("update ota task status error: %v", err)
	}
	if updated.Status != gatewayOTATaskStatusFailed || updated.ErrorMessage != "gateway offline" {
		t.Fatalf("unexpected updated ota task: %+v", updated)
	}
}

func TestCreateOTATaskValidation(t *testing.T) {
	svc := NewService()

	_, err := svc.CreateOTATask(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"",
		"https://cdn.example.com/firmware/gw_demo_001/v2.4.1.bin",
		"",
		"",
		"",
	)
	if err != ErrGatewayOTAFirmwareVersionRequired {
		t.Fatalf("unexpected missing version error: %v", err)
	}

	_, err = svc.CreateOTATask(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"v2.4.1",
		"",
		"",
		"",
		"",
	)
	if err != ErrGatewayOTAFirmwareURLRequired {
		t.Fatalf("unexpected missing url error: %v", err)
	}

	_, err = svc.CreateOTATask(
		"tenant_demo_jakarta",
		"gw_demo_001",
		"v2.4.1",
		"https://cdn.example.com/firmware/gw_demo_001/v2.4.1.bin",
		"invalid_sha256",
		"",
		"",
	)
	if err != ErrGatewayOTAFirmwareSHA256Invalid {
		t.Fatalf("unexpected invalid sha256 error: %v", err)
	}

	// empty sha256 → required
	_, err = svc.CreateOTATask(
		"tenant_demo_jakarta", "gw_demo_001", "v2.4.1",
		"https://cdn.example.com/firmware/gw_demo_001/v2.4.1.bin",
		"", strings.Repeat("b", 128), "",
	)
	if err != ErrGatewayOTAFirmwareSHA256Required {
		t.Fatalf("unexpected missing sha256 error: %v", err)
	}

	// valid sha256, empty signature → required
	_, err = svc.CreateOTATask(
		"tenant_demo_jakarta", "gw_demo_001", "v2.4.1",
		"https://cdn.example.com/firmware/gw_demo_001/v2.4.1.bin",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"", "",
	)
	if err != ErrGatewayOTAFirmwareSignatureRequired {
		t.Fatalf("unexpected missing signature error: %v", err)
	}

	// valid sha256, malformed signature → invalid
	_, err = svc.CreateOTATask(
		"tenant_demo_jakarta", "gw_demo_001", "v2.4.1",
		"https://cdn.example.com/firmware/gw_demo_001/v2.4.1.bin",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"not-a-valid-signature", "",
	)
	if err != ErrGatewayOTAFirmwareSignatureInvalid {
		t.Fatalf("unexpected invalid signature error: %v", err)
	}
}

func TestRecordFirmwareVersion(t *testing.T) {
	svc := NewService()
	if err := svc.RecordFirmwareVersion("tenant_demo_jakarta", "gw_demo_001", "1.4.0"); err != nil {
		t.Fatalf("record: %v", err)
	}
	// empty version must be a no-op (an older agent must not clobber a known version)
	if err := svc.RecordFirmwareVersion("tenant_demo_jakarta", "gw_demo_001", "  "); err != nil {
		t.Fatalf("empty record should be no-op, got %v", err)
	}
	found := false
	for _, g := range svc.List("tenant_demo_jakarta") {
		if g.ID == "gw_demo_001" {
			found = true
			if g.CurrentFirmwareVersion != "1.4.0" {
				t.Fatalf("want 1.4.0, got %q", g.CurrentFirmwareVersion)
			}
			if g.FirmwareReportedAt.IsZero() {
				t.Fatal("FirmwareReportedAt should be set")
			}
		}
	}
	if !found {
		t.Fatal("gw_demo_001 not found")
	}
	if err := svc.RecordFirmwareVersion("tenant_other", "gw_demo_001", "1.5.0"); err != ErrGatewayNotFound {
		t.Fatalf("want ErrGatewayNotFound for wrong tenant, got %v", err)
	}
}

func TestFirmwareSummary(t *testing.T) {
	svc := NewService()
	if err := svc.RecordFirmwareVersion("tenant_demo_jakarta", "gw_demo_001", "1.4.0"); err != nil {
		t.Fatal(err)
	}
	sum := svc.FirmwareSummary("tenant_demo_jakarta")
	if sum.Total < 1 {
		t.Fatalf("expected total>=1, got %d", sum.Total)
	}
	if sum.Reported < 1 {
		t.Fatalf("expected reported>=1, got %d", sum.Reported)
	}
	ok := false
	for _, v := range sum.Versions {
		if v.Version == "1.4.0" && v.Count >= 1 {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("expected 1.4.0 in versions, got %+v", sum.Versions)
	}
}
