import { useEffect, useState } from "react";

/**
 * Delays turning true so very short active periods (e.g. fast cache hits) do not
 * flash status text. When active becomes false, visibility drops immediately.
 */
export function useDelayedTrue(active: boolean, delayMs: number): boolean {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    if (delayMs <= 0) return;
    if (!active) {
      setVisible(false);
      return;
    }
    const id = window.setTimeout(() => setVisible(true), delayMs);
    return () => window.clearTimeout(id);
  }, [active, delayMs]);

  if (delayMs <= 0) {
    return active;
  }
  return visible;
}
