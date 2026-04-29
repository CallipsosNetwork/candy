# candy design docs

The grammar reference (`../grammar.md`) covers syntax. These docs cover
*how things fit together* — the architectural patterns the language is
built around. Read them when designing a new feature, integrating an
external service, or reviewing how a piece of the system connects to the
rest.

- [architecture.md](architecture.md) — Layered topology: controller → flow
  → actor. How dependencies point, how policies attach, where substrate
  lives, how events glue features together.
- [features.md](features.md) — Feature modules. The `prose` block. Folder
  vs. single-file layout. `exports`, `uses`, and `policies` fields.
- [externals.md](externals.md) — External actors: how SDKs and third-party
  services integrate as first-class actors with the `external` modifier.
  Webhooks as events, compensation across the network, idempotency.
