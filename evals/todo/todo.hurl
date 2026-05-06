# feature: todo
# scenarios: 23
# summary: RBAC over todos — Admin / Manager / User; create, edit, delete,
#          assign, list, toggle. Auth inlined from todo.candy.
#
# Bootstrap-admin gap: admin@candy.local signs up as a User below. A real
# test harness must promote that user to Admin (via a sidecar or direct DB
# write) before running the PATCH/DELETE/assign scenarios that require
# {{admin_token}} to carry the Admin role. All such scenarios are marked
# [d] in todo.md. The .hurl includes them; skip them in CI until the
# harness ships.

# === Setup: signup admin ===

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
admin_token: jsonpath "$.token"
[Asserts]
jsonpath "$.user_id" exists
jsonpath "$.token" exists

# NOTE [d]: At this point the test harness must promote admin_user_id to
# Admin before the scenarios below can run as specified. Example sidecar:
#   candy-test-promote --user {{admin_user_id}} --role Admin --db {{TEST_DB}}
# Until that harness exists, scenarios requiring Admin privilege will fail
# the role check and return 403 instead of the expected 2xx.

# === Setup: signup manager ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "{{manager_email}}",
  "password": "{{manager_password}}",
  "idempotency_key": "{{signup_manager_key}}"
}

HTTP 201
[Captures]
manager_user_id: jsonpath "$.user_id"
manager_token: jsonpath "$.token"
[Asserts]
jsonpath "$.user_id" exists
jsonpath "$.token" exists

# === Setup: signup user ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "{{user_email}}",
  "password": "{{user_password}}",
  "idempotency_key": "{{signup_user_key}}"
}

HTTP 201
[Captures]
user_id: jsonpath "$.user_id"
user_token: jsonpath "$.token"
[Asserts]
jsonpath "$.user_id" exists
jsonpath "$.token" exists

# === Setup: signup user2 ===

POST {{BASE_URL}}/signup
Content-Type: application/json
{
  "email": "{{user2_email}}",
  "password": "{{user2_password}}",
  "idempotency_key": "{{signup_user2_key}}"
}

HTTP 201
[Captures]
user2_id: jsonpath "$.user_id"
user2_token: jsonpath "$.token"
[Asserts]
jsonpath "$.user_id" exists
jsonpath "$.token" exists

# === A1 — login happy path ===

POST {{BASE_URL}}/login
Content-Type: application/json
{
  "email": "{{user_email}}",
  "password": "{{user_password}}",
  "idempotency_key": "login-user-001"
}

HTTP 200
[Asserts]
jsonpath "$.user_id" exists
jsonpath "$.role" exists
jsonpath "$.token" exists

# === A2 — logout happy path ===

POST {{BASE_URL}}/logout
Authorization: Bearer {{user_token}}
Content-Type: application/json
{
  "idempotency_key": "logout-user-001"
}

HTTP 204

# replay — idempotent revoke returns 204 again

POST {{BASE_URL}}/logout
Authorization: Bearer {{user_token}}
Content-Type: application/json
{
  "idempotency_key": "logout-user-001"
}

HTTP 204

# re-login to get a fresh user token for subsequent scenarios

POST {{BASE_URL}}/login
Content-Type: application/json
{
  "email": "{{user_email}}",
  "password": "{{user_password}}",
  "idempotency_key": "login-user-002"
}

HTTP 200
[Captures]
user_token: jsonpath "$.token"

# === B5 / Setup — admin promotes manager [d] ===
# Requires admin_token to carry Admin role (see bootstrap-admin gap above).

POST {{BASE_URL}}/admin/users/{{manager_user_id}}/promote
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "role": "Manager",
  "idempotency_key": "{{promote_manager_key}}"
}

HTTP 200
[Asserts]
jsonpath "$.promoted" == true

# === B1 — admin creates a todo [d] ===

POST {{BASE_URL}}/todos
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "text": "{{admin_todo_text}}",
  "idempotency_key": "create-admin-todo-001"
}

HTTP 201
[Captures]
admin_todo_id: jsonpath "$.todo_id"
[Asserts]
jsonpath "$.todo_id" exists

# === C1 — manager creates own todo ===

POST {{BASE_URL}}/todos
Authorization: Bearer {{manager_token}}
Content-Type: application/json
{
  "text": "{{manager_todo_text}}",
  "idempotency_key": "create-manager-todo-001"
}

