import { ApiError } from "@/api";
import type { GitReconcileReport, GitReconcileResult } from "@/types/git";

export function gitReconcileErrorMessage(err: unknown): string {
  if (!(err instanceof ApiError)) {
    return err instanceof Error ? err.message : "Reconcile failed";
  }
  if (err.code === "bootstrap_mismatch") {
    return "That checkout is not the same repository Hamix registered. Choose the main worktree for this repo.";
  }
  if (err.code === "has_running_task") {
    return err.message || "A task is still running against this repository or one of its worktrees.";
  }
  return err.message;
}

export function formatReconcileSuccess(result: GitReconcileResult): string {
  const { report, status } = result;
  const parts = summarizeReconcileReport(report);
  if (parts.length === 0) {
    return status === "partial"
      ? "Reconcile finished with unresolved items — review the repository card."
      : "Repository is up to date.";
  }
  const prefix = status === "partial" ? "Reconcile finished with updates" : "Reconcile complete";
  return `${prefix}: ${parts.join(", ")}.`;
}

function summarizeReconcileReport(report: GitReconcileReport): string[] {
  const parts: string[] = [];
  if (report.resolution_source === "discovered" && report.discovered_path) {
    parts.push(`found repository at ${report.discovered_path}`);
  }
  if (report.repo_path_updated) {
    parts.push("repository path updated");
  }
  if (report.worktrees_path_updated > 0) {
    const n = report.worktrees_path_updated;
    parts.push(`${n} worktree path${n === 1 ? "" : "s"} updated`);
  }
  if (report.worktrees_added > 0) {
    const n = report.worktrees_added;
    parts.push(`${n} worktree${n === 1 ? "" : "s"} added`);
  }
  if (report.worktrees_removed > 0) {
    const n = report.worktrees_removed;
    parts.push(`${n} worktree${n === 1 ? "" : "s"} removed`);
  }
  if (report.branches_head_updated > 0) {
    const n = report.branches_head_updated;
    parts.push(`${n} branch head${n === 1 ? "" : "s"} refreshed`);
  }
  return parts;
}
