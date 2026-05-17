# Hik-Connect (ISC OpenSDK) Integration Design

## Overview

Integrate Hikvision cloud video (Hik-Connect / ISC platform) into MistyPass so tenants can view live camera streams and cloud playback from the iOS app without requiring LAN access or MistyPass-hosted transcoding.

**Approach:** P2P video via ISC OpenSDK on the client. MistyPass backend acts as token broker only — video bytes never transit our servers.

**Key constraint:** Indonesia uses "Hik-Connect" (international EZVIZ equivalent) via the ISC OpenAPI platform. We integrate as an ISV (Independent Software Vendor) using a single MistyPass platform account, with tenant isolation enforced at the MistyPass application layer.

---

## 1. Architecture

```
┌──────────────┐         ┌──────────────────┐        ┌─────────────────┐
│  iOS App     │◄──P2P──►│  Hikvision Cloud │◄──────►│  Camera/NVR     │
│  (ISC SDK)   │         │  (ISC Platform)  │        │  (on-site)      │
└──────┬───────┘         └────────┬─────────┘        └─────────────────┘
       │ REST                     │ OpenAPI
       ▼                          ▼
┌──────────────────────────────────────────┐
│          MistyPass API                   │
│  ┌────────────┐  ┌───────────────────┐   │
│  │ hikconnect │  │ camera (existing) │   │
│  │  package   │  │    module         │   │
│  └────────────┘  └───────────────────┘   │
│          │                               │
│          ▼                               │
│  ┌──────────────┐                        │
│  │ Redis cache  │ (token + device list)  │
│  └──────────────┘                        │
└──────────────────────────────────────────┘
```

**Data flow — live preview:**
1. User taps play on camera in iOS app
2. App calls `GET /api/v1/app/cameras/{id}/cloud-token`
3. Backend exchanges ISV credentials for a scoped device token via ISC OpenAPI
4. Backend returns `{token, device_serial, channel}` to app
5. App initializes ISC OpenSDK player with token, establishes P2P to camera
6. Video streams directly camera→app (no server relay)

**Data flow — cloud playback:**
1. App calls `GET /api/v1/app/cameras/{id}/cloud-recordings?date=2026-05-17`
2. Backend queries ISC OpenAPI for recording segments
3. App receives time segments, initializes SDK playback with token + time range

---

## 2. Backend

### 2.1 New Package: `api/internal/modules/hikconnect/`

| File | Responsibility |
|------|---------------|
| `client.go` | HTTP client wrapping ISC OpenAPI (auth, token refresh, request signing) |
| `service.go` | Business logic: bind device, get token, list recordings, handle errors |
| `models.go` | ISC API request/response types, internal domain types |

### 2.2 ISC OpenAPI Client (`client.go`)

Wraps these ISC endpoints:

| ISC Endpoint | Purpose |
|---|---|
| `POST /api/lapp/token/get` | Get platform access token (AppKey+Secret) |
| `POST /api/lapp/device/add` | Bind device to ISV account by serial+verification code |
| `POST /api/lapp/device/delete` | Unbind device |
| `GET /api/lapp/device/list` | List bound devices with online status |
| `GET /api/lapp/device/info` | Single device info |
| `POST /api/lapp/token/getDeviceAccessToken` | Scoped token for SDK playback |
| `GET /api/lapp/video/by-time` | Cloud recording segments |

Authentication: `POST /api/lapp/token/get` with AppKey+AppSecret in body returns an access token. All subsequent calls include the access token as a query/body parameter. No per-request HMAC signing required.

Access token caching: Store in Redis with key `hikconnect:access_token`, TTL = token expiry minus 5 minutes.

### 2.3 Service Layer (`service.go`)

```go
type Service struct {
    client      *Client
    cameraStore CameraStore  // interface to camera module
    cache       RedisCache
}

// BindDevice registers a Hikvision device under the MistyPass ISV account.
// Called by admin when adding a cloud-enabled camera.
func (s *Service) BindDevice(ctx context.Context, serial, verifyCode string) error

// UnbindDevice removes device from ISV account.
func (s *Service) UnbindDevice(ctx context.Context, serial string) error

// GetPlaybackToken returns a short-lived token for ISC SDK initialization.
func (s *Service) GetPlaybackToken(ctx context.Context, deviceSerial string) (PlaybackToken, error)

// ListRecordings returns cloud recording segments for a date range.
func (s *Service) ListRecordings(ctx context.Context, serial string, start, end time.Time) ([]Recording, error)

// SyncDeviceStatus polls ISC for device online/offline state.
func (s *Service) SyncDeviceStatus(ctx context.Context) error
```

### 2.4 Camera Model Extensions

Add fields to the existing `Camera` struct (in-memory map today; will become DB columns when camera storage moves to PostgreSQL):

| Field | Type | Purpose |
|---|---|---|
| `cloud_provider` | string | `"hikconnect"`, `""` (local-only), future: `"dahua_dmss"` |
| `cloud_serial` | string | Device serial registered with ISC platform |
| `cloud_verified` | bool | Whether device binding is confirmed |
| `cloud_channels` | int | Number of channels (NVR may have multiple) |

