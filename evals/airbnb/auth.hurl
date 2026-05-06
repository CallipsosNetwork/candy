# airbnb/auth — 25 scenarios — full role verification (issue #10)
# Roles: Guest (default), Host (self-upgrade), Admin (promoted OOB by harness).
# Run: hurl --variables-file evals/airbnb/fixtures.env \
#           --variable BASE_URL=http://localhost:8080 \
#           --variable first_admin_token=<seeded-admin-bearer> \
#           evals/airbnb/auth.hurl

# === 1. Signup: Guest happy path ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{"email": "{{guest_email}}", "password": "{{guest_password}}", "idempotency_key": "{{signup_guest_key}}"}

HTTP 201
[Captures]
guest_user_id: jsonpath "$.user_id"
guest_signup_token: jsonpath "$.token"
[Asserts]
jsonpath "$.user_id" exists
jsonpath "$.token" exists

# === 2. Signup: WeakPassword — TooShort ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{"email": "weakpw-short@candy.local", "password": "short1", "idempotency_key": "signup-weak-short-001"}

HTTP 422
[Asserts]
jsonpath "$.error" == "weak_password"
jsonpath "$.reason" exists

# === 3. Signup: WeakPassword — MissingDigit ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{"email": "weakpw-nodigit@candy.local", "password": "alllowercaseonly", "idempotency_key": "signup-weak-nodigit-001"}

HTTP 422
[Asserts]
jsonpath "$.error" == "weak_password"
jsonpath "$.reason" exists

# === 4. Signup: WeakPassword — InBlocklist ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{"email": "weakpw-blocklist@candy.local", "password": "password123", "idempotency_key": "signup-weak-blocklist-001"}

HTTP 422
[Asserts]
jsonpath "$.error" == "weak_password"
jsonpath "$.reason" exists

# === 5. Signup: EmailTaken ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{"email": "{{guest_email}}", "password": "{{guest_password}}", "idempotency_key": "signup-guest-duplicate-001"}

HTTP 409
[Asserts]
jsonpath "$.error" == "email_taken"

# === 6. Signup: idempotency replay (same key → same ids) ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{"email": "{{guest_email}}", "password": "{{guest_password}}", "idempotency_key": "{{signup_guest_key}}"}

HTTP 201
[Asserts]
jsonpath "$.user_id" == {{guest_user_id}}

# === 7. Signup: Host bootstrap ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{"email": "{{host_email}}", "password": "{{host_password}}", "idempotency_key": "{{signup_host_key}}"}

HTTP 201
[Captures]
host_user_id: jsonpath "$.user_id"
host_token: jsonpath "$.token"
[Asserts]
jsonpath "$.user_id" exists
jsonpath "$.token" exists

# === 8. Login: happy path (role in response) ===

POST {{BASE_URL}}/login
Content-Type: application/json
{"email": "{{guest_email}}", "password": "{{guest_password}}"}

HTTP 200
[Captures]
guest_login_token: jsonpath "$.token"
[Asserts]
jsonpath "$.user_id" == {{guest_user_id}}
jsonpath "$.role" == "guest"
jsonpath "$.token" exists

# === 9. Login: wrong password → 401 ===

POST {{BASE_URL}}/login
Content-Type: application/json
{"email": "{{guest_email}}", "password": "definitely-not-the-right-password-99"}

HTTP 401
[Asserts]
jsonpath "$.error" == "invalid_credentials"

# === 10. Login: unknown email → 401 ===

POST {{BASE_URL}}/login
Content-Type: application/json
{"email": "nobody@candy.local", "password": "{{guest_password}}"}

HTTP 401
[Asserts]
jsonpath "$.error" == "invalid_credentials"

# === 11. Logout: happy path → 204 ===

POST {{BASE_URL}}/logout
Authorization: Bearer {{guest_login_token}}
Content-Type: application/json
{"idempotency_key": "logout-guest-login-001"}

HTTP 204

# Verify revocation: same token with a new key must now be 401.

POST {{BASE_URL}}/logout
Authorization: Bearer {{guest_login_token}}
Content-Type: application/json
{"idempotency_key": "logout-guest-revoke-check-001"}

HTTP 401

# === 12. Logout: missing bearer → 401 ===

POST {{BASE_URL}}/logout
Content-Type: application/json
{"idempotency_key": "logout-no-bearer-001"}

HTTP 401

# === 13. Logout: invalid bearer → 401 ===

POST {{BASE_URL}}/logout
Authorization: Bearer not-a-real-token
Content-Type: application/json
{"idempotency_key": "logout-bad-bearer-001"}

HTTP 401

# === 14. Logout: idempotency replay (same key → 204, spec: Revoke is idempotent) ===
# See auth.md §14 for the ordering ambiguity note.

POST {{BASE_URL}}/logout
Authorization: Bearer {{guest_login_token}}
Content-Type: application/json
{"idempotency_key": "logout-guest-login-001"}

