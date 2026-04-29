package httpx

import (
	"fmt"
	"net/http"
	"strings"
)

func withDeprecatedEndpoint(replacementPaths ...string) func(http.Handler) http.Handler {
	replacements := make([]string, 0, len(replacementPaths))
	for _, replacementPath := range replacementPaths {
		replacementPath = strings.TrimSpace(replacementPath)
		if replacementPath == "" {
			continue
		}
		replacements = append(replacements, replacementPath)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Deprecation", "true")
			if len(replacements) > 0 {
				w.Header().Set("X-MistyPass-Replacement", strings.Join(replacements, ", "))
				for _, replacementPath := range replacements {
					w.Header().Add("Link", fmt.Sprintf("<%s>; rel=\"successor-version\"", replacementPath))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
