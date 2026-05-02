package camera

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// HikvisionProvider implements CameraProvider for Hikvision cameras
// using the ISAPI HTTP interface.
type HikvisionProvider struct {
	client *http.Client
}

// NewHikvisionProvider creates a new Hikvision camera provider.
func NewHikvisionProvider() *HikvisionProvider {
	return &HikvisionProvider{
		client: newHTTPClient(),
	}
}

// hikvisionChannel maps the logical channel number to the ISAPI channel ID.
// Channel 1 becomes "101", channel 2 becomes "201", etc.
func hikvisionChannel(channel int) string {
	if channel <= 0 {
		channel = 1
	}
	return fmt.Sprintf("%d01", channel)
}

// Discover is not supported for Hikvision; returns empty results.
func (p *HikvisionProvider) Discover(_ context.Context, _ string) ([]DiscoveredCamera, error) {
	return nil, nil
}

// Snapshot captures a JPEG frame from the Hikvision ISAPI endpoint.
func (p *HikvisionProvider) Snapshot(ctx context.Context, cam CameraConfig) ([]byte, string, error) {
	ch := hikvisionChannel(cam.Channel)
	url := fmt.Sprintf("%s/ISAPI/Streaming/channels/%s/picture", baseURL(cam), ch)

	resp, err := digestAuthRequest(ctx, p.client, http.MethodGet, url, cam.Username, cam.Password)
	if err != nil {
		return nil, "", fmt.Errorf("hikvision snapshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("hikvision snapshot: server returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("hikvision snapshot: read body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return data, contentType, nil
}

// VideoLink returns an RTSP stream URL for the Hikvision camera.
func (p *HikvisionProvider) VideoLink(_ context.Context, cam CameraConfig) (string, error) {
	ch := hikvisionChannel(cam.Channel)
	rtspPort := 554
	return fmt.Sprintf("rtsp://%s:%s@%s:%d/Streaming/Channels/%s",
		cam.Username, cam.Password, cam.Host, rtspPort, ch), nil
}

// TestConnection checks Hikvision reachability via the ISAPI deviceInfo endpoint.
func (p *HikvisionProvider) TestConnection(ctx context.Context, cam CameraConfig) error {
	url := fmt.Sprintf("%s/ISAPI/System/deviceInfo", baseURL(cam))

	resp, err := digestAuthRequest(ctx, p.client, http.MethodGet, url, cam.Username, cam.Password)
	if err != nil {
		return fmt.Errorf("hikvision test: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hikvision test: device returned status %d", resp.StatusCode)
	}
	return nil
}

// Capabilities returns the Hikvision provider's capabilities.
func (p *HikvisionProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsDiscovery: false,
		SupportsSnapshot:  true,
		SupportsRTSP:      true,
		SupportsHLS:       false,
		SupportsPTZ:       false,
		SupportsEvents:    true,
	}
}
