type PagerSummaryInput = {
  tasksLength: number;
  listPage: number;
  listPageSize: number;
  rootTasksOnPage: number;
  hasNextPage: boolean;
};

/** Human-readable range line for `TaskPager` on the task list. */
export function taskListPagerSummary({
  tasksLength,
  listPage,
  listPageSize,
  rootTasksOnPage,
  hasNextPage,
}: PagerSummaryInput): string {
  if (tasksLength === 0) {
    return `Page ${listPage + 1} (no tasks on this page)`;
  }
  const start = listPage * listPageSize + 1;
  const end = listPage * listPageSize + rootTasksOnPage;
  return `${start}–${end}${hasNextPage ? "+" : ""}`;
}
