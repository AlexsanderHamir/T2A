import { useMemo, useState } from "react";
import { errorMessage } from "@/lib/errorMessage";
import {
  EmptyState,
  EmptyStateTimelineGlyph,
} from "@/shared/EmptyState";
import { formatRelativeTime } from "@/shared/time/relativeTime";
import { useNow } from "@/shared/useNow";
import {
  cycleStatusLabel,
  cycleStatusFillClass,
  cycleRunnerChipClass,
  formatDurationSeconds,
  formatRunnerModel,
  phaseLabel,
  phaseStatusFillClass,
  phaseStatusLabel,
} from "@/observability";
import type {
  CycleCommandRun,
  CycleCommit,
  CycleCriteriaReport,
  CycleGitContext,
  CycleVerifyReport,
  Phase,
  PhaseStatus,
  TaskCycle,
  TaskCyclePhase,
  TaskCyclesListResponse,
  VerifierKind,
} from "@/types/cycle";
import {
  useAgentRunProgress,
  type AgentRunProgress,
} from "../../../hooks/useAgentRunProgress";
import {
  useTaskCycle,
  useTaskCycleVerdicts,
  useTaskCycles,
} from "../../../hooks/useTaskCycles";
import { formatCycleLineageLabel } from "../../../cycleDisplay/cycleLineage";
import { CommitList } from "../commits/CommitList";
import { GitContextMeta } from "../commits/GitContextMeta";

type Props = {
  taskId: string;
  /**
   * Defaults to true. Pass `false` to suppress the panel entirely
   * (e.g. while the parent task query is still pending) so we don't
   * race the task fetch with a 404 from `/tasks/{id}/cycles` when
   * the id is still being resolved.
   */
  enabled?: boolean;
};

/**
 * Per-task observability surface mounted on TaskDetailPage. Composes
 * two pieces from the existing cycle substrate:
 *
 *   1. A live "current phase" ticker for the running cycle (top of
 *      the panel) — answers "what is the agent doing right now?"
 *      without having to scroll through events.
 *   2. A history list of every cycle ever recorded for this task
 *      (newest first), with each cycle's phases inlined as a
 *      mini-strip — answers "what has happened on this task?".
 *
 * Live updates piggy-back on the existing SSE invalidation:
 * `task_cycle_changed` already invalidates `taskQueryKeys.cycles`
 * (see useTaskEventStream), and selecting one running cycle re-uses
 * `useTaskCycle` whose key is also swept by the same invalidator.
 *
 * Designed to render correctly in five states (covered by tests):
 *   - loading                 (cycles query pending)
 *   - error                   (query failed; offers retry)
 *   - empty                   (no cycles ever recorded)
 *   - populated, no running   (only history, no live ticker)
 *   - populated, with running (live ticker + history)
 */
export function TaskCyclesPanel({ taskId, enabled = true }: Props) {
  const cyclesQuery = useTaskCycles(taskId, { enabled });
  const retryCycles = cyclesQuery.refetch;

  const { runningCycle, historyCycles } = useMemo(
    () => splitRunningAndHistory(cyclesQuery.data),
    [cyclesQuery.data],
  );

  const cyclesById = useMemo(
    () => indexCyclesById(cyclesQuery.data?.cycles ?? []),
    [cyclesQuery.data?.cycles],
  );

  return (
    <section
      className="task-detail-section task-cycles-panel"
      aria-labelledby="task-detail-cycles-heading"
    >
      <h3
        className="task-detail-section-heading"
        id="task-detail-cycles-heading"
      >
        <span>Execution cycles</span>
      </h3>

      {cyclesQuery.isPending ? (
        <CyclesLoading />
      ) : cyclesQuery.isError ? (
        <div className="err" role="alert">
          <p>
            {errorMessage(
              cyclesQuery.error,
              "Could not load execution cycles.",
            )}
          </p>
          <div className="task-detail-error-actions">
            <button
              type="button"
              className="secondary"
              onClick={() => {
                void retryCycles();
              }}
            >
              Try again
            </button>
          </div>
        </div>
      ) : historyCycles.length === 0 && !runningCycle ? (
        <EmptyState
          icon={<EmptyStateTimelineGlyph />}
          title="No execution cycles yet"
          description="Each agent run records phases here as execute → verify."
        />
      ) : (
        <>
          {runningCycle ? (
            <CurrentPhaseTicker
              taskId={taskId}
              cycle={runningCycle}
              cyclesById={cyclesById}
            />
          ) : null}
          <CycleHistoryList
            taskId={taskId}
            cycles={historyCycles}
            cyclesById={cyclesById}
            // The running cycle is also in history (newest), but we
            // already render its live phase strip above. Pass its id
            // so the list row can dedupe its phase preview to avoid
            // showing the same phases twice in the same viewport.
            runningCycleId={runningCycle?.id ?? null}
          />
        </>
      )}
    </section>
  );
}

