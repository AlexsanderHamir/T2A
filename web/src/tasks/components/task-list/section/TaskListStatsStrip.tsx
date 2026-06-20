import { useMemo } from "react";
import { isUiFeatureOmitted } from "@/launch/omittedFeatures";
import type { TaskStatsResponse } from "@/types";

type Props = {
  /** May be `null` while the stats query is still loading or has errored. */
  stats: TaskStatsResponse | null | undefined;
};

type Pill = {
  key: string;
  value: number;
  label: string;
  /**
   * Surface tokens only — avoids brand purple on the scoreboard per redesign.
   * `ready` = slate ready pill; `schedule` / `review` = distinct non-violet cues.
   */
  tone:
    | "default"
    | "ready"
    | "warn"
    | "schedule"
    | "review";
};

/**
 * Compact one-line summary that sits between the page heading and the
 * filters row when at least one task exists. Quiet by design — Stripe-style
 * "operator scoreboard" rather than a full dashboard. Each pill carries a
 * tonal accent only when the count is non-zero so the strip never reads as
 * loud noise on a fresh database. Labels are sentence case; tones use pill
 * tokens, not brand purple on this row.
 *
 * Stats fields read directly from `GET /tasks/stats`; the component is a
 * pure projection that hides itself when there is nothing useful to show.
 */
export function TaskListStatsStrip({ stats }: Props) {
  const scheduleUiEnabled = !isUiFeatureOmitted("schedule");
  const pills = useMemo<Pill[]>(() => {
    if (!stats || stats.total <= 0) return [];
    const ready = stats.ready ?? 0;
    const critical = stats.critical ?? 0;
    const scheduled = stats.scheduled ?? 0;
    const review = stats.by_status?.review ?? 0;
    const blocked = stats.by_status?.blocked ?? 0;
    const next: Pill[] = [
      { key: "total", value: stats.total, label: "Total", tone: "default" },
      { key: "ready", value: ready, label: "Ready", tone: "ready" },
    ];
    if (critical > 0) {
      next.push({
        key: "critical",
        value: critical,
        label: "Critical",
        tone: "warn",
      });
    }
    if (scheduleUiEnabled && scheduled > 0) {
      next.push({
        key: "scheduled",
        value: scheduled,
        label: "Scheduled",
        tone: "schedule",
      });
    }
    if (review > 0) {
      next.push({
        key: "review",
        value: review,
        label: "Review",
        tone: "review",
      });
    }
    if (blocked > 0) {
      next.push({
        key: "blocked",
        value: blocked,
        label: "Blocked",
        tone: "warn",
      });
    }
    return next;
  }, [scheduleUiEnabled, stats]);

  if (pills.length === 0) return null;

  return (
    <div
      className="task-list-stats-strip"
      role="status"
      aria-live="polite"
      data-testid="task-list-stats-strip"
    >
      <dl className="task-list-stats-strip__list">
        {pills.map((pill, index) => (
          <div
            key={pill.key}
            className="task-list-stats-strip__metric"
            data-tone={pill.tone}
            data-first={index === 0 ? "true" : undefined}
          >
            <dd
              className="task-list-stats-strip__value"
              data-testid={`task-list-stats-${pill.key}`}
            >
              {pill.value}
            </dd>
            <dt className="task-list-stats-strip__label">{pill.label}</dt>
          </div>
        ))}
      </dl>
    </div>
  );
}