HTTP 204

# === 15. Self-upgrade: Guest → Host ===

POST {{BASE_URL}}/me/upgrade-to-host
Authorization: Bearer {{host_token}}

HTTP 200
[Asserts]
jsonpath "$.role" == "host"

# Re-login to capture a host_login_token that reflects the new role.

POST {{BASE_URL}}/login
Content-Type: application/json
{"email": "{{host_email}}", "password": "{{host_password}}"}

HTTP 200
[Captures]
host_login_token: jsonpath "$.token"
[Asserts]
jsonpath "$.role" == "host"

# === 16. Self-upgrade: AlreadyHost → 409 ===

POST {{BASE_URL}}/me/upgrade-to-host
Authorization: Bearer {{host_login_token}}

HTTP 409
[Asserts]
jsonpath "$.error" == "already_host"

# === 17. Self-upgrade: wrong role (Admin tries) → 403 ===
# RoleGated(Guest) on this route is an exact/ceiling guard — Admin is above
# Guest in the hierarchy and this self-promotion route is Guest-only.
# See auth.md §17 for the judgment call on RoleGated semantics.

POST {{BASE_URL}}/me/upgrade-to-host
Authorization: Bearer {{first_admin_token}}

HTTP 403

# === 18. Admin promote: Guest → Host (happy path) ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{"email": "promotee@candy.local", "password": "correct horse battery staple promotee", "idempotency_key": "signup-promotee-001"}

HTTP 201
[Captures]
promotee_user_id: jsonpath "$.user_id"

POST {{BASE_URL}}/admin/users/{{promotee_user_id}}/promote
Authorization: Bearer {{first_admin_token}}
Content-Type: application/json
{"role": "host"}

HTTP 200
[Asserts]
jsonpath "$.promoted" == true

# === 19. Admin promote: InvalidPromotion (Admin → Guest demotion) → 422 ===
# Seed a second admin via signup + promote, then attempt to demote to Guest.

POST {{BASE_URL}}/signup
Content-Type: application/json
{"email": "admin2@candy.local", "password": "correct horse battery staple admin2", "idempotency_key": "signup-admin2-001"}

HTTP 201
[Captures]
admin2_user_id: jsonpath "$.user_id"

POST {{BASE_URL}}/admin/users/{{admin2_user_id}}/promote
Authorization: Bearer {{first_admin_token}}
Content-Type: application/json
{"role": "admin"}

HTTP 200

POST {{BASE_URL}}/admin/users/{{admin2_user_id}}/promote
Authorization: Bearer {{first_admin_token}}
Content-Type: application/json
{"role": "guest"}

HTTP 422
[Asserts]
jsonpath "$.error" == "invalid_promotion"

# === 20. Admin promote: wrong role (Guest tries) → 403 ===

POST {{BASE_URL}}/login
Content-Type: application/json
{"email": "{{guest_email}}", "password": "{{guest_password}}"}

HTTP 200
[Captures]
guest_token: jsonpath "$.token"

POST {{BASE_URL}}/admin/users/{{host_user_id}}/promote
Authorization: Bearer {{guest_token}}
Content-Type: application/json
{"role": "host"}

HTTP 403

# === 21. Admin verify: happy path ===

POST {{BASE_URL}}/admin/users/{{guest_user_id}}/verify
Authorization: Bearer {{first_admin_token}}
Content-Type: application/json
{"idempotency_key": "verify-guest-001"}

HTTP 200
[Asserts]
jsonpath "$.verified" == true

# === 22. Admin verify: AlreadyVerified → 409 (new key, genuine second attempt) ===

POST {{BASE_URL}}/admin/users/{{guest_user_id}}/verify
Authorization: Bearer {{first_admin_token}}
Content-Type: application/json
{"idempotency_key": "verify-guest-002"}

HTTP 409
[Asserts]
jsonpath "$.error" == "already_verified"

# === 23. Admin verify: UserNotFound → 404 ===

POST {{BASE_URL}}/admin/users/00000000-0000-0000-0000-000000000000/verify
Authorization: Bearer {{first_admin_token}}
Content-Type: application/json
{"idempotency_key": "verify-nonexistent-001"}

HTTP 404
[Asserts]
jsonpath "$.error" == "user_not_found"

# === 24. Admin verify: wrong role (Host tries) → 403 ===

POST {{BASE_URL}}/admin/users/{{guest_user_id}}/verify
Authorization: Bearer {{host_login_token}}
Content-Type: application/json
{"idempotency_key": "verify-guest-by-host-001"}

HTTP 403

# === 25. Admin verify: idempotency replay (same key → 200, no double-effect) ===

POST {{BASE_URL}}/admin/users/{{guest_user_id}}/verify
Authorization: Bearer {{first_admin_token}}
Content-Type: application/json
{"idempotency_key": "verify-guest-001"}

HTTP 200
[Asserts]
jsonpath "$.verified" == true
