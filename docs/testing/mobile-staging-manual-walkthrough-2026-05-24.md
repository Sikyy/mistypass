# Mobile Staging Manual Walkthrough

> Capability status: BLOCKED_EXTERNAL

Date: 2026-05-24 22:36:44 WIB

## Status

Blocked before app login. The configured staging host does not resolve from this machine:

```bash
curl -sS -o /tmp/mistypass-staging-health.txt -w "%{http_code}" https://staging-api.mistyislet.com/healthz
# curl: (6) Could not resolve host: staging-api.mistyislet.com
```

Because DNS fails before authentication, the iOS/Android staging walkthrough for login, doors, unlock, report export, and camera cloud recordings cannot be completed yet.

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

cat > .env.staging <<'EOF'
APP_ENV=staging
TZ=Asia/Jakarta
DEFAULT_TIMEZONE=Asia/Jakarta
ENABLE_DEMO_USERS=true
POSTGRES_PASSWORD=replace-with-strong-staging-password
REDIS_PASSWORD=replace-with-strong-staging-password
JWT_SECRET=replace-with-fixed-random-long-secret
GATEWAY_BOOTSTRAP_TOKEN=replace-with-fixed-random-token
CORS_ORIGIN=https://staging-admin.mistyislet.com
EOF

# Optional but recommended for report/invitation/wallet email smoke:
cat deploy/env/cloudflare-email.example.env >> .env.staging
# Before starting Compose, replace CLOUDFLARE_ACCOUNT_ID, CLOUDFLARE_EMAIL_API_TOKEN,
# and WALLET_ALERT_EMAIL_RECEIVER_MAP in .env.staging with real Mac mini-only values.

docker compose --env-file .env.staging up -d --build
curl -i http://127.0.0.1:8080/healthz
```

`.env.staging` is ignored by `.gitignore`; keep real secrets only on the Mac mini.

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

1. Build/run with `APP_ENV=staging` on `iPhone 17 Pro` simulator or a real iPhone.
2. Log in with the staging account.
3. Open place doors and verify the list loads from `/app/places/{placeId}/doors`.
4. Perform one approved unlock and verify success/error feedback.
5. Open Admin reports, export PDF, and verify a downloadable/export response.
6. Open Cameras, select a camera, and verify cloud token plus recordings states.

## Android Steps

Android real-device install is intentionally deferred. When ready:

1. Ensure a device or emulator is attached.
2. Build/run staging, noting that current `staging` build type uses release signing.
3. Log in with the staging account.
4. Open place doors and verify the list loads from `/app/places/{placeId}/doors`.
5. Perform one approved unlock and verify success/error feedback.
6. Open Admin export, export PDF, and verify response state.
7. Open Cameras, select a camera, and verify cloud token plus recordings states.

## Recheck

```bash
cd /Users/siky/code/MistyPass
curl -sS -o /tmp/mistypass-staging-health.txt -w "%{http_code}" https://staging-api.mistyislet.com/healthz
./docs/testing/mobile-app-smoke.zsh
```
