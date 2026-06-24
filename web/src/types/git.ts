export type GitRepository = {
  id: string;
  /** Legacy expand-phase field; omitted after contract migration (C8). */
  project_id?: string;
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
