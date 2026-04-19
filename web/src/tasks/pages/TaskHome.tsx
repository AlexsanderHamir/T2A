import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { DraftResumeModal } from "../components/draft-resume";
import { TaskCreateModal } from "../components/task-create-modal";
import { TaskListSection } from "../components/task-list";
import { useTasksApp } from "../hooks/useTasksApp";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

type KpiState =
  | { kind: "loading" }
  | { kind: "unavailable" }
  | { kind: "ready"; value: number };

/**
 * KPI cards must reflect the **whole** task table, not the (paged) `tasks`
 * array. While `taskStats` is loading we show a skeleton instead of a
 * misleading approximation derived from the current page; if stats settled
 * to `null` (server error) we show "—" with a "Stats unavailable" hint.
 */
function kpiState(
  raw: number | undefined,
  loading: boolean,
  hasStats: boolean,
): KpiState {
  if (typeof raw === "number") return { kind: "ready", value: raw };
  if (loading || !hasStats) return loading ? { kind: "loading" } : { kind: "unavailable" };
  return { kind: "unavailable" };
}

function KpiValue({ state, label }: { state: KpiState; label: string }) {
  if (state.kind === "ready") {
    return <p className="task-home-kpi-value">{state.value}</p>;
  }
  if (state.kind === "loading") {
    return (
      <p className="task-home-kpi-value" aria-hidden="true">
        <span className="skeleton-block skeleton-block--kpi-value" />
        <span className="visually-hidden">Loading {label}</span>
      </p>
    );
  }
  return (
    <p className="task-home-kpi-value task-home-kpi-value--unavailable" aria-label={`${label} unavailable`}>
      —
    </p>
  );
}

