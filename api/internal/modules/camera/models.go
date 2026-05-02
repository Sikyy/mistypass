package camera

import "time"

// Camera represents a registered camera in the system.
type Camera struct {
	ID                   string    `json:"id"`
	TenantID             string    `json:"tenant_id"`
	PlaceID              string    `json:"place_id"`
	DoorID               string    `json:"door_id,omitempty"`
	Name                 string    `json:"name"`
	Provider             string    `json:"provider"`
	Host                 string    `json:"host"`
	Port                 int       `json:"port"`
	Channel              int       `json:"channel"`
	UseTLS               bool      `json:"use_tls"`
	Status               string    `json:"status"`
	EncryptedCredentials string    `json:"-"`
	LastSnapshotAt       string    `json:"last_snapshot_at,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// CameraCreateRequest is the payload for creating a camera.
type CameraCreateRequest struct {
	TenantID string `json:"tenant_id"`
	PlaceID  string `json:"place_id"`
	DoorID   string `json:"door_id,omitempty"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Host     string `json:"host"`
	Port     int    `json:"port,omitempty"`
	Channel  int    `json:"channel,omitempty"`
	UseTLS   bool   `json:"use_tls,omitempty"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// CameraUpdateRequest is the payload for updating a camera.
type CameraUpdateRequest struct {
	Name     *string `json:"name,omitempty"`
	DoorID   *string `json:"door_id,omitempty"`
	Host     *string `json:"host,omitempty"`
	Port     *int    `json:"port,omitempty"`
	Channel  *int    `json:"channel,omitempty"`
	UseTLS   *bool   `json:"use_tls,omitempty"`
	Username *string `json:"username,omitempty"`
	Password *string `json:"password,omitempty"`
	Status   *string `json:"status,omitempty"`
}

// Snapshot represents a captured camera frame linked to an access event.
type Snapshot struct {
	ID             string    `json:"id"`
	CameraID       string    `json:"camera_id"`
	EventID        string    `json:"event_id,omitempty"`
	EventType      string    `json:"event_type,omitempty"`
	ContentType    string    `json:"content_type"`
	SizeBytes      int       `json:"size_bytes"`
	StoragePath    string    `json:"storage_path,omitempty"`
	SignedURL      string    `json:"signed_url,omitempty"`
	CapturedAt     time.Time `json:"captured_at"`
	RetentionUntil time.Time `json:"retention_until"`
}
