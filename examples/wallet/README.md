# wallet — admin-funded wallets, peer-to-peer transfers, scheduled transfers

A per-user wallet with role-separated funding and inlined authentication.
Auth (User + Session + Signup/Login/Logout + JWT) is inlined in `wallet.candy`
following the multi-feature standalone-project model — no `use` dependency on
a separate auth example.

Only an Admin may fund a wallet. A User may withdraw from their own wallet and
transfer peer-to-peer. Any authenticated user may schedule a future transfer;
a top-level `schedule` fires every minute and executes transfers whose
`fire_at` has passed.

Money is integer minor units in USD; floats are forbidden by the language.
Every state change appends a `JournalEntry` and the balance is *derived* from
`sum(journal.delta)` — there is no second copy of the truth.

## Permission matrix

| Operation              | Admin | User (owner) | User (non-owner) |
|------------------------|:-----:|:------------:|:----------------:|
| FundWallet             | yes   | no           | no               |
| Withdraw               | no    | yes          | no               |
| Transfer (source)      | no    | yes          | no               |
| ScheduleTransfer       | no    | yes          | no               |
| CancelScheduledTransfer| no    | yes (own)    | no               |
| GetBalance / GetJournal| own   | own          | no               |
| Promote user role      | yes   | no           | no               |

Admin does not bypass `WalletOwner`. Even an Admin cannot withdraw or transfer
from a wallet they do not own — the policy applies uniformly regardless of role.

## What this exercises

- **Inlined auth** — `actor User`, `actor Session`, `flow Signup/Login/Logout`
  with JWT-shaped sessions and argon2id hashing (GRAMMAR.md `prose`, `actor`).
- **Role enum** — `enum Role { Admin, User }` embedded in User state and
  Session payload; role gates enforced via `AdminGated` and `WalletOwner`
  policies.
- **Branded `Money` type** — `int { unit: minor, currency: USD, round: nearest }`.
- **Append-only journal** — `journal: [JournalEntry]` with derived balance.
- **AdminFund accept** — structurally separate from `Credit` so the admin path
  is unambiguous at the actor message level, not just via `EntryKind`.
- **Saga compensation** — `Transfer` debits source, credits destination; on
  credit failure a `Compensation` entry (distinct kind from `TransferIn`)
  restores the source balance.
- **Scheduled actor** — `ScheduledTransferActor` tracks each pending scheduled
  transfer as a first-class entity, queryable independently of wallet state.
- **Top-level schedule** — `schedule ExecuteScheduledTransfer ... every 1m`
  is the TIME-axis exercise: the runtime picks up every `Pending` actor whose
  `fire_at <= now` and executes the transfer (GRAMMAR.md "Scheduled flows").
- **Policy attachment at multiple scopes** — `BearerAuth` at feature scope;
  `AdminGated` at flow scope; `WalletOwner` at flow scope; `TransferAtomicity`
  at flow scope.

## Domain model

### Types

```
type Id           opaque  { max: 64 }
type Timestamp    instant { tz: utc }
type Key          opaque  { max: 128 }
type Money        int     { unit: minor, currency: USD, round: nearest }

enum Role         { Admin, User }
enum EntryKind    { Fund, Withdrawal, TransferOut, TransferIn, Compensation }
enum ScheduleStatus { Pending, Executed, Cancelled }
```

### Actors

**User(id: Id)** — email, PasswordHash, role: Role (default User), created.
Accepts `Promote(to: Role)` (admin-only at the controller route).

**Session(token: Token)** — user, role, issued, expires, revoked.
`Validate(now)` returns `{ user, role }` or rejects `SessionInvalid`.
`Revoke()` is idempotent.

**Wallet(owner: Id)** — `journal: [JournalEntry]`. Derived balance.
Invariants: `balance >= 0`, `balance == sum(journal.delta)`, all `(key, kind)`
pairs distinct. Accepts:

```
AdminFund(amount, by: Id, key, now) -> Result<JournalEntry, ReplayMismatch>
Credit(amount, kind, counterpart?, key, now) -> Result<JournalEntry, ReplayMismatch>
Debit(amount, kind, counterpart?, key, now)  -> Result<JournalEntry, InsufficientFunds | ReplayMismatch>
```

`AdminFund` always appends `kind: Fund`. It cannot be called via `Credit`.

**ScheduledTransferActor(id: Id)** — source, dest, amount, fire_at, key,
`status: ScheduleStatus = Pending`. Accepts `MarkExecuted` and `MarkCancelled`,
both guarded to `Pending` state.

### Flows

```
flow Signup(email, password, now, key)
  -> Result<{ user: Id, token: Token }, WeakPassword | EmailTaken>

flow Login(email, password, now)
  -> Result<{ user: Id, role: Role, token: Token }, InvalidCredentials>

flow Logout(token, now) -> unit

flow FundWallet(wallet: Id, amount, now, key)           [AdminGated]
  -> Result<JournalEntry, InvalidAmount | WalletNotFound>

flow Withdraw(wallet: Id, amount, now, key)             [WalletOwner]
  -> Result<JournalEntry, InsufficientFunds | InvalidAmount | WalletNotFound | NotAuthorized>

flow Transfer(from, to, amount, now, key)               [WalletOwner, TransferAtomicity]
  -> Result<{ out, in: JournalEntry }, InsufficientFunds | InvalidAmount |
            WalletNotFound | NotAuthorized | SelfTransfer>

flow ScheduleTransfer(from, to, amount, fire_at, now, key)  [WalletOwner]
  -> Result<{ schedule: Id }, InvalidAmount | WalletNotFound | NotAuthorized>

flow CancelScheduledTransfer(schedule: Id, now, key)
  -> Result<unit, ScheduleNotFound | AlreadyExecuted | NotAuthorized>

flow ExecuteScheduledTransfer(schedule: Id, now, key)
  -> Result<{ out, in: JournalEntry }, InsufficientFunds | ScheduleNotFound | AlreadyExecuted>
```

