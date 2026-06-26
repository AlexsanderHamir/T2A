import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fetchRepoFile, type RepoFileResult } from "@/api/repo";
import { useMentionRangeFileLoad } from "./useMentionRangeFileLoad";

vi.mock("@/api/repo", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api/repo")>();
  return { ...actual, fetchRepoFile: vi.fn() };
});

const sampleFile: RepoFileResult = {
  path: "src/foo.go",
  content: "hello\nworld\n",
  binary: false,
  truncated: false,
  size_bytes: 12,
  line_count: 2,
};

describe("useMentionRangeFileLoad", () => {
  beforeEach(() => {
    vi.mocked(fetchRepoFile).mockReset();
  });

  it("starts in loading and resolves to file on success", async () => {
    vi.mocked(fetchRepoFile).mockResolvedValue(sampleFile);

    const { result } = renderHook(() =>
      useMentionRangeFileLoad("src/foo.go"),
    );

    expect(result.current.loading).toBe(true);
    expect(result.current.file).toBeNull();
    expect(result.current.loadError).toBeNull();

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.file).toEqual(sampleFile);
    expect(result.current.loadError).toBeNull();
  });

  it("surfaces 'File preview is unavailable.' when the API returns null (503)", async () => {
    vi.mocked(fetchRepoFile).mockResolvedValue(null);

    const { result } = renderHook(() =>
      useMentionRangeFileLoad("src/foo.go"),
    );

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.loadError).toBe("File preview is unavailable.");
    expect(result.current.file).toBeNull();
  });

  it("uses errorMessage with 'Load failed' fallback when the fetch throws a non-Error", async () => {
    vi.mocked(fetchRepoFile).mockRejectedValue("boom");

    const { result } = renderHook(() =>
      useMentionRangeFileLoad("src/foo.go"),
    );

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.loadError).toBe("Load failed");
  });

  it("preserves the original Error message when the fetch rejects with one", async () => {
    vi.mocked(fetchRepoFile).mockRejectedValue(new Error("offline"));

    const { result } = renderHook(() =>
      useMentionRangeFileLoad("src/foo.go"),
    );

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.loadError).toBe("offline");
  });

  it("retry() refetches with the same path and clears prior error state", async () => {
    let calls = 0;
    vi.mocked(fetchRepoFile).mockImplementation(async () => {
      calls += 1;
      if (calls === 1) throw new Error("offline");
      return sampleFile;
    });

    const { result } = renderHook(() =>
      useMentionRangeFileLoad("src/foo.go"),
    );

    await waitFor(() => expect(result.current.loadError).toBe("offline"));

    act(() => {
      result.current.retry();
    });

    expect(result.current.loading).toBe(true);
    expect(result.current.loadError).toBeNull();

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.file).toEqual(sampleFile);
    expect(calls).toBe(2);
  });

  it("aborts the previous request when path changes", async () => {
    const seenSignals: AbortSignal[] = [];
    vi.mocked(fetchRepoFile).mockImplementation(async (_p, init) => {
      if (init?.signal) seenSignals.push(init.signal);
      return sampleFile;
    });

    const { rerender } = renderHook(
      ({ path }: { path: string }) => useMentionRangeFileLoad(path),
      { initialProps: { path: "src/foo.go" } },
    );

    await waitFor(() => expect(seenSignals.length).toBe(1));
    rerender({ path: "src/bar.go" });
    await waitFor(() => expect(seenSignals.length).toBe(2));

    expect(seenSignals[0]?.aborted).toBe(true);
    expect(seenSignals[1]?.aborted).toBe(false);
  });

  it("aborts in-flight request on unmount and does not commit state after", async () => {
    let resolveFile!: (value: RepoFileResult) => void;
    const deferred = new Promise<RepoFileResult>((resolve) => {
      resolveFile = resolve;
    });
    let lastSignal: AbortSignal | undefined;
    vi.mocked(fetchRepoFile).mockImplementation(async (_p, init) => {
      lastSignal = init?.signal;
      return deferred;
    });

    const { result, unmount } = renderHook(() =>
      useMentionRangeFileLoad("src/foo.go"),
    );

    expect(result.current.loading).toBe(true);
    unmount();
    expect(lastSignal?.aborted).toBe(true);

    resolveFile(sampleFile);
    await Promise.resolve();
    // No throw — and no state assertion needed since the hook is unmounted.
  });
});
