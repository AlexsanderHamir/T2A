# Documentation index

Long-form design and contracts live here; the root **`README.md`** stays commands and copy-paste.

## What to read

| Doc | Use it for |
|-----|------------|
| **[../AGENTS.md](../AGENTS.md)** | Short map for **humans and coding agents**: where code lives, what to run before finishing, link-out to rules. |
| **[../README.md](../README.md)** | Prerequisites, build/test, run **`dbcheck`** / **`taskapi`**, dev scripts, npm commands for **`web/`**. |
| **[DESIGN.md](./DESIGN.md)** | **`taskapi`**: HTTP + SSE, env vars, **`REPO_ROOT`** / **`/repo`**, persistence, limits, Mermaid for the server. |
| **[WEB.md](./WEB.md)** | **`web/`** SPA: React Query, SSE invalidation, **`parseTaskApi`**, **`web/src`** layout, tests. |

**Go:** route lists and behavior next to code — **`go doc`** on **`pkgs/tasks/...`**, **`pkgs/repo`**, **`internal/envload`**, **`cmd/taskapi`**, **`cmd/dbcheck`**.

## Where to put updates

| Change | Update |
|--------|--------|
| Flags, env, **`taskapi`** routes or timeouts | **`docs/DESIGN.md`** + relevant **`doc.go`**; root **`README`** only if command-line examples change. |
| **`web/`** only (components, hooks, no API contract change) | **`docs/WEB.md`**; root **`README`** only if npm scripts or env vars for Vite change. |
| **`dbcheck`** | Root **`README`** + **`cmd/dbcheck`** doc if flags change. |

**Cursor rules** (`.cursor/rules/`) are for tooling, not operators.