export function TaskHome({ app }: Props) {
  useDocumentTitle(undefined);
  const handleResumeDraft = (id: string) => {
    void app.resumeDraftByID(id).catch(() => {
      // Error state is exposed by the hook and rendered in the modal.
    });
  };
  const stats = app.taskStats ?? null;
  const statsLoading = app.taskStatsLoading;
  const hasStats = stats !== null;

  const totalState = kpiState(stats?.total, statsLoading, hasStats);
  const readyState = kpiState(
    stats?.by_status.ready ?? stats?.ready,
    statsLoading,
    hasStats,
  );
  const criticalState = kpiState(
    stats?.by_priority.critical ?? stats?.critical,
    statsLoading,
    hasStats,
  );
  const parentTasks = stats?.by_scope.parent;
  const subtaskTasks = stats?.by_scope.subtask;

  return (
    <>
      {app.createEntryDraftErrorHint ? (
        <div className="err error-banner" role="alert">
          <span className="error-banner__text">
            Saved drafts are unavailable right now, so a fresh task form was opened.
          </span>
          <button
            type="button"
            className="secondary"
            onClick={() => {
              void app.retryCreateEntryDraftLoad();
            }}
          >
            Retry loading drafts
          </button>
        </div>
      ) : null}
      {app.createModalOpen ? (
        <TaskCreateModal
          pending={app.createPending}
          saving={app.saving}
          draftSaving={app.draftSavePending}
          draftSaveLabel={app.draftSaveLabel}
          draftSaveError={app.draftSaveError}
          onClose={app.closeCreateModal}
          title={app.newTitle}
          prompt={app.newPrompt}
          priority={app.newPriority}
          taskType={app.newTaskType}
          checklistItems={app.newChecklistItems}
          onTitleChange={app.setNewTitle}
          onPromptChange={app.setNewPrompt}
          onPriorityChange={app.setNewPriority}
          onTaskTypeChange={app.setNewTaskType}
          onAppendChecklistCriterion={app.appendNewChecklistCriterion}
          onUpdateChecklistRow={app.updateNewChecklistRow}
          onRemoveChecklistRow={app.removeNewChecklistRow}
          pendingSubtasks={app.pendingSubtasks}
          onAddPendingSubtask={app.addPendingSubtask}
          onUpdatePendingSubtask={app.updatePendingSubtask}
          onRemovePendingSubtask={app.removePendingSubtask}
          evaluatePending={app.evaluatePending}
          evaluation={app.latestDraftEvaluation}
          dmapCommitLimit={app.newDmapCommitLimit}
          dmapDomain={app.newDmapDomain}
          dmapDescription={app.newDmapDescription}
          onDmapCommitLimitChange={app.setNewDmapCommitLimit}
          onDmapDomainChange={app.setNewDmapDomain}
          onDmapDescriptionChange={app.setNewDmapDescription}
          taskRunner={app.newTaskRunner}
          taskCursorModel={app.newTaskCursorModel}
          onTaskRunnerChange={app.setNewTaskRunner}
          onTaskCursorModelChange={app.setNewTaskCursorModel}
          onSaveDraft={() => void app.saveDraftNow()}
          onEvaluate={() => void app.evaluateDraftBeforeCreate()}
          onSubmit={(e) => void app.submitCreate(e)}
          createError={app.createError}
          evaluateError={app.evaluateError}
        />
      ) : null}
      {app.draftPickerOpen ? (
        <DraftResumeModal
          drafts={app.taskDrafts}
          onClose={() => app.setDraftPickerOpen(false)}
          onStartFresh={() => void app.startFreshDraft()}
          onResume={handleResumeDraft}
          loading={app.draftListLoading}
          loadError={app.draftListError}
          onRetryLoad={() => {
            void app.retryDraftList();
          }}
          resumePending={app.resumeDraftPending}
          resumeError={app.resumeDraftError}
        />
      ) : null}

      <div className="task-detail-content--enter">
        <section className="task-home-overview" aria-label="Task overview">
          <article
            className="task-home-kpi-card task-home-kpi-card--total"
            aria-busy={totalState.kind === "loading"}
          >
            <p className="task-home-kpi-label">Total tasks</p>
            <KpiValue state={totalState} label="Total tasks" />
            <p className="task-home-kpi-meta">
              {hasStats && typeof parentTasks === "number" && typeof subtaskTasks === "number"
                ? `${parentTasks} parent • ${subtaskTasks} subtask${subtaskTasks === 1 ? "" : "s"}`
                : statsLoading
                  ? "Loading breakdown…"
                  : "Breakdown unavailable"}
            </p>
          </article>
          <article
            className="task-home-kpi-card task-home-kpi-card--ready"
            aria-busy={readyState.kind === "loading"}
          >
            <p className="task-home-kpi-label">Ready tasks</p>
            <KpiValue state={readyState} label="Ready tasks" />
            <p className="task-home-kpi-meta">ready for agent pickup</p>
          </article>
          <article
            className="task-home-kpi-card task-home-kpi-card--attention"
            aria-busy={criticalState.kind === "loading"}
          >
            <p className="task-home-kpi-label">Critical</p>
            <KpiValue state={criticalState} label="Critical tasks" />
            <p className="task-home-kpi-meta">needs attention</p>
          </article>
        </section>

        <TaskListSection
          actions={
            <button
              type="button"
              className="task-home-new-task-btn"
              onClick={app.openCreateModal}
              disabled={app.createModalOpen}
            >
              New task
            </button>
          }
          tasks={app.tasks}
          rootTasksOnPage={app.rootTasksOnPage}
          loading={app.loading}
          refreshing={app.listRefreshing}
          saving={app.saving}
          hideBackgroundRefreshHint={app.sseLive}
          listPage={app.taskListPage}
          listPageSize={app.taskListPageSize}
          onListPageChange={app.setTaskListPage}
          onListFiltersChange={app.resetTaskListPage}
          hasNextPage={app.hasNextTaskPage}
          hasPrevPage={app.hasPrevTaskPage}
          onEdit={app.openEdit}
          onRequestDelete={app.requestDelete}
        />
      </div>
    </>
  );
}
