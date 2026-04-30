# wallet — money primitives, idempotency, and conservation invariants

A per-user wallet supporting topup, withdraw, and peer-to-peer transfer.
Money is integer minor units in USD; floats are forbidden by the
language. Every state change appends a journal entry, and the wallet's
balance is *derived* from the journal — there is no second copy of the
truth. Replays are safe: every flow accepts an idempotency `key`.

This is a pure in-spec example: no external payment SDKs, no providers,
no schedules. The interesting properties are the two-party transfer
saga, the journal-as-source-of-truth pattern, and the conservation
invariants that make double-spend bugs expressible as predicates.

## What this exercises

- **Branded `Money` type** — `int { unit: minor, currency: USD,
  round: nearest }` (GRAMMAR.md "type").
- **Append-only journal in actor state** — `journal: [JournalEntry]`
  with derived balance (GRAMMAR.md `derive`).
- **Predicate invariants** — `balance >= 0` and
  `balance == sum(journal.delta)` (GRAMMAR.md "invariant").
- **Idempotency keys** — `key: Key` on every replayable message and
  flow (GRAMMAR.md "Cross-cutting conventions").
- **Flow-scope policy attachment** — `TransferAtomicity` on `Transfer`
  (GRAMMAR.md "Policy attachment").
- **Saga compensation** — `Transfer` debits source, then credits
  destination; if the credit fails, the debit compensates.
- **Multi-actor flow** — `Transfer` touches two `Wallet` instances.

## Domain model

### Types

```
type Id        opaque  { max: 64 }
type Timestamp instant { tz: utc }
type Key       opaque  { max: 128 }
type Money     int     { unit: minor, currency: USD, round: nearest }

enum EntryKind { Topup, Withdrawal, TransferOut, TransferIn }

type JournalEntry {
  id:          Id
  kind:        EntryKind
  delta:       Money     // positive for credits, negative for debits
  counterpart: Id?       // peer wallet id for transfers
  key:         Key       // idempotency key for the originating flow
  at:          Timestamp
}
```

### Actors

**Wallet(id: Id)** — identified by `id`. State carries `owner: Id`,
`journal: [JournalEntry]`, and `created: Timestamp`. The balance is
*not* stored — it is `derive balance = sum(journal.delta)`. Two
invariants:

```
invariant balance >= 0
invariant balance == sum(journal.delta)
```

The second invariant is technically tautological by construction, but
declaring it makes the intent explicit and gives codegen a hook to
generate a self-check on every read in debug builds.

Accepts:

```
accepts Credit(amount: Money, kind: EntryKind, counterpart: Id?, key: Key, now: Timestamp)
  -> Result<JournalEntry, ReplayMismatch>

accepts Debit(amount: Money, kind: EntryKind, counterpart: Id?, key: Key, now: Timestamp)
  -> Result<JournalEntry, InsufficientFunds | ReplayMismatch>
```

Both are idempotent on `key`: replaying with the same key returns the
prior journal entry. `ReplayMismatch` covers the case where the same key
arrives with different parameters (a programming error in the caller).

### Flows

```
flow Topup(wallet: Id, amount: Money, key: Key, now: Timestamp)
  -> Result<JournalEntry, InvalidAmount>

flow Withdraw(wallet: Id, amount: Money, key: Key, now: Timestamp)
  -> Result<JournalEntry, InsufficientFunds | InvalidAmount>

flow Transfer(from: Id, to: Id, amount: Money, key: Key, now: Timestamp)
  -> Result<{ debit: JournalEntry, credit: JournalEntry },
            InsufficientFunds | InvalidAmount | SelfTransfer>
```

`Transfer` is the saga: debit the source wallet, then credit the
destination. If the credit fails for any reason, compensate the source
debit by issuing a reversing credit with a derived key. The
`TransferAtomicity` policy is attached at flow scope and asserts the
two-leg-or-zero-legs property.

### Controllers

| Method | Path                   | Target                                  | Auth   | Statuses                         |
|--------|------------------------|-----------------------------------------|--------|----------------------------------|
| GET    | /wallets/:id           | Wallet(id).balance                      | bearer | 200 / 404                        |
| GET    | /wallets/:id/journal   | Wallet(id).journal                      | bearer | 200                              |
| POST   | /wallets/:id/topup     | Topup(id, amount, key, now)             | bearer | 201 / 422 InvalidAmount          |
| POST   | /wallets/:id/withdraw  | Withdraw(id, amount, key, now)          | bearer | 201 / 422 / 409 InsufficientFunds|
| POST   | /transfers             | Transfer(from, to, amount, key, now)    | bearer | 201 / 422 / 409 / 422 SelfTransfer|

Bearer auth scopes reads and writes to the authenticated wallet owner.
Cross-owner reads are out of scope for this example.

### Events

```
event WalletTopped       { payload: { wallet: Id, amount: Money, at: Timestamp }, delivery: eager }
event WalletWithdrawn    { payload: { wallet: Id, amount: Money, at: Timestamp }, delivery: eager }
event WalletTransferred  { payload: { from: Id, to: Id, amount: Money, at: Timestamp }, delivery: eager }
```

All three are `eager` — at-least-once delivery. Subscribers must be
idempotent on the entry id.

### Policies

- **TransferAtomicity** — flow-scope on `Transfer`. Prose: a transfer
  either commits both legs or neither, regardless of failure mode.
  Examples cover (a) successful round-trip, (b) credit-fails-after-debit
  triggers compensation, (c) replay with same key returns the same
  result without double-applying, (d) self-transfer rejects with
  `SelfTransfer` before any state change.

### External dependencies

None. This example is intentionally substrate-only — `id` and
`database` from `preferences.candy`. The pedagogical point is that
non-trivial financial logic does not require an external SDK.

## Codegen targets

All four targets supported. Per-target idioms:

- **Go (chi)** — `Money` as `int64`; sqlc-generated journal queries.
- **Rust (axum)** — `Money` as `i64` newtype with arithmetic methods;
  `sqlx` transactions for the `Transfer` saga.
- **TypeScript (hono)** — `Money` as `bigint` (avoid Number for cents
  past 2^53); drizzle for the journal table.
- **Python (fastapi)** — `Money` as `int`; `sqlalchemy` Session for
  per-flow transactions.

## Conformance

Eval lives at `evals/wallet/wallet.hurl` (tracked by #28). Cover:

- Happy path: topup → withdraw → transfer → balances reconcile.
- Conservation: derived balance equals `sum(journal.delta)` after every
  operation.
- Insufficient funds: withdraw above balance returns 409 and does not
  append a journal entry.
- Self-transfer: rejects with `SelfTransfer` and does not touch state.
- Idempotency: replaying topup/withdraw/transfer with the same `key`
  returns the prior result; balance does not double-move.
- Atomicity: simulated credit failure on transfer leaves both wallets
  at their pre-transfer balance.

## Issue tracking

- Implementation: #6
- Eval: #28 (scaffold)

## Status

- [ ] Implementation pending
- [ ] Eval pending
