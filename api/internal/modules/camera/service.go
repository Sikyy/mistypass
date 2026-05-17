package camera

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrCameraNotFound    = errors.New("camera not found")
	ErrCameraNameRequired = errors.New("camera name is required")
	ErrCameraHostRequired = errors.New("camera host is required")
	ErrInvalidProvider    = errors.New("invalid camera provider")
	ErrProviderNotFound   = errors.New("camera provider not configured")
)

// CredentialVault encrypts/decrypts camera credentials.
type CredentialVault interface {
	EncryptVersioned(plaintext string) string
	DecryptVersioned(value string) string
}

// Service manages cameras and their providers.
type Service struct {
	mu        sync.RWMutex
	cameras   map[string]Camera
	snapshots map[string][]Snapshot
	providers map[string]CameraProvider
	vault     CredentialVault

	snapshotRetentionDays  int
	maxSnapshotsPerEvent   int
	snapshotTimeoutSeconds int
}

func NewService(vault CredentialVault) *Service {
	s := &Service{
		cameras:                make(map[string]Camera),
		snapshots:              make(map[string][]Snapshot),
		providers:              make(map[string]CameraProvider),
		vault:                  vault,
		snapshotRetentionDays:  30,
		maxSnapshotsPerEvent:   3,
		snapshotTimeoutSeconds: 10,
	}
	// Register all built-in providers
	s.providers[ProviderONVIF] = NewONVIFProvider()
	s.providers[ProviderHikvision] = NewHikvisionProvider()
	s.providers[ProviderDahua] = NewDahuaProvider()
	s.providers[ProviderZKTeco] = NewZKTecoProvider()
	s.providers[ProviderVIVOTEK] = NewVIVOTEKProvider()
	return s
}

func (s *Service) SetSnapshotRetentionDays(days int) {
	if days > 0 {
		s.snapshotRetentionDays = days
	}
}

func (s *Service) SetMaxSnapshotsPerEvent(max int) {
	if max > 0 {
		s.maxSnapshotsPerEvent = max
	}
}

func (s *Service) SetSnapshotTimeoutSeconds(sec int) {
	if sec > 0 {
		s.snapshotTimeoutSeconds = sec
	}
}

// --- CRUD ---

func (s *Service) List(tenantID string) []Camera {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tid := strings.TrimSpace(tenantID)
	items := make([]Camera, 0)
	for _, cam := range s.cameras {
		if tid != "" && cam.TenantID != tid {
			continue
		}
		items = append(items, cam)
	}
	return items
}

func (s *Service) Get(tenantID, cameraID string) (Camera, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cam, ok := s.cameras[strings.TrimSpace(cameraID)]
	if !ok || (tenantID != "" && cam.TenantID != strings.TrimSpace(tenantID)) {
		return Camera{}, ErrCameraNotFound
	}
	return cam, nil
}

