# Todo eval — RBAC over todos, three roles

**Scenarios: 23**

Three roles — Admin, Manager, User — govern create, edit, delete, assign, and
list. This file is the narrative plan; `todo.hurl` is the executable contract.

---

## Setup

Bootstrapping four actors before the RBAC scenarios can run. The setup phase
is the most involved of any eval file because it must establish three distinct
role levels.

### Actors

| Handle    | Email                  | Role    |
|-----------|------------------------|---------|
| admin     | admin@candy.local      | Admin   |
| manager   | manager@candy.local    | Manager |
| user      | alice@candy.local      | User    |
| user2     | bob@candy.local        | User    |

### Bootstrap-admin gap (deferred — see [d] scenarios)

The spec has no "first-admin" bootstrap endpoint. `POST /signup` always
creates a User. Promoting a user requires an existing Admin caller, creating
a chicken-and-egg problem for a fresh database.

**Resolution chosen: option (d) — out-of-band provisioning.**

In the `.hurl`, `admin@candy.local` signs up as a regular user. A comment
marks the point where a real test harness must promote that user to Admin
before the test suite continues — either via a sidecar script, a test-mode
seed endpoint, or a direct DB write. Until that harness exists, all scenarios
that require `{{admin_token}}` are tagged `[d]` and will be skipped by the
runner. Once the harness ships, removing the skip flag is the only change
needed.

### Step-by-step setup sequence

1. **Signup admin** — `POST /signup` with `admin@candy.local`. Captures
   `admin_user_id` and `admin_token` (role: User at this point).
   *[d] Test harness promotes this user to Admin before step 5.*

2. **Signup manager** — `POST /signup` with `manager@candy.local`. Captures
   `manager_user_id` and `manager_token` (role: User initially).

3. **Signup user** — `POST /signup` with `alice@candy.local`. Captures
   `user_id` and `user_token`.

4. **Signup user2** — `POST /signup` with `bob@candy.local`. Captures
   `user2_id` and `user2_token`.

5. **Promote manager** — Admin calls `POST /admin/users/{{manager_user_id}}/promote`
   with `{ role: "Manager" }`. Requires `{{admin_token}}` to be Admin-level.
   This is the first deferred-harness dependency.

---

## Scenarios

### Group A — Auth basics (abbreviated)

Full coverage lives in `evals/auth/auth.hurl`. These three scenarios confirm
the inlined auth in `todo.candy` behaves identically.

**A1 — Signup happy path**
`POST /signup` returns 201 with `user_id` and `token`.

**A2 — Login happy path**
`POST /login` returns 200 with `user_id`, `role`, and `token`.

**A3 — Logout happy path**
`POST /logout` returns 204. Replaying the same idempotency key also returns 204
(idempotent revoke).

---

### Group B — Admin scenarios

**B1 — Admin creates a todo**
Admin calls `POST /todos`. Returns 201 with `todo_id`. Captures
`admin_todo_id`. The todo's `creator_role` will be Admin — this drives the
delete-protection scenarios in Group D.

