import type { GitReconcileReport } from "@/types/git";

const MAX_DETAIL_ROWS = 3;

export function reconcileReportHasFollowUp(report: GitReconcileReport): boolean {
  return report.worktrees_skipped.length > 0 || report.needs_branch_bind.length > 0;
}

export function formatReconcileSkippedReason(reason: string): string {
  switch (reason) {
    case "path_and_branch_not_found":
      return "path not found on disk for bound branch";
    case "branch_checkout_mismatch":
      return "checkout branch does not match Hamix binding";
    case "has_task_ref":
      return "tasks still reference this worktree";
    default:
      return reason.replaceAll("_", " ");
  }
}

export function reconcileSkippedSummary(report: GitReconcileReport): string[] {
  return report.worktrees_skipped.slice(0, MAX_DETAIL_ROWS).map((row) => {
    const detail = formatReconcileSkippedReason(row.reason);
    return row.worktree_id ? `${row.worktree_id.slice(0, 8)}… — ${detail}` : detail;
  });
}

export function reconcileNeedsBindSummary(report: GitReconcileReport): string[] {
  return report.needs_branch_bind.slice(0, MAX_DETAIL_ROWS).map((row) => {
    const branch = row.branch.trim();
    return branch ? `${row.path} (${branch})` : row.path;
  });
}