func (s *Service) Create(req CameraCreateRequest) (Camera, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return Camera{}, ErrCameraNameRequired
	}
	host := strings.TrimSpace(req.Host)
	if host == "" {
		return Camera{}, ErrCameraHostRequired
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if _, ok := s.providers[provider]; !ok {
		return Camera{}, fmt.Errorf("%w: %s (supported: onvif, hikvision, dahua, zkteco, vivotek)", ErrInvalidProvider, provider)
	}

	id := generateCameraID()
	now := time.Now().UTC()
	port := req.Port
	if port <= 0 {
		port = 80
	}
	channel := req.Channel
	if channel <= 0 {
		channel = 1
	}

	// Encrypt credentials
	var encCreds string
	if req.Username != "" || req.Password != "" {
		creds, _ := json.Marshal(map[string]string{
			"username": req.Username,
			"password": req.Password,
		})
		if s.vault != nil {
			encCreds = s.vault.EncryptVersioned(string(creds))
		} else {
			encCreds = string(creds)
		}
	}

	cam := Camera{
		ID:                   id,
		TenantID:             strings.TrimSpace(req.TenantID),
		PlaceID:              strings.TrimSpace(req.PlaceID),
		DoorID:               strings.TrimSpace(req.DoorID),
		Name:                 name,
		Provider:             provider,
		Host:                 host,
		Port:                 port,
		Channel:              channel,
		UseTLS:               req.UseTLS,
		Status:               "active",
		EncryptedCredentials: encCreds,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	s.mu.Lock()
	s.cameras[id] = cam
	s.mu.Unlock()
	return cam, nil
}

func (s *Service) Update(tenantID, cameraID string, req CameraUpdateRequest) (Camera, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cam, ok := s.cameras[strings.TrimSpace(cameraID)]
	if !ok || (tenantID != "" && cam.TenantID != strings.TrimSpace(tenantID)) {
		return Camera{}, ErrCameraNotFound
	}

	if req.Name != nil {
		cam.Name = strings.TrimSpace(*req.Name)
	}
	if req.DoorID != nil {
		cam.DoorID = strings.TrimSpace(*req.DoorID)
	}
	if req.Host != nil {
		cam.Host = strings.TrimSpace(*req.Host)
	}
	if req.Port != nil {
		cam.Port = *req.Port
	}
	if req.Channel != nil {
		cam.Channel = *req.Channel
	}
	if req.UseTLS != nil {
		cam.UseTLS = *req.UseTLS
	}
	if req.Status != nil {
		cam.Status = strings.TrimSpace(*req.Status)
	}
	if req.CloudProvider != nil {
		cam.CloudProvider = strings.TrimSpace(*req.CloudProvider)
	}
	if req.CloudSerial != nil {
		cam.CloudSerial = strings.TrimSpace(*req.CloudSerial)
	}
	if req.CloudVerified != nil {
		cam.CloudVerified = *req.CloudVerified
	}
	if req.CloudChannels != nil {
		cam.CloudChannels = *req.CloudChannels
	}
	if req.Username != nil || req.Password != nil {
		oldCreds := s.decryptCredentials(cam.EncryptedCredentials)
		username := oldCreds["username"]
		password := oldCreds["password"]
		if req.Username != nil {
			username = *req.Username
		}
		if req.Password != nil {
			password = *req.Password
		}
		creds, _ := json.Marshal(map[string]string{
			"username": username,
			"password": password,
		})
		if s.vault != nil {
			cam.EncryptedCredentials = s.vault.EncryptVersioned(string(creds))
		} else {
			cam.EncryptedCredentials = string(creds)
		}
	}
	cam.UpdatedAt = time.Now().UTC()
	s.cameras[cameraID] = cam
	return cam, nil
}

func (s *Service) Delete(tenantID, cameraID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam, ok := s.cameras[strings.TrimSpace(cameraID)]
	if !ok || (tenantID != "" && cam.TenantID != strings.TrimSpace(tenantID)) {
		return ErrCameraNotFound
	}
	delete(s.cameras, cameraID)
	delete(s.snapshots, cameraID)
	return nil
}

// --- Provider Operations ---

func (s *Service) TestConnection(ctx context.Context, tenantID, cameraID string) error {
	cam, err := s.Get(tenantID, cameraID)
	if err != nil {
		return err
	}
	provider, ok := s.providers[cam.Provider]
	if !ok {
		return ErrProviderNotFound
	}
	return provider.TestConnection(ctx, s.cameraConfigFromRecord(cam))
}

func (s *Service) CaptureSnapshot(ctx context.Context, tenantID, cameraID, eventID, eventType string) (*Snapshot, error) {
	cam, err := s.Get(tenantID, cameraID)
	if err != nil {
		return nil, err
	}
	provider, ok := s.providers[cam.Provider]
	if !ok {
		return nil, ErrProviderNotFound
	}

	data, contentType, err := provider.Snapshot(ctx, s.cameraConfigFromRecord(cam))
	if err != nil {
		return nil, fmt.Errorf("snapshot failed: %w", err)
	}

	now := time.Now().UTC()
	snap := Snapshot{
		ID:             generateSnapshotID(),
		CameraID:       cameraID,
		EventID:        eventID,
		EventType:      eventType,
		ContentType:    contentType,
		SizeBytes:      len(data),
		CapturedAt:     now,
		RetentionUntil: now.AddDate(0, 0, s.snapshotRetentionDays),
	}

	s.mu.Lock()
	s.snapshots[cameraID] = append(s.snapshots[cameraID], snap)
	updated := s.cameras[cameraID]
	updated.LastSnapshotAt = now.Format(time.RFC3339)
	s.cameras[cameraID] = updated
	s.mu.Unlock()

	return &snap, nil
}

func (s *Service) GetVideoLink(ctx context.Context, tenantID, cameraID string) (string, error) {
	cam, err := s.Get(tenantID, cameraID)
	if err != nil {
		return "", err
	}
	provider, ok := s.providers[cam.Provider]
	if !ok {
		return "", ErrProviderNotFound
	}
	return provider.VideoLink(ctx, s.cameraConfigFromRecord(cam))
}

func (s *Service) DiscoverCameras(ctx context.Context, providerName, subnet string) ([]DiscoveredCamera, error) {
	provider, ok := s.providers[strings.ToLower(strings.TrimSpace(providerName))]
	if !ok {
		return nil, ErrProviderNotFound
	}
	return provider.Discover(ctx, subnet)
}

func (s *Service) ListSnapshots(tenantID, cameraID string) []Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if cameraID != "" {
		return s.snapshots[cameraID]
	}
	var all []Snapshot
	for _, cam := range s.cameras {
		if tenantID != "" && cam.TenantID != tenantID {
			continue
		}
		all = append(all, s.snapshots[cam.ID]...)
	}
	return all
}

