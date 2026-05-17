package httpx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/bus"
	"github.com/mistypass/cloud/api/internal/modules/camera"
)

// ─────────────────────────────────────────────────────────────────────────────
// PATCH /api/v1/app/me — Update user profile preferences (language, name)
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appUpdateMe(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		Name     *string `json:"name"`
		Language *string `json:"language"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Use current values as defaults
	nextName := user.Name
	nextLanguage := user.Language
	if req.Name != nil {
		nextName = *req.Name
	}
	if req.Language != nil {
		nextLanguage = *req.Language
	}

	updated, err := s.authService.UpdateUserProfile(user.ID, nextName, nextLanguage)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}

	// Enrich response with role_display_label and organization_name
	writeJSON(w, http.StatusOK, s.buildAppMePayload(
		updated.ID, updated.Name, updated.Email,
		updated.Role, updated.TenantID, updated.Language,
		updated.BuildingIDs,
	))
}

// appMeEnhanced returns the enhanced /me response with role and org info.
// GET /api/v1/app/me (replaces old appMe)
func (s *server) appMeEnhanced(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	writeJSON(w, http.StatusOK, s.buildAppMePayload(
		user.ID, user.Name, user.Email,
		user.Role, user.TenantID, user.Language,
		user.BuildingIDs,
	))
}

// buildAppMePayload constructs the enhanced /app/me response.
func (s *server) buildAppMePayload(userID, name, email, role, tenantID, language string, buildingIDs []string) map[string]any {
	roleLabel := roleDisplayLabel(role)
	orgName := ""
	tenants := s.tenantSvc.List()
	for _, t := range tenants {
		if t.ID == tenantID {
			orgName = t.Name
			break
		}
	}

	return map[string]any{
		"id":                 userID,
		"name":               name,
		"email":              email,
		"role":               role,
		"role_display_label": roleLabel,
		"tenant_id":          tenantID,
		"organization_name":  orgName,
		"building_ids":       buildingIDs,
		"language":           language,
	}
}

func roleDisplayLabel(role string) string {
	switch role {
	case "super_admin":
		return "Super Admin"
	case "tenant_admin":
		return "Building Admin"
	case "building_manager":
		return "Building Manager"
	case "resident":
		return "Resident"
	case "security":
		return "Security"
	default:
		return role
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PIN-based unlock
// GET  /api/v1/app/access/pin-code — Get current dynamic PIN (TOTP-based, 30s)
// POST /api/v1/app/access/pin-unlock — Verify PIN and unlock door
// ─────────────────────────────────────────────────────────────────────────────

const pinTOTPPeriod = 30 // seconds

func (s *server) appGetPINCode(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	now := time.Now().UTC()
	pin := generateTOTPPin(s.cfg.JWTSecret, user.TenantID, user.ID, now)
	validUntil := now.Truncate(time.Duration(pinTOTPPeriod) * time.Second).Add(time.Duration(pinTOTPPeriod) * time.Second)

	writeJSON(w, http.StatusOK, map[string]any{
		"pin":         pin,
		"valid_until": validUntil.Format(time.RFC3339),
		"period_secs": pinTOTPPeriod,
	})
}

func (s *server) appPINUnlock(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		LockID string `json:"lock_id"`
		PIN    string `json:"pin"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	lockID := strings.TrimSpace(req.LockID)
	pin := strings.TrimSpace(req.PIN)
	if lockID == "" || pin == "" {
		writeError(w, http.StatusBadRequest, "lock_id and pin are required")
		return
	}

	now := time.Now().UTC()
	tenantID := user.TenantID

	// Verify the door exists
	door, err := s.spaceSvc.GetDoor(tenantID, lockID)
	if err != nil {
		writeError(w, http.StatusNotFound, "door not found")
		return
	}

	// Check user access to this door (same logic as appUnlockDoor)
	accessUser, err := s.accessSvc.GetUser(tenantID, user.ID)
	if err != nil || accessUser.Status != "active" {
		s.recordAppAccessEvent(tenantID, lockID, user.Email, "access_denied", "user_not_active")
		writeJSON(w, http.StatusForbidden, map[string]any{
			"decision": "deny",
			"reason":   "user_not_active",
			"lock_id":  lockID,
		})
		return
	}

	allowed, groupName := s.checkUserLockAccess(tenantID, accessUser.GroupIDs, lockID)
	if !allowed {
		allowed = s.checkRoleAssignmentAccess(tenantID, user.ID, door.BuildingID)
		if allowed {
			groupName = "role_assignment"
		}
	}

	if !allowed {
		s.recordAppAccessEvent(tenantID, lockID, user.Email, "access_denied", "no_access")
		writeJSON(w, http.StatusForbidden, map[string]any{
			"decision": "deny",
			"reason":   "no_access",
			"lock_id":  lockID,
		})
		return
	}

	// Verify PIN — allow current period and previous period (clock skew tolerance)
	validPIN := false
	currentPIN := generateTOTPPin(s.cfg.JWTSecret, tenantID, user.ID, now)
	prevPIN := generateTOTPPin(s.cfg.JWTSecret, tenantID, user.ID, now.Add(-time.Duration(pinTOTPPeriod)*time.Second))
	if pin == currentPIN || pin == prevPIN {
		validPIN = true
	}

	if !validPIN {
		s.recordAppAccessEvent(tenantID, lockID, user.Email, "access_denied", "invalid_pin")
		writeJSON(w, http.StatusForbidden, map[string]any{
			"decision": "deny",
			"reason":   "invalid_pin",
			"lock_id":  lockID,
		})
		return
	}

	// Dispatch unlock command
	requestID := fmt.Sprintf("pin:%s:%s:%d", lockID, user.ID, now.UnixNano())
	dispatched := false

	if gw, found := s.gatewaySvc.FindGatewayByDoorID(tenantID, lockID); found {
		cmd := bus.GatewayCommand{
			RequestID: requestID,
			GatewayID: gw.ID,
			Command:   "unlock",
			LockID:    lockID,
			PlaceID:   door.BuildingID,
			TenantID:  tenantID,
			IssuedBy:  user.Email,
			IssuedAt:  now.Format(time.RFC3339),
		}
		subject := fmt.Sprintf("gateway.%s.command", gw.ID)
		if err := s.messageBus.PublishJSON(r.Context(), subject, cmd, nil); err != nil {
			s.logger.Warn("pin unlock: failed to dispatch", "error", err)
		} else if s.messageBus.Enabled() {
			dispatched = true
		}
	}

	status := "accepted"
	if dispatched {
		status = "dispatched"
	}

	s.recordAppAccessEvent(tenantID, lockID, user.Email, "access_granted", "pin_unlock")

	writeJSON(w, http.StatusOK, map[string]any{
		"decision":   "allow",
		"reason":     "access_granted",
		"status":     status,
		"request_id": requestID,
		"lock_id":    lockID,
		"lock_name":  door.Name,
		"user_id":    user.ID,
		"group_name": groupName,
		"method":     "pin",
		"issued_at":  now.Format(time.RFC3339),
	})
}

