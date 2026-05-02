package camera

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// ZKTecoProvider implements CameraProvider for ZKTeco cameras.
// ZKTeco devices are often OEM'd from Dahua or use ISAPI-compatible firmware,
// so this provider tries both CGI and ISAPI interfaces.
type ZKTecoProvider struct {
	client *http.Client
}

// NewZKTecoProvider creates a new ZKTeco camera provider.
func NewZKTecoProvider() *ZKTecoProvider {
	return &ZKTecoProvider{
		client: newHTTPClient(),
	}
}

// Discover is not supported for ZKTeco; returns empty results.
func (p *ZKTecoProvider) Discover(_ context.Context, _ string) ([]DiscoveredCamera, error) {
	return nil, nil
}

// Snapshot captures a JPEG frame from a ZKTeco camera.
// It tries the Dahua-compatible CGI endpoint first, then falls back to the
// ISAPI-compatible endpoint.
func (p *ZKTecoProvider) Snapshot(ctx context.Context, cam CameraConfig) ([]byte, string, error) {
	channel := cam.Channel
	if channel <= 0 {
		channel = 1
	}

	base := baseURL(cam)

	// Try Dahua-compatible CGI endpoint first.
	cgiURL := fmt.Sprintf("%s/cgi-bin/snapshot.cgi?channel=%d", base, channel)
	data, contentType, err := p.trySnapshot(ctx, cgiURL, cam.Username, cam.Password)
	if err == nil {
		return data, contentType, nil
	}

	// Fallback to ISAPI-compatible endpoint.
	ch := fmt.Sprintf("%d01", channel)
	isapiURL := fmt.Sprintf("%s/ISAPI/Streaming/channels/%s/picture", base, ch)
	data, contentType, err = p.trySnapshot(ctx, isapiURL, cam.Username, cam.Password)
	if err != nil {
		return nil, "", fmt.Errorf("zkteco snapshot: both endpoints failed: %w", err)
	}
	return data, contentType, nil
}

// trySnapshot attempts to fetch a snapshot from a single URL.
func (p *ZKTecoProvider) trySnapshot(ctx context.Context, url, username, password string) ([]byte, string, error) {
	resp, err := digestAuthRequest(ctx, p.client, http.MethodGet, url, username, password)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return data, contentType, nil
}

// VideoLink returns an RTSP stream URL for the ZKTeco camera.
// ZKTeco uses 0-indexed channels in the RTSP path (channel 1 -> ch0).
func (p *ZKTecoProvider) VideoLink(_ context.Context, cam CameraConfig) (string, error) {
	channel := cam.Channel
	if channel <= 0 {
		channel = 1
	}
	zeroIndexed := channel - 1

	return fmt.Sprintf("rtsp://%s:%s@%s:554/live/ch%d",
		cam.Username, cam.Password, cam.Host, zeroIndexed), nil
}

// TestConnection checks ZKTeco reachability by trying both Dahua CGI and ISAPI
// endpoints; succeeds if either responds with HTTP 200.
func (p *ZKTecoProvider) TestConnection(ctx context.Context, cam CameraConfig) error {
	base := baseURL(cam)

	// Try Dahua-compatible endpoint.
	dahuaURL := fmt.Sprintf("%s/cgi-bin/magicBox.cgi?action=getDeviceType", base)
	if err := p.tryTest(ctx, dahuaURL, cam.Username, cam.Password); err == nil {
		return nil
	}

	// Try ISAPI-compatible endpoint.
	isapiURL := fmt.Sprintf("%s/ISAPI/System/deviceInfo", base)
	if err := p.tryTest(ctx, isapiURL, cam.Username, cam.Password); err == nil {
		return nil
	}

	return fmt.Errorf("zkteco test: device unreachable on both CGI and ISAPI endpoints")
}

// tryTest attempts a connection test against a single URL.
func (p *ZKTecoProvider) tryTest(ctx context.Context, url, username, password string) error {
	resp, err := digestAuthRequest(ctx, p.client, http.MethodGet, url, username, password)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	return nil
}

// Capabilities returns the ZKTeco provider's capabilities.
func (p *ZKTecoProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsDiscovery: false,
		SupportsSnapshot:  true,
		SupportsRTSP:      true,
		SupportsHLS:       false,
		SupportsPTZ:       false,
		SupportsEvents:    false,
	}
}
