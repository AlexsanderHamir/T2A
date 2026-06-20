// Package readpolicy holds shared read-side limits for handler aggregates.
// Constants mirror the SPA query policy in web/src/tasks/queryPolicy.ts and
// useTasksApp so bootstrap and optional shell routes stay aligned with the
// client without importing HTTP or database packages.
package readpolicy

const (
	// BootstrapListLimit matches the SPA home-page task list (limit 20).
	BootstrapListLimit = 20

	// BootstrapProjectsLimit matches AppShell useProjects (limit 100).
	BootstrapProjectsLimit = 100

	// BootstrapDraftsLimit matches useTaskCreateFlow drafts query.
	BootstrapDraftsLimit = 50

	// ShellChecklistIncluded documents that GET /v1/tasks/{id}/shell (when
	// shipped) embeds checklist items alongside the task row.
	ShellChecklistIncluded = true
)
