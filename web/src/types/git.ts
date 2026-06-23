export type GitRepository = {
  id: string;
  project_id: string;
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
  created_at: string;
};

export type GitBranch = {
  id: string;
  repository_id: string;
  name: string;
  head_sha: string;
  created_at: string;
};

export type GitReconcileResult = {
  status: string;
};
