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

    // Halfway through the coalesce window: still nothing, the agent
    // worker emits frames ~1s apart so the trailing debounce must
    // outlast a single inter-frame gap to actually batch them.
    act(() => {
      vi.advanceTimersByTime(450);
    });
    expect(inv).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(500);
    });
    expect(inv).toHaveBeenCalled();
  });

  it("invalidates the task detail subtree on task_cycle_changed so worker-driven activity is live on the open task page", () => {
    // Prior behaviour kept the detail subtree warm on cycle frames as
    // an optimisation. Because the agent worker only emits cycle frames
    // (never task_updated), that left the open task detail page silently
    // stale: status flips (running → done), new audit events, and
    // checklist toggles all needed a manual refresh. We now treat cycle
    // frames as broad task invalidations; this test pins that contract.
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
      vi.advanceTimersByTime(950);
    });

    const calls = inv.mock.calls.map((c) => c[0]);
    expect(calls).toContainEqual({ queryKey: ["tasks", "list"] });
    expect(calls).toContainEqual({
      queryKey: ["tasks", "detail", "task-1"],
    });
    // Standalone "cycles only" invalidation must not also fire — it
    // would be redundant work and would defeat the dedup logic.
    for (const arg of calls) {
      const key = (arg as { queryKey: readonly unknown[] }).queryKey;
      expect(key).not.toEqual(["tasks", "detail", "task-1", "cycles"]);
    }
  });

  it("invalidates the home-page KPI stats cache on task/cycle frames so Ready/Critical counts stay live", () => {
    // Regression: the home-page KPI cards (Total / Ready / Critical) are
    // backed by a separate ["task-stats"] query that lives outside the
    // taskQueryKeys tree. SSE used to invalidate only listRoot + detail,
    // so a worker-driven `running → done` transition refreshed the row
    // in the table but left "Ready tasks: 1" frozen until a manual
    // mutation or a hard refresh — exactly the staleness the user
    // reported on the main page.
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
        data: '{"type":"task_cycle_changed","id":"task-stats-1","cycle_id":"cyc-1"}',
      });
    });
    act(() => {
      vi.advanceTimersByTime(950);
    });

    const calls = inv.mock.calls.map((c) => c[0]);
    expect(calls).toContainEqual({ queryKey: ["task-stats"] });
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
      vi.advanceTimersByTime(950);
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
      vi.advanceTimersByTime(950);
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

  it("coalesces a burst of agent worker cycle frames into a single flush", () => {
    // Regression: in production the agent worker emits ~6 task_cycle_changed
    // frames per task run, ~1s apart. A short trailing debounce never
    // batched them and each frame fired its own refetch storm. With the
    // new ~900ms window plus maxWait, frames arriving every ~700ms should
    // collapse into ONE flush (one detail invalidation per task) instead
    // of six. We assert that the per-task detail key is invalidated at
    // most once across the whole burst.
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const inv = vi.spyOn(qc, "invalidateQueries");

    renderHook(() => useTaskEventStream(), {
      wrapper: createWrapper(qc),
    });
    const mockES = getCurrentMockES();
    act(() => {
      mockES!.onopen?.();
    });

    // Emit 4 cycle frames 700ms apart — each one resets the debounce so
    // nothing flushes mid-burst.
    for (let i = 0; i < 4; i++) {
      act(() => {
        mockES!.onmessage?.({
          data: '{"type":"task_cycle_changed","id":"task-burst","cycle_id":"cyc-1"}',
        });
      });
      act(() => {
        vi.advanceTimersByTime(700);
      });
    }
    // Drain the trailing debounce.
    act(() => {
      vi.advanceTimersByTime(950);
    });

    const detailCalls = inv.mock.calls
      .map((c) => (c[0] as { queryKey: readonly unknown[] }).queryKey)
      .filter((k) => JSON.stringify(k) === JSON.stringify(["tasks", "detail", "task-burst"]));
    expect(detailCalls).toHaveLength(1);
  });

  it("forces a flush at maxWait so a continuous SSE stream cannot starve the UI", () => {
    // The trailing debounce alone could be reset forever by frames
    // arriving inside the coalesce window (e.g. multiple concurrent
    // tasks). The maxWait safety valve must force a flush so the open
    // task page still receives status updates under sustained load.
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const inv = vi.spyOn(qc, "invalidateQueries");

    renderHook(() => useTaskEventStream(), {
      wrapper: createWrapper(qc),
    });
    const mockES = getCurrentMockES();
    act(() => {
      mockES!.onopen?.();
    });

    // 12 frames every 300ms = 3.6s of continuous activity, all *inside*
    // the 900ms coalesce window — a naive debounce would never flush.
    for (let i = 0; i < 12; i++) {
      act(() => {
        mockES!.onmessage?.({
          data: '{"type":"task_cycle_changed","id":"task-stream","cycle_id":"cyc-1"}',
        });
      });
      act(() => {
        vi.advanceTimersByTime(300);
      });
    }

    const detailCalls = inv.mock.calls
      .map((c) => (c[0] as { queryKey: readonly unknown[] }).queryKey)
      .filter((k) => JSON.stringify(k) === JSON.stringify(["tasks", "detail", "task-stream"]));
    expect(detailCalls.length).toBeGreaterThanOrEqual(1);
  });

  it("settings_changed invalidates only the settings cache and never the task tree", () => {
    // Documented in docs/API-SSE.md: settings/cancel frames must "invalidate
    // only the settings cache slot ... without disturbing task caches; they
    // bypass the debounce batch". Regression: previously the trailing
    // debounce was armed for *every* frame, so settings/cancel frames
    // (which add nothing to pendingRef) fell through to the broad-fallback
    // branch in flushStreamInvalidation and refetched every active task
    // query SSE_INVALIDATE_WINDOW_MS later. The fix gates the timer on
    // whether there is anything pending to flush.
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
      mockES!.onmessage?.({ data: '{"type":"settings_changed"}' });
    });
    // Drain well past the debounce window + maxWait safety valve so any
    // timer-triggered fallback would have fired.
    act(() => {
      vi.advanceTimersByTime(3000);
    });

    const calls = inv.mock.calls.map(
      (c) => (c[0] as { queryKey: readonly unknown[] }).queryKey,
    );
    expect(calls).toContainEqual(["settings", "app"]);
    for (const key of calls) {
      expect(key).not.toEqual(["tasks"]);
      expect((key as readonly unknown[])[0]).not.toBe("tasks");
    }
  });

  it("agent_run_cancelled invalidates only the settings cache and never the task tree", () => {
    // Same contract as settings_changed: id-less notification, must not
    // disturb task caches. See docs/API-SSE.md.
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
      mockES!.onmessage?.({ data: '{"type":"agent_run_cancelled"}' });
    });
    act(() => {
      vi.advanceTimersByTime(3000);
    });

    const calls = inv.mock.calls.map(
      (c) => (c[0] as { queryKey: readonly unknown[] }).queryKey,
    );
    expect(calls).toContainEqual(["settings", "app"]);
    for (const key of calls) {
      expect(key).not.toEqual(["tasks"]);
      expect((key as readonly unknown[])[0]).not.toBe("tasks");
    }
  });

  it("a settings frame followed by a task frame still flushes the task invalidation", () => {
    // Guard against an over-eager fix that disables the debounce timer
    // entirely on settings frames: a *subsequent* task/cycle frame in the
    // same batch must still flush the broader task invalidation after the
    // coalesce window. Without this pin, gating the timer on "nothing
    // pending" could regress real task-driven invalidation if the order
    // of frames matters.
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
      mockES!.onmessage?.({ data: '{"type":"settings_changed"}' });
      mockES!.onmessage?.({
        data: '{"type":"task_updated","id":"task-mix"}',
      });
    });
    act(() => {
      vi.advanceTimersByTime(950);
    });

    const calls = inv.mock.calls.map(
      (c) => (c[0] as { queryKey: readonly unknown[] }).queryKey,
    );
    expect(calls).toContainEqual(["settings", "app"]);
    expect(calls).toContainEqual(["tasks", "list"]);
    expect(calls).toContainEqual(["tasks", "detail", "task-mix"]);
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
