package model

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"

// FromDomainProject copies a domain row to its persistence model.
func FromDomainProject(d domain.Project) Project {
	return Project{
		ID:             d.ID,
		Name:           d.Name,
		Description:    d.Description,
		Status:         d.Status,
		ContextSummary: d.ContextSummary,
		RepositoryID:   d.RepositoryID,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

// ToDomainProject copies a persistence row to domain.Project.
func ToDomainProject(m Project) domain.Project {
	return domain.Project{
		ID:             m.ID,
		Name:           m.Name,
		Description:    m.Description,
		Status:         m.Status,
		ContextSummary: m.ContextSummary,
		RepositoryID:   m.RepositoryID,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

// ToDomainProjects maps persistence rows to domain.Project.
func ToDomainProjects(rows []Project) []domain.Project {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.Project, len(rows))
	for i := range rows {
		out[i] = ToDomainProject(rows[i])
	}
	return out
}
