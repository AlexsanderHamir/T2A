import { Link } from "react-router-dom";
import type { TaskStatsRecentFailure } from "@/types/task";

/** Table cell preview — full text on hover via `title` (no expand control). */
const FAILURE_REASON_MAX_CHARS = 160;

type Props = {
  failures: TaskStatsRecentFailure[];
};

/**
 * Shared table body for cycle_failed rows (Observability snapshot and the
 * full failures list page). Deep-links match RecentFailuresTable.
 */
export function CycleFailuresTable({ failures }: Props) {
  return (
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
        <FailureReasonCell reason={failure.reason} />
      </td>
    </tr>
  );
}

function truncateFailureReasonPreview(text: string, maxLen: number): string {
  if (text.length <= maxLen) return text;
  const slice = text.slice(0, maxLen);
  const lastSpace = slice.lastIndexOf(" ");
  if (lastSpace > 0 && lastSpace >= maxLen * 0.45) {
    return `${slice.slice(0, lastSpace)}…`;
  }
  return `${slice.trimEnd()}…`;
}

function FailureReasonCell({ reason }: { reason: string }) {
  const trimmed = reason.trim();
  if (!trimmed) {
    return <em className="obs-failures-muted">(no reason recorded)</em>;
  }

  const display = truncateFailureReasonPreview(
    trimmed,
    FAILURE_REASON_MAX_CHARS,
  );
  const truncated = display !== trimmed;

  return (
    <div
      className={
        truncated
          ? "obs-failures-reason-cell obs-failures-reason-cell--truncated"
          : "obs-failures-reason-cell"
      }
      title={truncated ? trimmed : undefined}
    >
      <span className="obs-failures-reason-text">{display}</span>
      {truncated ? (
        <p className="obs-failures-reason-truncated-hint">
          <span className="obs-failures-reason-truncated-icon" aria-hidden>
            ⋯
          </span>
          Hover for the full message
        </p>
      ) : null}
    </div>
  );
}

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

function shortId(id: string): string {
  if (id.length <= 10) return id;
  return `${id.slice(0, 8)}…`;
}
