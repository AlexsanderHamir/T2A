package store

import (
	"context"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist"
	"gorm.io/gorm"
)

// BackfillCriteriaSatisfiedAt sets criteria_satisfied_at for tasks whose
// checklist is already complete. Idempotent migration helper.
func BackfillCriteriaSatisfiedAt(ctx context.Context, db *gorm.DB) error {
	return checklist.BackfillCriteriaSatisfiedAt(ctx, db)
}
