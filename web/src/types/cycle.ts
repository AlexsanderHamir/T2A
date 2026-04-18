/**
 * Web-side types for the execution cycles substrate. Mirrors the JSON shapes
 * pinned in `docs/EXECUTION-CYCLES.md`, `docs/API-HTTP.md` (Task execution
 * cycles) and `docs/API-SSE.md` (`task_cycle_changed`). Field names stay
 * snake_case to match the wire format and the parser invariant.
 */

/** `running` is the only non-terminal status; the other three are terminal. */
export type CycleStatus = "running" | "succeeded" | "failed" | "aborted";

/** Phases follow `domain.ValidPhaseTransition` in the backend. */
export type Phase = "diagnose" | "execute" | "verify" | "persist";

/** `running` is the only non-terminal status; the other three are terminal. */
export type PhaseStatus = "running" | "succeeded" | "failed" | "skipped";

export const CYCLE_STATUSES: CycleStatus[] = [
  "running",
  "succeeded",
  "failed",
  "aborted",
];

export const PHASES: Phase[] = ["diagnose", "execute", "verify", "persist"];

export const PHASE_STATUSES: PhaseStatus[] = [
  "running",
  "succeeded",
  "failed",
  "skipped",
];

/** One row from `GET /tasks/{id}/cycles` (or the cycle envelope of `GET /tasks/{id}/cycles/{cycleId}`). */
export type TaskCycle = {
  id: string;
  task_id: string;
  attempt_seq: number;
  status: CycleStatus;
  /** ISO 8601 from API. */
  started_at: string;
  /** ISO 8601 from API; absent while `status === "running"`. */
  ended_at?: string;
  triggered_by: "user" | "agent";
  /** Optional same-task lineage; absent for top-level attempts. */
  parent_cycle_id?: string;
  /** Free-form runner metadata; defaults to `{}` server-side. */
  meta: Record<string, unknown>;
};

/** One row from `GET /tasks/{id}/cycles/{cycleId}::phases`. */
export type TaskCyclePhase = {
  id: string;
  cycle_id: string;
  phase: Phase;
  phase_seq: number;
  status: PhaseStatus;
  started_at: string;
  /** ISO 8601 from API; absent while `status === "running"`. */
  ended_at?: string;
  /** Optional short human-readable note. */
  summary?: string;
  /** Structured per-phase output; defaults to `{}` server-side. */
  details: Record<string, unknown>;
  /** task_events.seq pointer to the most recent mirror row for this phase. */
  event_seq?: number;
};

/** Envelope for `GET /tasks/{id}/cycles`. */
export type TaskCyclesListResponse = {
  task_id: string;
  cycles: TaskCycle[];
  limit: number;
  has_more: boolean;
};

/** Envelope for `GET /tasks/{id}/cycles/{cycleId}` (cycle row + ordered phases). */
export type TaskCycleDetail = TaskCycle & {
  /** Ordered by `phase_seq ASC`. Always present (`[]` when none). */
  phases: TaskCyclePhase[];
};

/** Body for `POST /tasks/{id}/cycles`. Both fields are optional. */
export type StartTaskCycleInput = {
  /** Same-task lineage; omit (or pass `null`) for a top-level attempt. */
  parent_cycle_id?: string | null;
  /** Free-form runner metadata; small JSON object only. */
  meta?: Record<string, unknown>;
};

/** Body for `PATCH /tasks/{id}/cycles/{cycleId}`. */
export type TerminateTaskCycleInput = {
  /** Must be a terminal cycle status: `succeeded` / `failed` / `aborted`. */
  status: Exclude<CycleStatus, "running">;
  /** Optional short reason recorded on the audit mirror row. */
  reason?: string;
};

/** Body for `POST /tasks/{id}/cycles/{cycleId}/phases`. */
export type StartTaskCyclePhaseInput = {
  phase: Phase;
};

/** Body for `PATCH /tasks/{id}/cycles/{cycleId}/phases/{phaseSeq}`. */
export type CompleteTaskCyclePhaseInput = {
  /** Must be a terminal phase status: `succeeded` / `failed` / `skipped`. */
  status: Exclude<PhaseStatus, "running">;
  /** Optional human-readable note; omit to leave the column unchanged. */
  summary?: string;
  /** Optional structured per-phase output; defaults to `{}` server-side. */
  details?: Record<string, unknown>;
};
