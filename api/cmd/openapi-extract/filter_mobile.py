#!/usr/bin/env python3
"""Filter the full OpenAPI spec down to mobile app endpoints (/app/*)."""
import json
import sys

spec = json.load(sys.stdin)

mobile_paths = {k: v for k, v in spec.get("paths", {}).items() if "/app/" in k}

mobile_tags = set()
for path_item in mobile_paths.values():
    for op in path_item.values():
        if isinstance(op, dict):
            mobile_tags.update(op.get("tags", []))

mobile_spec = {
    "openapi": spec["openapi"],
    "info": {
        "title": "MistyPass Mobile API",
        "version": spec["info"]["version"],
        "description": "Mobile app endpoints extracted from the Mistyislet API.",
    },
    "servers": [{"url": "https://api.mistyislet.com"}],
    "tags": [t for t in spec.get("tags", []) if t.get("name") in mobile_tags],
    "paths": mobile_paths,
    "components": spec.get("components", {}),
}

json.dump(mobile_spec, sys.stdout, indent=2)
print()
