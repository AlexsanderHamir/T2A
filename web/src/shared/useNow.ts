import { useEffect, useState } from "react";

type UseNowOptions = {
  enabled?: boolean;
  intervalMs?: number;
};

const DEFAULT_INTERVAL_MS = 1000;

export function useNow(options?: UseNowOptions): number {
  const enabled = options?.enabled ?? true;
  const intervalMs = Math.max(250, options?.intervalMs ?? DEFAULT_INTERVAL_MS);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!enabled) return;

    let intervalId: ReturnType<typeof setInterval> | undefined;

    const clearTick = () => {
      if (intervalId !== undefined) {
        clearInterval(intervalId);
        intervalId = undefined;
      }
    };

    const startTick = () => {
      clearTick();
      setNow(Date.now());
      if (document.visibilityState === "hidden") return;
      intervalId = setInterval(() => setNow(Date.now()), intervalMs);
    };

    startTick();
    document.addEventListener("visibilitychange", startTick);

    return () => {
      clearTick();
      document.removeEventListener("visibilitychange", startTick);
    };
  }, [enabled, intervalMs]);

  return now;
}
