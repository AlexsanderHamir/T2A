# Domain documentation

Behavioral deep-dives for T2A subsystems: actors, workflows, wire contracts, design rationale, and known limits. Use these articles when you need to understand *why* production code behaves a certain way—not when you need table schemas or HTTP routes.

## About this series

Domain articles sit between reference docs and ADRs:

| Layer | Location | Answers |
| --- | --- | --- |
| Reference | [data-model.md](../data-model.md), [api.md](../api.md), [configuration.md](../configuration.md) | What is stored, exposed, and configurable |
| **Domain (this folder)** | `docs/domain/*.md` | How subsystems behave end-to-end in production |
| Architecture | [architecture.md](../architecture.md) | How components connect across the product |
| ADR | [adr/](../adr/) | Why a specific design was chosen at a point in time |

Schema, routes, and env vars remain authoritative in reference docs. Domain articles link to them and must not contradict them.

## How to use these docs

1. Start from the article index below when you are working on a bounded subsystem (checklist, execute phase, verify phase, …).
2. Read **Overview** and **Key concepts** for vocabulary and scope.
3. Use **Workflow** sections for step-by-step behavior that matches code paths.
4. Use **Wire contracts** and **Configuration** for implementation details at boundaries.
5. Check **Limitations** before changing behavior—you may be hitting an intentional trade-off documented in an ADR.

## Authoritative references

| Doc | Use for |
| --- | --- |
| [data-model.md](../data-model.md) | Schema, tables, report JSON shapes, edit locks |
| [api.md](../api.md) | HTTP routes, status codes, request bodies |
| [configuration.md](../configuration.md) | Env vars and `app_settings` fields |
| [architecture.md](../architecture.md) | System overview and component map |
| [adr/](../adr/) | Historical design decisions |

## Article template

New articles under `docs/domain/` should follow this outline so the series stays consistent:

1. **Title and one-line description** — What the article covers in one sentence.
2. **Applies to / Audience** — Optional metadata table (roles, packages, or features affected).
3. **In this article** — Anchor-linked table of contents.
4. **Overview** — Purpose, scope, and explicit out-of-scope items.
5. **Key concepts** — Terminology and trust boundaries (tables preferred).
6. **How it works** — Architecture diagram and high-level flow.
7. **Workflow** — Numbered procedural steps aligned with production code.
8. **Wire contracts** — Prompts, report files, DB rows, API surfaces.
9. **Configuration** — Operator knobs (link to [configuration.md](../configuration.md) for full reference).
10. **Best practices** — Deliberate design wins and recommended operator usage.
11. **Limitations** — Known trade-offs; tie each item to code or an ADR.
12. **See also** — Related docs and code map.

Use Microsoft-style callouts where they add clarity:

```markdown
> **Note** — Supplementary context that is easy to miss.

> **Important** — Contract or safety detail that affects correctness.

> **Warning** — Operator or contributor action that can cause failure.
```

## Articles

| Article | Description |
| --- | --- |
| [done-criteria.md](./done-criteria.md) | Done criteria lifecycle: definition, execute/verify loop, completion ledger |
| [execute-agent.md](./execute-agent.md) | Execute phase: prompt composition, runner invocation, criteria self-report, resume |
| [verify-agent.md](./verify-agent.md) | Verify phase: LLM judge, criterion commands, integrity checks, retries |