### Schedule

```candy
schedule ExecuteScheduledTransfer(schedule.id, now, generate())
  every 1m
  for any schedule in ScheduledTransferActor
  where status == Pending and fire_at <= now
```

The schedule fires every minute and collects all `ScheduledTransferActor`
instances that are `Pending` and past due. For each one it calls
`ExecuteScheduledTransfer`, which delegates to `Transfer` using the
schedule's own idempotency key — so a double-fire within the same minute
is a safe replay at the `Transfer` level.

The 1m cadence is intentional: it makes the time-axis behavior easy to
observe in local dev. A production deployment could use a lower-resolution
schedule (e.g. `every 5m`) with no semantic change.

### Events

```
UserSignedUp, UserLoggedIn, SessionRevoked        // auth lifecycle
WalletFunded                                      // admin funds wallet
WalletDebited                                     // user withdraws
TransferExecuted                                  // immediate p2p transfer
ScheduledTransferQueued                           // schedule created
ScheduledTransferExecuted                         // schedule fired and transferred
ScheduledTransferCancelled                        // schedule cancelled by owner
```

All events are `eager` (at-least-once). Subscribers must be idempotent on
entry id.

### Policies

- **PasswordStrength** — type-scope on `Password`. Length, digit, blocklist.
- **BearerAuth** — feature-scope. Validates JWT, binds `user` and `role`.
  Signup / Login override with `auth: none`.
- **AdminGated** — flow-scope on `FundWallet`; route-scope on `Promote`.
  Admin → ok, User → `NotAuthorized`.
- **WalletOwner** — flow-scope on `Withdraw`, `Transfer`, `ScheduleTransfer`.
  Caller id must equal wallet owner. Admin role does not exempt.
- **TransferAtomicity** — flow-scope on `Transfer`. Two-legs-or-zero saga with
  deterministic compensation key `key+"#compensate"`. Preserved verbatim from
  the base wallet spec, extended only to replace `TransferIn` compensation kind
  with the new `Compensation` kind for clearer journal semantics.

### Controllers

| Method | Path                            | Flow / Target                          | Auth   | Policies          |
|--------|---------------------------------|----------------------------------------|--------|-------------------|
| POST   | /signup                         | Signup                                 | none   |                   |
| POST   | /login                          | Login                                  | none   |                   |
| POST   | /logout                         | Logout                                 | bearer |                   |
| POST   | /admin/wallets/:owner/fund      | FundWallet                             | bearer | AdminGated        |
| POST   | /admin/users/:id/promote        | User(id).Promote                       | bearer | AdminGated        |
| GET    | /wallets/me                     | Wallet(self).balance                   | bearer |                   |
| GET    | /wallets/me/journal             | Wallet(self).journal                   | bearer |                   |
| POST   | /wallets/me/withdraw            | Withdraw                               | bearer |                   |
| POST   | /transfers                      | Transfer                               | bearer |                   |
| POST   | /transfers/schedule             | ScheduleTransfer                       | bearer |                   |
| POST   | /transfers/schedule/:id/cancel  | CancelScheduledTransfer                | bearer |                   |
| GET    | /transfers/schedule             | ScheduledTransferActor.where(...)      | bearer |                   |

### External dependencies

None. `preferences.candy` specifies SQLite + JWT + argon2 + a per-target
scheduler. No payment SDK, no queue.

## Codegen targets

- **Go (chi)** — `Money` as `int64`; `gocron` for the 1m schedule.
- **Rust (axum)** — `Money` as `i64` newtype; `tokio-cron-scheduler`.
- **TypeScript (hono)** — `Money` as `bigint`; `node-cron`.
- **Python (fastapi)** — `Money` as `int`; `apscheduler`.

## Conformance

Cover in evals:

- Admin can fund any wallet; non-admin funding returns 403.
- User can withdraw from own wallet; cross-owner withdraw returns 403.
- Admin withdraw on any wallet returns 403 (not an owner).
- Transfer: owner → other user succeeds; non-owner → other user returns 403.
- Self-transfer returns 422.
- Insufficient funds: withdraw / transfer above balance returns 409, no journal entry.
- Journal conservation: `sum(journal.delta) == balance` after every operation.
- Idempotency: replaying any flow with the same key returns the prior result.
- Transfer atomicity: simulated credit failure leaves both wallets at
  pre-transfer balance; Compensation entry visible in source journal.
- Schedule: ScheduleTransfer creates a Pending actor; after fire_at passes
  the schedule fires, Transfer executes, actor moves to Executed.
- CancelScheduledTransfer: Pending → Cancelled; re-cancel on Executed → 409.

## Issue tracking

- Implementation: #6
- Eval: #28 (scaffold)

## Status

- [ ] Implementation pending
- [ ] Eval pending
