package camera

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// DahuaProvider implements CameraProvider for Dahua cameras
// using the CGI HTTP interface.
type DahuaProvider struct {
	client *http.Client
}

// NewDahuaProvider creates a new Dahua camera provider.
func NewDahuaProvider() *DahuaProvider {
	return &DahuaProvider{
		client: newHTTPClient(),
	}
}

// Discover is not supported for Dahua; returns empty results.
func (p *DahuaProvider) Discover(_ context.Context, _ string) ([]DiscoveredCamera, error) {
	return nil, nil
}

// Snapshot captures a JPEG frame from the Dahua CGI snapshot endpoint.
func (p *DahuaProvider) Snapshot(ctx context.Context, cam CameraConfig) ([]byte, string, error) {
	channel := cam.Channel
	if channel <= 0 {
		channel = 1
	}

	url := fmt.Sprintf("%s/cgi-bin/snapshot.cgi?channel=%d", baseURL(cam), channel)

	resp, err := digestAuthRequest(ctx, p.client, http.MethodGet, url, cam.Username, cam.Password)
	if err != nil {
		return nil, "", fmt.Errorf("dahua snapshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("dahua snapshot: server returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("dahua snapshot: read body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return data, contentType, nil
}

// VideoLink returns an RTSP stream URL for the Dahua camera.
func (p *DahuaProvider) VideoLink(_ context.Context, cam CameraConfig) (string, error) {
	channel := cam.Channel
	if channel <= 0 {
		channel = 1
	}

	return fmt.Sprintf("rtsp://%s:%s@%s:554/cam/realmonitor?channel=%d&subtype=0",
		cam.Username, cam.Password, cam.Host, channel), nil
}

// TestConnection checks Dahua reachability via the magicBox CGI endpoint.
func (p *DahuaProvider) TestConnection(ctx context.Context, cam CameraConfig) error {
	url := fmt.Sprintf("%s/cgi-bin/magicBox.cgi?action=getDeviceType", baseURL(cam))

	resp, err := digestAuthRequest(ctx, p.client, http.MethodGet, url, cam.Username, cam.Password)
	if err != nil {
		return fmt.Errorf("dahua test: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dahua test: device returned status %d", resp.StatusCode)
	}
	return nil
}

// Capabilities returns the Dahua provider's capabilities.
func (p *DahuaProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsDiscovery: false,
		SupportsSnapshot:  true,
		SupportsRTSP:      true,
		SupportsHLS:       false,
		SupportsPTZ:       false,
		SupportsEvents:    true,
	}
}