function CyclesLoading() {
  // Two skeleton rows is enough to communicate "list incoming"
  // without hinting at a count we don't actually know yet.
  return (
    <ul
      className="task-cycles-list task-cycles-list--loading"
      aria-busy="true"
      aria-label="Loading execution cycles"
    >
      <li className="task-cycle-row task-cycle-row--skeleton" />
      <li className="task-cycle-row task-cycle-row--skeleton" />
    </ul>
  );
}

/**
 * Live "what is the agent doing right now?" indicator. We fetch the
 * full cycle detail (cycle + phases) for the running cycle so we can
 * highlight which phase is currently running and how long it has
 * been running. The query key is per-cycle and SSE-invalidated, so
 * the elapsed/phase will refresh on every `task_cycle_changed` frame.
 */
function CurrentPhaseTicker({
  taskId,
  cycle,
  cyclesById,
}: {
  taskId: string;
  cycle: TaskCycle;
  cyclesById: ReadonlyMap<string, TaskCycle>;
}) {
  const detailQuery = useTaskCycle(taskId, cycle.id);
  const now = useNow({ enabled: cycle.status === "running" });
  const lineage = formatCycleLineageLabel(cycle, cyclesById);

  return (
    <div
      className="task-cycle-ticker"
      data-testid="task-cycle-ticker"
    >
      <div className="task-cycle-ticker-row">
        <span className="task-cycle-ticker-eyebrow">Live</span>
        <span
          className={`cell-pill ${cycleStatusFillClass(cycle.status)}`}
          data-testid="task-cycle-ticker-status"
        >
          {cycleStatusLabel(cycle.status)}
        </span>
        <span className="task-cycle-ticker-attempt">
          Attempt #{cycle.attempt_seq}
          {lineage ? (
            <span className="task-cycle-lineage muted"> · {lineage}</span>
          ) : null}
        </span>
        <span
          className={`cell-pill ${cycleRunnerChipClass()}`}
          data-testid="task-cycle-ticker-runner"
        >
          {formatRunnerModel(cycle.cycle_meta)}
        </span>
        <span
          className="task-cycle-ticker-elapsed"
          data-testid="task-cycle-ticker-elapsed"
        >
          Started {formatDurationSeconds(elapsedSeconds(cycle.started_at, now))} ago
        </span>
      </div>
      <CurrentPhaseLine
        taskId={taskId}
        cycleId={cycle.id}
        detailQuery={detailQuery}
        now={now}
      />
    </div>
  );
}

/**
 * Bottom line of the ticker. We keep this as a small subcomponent so
 * the cycle-detail fetch can be in any of {pending, error, ready
 * with no running phase, ready with a running phase} without making
 * the parent ticker layout shift.
 */
