package httpx

// BuildOpenAPISpec returns the static OpenAPI 3.0.3 specification as a
// serialisable map. Exported so cmd/openapi-extract can dump it to a file
// without running the full server.
func BuildOpenAPISpec() map[string]any {
	return buildOpenAPISpec()
}
