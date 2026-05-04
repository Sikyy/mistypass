// Package integration provides connectors to third-party office SaaS platforms
// (Lark, Google Workspace, DingTalk) for employee sync and event notifications.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	larkBaseURL        = "https://open.larksuite.com/open-apis"
	larkTokenEndpoint  = "/auth/v3/tenant_access_token/internal"
	larkTokenRefreshAt = 30 * time.Minute // refresh well before 2h expiry
)

// LarkConfig holds Lark Open Platform credentials.
type LarkConfig struct {
	AppID             string `json:"app_id"`
	AppSecret         string `json:"app_secret"`
	EncryptKey        string `json:"encrypt_key"`        // for event subscription verification
	VerificationToken string `json:"verification_token"` // for event callback challenge
	BaseURL           string `json:"base_url"`           // override for testing, default: larkBaseURL
}

// LarkClient manages authentication and HTTP communication with Lark Open Platform.
type LarkClient struct {
	config  LarkConfig
	client  *http.Client
	baseURL string

	mu          sync.RWMutex
	accessToken string
	tokenExpiry time.Time
}

// NewLarkClient creates a Lark API client with automatic token management.
func NewLarkClient(cfg LarkConfig) *LarkClient {
	base := cfg.BaseURL
	if base == "" {
		base = larkBaseURL
	}
	return &LarkClient{
		config:  cfg,
		client:  &http.Client{Timeout: 15 * time.Second},
		baseURL: base,
	}
}

// GetToken returns a valid tenant_access_token, refreshing if needed.
func (c *LarkClient) GetToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		token := c.accessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	return c.refreshToken(ctx)
}

func (c *LarkClient) refreshToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	body, _ := json.Marshal(map[string]string{
		"app_id":     c.config.AppID,
		"app_secret": c.config.AppSecret,
	})

	url := c.baseURL + larkTokenEndpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("lark token request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("lark token decode: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("lark token error: code=%d msg=%s", result.Code, result.Msg)
	}

	c.accessToken = result.TenantAccessToken
	c.tokenExpiry = time.Now().Add(larkTokenRefreshAt)
	return c.accessToken, nil
}

// DoRequest performs an authenticated API request to Lark.
func (c *LarkClient) DoRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	token, err := c.GetToken(ctx)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lark request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lark read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("lark %s %s: status %d body=%s", method, path, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