function CurrentPhaseLine({
  taskId,
  cycleId,
  detailQuery,
  now,
}: {
  taskId: string;
  cycleId: string;
  detailQuery: ReturnType<typeof useTaskCycle>;
  now: number;
}) {
  if (detailQuery.isPending) {
    return (
      <p
        className="task-cycle-ticker-phase task-cycle-ticker-phase--pending"
        aria-busy="true"
      >
        Resolving current phase…
      </p>
    );
  }
  if (detailQuery.isError) {
    // Don't yell at the user — the cycle list still rendered. The
    // ticker is best-effort live state; the history below remains
    // authoritative.
    return (
      <p className="task-cycle-ticker-phase task-cycle-ticker-phase--error">
        Could not resolve current phase ({errorMessage(detailQuery.error, "unknown error")}).
      </p>
    );
  }
  const detail = detailQuery.data;
  const runningPhase = pickRunningPhase(detail.phases);
  if (!runningPhase) {
    // The cycle is "running" but no phase row is currently in the
    // running state — happens between phases (the worker has just
    // closed one and not yet started the next). Show the most
    // recently active phase so the operator has context.
    const lastPhase = pickLatestPhase(detail.phases);
    if (!lastPhase) {
      return (
        <p
          className="task-cycle-ticker-phase task-cycle-ticker-phase--idle"
          data-testid="task-cycle-ticker-phase"
        >
          No phase started yet.
        </p>
      );
    }
    return (
      <p
        className="task-cycle-ticker-phase"
        data-testid="task-cycle-ticker-phase"
      >
        Between phases · last:{" "}
        <span className={`cell-pill ${phaseStatusFillClass(lastPhase.status)}`}>
          {phaseLabel(lastPhase.phase)} {phaseStatusLabel(lastPhase.status).toLowerCase()}
        </span>
      </p>
    );
  }
  return (
    <>
      <p
        className="task-cycle-ticker-phase task-cycle-ticker-phase--running"
        data-testid="task-cycle-ticker-phase"
      >
        <span aria-live="polite">
          Now running:{" "}
          <span className={`cell-pill ${phaseStatusFillClass(runningPhase.status)}`}>
            {phaseLabel(runningPhase.phase)}
          </span>
        </span>{" "}
        <span className="task-cycle-ticker-phase-elapsed" aria-hidden="true">
          for {formatDurationSeconds(elapsedSeconds(runningPhase.started_at, now))}
        </span>
      </p>
      <PhaseProgress
        taskId={taskId}
        cycleId={cycleId}
        phaseSeq={runningPhase.phase_seq}
      />
    </>
  );
}

function PhaseProgress({
  taskId,
  cycleId,
  phaseSeq,
}: {
  taskId: string;
  cycleId: string;
  phaseSeq: number;
}) {
  const items = useAgentRunProgress(taskId, cycleId, phaseSeq);
  if (items.length === 0) {
    return (
      <p className="task-cycle-progress-empty" data-testid="task-cycle-progress-empty">
        Waiting for the next agent update…
      </p>
    );
  }
  return (
    <ol
      className="task-cycle-progress-list"
      aria-label="Recent agent progress"
      data-testid="task-cycle-progress-list"
    >
      {items.map((item, idx) => (
        <li
          key={`${item.receivedAt}:${idx}:${item.progress.kind}:${item.progress.subtype ?? ""}`}
          className="task-cycle-progress-item"
        >
          <span className="task-cycle-progress-kind">
            {progressKindLabel(item.progress.kind, item.progress.subtype)}
          </span>
          <span className="task-cycle-progress-message">
            {progressMessage(item.progress)}
          </span>
        </li>
      ))}
    </ol>
  );
}

/**
 * Newest-first list of every cycle. Each row shows the cycle's
 * status, attempt number, started/finished timestamps, and a small
 * phase strip. We render at most 8 cycles directly; if the API
 * paginated the list (`has_more === true`), we surface that as a
 * footnote so the operator knows the view is partial. We don't add
 * a "load more" button yet — phase data lives in /cycles/{id} so
 * each load triggers extra requests; punt that to a later stage.
 */
function CycleHistoryList({
  taskId,
  cycles,
  runningCycleId,
  cyclesById,
}: {
  taskId: string;
  cycles: TaskCycle[];
  runningCycleId: string | null;
  cyclesById: ReadonlyMap<string, TaskCycle>;
}) {
  if (cycles.length === 0) {
    // Defensive: parent already special-cased the "no history" empty
    // state, but a running-only state (no history yet) is plausible
    // and the ticker renders above without needing this list.
    return null;
  }
  return (
    <ol className="task-cycles-list" data-testid="task-cycles-list">
      {cycles.map((cycle) => (
        <CycleRow
          key={cycle.id}
          taskId={taskId}
          cycle={cycle}
          isLiveAbove={cycle.id === runningCycleId}
          cyclesById={cyclesById}
        />
      ))}
    </ol>
  );
}

