package main

import (
	"crypto/ed25519"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/otasig"
)

func otaTestLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.0", "1.2.0", 0},
		{"1.3.0", "1.2.9", 1},
		{"1.2.0", "1.10.0", -1},
		{"v2.0.0", "1.9.9", 1},
		{"1.2", "1.2.0", 0},
		{"1.2.alpha", "1.2.0", 0}, // malformed segment treated as 0 (MVP scope)
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSelectOTATaskPicksNewestAboveCurrent(t *testing.T) {
	tasks := []otaTask{
		{FirmwareVersion: "1.0.0", FirmwareURL: "u", FirmwareSignature: "s"},
		{FirmwareVersion: "1.3.0", FirmwareURL: "u", FirmwareSignature: "s"},
		{FirmwareVersion: "1.2.0", FirmwareURL: "u", FirmwareSignature: "s"},
	}
	got, ok := selectOTATask(tasks, "1.1.0")
	if !ok || got.FirmwareVersion != "1.3.0" {
		t.Fatalf("want 1.3.0, got %+v ok=%v", got, ok)
	}
}

func TestSelectOTATaskSkipsDowngradeEqualAndUnsigned(t *testing.T) {
	tasks := []otaTask{
		{FirmwareVersion: "1.0.0", FirmwareURL: "u", FirmwareSignature: "s"},
		{FirmwareVersion: "1.1.0", FirmwareURL: "u", FirmwareSignature: "s"},
		{FirmwareVersion: "2.0.0", FirmwareURL: "u", FirmwareSignature: ""}, // unsigned → ignored
	}
	if _, ok := selectOTATask(tasks, "1.1.0"); ok {
		t.Fatal("must not select <=current or unsigned tasks")
	}
}

func TestDownloadFirmware(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("firmware-bytes"))
	}))
	defer srv.Close()
	data, err := downloadFirmware(srv.Client(), srv.URL, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "firmware-bytes" {
		t.Fatalf("unexpected body %q", data)
	}
}

func TestDownloadFirmwareRejectsOversize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("0123456789"))
	}))
	defer srv.Close()
	if _, err := downloadFirmware(srv.Client(), srv.URL, 4); err == nil {
		t.Fatal("expected oversize error when body exceeds cap")
	}
}

func TestSwapBinaryBacksUpAndReplaces(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "agent")
	bak := bin + ".bak"
	if err := os.WriteFile(bin, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := swapBinary([]byte("NEW"), bin, bak); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(bin); string(b) != "NEW" {
		t.Fatalf("bin not replaced: %q", b)
	}
	if b, _ := os.ReadFile(bak); string(b) != "OLD" {
		t.Fatalf("backup not written: %q", b)
	}
}

func TestRestoreBinary(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "agent")
	bak := bin + ".bak"
	_ = os.WriteFile(bin, []byte("NEW-BROKEN"), 0o755)
	_ = os.WriteFile(bak, []byte("OLD-GOOD"), 0o755)
	if err := restoreBinary(bin, bak); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(bin); string(b) != "OLD-GOOD" {
		t.Fatalf("restore failed: %q", b)
	}
}

func TestOTAMarkerRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ota-pending.json")
	in := otaMarker{TaskID: "t1", NewVersion: "1.4.0", BakPath: "/x.bak", Attempts: 1}
	if err := writeOTAMarker(path, in); err != nil {
		t.Fatal(err)
	}
	got, ok, err := readOTAMarker(path)
	if err != nil || !ok {
		t.Fatalf("read marker ok=%v err=%v", ok, err)
	}
	if got.TaskID != "t1" || got.NewVersion != "1.4.0" {
		t.Fatalf("unexpected marker %+v", got)
	}
}

func TestReadOTAMarkerAbsent(t *testing.T) {
	_, ok, err := readOTAMarker(filepath.Join(t.TempDir(), "none.json"))
	if err != nil || ok {
		t.Fatalf("expected absent marker, ok=%v err=%v", ok, err)
	}
}

