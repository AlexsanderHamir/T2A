package handler

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"

type gitReconcileRequest struct {
	BootstrapPath string `json:"bootstrap_path"`
	Repair        bool   `json:"repair"`
	DryRun        bool   `json:"dry_run"`
}

type gitRelocateRepositoryRequest struct {
	Path string `json:"path"`
}

type gitRelocateWorktreeRequest struct {
	Path string `json:"path"`
}

type gitReconcileSkippedJSON struct {
	WorktreeID string `json:"worktree_id"`
	Reason     string `json:"reason"`
}

type gitReconcileNeedsBranchBindJSON struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
}

type gitReconcileReportJSON struct {
	RepoPathUpdated      bool                              `json:"repo_path_updated"`
	WorktreesPathUpdated int                               `json:"worktrees_path_updated"`
	WorktreesAdded       int                               `json:"worktrees_added"`
	WorktreesRemoved     int                               `json:"worktrees_removed"`
	BranchesHeadUpdated  int                               `json:"branches_head_updated"`
	ResolutionSource     string                            `json:"resolution_source,omitempty"`
	DiscoveredPath       string                            `json:"discovered_path,omitempty"`
	WorktreesSkipped     []gitReconcileSkippedJSON         `json:"worktrees_skipped,omitempty"`
	NeedsBranchBind      []gitReconcileNeedsBranchBindJSON `json:"needs_branch_bind,omitempty"`
}

type gitReconcileResponse struct {
	Status string                 `json:"status"`
	Report gitReconcileReportJSON `json:"report"`
}

//funclogmeasure:skip category=hot-path reason="Pure response mapper without I/O; operation trace is emitted by reconcile handlers."
func toGitReconcileResponse(out store.ReconcileGitOutput) gitReconcileResponse {
	skipped := make([]gitReconcileSkippedJSON, 0, len(out.Report.WorktreesSkipped))
	for _, s := range out.Report.WorktreesSkipped {
		skipped = append(skipped, gitReconcileSkippedJSON{
			WorktreeID: s.WorktreeID,
			Reason:     s.Reason,
		})
	}
	needsBind := make([]gitReconcileNeedsBranchBindJSON, 0, len(out.Report.NeedsBranchBind))
	for _, n := range out.Report.NeedsBranchBind {
		needsBind = append(needsBind, gitReconcileNeedsBranchBindJSON{
			Path:   n.Path,
			Branch: n.Branch,
		})
	}
	return gitReconcileResponse{
		Status: out.Status,
		Report: gitReconcileReportJSON{
			RepoPathUpdated:      out.Report.RepoPathUpdated,
			WorktreesPathUpdated: out.Report.WorktreesPathUpdated,
			WorktreesAdded:       out.Report.WorktreesAdded,
			WorktreesRemoved:     out.Report.WorktreesRemoved,
			BranchesHeadUpdated:  out.Report.BranchesHeadUpdated,
			ResolutionSource:     out.Report.ResolutionSource,
			DiscoveredPath:       out.Report.DiscoveredPath,
			WorktreesSkipped:     skipped,
			NeedsBranchBind:      needsBind,
		},
	}
}