These extend the existing camera record. A camera with `cloud_provider = ""` continues to work via LAN RTSP as today.

### 2.5 New API Routes

Added to `routes_app_redesign.go`:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/app/cameras/{id}/cloud-token` | Get ISC playback token for SDK |
| `GET` | `/app/cameras/{id}/cloud-recordings` | List cloud recording segments |
| `POST` | `/admin/cameras/{id}/cloud-bind` | Admin: bind device to ISC by serial |
| `DELETE` | `/admin/cameras/{id}/cloud-bind` | Admin: unbind device from ISC |
| `GET` | `/admin/cameras/cloud-status` | Admin: bulk device online status |

### 2.6 Configuration

Environment variables:

```
HIK_ISC_HOST=https://open.hikconnect.com       # ISC OpenAPI base URL (international)
HIK_ISC_APP_KEY=<isv-app-key>                  # ISV credentials from Hik partner portal
HIK_ISC_APP_SECRET=<isv-app-secret>
HIK_ISC_TOKEN_CACHE_TTL=6900                   # seconds (default: 115 min, token valid 2h)
```

---

## 3. iOS Client

### 3.1 VideoStreamProvider Protocol

Abstraction allowing the player view to work with multiple backends:

```swift
protocol VideoStreamProvider {
    var providerName: String { get }
    func preparePlayback(camera: Camera) async throws -> PlaybackSession
    func startLive(session: PlaybackSession, in view: UIView) async throws
    func startPlayback(session: PlaybackSession, from: Date, to: Date, in view: UIView) async throws
    func stop(session: PlaybackSession)
    func currentState(session: PlaybackSession) -> StreamState
}

enum StreamState {
    case idle, connecting, playing, buffering, error(String)
}

struct PlaybackSession {
    let token: String
    let deviceSerial: String
    let channel: Int
    let provider: String
}
```

### 3.2 HikConnectStreamProvider

Wraps ISC OpenSDK (HikVideo.framework):

```swift
final class HikConnectStreamProvider: VideoStreamProvider {
    let providerName = "hikconnect"

    func preparePlayback(camera: Camera) async throws -> PlaybackSession {
        // Call MistyPass API: GET /app/cameras/{id}/cloud-token
        let token = try await APIService.shared.fetchCloudToken(cameraId: camera.id)
        return PlaybackSession(
            token: token.accessToken,
            deviceSerial: token.deviceSerial,
            channel: token.channel,
            provider: providerName
        )
    }

    func startLive(session: PlaybackSession, in view: UIView) async throws {
        // Initialize ISC SDK player
        // HikVideoPlayer.shared.startRealPlay(...)
    }

    func startPlayback(session: PlaybackSession, from: Date, to: Date, in view: UIView) async throws {
        // HikVideoPlayer.shared.startPlayback(...)
    }

    func stop(session: PlaybackSession) {
        // HikVideoPlayer.shared.stopPlay()
    }
}
```

### 3.3 LAN RTSP Provider (existing behavior)

```swift
final class LANStreamProvider: VideoStreamProvider {
    let providerName = "lan"

    func preparePlayback(camera: Camera) async throws -> PlaybackSession {
        let link = try await APIService.shared.fetchCameraVideoLink(cameraId: camera.id)
        return PlaybackSession(
            token: link.videoUrl,  // RTSP URL stored in token field
            deviceSerial: camera.id,
            channel: 1,
            provider: providerName
        )
    }

    func startLive(session: PlaybackSession, in view: UIView) async throws {
        // Use AVPlayer with HLS proxy (existing DEBUG path)
        // or VLCKit for RTSP in production
    }
}
```

### 3.4 CameraPlayerContainerView

Replaces the current `CameraPlayerView`. Selects provider based on `camera.cloud_provider`:

```swift
struct CameraPlayerContainerView: View {
    let camera: Camera
    @State private var provider: VideoStreamProvider?
    @State private var session: PlaybackSession?
    @State private var state: StreamState = .idle

