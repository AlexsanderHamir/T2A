import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router-dom";
import type { TaskCyclePhase } from "@/types";

export function parsePhaseFilterParam(
  raw: string | null,
  phases: readonly TaskCyclePhase[],
): number | null {
  if (!raw) {
    return null;
  }
  const n = Number(raw);
  if (!Number.isInteger(n) || n <= 0) {
    return null;
  }
  const known = new Set(phases.map((p) => p.phase_seq));
  return known.has(n) ? n : null;
}

export function useAttemptPhaseFilter(phases: readonly TaskCyclePhase[]) {
  const [searchParams, setSearchParams] = useSearchParams();
  const filterPhaseSeq = useMemo(
    () => parsePhaseFilterParam(searchParams.get("phase"), phases),
    [searchParams, phases],
  );

  const setFilterPhaseSeq = useCallback(
    (seq: number | null) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          if (seq === null) {
            next.delete("phase");
          } else {
            next.set("phase", String(seq));
          }
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  const clearFilter = useCallback(
    () => setFilterPhaseSeq(null),
    [setFilterPhaseSeq],
  );

  return { filterPhaseSeq, setFilterPhaseSeq, clearFilter };
}
