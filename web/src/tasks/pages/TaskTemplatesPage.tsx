import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { listTaskTemplates } from "@/api";
import { TASK_TIMINGS } from "@/constants/tasks";
import { useDelayedTrue } from "@/lib/useDelayedTrue";
import { EmptyState } from "@/shared/EmptyState";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { formatRelativeTime } from "@/shared/time/relativeTime";
import { useNavigate } from "react-router-dom";
import { TaskListDeleteGlyph, TaskListEditGlyph } from "../components/task-list/table/TaskListRowActionIcons";
import { TaskDraftsListSkeleton } from "../components/skeletons";
import { useTasksAppContext } from "../app/TasksAppProvider";
import { taskQueryKeys } from "../task-query";

function isTemplateRowActionExcluded(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return true;
  return Boolean(target.closest("button, input, label"));
}

export function TaskTemplatesPage() {
  const app = useTasksAppContext();
  useDocumentTitle("Task templates");
  const navigate = useNavigate();
  const [searchInput, setSearchInput] = useState("");
  const [debouncedQ, setDebouncedQ] = useState("");
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [deletingTemplateId, setDeletingTemplateId] = useState<string | null>(null);
  const [exitingTemplateIds, setExitingTemplateIds] = useState<string[]>([]);
  const [batchError, setBatchError] = useState<string | null>(null);
  const deleteTimerRef = useRef<number | null>(null);

  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQ(searchInput.trim()), 300);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  const templatesQuery = useQuery({
    queryKey: taskQueryKeys.templates(debouncedQ ? { q: debouncedQ } : undefined),
    queryFn: ({ signal }) => listTaskTemplates({ q: debouncedQ || undefined, signal }),
  });

  const templates = templatesQuery.data ?? [];
  const loading = templatesQuery.isPending;
  const error = templatesQuery.isError
    ? templatesQuery.error instanceof Error
      ? templatesQuery.error.message
      : "Could not load templates."
    : null;
  const showSkeleton = useDelayedTrue(loading, TASK_TIMINGS.draftResumeMinLoadingMs);
  const renderNow = new Date();

  useEffect(() => {
    const ids = new Set(templates.map((t) => t.id));
    setSelectedIds((current) => current.filter((id) => ids.has(id)));
    setExitingTemplateIds((current) => current.filter((id) => ids.has(id)));
  }, [templates]);

  useEffect(() => {
    return () => {
      if (deleteTimerRef.current !== null) {
        window.clearTimeout(deleteTimerRef.current);
      }
    };
  }, []);

  const allSelected = templates.length > 0 && selectedIds.length === templates.length;
  const selectedCount = selectedIds.length;

  const toggleSelected = (id: string) => {
    setSelectedIds((current) =>
      current.includes(id) ? current.filter((value) => value !== id) : [...current, id],
    );
  };

  const toggleSelectAll = () => {
    setSelectedIds(allSelected ? [] : templates.map((t) => t.id));
  };

  const deleteTemplate = async (templateId: string) => {
    setDeletingTemplateId(templateId);
    setExitingTemplateIds((current) =>
      current.includes(templateId) ? current : [...current, templateId],
    );
    await new Promise<void>((resolve) => {
      deleteTimerRef.current = window.setTimeout(() => {
        deleteTimerRef.current = null;
        resolve();
      }, TASK_TIMINGS.draftDeleteExitMs);
    });
    try {
      await app.deleteTemplateByID(templateId);
      setSelectedIds((current) => current.filter((id) => id !== templateId));
    } catch {
      setExitingTemplateIds((current) => current.filter((id) => id !== templateId));
    } finally {
      setDeletingTemplateId((current) => (current === templateId ? null : current));
    }
  };

  const runBatchCreate = async () => {
    if (selectedIds.length === 0) return;
    setBatchError(null);
    try {
      const result = await app.instantiateTemplatesByIDs(selectedIds);
      if (result.errors.length > 0 && result.tasks.length === 0) {
        setBatchError(
          result.errors.map((entry) => `${entry.template_id}: ${entry.error}`).join(" "),
        );
        return;
      }
      if (result.errors.length > 0) {
        setBatchError(
          `Created ${result.tasks.length} task(s). Failed: ${result.errors
            .map((entry) => entry.template_id)
            .join(", ")}`,
        );
        setSelectedIds(result.errors.map((entry) => entry.template_id));
        return;
      }
      setSelectedIds([]);
      navigate("/");
    } catch (err) {
      setBatchError(err instanceof Error ? err.message : "Could not create tasks from templates.");
    }
  };

  const batchBar =
    selectedCount > 0 ? (
      <div className="template-batch-bar" role="region" aria-label="Batch actions">
        <span className="template-batch-bar__count">{selectedCount} selected</span>
        <button
          type="button"
          className="task-create-submit"
          disabled={app.instantiateTemplatesPending}
          onClick={() => void runBatchCreate()}
        >
          {app.instantiateTemplatesPending
            ? "Creating tasks…"
            : `Create tasks (${selectedCount})`}
        </button>
      </div>
    ) : null;

  return (
    <section className="panel task-list-section-panel task-detail-content--enter">
      <div className="task-list-toolbar">
        <header className="task-list-section-head">
          <div className="task-list-section-head__text">
            <h2 id="task-templates-heading" className="task-list-section-title">
              Task templates
            </h2>
          </div>
          <div className="task-list-section-actions">
            <button
              type="button"
              className="secondary"
              onClick={() => app.openTemplateCreateModal()}
            >
              New template
            </button>
          </div>
        </header>

        <div
          className="task-templates-search field grow task-list-search-field"
          role="search"
          aria-label="Search templates"
        >
          <label htmlFor="task-templates-search" className="visually-hidden">
            Search templates
          </label>
          <input
            id="task-templates-search"
            type="search"
            placeholder="Search by title…"
            autoComplete="off"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
          />
        </div>
      </div>

      {batchError ? (
        <div className="err" role="alert">
          <p>{batchError}</p>
        </div>
      ) : null}

      <div className="stack">
        {loading && showSkeleton ? <TaskDraftsListSkeleton /> : null}
        {!loading ? (
          <div className="stack task-list-content task-list-content--enter">
            {error ? (
              <div className="err" role="alert">
                <p>{error}</p>
                <div className="task-detail-error-actions">
                  <button
                    type="button"
                    className="secondary"
                    onClick={() => {
                      void templatesQuery.refetch();
                    }}
                  >
                    Try again
                  </button>
                </div>
              </div>
            ) : templates.length === 0 ? (
              <EmptyState
                title={debouncedQ ? "No matching templates" : "No templates yet"}
                description={
                  debouncedQ ? "Try a different search term." : undefined
                }
                className="empty-state--task-list-fresh"
              />
            ) : (
              <>
                <div className="template-list-toolbar">
                  <label className="template-select-all">
                    <input
                      type="checkbox"
                      checked={allSelected}
                      onChange={toggleSelectAll}
                      aria-label="Select all templates"
                    />
                    <span>Select all</span>
                  </label>
                </div>
                <ul className="draft-row-list" aria-label="Task templates">
                  {templates.map((template) => {
                    const lastEdited = template.updated_at || template.created_at;
                    const relative = formatRelativeTime(lastEdited, renderNow);
                    const isSelected = selectedIds.includes(template.id);
                    const isDeleting = deletingTemplateId === template.id;
                    const rowDisabled =
                      app.loadTemplatePending ||
                      app.deleteTemplatePending ||
                      exitingTemplateIds.includes(template.id);
                    return (
                      <li
                        key={template.id}
                        className={[
                          "draft-row",
                          "template-row",
                          isSelected ? "template-row--selected" : "",
                          rowDisabled ? "" : "draft-row--interactive",
                          exitingTemplateIds.includes(template.id) ? "draft-row--exit" : "",
                        ]
                          .filter(Boolean)
                          .join(" ")}
                        onClick={(e) => {
                          if (rowDisabled || isTemplateRowActionExcluded(e.target)) return;
                          toggleSelected(template.id);
                        }}
                        onKeyDown={(e) => {
                          if (rowDisabled || isTemplateRowActionExcluded(e.target)) return;
                          if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            toggleSelected(template.id);
                          }
                        }}
                        tabIndex={rowDisabled ? undefined : 0}
                        aria-label={`Template: ${template.name}`}
                        aria-selected={isSelected}
                      >
                        <div className="template-row__select">
                          <input
                            type="checkbox"
                            checked={isSelected}
                            aria-label={`Select ${template.name}`}
                            onChange={() => toggleSelected(template.id)}
                            onClick={(e) => e.stopPropagation()}
                          />
                        </div>
                        <div className="draft-row__meta">
                          <span className="draft-row__name" title={template.name}>
                            {template.name}
                          </span>
                          {lastEdited && relative ? (
                            <time
                              className="draft-row__time"
                              dateTime={lastEdited}
                              title={lastEdited}
                            >
                              Updated {relative}
                            </time>
                          ) : null}
                        </div>
                        <div className="draft-row__actions">
                          <div className="task-list-row-actions">
                            <button
                              type="button"
                              className="task-list-icon-btn task-list-icon-btn--edit"
                              aria-label={`Edit template "${template.name}"`}
                              onClick={(e) => {
                                e.stopPropagation();
                                void app.editTemplateByID(template.id);
                              }}
                              disabled={rowDisabled}
                            >
                              <TaskListEditGlyph />
                            </button>
                            <button
                              type="button"
                              className="task-list-icon-btn task-list-icon-btn--delete"
                              aria-label={
                                isDeleting
                                  ? `Deleting template "${template.name}"`
                                  : `Delete template "${template.name}"`
                              }
                              onClick={(e) => {
                                e.stopPropagation();
                                void deleteTemplate(template.id);
                              }}
                              disabled={rowDisabled}
                              aria-busy={isDeleting || undefined}
                            >
                              <TaskListDeleteGlyph />
                            </button>
                          </div>
                        </div>
                      </li>
                    );
                  })}
                </ul>
              </>
            )}
          </div>
        ) : null}
      </div>
      {batchBar}
    </section>
  );
}
