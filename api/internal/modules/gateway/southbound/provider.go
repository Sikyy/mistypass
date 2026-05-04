// Package southbound provides integrations with third-party access control hardware
// (Hikvision, ZKTeco, TTLock, etc.) for door unlock commands, user sync, and event ingestion.
//
// This is distinct from the camera package which handles video/snapshot.
// Southbound providers control physical access (doors, locks, turnstiles).
package southbound

import "context"

// DoorProvider defines the interface for third-party door controller integrations.
type DoorProvider interface {
	// Unlock sends an immediate unlock command to a specific door.
	Unlock(ctx context.Context, cfg DeviceConfig, doorIndex int) error

	// Lock sends an immediate lock command (if supported).
	Lock(ctx context.Context, cfg DeviceConfig, doorIndex int) error

	// SyncUsers pushes the user/credential list to the device for offline verification.
	SyncUsers(ctx context.Context, cfg DeviceConfig, users []DeviceUser) error

	// DeleteUser removes a user from the device (employee termination).
	DeleteUser(ctx context.Context, cfg DeviceConfig, userID string) error

	// GetDoorStatus returns current door state (open/closed/alarm).
	GetDoorStatus(ctx context.Context, cfg DeviceConfig, doorIndex int) (*DoorStatus, error)

	// SubscribeEvents opens a long-lived connection for real-time access events.
	// The handler is called for each event. Blocks until ctx is cancelled.
	SubscribeEvents(ctx context.Context, cfg DeviceConfig, handler func(DoorEvent)) error

	// TestConnection verifies the device credentials and reachability.
	TestConnection(ctx context.Context, cfg DeviceConfig) error

	// ProviderName returns the provider identifier (e.g. "hikvision", "zkteco").
	ProviderName() string
}

// DeviceConfig holds connection parameters for a door controller.
type DeviceConfig struct {
	Host     string            `json:"host"`
	Port     int               `json:"port"`
	Username string            `json:"username"`
	Password string            `json:"password"`
	UseTLS   bool              `json:"use_tls"`
	Extra    map[string]string `json:"extra,omitempty"` // provider-specific fields
}

// DeviceUser represents a user to be synced to the device.
type DeviceUser struct {
	UserID     string `json:"user_id"`
	Name       string `json:"name"`
	CardNumber string `json:"card_number,omitempty"` // physical card UID
	StartTime  string `json:"start_time,omitempty"`  // access valid from (ISO 8601)
	EndTime    string `json:"end_time,omitempty"`    // access valid until (ISO 8601)
	DoorIDs    []int  `json:"door_ids,omitempty"`    // which doors this user can access (device-local index)
}

// DoorStatus represents the current state of a door.
type DoorStatus struct {
	DoorIndex int    `json:"door_index"`
	State     string `json:"state"` // "open", "closed", "alarm", "unknown"
	Locked    bool   `json:"locked"`
}

// DoorEvent represents an access event reported by the device.
type DoorEvent struct {
	EventType  string `json:"event_type"`  // "access_granted", "access_denied", "door_opened", "alarm"
	UserID     string `json:"user_id,omitempty"`
	CardNumber string `json:"card_number,omitempty"`
	DoorIndex  int    `json:"door_index"`
	Timestamp  string `json:"timestamp"` // ISO 8601
	Extra      map[string]string `json:"extra,omitempty"`
}
