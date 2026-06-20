# Documentation index

“Read when” lookup for every doc under `docs/`.

| | |
| --- | --- |
| **Applies to** | Finding the right doc when you know your topic |
| **Audience** | Contributors and agents after [guide.md](./guide.md) |
| **Prerequisite** | [guide.md](./guide.md) for learning paths; [AGENTS.md](../AGENTS.md) for code edits |

## In this article

- [Overview](#overview)
- [Navigation](#navigation)
- [Reference and overview](#reference-and-overview)
- [Implementation and deep dive](#implementation-and-deep-dive)
- [See also](#see-also)

## Overview

Use [guide.md](./guide.md) to pick a learning path by goal. Use the tables below when you already know what you need.

> **Tip** — Agents: scoped paths and code locations live in [AGENTS.md](../AGENTS.md) and [agent-map.md](./agent-map.md).

## Navigation

| Doc | Read when |
| --- | --- |
| [guide.md](./guide.md) | You are new or need to pick a learning path by goal |
| [agent-map.md](./agent-map.md) | You need a repository path for a subsystem you are editing |

## Reference and overview

| Doc | Read when |
| --- | --- |
| **[execute-and-verify.md](./execute-and-verify.md)** | **You create tasks or write done criteria (checklist items).** |
| [architecture.md](./architecture.md) | You need how `taskapi`, the store, the agent worker, and SSE fit together |
| [data-model.md](./data-model.md) | You touch tasks, projects, cycles/phases, dependencies, gates, or checklists |
| [api.md](./api.md) | You need the REST + SSE endpoint surface (handler code is authoritative for status codes) |
| [configuration.md](./configuration.md) | You change env vars, app settings, or agent worker configuration |

## Implementation and deep dive

| Doc | Read when |
| --- | --- |
| [web.md](./web.md) | You work on the `web/` SPA |
| [contributing.md](./contributing.md) | Developer guide: features, handler growth, tests, troubleshooting |
| [domain/](./domain/) | You need why a subsystem behaves as it does — index: [domain/README.md](./domain/README.md) |
| [omitted-features.md](./omitted-features.md) | A feature exists in code but is hidden for launch |
| [adr/](./adr/) | You need the historical reason behind a design decision |

## See also

- [guide.md](./guide.md) — documentation layers and learning paths
- [AGENTS.md](../AGENTS.md) — scoped paths and code lookups
- [contributing.md](./contributing.md) — developer guide
- [CONTRIBUTING.md](../CONTRIBUTING.md) — GitHub PR stub (security + verify command)
