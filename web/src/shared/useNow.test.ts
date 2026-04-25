import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useNow } from "./useNow";

function setVisibilityState(value: DocumentVisibilityState) {
  Object.defineProperty(document, "visibilityState", {
    configurable: true,
    value,
  });
}

describe("useNow", () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    setVisibilityState("visible");
  });

  it("ticks on a steady interval while enabled", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-04-25T12:00:00.000Z"));

    const { result } = renderHook(() => useNow({ intervalMs: 1000 }));

    expect(result.current).toBe(Date.parse("2026-04-25T12:00:00.000Z"));

    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(result.current).toBe(Date.parse("2026-04-25T12:00:01.000Z"));
  });

  it("pauses while hidden and refreshes when visible again", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-04-25T12:00:00.000Z"));

    const { result } = renderHook(() => useNow({ intervalMs: 1000 }));
    expect(result.current).toBe(Date.parse("2026-04-25T12:00:00.000Z"));

    act(() => {
      setVisibilityState("hidden");
      document.dispatchEvent(new Event("visibilitychange"));
    });

    act(() => {
      vi.advanceTimersByTime(5000);
    });

    expect(result.current).toBe(Date.parse("2026-04-25T12:00:00.000Z"));

    act(() => {
      setVisibilityState("visible");
      document.dispatchEvent(new Event("visibilitychange"));
    });

    expect(result.current).toBe(Date.parse("2026-04-25T12:00:05.000Z"));
  });

  it("does not start an interval when disabled", () => {
    vi.useFakeTimers();
    const intervalSpy = vi.spyOn(globalThis, "setInterval");

    renderHook(() => useNow({ enabled: false }));

    expect(intervalSpy).not.toHaveBeenCalled();
  });
});
