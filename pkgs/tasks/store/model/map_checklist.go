package model

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"

func FromDomainTaskChecklistItem(d domain.TaskChecklistItem) TaskChecklistItem {
	return TaskChecklistItem{
		ID:        d.ID,
		TaskID:    d.TaskID,
		SortOrder: d.SortOrder,
		Text:      d.Text,
	}
}

func FromDomainTaskChecklistItemPtr(d *domain.TaskChecklistItem) *TaskChecklistItem {
	if d == nil {
		return nil
	}
	m := FromDomainTaskChecklistItem(*d)
	return &m
}

func ToDomainTaskChecklistItem(m TaskChecklistItem) domain.TaskChecklistItem {
	return domain.TaskChecklistItem{
		ID:        m.ID,
		TaskID:    m.TaskID,
		SortOrder: m.SortOrder,
		Text:      m.Text,
	}
}

func ToDomainTaskChecklistItems(rows []TaskChecklistItem) []domain.TaskChecklistItem {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.TaskChecklistItem, len(rows))
	for i := range rows {
		out[i] = ToDomainTaskChecklistItem(rows[i])
	}
	return out
}

func FromDomainTaskChecklistCompletion(d domain.TaskChecklistCompletion) TaskChecklistCompletion {
	return TaskChecklistCompletion{
		TaskID:            d.TaskID,
		ItemID:            d.ItemID,
		At:                d.At,
		By:                d.By,
		Evidence:          d.Evidence,
		VerifiedBy:        d.VerifiedBy,
		VerifierReasoning: d.VerifierReasoning,
		CycleID:           d.CycleID,
	}
}

func ToDomainTaskChecklistCompletion(m TaskChecklistCompletion) domain.TaskChecklistCompletion {
	return domain.TaskChecklistCompletion{
		TaskID:            m.TaskID,
		ItemID:            m.ItemID,
		At:                m.At,
		By:                m.By,
		Evidence:          m.Evidence,
		VerifiedBy:        m.VerifiedBy,
		VerifierReasoning: m.VerifierReasoning,
		CycleID:           m.CycleID,
	}
}

func ToDomainTaskChecklistCompletions(rows []TaskChecklistCompletion) []domain.TaskChecklistCompletion {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.TaskChecklistCompletion, len(rows))
	for i := range rows {
		out[i] = ToDomainTaskChecklistCompletion(rows[i])
	}
	return out
}

func FromDomainTaskChecklistItemCommand(d domain.TaskChecklistItemCommand) TaskChecklistItemCommand {
	return TaskChecklistItemCommand{
		ID:              d.ID,
		ItemID:          d.ItemID,
		SortOrder:       d.SortOrder,
		Command:         d.Command,
		ExpectedOutcome: d.ExpectedOutcome,
	}
}

func ToDomainTaskChecklistItemCommand(m TaskChecklistItemCommand) domain.TaskChecklistItemCommand {
	return domain.TaskChecklistItemCommand{
		ID:              m.ID,
		ItemID:          m.ItemID,
		SortOrder:       m.SortOrder,
		Command:         m.Command,
		ExpectedOutcome: m.ExpectedOutcome,
	}
}

func ToDomainTaskChecklistItemCommands(rows []TaskChecklistItemCommand) []domain.TaskChecklistItemCommand {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.TaskChecklistItemCommand, len(rows))
	for i := range rows {
		out[i] = ToDomainTaskChecklistItemCommand(rows[i])
	}
	return out
}
