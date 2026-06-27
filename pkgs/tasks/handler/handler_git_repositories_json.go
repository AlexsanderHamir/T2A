package handler

type gitRepositoryCreateJSON struct {
	Path          string `json:"path"`
	HostPath      string `json:"host_path"`
	DefaultBranch string `json:"default_branch"`
}

type gitRepositoriesListResponse struct {
	Repositories []gitRepositoryJSON `json:"repositories"`
}

type gitRepositoryJSON struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	GitCommonDir  string `json:"git_common_dir"`
	HostPath      string `json:"host_path"`
	DefaultBranch string `json:"default_branch"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type gitWorktreeCreateJSON struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	Branch       string `json:"branch"`
	CreateBranch bool   `json:"create_branch"`
	StartPoint   string `json:"start_point"`
	ForceRemove  bool   `json:"force_remove"`
}

type gitWorktreesListResponse struct {
	Worktrees []gitWorktreeJSON `json:"worktrees"`
}

type gitWorktreeJSON struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Path         string `json:"path"`
	HostPath     string `json:"host_path"`
	Name         string `json:"name"`
	IsMain       bool   `json:"is_main"`
	BranchID     string `json:"branch_id"`
	CreatedAt    string `json:"created_at"`
}

type gitBranchCreateJSON struct {
	Name       string `json:"name"`
	StartPoint string `json:"start_point"`
	Force      bool   `json:"force"`
}

type gitBranchesListResponse struct {
	Branches []gitBranchJSON `json:"branches"`
}

type gitBranchJSON struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Name         string `json:"name"`
	HeadSHA      string `json:"head_sha"`
	CreatedAt    string `json:"created_at"`
}

type gitWorktreeRegisterJSON struct {
	Path   string                     `json:"path"`
	Name   string                     `json:"name"`
	Branch *gitWorktreeBranchBindJSON `json:"branch,omitempty"`
}

type gitWorktreeBranchBindJSON struct {
	Name         string `json:"name"`
	CreateBranch bool   `json:"create_branch"`
	StartPoint   string `json:"start_point"`
}

type gitLiveBranchJSON struct {
	Name    string `json:"name"`
	HeadSHA string `json:"head_sha"`
}

type gitLiveBranchesListResponse struct {
	Branches []gitLiveBranchJSON `json:"branches"`
}

type gitLiveWorktreeJSON struct {
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	IsMain     bool   `json:"is_main"`
	Detached   bool   `json:"detached"`
	Registered bool   `json:"registered"`
}

type gitLiveWorktreesListResponse struct {
	Worktrees []gitLiveWorktreeJSON `json:"worktrees"`
}

type gitWorktreeProbeResponse struct {
	Path       string `json:"path"`
	Linked     bool   `json:"linked"`
	IsMain     bool   `json:"is_main"`
	Branch     string `json:"branch"`
	Registered bool   `json:"registered"`
}
