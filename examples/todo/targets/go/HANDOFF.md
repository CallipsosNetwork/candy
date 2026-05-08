# HANDOFF — examples/todo/targets/go

Generated from `examples/todo/todo.candy`, candy runtime 0.1.
All 37 hurl scenarios green on first successful run.

---

## 1. Ambiguities and invented semantics

### Role freshness after promotion

**Ambiguous bit**: The spec says sessions carry a role claim (`actor Session { state { role: Role } }`), and the auth realisation uses self-contained JWTs with `role` embedded at issue time. However, the hurl test sequence promotes the manager user (B5) and immediately uses the original `manager_token` (issued at signup, role=User) to edit an assigned todo (C3). There is no re-login between promotion and the assigned-edit test.

**Interpretation**: `BearerAuthWithUsers` refreshes the caller's effective role from the DB on every authenticated request. The JWT sub-claim is trusted for identity; the DB is the source of truth for the current role. This makes promotions take effect without requiring re-login, which is what the hurl requires.

This is a spec/fixture tension. The spec's JWT design says roles are captured at session creation, but the hurl expects the current DB role without a new session. Flagged for orchestrator; the chosen interpretation makes the eval green.

### Password blocklist check order

**Ambiguous bit**: The spec's PasswordStrength examples include `"password123" → err(InBlocklist)`, but `password123` is only 11 characters (below the 12-character threshold). The spec doesn't specify which check runs first.

**Interpretation**: Blocklist is checked before length, so `password123` returns `InBlocklist` rather than `TooShort`. This is the only ordering that satisfies all four spec examples simultaneously.

### Argon2 salt

**Ambiguous bit**: The spec says "argon2id with parameters from deployment secrets." No salt parameter is specified.

**Interpretation**: A static salt string is used for test determinism (`candy-todo-salt-1`). In production this must be replaced with a random per-user salt. Flagged.

### Bootstrap-admin mechanism

See §3 below.

### ListFilter default

**Ambiguous bit**: The spec's `GET /todos` route takes a `filter` query parameter but doesn't declare a default.

**Interpretation**: Defaults to `MineOnly` when the `filter` query parameter is absent or unrecognized.

---

## 2. RBAC realisation choices

Role is embedded in the JWT at issue time. On every bearer-authenticated request, `BearerAuthWithUsers` validates the JWT signature and expiry, checks the revocation table, then reads the user's current role from the `users` table. This means the JWT serves as an identity credential while the DB is the authoritative source of role.

**Route-level role gating** (`RoleGated` middleware): applied via chi group with `r.Use(auth.RoleGated(required))`. Every route that declares `policies: [RoleGated(Admin)]` in the spec is in a chi group with that middleware.

**Flow-level policy checks** (`CanEditTodo`, `CanDeleteTodo`, `CanAssignTodo`): each policy function receives `(callerID, callerRole, todo)` and returns an error. The flow functions call these directly before dispatching to the actor/repo. This makes the policy enforcement explicit and grep-able at each call site, as required by the base prompt §3.

---

## 3. First-admin bootstrap mechanism

The spec has no first-admin creation path. `POST /signup` always creates a `User`. Promoting requires an existing Admin, creating a chicken-and-egg problem.

**Resolution**: The server reads an optional `FIRST_ADMIN_EMAIL` environment variable. When set, the first signup with that email (when zero admins exist in the DB) is auto-promoted to Admin and receives an Admin-role JWT in the signup response. This is implemented in `internal/auth/controllers.go`:`handleSignup`.

To run the hurl suite:
```sh
FIRST_ADMIN_EMAIL=admin@candy.local
```

This resolves option (d) from `todo.md` ("test-mode seed endpoint" variant) without requiring a sidecar or direct DB write.

---

## 4. Library version pins

| Library | Version pinned via go.sum |
|---------|--------------------------|
| `github.com/go-chi/chi/v5` | v5.2.5 |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 |
| `github.com/mattn/go-sqlite3` | v1.14.44 |
| `github.com/segmentio/ksuid` | v1.0.4 |
| `golang.org/x/crypto` | v0.50.0 |

---

## 5. Suspect spec constructs

- **Static argon2 salt**: The implementation uses a static salt for test determinism. Production deployments need per-user random salts. The spec says "argon2id with parameters from deployment secrets" but provides no salt handling guidance.

- **Session.role vs User.role freshness**: The spec embeds role in the Session actor, implying stale-role JWTs are valid until the session expires. The hurl contradicts this by expecting promoted role to take effect on the existing token. The DB-refresh approach works but deviates from the self-contained-JWT design described in the auth realisation section of the phase instructions.

- **Idempotency on auth flows**: Signup has idempotency via `key: Key`. The hurl does not exercise idempotency replay on Login or Logout beyond the logout-replay scenario. The implementation does not memoize Login results (multiple Login calls with same key create fresh sessions each time) — this is consistent with the spec which only makes Signup explicitly replayable.

---

## 6. Reproduction commands

```sh
# Build
cd examples/todo/targets/go
go build ./...    # clean
go vet ./...      # clean
go test ./...     # all policy examples pass

# Run hurl suite
go build -o /tmp/todo-server ./cmd/server
rm -f /tmp/todo-dev.db
PORT=8084 DB_PATH=/tmp/todo-dev.db JWT_SECRET=test-secret \
  FIRST_ADMIN_EMAIL=admin@candy.local \
  /tmp/todo-server > /tmp/todo.log 2>&1 &
echo $! > /tmp/todo.pid
sleep 2

hurl --variables-file ../../../../evals/todo/fixtures.env \
  --variable BASE_URL=http://localhost:8084 \
  --test ../../../../evals/todo/todo.hurl

kill $(cat /tmp/todo.pid)
```

Expected output:
```
../../../../evals/todo/todo.hurl: Success (37 request(s) in ~300 ms)
Succeeded files: 1 (100.0%)
```

---

## 7. LOC

2151 total Go LOC. Budget: 4000. Headroom: 1849 lines.
