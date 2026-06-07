package main

import (
	"net/http"
	"net/http/httptest"
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
