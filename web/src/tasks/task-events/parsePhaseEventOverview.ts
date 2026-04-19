import type { TaskEventType } from "@/types/task";

/** Structured view for agent phase / cycle audit events (optional Raw JSON tab). */
export type PhaseEventOverviewModel = {
  phase: string;
  status: string;
  summary?: string;
  cycleId?: string;
  phaseSeq?: number;
  durationMs?: number;
  durationApiMs?: number;
  requestId?: string;
  sessionId?: string;
  usage?: {
    inputTokens?: number;
    outputTokens?: number;
    cacheReadTokens?: number;
    cacheWriteTokens?: number;
  };
  failureKind?: string;
  standardizedMessage?: string;
  stderrTail?: string;
};

function num(v: unknown): number | undefined {
  return typeof v === "number" && Number.isFinite(v) ? v : undefined;
}

function str(v: unknown): string | undefined {
  return typeof v === "string" && v.length > 0 ? v : undefined;
}

function readUsage(raw: unknown): PhaseEventOverviewModel["usage"] | undefined {
  if (!raw || typeof raw !== "object") return undefined;
  const o = raw as Record<string, unknown>;
  const inputTokens = num(o.inputTokens);
  const outputTokens = num(o.outputTokens);
  const cacheReadTokens = num(o.cacheReadTokens);
  const cacheWriteTokens = num(o.cacheWriteTokens);
  if (
    inputTokens === undefined &&
    outputTokens === undefined &&
    cacheReadTokens === undefined &&
    cacheWriteTokens === undefined
  ) {
    return undefined;
  }
  return {
    inputTokens,
    outputTokens,
    cacheReadTokens,
    cacheWriteTokens,
  };
}

function readDetailsBlob(details: unknown): {
  durationMs?: number;
  durationApiMs?: number;
  requestId?: string;
  sessionId?: string;
  usage?: PhaseEventOverviewModel["usage"];
  failureKind?: string;
  standardizedMessage?: string;
  stderrTail?: string;
} {
  if (!details || typeof details !== "object") return {};
  const d = details as Record<string, unknown>;
  return {
    durationMs: num(d.duration_ms),
    durationApiMs: num(d.duration_api_ms),
    requestId: str(d.request_id),
    sessionId: str(d.session_id),
    usage: readUsage(d.usage),
    failureKind: str(d.failure_kind),
    standardizedMessage: str(d.standardized_message),
    stderrTail: str(d.stderr_tail),
  };
}

const PHASE_OVERVIEW_TYPES = new Set<TaskEventType>([
  "phase_completed",
  "phase_failed",
]);

/**
 * Phase summaries are often markdown. Some payloads store newlines as the two
 * characters `\` + `n` instead of real line breaks; normalize so GFM tables
 * and lists parse correctly in the UI.
 */
export function normalizePhaseSummaryMarkdown(raw: string): string {
  let s = raw.replace(/^\n+/, "").trimEnd();
  s = s.replace(/\\r\\n/g, "\n").replace(/\\n/g, "\n").replace(/\\r/g, "\n");
  return s;
}

/**
 * When non-null, the event detail page can show an Overview tab with metrics
 * and a rendered summary before the raw JSON.
 */
export function parsePhaseEventOverview(
  type: TaskEventType,
  data: Record<string, unknown>,
): PhaseEventOverviewModel | null {
  if (!PHASE_OVERVIEW_TYPES.has(type)) return null;
  const phase = str(data.phase);
  const status = str(data.status);
  if (!phase || !status) return null;

  const summary = str(data.summary);
  const cycleId = str(data.cycle_id);
  const phaseSeq = num(data.phase_seq);

  const det = readDetailsBlob(data.details);
  return {
    phase,
    status,
    summary,
    cycleId,
    phaseSeq,
    durationMs: det.durationMs,
    durationApiMs: det.durationApiMs,
    requestId: det.requestId,
    sessionId: det.sessionId,
    usage: det.usage,
    failureKind: det.failureKind,
    standardizedMessage: det.standardizedMessage,
    stderrTail: det.stderrTail,
  };
}
