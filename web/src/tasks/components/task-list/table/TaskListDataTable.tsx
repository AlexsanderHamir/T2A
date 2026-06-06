import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import type { CSSProperties } from "react";
import { useTaskDetailPrefetcher } from "@/app/hooks/usePrefetchOnIntent";
import type { Task } from "@/types";
import type { TaskWithDepth } from "../../../task-tree";
import type { DeleteTargetInput } from "../../../hooks/useTaskDeleteFlow";
import {
  priorityPillClass,
  statusNeedsUserInput,
  statusPillClass,
} from "../../../task-display";
import { TaskListDeleteGlyph, TaskListEditGlyph } from "./TaskListRowActionIcons";
import { statusListLabel, taskListRowSubtitle } from "./taskListRowSubtitle";
import { previewTextFromPrompt } from "../../../task-prompt";
import { projectBadgeToneFromId } from "../../../projectBadgeTone";
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

function isTaskListRowNavExcluded(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return true;
  return Boolean(
    target.closest("a, button, input, select, textarea, label, [role='combobox']"),
  );
}

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
   * docs/api.md "DELETE /tasks/{id}".
   */
  onRequestDelete: (t: DeleteTargetInput) => void;
  /**
   * Optional bulk-selection bindings (Stage 5 of task scheduling).
   * Omit to keep the legacy no-checkbox layout for callers that
   * don't need bulk actions.
   */
  selection?: BulkSelectionProps;
  /** Maps `task.project_id` to a label for the Project column (e.g. from `GET /projects`). */
  projectNameById?: Record<string, string>;
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
  projectNameById = {},
}: Props) {
  const navigate = useNavigate();
  // Prefetch task detail (chunk + GET /tasks/{id}) on hover/focus so
  // the click → render path has zero wait for the common case. Both
  // operations are idempotent: chunks are cached after first import
  // and React Query dedups in-flight queries.
  const prefetchTaskDetail = useTaskDetailPrefetcher();
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
  const filterExitingRef = useRef<Map<string, TaskWithDepth>>(new Map());
  const displayOrderRef = useRef<TaskWithDepth[]>([]);
  const [exitingTick, setExitingTick] = useState(0);

  const filteredIds = useMemo(
    () => new Set(filteredTasks.map((t) => t.id)),
    [filteredTasks],
  );

  const tasksIds = useMemo(() => new Set(tasks.map((t) => t.id)), [tasks]);

  const prevFilteredRef = useRef<TaskWithDepth[]>([]);

  useLayoutEffect(() => {
    const prevOrder =
      displayOrderRef.current.length > 0
        ? displayOrderRef.current
        : prevFilteredRef.current;
    const nextIds = new Set(filteredTasks.map((t) => t.id));
    let scheduledFilterExit = false;

    for (const t of prevOrder) {
      if (nextIds.has(t.id)) continue;
      if (!tasksIds.has(t.id)) continue;
      if (filterExitingRef.current.has(t.id)) continue;
      filterExitingRef.current.set(t.id, t);
      window.setTimeout(() => {
        filterExitingRef.current.delete(t.id);
        displayOrderRef.current = displayOrderRef.current.filter(
          (row) => row.id !== t.id,
        );
        seenIdsRef.current.delete(t.id);
        setExitingTick((x) => x + 1);
      }, ROW_EXIT_MS);
      scheduledFilterExit = true;
    }

    for (const t of filteredTasks) {
      filterExitingRef.current.delete(t.id);
    }

    for (const pr of prevOrder) {
      if (filteredIds.has(pr.id)) continue;
      if (tasksIds.has(pr.id)) continue;
      if (exitingRef.current.has(pr.id)) continue;
      const timeoutId = window.setTimeout(() => {
        exitingRef.current.delete(pr.id);
        seenIdsRef.current.delete(pr.id);
        setExitingTick((x) => x + 1);
      }, ROW_EXIT_MS);
      exitingRef.current.set(pr.id, { task: pr, timeoutId });
    }

    const nextOrder: TaskWithDepth[] = [];
    const filteredById = new Map(filteredTasks.map((t) => [t.id, t]));
    for (const t of prevOrder) {
      const visible = filteredById.get(t.id);
      if (visible) {
        nextOrder.push(visible);
      } else if (filterExitingRef.current.has(t.id)) {
        nextOrder.push(filterExitingRef.current.get(t.id)!);
      }
    }
    for (const t of filteredTasks) {
      if (!nextOrder.some((row) => row.id === t.id)) {
        nextOrder.push(t);
      }
    }

    displayOrderRef.current = nextOrder;
    prevFilteredRef.current = filteredTasks;
    if (scheduledFilterExit) {
      setExitingTick((x) => x + 1);
    }
  }, [filteredTasks, tasksIds, filteredIds]);

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
      filterExitingRef.current.delete(t.id);
    }

    const clientExitIds = new Set(filterExitingRef.current.keys());
    for (const id of Array.from(seenIdsRef.current)) {
      if (filteredIds.has(id)) continue;
      if (exitingRef.current.has(id)) continue;
      if (clientExitIds.has(id)) continue;
      seenIdsRef.current.delete(id);
    }

    setEnteringIds((prevEntering) => {
      if (prevEntering.size === 0 && newlyEntering.size === 0) return prevEntering;
      return newlyEntering;
    });
  }, [filteredTasks, filteredIds, tasksIds]);

  useEffect(() => {
    const exiting = exitingRef.current;
    return () => {
      for (const { timeoutId } of exiting.values()) {
        clearTimeout(timeoutId);
      }
      exiting.clear();
    };
  }, []);

  // Walk `displayOrderRef` so filter/search exits stay in place while fading.
  const rowsToRender: Array<{
    task: TaskWithDepth;
    isEntering: boolean;
    isExiting: boolean;
    isFilterExit: boolean;
  }> = [];
  const filteredMap = useMemo(
    () => new Map(filteredTasks.map((t) => [t.id, t])),
    [filteredTasks],
  );
  const filterExitIds = useMemo(
    () => new Set(filterExitingRef.current.keys()),
    // eslint-disable-next-line react-hooks/exhaustive-deps -- ref map is the source of truth
    [exitingTick, filteredTasks],
  );
  const renderOrder =
    displayOrderRef.current.length > 0
      ? displayOrderRef.current
      : filteredTasks;
  const processed = new Set<string>();
  for (const t of renderOrder) {
    const visible = filteredMap.get(t.id);
    if (visible) {
      rowsToRender.push({
        task: visible,
        isEntering: enteringIds.has(t.id),
        isExiting: false,
        isFilterExit: false,
      });
      processed.add(t.id);
      continue;
    }
    if (filterExitIds.has(t.id)) {
      const exitingTask = filterExitingRef.current.get(t.id) ?? t;
      rowsToRender.push({
        task: exitingTask,
        isEntering: false,
        isExiting: true,
        isFilterExit: true,
      });
      processed.add(t.id);
    }
  }
  for (const t of filteredTasks) {
    if (processed.has(t.id)) continue;
    rowsToRender.push({
      task: t,
      isEntering: enteringIds.has(t.id),
      isExiting: false,
      isFilterExit: false,
    });
    processed.add(t.id);
  }
  for (const { task } of exitingRef.current.values()) {
    if (processed.has(task.id)) continue;
    rowsToRender.push({
      task,
      isEntering: false,
      isExiting: true,
      isFilterExit: false,
    });
  }
  void exitingTick;

  const colSpan = selection ? 6 : 5;
  const showSelectionCol = Boolean(selection);
  return (
    <div className="table-wrap task-list-table-wrap">
      <table className="task-list-table" aria-busy={refreshing}>
        <caption className="visually-hidden">{caption}</caption>
        <colgroup>
          {showSelectionCol ? <col className="task-list-col-select" /> : null}
          <col className="task-list-col-title" />
          <col className="task-list-col-status" />
          <col className="task-list-col-priority" />
          <col className="task-list-col-project" />
          <col className="task-list-col-actions" />
        </colgroup>
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
            <th scope="col">Project</th>
            <th scope="col">Actions</th>
          </tr>
        </thead>
        <tbody className="task-list-tbody">
          {tasks.length === 0 ? (
            <tr className="task-list-empty-row">
              <td colSpan={colSpan} className="task-list-empty-cell">
                <EmptyState
                  className="empty-state--in-table empty-state--task-list-fresh"
                  title="No tasks yet"
                  description="Create your first task to get started."
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
                  description="Adjust filters or clear search."
                />
              </td>
            </tr>
          ) : (
            rowsToRender.map(({ task: t, isEntering, isExiting, isFilterExit }) => {
              const promptPreview = previewTextFromPrompt(t.initial_prompt);
              const projectLabel =
                t.project_id != null && t.project_id !== ""
                  ? projectNameById[t.project_id]
                  : undefined;
              const hasProject = Boolean(
                t.project_id != null &&
                  t.project_id !== "" &&
                  projectLabel != null &&
                  projectLabel !== "",
              );
              const titleSubtitle = taskListRowSubtitle({
                depth: t.depth,
                hasProject,
                promptPreview,
              });
              const rowSelected =
                !isExiting && selection ? selection.isSelected(t.id) : false;
              const rowClass = [
                "task-list-row",
                isEntering ? "task-list-row--enter" : "",
                isExiting ? "task-list-row--exit" : "",
                isFilterExit ? "task-list-row--filter-exit" : "",
                !isExiting ? "task-list-row--navigable" : "",
              ]
                .filter(Boolean)
                .join(" ");
              const taskHref = `/tasks/${t.id}`;
              const onIntent = isExiting
                ? undefined
                : () => prefetchTaskDetail(t.id);
              return (
                <tr
                  key={t.id}
                  className={rowClass}
                  data-selected={rowSelected ? "true" : undefined}
                  aria-hidden={isExiting ? "true" : undefined}
                  onPointerEnter={onIntent}
                  onFocus={onIntent}
                  onClick={
                    isExiting
                      ? undefined
                      : (e) => {
                          if (isTaskListRowNavExcluded(e.target)) return;
                          navigate(taskHref);
                        }
                  }
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
                      to={taskHref}
                      className={[
                        "cell-title-link",
                        "cell-title-link--cell",
                        t.depth > 0 ? "cell-title-link--tree" : "",
                      ]
                        .filter(Boolean)
                        .join(" ")}
                      aria-label={`Open task details: ${t.title}`}
                      style={
                        t.depth > 0
                          ? ({
                              "--task-list-tree-depth": String(t.depth),
                            } as CSSProperties)
                          : undefined
                      }
                    >
                      <div className="cell-title-stack">
                        <span className="cell-title-main">
                          {t.depth > 0 ? (
                            <span className="task-subtask-marker" aria-hidden>
                              └{" "}
                            </span>
                          ) : null}
                          <span className="cell-title-text cell-title-text--primary">
                            {t.title}
                          </span>
                          <span
                            className="cell-title-open-hint"
                            aria-hidden="true"
                          >
                            →
                          </span>
                        </span>
                        {titleSubtitle ? (
                          <div className="cell-title-sub">{titleSubtitle}</div>
                        ) : null}
                      </div>
                    </Link>
                  </td>
                  <td className="cell-status">
                    <span
                      className={`${statusPillClass(t.status)} cell-pill--status-table`}
                      data-needs-user={
                        statusNeedsUserInput(t.status) ? "true" : undefined
                      }
                    >
                      {statusListLabel(t.status)}
                    </span>
                  </td>
                  <td className="cell-priority">
                    <span
                      className={`${priorityPillClass(t.priority)} cell-pill--priority-table`}
                    >
                      {t.priority}
                    </span>
                  </td>
                  <td className="cell-project">
                    {projectLabel ? (
                      <span
                        className="task-list-project-badge"
                        data-tone={String(
                          projectBadgeToneFromId(t.project_id ?? ""),
                        )}
                      >
                        {projectLabel}
                      </span>
                    ) : (
                      <span className="task-list-project-empty">—</span>
                    )}
                  </td>
                  <td className="cell-actions">
                    <div className="task-list-row-actions">
                      <button
                        type="button"
                        className="task-list-icon-btn task-list-icon-btn--edit"
                        aria-label={`Edit task "${t.title}"`}
                        onClick={() => onEdit(t)}
                        disabled={saving || isExiting}
                      >
                        <TaskListEditGlyph />
                      </button>
                      <button
                        type="button"
                        className="task-list-icon-btn task-list-icon-btn--delete"
                        aria-label={`Delete task "${t.title}"`}
                        onClick={() =>
                          onRequestDelete({
                            ...t,
                            subtaskCount: t.descendantCount ?? 0,
                          })
                        }
                        disabled={saving || isExiting}
                      >
                        <TaskListDeleteGlyph />
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
