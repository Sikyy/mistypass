#!/usr/bin/env zsh
set -euo pipefail

# Compare generated mobile route constants and mobile app route literals against
# docs/openapi-mobile.json.
#
# This catches path drift while the apps migrate from hand-written route strings
# to the generated typed constants under docs/generated/mobile-routes.

SCRIPT_DIR="${0:A:h}"
REPO_ROOT="${SCRIPT_DIR:h:h}"

SPEC_MOBILE="${SPEC_MOBILE:-${REPO_ROOT}/docs/openapi-mobile.json}"
ROUTE_GENERATOR="${ROUTE_GENERATOR:-${REPO_ROOT}/api/cmd/openapi-extract/generate_mobile_routes.py}"
ROUTE_CONTRACT="${ROUTE_CONTRACT:-${REPO_ROOT}/docs/generated/mobile-routes/mobile-route-contract.json}"
ROUTE_SWIFT="${ROUTE_SWIFT:-${REPO_ROOT}/docs/generated/mobile-routes/MobileAPIRoutes.generated.swift}"
ROUTE_KOTLIN="${ROUTE_KOTLIN:-${REPO_ROOT}/docs/generated/mobile-routes/MobileApiRoutes.generated.kt}"
IOS_REPO="${IOS_REPO:-/Users/siky/code/ios-MistyisletPass}"
ANDROID_REPO="${ANDROID_REPO:-/Users/siky/code/android-MistyisletPass}"

python3 "${ROUTE_GENERATOR}" \
  --spec "${SPEC_MOBILE}" \
  --swift-out "${ROUTE_SWIFT}" \
  --kotlin-out "${ROUTE_KOTLIN}" \
  --contract-out "${ROUTE_CONTRACT}" \
  --check

python3 - "${ROUTE_CONTRACT}" "${IOS_REPO}" "${ANDROID_REPO}" <<'PY'
import json
import re
import sys
from pathlib import Path

contract_path = Path(sys.argv[1])
ios_repo = Path(sys.argv[2])
android_repo = Path(sys.argv[3])

if not contract_path.exists():
    print(f"FAIL: generated mobile route contract not found: {contract_path}")
    sys.exit(1)

with contract_path.open("r", encoding="utf-8") as handle:
    contract = json.load(handle)


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


contract_paths = {normalize_path(route["path"]) for route in contract.get("operations", [])}
contract_paths.discard("")
contract_operations = {
    (route["method"].upper(), normalize_path(route["path"]))
    for route in contract.get("operations", [])
}
contract_operations.discard(("", ""))

STRING_LITERAL = re.compile(r'"(?:\\.|[^"\\])*"')
RETROFIT_ANNOTATION = re.compile(r"@(GET|POST|PUT|PATCH|DELETE)\(\"((?:\\.|[^\"\\])*)\"\)")
SWIFT_API_CONSTANT = re.compile(
    r"static\s+(?:let|var)\s+([A-Za-z0-9_]+)\s*(?::[^=]+)?=\s*\"((?:\\.|[^\"\\])*)\""
)
SWIFT_API_FUNCTION = re.compile(
    r"static\s+func\s+([A-Za-z0-9_]+)\([^)]*\)\s*->\s*String\s*\{\s*\"((?:\\.|[^\"\\])*)\"",
    re.MULTILINE | re.DOTALL,
)
SWIFT_API_CALL_START = re.compile(r"\b(get|post|put|patch|delete)\s*\(")
SWIFT_API_CALL_PATH = re.compile(r"path:\s*Constants\.API\.([A-Za-z0-9_]+)")


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


def line_number(text: str, offset: int) -> int:
    return text.count("\n", 0, offset) + 1


def scan_ios_api_methods(repo: Path) -> list[tuple[str, int, str]]:
    constants_path = repo / "MistyisletPass" / "Utilities" / "Constants.swift"
    service_path = repo / "MistyisletPass" / "Services" / "APIService.swift"
    if not constants_path.exists() or not service_path.exists():
        return []

    constants_text = constants_path.read_text(encoding="utf-8", errors="ignore")
    route_constants: dict[str, str] = {}
    for match in SWIFT_API_CONSTANT.finditer(constants_text):
        route_constants[match.group(1)] = normalize_path(match.group(2))
    for match in SWIFT_API_FUNCTION.finditer(constants_text):
        route_constants[match.group(1)] = normalize_path(match.group(2))

    issues = []
    service_text = service_path.read_text(encoding="utf-8", errors="ignore")
    call_starts = list(SWIFT_API_CALL_START.finditer(service_text))
    for index, match in enumerate(call_starts):
        method = match.group(1).upper()
        end = call_starts[index + 1].start() if index + 1 < len(call_starts) else len(service_text)
        snippet = service_text[match.start():min(end, match.start() + 500)]
        path_match = SWIFT_API_CALL_PATH.search(snippet)
        if not path_match:
            continue
        constant_name = path_match.group(1)
        candidate = route_constants.get(constant_name, "")
        if candidate and (method, candidate) not in contract_operations:
            issues.append(
                (
                    str(service_path),
                    line_number(service_text, match.start()),
                    f"{method} {candidate} via Constants.API.{constant_name}",
                )
            )
    return issues


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
    issues.extend(scan_ios_api_methods(repo))
    return issues


def scan_android(repo: Path) -> list[tuple[str, int, str]]:
    root = repo / "app" / "src" / "main" / "java"
    if not root.exists():
        print(f"SKIP: Android repo not found at {repo}")
        return []
    issues = []
    for path in sorted(root.rglob("*.kt")):
        text = path.read_text(encoding="utf-8", errors="ignore")
        for line_no, line in enumerate(text.splitlines(), start=1):
            for match in RETROFIT_ANNOTATION.finditer(line):
                method = match.group(1).upper()
                candidate = normalize_path(match.group(2))
                if candidate and (method, candidate) not in contract_operations:
                    issues.append((str(path), line_no, f"{method} {candidate}"))
            if "/app/" not in line and "app/" not in line:
                continue
            for value in string_literals(line):
                candidate = normalize_path(value)
                if not candidate:
                    continue
                if candidate not in contract_paths:
                    issues.append((str(path), line_no, candidate))
    return issues


issues = []
issues.extend(("iOS", *item) for item in scan_ios(ios_repo))
issues.extend(("Android", *item) for item in scan_android(android_repo))

if issues:
    print("FAIL: mobile app route literals missing from generated mobile route contract")
    for platform, file_path, line_no, candidate in issues:
        print(f"{platform}: {file_path}:{line_no}: {candidate}")
    sys.exit(1)

print("PASS: generated mobile route constants and app route literals match docs/openapi-mobile.json")
PY
