import { useEffect, useState } from "react";
import {
  probeWorktreeRepo,
  type RepoWorkspaceProbe,
} from "@/api";

/**
 * Probes whether `@` file mentions can search the task's selected worktree.
 *
 * When `worktreeId` is omitted (project-context editors and other surfaces
 * without git binding), the hook settles to `unavailable` without a network
 * call so stale Settings-based hints never appear.
 *
 * When `worktreeId` is passed but empty, the hook also settles immediately —
 * the create form shows a "select a worktree" hint instead of probing.
 *
 * Cleanup aborts the in-flight probe so unmounting the editor cancels the
 * request instead of relying on the 45s `repoFetchCombinedSignal` timeout.
 */
export function useRepoWorkspaceProbe(
  worktreeId?: string,
): RepoWorkspaceProbe | "pending" {
  const [probe, setProbe] = useState<RepoWorkspaceProbe | "pending">("pending");
  const worktreeScoped = worktreeId !== undefined;
  const trimmedWorktreeId = worktreeId?.trim() ?? "";

  useEffect(() => {
    const ac = new AbortController();
    setProbe("pending");

    if (!worktreeScoped || trimmedWorktreeId === "") {
      setProbe({ state: "unavailable" });
      return () => {
        ac.abort();
      };
    }

    void probeWorktreeRepo(trimmedWorktreeId, { signal: ac.signal }).then((p) => {
      if (ac.signal.aborted) return;
      setProbe(p);
    });
    return () => {
      ac.abort();
    };
  }, [worktreeScoped, trimmedWorktreeId]);

  return probe;
}
