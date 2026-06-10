package pdfgen

import (
	"bytes"
	"fmt"
)

// Badge is one printable ID badge.
type Badge struct {
	Name     string
	Role     string
	Building string
	Status   string
	QRBase64 string
}

// BadgeDoc is a set of badges sharing an organization header.
type BadgeDoc struct {
	Organization string
	LogoBase64   string
	Badges       []Badge
}

// RenderBadgesHTML renders the badge document to standalone HTML.
func (r *Renderer) RenderBadgesHTML(doc BadgeDoc) ([]byte, error) {
	if doc.LogoBase64 == "" {
		doc.LogoBase64 = r.logoBase64
	}
	var buf bytes.Buffer
	if err := r.badgeTemplate.ExecuteTemplate(&buf, "badge", doc); err != nil {
		return nil, fmt.Errorf("execute badge template: %w", err)
	}
	return buf.Bytes(), nil
}

// RenderBadgesPDF renders the badge document to PDF via Gotenberg.
func (r *Renderer) RenderBadgesPDF(client *GotenbergClient, doc BadgeDoc) ([]byte, error) {
	html, err := r.RenderBadgesHTML(doc)
	if err != nil {
		return nil, err
	}
	return client.ConvertHTML(html, DefaultPDFOptions())
}
