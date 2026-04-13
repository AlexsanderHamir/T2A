import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TaskEvent } from "@/types";
import { useTaskDetailEvents } from "./useTaskDetailEvents";

const { mockListEvents } = vi.hoisted(() => ({ mockListEvents: vi.fn() }));

vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return {
    ...actual,
    listTaskEvents: mockListEvents,
  };
});

const TASK_ID = "11111111-1111-4111-8111-111111111111";
const TASK_B = "22222222-2222-4222-8222-222222222222";

function ev(seq: number): TaskEvent {
  return {
    seq,
    at: "2024-01-01T00:00:00Z",
    type: "sync_ping",
    by: "agent",
    data: {},
  };
}

function createWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

function newQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

describe("useTaskDetailEvents", () => {
  beforeEach(() => {
    mockListEvents.mockReset();
    mockListEvents.mockResolvedValue({
      task_id: TASK_ID,
      events: [ev(30), ev(10)],
      total: 2,
      has_more_newer: false,
      has_more_older: true,
      approval_pending: false,
    });
  });

  it("does not fetch when disabled", () => {
    const qc = newQueryClient();
    renderHook(() => useTaskDetailEvents(TASK_ID, false), {
      wrapper: createWrapper(qc),
    });
    expect(mockListEvents).not.toHaveBeenCalled();
  });

  it("fetches head page when enabled", async () => {
    const qc = newQueryClient();
    renderHook(() => useTaskDetailEvents(TASK_ID, true), {
      wrapper: createWrapper(qc),
    });
    await waitFor(() => {
      expect(mockListEvents).toHaveBeenCalledWith(
        TASK_ID,
        expect.objectContaining({ limit: 20 }),
      );
    });
    expect(mockListEvents.mock.calls[0][1]).not.toHaveProperty("beforeSeq");
    expect(mockListEvents.mock.calls[0][1]).not.toHaveProperty("afterSeq");
  });

  it("resets to head when taskId changes", async () => {
    const qc = newQueryClient();
    const { rerender } = renderHook(
      ({ id, en }: { id: string; en: boolean }) => useTaskDetailEvents(id, en),
      {
        wrapper: createWrapper(qc),
        initialProps: { id: TASK_ID, en: true },
      },
    );
    await waitFor(() => expect(mockListEvents).toHaveBeenCalledWith(TASK_ID, expect.anything()));
    mockListEvents.mockClear();

    rerender({ id: TASK_B, en: true });
    await waitFor(() => {
      expect(mockListEvents).toHaveBeenCalledWith(
        TASK_B,
        expect.not.objectContaining({
          beforeSeq: expect.anything(),
          afterSeq: expect.anything(),
        }),
      );
    });
  });

  it("pager prev requests older events using max seq on page", async () => {
    const qc = newQueryClient();
    const { result } = renderHook(() => useTaskDetailEvents(TASK_ID, true), {
      wrapper: createWrapper(qc),
    });
    await waitFor(() => expect(result.current.eventsQuery.isSuccess).toBe(true));
    mockListEvents.mockClear();

    act(() => {
      result.current.onEventsPagerPrev();
    });

    await waitFor(() => {
      expect(mockListEvents).toHaveBeenCalledWith(
        TASK_ID,
        expect.objectContaining({ afterSeq: 30, limit: 20 }),
      );
    });
  });

  it("pager next requests newer events using min seq on page", async () => {
    const qc = newQueryClient();
    const { result } = renderHook(() => useTaskDetailEvents(TASK_ID, true), {
      wrapper: createWrapper(qc),
    });
    await waitFor(() => expect(result.current.eventsQuery.isSuccess).toBe(true));
    mockListEvents.mockClear();

    act(() => {
      result.current.onEventsPagerNext();
    });

    await waitFor(() => {
      expect(mockListEvents).toHaveBeenCalledWith(
        TASK_ID,
        expect.objectContaining({ beforeSeq: 10, limit: 20 }),
      );
    });
  });

  it("pager callbacks no-op when there are no events", async () => {
    mockListEvents.mockResolvedValue({
      task_id: TASK_ID,
      events: [],
      total: 0,
      approval_pending: false,
    });
    const qc = newQueryClient();
    const { result } = renderHook(() => useTaskDetailEvents(TASK_ID, true), {
      wrapper: createWrapper(qc),
    });
    await waitFor(() => expect(result.current.eventsQuery.isSuccess).toBe(true));
    mockListEvents.mockClear();

    act(() => {
      result.current.onEventsPagerPrev();
      result.current.onEventsPagerNext();
    });
    expect(mockListEvents).not.toHaveBeenCalled();
  });
});
