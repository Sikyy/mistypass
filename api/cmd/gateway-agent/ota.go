package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("firmware exceeds %d-byte cap", maxBytes)
	}
	return data, nil
}

// otaMarker records an in-flight self-update awaiting post-restart confirmation.
type otaMarker struct {
	TaskID     string `json:"task_id"`
	TenantID   string `json:"tenant_id"`
	GatewayID  string `json:"gateway_id"`
	NewVersion string `json:"new_version"`
	BakPath    string `json:"bak_path"`
	Attempts   int    `json:"attempts"`
	Confirmed  bool   `json:"confirmed"`
}

func writeOTAMarker(path string, m otaMarker) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return atomicWriteFile(path, data, 0o600)
}

func readOTAMarker(path string) (otaMarker, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return otaMarker{}, false, nil
	}
	if err != nil {
		return otaMarker{}, false, err
	}
	var m otaMarker
	if err := json.Unmarshal(data, &m); err != nil {
		return otaMarker{}, false, err
	}
	return m, true, nil
}

// swapBinary backs up the current binary at binPath to bakPath (skipping the
// backup only when binPath does not yet exist — first install), then atomically
// replaces binPath with newData. Both writes are atomic (temp + rename in the
// same directory). Replacing a running binary via rename is safe on Linux: the
// running process keeps the old (unlinked) inode until it exits.
func swapBinary(newData []byte, binPath, bakPath string) error {
	cur, err := os.ReadFile(binPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("backup read: %w", err)
	}
	if cur != nil {
		if err := atomicWriteFile(bakPath, cur, 0o755); err != nil {
			return fmt.Errorf("backup write: %w", err)
		}
	}
	return atomicWriteFile(binPath, newData, 0o755)
}

// restoreBinary atomically copies bakPath back over binPath (rollback). Atomic
// because rollback is the last line of defense — a partial write here would
// leave no working binary at all.
func restoreBinary(binPath, bakPath string) error {
	data, err := os.ReadFile(bakPath)
	if err != nil {
		return err
	}
	return atomicWriteFile(binPath, data, 0o755)
}

// (used by Task 5/6) keep otasig referenced so imports stay tidy across tasks.
var _ = otasig.Domain