// generateTOTPPin produces a 6-digit TOTP PIN using tenant+user as seed.
func generateTOTPPin(baseKey, tenantID, userID string, t time.Time) string {
	// TOTP: counter = floor(time / period)
	counter := uint64(t.Unix()) / uint64(pinTOTPPeriod)

	// Key derivation: HMAC-SHA256(baseKey:tenantID, userID)
	key := hmac.New(sha256.New, []byte(baseKey+":"+tenantID))
	key.Write([]byte(userID))
	secret := key.Sum(nil)

	// TOTP calculation (RFC 6238 style)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	mac := hmac.New(sha256.New, secret)
	mac.Write(buf)
	hash := mac.Sum(nil)

	// Dynamic truncation
	offset := hash[len(hash)-1] & 0x0f
	code := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff

	// 6-digit PIN
	pin := code % uint32(math.Pow(10, 6))
	return fmt.Sprintf("%06d", pin)
}

// ─────────────────────────────────────────────────────────────────────────────
// PUT /api/v1/app/access/doors/{doorId}/favorite — Toggle door favorite
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appToggleDoorFavorite(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	doorID := chi.URLParam(r, "doorId")
	if strings.TrimSpace(doorID) == "" {
		writeError(w, http.StatusBadRequest, "door ID is required")
		return
	}

	var req struct {
		Favorite bool `json:"favorite"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := user.TenantID

	// Verify door exists
	_, err := s.spaceSvc.GetDoor(tenantID, doorID)
	if err != nil {
		writeError(w, http.StatusNotFound, "door not found")
		return
	}

	// Store favorite in server-side map (per user)
	s.setDoorFavorite(user.ID, doorID, req.Favorite)

	writeJSON(w, http.StatusOK, map[string]any{
		"door_id":  doorID,
		"favorite": req.Favorite,
	})
}

// In-memory favorites store (production would use persistence)
func (s *server) setDoorFavorite(userID, doorID string, favorite bool) {
	s.doorFavoriteMu.Lock()
	defer s.doorFavoriteMu.Unlock()
	if s.doorFavorites == nil {
		s.doorFavorites = make(map[string]map[string]bool)
	}
	if s.doorFavorites[userID] == nil {
		s.doorFavorites[userID] = make(map[string]bool)
	}
	if favorite {
		s.doorFavorites[userID][doorID] = true
	} else {
		delete(s.doorFavorites[userID], doorID)
	}
}

func (s *server) isDoorFavorite(userID, doorID string) bool {
	s.doorFavoriteMu.RLock()
	defer s.doorFavoriteMu.RUnlock()
	if s.doorFavorites == nil {
		return false
	}
	return s.doorFavorites[userID][doorID]
}

// ─────────────────────────────────────────────────────────────────────────────
// Mobile camera endpoints
// GET  /api/v1/app/cameras — List cameras accessible to user
// GET  /api/v1/app/cameras/{id}/video-link — Get HLS/RTSP playback URL
// POST /api/v1/app/cameras/{id}/snapshot — Trigger manual snapshot
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appListCameras(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	tenantID := user.TenantID
	cameras := s.cameraSvc.List(tenantID)

	// For residents, only show cameras linked to doors they have access to
	// For admins, show all cameras in their tenant
	isAdmin := user.Role != "resident"

	var accessibleDoorIDs map[string]bool
	if !isAdmin {
		accessibleDoorIDs = s.getUserAccessibleDoorIDs(tenantID, user.ID)
	}

	items := make([]map[string]any, 0, len(cameras))
	for _, cam := range cameras {
		if !isAdmin {
			// Resident: check if camera's door is accessible
			if cam.DoorID != "" && !accessibleDoorIDs[cam.DoorID] {
				continue
			}
			if cam.DoorID == "" {
				continue // Residents can't see cameras not linked to a door
			}
		}

		items = append(items, map[string]any{
			"id":          cam.ID,
			"name":        cam.Name,
			"building_id": cam.PlaceID,
			"door_id":     cam.DoorID,
			"status":      cam.Status,
			"provider":    cam.Provider,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) appCameraVideoLink(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	cameraID := chi.URLParam(r, "cameraID")
	if strings.TrimSpace(cameraID) == "" {
		writeError(w, http.StatusBadRequest, "camera ID is required")
		return
	}

	tenantID := user.TenantID
	cam, err := s.cameraSvc.Get(tenantID, cameraID)
	if err != nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	// Check access: admin or resident with door access
	if user.Role == "resident" {
		accessibleDoorIDs := s.getUserAccessibleDoorIDs(tenantID, user.ID)
		if cam.DoorID == "" || !accessibleDoorIDs[cam.DoorID] {
			writeError(w, http.StatusForbidden, "no access to this camera")
			return
		}
	}

	videoLink, err := s.cameraSvc.GetVideoLink(r.Context(), tenantID, cameraID)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"camera_id":  cameraID,
		"video_url":  videoLink,
		"protocol":   "hls",
		"expires_at": time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339),
	})
}

func (s *server) appCameraSnapshot(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	cameraID := chi.URLParam(r, "cameraID")
	if strings.TrimSpace(cameraID) == "" {
		writeError(w, http.StatusBadRequest, "camera ID is required")
		return
	}

	tenantID := user.TenantID
	cam, err := s.cameraSvc.Get(tenantID, cameraID)
	if err != nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	// Only admins can manually trigger snapshots
	if user.Role == "resident" {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	snap, err := s.cameraSvc.CaptureSnapshot(r.Context(), tenantID, cameraID, "", "manual")
	if err != nil {
		writeInternalError(w, r, err)
		return
	}

	_ = cam // used for validation above
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         snap.ID,
		"camera_id":  cameraID,
		"url":        snap.SignedURL,
		"taken_at":   snap.CapturedAt,
		"event_type": "manual",
	})
}

func (s *server) appCameraCloudToken(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	cameraID := chi.URLParam(r, "cameraID")
	if strings.TrimSpace(cameraID) == "" {
		writeError(w, http.StatusBadRequest, "camera ID is required")
		return
	}

	tenantID := user.TenantID
	cam, err := s.cameraSvc.Get(tenantID, cameraID)
	if err != nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	if cam.CloudProvider == "" || cam.CloudSerial == "" {
		writeError(w, http.StatusBadRequest, "camera has no cloud provider configured")
		return
	}

	if !cam.CloudVerified {
		writeError(w, http.StatusConflict, "camera cloud binding not verified")
		return
	}

	// Check access: admin or resident with door access
	if user.Role == "resident" {
		accessibleDoorIDs := s.getUserAccessibleDoorIDs(tenantID, user.ID)
		if cam.DoorID == "" || !accessibleDoorIDs[cam.DoorID] {
			writeError(w, http.StatusForbidden, "no access to this camera")
			return
		}
	}

	if s.hikConnectSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "cloud video service not configured")
		return
	}

	channel := cam.CloudChannels
	if channel <= 0 {
		channel = 1
	}

	token, err := s.hikConnectSvc.GetPlaybackToken(r.Context(), cam.CloudSerial, channel)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}

	// Audit: log token issuance for cross-tenant incident investigation
	if s.auditSvc != nil {
		s.auditSvc.Append(tenantID, user.Email, user.Role, "cloud_token_issued", cameraID, cam.CloudSerial)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  token.AccessToken,
		"device_serial": token.DeviceSerial,
		"channel":       token.Channel,
		"expires_at":    token.ExpiresAt.Format(time.RFC3339),
	})
}

func (s *server) appCameraCloudRecordings(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	cameraID := chi.URLParam(r, "cameraID")
	if strings.TrimSpace(cameraID) == "" {
		writeError(w, http.StatusBadRequest, "camera ID is required")
		return
	}

	tenantID := user.TenantID
	cam, err := s.cameraSvc.Get(tenantID, cameraID)
	if err != nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	if cam.CloudProvider == "" || cam.CloudSerial == "" {
		writeError(w, http.StatusBadRequest, "camera has no cloud provider configured")
		return
	}

	// Check access
	if user.Role == "resident" {
		accessibleDoorIDs := s.getUserAccessibleDoorIDs(tenantID, user.ID)
		if cam.DoorID == "" || !accessibleDoorIDs[cam.DoorID] {
			writeError(w, http.StatusForbidden, "no access to this camera")
			return
		}
	}

	if s.hikConnectSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "cloud video service not configured")
		return
	}

	// Parse date query param (default: today)
	dateStr := r.URL.Query().Get("date")
	var start, end time.Time
	if dateStr != "" {
		parsed, parseErr := time.Parse("2006-01-02", dateStr)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid date format, expected YYYY-MM-DD")
			return
		}
		start = parsed
		end = parsed.Add(24*time.Hour - time.Second)
	} else {
		now := time.Now().UTC()
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		end = start.Add(24*time.Hour - time.Second)
	}

	recordings, err := s.hikConnectSvc.ListRecordings(r.Context(), cam.CloudSerial, start, end)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}

	items := make([]map[string]any, 0, len(recordings))
	for _, rec := range recordings {
		items = append(items, map[string]any{
			"start_time": rec.StartTime.Format(time.RFC3339),
			"end_time":   rec.EndTime.Format(time.RFC3339),
			"type":       rec.Type,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"camera_id":  cameraID,
		"date":       start.Format("2006-01-02"),
		"recordings": items,
	})
}

func (s *server) adminCameraCloudBind(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	if user.Role != "tenant_admin" && user.Role != "admin" {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	cameraID := chi.URLParam(r, "cameraID")
	if strings.TrimSpace(cameraID) == "" {
		writeError(w, http.StatusBadRequest, "camera ID is required")
		return
	}

	tenantID := user.TenantID
	cam, err := s.cameraSvc.Get(tenantID, cameraID)
	if err != nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	if s.hikConnectSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "cloud video service not configured")
		return
	}

	var body struct {
		DeviceSerial string `json:"device_serial"`
		ValidateCode string `json:"validate_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(body.DeviceSerial) == "" || strings.TrimSpace(body.ValidateCode) == "" {
		writeError(w, http.StatusBadRequest, "device_serial and validate_code are required")
		return
	}

	if err := s.hikConnectSvc.BindDevice(r.Context(), body.DeviceSerial, body.ValidateCode); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Update camera record with cloud binding info
	cloudProvider := "hikconnect"
	cloudSerial := body.DeviceSerial
	cloudVerified := true
	cloudChannels := 1
	_, updateErr := s.cameraSvc.Update(tenantID, cameraID, camera.CameraUpdateRequest{
		CloudProvider: &cloudProvider,
		CloudSerial:   &cloudSerial,
		CloudVerified: &cloudVerified,
		CloudChannels: &cloudChannels,
	})
	if updateErr != nil {
		writeInternalError(w, r, updateErr)
		return
	}

	_ = cam
	writeJSON(w, http.StatusOK, map[string]any{
		"camera_id":      cameraID,
		"device_serial":  body.DeviceSerial,
		"cloud_provider": "hikconnect",
		"verified":       true,
	})
}

