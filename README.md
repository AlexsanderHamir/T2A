# T2A

**T2A** is for **delegating lots of tasks to agents** and keeping humans and automation on the same page. Tasks live in **one shared place**, you **create and update them through a web API**, every important change is **recorded** (who did what), and **UIs or runners can listen for updates** instead of polling constantly.

This repo is the Go implementation (**`github.com/AlexsanderHamir/T2A`**). For a fuller picture of how it works and where the rough edges are, see **`docs/DESIGN.md`**.

## Prerequisites

- **Go** 1.25+
- A **Postgres** database and a **`.env`** file at the repo root (gitignored) with **`DATABASE_URL`** pointing at it.

## Build and test

```bash
go build ./...
go test ./...
```

## Run

```bash
go run ./cmd/dbcheck    # checks the database; add -migrate to update tables
go run ./cmd/taskapi    # starts the web server; add -h for -port, -env, -migrate
```

### API + web UI together

From the **repo root** (same **`.env`** / **`DATABASE_URL`**). **`scripts/dev.ps1`** (Windows) or **`scripts/dev.sh`** (Git Bash / macOS / Linux) installs **`web/`** deps, builds **`taskapi`** to **`taskapi-dev(.exe)`**, frees the API port if needed, starts **`taskapi`** then **`npm run dev`**. **Ctrl+C** stops Vite and the API.

If you use a port other than **8080**, set **`DEV_TASKAPI_PORT`** and match **`VITE_TASKAPI_ORIGIN`** for Vite (see *Web UI* below).

```powershell
.\scripts\dev.ps1
```

```bash
chmod +x ./scripts/dev.sh   # once, if needed
./scripts/dev.sh
```

With **`taskapi`** running (by default **`http://127.0.0.1:8080`**):

- Work with tasks at **`/tasks`** and **`/tasks/{id}`** — methods, query options, and error behavior are described in **`docs/DESIGN.md`**.
- Open **`/events`** in a client that supports **live streams** to hear when tasks change (same doc explains the format).

**Windows PowerShell:** use **`curl.exe`** and single-quoted JSON so Windows does not treat `curl` as a different command:

```powershell
curl.exe -s -X POST http://127.0.0.1:8080/tasks -H "Content-Type: application/json" -d '{"title":"live"}'
curl.exe -N http://127.0.0.1:8080/events
```

## Web UI (optional)

A **Vite + React + TypeScript** SPA in **`web/`** implements task **create, list, edit, and delete** against the same **`/tasks`** API as scripts and agents. **TanStack Query** holds server state (deduplication, cancellation, refetch on focus), **`GET /events`** (SSE) **invalidates** the list on a short debounce so bursts of events do not each trigger a full refetch, and **Delete** uses an **in-app confirmation** (not the browser’s `window.confirm`) so focus and typing keep working in embedded or Chromium-based hosts.

### Dev flow (browser → Vite → `taskapi`)

```mermaid
flowchart LR
  B[Browser on Vite port]
  V[Vite dev server]
  A["taskapi :8080"]
  B -->|HTML/JS/CSS| V
  V -->|proxies /tasks and /events| A
```

### Prerequisites

- **Node.js** (current LTS is fine) and **npm**, for **`web/`** only.

### Run locally

1. Start **`taskapi`** on **`127.0.0.1:8080`** (default).
2. In another terminal:

```bash
cd web
npm install
npm test
npm run dev
```

Open the URL Vite prints (usually **`http://localhost:5173`**). In dev, **`/tasks`** and **`/events`** are **proxied** to the API (see **`web/vite.config.ts`**), so the Go server does not need **CORS** for local UI work.

**Proxy target:** set **`VITE_TASKAPI_ORIGIN`** when starting Vite if `taskapi` is not at `http://127.0.0.1:8080` (for example `http://127.0.0.1:9090`).

### Scripts (run inside **`web/`**)

| Command | Purpose |
|--------|---------|
| **`npm run dev`** | Vite dev server with proxy. |
| **`npm test`** | **Vitest** + Testing Library (no real network; **`fetch`** / **`EventSource`** mocked). |
| **`npm run test:watch`** | Vitest watch mode. |
| **`npm run build`** | Typecheck and emit production assets to **`web/dist/`**. |
| **`npm run preview`** | Local preview of the **`dist`** build (still expects API reachability; configure proxy or same-origin yourself). |

### Source layout (`web/src/`)

| Path | Role |
|------|------|
| **`App.tsx`** | Root layout; wires **`useTasksApp`** to presentational components. |
| **`hooks/useTasksApp.ts`** | Form and dialog state, React Query **mutations** for create / update / delete, and composed list query. |
| **`hooks/useTaskEventStream.ts`** | **`EventSource`** on **`/events`**, debounced **`invalidateQueries`** for the task list. |
| **`queryClient.ts`** | Shared **QueryClient** defaults (stale time, retries, dev-only error logging). |
| **`taskQueryKeys.ts`** | Stable **query key** factory for the task list. |
| **`api.ts`** | Typed **`fetch`** wrappers for **`/tasks`** (and errors). |
| **`parseTaskApi.ts`** | Runtime checks on JSON from **`/tasks`** before the UI uses it. |
| **`types.ts`** | JSON-aligned TypeScript types. |
| **`components/`** | **`TaskCreateForm`**, **`TaskListSection`**, **`TaskEditForm`**, **`DeleteConfirmDialog`**, shared **`StatusSelect`** / **`PrioritySelect`**, **`ErrorBanner`**, **`StreamStatusHint`**. |
| **`test/`** | Vitest **`setup`** (RTL **`cleanup`**), **`EventSource`** stub, **`requestUrl`** helper for mocks. |

### Production / deployment

**`npm run build`** produces static files under **`web/dist/`**. The Go **`taskapi`** binary does **not** serve this folder. In production, either:

- Serve **`dist`** from the **same origin** as the API (single host, path routing), or  
- Put a **reverse proxy** in front that forwards **`/tasks`** and **`/events`** to **`taskapi`** and serves the SPA for other paths.

The API ships **without CORS** for arbitrary third-party origins; see **`docs/DESIGN.md`** (limitations). Adding CORS or embedding static files in Go would be a separate change.

## For developers

**Go:** route and type details live next to the code. From the repo root:

```bash
go doc -all ./pkgs/tasks/...
go doc -all ./internal/envload ./cmd/taskapi ./cmd/dbcheck
```

**Web:** TypeScript sources and tests are under **`web/src/`**; run **`npm test`** from **`web/`** (see *Web UI* above).
