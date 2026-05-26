package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/push"
)

const stateKeyMobilePushDevices = "mobile_push_devices"

type mobilePushSmokeRequest struct {
	TenantID string            `json:"tenant_id"`
	UserID   string            `json:"user_id"`
	DeviceID string            `json:"device_id"`
	Token    string            `json:"token"`
	Type     string            `json:"type"`
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	Data     map[string]string `json:"data"`
}

func (s *server) getMobilePushProviderStatus(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	missing := s.mobilePushMissingConfig()
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":                   s.mobilePushProvider != nil,
		"provider":                  firstNonEmptyString(providerName(s.mobilePushProvider), push.FCMProviderName),
		"configured":                s.mobilePushProvider != nil && len(missing) == 0,
		"missing":                   missing,
		"tenant_id":                 tenantID,
		"registered_android_tokens": s.countPushDevices(tenantID, "android"),
	})
}

func (s *server) sendMobilePushSmoke(w http.ResponseWriter, r *http.Request) {
	if s.mobilePushProvider == nil {
		writeError(w, http.StatusServiceUnavailable, "fcm push provider is not enabled")
		return
	}
	var request mobilePushSmokeRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, firstNonEmptyString(request.TenantID, r.URL.Query().Get("tenant_id")))
	if !ok {
		return
	}

	targetToken := strings.TrimSpace(request.Token)
	targetDevice := pushDevice{}
	if targetToken == "" {
		var found bool
		targetDevice, found = s.latestPushDevice(tenantID, request.UserID, request.DeviceID, "android")
		if !found {
			writeError(w, http.StatusNotFound, "no registered android push token found")
			return
		}
		targetToken = targetDevice.FCMToken
	}

	msgType := firstNonEmptyString(request.Type, "push_smoke")
	title := firstNonEmptyString(request.Title, "MistyPass push smoke")
	body := firstNonEmptyString(request.Body, fmt.Sprintf("FCM smoke at %s", time.Now().UTC().Format(time.RFC3339)))
	data := map[string]string{
		"type":      msgType,
		"tenant_id": tenantID,
		"smoke":     "true",
	}
	for key, value := range request.Data {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		data[key] = strings.TrimSpace(value)
	}

	receipt, err := s.mobilePushProvider.Send(r.Context(), push.Message{
		Token:            targetToken,
		Type:             msgType,
		Title:            title,
		Body:             body,
		Data:             data,
		AndroidChannelID: "credential_alerts",
		TTL:              5 * time.Minute,
	})
	if err != nil {
		if httpErr := (push.HTTPError{}); errors.As(err, &httpErr) {
			writeJSON(w, http.StatusBadGateway, map[string]any{
				"error":         "fcm push provider error",
				"provider":      s.mobilePushProvider.Provider(),
				"status_code":   httpErr.StatusCode,
				"retryable":     httpErr.Retryable(),
				"provider_body": httpErr.Body,
			})
			return
		}
		writeInternalError(w, r, err)
		return
	}

	if user, ok := authenticatedUser(r); ok {
		s.appendAuditLog(r, tenantID, "mobile_push_smoke_sent",
			fmt.Sprintf("provider=%s,user_id=%s,device_id=%s,message_id=%s", receipt.Provider, request.UserID, firstNonEmptyString(request.DeviceID, targetDevice.DeviceID), receipt.ProviderMessageID),
			"mobile_push")
		_ = user
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "sent",
		"receipt": receipt,
		"target": map[string]any{
			"tenant_id":     tenantID,
			"user_id":       firstNonEmptyString(request.UserID, targetDevice.UserID),
			"device_id":     firstNonEmptyString(request.DeviceID, targetDevice.DeviceID),
			"device_model":  targetDevice.DeviceModel,
			"platform":      firstNonEmptyString(targetDevice.Platform, "android"),
			"token_present": targetToken != "",
		},
	})
}

func (s *server) upsertPushDevice(device pushDevice) {
	token := strings.TrimSpace(device.FCMToken)
	if token == "" {
		return
	}
	now := time.Now().UTC()
	if device.Platform == "" {
		device.Platform = "android"
	}
	s.pushDeviceMu.Lock()
	if existing, ok := s.pushDevices[token]; ok && !existing.RegisteredAt.IsZero() {
		device.RegisteredAt = existing.RegisteredAt
	}
	if device.RegisteredAt.IsZero() {
		device.RegisteredAt = now
	}
	device.UpdatedAt = now
	s.pushDevices[token] = device
	s.evictStalePushDevicesLocked(now)
	s.persistPushDevicesLocked()
	s.pushDeviceMu.Unlock()
}

