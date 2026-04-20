---
name: Per-task runner & model attribution + observability slice-and-dice
overview: "Each task can pick its own runner and `cursor_model`, but neither the per-task UI nor the observability page surfaces which runner/model actually ran each attempt or how runs perform per (runner, model). Make the runner+model the first-class identity of an execution: persist it on every cycle, render it on the task detail (header + cycle history + live ticker), expose a `by_runner` / `by_model` / `by_runner_model` aggregation on `GET /tasks/stats`, and add a Runner/Model breakdown panel to the Observability page wired to the same SSE invalidation as the existing KPIs. Add the `model` Prometheus label so dashboards/alerts can slice by it too."
todos:
  - id: p1_runner_effective_model
    content: "Phase 1a-i: add Runner.EffectiveModel(req) to runner.Runner interface; implement on cursor + runnerfake; runner contract test"
    status: pending
  - id: p1_cycle_meta_model
    content: "Phase 1a-ii: persist cursor_model + cursor_model_effective alongside runner/runner_version/prompt_hash in TaskCycle.MetaJSON (worker.buildCycleMeta); thread req through process.go so the same Request feeds both buildCycleMeta and runner.Run"
    status: pending
  - id: p1_handler_cycle_meta_typed
    content: "Phase 1b: add typed cycle_meta projection (runner/runner_version/cursor_model/cursor_model_effective/prompt_hash) to handler_cycles_json so the SPA never has to parse free-form meta"
    status: pending
  - id: p1_web_cycle_types
    content: "Phase 1c: extend web/src/types/cycle.ts TaskCycle with typed cycle_meta and update parseTaskApi cycles parser + tests"
    status: pending
  - id: p2_stats_by_runner_model
    content: "Phase 2a: add RunnerStats (ByRunner / ByModel / ByRunnerModel + DurationP50/P95 succeeded-only) to pkgs/tasks/store/internal/stats.TaskStats; one new SQL pass keyed off cycle_meta.cursor_model_effective; no cardinality cap"
    status: pending
  - id: p2_handler_stats_wire
    content: "Phase 2b: extend GET /tasks/stats response with runner.{by_runner,by_model,by_runner_model,duration_p50_seconds_succeeded,duration_p95_seconds_succeeded}; pin in handler_http_list_stats_contract_test.go"
    status: pending
  - id: p2_web_stats_types
    content: "Phase 2c: extend web/src/types/task.ts TaskStatsResponse with the runner section; update parseTaskApi stats parser + tests"
    status: pending
  - id: p3_metrics_model_label
    content: "Phase 3: add NEW parallel series t2a_agent_runs_by_model_total{runner,model,terminal_status} + t2a_agent_run_duration_by_model_seconds{runner,model} alongside the existing unlabeled series (zero-break, D4); RunMetrics.RecordRun signature grows a model arg sourced from runner.EffectiveModel(req); update process.go + cleanup.go + e2e + recording rules"
    status: pending
  - id: p4_task_header_runtime
    content: "Phase 4a: TaskDetailHeader replaces the muted task-detail-agent-meta <p> with ONE combined cell-pill chip \"Cursor CLI · <model>\" rendered next to status/priority (D6); migrate test selectors to data-testid=task-detail-runtime"
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

## Decisions (locked)

These were resolved up-front so execution is straight-line. If a phase tempts you to revisit one of these, stop and re-ask — they affect more than one phase each.

