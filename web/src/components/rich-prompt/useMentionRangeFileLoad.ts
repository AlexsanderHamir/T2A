import { useCallback, useEffect, useState } from "react";
import { fetchRepoFile, type RepoFileResult } from "@/api/repo";
import { errorMessage } from "@/lib/errorMessage";

export type UseMentionRangeFileLoadResult = {
  loading: boolean;
  loadError: string | null;
  file: RepoFileResult | null;
  /** Triggers a fresh fetch using the current `path`. */
  retry: () => void;
};

/**
 * Loads the repo file behind a `@mention` range panel and exposes the three
 * UI states the panel renders.
 *
 * Behaviour:
 *  - On mount and on every `path` change, the previous request is aborted and
 *    a new fetch starts. Loading flips to `true`, `file` and `loadError`
 *    reset.
 *  - `fetchRepoFile` returning `null` means the server reported the workspace
 *    repo is not configured (HTTP 503). We surface a stable "File preview is
 *    unavailable." message instead of silently rendering empty space.
 *  - Thrown errors that are not abort-driven flow through `errorMessage` so
 *    non-`Error` throws still produce a friendly "Load failed" banner.
 *  - Calling `retry()` increments an internal nonce that re-runs the effect
 *    with the same path; aborts/cleanup keep the latest call as the winner.
 */
export function useMentionRangeFileLoad(
  path: string,
): UseMentionRangeFileLoadResult {
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [file, setFile] = useState<RepoFileResult | null>(null);
  const [retryTick, setRetryTick] = useState(0);

  useEffect(() => {
    let active = true;
    const ac = new AbortController();
    setLoading(true);
    setLoadError(null);
    setFile(null);
    void fetchRepoFile(path, { signal: ac.signal })
      .then((r) => {
        if (!active) return;
        if (r === null) {
          setLoadError("File preview is unavailable.");
          return;
        }
        setFile(r);
      })
      .catch((e: unknown) => {
        if (!active || ac.signal.aborted) return;
        setLoadError(errorMessage(e, "Load failed"));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
      ac.abort();
    };
  }, [path, retryTick]);

  const retry = useCallback(() => {
    setRetryTick((t) => t + 1);
  }, []);

  return { loading, loadError, file, retry };
}
