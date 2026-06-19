import { CopyableId } from "@/shared/CopyableId";
import type { TaskCyclePhase } from "@/types";

export function phaseRunCorrelationId(
  phase: TaskCyclePhase,
): string | undefined {
  const raw = phase.details?.run_correlation_id;
  return typeof raw === "string" && raw.length > 0 ? raw : undefined;
}

export function PhaseDebugDetails({ phase }: { phase: TaskCyclePhase }) {
  const id = phaseRunCorrelationId(phase);
  if (!id) {
    return null;
  }

  return (
    <details className="task-attempt-phase-debug">
      <summary className="task-attempt-phase-debug-summary">Debug</summary>
      <dl className="task-attempt-phase-debug-list">
        <div>
          <dt>Debug id</dt>
          <dd>
            <CopyableId value={id} copyLabel="Copy" />
          </dd>
        </div>
      </dl>
    </details>
  );
}
