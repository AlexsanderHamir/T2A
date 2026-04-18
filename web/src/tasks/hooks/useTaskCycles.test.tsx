import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useTaskCycle, useTaskCycles } from "./useTaskCycles";

function createWrapper() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0, staleTime: 0 } },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

const okJSON = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });

describe("useTaskCycles", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("fetches /tasks/{id}/cycles and parses the typed envelope", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      okJSON({
        task_id: "task-1",
        cycles: [
          {
            id: "cyc-1",
            task_id: "task-1",
            attempt_seq: 1,
            status: "running",
            started_at: "2026-04-18T10:00:00.000Z",
            triggered_by: "user",
            meta: {},
          },
        ],
        limit: 50,
        has_more: false,
      }),
    );

    const { result } = renderHook(() => useTaskCycles("task-1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data?.cycles).toHaveLength(1);
    expect(result.current.data?.cycles[0].status).toBe("running");
    expect(String(fetchSpy.mock.calls[0][0])).toBe("/tasks/task-1/cycles");
  });

  it("forwards limit query param when provided", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      okJSON({ task_id: "task-1", cycles: [], limit: 10, has_more: false }),
    );
    const { result } = renderHook(() => useTaskCycles("task-1", { limit: 10 }), {
      wrapper: createWrapper(),
    });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(String(fetchSpy.mock.calls[0][0])).toBe(
      "/tasks/task-1/cycles?limit=10",
    );
  });

  it("does not fetch when taskId is empty or enabled is false", () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    renderHook(() => useTaskCycles(""), { wrapper: createWrapper() });
    renderHook(() => useTaskCycles("task-1", { enabled: false }), {
      wrapper: createWrapper(),
    });
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("surfaces server errors via React Query", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ error: "task not found" }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const { result } = renderHook(() => useTaskCycles("task-1"), {
      wrapper: createWrapper(),
    });
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
    expect(result.current.error?.message).toContain("task not found");
  });
});

describe("useTaskCycle", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("fetches the cycle detail with phases and parses the envelope", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      okJSON({
        id: "cyc-1",
        task_id: "task-1",
        attempt_seq: 1,
        status: "succeeded",
        started_at: "2026-04-18T10:00:00.000Z",
        ended_at: "2026-04-18T10:01:30.000Z",
        triggered_by: "user",
        meta: {},
        phases: [
          {
            id: "ph-1",
            cycle_id: "cyc-1",
            phase: "diagnose",
            phase_seq: 1,
            status: "succeeded",
            started_at: "2026-04-18T10:00:01.000Z",
            ended_at: "2026-04-18T10:00:30.000Z",
            details: {},
          },
        ],
      }),
    );

    const { result } = renderHook(
      () => useTaskCycle("task-1", "cyc-1"),
      { wrapper: createWrapper() },
    );
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data?.phases[0].phase).toBe("diagnose");
    expect(String(fetchSpy.mock.calls[0][0])).toBe(
      "/tasks/task-1/cycles/cyc-1",
    );
  });

  it("does not fetch when taskId or cycleId is empty", () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    renderHook(() => useTaskCycle("", "cyc-1"), { wrapper: createWrapper() });
    renderHook(() => useTaskCycle("task-1", ""), { wrapper: createWrapper() });
    expect(fetchSpy).not.toHaveBeenCalled();
  });
});
