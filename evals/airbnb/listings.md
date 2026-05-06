# airbnb/listings eval scenarios

Covers `POST /listings`, `PATCH /listings/:id`, `POST /listings/:id/publish`,
`POST /listings/:id/hide`, and `GET /listings` (including the `AvailableInRange`
filter). Minute-granular slots. `HoldSlot`/`ReleaseSlot` are saga-internal
and tested indirectly (see §6 below).

---

## Setup

### 1. Bootstrap admin

The admin user must exist before listings can be created by promoted hosts. In
production harness bootstrapping this is a harness concern (the test runner
seeds an admin credential before executing this file). In the Hurl script, we
treat the admin as available via `/signup` with `admin_email` / `admin_password`
from `fixtures.env`.

> **[d]** Promoting a freshly signed-up user to Admin via
> `POST /admin/users/:id/promote` requires an existing Admin caller — a
> chicken-and-egg problem. The harness must seed the first admin out-of-band.
> In this script we assume the admin bootstrap has occurred (the `/signup` step
> for admin creates the account; the promote step is exercised in
> `evals/airbnb/auth.hurl`).

### 2. Sign up host A, sign up host B, sign up guest

Three actors:

- **host_a** — primary listing owner. Signed up, then self-upgraded to Host via
  `POST /me/upgrade-to-host`.
- **host_b** — a second Host, used for "wrong owner" 403 scenarios. Same
  upgrade path.
- **guest** — stays a Guest. Used only for role-gating scenarios.

All three sign up via `POST /signup`. Both hosts call `POST /me/upgrade-to-host`
immediately after signup. The guest does not upgrade.

---

## Scenarios

### Group 1 — Listing CRUD

#### S1: Create listing — ok (Host)

Host A posts `POST /listings` with valid body: `listing_title`,
`listing_description`, `listing_location`, `listing_price_per_minute`,
`listing_max_guests`, and `create_listing_key` from fixtures. Expects 201 with
`listing_id` in the body. The `listing_id` is captured for all subsequent
scenarios.

#### S2: Create listing — err InvalidListing (422)

Host A posts `POST /listings` with `maxGuests: 0`, violating the
`maxGuests >= 1` invariant. Expects 422 with `error: "invalid_listing"`.

#### S3: Create listing — wrong role Guest (403)

Guest posts `POST /listings` with a valid body. Expects 403 because the route
requires `RoleGated(Host)`.

#### S4: Update listing — ok (owner)

Host A patches `PATCH /listings/:id` (using `listing_id` from S1) with a new
`title`. Expects 200.

#### S5: Update listing — err InvalidUpdate (422)

Host A patches the listing with `maxGuests: 17`, violating the
`maxGuests <= 16` invariant. Expects 422 with `error: "invalid_update"`.

#### S6: Update listing — err ListingNotFound (404)

Host A patches `PATCH /listings/nonexistent-id-000`. Expects 404 with
`error: "listing_not_found"`.

#### S7: Update listing — wrong owner (other Host) → 403

Host B patches Host A's listing. Expects 403 because the `ListingOwner` policy
rejects callers who are not the listing's host (and Host B is not an Admin).

---

### Group 2 — Listing lifecycle (publish / hide)

#### S8: Publish listing — ok

Host A posts `POST /listings/:id/publish` using `publish_listing_key`. Listing
is in Draft with title and description from S1, so `DraftIncomplete` does not
fire. Expects 200.

#### S9: Publish listing — err AlreadyListed (409)

Host A replays `POST /listings/:id/publish` with a fresh idempotency key on
the same now-Listed listing. Expects 409 with `error: "already_listed"`.

Note: idempotency keys are per-request, not per-listing-state. Using a
different key is intentional here — we want to exercise the state-guard, not
the idempotency path.

#### S10: Publish listing — err DraftIncomplete (422)

Host A creates a new, incomplete listing: empty `description` string. Attempts
to publish it. Expects 422 with `error: "draft_incomplete"`.

Implementation note: the spec requires `length(description) > 0` before
publish. A listing created with `description: ""` satisfies `CreateListing`
(no invariant on description at creation) but fails `Publish`. A fresh
listing id is captured in this scenario block.

#### S11: Hide listing — ok

Host A posts `POST /listings/:id/hide` on the listing published in S8. Expects
200.

#### S12: Hide listing — err AlreadyHidden (409)

