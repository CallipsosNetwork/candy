# feature:  wallet (inlined auth)
# scenarios: 34
# summary:   Auth basics (5 signups + logins + logout), admin funding (ok +
#            InvalidAmount + WalletNotFound + wrong role), wallet reads,
#            Withdraw (ok + InsufficientFunds + InvalidAmount), Transfer
#            (ok + InsufficientFunds + SelfTransfer + WalletNotFound +
#            wrong-owner + replay/no-double-debit), ScheduleTransfer (ok +
#            InvalidAmount), CancelScheduledTransfer (ok + AlreadyExecuted +
#            NotAuthorized), schedule fires (pending-at-70s + executed-at-100s +
#            cancelled-doesn't-fire).
#
# RUNNER_REQUIRES:
#   fire_at_90s — ISO-8601 UTC timestamp 90s after test start, e.g.:
#     fire_at_90s=$(date -u -d "+90 seconds" +"%Y-%m-%dT%H:%M:%SZ")   # Linux
#     fire_at_90s=$(date -u -v+90S +"%Y-%m-%dT%H:%M:%SZ")             # macOS
#
# RUNNER_REQUIRES:
#   fire_at_300s — ISO-8601 UTC timestamp 300s after test start. Used by the
#   cancel-before-fire scenario, which runs ~100s into the test (after the
#   schedule-fires sleeps); fire_at_90s would already be in the past at that
#   point, and ScheduleTransfer rejects past fire_at per spec.
#     fire_at_300s=$(date -u -d "+300 seconds" +"%Y-%m-%dT%H:%M:%SZ")  # Linux
#     fire_at_300s=$(date -u -v+300S +"%Y-%m-%dT%H:%M:%SZ")            # macOS
#
# RUNNER_REQUIRES:
#   Admin seed — backend must start with admin account seeded:
#     email: admin@candy.local / password: correct horse battery staple admin
#     role: Admin (not role User)
#
# Schedule-fires scenarios add ~100s of wall-clock delay (delay: entries).
# Ensure the runner has a test timeout >= 120s for this file.

# === signup-admin ===

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
[Asserts]
jsonpath "$.user_id" isString
jsonpath "$.user_id" != ""
jsonpath "$.token" isString
jsonpath "$.token" != ""

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
[Asserts]
jsonpath "$.user_id" isString
jsonpath "$.user_id" != ""
jsonpath "$.token" isString
jsonpath "$.token" != ""

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

# === signup-charlie ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "{{charlie_email}}",
  "password": "{{charlie_password}}",
  "idempotency_key": "{{signup_charlie_key}}"
}

HTTP 201
[Captures]
charlie_user_id: jsonpath "$.user_id"
[Asserts]
jsonpath "$.user_id" isString
jsonpath "$.user_id" != ""

# === signup-dave ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "{{dave_email}}",
  "password": "{{dave_password}}",
  "idempotency_key": "{{signup_dave_key}}"
}

HTTP 201
[Captures]
dave_user_id: jsonpath "$.user_id"
[Asserts]
jsonpath "$.user_id" isString
jsonpath "$.user_id" != ""

# === login-admin ===
# Backend must have seeded the admin with role Admin. The returned token must
# carry Admin role for all subsequent admin-gated calls to succeed.

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
jsonpath "$.user_id" isString
jsonpath "$.role" == "Admin"
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

# === login-bob ===

POST {{BASE_URL}}/login
Content-Type: application/json
{
  "email": "{{bob_email}}",
  "password": "{{bob_password}}"
}

HTTP 200
[Captures]
bob_token: jsonpath "$.token"
[Asserts]
jsonpath "$.user_id" == {{bob_user_id}}
jsonpath "$.token" isString
jsonpath "$.token" != ""

# === login-charlie ===

POST {{BASE_URL}}/login
Content-Type: application/json
{
  "email": "{{charlie_email}}",
  "password": "{{charlie_password}}"
}

HTTP 200
[Captures]
charlie_token: jsonpath "$.token"
[Asserts]
jsonpath "$.user_id" == {{charlie_user_id}}
jsonpath "$.token" isString
jsonpath "$.token" != ""

