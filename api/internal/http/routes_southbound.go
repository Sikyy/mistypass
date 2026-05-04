package httpx

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	eventmod "github.com/mistypass/cloud/api/internal/modules/event"
	"github.com/mistypass/cloud/api/internal/modules/gateway/southbound"
)

// isPrivateHost returns true if the host resolves to a private/loopback/link-local address.
func isPrivateHost(host string) bool {
	ips, err := net.LookupHost(host)
	if err != nil {
		// If we can't resolve, also block (fail-closed)
		return true
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return true
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return true
		}
	}
	return false
}

// --- Southbound Device Gateway Routes ---
// These endpoints allow the cloud platform to send commands to third-party
// door controllers (Hikvision, ZKTeco) that are on the customer's local network.
// In production, commands are relayed via the Gateway Agent; these endpoints
// are for direct-access scenarios (device has public IP or VPN tunnel).

// southboundUnlock remotely unlocks a door on a third-party device.
// POST /api/v1/gateway/southbound/{provider}/{deviceID}/unlock
func (s *server) southboundUnlock(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	providerName := chi.URLParam(r, "provider")
	deviceID := chi.URLParam(r, "deviceID")

	var req struct {
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		DoorIndex int    `json:"door_index"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	provider := resolveProvider(providerName)
	if provider == nil {
		writeError(w, http.StatusBadRequest, "unsupported provider: "+providerName)
		return
	}

	// SSRF protection: block private/loopback/link-local targets
	if isPrivateHost(req.Host) {
		writeError(w, http.StatusForbidden, "target host resolves to a private or loopback address")
		return
	}

	cfg := southbound.DeviceConfig{
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := provider.Unlock(ctx, cfg, req.DoorIndex); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"status":  "failed",
			"error":   err.Error(),
			"device":  deviceID,
			"provider": providerName,
		})
		return
	}

	s.auditSvc.Append(user.TenantID, user.Email, user.Role, "southbound_unlock", deviceID, providerName)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "unlocked",
		"device":     deviceID,
		"provider":   providerName,
		"door_index": req.DoorIndex,
	})
}

// southboundSyncUsers pushes user/credential data to a third-party device.
// POST /api/v1/gateway/southbound/{provider}/{deviceID}/sync-users
func (s *server) southboundSyncUsers(w http.ResponseWriter, r *http.Request) {
	_, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	providerName := chi.URLParam(r, "provider")

	var req struct {
		Host     string                `json:"host"`
		Port     int                   `json:"port"`
		Username string                `json:"username"`
		Password string                `json:"password"`
		Users    []southbound.DeviceUser `json:"users"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	provider := resolveProvider(providerName)
	if provider == nil {
		writeError(w, http.StatusBadRequest, "unsupported provider: "+providerName)
		return
	}

	if isPrivateHost(req.Host) {
		writeError(w, http.StatusForbidden, "target host resolves to a private or loopback address")
		return
	}

	cfg := southbound.DeviceConfig{
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := provider.SyncUsers(ctx, cfg, req.Users); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"status": "failed",
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "synced",
		"user_count": len(req.Users),
	})
}

// southboundTestConnection tests connectivity to a third-party device.
// POST /api/v1/gateway/southbound/{provider}/test
func (s *server) southboundTestConnection(w http.ResponseWriter, r *http.Request) {
	_, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	providerName := chi.URLParam(r, "provider")
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	provider := resolveProvider(providerName)
	if provider == nil {
		writeError(w, http.StatusBadRequest, "unsupported provider: "+providerName)
		return
	}

	if isPrivateHost(req.Host) {
		writeError(w, http.StatusForbidden, "target host resolves to a private or loopback address")
		return
	}

	cfg := southbound.DeviceConfig{
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := provider.TestConnection(ctx, cfg); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"reachable": false,
			"error":     err.Error(),
			"provider":  providerName,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"reachable": true,
		"provider":  providerName,
	})
}

// southboundZKTecoPush handles push events from ZKTeco devices.
// POST /api/v1/gateway/southbound/zkteco/push
// ZKTeco devices are configured to POST events to this endpoint.
func (s *server) southboundZKTecoPush(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}

	doorEvent, err := southbound.ParseZKPushEvent(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	serialNumber := doorEvent.Extra["serial_number"]
	s.logger.Info("zkteco push event",
		"serial", serialNumber,
		"event_type", doorEvent.EventType,
		"user_id", doorEvent.UserID,
		"door", doorEvent.DoorIndex,
	)

	// Map device serial → gateway → tenant
	tenantID, gatewayID := s.resolveZKTecoDevice(serialNumber)
	if tenantID == "" {
		s.logger.Warn("zkteco push: unknown device serial, event stored without tenant", "serial", serialNumber)
	}

	// Ingest as access event
	if tenantID != "" {
		doorID := fmt.Sprintf("zkteco:%s:door%d", serialNumber, doorEvent.DoorIndex)
		s.eventSvc.IngestAccessEvent(eventmod.IngestAccessEventInput{
			TenantID:  tenantID,
			Type:      doorEvent.EventType,
			Actor:     doorEvent.UserID,
			DoorID:    doorID,
			GatewayID: gatewayID,
			Result:    fmt.Sprintf("card=%s serial=%s", doorEvent.CardNumber, serialNumber),
			At:        parseZKTimestamp(doorEvent.Timestamp),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "received",
		"tenant_id": tenantID,
		"gateway_id": gatewayID,
	})
}

// resolveProvider returns the DoorProvider for a given provider name.
func resolveProvider(name string) southbound.DoorProvider {
	switch name {
	case "hikvision":
		return southbound.NewHikvisionDoorProvider()
	case "zkteco":
		return southbound.NewZKTecoDoorProvider()
	default:
		return nil
	}
}

// resolveZKTecoDevice maps a ZKTeco device serial number to tenant and gateway IDs.
// It iterates all gateways across tenants, matching by serial number or device serial.
func (s *server) resolveZKTecoDevice(serialNumber string) (tenantID, gatewayID string) {
	if serialNumber == "" {
		return "", ""
	}
	// Search gateways across all known tenants
	for _, t := range s.tenantSvc.List() {
		for _, gw := range s.gatewaySvc.List(t.ID) {
			if gw.SerialNumber == serialNumber {
				return gw.TenantID, gw.ID
			}
			// Also check gateway device sub-entries
			for _, dev := range gw.Devices {
				if dev.SerialNumber == serialNumber {
					return gw.TenantID, gw.ID
				}
			}
		}
	}
	return "", ""
}

// parseZKTimestamp parses a ZKTeco timestamp string into time.Time.
func parseZKTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Now().UTC()
	}
	// ZKTeco uses "2006-01-02 15:04:05" format
	t, err := time.Parse("2006-01-02 15:04:05", ts)
	if err != nil {
		return time.Now().UTC()
	}
	return t
}
