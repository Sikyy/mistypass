package camera

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// digestAuthRequest performs an HTTP request with digest authentication.
// It is shared across all camera providers.
func digestAuthRequest(ctx context.Context, client *http.Client, method, url, username, password string) (*http.Response, error) {
	// First request to get the WWW-Authenticate challenge.
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("digest auth: create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("digest auth: initial request: %w", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		// No challenge needed; return the response directly.
		return resp, nil
	}
	resp.Body.Close()

	challenge := resp.Header.Get("WWW-Authenticate")
	if challenge == "" {
		return nil, fmt.Errorf("digest auth: server returned 401 without WWW-Authenticate header")
	}

	params := parseDigestChallenge(challenge)
	realm := params["realm"]
	nonce := params["nonce"]
	qop := params["qop"]
	opaque := params["opaque"]

	// Compute digest response per RFC 2617.
	ha1 := md5Hex(username + ":" + realm + ":" + password)
	ha2 := md5Hex(method + ":" + uriFromURL(url))

	nc := "00000001"
	cnonce := generateCNonce()

	var response string
	if strings.Contains(qop, "auth") {
		response = md5Hex(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":auth:" + ha2)
	} else {
		response = md5Hex(ha1 + ":" + nonce + ":" + ha2)
	}

	// Build the Authorization header.
	auth := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s"`,
		username, realm, nonce, uriFromURL(url), response)
	if strings.Contains(qop, "auth") {
		auth += fmt.Sprintf(`, qop=auth, nc=%s, cnonce="%s"`, nc, cnonce)
	}
	if opaque != "" {
		auth += fmt.Sprintf(`, opaque="%s"`, opaque)
	}

	// Second request with the computed authorization.
	req2, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("digest auth: create authenticated request: %w", err)
	}
	req2.Header.Set("Authorization", auth)

	resp2, err := client.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("digest auth: authenticated request: %w", err)
	}
	return resp2, nil
}

// parseDigestChallenge extracts key=value pairs from a WWW-Authenticate header.
func parseDigestChallenge(header string) map[string]string {
	result := make(map[string]string)
	header = strings.TrimPrefix(header, "Digest ")
	header = strings.TrimPrefix(header, "digest ")

	parts := strings.Split(header, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		idx := strings.IndexByte(part, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(part[:idx])
		val := strings.TrimSpace(part[idx+1:])
		val = strings.Trim(val, `"`)
		result[key] = val
	}
	return result
}

// md5Hex returns the hex-encoded MD5 hash of s.
func md5Hex(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// generateCNonce returns a random 8-byte hex string.
func generateCNonce() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// uriFromURL extracts the path (and query) portion of a full URL.
func uriFromURL(rawURL string) string {
	// Find the path after the host portion.
	idx := strings.Index(rawURL, "://")
	if idx < 0 {
		return rawURL
	}
	rest := rawURL[idx+3:]
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return "/"
	}
	return rest[slash:]
}

// baseURL builds the scheme://host:port prefix from a CameraConfig.
func baseURL(cam CameraConfig) string {
	scheme := "http"
	defaultPort := 80
	if cam.UseTLS {
		scheme = "https"
		defaultPort = 443
	}
	port := cam.Port
	if port <= 0 {
		port = defaultPort
	}
	return fmt.Sprintf("%s://%s:%d", scheme, cam.Host, port)
}

// newHTTPClient returns a basic http.Client with a 30-second timeout.
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

// ---- ONVIF Provider ----

// ONVIFProvider implements CameraProvider using the ONVIF protocol.
type ONVIFProvider struct {
	client *http.Client
}

// NewONVIFProvider creates a new ONVIF camera provider.
func NewONVIFProvider() *ONVIFProvider {
	return &ONVIFProvider{
		client: newHTTPClient(),
	}
}

// wsDiscoveryProbe is the WS-Discovery multicast probe XML.
const wsDiscoveryProbe = `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
            xmlns:a="http://schemas.xmlsoap.org/ws/2004/08/addressing"
            xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery"
            xmlns:dn="http://www.onvif.org/ver10/network/wsdl">
  <s:Header>
    <a:Action>http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</a:Action>
    <a:MessageID>urn:uuid:discover-001</a:MessageID>
    <a:To>urn:schemas-xmlsoap-org:ws:2005:04:discovery</a:To>
  </s:Header>
  <s:Body>
    <d:Probe>
      <d:Types>dn:NetworkVideoTransmitter</d:Types>
    </d:Probe>
  </s:Body>
</s:Envelope>`

// probeMatch represents a parsed WS-Discovery ProbeMatch response.
type probeMatch struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		ProbeMatches struct {
			Matches []struct {
				EndpointReference struct {
					Address string `xml:"Address"`
				} `xml:"EndpointReference"`
				Types  string `xml:"Types"`
				Scopes string `xml:"Scopes"`
				XAddrs string `xml:"XAddrs"`
			} `xml:"ProbeMatch"`
		} `xml:"ProbeMatches"`
	} `xml:"Body"`
}