HTTP 201
[Captures]
manager_todo_id: jsonpath "$.todo_id"
[Asserts]
jsonpath "$.todo_id" exists

# === D1 — user creates own todo ===

POST {{BASE_URL}}/todos
Authorization: Bearer {{user_token}}
Content-Type: application/json
{
  "text": "{{user_todo_text}}",
  "idempotency_key": "create-user-todo-001"
}

HTTP 201
[Captures]
user_todo_id: jsonpath "$.todo_id"
[Asserts]
jsonpath "$.todo_id" exists

# === F1 — create todo with empty text (EmptyText) ===

POST {{BASE_URL}}/todos
Authorization: Bearer {{user_token}}
Content-Type: application/json
{
  "text": "",
  "idempotency_key": "create-empty-001"
}

HTTP 422
[Asserts]
jsonpath "$.error" == "empty_text"

# === F2 — idempotency replay on CreateTodo ===

POST {{BASE_URL}}/todos
Authorization: Bearer {{user_token}}
Content-Type: application/json
{
  "text": "Idempotent todo",
  "idempotency_key": "create-idem-001"
}

HTTP 201
[Captures]
idem_todo_id: jsonpath "$.todo_id"

# replay — same key, same body; must return same todo_id

POST {{BASE_URL}}/todos
Authorization: Bearer {{user_token}}
Content-Type: application/json
{
  "text": "Idempotent todo",
  "idempotency_key": "create-idem-001"
}

HTTP 201
[Asserts]
jsonpath "$.todo_id" == {{idem_todo_id}}

# === D2 — user edits own todo ===

PATCH {{BASE_URL}}/todos/{{user_todo_id}}
Authorization: Bearer {{user_token}}
Content-Type: application/json
{
  "text": "Buy oat milk",
  "idempotency_key": "update-user-todo-001"
}

HTTP 200

# === B2 — admin edits user's todo [d] ===

PATCH {{BASE_URL}}/todos/{{user_todo_id}}
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "text": "Buy oat milk (admin edit)",
  "idempotency_key": "update-admin-on-user-001"
}

HTTP 200

# === C2 — manager edits own todo ===

PATCH {{BASE_URL}}/todos/{{manager_todo_id}}
Authorization: Bearer {{manager_token}}
Content-Type: application/json
{
  "text": "Manager weekly review (updated)",
  "idempotency_key": "update-manager-own-001"
}

HTTP 200

# === B4 — admin assigns todo to manager [d] ===

POST {{BASE_URL}}/admin/todos/{{admin_todo_id}}/assign
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "manager": "{{manager_user_id}}",
  "idempotency_key": "assign-todo-001"
}

HTTP 200
[Asserts]
jsonpath "$.assigned" == true

# === C3 — manager edits assigned todo [d] ===
# Depends on B4 having set assigned_to = manager_user_id on admin_todo_id.

PATCH {{BASE_URL}}/todos/{{admin_todo_id}}
Authorization: Bearer {{manager_token}}
Content-Type: application/json
{
  "text": "Admin task (manager assigned edit)",
  "idempotency_key": "update-manager-assigned-001"
}

HTTP 200

# === C4 — manager fails to edit unassigned todo (user's todo) ===

PATCH {{BASE_URL}}/todos/{{user_todo_id}}
Authorization: Bearer {{manager_token}}
Content-Type: application/json
{
  "text": "This should fail",
  "idempotency_key": "update-manager-unassigned-001"
}

HTTP 403
[Asserts]
jsonpath "$.error" == "not_authorized"

# === D3 — user fails to edit another user's todo ===

PATCH {{BASE_URL}}/todos/{{manager_todo_id}}
Authorization: Bearer {{user_token}}
Content-Type: application/json
{
  "text": "This should also fail",
  "idempotency_key": "update-user-other-001"
}

HTTP 403
[Asserts]
jsonpath "$.error" == "not_authorized"

# === E5 — PATCH on nonexistent todo (TodoNotFound) [d] ===

PATCH {{BASE_URL}}/todos/nonexistent-todo-id-00000
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "text": "Doesn't matter",
  "idempotency_key": "update-notfound-001"
}

HTTP 404
[Asserts]
jsonpath "$.error" == "not_found"

# === E3 — PATCH missing bearer ===