/**
 * One historical cycle. Always shows the cycle metadata; lazily
 * fetches the cycle detail (phases) so the cost scales with the
 * number of rows the user actually wants to inspect, not with the
 * length of the list. A row is "expanded" when its <details> is open;
 * we use the native element so keyboard + assistive tech "just work".
 */
function CycleRow({
  taskId,
  cycle,
  isLiveAbove,
  cyclesById,
}: {
  taskId: string;
  cycle: TaskCycle;
  isLiveAbove: boolean;
  cyclesById: ReadonlyMap<string, TaskCycle>;
}) {
  // We track open state in React (rather than letting the native
  // <details> open silently) so that we mount CycleRowPhases only
  // when the row is actually expanded. The cycle-detail fetch
  // therefore costs zero requests for collapsed rows — important
  // for tasks with long history where the operator only inspects
  // a couple of cycles.
  const [open, setOpen] = useState(false);
  const lineage = formatCycleLineageLabel(cycle, cyclesById);

  return (
    <li className="task-cycle-row" data-cycle-status={cycle.status}>
      <details
        open={open}
        onToggle={(e) => setOpen((e.currentTarget as HTMLDetailsElement).open)}
      >
        <summary className="task-cycle-row-summary">
          <span
            className={`cell-pill ${cycleStatusFillClass(cycle.status)}`}
            data-testid="task-cycle-row-status"
          >
            {cycleStatusLabel(cycle.status)}
          </span>
          <span className="task-cycle-row-attempt">
            Attempt #{cycle.attempt_seq}
            {lineage ? (
              <span className="task-cycle-lineage muted"> · {lineage}</span>
            ) : null}
          </span>
          <span className="task-cycle-row-when muted">
            {formatStartedToEnded(cycle)}
          </span>
          <span className="task-cycle-row-trigger muted">
            by {cycle.triggered_by}
          </span>
          <span
            className={`cell-pill ${cycleRunnerChipClass()}`}
            data-testid="task-cycle-row-runner"
          >
            {formatRunnerModel(cycle.cycle_meta)}
          </span>
          {isLiveAbove ? (
            <span
              className="task-cycle-row-livehint"
              aria-label="This cycle is shown in the live ticker above"
            >
              ↑ live
            </span>
          ) : null}
          <a
            className="task-cycle-row-attempt-link"
            href={`/tasks/${encodeURIComponent(taskId)}/cycles/${encodeURIComponent(cycle.id)}`}
            onClick={(e) => e.stopPropagation()}
          >
            View run details
          </a>
        </summary>
        {open ? (
          <>
            <CycleRowPhases taskId={taskId} cycleId={cycle.id} />
            <CycleRowVerdicts taskId={taskId} cycleId={cycle.id} />
          </>
        ) : null}
      </details>
    </li>
  );
}

/**
 * Per-criterion verdict block shown when a cycle row is expanded.
 * The data comes from `GET /tasks/{id}/cycles/{cycleId}/verdicts` —
 * cycles produced before the verdict-tables migration return empty
 * arrays, which we render as a small muted note rather than a hard
 * error so the operator immediately understands "no verdicts captured
 * for this cycle" without having to interpret a stack trace.
 *
 * Within an attempt we render verify-side rows first (the verifier's
 * judgement is what the operator usually opens this panel for) and
 * fall back to the agent-self-claimed criteria report when there's
 * no verify-side row (verify-failed early or verify-tampered cases).
 */