// Discover sends a WS-Discovery multicast probe and collects ONVIF device responses.
func (p *ONVIFProvider) Discover(ctx context.Context, subnet string) ([]DiscoveredCamera, error) {
	const multicastAddr = "239.255.255.250:3702"
	const discoveryTimeout = 3 * time.Second

	addr, err := net.ResolveUDPAddr("udp4", multicastAddr)
	if err != nil {
		return nil, fmt.Errorf("onvif discover: resolve multicast: %w", err)
	}

	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return nil, fmt.Errorf("onvif discover: listen udp: %w", err)
	}
	defer conn.Close()

	_, err = conn.WriteToUDP([]byte(wsDiscoveryProbe), addr)
	if err != nil {
		return nil, fmt.Errorf("onvif discover: send probe: %w", err)
	}

	deadline := time.Now().Add(discoveryTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	conn.SetReadDeadline(deadline)

	seen := make(map[string]bool)
	var cameras []DiscoveredCamera
	buf := make([]byte, 65535)

	for {
		n, remoteAddr, readErr := conn.ReadFromUDP(buf)
		if readErr != nil {
			// Timeout or context done; stop collecting.
			break
		}

		var match probeMatch
		if xmlErr := xml.Unmarshal(buf[:n], &match); xmlErr != nil {
			continue
		}

		for _, m := range match.Body.ProbeMatches.Matches {
			address := extractAddressFromXAddrs(m.XAddrs)
			if address == "" && remoteAddr != nil {
				address = remoteAddr.IP.String()
			}
			if seen[address] {
				continue
			}
			seen[address] = true

			name, manufacturer, model := parseScopesONVIF(m.Scopes)
			cameras = append(cameras, DiscoveredCamera{
				Name:         name,
				Manufacturer: manufacturer,
				Model:        model,
				Address:      address,
				Provider:     ProviderONVIF,
			})
		}
	}
	return cameras, nil
}

// extractAddressFromXAddrs pulls the host from the first XAddr URL.
func extractAddressFromXAddrs(xaddrs string) string {
	parts := strings.Fields(xaddrs)
	if len(parts) == 0 {
		return ""
	}
	u := parts[0]
	// Extract host from http://host:port/...
	idx := strings.Index(u, "://")
	if idx < 0 {
		return u
	}
	rest := u[idx+3:]
	// Strip path.
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		rest = rest[:slash]
	}
	// Strip port.
	if colon := strings.LastIndexByte(rest, ':'); colon >= 0 {
		rest = rest[:colon]
	}
	return rest
}

// parseScopesONVIF extracts name, manufacturer, and model from ONVIF scope URIs.
func parseScopesONVIF(scopes string) (name, manufacturer, model string) {
	for _, scope := range strings.Fields(scopes) {
		lower := strings.ToLower(scope)
		if strings.Contains(lower, "onvif://www.onvif.org/name/") {
			name = scope[strings.LastIndex(scope, "/")+1:]
		}
		if strings.Contains(lower, "onvif://www.onvif.org/manufacturer/") {
			manufacturer = scope[strings.LastIndex(scope, "/")+1:]
		}
		if strings.Contains(lower, "onvif://www.onvif.org/hardware/") {
			model = scope[strings.LastIndex(scope, "/")+1:]
		}
	}
	return
}

