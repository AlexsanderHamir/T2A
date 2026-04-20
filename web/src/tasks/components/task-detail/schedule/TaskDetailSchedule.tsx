import { useCallback, useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { Status, Task } from "@/types";
import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { patchTask as patchTaskApi } from "@/api";
import { errorMessage } from "@/lib/errorMessage";
import { useAppTimezone, formatInAppTimezone } from "@/shared/time/appTimezone";
import { SchedulePicker } from "@/shared/time/SchedulePicker";
import { taskQueryKeys } from "../../../task-query";

type Props = {
  task: Pick<Task, "id" | "status" | "pickup_not_before">;
};

const TERMINAL_STATUSES: ReadonlySet<Status> = new Set(["done", "failed"]);

/**
 * TaskDetailSchedule — operator-facing panel on the task detail
 * page for inspecting and changing a task's `pickup_not_before`.
 *
 * Visibility rules (per Stage 4 of the task scheduling plan):
 *  - **Hidden** entirely when the task is in a terminal state
 *    (`done` / `failed`) AND has no schedule. Terminal tasks never
 *    pick up again, and showing a "Schedule pickup" affordance
 *    that has no effect would be a UX trap.
 *  - **Read-only badge** when the task is terminal but already
 *    carried a schedule (rare but possible — a PATCH that flipped
 *    `done` while a future pickup was still set). Surfacing the
 *    badge keeps the historical fact visible; the Edit/Clear
 *    affordances are hidden because mutating a terminal task's
 *    pickup time is meaningless.
 *  - **Full panel** for any non-terminal task: badge + Edit +
 *    optional Clear (the latter only when there's an actual
 *    schedule to clear).
 *
 * The Edit modal reuses `SchedulePicker` from Stage 3 so the
 * create modal and the edit modal share quick-pick semantics,
 * timezone handling, and DST correctness without duplication.
 *
 * Live updates rely on the server's existing `task_updated` SSE
 * frame: any operator's change anywhere in the system invalidates
 * `taskQueryKeys.detail`, which re-renders this component with the
 * fresh value. No bespoke event wiring needed.
 *
 * The PATCH lives in a local `useMutation` rather than going
 * through `useTaskPatchFlow` because the latter requires the
 * caller to forward a full `TaskPatchInput` shape (title, prompt,
 * status, priority, task_type, checklist_inherit) — semantics that
 * don't apply when only the schedule changes. The local mutation
 * does the same query-invalidations (`taskQueryKeys.all` +
 * `task-stats`) so consumers see the same cache-refresh behaviour.
 */
export function TaskDetailSchedule({ task }: Props) {
  const tz = useAppTimezone();
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [draftSchedule, setDraftSchedule] = useState<string | null>(
    task.pickup_not_before ?? null,
  );

  const isTerminal = TERMINAL_STATUSES.has(task.status);
  const hasSchedule = Boolean(task.pickup_not_before);

  const patchMutation = useMutation({
    mutationFn: (next: string | null) =>
      patchTaskApi(task.id, { pickup_not_before: next }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      await queryClient.invalidateQueries({ queryKey: ["task-stats"] });
    },
  });

  const closeEditor = useCallback(() => {
    setEditing(false);
    if (patchMutation.isError) patchMutation.reset();
  }, [patchMutation]);

  const openEditor = useCallback(() => {
    setDraftSchedule(task.pickup_not_before ?? null);
    setEditing(true);
  }, [task.pickup_not_before]);

  // If the underlying task value changes while the editor is open
  // (a remote PATCH wins via SSE invalidation), re-seed the draft so
  // the modal reflects server truth instead of a stale draft. We
  // only re-seed when the editor *just* opened OR when the
  // underlying value actually changed; we deliberately do NOT
  // re-seed on every render so the operator's in-progress edits
  // aren't clobbered.
  useEffect(() => {
    if (!editing) {
      setDraftSchedule(task.pickup_not_before ?? null);
    }
  }, [editing, task.pickup_not_before]);

  const submitDraft = useCallback(() => {
    patchMutation.mutate(draftSchedule, {
      onSuccess: () => {
        setEditing(false);
      },
    });
  }, [draftSchedule, patchMutation]);

  const clearSchedule = useCallback(() => {
    patchMutation.mutate(null);
  }, [patchMutation]);

  if (!hasSchedule && isTerminal) {
    return null;
  }

  const formatted = task.pickup_not_before
    ? formatInAppTimezone(task.pickup_not_before, tz)
    : null;

  return (
    <div
      className="task-detail-schedule"
      data-testid="task-detail-schedule"
      data-state={
        hasSchedule ? "scheduled" : "unscheduled"
      }
    >
      {hasSchedule ? (
        <span
          className="task-detail-schedule-badge"
          data-testid="task-detail-schedule-badge"
        >
          <span aria-hidden="true" className="task-detail-schedule-badge-dot" />
          <span className="task-detail-schedule-badge-text">
            Scheduled for {formatted}
          </span>
        </span>
      ) : (
        <span className="task-detail-schedule-empty muted">
          No pickup scheduled.
        </span>
      )}

      {!isTerminal ? (
        <div className="task-detail-schedule-actions">
          <button
            type="button"
            className="secondary task-detail-schedule-edit"
            onClick={openEditor}
            data-testid="task-detail-schedule-edit"
          >
            {hasSchedule ? "Edit" : "Schedule"}
          </button>
          {hasSchedule ? (
            <button
              type="button"
              className="secondary task-detail-schedule-clear"
              onClick={clearSchedule}
              disabled={patchMutation.isPending}
              data-testid="task-detail-schedule-clear"
              aria-label="Clear scheduled pickup time"
            >
              {patchMutation.isPending ? "Clearing…" : "Clear"}
            </button>
          ) : null}
        </div>
      ) : null}

      {patchMutation.isError && !editing ? (
        <p
          className="err task-detail-schedule-err"
          role="alert"
          data-testid="task-detail-schedule-err"
        >
          {errorMessage(
            patchMutation.error,
            "Could not update the schedule.",
          )}
        </p>
      ) : null}

      {editing ? (
        <Modal
          onClose={closeEditor}
          labelledBy="task-detail-schedule-modal-title"
          busy={patchMutation.isPending}
          busyLabel="Saving schedule…"
          dismissibleWhileBusy
        >
          <section className="panel modal-sheet task-detail-schedule-modal-sheet">
            <h2 id="task-detail-schedule-modal-title">Edit pickup schedule</h2>
            <p className="muted">
              Pick a future time for the agent to start this task. Times are
              shown in <strong>{tz}</strong>.
            </p>
            <SchedulePicker
              value={draftSchedule}
              onChange={setDraftSchedule}
              appTimezone={tz}
              disabled={patchMutation.isPending}
              idPrefix="task-detail-schedule"
            />
            <MutationErrorBanner
              error={patchMutation.error}
              fallback="Could not update the schedule."
              className="task-detail-schedule-modal-err"
            />
            <div className="row stack-row-actions">
              <button
                type="button"
                className="secondary"
                onClick={closeEditor}
                disabled={patchMutation.isPending}
              >
                Cancel
              </button>
              <button
                type="button"
                className="task-create-submit"
                onClick={submitDraft}
                disabled={patchMutation.isPending}
              >
                {patchMutation.isPending ? "Saving…" : "Save schedule"}
              </button>
            </div>
          </section>
        </Modal>
      ) : null}
    </div>
  );
}
