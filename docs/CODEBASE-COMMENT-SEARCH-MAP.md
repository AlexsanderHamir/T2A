# Codebase map for comment-standard searches

This document breaks the repository into **search blocks** aligned with [`.cursor/rules/codebase_comments.mdc`](../.cursor/rules/codebase_comments.mdc). Use it to scope ripgrep, code review, or agent passes when auditing **why-not-what** comments, godoc/JSDoc contracts, SQL/migration headers, and maintenance markers (`TODO`, `eslint-disable`, `nolint`).

Canonical repo orientation remains [AGENTS.md](../AGENTS.md) and [docs/DESIGN.md](./DESIGN.md).

### Agent commit scope

Automated or agent-assisted commits should include **only** files that belong to the task at hand (files the agent created or intentionally edited for that task). Before committing, run `git status` and `git diff --staged`; avoid `git add -A` when the working tree contains unrelated local changes. Unrelated refactors belong in a separate commit by the author who owns that work.

---

## 1. Top-level layout (where to point search)

| Block | Path(s) | Comment focus (per MDC) |
|-------|---------|-------------------------|
| **Domain + errors** | `pkgs/tasks/domain/` | Package godoc; exported types/errors; non-obvious state rules |
| **HTTP surface** | `pkgs/tasks/handler/` | SSE ordering, middleware; exported handlers; request `Context` usage |
| **JSON helpers** | `pkgs/tasks/apijson/` | Security headers, error write helpers |
| **Middleware** | `pkgs/tasks/middleware/` | Layer order, timeout/idempotency rationale |
| **Call trace / logs** | `pkgs/tasks/calltrace/`, `pkgs/tasks/logctx/` | Context threading, log correlation |
| **Persistence** | `pkgs/tasks/store/`, `pkgs/tasks/postgres/` | Error mapping at DB boundary; `doc.go` in store trees; raw SQL intent |
| **Agents** | `pkgs/agents/`, `pkgs/agents/worker/`, `pkgs/agents/runner/` | Worker lifecycle, runner contracts |
| **Workspace repo** | `pkgs/repo/` | Optional `/repo` behavior |
| **Integration tests** | `internal/handlertest/`, `internal/middlewaretest/`, `pkgs/tasks/agentreconcile/` | Test-only; same comment bar for non-obvious setup |
| **Binaries** | `cmd/taskapi/`, `cmd/dbcheck/`, `cmd/funclogmeasure/` | Startup/shutdown, env |
| **Web SPA** | `web/src/` | Hooks (`tasks/hooks/`), API boundary (`api/`), components, `app/styles/` CSS magic numbers |
| **Deploy / ops** | `deploy/prometheus/` | Alert/recording rule intent |
| **Docs** | `docs/` | Product/API prose (not a substitute for code comments) |

**Schema note:** There are no checked-in versioned `.sql` migrations; schema evolves via GORM `AutoMigrate` ([docs/PERSISTENCE.md](./PERSISTENCE.md)). Comment standards for **complex raw SQL** apply under `pkgs/tasks/store/**` and similar, not migration files.

---

## 2. `doc.go` and package entry points (Go godoc baseline)

Ripgrep from repo root:

```bash
rg --files -g 'doc.go' pkgs/ internal/ cmd/
```

Notable package comments live beside: `pkgs/tasks/domain/doc.go`, `pkgs/tasks/store/doc.go`, `pkgs/tasks/handler/doc.go`, `pkgs/tasks/postgres/doc.go`, `pkgs/agents/worker/doc.go`, and several `store/internal/*/doc.go` files.

---

## 3. TypeScript / React blocks (JSDoc + inline)

| Area | Path | Search for |
|------|------|------------|
| SSE + query cache | `web/src/tasks/hooks/`, `web/src/tasks/task-query/` | Module comments; `useEffect` dependency omissions |
| API boundary | `web/src/api/` | `parse*` contracts; throw vs soft-fail |
| Feature UI | `web/src/tasks/components/`, `web/src/tasks/pages/` | Non-obvious layout/a11y |
| Styles | `web/src/app/styles/` | Magic numbers, z-index, browser quirks |

---

## 4. Recommended search recipes (MDC-aligned)

Run from repository root. Adjust paths when narrowing a PR.

### 4.1 Markers and suppressions

```bash
# TODO / FIXME / HACK in application code (should be structured per MDC §3.5)
rg -n "TODO|FIXME|HACK|XXX" pkgs/ internal/ cmd/ web/src --glob '!**/vendor/**'

# ESLint disables (need “why” inline per MDC §4.3)
rg -n "eslint-disable" web/src

# Go linter suppressions (need justification)
rg -n "//nolint|nolint:" pkgs/ internal/ cmd/
```

