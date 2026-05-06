# feature:  auth (standalone)
# scenarios: 13
# summary:   Signup (happy + 3 weak-password + email-taken + idempotency replay),
#            login (happy + 2 opaque InvalidCredentials), logout (happy + replay +
#            missing-bearer + invalid-bearer).

# === signup-alice ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "{{alice_email}}",
  "password": "{{alice_password}}",
  "idempotency_key": "{{signup_alice_key}}"
}

HTTP 201
[Captures]
alice_user_id: jsonpath "$.user_id"
alice_signup_token: jsonpath "$.token"
[Asserts]
jsonpath "$.user_id" isString
jsonpath "$.user_id" != ""
jsonpath "$.token" isString
jsonpath "$.token" != ""

# === signup-weak-too-short ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "weaktest-short@candy.local",
  "password": "{{weak_short}}",
  "idempotency_key": "signup-weak-short-001"
}

HTTP 422
[Asserts]
jsonpath "$.error" == "weak_password"
jsonpath "$.reason" == "too_short"

# === signup-weak-missing-digit ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "weaktest-digit@candy.local",
  "password": "{{weak_no_digit}}",
  "idempotency_key": "signup-weak-digit-001"
}

HTTP 422
[Asserts]
jsonpath "$.error" == "weak_password"
jsonpath "$.reason" == "missing_digit"

# === signup-weak-blocklisted ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "weaktest-block@candy.local",
  "password": "{{weak_blocklisted}}",
  "idempotency_key": "signup-weak-block-001"
}

HTTP 422
[Asserts]
jsonpath "$.error" == "weak_password"
jsonpath "$.reason" == "in_blocklist"

# === signup-bob ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "{{bob_email}}",
  "password": "{{bob_password}}",
  "idempotency_key": "{{signup_bob_key}}"
}

HTTP 201
[Captures]
bob_user_id: jsonpath "$.user_id"
[Asserts]
jsonpath "$.user_id" isString
jsonpath "$.user_id" != ""
jsonpath "$.token" isString
jsonpath "$.token" != ""

# === signup-email-taken ===
# Alice's email is already registered; any strong password triggers EmailTaken.

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "{{alice_email}}",
  "password": "{{bob_password}}",
  "idempotency_key": "signup-email-taken-001"
}

HTTP 409
[Asserts]
jsonpath "$.error" == "email_taken"

# === signup-idempotency-replay ===
# Exact replay of Alice's signup (same key). Must return 201 with the same user_id.

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "{{alice_email}}",
  "password": "{{alice_password}}",
  "idempotency_key": "{{signup_alice_key}}"
}

HTTP 201
[Asserts]
jsonpath "$.user_id" == {{alice_user_id}}
jsonpath "$.token" isString
jsonpath "$.token" != ""

# === login-alice ===

POST {{BASE_URL}}/login
Content-Type: application/json
{
  "email": "{{alice_email}}",
  "password": "{{alice_password}}"
}

HTTP 200
[Captures]
alice_token: jsonpath "$.token"
[Asserts]
jsonpath "$.user_id" == {{alice_user_id}}
jsonpath "$.token" isString
jsonpath "$.token" != ""

# === login-wrong-password ===
# InvalidCredentials is opaque — same body shape regardless of which field failed.

POST {{BASE_URL}}/login
Content-Type: application/json
{
  "email": "{{alice_email}}",
  "password": "wrong-password-99"
}

HTTP 401
[Asserts]
jsonpath "$.error" == "invalid_credentials"

# === login-no-such-email ===
# Unregistered email. Response must be identical to login-wrong-password —
# conformance check that the implementation does not leak which field failed.

POST {{BASE_URL}}/login
Content-Type: application/json
{
  "email": "nobody@candy.local",
  "password": "{{alice_password}}"
}

HTTP 401
[Asserts]
jsonpath "$.error" == "invalid_credentials"

# === logout-alice ===

POST {{BASE_URL}}/logout
Authorization: Bearer {{alice_token}}

HTTP 204

# === logout-replay ===
# Session.Revoke() is idempotent per spec. Re-sending the revoked token must
# still return 204, not 401 — the backend must not re-validate after revocation.

POST {{BASE_URL}}/logout
Authorization: Bearer {{alice_token}}

HTTP 204

# === logout-missing-bearer ===
# No Authorization header at all.

POST {{BASE_URL}}/logout

HTTP 401

# === logout-invalid-bearer ===
# Syntactically valid header format but the token is not a known session.

POST {{BASE_URL}}/logout
Authorization: Bearer bogus-token-xyz-000

HTTP 401
