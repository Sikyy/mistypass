package pdfgen

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"time"
)

type GotenbergClient struct {
	baseURL string
	client  *http.Client
}

type PDFOptions struct {
	PaperWidth      float64
	PaperHeight     float64
	MarginTop       float64
	MarginBottom    float64
	MarginLeft      float64
	MarginRight     float64
	PrintBackground bool
	WaitDelay       string
	Scale           float64
}

func DefaultPDFOptions() PDFOptions {
	return PDFOptions{
		PaperWidth:      8.27,
		PaperHeight:     11.69,
		MarginTop:       0.4,
		MarginBottom:    0.4,
		MarginLeft:      0.4,
		MarginRight:     0.4,
		PrintBackground: true,
		WaitDelay:       "4000ms",
		Scale:           1.0,
	}
}

func NewGotenbergClient(baseURL string, client *http.Client) *GotenbergClient {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &GotenbergClient{baseURL: baseURL, client: client}
}

func (c *GotenbergClient) ConvertHTML(html []byte, opts PDFOptions) ([]byte, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="files"; filename="index.html"`)
	header.Set("Content-Type", "text/html; charset=utf-8")
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, fmt.Errorf("create file part: %w", err)
	}
	if _, err := part.Write(html); err != nil {
		return nil, fmt.Errorf("write html: %w", err)
	}

	formFields := map[string]string{
		"paperWidth":      strconv.FormatFloat(opts.PaperWidth, 'f', 2, 64),
		"paperHeight":     strconv.FormatFloat(opts.PaperHeight, 'f', 2, 64),
		"marginTop":       strconv.FormatFloat(opts.MarginTop, 'f', 2, 64),
		"marginBottom":    strconv.FormatFloat(opts.MarginBottom, 'f', 2, 64),
		"marginLeft":      strconv.FormatFloat(opts.MarginLeft, 'f', 2, 64),
		"marginRight":     strconv.FormatFloat(opts.MarginRight, 'f', 2, 64),
		"printBackground": strconv.FormatBool(opts.PrintBackground),
		"scale":           strconv.FormatFloat(opts.Scale, 'f', 2, 64),
	}
	if opts.WaitDelay != "" {
		formFields["waitDelay"] = opts.WaitDelay
	}
	for k, v := range formFields {
		if err := writer.WriteField(k, v); err != nil {
			return nil, fmt.Errorf("write field %s: %w", k, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/forms/chromium/convert/html", &body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("gotenberg returned %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func (c *GotenbergClient) Healthy() bool {
	resp, err := c.client.Get(c.baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
