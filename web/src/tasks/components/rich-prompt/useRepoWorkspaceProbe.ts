import { useEffect, useState } from "react";
import { probeRepoWorkspace, type RepoWorkspaceProbe } from "@/api";

/**
 * Probes whether the running taskapi has a usable workspace repo so the rich
 * prompt editor knows whether to advertise `@` file mentions.
 *
 * Stays "pending" until the first probe resolves, then mirrors the API result.
 * Cleanup aborts the in-flight probe so unmounting the editor (closing the
 * create modal, navigating away) cancels the network request instead of
 * relying on the 45s `searchRepoCombinedSignal` timeout fallback.
 *
 * `probeRepoWorkspace` already swallows network errors and returns
 * `{state: "unknown"}`, so the abort-on-unmount cannot surface as an
 * unhandled rejection.
 */
export function useRepoWorkspaceProbe(): RepoWorkspaceProbe | "pending" {
  const [probe, setProbe] = useState<RepoWorkspaceProbe | "pending">("pending");

  useEffect(() => {
    const ac = new AbortController();
    setProbe("pending");
    void probeRepoWorkspace({ signal: ac.signal }).then((p) => {
      if (ac.signal.aborted) return;
      setProbe(p);
    });
    return () => {
      ac.abort();
    };
  }, []);

  return probe;
}
