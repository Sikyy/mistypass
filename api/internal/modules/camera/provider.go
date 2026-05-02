package camera

import "context"

// CameraProvider defines the interface that all camera vendor integrations must implement.
type CameraProvider interface {
	// Discover scans the network for cameras (ONVIF WS-Discovery or vendor-specific).
	Discover(ctx context.Context, subnet string) ([]DiscoveredCamera, error)

	// Snapshot captures a JPEG frame from the camera.
	// Returns image bytes, content type (e.g. "image/jpeg"), and error.
	Snapshot(ctx context.Context, cam CameraConfig) ([]byte, string, error)

	// VideoLink returns a streamable URL (RTSP or HLS) for live view.
	VideoLink(ctx context.Context, cam CameraConfig) (string, error)

	// TestConnection verifies the camera credentials and reachability.
	TestConnection(ctx context.Context, cam CameraConfig) error

	// Capabilities returns what this provider supports.
	Capabilities() ProviderCapabilities
}

// ProviderCapabilities describes what a provider can do.
type ProviderCapabilities struct {
	SupportsDiscovery bool `json:"supports_discovery"`
	SupportsSnapshot  bool `json:"supports_snapshot"`
	SupportsRTSP      bool `json:"supports_rtsp"`
	SupportsHLS       bool `json:"supports_hls"`
	SupportsPTZ       bool `json:"supports_ptz"`
	SupportsEvents    bool `json:"supports_events"`
}

// DiscoveredCamera represents a camera found during network discovery.
type DiscoveredCamera struct {
	Name         string `json:"name"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Address      string `json:"address"`
	MACAddress   string `json:"mac_address,omitempty"`
	Provider     string `json:"provider"`
}

// CameraConfig holds the connection parameters for a registered camera.
type CameraConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Channel  int    `json:"channel"`
	UseTLS   bool   `json:"use_tls"`
	// Provider-specific extra fields
	Extra map[string]string `json:"extra,omitempty"`
}

// ProviderName constants.
const (
	ProviderONVIF     = "onvif"
	ProviderHikvision = "hikvision"
	ProviderDahua     = "dahua"
	ProviderZKTeco    = "zkteco"
	ProviderVIVOTEK   = "vivotek"
)
