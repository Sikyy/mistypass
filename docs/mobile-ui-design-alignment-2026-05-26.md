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
- Verification:
  - Android: `./gradlew app:assembleStaging` passed.
  - iOS: `xcodebuild -scheme MistyisletPass-Staging -destination 'platform=iOS Simulator,name=iPhone 17 Pro' build` passed.

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

1. Android P0 screen pass: login, org/place selection, doors, credentials, and
   camera recordings.
2. iOS polish pass: check any remaining purple-derived assumptions after the
   token change and adjust spacing only where Android parity needs a clearer
   reference.
3. Visual QA: reinstall Android staging on Xiaomi 15 and iOS staging on the
   iPhone, capture screenshots for the five P0 flows, then compare against the
   website palette and iOS baseline.
4. After hardware is ready, repeat the same P0 flow with `Roaming-Test` so UI
   polish is validated against real door, camera, and gateway states.
