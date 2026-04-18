import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { RepoWorkspaceProbe } from "@/api";
import { useRepoWorkspaceProbe } from "./useRepoWorkspaceProbe";

vi.mock("@/api", async () => {
  const actual = await vi.importActual<typeof import("@/api")>("@/api");
  return {
    ...actual,
    probeRepoWorkspace: vi.fn(),
  };
});

import { probeRepoWorkspace } from "@/api";

const mockedProbe = vi.mocked(probeRepoWorkspace);

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

  it("starts in pending state and resolves to the probe result", async () => {
    const d = defer<RepoWorkspaceProbe>();
    mockedProbe.mockReturnValueOnce(d.promise);

    const { result } = renderHook(() => useRepoWorkspaceProbe());
    expect(result.current).toBe("pending");

    await act(async () => {
      d.resolve({ state: "available" });
    });

    expect(result.current).toEqual({ state: "available" });
  });

  it("forwards an abort signal to probeRepoWorkspace", () => {
    mockedProbe.mockReturnValueOnce(new Promise(() => {}));
    renderHook(() => useRepoWorkspaceProbe());
    expect(mockedProbe).toHaveBeenCalledTimes(1);
    const opts = mockedProbe.mock.calls[0][0];
    expect(opts?.signal).toBeInstanceOf(AbortSignal);
    expect(opts?.signal?.aborted).toBe(false);
  });

  it("aborts the in-flight probe on unmount", () => {
    mockedProbe.mockReturnValueOnce(new Promise(() => {}));
    const { unmount } = renderHook(() => useRepoWorkspaceProbe());
    const signal = mockedProbe.mock.calls[0][0]?.signal;
    expect(signal?.aborted).toBe(false);
    unmount();
    expect(signal?.aborted).toBe(true);
  });

  it("ignores a late probe resolution after unmount", async () => {
    const d = defer<RepoWorkspaceProbe>();
    mockedProbe.mockReturnValueOnce(d.promise);
    const { result, unmount } = renderHook(() => useRepoWorkspaceProbe());
    unmount();

    await act(async () => {
      d.resolve({ state: "available" });
    });

    expect(result.current).toBe("pending");
  });

  it("settles to broken when the probe resolves to broken", async () => {
    const d = defer<RepoWorkspaceProbe>();
    mockedProbe.mockReturnValueOnce(d.promise);
    const { result } = renderHook(() => useRepoWorkspaceProbe());

    await act(async () => {
      d.resolve({ state: "broken" });
    });

    await waitFor(() => {
      expect(result.current).toEqual({ state: "broken" });
    });
  });
});
