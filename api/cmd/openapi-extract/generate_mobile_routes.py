#!/usr/bin/env python3
"""Generate typed mobile route constants from docs/openapi-mobile.json."""

from __future__ import annotations

import argparse
import json
import re
import sys
from dataclasses import dataclass
from pathlib import Path


DEFAULT_OUTPUT_DIR = Path("../docs/generated/mobile-routes")
HTTP_METHODS = {"delete", "get", "patch", "post", "put"}
SWIFT_KEYWORDS = {
    "associatedtype",
    "class",
    "deinit",
    "enum",
    "extension",
    "fileprivate",
    "func",
    "import",
    "init",
    "inout",
    "internal",
    "let",
    "open",
    "operator",
    "private",
    "protocol",
    "public",
    "static",
    "struct",
    "subscript",
    "typealias",
    "var",
}
KOTLIN_KEYWORDS = {
    "as",
    "break",
    "class",
    "continue",
    "do",
    "else",
    "false",
    "for",
    "fun",
    "if",
    "in",
    "interface",
    "is",
    "null",
    "object",
    "package",
    "return",
    "super",
    "this",
    "throw",
    "true",
    "try",
    "typealias",
    "typeof",
    "val",
    "var",
    "when",
    "while",
}


@dataclass(frozen=True)
class Operation:
    method: str
    operation_id: str
    api_path: str
    app_path: str
    retrofit_path: str
    parameters: tuple[str, ...]
    swift_name: str
    kotlin_name: str


def lower_camel(value: str) -> str:
    parts = re.split(r"[^A-Za-z0-9]+", value)
    parts = [part for part in parts if part]
    if not parts:
        return "route"
    first, rest = parts[0], parts[1:]
    result = first[:1].lower() + first[1:] + "".join(part[:1].upper() + part[1:] for part in rest)
    if not re.match(r"[A-Za-z_]", result):
        result = "route" + result[:1].upper() + result[1:]
    return result


def identifier(value: str, keywords: set[str]) -> str:
    value = lower_camel(value)
    value = re.sub(r"[^A-Za-z0-9_]", "_", value)
    if value in keywords:
        value += "Value"
    return value


def fallback_operation_id(method: str, path: str) -> str:
    clean = path.removeprefix("/api/v1").strip("/")
    pieces = [method.lower()]
    pieces.extend(part for part in re.split(r"[^A-Za-z0-9]+", clean) if part)
    return lower_camel("_".join(pieces))


def app_path(api_path: str) -> str:
    path = api_path.removeprefix("/api/v1")
    if not path.startswith("/"):
        path = "/" + path
    return path


def params_for(path: str) -> tuple[str, ...]:
    seen: list[str] = []
    for param in re.findall(r"\{([^}/]+)\}", path):
        if param not in seen:
            seen.append(param)
    return tuple(seen)


def load_operations(spec_path: Path) -> list[Operation]:
    with spec_path.open("r", encoding="utf-8") as handle:
        spec = json.load(handle)

    operations: list[Operation] = []
    seen_operation_ids: set[str] = set()

    for api_path, path_item in sorted(spec.get("paths", {}).items()):
        if not isinstance(path_item, dict) or "/app/" not in api_path:
            continue
        for method, operation in sorted(path_item.items()):
            if method.lower() not in HTTP_METHODS or not isinstance(operation, dict):
                continue
            operation_id = operation.get("operationId") or fallback_operation_id(method, api_path)
            if operation_id in seen_operation_ids:
                raise ValueError(f"duplicate operationId: {operation_id}")
            seen_operation_ids.add(operation_id)

            normalized_app_path = app_path(api_path)
            operations.append(
                Operation(
                    method=method.upper(),
                    operation_id=operation_id,
                    api_path=api_path,
                    app_path=normalized_app_path,
                    retrofit_path=normalized_app_path.removeprefix("/"),
                    parameters=params_for(normalized_app_path),
                    swift_name=identifier(operation_id, SWIFT_KEYWORDS),
                    kotlin_name=identifier(operation_id, KOTLIN_KEYWORDS),
                )
            )

    if not operations:
        raise ValueError(f"no /app operations found in {spec_path}")

    return operations


def swift_path_expression(path: str, parameters: tuple[str, ...]) -> str:
    expression = path
    for param in parameters:
        expression = expression.replace("{" + param + "}", f"\\({identifier(param, SWIFT_KEYWORDS)})")
    return '"' + expression.replace('"', '\\"') + '"'


def kotlin_path_expression(path: str, parameters: tuple[str, ...]) -> str:
    expression = path
    for param in parameters:
        expression = expression.replace("{" + param + "}", "${" + identifier(param, KOTLIN_KEYWORDS) + "}")
    return '"' + expression.replace('"', '\\"') + '"'


def swift_param_list(parameters: tuple[str, ...]) -> str:
    return ", ".join(f"{identifier(param, SWIFT_KEYWORDS)}: String" for param in parameters)


