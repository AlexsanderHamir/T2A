import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { taskQueryKeys } from "../../../task-query";
import { useBulkDeleteMutation } from "./useBulkDeleteMutation";
import { useBulkScheduleMutation } from "./useBulkScheduleMutation";

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

describe("useBulkDeleteMutation overlapping runs", () => {
  /**
   * Overlapping `run()` calls must not clear `isPending` until all complete.
   * `it.fails` until hooks track in-flight depth; remove `.fails` in the fix commit.
   * Kept in its own describe: long-lived `act` + deferred promises; order avoids flakes.
   */
  it.fails("keeps isPending true until every overlapping bulk run has finished", async () => {
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
    const pSlow = act(async () => {
      await result.current.run(["slow"]);
      slowDone = true;
    });

    await waitFor(() => expect(mockedDelete).toHaveBeenCalledWith("slow"));

    await act(async () => {
      await result.current.run(["fast"]);
    });

    try {
      expect(slowDone).toBe(false);
      expect(result.current.isPending).toBe(true);
    } finally {
      await act(async () => {
        dSlow.resolve(undefined);
      });
      await pSlow;
    }
    expect(slowDone).toBe(true);
    expect(result.current.isPending).toBe(false);
  });
});
