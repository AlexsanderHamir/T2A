import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { listProjectContext } from "@/api";
import { projectQueryKeys } from "./queryKeys";

const ENTRY_META_LIMIT = 100;

type Props = {
  projectId: string;
};

export function ProjectContextEntryCard({ projectId }: Props) {
  const contextQuery = useQuery({
    queryKey: [...projectQueryKeys.context(projectId), "entry-meta"],
    queryFn: ({ signal }) =>
      listProjectContext(projectId, { signal, limit: ENTRY_META_LIMIT }),
    enabled: Boolean(projectId),
  });
  const n = contextQuery.data?.items?.length ?? 0;
  const atCap = n >= ENTRY_META_LIMIT;
  const label =
    contextQuery.isLoading || contextQuery.isFetching
      ? "Loading nodes…"
      : atCap
        ? `${ENTRY_META_LIMIT}+ nodes`
        : `${n} ${n === 1 ? "node" : "nodes"}`;

  return (
    <Link
      to={`/projects/${encodeURIComponent(projectId)}/context`}
      className="pd__context-card"
      aria-labelledby="pd-context-title"
      aria-label={`Open project context. ${label}`}
    >
      <div className="pd__context-icon" aria-hidden="true">
        <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
          <circle cx="10" cy="5" r="2" fill="currentColor" opacity="0.9" />
          <circle cx="5" cy="14" r="2" fill="currentColor" opacity="0.55" />
          <circle cx="15" cy="14" r="2" fill="currentColor" opacity="0.55" />
          <path d="M10 7v3M8.5 12l-2 1M11.5 12l2 1" stroke="currentColor" strokeWidth="1.2" opacity="0.35" />
        </svg>
      </div>
      <div className="pd__context-body">
        <h2 id="pd-context-title" className="pd__context-title">
          Project context
        </h2>
        <p className="pd__context-desc">Memory nodes, decisions, and constraints</p>
        <p className="pd__context-meta muted" aria-live="polite">
          {label}
        </p>
      </div>
      <svg className="pd__context-arrow" width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
        <path d="M6 4l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    </Link>
  );
}
