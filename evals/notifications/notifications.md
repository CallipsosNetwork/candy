# Notifications — scenario narrative

**Feature:** `notifications` (standalone)
**Spec under test:** `examples/notifications/notifications.candy`
**Hurl file:** `evals/notifications/notifications.hurl`
**Total scenarios:** 9 (4 executable, 5 deferred)

---

## Setup

Two users are bootstrapped: an Admin account (used to call the protected admin
routes) and a regular User account (Alice). Signing up Alice is also the trigger
mechanism for the one notification pathway that is HTTP-exercisable — `Signup`
emits `UserSignedUp`, which `NotificationWorker.subscribe UserSignedUp` picks up
and passes to `OnUserSignedUp`, which calls `SendEmail`. That dispatch creates a
`Notification` actor in storage. The admin scenarios then read that actor back.

No teardown. The backend starts with empty state.

---

## Bootstrap admin

The Admin account must be seeded before Alice is signed up, because Alice's
notification record is read via the admin routes. The spec assigns role `User` to
newly created accounts by default; the Admin account must be promoted out-of-band
(via a seed / migration step in the generated backend) or created via a special
bootstrap endpoint if the generated backend exposes one.

This is the same harness concern present in other eval files that use AdminGated
routes. The hurl script assumes the Admin account exists with `{{admin_email}}`
and `{{admin_password}}` before the first request runs.

---

## Auth basics [minimal]

Full auth coverage lives in `evals/auth/auth.hurl`. This file exercises only the
paths needed to obtain tokens for the scenarios below: admin login and user
signup + login. No weak-password variants, no idempotency replay, no
EmailTaken — those are in the auth eval.

---

## Scenarios

### 1. admin-login — obtain Admin bearer token

Log in as the pre-seeded Admin account.

- **Fixtures:** `{{admin_email}}`, `{{admin_password}}`
- **Expected:** 200; `token` is non-empty. Captures `admin_token`.

---

### 2. signup-alice — create User and trigger UserSignedUp

Sign up Alice. This is both the auth bootstrap for later login scenarios and
the mechanism that causes `NotificationWorker` to dispatch a welcome email via
`SendEmail`, creating a `Notification` actor in storage.

- **Fixtures:** `{{user_email}}`, `{{user_password}}`, `{{signup_user_key}}`
- **Expected:** 201; `user_id` and `token` are present. Captures `alice_user_id`
  and `alice_signup_token`.

**Side effect:** `Signup` emits `UserSignedUp { user: alice_user_id, email, at }`.
`NotificationWorker` receives the event and calls `OnUserSignedUp`, which calls
`SendEmail(email, "welcome", "UserSignedUp", alice_user_id, at, <key>)`.
`SendEmail` creates a `Notification` actor with `trigger_event: "UserSignedUp"`,
`trigger_id: alice_user_id`, `channel: Email`, `recipient: alice_email`.

The `Notification` status will be either `Sent` (if a real Postmark key is
present in the environment) or `Failed` (if no provider keys are configured and
all rescue arms exhaust). The admin read scenarios assert on the structural shape
of the record — `trigger_event`, `trigger_id`, `channel`, `recipient` — not the
delivery outcome, so both terminal states are valid.

---

### 3. login-alice — obtain User bearer token

Log in as Alice to get a token for the wrong-role tests.

- **Fixtures:** `{{user_email}}`, `{{user_password}}`
- **Expected:** 200; `token` is non-empty. Captures `alice_token`.

---

### 4. admin-get-notification — GET /admin/notifications/:id, Admin role

Retrieve the Notification record that was created during Alice's signup.
The notification id must be discovered first — this scenario queries
`GET /admin/notifications` filtered to surface the record, then reads it by id.

In practice the hurl script queries `GET /admin/notifications?status=Pending`
(or without a filter if the spec allows it) using the admin token, captures the
first result's id, and then reads it back directly. See scenario 7 for the
filtered list variant.

