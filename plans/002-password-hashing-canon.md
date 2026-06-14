# Plan 002: Pin the password-hashing recipe in the codegen prompts and bring all three Go targets up to it

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat e89c1d3..HEAD -- prompts/codegen-base.md prompts/codegen-go.md examples/auth/targets/go examples/todo/targets/go examples/wallet/targets/go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: LOW
- **Depends on**: plans/001-conformance-eval-runner.md (verification gate)
- **Category**: security
- **Planned at**: commit `e89c1d3`, 2026-06-12

## Why this matters

The canonical hash spec (`commons/types/hash.candy`) mandates per-call random
salts and constant-time verification. The codegen prompts never say so, and the
result is three divergent `verifyPassword` implementations across the three Go
targets — one with a **static hardcoded salt** (todo: every user with the same
password gets the same stored hash, enabling rainbow-table attacks on the whole
table), and two with non-constant-time hash comparisons (auth, wallet). candy's
whole pitch is "the spec is the contract"; a security property that lives in
the canonical spec but not in generated code is the worst kind of contract
violation. This plan fixes the source of truth (the prompts) and brings the
three generated trees up to it in the same change.

## Current state

The canonical contract — `commons/types/hash.candy` (whole file is ~44 lines):

```
verify:    constant-time          ← line 34
...
- "verify uses constant-time comparison (no early return on mismatch)"
- "two hash() calls on the same plaintext return different bytes (per-call salt)"
```

