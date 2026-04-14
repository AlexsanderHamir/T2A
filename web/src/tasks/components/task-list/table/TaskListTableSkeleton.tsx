const SKELETON_ROW_COUNT = 6;

type Props = {
  /** Visually hidden caption (must match the live table for consistency). */
  caption: string;
};

export function TaskListTableSkeleton({ caption }: Props) {
  return (
    <div
      className="task-list-skeleton task-list-phase-msg"
      role="status"
      aria-busy="true"
      aria-label="Loading tasks"
    >
      <div className="table-wrap task-list-table-wrap">
        <table className="task-list-table task-list-table--skeleton">
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
          <tbody aria-hidden="true">
            {Array.from({ length: SKELETON_ROW_COUNT }, (_, i) => (
              <tr key={i} className="task-list-skeleton-row">
                <td>
                  <span className="skeleton-block skeleton-block--title" />
                </td>
                <td>
                  <span className="skeleton-block skeleton-block--pill" />
                </td>
                <td>
                  <span className="skeleton-block skeleton-block--pill skeleton-block--pill-narrow" />
                </td>
                <td>
                  <span className="skeleton-block skeleton-block--prompt" />
                </td>
                <td>
                  <div className="task-list-skeleton-actions">
                    <span className="skeleton-block skeleton-block--btn" />
                    <span className="skeleton-block skeleton-block--btn" />
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
