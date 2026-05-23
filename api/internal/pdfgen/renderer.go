package pdfgen

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed assets/logo.png
var logoPNG []byte

var validReportTypes = map[string]bool{
	"weekly_analytics": true,
	"events":           true,
	"unlock_stats":     true,
	"user_presence":    true,
	"incidents":        true,
	"hardware":         true,
}

type Renderer struct {
	templates  map[string]*template.Template
	logoBase64 string
}

type templateData struct {
	Meta       ReportMeta
	LogoBase64 string
	DataJSON   template.JS
}

func NewRenderer() (*Renderer, error) {
	logoB64 := base64.StdEncoding.EncodeToString(logoPNG)
	templates := make(map[string]*template.Template)

	for rt := range validReportTypes {
		tmpl, err := template.ParseFS(templateFS,
			"templates/base.html",
			"templates/"+rt+".html",
		)
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", rt, err)
		}
		templates[rt] = tmpl
	}

	return &Renderer{
		templates:  templates,
		logoBase64: logoB64,
	}, nil
}

func (r *Renderer) RenderHTML(reportType string, meta ReportMeta, data any) ([]byte, error) {
	tmpl, ok := r.templates[reportType]
	if !ok {
		return nil, fmt.Errorf("unknown report type: %s", reportType)
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}

	td := templateData{
		Meta:       meta,
		LogoBase64: r.logoBase64,
		DataJSON:   template.JS(dataJSON),
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", td); err != nil {
		return nil, fmt.Errorf("execute template %s: %w", reportType, err)
	}
	return buf.Bytes(), nil
}

func (r *Renderer) RenderPDF(client *GotenbergClient, reportType string, meta ReportMeta, data any) ([]byte, error) {
	html, err := r.RenderHTML(reportType, meta, data)
	if err != nil {
		return nil, err
	}
	return client.ConvertHTML(html, DefaultPDFOptions())
}

func FormatPDFFilename(reportType string, start, end time.Time) string {
	return fmt.Sprintf("%s_%s_%s.pdf",
		reportType,
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
	)
}
