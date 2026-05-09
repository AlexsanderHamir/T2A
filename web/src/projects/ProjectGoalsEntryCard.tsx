import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { listProjectGoals } from "@/api";
import { projectQueryKeys } from "./queryKeys";

type Props = {
  projectId: string;
};

export function ProjectGoalsEntryCard({ projectId }: Props) {
  const goalsQuery = useQuery({
    queryKey: projectQueryKeys.goals(projectId),
    queryFn: ({ signal }) => listProjectGoals(projectId, { signal }),
    enabled: Boolean(projectId),
  });
  const n = goalsQuery.data?.goals?.length ?? 0;
  const label =
    goalsQuery.isLoading || goalsQuery.isFetching
      ? "Loading goals…"
      : `${n} ${n === 1 ? "goal" : "goals"}`;

  return (
    <Link
      to={`/projects/${encodeURIComponent(projectId)}/goals`}
      className="pd__context-card pd__context-card--goals"
      aria-labelledby="pd-goals-entry-title"
      aria-label={`Open project goals. ${label}`}
    >
      <div className="pd__context-icon" aria-hidden="true">
        <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
          <circle cx="6" cy="6" r="2.25" stroke="currentColor" strokeWidth="1.3" />
          <circle cx="14" cy="6" r="2.25" stroke="currentColor" strokeWidth="1.3" />
          <circle cx="10" cy="14" r="2.25" stroke="currentColor" strokeWidth="1.3" />
          <path
            d="M7.5 7.5L9 11M12.5 7.5L11 11M10 13.25v1.5"
            stroke="currentColor"
            strokeWidth="1.2"
            strokeLinecap="round"
            opacity="0.45"
          />
        </svg>
      </div>
      <div className="pd__context-body">
        <h2 id="pd-goals-entry-title" className="pd__context-title">
          Goals
        </h2>
        <p className="pd__context-desc">
          Outcomes, dependencies, and release gates before work moves to steps.
        </p>
        <p className="pd__context-meta muted" aria-live="polite">
          {label}
        </p>
      </div>
      <svg
        className="pd__context-arrow"
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        aria-hidden="true"
      >
        <path
          d="M6 4l4 4-4 4"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </Link>
  );
}
