import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { listTasks } from "@/api";

type Props = {
  projectId: string;
};

export function ProjectTasksPanel({ projectId }: Props) {
  const projectTasks = useQuery({
    queryKey: ["tasks", "project-members", projectId],
    queryFn: ({ signal }) => listTasks(200, 0, { signal }),
    enabled: Boolean(projectId),
  });
  const memberTasks = (projectTasks.data?.tasks ?? []).filter(
    (task) => task.project_id === projectId,
  );

  return (
    <section className="task-attempt-section">
      <h3>Recent project tasks</h3>
      {projectTasks.isLoading ? (
        <p className="muted">Loading tasks...</p>
      ) : memberTasks.length === 0 ? (
        <p className="muted">No tasks are assigned to this project yet.</p>
      ) : (
        <ol className="project-task-list">
          {memberTasks.slice(0, 8).map((task) => (
            <li key={task.id}>
              <Link to={`/tasks/${encodeURIComponent(task.id)}`}>
                {task.title}
              </Link>
              <span className="muted">{task.status}</span>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
