package hikconnect

import (
	"context"
	"fmt"
	"time"
)

// ISCClient defines the interface for ISC OpenAPI calls (mockable).
type ISCClient interface {
	GetAccessToken(ctx context.Context) (string, time.Time, error)
	AddDevice(ctx context.Context, accessToken, serial, validateCode string) error
	DeleteDevice(ctx context.Context, accessToken, serial string) error
	GetDeviceAccessToken(ctx context.Context, accessToken, serial string) (string, time.Time, error)
	ListRecordings(ctx context.Context, accessToken, serial string, start, end time.Time) ([]RecordingItem, error)
}

// TokenCache defines the interface for caching ISC tokens.
type TokenCache interface {
	GetHikConnectAccessToken() (string, bool, error)
	SetHikConnectAccessToken(token string, ttl time.Duration) error
	DeleteHikConnectAccessToken() error
	GetHikConnectDeviceToken(serial string) (string, bool, error)
	SetHikConnectDeviceToken(serial, token string, ttl time.Duration) error
}

type Service struct {
	client         ISCClient
	cache          TokenCache
	accessTokenTTL time.Duration
}

func NewService(client ISCClient, cache TokenCache, accessTokenTTL time.Duration) *Service {
	if accessTokenTTL <= 0 {
		accessTokenTTL = 115 * time.Minute
	}
	return &Service{
		client:         client,
		cache:          cache,
		accessTokenTTL: accessTokenTTL,
	}
}

func (s *Service) BindDevice(ctx context.Context, serial, verifyCode string) error {
	accessToken, err := s.ensureAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("bind device: %w", err)
	}
	return s.client.AddDevice(ctx, accessToken, serial, verifyCode)
}

func (s *Service) UnbindDevice(ctx context.Context, serial string) error {
	accessToken, err := s.ensureAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("unbind device: %w", err)
	}
	return s.client.DeleteDevice(ctx, accessToken, serial)
}

func (s *Service) GetPlaybackToken(ctx context.Context, serial string, channel int) (PlaybackToken, error) {
	// Check device token cache first
	if cached, found, err := s.cache.GetHikConnectDeviceToken(serial); err == nil && found {
		return PlaybackToken{
			AccessToken:  cached,
			DeviceSerial: serial,
			Channel:      channel,
			ExpiresAt:    time.Now().Add(5 * time.Minute), // approximate; real expiry tracked server-side
		}, nil
	}

	accessToken, err := s.ensureAccessToken(ctx)
	if err != nil {
		return PlaybackToken{}, fmt.Errorf("get playback token: %w", err)
	}

	deviceToken, expiresAt, err := s.client.GetDeviceAccessToken(ctx, accessToken, serial)
	if err != nil {
		return PlaybackToken{}, fmt.Errorf("get playback token: %w", err)
	}

	// Cache device token with TTL = actual_expiry - 2 minutes
	// ISC device tokens are valid ~7 days; caching near full expiry avoids unnecessary API calls
	ttl := time.Until(expiresAt) - 2*time.Minute
	if ttl > 0 {
		_ = s.cache.SetHikConnectDeviceToken(serial, deviceToken, ttl)
	}

	return PlaybackToken{
		AccessToken:  deviceToken,
		DeviceSerial: serial,
		Channel:      channel,
		ExpiresAt:    expiresAt,
	}, nil
}

func (s *Service) ListRecordings(ctx context.Context, serial string, start, end time.Time) ([]Recording, error) {
	accessToken, err := s.ensureAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recordings: %w", err)
	}

	items, err := s.client.ListRecordings(ctx, accessToken, serial, start, end)
	if err != nil {
		return nil, err
	}

	recordings := make([]Recording, 0, len(items))
	for _, item := range items {
		startTime, _ := time.Parse("2006-01-02 15:04:05", item.StartTime)
		endTime, _ := time.Parse("2006-01-02 15:04:05", item.EndTime)
		recordings = append(recordings, Recording{
			StartTime: startTime,
			EndTime:   endTime,
			Type:      recordingTypeName(item.Type),
		})
	}
	return recordings, nil
}

func (s *Service) ensureAccessToken(ctx context.Context) (string, error) {
	if cached, found, err := s.cache.GetHikConnectAccessToken(); err == nil && found {
		return cached, nil
	}

	token, expiresAt, err := s.client.GetAccessToken(ctx)
	if err != nil {
		return "", err
	}

	ttl := time.Until(expiresAt) - 5*time.Minute
	if ttl > s.accessTokenTTL {
		ttl = s.accessTokenTTL
	}
	if ttl > 0 {
		_ = s.cache.SetHikConnectAccessToken(token, ttl)
	}

	return token, nil
}

func recordingTypeName(code int) string {
	switch code {
	case 1:
		return "motion"
	case 2:
		return "continuous"
	case 3:
		return "alarm"
	default:
		return "unknown"
	}
}
