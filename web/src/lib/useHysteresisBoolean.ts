import { useEffect, useRef, useState } from "react";

/**
 * Smooths a noisy boolean: becomes true only after raw stays true for onDelayMs,
 * and false only after raw stays false for offDelayMs.
 */
export function useHysteresisBoolean(
  raw: boolean,
  onDelayMs: number,
  offDelayMs: number,
): boolean {
  const [stable, setStable] = useState(false);
  const onTimerRef = useRef<number | undefined>(undefined);
  const offTimerRef = useRef<number | undefined>(undefined);

  useEffect(() => {
    if (raw) {
      if (offTimerRef.current !== undefined) {
        window.clearTimeout(offTimerRef.current);
        offTimerRef.current = undefined;
      }
      if (stable) {
        return () => {
          if (onTimerRef.current !== undefined) {
            window.clearTimeout(onTimerRef.current);
            onTimerRef.current = undefined;
          }
        };
      }
      if (onDelayMs <= 0) {
        setStable(true);
        return;
      }
      if (onTimerRef.current !== undefined) {
        window.clearTimeout(onTimerRef.current);
      }
      onTimerRef.current = window.setTimeout(() => {
        onTimerRef.current = undefined;
        setStable(true);
      }, onDelayMs);
      return () => {
        if (onTimerRef.current !== undefined) {
          window.clearTimeout(onTimerRef.current);
          onTimerRef.current = undefined;
        }
      };
    }

    if (onTimerRef.current !== undefined) {
      window.clearTimeout(onTimerRef.current);
      onTimerRef.current = undefined;
    }
    if (!stable) {
      if (offTimerRef.current !== undefined) {
        window.clearTimeout(offTimerRef.current);
        offTimerRef.current = undefined;
      }
      return;
    }
    if (offDelayMs <= 0) {
      setStable(false);
      return;
    }
    if (offTimerRef.current !== undefined) {
      window.clearTimeout(offTimerRef.current);
    }
    offTimerRef.current = window.setTimeout(() => {
      offTimerRef.current = undefined;
      setStable(false);
    }, offDelayMs);
    return () => {
      if (offTimerRef.current !== undefined) {
        window.clearTimeout(offTimerRef.current);
        offTimerRef.current = undefined;
      }
    };
  }, [raw, stable, onDelayMs, offDelayMs]);

  return stable;
}
