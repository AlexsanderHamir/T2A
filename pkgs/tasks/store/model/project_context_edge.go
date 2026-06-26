package model

import (
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// ProjectContextEdge is the GORM persistence shape for domain.ProjectContextEdge.
type ProjectContextEdge struct {
	ID              string                        `gorm:"primaryKey"`
	ProjectID       string                        `gorm:"not null;index;uniqueIndex:idx_project_context_edge_unique,priority:1"`
	SourceContextID string                        `gorm:"not null;index;uniqueIndex:idx_project_context_edge_unique,priority:2"`
	TargetContextID string                        `gorm:"not null;index;uniqueIndex:idx_project_context_edge_unique,priority:3"`
	Relation        domain.ProjectContextRelation `gorm:"not null;index;uniqueIndex:idx_project_context_edge_unique,priority:4;check:chk_project_context_relation,relation IN ('supports','blocks','refines','depends_on','related')"`
	Strength        int                           `gorm:"not null;default:3;check:chk_project_context_strength,strength >= 1 AND strength <= 5"`
	Note            string                        `gorm:"type:text;not null;default:''"`
	CreatedAt       time.Time                     `gorm:"not null;index"`
	UpdatedAt       time.Time                     `gorm:"not null;index"`

	Project *Project            `gorm:"foreignKey:ProjectID;references:ID;constraint:OnDelete:CASCADE"`
	Source  *ProjectContextItem `gorm:"foreignKey:SourceContextID;references:ID;constraint:OnDelete:CASCADE"`
	Target  *ProjectContextItem `gorm:"foreignKey:TargetContextID;references:ID;constraint:OnDelete:CASCADE"`
}

// TableName pins the project_context_edges table name.
func (ProjectContextEdge) TableName() string { return "project_context_edges" }
