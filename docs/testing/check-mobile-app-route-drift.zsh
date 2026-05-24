#!/usr/bin/env zsh
set -euo pipefail

# Compare mobile app route literals against docs/openapi-mobile.json.
#
# This is intentionally lightweight: it scans production Swift/Kotlin string
# literals for /app/* route shapes and checks that each shape exists in the
# mobile OpenAPI contract. It catches path drift while the full generated-client
# work is still pending.

SCRIPT_DIR="${0:A:h}"
REPO_ROOT="${SCRIPT_DIR:h:h}"

SPEC_MOBILE="${SPEC_MOBILE:-${REPO_ROOT}/docs/openapi-mobile.json}"
IOS_REPO="${IOS_REPO:-/Users/siky/code/ios-MistyisletPass}"
ANDROID_REPO="${ANDROID_REPO:-/Users/siky/code/android-MistyisletPass}"

python3 - "${SPEC_MOBILE}" "${IOS_REPO}" "${ANDROID_REPO}" <<'PY'
import json
import re
import sys
from pathlib import Path

spec_path = Path(sys.argv[1])
ios_repo = Path(sys.argv[2])
android_repo = Path(sys.argv[3])

if not spec_path.exists():
    print(f"FAIL: mobile OpenAPI spec not found: {spec_path}")
    sys.exit(1)

with spec_path.open("r", encoding="utf-8") as handle:
    spec = json.load(handle)


def normalize_path(path: str) -> str:
    if not path:
        return ""

    if "/app/" not in path and not path.startswith("app/"):
        return ""

    start = path.find("/app/")
    if start == -1:
        start = path.find("app/")
    path = path[start:]

    if not path.startswith("/"):
        path = "/" + path

    path = path.split("?", 1)[0].split("#", 1)[0]
    path = re.sub(r"\\\([^)]+\)", "{}", path)
    path = re.sub(r"\$\{[^}]+\}", "{}", path)
    path = re.sub(r"\{[^}/]+\}", "{}", path)
    path = re.sub(r"/+", "/", path)
    if len(path) > 1:
        path = path.rstrip("/")
    return path


contract_paths = {
    normalize_path(route.removeprefix("/api/v1"))
    for route in spec.get("paths", {})
}
contract_paths.discard("")

STRING_LITERAL = re.compile(r'"(?:\\.|[^"\\])*"')


def string_literals(text: str) -> list[str]:
    values = []
    for match in STRING_LITERAL.finditer(text):
        raw = match.group(0)[1:-1]
        values.append(raw)
    return values


def scan_file(path: Path) -> list[tuple[int, str]]:
    findings = []
    text = path.read_text(encoding="utf-8", errors="ignore")
    lines = text.splitlines()

    for line_no, line in enumerate(lines, start=1):
        if "/app/" not in line and "app/" not in line:
            continue
        for value in string_literals(line):
            candidate = normalize_path(value)
            if candidate:
                findings.append((line_no, candidate))
    return findings


def scan_ios(repo: Path) -> list[tuple[str, int, str]]:
    root = repo / "MistyisletPass"
    if not root.exists():
        print(f"SKIP: iOS repo not found at {repo}")
        return []
    issues = []
    for path in sorted(root.rglob("*.swift")):
        for line_no, candidate in scan_file(path):
            if candidate not in contract_paths:
                issues.append((str(path), line_no, candidate))
    return issues


def scan_android(repo: Path) -> list[tuple[str, int, str]]:
    root = repo / "app" / "src" / "main" / "java"
    if not root.exists():
        print(f"SKIP: Android repo not found at {repo}")
        return []
    issues = []
    for path in sorted(root.rglob("*.kt")):
        for line_no, candidate in scan_file(path):
            if candidate not in contract_paths:
                issues.append((str(path), line_no, candidate))
    return issues


issues = []
issues.extend(("iOS", *item) for item in scan_ios(ios_repo))
issues.extend(("Android", *item) for item in scan_android(android_repo))

if issues:
    print("FAIL: mobile app route literals missing from docs/openapi-mobile.json")
    for platform, file_path, line_no, candidate in issues:
        print(f"{platform}: {file_path}:{line_no}: {candidate}")
    sys.exit(1)

print("PASS: mobile app route literals match docs/openapi-mobile.json")
PY
