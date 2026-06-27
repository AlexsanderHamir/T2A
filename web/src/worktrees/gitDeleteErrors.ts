import { ApiError } from "@/api";

export type GitDeleteTarget =
  | { kind: "repository"; id: string; label: string; repositoryId: string }
  | { kind: "worktree"; id: string; label: string; repositoryId: string };

export function gitDeleteErrorMessage(err: unknown): string {
  if (!(err instanceof ApiError)) {
    return err instanceof Error ? err.message : "Delete failed";
  }
  if (err.code === "branch_checked_out") {
    return "This branch is checked out in another worktree. Switch that worktree to a different branch first.";
  }
  if (err.code === "has_running_task") {
    return err.message || "A task is still running against this git resource.";
  }
  return err.message;
}

export function gitDeleteBlocked(err: unknown): boolean {
  return err instanceof ApiError && err.code === "has_running_task";
}
