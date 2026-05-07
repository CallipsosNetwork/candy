# candy codegen — Go overlay

Apply on top of `codegen-base.md`. Before emitting any Go, **load the
`golang-best-practices` skill** for idiom guidance (concurrency, error
wrapping, package layout, generics use). This overlay specifies only the
candy-to-Go bindings.

Default framework: `chi` (HTTP). Default DB: per `preferences.candy`;
typical is `sqlite` via `mattn/go-sqlite3` for examples, swappable.

---

## Project layout

```
targets/go/
  go.mod
  go.sum
  cmd/
    server/main.go               — wires DI, scheduler, HTTP server.
  internal/
    <feature>/
      actors.go                  — actor structs + handlers.
      flows.go                   — flow functions.
      controllers.go             — chi routes.
      policies.go                — policy functions.
      events.go                  — event types + subscribers.
    shared/
      types.go                   — branded types.
      events.go                  — cross-feature events.
      errors.go                  — declared error variants (typed).
    runtime/
      db.go                      — connection pool.
      scheduler.go               — schedule executor.
      eventbus.go                — event delivery.
      webhooks.go                — webhook signature verification helpers.
  test/
    integration_test.go          — hurl runner harness (shells out).
```

Module path: `github.com/<owner>/<project>` if known; otherwise infer
from `candy.toml`'s `[project].name` and emit a `// TODO: set module
path` comment in `go.mod`.

---

## Block-by-block bindings

### `actor`

```go
type Booking struct {
    ID     BookingId
    Status BookingStatus
    // ... one field per state entry, exported.
}

type BookingRepo struct{ db *sql.DB }

func (r *BookingRepo) Create(ctx context.Context, init BookingInit) (*Booking, error) { ... }
func (r *BookingRepo) FindByID(ctx context.Context, id BookingId) (*Booking, error)   { ... }
// One method per `accepts`:
func (r *BookingRepo) Confirm(ctx context.Context, id BookingId, args ConfirmArgs, now time.Time, key IdempotencyKey) (ConfirmOk, error) { ... }
```

- One package per feature under `internal/<feature>/`.
- State persists via the chosen DB library; the `<Actor>Repo` struct
  owns reads/writes for that actor's table only.
- `derive` accessors are methods on the actor struct (computed each
  call; never stored).
- `invariant` predicates run before commit; failure returns a typed
  error.
- `audit` is an append-only sibling table; emitted via an `audit*`
  method; the audit method never updates rows.

### `external actor`

```go
type Payments interface {
    Charge(ctx context.Context, amt Money, src PaymentMethod, key IdempotencyKey) (ChargeId, error)
}

// One implementation per declared provider tag.
type stripePayments struct{ client *stripe.Client }
func NewStripePayments(secret string) Payments { ... }
```

- Multi-provider: emit `type Payments interface { ... }` plus one
  implementation per tag. A registry maps tag → implementation.
- Webhook routes are emitted at `/webhooks/<provider>/<event>` (e.g.
  `/webhooks/stripe/charge-succeeded`); the body is the provider's
  native payload. Verify signature, map to declared `emits` event,
  dispatch to subscribers.

### `flow`

A function returning `(Ok, error)`. Steps are sequential await calls.
Compensation:

```go
func PlaceBooking(ctx context.Context, deps Deps, args PlaceBookingArgs, now time.Time, key IdempotencyKey) (BookingConfirmed, error) {
    bookingId := BookingId(deps.Generate())            // pre-generated; see base §5.
    held, err := deps.Listings.HoldDates(ctx, args.ListingId, args.Range, bookingId, now)
    if err != nil { return BookingConfirmed{}, ErrHoldFailed }
    paid, err := deps.Payments.Charge(ctx, args.Amount, args.Source, key)
    if err != nil {
        _ = deps.Listings.ReleaseDates(ctx, held)      // compensate.
        return BookingConfirmed{}, ErrPaymentDeclined
    }
    // ...
    return BookingConfirmed{ID: bookingId, Charge: paid}, nil
}
```

- Pass dependencies as a `Deps` struct or via a constructor closure.
  No globals.
- `tell` (fire-and-forget) → spawn a goroutine; do not await.
- Errors are typed sum variants (see §errors).

### `controller`

```go
func MountAuth(r chi.Router, deps Deps) {
    r.Post("/signup", handleSignup(deps))
    r.Group(func(r chi.Router) {
        r.Use(BearerAuth(deps))
        r.Post("/logout", handleLogout(deps))
    })
}

