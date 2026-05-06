# airbnb/coupons — Scenario narrative

Feature: `coupons.candy` — Coupon actor, CouponEligibility policy, admin CRUD, guest validate.
Hurl: `evals/airbnb/coupons.hurl`
Issue: #11

Coupons are platform-issued discount codes. Admins create them; guests validate
them at checkout. Redemption and restoration are saga-internal (called by the
booking saga's `RedeemCoupon` / `RestoreCoupon` flows) and have no direct HTTP
surface. This eval covers the three public endpoints: `POST /admin/coupons`,
`DELETE /admin/coupons/:id`, and `GET /coupons/:code/validate`.

---

## Setup

### Bootstrap admin

The admin user must be injected out-of-band by the test harness before this file
runs. The same chicken-and-egg constraint applies here as in `auth.hurl` and
`listings.hurl`: only an existing Admin can call `/admin/coupons`, but no Admin
exists in a blank-state backend.

> **[d]** Admin bootstrap is a harness concern. The harness seeds one admin
> credential via a direct DB insert or a test-mode `/internal/seed-admin`
> endpoint and supplies its bearer token via
> `--variable first_admin_token=<value>`. The `.hurl` file is written using
> `{{first_admin_token}}` and is ready to run once the seed mechanism exists.

### Sign up a guest

The guest (`guest@candy.local` / `{{guest_password}}`) is bootstrapped inline
via `POST /signup`. The guest token is used for:

- `GET /coupons/:code/validate` happy-path and error scenarios.
- The "wrong role → 403" check on admin routes.

### Coupon seed data (from fixtures.env)

| Variable          | Value        | Purpose                         |
|-------------------|--------------|---------------------------------|
| `coupon_50pct`    | `TEST50`     | 50% coupon created in S1        |
| `coupon_100pct`   | `FULL100`    | 100% coupon created in S4       |
| `coupon_invalid`  | `NOSUCHCODE` | code that is never created      |

---

## Scenarios

### Group 1 — POST /admin/coupons

#### S1: Create coupon — ok (50% off, Admin)

Admin posts `POST /admin/coupons` with:

```json
{
  "code":    "TEST50",
  "kind":    "Percent",
  "value":   50,
  "maxUses": 100,
  "expires": "<far future timestamp>",
  "idempotency_key": "{{create_coupon_key}}"
}
```

Expects 201 with `coupon_id` in the body. Captures `coupon_50_id`.

The expires timestamp must be strictly after now; the harness must supply a
value far enough in the future that it doesn't expire during the test run.
Use `--variable expires_far=<epoch>` injected by the harness wrapper, or a
hardcoded RFC3339 date well beyond the test horizon (e.g., `2099-01-01T00:00:00Z`).

#### S2: Create coupon — ok (100% off, Admin)

Admin posts `POST /admin/coupons` with code `FULL100`, kind `Percent`, value
`100`, `maxUses: 1`, same far-future expires. Expects 201. Captures `coupon_100_id`.

`maxUses: 1` is deliberate — this coupon is used later for the Exhausted and
AlreadyRedeemed scenarios. Because no direct redemption endpoint exists (see
§Deferred below), these cases are deferred.

#### S3: Create coupon — err InvalidCoupon (negative value → 422)

Admin posts with `kind: "Percent"`, `value: -5`. Expects 422,
`error: "invalid_coupon"`. The `reason` field should indicate the bounds
violation (`"percent value must be 1–100"` or equivalent).

#### S4: Create coupon — err InvalidCoupon (expires in the past → 422)

Admin posts with `kind: "Percent"`, `value: 10`, `maxUses: 1`,
`expires: "2000-01-01T00:00:00Z"` (past). Expects 422, `error: "invalid_coupon"`.
The spec's `future` step rejects `expires <= now`.

#### S5: Create coupon — err CodeTaken (409)

Admin posts `POST /admin/coupons` again with code `TEST50` (already created in
S1) but a different idempotency key (to force the state-guard, not the
idempotency path). Expects 409, `error: "code_taken"`.

#### S6: Create coupon — wrong role (guest → 403)

Guest posts `POST /admin/coupons` with a valid body. Expects 403 because the
route carries `AdminGated` — only callers with role Admin pass.

---

### Group 2 — DELETE /admin/coupons/:id

#### S7: Delete coupon — ok (204)

Admin deletes the `FULL100` coupon (`{{coupon_100_id}}`). The coupon has no
redemptions at this point (Exhausted and AlreadyRedeemed scenarios are deferred
— see below). Expects 204 (no body).

Note: `TEST50` is intentionally kept alive for the validate scenarios in Group 3.
`FULL100` is deleted here so the delete-ok path has a coupon it can remove without
disrupting later scenarios.

#### S8: Delete coupon — err CouponNotFound (404)

Admin deletes `{{coupon_100_id}}` again (already deleted in S7). Expects 404,
`error: "coupon_not_found"`.

Alternatively, use a well-formed but non-existent UUID sentinel
(`00000000-0000-0000-0000-000000000000`). Either approach is valid; the `.hurl`
uses the re-delete approach since it avoids a second sentinel variable.

#### S9: Delete coupon — err CouponInUse (409) `[d]`

**Deferred.** `CouponInUse` fires when `length(c.redemptions) > 0`. Redemptions
are appended by the `RedeemCoupon` flow, which is called internally by the
booking saga — there is no standalone HTTP endpoint for direct redemption.

To exercise this variant a booking must be completed (which redeems the coupon).
That requires the listings and booking evals to have run and created a confirmed
booking referencing the coupon.

> **[d]** This scenario is documented here and omitted from the `.hurl`. It will
> be exercised by an integration scenario in `evals/airbnb/booking.hurl` once
> that eval's happy path lands. The dependency is:
> `booking.hurl` happy path → coupon gets redeemed → attempt delete → 409.

---

### Group 3 — GET /coupons/:code/validate

All validate scenarios are called by the guest user (`{{guest_token}}`). The
`ValidateCoupon` flow is a pure read; it does not mutate coupon state.

#### S10: Validate coupon — ok (TEST50)

Guest calls `GET /coupons/TEST50/validate`. `TEST50` exists, is not expired, is
not exhausted (100 uses, 0 redeemed), and the guest has not redeemed it.

Expects 200 with:

```json
{
  "coupon_id": "<id>",
  "kind":      "Percent",
  "value":     50
}
```

Asserts all three fields are present and `value == 50`.

#### S11: Validate coupon — err CouponNotFound (404)

Guest calls `GET /coupons/NOSUCHCODE/validate`. The code `NOSUCHCODE`
(`{{coupon_invalid}}`) was never created. Expects 404, `error: "coupon_not_found"`.

#### S12: Validate coupon — err Expired (410)

An expired coupon must exist before this scenario can be exercised. The approach:

1. Admin creates a short-lived coupon with `expires` set to a timestamp
   2 seconds in the future (using `--variable expires_soon=<epoch+2>` injected
   by the harness wrapper).
2. The `.hurl` includes a `[Options] delay: 3000` directive on the next request
   (Hurl 4.x `delay` option) to pause 3 seconds before the validate call.
3. Guest calls `GET /coupons/EXPIRES_SOON/validate`. Expects 410,
   `error: "coupon_expired"`.

**Runner concern.** The `delay` value (3000 ms) assumes the backend's clock and
the test runner's clock are in sync with at most ~1 second of drift. If the
backend uses a server-side `now` that is ahead of the test runner, the 2-second
window may expire before the `delay` fires. Prefer setting the expiry window to
5 seconds and the delay to 6 seconds if the harness environment has higher
clock drift. This is documented here rather than in the `.hurl` so the runner
can adjust `expires_soon` and the delay constant independently.

#### S13: Validate coupon — err Exhausted (410) `[d]`

**Deferred.** `Exhausted` fires when `length(redemptions) >= maxUses`. Reaching
this state requires at least one successful redemption, which requires a
completed booking. No direct redemption HTTP endpoint exists.

> **[d]** Scenario depends on the booking saga calling `RedeemCoupon`. Will be
> wired in `evals/airbnb/booking.hurl`. Setup path: create a coupon with
> `maxUses: 1`, complete one booking using it, then call validate → expect 410.

#### S14: Validate coupon — err AlreadyRedeemed (409) `[d]`

**Deferred.** `AlreadyRedeemed` fires when the requesting user's id already
appears in the coupon's redemptions journal. Same dependency as S13: requires a
completed booking that redeems the coupon under the guest's user id.

> **[d]** Setup path: complete a booking (which calls `RedeemCoupon` for the
> guest), then call validate again as the same guest → expect 409,
> `error: "already_redeemed"`.

---

## Coverage summary

| Scenario | Endpoint                      | Variant                            | Status |
|----------|-------------------------------|------------------------------------|--------|
| S1       | POST /admin/coupons           | ok → 201 (50%, 100 uses)           | live   |
| S2       | POST /admin/coupons           | ok → 201 (100%, 1 use)             | live   |
| S3       | POST /admin/coupons           | err InvalidCoupon (bad value) → 422 | live  |
| S4       | POST /admin/coupons           | err InvalidCoupon (past expiry) → 422 | live |
| S5       | POST /admin/coupons           | err CodeTaken → 409                | live   |
| S6       | POST /admin/coupons           | wrong role (Guest) → 403           | live   |
| S7       | DELETE /admin/coupons/:id     | ok → 204                           | live   |
| S8       | DELETE /admin/coupons/:id     | err CouponNotFound → 404           | live   |
| S9       | DELETE /admin/coupons/:id     | err CouponInUse → 409              | `[d]`  |
| S10      | GET /coupons/:code/validate   | ok → 200                           | live   |
| S11      | GET /coupons/:code/validate   | err CouponNotFound → 404           | live   |
| S12      | GET /coupons/:code/validate   | err Expired → 410                  | live   |
| S13      | GET /coupons/:code/validate   | err Exhausted → 410                | `[d]`  |
| S14      | GET /coupons/:code/validate   | err AlreadyRedeemed → 409          | `[d]`  |

---

## Deferred items

| Tag   | Scenario | Reason                                                                     |
|-------|----------|----------------------------------------------------------------------------|
| `[d]` | Setup    | Admin bootstrap — first admin seeded OOB by harness (`first_admin_token`)  |
| `[d]` | S9       | CouponInUse — requires active redemption from booking saga; no direct HTTP  |
| `[d]` | S13      | Exhausted — requires completed booking to push redemption count to maxUses  |
| `[d]` | S14      | AlreadyRedeemed — requires completed booking under the same guest user id   |

S9, S13, and S14 will be wired in `evals/airbnb/booking.hurl` once the booking
happy-path scenario is established. The booking eval can then hold references to
the coupon ids created here, provided the two files are run in sequence with
shared state — or booking.hurl bootstraps its own coupons.

---

## Judgment calls

**Expired test via delay.** Hurl 4.x supports `[Options] delay: <ms>` at the
entry level. This is used to sleep through a short-lived coupon's expiry window
(S12). The delay is set to 3000 ms with a 2-second expiry window. Adjust both
if clock drift in the target environment is larger than 1 second.

**FULL100 deleted before validate scenarios.** S7 deletes `FULL100`
(`coupon_100_id`) to exercise the delete-ok path. This means S13/S14 (when
they land in booking.hurl) must create their own fresh `maxUses: 1` coupon
rather than reusing `FULL100`. This is intentional: each eval bootstraps its
own state.

**CodeTaken uses a new idempotency key.** S5 deliberately uses a different
idempotency key from S1 to exercise the state-guard (CodeTaken) rather than the
idempotency path (which would silently return the S1 result). This matches the
pattern established in `listings.hurl` S9 (AlreadyListed with a fresh key).

**Validate is guest-accessible.** The spec's `GET /coupons/:code/validate`
carries only `auth: bearer` (no `AdminGated`). Any authenticated user can call
it. The wrong-role case for this endpoint is not applicable; the only auth
failure is a missing/invalid bearer (not exercised here — that's auth.hurl's
domain).
