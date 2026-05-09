import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { listProjectSteps } from "@/api";
import { projectQueryKeys } from "./queryKeys";

type Props = {
  projectId: string;
};

export function ProjectStepsEntryCard({ projectId }: Props) {
  const stepsQuery = useQuery({
    queryKey: projectQueryKeys.steps(projectId),
    queryFn: ({ signal }) => listProjectSteps(projectId, { signal }),
    enabled: Boolean(projectId),
  });
  const n = stepsQuery.data?.steps?.length ?? 0;
  const label =
    stepsQuery.isLoading || stepsQuery.isFetching
      ? "Loading steps…"
      : `${n} ${n === 1 ? "stage" : "stages"}`;

  return (
    <Link
      to={`/projects/${encodeURIComponent(projectId)}/steps`}
      className="pd__context-card pd__context-card--steps"
      aria-labelledby="pd-steps-entry-title"
      aria-label={`Open project steps. ${label}`}
    >
      <div className="pd__context-icon" aria-hidden="true">
        <svg width="20" height="20" viewBox="0 0 18 18" fill="none">
          <path
            d="M4 5.5h10M4 9h10M4 12.5h6"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
          />
          <rect
            x="2.25"
            y="2.25"
            width="13.5"
            height="13.5"
            rx="3"
            stroke="currentColor"
            strokeWidth="1.2"
            opacity="0.45"
          />
        </svg>
      </div>
      <div className="pd__context-body">
        <h2 id="pd-steps-entry-title" className="pd__context-title">
          Steps
        </h2>
        <p className="pd__context-desc">
          Ordered stages, criteria, and gate releases when work is complete.
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
