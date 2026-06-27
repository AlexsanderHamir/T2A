package model

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainProjectContextItem(d domain.ProjectContextItem) ProjectContextItem {
	return ProjectContextItem{
		ID:            d.ID,
		ProjectID:     d.ProjectID,
		Kind:          d.Kind,
		Title:         d.Title,
		Body:          d.Body,
		SourceTaskID:  d.SourceTaskID,
		SourceCycleID: d.SourceCycleID,
		CreatedBy:     d.CreatedBy,
		Pinned:        d.Pinned,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainProjectContextItem(m ProjectContextItem) domain.ProjectContextItem {
	return domain.ProjectContextItem{
		ID:            m.ID,
		ProjectID:     m.ProjectID,
		Kind:          m.Kind,
		Title:         m.Title,
		Body:          m.Body,
		SourceTaskID:  m.SourceTaskID,
		SourceCycleID: m.SourceCycleID,
		CreatedBy:     m.CreatedBy,
		Pinned:        m.Pinned,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainProjectContextItems(rows []ProjectContextItem) []domain.ProjectContextItem {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.ProjectContextItem, len(rows))
	for i := range rows {
		out[i] = ToDomainProjectContextItem(rows[i])
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainProjectContextEdge(d domain.ProjectContextEdge) ProjectContextEdge {
	return ProjectContextEdge{
		ID:              d.ID,
		ProjectID:       d.ProjectID,
		SourceContextID: d.SourceContextID,
		TargetContextID: d.TargetContextID,
		Relation:        d.Relation,
		Strength:        d.Strength,
		Note:            d.Note,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainProjectContextEdge(m ProjectContextEdge) domain.ProjectContextEdge {
	return domain.ProjectContextEdge{
		ID:              m.ID,
		ProjectID:       m.ProjectID,
		SourceContextID: m.SourceContextID,
		TargetContextID: m.TargetContextID,
		Relation:        m.Relation,
		Strength:        m.Strength,
		Note:            m.Note,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainProjectContextEdges(rows []ProjectContextEdge) []domain.ProjectContextEdge {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.ProjectContextEdge, len(rows))
	for i := range rows {
		out[i] = ToDomainProjectContextEdge(rows[i])
	}
	return out
}
