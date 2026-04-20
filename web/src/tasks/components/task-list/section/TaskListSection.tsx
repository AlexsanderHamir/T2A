import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useDelayedTrue } from "@/lib/useDelayedTrue";
import { TaskListDataTable } from "../table/TaskListDataTable";
import { TaskListFilters } from "../filters/TaskListFilters";
import { TaskListSectionHeading } from "./TaskListSectionHeading";
import { TaskPager } from "../pager/TaskPager";
import type { Task } from "@/types";
import type { TaskWithDepth } from "../../../task-tree";
import type { DeleteTargetInput } from "../../../hooks/useTaskDeleteFlow";
import type { EmptyStateAction } from "@/shared/EmptyState";
import {
  filterTasksForListView,
  type TaskListClientPriorityFilter,
  type TaskListClientStatusFilter,
} from "../filters/taskListClientFilter";
import { taskListPagerSummary } from "../pager/taskListPagerSummary";
import { TaskListTableSkeleton } from "../table/TaskListTableSkeleton";
import { useAppTimezone } from "@/shared/time/appTimezone";
import {
  TaskBulkRescheduleModal,
  TaskListBulkActionBar,
  useBulkScheduleMutation,
  useTaskListSelection,
} from "../bulk";

type Props = {
  tasks: TaskWithDepth[];
  /** Root tasks returned for this list page (for pager copy; rows may include nested subtasks). */
  rootTasksOnPage: number;
  loading: boolean;
  /** Background refetch in progress (list still visible). */
  refreshing: boolean;
  /** A create/update/delete request is in flight. */
  saving: boolean;
  /**
   * When true, hide the background “Syncing with server…” line (e.g. live SSE
   * already drives refetches; avoids duplicate status with the header).
   */
  hideBackgroundRefreshHint?: boolean;
  /** Zero-based server list page (see `GET /tasks` offset). */
  listPage: number;
  listPageSize: number;
  onListPageChange: (page: number) => void;
  /** Reset to first server page when filters change. */
  onListFiltersChange: () => void;
  hasNextPage: boolean;
  hasPrevPage: boolean;
  /**
   * When true (default), the loading line waits briefly before appearing. Set false in tests.
   * List “syncing” is smoothed in `useTasksApp` (hysteresis on refetch).
   */
  smoothTransitions?: boolean;
  onEdit: (t: Task) => void;
  /**
   * Opens in-app delete confirmation (do not call `window.confirm` from the
   * table). The table forwards the row's pre-computed `descendantCount` via
   * `subtaskCount` so the confirm dialog can warn about the cascade — see
   * docs/API-HTTP.md "DELETE /tasks/{id}".
   */
  onRequestDelete: (t: DeleteTargetInput) => void;
  /** Primary action when the server returned no tasks (e.g. open create modal). */
  emptyListAction?: EmptyStateAction;
  /** Optional toolbar on the title row (e.g. home “New task”). */
  actions?: ReactNode;
};

const LOADING_STATUS_DELAY_MS = 220;

const TASK_LIST_TABLE_CAPTION =
  "All tasks: title, status, priority, prompt preview, and row actions.";

