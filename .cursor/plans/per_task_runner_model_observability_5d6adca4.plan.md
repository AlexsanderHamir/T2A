---
name: Per-task runner & model attribution + observability slice-and-dice
overview: "Each task can pick its own runner and `cursor_model`, but neither the per-task UI nor the observability page surfaces which runner/model actually ran each attempt or how runs perform per (runner, model). Make the runner+model the first-class identity of an execution: persist it on every cycle, render it on the task detail (header + cycle history + live ticker), expose a `by_runner` / `by_model` / `by_runner_model` aggregation on `GET /tasks/stats`, and add a Runner/Model breakdown panel to the Observability page wired to the same SSE invalidation as the existing KPIs. Add the `model` Prometheus label so dashboards/alerts can slice by it too."
todos:
  - id: p1_cycle_meta_model
    content: "Phase 1a: persist cursor_model alongside runner+runner_version in TaskCycle.MetaJSON (worker.buildCycleMeta) and bump the wire shape on GET /tasks/{id}/cycles[/{cycleId}]"
    status: pending
  - id: p1_handler_cycle_meta_typed
    content: "Phase 1b: add typed cycle_meta projection (runner/runner_version/cursor_model/prompt_hash) to handler_cycles_json so the SPA never has to parse free-form meta"
    status: pending
  - id: p1_web_cycle_types
    content: "Phase 1c: extend web/src/types/cycle.ts TaskCycle with typed cycle_meta and update parseTaskApi cycles parser + tests"
    status: pending
  - id: p2_stats_by_runner_model
    content: "Phase 2a: add ByRunner / ByModel / ByRunnerModel maps + RunDurationP50P95 to pkgs/tasks/store/internal/stats.TaskStats; one new SQL pass over task_cycles ⨝ tasks; null model bucketed as \"\""
    status: pending
  - id: p2_handler_stats_wire
    content: "Phase 2b: extend GET /tasks/stats response with by_runner / by_model / by_runner_model + duration percentiles; pin in handler_http_list_stats_contract_test.go"
    status: pending
  - id: p2_web_stats_types
    content: "Phase 2c: extend web/src/types/task.ts TaskStatsResponse with by_runner / by_model / by_runner_model and run-duration percentiles; update parseTaskApi stats parser + tests"
    status: pending
  - id: p3_metrics_model_label
    content: "Phase 3: add model label to t2a_agent_runs_total and t2a_agent_run_duration_seconds in internal/taskapi/agent_worker_metrics.go (worker.RunMetrics.RecordRun signature gains a model arg); update process.go + cleanup.go + e2e assertion"
    status: pending
  - id: p4_task_header_runtime
    content: "Phase 4a: TaskDetailHeader gains a small \"runtime\" tag chain showing runner + cursor_model with default-model fallback styled like the existing pills"
    status: pending
  - id: p4_cycle_runner_chip
    content: "Phase 4b: TaskCyclesPanel — add a runner/model chip on each CycleRow summary and the CurrentPhaseTicker eyebrow; defensive when meta missing"
    status: pending
  - id: p4_view_model_helpers
    content: "Phase 4c: cyclesViewModel.ts — add formatRunnerModel + RUNNER_LABELS single-source-of-truth + cycleRunnerChipClass helpers re-exported from @/observability"
    status: pending
  - id: p5_obs_breakdown_panel
    content: "Phase 5a: new ObservabilityRunnerBreakdown.tsx mounted in ObservabilityPage between Overview and Cycles — per-runner table with per-model rows; columns: total / succeeded / failed / aborted / p50 / p95"
    status: pending
  - id: p5_obs_styles
    content: "Phase 5b: observability.css — table + sparkline-row classes; reuse cell-pill--cycle-* swatches; respects prefers-reduced-motion"
    status: pending
  - id: p5_obs_tests
    content: "Phase 5c: ObservabilityRunnerBreakdown.test.tsx — empty / single-runner / multi-runner-with-models / collapsed-default-model cases"
    status: pending
  - id: p6_docs
    content: "Phase 6: docs/EXECUTION-CYCLES.md (cycle_meta), docs/API-HTTP.md (/tasks/stats), docs/OBSERVABILITY.md (new metric label + new panel), docs/AGENT-WORKER.md (model label) — and TROUBLESHOOTING.md for empty buckets"
    status: pending
  - id: rollout_backfill
    content: "Rollout: no schema migration needed (MetaJSON is jsonb); add a one-shot startup log line counting cycles with empty cursor_model in meta so operators know how much of the history is pre-feature; keep the breakdown defensive for those rows"
    status: pending
