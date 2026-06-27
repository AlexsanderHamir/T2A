package handler

import (
	"context"
	"log/slog"
	"net/http"
	"os/exec"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/repo"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/postgres"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// Task routes: see README.md (handler_task_*.go). /repo: repo_handlers.go. SSE: sse.go.
// Settings routes: handler_settings.go (GET/PATCH /settings, POST /settings/probe-cursor,
// POST /settings/list-cursor-models, POST /settings/cancel-current-run).
// Runner routes: handler_runners.go (GET /runners, GET /runners/{id}/config-schema,
// POST /runners/{id}/probe, POST /runners/{id}/list-models,
// POST /runners/{id}/validate-config).

// AgentWorkerControl is the narrow surface the /settings handlers use
// to drive the in-process agent worker. The cmd/taskapi supervisor
// implements it; tests can stub it out (or pass nil to disable the
// supervisor-aware endpoints — they then return 503).
//
// Reload is invoked after PATCH /settings persists so the worker
// picks up the new config without a process restart. CancelCurrentRun
// is the explicit "stop the runaway run" knob exposed at
// POST /settings/cancel-current-run; it returns true when there was
// an in-flight run to cancel. ProbeRunner is invoked from POST
// /settings/probe-cursor so the SPA can validate a binary path
// against the configured runner before saving.
type AgentWorkerControl interface {
	CancelCurrentRun() bool
	Reload(ctx context.Context) error
	ProbeRunner(ctx context.Context, runnerID, binaryPath string, timeout time.Duration) (version, resolvedBin string, err error)
}

// Handler carries dependencies for the mounted REST routes, SSE stream, repo
// helpers, and optional agent worker control. Use NewHandler; the zero value
// is not usable.
type Handler struct {
	store          *store.Store
	hub            *SSEHub
	repoProv       RepoProvider
	agent          AgentWorkerControl
	systemHealthFn systemHealthSnapshotter
	git            gitwork.Service
	pathMap        *PathMap
	gitAvailable   bool
	schemaDrift    postgres.SchemaDriftReport
}

// NewHandler returns the task REST API and GET /events (SSE) when hub is non-nil.
//
// rep is the legacy static workspace root: pass nil to disable /repo
// routes (they return 409 repo_root_not_configured) or pre-open one
// for tests that want a fixed tmpdir. The production wiring should
// instead pass nil here and call WithRepoProvider with a settings-
// backed provider so the repo follows AppSettings.RepoRoot live.
//
// agent is optional: when nil, settings-control endpoints (PATCH /settings,
// POST /settings/probe-cursor, POST /settings/cancel-current-run) respond 503.
// GET /settings still works without it (read-only).
func NewHandler(s *store.Store, hub *SSEHub, rep *repo.Root, opts ...HandlerOption) http.Handler {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.NewHandler")
	_, gitErr := exec.LookPath("git")
	h := &Handler{
		store:        s,
		hub:          hub,
		repoProv:     NewStaticRepoProvider(rep),
		git:          gitwork.New(),
		pathMap:      &PathMap{},
		gitAvailable: gitErr == nil,
	}
	for _, opt := range opts {
		opt(h)
	}
	m := http.NewServeMux()
	m.Handle("GET /health", http.HandlerFunc(health))
	m.Handle("GET /health/live", http.HandlerFunc(healthLive))
	m.Handle("GET /health/ready", http.HandlerFunc(h.healthReady))
	m.Handle("GET /system/health", http.HandlerFunc(h.systemHealth))
	m.Handle("GET /events", http.HandlerFunc(h.streamEvents))
	m.Handle("GET /projects", http.HandlerFunc(h.listProjects))
	m.Handle("POST /projects", http.HandlerFunc(h.createProject))
	m.Handle("GET /projects/{id}", http.HandlerFunc(h.getProject))
	m.Handle("PATCH /projects/{id}", http.HandlerFunc(h.patchProject))
	m.Handle("DELETE /projects/{id}", http.HandlerFunc(h.deleteProject))
	m.Handle("GET /projects/{id}/context", http.HandlerFunc(h.listProjectContext))
	m.Handle("POST /projects/{id}/context", http.HandlerFunc(h.createProjectContext))
	m.Handle("POST /projects/{id}/context/edges", http.HandlerFunc(h.createProjectContextEdge))
	m.Handle("PATCH /projects/{id}/context/edges/{edgeId}", http.HandlerFunc(h.patchProjectContextEdge))
	m.Handle("DELETE /projects/{id}/context/edges/{edgeId}", http.HandlerFunc(h.deleteProjectContextEdge))
	m.Handle("PATCH /projects/{id}/context/{contextId}", http.HandlerFunc(h.patchProjectContext))
	m.Handle("DELETE /projects/{id}/context/{contextId}", http.HandlerFunc(h.deleteProjectContext))
	m.Handle("GET /projects/{id}/git/repositories", http.HandlerFunc(h.listGitRepositories))
	m.Handle("POST /projects/{id}/git/repositories", http.HandlerFunc(h.createGitRepository))
	m.Handle("GET /projects/{id}/git/repositories/{repoId}", http.HandlerFunc(h.getGitRepository))
	m.Handle("DELETE /projects/{id}/git/repositories/{repoId}", http.HandlerFunc(h.deleteGitRepository))
	m.Handle("GET /projects/{id}/git/repositories/{repoId}/worktrees", http.HandlerFunc(h.listGitWorktrees))
	m.Handle("POST /projects/{id}/git/repositories/{repoId}/worktrees", http.HandlerFunc(h.createGitWorktree))
	m.Handle("DELETE /projects/{id}/git/worktrees/{worktreeId}", http.HandlerFunc(h.deleteGitWorktree))
	m.Handle("GET /projects/{id}/git/repositories/{repoId}/branches", http.HandlerFunc(h.listGitBranches))
	m.Handle("POST /projects/{id}/git/repositories/{repoId}/branches", http.HandlerFunc(h.createGitBranch))
	m.Handle("DELETE /projects/{id}/git/branches/{branchId}", http.HandlerFunc(h.deleteGitBranch))
	m.Handle("POST /projects/{id}/git/repositories/{repoId}/reconcile", http.HandlerFunc(h.reconcileGitRepository))
	m.Handle("GET /git/repositories", http.HandlerFunc(h.listGlobalGitRepositories))
	m.Handle("POST /git/repositories", http.HandlerFunc(h.createGlobalGitRepository))
	m.Handle("GET /git/repositories/{repoId}", http.HandlerFunc(h.getGlobalGitRepository))
	m.Handle("DELETE /git/repositories/{repoId}", http.HandlerFunc(h.deleteGlobalGitRepository))
	m.Handle("GET /git/repositories/{repoId}/worktrees", http.HandlerFunc(h.listGlobalGitWorktrees))
	m.Handle("GET /git/repositories/{repoId}/worktrees/live", http.HandlerFunc(h.listGlobalGitWorktreesLive))
	m.Handle("POST /git/repositories/{repoId}/worktrees", http.HandlerFunc(h.createGlobalGitWorktree))
	m.Handle("POST /git/repositories/{repoId}/worktrees/register", http.HandlerFunc(h.registerGlobalGitWorktree))
	m.Handle("POST /git/repositories/{repoId}/reconcile", http.HandlerFunc(h.reconcileGlobalGitRepository))
	m.Handle("DELETE /git/worktrees/{worktreeId}", http.HandlerFunc(h.deleteGlobalGitWorktree))
	m.Handle("GET /git/repositories/{repoId}/branches", http.HandlerFunc(h.listGlobalGitBranches))
	m.Handle("GET /git/repositories/{repoId}/branches/live", http.HandlerFunc(h.listGlobalGitBranchesLive))
	m.Handle("GET /git/repositories/{repoId}/projects", http.HandlerFunc(h.listRepoProjects))
	m.Handle("POST /tasks", http.HandlerFunc(h.create))
	m.Handle("GET /task-drafts", http.HandlerFunc(h.listTaskDrafts))
	m.Handle("POST /task-drafts", http.HandlerFunc(h.saveTaskDraft))
	m.Handle("GET /task-drafts/{id}", http.HandlerFunc(h.getTaskDraft))
	m.Handle("DELETE /task-drafts/{id}", http.HandlerFunc(h.deleteTaskDraft))
	m.Handle("GET /task-templates", http.HandlerFunc(h.listTaskTemplates))
	m.Handle("POST /task-templates", http.HandlerFunc(h.saveTaskTemplate))
	m.Handle("GET /task-templates/{id}", http.HandlerFunc(h.getTaskTemplate))
	m.Handle("PATCH /task-templates/{id}", http.HandlerFunc(h.patchTaskTemplate))
	m.Handle("DELETE /task-templates/{id}", http.HandlerFunc(h.deleteTaskTemplate))
	m.Handle("POST /task-templates/instantiate", http.HandlerFunc(h.instantiateTaskTemplates))
	m.Handle("GET /tasks", http.HandlerFunc(h.list))
	m.Handle("GET /tasks/stats", http.HandlerFunc(h.stats))
	m.Handle("GET /tasks/cycle-failures", http.HandlerFunc(h.cycleFailures))
	m.Handle("GET /tasks/{id}/checklist", http.HandlerFunc(h.getChecklist))
	m.Handle("POST /tasks/{id}/checklist/items", http.HandlerFunc(h.postChecklistItem))
	m.Handle("PATCH /tasks/{id}/checklist/items/{itemId}", http.HandlerFunc(h.patchChecklistItem))
	m.Handle("DELETE /tasks/{id}/checklist/items/{itemId}", http.HandlerFunc(h.deleteChecklistItem))
	m.Handle("GET /tasks/{id}/events/{seq}", http.HandlerFunc(h.taskEvent))
	m.Handle("PATCH /tasks/{id}/events/{seq}", http.HandlerFunc(h.patchTaskEventUserResponse))
	m.Handle("GET /tasks/{id}/events", http.HandlerFunc(h.taskEvents))
	m.Handle("POST /tasks/{id}/cycles", http.HandlerFunc(h.postTaskCycle))
	m.Handle("GET /tasks/{id}/cycles", http.HandlerFunc(h.getTaskCycles))
	m.Handle("GET /tasks/{id}/cycles/{cycleId}/stream", http.HandlerFunc(h.getTaskCycleStream))
	m.Handle("GET /tasks/{id}/commits", http.HandlerFunc(h.getTaskCommits))
	m.Handle("GET /tasks/{id}/cycles/{cycleId}/verdicts", http.HandlerFunc(h.getTaskCycleVerdicts))
	m.Handle("GET /tasks/{id}/cycles/{cycleId}", http.HandlerFunc(h.getTaskCycle))
	m.Handle("PATCH /tasks/{id}/cycles/{cycleId}", http.HandlerFunc(h.patchTaskCycle))
	m.Handle("POST /tasks/{id}/cycles/{cycleId}/phases", http.HandlerFunc(h.postTaskCyclePhase))
	m.Handle("PATCH /tasks/{id}/cycles/{cycleId}/phases/{phaseSeq}", http.HandlerFunc(h.patchTaskCyclePhase))
	m.Handle("GET /tasks/{id}/dependencies", http.HandlerFunc(h.listTaskDependencies))
	m.Handle("POST /tasks/{id}/dependencies", http.HandlerFunc(h.addTaskDependency))
	m.Handle("DELETE /tasks/{id}/dependencies/{depId}", http.HandlerFunc(h.removeTaskDependency))
	m.Handle("PATCH /tasks/{id}/gate", http.HandlerFunc(h.patchTaskGate))
	m.Handle("POST /tasks/{id}/retry", http.HandlerFunc(h.postTaskRetry))
	m.Handle("GET /tasks/{id}", http.HandlerFunc(h.get))
	m.Handle("PATCH /tasks/{id}", http.HandlerFunc(h.patch))
	m.Handle("DELETE /tasks/{id}", http.HandlerFunc(h.delete))
	m.Handle("GET /repo/search", http.HandlerFunc(h.repoSearch))
	m.Handle("GET /repo/file", http.HandlerFunc(h.repoFile))
	m.Handle("GET /repo/validate-range", http.HandlerFunc(h.repoValidateRange))
	m.Handle("GET /repo/diff", http.HandlerFunc(h.repoDiff))
	m.Handle("GET /settings", http.HandlerFunc(h.getSettings))
	m.Handle("GET /settings/workspace-roots", http.HandlerFunc(h.workspaceRoots))
	m.Handle("GET /settings/browse-dirs", http.HandlerFunc(h.browseDirs))
	m.Handle("GET /settings/git-probe", http.HandlerFunc(h.gitRepositoryProbe))
	m.Handle("PATCH /settings", http.HandlerFunc(h.patchSettings))
	m.Handle("POST /settings/probe-cursor", http.HandlerFunc(h.probeCursor))
	m.Handle("POST /settings/list-cursor-models", http.HandlerFunc(h.listCursorModels))
	m.Handle("POST /settings/cancel-current-run", http.HandlerFunc(h.cancelCurrentRun))
	m.Handle("GET /runners", http.HandlerFunc(h.listRunners))
	m.Handle("GET /runners/{id}/config-schema", http.HandlerFunc(h.runnerConfigSchema))
	m.Handle("POST /runners/{id}/probe", http.HandlerFunc(h.probeRunner))
	m.Handle("POST /runners/{id}/list-models", http.HandlerFunc(h.listRunnerModels))
	m.Handle("POST /runners/{id}/validate-config", http.HandlerFunc(h.validateRunnerConfig))
	// /v1/rum is the SPA-side Real User Monitoring beacon. Documented
	// in docs/architecture.md; the browser ships batches via
	// `navigator.sendBeacon` so the server returns 204 with no body.
	// Rate-limited via the global per-IP middleware (WithRateLimit),
	// not separately, so a misbehaving SPA cannot amplify a load
	// incident into a metrics-storage bill.
	m.Handle("POST /v1/rum", http.HandlerFunc(h.postRUM))
	// /v1/bootstrap is the cold-start aggregate the SPA uses to seed
	// its TanStack Query cache from a single round trip — combines
	// settings, root tasks page, stats, projects, and drafts head.
	// Documented in docs/api.md; clients must tolerate 5xx and fall
	// back to per-endpoint fan-out.
	m.Handle("GET /v1/bootstrap", http.HandlerFunc(h.bootstrap))
	return m
}

// HandlerOption configures the Handler at construction time. Optional
// because most callers (tests, embedding) only need the core surface.
type HandlerOption func(*Handler)

// WithAgentWorkerControl wires the supervisor that owns the in-process
// agent worker so PATCH /settings can hot-reload, POST
// /settings/probe-cursor can probe the runner, and POST
// /settings/cancel-current-run can cancel an in-flight run. Pass nil
// (or omit the option) to disable those endpoints — they then return
// 503 service_unavailable and GET /settings still works.
func WithAgentWorkerControl(c AgentWorkerControl) HandlerOption {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.WithAgentWorkerControl")
	return func(h *Handler) {
		h.agent = c
	}
}

// WithRepoProvider replaces the default static repo wiring with a
// dynamic provider. cmd/taskapi passes a NewSettingsRepoProvider so
// /repo/* and prompt-mention validation always look at the current
// AppSettings.RepoRoot; tests rarely need this option (the rep
// argument to NewHandler covers the static case).
func WithRepoProvider(p RepoProvider) HandlerOption {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.WithRepoProvider")
	return func(h *Handler) {
		if p != nil {
			h.repoProv = p
		}
	}
}

// WithSchemaDriftReport wires startup schema revision drift for GET /health/ready.
func WithSchemaDriftReport(r postgres.SchemaDriftReport) HandlerOption {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.WithSchemaDriftReport")
	return func(h *Handler) {
		h.schemaDrift = r
	}
}
