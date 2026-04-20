import type { CycleStatus } from "@/types/cycle";
import type {
  TaskStatsResponse,
  TaskStatsRunnerBucket,
} from "@/types/task";
import { formatDurationSeconds } from "./systemHealthViewModel";
import { runnerLabel } from "./cyclesViewModel";

type Props = {
  stats: TaskStatsResponse | null | undefined;
  loading: boolean;
};

/**
 * Flattened row for the breakdown table. One per (runner, model) pair
 * plus one "runner totals" row per runner so the operator can compare
 * a single model to the runner it lives under.
 */
/**
 * Three flavors of rows live in the same flat list the table renders
 * so the sort order ("busiest runner first, then its busiest model,
 * then resolved-model sub-rows") falls out of a single
 * [].push()-then-render pipeline:
 *
 *   - `isRunnerTotal`  → one per runner, aggregates every model under
 *                        that runner (sourced from `by_runner`).
 *   - neither flag     → one per (runner, effective model) pair
 *                        (sourced from `by_runner_model`).
 *   - `isResolved`     → one per (runner, effective model, resolved
 *                        model) triple (sourced from
 *                        `by_runner_model_resolved`). Only emitted
 *                        when the resolved model is observed and
 *                        differs meaningfully from the effective
 *                        selection (e.g. operator picked "auto" →
 *                        cursor-agent routed to "Claude 4 Sonnet").
 *                        Rendered indented under its parent model
 *                        row so the operator can read it as a
 *                        breakdown, not a peer.
 */
type RunnerModelRow = {
  runner: string;
  model: string;
  resolved?: string;
  label: string;
  bucket: TaskStatsRunnerBucket;
  isRunnerTotal: boolean;
  isResolved?: boolean;
};

const TABLE_COLUMNS = [
  "Runner · model",
  "Total",
  "Succeeded",
  "Failed",
  "Aborted",
  "Running",
  "Success rate",
  "p50 (succeeded)",
  "p95 (succeeded)",
] as const;

/**
 * Per-runner / per-model breakdown panel. Mounts between the Overview
 * KPIs and the Cycles & phases heatmap on the Observability page (plan
 * decision D5) so the operator's drill-down order reads:
 *   top-level KPIs → runner/model attribution → per-phase failure map.
 *
 * Reads `stats.runner` verbatim from `GET /tasks/stats`. Four
 * aggregations are available on the wire; this panel uses:
 *   - `by_runner` for the "all models" summary row per runner,
 *   - `by_runner_model` for the inner (runner, effective model) rows,
 *   - `by_runner_model_resolved` for the optional "↳ Auto → Claude 4
 *     Sonnet" sub-row that appears under a (runner, model) row when
 *     the adapter observed what concrete model the CLI actually
 *     routed to (today: cursor-agent's `system.init.model` event).
 *     The sub-row is suppressed when the resolved model equals the
 *     effective model, since that row would just duplicate its
 *     parent; it's only shown when there's a real delta worth
 *     surfacing to the operator (the `auto` case is the canonical
 *     one).
 *
 * Percentile columns only observe succeeded cycles (plan decision D3);
 * the column headers make that explicit so operators don't wonder why
 * p95 looks lower than their alerting rule threshold. Cells render "—"
 * when a model has no succeeded cycles yet — rendering "0s" there would
 * mislead more than it informs.
 */