func (s *server) adminCameraCloudUnbind(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	if user.Role != "tenant_admin" && user.Role != "admin" {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	cameraID := chi.URLParam(r, "cameraID")
	if strings.TrimSpace(cameraID) == "" {
		writeError(w, http.StatusBadRequest, "camera ID is required")
		return
	}

	tenantID := user.TenantID
	cam, err := s.cameraSvc.Get(tenantID, cameraID)
	if err != nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	if cam.CloudSerial == "" {
		writeError(w, http.StatusBadRequest, "camera has no cloud binding")
		return
	}

	if s.hikConnectSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "cloud video service not configured")
		return
	}

	if err := s.hikConnectSvc.UnbindDevice(r.Context(), cam.CloudSerial); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Clear cloud fields
	emptyStr := ""
	falseBool := false
	zeroInt := 0
	_, updateErr := s.cameraSvc.Update(tenantID, cameraID, camera.CameraUpdateRequest{
		CloudProvider: &emptyStr,
		CloudSerial:   &emptyStr,
		CloudVerified: &falseBool,
		CloudChannels: &zeroInt,
	})
	if updateErr != nil {
		writeInternalError(w, r, updateErr)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"camera_id": cameraID,
		"unbound":   true,
	})
}

// getUserAccessibleDoorIDs returns set of door IDs the user can access.
func (s *server) getUserAccessibleDoorIDs(tenantID, userID string) map[string]bool {
	result := make(map[string]bool)

	accessUser, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil {
		return result
	}

	allDoors := s.spaceSvc.ListDoors(tenantID)
	doorGroups := s.spaceSvc.ListDoorGroups(tenantID)
	userGroups := s.accessSvc.ListUserGroups(tenantID)

	// Door group membership
	for _, dg := range doorGroups {
		for _, doorID := range dg.DoorIDs {
			door := findDoorByID(allDoors, doorID)
			if door == nil {
				continue
			}
			for _, ug := range userGroups {
				if ug.BuildingID != door.BuildingID && ug.PlaceID != door.BuildingID {
					continue
				}
				for _, ugID := range accessUser.GroupIDs {
					if ugID == ug.ID {
						result[doorID] = true
					}
				}
			}
		}
	}

	// Building-level user group membership
	for _, ug := range userGroups {
		for _, ugID := range accessUser.GroupIDs {
			if ugID != ug.ID {
				continue
			}
			for _, door := range allDoors {
				if door.BuildingID == ug.BuildingID || door.BuildingID == ug.PlaceID {
					result[door.ID] = true
				}
			}
		}
	}

	// Role assignments
	assignments := s.accessSvc.ListRoleAssignments(tenantID)
	for _, ra := range assignments {
		if ra.AssigneeType != "User" || ra.AssigneeID != userID {
			continue
		}
		for _, door := range allDoors {
			if ra.AppliesToType == "Organization" || (ra.AppliesToType == "Place" && ra.AppliesToID == door.BuildingID) {
				result[door.ID] = true
			}
		}
	}

	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Enhanced existing endpoints — field extensions
// ─────────────────────────────────────────────────────────────────────────────

// appAccessMyDoorsEnhanced replaces appAccessMyDoors with extra fields:
// last_unlock_at, is_favorite
func (s *server) appAccessMyDoorsEnhanced(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	tenantID := user.TenantID
	accessUser, err := s.accessSvc.GetUser(tenantID, user.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}

	allDoors := s.spaceSvc.ListDoors(tenantID)
	doorGroups := s.spaceSvc.ListDoorGroups(tenantID)
	userGroups := s.accessSvc.ListUserGroups(tenantID)

	// Build set of accessible door IDs
	accessibleDoorIDs := make(map[string]string) // doorID → group name
	for _, dg := range doorGroups {
		for _, doorID := range dg.DoorIDs {
			door := findDoorByID(allDoors, doorID)
			if door == nil {
				continue
			}
			for _, ug := range userGroups {
				if ug.BuildingID != door.BuildingID && ug.PlaceID != door.BuildingID {
					continue
				}
				for _, ugID := range accessUser.GroupIDs {
					if ugID == ug.ID {
						accessibleDoorIDs[doorID] = ug.Name
					}
				}
			}
		}
	}

	// Building-level user group membership
	for _, ug := range userGroups {
		for _, ugID := range accessUser.GroupIDs {
			if ugID != ug.ID {
				continue
			}
			for _, door := range allDoors {
				if door.BuildingID != ug.BuildingID && door.BuildingID != ug.PlaceID {
					continue
				}
				if _, exists := accessibleDoorIDs[door.ID]; !exists {
					accessibleDoorIDs[door.ID] = ug.Name
				}
			}
		}
	}

	// Role assignments
	assignments := s.accessSvc.ListRoleAssignments(tenantID)
	for _, ra := range assignments {
		if ra.AssigneeType != "User" || ra.AssigneeID != user.ID {
			continue
		}
		for _, door := range allDoors {
			if ra.AppliesToType == "Organization" || (ra.AppliesToType == "Place" && ra.AppliesToID == door.BuildingID) {
				if _, exists := accessibleDoorIDs[door.ID]; !exists {
					accessibleDoorIDs[door.ID] = "role_assignment"
				}
			}
		}
	}

	// Build last_unlock_at map from events
	events := s.eventSvc.ListAccessEvents(tenantID)
	lastUnlock := make(map[string]time.Time) // doorID → latest unlock time
	for _, ev := range events {
		if ev.Type == "access_granted" {
			if prev, exists := lastUnlock[ev.DoorID]; !exists || ev.At.After(prev) {
				lastUnlock[ev.DoorID] = ev.At
			}
		}
	}

	items := make([]map[string]any, 0, len(accessibleDoorIDs))
	for _, door := range allDoors {
		groupName, accessible := accessibleDoorIDs[door.ID]
		if !accessible {
			continue
		}
		gw, hasGateway := s.gatewaySvc.FindGatewayByDoorID(tenantID, door.ID)
		gwStatus := "disconnected"
		if hasGateway {
			gwStatus = gw.Status
		}

		item := map[string]any{
			"id":             door.ID,
			"name":           door.Name,
			"building_id":    door.BuildingID,
			"area_id":        door.AreaID,
			"status":         door.Status,
			"gateway_status": gwStatus,
			"group_name":     groupName,
			"can_unlock":     hasGateway && gw.Status == "online",
			"is_favorite":    s.isDoorFavorite(user.ID, door.ID),
		}

		if t, ok := lastUnlock[door.ID]; ok {
			item["last_unlock_at"] = t.Format(time.RFC3339)
		} else {
			item["last_unlock_at"] = nil
		}

		items = append(items, item)
	}

	// Sort favorites first
	sortedItems := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if fav, _ := item["is_favorite"].(bool); fav {
			sortedItems = append(sortedItems, item)
		}
	}
	for _, item := range items {
		if fav, _ := item["is_favorite"].(bool); !fav {
			sortedItems = append(sortedItems, item)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": sortedItems,
		"pagination": map[string]any{
			"offset":   0,
			"limit":    len(sortedItems),
			"total":    len(sortedItems),
			"has_more": false,
		},
	})
}

