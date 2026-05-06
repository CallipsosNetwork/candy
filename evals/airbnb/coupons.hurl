# airbnb/coupons — Coupon admin CRUD and guest validate
# Scenarios: 14 total (10 live, 4 deferred [d])
# Covers: POST /admin/coupons, DELETE /admin/coupons/:id, GET /coupons/:code/validate
#
# Required harness variables (--variable):
#   first_admin_token=<bearer>   seeded out-of-band; no self-service admin signup path
#   expires_far=<RFC3339>        timestamp well beyond test horizon, e.g. 2099-01-01T00:00:00Z
#   expires_soon=<RFC3339>       timestamp ~2 seconds after test invocation (for S12 expiry test)
#
# Standard variables from --variables-file evals/airbnb/fixtures.env

# === S0: Guest signup (bootstrap) ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email":           "{{guest_email}}",
  "password":        "{{guest_password}}",
  "idempotency_key": "{{signup_guest_key}}"
}

HTTP 201
[Captures]
guest_user_id: jsonpath "$.user_id"
guest_token:   jsonpath "$.token"

# === S1: POST /admin/coupons — ok (50% off, 100 uses) ===

POST {{BASE_URL}}/admin/coupons
Authorization: Bearer {{first_admin_token}}
Content-Type: application/json
{
  "code":            "{{coupon_50pct}}",
  "kind":            "Percent",
  "value":           50,
  "maxUses":         100,
  "expires":         "{{expires_far}}",
  "idempotency_key": "{{create_coupon_key}}"
}

HTTP 201
[Asserts]
jsonpath "$.coupon_id" exists
[Captures]
coupon_50_id: jsonpath "$.coupon_id"

# === S2: POST /admin/coupons — ok (100% off, 1 use) ===

POST {{BASE_URL}}/admin/coupons
Authorization: Bearer {{first_admin_token}}
Content-Type: application/json
{
  "code":            "{{coupon_100pct}}",
  "kind":            "Percent",
  "value":           100,
  "maxUses":         1,
  "expires":         "{{expires_far}}",
  "idempotency_key": "create-coupon-full100-001"
}

HTTP 201
[Asserts]
jsonpath "$.coupon_id" exists
[Captures]
coupon_100_id: jsonpath "$.coupon_id"

# === S3: POST /admin/coupons — err InvalidCoupon (percent value out of range) ===

POST {{BASE_URL}}/admin/coupons
Authorization: Bearer {{first_admin_token}}
Content-Type: application/json
{
  "code":            "BADPCT",
  "kind":            "Percent",
  "value":           -5,
  "maxUses":         10,
  "expires":         "{{expires_far}}",
  "idempotency_key": "create-coupon-bad-001"
}

HTTP 422
[Asserts]
jsonpath "$.error" == "invalid_coupon"
jsonpath "$.reason" exists

# === S4: POST /admin/coupons — err InvalidCoupon (expires in the past) ===

POST {{BASE_URL}}/admin/coupons
Authorization: Bearer {{first_admin_token}}
Content-Type: application/json
{
  "code":            "PASTEXP",
  "kind":            "Percent",
  "value":           10,
  "maxUses":         1,
  "expires":         "2000-01-01T00:00:00Z",
  "idempotency_key": "create-coupon-past-001"
}

HTTP 422
[Asserts]
jsonpath "$.error" == "invalid_coupon"
jsonpath "$.reason" exists

# === S5: POST /admin/coupons — err CodeTaken (same code, fresh idempotency key) ===

POST {{BASE_URL}}/admin/coupons
Authorization: Bearer {{first_admin_token}}
Content-Type: application/json
{
  "code":            "{{coupon_50pct}}",
  "kind":            "Percent",
  "value":           25,
  "maxUses":         50,
  "expires":         "{{expires_far}}",
  "idempotency_key": "create-coupon-dupe-001"
}

HTTP 409
[Asserts]
jsonpath "$.error" == "code_taken"

# === S6: POST /admin/coupons — wrong role (Guest → 403) ===

POST {{BASE_URL}}/admin/coupons
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "code":            "GUESTBAD",
  "kind":            "Percent",
  "value":           10,
  "maxUses":         1,
  "expires":         "{{expires_far}}",
  "idempotency_key": "create-coupon-guest-001"
}