func (s *server) latestPushDevice(tenantID, userID, deviceID, platform string) (pushDevice, bool) {
	tenantID = strings.TrimSpace(tenantID)
	userID = strings.TrimSpace(userID)
	deviceID = strings.TrimSpace(deviceID)
	platform = strings.ToLower(strings.TrimSpace(platform))
	s.pushDeviceMu.Lock()
	defer s.pushDeviceMu.Unlock()

	var latest pushDevice
	found := false
	for _, device := range s.pushDevices {
		if tenantID != "" && device.TenantID != tenantID {
			continue
		}
		if userID != "" && device.UserID != userID {
			continue
		}
		if deviceID != "" && device.DeviceID != deviceID {
			continue
		}
		if platform != "" && strings.ToLower(strings.TrimSpace(device.Platform)) != platform {
			continue
		}
		if !found || devicePushUpdatedAt(device).After(devicePushUpdatedAt(latest)) {
			latest = device
			found = true
		}
	}
	return latest, found
}

func (s *server) countPushDevices(tenantID, platform string) int {
	tenantID = strings.TrimSpace(tenantID)
	platform = strings.ToLower(strings.TrimSpace(platform))
	s.pushDeviceMu.Lock()
	defer s.pushDeviceMu.Unlock()

	count := 0
	for _, device := range s.pushDevices {
		if tenantID != "" && device.TenantID != tenantID {
			continue
		}
		if platform != "" && strings.ToLower(strings.TrimSpace(device.Platform)) != platform {
			continue
		}
		count++
	}
	return count
}

func (s *server) restorePushDevicesFromState() {
	if s.stateStore == nil {
		return
	}
	var snapshot pushDeviceStateSnapshot
	found, err := s.stateStore.Load(stateKeyMobilePushDevices, &snapshot)
	if err != nil {
		s.loggerOrDefault().Warn("restore mobile push devices failed", "error", err)
		return
	}
	if !found {
		return
	}
	now := time.Now().UTC()
	s.pushDeviceMu.Lock()
	for _, device := range snapshot.Devices {
		token := strings.TrimSpace(device.FCMToken)
		if token == "" {
			continue
		}
		if device.RegisteredAt.IsZero() {
			device.RegisteredAt = now
		}
		if device.UpdatedAt.IsZero() {
			device.UpdatedAt = device.RegisteredAt
		}
		s.pushDevices[token] = device
	}
	s.evictStalePushDevicesLocked(now)
	s.pushDeviceMu.Unlock()
}

func (s *server) persistPushDevicesLocked() {
	if s.stateStore == nil {
		return
	}
	devices := make([]pushDevice, 0, len(s.pushDevices))
	for _, device := range s.pushDevices {
		devices = append(devices, device)
	}
	sort.Slice(devices, func(i, j int) bool {
		return devicePushUpdatedAt(devices[i]).After(devicePushUpdatedAt(devices[j]))
	})
	if err := s.stateStore.Save(stateKeyMobilePushDevices, pushDeviceStateSnapshot{Devices: devices}); err != nil {
		s.loggerOrDefault().Warn("persist mobile push devices failed", "error", err)
	}
}

func (s *server) evictStalePushDevicesLocked(now time.Time) {
	if len(s.pushDevices) <= 1000 {
		return
	}
	cutoff := now.Add(-90 * 24 * time.Hour)
	for token, device := range s.pushDevices {
		if devicePushUpdatedAt(device).Before(cutoff) {
			delete(s.pushDevices, token)
		}
	}
}

func devicePushUpdatedAt(device pushDevice) time.Time {
	if !device.UpdatedAt.IsZero() {
		return device.UpdatedAt
	}
	return device.RegisteredAt
}

func (s *server) mobilePushMissingConfig() []string {
	if s.mobilePushProvider != nil {
		return nil
	}
	if !s.cfg.FCMEnabled {
		return []string{"FCM_ENABLED"}
	}
	missing := make([]string, 0, 3)
	if strings.TrimSpace(s.cfg.FCMProjectID) == "" {
		missing = append(missing, "FCM_PROJECT_ID")
	}
	if strings.TrimSpace(s.cfg.FCMServiceAccountFile) == "" && strings.TrimSpace(s.cfg.FCMServiceAccountJSON) == "" {
		missing = append(missing, "FCM_SERVICE_ACCOUNT_FILE")
	}
	return missing
}

func providerName(provider push.Provider) string {
	if provider == nil {
		return ""
	}
	return provider.Provider()
}
