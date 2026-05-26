# Mobile UI Design Alignment

> Capability status: CONTRACT_READY

Date: 2026-05-26 WIB

## Source Of Truth

Use the iOS app as the interaction and information architecture reference, then
apply the visual language from `web-mistypass`:

- Website palette from `/Users/siky/code/web-mistypass/tailwind.config.ts`.
- Website tone: calm, hardware-aware, operational, and restrained.
- iOS app pattern: native navigation, clear cards, hold-to-unlock interaction,
  platform credential filtering, and concise status language.

## Brand Tokens

| Token | Hex | Usage |
|---|---:|---|
| `obsidian` | `#070806` | dark app background, premium surfaces |
| `graphite` | `#141510` | dark panels and raised surfaces |
| `mist` | `#F5F0E6` | warm light background and dark-mode text |
| `smoke` | `#BEB8AA` | secondary text and dividers |
| `teal` | `#62B7A8` | primary action, focus, selected state |
| `brass` | `#C9A25B` | warning, pending, and caution accents |
| `moss` | `#7F9B6B` | success and online status |
| `copper` | `#A96E42` | secondary warning / physical hardware accent |

Avoid reintroducing the previous blue-purple primary color (`#4F55FF`) as a
main app accent. It can remain only in historical screenshots or legacy docs.

## Completed

- iOS `BrandPrimary` asset now uses website teal `#62B7A8`.
- Android Compose theme now exposes website-aligned tokens:
  `Obsidian`, `Graphite`, `Mist`, `Smoke`, `Brass`, `Copper`, `Teal`, and `Moss`.
- Android light and dark Material color schemes now use teal as primary, warm
  mist backgrounds, and brass/moss status colors.
- Android first layout parity pass started:
  - Login/auth steps now follow the iOS left-aligned step layout while keeping
    the website dark mist background and subtle noise.
  - Door cards now follow the iOS information order: status icon, door name,
    BLE indicator, save state, status tags, location, then hold-to-unlock.
  - Dashboard rows now use website-aligned semantic accents instead of the
    previous blue/purple/orange placeholder palette.
- Verification:
  - Android: `./gradlew app:assembleStaging` passed after the first layout
    parity pass, then the staging APK was installed on Xiaomi 15 for visual QA.
  - iOS: `xcodebuild -scheme MistyisletPass-Staging -destination 'platform=iOS Simulator,name=iPhone 17 Pro' build` passed.

## Android 17 Readiness

Official references:

- Android 17 overview: <https://developer.android.com/about/versions/17>
- Android 17 features and APIs: <https://developer.android.com/about/versions/17/features>
- Android 17 behavior changes: <https://developer.android.com/about/versions/17/behavior-changes-17>

Current Android app baseline:

- `compileSdk = 35`
- `targetSdk = 35`
- `minSdk = 26`

Recommendation:

- Do not move production/staging to API 37 immediately. First finish UI parity,
  Firebase push smoke, and hardware-facing staging tests on the current SDK.
- Open a separate Android 17 readiness branch once CI and local SDK images have
  API 37 available.

Useful Android 17 items for MistyPass:

- Local network permission: relevant when the app directly provisions or talks
  to local gateways, cameras, or readers on LAN. Today the app mostly uses cloud
  API plus BLE, so this is P1 for gateway setup and local diagnostics, not a P0
  UI dependency.
- Adaptive/resizable app behavior: useful for tablets, foldables, desktop mode,
  and large Android devices. The current iOS-layout parity pass should avoid
  phone-only assumptions and keep list/detail surfaces ready for window-size
  adaptation.
- Contact picker / privacy-preserving contact selection: useful later for
  visitor invites and access sharing without requesting broad contacts access.
- Bluetooth compatibility: current unlock path uses BLE GATT/Nordic rather than
  classic RFCOMM sockets. Android 17 Bluetooth behavior changes should be
  tracked in compatibility testing, but no immediate code change is required.
- TLS/network behavior: staging and production API calls should be smoke-tested
  after target SDK uplift, but no app UI decision depends on it.

## Screen Priority

P0:

- Login/auth: align Android with iOS step layout and website tone.
- Orgs, places, doors: make hierarchy, cards, and status badges consistent.
- Door card and hold-to-unlock: keep iOS as the interaction template; Android
  should match the same status ordering, copy weight, and hold-progress affordance.
- Wallet/credentials: ensure only the current platform credential appears and
  the visual treatment matches iOS pass cards.
- History/camera recordings: match event grouping, recording thumbnail state,
  and empty/loading/error states.

P1:

- Admin mobile screens: use the same tokens but keep denser operational layouts.
- Visitor flows: align sheet rhythm, inputs, and QR pass presentation.
- Profile/settings: make account, device, NFC, BLE, and language sections match
  iOS grouping.

## Acceptance Criteria

- iOS and Android use the same primary accent (`#62B7A8`) and status color
  semantics.
- Android high-frequency screens should feel like the same product as iOS,
  while still respecting Material navigation and system controls.
- Website language is visible through color, restraint, and hardware/access
  terminology, not through marketing-style hero content inside the app.
- No screen should rely on demo-only labels for real staging data; demo/real
  states must be visually distinguishable once hardware testing resumes.

## Next Implementation Steps

1. Android P0 screen pass: finish org/place selection, doors, credentials, and
   camera recordings against the iOS interaction hierarchy.
2. Android 17 readiness branch: test API 37 SDK/toolchain, add local network
   permission only when a real LAN gateway/camera provisioning flow needs it,
   and verify large-screen behavior.
3. iOS polish pass: check any remaining purple-derived assumptions after the
   token change and adjust spacing only where Android parity needs a clearer
   reference.
4. Visual QA: reinstall Android staging on Xiaomi 15 and iOS staging on the
   iPhone, capture screenshots for the five P0 flows, then compare against the
   website palette and iOS baseline.
5. After hardware is ready, repeat the same P0 flow with `Roaming-Test` so UI
   polish is validated against real door, camera, and gateway states.
