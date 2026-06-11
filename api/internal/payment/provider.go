package payment

import (
	"bytes"
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MidtransEndpointDefault is the Snap sandbox base URL; production uses
// https://app.midtrans.com.
const MidtransEndpointDefault = "https://app.sandbox.midtrans.com"

// Provider creates hosted payment links for bookings.
type Provider interface {
	Provider() string
	CreatePaymentLink(ctx context.Context, req Request) (Link, error)
}

type Request struct {
	TenantID     string
	OrderID      string
	AmountIDR    int64
	CustomerName string
	Description  string
}

type Link struct {
	Provider    string
	Token       string
	RedirectURL string
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("payment provider returned status %d: %s", e.StatusCode, e.Body)
}

// --- Midtrans Snap ---

type MidtransOptions struct {
	Endpoint  string
	ServerKey string
	Timeout   time.Duration
}

type MidtransProvider struct {
	endpoint  string
	serverKey string
	client    *http.Client
}

func NewMidtransProvider(opts MidtransOptions) *MidtransProvider {
	endpoint := strings.TrimRight(strings.TrimSpace(opts.Endpoint), "/")
	if endpoint == "" {
		endpoint = MidtransEndpointDefault
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &MidtransProvider{
		endpoint:  endpoint,
		serverKey: strings.TrimSpace(opts.ServerKey),
		client:    &http.Client{Timeout: timeout},
	}
}

func (p *MidtransProvider) Provider() string { return "midtrans" }

// CreatePaymentLink creates a Snap transaction and returns the hosted checkout
// token + redirect URL.
func (p *MidtransProvider) CreatePaymentLink(ctx context.Context, req Request) (Link, error) {
	if strings.TrimSpace(req.OrderID) == "" {
		return Link{}, fmt.Errorf("order id is required")
	}
	if req.AmountIDR <= 0 {
		return Link{}, fmt.Errorf("amount must be positive")
	}

	payload := map[string]any{
		"transaction_details": map[string]any{
			"order_id":     req.OrderID,
			"gross_amount": req.AmountIDR,
		},
		"item_details": []map[string]any{{
			"id":       req.OrderID,
			"price":    req.AmountIDR,
			"quantity": 1,
			"name":     firstNonEmpty(req.Description, "Booking"),
		}},
		"customer_details": map[string]any{
			"first_name": firstNonEmpty(req.CustomerName, "Guest"),
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Link{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint+"/snap/v1/transactions", bytes.NewReader(body))
	if err != nil {
		return Link{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(p.serverKey+":")))

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return Link{}, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Link{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Link{}, HTTPError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(raw))}
	}

	var parsed struct {
		Token       string `json:"token"`
		RedirectURL string `json:"redirect_url"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Link{}, fmt.Errorf("decode snap response: %w", err)
	}
	if parsed.Token == "" || parsed.RedirectURL == "" {
		return Link{}, fmt.Errorf("snap response missing token or redirect_url")
	}
	return Link{Provider: "midtrans", Token: parsed.Token, RedirectURL: parsed.RedirectURL}, nil
}

// midtransSignature computes the Midtrans notification signature:
// sha512(order_id + status_code + gross_amount + server_key), hex-encoded.
func midtransSignature(orderID, statusCode, grossAmount, serverKey string) string {
	digest := sha512.Sum512([]byte(orderID + statusCode + grossAmount + serverKey))
	return hex.EncodeToString(digest[:])
}

// VerifyMidtransSignature checks a webhook notification signature in constant time.
func VerifyMidtransSignature(orderID, statusCode, grossAmount, serverKey, signatureKey string) bool {
	expected := midtransSignature(orderID, statusCode, grossAmount, serverKey)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(strings.TrimSpace(signatureKey)))) == 1
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
