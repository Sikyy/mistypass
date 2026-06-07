package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/otasig"
)

const otaMaxFirmwareBytes = 256 << 20 // 256 MiB download cap

// errOTAVerifyFailed marks a deterministic signature/hash verification failure
// (as opposed to a transient download error). Such a task is skipped for the
// rest of this process lifetime to avoid re-downloading firmware that will
// never verify.
var errOTAVerifyFailed = errors.New("ota firmware verification failed")

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
	data, err := os.ReadFile(path) // #nosec G304 -- marker path is agent-controlled (deviceTokenFile dir), not user input
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
	cur, err := os.ReadFile(binPath) // #nosec G304 -- binPath is the agent's own executable (os.Executable), not user input
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
	data, err := os.ReadFile(bakPath) // #nosec G304 -- bakPath is the agent's own backup (binPath+".bak"), not user input
	if err != nil {
		return err
	}
	return atomicWriteFile(binPath, data, 0o755)
}

func (a *Agent) newOTAHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Minute}
}

func (a *Agent) otaMarkerPath() string {
	return filepath.Join(filepath.Dir(a.deviceTokenFile), "ota-pending.json")
}

// otaReportBody builds the JSON for POST /api/v1/gateway/ota/report.
// status ∈ {dispatching, succeeded, failed} (server enum has no "rolled_back";
// a rollback is reported as failed with an explanatory error_message).
func otaReportBody(gatewayID, tenantID, taskID, status, errMsg string) []byte {
	b, _ := json.Marshal(map[string]string{
		"gateway_id":    gatewayID,
		"tenant_id":     tenantID,
		"task_id":       taskID,
		"status":        status,
		"error_message": errMsg,
	})
	return b
}

func (a *Agent) reportOTA(task otaTask, status, errMsg string) error {
	resp, err := a.apiRequest("POST", "/api/v1/gateway/ota/report",
		otaReportBody(a.gatewayID, a.tenantID, task.ID, status, errMsg))
	if err != nil {
		a.logger.Warn("OTA report failed", "task", task.ID, "status", status, "error", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		a.logger.Warn("OTA report non-200", "task", task.ID, "status", status, "code", resp.StatusCode)
		return fmt.Errorf("report status %d", resp.StatusCode)
	}
	return nil
}

// maybeApplyOTA is invoked from pullConfig with the cloud's pending tasks.
func (a *Agent) maybeApplyOTA(tasks []otaTask) {
	if len(a.otaPublicKeys) == 0 {
		if len(tasks) > 0 {
			a.logger.Warn("OTA tasks present but no --ota-pubkey configured; ignoring", "count", len(tasks))
		}
		return
	}
	task, ok := selectOTATask(tasks, a.agentVersion)
	if !ok {
		return
	}
	a.mu.RLock()
	skip := a.otaVerifyFailed[task.ID]
	a.mu.RUnlock()
	if skip {
		return // already failed verification this process lifetime; don't re-download
	}
	if err := a.runOTA(task); err != nil {
		a.logger.Error("OTA apply failed", "task", task.ID, "error", err)
		if errors.Is(err, errOTAVerifyFailed) {
			a.mu.Lock()
			if a.otaVerifyFailed == nil {
				a.otaVerifyFailed = make(map[string]bool)
			}
			a.otaVerifyFailed[task.ID] = true
			a.mu.Unlock()
		}
		_ = a.reportOTA(task, "failed", err.Error())
	}
}

// runOTA downloads, verifies, and installs one firmware task, then exits so
// systemd restarts into the new binary. On any pre-install failure it returns
// an error WITHOUT touching the running binary.
func (a *Agent) runOTA(task otaTask) error {
	_ = a.reportOTA(task, "dispatching", "")

	data, err := downloadFirmware(a.newOTAHTTPClient(), task.FirmwareURL, otaMaxFirmwareBytes)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	if err := otasig.VerifyArtifact(a.otaPublicKeys, task.FirmwareVersion, task.FirmwareSHA256, task.FirmwareSignature, data); err != nil {
		return fmt.Errorf("%w: %v", errOTAVerifyFailed, err)
	}

	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate self: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(binPath); err == nil {
		binPath = resolved
	}
	bakPath := binPath + ".bak"

	if err := swapBinary(data, binPath, bakPath); err != nil {
		return fmt.Errorf("install: %w", err)
	}
	if err := writeOTAMarker(a.otaMarkerPath(), otaMarker{
		TaskID:     task.ID,
		TenantID:   task.TenantID,
		GatewayID:  task.GatewayID,
		NewVersion: task.FirmwareVersion,
		BakPath:    bakPath,
	}); err != nil {
		_ = restoreBinary(binPath, bakPath) // no confirm channel → abort safely
		return fmt.Errorf("write marker: %w", err)
	}

	a.logger.Info("OTA installed; exiting for systemd restart into new binary", "version", task.FirmwareVersion)
	os.Exit(0) // systemd Restart=always relaunches the new binary
	return nil // unreachable
}

// confirmPendingOTA finalizes a self-update once a post-restart pullConfig
// succeeds (proxy for "new binary is healthy"). Idempotent; retried each pull
// until the success report lands.
func (a *Agent) confirmPendingOTA() {
	path := a.otaMarkerPath()
	m, ok, err := readOTAMarker(path)
	if err != nil || !ok || m.Confirmed {
		return
	}
	if err := a.reportOTA(otaTask{ID: m.TaskID, TenantID: m.TenantID, GatewayID: m.GatewayID}, "succeeded", ""); err != nil {
		return // keep marker; retry next successful pull
	}
	m.Confirmed = true
	_ = writeOTAMarker(path, m) // tombstone before cleanup: a crash here must not trigger rollback of a healthy update
	_ = os.Remove(m.BakPath)
	_ = os.Remove(path)
	a.logger.Info("OTA confirmed healthy", "version", m.NewVersion)
}

// otaWatchdog rolls back if a pending update is not confirmed within timeout
// (covers "new binary starts but never reaches the cloud").
func (a *Agent) otaWatchdog(timeout time.Duration) {
	m, ok, err := readOTAMarker(a.otaMarkerPath())
	if err != nil || !ok || m.Confirmed {
		return
	}
	select {
	case <-time.After(timeout):
	case <-a.stopCh:
		return
	}
	if m2, ok, _ := readOTAMarker(a.otaMarkerPath()); !ok || m2.Confirmed {
		return // confirmed meanwhile
	}
	binPath, err := os.Executable()
	if err != nil {
		a.logger.Error("OTA watchdog: cannot locate self, skipping rollback", "error", err)
		return
	}
	if resolved, err := filepath.EvalSymlinks(binPath); err == nil {
		binPath = resolved
	}
	if err := restoreBinary(binPath, m.BakPath); err != nil {
		a.logger.Error("OTA rollback restore failed", "error", err)
		return
	}
	_ = a.reportOTA(otaTask{ID: m.TaskID, TenantID: m.TenantID, GatewayID: m.GatewayID}, "failed", "post-update health check timed out; rolled back")
	_ = os.Remove(a.otaMarkerPath())
	a.logger.Warn("OTA rolled back after health timeout; exiting for systemd restart into previous binary", "version", m.NewVersion)
	os.Exit(0)
}
