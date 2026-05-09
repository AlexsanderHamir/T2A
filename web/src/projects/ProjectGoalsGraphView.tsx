import { useCallback, useLayoutEffect, useMemo, useRef, useState } from "react";
import type { ProjectGoal } from "@/types";
import { goalsByLayerColumns } from "./projectGoalGraphLayout";
import { truncateGraphDescription, truncateGraphTitle } from "./projectListDisplayText";

function gateLabel(s: ProjectGoal["gate_status"]): string {
  switch (s) {
    case "locked":
      return "Locked";
    case "active":
      return "Active";
    case "pending_release":
      return "Pending release";
    case "released":
      return "Released";
    default:
      return s;
  }
}

function uiPhase(goal: ProjectGoal): "done" | "active" | "pending" | "blocked" {
  if (goal.gate_status === "released") return "done";
  if (goal.gate_hold) return "blocked";
  if (goal.gate_status === "active" || goal.gate_status === "pending_release") return "active";
  return "pending";
}

function criteriaProgressPct(goal: ProjectGoal): number {
  if (goal.criteria.length === 0) return 0;
  const done = goal.criteria.filter((c) => c.done).length;
  return Math.round((done / goal.criteria.length) * 100);
}

type Props = {
  goals: ProjectGoal[];
};

export function ProjectGoalsGraphView({ goals }: Props) {
  const columns = useMemo(() => goalsByLayerColumns(goals), [goals]);
  const idSet = useMemo(() => new Set(goals.map((g) => g.id)), [goals]);
  const canvasRef = useRef<HTMLDivElement>(null);
  const cardRefs = useRef<Map<string, HTMLDivElement>>(new Map());
  const [svgPaths, setSvgPaths] = useState<string[]>([]);
  const [svgSize, setSvgSize] = useState({ w: 0, h: 0 });

  const measureAndRoute = useCallback(() => {
    const root = canvasRef.current;
    if (!root) return;
    const rootRect = root.getBoundingClientRect();
    setSvgSize({ w: rootRect.width, h: rootRect.height });
    const paths: string[] = [];
    for (const g of goals) {
      for (const depId of g.depends_on_goal_ids) {
        if (!idSet.has(depId)) continue;
        const fromEl = cardRefs.current.get(depId);
        const toEl = cardRefs.current.get(g.id);
        if (!fromEl || !toEl) continue;
        const a = fromEl.getBoundingClientRect();
        const b = toEl.getBoundingClientRect();
        const x1 = a.right - rootRect.left;
        const y1 = a.top + a.height / 2 - rootRect.top;
        const x2 = b.left - rootRect.left;
        const y2 = b.top + b.height / 2 - rootRect.top;
        const mid = x1 + Math.max(24, (x2 - x1) * 0.45);
        paths.push(
          `M ${x1.toFixed(1)} ${y1.toFixed(1)} C ${mid.toFixed(1)} ${y1.toFixed(1)}, ${mid.toFixed(1)} ${y2.toFixed(1)}, ${x2.toFixed(1)} ${y2.toFixed(1)}`,
        );
      }
    }
    setSvgPaths(paths);
  }, [goals, idSet]);

  useLayoutEffect(() => {
    measureAndRoute();
    const raf = window.requestAnimationFrame(() => measureAndRoute());
    const root = canvasRef.current;
    if (!root || typeof ResizeObserver === "undefined") {
      return () => window.cancelAnimationFrame(raf);
    }
    const ro = new ResizeObserver(() => {
      measureAndRoute();
    });
    ro.observe(root);
    return () => {
      window.cancelAnimationFrame(raf);
      ro.disconnect();
    };
  }, [measureAndRoute]);

  const setCardRef = (id: string) => (el: HTMLDivElement | null) => {
    if (el) cardRefs.current.set(id, el);
    else cardRefs.current.delete(id);
  };

  return (
    <div className="pg__graph-canvas" ref={canvasRef}>
      {svgSize.w > 0 && svgSize.h > 0 ? (
        <svg
          className="pg__graph-svg"
          width={svgSize.w}
          height={svgSize.h}
          aria-hidden="true"
        >
          {svgPaths.map((d, i) => (
            <path
              key={i}
              d={d}
              fill="none"
              stroke="currentColor"
              strokeWidth={1.75}
              vectorEffect="non-scaling-stroke"
            />
          ))}
        </svg>
      ) : null}
      <div className="pg__graph-cols">
        {columns.map((col, colIdx) => (
          <div key={colIdx} className="pg__graph-col">
            {col.map((g) => {
              const phase = uiPhase(g);
              const pct = criteriaProgressPct(g);
              const titleFull = g.title.trim();
              const titleShown = truncateGraphTitle(g.title);
              const descFull = g.description.trim();
              const descShown = descFull ? truncateGraphDescription(g.description) : "";
              return (
                <article
                  key={g.id}
                  ref={setCardRef(g.id)}
                  className={`pg__graph-node pg__graph-node--${phase}`}
                >
                  <header className="pg__graph-node-head">
                    <span className={`ps__dot ps__dot--${phase}`} aria-hidden="true" />
                    <span className="pg__graph-node-status">{gateLabel(g.gate_status)}</span>
                  </header>
                  <h3
                    className="pg__graph-node-title"
                    title={titleFull !== titleShown ? titleFull : undefined}
                  >
                    {titleShown}
                  </h3>
                  {descShown ? (
                    <p
                      className="pg__graph-node-desc"
                      title={descFull !== descShown ? descFull : undefined}
                    >
                      {descShown}
                    </p>
                  ) : null}
                  <div
                    className="pg__graph-node-meter"
                    role="img"
                    aria-label={
                      g.criteria.length === 0
                        ? "No criteria"
                        : `${pct}% criteria complete`
                    }
                  >
                    <div
                      className="pg__graph-node-meter-fill"
                      style={{ width: `${pct}%` }}
                    />
                  </div>
                </article>
              );
            })}
          </div>
        ))}
      </div>
    </div>
  );
}
