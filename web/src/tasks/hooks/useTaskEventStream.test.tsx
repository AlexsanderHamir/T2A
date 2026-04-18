import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useTaskEventStream } from "./useTaskEventStream";

type MockES = {
  onopen: (() => void) | null;
  onmessage: ((ev: { data?: string }) => void) | null;
  onerror: (() => void) | null;
  close: ReturnType<typeof vi.fn>;
  readyState: number;
};

let getCurrentMockES: () => MockES | null;

function createWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

describe("useTaskEventStream", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    class MockEventSource implements MockES {
      static latest: MockEventSource | null = null;
      onopen: (() => void) | null = null;
      onmessage: ((ev: { data?: string }) => void) | null = null;
      onerror: (() => void) | null = null;
      close = vi.fn();
      readyState = 0;
      constructor() {
        MockEventSource.latest = this;
      }
    }
    getCurrentMockES = () => MockEventSource.latest;
    vi.stubGlobal("EventSource", MockEventSource);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("debounced SSE message triggers query invalidation after delay", () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const inv = vi.spyOn(qc, "invalidateQueries");

    renderHook(() => useTaskEventStream(), {
      wrapper: createWrapper(qc),
    });

    const mockES = getCurrentMockES();
    expect(mockES).not.toBeNull();
    act(() => {
      mockES!.onopen?.();
    });
    act(() => {
      mockES!.onmessage?.({
        data: '{"type":"task_updated","id":"11111111-1111-4111-8111-111111111111"}',
      });
    });
    expect(inv).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(400);
    });
    expect(inv).toHaveBeenCalled();
  });

  it("invalidates only the cycles subtree on task_cycle_changed (no detail churn)", () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const inv = vi.spyOn(qc, "invalidateQueries");

    renderHook(() => useTaskEventStream(), {
      wrapper: createWrapper(qc),
    });

    const mockES = getCurrentMockES();
    expect(mockES).not.toBeNull();
    act(() => {
      mockES!.onopen?.();
    });
    act(() => {
      mockES!.onmessage?.({
        data: '{"type":"task_cycle_changed","id":"task-1","cycle_id":"cyc-1"}',
      });
    });
    act(() => {
      vi.advanceTimersByTime(400);
    });

    const calls = inv.mock.calls.map((c) => c[0]);
    expect(calls).toContainEqual({
      queryKey: ["tasks", "detail", "task-1", "cycles"],
    });
    for (const arg of calls) {
      const key = (arg as { queryKey: readonly unknown[] }).queryKey;
      expect(key).not.toEqual(["tasks", "detail", "task-1"]);
      expect(key).not.toEqual(["tasks", "list"]);
      expect(key).not.toEqual(["tasks"]);
    }
  });

  it("falls back to broad invalidation when no recognised frame arrives", () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const inv = vi.spyOn(qc, "invalidateQueries");

    renderHook(() => useTaskEventStream(), {
      wrapper: createWrapper(qc),
    });

    const mockES = getCurrentMockES();
    act(() => {
      mockES!.onopen?.();
    });
    act(() => {
      mockES!.onmessage?.({ data: "{}" });
    });
    act(() => {
      vi.advanceTimersByTime(400);
    });
    expect(inv).toHaveBeenCalledWith({ queryKey: ["tasks"] });
  });

  it("dedupes cycle invalidation when the same task already has a broad invalidation pending", () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const inv = vi.spyOn(qc, "invalidateQueries");

    renderHook(() => useTaskEventStream(), {
      wrapper: createWrapper(qc),
    });
    const mockES = getCurrentMockES();
    act(() => {
      mockES!.onopen?.();
    });
    act(() => {
      mockES!.onmessage?.({
        data: '{"type":"task_updated","id":"task-1"}',
      });
      mockES!.onmessage?.({
        data: '{"type":"task_cycle_changed","id":"task-1","cycle_id":"cyc-1"}',
      });
    });
    act(() => {
      vi.advanceTimersByTime(400);
    });
    const calls = inv.mock.calls.map((c) => c[0]);
    const cyclesOnlyCalls = calls.filter(
      (c) =>
        JSON.stringify((c as { queryKey: unknown[] }).queryKey) ===
        JSON.stringify(["tasks", "detail", "task-1", "cycles"]),
    );
    expect(cyclesOnlyCalls).toHaveLength(0);
    expect(calls).toContainEqual({
      queryKey: ["tasks", "detail", "task-1"],
    });
  });

  it("does not invalidate queries after unmount before debounce elapses", () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const inv = vi.spyOn(qc, "invalidateQueries");

    const { unmount } = renderHook(() => useTaskEventStream(), {
      wrapper: createWrapper(qc),
    });

    const mockES = getCurrentMockES();
    expect(mockES).not.toBeNull();
    act(() => {
      mockES!.onopen?.();
    });
    act(() => {
      mockES!.onmessage?.({ data: "{}" });
    });
    act(() => {
      vi.advanceTimersByTime(100);
    });
    unmount();
    act(() => {
      vi.advanceTimersByTime(400);
    });
    expect(inv).not.toHaveBeenCalled();
  });
});
