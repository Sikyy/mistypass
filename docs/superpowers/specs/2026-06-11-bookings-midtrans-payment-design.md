# Bookings Payment (Midtrans Snap) — Design

> Date: 2026-06-11
> Status: approved (Midtrans Snap first; charge-when-priced)
> Source: docs/kisi-gap-analysis.md §2.2 — Kisi Bookings has Stripe payment; our
> strategy doc picks Midtrans/Xendit for the Indonesian market (QRIS/GoPay/OVO).

## 1. Goal

Charge for bookable spaces that carry a price. A booking of a priced space is
created as `pending_payment` with a Midtrans Snap payment link; the Midtrans
webhook settles it to `confirmed` (paid) or `cancelled` (expired/failed). Free
spaces (price 0) keep today's behavior exactly. Provider is abstracted so Xendit
can be added later, mirroring the `internal/mail` provider pattern (configurable
endpoint → fully testable with httptest; real sandbox keys plug in via env).

## 2. Payment package — `internal/payment`

- `Provider` interface: `Provider() string`, `CreatePaymentLink(ctx, Request) (Link, error)`.
- `Request { TenantID, OrderID, AmountIDR int64, CustomerName, Description string }`
- `Link { Provider, Token, RedirectURL string }`
- `MidtransProvider` (`Options { Endpoint, ServerKey string, Timeout time.Duration }`,
  endpoint default `https://app.sandbox.midtrans.com`): POST
  `{endpoint}/snap/v1/transactions`, Basic auth `base64(serverKey + ":")`, body
  `{transaction_details:{order_id, gross_amount}, customer_details:{first_name},
  item_details:[{id, price, quantity:1, name}]}` → `{token, redirect_url}`.
  Non-2xx → typed HTTPError (mirror mail.HTTPError).
- `VerifyMidtransSignature(orderID, statusCode, grossAmount, serverKey,
  signatureKey string) bool` — SHA-512 hex of `orderID+statusCode+grossAmount+serverKey`,
  constant-time compare (Midtrans notification signature rule).

## 3. Data model (access service)

- `BookableSpace.PriceIDR int64` (`price_idr`, 0 = free) + plumbed through
  Create/Update inputs and the space handlers.
- `Booking` gains: `PriceIDR int64`, `PaymentOrderID`, `PaymentURL`,
  `PaymentStatus` (`pending|paid|expired|failed`), `PaidAt` (all omitempty).
- `CreateBooking`: when the space's `PriceIDR > 0`, the booking is created with
  `Status "pending_payment"`, `PriceIDR` copied, `PaymentStatus "pending"`.
  `pending_payment` bookings COUNT toward capacity/conflict overlap (they hold
  the slot while payment is in flight).
- New methods:
  - `AttachBookingPayment(tenantID, bookingID, orderID, paymentURL string) (Booking, error)`
  - `SettleBookingPaymentByOrderID(orderID, outcome string) (Booking, error)` —
    outcome `paid` → Status `confirmed`, PaymentStatus `paid`, PaidAt now;
    `expired`/`failed` → Status `cancelled`, PaymentStatus = outcome. Unknown
    order → ErrBookingNotFound. (No tenant param: webhook has no tenant; order
    IDs are globally unique booking IDs.)
- `UpdateBooking` status whitelist gains `pending_payment` (so admins can see /
  manually fix), and the booking check-in path continues to require `confirmed`.

## 4. Config + wiring

- Config: `PaymentProvider` (`PAYMENT_PROVIDER`, "" default), `MidtransEndpoint`
  (`MIDTRANS_ENDPOINT`, sandbox default), `MidtransServerKey`
  (`MIDTRANS_SERVER_KEY`). Payment enabled = provider "midtrans" + key set.
- Server holds `bookingPaymentProvider payment.Provider` (nil when not configured).
- `createBooking` handler: after CreateBooking, if status is `pending_payment`:
  - provider nil → 503 "booking requires payment but no payment provider is
    configured" (the pending booking is cancelled via UpdateBooking so it does
    not hold the slot);
  - provider call fails → cancel the booking, 502;
  - success → `AttachBookingPayment`, audit `booking_payment_link_created`,
    201 response includes `payment_url`/`payment_order_id`/`payment_status`.

## 5. Webhook

`POST /api/v1/webhooks/payment/midtrans` — public, `withEnterpriseWebhookRateLimit`.
Body (Midtrans notification): `order_id, status_code, gross_amount,
transaction_status, signature_key`. Steps: provider configured else 503; verify
signature (invalid → 403); map `transaction_status`: `capture|settlement` →
paid, `expire` → expired, `cancel|deny` → failed, anything else (e.g. `pending`)
→ 200 ignored. Settle via `SettleBookingPaymentByOrderID`; unknown order → 200
(no enumeration oracle, logged). Audit `booking_payment_settled`.

## 6. Testing (TDD)

payment pkg: CreatePaymentLink happy path against httptest Snap mock (asserts
auth header + order_id + gross_amount, returns token/redirect_url); non-2xx →
error. Signature verify: valid / tampered.

httpx integration (router with `MidtransEndpoint` pointed at httptest mock):
- priced space → create booking → 201 `pending_payment` + payment_url + order id.
- webhook settlement with valid signature → booking `confirmed`/`paid`/PaidAt set.
- webhook with bad signature → 403, booking unchanged.
- webhook expire → booking `cancelled`/`expired`.
- free space → `confirmed` immediately (no payment fields) — regression.
- priced space with no provider configured → 503 and the booking does not
  survive as a slot-holding zombie (status cancelled).
- pending_payment booking holds capacity: second booking of the same
  single-occupancy slot → 409.

## 7. Out of scope / future
Xendit provider; refunds; partial payments; per-hour pricing; payment receipts
by email; frontend payment status UI; Kisi-style required agreements at booking
(separate small feature, can reuse the NDA pattern).
