package handler

import (
	"log/slog"
	"net/http"

	"golang.org/x/sync/errgroup"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/handler/readpolicy"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// bootstrapTasksPayload mirrors listResponse so the SPA can seed
// taskQueryKeys.list directly from bootstrap without a follow-up
// GET /tasks call. The wire shape is identical on purpose.
type bootstrapTasksPayload = listResponse

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
		taskRows []domain.Task
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
		rows, more, err := h.store.ListFlatPage(gctx, readpolicy.BootstrapListLimit, 0, nil)
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
		v, err := h.store.ListProjects(gctx, false, readpolicy.BootstrapProjectsLimit)
		if err == nil {
			projects = v
		}
		return err
	})
	g.Go(func() error {
		v, err := h.store.ListDrafts(gctx, readpolicy.BootstrapDraftsLimit)
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
		Tasks:    buildListResponse(taskRows, readpolicy.BootstrapListLimit, 0, hasMore),
		Stats:    taskStatsResponseFromStore(stats),
		Projects: projectsListResponse{
			Projects: projects,
			Limit:    readpolicy.BootstrapProjectsLimit,
		},
		Drafts: bootstrapDraftsPayload{Drafts: drafts},
	}
	writeJSONWithETag(w, r, op, http.StatusOK, resp)
}
