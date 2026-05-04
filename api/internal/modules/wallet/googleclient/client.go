package googleclient

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	walletIssuerScope = "https://www.googleapis.com/auth/wallet_object.issuer"
	defaultTokenURI   = "https://oauth2.googleapis.com/token"
	defaultIssuerAPI  = "https://walletobjects.googleapis.com/walletobjects/v1"
)

type ValidateInput struct {
	IssuerID            string
	ServiceAccountEmail string
	KeyRef              string
}

type ValidateItem struct {
	Field   string
	Status  string
	Message string
}

type ValidateReport struct {
	Items []ValidateItem
}

type serviceAccountCredential struct {
	Type         string `json:"type"`
	ProjectID    string `json:"project_id"`
	PrivateKeyID string `json:"private_key_id"`
	PrivateKey   string `json:"private_key"`
	ClientEmail  string `json:"client_email"`
	TokenURI     string `json:"token_uri"`
}

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

func ValidateConfig(ctx context.Context, input ValidateInput) (ValidateReport, error) {
	credJSON, err := readCredentialJSON(input.KeyRef)
	if err != nil {
		return ValidateReport{}, err
	}

	var cred serviceAccountCredential
	if err := json.Unmarshal(credJSON, &cred); err != nil {
		return ValidateReport{}, fmt.Errorf("decode service account json: %w", err)
	}

	clientEmail := strings.TrimSpace(strings.ToLower(cred.ClientEmail))
	if clientEmail == "" {
		return ValidateReport{}, errors.New("service account json missing client_email")
	}

	requestEmail := strings.TrimSpace(strings.ToLower(input.ServiceAccountEmail))
	if requestEmail == "" {
		return ValidateReport{}, errors.New("service_account_email is required")
	}
	if clientEmail != requestEmail {
		return ValidateReport{}, fmt.Errorf("service_account_email mismatch: request=%s credential=%s", requestEmail, clientEmail)
	}

	tokenURI := strings.TrimSpace(cred.TokenURI)
	if tokenURI == "" {
		tokenURI = defaultTokenURI
	}

	privateKey, err := parsePrivateKey(cred.PrivateKey)
	if err != nil {
		return ValidateReport{}, err
	}

	assertion, err := buildJWTAssertion(cred.ClientEmail, tokenURI, privateKey)
	if err != nil {
		return ValidateReport{}, err
	}

	accessToken, err := exchangeServiceAccountToken(ctx, tokenURI, assertion)
	if err != nil {
		return ValidateReport{}, err
	}

	if err := verifyIssuer(ctx, accessToken, strings.TrimSpace(input.IssuerID)); err != nil {
		return ValidateReport{}, err
	}

	return ValidateReport{
		Items: []ValidateItem{
			{Field: "google_oauth", Status: "ok", Message: "google oauth token exchange succeeded"},
			{Field: "google_issuer", Status: "ok", Message: "google issuer endpoint reachable and issuer exists"},
		},
	}, nil
}

func readCredentialJSON(keyRef string) ([]byte, error) {
	ref := strings.TrimSpace(keyRef)
	if ref == "" {
		return nil, errors.New("key_ref is required")
	}

	switch {
	case strings.HasPrefix(ref, "env://"):
		envName := strings.TrimPrefix(ref, "env://")
		return readCredentialJSONFromEnv(envName)
	case strings.HasPrefix(ref, "secret://env/"):
		envName := strings.TrimPrefix(ref, "secret://env/")
		return readCredentialJSONFromEnv(envName)
	case strings.HasPrefix(ref, "file://"):
		path := strings.TrimPrefix(ref, "file://")
		if strings.TrimSpace(path) == "" {
			return nil, errors.New("file:// key_ref missing path")
		}
		data, err := os.ReadFile(path) // #nosec G304 -- path comes from admin-set tenant config (key_ref), not request input
		if err != nil {
			return nil, fmt.Errorf("read service account json file: %w", err)
		}
		return data, nil
	default:
		return nil, errors.New("unsupported key_ref for remote validation; use env://<ENV_NAME>, secret://env/<ENV_NAME>, or file://<path>")
	}
}

func readCredentialJSONFromEnv(name string) ([]byte, error) {
	envName := strings.TrimSpace(name)
	if envName == "" {
		return nil, errors.New("env key_ref missing env var name")
	}
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return nil, fmt.Errorf("credential env %s is empty", envName)
	}
	return []byte(value), nil
}

func parsePrivateKey(raw string) (*rsa.PrivateKey, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "\\n", "\n")
	if normalized == "" {
		return nil, errors.New("service account json missing private_key")
	}

	block, _ := pem.Decode([]byte(normalized))
	if block == nil {
		return nil, errors.New("decode private_key pem failed")
	}

	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private_key is not rsa key")
		}
		return rsaKey, nil
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private_key: %w", err)
	}
	return key, nil
}

func buildJWTAssertion(clientEmail, tokenURI string, privateKey *rsa.PrivateKey) (string, error) {
	now := time.Now().UTC()
	claims := map[string]any{
		"iss":   clientEmail,
		"sub":   clientEmail,
		"aud":   tokenURI,
		"scope": walletIssuerScope,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	header := map[string]any{
		"alg": "RS256",
		"typ": "JWT",
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	token := encodeSegment(headerJSON) + "." + encodeSegment(claimsJSON)
	hashed := sha256.Sum256([]byte(token))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", fmt.Errorf("sign service account jwt: %w", err)
	}

	return token + "." + encodeSegment(signature), nil
}

func exchangeServiceAccountToken(ctx context.Context, tokenURI, assertion string) (string, error) {
	values := url.Values{}
	values.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	values.Set("assertion", assertion)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		tokenURI,
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request google oauth token failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("google oauth token exchange failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var token oauthTokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return "", fmt.Errorf("decode google oauth response: %w", err)
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return "", errors.New("google oauth response missing access_token")
	}
	return token.AccessToken, nil
}

func verifyIssuer(ctx context.Context, accessToken, issuerID string) error {
	nextIssuerID := strings.TrimSpace(issuerID)
	if nextIssuerID == "" {
		return errors.New("issuer_id is required")
	}

	endpoint := fmt.Sprintf("%s/issuer/%s", defaultIssuerAPI, url.PathEscape(nextIssuerID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request google issuer endpoint failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("google issuer verify failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func encodeSegment(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
