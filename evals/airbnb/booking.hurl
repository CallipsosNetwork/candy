# airbnb/booking.hurl
# Scenarios: 25  (A1–A3, B1, C1, E5, F2–F3, G1–G3 runnable unconditionally;
#                 E1–E4 [~] require runner-injected time vars;
#                 B2, B3, D1–D3, F1, H1–H5 [d]/[~] documented in booking.md)
# Summary: multi-actor PlaceBooking saga — hold, coupon, charge, cancel, complete.

# ============================================================
# === SETUP: Admin signup / bootstrap ========================
# ============================================================
# The first admin must be supplied by the harness via:
#   --variable admin_token=<value>
# OR via a test-mode seed endpoint (see auth.md "Deferred: admin bootstrap").
# The block below uses {{admin_token}} directly; remove if using seed endpoint.

# === SETUP: Host signup =====================================

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "{{host_email}}",
  "password": "{{host_password}}",
  "idempotency_key": "{{signup_host_key}}"
}
HTTP 201
[Captures]
host_token: jsonpath "$.token"
host_user_id: jsonpath "$.user_id"

# === SETUP: Host self-upgrade to Host role ==================

POST {{BASE_URL}}/me/upgrade-to-host
Authorization: Bearer {{host_token}}
Content-Type: application/json
{
  "idempotency_key": "{{upgrade_to_host_key}}"
}
HTTP 200

# === SETUP: Guest signup ====================================

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "{{guest_email}}",
  "password": "{{guest_password}}",
  "idempotency_key": "{{signup_guest_key}}"
}
HTTP 201
[Captures]
guest_token: jsonpath "$.token"
guest_user_id: jsonpath "$.user_id"

# === SETUP: Other-user signup (for auth check) ==============

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "other@candy.local",
  "password": "correct horse battery staple other",
  "idempotency_key": "signup-other-001"
}
HTTP 201
[Captures]
other_token: jsonpath "$.token"

# === SETUP: Create listing ==================================

POST {{BASE_URL}}/listings
Authorization: Bearer {{host_token}}
Content-Type: application/json
{
  "title": "{{listing_title}}",
  "description": "{{listing_description}}",
  "location": "{{listing_location}}",
  "pricePerMinute": {{listing_price_per_minute}},
  "maxGuests": {{listing_max_guests}},
  "idempotency_key": "{{create_listing_key}}"
}
HTTP 201
[Captures]
listing_id: jsonpath "$.listing_id"

# === SETUP: Publish listing =================================

POST {{BASE_URL}}/listings/{{listing_id}}/publish
Authorization: Bearer {{host_token}}
Content-Type: application/json
{
  "idempotency_key": "{{publish_listing_key}}"
}
HTTP 200

# === SETUP: Create 50% coupon ===============================

POST {{BASE_URL}}/admin/coupons
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "code": "{{coupon_50pct}}",
  "kind": "Percent",
  "value": 50,
  "maxUses": 100,
  "expires": "2099-12-31T23:59:59Z",
  "idempotency_key": "{{create_coupon_key}}-50pct"
}
HTTP 201
[Captures]
coupon_50_id: jsonpath "$.coupon_id"

# === SETUP: Create 100% coupon ==============================

POST {{BASE_URL}}/admin/coupons
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "code": "{{coupon_100pct}}",
  "kind": "Percent",
  "value": 100,
  "maxUses": 100,
  "expires": "2099-12-31T23:59:59Z",
  "idempotency_key": "{{create_coupon_key}}-100pct"
}
HTTP 201
[Captures]
coupon_100_id: jsonpath "$.coupon_id"

# ============================================================
# === A1 — PlaceBooking, no coupon → 201 =====================
# ============================================================
# Static far-future slot: 2099-01-01 12:00–12:20 (20 min).
# total = pricePerMinute(50) * 20 = 1000 minor units.

POST {{BASE_URL}}/bookings
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "listing": "{{listing_id}}",
  "dates": {
    "from": "2099-01-01T12:00:00Z",
    "to":   "2099-01-01T12:20:00Z"
  },
  "source": "{{test_payment_method}}",
  "idempotency_key": "{{place_booking_key}}"
}
HTTP 201
[Captures]
booking_id: jsonpath "$.booking_id"
[Asserts]
jsonpath "$.booking_id" isString
jsonpath "$.total" == 1000

# ============================================================
# === A2 — PlaceBooking, 50% coupon → 201 ====================
# ============================================================
# Distinct slot: 2099-06-01 10:00–10:20.
# total = 1000 * 50% = 500 minor units.

