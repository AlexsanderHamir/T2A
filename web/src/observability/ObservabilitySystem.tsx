import type { SystemHealthResponse } from "@/types";
import { KpiCard } from "./KpiCard";
import { kpiState } from "./kpiState";
import { StackedBar } from "./StackedBar";
import {
  REQUEST_CLASS_DISPLAY_ORDER,
  TERMINAL_STATUS_DISPLAY_ORDER,
  formatDurationSeconds,
  formatLatencySeconds,
  formatNumber,
  formatRatio,
  requestClassFillClass,
  requestClassLabel,
  summarize,
  terminalStatusFillClass,
  terminalStatusLabel,
} from "./systemHealthViewModel";

type Props = {
  health: SystemHealthResponse | null | undefined;
  loading: boolean;
};

/**
 * System health pane (Stage 3). Renders the operator-facing snapshot
 * surfaced by `GET /system/health`. The pane intentionally mirrors the
 * `ObservabilityCycles` layout — a header with a one-line summary, a
 * KPI grid for headline numbers, two stacked bars for distributions,
 * and a build-info footer — so a user scrolling the page does not
 * have to relearn navigation between sections.
 *
 * The pane handles three states (matching the rest of the page):
 *
 *   1. `loading` (no payload yet) → skeleton KPIs and a "Loading…"
 *      caption. Distribution bars are hidden so we don't render a
 *      flicker of empty chrome.
 *   2. `unavailable` (request errored or backend is too old to expose
 *      the route) → KPIs render "—" and a "snapshot unavailable"
 *      caption. The caller stays mounted so the next poll can recover.
 *   3. `ready` → real numbers.
 */
export function ObservabilitySystem({ health, loading }: Props) {
  const hasHealth = health != null;
  const summary = summarize(health, loading);

  if (!hasHealth) {
    return (
      <section
        className={`obs-system obs-system--${summary.level}`}
        aria-label="System health"
      >
        <header className="obs-system-head">
          <h3 className="obs-system-title">System health</h3>
          <p className="obs-system-subtitle">{summary.caption}</p>
        </header>
        <SystemHealthKpiGrid health={undefined} loading={loading} />
      </section>
    );
  }

  const requestSegments = REQUEST_CLASS_DISPLAY_ORDER.map((c) => ({
    id: c,
    label: requestClassLabel(c),
    value: health.http.requests_by_class[c] ?? 0,
    fillClass: requestClassFillClass(c),
  }));
  const totalRequests = requestSegments.reduce((acc, s) => acc + s.value, 0);

  const agentSegments = TERMINAL_STATUS_DISPLAY_ORDER.map((s) => {
    const v = health.agent.runs_by_terminal_status[s] ?? 0;
    return {
      id: s,
      label: terminalStatusLabel(s),
      value: v,
      fillClass: terminalStatusFillClass(s),
    };
  });
  const totalRuns = agentSegments.reduce((acc, s) => acc + s.value, 0);

  return (
    <section
      className={`obs-system obs-system--${summary.level}`}
      aria-label="System health"
    >
      <header className="obs-system-head">
        <h3 className="obs-system-title">System health</h3>
        <p className="obs-system-subtitle">{summary.caption}</p>
      </header>
      <SystemHealthKpiGrid health={health} loading={loading} />
      <div className="obs-system-grid">
        <StackedBar
          title="HTTP responses by class"
          segments={requestSegments}
          caption={
            totalRequests === 0
              ? "No requests recorded yet"
              : `${formatNumber(totalRequests)} response${totalRequests === 1 ? "" : "s"}`
          }
        />
        <StackedBar
          title="Agent runs by outcome"
          segments={agentSegments}
          caption={
            totalRuns === 0
              ? "No agent runs recorded yet"
              : `${formatNumber(totalRuns)} run${totalRuns === 1 ? "" : "s"}`
          }
        />
      </div>
      <SystemHealthBuildFooter health={health} />
    </section>
  );
}

