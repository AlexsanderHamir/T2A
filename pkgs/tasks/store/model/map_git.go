package model

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"

func FromDomainGitRepository(d domain.GitRepository) GitRepository {
	return GitRepository{
		ID:            d.ID,
		Path:          d.Path,
		HostPath:      d.HostPath,
		DefaultBranch: d.DefaultBranch,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

func FromDomainGitRepositories(rows []domain.GitRepository) []GitRepository {
	if len(rows) == 0 {
		return nil
	}
	out := make([]GitRepository, len(rows))
	for i := range rows {
		out[i] = FromDomainGitRepository(rows[i])
	}
	return out
}

func ToDomainGitRepository(m GitRepository) domain.GitRepository {
	return domain.GitRepository{
		ID:            m.ID,
		Path:          m.Path,
		HostPath:      m.HostPath,
		DefaultBranch: m.DefaultBranch,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

func ToDomainGitRepositories(rows []GitRepository) []domain.GitRepository {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.GitRepository, len(rows))
	for i := range rows {
		out[i] = ToDomainGitRepository(rows[i])
	}
	return out
}

func FromDomainGitWorktree(d domain.GitWorktree) GitWorktree {
	return GitWorktree{
		ID:             d.ID,
		RepositoryID:   d.RepositoryID,
		Path:           d.Path,
		Name:           d.Name,
		IsMain:         d.IsMain,
		ActiveBranchID: d.ActiveBranchID,
		CreatedAt:      d.CreatedAt,
	}
}

func FromDomainGitWorktrees(rows []domain.GitWorktree) []GitWorktree {
	if len(rows) == 0 {
		return nil
	}
	out := make([]GitWorktree, len(rows))
	for i := range rows {
		out[i] = FromDomainGitWorktree(rows[i])
	}
	return out
}

func ToDomainGitWorktree(m GitWorktree) domain.GitWorktree {
	return domain.GitWorktree{
		ID:             m.ID,
		RepositoryID:   m.RepositoryID,
		Path:           m.Path,
		Name:           m.Name,
		IsMain:         m.IsMain,
		ActiveBranchID: m.ActiveBranchID,
		CreatedAt:      m.CreatedAt,
	}
}

func ToDomainGitWorktrees(rows []GitWorktree) []domain.GitWorktree {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.GitWorktree, len(rows))
	for i := range rows {
		out[i] = ToDomainGitWorktree(rows[i])
	}
	return out
}

func FromDomainGitBranch(d domain.GitBranch) GitBranch {
	return GitBranch{
		ID:           d.ID,
		RepositoryID: d.RepositoryID,
		Name:         d.Name,
		HeadSHA:      d.HeadSHA,
		CreatedAt:    d.CreatedAt,
	}
}

func FromDomainGitBranches(rows []domain.GitBranch) []GitBranch {
	if len(rows) == 0 {
		return nil
	}
	out := make([]GitBranch, len(rows))
	for i := range rows {
		out[i] = FromDomainGitBranch(rows[i])
	}
	return out
}

func ToDomainGitBranch(m GitBranch) domain.GitBranch {
	return domain.GitBranch{
		ID:           m.ID,
		RepositoryID: m.RepositoryID,
		Name:         m.Name,
		HeadSHA:      m.HeadSHA,
		CreatedAt:    m.CreatedAt,
	}
}

func ToDomainGitBranches(rows []GitBranch) []domain.GitBranch {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.GitBranch, len(rows))
	for i := range rows {
		out[i] = ToDomainGitBranch(rows[i])
	}
	return out
}

func FromDomainWorktreeBranch(d domain.WorktreeBranch) WorktreeBranch {
	return WorktreeBranch{
		ID:         d.ID,
		WorktreeID: d.WorktreeID,
		BranchID:   d.BranchID,
		CreatedAt:  d.CreatedAt,
	}
}

func ToDomainWorktreeBranch(m WorktreeBranch) domain.WorktreeBranch {
	return domain.WorktreeBranch{
		ID:         m.ID,
		WorktreeID: m.WorktreeID,
		BranchID:   m.BranchID,
		CreatedAt:  m.CreatedAt,
	}
}

func ToDomainWorktreeBranches(rows []WorktreeBranch) []domain.WorktreeBranch {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.WorktreeBranch, len(rows))
	for i := range rows {
		out[i] = ToDomainWorktreeBranch(rows[i])
	}
	return out
}
