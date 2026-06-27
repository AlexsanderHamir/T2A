package model

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainTaskChecklistItem(d domain.TaskChecklistItem) TaskChecklistItem {
	return TaskChecklistItem{
		ID:        d.ID,
		TaskID:    d.TaskID,
		SortOrder: d.SortOrder,
		Text:      d.Text,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainTaskChecklistItemPtr(d *domain.TaskChecklistItem) *TaskChecklistItem {
	if d == nil {
		return nil
	}
	m := FromDomainTaskChecklistItem(*d)
	return &m
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainTaskChecklistItem(m TaskChecklistItem) domain.TaskChecklistItem {
	return domain.TaskChecklistItem{
		ID:        m.ID,
		TaskID:    m.TaskID,
		SortOrder: m.SortOrder,
		Text:      m.Text,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainTaskChecklistItemCommand(d domain.TaskChecklistItemCommand) TaskChecklistItemCommand {
	return TaskChecklistItemCommand{
		ID:              d.ID,
		ItemID:          d.ItemID,
		SortOrder:       d.SortOrder,
		Command:         d.Command,
		ExpectedOutcome: d.ExpectedOutcome,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainTaskChecklistItemCommand(m TaskChecklistItemCommand) domain.TaskChecklistItemCommand {
	return domain.TaskChecklistItemCommand{
		ID:              m.ID,
		ItemID:          m.ItemID,
		SortOrder:       m.SortOrder,
		Command:         m.Command,
		ExpectedOutcome: m.ExpectedOutcome,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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
