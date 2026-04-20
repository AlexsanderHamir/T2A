import { useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import type { CSSProperties } from "react";
import type { Task } from "@/types";
import type { TaskWithDepth } from "../../../task-tree";
import type { DeleteTargetInput } from "../../../hooks/useTaskDeleteFlow";
import {
  priorityPillClass,
  statusNeedsUserInput,
  statusPillClass,
} from "../../../task-display";
import { previewTextFromPrompt } from "../../../task-prompt";
import {
  EmptyState,
  EmptyStateFilterGlyph,
  type EmptyStateAction,
} from "@/shared/EmptyState";

/**
 * Matches the `task-list-row-fade-out` keyframe duration in
 * app-task-list-and-mentions.css (--duration-normal ≈ 200ms).
 * A hair longer than --duration-normal so we don't yank the row
 * off in the final frame. Kept in JS so the cleanup timer doesn't
 * fight the CSS fallback for users whose onAnimationEnd never
 * fires (e.g. tab backgrounded mid-exit).
 */
const ROW_EXIT_MS = 220;

/**
 * Optional bulk-selection bindings. When omitted, the table renders
 * without the leftmost checkbox column (callers that don't want
 * bulk actions — e.g. embedded subtask widgets — get the
 * historical layout for free). When provided, the table renders
 * a header tri-state checkbox plus a per-row checkbox column;
 * the parent owns the state via `useTaskListSelection`.
 */
type BulkSelectionProps = {
  isSelected: (id: string) => boolean;
  onRowToggle: (id: string) => void;
  allVisibleSelected: boolean;
  someVisibleSelected: boolean;
  onToggleAllVisible: () => void;
};

type Props = {
  caption: string;
  /** Reflects background refetch while the table stays visible. */
  refreshing: boolean;
  tasks: TaskWithDepth[];
  filteredTasks: TaskWithDepth[];
  saving: boolean;
  emptyListAction?: EmptyStateAction;
  onEdit: (t: Task) => void;
  /**
   * Receives the table row plus the pre-computed `subtaskCount` carried by
   * `TaskWithDepth.descendantCount`. Forwarded to `useTaskDeleteFlow` so the
   * confirm dialog can warn the user about the cascade documented in
   * docs/API-HTTP.md "DELETE /tasks/{id}".
   */
  onRequestDelete: (t: DeleteTargetInput) => void;
  /**
   * Optional bulk-selection bindings (Stage 5 of task scheduling).
   * Omit to keep the legacy no-checkbox layout for callers that
   * don't need bulk actions.
   */
  selection?: BulkSelectionProps;
};

export function TaskListDataTable({
  caption,
  refreshing,
  tasks,
  filteredTasks,
  saving,
  emptyListAction,
  onEdit,
  onRequestDelete,
  selection,
}: Props) {
  const headerCheckboxRef = useRef<HTMLInputElement | null>(null);

  // Drive the header checkbox's `indeterminate` flag — it isn't a
  // settable HTML attribute, so we have to poke the DOM property
  // ourselves whenever the upstream tri-state changes. (Standard
  // React idiom; same shape used everywhere else a tri-state
  // checkbox lives in the codebase.)
  useEffect(() => {
    if (!selection || !headerCheckboxRef.current) return;
    headerCheckboxRef.current.indeterminate = selection.someVisibleSelected;
  }, [selection]);

  // Phase 3b: track ids we've already rendered so only newly-inserted
  // rows animate in. A `useRef<Set<string>>` is used rather than state
  // because we never want this to cause a re-render — it's a passive
  // filter on top of React's own reconciliation. `enteringIds` is the
  // mirror-state we compare against on render.
  const seenIdsRef = useRef<Set<string>>(new Set());
  const [enteringIds, setEnteringIds] = useState<Set<string>>(new Set());

  // Phase 3c: track ids that just left `filteredTasks` so we can keep
  // them mounted for ROW_EXIT_MS with the --exit class. The cached
  // TaskWithDepth is pinned at the moment the id left so the row
  // renders with its last-known data while it fades out (otherwise
  // we'd have to look it back up in `tasks` and risk stale-column
  // data races).
  type ExitingRow = { task: TaskWithDepth; timeoutId: number };
  const exitingRef = useRef<Map<string, ExitingRow>>(new Map());
  const [exitingTick, setExitingTick] = useState(0);

  const filteredIds = useMemo(
    () => new Set(filteredTasks.map((t) => t.id)),
    [filteredTasks],
  );

  const prevFilteredRef = useRef<TaskWithDepth[]>([]);
  useEffect(() => {
    const newlyEntering = new Set<string>();
    for (const t of filteredTasks) {
      if (!seenIdsRef.current.has(t.id)) {
        newlyEntering.add(t.id);
        seenIdsRef.current.add(t.id);
      }
      // If a row was in `exitingRef` (e.g. filter re-admitted an id
      // that had just been removed) cancel its timeout and revive it.
      const pendingExit = exitingRef.current.get(t.id);
      if (pendingExit) {
        clearTimeout(pendingExit.timeoutId);
        exitingRef.current.delete(t.id);
      }
    }

    // Schedule exit animations for ids that truly left the source
    // data (delete / optimistic delete / SSE task_deleted), NOT ids
    // that were merely filtered out by the status / priority / title
    // chips. Filter-out should snap because (a) it's a user-initiated
    // view change where instant feedback is expected, and (b) fading
    // the whole filtered-out set would look like a long transition
    // every time a chip changes. The discriminator is whether the id
    // still exists in the unfiltered `tasks` prop.
    const tasksIds = new Set(tasks.map((t) => t.id));
    const prev = prevFilteredRef.current;
    let scheduledExit = false;
    for (const pr of prev) {
      if (filteredIds.has(pr.id)) continue;
      if (tasksIds.has(pr.id)) continue; // still in source; filter-out
      if (exitingRef.current.has(pr.id)) continue;
      const timeoutId = window.setTimeout(() => {
        exitingRef.current.delete(pr.id);
        seenIdsRef.current.delete(pr.id);
        setExitingTick((x) => x + 1);
      }, ROW_EXIT_MS);
      exitingRef.current.set(pr.id, { task: pr, timeoutId });
      scheduledExit = true;
    }
    // Drop ids from seenIds that are no longer visible AND aren't
    // exiting, so when a filter chip is cleared they re-enter with
    // an animation rather than snapping in silently. Exiting ids
    // are removed by their own timeout cleanup above.
    for (const id of Array.from(seenIdsRef.current)) {
      if (filteredIds.has(id)) continue;
      if (exitingRef.current.has(id)) continue;
      seenIdsRef.current.delete(id);
    }
    prevFilteredRef.current = filteredTasks;

    setEnteringIds((prev) => {
      if (prev.size === 0 && newlyEntering.size === 0) return prev;
      return newlyEntering;
    });
    if (scheduledExit) {
      setExitingTick((x) => x + 1);
    }
  }, [filteredTasks, filteredIds, tasks]);

  useEffect(() => {
    const exiting = exitingRef.current;
    return () => {
      for (const { timeoutId } of exiting.values()) {
        clearTimeout(timeoutId);
      }
      exiting.clear();
    };
  }, []);

  // Merge filteredTasks with exiting rows for render. Exiting rows are
  // appended at the bottom rather than their old position: the CSS
  // fade-out with translateY(8px) reads naturally that way and we
  // don't have to preserve the original interleaving across a tree
  // flatten.
  const rowsToRender: Array<{
    task: TaskWithDepth;
    isEntering: boolean;
    isExiting: boolean;
  }> = [];
  for (const t of filteredTasks) {
    rowsToRender.push({
      task: t,
      isEntering: enteringIds.has(t.id),
      isExiting: false,
    });
  }
  for (const { task } of exitingRef.current.values()) {
    rowsToRender.push({ task, isEntering: false, isExiting: true });
  }
  void exitingTick;

  const colSpan = selection ? 6 : 5;
  const showSelectionCol = Boolean(selection);
  return (
    <div className="table-wrap task-list-table-wrap">
      <table className="task-list-table" aria-busy={refreshing}>
        <caption className="visually-hidden">{caption}</caption>
        <thead>
          <tr>
            {showSelectionCol && selection ? (
              <th scope="col" className="task-list-select-col">
                <input
                  ref={headerCheckboxRef}
                  type="checkbox"
                  className="task-list-select-checkbox"
                  aria-label={
                    selection.allVisibleSelected
                      ? "Deselect all visible tasks"
                      : "Select all visible tasks"
                  }
                  checked={selection.allVisibleSelected}
                  onChange={selection.onToggleAllVisible}
                  data-testid="task-list-select-all"
                  disabled={filteredTasks.length === 0}
                />
              </th>
            ) : null}
            <th scope="col">Title</th>
            <th scope="col">Status</th>
            <th scope="col">Priority</th>
            <th scope="col">Prompt</th>
            <th scope="col">Actions</th>
          </tr>
        </thead>
        <tbody className="task-list-tbody">
          {tasks.length === 0 ? (
            <tr className="task-list-empty-row">
              <td colSpan={colSpan} className="task-list-empty-cell">
                <EmptyState
                  className="empty-state--in-table"
                  title="No tasks yet"
                  description={
                    <>
                      Use <strong>New task</strong> above to add your first task.
                      Status, priority, and prompt previews appear here.
                    </>
                  }
                  action={emptyListAction}
                />
              </td>
            </tr>
          ) : rowsToRender.length === 0 ? (
            <tr className="task-list-empty-row">
              <td colSpan={colSpan} className="task-list-empty-cell">
                <EmptyState
                  className="empty-state--in-table"
                  icon={<EmptyStateFilterGlyph />}
                  title="No matching tasks"
                  description="Try a different status or priority, or clear the title search."
                />
              </td>
            </tr>
          ) : (
            rowsToRender.map(({ task: t, isEntering, isExiting }) => {
              const promptPreview = previewTextFromPrompt(t.initial_prompt);
              const rowSelected =
                !isExiting && selection ? selection.isSelected(t.id) : false;
              const rowClass = [
                "task-list-row",
                isEntering ? "task-list-row--enter" : "",
                isExiting ? "task-list-row--exit" : "",
              ]
                .filter(Boolean)
                .join(" ");
              return (
                <tr
                  key={t.id}
                  className={rowClass}
                  data-selected={rowSelected ? "true" : undefined}
                  aria-hidden={isExiting ? "true" : undefined}
                >
                  {showSelectionCol && selection ? (
                    <td className="task-list-select-col">
                      <input
                        type="checkbox"
                        className="task-list-select-checkbox"
                        aria-label={
                          rowSelected
                            ? `Deselect task "${t.title}"`
                            : `Select task "${t.title}"`
                        }
                        checked={rowSelected}
                        onChange={() => selection.onRowToggle(t.id)}
                        data-testid={`task-list-select-row-${t.id}`}
                        disabled={isExiting}
                      />
                    </td>
                  ) : null}
                  <td className="cell-title">
                    <Link
                      to={`/tasks/${t.id}`}
                      className={
                        t.depth > 0
                          ? "cell-title-link cell-title-link--tree"
                          : "cell-title-link"
                      }
                      aria-label={`Open task details: ${t.title}`}
                      style={
                        t.depth > 0
                          ? ({
                              "--task-list-tree-depth": String(t.depth),
                            } as CSSProperties)
                          : undefined
                      }
                    >
                      {t.depth > 0 ? (
                        <span className="task-subtask-marker" aria-hidden>
                          └{" "}
                        </span>
                      ) : null}
                      <span className="cell-title-text">{t.title}</span>
                      <span
                        className="cell-title-open-hint"
                        aria-hidden="true"
                      >
                        →
                      </span>
                    </Link>
                  </td>
                  <td>
                    <span
                      className={statusPillClass(t.status)}
                      data-needs-user={
                        statusNeedsUserInput(t.status) ? "true" : undefined
                      }
                    >
                      {t.status}
                    </span>
                  </td>
                  <td>
                    <span className={priorityPillClass(t.priority)}>
                      {t.priority}
                    </span>
                  </td>
                  <td>
                    <div className="prompt-preview" title={promptPreview}>
                      {promptPreview || "—"}
                    </div>
                  </td>
                  <td>
                    <div className="actions">
                      <button
                        type="button"
                        className="secondary btn-table"
                        aria-label={`Edit task "${t.title}"`}
                        onClick={() => onEdit(t)}
                        disabled={saving || isExiting}
                      >
                        Edit
                      </button>
                      <button
                        type="button"
                        className="danger btn-table"
                        aria-label={`Delete task "${t.title}"`}
                        onClick={() =>
                          onRequestDelete({
                            ...t,
                            subtaskCount: t.descendantCount ?? 0,
                          })
                        }
                        disabled={saving || isExiting}
                      >
                        Delete
                      </button>
                    </div>
                  </td>
                </tr>
              );
            })
          )}
        </tbody>
      </table>
    </div>
  );
}
