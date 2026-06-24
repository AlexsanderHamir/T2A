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
	ProjectID     string `json:"project_id"`
	Path          string `json:"path"`
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
	ID             string  `json:"id"`
	RepositoryID   string  `json:"repository_id"`
	Path           string  `json:"path"`
	HostPath       string  `json:"host_path"`
	Name           string  `json:"name"`
	IsMain         bool    `json:"is_main"`
	ActiveBranchID *string `json:"active_branch_id,omitempty"`
	CreatedAt      string  `json:"created_at"`
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

type gitReconcileResponse struct {
	Status string `json:"status"`
}

type gitWorktreeRegisterJSON struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type gitWorktreeBranchAssociateJSON struct {
	BranchID     string `json:"branch_id"`
	Name         string `json:"name"`
	StartPoint   string `json:"start_point"`
	CreateBranch bool   `json:"create_branch"`
}

type gitWorktreeBranchesListResponse struct {
	Associations []worktreeBranchJSON `json:"associations"`
}

type worktreeBranchJSON struct {
	ID         string `json:"id"`
	WorktreeID string `json:"worktree_id"`
	BranchID   string `json:"branch_id"`
	CreatedAt  string `json:"created_at"`
}

type gitLiveBranchJSON struct {
	Name    string `json:"name"`
	HeadSHA string `json:"head_sha"`
}

type gitLiveBranchesListResponse struct {
	Branches []gitLiveBranchJSON `json:"branches"`
}
