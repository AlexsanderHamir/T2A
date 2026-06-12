package ready

import (
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

func applyDequeuableTaskPredicates(q *gorm.DB, db *gorm.DB) *gorm.DB {
	q = q.Where(`NOT EXISTS (
		SELECT 1 FROM task_dependencies td
		INNER JOIN tasks dep ON dep.id = td.depends_on_task_id
		WHERE td.task_id = tasks.id AND (
			(COALESCE(td.satisfies, ?) = ? AND dep.status <> ?)
			OR (td.satisfies = ? AND dep.criteria_satisfied_at IS NULL)
		)
	)`, string(domain.DependencySatisfiesDone), string(domain.DependencySatisfiesDone), domain.StatusDone,
		string(domain.DependencySatisfiesCriteriaComplete))
	if UseSQLiteEventRowID(db) {
		return q.Where("(tasks.gate IS NULL OR json_extract(tasks.gate, '$.status') = ?)", string(domain.GateStatusReleased))
	}
	return q.Where("(tasks.gate IS NULL OR tasks.gate->>'status' = ?)", string(domain.GateStatusReleased))
}
