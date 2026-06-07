package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

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