**B2 — Admin edits any todo (user's todo)**
Admin calls `PATCH /todos/{{user_todo_id}}`. Returns 200. Demonstrates
admin override of CanEditTodo.

**B3 — Admin deletes any todo**
Admin calls `DELETE /todos/{{manager_todo_id}}`. Returns 204.

**B4 — Admin assigns todo to manager**
Admin calls `POST /admin/todos/{{admin_todo_id}}/assign` with
`{ manager: {{manager_user_id}} }`. Returns 200 `{ assigned: true }`.

**B5 — Admin promotes manager** *(setup scenario, also tested as RBAC)*
Admin calls `POST /admin/users/{{manager_user_id}}/promote` with
`{ role: "Manager" }`. Returns 200 `{ promoted: true }`.

**B6 — Admin lists all todos**
Admin calls `GET /todos?filter=All`. Returns 200 with a `todos` array.

---

### Group C — Manager scenarios

**C1 — Manager creates own todo**
Manager calls `POST /todos`. Returns 201. Captures `manager_todo_id`.

**C2 — Manager edits own todo**
Manager calls `PATCH /todos/{{manager_todo_id}}`. Returns 200.

**C3 — Manager edits assigned todo**
After B4 (admin assigns `admin_todo_id` to manager), manager calls
`PATCH /todos/{{admin_todo_id}}`. Returns 200 — manager is the assignee.

**C4 — Manager fails to edit unassigned todo (user's todo)**
Manager calls `PATCH /todos/{{user_todo_id}}`. Returns 403
`{ error: "not_authorized" }`. Manager has no assignment on this todo.

**C5 — Manager fails to delete**
Manager calls `DELETE /todos/{{manager_todo_id}}`. Returns 403
`{ error: "not_authorized" }`. CanDeleteTodo blocks all managers.

---

### Group D — User scenarios

**D1 — User creates own todo**
User calls `POST /todos`. Returns 201. Captures `user_todo_id`.

**D2 — User edits own todo**
User calls `PATCH /todos/{{user_todo_id}}`. Returns 200.

**D3 — User fails to edit another user's todo**
User calls `PATCH /todos/{{manager_todo_id}}`. Returns 403
`{ error: "not_authorized" }`. User is not the owner, not an admin,
and not an assigned manager.

**D4 — User deletes own user-created todo**
User calls `DELETE /todos/{{user_todo_id}}`. Returns 204. The todo's
`creator_role` is User, so CanDeleteTodo permits it.

**D5 — User fails to delete admin-created todo**
User calls `DELETE /todos/{{admin_todo_id}}`. Returns 403
`{ error: "not_authorized" }`. The todo's `creator_role` is Admin;
CanDeleteTodo blocks non-admin callers regardless of ownership.

---

### Group E — Authorization matrix extras

**E1 — User tries to promote (wrong role)**
User calls `POST /admin/users/{{user2_id}}/promote`. Returns 403.
RoleGated(Admin) blocks non-admins.

**E2 — User tries to list all todos (wrong role)**
User calls `GET /todos?filter=All`. Returns 403 `{ error: "not_authorized" }`.
ListTodos rejects non-admin callers who pass `All`.

**E3 — PATCH with missing bearer**
`PATCH /todos/{{user_todo_id}}` with no `Authorization` header. Returns 401.

**E4 — PATCH with invalid bearer**
`PATCH /todos/{{user_todo_id}}` with `Authorization: Bearer invalid`. Returns 401.

**E5 — PATCH on nonexistent todo**
Admin calls `PATCH /todos/nonexistent-todo-id`. Returns 404
`{ error: "not_found" }`.

---

### Group F — Todo-specific error variants

**F1 — CreateTodo empty text**
Any authenticated user posts `POST /todos` with `{ text: "" }`. Returns 422
`{ error: "empty_text" }`.

**F2 — CreateTodo idempotency replay**
User posts `POST /todos` with a fixed `idempotency_key`. Replays the same
request. Both responses return 201 with the same `todo_id`. A follow-up
`GET /todos?filter=MineOnly` (or list call) confirms only one todo was
created.

**F3 — ToggleTodo happy path**
Owner calls `POST /todos/{{user_todo_id}}/toggle`. Returns 200
`{ done: true }`. Toggling again returns `{ done: false }`.

**F4 — ToggleTodo not authorized**
user2 calls `POST /todos/{{user_todo_id}}/toggle`. Returns 403
`{ error: "not_authorized" }`.

**F5 — AssignTodo NotManager (target is User)**
Admin calls `POST /admin/todos/:id/assign` with `{ manager: {{user_id}} }`.
`user_id` holds the User role. Returns 422 `{ error: "not_a_manager" }`.

---

## Authorization matrix

Role × action grid. Every cell is exercised by the scenarios above.

| Action                          | Admin  | Manager (assigned) | Manager (not assigned) | User (owner) | User (non-owner) |
|---------------------------------|--------|--------------------|------------------------|--------------|------------------|
| Create todo                     | ok     | ok                 | ok                     | ok           | ok               |
| Edit todo (PATCH)               | ok     | ok (C3)            | 403 (C4)               | ok (D2)      | 403 (D3)         |
| Toggle todo                     | ok     | ok                 | 403 (F4)               | ok (F3)      | 403 (F4)         |
| Delete todo (user-created)      | ok     | 403 (C5)           | 403 (C5)               | ok (D4)      | 403              |
| Delete todo (admin-created)     | ok     | 403               | 403                     | 403 (D5)     | 403              |
| Assign todo                     | ok     | 403 (E1)           | 403 (E1)               | 403 (E1)     | 403 (E1)         |
| List all todos (filter=All)     | ok (B6)| 403 (E2)           | 403 (E2)               | 403 (E2)     | 403 (E2)         |
| Promote user                    | ok (B5)| 403 (E1)           | 403 (E1)               | 403 (E1)     | 403 (E1)         |

---

## Coverage map

Mapping each COVERAGE.md row (todo section) to the scenario that covers it:

- `POST /signup` ok → 201 ................. A1 (setup: all four signups)
- `POST /login` ok → 200 .................. A2
- `POST /logout` ok → 204 ................. A3
- `POST /logout` replay → 204 ............. A3 (second logout call)
- `POST /logout` missing bearer → 401 ..... E3 (bearer auth rule, same policy)
- `POST /logout` invalid bearer → 401 ..... E4
- `POST /admin/users/:id/promote` ok → 200 . B5 (setup promote manager)
- `POST /admin/users/:id/promote` wrong role → 403 .. E1
- `POST /todos` ok → 201 .................. B1 / C1 / D1
- `POST /todos` err EmptyText → 422 ....... F1
- `POST /todos` idempotency replay ......... F2
- `PATCH /todos/:id` ok (owner) → 200 ..... D2
- `PATCH /todos/:id` ok (admin) → 200 ..... B2
- `PATCH /todos/:id` ok (manager assigned) → 200 .. C3
- `PATCH /todos/:id` err NotAuthorized (other user) → 403 .. D3
- `PATCH /todos/:id` err NotAuthorized (manager not assigned) → 403 .. C4
- `PATCH /todos/:id` err TodoNotFound → 404 . E5
- `POST /todos/:id/toggle` ok → 200 ....... F3
- `POST /todos/:id/toggle` err NotAuthorized → 403 .. F4
- `DELETE /todos/:id` ok (owner deletes own user-todo) → 204 .. D4
- `DELETE /todos/:id` ok (admin) → 204 .... B3
- `DELETE /todos/:id` err NotAuthorized (owner deletes admin-todo) → 403 .. D5
- `DELETE /todos/:id` err NotAuthorized (manager) → 403 .. C5
- `POST /admin/todos/:id/assign` ok (Admin) → 200 .. B4
- `POST /admin/todos/:id/assign` err NotManager (target is User) → 422 .. F5
- `POST /admin/todos/:id/assign` wrong role → 403 .. E1
- `GET /todos?filter=All` ok (Admin) → 200 . B6
- `GET /todos?filter=All` err NotAuthorized (User) → 403 .. E2

---

## Deferred scenarios

`[d]` — requires admin-bootstrap harness to be in place before the test file
can run end-to-end. All scenarios in groups B, C, D, E, F that use
`{{admin_token}}` or depend on the promoted manager are deferred in this sense.
The `.hurl` file includes all scenarios; the bootstrap comment marks the gap.

No saga compensation paths apply to this spec (no multi-step sagas with
`compensate`). No scheduled flows. No webhook handlers.