| # | Decision | Choice | Implication |
|---|---|---|---|
| D1 | What `cycle_meta.cursor_model` and the Prometheus `model` label record | **Resolved at cycle start to the most accurate value: the model the runner will actually use.** Compute it via a new tiny capability on the runner (see Phase 1a) so the resolution lives in the adapter (`req.CursorModel` → fallback to `DefaultCursorModel` from app_settings) rather than being re-implemented in the worker. | Truthful audit. Eliminates the empty-string bucket. Diverges from `tasks.cursor_model` whenever the operator picked the default; we record the divergence by writing both keys (see Phase 1a). |
| D2 | How the "default" model renders | **Resolved to the concrete model name at cycle start; no `default` bucket exists.** Same value flows to the chip, the breakdown panel, and Prometheus. | The UI never has to special-case "default model" copy. Promql queries always have a model value. The frontmatter's defensive "render `default model`" copy applies only to **pre-feature cycles** where `cycle_meta.cursor_model_effective` is empty. |
| D3 | Which cycles feed the p50/p95 wall-clock per (runner, model) | **Succeeded only.** Failed/aborted runs are typically timeout-bounded and would skew p95 with noise unrelated to the model. | The breakdown panel labels its columns "p50 (succeeded)" / "p95 (succeeded)" so the operator never wonders. The Prometheus histogram keeps observing all terminal runs (existing behaviour) — the panel and the histogram answer different questions. |
| D4 | Adding the `model` label without breaking existing dashboards | **Parallel series, zero break.** Keep `t2a_agent_runs_total{runner,terminal_status}` and `t2a_agent_run_duration_seconds{runner}` byte-identical. Add NEW series `t2a_agent_runs_by_model_total{runner,model,terminal_status}` and `t2a_agent_run_duration_by_model_seconds{runner,model}`. Emit both from the same `RecordRun` adapter (one extra `Inc()` / `Observe()` per call — negligible). | No CHANGELOG break. No dashboard migration. The mild duplication is the cost of additivity. Recording rules in `deploy/prometheus/t2a-taskapi-rules.yaml` get the new series; existing rules untouched. |
| D5 | Where the new Runner/Model breakdown panel mounts | **Between `ObservabilityOverview` and `ObservabilityCycles`.** | Operator drill-down order: top KPIs → runner/model identity → per-phase heatmap. Matches existing visual rhythm. |
| D6 | Header chip placement | **One combined chip `Cursor CLI · opus`** rendered next to the existing status/priority pills. | Less visual weight than two chips, more registration than a muted line. Keeps the header height unchanged. |
| D7 | Cardinality cap on model values in stats + Prometheus | **No cap.** Today < 10 cursor models in practice; a future explosion is the future's problem and would surface in `t2a_agent_runs_by_model_total` cardinality immediately. | Simpler code path (no `__other__` bucket). Documented in `docs/OBSERVABILITY.md` so operators know to watch cardinality. |
| D8 | Delivery shape | **Per-phase commits on `main`, push after each, no PR.** Each phase is independently revertable; the order is fixed (1 → 2 → 3 → 4 → 5 → 6) because the SPA pieces (4–5) consume the wire shape produced by 1–2. | Smaller blast radius per commit. Phase 6 (docs) commits last so docs always describe shipped behaviour. See "Commit-and-push checklist" at the bottom. |

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
- Backfilling pre-feature cycles. Pre-feature rows render with empty `cursor_model_effective`; the SPA shows them as "default model". Their `tasks.runner` value (recoverable today) is correct; the model-at-the-time is not. The rollout log line tells operators the magnitude so they can decide on a one-shot script.

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
  Worker --> CycleMeta[buildCycleMeta:<br/>runner, runner_version,<br/>cursor_model + cursor_model_effective,<br/>prompt_hash]
  CycleMeta --> CyclesTable[(task_cycles.meta_json)]
  Worker --> Metrics[(t2a_agent_runs_total {runner, terminal_status} — UNCHANGED<br/>+ t2a_agent_runs_by_model_total {runner, model, terminal_status} — NEW)]
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

### 1a. Widen `buildCycleMeta` (records BOTH intent and effective; D1 + D2)

We need the **most accurate** model — what the runner actually executed against — without re-implementing fallback logic in the worker. Two coupled changes:

- File: [`pkgs/agents/runner/runner.go`](pkgs/agents/runner/runner.go)
  - Add a tiny capability to the `Runner` interface:
    ```go
    type Runner interface {
        Run(ctx context.Context, req Request) (Result, error)
        Name() string
        Version() string
        // EffectiveModel returns the concrete model the adapter would use
        // for `req` after applying its own defaults (e.g. cursor's
        // DefaultCursorModel fallback). Pure function, MUST NOT touch
        // network or the filesystem; called from the worker on the hot
        // path before each cycle starts.
        EffectiveModel(req Request) string
    }
    ```
  - Implement on `pkgs/agents/runner/cursor/cursor.go` using the existing fallback at L129-131 (`req.CursorModel` → `defaultCursorModel`). Implement on `pkgs/agents/runner/runnerfake/` returning whatever the script entry pinned. Update the runner contract test.