// onvifSOAPEnvelope wraps a SOAP body for ONVIF requests.
func onvifSOAPEnvelope(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
            xmlns:media="http://www.onvif.org/ver10/media/wsdl"
            xmlns:tt="http://www.onvif.org/ver10/schema"
            xmlns:tds="http://www.onvif.org/ver10/device/wsdl">
  <s:Body>` + body + `</s:Body>
</s:Envelope>`
}

// onvifSnapshotURIResponse parses the GetSnapshotUri SOAP response.
type onvifSnapshotURIResponse struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Response struct {
			MediaURI struct {
				URI string `xml:"Uri"`
			} `xml:"MediaUri"`
		} `xml:"GetSnapshotUriResponse"`
	} `xml:"Body"`
}

// onvifStreamURIResponse parses the GetStreamUri SOAP response.
type onvifStreamURIResponse struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Response struct {
			MediaURI struct {
				URI string `xml:"Uri"`
			} `xml:"MediaUri"`
		} `xml:"GetStreamUriResponse"`
	} `xml:"Body"`
}

// Snapshot captures a JPEG frame via ONVIF GetSnapshotUri.
func (p *ONVIFProvider) Snapshot(ctx context.Context, cam CameraConfig) ([]byte, string, error) {
	profileToken := cam.Extra["profile_token"]
	if profileToken == "" {
		profileToken = "Profile_1"
	}

	soapBody := fmt.Sprintf(`<media:GetSnapshotUri>
      <media:ProfileToken>%s</media:ProfileToken>
    </media:GetSnapshotUri>`, profileToken)

	envelope := onvifSOAPEnvelope(soapBody)
	serviceURL := baseURL(cam) + "/onvif/media_service"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serviceURL, strings.NewReader(envelope))
	if err != nil {
		return nil, "", fmt.Errorf("onvif snapshot: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("onvif snapshot: soap request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("onvif snapshot: soap request returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("onvif snapshot: read soap response: %w", err)
	}

	var parsed onvifSnapshotURIResponse
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nil, "", fmt.Errorf("onvif snapshot: parse soap response: %w", err)
	}

	snapshotURI := parsed.Body.Response.MediaURI.URI
	if snapshotURI == "" {
		return nil, "", fmt.Errorf("onvif snapshot: empty snapshot URI in response")
	}

	// Fetch the actual JPEG using digest auth.
	imgResp, err := digestAuthRequest(ctx, p.client, http.MethodGet, snapshotURI, cam.Username, cam.Password)
	if err != nil {
		return nil, "", fmt.Errorf("onvif snapshot: fetch image: %w", err)
	}
	defer imgResp.Body.Close()

	if imgResp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("onvif snapshot: image request returned status %d", imgResp.StatusCode)
	}

	imgData, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("onvif snapshot: read image: %w", err)
	}

	contentType := imgResp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return imgData, contentType, nil
}

// VideoLink returns an RTSP stream URI via ONVIF GetStreamUri.
func (p *ONVIFProvider) VideoLink(ctx context.Context, cam CameraConfig) (string, error) {
	profileToken := cam.Extra["profile_token"]
	if profileToken == "" {
		profileToken = "Profile_1"
	}

	soapBody := fmt.Sprintf(`<media:GetStreamUri>
      <media:StreamSetup>
        <tt:Stream>RTP-Unicast</tt:Stream>
        <tt:Transport><tt:Protocol>RTSP</tt:Protocol></tt:Transport>
      </media:StreamSetup>
      <media:ProfileToken>%s</media:ProfileToken>
    </media:GetStreamUri>`, profileToken)

	envelope := onvifSOAPEnvelope(soapBody)
	serviceURL := baseURL(cam) + "/onvif/media_service"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serviceURL, strings.NewReader(envelope))
	if err != nil {
		return "", fmt.Errorf("onvif video link: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("onvif video link: soap request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("onvif video link: soap request returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("onvif video link: read soap response: %w", err)
	}

	var parsed onvifStreamURIResponse
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("onvif video link: parse soap response: %w", err)
	}

	streamURI := parsed.Body.Response.MediaURI.URI
	if streamURI == "" {
		return "", fmt.Errorf("onvif video link: empty stream URI in response")
	}

	// Inject credentials into the RTSP URI.
	streamURI = injectRTSPCredentials(streamURI, cam.Username, cam.Password)
	return streamURI, nil
}

// injectRTSPCredentials inserts user:pass into an rtsp:// URL.
func injectRTSPCredentials(uri, username, password string) string {
	const prefix = "rtsp://"
	if !strings.HasPrefix(uri, prefix) {
		return uri
	}
	rest := uri[len(prefix):]
	return fmt.Sprintf("rtsp://%s:%s@%s", username, password, rest)
}

// TestConnection verifies camera reachability via ONVIF GetDeviceInformation.
func (p *ONVIFProvider) TestConnection(ctx context.Context, cam CameraConfig) error {
	soapBody := `<tds:GetDeviceInformation/>`
	envelope := onvifSOAPEnvelope(soapBody)
	serviceURL := baseURL(cam) + "/onvif/device_service"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serviceURL, strings.NewReader(envelope))
	if err != nil {
		return fmt.Errorf("onvif test: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("onvif test: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("onvif test: device returned status %d", resp.StatusCode)
	}
	return nil
}

// Capabilities returns the ONVIF provider's capabilities.
func (p *ONVIFProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsDiscovery: true,
		SupportsSnapshot:  true,
		SupportsRTSP:      true,
		SupportsHLS:       false,
		SupportsPTZ:       false,
		SupportsEvents:    true,
	}
}