PATCH {{BASE_URL}}/todos/{{user_todo_id}}
Content-Type: application/json
{
  "text": "No auth",
  "idempotency_key": "update-noauth-001"
}

HTTP 401

# === E4 — PATCH invalid bearer ===

PATCH {{BASE_URL}}/todos/{{user_todo_id}}
Authorization: Bearer not-a-real-token
Content-Type: application/json
{
  "text": "Bad auth",
  "idempotency_key": "update-badauth-001"
}

HTTP 401

# === F3 — toggle todo happy path (owner) ===

POST {{BASE_URL}}/todos/{{user_todo_id}}/toggle
Authorization: Bearer {{user_token}}
Content-Type: application/json
{
  "idempotency_key": "toggle-user-001"
}

HTTP 200
[Asserts]
jsonpath "$.done" == true

# toggle again — flips back to false

POST {{BASE_URL}}/todos/{{user_todo_id}}/toggle
Authorization: Bearer {{user_token}}
Content-Type: application/json
{
  "idempotency_key": "toggle-user-002"
}

HTTP 200
[Asserts]
jsonpath "$.done" == false

# === F4 — toggle not authorized (user2 on user's todo) ===

POST {{BASE_URL}}/todos/{{user_todo_id}}/toggle
Authorization: Bearer {{user2_token}}
Content-Type: application/json
{
  "idempotency_key": "toggle-user2-001"
}

HTTP 403
[Asserts]
jsonpath "$.error" == "not_authorized"

# === F5 — assign todo: target is User, not Manager (NotManager) [d] ===

POST {{BASE_URL}}/admin/todos/{{admin_todo_id}}/assign
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "manager": "{{user_id}}",
  "idempotency_key": "assign-notmanager-001"
}

HTTP 422
[Asserts]
jsonpath "$.error" == "not_a_manager"

# === E1 — user tries to promote (wrong role — RoleGated(Admin)) ===

POST {{BASE_URL}}/admin/users/{{user2_id}}/promote
Authorization: Bearer {{user_token}}
Content-Type: application/json
{
  "role": "Manager",
  "idempotency_key": "promote-wrongrole-001"
}

HTTP 403

# === E1b — user tries to assign todo (wrong role — RoleGated(Admin)) ===

POST {{BASE_URL}}/admin/todos/{{user_todo_id}}/assign
Authorization: Bearer {{user_token}}
Content-Type: application/json
{
  "manager": "{{manager_user_id}}",
  "idempotency_key": "assign-wrongrole-001"
}

HTTP 403

# === B6 — admin lists all todos (filter=All) [d] ===

GET {{BASE_URL}}/todos?filter=All
Authorization: Bearer {{admin_token}}

HTTP 200
[Asserts]
jsonpath "$.todos" exists
jsonpath "$.todos" isCollection

# === E2 — user tries to list all todos (filter=All — NotAuthorized) ===

GET {{BASE_URL}}/todos?filter=All
Authorization: Bearer {{user_token}}

HTTP 403
[Asserts]
jsonpath "$.error" == "not_authorized"

# === D4 — user deletes own user-created todo ===
# Uses idem_todo_id (created by user with creator_role=User).

DELETE {{BASE_URL}}/todos/{{idem_todo_id}}
Authorization: Bearer {{user_token}}
Content-Type: application/json
{
  "idempotency_key": "delete-user-own-001"
}

HTTP 204

# === D5 — user fails to delete admin-created todo [d] ===
# admin_todo_id has creator_role=Admin; user cannot delete it.

DELETE {{BASE_URL}}/todos/{{admin_todo_id}}
Authorization: Bearer {{user_token}}
Content-Type: application/json
{
  "idempotency_key": "delete-user-admin-001"
}

HTTP 403
[Asserts]
jsonpath "$.error" == "not_authorized"

# === C5 — manager fails to delete (CanDeleteTodo blocks all managers) ===

DELETE {{BASE_URL}}/todos/{{manager_todo_id}}
Authorization: Bearer {{manager_token}}
Content-Type: application/json
{
  "idempotency_key": "delete-manager-own-001"
}

HTTP 403
[Asserts]
jsonpath "$.error" == "not_authorized"

# === B3 — admin deletes any todo [d] ===

DELETE {{BASE_URL}}/todos/{{manager_todo_id}}
Authorization: Bearer {{admin_token}}
Content-Type: application/json
{
  "idempotency_key": "delete-admin-any-001"
}

HTTP 204