function SystemHealthKpiGrid({
  health,
  loading,
}: {
  health: SystemHealthResponse | undefined;
  loading: boolean;
}) {
  const has = health != null;
  // KpiCard renders numbers; we pre-format and pass via meta where the
  // unit matters more than the digit (latencies, ratios). The headline
  // stays a raw integer so the eye lands on the count.
  const inFlight = kpiState(health?.http.in_flight, loading, has);
  const totalReq = kpiState(health?.http.requests_total, loading, has);
  const subs = kpiState(health?.sse.subscribers, loading, has);
  const dropped = kpiState(health?.sse.dropped_frames_total, loading, has);
  const dbInUse = kpiState(health?.db_pool.in_use_connections, loading, has);
  const queueDepth = kpiState(health?.agent.queue_depth, loading, has);

  return (
    <section className="obs-kpi-grid" aria-label="System headline counters">
      <KpiCard
        label="HTTP in-flight"
        state={inFlight}
        meta={
          health
            ? `${formatLatencySeconds(health.http.duration_seconds.p50)} p50 · ${formatLatencySeconds(
                health.http.duration_seconds.p95,
              )} p95`
            : "request latency"
        }
        tone="info"
        testId="obs-system-kpi-in-flight"
      />
      <KpiCard
        label="HTTP requests"
        state={totalReq}
        meta={
          health
            ? `${formatNumber(health.http.duration_seconds.count)} timed`
            : "since process start"
        }
        tone="neutral"
        testId="obs-system-kpi-requests"
      />
      <KpiCard
        label="SSE subscribers"
        state={subs}
        meta={
          health
            ? `${formatNumber(health.sse.dropped_frames_total)} dropped frame${
                health.sse.dropped_frames_total === 1 ? "" : "s"
              }`
            : "live event listeners"
        }
        tone={
          (health?.sse.dropped_frames_total ?? 0) > 0 ? "warning" : "info"
        }
        testId="obs-system-kpi-sse-subs"
      />
      <KpiCard
        label="Dropped SSE frames"
        state={dropped}
        meta="slow-client backpressure"
        tone={(health?.sse.dropped_frames_total ?? 0) > 0 ? "danger" : "positive"}
        testId="obs-system-kpi-sse-dropped"
      />
      <KpiCard
        label="DB connections"
        state={dbInUse}
        meta={
          health
            ? `${formatRatio(
                health.db_pool.open_connections,
                health.db_pool.max_open_connections,
              )} open · ${formatNumber(health.db_pool.idle_connections)} idle`
            : "pool utilization"
        }
        tone={
          health &&
          health.db_pool.max_open_connections > 0 &&
          health.db_pool.in_use_connections >=
            health.db_pool.max_open_connections
            ? "danger"
            : "info"
        }
        testId="obs-system-kpi-db-in-use"
      />
      <KpiCard
        label="Agent queue"
        state={queueDepth}
        meta={
          health
            ? `${formatRatio(
                health.agent.queue_depth,
                health.agent.queue_capacity,
              )} pending · ${formatNumber(health.agent.runs_total)} run${
                health.agent.runs_total === 1 ? "" : "s"
              }`
            : "agent worker depth"
        }
        tone={
          health &&
          health.agent.queue_capacity > 0 &&
          health.agent.queue_depth >= health.agent.queue_capacity
            ? "warning"
            : "info"
        }
        testId="obs-system-kpi-agent-queue"
      />
    </section>
  );
}

function SystemHealthBuildFooter({ health }: { health: SystemHealthResponse }) {
  // Use the server's clock for "as of" rather than `Date.now()` so a
  // skewed laptop clock doesn't whisper "data is stale" when it isn't.
  const asOf = health.now;
  return (
    <footer className="obs-system-foot" aria-label="Build and uptime">
      <dl className="obs-system-meta">
        <div className="obs-system-meta-row">
          <dt>Version</dt>
          <dd>
            <code>{health.build.version}</code>
          </dd>
        </div>
        <div className="obs-system-meta-row">
          <dt>Revision</dt>
          <dd>
            <code>{health.build.revision}</code>
          </dd>
        </div>
        <div className="obs-system-meta-row">
          <dt>Go runtime</dt>
          <dd>
            <code>{health.build.go_version}</code>
          </dd>
        </div>
        <div className="obs-system-meta-row">
          <dt>Uptime</dt>
          <dd>{formatDurationSeconds(health.uptime_seconds)}</dd>
        </div>
        <div className="obs-system-meta-row">
          <dt>Snapshot</dt>
          <dd>
            <time dateTime={asOf}>{asOf}</time>
          </dd>
        </div>
      </dl>
    </footer>
  );
}
