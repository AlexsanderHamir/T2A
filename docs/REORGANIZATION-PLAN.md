# Reorganization plan (PoC → maintainable backend)

This document is the **agreed roadmap** for cleaning structure and docs **before** major new surfaces (e.g. Cursor CLI). It is intentionally **phased**: each phase should leave the repo **buildable, tested, and shippable**.

## 1. Goals and non-goals

### Goals

- **Clear dependency direction:** domain has no I/O; store depends on domain + DB; HTTP layer depends on store + domain; `cmd` wires only.
- **Navigable docs:** a short “start here” path; **contracts** (routes, JSON, env) easy to find without reading 400+ lines of narrative.
- **Reviewable file sizes:** fewer “misc” files; handler and store split by **resource or concern**, not arbitrary churn.
- **One obvious place** for cross-cutting HTTP concerns (middleware, metrics, SSE) vs route handlers.

### Non-goals (for this pass)

- Rewriting persistence to raw `database/sql` everywhere (GORM can stay; optional later slice).
- Microservices or extracting `taskapi` into multiple binaries.
- Perfect DDD—aim for **boring, legible** Go layout.

## 2. Current pain (baseline)

| Area | Symptom | Notes |
|------|---------|--------|
| **`pkgs/tasks/handler`** | Many `*.go` files mixing routes, middleware, SSE, metrics, idempotency, tests | High cognitive load; unclear import graph for newcomers. |
| **`pkgs/tasks/store`** | Many `store_*.go` slices | Good for merge size, bad without a **map** of which file owns what. |
| **`docs/DESIGN.md`** | Long single file (~400+ lines) mixing API tables, diagrams, env, persistence, extensibility | Hard to patch without conflicts; hard to link from PRs. |
| **`pkgs/agents` vs `pkgs/tasks/store`** | Runtime wiring in `cmd/taskapi` is clear, but **domain of “agent delivery”** spans packages | Needs a short **architecture** page, not only env prose. |

## 3. Target package model (dependency rules)

```text
cmd/taskapi, cmd/dbcheck
    → internal/envload (optional future: internal/taskapiconfig)
    → pkgs/agents          (in-process queue; no import of handler)
    → pkgs/tasks/handler   (HTTP + SSE; imports store, domain, repo)
    → pkgs/tasks/store     (DB; imports domain)
    → pkgs/tasks/postgres  (dialect, migrate, open)
    → pkgs/tasks/domain    (types, sentinels; no store/handler)
    → pkgs/repo            (optional workspace)
```

**Rules**

- **`domain`** must not import `store`, `handler`, `postgres`, or `agents`.
- **`store`** must not import `handler`.
- **`agents`** may import `store` for reconcile typing (today); avoid importing `handler`.

## 4. Documentation reorganization (do first)

Documentation changes are **low risk** and **unlock** everything else (CLI and agents will read docs first).

### 4.1 Split `docs/DESIGN.md` into focused files

Suggested split (titles indicative):

| New file | Contents |
|----------|-----------|
| `docs/API-HTTP.md` | Route table, query/body limits, status codes, error JSON shape, idempotency header behavior. |
| `docs/API-SSE.md` | `GET /events` contract, event line format, reconnect behavior. |
| `docs/RUNTIME-ENV.md` | Env var table(s) for `taskapi` / `dbcheck`; defaults; log-related env. |
| `docs/PERSISTENCE.md` | GORM models overview, migrate story, SQLite vs Postgres notes, limitations. |
| `docs/AGENT-QUEUE.md` | Ready-task notifier, `MemoryQueue`, reconcile loop, fairness ordering, operational limits. |
| `docs/EXTENSIBILITY.md` | Move “how to add a field” slice from DESIGN (keep short). |

**`docs/DESIGN.md`** becomes a **≤2 screen** hub: purpose, links to the above, one architecture diagram, and “deprecated: full narrative moved to …” during transition.

### 4.2 Update indexes

- **`docs/README.md`**: add rows for each new file; keep “where to put updates” table accurate.
- **`AGENTS.md`**: replace long DESIGN pointer with “read `docs/README.md` then API-HTTP + RUNTIME-ENV for taskapi.”
- **HTTP/SSE contracts:** primary docs are **`docs/API-HTTP.md`** and **`docs/API-SSE.md`** (handler rule and autonomous-agent rule updated to match).

### 4.3 Style

- Prefer **tables + short paragraphs** over long prose in contract docs.
- Mermaid: **one diagram per doc** where it helps; avoid duplicating the same chart in three files.

**Exit criteria (phase 4):** contributors can answer “what’s the PATCH body for checklist?” without scrolling unrelated sections.

## 5. Go: `pkgs/tasks/handler` reorganization

**Problem:** one package, many files; hard to see “router vs middleware vs resource.”

### Option A (recommended for PoC): **subpackages under `internal/`**

Move HTTP implementation to something like:

```text
internal/taskapi/
  mux.go              // NewMux(deps) http.Handler — small
  middleware/         // accesslog, recovery, rate, auth, body, timeout, idempotency, metrics
  tasks/              // CRUD + list + stats + drafts + evaluate
  events/             // SSE + task event routes
  checklist/
  health/
  repo/               // thin wrappers → pkgs/repo (or keep repo_handlers in tasks if preferred)
```