(Note: `commons/` specs are not yet consumed via `use spec` by the examples —
linter support is pending. They are still the ratified canon per PR #43.)

The prompt gap — `prompts/codegen-base.md:349-350` (§5 Reserved primitives) is
all the prompts say about hashing:

```
| `hash(value)`      | A hash of `value` using the target's chosen hash library. |
| `verify(v, h)`     | Verifies `v` against hash `h`.                       |
```

`prompts/codegen-go.md:264` adds only: `- Hash: per preferences. Default:
` `golang.org/x/crypto/argon2`.`

The three divergent generated implementations:

1. `examples/todo/targets/go/internal/auth/flows.go:167-185` — **static salt**:

```go
func hashPassword(plain []byte) []byte {
	salt := []byte("candy-todo-salt-1") // static salt for deterministic test repro; prod uses random
	key := argon2.IDKey(plain, salt, 1, 64*1024, 4, 32)
	return key
}

// verifyPassword checks a plaintext against the stored hash.
func verifyPassword(plain, stored []byte) bool {
	candidate := hashPassword(plain)
	...XOR loop (constant-time — this part is fine)...
}
```

2. `examples/auth/targets/go/internal/auth/flows.go:193-213` — KSUID-string
   salt (random enough), but non-constant-time compare:

```go
func hashPassword(password string) string {
	salt := []byte(ksuid.New().String())
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return fmt.Sprintf("argon2id$%x$%x", salt, hash)
}
...
	candidate := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return string(candidate) == string(expected)     // ← flows.go:212
```

3. `examples/wallet/targets/go/internal/auth/flows.go:28-50` — random 16-byte
   salt (good), but non-constant-time compare on hex strings:

```go
	hashBytes := argon2.IDKey([]byte(string(p)), saltBytes, 1, 64*1024, 4, 32)
	return hex.EncodeToString(hashBytes) == parts[1]  // ← flows.go:49
```

The Rust target (`examples/auth/targets/rust`) uses the `argon2` crate's
`verify_password`, which handles salting and constant-time comparison
internally — it conforms; read-only sanity check only.

All preferences pin argon2 for Go (`examples/{auth,todo,wallet}/preferences.candy`
— `when need hash use argon2`), so the library choice is right everywhere; only
the recipe around it diverged.

Repo conventions that apply:
- Generated files start with the codegen header and "do not edit — regenerate
  from spec". This plan is a **manual regeneration**: the prompt (source of
  truth) changes first, and the generated trees are updated to match in the
  same PR. Precedent: commits `dd23c22`, `aadd902` in `git log` patch generated
  trees alongside their source-of-truth change.
- Don't modify specs or fixtures to make tests pass.
- Each generated tree has a `HANDOFF.md` judgment-call log — append an entry
  describing this change in each tree you touch.

## Commands you will need

| Purpose | Command | Expected on success |
|---|---|---|
| Build each Go target | `cd examples/<f>/targets/go && go build ./... && go vet ./...` | exit 0 |
| Conformance (needs plan 001) | `scripts/run-evals.sh --all` | 4× PASS |
| Conformance fallback (001 not landed) | manual: start backend per `examples/<f>/targets/go/cmd/server`, then `hurl --variables-file evals/<f>/fixtures.env --variable BASE_URL=http://localhost:8080 evals/<f>/<f>.hurl --test` | all asserts pass |
| Grep gate | see Done criteria | |

## Steps

### Step 1: Add the hashing recipe to `prompts/codegen-base.md`

In §5 (Reserved primitives), replace the two table rows at lines 349–350 with
rows that carry the contract, and add a short paragraph after the table. The
contract to encode (wording yours, content fixed):

- `hash(value)`: uses the library pinned by `preferences.candy` (`when need
  hash use ...`). A **new random salt (≥16 bytes, CSPRNG) per call** — two
  calls on the same plaintext must produce different stored values. The stored
  format must embed everything verification needs (salt + parameters + digest),
  e.g. the PHC string format for argon2.
- `verify(v, h)`: recomputes with the salt/parameters embedded in `h` and
  compares with a **constant-time comparison** (e.g. Go
  `crypto/subtle.ConstantTimeCompare`; or the library's built-in verify, like
  the Rust `argon2` crate's `verify_password`). Never `==` on hash bytes or
  their hex/string encodings.
- Reference `commons/types/hash.candy` as the canonical spec for these
  properties.

**Verify**: `grep -n "constant-time" prompts/codegen-base.md` → ≥1 match;
`grep -in "per-call\|per call" prompts/codegen-base.md` → ≥1 match.

### Step 2: Make the Go overlay concrete

In `prompts/codegen-go.md` (the hash bullet is at line 264), expand it to the
concrete recipe: `golang.org/x/crypto/argon2` `IDKey` with a per-call
`crypto/rand` 16-byte salt, stored as a self-describing string (salt + digest,
hex or PHC), verified via `crypto/subtle.ConstantTimeCompare` on the raw
digest bytes. Keep it short (2–4 lines) — the overlay style is terse.

**Verify**: `grep -n "ConstantTimeCompare" prompts/codegen-go.md` → 1 match.

### Step 3: Fix todo/go (static salt — the security defect)

In `examples/todo/targets/go/internal/auth/flows.go:167-185`:

- `hashPassword`: generate a 16-byte salt via `crypto/rand.Read`, and return a
  stored value that embeds it (match the auth target's shape:
  `fmt.Sprintf("argon2id$%x$%x", salt, hash)` — and store/compare as `[]byte`
  of that string, since todo's repo layer passes `[]byte`).
- `verifyPassword`: parse the stored format, recompute with the embedded salt,
  compare digests with `crypto/subtle.ConstantTimeCompare`.
- Update imports (`crypto/rand`, `crypto/subtle`, `fmt`, drop nothing still
  used). If the `users` table schema or repo layer assumes a fixed 32-byte
  hash length, adjust only what the new format strictly requires.
- Append a dated entry to `examples/todo/targets/go/HANDOFF.md` noting the
  change and that it implements the prompts' new hashing contract.

Stored hashes change shape, but every hurl run bootstraps users from scratch
on an empty DB (`evals/README.md:107-115`), so no fixture depends on the old
format. The old comment claims the static salt was "for deterministic test
repro" — no test asserts on hash values (verify with the grep in Done
criteria before relying on this).

**Verify**: `cd examples/todo/targets/go && go build ./... && go vet ./...` →
exit 0; `grep -rn "candy-todo-salt" .` → no matches.

### Step 4: Fix auth/go and wallet/go (constant-time compare)

- `examples/auth/targets/go/internal/auth/flows.go:212`: replace
  `return string(candidate) == string(expected)` with
  `return subtle.ConstantTimeCompare(candidate, expected) == 1`
  (import `crypto/subtle`). Leave the KSUID salt as-is — it is per-call and
  unpredictable enough; changing the stored format here is not required.
- `examples/wallet/targets/go/internal/auth/flows.go:49`: replace the hex
  string `==` with `subtle.ConstantTimeCompare` on the raw digest bytes
  (compare `hashBytes` against the hex-decoded `parts[1]`; reuse the existing
  hex decode error-handling style in that function).
- Append a dated entry to each tree's `HANDOFF.md`.

**Verify**: for each of the two targets: `go build ./... && go vet ./...` →
exit 0.

### Step 5: Conformance gate

**Verify**: `scripts/run-evals.sh --all` → 4× PASS (or the manual hurl
fallback for auth/go, todo/go, wallet/go, auth/rust if plan 001 hasn't landed).
Signup/login scenarios in each suite exercise hash + verify end-to-end.

## Test plan

The hurl suites are the behavioral tests (signup → login happy path proves
hash/verify round-trips; wrong-password scenarios prove rejection). Plus two
property greps as regression tripwires (see Done criteria). No new Go unit
tests — the generated trees ship no test scaffolding today and introducing one
is out of scope (NEXT.md item 5 covers target-native test emission).

## Done criteria

Machine-checkable. ALL must hold:

- [ ] `grep -rn 'candy-todo-salt' examples/` → no matches
- [ ] `grep -rn 'ConstantTimeCompare' examples/auth/targets/go examples/todo/targets/go examples/wallet/targets/go` → ≥3 matches (one per tree)
- [ ] In the three Go trees, no `==` comparison on argon2 output remains: `grep -rn 'candidate) == \|candidate ==\|EncodeToString(hashBytes) ==' examples/*/targets/go/` → no matches
- [ ] `go build ./... && go vet ./...` exits 0 in all three Go target dirs
- [ ] `scripts/run-evals.sh --all` → 4× PASS
- [ ] `prompts/codegen-base.md` and `prompts/codegen-go.md` contain the recipe (Step 1/2 greps)
- [ ] `git status` shows changes only in: the two prompt files, the three Go target trees (flows.go + HANDOFF.md + possibly imports), `plans/README.md`
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- Any hurl suite fails after the change. Likely causes: the todo stored-hash
  format change broke a code path you didn't see (e.g. a fixed-length column).
  Report the failing scenario verbatim — do not edit specs/fixtures.
- The todo repo layer or schema turns out to assert a fixed hash length or
  format in more than ~2 places — that suggests a real regeneration is cheaper
  than a patch; report instead of spreading edits.
- You find hash *values* asserted anywhere in `evals/` or `examples/todo/targets/go/test/`
  (the deterministic-salt comment hints someone may have relied on it):
  `grep -rn "argon2\|6172676f" evals/ examples/todo/targets/go/test/ 2>/dev/null`
  turning up assertions on stored hashes is a STOP.
- You are tempted to touch the Rust target. Its argon2 usage already conforms;
  it is out of scope.

## Maintenance notes

- NEXT.md item 7 (codegen-time preferences/spec drift checker) is the
  mechanical guard that would have caught this class of bug — when it lands,
  "constant-time verify" and "per-call salt" from `commons/types/hash.candy`
  are prime first rules. NEXT.md item 4 (`use spec` linter support) is the
  longer-term fix that makes the commons contract enforceable.
- Reviewer scrutiny: the todo stored format change (Step 3) is the only
  behavior-shape change; check its parse/format round-trip carefully.
- Deferred: making the wallet/auth stored formats identical to todo's
  (PHC-style unification). Cosmetic once all three satisfy the contract;
  unify at next true regeneration.
