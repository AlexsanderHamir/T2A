export type GitRepository = {
  id: string;
  path: string;
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
  active_branch_id?: string;
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

/** Worktree↔branch association row. */
export type WorktreeBranch = {
  id: string;
  worktree_id: string;
  branch_id: string;
  created_at: string;
};

export type GitReconcileResult = {
  status: string;
};
