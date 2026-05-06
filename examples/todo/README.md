# todo

Todo CRUD with three-role RBAC (admin / manager / user). Inlines auth — signup, login, logout, JWT sessions — because each candy example is a standalone project. Absorbs the scope of the now-deleted standalone `rbac` example.

## What it demonstrates

- `policy` blocks as the primary RBAC primitive — no external policy engine (Casbin, Oso, Casl). Permission rules live in candy-native `policy` blocks with `examples:` that cover every cell of the role matrix.
- Flow-scoped policy attachment (`policies: [CanEditTodo]` inside a flow declaration).
- Route-scoped `RoleGated(Admin)` layered on top of feature-scoped `BearerAuth`.
- `creator_role` captured at create time so delete protection survives role changes.
- Manager assignment per-todo, not per-user pair.

## Permission matrix

| Action            | Admin | Manager (assigned) | Manager (not assigned) | User (owner) | User (not owner) |
|-------------------|-------|--------------------|------------------------|--------------|------------------|
| Create todo       | ✅    | ✅                 | ✅                     | ✅           | ✅               |
| Edit any todo     | ✅    | ✅                 | ❌                     | —            | —                |
| Edit own todo     | ✅    | ✅                 | ✅                     | ✅           | ❌               |
| Delete any todo   | ✅    | ❌                 | ❌                     | —            | —                |
| Delete own todo   | ✅    | ❌                 | ❌                     | ✅ *         | ❌               |
| Assign todo       | ✅    | ❌                 | ❌                     | ❌           | ❌               |
| List all todos    | ✅    | ❌                 | ❌                     | ❌           | ❌               |

\* A user cannot delete their own todo if it was originally created by an admin (`creator_role = Admin`).

## Actors

- **User** — holds role (Admin / Manager / User). Accepts `Verify` and `Promote`.
- **Session** — JWT-backed; carries `user` id and `role`. Accepts `Validate` and `Revoke`.
- **Todo** — the core entity. State includes `owner`, `creator_role`, and `assigned_to?`.

## Policies

| Policy            | Scope    | Rule summary                                              |
|-------------------|----------|-----------------------------------------------------------|
| `PasswordStrength` | type    | length ≥ 12, letter + digit, not blocklisted              |
| `BearerAuth`      | feature  | Validates JWT; binds session.user + session.role          |
| `RoleGated`       | route    | Parameterized minimum-role gate (User < Manager < Admin)  |
| `CanEditTodo`     | flow     | Owner OR admin OR assigned manager                        |
| `CanDeleteTodo`   | flow     | Admin always; owner only when creator_role != Admin; manager never |
| `CanAssignTodo`   | flow     | Admin only                                                |

## Routes

```
POST   /signup                      — create account (no auth)
POST   /login                       — authenticate, get JWT (no auth)
POST   /logout                      — revoke session

POST   /admin/users/:id/promote     — change a user's role [Admin]

POST   /todos                       — create todo [any authenticated]
PATCH  /todos/:id                   — edit todo text [CanEditTodo]
POST   /todos/:id/toggle            — flip done flag [CanEditTodo]
DELETE /todos/:id                   — delete todo [CanDeleteTodo]
GET    /todos?filter=...            — list todos (All | MineOnly | AssignedToMe)

POST   /admin/todos/:id/assign      — assign todo to a manager [Admin]
```

## Preferences

SQLite + JWT + argon2. Same library choices as `examples/auth` — lightweight enough for eval/dev; swap to Postgres at the database layer for production without touching the spec.
