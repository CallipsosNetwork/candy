# rbac — role-based access control via a swappable external library

A small access-control example. Users are granted roles; roles carry
permissions over Resources. Authorization decisions are delegated to a
third-party RBAC library — Casbin, Oso, or Casl — and the candy spec
declares the contract abstractly so the library is one preference edit
away from being swapped. The interesting property: the policy data lives
in candy actors; the policy *evaluation* lives in the external library.

This is the canonical demonstration of `external actor` with
`providers:` and the `Actor[Tag]` selector — the smallest spec that
exercises the multi-provider grammar end to end.

## What this exercises

- **`external actor` with `providers:`** — three named providers
  (Casbin, Oso, Casl) declared in one block (GRAMMAR.md "Multiple
  providers").
- **`Actor[Tag]` selector at call sites** — flows pick a provider per
  call (GRAMMAR.md "Multiple providers").
- **`prose` block with `uses:`** — declares the external dependency and
  the specific operations consumed (GRAMMAR.md "prose").
- **Policy attachment at controller scope and route scope** — admin-only
  routes gated by `RoleGated` (GRAMMAR.md "Policy attachment").
- **Resource actor with state** — Resources are owned by Users; the spec
  models ownership and creation.
- **Audit on User** — every grant and revoke appends to an
  append-only audit list (GRAMMAR.md `audit`).
- **Idempotency** — `Grant` and `Revoke` accept `key: Key` so replays
  are safe.

## Domain model

### Types

```
type Id            opaque  { max: 64 }
type Timestamp     instant { tz: utc }
type Key           opaque  { max: 128 }
type ResourceId    opaque  { max: 64 }
type ResourceKind  string  { max: 64 }    // e.g. "document", "project"

enum Role        { Admin, Editor, Viewer }
enum Permission  { Read, Write, Delete, Share }
enum Action      { Read, Write, Delete, Share }
```

### Actors

**User(id: Id)** — identified by `id`. State carries the set of
`Role`s currently assigned to the user and the user's `created`
timestamp. Audit log records every grant and revoke (subject, role,
actor that performed the change, timestamp). Invariant: a user holds
each role at most once.

**Resource(id: ResourceId)** — identified by `id`. State carries
`owner: Id`, `kind: ResourceKind`, and `created: Timestamp`. The
resource's owner is implicitly granted full permissions; the external
RBAC library is the source of truth for everyone else. Invariant:
`owner` is set at creation and never changes.

### Flows

```
flow Grant(target: Id, role: Role, now: Timestamp, key: Key)
  -> Result<unit, AlreadyGranted | NotAuthorized>

flow Revoke(target: Id, role: Role, now: Timestamp, key: Key)
  -> Result<unit, NotGranted | NotAuthorized>

flow Check(subject: Id, action: Action, resource: ResourceId)
  -> Result<Allowed, Denied>

flow CreateResource(owner: Id, kind: ResourceKind, now: Timestamp, key: Key)
  -> Result<ResourceId, InvalidKind>
```

- `Grant` and `Revoke` are admin-only (enforced by `RoleGated` on the
  route). Both update the User actor's role set, push to the audit log,
  and call `RBAC[Casbin].AddPolicy` / `RemovePolicy` to keep the
  external policy store in sync. Both are idempotent on `key`.
- `Check` is a fast-path read. Resolves owner-implicit permission first;
  on miss, calls `RBAC[Casbin].Enforce(subject, action, resource)` and
  emits `AccessChecked`.
- `CreateResource` mints a new `ResourceId`, persists ownership, and
  registers the owner-implicit policy with the external library.

### Controllers

| Method  | Path                    | Target                                  | Auth     | Statuses                                    |
|---------|-------------------------|-----------------------------------------|----------|---------------------------------------------|
| POST    | /resources              | CreateResource(self, kind, now, key)    | bearer   | 201 / 422 InvalidKind                       |
| POST    | /users/:id/roles        | Grant(id, role, now, key)               | bearer + RoleGated(Admin) | 204 / 409 AlreadyGranted / 403 NotAuthorized |
| DELETE  | /users/:id/roles/:role  | Revoke(id, role, now, key)              | bearer + RoleGated(Admin) | 204 / 404 NotGranted / 403 NotAuthorized     |
| GET     | /check                  | Check(self, action, resource)           | bearer   | 200 { allowed: bool } / 403 Denied          |

The `RoleGated(Admin)` route-scope policy runs before dispatch and
rejects callers who do not hold the `Admin` role.

### Events

```
event RoleGranted   { payload: { actor: Id, target: Id, role: Role, at: Timestamp }, delivery: eager }
event RoleRevoked   { payload: { actor: Id, target: Id, role: Role, at: Timestamp }, delivery: eager }
event AccessChecked { payload: { subject: Id, action: Action, resource: ResourceId, allowed: bool, at: Timestamp }, delivery: eager }
```

`AccessChecked` is high-volume; subscribers are expected to sample
or batch.

### Policies

- **RoleGated** — feature-scope policy declared in the `prose` block
  and attached at the route scope on admin-only routes. Takes a required
  role; rejects callers who do not hold it. Examples cover Admin,
  Editor, and Viewer cases.

### External dependencies

**`external actor RBAC` with `providers: [Casbin, Oso, Casl]`.**

Each provider has its own `config:` block (model file path, policy
store URL, or API key depending on provider). The accepts surface is:

```
accepts Enforce(subject: Id, action: Action, object: ResourceId)
  -> Result<bool, RBACError>

accepts AddPolicy(subject: Id, role: Role, object: ResourceId?)
  -> Result<unit, RBACError>

accepts RemovePolicy(subject: Id, role: Role, object: ResourceId?)
  -> Result<unit, RBACError>
```

No `emits` — RBAC libraries are synchronous decision engines, not event
sources.

Provider selection in this example uses `RBAC[Casbin]` everywhere; the
point is that swapping to `RBAC[Oso]` is a one-token edit. A real
deployment that wanted live failover would use the rescue chain pattern
shown in GRAMMAR.md.

**Cross-target portability note.** `Casl` is JavaScript-only and is bound
only on the TypeScript target in `preferences.candy`. Calls that go
through `RBAC[Casl]` from Go/Rust/Python will fail at codegen time. Pick
`Casbin` or `Oso` for portable code paths; reserve `Casl` for TS-specific
flows.

## Codegen targets

All four targets supported. Per-target idioms:

- **Go (chi)** — Casbin via `casbin-go`; route middleware enforces
  `RoleGated`.
- **Rust (axum)** — Casbin via `casbin-rs`; tower layer for `RoleGated`.
- **TypeScript (hono)** — Casl as the abstract layer; hono middleware
  for `RoleGated`.
- **Python (fastapi)** — Casbin Python; FastAPI dependency for
  `RoleGated`.

## Conformance

Eval lives at `evals/rbac/rbac.hurl` (tracked by #28). Cover:

- Happy path: admin grants Editor → user can write a resource → admin
  revokes → user cannot write.
- Owner shortcut: resource owner can always read/write/delete their own
  resource without an explicit grant.
- Authorization: non-admin attempts to grant → 403.
- Idempotency: same `key` on `Grant` returns the same result without
  double-applying.
- Audit: every grant/revoke appears in the User's audit log.

## Issue tracking

- Implementation: #25
- Eval: #28 (scaffold)

## Status

- [ ] Implementation pending
- [ ] Eval pending
