import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useHysteresisBoolean } from "./useHysteresisBoolean";

describe("useHysteresisBoolean", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows true immediately when onDelayMs is 0 and raw is true", () => {
    const { result } = renderHook(() => useHysteresisBoolean(true, 0, 100));
    expect(result.current).toBe(true);
  });

  it("shows false immediately when offDelayMs is 0 and raw is false", () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(
      ({ raw }: { raw: boolean }) => useHysteresisBoolean(raw, 50, 0),
      { initialProps: { raw: true } },
    );
    act(() => {
      vi.advanceTimersByTime(50);
    });
    expect(result.current).toBe(true);

    rerender({ raw: false });
    expect(result.current).toBe(false);
    vi.useRealTimers();
  });

  it("delays on and off according to thresholds", () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(
      ({ raw }: { raw: boolean }) => useHysteresisBoolean(raw, 200, 300),
      { initialProps: { raw: true } },
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

    rerender({ raw: false });
    expect(result.current).toBe(true);
    act(() => {
      vi.advanceTimersByTime(299);
    });
    expect(result.current).toBe(true);
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe(false);
  });

  it("cancels pending on when raw drops before onDelayMs", () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(
      ({ raw }: { raw: boolean }) => useHysteresisBoolean(raw, 200, 100),
      { initialProps: { raw: true } },
    );

    act(() => {
      vi.advanceTimersByTime(100);
    });
    rerender({ raw: false });
    act(() => {
      vi.advanceTimersByTime(500);
    });
    expect(result.current).toBe(false);
  });
});
