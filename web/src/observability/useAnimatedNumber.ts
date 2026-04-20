import { useEffect, useRef, useState } from "react";

/**
 * Default tween duration for KPI number changes. 380ms matches the
 * existing `--duration-ui-phase` token used by row enter animations
 * so the dashboard reads as a single motion family. Passed in `ms`
 * rather than reading the CSS token at runtime to keep the hook
 * SSR-safe and allow per-call overrides for very-large deltas.
 */
const DEFAULT_DURATION_MS = 380;

/**
 * easeOutCubic: starts fast, slows to the target. Matches the shape
 * of `--ease-ui` (cubic-bezier(0.2, 0.8, 0.2, 1)) closely enough for
 * integer ticks. We don't need framer-motion-grade fidelity here —
 * the perceived effect is "number glides to new value" rather than
 * "number snaps".
 */
function easeOutCubic(t: number): number {
  const clamped = Math.max(0, Math.min(1, t));
  const inv = 1 - clamped;
  return 1 - inv * inv * inv;
}

function prefersReducedMotion(): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return false;
  }
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

/**
 * Returns an integer that smoothly tweens from its previous value
 * towards `target` over `durationMs` on every change. Snap-to-target
 * if the user prefers reduced motion, if the target is not finite,
 * or if the current and target value are equal.
 *
 * RAF-driven (no timers), so a backgrounded tab won't burn CPU on
 * the tween (the browser pauses RAF). Cleans up on unmount.
 */
export function useAnimatedNumber(
  target: number,
  durationMs: number = DEFAULT_DURATION_MS,
): number {
  const [display, setDisplay] = useState<number>(
    Number.isFinite(target) ? target : 0,
  );
  // Pin the "from" value at animation start so a mid-tween target
  // change animates from wherever the display currently is, not from
  // the previous start. Same idiom as react-spring's `immediate`.
  const fromRef = useRef<number>(display);
  const startedAtRef = useRef<number | null>(null);
  const rafRef = useRef<number | null>(null);

  useEffect(() => {
    if (!Number.isFinite(target)) {
      setDisplay(0);
      return;
    }
    if (prefersReducedMotion() || durationMs <= 0) {
      setDisplay(target);
      return;
    }
    fromRef.current = display;
    startedAtRef.current = null;

    const step = (now: number) => {
      if (startedAtRef.current === null) {
        startedAtRef.current = now;
      }
      const elapsed = now - startedAtRef.current;
      const t = Math.max(0, Math.min(1, elapsed / durationMs));
      const eased = easeOutCubic(t);
      const from = fromRef.current;
      const next = Math.round(from + (target - from) * eased);
      setDisplay(next);
      if (t < 1) {
        rafRef.current = requestAnimationFrame(step);
      } else {
        setDisplay(target);
        rafRef.current = null;
      }
    };
    rafRef.current = requestAnimationFrame(step);

    return () => {
      if (rafRef.current !== null) {
        cancelAnimationFrame(rafRef.current);
        rafRef.current = null;
      }
    };
    // `display` is intentionally excluded from deps: it changes on
    // every animation frame and would restart the tween. `target`
    // changing is the only trigger that should start a new tween.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [target, durationMs]);

  return display;
}
