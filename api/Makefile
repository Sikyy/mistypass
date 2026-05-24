.PHONY: openapi openapi-mobile mobile-route-constants mobile-route-constants-check openapi-swift openapi-kotlin

DOCS_DIR   := ../docs
SPEC_FULL  := $(DOCS_DIR)/openapi.json
SPEC_MOBILE:= $(DOCS_DIR)/openapi-mobile.json

# --- Extract ---

openapi: ## Dump full OpenAPI spec from Go source
	go run ./cmd/openapi-extract/ > $(SPEC_FULL)

openapi-mobile: openapi ## Filter to mobile /app/* endpoints only
	python3 cmd/openapi-extract/filter_mobile.py < $(SPEC_FULL) > $(SPEC_MOBILE)

# --- Mobile route constants ---

mobile-route-constants: openapi-mobile ## Generate typed Swift/Kotlin route constants from mobile OpenAPI
	python3 cmd/openapi-extract/generate_mobile_routes.py --spec $(SPEC_MOBILE)

mobile-route-constants-check: ## Verify generated mobile route constants are in sync
	python3 cmd/openapi-extract/generate_mobile_routes.py --spec $(SPEC_MOBILE) --check

# --- Codegen (requires: npm i -g @openapitools/openapi-generator-cli) ---

SWIFT_OUT  := ../generated/swift
KOTLIN_OUT := ../generated/kotlin

openapi-swift: openapi-mobile ## Generate Swift client from mobile spec
	openapi-generator-cli generate \
		-i $(SPEC_MOBILE) \
		-g swift6 \
		-o $(SWIFT_OUT) \
		--additional-properties=projectName=MistyPassAPI,responseAs=AsyncAwait

openapi-kotlin: openapi-mobile ## Generate Kotlin client from mobile spec
	openapi-generator-cli generate \
		-i $(SPEC_MOBILE) \
		-g kotlin \
		-o $(KOTLIN_OUT) \
		--additional-properties=library=jvm-retrofit2,serializationLibrary=kotlinx_serialization,groupId=com.mistyislet,artifactId=api-client