func handleSignup(deps Deps) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var body SignupBody
        if err := decode(r, &body); err != nil { writeError(w, 400, "bad_request"); return }
        out, err := Signup(r.Context(), deps, body, time.Now().UTC(), readIdemKey(r))
        // map exact ok/err to declared status + body shape.
    }
}
```

- One handler per route. Handlers parse → validate → dispatch → map.
- `auth: bearer` middleware: extracts the bearer token, validates per
  the `BearerAuth` policy in spec, sets the principal id on context;
  401 on absent/invalid.
- `now := time.Now().UTC()` at the boundary; never inside business
  logic.
- Idempotency-Key header → `IdempotencyKey` type; passed into the
  flow.

### `policy`

A free function with `Result`-style return:

```go
func PasswordStrength(p Password) error {
    if len(p) < 12 { return ErrWeakPassword }
    return nil
}
```

For policies attached at `controller` scope, emit middleware. For
`actor`/`flow`/`type` scope, emit a wrapped call before the protected
operation. For `prose` scope, apply at every controller and flow in
the feature.

### `event` and subscribe

```go
type ChargeSucceeded struct {
    Charge ChargeId
    At     time.Time
}

type EventBus interface {
    Publish(ctx context.Context, ev any) error
    Subscribe(eventType reflect.Type, h func(ctx context.Context, ev any) error)
}
```

- `delivery: eager` — at-least-once. Subscribers must be idempotent.
- `delivery: strict` — transactional outbox: write event in same tx as
  the state change, deliver on commit, mark delivered.
- `delivery: weak` — best-effort goroutine; drop on failure.

### `type` and `enum`

```go
type Money int64                                    // unit: minor; currency carried separately.
type Email string                                   // validated at construction by NewEmail.
type Password string                                // validated by PasswordStrength policy.
type BookingId string

type BookingStatus int
const (
    BookingPending BookingStatus = iota
    BookingConfirmed
    BookingCancelled
)
```

- Branded primitives → named scalars. Constructors validate when the
  type has format/max constraints.
- `Money` is `int64`; never `float64`.
- `decimal` types use `github.com/shopspring/decimal`.
- `instant { tz: utc }` → `time.Time` always converted with `.UTC()`.
- `Result<Ok, Err>` → `(Ok, error)` with typed sentinel errors. Use
  `errors.Is` for variant checks.

### `schedule`

```go
// In runtime/scheduler.go a cron-like ticker; on each fire:
func runChargeCycle(ctx context.Context, deps Deps, now time.Time) {
    subs, _ := deps.Subscriptions.QueryActive(ctx)
    for _, sub := range subs {
        _, _ = ChargeCycle(ctx, deps, sub, now)
    }
}
```

- Default scheduler: a goroutine ticking at the lowest declared
  cadence; finer-grained schedules use their own ticker. `preferences.candy`
  may override via `when need scheduler use <library>`.
- One-shot schedules (`at <expr> for any X in Y where ...`) use a
  per-instance computed firing time persisted in a `scheduled_jobs`
  table; the runtime sweeps it at the smallest declared interval.

---

## Errors

Generate one typed sentinel per declared error variant:

```go
var (
    ErrWeakPassword       = errors.New("weak_password")
    ErrEmailAlreadyTaken  = errors.New("email_already_taken")
    ErrPaymentDeclined    = errors.New("payment_declined")
)
```

Each `controller` `err(Variant) -> Status { ... }` mapping uses
`errors.Is(err, ErrXxx)` to dispatch, then writes the declared body
shape.

For variants with payload (`err(BadRange { reason })`), wrap with a
struct error:

```go
type BadRangeErr struct{ Reason string }
func (e *BadRangeErr) Error() string { return "bad_range: " + e.Reason }
```

---

## Runtime substrate

- HTTP: `chi`. Mount per-feature subrouters under their conventional
  path prefix (`/auth`, `/bookings`, ...).
- DB: per `preferences.candy`. Default for examples: `sqlite3` via
  `database/sql` + `github.com/mattn/go-sqlite3`. Schema generated
  once at startup; no migration framework in v0.1.
- JWT: per `preferences.candy`. Default: `github.com/golang-jwt/jwt/v5`.
- Hash: per preferences. Default: `golang.org/x/crypto/argon2`.
- ID generation: per preferences. Default: `github.com/segmentio/ksuid`.
- Logging: `log/slog` (structured).
- Context: `context.Context` is the first parameter of every exported
  function that crosses an I/O boundary.

---

## Conventions

- Package per feature; package name = lowercase feature name.
- File per block category (`actors.go`, `flows.go`, ...). Multiple
  small files beat one large file when a feature has > 5 actors / > 5
  flows.
- Field naming: lowerCamelCase for unexported, UpperCamelCase for
  exported. `BookingId`/`bookingId`; `UserId`/`userId`. Match Go
  vet's view of acronyms (`ID` not `Id`) where the ID type is
  Go-native; for spec types named `BookingId` keep that exact spelling
  to round-trip with the spec.
- `gofmt -s` clean. `go vet ./...` clean. `golangci-lint run` clean
  if a config exists.

---

## Verification before reporting done

```sh
cd targets/go
go vet ./...
go build ./...
go test ./...
hurl --variables-file ../../evals/<feature>/fixtures.env \
     --variable BASE_URL=http://localhost:8080 \
     ../../evals/<feature>/<feature>.hurl
```

All four must pass. If a hurl scenario fails, the spec mapping is wrong
or the runtime substrate is wrong — do not edit the hurl.
