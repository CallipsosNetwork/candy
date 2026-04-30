# notifications — inlined auth + event-driven fan-out to email and SMS

This example demonstrates the canonical auth pattern (signup, login, logout,
session lifecycle) inlined into a notifications feature, plus multi-provider
fan-out delivery to email and SMS channels.

Auth follows the same shape as `examples/auth/auth.candy`: JWT-backed sessions,
argon2id password hashing, a `Role` enum (`Admin`, `User`) for RBAC. Admin
users can inspect notification delivery state via the admin routes; User-role
accounts are notification recipients.

Email is delivered via a four-provider rescue chain: **Postmark → SendGrid →
Mailgun → Resend**. Postmark is the canonical default. Whether codegen wires
Postmark through its SDK or via HTTP fetch is a codegen concern, not a spec
concern — the spec declares only the provider tag. The rescue chain falls
through to the next provider on any failure and emits `NotificationFailed`
if all providers are exhausted.

SMS uses **Twilio → Vonage** (unchanged).

## What this exercises

- **Inlined canonical auth with RBAC** — `User` carries a `Role` field
  (`Admin` | `User`); `Session` carries it too so `AdminGated` can evaluate
  without a database lookup.
- **`AdminGated` policy on controller routes** — `GET /admin/notifications/:id`
  and `GET /admin/notifications` require `role == Admin`.
- **`external actor` with `providers:` for Email and SMS** — Email lists four
  providers, Postmark first; SMS lists two.
- **`Actor[Tag]` selection in flows** — `Email[Postmark]`, `Email[SendGrid]`,
  `SMS[Twilio]`, etc.
- **Rescue chains** — Postmark-first four-step email chain; Twilio-first
  two-step SMS chain.
- **`subscribe` to internal events** — `NotificationWorker` subscribes to
  `UserSignedUp`, `OrderShipped`, `BookingConfirmed`, `PasswordReset`.
- **Fan-out pattern** — one source event triggers multiple delivery attempts
  across channels.
- **Idempotency** — every `Send` carries a `key: Key`; replaying with the
  same key does not duplicate.
- **DeliveryAtLeastOnce** — every trigger event resolves to either
  `NotificationSent` or `NotificationFailed`; no silent drops.

## Domain model

### Types

```
type Id           opaque  { max: 64 }
type Timestamp    instant { tz: utc }
type Key          opaque  { max: 128 }
type Email        string  { max: 320, format: rfc5322 }
type Password     string  { ... }
type PasswordHash opaque
type Token        opaque  { max: 256 }
type Phone        string  { max: 32, format: e164 }
type MessageId    opaque  { max: 128 }

enum Role               { Admin, User }
enum Channel            { Email, SMS }
enum NotificationStatus { Pending, Sent, Failed }
```

### Auth actors

**User(id: Id)** — email, argon2id hash, role (default User), created,
verified. Invariant: email is unique.

**Session(token: Token)** — user id, role, issued, expires (7 days),
revoked flag. `Validate` checks revocation + expiry and returns `{ user, role }`.
`Revoke` is idempotent.

### Notification actors

**Notification(id: Id)** — one instance per dispatch. Tracks channel,
recipient, template, status, provider used, message id, attempt count.
`MarkSent` and `MarkFailed` are the only mutations. Subscribes to
`Email.Delivered` and `SMS.Delivered` for async webhook reconciliation.

**NotificationWorker(id: Id)** — singleton coordinator. Stateless; all
persistent tracking lives in individual Notification actors. Subscribes
to trigger events and fans out.

### Flows

**Auth:** `Signup`, `Login`, `Logout` — canonical shape from auth.candy.

**Delivery:**

```
flow SendEmail(to, template, trigger_event, trigger_id, now, key)
  -> Result<{ notification: Id, provider: string, message: MessageId }, AllProvidersFailed>

flow SendSMS(to, template, trigger_event, trigger_id, now, key)
  -> Result<{ notification: Id, provider: string, message: MessageId }, AllProvidersFailed>
```