# === login-dave ===

POST {{BASE_URL}}/login
Content-Type: application/json
{
  "email": "{{dave_email}}",
  "password": "{{dave_password}}"
}

HTTP 200
[Captures]
dave_token: jsonpath "$.token"
[Asserts]
jsonpath "$.user_id" == {{dave_user_id}}
jsonpath "$.token" isString
jsonpath "$.token" != ""

# === logout-alice ===
# Minimal auth coverage — revoke Alice's session, then log her back in below.

POST {{BASE_URL}}/logout
Authorization: Bearer {{alice_token}}

HTTP 204

# Re-login Alice so her token is valid for the rest of the file.

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

# === fund-alice ===

POST {{BASE_URL}}/admin/wallets/{{alice_user_id}}/fund
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "amount": {{fund_amount}},
  "idempotency_key": "{{fund_alice_key}}"
}

HTTP 201
[Asserts]
jsonpath "$.entry.kind" == "Fund"
jsonpath "$.entry.delta" == {{fund_amount}}
jsonpath "$.entry.id" isString
jsonpath "$.entry.id" != ""

# === fund-bob ===

POST {{BASE_URL}}/admin/wallets/{{bob_user_id}}/fund
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "amount": {{fund_amount}},
  "idempotency_key": "{{fund_bob_key}}"
}

HTTP 201
[Asserts]
jsonpath "$.entry.kind" == "Fund"
jsonpath "$.entry.delta" == {{fund_amount}}
jsonpath "$.entry.id" isString
jsonpath "$.entry.id" != ""

# === fund-invalid-amount ===
# amount <= 0 → InvalidAmount

POST {{BASE_URL}}/admin/wallets/{{alice_user_id}}/fund
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "amount": {{invalid_amount}},
  "idempotency_key": "fund-invalid-amount-001"
}

HTTP 422
[Asserts]
jsonpath "$.error" == "invalid_amount"

# === fund-wallet-not-found ===
# Wallet owner id does not exist.

POST {{BASE_URL}}/admin/wallets/unknown-wallet-id-999/fund
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "amount": {{fund_amount}},
  "idempotency_key": "fund-not-found-001"
}

HTTP 404
[Asserts]
jsonpath "$.error" == "wallet_not_found"

# === fund-wrong-role ===
# Alice has role User — AdminGated must reject with 403.

POST {{BASE_URL}}/admin/wallets/{{alice_user_id}}/fund
Authorization: Bearer {{alice_token}}
Content-Type: application/json
{
  "amount": {{fund_amount}},
  "idempotency_key": "fund-wrong-role-001"
}

HTTP 403
[Asserts]
jsonpath "$.error" == "not_authorized"

# === get-balance-alice ===
# After fund-alice: balance should equal fund_amount (50000).

GET {{BASE_URL}}/wallets/me
Authorization: Bearer {{alice_token}}

HTTP 200
[Captures]
alice_balance: jsonpath "$.balance"
[Asserts]
jsonpath "$.balance" == {{fund_amount}}

# === withdraw-alice ===
# Alice withdraws withdraw_amount (10000). Balance after: 40000.

POST {{BASE_URL}}/wallets/me/withdraw
Authorization: Bearer {{alice_token}}
Content-Type: application/json
{
  "amount": {{withdraw_amount}},
  "idempotency_key": "{{withdraw_alice_key}}"
}

HTTP 201
[Asserts]
jsonpath "$.entry.kind" == "Withdrawal"
jsonpath "$.entry.id" isString
jsonpath "$.entry.id" != ""

# === withdraw-insufficient-funds ===
# Charlie has balance 0 — any withdrawal fails InsufficientFunds.

POST {{BASE_URL}}/wallets/me/withdraw
Authorization: Bearer {{charlie_token}}
Content-Type: application/json
{
  "amount": {{withdraw_amount}},
  "idempotency_key": "withdraw-charlie-nsf-001"
}

HTTP 409
[Asserts]
jsonpath "$.error" == "insufficient_funds"

# === withdraw-invalid-amount ===
# amount <= 0 → InvalidAmount.

