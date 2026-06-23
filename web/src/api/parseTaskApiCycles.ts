import { errorMessage } from "@/lib/errorMessage";
import {
  type CycleCommandRun,
  type CycleCommit,
  type CycleCriteriaReport,
  type CycleGitContext,
  type CycleMeta,
  type CycleVerdictsResponse,
  type CycleVerifyReport,
  type TaskCommit,
  type TaskCommitsResponse,
  type TaskCycle,
  type TaskCycleDetail,
  type TaskCyclePhase,
  type TaskCycleStreamEvent,
  type TaskCycleStreamResponse,
  type TaskCyclesListResponse,
  type VerifierKind,
  VERIFIER_KIND_WIRE_VALUES,
} from "@/types";
import {
  isRecord,
  parseActor,
  parseBooleanField,
  parseCycleStatus,
  parseFiniteNumber,
  parseISO8601Required,
  parseNonEmptyString,
  parseObjectField,
  parseOptionalNonEmptyId,
  parseOptionalParseableDate,
  parsePhase,
  parsePhaseStatus,
  parseString,
} from "./parseTaskApiCore";

/**
 * Validates the typed `cycle_meta` projection introduced in Phase 1b of
 * the per-task runner/model attribution plan. The server always emits
 * the full object (zero-value when the underlying `meta` predates the
 * projection keys), so the parser tolerates a missing `cycle_meta`
 * field — falling back to `meta` when present, then to all-empty
 * strings — for forward and backward compatibility with older API
 * snapshots embedded in tests / fixtures.
 *
 * Empty strings are SEMANTIC and propagated verbatim; see
 * {@link CycleMeta} for the rendering contract.
 */
function parseCycleMeta(value: unknown, meta: Record<string, unknown>): CycleMeta {
  const source = isRecord(value) ? value : meta;
  const stringField = (raw: unknown): string => (typeof raw === "string" ? raw : "");
  return {
    runner: stringField(source.runner),
    runner_version: stringField(source.runner_version),
    cursor_model: stringField(source.cursor_model),
    cursor_model_effective: stringField(source.cursor_model_effective),
    prompt_hash: stringField(source.prompt_hash),
  };
}

/** Validates one cycle row from `GET /tasks/{id}/cycles[*]` (also used by detail). */
export function parseTaskCycle(value: unknown): TaskCycle {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: cycle must be an object");
  }
  const meta = parseObjectField(value.meta, "meta");
  const out: TaskCycle = {
    id: parseNonEmptyString(value.id, "id"),
    task_id: parseNonEmptyString(value.task_id, "task_id"),
    attempt_seq: parseFiniteNumber(value.attempt_seq, "attempt_seq"),
    status: parseCycleStatus(value.status),
    started_at: parseISO8601Required(value.started_at, "started_at"),
    triggered_by: parseActor(value.triggered_by),
    meta,
    cycle_meta: parseCycleMeta(value.cycle_meta, meta),
  };
  const ended = parseOptionalParseableDate(value.ended_at, "ended_at");
  if (ended !== undefined) out.ended_at = ended;
  const parent = parseOptionalNonEmptyId(value.parent_cycle_id, "parent_cycle_id");
  if (parent !== undefined) out.parent_cycle_id = parent;
  return out;
}

/** Validates one phase row inside a cycle detail or PATCH/POST phase response. */
export function parseTaskCyclePhase(value: unknown): TaskCyclePhase {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: phase must be an object");
  }
  const out: TaskCyclePhase = {
    id: parseNonEmptyString(value.id, "id"),
    cycle_id: parseNonEmptyString(value.cycle_id, "cycle_id"),
    phase: parsePhase(value.phase),
    phase_seq: parseFiniteNumber(value.phase_seq, "phase_seq"),
    status: parsePhaseStatus(value.status),
    started_at: parseISO8601Required(value.started_at, "started_at"),
    details: parseObjectField(value.details, "details"),
  };
  const ended = parseOptionalParseableDate(value.ended_at, "ended_at");
  if (ended !== undefined) out.ended_at = ended;
  if (value.summary !== undefined && value.summary !== null) {
    out.summary = parseString(value.summary, "summary");
  }
  if (value.event_seq !== undefined && value.event_seq !== null) {
    out.event_seq = parseFiniteNumber(value.event_seq, "event_seq");
  }
  return out;
}

/** Validates `GET /tasks/{id}/cycles` envelope. */
export function parseTaskCyclesListResponse(
  value: unknown,
): TaskCyclesListResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: cycles payload must be an object");
  }
  const raw = value.cycles;
  if (!Array.isArray(raw)) {
    throw new Error("Invalid API response: cycles must be an array");
  }
  const cycles = raw.map((item, i) => {
    try {
      return parseTaskCycle(item);
    } catch (e) {
      throw new Error(`Invalid API response: cycles[${i}]: ${errorMessage(e)}`);
    }
  });
  return {
    task_id: parseNonEmptyString(value.task_id, "task_id"),
    cycles,
    limit: parseFiniteNumber(value.limit, "limit"),
    has_more: parseBooleanField(value.has_more, "has_more"),
  };
}

