package model

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"

func FromDomainTaskCycleCommit(d domain.TaskCycleCommit) TaskCycleCommit {
	return TaskCycleCommit{
		ID:          d.ID,
		TaskID:      d.TaskID,
		CycleID:     d.CycleID,
		PhaseSeq:    d.PhaseSeq,
		Seq:         d.Seq,
		Repo:        d.Repo,
		Worktree:    d.Worktree,
		Branch:      d.Branch,
		SHA:         d.SHA,
		CommittedAt: d.CommittedAt,
		Message:     d.Message,
		RecordedAt:  d.RecordedAt,
	}
}

func FromDomainTaskCycleCommits(rows []domain.TaskCycleCommit) []TaskCycleCommit {
	if len(rows) == 0 {
		return nil
	}
	out := make([]TaskCycleCommit, len(rows))
	for i := range rows {
		out[i] = FromDomainTaskCycleCommit(rows[i])
	}
	return out
}

func ToDomainTaskCycleCommit(m TaskCycleCommit) domain.TaskCycleCommit {
	return domain.TaskCycleCommit{
		ID:          m.ID,
		TaskID:      m.TaskID,
		CycleID:     m.CycleID,
		PhaseSeq:    m.PhaseSeq,
		Seq:         m.Seq,
		Repo:        m.Repo,
		Worktree:    m.Worktree,
		Branch:      m.Branch,
		SHA:         m.SHA,
		CommittedAt: m.CommittedAt,
		Message:     m.Message,
		RecordedAt:  m.RecordedAt,
	}
}

func ToDomainTaskCycleCommits(rows []TaskCycleCommit) []domain.TaskCycleCommit {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.TaskCycleCommit, len(rows))
	for i := range rows {
		out[i] = ToDomainTaskCycleCommit(rows[i])
	}
	return out
}
