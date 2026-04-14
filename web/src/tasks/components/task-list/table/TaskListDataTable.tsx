import { Link } from "react-router-dom";
import type { CSSProperties } from "react";
import type { Task } from "@/types";
import type { TaskWithDepth } from "../../../task-tree";
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

type Props = {
  caption: string;
  /** Reflects background refetch while the table stays visible. */
  refreshing: boolean;
  tasks: TaskWithDepth[];
  filteredTasks: TaskWithDepth[];
  saving: boolean;
  emptyListAction?: EmptyStateAction;
  onEdit: (t: Task) => void;
  onRequestDelete: (t: Task) => void;
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
}: Props) {
  return (
    <div className="table-wrap task-list-table-wrap">
      <table className="task-list-table" aria-busy={refreshing}>
        <caption className="visually-hidden">{caption}</caption>
        <thead>
          <tr>
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
              <td colSpan={5} className="task-list-empty-cell">
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
              <td colSpan={5} className="task-list-empty-cell">
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
              return (
                <tr key={t.id} className="task-list-row">
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
                        onClick={() => onRequestDelete(t)}
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
