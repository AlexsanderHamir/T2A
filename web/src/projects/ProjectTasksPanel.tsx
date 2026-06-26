import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { listTasks } from "@/api";
import { taskQueryKeys } from "@/lib/taskQueryKeys";

type Props = {
  projectId: string;
};

export function ProjectTasksPanel({ projectId }: Props) {
  const projectTasks = useQuery({
    queryKey: taskQueryKeys.list({ limit: 200, offset: 0 }),
    queryFn: ({ signal }) => listTasks(200, 0, { signal }),
    enabled: Boolean(projectId),
  });

  const memberTasks = (projectTasks.data?.tasks ?? []).filter(
    (task) => task.project_id === projectId,
  );

  return (
    <section className="pd__card" aria-labelledby="pd-tasks-title">
      <h2 id="pd-tasks-title" className="pd__card-eyebrow">
        Linked tasks
      </h2>

      {projectTasks.isLoading ? <TaskListSkeleton /> : null}

      {!projectTasks.isLoading && memberTasks.length === 0 ? (
        <p className="pd__empty">No tasks linked to this project yet</p>
      ) : null}

      {memberTasks.length > 0 ? (
        <ul className="pd__task-list">
          {memberTasks.slice(0, 12).map((task) => (
            <li key={task.id}>
              <Link
                to={`/tasks/${encodeURIComponent(task.id)}`}
                className="pd__task-row"
              >
                <span className="pd__task-title">{task.title}</span>
                <span className="pd__task-chip">{task.status}</span>
              </Link>
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}

function TaskListSkeleton() {
  return (
    <div className="pd__task-skeleton" aria-hidden="true">
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="pd__task-skeleton-row">
          <span className="pd__shimmer" style={{ width: `${58 - i * 12}%`, height: "0.875rem" }} />
          <span className="pd__shimmer" style={{ width: "3.5rem", height: "0.75rem" }} />
        </div>
      ))}
    </div>
  );
}