`SendEmail` rescue chain: `Email[Postmark]` → `Email[SendGrid]` →
`Email[Mailgun]` → `Email[Resend]`. Each failed arm calls
`Notification.MarkFailed` before trying the next provider.

**Fan-out handlers:** `OnUserSignedUp`, `OnOrderShipped`,
`OnBookingConfirmed`, `OnPasswordReset` — invoked by `NotificationWorker`
subscribe blocks. All are best-effort: provider exhaustion emits
`NotificationFailed` but the subscriber pipeline is not blocked.

### Controllers

| Method | Path                          | Auth   | Policy     | Target                                |
|--------|-------------------------------|--------|------------|---------------------------------------|
| POST   | /signup                       | none   |            | Signup(email, password, now, key)     |
| POST   | /login                        | none   |            | Login(email, password, now)           |
| POST   | /logout                       | bearer |            | Logout(bearer, now)                   |
| GET    | /admin/notifications/:id      | bearer | AdminGated | Notification(id)                      |
| GET    | /admin/notifications          | bearer | AdminGated | Notification.where(status == status)  |

Admin routes require `role == Admin`. The `BearerAuth` check runs first
(from the prose-level `policies:` list); `AdminGated` runs after.

### Events

Auth events:

```
event UserSignedUp   { payload: { user: Id, email: Email, at: Timestamp }, delivery: eager }
event UserLoggedIn   { payload: { user: Id, at: Timestamp },               delivery: eager }
event UserVerified   { payload: { user: Id, at: Timestamp },               delivery: eager }
event SessionRevoked { payload: { token: Token, user: Id, at: Timestamp }, delivery: eager }
```

Notification events:

```
event NotificationSent   { payload: { notification: Id, channel: Channel, provider: string, at: Timestamp }, delivery: eager }
event NotificationFailed { payload: { notification: Id, channel: Channel, attempts: int, at: Timestamp },    delivery: strict }
```

`NotificationFailed` is `strict` because it may trigger alerts or
compensating actions upstream.

### Policies

- **PasswordStrength** — argon2id hashing; blocklist check; length + complexity rules.
- **BearerAuth** — feature-scoped; all authenticated routes go through this.
- **AdminGated** — controller-scoped on the Notifications controller.
- **DeliveryAtLeastOnce** — feature-scoped; asserts no silent drops.

### External dependencies

**`external actor Email` with `providers: [Postmark, SendGrid, Mailgun, Resend]`.**

Postmark is first in the list and first in the rescue chain. The spec does not
prescribe SDK vs HTTP-API — that is a codegen concern. Each provider has a
separate `config` block with its own secrets.

```
accepts Send(to: Email, subject: string, body: string, key: Key)
  -> Result<MessageId, EmailDeclined | NetworkError | RateLimited>

emits Delivered { message: MessageId, at: Timestamp }
emits Bounced   { message: MessageId, reason: string, at: Timestamp }
```

**`external actor SMS` with `providers: [Twilio, Vonage]`.**

```
accepts Send(to: Phone, body: string, key: Key)
  -> Result<MessageId, SMSDeclined | NetworkError | RateLimited>

emits Delivered { message: MessageId, at: Timestamp }
emits Failed    { message: MessageId, reason: string, at: Timestamp }
```

## Codegen targets

All four targets supported. Per-target idioms:

- **Go (chi)** — `postmark-go` for Postmark; official Go SDKs for other
  providers; `asynq` for background dispatch; `golang-jwt` + `argon2` for auth.
- **Rust (axum)** — `postmark` crate; provider clients behind a trait;
  `jsonwebtoken` + `argon2` for auth; tokio tasks for fan-out.
- **TypeScript (hono)** — `postmark` npm package; BullMQ for the dispatch
  queue; `jsonwebtoken` + `argon2` for auth; ESM only.
- **Python (fastapi)** — `postmarker`; Celery for the dispatch queue;
  `pyjwt` + `argon2-cffi` for auth.

## Status

- [ ] Implementation pending
- [ ] Eval pending