POST {{BASE_URL}}/bookings
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "listing": "{{listing_id}}",
  "dates": {
    "from": "2099-06-01T10:00:00Z",
    "to":   "2099-06-01T10:20:00Z"
  },
  "source": "{{test_payment_method}}",
  "code": "{{coupon_50pct}}",
  "idempotency_key": "place-booking-50pct-001"
}
HTTP 201
[Captures]
booking_id_50pct: jsonpath "$.booking_id"
[Asserts]
jsonpath "$.booking_id" isString
jsonpath "$.total" == 500

# ============================================================
# === A3 — PlaceBooking, 100% coupon → 201, total == 0 =======
# ============================================================
# Distinct slot: 2099-07-01 09:00–09:20.
# total = 1000 * 100% discount = 0.

POST {{BASE_URL}}/bookings
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "listing": "{{listing_id}}",
  "dates": {
    "from": "2099-07-01T09:00:00Z",
    "to":   "2099-07-01T09:20:00Z"
  },
  "source": "{{test_payment_method}}",
  "code": "{{coupon_100pct}}",
  "idempotency_key": "place-booking-full-001"
}
HTTP 201
[Captures]
booking_id_full: jsonpath "$.booking_id"
[Asserts]
jsonpath "$.booking_id" isString
jsonpath "$.total" == 0

# ============================================================
# === B1 — SlotUnavailable: same slot as A1 → 409 ============
# ============================================================
# Fresh idempotency key — genuine second request, not a replay.
# The Calendar hold from A1 causes HoldSlot to reject.

POST {{BASE_URL}}/bookings
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "listing": "{{listing_id}}",
  "dates": {
    "from": "2099-01-01T12:00:00Z",
    "to":   "2099-01-01T12:20:00Z"
  },
  "source": "{{test_payment_method}}",
  "idempotency_key": "place-booking-slot-conflict-001"
}
HTTP 409
[Asserts]
jsonpath "$.error" == "slot_unavailable"

# State readback: A1 booking is unaffected.
GET {{BASE_URL}}/bookings/{{booking_id}}
Authorization: Bearer {{guest_token}}
HTTP 200
[Asserts]
jsonpath "$.booking_id" == "{{booking_id}}"

# ============================================================
# === B2 — PaymentDeclined → 402 [d] =========================
# ============================================================
# DEFERRED: requires a Payments mock that forces card decline.
# See booking.md §B2 for the full scenario and compensation spec.
#
# TODO: when harness supports decline injection, add:
#   POST {{BASE_URL}}/bookings
#   ... source: "pm_card_chargeDeclined" ...
#   HTTP 402
#   jsonpath "$.error" == "payment_declined"
# Then verify slot released: re-book same dates → 201.

# ============================================================
# === B3 — CouponConflict → 422 [d] ==========================
# ============================================================
# DEFERRED: requires concurrency simulator or step-level failure injection.
# See booking.md §B3 for the compensation chain spec.
#
# TODO: when harness supports validate-succeed/redeem-fail injection:
#   HTTP 422
#   jsonpath "$.error" == "coupon_conflict"
# Verify: charge refunded, slot released.

# ============================================================
# === C1 — PlaceBooking idempotency replay ====================
# ============================================================
# Exact replay of A1 — same body, same key. Must return 201
# with the same booking_id and total. No new record created.

POST {{BASE_URL}}/bookings
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "listing": "{{listing_id}}",
  "dates": {
    "from": "2099-01-01T12:00:00Z",
    "to":   "2099-01-01T12:20:00Z"
  },
  "source": "{{test_payment_method}}",
  "idempotency_key": "{{place_booking_key}}"
}
HTTP 201
[Asserts]
jsonpath "$.booking_id" == "{{booking_id}}"
jsonpath "$.total" == 1000

# ============================================================
# === D1–D3 — PlaceBookingWithFallback [d] ===================
# ============================================================
# All three sub-cases deferred — require provider failure injection.
# See booking.md §D for full scenarios.
#
# D1: Stripe succeeds → 201, provider == "stripe"
# D2: Stripe fails, Polar succeeds → 201, provider == "polar"
# D3: All fail → 502, error == "all_providers_failed"

# ============================================================
# === E1 — CancelBooking: Free refund → 200 [~] ==============
# ============================================================
# Requires runner to inject:
#   booking_start_free  = now + 120s (RFC-3339)
#   booking_end_free    = now + 1320s (RFC-3339)
# dates.from is 120s out → CancellationPolicy returns Free → refund = total.

