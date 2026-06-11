package httpx

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/access"
)

const bookingPaymentTestServerKey = "SB-test-server-key"

func newSnapMock(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/snap/v1/transactions" {
			t.Errorf("unexpected snap path %s", r.URL.Path)
		}
		var body struct {
			TransactionDetails struct {
				OrderID string `json:"order_id"`
			} `json:"transaction_details"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"token":"snap-%s","redirect_url":"https://snap.test/redir/%s"}`, body.TransactionDetails.OrderID, body.TransactionDetails.OrderID)
	}))
}

func newBookingPaymentRouter(t *testing.T, snapURL string) (http.Handler, string) {
	t.Helper()
	router, _, err := NewRouter(config.Config{
		JWTSecret:         "booking-payment-test",
		EnableDemoUsers:   true,
		PaymentProvider:   "midtrans",
		MidtransEndpoint:  snapURL,
		MidtransServerKey: bookingPaymentTestServerKey,
	}, nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	return router, referenceAPILogin(t, router, "organization.admin@mistypass.local")
}

func createPricedSpace(t *testing.T, router http.Handler, token string, priceIDR int64, capacityMode string) string {
	t.Helper()
	body := []byte(fmt.Sprintf(`{"tenant_id":"tenant_demo_jakarta","name":"Paid Room","space_type":"meeting_room","capacity_mode":"%s","max_capacity":1,"requires_booking":true,"enabled":true,"price_idr":%d}`, capacityMode, priceIDR))
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/bookable-spaces", token, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected space create 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var space access.BookableSpace
	if err := json.Unmarshal(rec.Body.Bytes(), &space); err != nil {
		t.Fatalf("decode space: %v", err)
	}
	if space.PriceIDR != priceIDR {
		t.Fatalf("expected price %d round-trip, got %+v", priceIDR, space)
	}
	return space.ID
}

func createBookingFor(t *testing.T, router http.Handler, token, spaceID, start, end string) (*httptest.ResponseRecorder, access.Booking) {
	t.Helper()
	body := []byte(`{"tenant_id":"tenant_demo_jakarta","space_id":"` + spaceID + `","user_id":"usr_1001","user_name":"Andri","title":"Standup","start_time":"` + start + `","end_time":"` + end + `"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/bookings", token, body)
	var booking access.Booking
	_ = json.Unmarshal(rec.Body.Bytes(), &booking)
	return rec, booking
}

func midtransWebhookBody(orderID, transactionStatus string, grossAmount string) []byte {
	digest := sha512.Sum512([]byte(orderID + "200" + grossAmount + bookingPaymentTestServerKey))
	payload := map[string]string{
		"order_id":           orderID,
		"status_code":        "200",
		"gross_amount":       grossAmount,
		"transaction_status": transactionStatus,
		"signature_key":      hex.EncodeToString(digest[:]),
	}
	raw, _ := json.Marshal(payload)
	return raw
}

func TestBookingPaymentFlowSettlement(t *testing.T) {
	snap := newSnapMock(t)
	defer snap.Close()
	router, token := newBookingPaymentRouter(t, snap.URL)
	spaceID := createPricedSpace(t, router, token, 150000, "single_occupancy")

	rec, booking := createBookingFor(t, router, token, spaceID, "2026-07-01T09:00:00Z", "2026-07-01T10:00:00Z")
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected booking 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if booking.Status != "pending_payment" || booking.PaymentStatus != "pending" {
		t.Fatalf("expected pending_payment booking, got %+v", booking)
	}
	if booking.PaymentOrderID != booking.ID || booking.PaymentURL == "" || booking.PriceIDR != 150000 {
		t.Fatalf("expected payment link attached, got %+v", booking)
	}

	// Settlement webhook confirms the booking.
	webhook := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/webhooks/payment/midtrans", "", midtransWebhookBody(booking.ID, "settlement", "150000.00"))
	if webhook.Code != http.StatusOK {
		t.Fatalf("expected webhook 200, got %d body=%s", webhook.Code, webhook.Body.String())
	}
	getRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/bookings/"+booking.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	var settled access.Booking
	_ = json.Unmarshal(getRec.Body.Bytes(), &settled)
	if settled.Status != "confirmed" || settled.PaymentStatus != "paid" || settled.PaidAt == "" {
		t.Fatalf("expected confirmed+paid booking after settlement, got %+v", settled)
	}
}

func TestBookingPaymentWebhookBadSignature(t *testing.T) {
	snap := newSnapMock(t)
	defer snap.Close()
	router, token := newBookingPaymentRouter(t, snap.URL)
	spaceID := createPricedSpace(t, router, token, 90000, "single_occupancy")
	_, booking := createBookingFor(t, router, token, spaceID, "2026-07-02T09:00:00Z", "2026-07-02T10:00:00Z")

	payload := map[string]string{
		"order_id":           booking.ID,
		"status_code":        "200",
		"gross_amount":       "90000.00",
		"transaction_status": "settlement",
		"signature_key":      strings.Repeat("0", 128),
	}
	tampered, _ := json.Marshal(payload)
	webhook := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/webhooks/payment/midtrans", "", tampered)
	if webhook.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for bad signature, got %d body=%s", webhook.Code, webhook.Body.String())
	}

	getRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/bookings/"+booking.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	var unchanged access.Booking
	_ = json.Unmarshal(getRec.Body.Bytes(), &unchanged)
	if unchanged.Status != "pending_payment" {
		t.Fatalf("expected booking unchanged after bad signature, got %+v", unchanged)
	}
}

func TestBookingPaymentWebhookExpiry(t *testing.T) {
	snap := newSnapMock(t)
	defer snap.Close()
	router, token := newBookingPaymentRouter(t, snap.URL)
	spaceID := createPricedSpace(t, router, token, 50000, "single_occupancy")
	_, booking := createBookingFor(t, router, token, spaceID, "2026-07-03T09:00:00Z", "2026-07-03T10:00:00Z")

	webhook := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/webhooks/payment/midtrans", "", midtransWebhookBody(booking.ID, "expire", "50000.00"))
	if webhook.Code != http.StatusOK {
		t.Fatalf("expected webhook 200, got %d body=%s", webhook.Code, webhook.Body.String())
	}
	getRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/bookings/"+booking.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	var expired access.Booking
	_ = json.Unmarshal(getRec.Body.Bytes(), &expired)
	if expired.Status != "cancelled" || expired.PaymentStatus != "expired" {
		t.Fatalf("expected cancelled+expired booking, got %+v", expired)
	}
}

func TestBookingPaymentPendingHoldsCapacity(t *testing.T) {
	snap := newSnapMock(t)
	defer snap.Close()
	router, token := newBookingPaymentRouter(t, snap.URL)
	spaceID := createPricedSpace(t, router, token, 80000, "single_occupancy")

	first, _ := createBookingFor(t, router, token, spaceID, "2026-07-04T09:00:00Z", "2026-07-04T10:00:00Z")
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first booking 201, got %d", first.Code)
	}
	second, _ := createBookingFor(t, router, token, spaceID, "2026-07-04T09:30:00Z", "2026-07-04T10:30:00Z")
	if second.Code != http.StatusConflict {
		t.Fatalf("expected overlapping booking 409 while payment pending, got %d body=%s", second.Code, second.Body.String())
	}
}

func TestBookingFreeSpaceUnchanged(t *testing.T) {
	snap := newSnapMock(t)
	defer snap.Close()
	router, token := newBookingPaymentRouter(t, snap.URL)
	spaceID := createPricedSpace(t, router, token, 0, "single_occupancy")

	rec, booking := createBookingFor(t, router, token, spaceID, "2026-07-05T09:00:00Z", "2026-07-05T10:00:00Z")
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected booking 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if booking.Status != "confirmed" || booking.PaymentStatus != "" || booking.PaymentURL != "" {
		t.Fatalf("expected free booking confirmed without payment fields, got %+v", booking)
	}
}

func TestBookingPricedSpaceWithoutProvider(t *testing.T) {
	router, _, err := NewRouter(config.Config{JWTSecret: "booking-noprovider-test", EnableDemoUsers: true}, nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	spaceID := createPricedSpace(t, router, token, 60000, "single_occupancy")

	rec, _ := createBookingFor(t, router, token, spaceID, "2026-07-06T09:00:00Z", "2026-07-06T10:00:00Z")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without payment provider, got %d body=%s", rec.Code, rec.Body.String())
	}

	// The failed booking must not survive as a slot-holding zombie.
	retry, _ := createBookingFor(t, router, token, spaceID, "2026-07-06T09:00:00Z", "2026-07-06T10:00:00Z")
	if retry.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected retry to hit 503 again (slot free), got %d body=%s", retry.Code, retry.Body.String())
	}
}
