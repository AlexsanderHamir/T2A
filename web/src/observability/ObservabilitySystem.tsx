import type { SystemHealthResponse } from "@/types";
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
 * surfaced by `GET /system/health`. The command center owns the most
 * actionable runtime counters; this pane keeps lower-level capacity,
 * traffic, distribution, and build detail in one supporting section.
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
      <SystemRuntimeGrid health={undefined} loading={loading} />
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
      <SystemRuntimeGrid health={health} loading={loading} />
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

function SystemRuntimeGrid({
  health,
  loading,
}: {
  health: SystemHealthResponse | undefined;
  loading: boolean;
}) {
  return (
    <section className="obs-system-metrics" aria-label="Runtime detail">
      <RuntimeMetric
        label="HTTP latency"
        value={
          health
            ? `${formatLatencySeconds(health.http.duration_seconds.p50)} p50`
            : undefined
        }
        meta={
          health
            ? `${formatLatencySeconds(health.http.duration_seconds.p95)} p95 · ${formatNumber(
                health.http.in_flight,
              )} in flight`
            : "request latency"
        }
        loading={loading}
        available={health != null}
        testId="obs-system-runtime-latency"
      />
      <RuntimeMetric
        label="HTTP traffic"
        value={health ? formatNumber(health.http.requests_total) : undefined}
        meta={
          health
            ? `${formatNumber(health.http.duration_seconds.count)} timed requests`
            : "since process start"
        }
        loading={loading}
        available={health != null}
        testId="obs-system-runtime-traffic"
      />
      <RuntimeMetric
        label="DB pool"
        value={
          health
            ? formatRatio(
                health.db_pool.in_use_connections,
                health.db_pool.max_open_connections,
              )
            : undefined
        }
        meta={
          health
            ? `${formatRatio(
                health.db_pool.open_connections,
                health.db_pool.max_open_connections,
              )} open · ${formatNumber(health.db_pool.idle_connections)} idle`
            : "pool utilization"
        }
        loading={loading}
        available={health != null}
        testId="obs-system-runtime-db"
      />
      <RuntimeMetric
        label="SSE stream"
        value={health ? formatNumber(health.sse.subscribers) : undefined}
        meta={
          health
            ? `${formatNumber(health.sse.dropped_frames_total)} dropped frame${
                health.sse.dropped_frames_total === 1 ? "" : "s"
              }`
            : "live event listeners"
        }
        loading={loading}
        available={health != null}
        testId="obs-system-runtime-sse"
      />
      <RuntimeMetric
        label="Agent runs"
        value={health ? formatNumber(health.agent.runs_total) : undefined}
        meta={
          health
            ? `${formatNumber(health.agent.runs_by_terminal_status.failed ?? 0)} failed · ${formatNumber(
                health.agent.runs_by_terminal_status.aborted ?? 0,
              )} aborted`
            : "terminal outcomes"
        }
        loading={loading}
        available={health != null}
        testId="obs-system-runtime-agent"
      />
      <RuntimeMetric
        label="Queue capacity"
        value={
          health ? formatRatio(health.agent.queue_depth, health.agent.queue_capacity) : undefined
        }
        meta={health?.agent.paused ? "agent paused" : "pending worker jobs"}
        loading={loading}
        available={health != null}
        testId="obs-system-runtime-queue"
      />
    </section>
  );
}

function RuntimeMetric({
  label,
  value,
  meta,
  loading,
  available,
  testId,
}: {
  label: string;
  value: string | undefined;
  meta: string;
  loading: boolean;
  available: boolean;
  testId: string;
}) {
  return (
    <article
      className="obs-system-metric"
      aria-busy={loading && !available}
      data-testid={testId}
    >
      <p className="obs-system-metric-label">{label}</p>
      <p className="obs-system-metric-value">
        {loading && !available ? "Loading" : available ? value : "—"}
      </p>
      <p className="obs-system-metric-meta">{meta}</p>
    </article>
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
