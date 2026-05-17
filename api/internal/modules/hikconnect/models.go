package hikconnect

import "time"

// --- ISC OpenAPI request/response types ---

// TokenResponse is the ISC /api/lapp/token/get response.
type TokenResponse struct {
	Code string         `json:"code"`
	Msg  string         `json:"msg"`
	Data TokenDataField `json:"data"`
}

type TokenDataField struct {
	AccessToken string `json:"accessToken"`
	ExpireTime  int64  `json:"expireTime"` // milliseconds since epoch
}

// DeviceAddRequest is the body for /api/lapp/device/add.
type DeviceAddRequest struct {
	AccessToken  string `json:"accessToken"`
	DeviceSerial string `json:"deviceSerial"`
	ValidateCode string `json:"validateCode"`
}

// DeviceResponse is a generic ISC device operation response.
type DeviceResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

// DeviceListResponse is the ISC /api/lapp/device/list response.
type DeviceListResponse struct {
	Code string           `json:"code"`
	Msg  string           `json:"msg"`
	Data []DeviceListItem `json:"data"`
}

type DeviceListItem struct {
	DeviceSerial string `json:"deviceSerial"`
	DeviceName   string `json:"deviceName"`
	Status       int    `json:"status"` // 1=online, 2=offline
	Defence      int    `json:"defence"`
	DeviceType   string `json:"deviceType"`
}

// DeviceAccessTokenResponse is the ISC /api/lapp/token/getDeviceAccessToken response.
type DeviceAccessTokenResponse struct {
	Code string                     `json:"code"`
	Msg  string                     `json:"msg"`
	Data DeviceAccessTokenDataField `json:"data"`
}

type DeviceAccessTokenDataField struct {
	AccessToken string `json:"accessToken"`
	ExpireTime  int64  `json:"expireTime"`
}

// RecordingListResponse is the ISC /api/lapp/video/by-time response.
type RecordingListResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data []RecordingItem `json:"data"`
}

type RecordingItem struct {
	StartTime string `json:"startTime"` // "2026-05-17 10:00:00"
	EndTime   string `json:"endTime"`
	Type      int    `json:"type"`
}

// --- Domain types (returned to callers) ---

// PlaybackToken is what the mobile app needs to initialize the ISC SDK.
type PlaybackToken struct {
	AccessToken  string    `json:"access_token"`
	DeviceSerial string    `json:"device_serial"`
	Channel      int       `json:"channel"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Recording represents a cloud recording segment.
type Recording struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Type      string    `json:"type"` // "motion", "continuous", "alarm"
}
