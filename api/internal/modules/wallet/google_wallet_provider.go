package wallet

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// GoogleWalletProvider creates pass classes and objects for Google Wallet.
// The current implementation is a mock that produces structurally valid
// pass objects and JWT save links without calling the real Google Wallet API.
// Swap for a real implementation with service account auth for production.
type GoogleWalletProvider struct {
	IssuerID            string
	ServiceAccountEmail string
	ClassPrefix         string
}

type GoogleWalletConfig struct {
	IssuerID            string `json:"issuer_id"`
	ServiceAccountEmail string `json:"service_account_email"`
	ClassPrefix         string `json:"class_prefix"`
}

type GoogleWalletPassClass struct {
	ID               string `json:"id"`
	IssuerName       string `json:"issuer_name"`
	ClassTemplateID  string `json:"class_template_id"`
	ReviewStatus     string `json:"review_status"`
	NfcEnabled       bool   `json:"nfc_enabled"`
	CreatedAt        string `json:"created_at"`
	MockProvider     bool   `json:"mock_provider"`
}

type GoogleWalletPassObject struct {
	ID             string `json:"id"`
	ClassID        string `json:"class_id"`
	ObjectID       string `json:"object_id"`
	HolderName     string `json:"holder_name"`
	HolderEmail    string `json:"holder_email"`
	State          string `json:"state"`
	NfcPayload     string `json:"nfc_payload"`
	Barcode        string `json:"barcode"`
	SaveLink       string `json:"save_link"`
	SaveLinkJWT    string `json:"save_link_jwt"`
	CreatedAt      string `json:"created_at"`
	MockProvider   bool   `json:"mock_provider"`
}

type GoogleWalletUpdateResult struct {
	ObjectID   string `json:"object_id"`
	State      string `json:"state"`
	UpdatedAt  string `json:"updated_at"`
}

func NewGoogleWalletProvider(cfg GoogleWalletConfig) *GoogleWalletProvider {
	issuerID := strings.TrimSpace(cfg.IssuerID)
	if issuerID == "" {
		issuerID = "mistyislet_dev_issuer"
	}
	sae := strings.TrimSpace(cfg.ServiceAccountEmail)
	if sae == "" {
		sae = "wallet@mistyislet-dev.iam.gserviceaccount.com"
	}
	prefix := strings.TrimSpace(cfg.ClassPrefix)
	if prefix == "" {
		prefix = "mistyislet.access"
	}
	return &GoogleWalletProvider{
		IssuerID:            issuerID,
		ServiceAccountEmail: sae,
		ClassPrefix:         prefix,
	}
}

// CreatePassClass creates a Google Wallet pass class (template).
// In production, this calls POST https://walletobjects.googleapis.com/walletobjects/v1/genericClass
func (p *GoogleWalletProvider) CreatePassClass(tenantID, templateID, templateName string) (GoogleWalletPassClass, error) {
	classID := fmt.Sprintf("%s.%s.class.%s", p.IssuerID, p.ClassPrefix, templateID)
	return GoogleWalletPassClass{
		ID:              classID,
		IssuerName:      "Mistyislet",
		ClassTemplateID: templateID,
		ReviewStatus:    "UNDER_REVIEW",
		NfcEnabled:      true,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		MockProvider:    true,
	}, nil
}

// IssuePassObject creates a Google Wallet pass object and generates a save link.
// In production, this calls POST https://walletobjects.googleapis.com/walletobjects/v1/genericObject
// and signs a JWT for the save URL.
func (p *GoogleWalletProvider) IssuePassObject(tenantID, passID, classID, holderName, holderEmail string) (GoogleWalletPassObject, error) {
	objectID := fmt.Sprintf("%s.%s.%s", p.IssuerID, p.ClassPrefix, passID)

	nfcBytes := make([]byte, 8)
	rand.Read(nfcBytes)
	nfcPayload := fmt.Sprintf("mistyislet:%s:%s", passID, hex.EncodeToString(nfcBytes))

	barcodeBytes := make([]byte, 6)
	rand.Read(barcodeBytes)
	barcode := hex.EncodeToString(barcodeBytes)

	now := time.Now().UTC()

	// Build mock JWT save link
	// In production: sign with service account private key using RS256
	jwtPayload := map[string]any{
		"iss": p.ServiceAccountEmail,
		"aud": "google",
		"typ": "savetowallet",
		"iat": now.Unix(),
		"payload": map[string]any{
			"genericObjects": []map[string]any{
				{
					"id":      objectID,
					"classId": classID,
					"genericType": "GENERIC_TYPE_UNSPECIFIED",
					"cardTitle": map[string]any{
						"defaultValue": map[string]string{
							"language": "en",
							"value":    "Mistyislet Access",
						},
					},
					"header": map[string]any{
						"defaultValue": map[string]string{
							"language": "en",
							"value":    holderName,
						},
					},
					"subheader": map[string]any{
						"defaultValue": map[string]string{
							"language": "en",
							"value":    holderEmail,
						},
					},
					"barcode": map[string]any{
						"type":  "QR_CODE",
						"value": nfcPayload,
					},
					"state": "ACTIVE",
				},
			},
		},
	}
	jwtBytes, _ := json.Marshal(jwtPayload)
	mockJWT := mockJWTSign(jwtBytes)
	saveLink := "https://pay.google.com/gp/v/save/" + mockJWT

	return GoogleWalletPassObject{
		ID:           passID,
		ClassID:      classID,
		ObjectID:     objectID,
		HolderName:   holderName,
		HolderEmail:  holderEmail,
		State:        "ACTIVE",
		NfcPayload:   nfcPayload,
		Barcode:      barcode,
		SaveLink:     saveLink,
		SaveLinkJWT:  mockJWT,
		CreatedAt:    now.Format(time.RFC3339),
		MockProvider: true,
	}, nil
}

// UpdatePassObjectState updates the pass object state (ACTIVE/INACTIVE/EXPIRED).
// In production: PATCH https://walletobjects.googleapis.com/walletobjects/v1/genericObject/{objectId}
func (p *GoogleWalletProvider) UpdatePassObjectState(objectID, state string) (GoogleWalletUpdateResult, error) {
	return GoogleWalletUpdateResult{
		ObjectID:  objectID,
		State:     strings.ToUpper(strings.TrimSpace(state)),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// mockJWTSign produces a structurally valid but unsigned JWT for development.
// Format: base64url(header).base64url(payload).mock_signature
func mockJWTSign(payload []byte) string {
	header := `{"alg":"RS256","typ":"JWT"}`
	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(header))
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	sigInput := headerB64 + "." + payloadB64
	sigHash := sha256.Sum256([]byte(sigInput))
	sigB64 := base64.RawURLEncoding.EncodeToString(sigHash[:])
	return headerB64 + "." + payloadB64 + "." + sigB64
}
