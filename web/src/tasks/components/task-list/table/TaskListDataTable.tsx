import { useEffect, useRef } from "react";
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
          ) : filteredTasks.length === 0 ? (
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
            filteredTasks.map((t) => {
              const promptPreview = previewTextFromPrompt(t.initial_prompt);
              const rowSelected = selection ? selection.isSelected(t.id) : false;
              return (
                <tr
                  key={t.id}
                  className="task-list-row"
                  data-selected={rowSelected ? "true" : undefined}
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
                        disabled={saving}
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
                        disabled={saving}
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