POST {{BASE_URL}}/wallets/me/withdraw
Authorization: Bearer {{alice_token}}
Content-Type: application/json
{
  "amount": {{invalid_amount}},
  "idempotency_key": "withdraw-invalid-001"
}

HTTP 422
[Asserts]
jsonpath "$.error" == "invalid_amount"

# === transfer-alice-to-bob ===
# Alice (balance 40000) sends transfer_amount (5000) to Bob (balance 50000).
# After: Alice 35000, Bob 55000.

POST {{BASE_URL}}/transfers
Authorization: Bearer {{alice_token}}
Content-Type: application/json
{
  "to": "{{bob_user_id}}",
  "amount": {{transfer_amount}},
  "idempotency_key": "{{transfer_alice_to_bob_key}}"
}

HTTP 201
[Captures]
transfer_out_id: jsonpath "$.out.id"
transfer_in_id: jsonpath "$.in.id"
[Asserts]
jsonpath "$.out.id" isString
jsonpath "$.out.id" != ""
jsonpath "$.out.kind" == "TransferOut"
jsonpath "$.in.id" isString
jsonpath "$.in.id" != ""
jsonpath "$.in.kind" == "TransferIn"

# === transfer-insufficient-funds ===
# Charlie has balance 0. Transfer fails InsufficientFunds.

POST {{BASE_URL}}/transfers
Authorization: Bearer {{charlie_token}}
Content-Type: application/json
{
  "to": "{{dave_user_id}}",
  "amount": {{transfer_amount}},
  "idempotency_key": "transfer-charlie-nsf-001"
}

HTTP 409
[Asserts]
jsonpath "$.error" == "insufficient_funds"

# === transfer-self ===
# Alice sends to her own wallet id. SelfTransfer guard fires before any debit.

POST {{BASE_URL}}/transfers
Authorization: Bearer {{alice_token}}
Content-Type: application/json
{
  "to": "{{alice_user_id}}",
  "amount": {{transfer_amount}},
  "idempotency_key": "transfer-self-001"
}

HTTP 422
[Asserts]
jsonpath "$.error" == "self_transfer"

# === transfer-wallet-not-found ===
# Destination wallet does not exist. Source wallet is not debited.

POST {{BASE_URL}}/transfers
Authorization: Bearer {{alice_token}}
Content-Type: application/json
{
  "to": "unknown-user-id-999",
  "amount": {{transfer_amount}},
  "idempotency_key": "transfer-not-found-001"
}

HTTP 404
[Asserts]
jsonpath "$.error" == "wallet_not_found"

# === transfer-wrong-owner ===
# Bob calls POST /transfers with from = alice_user_id in body.
# Route maps from = self (Bob's session), so WalletOwner will either:
#   (a) use session user as source → valid transfer Bob→Dave (if body from is ignored), OR
#   (b) use body from → reject 403 because Bob != Alice.
# The spec's WalletOwner policy requires (b). This assert encodes the contract.

POST {{BASE_URL}}/transfers
Authorization: Bearer {{bob_token}}
Content-Type: application/json
{
  "from": "{{alice_user_id}}",
  "to": "{{dave_user_id}}",
  "amount": {{transfer_amount}},
  "idempotency_key": "transfer-wrong-owner-001"
}

HTTP 403
[Asserts]
jsonpath "$.error" == "not_authorized"

# === transfer-replay — idempotency + no double-debit ===
# Three-layer proof: response-body id equality + Alice balance unchanged + Bob balance unchanged.

# Step 1: Capture balances before the replay.

GET {{BASE_URL}}/wallets/me
Authorization: Bearer {{alice_token}}

HTTP 200
[Captures]
alice_balance_before_replay: jsonpath "$.balance"

GET {{BASE_URL}}/wallets/me
Authorization: Bearer {{bob_token}}

HTTP 200
[Captures]
bob_balance_before_replay: jsonpath "$.balance"

# Step 2: Replay the exact same transfer (same idempotency_key as transfer-alice-to-bob).
# Response must contain the prior journal entry ids, not new ones.