POST {{BASE_URL}}/bookings
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "listing": "{{listing_id}}",
  "dates": {
    "from": "{{booking_start_free}}",
    "to":   "{{booking_end_free}}"
  },
  "source": "{{test_payment_method}}",
  "idempotency_key": "place-booking-cancel-free-001"
}
HTTP 201
[Captures]
booking_id_cancel_free: jsonpath "$.booking_id"

POST {{BASE_URL}}/bookings/{{booking_id_cancel_free}}/cancel
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "reason": "changed my mind",
  "idempotency_key": "{{cancel_booking_key}}-free"
}
HTTP 200
[Asserts]
jsonpath "$.refund" == 1000

# ============================================================
# === E2 — CancelBooking: Partial refund → 200 [~] ===========
# ============================================================
# Requires runner to inject:
#   booking_start_partial = now + 45s (RFC-3339)
#   booking_end_partial   = now + 1245s (RFC-3339)
# dates.from 45s out → Partial → refund = total / 2.

POST {{BASE_URL}}/bookings
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "listing": "{{listing_id}}",
  "dates": {
    "from": "{{booking_start_partial}}",
    "to":   "{{booking_end_partial}}"
  },
  "source": "{{test_payment_method}}",
  "idempotency_key": "place-booking-cancel-partial-001"
}
HTTP 201
[Captures]
booking_id_cancel_partial: jsonpath "$.booking_id"

POST {{BASE_URL}}/bookings/{{booking_id_cancel_partial}}/cancel
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "reason": "too close for comfort",
  "idempotency_key": "{{cancel_booking_key}}-partial"
}
HTTP 200
[Asserts]
jsonpath "$.refund" == 500

# ============================================================
# === E3 — CancelBooking: NonRefundable → 200 [~] ============
# ============================================================
# Requires runner to inject:
#   booking_start_nonrefundable = now + 15s (RFC-3339)
#   booking_end_nonrefundable   = now + 1215s (RFC-3339)
# dates.from 15s out → NonRefundable → refund = 0.
# Flow succeeds (200) but issues no refund.

POST {{BASE_URL}}/bookings
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "listing": "{{listing_id}}",
  "dates": {
    "from": "{{booking_start_nonrefundable}}",
    "to":   "{{booking_end_nonrefundable}}"
  },
  "source": "{{test_payment_method}}",
  "idempotency_key": "place-booking-cancel-nr-001"
}
HTTP 201
[Captures]
booking_id_cancel_nr: jsonpath "$.booking_id"

POST {{BASE_URL}}/bookings/{{booking_id_cancel_nr}}/cancel
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "reason": "last minute change",
  "idempotency_key": "{{cancel_booking_key}}-nr"
}
HTTP 200
[Asserts]
jsonpath "$.refund" == 0

# ============================================================
# === E4 — CancelBooking: BookingStarted → 409 [~] ===========
# ============================================================
# Requires runner to inject:
#   booking_start_past = now - 10s (RFC-3339)
#   booking_end_past   = now + 1190s (RFC-3339)
# dates.from is in the past → CancellationPolicy returns BookingStarted.
#
# NOTE: PlaceBooking may itself reject a past dates.from. If so, this
# scenario must be restructured (see booking.md §E4 judgment call).

POST {{BASE_URL}}/bookings
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "listing": "{{listing_id}}",
  "dates": {
    "from": "{{booking_start_past}}",
    "to":   "{{booking_end_past}}"
  },
  "source": "{{test_payment_method}}",
  "idempotency_key": "place-booking-cancel-started-001"
}
HTTP 201
[Captures]
booking_id_cancel_started: jsonpath "$.booking_id"

POST {{BASE_URL}}/bookings/{{booking_id_cancel_started}}/cancel
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "reason": "already started",
  "idempotency_key": "{{cancel_booking_key}}-started"
}
HTTP 409
[Asserts]
jsonpath "$.error" == "booking_started"

# ============================================================
# === E5 — CancelBooking: BookingNotFound → 404 ==============
# ============================================================

POST {{BASE_URL}}/bookings/00000000-0000-0000-0000-000000000000/cancel
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "reason": "no such booking",
  "idempotency_key": "cancel-not-found-001"
}
HTTP 404
[Asserts]
jsonpath "$.error" == "booking_not_found"

# ============================================================
# === F1 — CompleteBooking: happy path → 200 [d] =============
# ============================================================
# DEFERRED: requires Confirmed status (webhook) + now >= dates.to.
# See booking.md §F1. When harness supports time advancement:
#   POST {{BASE_URL}}/bookings/{{booking_id_completed}}/complete
#   HTTP 200
#   jsonpath "$.transfer_id" isString

