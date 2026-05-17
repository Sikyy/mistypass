package hikconnect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL   string
	appKey    string
	appSecret string
	http      *http.Client
}

func NewClient(baseURL, appKey, appSecret string) *Client {
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		appKey:    appKey,
		appSecret: appSecret,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) GetAccessToken(ctx context.Context) (string, time.Time, error) {
	form := url.Values{
		"appKey":    {c.appKey},
		"appSecret": {c.appSecret},
	}
	var resp TokenResponse
	if err := c.postForm(ctx, "/api/lapp/token/get", form, &resp); err != nil {
		return "", time.Time{}, fmt.Errorf("get access token: %w", err)
	}
	if resp.Code != "200" {
		return "", time.Time{}, fmt.Errorf("ISC token error: code=%s msg=%s", resp.Code, resp.Msg)
	}
	expiresAt := time.UnixMilli(resp.Data.ExpireTime).UTC()
	return resp.Data.AccessToken, expiresAt, nil
}

func (c *Client) AddDevice(ctx context.Context, accessToken, serial, validateCode string) error {
	form := url.Values{
		"accessToken":  {accessToken},
		"deviceSerial": {serial},
		"validateCode": {validateCode},
	}
	var resp DeviceResponse
	if err := c.postForm(ctx, "/api/lapp/device/add", form, &resp); err != nil {
		return fmt.Errorf("add device: %w", err)
	}
	if resp.Code != "200" {
		return fmt.Errorf("ISC add device error: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *Client) DeleteDevice(ctx context.Context, accessToken, serial string) error {
	form := url.Values{
		"accessToken":  {accessToken},
		"deviceSerial": {serial},
	}
	var resp DeviceResponse
	if err := c.postForm(ctx, "/api/lapp/device/delete", form, &resp); err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	if resp.Code != "200" {
		return fmt.Errorf("ISC delete device error: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *Client) GetDeviceAccessToken(ctx context.Context, accessToken, serial string) (string, time.Time, error) {
	form := url.Values{
		"accessToken":  {accessToken},
		"deviceSerial": {serial},
	}
	var resp DeviceAccessTokenResponse
	if err := c.postForm(ctx, "/api/lapp/token/getDeviceAccessToken", form, &resp); err != nil {
		return "", time.Time{}, fmt.Errorf("get device access token: %w", err)
	}
	if resp.Code != "200" {
		return "", time.Time{}, fmt.Errorf("ISC device token error: code=%s msg=%s", resp.Code, resp.Msg)
	}
	expiresAt := time.UnixMilli(resp.Data.ExpireTime).UTC()
	return resp.Data.AccessToken, expiresAt, nil
}

func (c *Client) ListRecordings(ctx context.Context, accessToken, serial string, start, end time.Time) ([]RecordingItem, error) {
	form := url.Values{
		"accessToken":  {accessToken},
		"deviceSerial": {serial},
		"startTime":    {start.Format("2006-01-02 15:04:05")},
		"endTime":      {end.Format("2006-01-02 15:04:05")},
	}
	var resp RecordingListResponse
	if err := c.postForm(ctx, "/api/lapp/video/by-time", form, &resp); err != nil {
		return nil, fmt.Errorf("list recordings: %w", err)
	}
	if resp.Code != "200" {
		return nil, fmt.Errorf("ISC recordings error: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return resp.Data, nil
}

func (c *Client) postForm(ctx context.Context, path string, form url.Values, dest any) error {
	reqURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ISC HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}