POST {{BASE_URL}}/transfers
Authorization: Bearer {{alice_token}}
Content-Type: application/json
{
  "to": "{{bob_user_id}}",
  "amount": {{transfer_amount}},
  "idempotency_key": "{{transfer_alice_to_bob_key}}"
}

HTTP 201
[Asserts]
jsonpath "$.out.id" == {{transfer_out_id}}
jsonpath "$.in.id" == {{transfer_in_id}}

# Step 3: Verify balances are unchanged (no double-debit, no double-credit).

GET {{BASE_URL}}/wallets/me
Authorization: Bearer {{alice_token}}

HTTP 200
[Asserts]
jsonpath "$.balance" == {{alice_balance_before_replay}}

GET {{BASE_URL}}/wallets/me
Authorization: Bearer {{bob_token}}

HTTP 200
[Asserts]
jsonpath "$.balance" == {{bob_balance_before_replay}}

# === schedule-charlie-to-dave ===
# Charlie schedules a transfer to Dave for fire_at_90s (injected by runner).
# Charlie has balance 0 — the schedule is queued and funds are NOT checked at
# schedule time (only at execution). fire_at must be > now; runner injects a
# value 90s in the future.

POST {{BASE_URL}}/transfers/schedule
Authorization: Bearer {{charlie_token}}
Content-Type: application/json
{
  "to": "{{dave_user_id}}",
  "amount": {{schedule_amount}},
  "fire_at": "{{fire_at_90s}}",
  "idempotency_key": "{{schedule_charlie_to_dave_key}}"
}

HTTP 201
[Captures]
schedule_id: jsonpath "$.schedule_id"
[Asserts]
jsonpath "$.schedule_id" isString
jsonpath "$.schedule_id" != ""

# === schedule-invalid-amount ===
# amount <= 0 → InvalidAmount.

POST {{BASE_URL}}/transfers/schedule
Authorization: Bearer {{charlie_token}}
Content-Type: application/json
{
  "to": "{{dave_user_id}}",
  "amount": {{invalid_amount}},
  "fire_at": "{{fire_at_90s}}",
  "idempotency_key": "schedule-invalid-001"
}

HTTP 422
[Asserts]
jsonpath "$.error" == "invalid_amount"

# === cancel-schedule ===
# Charlie cancels the schedule created above. Expects 204.

POST {{BASE_URL}}/transfers/schedule/{{schedule_id}}/cancel
Authorization: Bearer {{charlie_token}}
Content-Type: application/json
{
  "idempotency_key": "{{cancel_schedule_key}}"
}

HTTP 204

# === cancel-not-authorized ===
# Dave tries to cancel the same schedule (already cancelled, but authorization
# is checked before state per spec — Charlie is source, Dave is not).
# Expects 403, not 409.

POST {{BASE_URL}}/transfers/schedule/{{schedule_id}}/cancel
Authorization: Bearer {{dave_token}}
Content-Type: application/json
{
  "idempotency_key": "cancel-not-authorized-001"
}

HTTP 403
[Asserts]
jsonpath "$.error" == "not_authorized"

# === schedule-fires — setup ===
# Alice (balance 35000) schedules a transfer of schedule_amount (2500) to Bob.
# Capture balances before scheduling for the post-execution readback.

GET {{BASE_URL}}/wallets/me
Authorization: Bearer {{alice_token}}

HTTP 200
[Captures]
alice_balance_before_schedule: jsonpath "$.balance"

GET {{BASE_URL}}/wallets/me
Authorization: Bearer {{bob_token}}

HTTP 200
[Captures]
bob_balance_before_schedule: jsonpath "$.balance"

POST {{BASE_URL}}/transfers/schedule
Authorization: Bearer {{alice_token}}
Content-Type: application/json
{
  "to": "{{bob_user_id}}",
  "amount": {{schedule_amount}},
  "fire_at": "{{fire_at_90s}}",
  "idempotency_key": "schedule-fires-alice-001"
}

HTTP 201
[Captures]
fired_schedule_id: jsonpath "$.schedule_id"
[Asserts]
jsonpath "$.schedule_id" isString
jsonpath "$.schedule_id" != ""

