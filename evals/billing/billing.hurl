# feature:  billing
# scenarios: 20 (12 runnable, 8 deferred [d])
# summary:   Auth bootstrap (Admin + Customer), plan management (CreatePlan ok +
#            wrong-role 403, DeletePlan ok), subscription lifecycle (Subscribe ok +
#            InvalidPlan, Cancel ok + AlreadyCancelled, Reactivate NotSuspended 409,
#            Reactivate ok [d]), schedule cadence (ChargeCycle fires within 60s [~]),
#            charge-success [d], charge-failure [d], RetryDue [d], EscalateOverdue [d].

# === signup-admin ===
# Bootstrap the Admin account. Role promotion is codegen-provisioned for the
# first account, or handled via a seed migration — this signup bootstraps the
# email/password identity.

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "{{admin_email}}",
  "password": "{{admin_password}}",
  "idempotency_key": "{{signup_admin_key}}"
}

HTTP 201
[Captures]
admin_user_id: jsonpath "$.user_id"
admin_signup_token: jsonpath "$.token"
[Asserts]
jsonpath "$.user_id" isString
jsonpath "$.user_id" != ""
jsonpath "$.token" isString
jsonpath "$.token" != ""

# === signup-customer ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "{{customer_email}}",
  "password": "{{customer_password}}",
  "idempotency_key": "{{signup_customer_key}}"
}

HTTP 201
[Captures]
customer_user_id: jsonpath "$.user_id"
customer_token: jsonpath "$.token"
[Asserts]
jsonpath "$.user_id" isString
jsonpath "$.user_id" != ""
jsonpath "$.token" isString
jsonpath "$.token" != ""

# === login-admin ===
# Re-login to obtain a fresh admin bearer for all plan management routes.

POST {{BASE_URL}}/login
Content-Type: application/json
{
  "email": "{{admin_email}}",
  "password": "{{admin_password}}"
}

HTTP 200
[Captures]
admin_token: jsonpath "$.token"
[Asserts]
jsonpath "$.user_id" == {{admin_user_id}}
jsonpath "$.token" isString
jsonpath "$.token" != ""

# === logout-customer ===
# Minimal auth coverage: confirm logout returns 204.
# Missing-bearer and invalid-bearer cases are covered exhaustively in auth.hurl.

POST {{BASE_URL}}/logout
Authorization: Bearer {{customer_token}}

HTTP 204

# Re-login customer so the token is live for subsequent scenarios.

POST {{BASE_URL}}/login
Content-Type: application/json
{
  "email": "{{customer_email}}",
  "password": "{{customer_password}}"
}

HTTP 200
[Captures]
customer_token: jsonpath "$.token"

# === create-plan ===
# Admin creates the 60s test plan used by all subscription scenarios.

POST {{BASE_URL}}/admin/plans
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "name": "{{plan_name}}",
  "amount": {{plan_amount}},
  "interval": {{plan_interval_seconds}},
  "retry_delay": {{plan_retry_delay_seconds}},
  "escalation_window": {{plan_escalation_window_seconds}},
  "idempotency_key": "{{create_plan_key}}"
}

HTTP 201
[Captures]
plan_id: jsonpath "$.plan_id"
[Asserts]
jsonpath "$.plan_id" isString
jsonpath "$.plan_id" != ""

# === create-plan-wrong-role ===
# Customer attempts to create a plan. AdminGated policy must reject with 403.

POST {{BASE_URL}}/admin/plans
Authorization: Bearer {{customer_token}}
Content-Type: application/json
{
  "name": "Unauthorized Plan",
  "amount": 100,
  "interval": 60,
  "retry_delay": 60,
  "escalation_window": 300,
  "idempotency_key": "create-plan-wrong-role-001"
}

HTTP 403

# === delete-plan ===
# Admin creates a throwaway plan then deletes it. Primary plan_id is preserved
# for all subscription scenarios that follow.