    var body: some View {
        ZStack {
            VideoRenderView(provider: provider, session: session)
            // Overlay: loading, error, controls
        }
        .task {
            provider = StreamProviderFactory.make(for: camera)
            do {
                session = try await provider?.preparePlayback(camera: camera)
                try await provider?.startLive(session: session!, in: renderView)
            } catch {
                state = .error(error.localizedDescription)
            }
        }
    }
}
```

### 3.5 SDK Integration

- Add ISC OpenSDK (HikVideo.framework) via CocoaPods or manual framework embed
- SDK binary is ~15MB, iOS 13+
- Requires camera permission (already granted) and network permission
- SDK initialization in AppDelegate with ISV AppKey

---

## 4. Error Handling & Degradation

### 4.1 Device Offline

- ISC reports device offline → backend caches status
- App shows "Camera offline — check network at site"
- Retry button polls status every 30s (max 5 retries then back off)

### 4.2 No Cloud Storage Subscription

- Some cameras have no cloud storage plan (Hik-Connect cloud costs extra)
- `GET /cloud-recordings` returns empty list → app shows "Live only — no cloud recordings"
- Live P2P still works regardless of cloud storage plan

### 4.3 ISC Rate Limiting

- ISC OpenAPI: 100 calls/second per AppKey
- Backend queues and deduplicates token requests per device
- If rate limited (HTTP 429 or error code 10004): exponential backoff, return cached token if not expired
- Redis-based sliding window counter: `hikconnect:rate:{minute}` with 60s TTL

### 4.4 P2P Connection Failure

- SDK reports P2P establishment failed (NAT traversal issue)
- App retries once with 3s delay
- If still failing: show "Unable to connect — camera may be behind strict firewall"
- Suggest: "Enable UPnP on site router or contact building IT"

### 4.5 ISC Platform Unavailable

- Backend health check: periodic ping to ISC `/api/lapp/token/get`
- If ISC is down: serve cached tokens (if still valid), return 503 for new requests
- App falls back to LAN RTSP if device is on same network (detect via subnet comparison)
- Monitoring: alert if ISC error rate > 5% over 5 minutes

### 4.6 Token Caching Strategy

| Key | TTL | Purpose |
|---|---|---|
| `hikconnect:access_token` | token_expiry - 5min | Platform auth token |
| `hikconnect:device_token:{serial}` | 5min | Per-device SDK token |
| `hikconnect:device_list` | 10min | Bound device inventory |
| `hikconnect:recordings:{serial}:{date}` | 2min | Recording segment cache |

Cache-aside pattern: check Redis → miss → call ISC → store in Redis → return.

### 4.7 Feature Degradation by Subscription

MistyPass tenant subscription determines available features:

| Feature | Free | Standard | Pro |
|---|---|---|---|
| Live P2P preview | 1 camera | 5 cameras | Unlimited |
| Cloud playback | - | 7 days | 30 days |
| Event snapshots | - | 100/month | Unlimited |
| Multi-channel NVR | - | - | Yes |

Enforcement at API layer — ISC tokens only issued if tenant subscription covers the request.

---

## 5. Security

### 5.1 Credential Storage

- ISV AppKey/AppSecret: environment variables only, never in DB
- Device verification codes: encrypted in camera record (existing vault), cleared after successful bind
- ISC access tokens: Redis only, not persisted to disk
- Device-scoped SDK tokens: short-lived (5min), never stored client-side beyond session

### 5.2 Tenant Isolation

- All ISC devices bound under single MistyPass ISV account
- MistyPass enforces: camera record belongs to requesting tenant (existing `tenantID` check)
- A tenant cannot request cloud-token for another tenant's camera
- Admin cloud-bind verifies camera belongs to their tenant before ISC registration

### 5.3 Token Scoping

- ISC device access tokens are scoped to specific device serial
- MistyPass adds additional layer: only issue token if camera.tenant_id matches auth context
- Tokens passed to SDK client-side — SDK handles encryption of P2P channel

---

## 6. Migration Path

### Phase 1: Hik-Connect ISV (this spec)
- Single ISV account, admin-initiated binding
- Live P2P + cloud playback via ISC OpenSDK
- Hikvision cameras only

### Phase 2: App-Side QR Binding (future)
- User scans camera QR code in-app to bind
- Ownership transfer flow if device was on personal Hik-Connect account

### Phase 3: ISUP Self-Hosted (future, if needed)
- For customers requiring data sovereignty
- MistyPass runs its own video relay
- Eliminates dependency on Hik-Connect cloud

### Phase 4: Multi-Brand Cloud
- Dahua DMSS integration (same VideoStreamProvider pattern)
- Extend `cloud_provider` field for new vendors

---

## 7. Dependencies

| Dependency | Version | Purpose |
|---|---|---|
| ISC OpenSDK (HikVideo.framework) | 2.x | iOS P2P video playback |
| Redis | existing | Token and device list cache |
| ISC OpenAPI account | ISV partner | API access (apply via partner.hikvision.com) |

### ISV Account Setup (prerequisite)

1. Register at partner.hikvision.com (international) or open.ys7.com (China)
2. Create application → receive AppKey + AppSecret
3. Application type: "Server" (for backend API calls)
4. No per-device licensing fee — ISV access is free, customer pays Hik-Connect subscription

---

## 8. Testing Strategy

| Layer | Approach |
|---|---|
| `hikconnect/client.go` | Unit test with HTTP mock server (recorded ISC responses) |
| `hikconnect/service.go` | Unit test with mock client + mock camera store |
| API routes | Integration test with mock hikconnect service |
| iOS provider | Unit test with mock API responses |
| E2E | Manual: real ISC sandbox account + test camera (dev Hikvision device) |

ISC provides a sandbox environment for development testing. Real device required for P2P validation.
