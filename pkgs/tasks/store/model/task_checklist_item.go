package model

// TaskChecklistItem is the GORM persistence shape for domain.TaskChecklistItem.
type TaskChecklistItem struct {
	ID        string `gorm:"primaryKey"`
	TaskID    string `gorm:"not null;index"`
	SortOrder int    `gorm:"not null"`
	Text      string `gorm:"not null;type:text"`
}

// TableName pins the task_checklist_items table name.
func (TaskChecklistItem) TableName() string { return "task_checklist_items" }
