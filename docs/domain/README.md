# Domain documentation

Deep-dives on how specific subsystems behave — actors, flows, wire contracts, strengths, and limitations. Use these when you need to understand *why* something works the way it does, not just what tables or routes exist.

Reference docs stay authoritative for contracts:

| Doc | Use for |
| --- | --- |
| [data-model.md](../data-model.md) | Schema, tables, report JSON shapes, edit locks |
| [api.md](../api.md) | HTTP routes, status codes, request bodies |
| [configuration.md](../configuration.md) | Env vars and `app_settings` fields |
| [architecture.md](../architecture.md) | System overview and component map |
| [adr/](../adr/) | Historical design decisions |

## Template for new domain docs

Each file under `docs/domain/` should follow this outline:

1. **Purpose and scope** — What the subsystem does and what it explicitly does not cover.
2. **Actors and trust boundaries** — Who does what; what is trusted vs asserted.
3. **End-to-end flow** — Numbered steps matching production code paths.
4. **Inputs / outputs** — Prompts, report files, DB rows, API surfaces.
5. **Configuration** — Operator knobs (link to [configuration.md](../configuration.md) for full reference).
6. **Strengths** — Deliberate design wins.
7. **Limitations and known trade-offs** — Where the system stops; tie each item to code or an ADR.
8. **Related docs and code map** — Pointers for contributors.

## Index

| Doc | Topic |
| --- | --- |
| [done-criteria.md](./done-criteria.md) | Done criteria lifecycle: definition, execute/verify loop, completions |
| [verify-agent.md](./verify-agent.md) | Verify phase: LLM judge, criterion commands, integrity checks, retries |
