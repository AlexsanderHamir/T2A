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
type RunnerModelRow = {
  runner: string;
  model: string;
  label: string;
  bucket: TaskStatsRunnerBucket;
  isRunnerTotal: boolean;
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
 * Reads `stats.runner` verbatim from `GET /tasks/stats`. Three aggregations
 * are available on the wire; this panel uses `by_runner_model` for the
 * inner rows and `by_runner` for the "all models" summary row so the
 * operator can always see the runner's totals even when one model
 * dominates the inner rows.
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
      className={
        row.isRunnerTotal
          ? "obs-runner-row obs-runner-row--total"
          : "obs-runner-row"
      }
      data-testid={`obs-runner-row-${rowKey(row)}`}
    >
      <th scope="row" className="obs-runner-cell--label">
        <span
          className={
            row.isRunnerTotal
              ? "cell-pill cell-pill--runtime obs-runner-pill--total"
              : "cell-pill cell-pill--runtime"
          }
        >
          {row.label}
        </span>
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
  return `runner|${row.runner}|${row.model}`;
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
  const { by_runner, by_runner_model } = stats.runner;
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
    out.push(...modelRows);
  }
  return out;
}

function formatRunnerModelRowLabel(runner: string, model: string): string {
  const r = runnerLabel(runner);
  const m = model.trim();
  if (!m) return `${r} · default model`;
  return `${r} · ${m}`;
}