func TestSwapBinaryFirstInstall(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "agent")
	bak := bin + ".bak"
	// bin does not exist yet — first install, no backup expected.
	if err := swapBinary([]byte("NEW"), bin, bak); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(bin); string(b) != "NEW" {
		t.Fatalf("bin not written: %q", b)
	}
	if _, err := os.Stat(bak); !os.IsNotExist(err) {
		t.Fatal("backup should not exist on first install")
	}
}

func TestSwapBinaryAbortsWhenCurrentUnreadable(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "agent")
	if err := os.Mkdir(bin, 0o755); err != nil { // bin is a directory → ReadFile fails, not ENOENT
		t.Fatal(err)
	}
	if err := swapBinary([]byte("NEW"), bin, bin+".bak"); err == nil {
		t.Fatal("expected error when current binary is unreadable (not a clean first-install)")
	}
}

func TestOTAReportBody(t *testing.T) {
	body := otaReportBody("gw1", "t1", "task1", "failed", "boom")
	got := string(body)
	for _, want := range []string{`"gateway_id":"gw1"`, `"tenant_id":"t1"`, `"task_id":"task1"`, `"status":"failed"`, `"error_message":"boom"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("body %s missing %s", got, want)
		}
	}
}

// confirmPendingOTA must only report success when the RUNNING binary is the
// pending version; otherwise it leaves the marker for the watchdog to roll back.
func TestConfirmPendingOTAVersionGate(t *testing.T) {
	dir := t.TempDir()
	bak := filepath.Join(dir, "agent.bak")
	_ = os.WriteFile(bak, []byte("old"), 0o755)
	var reported []string
	a := &Agent{
		logger:          otaTestLogger(),
		deviceTokenFile: filepath.Join(dir, "device-token"),
		agentVersion:    "1.3.0",
		reportOTAFn:     func(_ otaTask, status, _ string) error { reported = append(reported, status); return nil },
	}
	if err := writeOTAMarker(a.otaMarkerPath(), otaMarker{TaskID: "t1", NewVersion: "1.4.0", BakPath: bak}); err != nil {
		t.Fatal(err)
	}

	// running 1.3.0 != pending 1.4.0 → must NOT confirm, marker stays.
	a.confirmPendingOTA()
	if len(reported) != 0 {
		t.Fatalf("must not report on version mismatch, got %v", reported)
	}
	if _, ok, _ := readOTAMarker(a.otaMarkerPath()); !ok {
		t.Fatal("marker must remain for rollback on version mismatch")
	}

	// running version now matches → confirm + cleanup.
	a.agentVersion = "1.4.0"
	a.confirmPendingOTA()
	if len(reported) != 1 || reported[0] != "succeeded" {
		t.Fatalf("want one 'succeeded', got %v", reported)
	}
	if _, ok, _ := readOTAMarker(a.otaMarkerPath()); ok {
		t.Fatal("marker should be removed after confirm")
	}
	if _, err := os.Stat(bak); !os.IsNotExist(err) {
		t.Fatal("bak should be removed after confirm")
	}
}

// A task that fails signature verification must be downloaded at most once per
// process lifetime (in-memory skip-set), not re-fetched on every poll.
func TestMaybeApplyOTASkipsVerifyFailedTask(t *testing.T) {
	pub, _, _ := otasig.GenerateKey()
	body := []byte("unsigned firmware bytes")
	var hits int32
	fw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write(body)
	}))
	defer fw.Close()

	dir := t.TempDir()
	a := &Agent{
		logger:          otaTestLogger(),
		deviceTokenFile: filepath.Join(dir, "device-token"),
		agentVersion:    "1.0.0",
		otaPublicKeys:   []ed25519.PublicKey{pub},
		resolveSelf:     func() (string, error) { return filepath.Join(dir, "agent"), nil },
		exitFunc:        func(int) {},
		reportOTAFn:     func(otaTask, string, string) error { return nil },
	}
	task := otaTask{
		ID: "t1", FirmwareVersion: "1.4.0", FirmwareURL: fw.URL,
		FirmwareSHA256:    otasig.SHA256Hex(body),   // sha matches → fails at the signature step
		FirmwareSignature: strings.Repeat("0", 128), // valid-length but invalid Ed25519 signature
	}
	a.maybeApplyOTA([]otaTask{task})
	a.maybeApplyOTA([]otaTask{task})
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("verify-failed task must be downloaded once, got %d hits", got)
	}
}

// runOTA happy path: download → verify → write marker → swap → exit, leaving the
// new binary in place, the old one in .bak, and a marker that points to it.
func TestRunOTAInstallsAndMarks(t *testing.T) {
	pub, priv, _ := otasig.GenerateKey()
	fwBytes := []byte("new agent binary 1.4.0")
	sha := otasig.SHA256Hex(fwBytes)
	sig := otasig.Sign(priv, "1.4.0", sha)
	fw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(fwBytes) }))
	defer fw.Close()

	dir := t.TempDir()
	binPath := filepath.Join(dir, "agent")
	_ = os.WriteFile(binPath, []byte("old binary"), 0o755)

	var statuses []string
	exited := false
	a := &Agent{
		logger:          otaTestLogger(),
		deviceTokenFile: filepath.Join(dir, "device-token"),
		agentVersion:    "1.0.0",
		otaPublicKeys:   []ed25519.PublicKey{pub},
		resolveSelf:     func() (string, error) { return binPath, nil },
		exitFunc:        func(int) { exited = true },
		reportOTAFn:     func(_ otaTask, status, _ string) error { statuses = append(statuses, status); return nil },
	}
	task := otaTask{ID: "t1", TenantID: "tn", GatewayID: "gw", FirmwareVersion: "1.4.0", FirmwareURL: fw.URL, FirmwareSHA256: sha, FirmwareSignature: sig}
	if err := a.runOTA(task); err != nil {
		t.Fatalf("runOTA: %v", err)
	}
	if !exited {
		t.Fatal("expected exitFunc to be called")
	}
	if b, _ := os.ReadFile(binPath); string(b) != string(fwBytes) {
		t.Fatalf("binary not swapped: %q", b)
	}
	if b, _ := os.ReadFile(binPath + ".bak"); string(b) != "old binary" {
		t.Fatalf("backup not written: %q", b)
	}
	m, ok, _ := readOTAMarker(a.otaMarkerPath())
	if !ok || m.NewVersion != "1.4.0" || m.BakPath != binPath+".bak" {
		t.Fatalf("marker wrong: %+v ok=%v", m, ok)
	}
	if len(statuses) == 0 || statuses[0] != "dispatching" {
		t.Fatalf("expected 'dispatching' first, got %v", statuses)
	}
}

// otaWatchdog rolls back when a pending update is never confirmed within timeout.
func TestOTAWatchdogRollsBackUnconfirmed(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "agent")
	bak := binPath + ".bak"
	_ = os.WriteFile(binPath, []byte("new-broken"), 0o755)
	_ = os.WriteFile(bak, []byte("old-good"), 0o755)

	var failedMsg string
	exited := false
	a := &Agent{
		logger:          otaTestLogger(),
		deviceTokenFile: filepath.Join(dir, "device-token"),
		stopCh:          make(chan struct{}),
		resolveSelf:     func() (string, error) { return binPath, nil },
		exitFunc:        func(int) { exited = true },
		reportOTAFn: func(_ otaTask, status, msg string) error {
			if status == "failed" {
				failedMsg = msg
			}
			return nil
		},
	}
	if err := writeOTAMarker(a.otaMarkerPath(), otaMarker{TaskID: "t1", NewVersion: "1.4.0", BakPath: bak}); err != nil {
		t.Fatal(err)
	}

	a.otaWatchdog(10 * time.Millisecond)

	if b, _ := os.ReadFile(binPath); string(b) != "old-good" {
		t.Fatalf("binary not rolled back: %q", b)
	}
	if failedMsg == "" {
		t.Fatal("expected a 'failed' rollback report")
	}
	if !exited {
		t.Fatal("expected exitFunc after rollback")
	}
	if _, ok, _ := readOTAMarker(a.otaMarkerPath()); ok {
		t.Fatal("marker should be removed after rollback")
	}
}
