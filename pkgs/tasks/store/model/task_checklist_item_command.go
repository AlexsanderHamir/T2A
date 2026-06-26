package model

// TaskChecklistItemCommand is the GORM persistence shape for domain.TaskChecklistItemCommand.
type TaskChecklistItemCommand struct {
	ID              string `gorm:"primaryKey"`
	ItemID          string `gorm:"not null;index"`
	SortOrder       int    `gorm:"not null"`
	Command         string `gorm:"not null;type:text"`
	ExpectedOutcome string `gorm:"not null;default:'';type:text"`
}

// TableName pins the task_checklist_item_commands table name.
func (TaskChecklistItemCommand) TableName() string { return "task_checklist_item_commands" }
