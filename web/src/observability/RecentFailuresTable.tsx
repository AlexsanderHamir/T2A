import { Link } from "react-router-dom";
import type { TaskStatsRecentFailure } from "@/types/task";
import { CycleFailuresTable } from "./CycleFailuresTable";

type Props = {
  failures: TaskStatsRecentFailure[];
  /** When set, the section title links here (e.g. full failures list). */
  titleHref?: string;
};

/**
 * Recent cycle failures rendered as a compact table. Each row deep-
 * links to the originating audit event (`/tasks/{task_id}/events/{seq}`)
 * so the operator is one click away from the full payload, response
 * thread, and follow-up actions.
 *
 * Empty state is intentional: an empty array is the steady state on a
 * healthy database, so we render a friendly note rather than collapse
 * the section to zero height.
 */
export function RecentFailuresTable({ failures, titleHref }: Props) {
  const title = titleHref ? (
    <Link to={titleHref} className="obs-failures-title-link">
      Recent failures
    </Link>
  ) : (
    "Recent failures"
  );
  return (
    <section className="obs-failures" aria-label="Recent cycle failures">
      <header className="obs-failures-head">
        <h3 className="obs-failures-title">{title}</h3>
        <p className="obs-failures-caption">
          {failures.length === 0
            ? "No recent cycle failures — the agent worker is clean."
            : `Last ${failures.length} cycle ${failures.length === 1 ? "failure" : "failures"} (newest first)`}
        </p>
      </header>
      {failures.length === 0 ? null : (
        <div className="obs-failures-tablewrap" role="region" aria-label="Recent failures">
          <CycleFailuresTable failures={failures} />
        </div>
      )}
    </section>
  );
}