// ListSnapshotsByEventID returns all snapshots linked to a specific event across all cameras.
func (s *Service) ListSnapshotsByEventID(tenantID, eventID string) []Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var matched []Snapshot
	for _, cam := range s.cameras {
		if tenantID != "" && cam.TenantID != tenantID {
			continue
		}
		for _, snap := range s.snapshots[cam.ID] {
			if snap.EventID == eventID {
				matched = append(matched, snap)
			}
		}
	}
	return matched
}

// TriggerEventSnapshot captures snapshots from all cameras mapped to the given door.
func (s *Service) TriggerEventSnapshot(ctx context.Context, tenantID, doorID, eventID, eventType string) []Snapshot {
	s.mu.RLock()
	var matched []Camera
	for _, cam := range s.cameras {
		if cam.TenantID == tenantID && cam.DoorID == doorID && cam.Status == "active" {
			matched = append(matched, cam)
		}
	}
	s.mu.RUnlock()

	var results []Snapshot
	for i, cam := range matched {
		if i >= s.maxSnapshotsPerEvent {
			break
		}
		snap, err := s.CaptureSnapshot(ctx, tenantID, cam.ID, eventID, eventType)
		if err != nil {
			continue
		}
		results = append(results, *snap)
	}
	return results
}

func (s *Service) GetProviderCapabilities(providerName string) (ProviderCapabilities, error) {
	provider, ok := s.providers[strings.ToLower(strings.TrimSpace(providerName))]
	if !ok {
		return ProviderCapabilities{}, ErrProviderNotFound
	}
	return provider.Capabilities(), nil
}

// --- Helpers ---

func (s *Service) cameraConfigFromRecord(cam Camera) CameraConfig {
	creds := s.decryptCredentials(cam.EncryptedCredentials)
	return CameraConfig{
		Host:     cam.Host,
		Port:     cam.Port,
		Username: creds["username"],
		Password: creds["password"],
		Channel:  cam.Channel,
		UseTLS:   cam.UseTLS,
	}
}

func (s *Service) decryptCredentials(encrypted string) map[string]string {
	if encrypted == "" {
		return map[string]string{}
	}
	plaintext := encrypted
	if s.vault != nil && strings.HasPrefix(encrypted, "enc:") {
		plaintext = s.vault.DecryptVersioned(encrypted)
	}
	var creds map[string]string
	if err := json.Unmarshal([]byte(plaintext), &creds); err != nil {
		return map[string]string{}
	}
	return creds
}

func generateCameraID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "cam_" + hex.EncodeToString(b)
}

func generateSnapshotID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "snap_" + hex.EncodeToString(b)
}
