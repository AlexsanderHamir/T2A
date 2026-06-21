# Contributing to Hamix

Set up the repo, verify your change, and find the right documentation for learning or editing.

| | |
| --- | --- |
| **Applies to** | First-time setup, pull requests, finding docs |
| **Audience** | Human contributors and agents |
| **Prerequisite** | None — start here after cloning |

## In this article

- [Requirements](#requirements)
- [Setup](#setup)
- [Before you open a PR](#before-you-open-a-pr)
- [Where to go next](#where-to-go-next)
- [Stuck?](#stuck)
- [Security](#security)
- [See also](#see-also)

## Requirements

- **Database** — `DATABASE_URL` in repo-root `.env` (always)
- **Never commit** `.env` or secrets

**Docker path** — Docker only; no Go or Node on the host. See [docs/docker.md](docs/docker.md).

**Native path** — **Go** 1.25+ and **Node** 20+ (npm/npx included; for the web UI).

> **Warning** — Workspace repo path, agent worker settings, cursor binary, and run timeout are configured in the SPA **Settings** page (`/settings`), not in `.env`. See [docs/configuration.md](docs/configuration.md).

## Setup

1. Copy `.env.example` to `.env` and set `DATABASE_URL`.

### Docker

```bash
./scripts/docker-build.sh        # Unix
.\scripts\docker-build.ps1       # Windows PowerShell
docker compose up
```

Taskapi migrates the schema on startup (same as native dev). See [Schema migrations in docs/configuration.md](docs/configuration.md) and [docs/docker.md](docs/docker.md).

API: `http://127.0.0.1:8080` · Web: `http://localhost:5173`

### Native

2. Run API + web (taskapi migrates on startup):

```bash
./scripts/dev.sh        # Unix — chmod +x once if needed
.\scripts\dev.ps1       # Windows
```

API: `http://127.0.0.1:8080` · Web: `http://localhost:5173`

Optional manual migrate: `go run ./cmd/dbcheck -migrate` — [Schema migrations in configuration.md](docs/configuration.md).

## Before you open a PR

Verification steps live in `scripts/check-go.sh` / `scripts/check-web.sh` (and PowerShell twins). CI runs those leaf scripts directly — not duplicated commands in `.github/workflows/ci.yml`.

| I want to… | Command |
|------------|---------|
| Run everything (Docker, no local Go/Node) | `docker compose run --rm dev ./scripts/check.sh --install` |
| Run everything locally | `./scripts/check.sh` or `.\scripts\check.ps1` |
| First run / lockfile changed | `./scripts/check.sh --install` or `.\scripts\check.ps1 -Install` |
| Same as CI backend | `./scripts/check-go.sh --verbose` or `.\scripts\check-go.ps1 -Verbose` |
| Same as CI web | `./scripts/check-web.sh --install --verbose` or `.\scripts\check-web.ps1 -Install -Verbose` |
| Go only (fast) | `./scripts/check.sh --go-only` or `.\scripts\check.ps1 -GoOnly` |
| Full logs | add `--verbose` / `-Verbose` |

Quiet by default: one line per step on success; full tool output only on failure. Each script accepts `--help` / `-Help` for its step list and flags.

Also:

- [ ] Changed an API endpoint → update [docs/api.md](docs/api.md) in the same PR
- [ ] New behavior → add or update a test
- [ ] User-visible change → update the relevant doc

Coding conventions (where to put API calls, how the live UI updates, etc.): [AGENTS.md](AGENTS.md).

## Where to go next

Pick **one** row. Do not read the whole tree.

| I want to… | Start here |
| --- | --- |
| **Learn the project** — how docs fit together | [docs/guide.md](docs/guide.md) |
| **Use Hamix** — create tasks, write checklist criteria | [docs/execute-and-verify.md](docs/execute-and-verify.md) |
| **Edit code** — find a file or doc for a specific task | [AGENTS.md](AGENTS.md) § [Where to find X](AGENTS.md#where-to-find-x) |
| **Edit code** — pick reading order for my kind of change | [AGENTS.md](AGENTS.md) § [Scoped paths](AGENTS.md#scoped-paths) |
| **Look up routes, schema, or env vars** | [docs/api.md](docs/api.md), [docs/data-model.md](docs/data-model.md), [docs/configuration.md](docs/configuration.md) |
| **Find any doc by topic** | [docs/README.md](docs/README.md) |
| **Subsystem code paths** | [docs/agent-map.md](docs/agent-map.md) |

Vertical slice (domain → store → handler → optional web): follow [AGENTS.md](AGENTS.md) scoped paths, then `pkgs/tasks/handler/README.md` and [docs/domain/persistence.md](docs/domain/persistence.md).

## Stuck?

| Symptom | Fix |
| --- | --- |
| Full reload on `/tasks/<id>` shows raw JSON | Restart Vite; see `web/vite.config.ts` HTML bypass for `/tasks` proxy |
| SSE connected but Updates timeline empty | `HAMIX_SSE_TEST=1` in `.env`, restart `taskapi` — [docs/configuration.md](docs/configuration.md) |
| Fetch / EventSource errors | Confirm `taskapi` on `:8080` and dev script running |
| No repository for file search | Set **Workspace repository** in SPA Settings — [docs/domain/workspace-repo.md](docs/domain/workspace-repo.md) |
| Tests fail with database errors | Use `internal/tasktestdb/` (SQLite); gate real Postgres with `//go:build integration` |
| Match API error to logs | `request_id` in JSON body / `X-Request-ID` header on access logs |
| Still failing local checks | Re-run `go test ./... -count=1`; compare [CI](.github/workflows/ci.yml); full bar above |

More edit lookups: [AGENTS.md](AGENTS.md#where-to-find-x).

## Security

For **undisclosed vulnerabilities**, use [SECURITY.md](SECURITY.md) (private GitHub advisory, not a public issue).

## See also

- [README.md](README.md) — product overview and quick start
- [docs/guide.md](docs/guide.md) — documentation map and learning paths
- [AGENTS.md](AGENTS.md) — scoped paths, Where to find X, verify commands