/** Validates `GET /tasks/{id}/cycles/{cycleId}` envelope (cycle row + ordered phases). */
export function parseTaskCycleDetail(value: unknown): TaskCycleDetail {
  const cycle = parseTaskCycle(value);
  if (!isRecord(value)) {
    throw new Error("Invalid API response: cycle detail must be an object");
  }
  const raw = value.phases;
  if (!Array.isArray(raw)) {
    throw new Error("Invalid API response: phases must be an array");
  }
  const phases = raw.map((item, i) => {
    try {
      return parseTaskCyclePhase(item);
    } catch (e) {
      throw new Error(`Invalid API response: phases[${i}]: ${errorMessage(e)}`);
    }
  });
  return { ...cycle, phases };
}

export function parseTaskCycleStreamEvent(value: unknown): TaskCycleStreamEvent {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: stream event must be an object");
  }
  const out: TaskCycleStreamEvent = {
    id: parseNonEmptyString(value.id, "id"),
    task_id: parseNonEmptyString(value.task_id, "task_id"),
    cycle_id: parseNonEmptyString(value.cycle_id, "cycle_id"),
    phase_seq: parseFiniteNumber(value.phase_seq, "phase_seq"),
    stream_seq: parseFiniteNumber(value.stream_seq, "stream_seq"),
    at: parseISO8601Required(value.at, "at"),
    source: parseNonEmptyString(value.source, "source"),
    kind: parseNonEmptyString(value.kind, "kind"),
    payload: parseObjectField(value.payload, "payload"),
  };
  if (value.subtype !== undefined && value.subtype !== null) {
    out.subtype = parseString(value.subtype, "subtype");
  }
  if (value.message !== undefined && value.message !== null) {
    out.message = parseString(value.message, "message");
  }
  if (value.tool !== undefined && value.tool !== null) {
    out.tool = parseString(value.tool, "tool");
  }
  return out;
}

const verifierKindValues: ReadonlySet<VerifierKind> = new Set(VERIFIER_KIND_WIRE_VALUES);

function parseVerifierKind(value: unknown): VerifierKind {
  if (typeof value !== "string") return "";
  if (!verifierKindValues.has(value as VerifierKind)) return "";
  return value as VerifierKind;
}

export function parseCycleCriteriaReport(value: unknown): CycleCriteriaReport {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: criteria report must be an object");
  }
  return {
    id: parseNonEmptyString(value.id, "id"),
    cycle_id: parseNonEmptyString(value.cycle_id, "cycle_id"),
    attempt_seq: parseFiniteNumber(value.attempt_seq, "attempt_seq"),
    criterion_id: parseNonEmptyString(value.criterion_id, "criterion_id"),
    claimed_done: parseBooleanField(value.claimed_done, "claimed_done"),
    evidence: typeof value.evidence === "string" ? value.evidence : "",
    written_at: parseISO8601Required(value.written_at, "written_at"),
  };
}

export function parseCycleVerifyReport(value: unknown): CycleVerifyReport {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: verify report must be an object");
  }
  return {
    id: parseNonEmptyString(value.id, "id"),
    cycle_id: parseNonEmptyString(value.cycle_id, "cycle_id"),
    attempt_seq: parseFiniteNumber(value.attempt_seq, "attempt_seq"),
    criterion_id: parseNonEmptyString(value.criterion_id, "criterion_id"),
    verified: parseBooleanField(value.verified, "verified"),
    verifier_kind: parseVerifierKind(value.verifier_kind),
    reasoning: typeof value.reasoning === "string" ? value.reasoning : "",
    written_at: parseISO8601Required(value.written_at, "written_at"),
  };
}

export function parseCycleCommandRun(value: unknown): CycleCommandRun {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: command run must be an object");
  }
  return {
    id: parseNonEmptyString(value.id, "id"),
    cycle_id: parseNonEmptyString(value.cycle_id, "cycle_id"),
    attempt_seq: parseFiniteNumber(value.attempt_seq, "attempt_seq"),
    criterion_id: parseNonEmptyString(value.criterion_id, "criterion_id"),
    command_seq: parseFiniteNumber(value.command_seq, "command_seq"),
    exit_code: parseFiniteNumber(value.exit_code, "exit_code"),
    meta_path: typeof value.meta_path === "string" ? value.meta_path : "",
    written_at: parseISO8601Required(value.written_at, "written_at"),
  };
}

function parseCycleGitContext(value: unknown): CycleGitContext | undefined {
  if (!isRecord(value)) {
    return undefined;
  }
  return {
    repo: typeof value.repo === "string" ? value.repo : "",
    worktree: typeof value.worktree === "string" ? value.worktree : "",
    branch: typeof value.branch === "string" ? value.branch : "",
  };
}

