package model

import (
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// ParityPair binds a domain struct prototype to its model counterpart for
// schema- and field-parity guards. Later phases append entries here.
type ParityPair struct {
	Name   string
	Domain any
	Model  any
	Table  string
	// DomainMigrateExtra lists additional domain structs AutoMigrate must run
	// before the primary domain type (e.g. parent tables for association FKs).
	// Schema comparison ignores FK constraints; extras exist so domain migrate
	// succeeds when associations reference other tables.
	DomainMigrateExtra []any
}

// ParityPairs is the single registry both parity tests iterate.
var ParityPairs = []ParityPair{
	{
		Name:   "AppSettings",
		Domain: &domain.AppSettings{},
		Model:  &AppSettings{},
		Table:  "app_settings",
	},
	{
		Name:   "TaskEvent",
		Domain: &domain.TaskEvent{},
		Model:  &TaskEvent{},
		Table:  "task_events",
		DomainMigrateExtra: []any{
			&domain.Task{},
		},
	},
}