POST {{BASE_URL}}/admin/plans
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "name": "Throwaway Plan",
  "amount": 500,
  "interval": 120,
  "retry_delay": 120,
  "escalation_window": 600,
  "idempotency_key": "create-plan-throwaway-001"
}

HTTP 201
[Captures]
throwaway_plan_id: jsonpath "$.plan_id"

DELETE {{BASE_URL}}/admin/plans/{{throwaway_plan_id}}
Authorization: Bearer {{admin_token}}

HTTP 204

# === subscribe ===
# Customer subscribes to the 60s test plan. next_charge_date is set to now by
# the Subscribe flow, so ChargeCycle picks it up on the first schedule tick.

POST {{BASE_URL}}/subscriptions
Authorization: Bearer {{customer_token}}
Content-Type: application/json
{
  "plan": "{{plan_id}}",
  "source": "{{test_payment_method}}",
  "idempotency_key": "{{subscribe_key}}"
}

HTTP 201
[Captures]
subscription_id: jsonpath "$.subscription_id"
[Asserts]
jsonpath "$.subscription_id" isString
jsonpath "$.subscription_id" != ""

# === subscribe-invalid-plan ===
# Non-existent plan id. Subscribe must reject InvalidPlan with 422.

POST {{BASE_URL}}/subscriptions
Authorization: Bearer {{customer_token}}
Content-Type: application/json
{
  "plan": "plan-does-not-exist-000",
  "source": "{{test_payment_method}}",
  "idempotency_key": "subscribe-invalid-plan-001"
}

HTTP 422
[Asserts]
jsonpath "$.error" == "invalid_plan"

# === cancel-subscription ===
# Customer cancels the subscription created in subscribe. Status moves to Cancelled.

POST {{BASE_URL}}/subscriptions/{{subscription_id}}/cancel
Authorization: Bearer {{customer_token}}
Content-Type: application/json
{
  "idempotency_key": "{{cancel_subscription_key}}"
}

HTTP 204

# === cancel-subscription-already-cancelled ===
# Replay the cancel. AlreadyCancelled must return 409.

POST {{BASE_URL}}/subscriptions/{{subscription_id}}/cancel
Authorization: Bearer {{customer_token}}
Content-Type: application/json
{
  "idempotency_key": "cancel-already-cancelled-001"
}

HTTP 409
[Asserts]
jsonpath "$.error" == "already_cancelled"

# === reactivate-not-suspended ===
# The subscription above is Cancelled, not Suspended. Restore() rejects
# NotSuspended for any status other than Suspended.

POST {{BASE_URL}}/subscriptions/{{subscription_id}}/reactivate
Authorization: Bearer {{customer_token}}
Content-Type: application/json
{
  "source": "{{test_payment_method}}",
  "idempotency_key": "reactivate-not-suspended-001"
}

HTTP 409
[Asserts]
jsonpath "$.error" == "not_suspended"

# === reactivate-suspended [d] ===
#
# DEFERRED — requires a subscription in Suspended state. Suspended is only
# reachable after the charge-failure path runs through EscalateOverdue (three
# consecutive declines or first_failed_at + escalation_window elapsed). That
# path requires Polar sandbox forced-decline cards, which are not yet part of
# the test runner contract.
#
# When unblocked:
#   1. Subscribe a second customer with a forced-decline payment method.
#   2. Sleep through three RetryDue cycles (3 × retry_settle_window = ~390s)
#      or sleep escalation_settle_window_seconds (400s) past first_failed_at.
#   3. Read back — assert status == "Suspended".
#   4. POST /subscriptions/:id/reactivate with a fresh source.
#   5. Assert HTTP 200.
#   6. Read back — assert status == "Active", attempts == 0.

# === schedule-charge-cycle-cadence [~] ===
# Subscribe a second customer subscription so that next_charge_date <= now.
# Sleep schedule_settle_window_seconds (70s) to allow one full schedule tick
# plus a buffer. Read back and assert the schedule ran — status must be Active
# or PastDue (sandbox outcome is environment-specific; we prove firing, not result).