### 4.2 Context propagation (request path vs tests)

```bash
# Request handlers should prefer r.Context(); tests often use context.Background — scope to non-test when fixing prod
rg -n "context\\.(Background|TODO)\\(\\)" pkgs/tasks/handler --glob '!*_test.go'
```

### 4.3 React hook dependency discipline

```bash
rg -n "exhaustive-deps|react-hooks/exhaustive-deps" web/src
```

### 4.4 SQL and persistence

```bash
rg -n "Raw\\(|Exec\\(|QueryRow" pkgs/tasks/store pkgs/tasks/postgres
```

---

## 5. Baseline audit snapshot (2026-04-19)

Searches executed against this map:

| Check | Result (high level) |
|-------|---------------------|
| `TODO\|FIXME\|HACK` in `pkgs/`, `internal/`, `cmd/`, `web/src` | No matches in application source at time of scan (markers appear in docs and `.cursor/rules/` only). |
| `eslint-disable` in `web/src` | No matches. |
| `nolint` in Go | No matches. |
| `context.Background` in `pkgs/tasks/handler` excluding `*_test.go` | Non-test usage includes defensive fallback in `requestCtx` when `*http.Request` is nil (`handler_http_json.go`); remainder is test-only as expected. |
| `exhaustive-deps` | No matches in `web/src`. |

Re-run these commands before a comment-focused PR or when updating `.cursor/rules/codebase_comments.mdc`.

---

## 6. Related rules and docs

- [`.cursor/rules/codebase_comments.mdc`](../.cursor/rules/codebase_comments.mdc) — authoritative commenting standard
- [`docs/WEB.md`](./WEB.md) — `web/src` module map
- [`pkgs/tasks/handler/README.md`](../pkgs/tasks/handler/README.md) — handler file map
- [`pkgs/tasks/store/README.md`](../pkgs/tasks/store/README.md) — store facade map

---

## 7. Stage 2 — deeper pass (after baseline ripgrep)

Use this when moving from mechanical searches (§4–5) to judgment-heavy review:

1. **Exports:** Spot-check `godoc` on exported symbols in one package you touched (e.g. `pkgs/tasks/handler/` or `pkgs/tasks/domain/`): comments should state contract and non-obvious behavior, not restate the name (MDC §3.2).
2. **Raw SQL / complex queries:** For any `Raw`, multi-line `const q`, or planner-sensitive query in `pkgs/tasks/store/`, confirm there is intent where the SQL shape is non-obvious (MDC §5.2).
3. **React hooks:** For new or changed `useEffect` / `useLayoutEffect`, confirm dependency arrays are correct; if a dependency is intentionally omitted, the MDC requires an inline reason (and typically an eslint directive with that reason).
4. **CSS:** Flag new magic numbers in `web/src/app/styles/` for a short “why” comment (MDC §4.4).

---

## 8. Stage 2 execution log (sample pass)

Results from applying §7 to a narrow slice of the tree (not a full-repo audit):

| Focus | Finding |
|-------|---------|
| **Domain exports** | `pkgs/tasks/domain/errors.go` sentinel vars lacked godoc; added contract + HTTP mapping (see §3.2). `doc.go` index updated to link `[ErrConflict]`. |
| **Handler exports** | `Handler` struct had no godoc while `NewHandler` and options were documented; added a short construction contract (§3.2). |
| **Store facade** | `Store`, `NewStore`, and `SetReadyTaskNotifier` already documented in `store.go`; package `doc.go` covers architecture. No change in this pass. |
| **Raw SQL (`store/`)** | Production `Raw` uses are `COALESCE(MAX(...),0)` next-key allocation in `kernel.NextEventSeq`, `cycles.nextAttemptSeqInTx`, `cycles.nextPhaseSeqInTx`. `NextEventSeq` already documents concurrency and why `MAX(seq)` runs under a task row lock; the cycle helpers are unexported `MAX+1` patterns next to clear function names — no extra SQL comment added. |
| **Hooks** | `useEffect` sites under `web/src/tasks/hooks/` use normal dependency arrays; no `eslint-disable` for exhaustive-deps in the tree. |
| **CSS** | No new magic-number sweep in this pass; treat opportunistically when touching styles. |
| **API boundary (`parseTaskApi`)** | All exported `parse*` functions now have one-line JSDoc (route + contract); throws on invalid JSON per §4.2. |