- File: [`pkgs/agents/worker/meta.go`](pkgs/agents/worker/meta.go)
  - New signature: `buildCycleMeta(r runner.Runner, task *domain.Task, req runner.Request, prompt string) []byte`.
  - The JSON object grows from 3 keys to 5:
    ```json
    {
      "runner": "cursor",
      "runner_version": "2.x.y",
      "cursor_model": "",                  // D1: intent (tasks.cursor_model verbatim)
      "cursor_model_effective": "opus",    // D1: what the runner will actually use
      "prompt_hash": "…"
    }
    ```
  - Both keys are always present (empty string when unknown). The SPA chip + breakdown panel + Prometheus label all read `cursor_model_effective`; `cursor_model` stays around for forensics ("did the operator pick this explicitly or fall through?").

- Update the call site in `process.go` (~L209-215) to construct the `runner.Request` first (it's already needed for `invokeRunner`), pass it to `buildCycleMeta`, and reuse the same `req` when calling `runner.Run`. Single source of truth for the resolved model on this attempt.

- Tests: extend `meta_test.go` (or add one) to assert the five-key shape; assert that `cursor_model_effective` falls back to the runner's default when `task.CursorModel == ""`; assert both keys serialise as `""` (not omitted) for tests with no model configured anywhere.

### 1b. Typed `cycle_meta` on the wire
- File: [`pkgs/tasks/handler/handler_cycles_json.go`](pkgs/tasks/handler/handler_cycles_json.go)
- Today the responses (`cycleResponse`, `cycleDetailResponse`, etc) carry `Meta json.RawMessage`. Add a sibling typed projection so the SPA never has to parse free-form JSON to render the chip:

```go
type cycleMetaProjection struct {
    Runner               string `json:"runner"`
    RunnerVersion        string `json:"runner_version"`
    CursorModel          string `json:"cursor_model"`            // intent
    CursorModelEffective string `json:"cursor_model_effective"`  // what actually ran
    PromptHash           string `json:"prompt_hash"`
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
  cursor_model: string;            // intent (operator's selection; "" === default)
  cursor_model_effective: string;  // what the runner actually used (the truth source for chips/breakdown)
  prompt_hash: string;
};

export type TaskCycle = {
  // ...existing fields
  meta: Record<string, unknown>;
  cycle_meta: CycleMeta;
};
```

- Update `web/src/api/parseTaskApi*.ts` (the cycles parser) to validate the five string fields with the existing `expectString` helper; missing → empty string (defensive, matches the backend's zero-value-on-parse-fail). Pre-feature cycles get all-empty strings; UI falls back to "default model" copy in that case.
- Tests: extend `parseTaskApi.cycles.test.ts` for both new fields; assert that pre-feature cycles (all empty) round-trip without throwing.

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
- Always seed `ByRunner["cursor"]`, `ByRunnerModel["cursor|"]` so the wire shape is stable on a fresh DB (matches the `allPhases` seeding pattern at L94). `ByModel` and the duration maps are intentionally empty `{}` on a fresh DB — there is no canonical "default" model bucket (D2: every cycle resolves to a concrete model name; the empty key only appears for pre-feature cycles).
- The aggregation key is `cursor_model_effective` extracted from `task_cycles.meta_json` (jsonb path `meta_json->>'cursor_model_effective'`), NOT `tasks.cursor_model`. The two diverge whenever the operator picked the default; the breakdown is supposed to make that divergence visible.
- **No cardinality cap** (D7). Distinct model values flow straight through to maps and Prometheus labels. Documented in `docs/OBSERVABILITY.md` so operators know to watch cardinality if a future runner explodes the model set.
- p50/p95 are computed over **succeeded cycles only** (D3): `WHERE c.status = 'succeeded' AND c.ended_at IS NOT NULL`. Failed/aborted cycles are bounded by the per-run timeout and would inject a spike at the cap value into p95 that has nothing to do with the model. Field names on the wire reflect this: `duration_p50_seconds_succeeded` / `duration_p95_seconds_succeeded`.

### 2b. Wire shape + handler
- File: [`pkgs/tasks/handler/handler_stats.go`](pkgs/tasks/handler/handler_stats.go) (or whichever owns the marshaller)
- New JSON section:

```json
{
  "runner": {
    "by_runner": { "cursor": { "succeeded": 12, "failed": 3, "aborted": 0, "running": 1 } },
    "by_model":  { "opus": {"succeeded": 8, "failed": 2, "aborted": 0, "running": 1}, "sonnet-4.5": {"succeeded": 4, "failed": 1} },
    "by_runner_model": { "cursor|opus": {...}, "cursor|sonnet-4.5": {...} },
    "duration_p50_seconds_succeeded": { "cursor|opus": 41.7, "cursor|sonnet-4.5": 12.3 },
    "duration_p95_seconds_succeeded": { "cursor|opus": 92.4, "cursor|sonnet-4.5": 18.0 }
  }
}
```

(Empty-string model keys appear ONLY for pre-feature cycles whose `cycle_meta.cursor_model_effective` was never recorded; the SPA renders those as "default model".)

- Pin `handler_http_list_stats_contract_test.go`: presence of all four sub-objects on a fresh DB (empty inner maps) and a populated case.

### 2c. Web types + parser
- File: [`web/src/types/task.ts`](web/src/types/task.ts)
- Extend `TaskStatsResponse` with a `runner: TaskStatsRunner` field; mirror the JSON shape one-to-one; document `cursor_model === ""` as "default model".
- Update `parseTaskApi.stats.test.ts` to cover the new shape (presence + numeric coercion).

---

## Phase 3 — Prometheus model series (parallel, zero-break)

**Touchpoints**: `pkgs/agents/worker/{metrics.go,process.go,cleanup.go,worker.go,meta.go}`, `internal/taskapi/agent_worker_metrics.go`, `pkgs/tasks/agentreconcile/agent_real_cursor_e2e_test.go`, `deploy/prometheus/t2a-taskapi-rules.yaml`.

Per **D4**: existing series stay byte-identical so no dashboard or alert breaks. Add new parallel series that carry the `model` label.

- File: [`pkgs/agents/worker/metrics.go`](pkgs/agents/worker/metrics.go)
- Change the seam (additive):

```go
type RunMetrics interface {
    // Unchanged signature, model is the NEW second arg. Existing callers
    // that pass an empty string for model see the same observation
    // pattern as before on the legacy series, plus a model="" sample on
    // the new series.
    RecordRun(runner, model, terminalStatus string, duration time.Duration)
}
```

- Update `(*Worker).recordRun` and every call site in `process.go` / `cleanup.go` to pass `state.effectiveModel` (a new field on `processState` populated at the same time as `state.startedAt`, sourced from `runner.EffectiveModel(req)` from Phase 1a). Cleanup-path calls (panic, shutdown abort, best-effort terminate) reuse the same field.
- File: [`internal/taskapi/agent_worker_metrics.go`](internal/taskapi/agent_worker_metrics.go) — register **two new** series alongside the existing ones:

```go
// Existing (UNCHANGED):
//   t2a_agent_runs_total{runner, terminal_status}
//   t2a_agent_run_duration_seconds{runner}
//
// New (added in this phase):
//   t2a_agent_runs_by_model_total{runner, model, terminal_status}
//   t2a_agent_run_duration_by_model_seconds{runner, model}
```

The adapter's `RecordRun` does both pairs of `Inc()` / `Observe()` per call (4 cheap writes; bounded). The legacy series becomes a `sum by (runner, terminal_status) (t2a_agent_runs_by_model_total)` of the new one — identical semantics, but we keep the literal series so existing scrapes / dashboards / alerts keep working with no migration.

- E2E: extend [`pkgs/tasks/agentreconcile/agent_real_cursor_e2e_test.go:665`](pkgs/tasks/agentreconcile/agent_real_cursor_e2e_test.go) to assert that BOTH the legacy and the new `_by_model_` series are present after a real run.
- Update [`deploy/prometheus/t2a-taskapi-rules.yaml`](deploy/prometheus/t2a-taskapi-rules.yaml): add recording rules + alert templates for the new series; existing rules untouched.

---

## Phase 4 — Per-task UI: surface runner/model on every attempt

**Touchpoints**: `web/src/tasks/components/task-detail/layout/TaskDetailHeader.tsx`, `web/src/tasks/components/task-detail/cycles/TaskCyclesPanel.tsx`, `web/src/observability/cyclesViewModel.ts`, `web/src/observability/index.ts`, `web/src/app/styles/app-task-detail.css`.

### 4a. Header runtime tag — one combined chip (D6)
- File: [`web/src/tasks/components/task-detail/layout/TaskDetailHeader.tsx`](web/src/tasks/components/task-detail/layout/TaskDetailHeader.tsx)
- Today the header renders a single muted `<p>` for runner + model (L54-59). Replace it with **one** combined `cell-pill`-styled chip rendered next to the existing status/priority pills inside `.task-detail-meta`:
  - Label: `Cursor CLI · opus` (or `Cursor CLI · default model` for the rare task with no model selected anywhere — should be near-zero in practice now that D2 resolves at cycle start, but the create-task form may still set `cursor_model: ""`).
  - Class: `cell-pill cell-pill--runtime` (new variant; muted-but-readable border + neutral surface so it doesn't compete with status/priority colour).
  - Source: `task.cursor_model` for the header (the task's intent — it's not yet running). The cycle history below will show the effective per-attempt value via Phase 4b.
- Remove the muted `<p class="task-detail-agent-meta">`. Migrate any test selectors (search the codebase for `task-detail-agent-meta`) to target the new chip via a `data-testid="task-detail-runtime"` attribute on the chip.
- Keep header height byte-identical: the chip lives in the same row as status/priority, no new line added.

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
    8. p50 wall-clock — **succeeded only** (D3); column header reads "p50 (succeeded)".
    9. p95 wall-clock — **succeeded only** (D3); column header reads "p95 (succeeded)". Cells render `—` when the model has no succeeded cycles yet so the operator never sees a misleading "0s".
  - Use `formatDurationSeconds` from `@/observability` for both percentile cells.
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

  where `N` is `SELECT count(*) FROM task_cycles WHERE meta_json::jsonb -> 'cursor_model_effective' IS NULL` (Postgres) / equivalent JSON1 query (SQLite). Gives operators a single number to decide if they want to run a one-shot script later.
- **Prometheus dashboards / alerts**: per **D4** the existing `t2a_agent_runs_total{runner,terminal_status}` and `t2a_agent_run_duration_seconds{runner}` series are byte-identical after this change; new `_by_model_` series are additive. Zero dashboard migration required; new Grafana panels can be added incrementally.
- **No feature flag**: every change is read-side or additive metadata. Per-phase commits (D8) make rollback trivial — `git revert` the offending commit; no later commit depends on a reverted earlier commit's wire shape (Phase 4–5 tolerate empty `cycle_meta` because the parser defaults to `""`).

---

## Commit-and-push checklist (D8 — per-phase commits, no PR)

Each phase becomes one commit on `main`, pushed immediately after the per-phase tests pass. Order is fixed (a later phase reads a shape produced by an earlier one).

| Order | Commit subject (conventional commits) | Phase coverage | Pre-push verification |
|------|---|---|---|
| 1 | `feat(runner): add EffectiveModel + record cursor_model + cursor_model_effective on cycle meta` | 1a-i, 1a-ii | `go test ./pkgs/agents/... ./pkgs/tasks/agentreconcile/... -count=1` |
| 2 | `feat(api): expose typed cycle_meta on /tasks/{id}/cycles[/{cycleId}]` | 1b, 1c | `go test ./pkgs/tasks/handler/... -count=1`; `cd web && npm test -- --run parseTaskApi` |
| 3 | `feat(stats): aggregate cycles by runner & effective model on /tasks/stats` | 2a, 2b, 2c | `go test ./pkgs/tasks/store/... ./pkgs/tasks/handler/... -count=1`; `cd web && npm test -- --run parseTaskApi` |
| 4 | `feat(metrics): add t2a_agent_runs_by_model_total + duration histogram, parallel to existing series` | 3 | `go test ./pkgs/agents/worker/... ./internal/taskapi/... ./pkgs/tasks/agentreconcile/... -count=1`; manually scrape `/metrics` from a smoke run and confirm both old & new series present |
| 5 | `feat(web): show runner & effective model on TaskDetailHeader + every cycle row` | 4a, 4b, 4c | `cd web && npm test -- --run && npm run lint && npm run build` |
| 6 | `feat(web): add Runner / Model breakdown panel to Observability page` | 5a, 5b, 5c, 5d | `cd web && npm test -- --run && npm run lint && npm run build` |
| 7 | `docs: per-task runner & model attribution + observability slice-and-dice` | 6 | `.\scripts\check.ps1` (full bar) — last commit, so docs describe the shipped state |

Per AGENTS.md "Commands to run before you finish": `git status` → `git add` (scoped) → `git commit` → `git push origin main` after **every** commit above. Do not batch commits before pushing; each commit is independently revertable in production.

If any pre-push verification fails: fix forward in the same commit (amend allowed only if not yet pushed; otherwise add a follow-up commit `fix(...): ...` and push that).

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
