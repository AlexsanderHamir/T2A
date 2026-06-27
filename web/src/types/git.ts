export type GitRepository = {
  id: string;
  path: string;
  git_common_dir: string;
  host_path: string;
  default_branch: string;
  created_at: string;
  updated_at: string;
};

export type GitWorktree = {
  id: string;
  repository_id: string;
  path: string;
  name: string;
  is_main: boolean;
  branch_id?: string;
  created_at: string;
};

export type GitBranch = {
  id: string;
  repository_id: string;
  name: string;
  head_sha: string;
  created_at: string;
};

/** Live ref from `GET /git/repositories/{repoId}/branches/live`. */
export type GitLiveBranch = {
  name: string;
  head_sha: string;
};

/** Linked worktree from `GET /git/repositories/{repoId}/worktrees/live`. */
export type GitLiveWorktree = {
  path: string;
  branch: string;
  is_main: boolean;
  detached: boolean;
  registered: boolean;
};

export type GitWorktreeBranchBind = {
  name: string;
  create_branch?: boolean;
  start_point?: string;
};

/** Probe result from `GET /git/repositories/{repoId}/worktrees/probe`. */
export type GitWorktreeProbe = {
  path: string;
  linked: boolean;
  is_main: boolean;
  branch: string;
  registered: boolean;
};

export type GitReconcileSkipped = {
  worktree_id: string;
  reason: string;
};

export type GitReconcileNeedsBranchBind = {
  path: string;
  branch: string;
};

export type GitReconcileReport = {
  repo_path_updated: boolean;
  worktrees_path_updated: number;
  worktrees_added: number;
  worktrees_removed: number;
  branches_head_updated: number;
  worktrees_skipped: GitReconcileSkipped[];
  needs_branch_bind: GitReconcileNeedsBranchBind[];
};

export type GitReconcileResult = {
  status: string;
  report: GitReconcileReport;
};

export type GitReconcileInput = {
  bootstrap_path?: string;
  repair?: boolean;
  dry_run?: boolean;
};
