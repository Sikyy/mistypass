package pdfgen

import (
	"strings"
	"testing"
)

func TestRenderBadgesHTML(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	doc := BadgeDoc{
		Organization: "Acme Jakarta",
		Badges: []Badge{
			{Name: "Andri Pratama", Role: "operator", Building: "HQ Tower", Status: "active", QRBase64: "AAAA"},
			{Name: "Siky", Role: "tenant_admin", Building: "HQ Tower", Status: "active", QRBase64: "BBBB"},
			{Name: "Rina", Role: "operator", Building: "HQ Tower", Status: "suspended", QRBase64: "CCCC"},
		},
	}
	out, err := r.RenderBadgesHTML(doc)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{"Andri Pratama", "Siky", "Rina", "tenant_admin", "suspended", "Acme Jakarta", "Scan to verify"} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected html to contain %q", want)
		}
	}
	if n := strings.Count(html, "data:image/png;base64,BBBB"); n != 1 {
		t.Fatalf("expected each badge QR embedded once, got %d for BBBB", n)
	}
	if n := strings.Count(html, `class="badge"`); n != 3 {
		t.Fatalf("expected 3 badge cards, got %d", n)
	}
}
