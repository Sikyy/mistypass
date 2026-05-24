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
