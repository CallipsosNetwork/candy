# feature:  airbnb/listings
# scenarios: 14 (S1–S14)
# summary:  Listing CRUD, lifecycle (publish/hide), visibility filter, role gates.
#           HoldSlot/ReleaseSlot are saga-internal — AvailableInRange with an
#           active hold is [d] (exercised by booking.hurl). S14 covers the filter
#           path against a listing with no holds.
#
# Run:
#   from_ts=$(( $(date +%s) + 120 ))
#   to_ts=$(( from_ts + 20 * 60 ))
#   hurl --variables-file evals/airbnb/fixtures.env \
#        --variable BASE_URL=http://localhost:8080 \
#        --variable from_ts=$from_ts \
#        --variable to_ts=$to_ts \
#        evals/airbnb/listings.hurl

# === SETUP: sign up admin ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "{{admin_email}}",
  "password": "{{admin_password}}",
  "idempotency_key": "{{signup_admin_key}}"
}

HTTP 201
[Captures]
admin_token: jsonpath "$.token"
admin_id: jsonpath "$.user_id"

# === SETUP: sign up host A ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "{{host_email}}",
  "password": "{{host_password}}",
  "idempotency_key": "{{signup_host_key}}"
}

HTTP 201
[Captures]
host_a_token: jsonpath "$.token"
host_a_id: jsonpath "$.user_id"

# === SETUP: upgrade host A to Host ===

POST {{BASE_URL}}/me/upgrade-to-host
Authorization: Bearer {{host_a_token}}

HTTP 200
[Asserts]
jsonpath "$.role" == "host"

# === SETUP: sign up host B ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "host2@candy.local",
  "password": "correct horse battery staple host2",
  "idempotency_key": "signup-host-b-001"
}

HTTP 201
[Captures]
host_b_token: jsonpath "$.token"
host_b_id: jsonpath "$.user_id"

# === SETUP: upgrade host B to Host ===

POST {{BASE_URL}}/me/upgrade-to-host
Authorization: Bearer {{host_b_token}}

HTTP 200

# === SETUP: sign up guest ===

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

# === S1: POST /listings — ok (Host) ===

POST {{BASE_URL}}/listings
Authorization: Bearer {{host_a_token}}
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
[Asserts]
jsonpath "$.listing_id" exists

# === S2: POST /listings — err InvalidListing (maxGuests: 0) ===

POST {{BASE_URL}}/listings
Authorization: Bearer {{host_a_token}}
Content-Type: application/json
{
  "title": "{{listing_title}}",
  "description": "{{listing_description}}",
  "location": "{{listing_location}}",
  "pricePerMinute": {{listing_price_per_minute}},
  "maxGuests": 0,
  "idempotency_key": "create-listing-invalid-001"
}

HTTP 422
[Asserts]
jsonpath "$.error" == "invalid_listing"

# === S3: POST /listings — wrong role Guest → 403 ===

POST {{BASE_URL}}/listings
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{
  "title": "{{listing_title}}",
  "description": "{{listing_description}}",
  "location": "{{listing_location}}",
  "pricePerMinute": {{listing_price_per_minute}},
  "maxGuests": {{listing_max_guests}},
  "idempotency_key": "create-listing-guest-001"
}

HTTP 403

# === S4: PATCH /listings/:id — ok (owner) ===

PATCH {{BASE_URL}}/listings/{{listing_id}}
Authorization: Bearer {{host_a_token}}
Content-Type: application/json
{
  "title": "Updated Studio Loft",
  "idempotency_key": "update-listing-001"
}

HTTP 200

# === S5: PATCH /listings/:id — err InvalidUpdate (maxGuests: 17) ===

PATCH {{BASE_URL}}/listings/{{listing_id}}
Authorization: Bearer {{host_a_token}}
Content-Type: application/json
{
  "maxGuests": 17,
  "idempotency_key": "update-listing-invalid-001"
}

HTTP 422
[Asserts]
jsonpath "$.error" == "invalid_update"

# === S6: PATCH /listings/:id — err ListingNotFound ===