function CycleRowVerdicts({
  taskId,
  cycleId,
}: {
  taskId: string;
  cycleId: string;
}) {
  const verdictsQuery = useTaskCycleVerdicts(taskId, cycleId);
  if (verdictsQuery.isPending) {
    return (
      <p className="task-cycle-row-verdicts muted" aria-busy="true">
        Loading verdicts…
      </p>
    );
  }
  if (verdictsQuery.isError) {
    return (
      <p className="task-cycle-row-verdicts err" role="alert">
        {errorMessage(verdictsQuery.error, "Could not load verdicts.")}
      </p>
    );
  }
  const data = verdictsQuery.data;
  if (
    data.criteria_reports.length === 0 &&
    data.verify_reports.length === 0 &&
    data.command_runs.length === 0 &&
    data.commits.length === 0
  ) {
    return (
      <p
        className="task-cycle-row-verdicts muted"
        data-testid="task-cycle-verdicts-empty"
      >
        No verdicts captured for this cycle.
      </p>
    );
  }
  const groups = groupVerdictsByAttempt(
    data.criteria_reports,
    data.verify_reports,
  );
  return (
    <div
      className="task-cycle-row-verdicts"
      data-testid="task-cycle-verdicts"
    >
      <CycleCommitsSummary
        gitContext={data.git_context}
        commits={data.commits}
      />
      <h4 className="task-cycle-row-verdicts-heading">Verdicts</h4>
      {groups.map((group) => (
        <section
          key={group.attemptSeq}
          className="task-cycle-verdicts-attempt"
          aria-label={`Attempt ${group.attemptSeq}`}
        >
          <p className="task-cycle-verdicts-attempt-eyebrow muted">
            Attempt #{group.attemptSeq}
          </p>
          <ul className="task-cycle-verdicts-list">
            {group.rows.map((row) => (
              <li
                key={row.criterionId}
                className="task-cycle-verdict-item"
                data-verified={String(row.verified)}
              >
                <header className="task-cycle-verdict-item-header">
                  <span
                    className={`cell-pill ${verdictPillClass(row.verified)}`}
                  >
                    {row.verified ? "Verified" : "Not verified"}
                  </span>
                  {row.verifierKind ? (
                    <span className="task-cycle-verdict-kind muted">
                      {verifierKindLabel(row.verifierKind)}
                    </span>
                  ) : null}
                  <span className="task-cycle-verdict-criterion">
                    {row.criterionId}
                  </span>
                </header>
                {row.reasoning ? (
                  <p className="task-cycle-verdict-reasoning">
                    {row.reasoning}
                  </p>
                ) : row.evidence ? (
                  <p className="task-cycle-verdict-evidence muted">
                    Agent-claimed evidence: {row.evidence}
                  </p>
                ) : null}
              </li>
            ))}
          </ul>
          <CycleCommandRunsSummary
            runs={commandRunsForAttempt(data.command_runs, group.attemptSeq)}
          />
        </section>
      ))}
      {groups.length === 0 && data.command_runs.length > 0
        ? commandRunAttemptSeqs(data.command_runs).map((attemptSeq) => (
            <section
              key={attemptSeq}
              className="task-cycle-verdicts-attempt"
              aria-label={`Attempt ${attemptSeq}`}
            >
              <p className="task-cycle-verdicts-attempt-eyebrow muted">
                Attempt #{attemptSeq}
              </p>
              <CycleCommandRunsSummary
                runs={commandRunsForAttempt(data.command_runs, attemptSeq)}
              />
            </section>
          ))
        : null}
    </div>
  );
}

function commandRunsForAttempt(
  runs: ReadonlyArray<CycleCommandRun>,
  attemptSeq: number,
): CycleCommandRun[] {
  return runs.filter((r) => r.attempt_seq === attemptSeq);
}

function commandRunAttemptSeqs(runs: ReadonlyArray<CycleCommandRun>): number[] {
  const seen = new Set<number>();
  for (const r of runs) {
    seen.add(r.attempt_seq);
  }
  return Array.from(seen).sort((a, b) => a - b);
}

function CycleCommitsSummary({
  gitContext,
  commits,
}: {
  gitContext?: CycleGitContext;
  commits: ReadonlyArray<CycleCommit>;
}) {
  if (commits.length === 0) {
    return null;
  }
  const ctx = gitContext ?? {
    repo: commits[0]?.repo ?? "",
    worktree: commits[0]?.worktree ?? "",
    branch: commits[commits.length - 1]?.branch ?? commits[0]?.branch ?? "",
  };
  return (
    <section
      className="task-cycle-commits"
      data-testid="task-cycle-commits"
      aria-label="Git commits"
    >
      <h4 className="task-cycle-row-verdicts-heading">Commits</h4>
      <GitContextMeta context={ctx} />
      <CommitList commits={commits} />
    </section>
  );
}