export function ObservabilityRunnerBreakdown({ stats, loading }: Props) {
  if (!stats) {
    return (
      <section
        className="obs-runner-breakdown"
        aria-label="Runner and model breakdown"
      >
        <header className="obs-runner-breakdown-head">
          <h3 className="obs-runner-breakdown-title">Runner &amp; model</h3>
          <p className="obs-runner-breakdown-subtitle">
            {loading
              ? "Loading runner attribution…"
              : "Runner attribution unavailable."}
          </p>
        </header>
      </section>
    );
  }

  const rows = buildRows(stats);
  const runnerCount = Object.keys(stats.runner.by_runner).length;
  const modelCount = Object.keys(stats.runner.by_model).length;
  const totalCycles = rows.reduce(
    (acc, r) => (r.isRunnerTotal ? acc + bucketTotal(r.bucket) : acc),
    0,
  );

  if (rows.length === 0) {
    return (
      <section
        className="obs-runner-breakdown"
        aria-label="Runner and model breakdown"
      >
        <header className="obs-runner-breakdown-head">
          <h3 className="obs-runner-breakdown-title">Runner &amp; model</h3>
          <p className="obs-runner-breakdown-subtitle">
            No runner attribution yet — start a task to populate this view.
          </p>
        </header>
      </section>
    );
  }

  return (
    <section
      className="obs-runner-breakdown"
      aria-label="Runner and model breakdown"
    >
      <header className="obs-runner-breakdown-head">
        <h3 className="obs-runner-breakdown-title">Runner &amp; model</h3>
        <p className="obs-runner-breakdown-subtitle">
          {runnerCount} {runnerCount === 1 ? "runner" : "runners"} ·{" "}
          {modelCount} {modelCount === 1 ? "model" : "models"} · {totalCycles}{" "}
          total {totalCycles === 1 ? "cycle" : "cycles"}
        </p>
      </header>
      <div className="obs-runner-breakdown-tablewrap">
        <table
          className="obs-runner-breakdown-table"
          data-testid="obs-runner-breakdown-table"
        >
          <thead>
            <tr>
              {TABLE_COLUMNS.map((c) => (
                <th key={c} scope="col">
                  {c}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <RunnerRow key={rowKey(row)} row={row} />
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function RunnerRow({ row }: { row: RunnerModelRow }) {
  const bucket = row.bucket;
  const total = bucketTotal(bucket);
  const succeeded = bucket.by_status.succeeded ?? 0;
  const failed = bucket.by_status.failed ?? 0;
  const aborted = bucket.by_status.aborted ?? 0;
  const running = bucket.by_status.running ?? 0;
  const rate = successRate(bucket);
  return (
    <tr
      className={rowClassName(row)}
      data-testid={`obs-runner-row-${rowKey(row)}`}
    >
      <th scope="row" className="obs-runner-cell--label">
        <span className={pillClassName(row)}>{row.label}</span>
      </th>
      <td className="obs-runner-cell--num">{total}</td>
      <td className="obs-runner-cell--num obs-runner-cell--succeeded">
        {succeeded}
      </td>
      <td className="obs-runner-cell--num obs-runner-cell--failed">
        {failed}
      </td>
      <td className="obs-runner-cell--num obs-runner-cell--aborted">
        {aborted}
      </td>
      <td className="obs-runner-cell--num">{running}</td>
      <td
        className={`obs-runner-cell--num obs-runner-cell--rate ${successRateClass(rate)}`}
      >
        {rate === null ? "—" : `${Math.round(rate * 100)}%`}
      </td>
      <td className="obs-runner-cell--num">
        {succeeded > 0
          ? formatDurationSeconds(bucket.duration_p50_succeeded_seconds)
          : "—"}
      </td>
      <td className="obs-runner-cell--num">
        {succeeded > 0
          ? formatDurationSeconds(bucket.duration_p95_succeeded_seconds)
          : "—"}
      </td>
    </tr>
  );
}

function rowKey(row: RunnerModelRow): string {
  if (row.isRunnerTotal) {
    return `runner|${row.runner}|__total__`;
  }
  if (row.isResolved) {
    return `runner|${row.runner}|${row.model}|${row.resolved ?? ""}`;
  }
  return `runner|${row.runner}|${row.model}`;
}

function rowClassName(row: RunnerModelRow): string {
  if (row.isRunnerTotal) return "obs-runner-row obs-runner-row--total";
  if (row.isResolved) return "obs-runner-row obs-runner-row--resolved";
  return "obs-runner-row";
}

function pillClassName(row: RunnerModelRow): string {
  if (row.isRunnerTotal) {
    return "cell-pill cell-pill--runtime obs-runner-pill--total";
  }
  if (row.isResolved) {
    return "cell-pill cell-pill--runtime obs-runner-pill--resolved";
  }
  return "cell-pill cell-pill--runtime";
}

function bucketTotal(b: TaskStatsRunnerBucket): number {
  let acc = 0;
  for (const s of ["running", "succeeded", "failed", "aborted"] as CycleStatus[]) {
    acc += b.by_status[s] ?? 0;
  }
  return acc;
}

/**
 * Success rate counted against the terminal denominator only — running
 * cycles haven't voted yet, so including them would artificially depress
 * a busy runner's score. Returns `null` when no terminal cycles have
 * been recorded so the cell renders "—" rather than a fake 0%.
 */
function successRate(b: TaskStatsRunnerBucket): number | null {
  const succeeded = b.by_status.succeeded ?? 0;
  const failed = b.by_status.failed ?? 0;
  const aborted = b.by_status.aborted ?? 0;
  const terminal = succeeded + failed + aborted;
  if (terminal === 0) return null;
  return succeeded / terminal;
}

function successRateClass(rate: number | null): string {
  if (rate === null) return "obs-runner-cell--rate-unknown";
  if (rate >= 0.9) return "obs-runner-cell--rate-high";
  if (rate >= 0.6) return "obs-runner-cell--rate-mid";
  return "obs-runner-cell--rate-low";
}

/**
 * Builds the flat row list the table renders. One "runner total" row
 * per runner followed by its per-model rows, ordered by total cycles
 * descending so the busiest runner/model lands at the top.
 *
 * Pre-feature cycles whose meta didn't yet carry the runner/model keys
 * are bucketed server-side under the empty string; we render them as
 * "unknown runner" / "default model" via runnerLabel() + the empty-model
 * display copy so the totals in the panel always reconcile with
 * cycles.by_status on the Overview.
 */
function buildRows(stats: TaskStatsResponse): RunnerModelRow[] {
  const { by_runner, by_runner_model, by_runner_model_resolved } = stats.runner;
  const runnerNames = Object.keys(by_runner).sort((a, b) => {
    const ta = bucketTotal(by_runner[a]);
    const tb = bucketTotal(by_runner[b]);
    if (ta !== tb) return tb - ta;
    return a.localeCompare(b);
  });

  const out: RunnerModelRow[] = [];
  for (const runner of runnerNames) {
    const runnerBucket = by_runner[runner];
    out.push({
      runner,
      model: "",
      label: `${runnerLabel(runner)} · all models`,
      bucket: runnerBucket,
      isRunnerTotal: true,
    });

    const modelRows: RunnerModelRow[] = [];
    const prefix = `${runner}|`;
    for (const key of Object.keys(by_runner_model)) {
      if (!key.startsWith(prefix)) continue;
      const model = key.slice(prefix.length);
      modelRows.push({
        runner,
        model,
        label: formatRunnerModelRowLabel(runner, model),
        bucket: by_runner_model[key],
        isRunnerTotal: false,
      });
    }
    modelRows.sort((a, b) => {
      const ta = bucketTotal(a.bucket);
      const tb = bucketTotal(b.bucket);
      if (ta !== tb) return tb - ta;
      return a.model.localeCompare(b.model);
    });

    for (const modelRow of modelRows) {
      out.push(modelRow);
      // Interleave resolved sub-rows directly under their parent
      // (runner, effective model) row so the table reads like a
      // drill-down: runner → model → "… → actually routed to X".
      const resolvedRows = collectResolvedRows(
        runner,
        modelRow.model,
        by_runner_model_resolved,
      );
      out.push(...resolvedRows);
    }
  }
  return out;
}

/**
 * Collect resolved-model sub-rows for a given (runner, effective
 * model) pair. The resolved aggregation's keys are
 * `runner|effective|resolved`, so we match by the prefix
 * `runner|effective|`. We intentionally do NOT emit a sub-row when
 * the resolved model equals the effective model: that's the boring
 * case (operator picked a concrete model and got that same model
 * back) and the extra row would just be visual noise. The interesting
 * case — and the whole reason the resolved aggregation exists — is
 * when the operator picked `auto` (empty effective) and the adapter
 * reports a concrete resolved model like "Claude 4 Sonnet".
 */
function collectResolvedRows(
  runner: string,
  model: string,
  byResolved: Record<string, TaskStatsRunnerBucket>,
): RunnerModelRow[] {
  const prefix = `${runner}|${model}|`;
  const rows: RunnerModelRow[] = [];
  for (const key of Object.keys(byResolved)) {
    if (!key.startsWith(prefix)) continue;
    const resolved = key.slice(prefix.length);
    const resolvedTrim = resolved.trim();
    if (!resolvedTrim) continue;
    if (resolvedTrim === model.trim()) continue;
    rows.push({
      runner,
      model,
      resolved,
      label: formatResolvedRowLabel(runner, model, resolved),
      bucket: byResolved[key],
      isRunnerTotal: false,
      isResolved: true,
    });
  }
  rows.sort((a, b) => {
    const ta = bucketTotal(a.bucket);
    const tb = bucketTotal(b.bucket);
    if (ta !== tb) return tb - ta;
    return (a.resolved ?? "").localeCompare(b.resolved ?? "");
  });
  return rows;
}

function formatRunnerModelRowLabel(runner: string, model: string): string {
  const r = runnerLabel(runner);
  const m = model.trim();
  if (!m) return `${r} · default model`;
  return `${r} · ${m}`;
}

/**
 * Label for a resolved-model sub-row. Reads as "↳ Auto → Claude 4
 * Sonnet" when the operator picked auto (empty effective model), and
 * as "↳ opus-4 → claude-4-sonnet" when the CLI re-routed a concrete
 * selection (rare today, but the schema supports it so the UI stays
 * honest if a future adapter exposes it). The arrow prefix is the
 * visual cue that this row is a child of the row above it; indentation
 * styling lives in CSS.
 */
function formatResolvedRowLabel(
  _runner: string,
  model: string,
  resolved: string,
): string {
  const m = model.trim();
  const r = resolved.trim();
  const intent = m ? m : "Auto";
  return `↳ ${intent} → ${r}`;
}
