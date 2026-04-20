import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAnimatedNumber } from "./useAnimatedNumber";

describe("useAnimatedNumber", () => {
  let rafCallbacks: Array<(now: number) => void> = [];
  let rafCounter = 0;
  const origRAF = global.requestAnimationFrame;
  const origCAF = global.cancelAnimationFrame;

  beforeEach(() => {
    rafCallbacks = [];
    rafCounter = 0;
    global.requestAnimationFrame = ((cb: (now: number) => void) => {
      rafCounter += 1;
      rafCallbacks.push(cb);
      return rafCounter;
    }) as unknown as typeof requestAnimationFrame;
    global.cancelAnimationFrame = vi.fn() as unknown as typeof cancelAnimationFrame;
  });

  afterEach(() => {
    global.requestAnimationFrame = origRAF;
    global.cancelAnimationFrame = origCAF;
  });

  function flushRAF(nowMs: number) {
    const cbs = rafCallbacks;
    rafCallbacks = [];
    for (const cb of cbs) cb(nowMs);
  }

  it("starts at target and returns it when unchanged", () => {
    const { result } = renderHook(() => useAnimatedNumber(42, 100));
    expect(result.current).toBe(42);
  });

  it("tweens to a new target over the duration", () => {
    const { result, rerender } = renderHook(
      ({ v }: { v: number }) => useAnimatedNumber(v, 200),
      { initialProps: { v: 0 } },
    );
    expect(result.current).toBe(0);

    rerender({ v: 100 });
    act(() => {
      flushRAF(0);
    });
    act(() => {
      flushRAF(100);
    });
    expect(result.current).toBeGreaterThan(0);
    expect(result.current).toBeLessThan(100);

    act(() => {
      flushRAF(200);
    });
    expect(result.current).toBe(100);
  });

  it("snaps when target is not finite", () => {
    const { result, rerender } = renderHook(
      ({ v }: { v: number }) => useAnimatedNumber(v, 200),
      { initialProps: { v: 10 } },
    );
    rerender({ v: Number.NaN });
    expect(result.current).toBe(0);
  });

  it("snaps when durationMs <= 0", () => {
    const { result, rerender } = renderHook(
      ({ v }: { v: number }) => useAnimatedNumber(v, 0),
      { initialProps: { v: 0 } },
    );
    rerender({ v: 500 });
    expect(result.current).toBe(500);
  });
});