export function TaskListSection({
  tasks,
  rootTasksOnPage,
  loading,
  refreshing,
  saving,
  hideBackgroundRefreshHint = false,
  listPage,
  listPageSize,
  onListPageChange,
  onListFiltersChange,
  hasNextPage,
  hasPrevPage,
  smoothTransitions = true,
  onEdit,
  onRequestDelete,
  emptyListAction,
  actions,
}: Props) {
  const statusDelayMs = smoothTransitions ? LOADING_STATUS_DELAY_MS : 0;
  const showLoadingLine = useDelayedTrue(loading, statusDelayMs);

  const [statusFilter, setStatusFilter] =
    useState<TaskListClientStatusFilter>("all");
  const [priorityFilter, setPriorityFilter] =
    useState<TaskListClientPriorityFilter>("all");
  const [titleSearch, setTitleSearch] = useState("");

  const filteredTasks = useMemo(
    () =>
      filterTasksForListView(
        tasks,
        statusFilter,
        priorityFilter,
        titleSearch,
      ),
    [tasks, statusFilter, priorityFilter, titleSearch],
  );

  const visibleIds = useMemo(
    () => filteredTasks.map((t) => t.id),
    [filteredTasks],
  );
  const selection = useTaskListSelection(visibleIds);
  const appTimezone = useAppTimezone();
  const bulkSchedule = useBulkScheduleMutation();
  const [rescheduleModalOpen, setRescheduleModalOpen] = useState(false);
  const [bulkErrorBanner, setBulkErrorBanner] = useState<string | null>(null);

  const selectedScheduledIds = useMemo(() => {
    const visibleSelected = new Set(selection.selectedVisibleIds);
    return filteredTasks
      .filter(
        (t) =>
          visibleSelected.has(t.id) && Boolean(t.pickup_not_before),
      )
      .map((t) => t.id);
  }, [filteredTasks, selection.selectedVisibleIds]);

  const skipFiltersResetOnMount = useRef(true);
  // Pull `clearSelection` out of `selection` so the filter-reset
  // effect's dependency array doesn't include the whole selection
  // object (it's a fresh reference on every render — see
  // useTaskListSelection — and depending on it would re-run the
  // effect after every state update, which would *clear the
  // running selection on every checkbox toggle*. The hook stabilises
  // `clearSelection` via useCallback so this is safe.)
  const { clearSelection } = selection;
  useEffect(() => {
    if (skipFiltersResetOnMount.current) {
      skipFiltersResetOnMount.current = false;
      return;
    }
    onListFiltersChange();
    // Per the locked plan: "Selection state clears on filter
    // change, sort change, or successful bulk action — preventing
    // the classic 'I selected 12, applied filter, now Apply to
    // selection targets things I cant see'".
    clearSelection();
  }, [
    statusFilter,
    priorityFilter,
    titleSearch,
    onListFiltersChange,
    clearSelection,
  ]);

  const closeReschedule = useCallback(() => {
    setRescheduleModalOpen(false);
    bulkSchedule.reset();
  }, [bulkSchedule]);

  const handleRescheduleSubmit = useCallback(
    async (next: string | null) => {
      const ids = selection.selectedVisibleIds;
      if (ids.length === 0) {
        setRescheduleModalOpen(false);
        return;
      }
      const result = await bulkSchedule.run(ids, next);
      if (result.failed.length === 0) {
        setRescheduleModalOpen(false);
        selection.clearSelection();
        setBulkErrorBanner(null);
      } else {
        setBulkErrorBanner(formatBulkFailure(result.failed.length, result.attempted));
      }
    },
    [bulkSchedule, selection],
  );

  const handleClearSchedule = useCallback(async () => {
    const ids = selectedScheduledIds;
    if (ids.length === 0) return;
    if (ids.length > 5) {
      const ok = window.confirm(
        `Clear scheduled pickup on ${ids.length} tasks? They will be eligible for the agent immediately.`,
      );
      if (!ok) return;
    }
    const result = await bulkSchedule.run(ids, null);
    if (result.failed.length === 0) {
      selection.clearSelection();
      setBulkErrorBanner(null);
    } else {
      setBulkErrorBanner(formatBulkFailure(result.failed.length, result.attempted));
    }
  }, [bulkSchedule, selectedScheduledIds, selection]);

  const handleCancelSelection = useCallback(() => {
    selection.clearSelection();
    setBulkErrorBanner(null);
  }, [selection]);

  const showTaskPager =
    !loading && (hasPrevPage || hasNextPage || tasks.length === listPageSize);

  return (
    <section className="panel" aria-labelledby="task-list-heading">
      <TaskListSectionHeading actions={actions} />
      {refreshing && !loading && !hideBackgroundRefreshHint ? (
        <p className="sync-hint task-list-phase-msg" aria-live="polite" role="status">
          Syncing with server…
        </p>
      ) : null}
      {loading && showLoadingLine ? (
        <TaskListTableSkeleton caption={TASK_LIST_TABLE_CAPTION} />
      ) : null}
      {!loading ? (
        <div className="task-list-content task-list-content--enter">
          <TaskListFilters
            statusFilter={statusFilter}
            onStatusFilterChange={(v) =>
              setStatusFilter(v as TaskListClientStatusFilter)
            }
            priorityFilter={priorityFilter}
            onPriorityFilterChange={(v) =>
              setPriorityFilter(v as TaskListClientPriorityFilter)
            }
            titleSearch={titleSearch}
            onTitleSearchChange={setTitleSearch}
          />
          <TaskListDataTable
            caption={TASK_LIST_TABLE_CAPTION}
            refreshing={refreshing}
            tasks={tasks}
            filteredTasks={filteredTasks}
            saving={saving}
            emptyListAction={emptyListAction}
            onEdit={onEdit}
            onRequestDelete={onRequestDelete}
            selection={{
              isSelected: selection.isSelected,
              onRowToggle: selection.toggle,
              allVisibleSelected: selection.allVisibleSelected,
              someVisibleSelected: selection.someVisibleSelected,
              onToggleAllVisible: selection.toggleAllVisible,
            }}
          />
          {bulkErrorBanner ? (
            <p
              className="err task-list-bulk-error"
              role="alert"
              data-testid="task-list-bulk-error"
            >
              {bulkErrorBanner}
            </p>
          ) : null}
          {showTaskPager ? (
            <TaskPager
              navLabel="Task list pages"
              summary={taskListPagerSummary({
                tasksLength: tasks.length,
                listPage,
                listPageSize,
                rootTasksOnPage,
                hasNextPage,
              })}
              onPrev={() => onListPageChange(listPage - 1)}
              onNext={() => onListPageChange(listPage + 1)}
              disablePrev={!hasPrevPage}
              disableNext={!hasNextPage}
            />
          ) : null}
        </div>
      ) : null}
      <TaskListBulkActionBar
        selectedCount={selection.selectedVisibleIds.length}
        scheduledCount={selectedScheduledIds.length}
        busy={bulkSchedule.isPending}
        onReschedule={() => {
          setBulkErrorBanner(null);
          setRescheduleModalOpen(true);
        }}
        onClearSchedule={handleClearSchedule}
        onCancel={handleCancelSelection}
      />
      {rescheduleModalOpen ? (
        <TaskBulkRescheduleModal
          selectedCount={selection.selectedVisibleIds.length}
          appTimezone={appTimezone}
          busy={bulkSchedule.isPending}
          error={bulkErrorBanner}
          onClose={closeReschedule}
          onSubmit={handleRescheduleSubmit}
        />
      ) : null}
    </section>
  );
}

function formatBulkFailure(failedCount: number, attempted: number): string {
  return `${failedCount} of ${attempted} reschedules failed. The successful ones already updated; the failed rows kept their previous schedule. Try again or check the task detail pages for details.`;
}
