package handler

import (
	"log/slog"
	"net/http"

	"golang.org/x/sync/errgroup"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// bootstrapTasksPayload mirrors listResponse so the SPA can seed
// taskQueryKeys.list directly from bootstrap without a follow-up
// GET /tasks call. The wire shape is identical on purpose.
type bootstrapTasksPayload struct {
	Tasks   []store.TaskNode `json:"tasks"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
	HasMore bool             `json:"has_more"`
}

// bootstrapDraftsPayload mirrors the GET /task-drafts envelope so the
// SPA's existing draft list parser consumes it unchanged.
type bootstrapDraftsPayload struct {
	Drafts any `json:"drafts"`
}

// bootstrapResponse is the aggregate payload for GET /v1/bootstrap.
// Each field corresponds to one of the cold-start fetches the SPA used
// to fan out at App mount (settings, root list, stats, projects,
// drafts). Sub-call failure aborts the whole response with 5xx —
// partial bootstrap is more painful for the client to handle than
// falling back to the per-endpoint fan-out it already has to support.
//
// This endpoint is intentionally an *optimization hint*, not the
// canonical shape for the listed resources. Clients that want the
// precise per-endpoint guarantees should keep using GET /tasks,
// GET /settings, etc. — bootstrap clients must tolerate its absence
// (older or stripped-down servers) and gracefully fall back.
type bootstrapResponse struct {
	Settings settingsResponse       `json:"settings"`
	Tasks    bootstrapTasksPayload  `json:"tasks"`
	Stats    taskStatsResponse      `json:"stats"`
	Projects projectsListResponse   `json:"projects"`
	Drafts   bootstrapDraftsPayload `json:"drafts"`
}

// bootstrapDefaultListLimit matches the SPA home-page list fetch
// (web/src/tasks/hooks/useTasksApp.ts: limit 20). Matching the SPA
// guarantees the seeded cache is byte-identical to a fresh
// GET /tasks?limit=20.
const bootstrapDefaultListLimit = 20

// bootstrapDefaultProjectsLimit matches the SPA's AppShell-level
// useProjects call (limit 100) — keeps the seeded projects cache
// useful for the create-task modal and project navigation.
const bootstrapDefaultProjectsLimit = 100

// bootstrapDefaultDraftsLimit matches useTaskCreateFlow.draftsQuery.
const bootstrapDefaultDraftsLimit = 50

// bootstrap serves GET /v1/bootstrap. It composes the five cold-start
// reads in parallel via errgroup so the SPA can seed its TanStack
// Query cache from a single round trip. Any sub-call failure aborts
// the whole response with 5xx and the client falls back to its
// per-endpoint fan-out.
func (h *Handler) bootstrap(w http.ResponseWriter, r *http.Request) {
	const op = "bootstrap.aggregate"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.bootstrap")
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)

	ctx := r.Context()
	var (
		settings store.AppSettings
		taskRows []store.TaskNode
		hasMore  bool
		stats    store.TaskStats
		projects []domain.Project
		drafts   any
	)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		v, err := h.store.GetSettings(gctx)
		if err == nil {
			settings = v
		}
		return err
	})
	g.Go(func() error {
		rows, more, err := h.store.ListRootForest(gctx, bootstrapDefaultListLimit, 0)
		if err == nil {
			taskRows = rows
			hasMore = more
		}
		return err
	})
	g.Go(func() error {
		v, err := h.store.TaskStats(gctx)
		if err == nil {
			stats = v
		}
		return err
	})
	g.Go(func() error {
		v, err := h.store.ListProjects(gctx, false, bootstrapDefaultProjectsLimit)
		if err == nil {
			projects = v
		}
		return err
	})
	g.Go(func() error {
		v, err := h.store.ListDrafts(gctx, bootstrapDefaultDraftsLimit)
		if err == nil {
			drafts = v
		}
		return err
	})
	if err := g.Wait(); err != nil {
		writeStoreError(w, r, op, err)
		return
	}

	resp := bootstrapResponse{
		Settings: settingsResponseFrom(settings),
		Tasks: bootstrapTasksPayload{
			Tasks:   taskRows,
			Limit:   bootstrapDefaultListLimit,
			Offset:  0,
			HasMore: hasMore,
		},
		Stats: taskStatsResponseFromStore(stats),
		Projects: projectsListResponse{
			Projects: projects,
			Limit:    bootstrapDefaultProjectsLimit,
		},
		Drafts: bootstrapDraftsPayload{Drafts: drafts},
	}
	writeJSONWithETag(w, r, op, http.StatusOK, resp)
}
