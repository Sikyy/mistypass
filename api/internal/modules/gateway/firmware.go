package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"
)

var ErrGatewayFirmwareNotFound = errors.New("gateway firmware not found")
var ErrGatewayFirmwareVersionRequired = errors.New("gateway firmware version is required")
var ErrGatewayFirmwareSHA256Invalid = errors.New("gateway firmware sha256 is invalid")
var ErrGatewayFirmwareSignatureInvalid = errors.New("gateway firmware signature is invalid (expected hex-encoded Ed25519 signature)")

// GatewayFirmware is one stored, signed firmware artifact in the registry.
type GatewayFirmware struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	Version    string    `json:"version"`
	Channel    string    `json:"channel,omitempty"`
	SHA256     string    `json:"sha256"`
	Signature  string    `json:"signature"`
	SizeBytes  int64     `json:"size_bytes"`
	UploadedBy string    `json:"uploaded_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// CreateFirmwareInput carries validated metadata for a firmware record. The
// HTTP layer verifies sha256 against the uploaded bytes and stores the blob.
type CreateFirmwareInput struct {
	TenantID   string
	Version    string
	Channel    string
	SHA256     string
	Signature  string
	SizeBytes  int64
	UploadedBy string
}

func firmwareRecordID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "fw_" + hex.EncodeToString(b), nil
}

// CreateFirmware validates metadata, mints an id, prepends the record, persists.
func (s *Service) CreateFirmware(in CreateFirmwareInput) (GatewayFirmware, error) {
	version := strings.TrimSpace(in.Version)
	if version == "" {
		return GatewayFirmware{}, ErrGatewayFirmwareVersionRequired
	}
	sha := strings.ToLower(strings.TrimSpace(in.SHA256))
	if !isValidSHA256Hex(sha) {
		return GatewayFirmware{}, ErrGatewayFirmwareSHA256Invalid
	}
	sig := strings.ToLower(strings.TrimSpace(in.Signature))
	if !isValidEd25519SignatureHex(sig) {
		return GatewayFirmware{}, ErrGatewayFirmwareSignatureInvalid
	}
	id, err := firmwareRecordID()
	if err != nil {
		return GatewayFirmware{}, err
	}
	fw := GatewayFirmware{
		ID:         id,
		TenantID:   strings.TrimSpace(in.TenantID),
		Version:    version,
		Channel:    strings.TrimSpace(in.Channel),
		SHA256:     sha,
		Signature:  sig,
		SizeBytes:  in.SizeBytes,
		UploadedBy: strings.TrimSpace(in.UploadedBy),
		CreatedAt:  time.Now().UTC(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.firmwares = append([]GatewayFirmware{fw}, s.firmwares...)
	if err := s.persistLocked(); err != nil {
		return GatewayFirmware{}, err
	}
	return fw, nil
}

// ListFirmware returns a tenant's firmware (optionally filtered by channel), newest first.
func (s *Service) ListFirmware(tenantID, channel string) []GatewayFirmware {
	ft := strings.TrimSpace(tenantID)
	fc := strings.TrimSpace(channel)
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]GatewayFirmware, 0, len(s.firmwares))
	for i := range s.firmwares {
		if ft != "" && s.firmwares[i].TenantID != ft {
			continue
		}
		if fc != "" && s.firmwares[i].Channel != fc {
			continue
		}
		items = append(items, s.firmwares[i])
	}
	sort.Slice(items, func(a, b int) bool { return items[a].CreatedAt.After(items[b].CreatedAt) })
	return items
}

// GetFirmware returns a tenant-scoped firmware record (for task creation / listing).
func (s *Service) GetFirmware(tenantID, id string) (GatewayFirmware, error) {
	ft := strings.TrimSpace(tenantID)
	nid := strings.TrimSpace(id)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if fw, ok := s.findFirmwareLocked(nid, ft); ok {
		return fw, nil
	}
	return GatewayFirmware{}, ErrGatewayFirmwareNotFound
}

// GetFirmwareByID returns a firmware record without a tenant filter — only the
// HMAC-signed download path uses this (isolation: task creation checks tenant,
// the signed URL is server-minted, the id is unguessable).
func (s *Service) GetFirmwareByID(id string) (GatewayFirmware, error) {
	nid := strings.TrimSpace(id)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if fw, ok := s.findFirmwareLocked(nid, ""); ok {
		return fw, nil
	}
	return GatewayFirmware{}, ErrGatewayFirmwareNotFound
}

// findFirmwareLocked returns the firmware by id (tenant filter if non-empty).
// Caller must hold s.mu.
func (s *Service) findFirmwareLocked(id, tenantID string) (GatewayFirmware, bool) {
	for i := range s.firmwares {
		if s.firmwares[i].ID == id && (tenantID == "" || s.firmwares[i].TenantID == tenantID) {
			return s.firmwares[i], true
		}
	}
	return GatewayFirmware{}, false
}

func cloneGatewayFirmwares(in []GatewayFirmware) []GatewayFirmware {
	if in == nil {
		return nil
	}
	out := make([]GatewayFirmware, len(in))
	copy(out, in)
	return out
}