Host A posts `POST /listings/:id/hide` again on the same listing. Expects 409
with `error: "already_hidden"`.

---

### Group 3 — Listing visibility / filter

#### S13: GET /listings — ok (default Listed filter)

Before this scenario, Host A re-publishes the listing hidden in S11 (using a
new idempotency key). `GET /listings` with no filter parameter returns the
default `Listed` set. Expects 200 with a `listings` array containing at least
one entry and each entry has an `id` field.

#### S14: GET /listings — AvailableInRange filter respects holds

This is the key calendar integration test. It exercises `HoldSlot` and
`ReleaseSlot` indirectly through a route if one exists, or is deferred if not.

See §6 below.

---

### Group 4 — Validation failures (summary)

Validation failures are covered inline in Groups 1–2 above:

| Error             | Scenario | Trigger                                |
|-------------------|----------|----------------------------------------|
| `InvalidListing`  | S2       | `maxGuests: 0`                         |
| `InvalidUpdate`   | S5       | `maxGuests: 17`                        |
| `ListingNotFound` | S6       | non-existent listing id in PATCH       |
| `DraftIncomplete` | S10      | publish a listing with empty description |
| `AlreadyListed`   | S9       | publish an already-Listed listing      |
| `AlreadyHidden`   | S12      | hide an already-Hidden listing         |

---

## §6 — HoldSlot / ReleaseSlot and AvailableInRange

`HoldSlot` and `ReleaseSlot` are declared in the `listings.candy` controller
spec but are **not** exposed as public HTTP routes in the `Listings` controller.
They are exported flows consumed directly by the booking saga (another Candy
feature boundary). No `POST /listings/:id/hold` or `POST /listings/:id/release`
route exists in the spec.

Therefore:

> **[d]** `AvailableInRange` hold/release verification is deferred. It is
> exercised end-to-end by `evals/airbnb/booking.hurl` via the `POST /bookings`
> happy path, which calls `HoldSlot` internally. Once a booking exists for a
> slot, a `GET /listings?filter=AvailableInRange&from=...&to=...` issued from
> this file (after running the booking file) would confirm the exclusion.
> Because each `.hurl` file is self-contained and cannot depend on another
> file's state, the booking-saga-driven path is out of scope here.

What this file does cover for `AvailableInRange` (S14): the query itself — a
`GET /listings?filter=AvailableInRange&from=T&to=T2` against a listing with no
holds returns it in the results. This confirms the filter is wired and parses
the `from`/`to` query parameters. Time values are supplied as pre-computed
Unix epoch seconds injected by the test runner at run time (see below).

### Time-based query parameter approach

Hurl 4.x has no built-in arithmetic over `now`. Options considered:

1. **Server-returned timestamp** — capture a timestamp from a prior request
   (e.g. the `created_at` from the listing create response), add a fixed offset.
   Rejected: the spec does not guarantee `created_at` in the create response
   body, and arithmetic in captures is not supported.

2. **Pre-computed fixture variables** — `fixtures.env` is static. The test
   runner would need to rewrite it per run, which is brittle.

3. **Test-runner injection** — the standard Hurl approach: pass
   `--variable from_ts=<epoch>` and `--variable to_ts=<epoch>` on the command
   line, computed by the harness wrapper script just before invocation.

**Decision**: option 3. The `.hurl` file uses `{{from_ts}}` and `{{to_ts}}`
variables. The harness wrapper (outside this file) computes:

```sh
from_ts=$(( $(date +%s) + booking_offset_seconds ))
to_ts=$(( from_ts + booking_minutes * 60 ))
hurl --variables-file evals/airbnb/fixtures.env \
     --variable BASE_URL=... \
     --variable from_ts=$from_ts \
     --variable to_ts=$to_ts \
     evals/airbnb/listings.hurl
```

This matches the `booking_offset_seconds` and `booking_minutes` values already
in `fixtures.env`.

---

## Deferred items summary

| Tag   | Item                                                                 |
|-------|----------------------------------------------------------------------|
| `[d]` | Admin bootstrap — first admin seeded by harness out-of-band          |
| `[d]` | AvailableInRange with an active hold — no public HoldSlot route; exercised by booking.hurl |
| `[d]` | HoldSlot idempotency replay — saga-internal, no HTTP surface         |
| `[d]` | ReleaseSlot idempotency replay — saga-internal, no HTTP surface      |
