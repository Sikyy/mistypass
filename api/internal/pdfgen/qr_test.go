package pdfgen

import (
	"encoding/base64"
	"testing"
)

func TestEncodeQRPNGBase64(t *testing.T) {
	out, err := EncodeQRPNGBase64("https://example.com/api/v1/badges/verify?token=abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(out)
	if err != nil {
		t.Fatalf("output is not valid base64: %v", err)
	}
	if len(raw) < 8 || string(raw[1:4]) != "PNG" {
		t.Fatalf("expected PNG bytes, got %d bytes", len(raw))
	}
}

func TestEncodeQRPNGBase64RejectsEmpty(t *testing.T) {
	if _, err := EncodeQRPNGBase64("   "); err == nil {
		t.Fatal("expected error for empty content")
	}
}