POST {{BASE_URL}}/subscriptions
Authorization: Bearer {{customer_token}}
Content-Type: application/json
{
  "plan": "{{plan_id}}",
  "source": "{{test_payment_method}}",
  "idempotency_key": "subscribe-cadence-001"
}

HTTP 201
[Captures]
cadence_subscription_id: jsonpath "$.subscription_id"
[Asserts]
jsonpath "$.subscription_id" isString
jsonpath "$.subscription_id" != ""

# Sleep past next_charge_date + one schedule cadence window.
# The runner injects this wait via --sleep or a runner-level hook.
# See evals/README.md "runner contract" for how sleep is handled.
GET {{BASE_URL}}/_test/sleep/{{schedule_settle_window_seconds}}

HTTP 200

GET {{BASE_URL}}/subscriptions/{{cadence_subscription_id}}
Authorization: Bearer {{customer_token}}

HTTP 200
[Asserts]
# The schedule ran. Status is Active (charge succeeded) or PastDue (charge
# declined). Both prove ChargeCycle fired — we do not pin to one outcome
# because Polar sandbox card behavior is environment-specific.
jsonpath "$.status" in ["Active", "PastDue"]
# Confirm ChargeCycle did meaningful work: either a charge id or a failure
# timestamp must be present.
jsonpath "$.last_charge != null || $.last_failed_at != null" isString

# === schedule-charge-cycle-success [d] ===
#
# DEFERRED — requires a Polar sandbox test card that guarantees a successful
# charge. The card token vocabulary is not yet stabilized in the test runner.
#
# When unblocked:
#   - Use the Polar forced-success token as source in the subscribe call.
#   - After schedule_settle_window_seconds, assert:
#       status == "Active"
#       last_charge isString && last_charge != ""
#       attempts == 0
#       next_charge_date > (original next_charge_date)   // advanced by interval

# === schedule-charge-cycle-failure [d] ===
#
# DEFERRED — requires a Polar sandbox forced-decline token.
#
# When unblocked:
#   - Use the forced-decline token as source.
#   - After schedule_settle_window_seconds, assert:
#       status == "PastDue"
#       attempts == 1
#       last_failed_at isString && last_failed_at != ""
#       first_failed_at isString && first_failed_at != ""
#       last_charge == null

# === schedule-retry-due [d] ===
#
# DEFERRED — depends on schedule-charge-cycle-failure producing a PastDue
# subscription. Requires forced-decline cards throughout the retry path.
#
# When unblocked:
#   - Starting from PastDue (attempts == 1) produced by schedule-charge-cycle-failure.
#   - Sleep retry_settle_window_seconds (130s) = retry_delay (60s) + 1m cadence.
#   - Read back:
#       If retry succeeded: status == "Active", attempts == 0
#       If retry also failed: status == "PastDue", attempts == 2
#   - In either case, updated timestamp must be later than the post-failure checkpoint,
#     proving RetryDue ran.

# === schedule-escalate-overdue [d] ===
#
# DEFERRED — requires three consecutive forced-decline RetryDue ticks to reach
# attempts == 3, or first_failed_at + escalation_window (300s) elapsed.
#
# With retry_delay = 60s and 1m schedule cadence:
#   - Failure 1 at t=0      → attempts == 1
#   - RetryDue 1 at t~=130s → attempts == 2
#   - RetryDue 2 at t~=260s → attempts == 3  (attempts >= 3 gate fires)
#   - EscalateOverdue fires on the next 1m tick after attempts >= 3.
#   - Total wall time from first failure: ~320s.
#
# Alternatively, sleep escalation_settle_window_seconds (400s) past first_failed_at
# to let the time-based gate (first_failed_at + escalation_window <= now) fire.
#
# When unblocked:
#   - After three forced-decline ticks, sleep escalation_settle_window_seconds.
#   - Read back: assert status == "Suspended".
#   - Confirm SubscriptionSuspended event was emitted exactly once
#     (via event log endpoint or readback field, when that surface exists).
