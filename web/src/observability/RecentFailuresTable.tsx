import { Link } from "react-router-dom";
import type { TaskStatsRecentFailure } from "@/types/task";

type Props = {
  failures: TaskStatsRecentFailure[];
};

/**
 * Recent cycle failures rendered as a compact table. Each row deep-
 * links to the originating audit event (`/tasks/{task_id}/events/{seq}`)
 * so the operator is one click away from the full payload, response
 * thread, and follow-up actions.
 *
 * Empty state is intentional: an empty array is the steady state on a
 * healthy database, so we render a friendly note rather than collapse
 * the section to zero height.
 */
export function RecentFailuresTable({ failures }: Props) {
  return (
    <section className="obs-failures" aria-label="Recent cycle failures">
      <header className="obs-failures-head">
        <h3 className="obs-failures-title">Recent failures</h3>
        <p className="obs-failures-caption">
          {failures.length === 0
            ? "No recent cycle failures — the agent worker is clean."
            : `Last ${failures.length} cycle ${failures.length === 1 ? "failure" : "failures"} (newest first)`}
        </p>
      </header>
      {failures.length === 0 ? null : (
        <div className="obs-failures-tablewrap" role="region" aria-label="Recent failures">
          <table className="obs-failures-table">
            <thead>
              <tr>
                <th scope="col">When</th>
                <th scope="col">Task</th>
                <th scope="col">Attempt</th>
                <th scope="col">Status</th>
                <th scope="col">Reason</th>
              </tr>
            </thead>
            <tbody>
              {failures.map((f) => (
                <FailureRow key={`${f.task_id}-${f.event_seq}`} failure={f} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function FailureRow({ failure }: { failure: TaskStatsRecentFailure }) {
  const eventHref = `/tasks/${failure.task_id}/events/${failure.event_seq}`;
  const taskHref = `/tasks/${failure.task_id}`;
  const statusClass =
    failure.status === "aborted"
      ? "cell-pill--status-blocked"
      : "cell-pill--status-failed";
  return (
    <tr data-testid={`obs-failure-row-${failure.task_id}-${failure.event_seq}`}>
      <td>
        <Link to={eventHref} className="obs-failures-link">
          <time dateTime={failure.at}>{formatTimestamp(failure.at)}</time>
        </Link>
      </td>
      <td>
        <Link to={taskHref} className="obs-failures-link obs-failures-link--mono">
          {shortId(failure.task_id)}
        </Link>
      </td>
      <td>#{failure.attempt_seq}</td>
      <td>
        <span className={`obs-failures-pill ${statusClass}`}>
          {failure.status}
        </span>
      </td>
      <td className="obs-failures-reason">
        {failure.reason ? failure.reason : <em className="obs-failures-muted">(no reason recorded)</em>}
      </td>
    </tr>
  );
}

/**
 * Renders an ISO timestamp as a locale-aware short form. Falls back to
 * the raw string if Date parsing fails — robust against future server
 * formats and dev SSE replay payloads.
 */
function formatTimestamp(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

/**
 * Truncates a UUID-style id to the first 8 chars so the table column
 * stays narrow without losing identification (the whole id is still
 * available via the deep link's title and the URL bar after click).
 */
function shortId(id: string): string {
  if (id.length <= 10) return id;
  return `${id.slice(0, 8)}…`;
}
