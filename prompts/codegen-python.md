# candy codegen — Python overlay

Apply on top of `codegen-base.md`. Default framework: `fastapi` (HTTP).
Default Python: 3.11+. Default async runtime: `asyncio` via uvicorn.

There is no `python-best-practices` skill in v0.1, so this overlay
carries more inline idiom than Go and Rust.

---

## Project layout

```
targets/python/
  pyproject.toml               — uv or pip-installable; ruff + mypy config.
  src/
    main.py                    — FastAPI app + uvicorn entry.
    <project>/
      __init__.py
      <feature>/
        __init__.py
        actors.py
        flows.py
        controllers.py
        policies.py
        events.py
      shared/
        types.py
        events.py
        errors.py
      runtime/
        db.py
        scheduler.py
        event_bus.py
        webhooks.py
  tests/
    test_integration.py        — hurl runner harness.
```

Package layout uses a single distribution package named after
`candy.toml`'s `[project].name`.

---

## Block-by-block bindings

### `actor`

```py
from dataclasses import dataclass

@dataclass(frozen=True)
class Booking:
    id: BookingId
    status: BookingStatus
    # one field per state entry; frozen where mutation is not modelled.

class BookingRepo:
    def __init__(self, db: Database) -> None:
        self._db = db

    async def create(self, init: BookingInit) -> Booking: ...
    async def find_by_id(self, id: BookingId) -> Booking | None: ...
    async def confirm(
        self, id: BookingId, args: ConfirmArgs, now: Timestamp, key: IdempotencyKey,
    ) -> Result[ConfirmOk, ConfirmErr]: ...
```

- One module per feature; one repo class per actor.
- `derive` accessors are `@property` getters (computed each call).
- `invariant` predicates raise a declared exception before commit.
- `audit` tables are append-only.

### `external actor`

```py
from typing import Protocol

class Payments(Protocol):
    async def charge(self, amount: Money, source: PaymentMethod, key: IdempotencyKey) \
        -> Result[ChargeId, PaymentError]: ...

class StripePayments:
    def __init__(self, client: stripe.Client) -> None: ...
    async def charge(self, ...): ...
```

- Multi-provider: `Protocol` + one class per tag; a `dict[Tag, Payments]`
  registry in `runtime/`.
- Webhook routes: `POST /webhooks/{provider}/{event}`. Verify signature
  via the provider's library, map payload to the declared `emits`
  event, dispatch.

### `flow`

```py
async def place_booking(
    deps: Deps,
    args: PlaceBookingArgs,
    now: Timestamp,
    key: IdempotencyKey,
) -> Result[BookingConfirmed, BookingError]:
    booking_id = BookingId.generate()                # pre-generated; base §5.
    held = await deps.listings.hold_dates(args.listing_id, args.range, booking_id, now)
    if held.is_err(): return Err(BookingError.HOLD_FAILED)
    paid = await deps.payments.charge(args.amount, args.source, key)
    if paid.is_err():
        await deps.listings.release_dates(held.unwrap())     # compensate.
        return Err(BookingError.PAYMENT_DECLINED)
    return Ok(BookingConfirmed(id=booking_id, charge=paid.unwrap()))
```

- Use a result-like `Result[T, E]` (e.g. `returns.Result` from the
  `returns` library, or a small in-project implementation). Don't
  raise for declared errors.
- `tell` (fire-and-forget): `asyncio.create_task(...)` without
  awaiting; ensure the runtime captures unhandled-task exceptions.

### `controller`

```py
from fastapi import APIRouter, Depends, HTTPException, Header, Request

router = APIRouter()

@router.post('/signup', status_code=201)
async def handle_signup(body: SignupBody, request: Request) -> SignupResponse:
    now = Timestamp.now_utc()
    key = read_idem_key(request.headers)
    result = await signup(deps, body, now, key)
    match result:
        case Ok(value):                              return value
        case Err('weak_password'):                   raise HTTPException(400, detail='weak_password')
        case Err('email_taken'):                     raise HTTPException(409, detail='email_taken')
```

- One handler per route. Pydantic models for request/response shape.
- `auth: bearer` → a `Depends(...)` that extracts and validates the
  token; raises `HTTPException(401)` on missing/invalid.
