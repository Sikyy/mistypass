package main

import (
	"encoding/json"
	"fmt"
	"os"

	httpx "github.com/mistypass/cloud/api/internal/http"
)

func main() {
	spec := httpx.BuildOpenAPISpec()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(spec); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}
}