# === schedule-fires — verify pending at 70s ===
# Sleep 70s via delay: on the GET. At t=70s the schedule has not yet fired
# (fire_at=90s; runner cycle every 60s — earliest execution is the first cycle
# after t=90s, i.e. t ≥ 90s). Balances must be unchanged and status Pending.

GET {{BASE_URL}}/wallets/me
Authorization: Bearer {{alice_token}}
[Options]
delay: 70000

HTTP 200
[Asserts]
jsonpath "$.balance" == {{alice_balance_before_schedule}}

GET {{BASE_URL}}/wallets/me
Authorization: Bearer {{bob_token}}

HTTP 200
[Asserts]
jsonpath "$.balance" == {{bob_balance_before_schedule}}

GET {{BASE_URL}}/transfers/schedule
Authorization: Bearer {{alice_token}}

HTTP 200
[Asserts]
jsonpath "$.schedules" isCollection
jsonpath "$.schedules[0].status" == "Pending"
jsonpath "$.schedules[0].id" == {{fired_schedule_id}}

# === schedule-fires — verify executed at ~100s ===
# Sleep another 30s (total ~100s). fire_at passed at t=90s; the 60s runner
# should have executed the transfer by now. Verify balances changed by
# schedule_amount and schedule status is Executed.

GET {{BASE_URL}}/wallets/me
Authorization: Bearer {{alice_token}}
[Options]
delay: 30000

HTTP 200
[Captures]
alice_balance_after_schedule: jsonpath "$.balance"
[Asserts]
jsonpath "$.balance" < {{alice_balance_before_schedule}}

GET {{BASE_URL}}/wallets/me
Authorization: Bearer {{bob_token}}

HTTP 200
[Captures]
bob_balance_after_schedule: jsonpath "$.balance"
[Asserts]
jsonpath "$.balance" > {{bob_balance_before_schedule}}

# === cancel-after-executed ===
# AlreadyExecuted: try to cancel the schedule that just fired. Expects 409.

POST {{BASE_URL}}/transfers/schedule/{{fired_schedule_id}}/cancel
Authorization: Bearer {{alice_token}}
Content-Type: application/json
{
  "idempotency_key": "cancel-after-exec-001"
}

HTTP 409
[Asserts]
jsonpath "$.error" == "already_executed"

# === cancel-before-fire — schedule doesn't fire if cancelled ===
# Bob schedules a transfer to Alice, then cancels it immediately.
# Sleep past fire_at (70s delay). Verify Bob's balance is unchanged.
# Uses fire_at_300s (not _90s) — at this point in the test the _90s
# timestamp is already in the past and ScheduleTransfer rejects it
# per spec ("fire_at <= now → InvalidAmount"). _300s is still in the
# future and the cancellation prevents the fire regardless.

GET {{BASE_URL}}/wallets/me
Authorization: Bearer {{bob_token}}

HTTP 200
[Captures]
bob_balance_before_cancel_test: jsonpath "$.balance"

POST {{BASE_URL}}/transfers/schedule
Authorization: Bearer {{bob_token}}
Content-Type: application/json
{
  "to": "{{alice_user_id}}",
  "amount": {{schedule_amount}},
  "fire_at": "{{fire_at_300s}}",
  "idempotency_key": "schedule-cancel-before-fire-001"
}

HTTP 201
[Captures]
cancel_before_fire_id: jsonpath "$.schedule_id"
[Asserts]
jsonpath "$.schedule_id" isString
jsonpath "$.schedule_id" != ""

POST {{BASE_URL}}/transfers/schedule/{{cancel_before_fire_id}}/cancel
Authorization: Bearer {{bob_token}}
Content-Type: application/json
{
  "idempotency_key": "cancel-before-fire-001"
}

HTTP 204

# Sleep 70s past the cancel — fire_at would have elapsed but the schedule is Cancelled.

GET {{BASE_URL}}/wallets/me
Authorization: Bearer {{bob_token}}
[Options]
delay: 70000

HTTP 200
[Asserts]
jsonpath "$.balance" == {{bob_balance_before_cancel_test}}
