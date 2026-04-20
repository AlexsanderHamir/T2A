import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { taskQueryKeys } from "../../../task-query";
import {
  type BulkDeleteResult,
  useBulkDeleteMutation,
} from "./useBulkDeleteMutation";
import {
  type BulkScheduleResult,
  useBulkScheduleMutation,
} from "./useBulkScheduleMutation";

const { mockDeleteTask, mockPatchTask } = vi.hoisted(() => ({
  mockDeleteTask: vi.fn(),
  mockPatchTask: vi.fn(),
}));

vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return {
    ...actual,
    deleteTask: mockDeleteTask,
    patchTask: mockPatchTask,
  };
});

import { deleteTask, patchTask } from "@/api";

const mockedDelete = vi.mocked(deleteTask);
const mockedPatch = vi.mocked(patchTask);

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

function makeWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }
  return { Wrapper, queryClient };
}

describe("useBulkDeleteMutation", () => {
  beforeEach(() => {
    mockedDelete.mockReset();
  });

  it("does not call the API or invalidate when selection is empty", async () => {
    const { Wrapper, queryClient } = makeWrapper();
    const inv = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useBulkDeleteMutation(), {
      wrapper: Wrapper,
    });

    await act(async () => {
      await result.current.run([]);
    });

    expect(mockedDelete).not.toHaveBeenCalled();
    expect(inv).not.toHaveBeenCalled();
  });

  it("still invalidates task queries when some deletes fail (partial success)", async () => {
    const { Wrapper, queryClient } = makeWrapper();
    const inv = vi.spyOn(queryClient, "invalidateQueries");

    mockedDelete
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error("server no"));

    const { result } = renderHook(() => useBulkDeleteMutation(), {
      wrapper: Wrapper,
    });

    await act(async () => {
      await result.current.run(["ok-id", "bad-id"]);
    });

    expect(inv.mock.calls.map((c) => c[0]?.queryKey)).toEqual([
      taskQueryKeys.all,
      taskQueryKeys.stats(),
    ]);
    expect(result.current.lastResult).toMatchObject({
      attempted: 2,
      succeeded: 1,
      failed: [{ taskId: "bad-id" }],
    });
  });
});

describe("useBulkScheduleMutation", () => {
  beforeEach(() => {
    mockedPatch.mockReset();
  });

  it("still invalidates when some patches fail", async () => {
    const { Wrapper, queryClient } = makeWrapper();
    const inv = vi.spyOn(queryClient, "invalidateQueries");

    mockedPatch
      .mockResolvedValueOnce({} as never)
      .mockRejectedValueOnce(new Error("conflict"));

    const { result } = renderHook(() => useBulkScheduleMutation(), {
      wrapper: Wrapper,
    });

    await act(async () => {
      await result.current.run(["a", "b"], "2026-01-01T00:00:00Z");
    });

    expect(inv.mock.calls.map((c) => c[0]?.queryKey)).toEqual([
      taskQueryKeys.all,
      taskQueryKeys.stats(),
    ]);
  });
});

describe("useBulkScheduleMutation overlapping runs", () => {
  beforeEach(() => {
    mockedPatch.mockReset();
  });

  /**
   * Same contract as bulk delete: shared `isPending` + in-flight ref.
   * Split `act` boundaries for React 18 flush semantics (see delete overlap test).
   */
  it("keeps isPending true until every overlapping bulk run has finished", async () => {
    const dSlow = deferred<void>();
    const { Wrapper } = makeWrapper();
    const when = "2026-01-01T00:00:00Z";

    mockedPatch.mockImplementation(async (id: string) => {
      if (id === "slow") {
        await dSlow.promise;
        return {} as never;
      }
      return {} as never;
    });

    const { result } = renderHook(() => useBulkScheduleMutation(), {
      wrapper: Wrapper,
    });

    let slowDone = false;
    let pSlow!: Promise<BulkScheduleResult>;

    await act(() => {
      pSlow = result.current.run(["slow"], when).finally(() => {
        slowDone = true;
      });
    });

    await waitFor(() =>
      expect(mockedPatch).toHaveBeenCalledWith("slow", {
        pickup_not_before: when,
      }),
    );
    expect(result.current.isPending).toBe(true);

    await act(async () => {
      await result.current.run(["fast"], when);
    });
    expect(slowDone).toBe(false);
    expect(result.current.isPending).toBe(true);

    await act(async () => {
      dSlow.resolve(undefined);
      await pSlow;
    });
    expect(slowDone).toBe(true);
    expect(result.current.isPending).toBe(false);
  });
});

describe("useBulkDeleteMutation overlapping runs", () => {
  beforeEach(() => {
    mockedDelete.mockReset();
  });

  /**
   * Overlapping `run()` calls must not clear `isPending` until all complete.
   * Avoid one long `await act(async () => …)` around the overlap: React 18 defers
   * state flushes until that act callback returns, so `isPending` would still
   * read `false` mid-flight.
   */
  it("keeps isPending true until every overlapping bulk run has finished", async () => {
    const dSlow = deferred<void>();
    const { Wrapper } = makeWrapper();

    mockedDelete.mockImplementation(async (id: string) => {
      if (id === "slow") {
        await dSlow.promise;
        return;
      }
      return undefined;
    });

    const { result } = renderHook(() => useBulkDeleteMutation(), {
      wrapper: Wrapper,
    });

    let slowDone = false;
    let pSlow!: Promise<BulkDeleteResult>;

    await act(() => {
      pSlow = result.current.run(["slow"]).finally(() => {
        slowDone = true;
      });
    });

    await waitFor(() => expect(mockedDelete).toHaveBeenCalledWith("slow"));
    expect(result.current.isPending).toBe(true);

    await act(async () => {
      await result.current.run(["fast"]);
    });
    expect(slowDone).toBe(false);
    expect(result.current.isPending).toBe(true);

    await act(async () => {
      dSlow.resolve(undefined);
      await pSlow;
    });
    expect(slowDone).toBe(true);
    expect(result.current.isPending).toBe(false);
  });
});
