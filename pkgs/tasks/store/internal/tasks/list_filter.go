package tasks

import (
	"encoding/json"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/internal/ready"
	"gorm.io/gorm"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func applyListFilter(q *gorm.DB, db *gorm.DB, filter *ListFilter) *gorm.DB {
	if filter == nil {
		return q
	}
	if filter.Tag != nil {
		tag := strings.TrimSpace(*filter.Tag)
		if tag != "" {
			if ready.UseSQLiteEventRowID(db) {
				q = q.Where("EXISTS (SELECT 1 FROM json_each(tasks.tags) je WHERE je.value = ?)", tag)
			} else {
				b, _ := json.Marshal([]string{tag})
				q = q.Where("tasks.tags @> ?", string(b))
			}
		}
	}
	if filter.Milestone != nil {
		m := strings.TrimSpace(*filter.Milestone)
		if m != "" {
			q = q.Where("tasks.milestone = ?", m)
		}
	}
	return q
}
