import { Link } from "react-router-dom";
import type { TaskEvent } from "@/types";
import { VerificationCriteriaList } from "../../shared/VerificationCriteriaList";
import {
  awaitingUserReply,
  eventDisplayLabel,
  eventTypeNeedsUserInput,
  parsePhaseEventOverview,
  resolveAttemptAuditRightColumn,
  type AttemptAuditRightColumn,
} from "../../../task-events";

export function AttemptAuditTimeline({
  events,
  taskId,
  ariaLabelledBy,
}: {
  events: TaskEvent[];
  taskId: string;
  ariaLabelledBy: string;
}) {
  return (
    <ol className="attempt-audit-timeline" aria-labelledby={ariaLabelledBy}>
      {events.map((ev) => (
        <AttemptAuditRow key={ev.seq} ev={ev} taskId={taskId} />
      ))}
    </ol>
  );
}

function AttemptAuditRow({ ev, taskId }: { ev: TaskEvent; taskId: string }) {
  const needsUser = eventTypeNeedsUserInput(ev.type);
  const rightColumn = resolveAttemptAuditRightColumn(ev);
  const eventHref = `/tasks/${encodeURIComponent(taskId)}/events/${ev.seq}`;
  const label = eventDisplayLabel(ev);

  return (
    <li
      className={
        needsUser
          ? "attempt-audit-row attempt-audit-row--needs-user"
          : "attempt-audit-row"
      }
      data-needs-user={needsUser ? "true" : undefined}
    >
      <Link
        className="attempt-audit-row-hit"
        to={eventHref}
        aria-label={
          needsUser
            ? awaitingUserReply(ev)
              ? `${label} — needs your input`
              : `${label} — waiting on agent`
            : undefined
        }
      >
        <AttemptAuditRowHead ev={ev} label={label} rightColumn={rightColumn} />
      </Link>
      <AttemptAuditVerificationDetail ev={ev} />
      <AttemptAuditThread ev={ev} />
    </li>
  );
}

function AttemptAuditRowHead({
  ev,
  label,
  rightColumn,
}: {
  ev: TaskEvent;
  label: string;
  rightColumn: AttemptAuditRightColumn | null;
}) {
  const needsUser = eventTypeNeedsUserInput(ev.type);
  return (
    <>
      <time className="attempt-audit-time" dateTime={ev.at}>
        {new Date(ev.at).toLocaleTimeString(undefined, {
          hour: "numeric",
          minute: "2-digit",
        })}
      </time>
      <span className="attempt-audit-label">{label}</span>
      {rightColumn ? (
        <span
          className={attemptAuditRightColumnClassName(rightColumn)}
          title={rightColumn.title}
          aria-label={rightColumn.ariaLabel}
        >
          {rightColumn.label}
        </span>
      ) : null}
      {needsUser && awaitingUserReply(ev) ? (
        <span className="attempt-audit-needs-user">Needs your input</span>
      ) : null}
    </>
  );
}

function attemptAuditRightColumnClassName(
  column: AttemptAuditRightColumn,
): string {
  if (column.variant === "phase") {
    return "attempt-audit-preview attempt-audit-preview--phase-seq";
  }
  if (column.variant === "scope") {
    return `attempt-audit-preview attempt-audit-preview--scope attempt-audit-preview--scope-${column.tone ?? "neutral"}`;
  }
  return "attempt-audit-preview attempt-audit-preview--detail";
}

function AttemptAuditVerificationDetail({ ev }: { ev: TaskEvent }) {
  const overview = parsePhaseEventOverview(ev.type, ev.data);
  const verification = overview?.verification;
  if (!verification || overview?.phase !== "verify") {
    return null;
  }
  const failedOnly =
    ev.type === "phase_failed"
      ? verification.criteria.filter((row) => !row.verified)
      : verification.criteria;
  if (failedOnly.length === 0 && ev.type === "phase_failed") {
    return null;
  }
  const criteria =
    ev.type === "phase_completed" ? verification.criteria : failedOnly;
  const heading =
    ev.type === "phase_completed"
      ? `${verification.passedCount} criteria verified`
      : `${verification.failedCount} of ${verification.passedCount + verification.failedCount} criteria failed`;
  return (
    <div className="attempt-audit-verification">
      <VerificationCriteriaList
        criteria={criteria}
        heading={heading}
        attemptSeq={verification.attemptSeq}
      />
    </div>
  );
}

function AttemptAuditThread({ ev }: { ev: TaskEvent }) {
  if (ev.response_thread && ev.response_thread.length > 0) {
    return (
      <ul className="attempt-audit-thread">
        {ev.response_thread.map((m, i) => (
          <li
            key={`${m.at}-${i}`}
            className={`attempt-audit-thread-msg attempt-audit-thread-msg--${m.by}`}
          >
            <span className="attempt-audit-thread-meta">
              <strong>{m.by === "agent" ? "Agent" : "You"}</strong>
              <time dateTime={m.at}>
                {new Date(m.at).toLocaleTimeString(undefined, {
                  hour: "numeric",
                  minute: "2-digit",
                })}
              </time>
            </span>
            <span className="attempt-audit-thread-body">{m.body}</span>
          </li>
        ))}
      </ul>
    );
  }

  if (ev.user_response) {
    return (
      <div className="attempt-audit-reply">
        <span className="attempt-audit-reply-label">Your reply</span>
        <p className="attempt-audit-reply-body">{ev.user_response}</p>
      </div>
    );
  }

  return null;
}