isProject: false
---

## Goals & non-goals

**Goals**
- The task detail page answers "what runner and model handled this task?" for both the latest attempt (header) and every historical attempt (cycle history), without an extra API call.
- The Observability page answers "how does success rate / latency vary across runners and models?" with a single panel that updates on the same SSE invalidation as the rest of the KPIs.
- Prometheus dashboards can slice agent runs and run-duration by (runner, model) so operators can detect a bad model rollout (`model=opus → terminal_status=failed` spiking) without adding ad-hoc instrumentation.
- Wire shape, parser, and contract tests stay the chokepoint: a future runner/model addition trips the parser before it ships.

**Non-goals (this plan)**
- Adding new runners (Claude Code, Codex). The runner registry already supports them; this plan only makes the existing one observable per-cycle.
- Per-token / per-cost accounting. We surface terminal status and wall-clock duration only — token/cost belongs in a future RUM pass.
- Editing model selection from the task detail page. Selection stays in `SettingsPage` and the task create form (`web/src/types/task.ts:301-302`); this plan is read-side.
- Schema migrations. `task_cycles.meta_json` is already `jsonb` with default `{}`; we widen the payload, not the table.
- Backfilling pre-feature cycles. Old cycles will render `runner: cursor · default model` (their actual runner is known from `tasks.runner` but the model at the time isn't recoverable); the rollout log line tells operators the magnitude.

---

## Architecture

```mermaid
flowchart LR
  Settings[app_settings.cursor_model] --> WorkerCfg[worker config]
  TaskRow[(tasks.runner / cursor_model)] --> WorkerReq[runner.Request]
  WorkerCfg --> WorkerReq
  WorkerReq --> Adapter[runner adapter]
  Adapter --> Result[runner.Result]
  Result --> Worker[worker.processOne]
  Worker --> CycleMeta[buildCycleMeta:<br/>runner, runner_version,<br/>cursor_model, prompt_hash]
  CycleMeta --> CyclesTable[(task_cycles.meta_json)]
  Worker --> Metrics[(t2a_agent_runs_total<br/>{runner, model, terminal_status})]
  CyclesTable --> StatsAgg[stats.Get<br/>by_runner / by_model / by_runner_model]
  CyclesTable --> CycleAPI[GET /tasks/:id/cycles]
  StatsAgg --> StatsAPI[GET /tasks/stats]
  CycleAPI --> SPA1[TaskDetailHeader + TaskCyclesPanel]
  StatsAPI --> SPA2[ObservabilityRunnerBreakdown]
  SSE[task_cycle_changed / settings_changed] -. invalidate .-> SPA1
  SSE -. invalidate ["task-stats"] .-> SPA2
```

The contract: **the (runner, cursor_model) tuple at the moment of `StartCycle` is the first-class identity of one attempt**. Tasks own the *intent* (`tasks.runner` / `tasks.cursor_model`); cycles own the *outcome* (`meta_json.runner` / `meta_json.cursor_model`). The two can diverge (e.g. operator changed the default model between attempt #1 and attempt #2), and that divergence is exactly what the breakdown panel is supposed to make visible.

---

## Phase 1 — Cycle-level runner/model audit

**Touchpoints**: `pkgs/agents/worker/meta.go`, `pkgs/agents/worker/process.go`, `pkgs/tasks/handler/handler_cycles.go`, `pkgs/tasks/handler/handler_cycles_json.go`, `web/src/types/cycle.ts`, `web/src/api/parseTaskApi*.ts`.

### 1a. Widen `buildCycleMeta`
- File: [`pkgs/agents/worker/meta.go`](pkgs/agents/worker/meta.go)
- Add a `task *domain.Task` argument so the worker can record the model the task asked for. Today the signature is `buildCycleMeta(r runner.Runner, prompt string)`; new signature: `buildCycleMeta(r runner.Runner, task *domain.Task, prompt string)`.
- Append `"cursor_model": task.CursorModel` (empty string when default) to the JSON object. Keys to preserve byte-for-byte: `runner`, `runner_version`, `prompt_hash`. New key: `cursor_model`.
- Update the call site in `process.go` (~L209-215) to pass `task`.
- Test: extend `meta_test.go` (or add one) to assert the four-key shape and that an empty `CursorModel` serialises as `""` (not omitted) — the SPA wants a stable shape, not a sometimes-present key.

### 1b. Typed `cycle_meta` on the wire
- File: [`pkgs/tasks/handler/handler_cycles_json.go`](pkgs/tasks/handler/handler_cycles_json.go)
- Today the responses (`cycleResponse`, `cycleDetailResponse`, etc) carry `Meta json.RawMessage`. Add a sibling typed projection so the SPA never has to parse free-form JSON to render the chip:

```go
type cycleMetaProjection struct {
    Runner        string `json:"runner"`
    RunnerVersion string `json:"runner_version"`
    CursorModel   string `json:"cursor_model"`
    PromptHash    string `json:"prompt_hash"`
}
```

- Populate in the cycle marshallers by `json.Unmarshal`-ing `cycle.MetaJSON` into the struct (defensive: zero-value on parse failure, log at Debug per existing convention in `scan_failures.go`).
- Wire field name: `cycle_meta`. Keep `meta` (raw) for forward-compat — anything new added to `meta_json` later (e.g. `tokens_in`) shows up there without a wire bump.
- Pin in `handler_cycles_*_test.go`: assert `cycle_meta.runner == "cursor"` after the worker runs.

### 1c. Web types + parser
- File: [`web/src/types/cycle.ts`](web/src/types/cycle.ts)
- Extend `TaskCycle` with:

```ts
export type CycleMeta = {
  runner: string;
  runner_version: string;
  cursor_model: string;
  prompt_hash: string;
};

export type TaskCycle = {
  // ...existing fields
  meta: Record<string, unknown>;
  cycle_meta: CycleMeta;
};
```

- Update `web/src/api/parseTaskApi*.ts` (the cycles parser) to validate the four string fields with the existing `expectString` helper; missing → empty string (defensive, matches the backend's zero-value-on-parse-fail). Pre-feature cycles get all-empty strings.
- Tests: extend `parseTaskApi.cycles.test.ts` for the new field; assert default-model behaviour (`cursor_model === ""`).

---

## Phase 2 — `/tasks/stats` slice-and-dice

**Touchpoints**: `pkgs/tasks/store/internal/stats/{stats.go,scan.go,scan_runner.go (new)}`, `pkgs/tasks/handler/handler_stats.go`, `web/src/types/task.ts`, `web/src/api/parseTaskApi*.ts`.

### 2a. Aggregate in the store
- File: [`pkgs/tasks/store/internal/stats/stats.go`](pkgs/tasks/store/internal/stats/stats.go)
- Extend `TaskStats`:

```go
type TaskStats struct {
    // ...existing
    Runner    RunnerStats
}

type RunnerStats struct {
    // by_runner["cursor"]["succeeded"|"failed"|"aborted"|"running"] = count
    ByRunner       map[string]map[domain.CycleStatus]int64
    // by_model[""|"opus"|...]  same value shape
    ByModel        map[string]map[domain.CycleStatus]int64
    // composite key "runner|model" → status counts; the SPA pivots
    // this into a runner ⊃ model tree without another round-trip.
    ByRunnerModel  map[string]map[domain.CycleStatus]int64
    // p50/p95 wall-clock seconds per runner|model, computed in SQL
    // via percentile_cont (Postgres) / percentile-of-grouped-array
    // (SQLite test path); empty when no terminal cycles for that key.
    DurationP50    map[string]float64
    DurationP95    map[string]float64
}
```

- New file: `scan_runner.go` with one query that joins `task_cycles c` with `tasks t` on `c.task_id = t.id`, groups by `(t.runner, t.cursor_model, c.status)`, and sums `count(*)` plus `epoch(c.ended_at - c.started_at)` for the percentile pass. SQLite test path: emulate `percentile_cont` with a Go-side bucketing pass (the `agentreconcile` test fixture is the only place this matters; keep the Go path as the canonical implementation and Postgres-special-case the SQL when measurement says it's worth it).
- Always seed `ByRunner["cursor"]`, `ByModel[""]`, `ByRunnerModel["cursor|"]` so the wire shape is stable on a fresh DB (matches the `allPhases` seeding pattern at L94).
- Cardinality guard: cap distinct model values at 32 per runner; bucket overflow into `__other__`. Today there are < 10 cursor models; the cap is a future-proof fence, not a current need.

### 2b. Wire shape + handler
- File: [`pkgs/tasks/handler/handler_stats.go`](pkgs/tasks/handler/handler_stats.go) (or whichever owns the marshaller)
- New JSON section:

```json
{
  "runner": {
    "by_runner": { "cursor": { "succeeded": 12, "failed": 3, "aborted": 0, "running": 1 } },
    "by_model":  { "": {"succeeded": 4, "failed": 1}, "opus": {"succeeded": 8, "failed": 2, "aborted": 0, "running": 1} },
    "by_runner_model": { "cursor|": {...}, "cursor|opus": {...} },
    "duration_p50_seconds": { "cursor|": 12.3, "cursor|opus": 41.7 },
    "duration_p95_seconds": { "cursor|": 18.0, "cursor|opus": 92.4 }
  }
}
```

- Pin `handler_http_list_stats_contract_test.go`: presence of all four sub-objects on a fresh DB (empty inner maps) and a populated case.

### 2c. Web types + parser
- File: [`web/src/types/task.ts`](web/src/types/task.ts)
- Extend `TaskStatsResponse` with a `runner: TaskStatsRunner` field; mirror the JSON shape one-to-one; document `cursor_model === ""` as "default model".
- Update `parseTaskApi.stats.test.ts` to cover the new shape (presence + numeric coercion).

---

## Phase 3 — Prometheus model label

**Touchpoints**: `pkgs/agents/worker/{metrics.go,process.go,cleanup.go,worker.go,meta.go}`, `internal/taskapi/agent_worker_metrics.go`, `pkgs/tasks/agentreconcile/agent_real_cursor_e2e_test.go`, `deploy/prometheus/t2a-taskapi-rules.yaml`.

- File: [`pkgs/agents/worker/metrics.go`](pkgs/agents/worker/metrics.go)
- Change the seam:

```go
type RunMetrics interface {
    RecordRun(runner, model, terminalStatus string, duration time.Duration)
}
```

- Update `(*Worker).recordRun` to accept and pass `model`. Source: the `runner.Request.CursorModel` actually used (worker already knows it; thread it through `processState` since we need the value at terminate time, including the cleanup paths).
- File: [`internal/taskapi/agent_worker_metrics.go`](internal/taskapi/agent_worker_metrics.go) — add `"model"` to both `[]string{"runner", ...}` label sets. Cardinality: bounded by the same 32-per-runner fence as Phase 2a; for production today this stays single-digit.
- E2E: extend [`pkgs/tasks/agentreconcile/agent_real_cursor_e2e_test.go:665`](pkgs/tasks/agentreconcile/agent_real_cursor_e2e_test.go) to assert the `model` label is present (value may be empty for "default model" runs).
- Update the recording rule file in `deploy/prometheus/t2a-taskapi-rules.yaml` to keep aggregations valid (drop `model` in the `sum by (runner, terminal_status)` rules so existing dashboards don't double-count).

---

## Phase 4 — Per-task UI: surface runner/model on every attempt

**Touchpoints**: `web/src/tasks/components/task-detail/layout/TaskDetailHeader.tsx`, `web/src/tasks/components/task-detail/cycles/TaskCyclesPanel.tsx`, `web/src/observability/cyclesViewModel.ts`, `web/src/observability/index.ts`, `web/src/app/styles/app-task-detail.css`.

### 4a. Header runtime tag (incremental, not a redo)
- File: [`web/src/tasks/components/task-detail/layout/TaskDetailHeader.tsx`](web/src/tasks/components/task-detail/layout/TaskDetailHeader.tsx)
- Today the header renders a single muted `<p>` for runner + model (L54-59). Replace with two `cell-pill`-styled chips so it visually registers as identity, not metadata:
  - chip 1: `runner cursor` (uses `cell-pill cell-pill--runner-cursor`).
  - chip 2: `model opus` or `default model` when empty.
- Keep the existing `task-detail-agent-meta` class on the wrapping element so existing tests / styles still target it; the visual is additive.

### 4b. Cycle history runner/model chip
- File: [`web/src/tasks/components/task-detail/cycles/TaskCyclesPanel.tsx`](web/src/tasks/components/task-detail/cycles/TaskCyclesPanel.tsx)
- In `CycleRow` (L311+) summary, after the "by triggered_by" span, render a small chip from `cycle.cycle_meta`:

```tsx
<span className="task-cycle-row-runner muted" data-testid="task-cycle-row-runner">
  {formatRunnerModel(cycle.cycle_meta)}
</span>
```

- In `CurrentPhaseTicker` (L148+), add the same chip to the eyebrow row. The live ticker already invalidates on `task_cycle_changed`, so a model change between attempts will re-render automatically.
- Defensive: when `cycle_meta.runner === ""` (pre-feature cycles), render `unknown runner` muted; when it's set but `cursor_model === ""`, render `cursor · default model`.

### 4c. View-model helpers
- File: [`web/src/observability/cyclesViewModel.ts`](web/src/observability/cyclesViewModel.ts)
- Add `RUNNER_LABELS = { cursor: "Cursor CLI" }` (single source of truth — currently duplicated in `TaskDetailHeader`; this phase eliminates the duplicate).
- Add `formatRunnerModel(meta: CycleMeta): string` returning `"Cursor CLI · opus"` / `"Cursor CLI · default model"` / `"unknown runner"`.
- Re-export both from [`web/src/observability/index.ts`](web/src/observability/index.ts) so `tasks/components/task-detail/cycles/` consumes the same helpers as the obs page.
- Tests: pure-function tests next to the helper.

---

## Phase 5 — Observability page: Runner / Model breakdown

**Touchpoints**: new `web/src/observability/ObservabilityRunnerBreakdown.tsx` + test, `web/src/observability/ObservabilityPage.tsx`, `web/src/observability/observability.css`.

### 5a. The panel
- New file: `ObservabilityRunnerBreakdown.tsx`. Receives `stats: TaskStatsResponse | null | undefined` and `loading: boolean` (matches the contract used by `ObservabilityCycles` / `ObservabilityOverview`).
- Layout: a two-level table.
  - Outer rows: one per runner (currently `cursor`).
  - Inner rows (collapsible via native `<details>`, matching `TaskCyclesPanel.CycleRow`): one per model in `by_runner_model` whose key starts with `runner|`. Show `(default)` for empty model. Newest-attempts-first ordering by `total = sum(status counts)`.
  - Columns:
    1. Runner / model name (chip styled like 4a/4b).
    2. Total cycles.
    3. Succeeded (`cell-pill--cycle-succeeded`).
    4. Failed.
    5. Aborted.
    6. Running.
    7. Success rate (`succeeded / (succeeded+failed+aborted)`); color via existing semantic tokens.
    8. p50 wall-clock (`formatDurationSeconds` from `@/observability`).
    9. p95 wall-clock.
- Top-of-panel summary line: "N runners · M models · X total cycles".
- Edge cases:
  - Empty database → identical empty-state pattern as `ObservabilityCycles`.
  - Single runner with one model → still render the table (collapsing it into a one-line KPI hides the *contract* of "we attribute by runner+model").
  - Pre-feature cycles → bucket under `cursor|` (default model); the breakdown stays correct because the backend reads `tasks.runner` / `tasks.cursor_model` not `meta_json`.

### 5b. Styles
- File: [`web/src/observability/observability.css`](web/src/observability/observability.css)
- New classes: `.obs-runner-breakdown`, `.obs-runner-breakdown-table`, `.obs-runner-row`, `.obs-runner-row--collapsible`, `.obs-runner-cell--success-rate`. Reuse `--surface-card`, `--border-hairline`, `--space-*` tokens. Sticky table header uses the same pattern as `RecentFailuresTable`.
- Animation: success-rate cell number uses `useAnimatedNumber` (already pulled in by Phase 3d of the realtime smoothness plan; if that plan hasn't shipped yet, gate behind a graceful fallback to the plain number).

### 5c. Tests
- New file: `ObservabilityRunnerBreakdown.test.tsx`. Cases:
  - Empty stats (loading and resolved).
  - Single runner, single model, several cycles — verify all six columns + success-rate math.
  - Single runner, two models — verify collapsible expand/collapse, default model labelled correctly.
  - Multiple runners (synthesise with `claude` even though no adapter exists yet — exercises the per-runner ordering and totals).

### 5d. Mount
- File: [`web/src/observability/ObservabilityPage.tsx`](web/src/observability/ObservabilityPage.tsx)
- Insert `<ObservabilityRunnerBreakdown stats={stats} loading={loading} />` between `ObservabilityOverview` and `ObservabilityCycles` so the operator's eye lands on it after the top-level KPIs and before the per-phase heatmap (the natural drill-down order).

---

## Phase 6 — Docs

- [`docs/EXECUTION-CYCLES.md`](docs/EXECUTION-CYCLES.md): new "Cycle metadata" subsection documenting the four-key `cycle_meta` shape and what consumers can rely on (stable keys, defensive empty strings).
- [`docs/API-HTTP.md`](docs/API-HTTP.md): extend the `/tasks/stats` body section with `runner.{by_runner,by_model,by_runner_model,duration_p50_seconds,duration_p95_seconds}`; extend the `/tasks/{id}/cycles[/{cycleId}]` sections with `cycle_meta`.
- [`docs/OBSERVABILITY.md`](docs/OBSERVABILITY.md): document the new `model` label on `t2a_agent_runs_total` / `t2a_agent_run_duration_seconds`; add a "Runner / model breakdown" subsection pointing at the new SPA panel.
- [`docs/AGENT-WORKER.md`](docs/AGENT-WORKER.md): the metrics table at L383+ needs the `model` label row.
- [`docs/TROUBLESHOOTING.md`](docs/TROUBLESHOOTING.md): "Empty model column on the breakdown panel" → explain pre-feature cycles + the rollout log line.

---

## Rollout & migration

- **No schema migration**: `task_cycles.meta_json` is `jsonb default '{}'`; widening the producer is forward-compat.
- **Pre-feature backfill is intentionally skipped**. We log once on supervisor start:

```
slog.Info("agent worker: pre-feature cycles", "count", N,
    "operation", "taskapi.agentWorkerSupervisor.startup.preFeatureCycleCount")
```

  where `N` is `SELECT count(*) FROM task_cycles WHERE meta_json::jsonb -> 'cursor_model' IS NULL` (Postgres) / equivalent JSON1 query (SQLite). Gives operators a single number to decide if they want to run a one-shot script later.
- **Prometheus dashboards / alerts**: the `model` label is additive, but `sum without (model) (rate(t2a_agent_runs_total[5m]))` keeps existing single-runner panels working unchanged. Update `deploy/prometheus/t2a-taskapi-rules.yaml` recording rules in the same PR so we never publish a build with mismatched cardinalities.
- **No feature flag**: every change is read-side or additive metadata; rollback is `git revert` of the Phase 5 mount + Phase 3 label add. Phases 1–2 (server-side widening) are safe to leave in production even if the SPA pieces are reverted, because the SPA already tolerates extra wire fields (`parseTaskApi` only validates known keys).

---

## Test bar (per `06-testing.mdc` and `10-web-ui.mdc`)

- Backend: extend the existing handler contract tests; new `stats.scan_runner_test.go` over the SQLite test fixture; e2e adjustment in `agentreconcile`.
- Frontend: per-component tests (Phase 4 + 5); parser tests (Phase 1c + 2c).
- `funclogmeasure -enforce`: every new exported func gets the `slog.Debug("trace", ...)` opening per [`docs/OBSERVABILITY.md`](docs/OBSERVABILITY.md).
- Local check: `.\scripts\check.ps1` from repo root must pass before commit.

---

## Open questions (deliberately punted)

1. **Per-attempt model override on the task detail page**: the task model is set at create time / via SettingsPage. A "rerun on model X" affordance could live on the `TaskDetailAttentionBar` requeue path. Out of scope for this plan; the breakdown panel will tell operators *whether* it's worth building.
2. **Cost / token observability**: would extend `cycle_meta` and the `runner` aggregations with `tokens_in / tokens_out / usd_estimate`. Cleanly orthogonal — same `cycle_meta` chokepoint, same `RunnerStats` shape with two more value maps. Defer until a runner adapter actually surfaces the data (Cursor CLI does in `--output-format json`; today we discard it).
3. **Per-task duration percentiles** (vs. per-runner-model). Same query shape, different `GROUP BY`. Punt until a user actually asks; the per-cycle wall-clock is already on `TaskCyclesPanel` row-by-row.
