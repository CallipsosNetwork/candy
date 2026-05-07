# candy codegen — TypeScript overlay

Apply on top of `codegen-base.md`. Default framework: `hono` (HTTP).
Default runtime: Node 20+; ESM only.

There is no `typescript-best-practices` skill in v0.1, so this overlay
carries more inline idiom than the Go and Rust overlays. The
`react-best-practices` skill is **not** appropriate — that's a Next.js
performance skill, not a backend skill. Do not load it.

---

## Project layout

```
targets/typescript/
  package.json
  tsconfig.json                — strict, ESM, target ES2022.
  src/
    index.ts                   — Hono app + serve.
    <feature>/
      actors.ts
      flows.ts
      controllers.ts
      policies.ts
      events.ts
      index.ts                 — re-exports.
    shared/
      types.ts
      events.ts
      errors.ts
    runtime/
      db.ts
      scheduler.ts
      eventBus.ts
      webhooks.ts
  test/
    integration.test.ts        — hurl runner harness.
```

`package.json`: `"type": "module"`; `engines.node >= 20`.
`tsconfig.json`: `strict: true`, `noUncheckedIndexedAccess: true`,
`exactOptionalPropertyTypes: true`, `module: ESNext`,
`moduleResolution: bundler`.

---

## Block-by-block bindings

### `actor`

```ts
export type Booking = {
  id: BookingId;
  status: BookingStatus;
  // one field per state entry; readonly where the type allows.
};

export class BookingRepo {
  constructor(private db: Database) {}
  async create(init: BookingInit): Promise<Booking> { ... }
  async findById(id: BookingId): Promise<Booking | null> { ... }
  async confirm(
    id: BookingId, args: ConfirmArgs, now: Timestamp, key: IdempotencyKey,
  ): Promise<Result<ConfirmOk, ConfirmErr>> { ... }
}
```

- Module per feature.
- `<Actor>Repo` class owns reads/writes for that actor's table.
- `derive` accessors → getter methods or pure functions; never stored.
- `invariant` predicates → throw a typed error before commit.
- `audit` tables → append-only.

### `external actor`

```ts
export interface Payments {
  charge(amount: Money, source: PaymentMethod, key: IdempotencyKey)
    : Promise<Result<ChargeId, PaymentError>>;
}

export class StripePayments implements Payments { ... }
```

- Multi-provider: interface + one class per tag. A `Map<Tag, Payments>`
  registry lives in `runtime/`.
- Webhook routes: `POST /webhooks/:provider/:event`. Verify signature,
  map payload, dispatch.

### `flow`

```ts
export async function placeBooking(
  deps: Deps,
  args: PlaceBookingArgs,
  now: Timestamp,
  key: IdempotencyKey,
): Promise<Result<BookingConfirmed, BookingError>> {
  const bookingId = generateBookingId();              // pre-generated; base §5.
  const held = await deps.listings.holdDates(args.listingId, args.range, bookingId, now);
  if (!held.ok) return err('hold_failed');
  const paid = await deps.payments.charge(args.amount, args.source, key);
  if (!paid.ok) {
    await deps.listings.releaseDates(held.value);     // compensate.
    return err('payment_declined');
  }
  return ok({ id: bookingId, charge: paid.value });
}
```

- Use a `Result<T, E>` discriminated union throughout flows. Don't
  throw for declared errors; throws are reserved for runtime bugs.
- `tell` (fire-and-forget): call without `await`; ensure the runtime
  swallows rejections via a top-level handler.

### `controller`

```ts
const auth = new Hono<{ Variables: { principalId: UserId } }>();

auth.post('/signup', async (c) => {
  const body = await c.req.json<SignupBody>();
  const result = await signup(deps, body, Timestamp.nowUtc(), readIdemKey(c));
  if (!result.ok) {
    if (result.error === 'weak_password')  return c.json({ error: 'weak_password' }, 400);
    if (result.error === 'email_taken')    return c.json({ error: 'email_taken' }, 409);
  }
  return c.json(result.value, 201);
});
```

- Hono router per feature; mount under conventional path.
- `auth: bearer` → middleware that extracts and validates the bearer,
  populates `c.var.principalId`, returns 401 on missing/invalid.
- `now` is captured at the route boundary; never inside handlers.

### `policy`

Pure function returning `Result<void, ...>`:

```ts
export function passwordStrength(p: Password): Result<void, 'weak_password'> {
  if (p.length < 12) return err('weak_password');
  return ok(undefined);
}
```

