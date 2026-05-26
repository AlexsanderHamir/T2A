package tasks

import (
	"encoding/json"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/ready"
	"gorm.io/gorm"
)

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
