package talenta

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestVerifyWebhookSignature(t *testing.T) {
	body := []byte(`{"event_type":"talenta.employee.detail.created","employee":{"id":"EMP-001"}}`)
	date := time.Date(2026, 4, 22, 8, 30, 0, 0, time.UTC)
	input := WebhookSignatureInput{
		Date:         date.Format(http.TimeFormat),
		Method:       http.MethodPost,
		RequestURI:   "/api/v1/enterprise/hris-webhook/hrc_talenta_jakarta?tenant=demo",
		Proto:        "HTTP/1.1",
		Body:         body,
		ClientID:     "mekari-client-id-001",
		ClientSecret: "mekari-client-secret-001",
		Now:          date.Add(2 * time.Minute),
	}
	input.Digest = "SHA-256=" + webhookDigest(body)
	input.Authorization = signWebhookAuthorization(input)

	if err := VerifyWebhookSignature(input); err != nil {
		t.Fatalf("expected signature verification success: %v", err)
	}
}

func TestVerifyWebhookSignatureRejectsStaleDate(t *testing.T) {
	body := []byte(`{"event_type":"talenta.employee.detail.created"}`)
	date := time.Date(2026, 4, 22, 8, 30, 0, 0, time.UTC)
	input := WebhookSignatureInput{
		Date:         date.Format(http.TimeFormat),
		Method:       http.MethodPost,
		RequestURI:   "/api/v1/enterprise/hris-webhook/hrc_talenta_jakarta",
		Proto:        "HTTP/1.1",
		Body:         body,
		ClientSecret: "mekari-client-secret-001",
		Now:          date.Add(301 * time.Second),
	}
	input.Digest = "SHA-256=" + webhookDigest(body)
	input.Authorization = signWebhookAuthorization(input)

	err := VerifyWebhookSignature(input)
	if !errors.Is(err, ErrWebhookDateSkewExceeded) {
		t.Fatalf("expected date skew error, got %v", err)
	}
}

func TestVerifyWebhookSignatureRejectsDigestMismatch(t *testing.T) {
	body := []byte(`{"event_type":"talenta.employee.detail.created"}`)
	date := time.Date(2026, 4, 22, 8, 30, 0, 0, time.UTC)
	input := WebhookSignatureInput{
		Date:         date.Format(http.TimeFormat),
		Method:       http.MethodPost,
		RequestURI:   "/api/v1/enterprise/hris-webhook/hrc_talenta_jakarta",
		Proto:        "HTTP/1.1",
		Body:         body,
		ClientSecret: "mekari-client-secret-001",
		Now:          date.Add(2 * time.Minute),
	}
	input.Digest = "SHA-256=" + webhookDigest([]byte(`{"different":true}`))
	input.Authorization = signWebhookAuthorization(input)

	err := VerifyWebhookSignature(input)
	if !errors.Is(err, ErrWebhookDigestMismatch) {
		t.Fatalf("expected digest mismatch, got %v", err)
	}
}

func TestVerifyWebhookSignatureRejectsSignatureMismatch(t *testing.T) {
	body := []byte(`{"event_type":"talenta.employee.detail.created"}`)
	date := time.Date(2026, 4, 22, 8, 30, 0, 0, time.UTC)
	input := WebhookSignatureInput{
		Date:         date.Format(http.TimeFormat),
		Method:       http.MethodPost,
		RequestURI:   "/api/v1/enterprise/hris-webhook/hrc_talenta_jakarta",
		Proto:        "HTTP/1.1",
		Body:         body,
		ClientSecret: "mekari-client-secret-001",
		Now:          date.Add(2 * time.Minute),
	}
	input.Digest = "SHA-256=" + webhookDigest(body)
	input.Authorization = signWebhookAuthorization(WebhookSignatureInput{
		Date:         input.Date,
		Method:       input.Method,
		RequestURI:   input.RequestURI,
		Proto:        input.Proto,
		Body:         input.Body,
		ClientSecret: "different-secret",
	})

	err := VerifyWebhookSignature(input)
	if !errors.Is(err, ErrWebhookSignatureMismatch) {
		t.Fatalf("expected signature mismatch, got %v", err)
	}
}

func TestVerifyWebhookSignatureRejectsClientIDMismatch(t *testing.T) {
	body := []byte(`{"event_type":"talenta.employee.detail.created"}`)
	date := time.Date(2026, 4, 22, 8, 30, 0, 0, time.UTC)
	input := WebhookSignatureInput{
		Date:         date.Format(http.TimeFormat),
		Method:       http.MethodPost,
		RequestURI:   "/api/v1/enterprise/hris-webhook/hrc_talenta_jakarta",
		Proto:        "HTTP/1.1",
		Body:         body,
		ClientID:     "expected-client-id",
		ClientSecret: "mekari-client-secret-001",
		Now:          date.Add(2 * time.Minute),
	}
	input.Digest = "SHA-256=" + webhookDigest(body)
	input.Authorization = signWebhookAuthorization(WebhookSignatureInput{
		Date:         input.Date,
		Method:       input.Method,
		RequestURI:   input.RequestURI,
		Proto:        input.Proto,
		Body:         input.Body,
		ClientID:     "different-client-id",
		ClientSecret: input.ClientSecret,
	})

	err := VerifyWebhookSignature(input)
	if !errors.Is(err, ErrWebhookClientIDMismatch) {
		t.Fatalf("expected client id mismatch, got %v", err)
	}
}

func signWebhookAuthorization(input WebhookSignatureInput) string {
	clientID := input.ClientID
	if clientID == "" {
		clientID = "mekari-client-id-001"
	}
	mac := hmac.New(sha256.New, []byte(input.ClientSecret))
	mac.Write([]byte(webhookStringToSign(input.Date, input.Method, input.RequestURI, input.Proto)))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return `hmac username="` + clientID + `", algorithm="hmac-sha256", headers="date request-line", signature="` + signature + `"`
}
