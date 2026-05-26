# Mobile Staging Manual Walkthrough

> Capability status: CONTRACT_READY

Date: 2026-05-25 WIB

## Status

Staging API is reachable, Android staging install smoke passed on a Xiaomi 15,
and the main app-facing API paths have been smoke tested from this machine.

Completed on 2026-05-25:

- `https://staging-api.mistyislet.com/healthz` returns `200`.
- Xiaomi 15 real device was detected by `adb` as `d766dd19` (`24129PN74C/dada`).
- `./gradlew app:installStaging` installed the staging APK after removing an older package signed with a different key.
- Staging app auth login returned `200` for `tenant.admin@sudirman.co`.
- `/api/v1/app/orgs` currently depends on org membership rows; the API now
  falls back to the authenticated user's `TenantID` when no membership row
  exists, so demo/staging users do not get stuck on an empty organization list.
- `/api/v1/app/orgs/tenant_demo_jakarta/places` returned two demo places.
- `/api/v1/app/places/building_demo_001/doors` returned six demo doors.
- `/api/v1/app/places/building_demo_001/reports/export` returned a PDF export URL.
- `/api/v1/app/cameras` returned an empty list; cloud recordings UI should show the empty state until staging has a camera with recordings.
- iOS staging scheme PR opened: [IOS-mistypass #12](https://github.com/Sikyy/IOS-mistypass/pull/12).
  `MistyisletPass-Staging` simulator tests passed on iPhone 17 Pro simulator
  with 180 tests and 0 failures after adding the platform-credential regression
  case.
- iOS real device was detected as `Siky的iPhone` (`iPhone18,1`, id
  `5AE18EEF-4212-5F2A-B362-11009B9043F1`).
- iOS true-device staging build/install/launch passed after setting the Debug
  app `APP_ENV` to `staging` and using `com.mistyislet.pass.staging.widget`
  for the Debug widget Bundle ID. The main app Bundle ID remains
  `com.mistyislet.pass`.
- iOS manual walkthrough confirmed places/doors are listed. Report export
  presents an exportable file URL; this mobile export path does not send email.
  Cloudflare email delivery is covered by the report schedule send path instead.
- iOS camera cloud recordings rendered successfully with the real Hikvision
  camera path visible in the app. A Xiaomi 15 Android BLE credential was
  incorrectly shown in the iOS pass list; [IOS-mistypass #12](https://github.com/Sikyy/IOS-mistypass/pull/12)
  now filters mobile credentials to the current iOS platform for profile, wallet,
  and credential-renewal checks, and the fixed build was installed on the real
  device.
- 2026-05-26 update: reopening the installed iOS staging app confirmed the
  Xiaomi 15 BLE pass no longer appears.

```bash
curl -sS -o /tmp/mistypass-staging-health.txt -w "%{http_code}" https://staging-api.mistyislet.com/healthz
# 200
```

Remaining manual device checks: trigger one unlock only after choosing a safe
staging door. Android FCM true push remains blocked on Firebase
`google-services.json`, Firebase service account JSON, and Mac mini FCM env.

## Mac Mini Staging API Deployment

Recommended topology:

```text
staging-api.mistyislet.com
  -> Cloudflare Tunnel
  -> Mac mini http://127.0.0.1:8080
  -> MistyPass API container
```

Use this default Compose flow when the Mac mini is dedicated to staging or does not already run another MistyPass Compose stack with the same container names/ports. Do not point `staging-api.mistyislet.com` at production unless staging app traffic is intentionally allowed to touch production data.

On the Mac mini:

```bash
cd /Users/siky/code/MistyPass
git pull --ff-only github main

cp deploy/env/macmini-staging.example.env .env.staging
# Before starting Compose, replace all change-me / replace-* values in .env.staging.
# Keep real Cloudflare tokens and other secrets only on the Mac mini.

docker compose --env-file .env.staging up -d --build
curl -i http://127.0.0.1:8080/healthz
```

`.env.staging` is ignored by `.gitignore`; keep real secrets only on the Mac mini.

### Staging Secrets

Do not reuse the placeholder values from `deploy/env/macmini-staging.example.env`.
Generate the base secrets on the Mac mini, then paste them into `.env.staging`:

```bash
printf 'POSTGRES_PASSWORD=%s\nREDIS_PASSWORD=%s\nJWT_SECRET=%s\nHRIS_VAULT_MASTER_KEY=%s\nGATEWAY_BOOTSTRAP_TOKEN=%s\n' \
  "$(openssl rand -hex 24)" \
  "$(openssl rand -hex 24)" \
  "$(openssl rand -hex 32)" \
  "$(openssl rand -hex 32)" \
  "$(openssl rand -hex 32)"
```

Use hex-only values here because `POSTGRES_PASSWORD` is interpolated into
`DATABASE_URL`; special characters such as `@`, `:`, `/`, `?`, `#`, and `&`
can break URL parsing if they are not encoded.

| Variable | What it controls | Rotation note |
|---|---|---|
| `POSTGRES_PASSWORD` | Docker Postgres password used by Postgres, pgbouncer, and the API database URL. | Rotating it requires updating the DB/pgbouncer/API configuration together. |
| `REDIS_PASSWORD` | Redis AUTH password used by Redis and the API. | Rotating it requires updating Redis and the API together. |
| `JWT_SECRET` | API JWT signing key for access/session tokens. | Keep stable; changing it invalidates active sessions/tokens. |
| `HRIS_VAULT_MASTER_KEY` | Root secret for HRIS/MFA vault encryption. | Keep stable; changing it can make existing encrypted secrets unreadable unless key rotation is configured through `HRIS_VAULT_MASTER_KEY_PREVIOUS`. |
| `GATEWAY_BOOTSTRAP_TOKEN` | Bootstrap token used when a new gateway registers with the API. | New gateway registration must use the current value; store it securely and do not put it on devices after provisioning. |

Store the generated values in a password manager or secure operations note.
The auto-update script pulls code and rebuilds containers, but it does not copy
the example env template over `.env.staging`, so existing Mac mini secrets stay
in place across future `git pull` updates.

### Android Push / Firebase

Android FCM uses the unified mobile device registration route:

```http
POST /api/v1/app/devices/register
```

Request body:

```json
{
  "fcm_token": "...",
  "device_id": "...",
  "device_model": "Xiaomi 15",
  "platform": "android"
}
```

Production/staging FCM enablement needs both sides configured:

1. Android repo: add Firebase `google-services.json` for package
   `com.mistyislet.app` at `app/google-services.json`, then build/install the
   staging APK. The Android code path already applies the Google Services plugin
   when the file exists, fetches an FCM token, and registers it through
   `/api/v1/app/devices/register`.
2. Mac mini API: put the Firebase service account JSON at
   `/Users/siky/code/MistyPass/secrets/firebase-service-account.json`.
3. In `/Users/siky/code/MistyPass/.env.staging`, set:

```env
FCM_ENABLED=true
FCM_PROJECT_ID=<firebase-project-id>
FCM_SERVICE_ACCOUNT_FILE=/run/secrets/mistypass/firebase-service-account.json
FCM_TIMEOUT=10s
```

`docker-compose.yml` mounts `./secrets` into the API container read-only at
`/run/secrets/mistypass`; `secrets/` is gitignored, so future git pulls do not
overwrite the service account file.

After the Xiaomi 15 logs in and registers its token, verify:

```bash
curl -sS "$API_BASE_URL/api/v1/mobile-push/provider-status?tenant_id=tenant_demo_jakarta" \
  -H "Authorization: Bearer $TOKEN"

curl -sS -X POST "$API_BASE_URL/api/v1/mobile-push/smoke" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"tenant_demo_jakarta","title":"MistyPass staging push","body":"FCM smoke from Mac mini"}'
```

2026-05-26 check: the current `https://staging-api.mistyislet.com` deployment
returned `404` for `/api/v1/mobile-push/provider-status`, so the Mac mini must
first pull/deploy the branch that contains `routes_mobile_push.go` before the
real Xiaomi 15 push smoke can run.

## Mac Mini Auto-Update

The staging guide now includes a one-shot updater that fetches `github/main`,
fast-forwards only, rebuilds the Compose stack when code changed, and checks
local health:

```bash
cd /Users/siky/code/MistyPass
REPO_DIR=/Users/siky/code/MistyPass \
ENV_FILE=/Users/siky/code/MistyPass/.env.staging \
./deploy/macmini/update-and-redeploy.zsh
```

Optional launchd schedule:

```bash
cp deploy/macmini/com.mistypass.staging-auto-update.plist.example ~/Library/LaunchAgents/com.mistypass.staging-auto-update.plist
launchctl unload ~/Library/LaunchAgents/com.mistypass.staging-auto-update.plist 2>/dev/null || true
launchctl load ~/Library/LaunchAgents/com.mistypass.staging-auto-update.plist
launchctl start com.mistypass.staging-auto-update
```

The example launchd job checks every 10 minutes and writes logs to
`/tmp/mistypass-staging-auto-update.log` and
`/tmp/mistypass-staging-auto-update.err`.

Cloudflare Zero Trust setup:

1. Create a Cloudflare Tunnel for the Mac mini.
2. Install and run `cloudflared` on the Mac mini using the token Cloudflare provides.
3. Add a public hostname:
   - Hostname: `staging-api.mistyislet.com`
   - Service: `http://localhost:8080`
4. Cloudflare should create the DNS record automatically. If creating it manually, use:
   - Type: `CNAME`
   - Name: `staging-api`
   - Target: `<tunnel-id>.cfargotunnel.com`
   - Proxy: on

External readiness check:

```bash
curl -i https://staging-api.mistyislet.com/healthz
```

Expected result: `200 OK`. After that, iOS/Android staging base URL is ready at:

```text
https://staging-api.mistyislet.com/api/v1
```

If the same Mac mini later runs both production and staging, add a dedicated staging Compose override first so container names, volumes, ports, Redis key prefix, NATS subject prefix, and databases are isolated.

## Minimum Inputs

- Staging API DNS reachable for `https://staging-api.mistyislet.com/api/v1`.
- Staging account with access to at least one place.
- At least one online door that is safe to unlock during the walkthrough.
- At least one report-exportable place.
- A staging camera with cloud recordings, or confirmation that the expected result is an empty recordings state.

## iOS Steps

1. Build/run with the `MistyisletPass-Staging` scheme on `iPhone 17 Pro`
   simulator or a real iPhone.
2. Log in with the staging account.
3. Open place doors and verify the list loads from `/app/places/{placeId}/doors`.
4. Perform one approved unlock and verify success/error feedback.
5. Open Admin reports, export PDF, and verify a downloadable/export response.
6. Open Cameras, select a camera, and verify cloud token plus recordings states.

If real-device signing fails, first confirm Xcode Settings -> Accounts has the
enterprise team logged in. The personal team cannot sign the current app because
the app declares NFC Tag Reading; the widget target also needs a provisioning
profile whose App Group entitlement matches `group.com.mistyislet.pass`.

## Android Steps

Android real-device install passed on Xiaomi 15. Continue with the installed
staging app:

1. Log in with `tenant.admin@sudirman.co / admin123`.
2. Open place doors and verify the list loads from `/app/places/{placeId}/doors`.
3. Perform one approved unlock and verify success/error feedback.
4. Open Admin export, export PDF, and verify response state.
5. Open Cameras and verify cloud token plus recordings. The iOS path has already
   shown the real Hikvision feed/recordings; Android can reuse the same enabled
   staging camera for parity.

## Recheck

```bash
cd /Users/siky/code/MistyPass
curl -sS -o /tmp/mistypass-staging-health.txt -w "%{http_code}" https://staging-api.mistyislet.com/healthz
./docs/testing/mobile-app-smoke.zsh
```
