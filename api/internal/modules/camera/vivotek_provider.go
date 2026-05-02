package camera

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// VIVOTEKProvider implements CameraProvider for VIVOTEK cameras
// using the CGI HTTP interface.
type VIVOTEKProvider struct {
	client *http.Client
}

// NewVIVOTEKProvider creates a new VIVOTEK camera provider.
func NewVIVOTEKProvider() *VIVOTEKProvider {
	return &VIVOTEKProvider{
		client: newHTTPClient(),
	}
}

// Discover is not supported for VIVOTEK; returns empty results.
func (p *VIVOTEKProvider) Discover(_ context.Context, _ string) ([]DiscoveredCamera, error) {
	return nil, nil
}

// Snapshot captures a JPEG frame from the VIVOTEK CGI video endpoint.
func (p *VIVOTEKProvider) Snapshot(ctx context.Context, cam CameraConfig) ([]byte, string, error) {
	channel := cam.Channel
	if channel <= 0 {
		channel = 1
	}

	url := fmt.Sprintf("%s/cgi-bin/viewer/video.jpg?quality=5&streamid=%d", baseURL(cam), channel)

	resp, err := digestAuthRequest(ctx, p.client, http.MethodGet, url, cam.Username, cam.Password)
	if err != nil {
		return nil, "", fmt.Errorf("vivotek snapshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("vivotek snapshot: server returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("vivotek snapshot: read body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return data, contentType, nil
}

// VideoLink returns an RTSP stream URL for the VIVOTEK camera.
func (p *VIVOTEKProvider) VideoLink(_ context.Context, cam CameraConfig) (string, error) {
	return fmt.Sprintf("rtsp://%s:%s@%s:554/live.sdp",
		cam.Username, cam.Password, cam.Host), nil
}

// TestConnection checks VIVOTEK reachability via the admin getparam CGI endpoint.
func (p *VIVOTEKProvider) TestConnection(ctx context.Context, cam CameraConfig) error {
	url := fmt.Sprintf("%s/cgi-bin/admin/getparam.cgi?system_hostname", baseURL(cam))

	resp, err := digestAuthRequest(ctx, p.client, http.MethodGet, url, cam.Username, cam.Password)
	if err != nil {
		return fmt.Errorf("vivotek test: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vivotek test: device returned status %d", resp.StatusCode)
	}
	return nil
}

// Capabilities returns the VIVOTEK provider's capabilities.
func (p *VIVOTEKProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsDiscovery: false,
		SupportsSnapshot:  true,
		SupportsRTSP:      true,
		SupportsHLS:       false,
		SupportsPTZ:       false,
		SupportsEvents:    true,
	}
}