- `now` is captured at the boundary.

### `policy`

```py
def password_strength(p: Password) -> Result[None, str]:
    if len(p) < 12:
        return Err('weak_password')
    return Ok(None)
```

- `controller`-scope policies → FastAPI dependencies.
- `actor`/`flow`/`type` scope: explicit call before the protected op.

### `event`

```py
@dataclass(frozen=True)
class ChargeSucceeded:
    charge: ChargeId
    at: Timestamp

class EventBus(Protocol):
    async def publish(self, ev: object) -> None: ...
    def subscribe(self, event_type: type, h: Callable[[object], Awaitable[None]]) -> None: ...
```

- `delivery: strict` → transactional outbox.
- `delivery: eager` → at-least-once via Celery, RQ, or NATS per
  preferences.
- `delivery: weak` → `asyncio.create_task` with a global error logger.

### `type` and `enum`

```py
from typing import NewType, Literal

Money    = NewType('Money', int)                    # integer minor units.
Email    = NewType('Email', str)
BookingId = NewType('BookingId', str)

class BookingStatus(str, Enum):
    PENDING   = 'pending'
    CONFIRMED = 'confirmed'
    CANCELLED = 'cancelled'

# Result type:
@dataclass(frozen=True)
class Ok(Generic[T]):  value: T
@dataclass(frozen=True)
class Err(Generic[E]): error: E
Result = Ok[T] | Err[E]
```

- **Money is `int`** (minor units). Floats are forbidden.
- `decimal` types use `decimal.Decimal`; never `float`.
- `instant { tz: utc }` → `datetime.datetime` with `tzinfo=timezone.utc`.
  Build a small `Timestamp` wrapper alias.
- Sum errors: string-literal `Literal[...]` types or `Enum`.
- Validators (for `format`/`max`): construct via `parse` factory
  functions that return `Result`.

### `schedule`

```py
async def run_charge_cycle(deps: Deps, now: Timestamp) -> None:
    subs = await deps.subscriptions.query_active()
    for sub in subs:
        await charge_cycle(deps, sub, now)

# In runtime/scheduler.py: APScheduler or asyncio-based cron.
```

- Default scheduler library: `APScheduler` (per preferences). One-shot
  schedules persist firing times in `scheduled_jobs`; the runtime
  sweeps on the smallest declared interval.

---

## Errors

Declared error variants are `Literal` strings or `Enum` members. Don't
raise for declared errors — flow them through `Result`. Reserved for
unexpected runtime failures: `Exception` subclasses logged at the
boundary.

---

## Runtime substrate

- HTTP: `fastapi` + `uvicorn`.
- DB: per preferences. Default examples: `sqlite3` (stdlib) or
  `aiosqlite`. Production swaps to `asyncpg` + `sqlalchemy` 2.0 async.
- JWT: per preferences. Default: `pyjwt`.
- Hash: per preferences. Default: `argon2-cffi`.
- ID: per preferences. Default: `python-ulid`.
- Logging: `structlog` or stdlib `logging`.
- Validation: `pydantic` v2 for request/response models.

---

## Conventions

- Python 3.11+. Use `from __future__ import annotations` only when
  necessary; PEP 604 union syntax (`A | B`) is the default.
- Spec identifier `BookingId` → Python `BookingId`; spec field
  `booking_id` → Python field `booking_id` (snake_case fields per PEP 8).
- `mypy --strict` clean (or `pyright` strict).
- `ruff check .` clean. `ruff format .` clean.
- No `Any` outside well-marked boundaries (e.g., raw provider payloads
  before mapping).
- Type-hint every public function. `from __future__ import annotations`
  is fine but inferable types are preferred.

---

## Verification before reporting done

```sh
cd targets/python
uv sync                          # or: pip install -e '.[dev]'
ruff check .
ruff format --check .
mypy src
pytest
uvicorn src.main:app --port 8000 &
hurl --variables-file ../../evals/<feature>/fixtures.env \
     --variable BASE_URL=http://localhost:8000 \
     ../../evals/<feature>/<feature>.hurl
```

All six must pass. Don't edit hurl files to make them green.
