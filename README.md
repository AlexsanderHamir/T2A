# T2A

**T2A** delegates many tasks to agents while keeping humans and automation aligned: one shared task store, a **web API**, an **audit trail**, and **live update hints** (`GET /events`) so clients refetch JSON instead of polling blindly.

Go implementation: **`github.com/AlexsanderHamir/T2A`**.

**Documentation:** **[`docs/README.md`](docs/README.md)** (what to read first) · **[`docs/DESIGN.md`](docs/DESIGN.md)** ( **`taskapi`**, HTTP, SSE, DB) · **[`docs/WEB.md`](docs/WEB.md)** (optional **`web/`** SPA).

## Prerequisites

- **Go** 1.25+
- **Postgres** and a repo-root **`.env`** (gitignored) with **`DATABASE_URL`**

## Build and test

```bash
go build ./...
go test ./...
```

## Run

```bash
go run ./cmd/dbcheck    # DB check; add -migrate to apply schema
go run ./cmd/taskapi    # HTTP server; -h for -port, -env, -migrate
```

### API + web together

From the repo root, **`scripts/dev.ps1`** (Windows) or **`scripts/dev.sh`** installs **`web/`** deps, builds **`taskapi`** to **`taskapi-dev(.exe)`**, frees the API port, starts **`taskapi`** then **`npm run dev`**. **Ctrl+C** stops both.

Non-default API port: set **`DEV_TASKAPI_PORT`** and **`VITE_TASKAPI_ORIGIN`** to match (see *Web UI*).

```powershell
.\scripts\dev.ps1
```

```bash
chmod +x ./scripts/dev.sh   # once if needed
./scripts/dev.sh
```

With **`taskapi`** on **`http://127.0.0.1:8080`** by default: REST at **`/tasks`**, SSE at **`/events`** — details in **`docs/DESIGN.md`**.

**Windows PowerShell:** use **`curl.exe`** and single-quoted JSON:

```powershell
curl.exe -s -X POST http://127.0.0.1:8080/tasks -H "Content-Type: application/json" -d '{"title":"live"}'
curl.exe -N http://127.0.0.1:8080/events
```

## Web UI (optional)

**`web/`** is a Vite + React + TypeScript SPA for task CRUD. Behavior, **`web/src`** layout, and diagrams: **`docs/WEB.md`**.

```bash
cd web
npm install
npm test
npm run dev
```

Opens Vite’s URL (often **`http://localhost:5173`**). Dev server **proxies** **`/tasks`**, **`/events`**, **`/repo`** to **`taskapi`** (**`web/vite.config.ts`**). **`VITE_TASKAPI_ORIGIN`** overrides the proxy target if the API is not **`http://127.0.0.1:8080`**.

| Command | Purpose |
|---------|---------|
| **`npm run dev`** | Dev server + proxy |
| **`npm test`** | Vitest (no real network; mocks) |
| **`npm run test:watch`** | Watch mode |
| **`npm run build`** | Typecheck → **`web/dist/`** |
| **`npm run preview`** | Preview **`dist`** (you still need API routing) |

**Production:** build static assets; serve **`dist`** same-origin as the API or behind a reverse proxy (**`taskapi`** does not serve **`dist`**). No CORS in the binary — see **`docs/DESIGN.md`** (limitations).

## For developers

```bash
go doc -all ./pkgs/tasks/...
go doc -all ./internal/envload ./cmd/taskapi ./cmd/dbcheck
```

**Web:** **`npm test`** from **`web/`**; details **`docs/WEB.md`**.
