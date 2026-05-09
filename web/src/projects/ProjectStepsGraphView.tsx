import { useCallback, useLayoutEffect, useMemo, useRef, useState } from "react";
import type { ProjectStep } from "@/types";
import { truncateGraphDescription, truncateGraphTitle } from "./projectListDisplayText";

type Phase = "done" | "active" | "pending" | "blocked";

type Props = {
  steps: ProjectStep[];
  phaseOf: (step: ProjectStep) => Phase;
  gateLabel: (s: ProjectStep["gate_status"]) => string;
  tasksByStepId: Map<string, { total: number; done: number }>;
};

function taskProgressPct(stats: { total: number; done: number } | undefined): number {
  if (!stats || stats.total === 0) return 0;
  return Math.round((stats.done / stats.total) * 100);
}

export function ProjectStepsGraphView({ steps, phaseOf, gateLabel, tasksByStepId }: Props) {
  const canvasRef = useRef<HTMLDivElement>(null);
  const cardRefs = useRef<Map<string, HTMLDivElement>>(new Map());
  const [svgPaths, setSvgPaths] = useState<string[]>([]);
  const [svgSize, setSvgSize] = useState({ w: 0, h: 0 });

  const pairs = useMemo(() => {
    const out: Array<{ from: string; to: string }> = [];
    for (let i = 1; i < steps.length; i++) {
      out.push({ from: steps[i - 1].id, to: steps[i].id });
    }
    return out;
  }, [steps]);

  const measureAndRoute = useCallback(() => {
    const root = canvasRef.current;
    if (!root) return;
    const rootRect = root.getBoundingClientRect();
    setSvgSize({ w: rootRect.width, h: rootRect.height });
    const paths: string[] = [];
    for (const { from, to } of pairs) {
      const fromEl = cardRefs.current.get(from);
      const toEl = cardRefs.current.get(to);
      if (!fromEl || !toEl) continue;
      const a = fromEl.getBoundingClientRect();
      const b = toEl.getBoundingClientRect();
      const x1 = a.right - rootRect.left;
      const y1 = a.top + a.height / 2 - rootRect.top;
      const x2 = b.left - rootRect.left;
      const y2 = b.top + b.height / 2 - rootRect.top;
      const mid = x1 + Math.max(20, (x2 - x1) * 0.5);
      paths.push(
        `M ${x1.toFixed(1)} ${y1.toFixed(1)} C ${mid.toFixed(1)} ${y1.toFixed(1)}, ${mid.toFixed(1)} ${y2.toFixed(1)}, ${x2.toFixed(1)} ${y2.toFixed(1)}`,
      );
    }
    setSvgPaths(paths);
  }, [pairs]);

  useLayoutEffect(() => {
    measureAndRoute();
    const raf = window.requestAnimationFrame(() => measureAndRoute());
    const root = canvasRef.current;
    if (!root || typeof ResizeObserver === "undefined") {
      return () => window.cancelAnimationFrame(raf);
    }
    const ro = new ResizeObserver(() => measureAndRoute());
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
    <div className="ps__graph-canvas" ref={canvasRef}>
      {svgSize.w > 0 && svgSize.h > 0 ? (
        <svg
          className="ps__graph-svg"
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
      <div className="ps__graph-nodes" role="list">
        {steps.map((step) => {
          const phase = phaseOf(step);
          const stats = tasksByStepId.get(step.id);
          const pct = taskProgressPct(stats);
          const titleFull = step.title.trim();
          const titleShown = truncateGraphTitle(step.title);
          const descFull = step.description.trim();
          const descShown = descFull ? truncateGraphDescription(step.description) : "";
          return (
            <article
              key={step.id}
              ref={setCardRef(step.id)}
              className={`ps__graph-node ps__graph-node--${phase}`}
              role="listitem"
            >
              <header className="ps__graph-node-head">
                <span className={`ps__dot ps__dot--${phase}`} aria-hidden="true" />
                <span className="ps__graph-node-status">{gateLabel(step.gate_status)}</span>
              </header>
              <h3
                className="ps__graph-node-title"
                title={titleFull !== titleShown ? titleFull : undefined}
              >
                {titleShown}
              </h3>
              {descShown ? (
                <p
                  className="ps__graph-node-desc"
                  title={descFull !== descShown ? descFull : undefined}
                >
                  {descShown}
                </p>
              ) : null}
              <div
                className="ps__graph-node-meter"
                role="img"
                aria-label={
                  !stats || stats.total === 0
                    ? "No tasks in this step"
                    : `${stats.done} of ${stats.total} tasks done`
                }
              >
                <div
                  className="ps__graph-node-meter-fill"
                  style={{ width: `${pct}%` }}
                />
              </div>
            </article>
          );
        })}
      </div>
    </div>
  );
}