- `controller`-scope policies are Hono middleware.
- `actor`/`flow`/`type` scope: explicit call before the protected op.

### `event`

```ts
export type ChargeSucceeded = {
  type: 'charge_succeeded';
  charge: ChargeId;
  at: Timestamp;
};

export interface EventBus {
  publish<E extends EventEnvelope>(ev: E): Promise<void>;
  subscribe<E extends EventEnvelope>(type: E['type'], h: (ev: E) => Promise<void>): void;
}
```

- `delivery: strict` → transactional outbox.
- `delivery: eager` → at-least-once via the chosen queue (`bullmq`,
  `bee-queue`, etc.) per preferences.
- `delivery: weak` → in-process fire-and-forget `Promise`.

### `type` and `enum`

```ts
// Branded primitives via tagged types:
declare const __brand: unique symbol;
export type Money = number & { [__brand]: 'Money' };           // integer minor units.
export type Email = string & { [__brand]: 'Email' };
export type BookingId = string & { [__brand]: 'BookingId' };

export const Money = {
  cents(n: number): Money {
    if (!Number.isInteger(n)) throw new Error('money_must_be_integer');
    return n as Money;
  },
};

export const Email = {
  parse(s: string): Result<Email, 'bad_email'> { ... },
};

export type BookingStatus = 'pending' | 'confirmed' | 'cancelled';

export type Result<T, E> =
  | { ok: true; value: T }
  | { ok: false; error: E };

export const ok = <T>(v: T): Result<T, never>      => ({ ok: true,  value: v });
export const err = <E>(e: E): Result<never, E>     => ({ ok: false, error: e });
```

- **Money is integer cents**, typed as `number & { brand }`. Generated
  arithmetic preserves the brand. Floats are forbidden.
- `decimal` types: use `decimal.js` per preferences (`when need decimal use decimal.js`).
- `instant { tz: utc }` → `Date` always constructed in UTC, plus a
  `Timestamp` opaque alias for the spec name.
- Sum errors: string-literal unions for variant names; objects
  carrying payloads.

### `schedule`

```ts
// In runtime/scheduler.ts:
const ticker = setInterval(async () => {
  const now = Timestamp.nowUtc();
  const subs = await deps.subscriptions.queryActive();
  await Promise.allSettled(subs.map((s) => chargeCycle(deps, s, now)));
}, 30 * 24 * 60 * 60 * 1000);
```

- For real cadences (60s, 1h, 1d), use `node-cron` or per-target
  preference. For one-shot schedules, persist firing times in
  `scheduled_jobs` and sweep on the smallest declared interval.

---

## Errors

Error variants are string-literal types. Each declared variant is a
tag; payload-bearing variants are objects with a `tag` discriminator.
Throwing is reserved for runtime bugs (DB unreachable, etc.); declared
errors flow through `Result`.

```ts
export type SignupError = 'weak_password' | 'email_taken';
export type BadRange = { tag: 'bad_range'; reason: string };
```

---

## Runtime substrate

- HTTP: `hono` + `@hono/node-server`.
- DB: per preferences. Default examples: `better-sqlite3` (sync) or
  `node:sqlite` if Node 20+; production swaps to `pg` + `kysely` or
  `drizzle`.
- JWT: per preferences. Default: `jsonwebtoken`.
- Hash: per preferences. Default: `@node-rs/argon2`.
- ID: per preferences. Default: `cuid2`.
- Logging: `pino`.
- Validation: `zod` for request body schemas.

---

## Conventions

- ESM only. Use `node:` prefix for builtins. No CommonJS interop in
  generated code.
- Spec identifier `BookingId` → TS `BookingId` brand; spec field
  `booking_id` → TS field `bookingId` (lowerCamelCase fields per JS
  convention).
- `tsc --noEmit` clean. `eslint .` clean (config emitted with the
  project; `@typescript-eslint/recommended-type-checked`).
- No `any`. No non-null assertions outside test setup.
- Top-level `await` is fine in `index.ts`.

---

## Verification before reporting done

```sh
cd targets/typescript
npm install
npx tsc --noEmit
npx eslint .
npm test
npm run start &
hurl --variables-file ../../evals/<feature>/fixtures.env \
     --variable BASE_URL=http://localhost:3000 \
     ../../evals/<feature>/<feature>.hurl
```

All five must pass. Don't edit hurl files to make them green.