function CycleCommandRunsSummary({
  runs,
}: {
  runs: ReadonlyArray<CycleCommandRun>;
}) {
  if (runs.length === 0) {
    return null;
  }
  return (
    <ul className="task-cycle-command-runs-list">
      {runs.map((run) => (
        <li key={run.id} className="task-cycle-command-run muted">
          <span className="task-cycle-command-run-label">
            [{run.criterion_id}] command {run.command_seq}
          </span>
          <span className="task-cycle-command-run-meta">
            exit {run.exit_code}
          </span>
        </li>
      ))}
    </ul>
  );
}

type VerdictRow = {
  criterionId: string;
  verified: boolean;
  verifierKind: VerifierKind | "";
  reasoning: string;
  evidence: string;
};

type AttemptGroup = {
  attemptSeq: number;
  rows: VerdictRow[];
};

/**
 * Joins criteria-side and verify-side rows by `(attempt_seq, criterion_id)`.
 * The verify row wins when both exist; the criteria row provides
 * `evidence` text used as the fallback display when no verify row was
 * recorded (e.g. the verifier never ran for that criterion).
 */
function groupVerdictsByAttempt(
  criteria: ReadonlyArray<CycleCriteriaReport>,
  verify: ReadonlyArray<CycleVerifyReport>,
): AttemptGroup[] {
  const byAttempt = new Map<number, Map<string, VerdictRow>>();
  const ensure = (attemptSeq: number): Map<string, VerdictRow> => {
    let m = byAttempt.get(attemptSeq);
    if (!m) {
      m = new Map();
      byAttempt.set(attemptSeq, m);
    }
    return m;
  };
  for (const c of criteria) {
    const m = ensure(c.attempt_seq);
    m.set(c.criterion_id, {
      criterionId: c.criterion_id,
      verified: false,
      verifierKind: "",
      reasoning: "",
      evidence: c.evidence,
    });
  }
  for (const v of verify) {
    const m = ensure(v.attempt_seq);
    const existing = m.get(v.criterion_id);
    m.set(v.criterion_id, {
      criterionId: v.criterion_id,
      verified: v.verified,
      verifierKind: v.verifier_kind,
      reasoning: v.reasoning,
      evidence: existing?.evidence ?? "",
    });
  }
  const groups: AttemptGroup[] = [];
  for (const [attemptSeq, m] of byAttempt) {
    const rows = Array.from(m.values()).sort((a, b) =>
      a.criterionId.localeCompare(b.criterionId),
    );
    groups.push({ attemptSeq, rows });
  }
  groups.sort((a, b) => a.attemptSeq - b.attemptSeq);
  return groups;
}

function verdictPillClass(verified: boolean): string {
  return verified ? "cell-pill--status-done" : "cell-pill--status-failed";
}

function verifierKindLabel(kind: VerifierKind | ""): string {
  switch (kind) {
    case "verify_agent":
      return "Verify agent";
    case "agent_self":
      return "Self-reported";
    case "deterministic_check":
      return "Deterministic check";
    case "human_override":
      return "Human override";
    case "legacy":
      return "Legacy";
    default:
      return "";
  }
}

/**
 * Phase list shown when a cycle row is expanded. Mounting this
 * component triggers the cycle-detail query (useTaskCycle), so the
 * network cost is paid only when the operator opens the row.
 */
function CycleRowPhases({
  taskId,
  cycleId,
}: {
  taskId: string;
  cycleId: string;
}) {
  const detailQuery = useTaskCycle(taskId, cycleId);
  const phases = detailQuery.data?.phases ?? [];
  const hasRunningPhase = phases.some((phase) => phase.status === "running");
  const now = useNow({ enabled: hasRunningPhase });

  if (detailQuery.isPending) {
    return (
      <p className="task-cycle-row-phases muted" aria-busy="true">
        Loading phases…
      </p>
    );
  }
  if (detailQuery.isError) {
    return (
      <p className="task-cycle-row-phases err" role="alert">
        {errorMessage(detailQuery.error, "Could not load phases.")}
      </p>
    );
  }
  if (phases.length === 0) {
    return (
      <p className="task-cycle-row-phases muted">
        No phases recorded for this cycle.
      </p>
    );
  }
  return (
    <ol className="task-cycle-phase-list" aria-label="Phases for this cycle">
      {phases.map((phase) => (
        <li
          key={phase.id}
          className="task-cycle-phase-item"
          data-phase-status={phase.status}
        >
          <span className="task-cycle-phase-name">
            {phaseLabel(phase.phase)}
          </span>
          <span
            className={`cell-pill ${phaseStatusFillClass(phase.status)}`}
          >
            {phaseStatusLabel(phase.status)}
          </span>
          <span className="task-cycle-phase-duration muted">
            {formatPhaseDuration(phase, now)}
          </span>
          {phase.summary ? (
            <span className="task-cycle-phase-summary">{phase.summary}</span>
          ) : null}
        </li>
      ))}
    </ol>
  );
}

