package httpx

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/mistypass/cloud/api/internal/payment"
)

// receiveMidtransPaymentWebhook handles POST /api/v1/webhooks/payment/midtrans.
// Public (Midtrans calls it), rate-limited, authenticated by the notification
// signature: sha512(order_id + status_code + gross_amount + server_key).
func (s *server) receiveMidtransPaymentWebhook(w http.ResponseWriter, r *http.Request) {
	if s.bookingPaymentProvider == nil || strings.TrimSpace(s.cfg.MidtransServerKey) == "" {
		writeError(w, http.StatusServiceUnavailable, "payment provider is not configured")
		return
	}

	var notification struct {
		OrderID           string `json:"order_id"`
		StatusCode        string `json:"status_code"`
		GrossAmount       string `json:"gross_amount"`
		TransactionStatus string `json:"transaction_status"`
		SignatureKey      string `json:"signature_key"`
	}
	if err := decodeJSON(r, &notification); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(notification.OrderID) == "" {
		writeError(w, http.StatusBadRequest, "order_id is required")
		return
	}

	if !payment.VerifyMidtransSignature(notification.OrderID, notification.StatusCode, notification.GrossAmount, s.cfg.MidtransServerKey, notification.SignatureKey) {
		writeError(w, http.StatusForbidden, "invalid notification signature")
		return
	}

	outcome := ""
	switch strings.ToLower(strings.TrimSpace(notification.TransactionStatus)) {
	case "capture", "settlement":
		outcome = "paid"
	case "expire":
		outcome = "expired"
	case "cancel", "deny":
		outcome = "failed"
	default:
		// e.g. "pending" — acknowledged, nothing to settle yet.
		writeJSON(w, http.StatusOK, map[string]any{"status": "ignored"})
		return
	}

	booking, err := s.accessSvc.SettleBookingPaymentByOrderID(notification.OrderID, outcome)
	if err != nil {
		// Unknown order id: acknowledge without leaking whether it exists.
		s.logger.Warn("midtrans webhook for unknown order", "order_id", notification.OrderID)
		writeJSON(w, http.StatusOK, map[string]any{"status": "ignored"})
		return
	}

	s.appendAuditLog(r, booking.TenantID, "booking_payment_settled",
		fmt.Sprintf("booking_id=%s,outcome=%s,transaction_status=%s", booking.ID, outcome, notification.TransactionStatus), "access")
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "booking_id": booking.ID, "payment_status": booking.PaymentStatus})
}