// appAccessLogsEnhanced extends appAccessLogs with snapshot_urls field.
func (s *server) appAccessLogsEnhanced(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	events := s.eventSvc.ListAccessEvents(user.TenantID)

	// Build door name lookup for enrichment
	doors := s.spaceSvc.ListDoors(user.TenantID)
	doorNames := make(map[string]string, len(doors))
	for _, d := range doors {
		doorNames[d.ID] = d.Name
	}

	// Parse pagination params
	offset, limit := parsePagination(r, len(events))
	total := len(events)

	if offset > len(events) {
		offset = len(events)
	}
	end := offset + limit
	if end > len(events) {
		end = len(events)
	}
	page := events[offset:end]

	// Enrich with door_name and snapshot_urls
	items := make([]map[string]any, 0, len(page))
	for _, ev := range page {
		// Find snapshots linked to this event
		snapshots := s.cameraSvc.ListSnapshotsByEventID(user.TenantID, ev.ID)
		snapshotURLs := make([]string, 0, len(snapshots))
		for _, snap := range snapshots {
			snapshotURLs = append(snapshotURLs, snap.SignedURL)
		}

		items = append(items, map[string]any{
			"id":            ev.ID,
			"door_id":       ev.DoorID,
			"door_name":     doorNames[ev.DoorID],
			"type":          ev.Type,
			"result":        ev.Result,
			"actor":         ev.Actor,
			"at":            ev.At,
			"snapshot_urls": snapshotURLs,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"pagination": map[string]any{
			"offset":   offset,
			"limit":    limit,
			"total":    total,
			"has_more": end < total,
		},
	})
}

// appCredentialsEnhanced extends credentials with device_name and activated_at.
func (s *server) appCredentialsEnhanced(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	passes := s.walletSvc.ListPasses(user.TenantID)
	items := make([]map[string]any, 0, len(passes))
	for i := range passes {
		if passes[i].TargetType != "user" {
			continue
		}
		if passes[i].TargetID != user.ID {
			continue
		}

		items = append(items, map[string]any{
			"id":              passes[i].ID,
			"type":            firstNonEmptyString(passes[i].CredentialKind, "wallet"),
			"provider":        passes[i].Provider,
			"credential_kind": passes[i].CredentialKind,
			"status":          passes[i].Status,
			"save_link":       passes[i].SaveLink,
			"card_number":     passes[i].CardNumber,
			"user_id":         user.ID,
			"issued_at":       passes[i].IssuedAt,
			"created_at":      passes[i].IssuedAt,
			"device_name":     passes[i].DeviceName,
			"activated_at":    passes[i].ActivatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

// appListVisitorPassesEnhanced extends visitor passes with valid_from/valid_until/display_label.
func (s *server) appListVisitorPassesEnhanced(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	all := s.accessSvc.ListVisitorPasses(user.TenantID)
	total := len(all)
	offset, limit := parsePagination(r, total)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := all[offset:end]

	items := make([]map[string]any, 0, len(page))
	for _, pass := range page {
		displayLabel := pass.Visitor
		if displayLabel == "" {
			displayLabel = "Visitor"
		}

		items = append(items, map[string]any{
			"id":              pass.ID,
			"tenant_id":      pass.TenantID,
			"building_id":    pass.BuildingID,
			"host":           pass.Host,
			"visitor":        pass.Visitor,
			"delivery_method": pass.DeliveryMethod,
			"expires_at":     pass.ExpiresAt,
			"created_at":     pass.CreatedAt,
			"valid_from":     pass.CreatedAt.Format(time.RFC3339),
			"valid_until":    pass.ExpiresAt,
			"display_label":  displayLabel,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"pagination": map[string]any{
			"offset":   offset,
			"limit":    limit,
			"total":    total,
			"has_more": end < total,
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// appCreateVisitorPassEnhanced accepts valid_from, valid_until or ttl_hours.
// POST /api/v1/app/visitor-passes
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appCreateVisitorPassEnhanced(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var req struct {
		BuildingID     string  `json:"building_id"`
		Visitor        string  `json:"visitor"`
		DeliveryMethod string  `json:"delivery_method"`
		ValidFrom      string  `json:"valid_from"`
		ValidUntil     string  `json:"valid_until"`
		TTLHours       float64 `json:"ttl_hours"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	visitor := strings.TrimSpace(req.Visitor)
	if visitor == "" {
		writeError(w, http.StatusBadRequest, "visitor is required")
		return
	}

	now := time.Now().UTC()
	validFrom := now
	var validUntil time.Time

	// Parse valid_from if provided
	if req.ValidFrom != "" {
		parsed, err := time.Parse(time.RFC3339, req.ValidFrom)
		if err == nil {
			validFrom = parsed
		}
	}

	// Determine valid_until
	if req.ValidUntil != "" {
		parsed, err := time.Parse(time.RFC3339, req.ValidUntil)
		if err == nil {
			validUntil = parsed
		}
	} else if req.TTLHours > 0 {
		validUntil = validFrom.Add(time.Duration(req.TTLHours * float64(time.Hour)))
	} else {
		// Default: 24 hours
		validUntil = validFrom.Add(24 * time.Hour)
	}

	tenantID := user.TenantID
	pass, err := s.accessSvc.CreateVisitorPass(tenantID, req.BuildingID, user.Email, visitor, req.DeliveryMethod, validUntil.Format(time.RFC3339))
	if err != nil {
		writeInternalError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":              pass.ID,
		"tenant_id":      pass.TenantID,
		"building_id":    pass.BuildingID,
		"host":           pass.Host,
		"visitor":        pass.Visitor,
		"delivery_method": pass.DeliveryMethod,
		"expires_at":     pass.ExpiresAt,
		"created_at":     pass.CreatedAt,
		"valid_from":     validFrom.Format(time.RFC3339),
		"valid_until":    validUntil.Format(time.RFC3339),
		"display_label":  visitor,
	})
}

