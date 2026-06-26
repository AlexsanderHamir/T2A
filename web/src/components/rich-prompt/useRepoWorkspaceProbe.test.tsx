import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { RepoWorkspaceProbe } from "@/api";
import { useRepoWorkspaceProbe } from "./useRepoWorkspaceProbe";

vi.mock("@/api", async () => {
  const actual = await vi.importActual<typeof import("@/api")>("@/api");
  return {
    ...actual,
    probeWorktreeRepo: vi.fn(),
  };
});

import { probeWorktreeRepo } from "@/api";

const mockedProbe = vi.mocked(probeWorktreeRepo);

type Deferred<T> = {
  promise: Promise<T>;
  resolve: (v: T) => void;
};

function defer<T>(): Deferred<T> {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

describe("useRepoWorkspaceProbe", () => {
  beforeEach(() => {
    mockedProbe.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("starts in pending state and resolves to the probe result for a worktree", async () => {
    const d = defer<RepoWorkspaceProbe>();
    mockedProbe.mockReturnValueOnce(d.promise);

    const { result } = renderHook(() => useRepoWorkspaceProbe("wt-1"));
    expect(result.current).toBe("pending");

    await act(async () => {
      d.resolve({ state: "available" });
    });

    expect(result.current).toEqual({ state: "available" });
    expect(mockedProbe).toHaveBeenCalledWith("wt-1", expect.any(Object));
  });

  it("settles to unavailable immediately when worktreeId is empty", async () => {
    const { result } = renderHook(() => useRepoWorkspaceProbe(""));
    await waitFor(() => {
      expect(result.current).toEqual({ state: "unavailable" });
    });
    expect(mockedProbe).not.toHaveBeenCalled();
  });

  it("settles to unavailable without a network call when worktreeId is omitted", async () => {
    const { result } = renderHook(() => useRepoWorkspaceProbe(undefined));
    await waitFor(() => {
      expect(result.current).toEqual({ state: "unavailable" });
    });
    expect(mockedProbe).not.toHaveBeenCalled();
  });

  it("forwards an abort signal to probeWorktreeRepo", () => {
    mockedProbe.mockReturnValueOnce(new Promise(() => {}));
    renderHook(() => useRepoWorkspaceProbe("wt-1"));
    expect(mockedProbe).toHaveBeenCalledTimes(1);
    const opts = mockedProbe.mock.calls[0][1];
    expect(opts?.signal).toBeInstanceOf(AbortSignal);
    expect(opts?.signal?.aborted).toBe(false);
  });

  it("aborts the in-flight probe on unmount", () => {
    mockedProbe.mockReturnValueOnce(new Promise(() => {}));
    const { unmount } = renderHook(() => useRepoWorkspaceProbe("wt-1"));
    const signal = mockedProbe.mock.calls[0][1]?.signal;
    expect(signal?.aborted).toBe(false);
    unmount();
    expect(signal?.aborted).toBe(true);
  });

  it("ignores a late probe resolution after unmount", async () => {
    const d = defer<RepoWorkspaceProbe>();
    mockedProbe.mockReturnValueOnce(d.promise);
    const { result, unmount } = renderHook(() => useRepoWorkspaceProbe("wt-1"));
    unmount();

    await act(async () => {
      d.resolve({ state: "available" });
    });

    expect(result.current).toBe("pending");
  });

  it("re-probes when the worktree id changes", async () => {
    mockedProbe
      .mockResolvedValueOnce({ state: "available" })
      .mockResolvedValueOnce({ state: "unavailable" });

    const { result, rerender } = renderHook(
      ({ id }: { id: string }) => useRepoWorkspaceProbe(id),
      { initialProps: { id: "wt-a" } },
    );

    await waitFor(() => {
      expect(result.current).toEqual({ state: "available" });
    });

    rerender({ id: "wt-b" });

    await waitFor(() => {
      expect(result.current).toEqual({ state: "unavailable" });
    });
    expect(mockedProbe).toHaveBeenLastCalledWith("wt-b", expect.any(Object));
  });
});