HTTP 403

# === S7: DELETE /admin/coupons/:id — ok (FULL100 has no redemptions) ===

DELETE {{BASE_URL}}/admin/coupons/{{coupon_100_id}}
Authorization: Bearer {{first_admin_token}}
Content-Type: application/json
{
  "idempotency_key": "delete-coupon-full100-001"
}

HTTP 204

# === S8: DELETE /admin/coupons/:id — err CouponNotFound (already deleted) ===

DELETE {{BASE_URL}}/admin/coupons/{{coupon_100_id}}
Authorization: Bearer {{first_admin_token}}
Content-Type: application/json
{
  "idempotency_key": "delete-coupon-full100-002"
}

HTTP 404
[Asserts]
jsonpath "$.error" == "coupon_not_found"

# === S9: DELETE /admin/coupons/:id — err CouponInUse [d] ===
#
# DEFERRED: CouponInUse requires length(redemptions) > 0, which requires
# a completed booking. RedeemCoupon is called internally by the booking saga
# and has no standalone HTTP endpoint.
#
# This scenario will be exercised in evals/airbnb/booking.hurl:
#   1. Create a fresh coupon (maxUses: 1).
#   2. Complete a booking using that coupon (booking saga calls RedeemCoupon).
#   3. Attempt DELETE on the coupon → expect 409 "coupon_in_use".

# === S10: Create short-lived coupon for expiry test (admin setup for S12) ===

POST {{BASE_URL}}/admin/coupons
Authorization: Bearer {{first_admin_token}}
Content-Type: application/json
{
  "code":            "EXPIRES_SOON",
  "kind":            "Percent",
  "value":           5,
  "maxUses":         10,
  "expires":         "{{expires_soon}}",
  "idempotency_key": "create-coupon-expires-soon-001"
}

HTTP 201
[Asserts]
jsonpath "$.coupon_id" exists

# === S11: GET /coupons/:code/validate — ok (TEST50) ===

GET {{BASE_URL}}/coupons/{{coupon_50pct}}/validate
Authorization: Bearer {{guest_token}}

HTTP 200
[Asserts]
jsonpath "$.coupon_id" exists
jsonpath "$.kind"      == "Percent"
jsonpath "$.value"     == 50

# === S12: GET /coupons/:code/validate — err CouponNotFound ===

GET {{BASE_URL}}/coupons/{{coupon_invalid}}/validate
Authorization: Bearer {{guest_token}}

HTTP 404
[Asserts]
jsonpath "$.error" == "coupon_not_found"

# === S13: GET /coupons/:code/validate — err Expired ===
# Wait for the short-lived coupon created in S10 to expire (delay > expiry window).
# expires_soon is ~2s in the future; delay 3000ms ensures we are past it.
# Adjust delay and expiry window if clock drift in the target environment exceeds 1s.

GET {{BASE_URL}}/coupons/EXPIRES_SOON/validate
Authorization: Bearer {{guest_token}}
[Options]
delay: 3000

HTTP 410
[Asserts]
jsonpath "$.error" == "coupon_expired"

# === S14: GET /coupons/:code/validate — err Exhausted [d] ===
#
# DEFERRED: Exhausted requires length(redemptions) >= maxUses.
# No standalone redemption HTTP endpoint exists — RedeemCoupon is saga-internal.
#
# Setup path (booking.hurl):
#   1. Create a coupon with maxUses: 1.
#   2. Complete one booking using it (RedeemCoupon is called by the saga).
#   3. GET /coupons/<code>/validate as any user → expect 410 "coupon_exhausted".

# === S15: GET /coupons/:code/validate — err AlreadyRedeemed [d] ===
#
# DEFERRED: AlreadyRedeemed requires the requesting user's id to already appear
# in the coupon's redemptions journal. Same dependency as S14.
#
# Setup path (booking.hurl):
#   1. Create a coupon with maxUses: 10 (so it isn't exhausted after one use).
#   2. Complete a booking as guest user (RedeemCoupon records guest's user id).
#   3. GET /coupons/<code>/validate as the same guest → expect 409 "already_redeemed".