# ============================================================
# === F2 — CompleteBooking: NotConfirmed → 409 ===============
# ============================================================
# Uses booking_id from A1. If the backend auto-confirms, this may
# return 409 already_completed or 409 booking_not_ended instead —
# see booking.md §F2 for the ambiguity note.

POST {{BASE_URL}}/bookings/{{booking_id}}/complete
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "idempotency_key": "complete-not-confirmed-001"
}
HTTP 409
[Asserts]
jsonpath "$.error" == "not_confirmed"

# ============================================================
# === F3 — CompleteBooking: BookingNotEnded → 409 ============
# ============================================================
# Uses booking_id from A1 (dates.to = 2099-01-01T12:20:00Z).
# now < dates.to → BookingNotEnded.
# If backend auto-confirms, this booking is Confirmed; CompleteBooking
# will hit the BookingNotEnded check before the NotConfirmed check.

POST {{BASE_URL}}/bookings/{{booking_id}}/complete
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "idempotency_key": "complete-not-ended-001"
}
HTTP 409
[Asserts]
jsonpath "$.error" == "booking_not_ended"

# ============================================================
# === G1 — GET /bookings/:id — guest reads own booking → 200 =
# ============================================================

GET {{BASE_URL}}/bookings/{{booking_id}}
Authorization: Bearer {{guest_token}}
HTTP 200
[Asserts]
jsonpath "$.booking_id" == "{{booking_id}}"
jsonpath "$.listing" == "{{listing_id}}"
jsonpath "$.guest" == "{{guest_user_id}}"
jsonpath "$.status" isString

# ============================================================
# === G2 — GET /bookings/:id — host reads own booking → 200 ==
# ============================================================

GET {{BASE_URL}}/bookings/{{booking_id}}
Authorization: Bearer {{host_token}}
HTTP 200
[Asserts]
jsonpath "$.booking_id" == "{{booking_id}}"
jsonpath "$.host" == "{{host_user_id}}"

# ============================================================
# === G3 — GET /bookings/:id — other user → 403 ==============
# ============================================================

GET {{BASE_URL}}/bookings/{{booking_id}}
Authorization: Bearer {{other_token}}
HTTP 403
[Asserts]
jsonpath "$.error" == "not_authorized"

# ============================================================
# === H1–H5 — Schedule: SendBookingReminder [~] ==============
# ============================================================
# Requires runner to:
#   1. Inject booking_start_reminder = now + 90s (RFC-3339)
#              booking_end_reminder   = now + 1290s (RFC-3339)
#   2. Support sleep steps (30s, then 30s more).
#   3. Have the scheduler running in eval mode (1-minute tick active).
#
# Hurl does not natively support sleep. The runner must wrap these
# blocks in a shell script with `sleep` between hurl invocations,
# or use a hurl runner extension. See booking.md §H for full spec.

# H1 — Place near-future booking [~]

POST {{BASE_URL}}/bookings
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "listing": "{{listing_id}}",
  "dates": {
    "from": "{{booking_start_reminder}}",
    "to":   "{{booking_end_reminder}}"
  },
  "source": "{{test_payment_method}}",
  "idempotency_key": "place-booking-reminder-001"
}
HTTP 201
[Captures]
booking_id_reminder: jsonpath "$.booking_id"

# H2 — Pre-sleep: booking exists, reminder_sent == false [~]

GET {{BASE_URL}}/bookings/{{booking_id_reminder}}
Authorization: Bearer {{guest_token}}
HTTP 200
[Asserts]
jsonpath "$.booking_id" == "{{booking_id_reminder}}"
jsonpath "$.reminder_sent" == false

# --- Runner sleeps 30s here ---
# H3 — 30s in: schedule has not yet fired, reminder_sent still false [~]
# (This block must run after the 30s sleep.)

GET {{BASE_URL}}/bookings/{{booking_id_reminder}}
Authorization: Bearer {{guest_token}}
HTTP 200
[Asserts]
jsonpath "$.reminder_sent" == false

# --- Runner sleeps another 30s here (60s total from H1) ---
# H4 — 60s in: schedule fired, reminder_sent == true [~]
# dates.from - 60s <= now < dates.from is now satisfied.

GET {{BASE_URL}}/bookings/{{booking_id_reminder}}
Authorization: Bearer {{guest_token}}
HTTP 200
[Asserts]
jsonpath "$.reminder_sent" == true

# --- Runner sleeps another 30s here ---
# H5 — reminder_sent guard: no refire, value stays true [~]

GET {{BASE_URL}}/bookings/{{booking_id_reminder}}
Authorization: Bearer {{guest_token}}
HTTP 200
[Asserts]
jsonpath "$.reminder_sent" == true