`pkgs/tasks/handler` becomes either:

- **Deprecated shim** re-exporting `internal/taskapi` for one release, or
- **Deleted** after updating imports (`cmd/taskapi`, tests).

**Pros:** clear boundary; `cmd` imports one assembly package. **Cons:** larger move; fix tests in batches.

### Option B (lighter): **same package, clearer file naming + doc**

- Prefix files: `mw_*.go`, `route_tasks_*.go`, `route_events_*.go`, `sse.go` stays.
- Add **`handler/README.md`** (package-local) map: file → responsibility.

**Pros:** low churn. **Cons:** still one giant package; less compile-time isolation.

**Recommendation:** **Option B first** (1–2 PRs), then **Option A** if handler keeps growing.

**Exit criteria:** a new contributor finds the PATCH checklist handler in **one** predictable place.

## 6. Go: `pkgs/tasks/store` reorganization

**Today:** many `store_*.go` files (good for size) but needs a **map**.

### 6.1 Add `pkgs/tasks/store/README.md` (or `doc.go` extension)

Table: **concern → file(s)** (CRUD, tree, events, checklist, drafts, health, ready-queue lists, devsim, …).

### 6.2 Optional later: subpackages

Only if store keeps growing:

```text
pkgs/tasks/store/
  crud/
  query/      // list, forest, stats
  events/
  checklist/
  drafts/
```

**Risk:** cyclic imports if not careful; **defer** until README map proves insufficient.

**Exit criteria:** every new store method has an obvious home before the PR lands.

## 7. Go: `pkgs/agents` and `cmd/taskapi`

- Keep **`pkgs/agents`** small: queue + reconcile; **no** HTTP types.
- Move **env parsing** for queue/reconcile to **`internal/taskapiconfig`** (optional) so `cmd/taskapi/run.go` stays “wire only” per engineering bar.
- Document in **`docs/AGENT-QUEUE.md`** how `store.ReadyTaskNotifier` relates to `MemoryQueue` and tests (`agentreconcile`).

## 8. Testing and CI strategy during moves

- **No big-bang:** each PR moves one slice (e.g. “split DESIGN only” or “handler middleware subfolder”).
- After each phase: `scripts/check.ps1` / CI green.
- Prefer **import aliases / thin wrappers** over massive rename PRs when possible.

## 9. Suggested sequencing (milestones)

| Milestone | Deliverable | Risk |
|-----------|-------------|------|
| **M1** | Split `DESIGN.md` + update `docs/README.md` + `AGENTS.md` links | Low — **done** (hub `DESIGN.md` + `API-HTTP.md`, `API-SSE.md`, `RUNTIME-ENV.md`, `AGENT-QUEUE.md`, `PERSISTENCE.md`, `EXTENSIBILITY.md`; indexes and cross-links updated). |
| **M2** | `docs/AGENT-QUEUE.md` + store `README` map | Low — **done** (`docs/AGENT-QUEUE.md` from M1; `pkgs/tasks/store/README.md` concern→file map + `doc.go` pointer; `docs/PERSISTENCE.md` link). |
| **M3** | Handler file naming / grouping + package README map (Option B) | Medium — **README done** (`pkgs/tasks/handler/README.md`, `doc.go` + `handler.go` pointers). Optional **`mw_*` / `route_*` renames** deferred (high churn). |
| **M4** | Extract `internal/taskapi` (Option A) OR defer if M3 suffices | Medium–High — **slice 1 done** (`internal/taskapi.NewHTTPHandler` = middleware assembly moved out of `cmd/taskapi`; splitting `pkgs/tasks/handler` into subpackages still **deferred**). |
| **M5** | Optional `internal/taskapiconfig` for env | Low — **done** (`internal/taskapiconfig`: agent queue, reconcile, listen host, log level, minimized logging, dev SSE interval; `cmd/taskapi` slimmed; tests moved). |

**Stop line:** if M1–M3 achieve clarity for Cursor CLI work, **pause** M4 until the CLI stresses the layout.

**Post-reorg polish (ongoing):** keep `docs/DESIGN.md` hub, `RUNTIME-ENV.md`, `API-HTTP.md`, and `CONTRIBUTING.md` pointing at `internal/taskapi` / `internal/taskapiconfig` whenever wiring moves. **`cmd/taskapi/README.md`** documents the binary’s `*.go` files and wiring-only imports.

## 10. What not to do

- Do not rename `module` path or move `domain` types casually (breaks importers and mental model).
- Do not split packages purely by line count mid-function.
- Do not duplicate API tables in README, DESIGN hub, and API-HTTP—**single authoritative table** per concern.

## 11. Success metrics (lightweight)

- **Time-to-answer** for “where is X implemented?” (informal: new contributor trial).
- **PR conflict rate** on docs (should drop after DESIGN split).
- **Lines in `docs/DESIGN.md` hub** (target: short).

---

**Next action:** execute **M1** as the first PR (documentation only), then reassess before touching `handler` imports.
