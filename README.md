# T2A

A control plane for agent-heavy workflows. Humans no longer need to live inside the IDE—execution happens via agents like Claude and Cursor CLI. T2A enables humans to work on many tasks in parallel, simplifying context switching and supporting fully asynchronous workflows.

## Prerequisites

- Go 1.25+
- Postgres and a repo-root `.env` (gitignored) with `DATABASE_URL` — copy from [.env.example](.env.example)

## Build and test

```bash
go build ./...
go test ./...
```

Full local verification (`gofmt`, `go vet`, `go test`, `funclogmeasure -enforce`, `web/` test, lint, standards check, and build): `.\scripts\check.ps1` (Windows) or `./scripts/check.sh` (Unix). To run only Go steps, set `CHECK_SKIP_WEB=1`; to skip the slog audit, set `CHECK_SKIP_FUNCLOG=1` (see [AGENTS.md](AGENTS.md)).

## Run

```bash
go run ./cmd/dbcheck    # DB check; add -migrate to apply schema
go run ./cmd/taskapi    # HTTP server; -h for -port, -env, -logdir, -loglevel, -disable-logging (T2A_DISABLE_LOGGING=1 for no JSONL, errors only to stderr)
```

### API + web together

From the repo root, `scripts/dev.ps1` (Windows) or `scripts/dev.sh` installs `web/` deps, builds `taskapi` to `taskapi-dev(.exe)`, frees the API port, starts `taskapi` then `npm run dev`. Ctrl+C stops both.

For a non-default API port, set `DEV_TASKAPI_PORT` and `VITE_TASKAPI_ORIGIN` to match (see Web UI below).

```powershell
.\scripts\dev.ps1
```

```bash
chmod +x ./scripts/dev.sh   # once if needed
./scripts/dev.sh
```

By default `taskapi` listens on `http://127.0.0.1:8080` with REST `/tasks` and SSE `/events`. For **synthetic SSE** during UI work, set `T2A_SSE_TEST=1` and optional `T2A_SSE_TEST_INTERVAL` (default `3s`; `0` disables the ticker). Behavior: [docs/API-SSE.md](docs/API-SSE.md) (synthetic SSE); limits: [docs/DESIGN.md](docs/DESIGN.md) hub.

Windows PowerShell: use `curl.exe` and single-quoted JSON:

```powershell
curl.exe -s -X POST http://127.0.0.1:8080/tasks -H "Content-Type: application/json" -d '{"title":"live"}'
curl.exe -N http://127.0.0.1:8080/events
```

## Web UI (optional)

Vite + React + TypeScript SPA — layout, React Query, SSE invalidation: [docs/WEB.md](docs/WEB.md).

```bash
cd web
npm install
npm test
npm run lint
npm run dev
```

Opens Vite (often `http://localhost:5173`). Proxy targets `/tasks`, `/events`, `/repo` → `taskapi` (`web/vite.config.ts`). Override with `VITE_TASKAPI_ORIGIN` if the API is not `http://127.0.0.1:8080`.

| Command | Purpose |
|---------|---------|
| `npm run dev` | Dev server + proxy |
| `npm test` | Vitest (no real network; mocks) |
| `npm run lint` | ESLint (`web/eslint.config.js`) |
| `npm run test:watch` | Watch mode |
| `npm run build` | Typecheck → `web/dist/` |
| `npm run preview` | Preview `dist` (you still need API routing) |

Production: build static assets; serve `dist` same-origin as the API or behind a reverse proxy (`taskapi` does not serve `dist`). No CORS in the binary — [docs/DESIGN.md](docs/DESIGN.md#limitations).

## For developers

- [AGENTS.md](AGENTS.md) — repo map, checks, pitfalls  
- [CONTRIBUTING.md](CONTRIBUTING.md) — PRs, API / `parseTaskApi` sync  
- Extend the tasks stack: [docs/EXTENSIBILITY.md](docs/EXTENSIBILITY.md)  
- Workspace repo (`app_settings.repo_root`, `/repo`): [docs/SETTINGS.md](docs/SETTINGS.md), [docs/API-HTTP.md](docs/API-HTTP.md#workspace-repo)  
- Cursor-assisted edits: `.cursor/rules/CODE_STANDARDS.mdc`, `.cursor/rules/BACKEND_AUTOMATION/`, `.cursor/rules/UI_AUTOMATION/`, and `.cursor/rules/BUG_HUNTING/` (see CONTRIBUTING)

```bash
go doc -all ./pkgs/tasks/...
go doc -all ./internal/envload ./cmd/taskapi ./cmd/dbcheck
```

## License

[MIT](LICENSE).
