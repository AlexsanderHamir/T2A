package model

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainTaskCycleCriteriaReport(d domain.TaskCycleCriteriaReport) TaskCycleCriteriaReport {
	return TaskCycleCriteriaReport{
		ID:          d.ID,
		CycleID:     d.CycleID,
		AttemptSeq:  d.AttemptSeq,
		CriterionID: d.CriterionID,
		ClaimedDone: d.ClaimedDone,
		Evidence:    d.Evidence,
		WrittenAt:   d.WrittenAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainTaskCycleCriteriaReports(rows []domain.TaskCycleCriteriaReport) []TaskCycleCriteriaReport {
	if len(rows) == 0 {
		return nil
	}
	out := make([]TaskCycleCriteriaReport, len(rows))
	for i := range rows {
		out[i] = FromDomainTaskCycleCriteriaReport(rows[i])
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainTaskCycleCriteriaReport(m TaskCycleCriteriaReport) domain.TaskCycleCriteriaReport {
	return domain.TaskCycleCriteriaReport{
		ID:          m.ID,
		CycleID:     m.CycleID,
		AttemptSeq:  m.AttemptSeq,
		CriterionID: m.CriterionID,
		ClaimedDone: m.ClaimedDone,
		Evidence:    m.Evidence,
		WrittenAt:   m.WrittenAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainTaskCycleCriteriaReportPtr(m *TaskCycleCriteriaReport) *domain.TaskCycleCriteriaReport {
	if m == nil {
		return nil
	}
	d := ToDomainTaskCycleCriteriaReport(*m)
	return &d
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainTaskCycleCriteriaReports(rows []TaskCycleCriteriaReport) []domain.TaskCycleCriteriaReport {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.TaskCycleCriteriaReport, len(rows))
	for i := range rows {
		out[i] = ToDomainTaskCycleCriteriaReport(rows[i])
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainTaskCycleVerifyReport(d domain.TaskCycleVerifyReport) TaskCycleVerifyReport {
	return TaskCycleVerifyReport{
		ID:           d.ID,
		CycleID:      d.CycleID,
		AttemptSeq:   d.AttemptSeq,
		CriterionID:  d.CriterionID,
		Verified:     d.Verified,
		VerifierKind: d.VerifierKind,
		Reasoning:    d.Reasoning,
		WrittenAt:    d.WrittenAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainTaskCycleVerifyReports(rows []domain.TaskCycleVerifyReport) []TaskCycleVerifyReport {
	if len(rows) == 0 {
		return nil
	}
	out := make([]TaskCycleVerifyReport, len(rows))
	for i := range rows {
		out[i] = FromDomainTaskCycleVerifyReport(rows[i])
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainTaskCycleVerifyReport(m TaskCycleVerifyReport) domain.TaskCycleVerifyReport {
	return domain.TaskCycleVerifyReport{
		ID:           m.ID,
		CycleID:      m.CycleID,
		AttemptSeq:   m.AttemptSeq,
		CriterionID:  m.CriterionID,
		Verified:     m.Verified,
		VerifierKind: m.VerifierKind,
		Reasoning:    m.Reasoning,
		WrittenAt:    m.WrittenAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainTaskCycleVerifyReports(rows []TaskCycleVerifyReport) []domain.TaskCycleVerifyReport {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.TaskCycleVerifyReport, len(rows))
	for i := range rows {
		out[i] = ToDomainTaskCycleVerifyReport(rows[i])
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainTaskCycleCommandRun(d domain.TaskCycleCommandRun) TaskCycleCommandRun {
	return TaskCycleCommandRun{
		ID:          d.ID,
		CycleID:     d.CycleID,
		AttemptSeq:  d.AttemptSeq,
		CriterionID: d.CriterionID,
		CommandSeq:  d.CommandSeq,
		ExitCode:    d.ExitCode,
		MetaPath:    d.MetaPath,
		WrittenAt:   d.WrittenAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainTaskCycleCommandRuns(rows []domain.TaskCycleCommandRun) []TaskCycleCommandRun {
	if len(rows) == 0 {
		return nil
	}
	out := make([]TaskCycleCommandRun, len(rows))
	for i := range rows {
		out[i] = FromDomainTaskCycleCommandRun(rows[i])
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainTaskCycleCommandRun(m TaskCycleCommandRun) domain.TaskCycleCommandRun {
	return domain.TaskCycleCommandRun{
		ID:          m.ID,
		CycleID:     m.CycleID,
		AttemptSeq:  m.AttemptSeq,
		CriterionID: m.CriterionID,
		CommandSeq:  m.CommandSeq,
		ExitCode:    m.ExitCode,
		MetaPath:    m.MetaPath,
		WrittenAt:   m.WrittenAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainTaskCycleCommandRuns(rows []TaskCycleCommandRun) []domain.TaskCycleCommandRun {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.TaskCycleCommandRun, len(rows))
	for i := range rows {
		out[i] = ToDomainTaskCycleCommandRun(rows[i])
	}
	return out
}
