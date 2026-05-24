#!/usr/bin/env zsh
set -euo pipefail

# Mobile app smoke checks for the three-platform integration work.
#
# Defaults assume the standard local checkout layout:
#   /Users/siky/code/MistyPass
#   /Users/siky/code/ios-MistyisletPass
#   /Users/siky/code/android-MistyisletPass
#
# Notes:
#   - Do not pass CODE_SIGNING_ALLOWED=NO to the iOS simulator test. The
#     Keychain tests require the simulator "Sign to Run Locally" entitlement.
#   - Android device install is opt-in because CI and local machines often have
#     no attached emulator/device. Set RUN_ANDROID_DEVICE=1 to install.

SCRIPT_DIR="${0:A:h}"

IOS_REPO="${IOS_REPO:-/Users/siky/code/ios-MistyisletPass}"
IOS_PROJECT="${IOS_PROJECT:-MistyisletPass.xcodeproj}"
IOS_SCHEME="${IOS_SCHEME:-MistyisletPass}"
IOS_DESTINATION="${IOS_DESTINATION:-platform=iOS Simulator,name=iPhone 17 Pro}"
IOS_DERIVED_DATA="${IOS_DERIVED_DATA:-/tmp/ios-MistyisletPass-SmokeDerivedData}"

ANDROID_REPO="${ANDROID_REPO:-/Users/siky/code/android-MistyisletPass}"
API_PORT="${API_PORT:-8080}"
RUN_ANDROID_DEVICE="${RUN_ANDROID_DEVICE:-0}"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m'

pass_count=0
fail_count=0
skip_count=0

function pass() {
  echo "${GREEN}PASS${NC} $1"
  ((++pass_count))
}

function fail() {
  echo "${RED}FAIL${NC} $1"
  ((++fail_count))
}

function skip() {
  echo "${YELLOW}SKIP${NC} $1"
  ((++skip_count))
}

function header() {
  echo ""
  echo "${YELLOW}== $1 ==${NC}"
}

header "Mobile route contract drift"

if "${SCRIPT_DIR}/check-mobile-app-route-drift.zsh"; then
  pass "iOS/Android route literals match mobile OpenAPI"
else
  fail "iOS/Android route literals match mobile OpenAPI"
fi

header "iOS simulator tests"

if [[ ! -d "${IOS_REPO}" ]]; then
  skip "iOS repo not found at ${IOS_REPO}"
elif ! command -v xcodebuild >/dev/null 2>&1; then
  skip "xcodebuild not found"
else
  echo "Repo: ${IOS_REPO}"
  echo "Destination: ${IOS_DESTINATION}"
  if (
    cd "${IOS_REPO}"
    xcodebuild test \
      -project "${IOS_PROJECT}" \
      -scheme "${IOS_SCHEME}" \
      -destination "${IOS_DESTINATION}" \
      -derivedDataPath "${IOS_DERIVED_DATA}"
  ); then
    pass "iOS simulator test suite"
  else
    fail "iOS simulator test suite"
  fi
fi

header "Android JVM/build smoke"

if [[ ! -x "${ANDROID_REPO}/gradlew" ]]; then
  skip "Android gradlew not found at ${ANDROID_REPO}/gradlew"
else
  echo "Repo: ${ANDROID_REPO}"
  if (
    cd "${ANDROID_REPO}"
    ./gradlew testDebugUnitTest assembleDebug
  ); then
    pass "Android debug unit tests and APK build"
  else
    fail "Android debug unit tests and APK build"
  fi
fi

header "Android optional device install"

if [[ "${RUN_ANDROID_DEVICE}" != "1" ]]; then
  skip "Set RUN_ANDROID_DEVICE=1 to run adb reverse and installDebug"
elif ! command -v adb >/dev/null 2>&1; then
  skip "adb not found"
else
  device_count="$(adb devices | awk 'NR > 1 && $2 == "device" { count++ } END { print count + 0 }')"
  if [[ "${device_count}" == "0" ]]; then
    skip "No attached Android emulator/device"
  elif (
    adb reverse "tcp:${API_PORT}" "tcp:${API_PORT}"
    cd "${ANDROID_REPO}"
    ./gradlew installDebug
  ); then
    pass "Android adb reverse and installDebug"
  else
    fail "Android adb reverse and installDebug"
  fi
fi

header "Summary"

echo "Passed: ${pass_count}"
echo "Skipped: ${skip_count}"
echo "Failed: ${fail_count}"

if [[ "${fail_count}" -gt 0 ]]; then
  exit 1
fi