def kotlin_param_list(parameters: tuple[str, ...]) -> str:
    return ", ".join(f"{identifier(param, KOTLIN_KEYWORDS)}: String" for param in parameters)


def generate_swift(operations: list[Operation]) -> str:
    lines = [
        "// Code generated by api/cmd/openapi-extract/generate_mobile_routes.py. DO NOT EDIT.",
        "import Foundation",
        "",
        "enum MobileAPIHTTPMethod: String, Codable, Equatable {",
        '    case delete = "DELETE"',
        '    case get = "GET"',
        '    case patch = "PATCH"',
        '    case post = "POST"',
        '    case put = "PUT"',
        "}",
        "",
        "struct MobileAPIRoute: Codable, Equatable {",
        "    let method: MobileAPIHTTPMethod",
        "    let path: String",
        "}",
        "",
        "enum MobileAPIRoutes {",
    ]

    for op in operations:
        method_case = op.method.lower()
        if op.parameters:
            lines.append(f"    static func {op.swift_name}({swift_param_list(op.parameters)}) -> MobileAPIRoute {{")
            lines.append(f"        MobileAPIRoute(method: .{method_case}, path: {swift_path_expression(op.app_path, op.parameters)})")
            lines.append("    }")
        else:
            lines.append(
                f"    static let {op.swift_name} = MobileAPIRoute(method: .{method_case}, path: \"{op.app_path}\")"
            )
    lines.append("}")
    lines.append("")
    return "\n".join(lines)


def generate_kotlin(operations: list[Operation]) -> str:
    lines = [
        "// Code generated by api/cmd/openapi-extract/generate_mobile_routes.py. DO NOT EDIT.",
        "package com.mistyislet.app.data.api",
        "",
        "enum class MobileApiHttpMethod {",
        "    DELETE,",
        "    GET,",
        "    PATCH,",
        "    POST,",
        "    PUT,",
        "}",
        "",
        "data class MobileApiRoute(",
        "    val method: MobileApiHttpMethod,",
        "    val path: String,",
        ")",
        "",
        "object MobileApiRoutes {",
    ]

    for op in operations:
        path_const = f"{op.kotlin_name}Path"
        retrofit_const = f"{op.kotlin_name}RetrofitPath"
        lines.append(f"    const val {path_const}: String = \"{op.app_path}\"")
        lines.append(f"    const val {retrofit_const}: String = \"{op.retrofit_path}\"")
        if op.parameters:
            lines.append(f"    fun {op.kotlin_name}({kotlin_param_list(op.parameters)}): MobileApiRoute =")
            lines.append(
                f"        MobileApiRoute(MobileApiHttpMethod.{op.method}, {kotlin_path_expression(op.app_path, op.parameters)})"
            )
        else:
            lines.append(
                f"    val {op.kotlin_name}: MobileApiRoute = MobileApiRoute(MobileApiHttpMethod.{op.method}, {path_const})"
            )
    lines.append("}")
    lines.append("")
    return "\n".join(lines)


def generate_contract(operations: list[Operation]) -> str:
    payload = {
        "generatedBy": "api/cmd/openapi-extract/generate_mobile_routes.py",
        "operationCount": len(operations),
        "operations": [
            {
                "operationId": op.operation_id,
                "method": op.method,
                "apiPath": op.api_path,
                "path": op.app_path,
                "retrofitPath": op.retrofit_path,
                "parameters": list(op.parameters),
                "swiftName": op.swift_name,
                "kotlinName": op.kotlin_name,
            }
            for op in operations
        ],
    }
    return json.dumps(payload, indent=2) + "\n"


def write_or_check(path: Path, content: str, check: bool) -> bool:
    if check:
        if not path.exists():
            print(f"FAIL: generated file is missing: {path}")
            return False
        current = path.read_text(encoding="utf-8")
        if current != content:
            print(f"FAIL: generated file is stale: {path}")
            return False
        return True

    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    print(f"wrote {path}")
    return True


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--spec", type=Path, default=Path("../docs/openapi-mobile.json"))
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT_DIR)
    parser.add_argument("--swift-out", type=Path)
    parser.add_argument("--kotlin-out", type=Path)
    parser.add_argument("--contract-out", type=Path)
    parser.add_argument("--check", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    operations = load_operations(args.spec)
    output_dir = args.output_dir
    swift_out = args.swift_out or output_dir / "MobileAPIRoutes.generated.swift"
    kotlin_out = args.kotlin_out or output_dir / "MobileApiRoutes.generated.kt"
    contract_out = args.contract_out or output_dir / "mobile-route-contract.json"

    results = [
        write_or_check(swift_out, generate_swift(operations), args.check),
        write_or_check(kotlin_out, generate_kotlin(operations), args.check),
        write_or_check(contract_out, generate_contract(operations), args.check),
    ]

    if not all(results):
        print("Run `cd api && make mobile-route-constants` to refresh generated mobile route constants.")
        return 1

    if args.check:
        print(f"PASS: {len(operations)} generated mobile routes are up to date")
    return 0


if __name__ == "__main__":
    sys.exit(main())
