import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useDelayedTrue } from "./useDelayedTrue";

describe("useDelayedTrue", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns active immediately when delayMs is 0", () => {
    const { result, rerender } = renderHook(
      ({ active, d }: { active: boolean; d: number }) =>
        useDelayedTrue(active, d),
      { initialProps: { active: true, d: 0 } },
    );
    expect(result.current).toBe(true);
    rerender({ active: false, d: 0 });
    expect(result.current).toBe(false);
  });

  it("delays true until after delayMs", () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(
      ({ active }: { active: boolean }) => useDelayedTrue(active, 200),
      { initialProps: { active: true } },
    );
    expect(result.current).toBe(false);
    act(() => {
      vi.advanceTimersByTime(199);
    });
    expect(result.current).toBe(false);
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe(true);

    rerender({ active: false });
    expect(result.current).toBe(false);
  });
});
