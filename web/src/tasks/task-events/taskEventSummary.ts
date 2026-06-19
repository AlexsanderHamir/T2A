import type { TaskEvent, TaskEventType } from "@/types";
import {
  normalizePhaseSummaryMarkdown,
  parseCycleTerminalOverview,
  parsePhaseEventOverview,
} from "./parsePhaseEventOverview";

const PHASE_LIFECYCLE_TYPES = new Set<TaskEventType>([
  "phase_started",
  "phase_completed",
  "phase_failed",
  "phase_skipped",
]);

const TRANSITION_TYPES = new Set<TaskEventType>([
  "status_changed",
  "priority_changed",
  "message_added",
  "prompt_appended",
]);

function truncate(text: string, max: number): string {
  const trimmed = text.trim();
  if (trimmed.length <= max) return trimmed;
  return `${trimmed.slice(0, max - 1).trimEnd()}…`;
}

/** Strips markdown noise for inline UI; prefers the first substantive line. */
function phaseSummaryPlainLine(raw: string): string {
  const normalized = normalizePhaseSummaryMarkdown(raw);
  const firstLine =
    normalized
      .split(/\r?\n/)
      .map((line) => line.trim())
      .find((line) => line.length > 0) ?? normalized;

  return firstLine
    .replace(/^#{1,6}\s+/, "")
    .replace(/`([^`]+)`/g, "$1")
    .replace(/\*\*([^*]+)\*\*/g, "$1")
    .replace(/\*([^*]+)\*/g, "$1")
    .replace(/_([^_]+)_/g, "$1")
    .replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")
    .replace(/\s+/g, " ")
    .trim();
}

/**
 * Short plain-text preview for phase.summary in compact UI (steppers, lists).
 * Full markdown remains on the event detail page.
 */
export function formatPhaseSummaryCompact(
  summary: string | undefined | null,
  max = 120,
): string | null {
  if (!summary?.trim()) return null;
  const plain = phaseSummaryPlainLine(summary);
  if (!plain) return null;
  return truncate(plain, max);
}

/**
 * One-line human summary for compact timelines. Omits raw JSON and internal
 * ids — callers link to the full event page for payload inspection.
 */
export function formatEventSummaryCompact(ev: TaskEvent): string | null {
  const cycleOverview = parseCycleTerminalOverview(ev.type, ev.data);
  if (cycleOverview) {
    if (cycleOverview.failureSummary) {
      return truncate(cycleOverview.failureSummary, 140);
    }
    if (cycleOverview.reason) return truncate(cycleOverview.reason, 140);
    return null;
  }

  const phaseOverview = parsePhaseEventOverview(ev.type, ev.data);
  if (phaseOverview) {
    if (phaseOverview.standardizedMessage) {
      return truncate(phaseOverview.standardizedMessage, 140);
    }
    if (phaseOverview.summary) {
      return formatPhaseSummaryCompact(phaseOverview.summary, 140);
    }
    return null;
  }

  if (TRANSITION_TYPES.has(ev.type)) {
    const from = ev.data.from;
    const to = ev.data.to;
    if (typeof from === "string" && typeof to === "string") {
      return `${from} → ${to}`;
    }
  }

  if (ev.type === "checklist_item_removed") {
    const text = ev.data.text;
    if (typeof text === "string") return truncate(text, 140);
  }

  if (ev.type === "task_retry_requested") {
    const mode = ev.data.mode;
    if (typeof mode === "string" && mode.trim()) {
      return mode === "fresh" ? "Start over" : mode === "resume" ? "Resume from failure" : mode;
    }
  }

  return null;
}

/**
 * Right-column preview for the attempt audit timeline. Phase lifecycle rows
 * show `PHASE {n}` for correlation with the Phases track at the top of the
 * page; long phase summaries stay on the event detail page. The chip uses
 * the explicit `PHASE` prefix (not `P`) to read as a self-describing label
 * rather than a code that requires the user to learn the legend.
 */
export function formatAttemptAuditPreview(ev: TaskEvent): string | null {
  if (PHASE_LIFECYCLE_TYPES.has(ev.type)) {
    const phase = ev.data.phase;
    if (phase === "verify") {
      const seq = ev.data.phase_seq;
      if (typeof seq === "number" && Number.isFinite(seq)) {
        return `PHASE ${seq}`;
      }
    }
  }

  if (ev.type === "phase_failed") {
    const phaseOverview = parsePhaseEventOverview(ev.type, ev.data);
    if (phaseOverview?.verification) {
      const { failedCount, passedCount } = phaseOverview.verification;
      const total = failedCount + passedCount;
      if (failedCount > 0 && total > 0) {
        return `${failedCount} of ${total} failed`;
      }
    }
    if (phaseOverview?.standardizedMessage) {
      return truncate(phaseOverview.standardizedMessage, 140);
    }
    if (phaseOverview?.summary) {
      const compact = formatPhaseSummaryCompact(phaseOverview.summary, 140);
      if (compact && compact !== "verification failed") {
        return compact;
      }
    }
  }

  if (PHASE_LIFECYCLE_TYPES.has(ev.type)) {
    const seq = ev.data.phase_seq;
    if (typeof seq === "number" && Number.isFinite(seq)) {
      return `PHASE ${seq}`;
    }
    return null;
  }

  return formatEventSummaryCompact(ev);
}

export type AttemptAuditRightColumnVariant = "phase" | "detail" | "scope";

export type AttemptAuditScopeTone =
  | "cycle"
  | "checklist"
  | "task"
  | "approval"
  | "neutral";

export type AttemptAuditRightColumn = {
  label: string;
  variant: AttemptAuditRightColumnVariant;
  tone?: AttemptAuditScopeTone;
  title?: string;
  ariaLabel?: string;
};

const CYCLE_SCOPE_TYPES = new Set<TaskEventType>([
  "cycle_started",
  "cycle_completed",
  "cycle_failed",
]);

const CHECKLIST_SCOPE_TYPES = new Set<TaskEventType>([
  "checklist_item_added",
  "checklist_item_toggled",
  "checklist_item_updated",
  "checklist_item_removed",
]);

const APPROVAL_SCOPE_TYPES = new Set<TaskEventType>([
  "approval_requested",
  "approval_granted",
]);

const TASK_SCOPE_TYPES = new Set<TaskEventType>([
  "task_created",
  "status_changed",
  "priority_changed",
  "prompt_appended",
  "context_added",
  "constraint_added",
  "success_criterion_added",
  "non_goal_added",
  "plan_added",
  "message_added",
  "artifact_added",
  "task_completed",
  "task_failed",
  "task_retry_requested",
  "sync_ping",
]);

function resolveAttemptAuditScope(
  ev: TaskEvent,
): AttemptAuditRightColumn | null {
  if (CYCLE_SCOPE_TYPES.has(ev.type)) {
    return {
      label: "CYCLE",
      variant: "scope",
      tone: "cycle",
      title: "Applies to the whole execution attempt",
      ariaLabel: "Whole attempt",
    };
  }
  if (CHECKLIST_SCOPE_TYPES.has(ev.type)) {
    return {
      label: "CHECKLIST",
      variant: "scope",
      tone: "checklist",
      title: "Done criteria or checklist change",
      ariaLabel: "Checklist",
    };
  }
  if (APPROVAL_SCOPE_TYPES.has(ev.type)) {
    return {
      label: "APPROVAL",
      variant: "scope",
      tone: "approval",
      title: "Human approval gate",
      ariaLabel: "Approval",
    };
  }
  if (TASK_SCOPE_TYPES.has(ev.type)) {
    return {
      label: "TASK",
      variant: "scope",
      tone: "task",
      title: "Task-level change outside a single phase",
      ariaLabel: "Task",
    };
  }
  return {
    label: "TASK",
    variant: "scope",
    tone: "neutral",
    title: "Task audit event",
    ariaLabel: "Task",
  };
}

/**
 * Right-column chip for the attempt audit timeline. Phase lifecycle rows show
 * `PHASE {n}`; transition rows show compact detail text; everything else gets
 * a scope label (`CYCLE`, `CHECKLIST`, …) so the column never looks empty.
 */
export function resolveAttemptAuditRightColumn(
  ev: TaskEvent,
): AttemptAuditRightColumn | null {
  // Cycle lifecycle rows are scope-level: the row label ("Attempt
  // started/completed/failed") already conveys the outcome, and the
  // failure reason lives one click away on the event detail page. Falling
  // through to the detail-text branch (which surfaced cycle_failed's
  // reason inline) made the failed row look structurally different from
  // the completed/started rows — visually noisy and inconsistent with
  // every other CYCLE-scope event.
  if (CYCLE_SCOPE_TYPES.has(ev.type)) {
    return resolveAttemptAuditScope(ev);
  }
  const preview = formatAttemptAuditPreview(ev);
  if (preview) {
    const phaseMatch = /^PHASE (\d+)$/.exec(preview);
    if (phaseMatch) {
      const seq = Number(phaseMatch[1]);
      return {
        label: preview,
        variant: "phase",
        ariaLabel: `Phase ${seq}`,
      };
    }
    return {
      label: preview,
      variant: "detail",
      title: preview,
    };
  }
  return resolveAttemptAuditScope(ev);
}