/**
 * Splits the cycle list into the (at most one) running cycle and the
 * full history. The history *includes* the running cycle so the list
 * order stays stable across the running→terminal transition (the row
 * stays in place; only its status pill flips). The running cycle is
 * surfaced separately so the live ticker can render above without
 * having to scan the list. The backend orders cycles newest-first.
 */
function splitRunningAndHistory(
  envelope: TaskCyclesListResponse | undefined,
): { runningCycle: TaskCycle | null; historyCycles: TaskCycle[] } {
  if (!envelope) return { runningCycle: null, historyCycles: [] };
  const running = envelope.cycles.find((c) => c.status === "running") ?? null;
  return { runningCycle: running, historyCycles: envelope.cycles };
}

function indexCyclesById(cycles: ReadonlyArray<TaskCycle>): Map<string, TaskCycle> {
  const map = new Map<string, TaskCycle>();
  for (const cycle of cycles) {
    map.set(cycle.id, cycle);
  }
  return map;
}

function pickRunningPhase(
  phases: ReadonlyArray<TaskCyclePhase>,
): TaskCyclePhase | null {
  return phases.find((p) => p.status === "running") ?? null;
}

function pickLatestPhase(
  phases: ReadonlyArray<TaskCyclePhase>,
): TaskCyclePhase | null {
  if (phases.length === 0) return null;
  // Server returns phases ordered by phase_seq ASC; the most recently
  // touched phase is the one with the highest seq.
  let best: TaskCyclePhase = phases[0];
  for (const p of phases) {
    if (p.phase_seq > best.phase_seq) best = p;
  }
  return best;
}

function elapsedSeconds(isoStart: string, now: number): number {
  const start = Date.parse(isoStart);
  if (!Number.isFinite(start)) return 0;
  return Math.max(0, (now - start) / 1000);
}

function formatStartedToEnded(cycle: TaskCycle): string {
  const start = formatLocalTime(cycle.started_at);
  if (cycle.status === "running" || !cycle.ended_at) {
    return `${start} → in progress`;
  }
  const end = formatLocalTime(cycle.ended_at);
  return `${start} → ${end}`;
}

function formatLocalTime(iso: string): string {
  const ts = Date.parse(iso);
  if (!Number.isFinite(ts)) return iso;
  // Locale-aware short time + date — chosen to match the task event
  // timeline's existing visual rhythm (short, scan-friendly).
  const d = new Date(ts);
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatPhaseDuration(phase: TaskCyclePhase, now: number): string {
  const start = Date.parse(phase.started_at);
  if (!Number.isFinite(start)) return "—";
  const end = phase.ended_at ? Date.parse(phase.ended_at) : now;
  if (!Number.isFinite(end) || end < start) return "—";
  return formatDurationSeconds((end - start) / 1000);
}

function progressKindLabel(kind: string, subtype: string | undefined): string {
  if (kind === "tool_call") {
    if (subtype === "completed" || subtype === "success" || subtype === "done") {
      return "Done";
    }
    if (subtype === "failed" || subtype === "error") {
      return "Failed";
    }
    return "Tool";
  }
  if (kind === "assistant") {
    return "Agent";
  }
  if (kind === "system") {
    return "Session";
  }
  return "Update";
}

function progressMessage(progress: AgentRunProgress): string {
  if (progress.message) {
    return progress.message;
  }
  if (progress.tool) {
    return progress.tool;
  }
  return "Working…";
}

// Re-exported for tests so they can construct fixtures without
// owning the Phase/PhaseStatus type imports.
export type { Phase, PhaseStatus };