PATCH {{BASE_URL}}/listings/nonexistent-id-000
Authorization: Bearer {{host_a_token}}
Content-Type: application/json
{
  "title": "Irrelevant",
  "idempotency_key": "update-listing-notfound-001"
}

HTTP 404
[Asserts]
jsonpath "$.error" == "listing_not_found"

# === S7: PATCH /listings/:id — wrong owner (other Host) → 403 ===

PATCH {{BASE_URL}}/listings/{{listing_id}}
Authorization: Bearer {{host_b_token}}
Content-Type: application/json
{
  "title": "Host B Should Not Update This",
  "idempotency_key": "update-listing-hostb-001"
}

HTTP 403

# === S8: POST /listings/:id/publish — ok ===

POST {{BASE_URL}}/listings/{{listing_id}}/publish
Authorization: Bearer {{host_a_token}}
Content-Type: application/json
{
  "idempotency_key": "{{publish_listing_key}}"
}

HTTP 200

# === S9: POST /listings/:id/publish — err AlreadyListed ===

POST {{BASE_URL}}/listings/{{listing_id}}/publish
Authorization: Bearer {{host_a_token}}
Content-Type: application/json
{
  "idempotency_key": "publish-listing-002"
}

HTTP 409
[Asserts]
jsonpath "$.error" == "already_listed"

# === S10: POST /listings/:id/publish — err DraftIncomplete ===
# Create a new listing with an empty description; publish should fail.

POST {{BASE_URL}}/listings
Authorization: Bearer {{host_a_token}}
Content-Type: application/json
{
  "title": "Incomplete Draft",
  "description": "",
  "location": "{{listing_location}}",
  "pricePerMinute": {{listing_price_per_minute}},
  "maxGuests": {{listing_max_guests}},
  "idempotency_key": "create-incomplete-001"
}

HTTP 201
[Captures]
incomplete_listing_id: jsonpath "$.listing_id"

POST {{BASE_URL}}/listings/{{incomplete_listing_id}}/publish
Authorization: Bearer {{host_a_token}}
Content-Type: application/json
{
  "idempotency_key": "publish-incomplete-001"
}

HTTP 422
[Asserts]
jsonpath "$.error" == "draft_incomplete"

# === S11: POST /listings/:id/hide — ok ===

POST {{BASE_URL}}/listings/{{listing_id}}/hide
Authorization: Bearer {{host_a_token}}
Content-Type: application/json
{
  "idempotency_key": "hide-listing-001"
}

HTTP 200

# === S12: POST /listings/:id/hide — err AlreadyHidden ===

POST {{BASE_URL}}/listings/{{listing_id}}/hide
Authorization: Bearer {{host_a_token}}
Content-Type: application/json
{
  "idempotency_key": "hide-listing-002"
}

HTTP 409
[Asserts]
jsonpath "$.error" == "already_hidden"

# === S13: GET /listings — ok (default Listed filter) ===
# Re-publish the listing so it appears in the Listed set.

POST {{BASE_URL}}/listings/{{listing_id}}/publish
Authorization: Bearer {{host_a_token}}
Content-Type: application/json
{
  "idempotency_key": "publish-listing-003"
}

HTTP 200

GET {{BASE_URL}}/listings

HTTP 200
[Asserts]
jsonpath "$.listings" isCollection
jsonpath "$.listings[0].id" exists

# === S14: GET /listings — AvailableInRange (no holds → listing present) ===
# [d] AvailableInRange with an active hold is deferred: HoldSlot and ReleaseSlot
# have no public HTTP route in the Listings controller; they are consumed
# internally by the booking saga. The hold/release path is exercised by
# evals/airbnb/booking.hurl via POST /bookings.
#
# This scenario confirms the filter is wired and correctly parses from/to.
# The listing published above has no holds, so it must appear in results.
# from_ts and to_ts are injected by the harness at run time (see file header).

GET {{BASE_URL}}/listings?filter=AvailableInRange&from={{from_ts}}&to={{to_ts}}

HTTP 200
[Asserts]
jsonpath "$.listings" isCollection
jsonpath "$.listings[*].id" includes "{{listing_id}}"
