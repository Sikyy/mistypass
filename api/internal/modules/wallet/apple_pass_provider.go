package wallet

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ApplePassProvider generates .pkpass bundles for Apple Wallet integration.
// The current implementation is a mock that produces structurally valid but
// unsigned pass bundles suitable for development and testing. Swap for a real
// implementation with Pass Type Certificate signing for production.
type ApplePassProvider struct {
	TeamID         string
	PassTypeID     string
	OrganizationID string
	WebServiceURL  string
}

type ApplePassConfig struct {
	TeamID         string `json:"team_id"`
	PassTypeID     string `json:"pass_type_id"`
	OrganizationID string `json:"organization_id"`
	WebServiceURL  string `json:"web_service_url"`
}

type ApplePassBundle struct {
	SerialNumber  string `json:"serial_number"`
	AuthToken     string `json:"auth_token"`
	PassJSON      string `json:"pass_json"`
	ManifestJSON  string `json:"manifest_json"`
	Signature     string `json:"signature"`
	SaveLink      string `json:"save_link"`
	NfcPayload    string `json:"nfc_payload"`
	BundleSize    int    `json:"bundle_size"`
	CreatedAt     string `json:"created_at"`
	MockProvider  bool   `json:"mock_provider"`
}

type ApplePassDeviceRegistration struct {
	DeviceLibraryID string `json:"device_library_id"`
	PushToken       string `json:"push_token"`
	SerialNumber    string `json:"serial_number"`
	PassTypeID      string `json:"pass_type_id"`
	RegisteredAt    string `json:"registered_at"`
}

func NewApplePassProvider(cfg ApplePassConfig) *ApplePassProvider {
	teamID := strings.TrimSpace(cfg.TeamID)
	if teamID == "" {
		teamID = "MISTYISLET_DEV"
	}
	passTypeID := strings.TrimSpace(cfg.PassTypeID)
	if passTypeID == "" {
		passTypeID = "pass.com.mistyislet.access"
	}
	orgID := strings.TrimSpace(cfg.OrganizationID)
	if orgID == "" {
		orgID = "mistyislet"
	}
	webServiceURL := strings.TrimSpace(cfg.WebServiceURL)
	if webServiceURL == "" {
		webServiceURL = "https://api.mistyislet.com/v1/passes"
	}
	return &ApplePassProvider{
		TeamID:         teamID,
		PassTypeID:     passTypeID,
		OrganizationID: orgID,
		WebServiceURL:  webServiceURL,
	}
}

// IssuePass generates a mock .pkpass bundle with structurally valid pass.json,
// manifest, and a placeholder signature. In production, the signature would use
// the Pass Type Certificate + Apple WWDR intermediate.
func (p *ApplePassProvider) IssuePass(tenantID, holderName, holderEmail, passID string) (ApplePassBundle, error) {
	serialBytes := make([]byte, 12)
	rand.Read(serialBytes)
	serialNumber := hex.EncodeToString(serialBytes)

	authTokenBytes := make([]byte, 16)
	rand.Read(authTokenBytes)
	authToken := hex.EncodeToString(authTokenBytes)

	nfcPayloadBytes := make([]byte, 8)
	rand.Read(nfcPayloadBytes)
	nfcPayload := fmt.Sprintf("mistyislet:%s:%s", passID, hex.EncodeToString(nfcPayloadBytes))

	now := time.Now().UTC()

	passJSON := map[string]any{
		"formatVersion":       1,
		"passTypeIdentifier":  p.PassTypeID,
		"serialNumber":        serialNumber,
		"teamIdentifier":      p.TeamID,
		"organizationName":    "Mistyislet",
		"description":         "Mistyislet Access Pass",
		"authenticationToken": authToken,
		"webServiceURL":       p.WebServiceURL,
		"nfc": map[string]any{
			"message":              nfcPayload,
			"encryptionPublicKey":  "",
			"requiresAuthentication": false,
		},
		"generic": map[string]any{
			"primaryFields": []map[string]any{
				{"key": "holder", "label": "Holder", "value": holderName},
			},
			"secondaryFields": []map[string]any{
				{"key": "email", "label": "Email", "value": holderEmail},
				{"key": "org", "label": "Organization", "value": p.OrganizationID},
			},
			"auxiliaryFields": []map[string]any{
				{"key": "pass_id", "label": "Pass ID", "value": passID},
			},
			"backFields": []map[string]any{
				{"key": "issued", "label": "Issued", "value": now.Format(time.RFC3339)},
				{"key": "tenant", "label": "Tenant", "value": tenantID},
			},
		},
		"barcode": map[string]any{
			"format":          "PKBarcodeFormatQR",
			"message":         nfcPayload,
			"messageEncoding": "iso-8859-1",
		},
		"backgroundColor": "rgb(32, 36, 67)",
		"foregroundColor": "rgb(255, 255, 255)",
		"labelColor":      "rgb(200, 200, 220)",
	}

	passJSONBytes, _ := json.MarshalIndent(passJSON, "", "  ")

	// manifest: SHA256 of each file in the bundle
	passHash := sha256.Sum256(passJSONBytes)
	manifest := map[string]string{
		"pass.json":  hex.EncodeToString(passHash[:]),
		"icon.png":   "placeholder_icon_hash",
		"icon@2x.png": "placeholder_icon_2x_hash",
		"logo.png":   "placeholder_logo_hash",
	}
	manifestBytes, _ := json.Marshal(manifest)

	// mock signature (in production: PKCS#7 detached signature using Pass Type Certificate)
	sigHash := sha256.Sum256(manifestBytes)
	mockSignature := "MOCK_PKCS7_" + hex.EncodeToString(sigHash[:])

	return ApplePassBundle{
		SerialNumber: serialNumber,
		AuthToken:    authToken,
		PassJSON:     string(passJSONBytes),
		ManifestJSON: string(manifestBytes),
		Signature:    mockSignature,
		SaveLink:     fmt.Sprintf("%s/v1/passes/%s/%s", p.WebServiceURL, p.PassTypeID, serialNumber),
		NfcPayload:   nfcPayload,
		BundleSize:   len(passJSONBytes) + len(manifestBytes) + len(mockSignature),
		CreatedAt:    now.Format(time.RFC3339),
		MockProvider: true,
	}, nil
}

// RegisterDevice records a device registration for push update notifications.
// In production, this stores the push token and sends APNs when the pass updates.
func (p *ApplePassProvider) RegisterDevice(serialNumber, deviceLibraryID, pushToken string) (ApplePassDeviceRegistration, error) {
	return ApplePassDeviceRegistration{
		DeviceLibraryID: strings.TrimSpace(deviceLibraryID),
		PushToken:       strings.TrimSpace(pushToken),
		SerialNumber:    strings.TrimSpace(serialNumber),
		PassTypeID:      p.PassTypeID,
		RegisteredAt:    time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// NotifyPassUpdate would send an APNs push to the device to trigger a pass refresh.
// Mock implementation logs the intent and returns success.
func (p *ApplePassProvider) NotifyPassUpdate(pushToken, serialNumber string) error {
	// In production: send empty push to APNs → device calls webServiceURL to get updated pass
	return nil
}