export function parseCycleCommit(value: unknown): CycleCommit {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: commit must be an object");
  }
  return {
    seq: parseFiniteNumber(value.seq, "seq"),
    repo: typeof value.repo === "string" ? value.repo : "",
    worktree: typeof value.worktree === "string" ? value.worktree : "",
    branch: typeof value.branch === "string" ? value.branch : "",
    sha: parseNonEmptyString(value.sha, "sha"),
    committed_at: parseISO8601Required(value.committed_at, "committed_at"),
    message: typeof value.message === "string" ? value.message : "",
  };
}

export function parseTaskCommit(value: unknown): TaskCommit {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: task commit must be an object");
  }
  const base = parseCycleCommit(value);
  return {
    ...base,
    cycle_id: parseNonEmptyString(value.cycle_id, "cycle_id"),
    attempt_seq: parseFiniteNumber(value.attempt_seq, "attempt_seq"),
  };
}

export function parseTaskCommitsResponse(value: unknown): TaskCommitsResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: commits payload must be an object");
  }
  const raw = value.commits;
  if (!Array.isArray(raw)) {
    throw new Error("Invalid API response: commits must be an array");
  }
  return {
    task_id: parseNonEmptyString(value.task_id, "task_id"),
    commits: raw.map((item, i) => {
      try {
        return parseTaskCommit(item);
      } catch (e) {
        throw new Error(`Invalid API response: commits[${i}]: ${errorMessage(e)}`);
      }
    }),
  };
}

/**
 * Validates `GET /tasks/{id}/cycles/{cycleId}/verdicts`. All arrays
 * are mandatory but may be empty (pre-PR2 cycles produce no rows).
 */
export function parseCycleVerdictsResponse(
  value: unknown,
): CycleVerdictsResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: verdicts payload must be an object");
  }
  const rawCriteria = value.criteria_reports;
  if (!Array.isArray(rawCriteria)) {
    throw new Error("Invalid API response: criteria_reports must be an array");
  }
  const rawVerify = value.verify_reports;
  if (!Array.isArray(rawVerify)) {
    throw new Error("Invalid API response: verify_reports must be an array");
  }
  const rawCommands = value.command_runs;
  if (!Array.isArray(rawCommands)) {
    throw new Error("Invalid API response: command_runs must be an array");
  }
  const rawCommits = value.commits;
  if (!Array.isArray(rawCommits)) {
    throw new Error("Invalid API response: commits must be an array");
  }
  const criteriaReports = rawCriteria.map((item, i) => {
    try {
      return parseCycleCriteriaReport(item);
    } catch (e) {
      throw new Error(`Invalid API response: criteria_reports[${i}]: ${errorMessage(e)}`);
    }
  });
  const verifyReports = rawVerify.map((item, i) => {
    try {
      return parseCycleVerifyReport(item);
    } catch (e) {
      throw new Error(`Invalid API response: verify_reports[${i}]: ${errorMessage(e)}`);
    }
  });
  const commandRuns = rawCommands.map((item, i) => {
    try {
      return parseCycleCommandRun(item);
    } catch (e) {
      throw new Error(`Invalid API response: command_runs[${i}]: ${errorMessage(e)}`);
    }
  });
  const commits = rawCommits.map((item, i) => {
    try {
      return parseCycleCommit(item);
    } catch (e) {
      throw new Error(`Invalid API response: commits[${i}]: ${errorMessage(e)}`);
    }
  });
  const gitContext = parseCycleGitContext(value.git_context);
  return {
    task_id: parseNonEmptyString(value.task_id, "task_id"),
    cycle_id: parseNonEmptyString(value.cycle_id, "cycle_id"),
    ...(gitContext !== undefined ? { git_context: gitContext } : {}),
    commits,
    criteria_reports: criteriaReports,
    verify_reports: verifyReports,
    command_runs: commandRuns,
  };
}

export function parseTaskCycleStreamResponse(
  value: unknown,
): TaskCycleStreamResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: cycle stream payload must be an object");
  }
  const raw = value.events;
  if (!Array.isArray(raw)) {
    throw new Error("Invalid API response: events must be an array");
  }
  const events = raw.map((item, i) => {
    try {
      return parseTaskCycleStreamEvent(item);
    } catch (e) {
      throw new Error(`Invalid API response: events[${i}]: ${errorMessage(e)}`);
    }
  });
  const out: TaskCycleStreamResponse = {
    task_id: parseNonEmptyString(value.task_id, "task_id"),
    cycle_id: parseNonEmptyString(value.cycle_id, "cycle_id"),
    events,
    limit: parseFiniteNumber(value.limit, "limit"),
    has_more: parseBooleanField(value.has_more, "has_more"),
  };
  if (value.next_after_seq !== undefined && value.next_after_seq !== null) {
    out.next_after_seq = parseFiniteNumber(value.next_after_seq, "next_after_seq");
  }
  return out;
}
