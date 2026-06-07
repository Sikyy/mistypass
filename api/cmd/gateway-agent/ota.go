package main

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/mistypass/cloud/api/internal/otasig"
)

const otaMaxFirmwareBytes = 256 << 20 // 256 MiB download cap

// otaTask mirrors gateway.GatewayOTATask as delivered in the config/pull
// response under "pending_ota_tasks".
type otaTask struct {
	ID                string `json:"id"`
	GatewayID         string `json:"gateway_id"`
	TenantID          string `json:"tenant_id"`
	FirmwareVersion   string `json:"firmware_version"`
	FirmwareURL       string `json:"firmware_url"`
	FirmwareSHA256    string `json:"firmware_sha256"`
	FirmwareSignature string `json:"firmware_signature"`
	Status            string `json:"status"`
}

// compareVersions compares dotted numeric versions (a leading "v" is ignored).
// Returns -1 if a<b, 0 if equal, +1 if a>b. Non-numeric parts count as 0
// (MVP scope: no pre-release/build-metadata handling).
func compareVersions(a, b string) int {
	as := strings.Split(strings.TrimPrefix(strings.TrimSpace(a), "v"), ".")
	bs := strings.Split(strings.TrimPrefix(strings.TrimSpace(b), "v"), ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var ai, bi int
		if i < len(as) {
			ai, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bi, _ = strconv.Atoi(bs[i])
		}
		if ai != bi {
			if ai < bi {
				return -1
			}
			return 1
		}
	}
	return 0
}

// selectOTATask returns the newest signed task strictly newer than
// currentVersion. ok=false when there is nothing to apply.
func selectOTATask(tasks []otaTask, currentVersion string) (otaTask, bool) {
	var best otaTask
	found := false
	for _, t := range tasks {
		if strings.TrimSpace(t.FirmwareURL) == "" || strings.TrimSpace(t.FirmwareSignature) == "" {
			continue
		}
		if compareVersions(t.FirmwareVersion, currentVersion) <= 0 {
			continue // anti-downgrade
		}
		if !found || compareVersions(t.FirmwareVersion, best.FirmwareVersion) > 0 {
			best, found = t, true
		}
	}
	return best, found
}

// downloadFirmware fetches up to maxBytes from url.
func downloadFirmware(client *http.Client, url string, maxBytes int64) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBytes))
}

// (used by Task 5/6) keep otasig referenced so imports stay tidy across tasks.
var _ = otasig.Domain
