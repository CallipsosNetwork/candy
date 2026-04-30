# candy

**candy** is a specification language for stateful backends. You describe a
system as **actors with state**, **flows that compose actors**, **controllers
that expose flows over HTTP**, **policies that capture rules**, and **events
that propagate**. From one spec, AI generates idiomatic backends in Go, Rust,
TypeScript, or Python.

The language is small (~48 single-word keywords), prose-heavy where prose
serves it, rigorous where ambiguity costs.

Files use the `.candy` extension.

---

## Why candy

Backend code repeats itself across languages: the same actor with state, the
same saga with compensation, the same controller mapping HTTP to a handler.
What differs between a Go service and a Rust service is mostly idiom, not
intent. candy captures the intent once — the actors, flows, invariants,
policies — and leaves the idiom to the generator.

A `.candy` file is meant to be readable by a product person and precise enough
for a code generator. Prose lives in `intent:` and `examples:` fields;
structure lives in typed blocks.

---

## The five word-axes

Every keyword belongs to one of five families:

| Axis        | What it expresses                          | Examples                              |
|-------------|--------------------------------------------|---------------------------------------|
| `ENTITY`    | things that exist                          | `actor`, `flow`, `controller`, `event`, `policy`, `type`, `enum`, `target` |
| `ACTION`    | things that happen                         | `ask`, `tell`, `emit`, `effect`, `commit`, `compensate`, `reject`, `use` |
| `TIME`      | when, in what order, for how long          | `now`, `then`, `after`, `before`, `until`, `expire` |
| `CONDITION` | under what circumstances                   | `if`, `else`, `when`, `require`, `invariant`, `unless`, `need` |
| `INTENT`    | why this exists, what good looks like      | `intent`, `examples`, `because`       |

See `grammar.md` for the full reference.

---

## Hello, candy

The smallest useful program — a flow and a controller that exposes it:

```candy
flow Hello(name: string) -> string {
  intent: "Greet someone by name."
  commit "Hello, ${name}!"
}

controller Greetings {
  GET /hello/:name -> Hello(name) {
    auth: none
    map:
      ok(message) -> 200 { greeting: message }
  }
}
```

That is a complete spec. A generator turns it into an HTTP server in the
target language with the route, handler, and response shape wired up.

---

## Examples

The `examples/` directory walks the language from trivial to realistic:

| File                  | Demonstrates                                                                 |
|-----------------------|------------------------------------------------------------------------------|
| `examples/hello.candy` | The minimum: a flow, a controller, a route mapping.                          |
| `examples/todo.candy`  | A stateful actor with list state, derived views, multiple messages, events, and a controller with several routes. |
| `examples/auth.candy`  | Cross-actor flow (`Signup`), prose-driven `policy`, time-bound sessions, idempotency keys, and opaque errors. |

Read them in that order. Each one introduces one or two new pieces of the
grammar without revisiting prior ground.

---

## Hard rules

A few constraints are baked into the language. They are listed in full in
`grammar.md`; the load-bearing ones:

1. **No underscores in keywords.** Compounds are single words or two real
   words composed.
2. **One source of truth.** If a value can be derived, use `derive`. Never
   store what you can compute.
3. **No floats for money.** Money is integer minor units; currency is pinned
   in the type declaration.
4. **Time is UTC; `now` is an input.** Actors and flows receive `now` as a
   parameter. No global clock.
5. **Idempotency keys are explicit.** Replayable messages declare a
   `key: Key` parameter. Replay returns the prior result without re-running
   effects.
6. **One actor owns its state.** No other actor reads or writes another
   actor's state directly. Cross-actor work goes through a `flow`.

---

## Project layout

A candy project is one or more `.candy` files in a directory. Declarations
resolve across files; there is no import statement. For non-trivial projects:

```
project/
  actors/<Name>.candy        // one actor per file
  flows/<Name>.candy         // one flow per file
  controllers/<Name>.candy   // one controller per file
  policies/<Name>.candy      // one policy per file
  types.candy                // shared types and enums
  events.candy               // shared event declarations
  invariants.candy           // system-level invariants
```

Small projects flatten to a single `.candy` file. The examples in this
repository all do.

---

## Relation to harness engineering

candy and OpenAI's *harness engineering* (Feb 2026) share the thesis that
engineers don't write code anymore — agents do — and the engineer's job
becomes building the structure that lets agents do useful work. They
diverge on which layer that structure lives in.

Harness engineering invests in the **environment** around the agent: a
short `AGENTS.md` as table-of-contents, a `docs/` directory as
system-of-record, scaffolds for design/code/review/test. Constraints are
documented conventions in natural language. The repository is the source
of truth; agents extend it.

candy invests in the **contract** the agent must honor: a grammar
(~50 single-word keywords), typed I/O on every block, attached policies,
and a multi-target conformance suite. Constraints are machine-checkable.
The spec is the source of truth; code in `targets/<lang>/` is regenerated
artifact.

| Dimension           | Harness engineering   | candy                         |
|---------------------|-----------------------|-------------------------------|
| Source of truth     | Codebase              | `.candy` spec                  |
| Constraint layer    | Markdown conventions  | Grammar + types + policies    |
| Agent action        | Writes code (commits) | Materializes code from spec   |
| Targets per project | One                   | Many (Go, Rust, TS, Python)   |
| Bugfix loop         | New commit            | Spec change + regenerate      |
| Verifiability       | Code review           | Conformance + multi-target    |

**When each wins.** Harness engineering fits sprawling, evolving products
with emergent requirements that resist clean specification. candy fits
backends with clean contracts (CRUD, sagas, business rules) where
multi-target portability is valuable.

**They compose.** candy itself would be built using harness practices —
the eventual compiler, codegen prompts, and conformance runner are
agent-developed code organized via `AGENTS.md` → `docs/`. The candy
*language* is a domain-specific harness; harness engineering is the
broader practice of which candy is one specialized instance.

See [Harness engineering | OpenAI](https://openai.com/index/harness-engineering/)
and [Martin Fowler's writeup](https://martinfowler.com/articles/harness-engineering.html).

---

## Reference

- `grammar.md` — full language reference: every block type, every keyword,
  cross-cutting conventions.
- `docs/` — design rationale: architecture, features, externals.
- `examples/` — three runnable specs, ordered by complexity.