- **Auth:** `Authorization: Bearer {{admin_token}}`
- **Expected:** 200; response is a notification object with:
  - `trigger_event` present and non-empty
  - `channel` present (`"Email"` or `"email"` per codegen casing)
  - `recipient` present (Alice's email address)
  - `status` is one of `"Pending"`, `"Sent"`, or `"Failed"` — delivery outcome
    is not asserted because it depends on whether real provider keys are present.

---

### 5. admin-get-notification-wrong-role — GET /admin/notifications/:id, User role

Attempt the same read using Alice's User-role token.

- **Auth:** `Authorization: Bearer {{alice_token}}`
- **Expected:** 403; `{"error": "not_authorized"}` (or the codegen-canonical
  snake_case of `NotAuthorized`).

---

### 6. admin-get-notification-no-bearer — GET /admin/notifications/:id, no token

Attempt the read with no Authorization header.

- **Expected:** 401.

---

### 7. admin-list-notifications-failed — GET /admin/notifications?status=Failed, Admin

Query the filtered list for Failed notifications. When no provider keys are
configured this will return the notification created during Alice's signup.
When real keys are present the list may be empty — both outcomes are valid.

- **Auth:** `Authorization: Bearer {{admin_token}}`
- **Expected:** 200; response is an array (possibly empty).

---

### 8. admin-list-notifications-wrong-role — GET /admin/notifications?status=Failed, User

- **Auth:** `Authorization: Bearer {{alice_token}}`
- **Expected:** 403; `{"error": "not_authorized"}`.

---

## Deferred scenarios [d]

The scenarios below are documented here as behavioral contracts. They are omitted
from `notifications.hurl` because they require either a real provider sandbox or
a test-mode adapter that records calls and exposes a query endpoint — neither of
which exists yet. When the harness lands, these scenarios move to the hurl file.

---

### [d] Worker subscribes UserSignedUp → Email[Postmark]

After `UserSignedUp` is emitted, `NotificationWorker` calls `OnUserSignedUp`,
which calls `SendEmail`, which tries `Email[Postmark].Send`. With a real Postmark
sandbox key configured, the dispatch succeeds and the `Notification` actor
transitions to `Sent` with `provider_used: "Postmark"`. Asserting this requires
either:

- A test-mode adapter that records `Email[Postmark].Send` calls and exposes a
  `GET /test/email-log` query endpoint, or
- A Postmark sandbox with API access to verify the message was received.

Neither is in scope until the adapter harness ships.

---

### [d] Worker subscribes UserSignedUp → fallback to SendGrid

Postmark is configured to reject (stub returns `EmailDeclined`). `SendEmail`
falls through the rescue chain to `Email[SendGrid].Send`, which succeeds.
The `Notification` actor's `provider_used` should be `"SendGrid"`. Requires
failure-injection in the Postmark adapter stub.

---

### [d] Worker subscribes UserSignedUp → all providers fail → NotificationFailed

All four email providers reject. `SendEmail` exhausts the rescue chain and emits
`NotificationFailed`. The `Notification` actor lands in `Failed` with `attempts`
incremented for each provider tried. Requires failure-injection in all four email
adapter stubs.

---

### [d] Worker subscribes OrderShipped → Email + SMS (if phone present)

`OrderShipped` with `recipient_phone` set triggers both `SendEmail` and `SendSMS`
in `OnOrderShipped`. Two `Notification` actors are created — one for `channel:
Email` and one for `channel: SMS`. Asserting both requires a test-mode adapter
for both the Email and SMS externals.

---

### [d] Webhook backstop: Email.Delivered → Notification.MarkSent

A `Notification` in `Pending` state (dispatch succeeded but async delivery not
yet confirmed) receives an `Email.Delivered` webhook from Postmark. The
`Notification.subscribe Email.Delivered` handler checks `message_id == message`
and calls `MarkSent`. The status transitions to `Sent`. Requires a webhook
harness to inject the `Email.Delivered` event directly into the event bus, or a
Postmark test environment that sends webhooks synchronously.

---

## Coverage map

| COVERAGE.md row                                       | Scenario                              |
|-------------------------------------------------------|---------------------------------------|
| `POST /signup` / `/login` / `/logout` (auth)          | admin-login, signup-alice, login-alice |
| Worker subscribes UserSignedUp → Email[Postmark]      | **[d]** see deferred section          |
| Postmark fails → falls back to SendGrid               | **[d]** see deferred section          |
| all providers fail → NotificationFailed               | **[d]** see deferred section          |
| Worker subscribes OrderShipped → Email + SMS          | **[d]** see deferred section          |
| Webhook: Email Delivered → Notification.MarkSent      | **[d]** see deferred section          |
| `GET /admin/notifications/:id` ok (Admin) → 200       | admin-get-notification                |
| `GET /admin/notifications/:id` wrong role → 403       | admin-get-notification-wrong-role     |
| `GET /admin/notifications?status=Failed` ok → 200     | admin-list-notifications-failed       |
| `GET /admin/notifications?status=Failed` wrong role   | admin-list-notifications-wrong-role   |

---

## Judgment calls

**signup-alice as notification trigger:** The only way to produce a `Notification`
record without an external stub harness is to sign a user up, which fires
`UserSignedUp` and causes the worker to dispatch. This doubles as the user
bootstrap. The read scenarios depend on this side effect — if the worker or email
adapter is not wired in the generated backend, the admin read scenarios will fail
with 404, which is itself a conformance failure.

**Status-agnostic assertions on the notification record:** The admin read
scenarios assert structural shape (`trigger_event`, `channel`, `recipient`) but
not `status`. A backend without real provider keys will produce `Failed`; one
with a sandbox key will produce `Sent`. Both are valid. Pinning `status` would
make the test environment-dependent, which violates the reproducibility goal.

**Admin bootstrap via out-of-band seed:** The Admin account is assumed to exist
before the hurl script runs. The spec assigns role `User` by default; promoting
to Admin is a deployment-time seed concern, not something expressible in a pure
HTTP test without a pre-existing admin. This is consistent with how other eval
files in this repo handle admin bootstrap.

**`GET /admin/notifications` without status filter:** The spec declares the route
as `GET /admin/notifications -> Notification.where(status == query.status)`. If
`query.status` is absent the behavior is codegen-defined (return all, or 422).
The hurl file uses explicit `?status=Failed` to stay within the declared contract
and avoid testing undefined behavior.
