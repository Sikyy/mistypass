package payment

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMidtransCreatePaymentLink(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/snap/v1/transactions" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"snap-token-123","redirect_url":"https://app.sandbox.midtrans.com/snap/v4/redirection/snap-token-123"}`))
	}))
	defer mock.Close()

	provider := NewMidtransProvider(MidtransOptions{Endpoint: mock.URL, ServerKey: "SB-server-key", Timeout: 2 * time.Second})
	link, err := provider.CreatePaymentLink(context.Background(), Request{
		TenantID:     "tenant_demo_jakarta",
		OrderID:      "bkg_123",
		AmountIDR:    150000,
		CustomerName: "Andri",
		Description:  "Meeting Room A",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link.Provider != "midtrans" || link.Token != "snap-token-123" || !strings.Contains(link.RedirectURL, "snap-token-123") {
		t.Fatalf("unexpected link: %+v", link)
	}

	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("SB-server-key:"))
	if gotAuth != wantAuth {
		t.Fatalf("expected basic auth from server key, got %q", gotAuth)
	}
	td, _ := gotBody["transaction_details"].(map[string]any)
	if td == nil || td["order_id"] != "bkg_123" || td["gross_amount"] != float64(150000) {
		t.Fatalf("unexpected transaction_details: %#v", gotBody)
	}
}

func TestMidtransCreatePaymentLinkHTTPError(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error_messages":["unauthorized"]}`))
	}))
	defer mock.Close()

	provider := NewMidtransProvider(MidtransOptions{Endpoint: mock.URL, ServerKey: "bad", Timeout: 2 * time.Second})
	if _, err := provider.CreatePaymentLink(context.Background(), Request{OrderID: "bkg_1", AmountIDR: 1000}); err == nil {
		t.Fatal("expected error for non-2xx response")
	}
}

func TestVerifyMidtransSignature(t *testing.T) {
	// signature = sha512(order_id + status_code + gross_amount + server_key)
	orderID, statusCode, gross, key := "bkg_123", "200", "150000.00", "SB-server-key"
	valid := midtransSignature(orderID, statusCode, gross, key)
	if !VerifyMidtransSignature(orderID, statusCode, gross, key, valid) {
		t.Fatal("expected valid signature to verify")
	}
	if VerifyMidtransSignature(orderID, statusCode, gross, key, valid+"00") {
		t.Fatal("expected tampered signature to fail")
	}
	if VerifyMidtransSignature("bkg_other", statusCode, gross, key, valid) {
		t.Fatal("expected signature for different order to fail")
	}
}
