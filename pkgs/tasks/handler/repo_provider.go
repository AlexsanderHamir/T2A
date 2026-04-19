package handler

import (
	"context"
	"log/slog"
	"sync"

	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// RepoProvider returns the *repo.Root that /repo/* handlers and prompt
// mention validation should consult for the current request. The
// indirection lets the production wiring read AppSettings.RepoRoot at
// request time (so PATCH /settings flips behaviour without a restart)
// while tests can pin a static dir via NewStaticRepoProvider.
//
// The reason string distinguishes "not configured" (operator hasn't
// picked a repo yet) from "open failed" (path went away or stopped
// being a directory). Handlers map the former to HTTP 409 and the
// latter to HTTP 500 so the SPA can render the right banner.
type RepoProvider interface {
	Repo(ctx context.Context) (root *repo.Root, reason string, err error)
}

// Sentinel reason strings returned by RepoProvider.Repo. The wire
// values are pinned because the SPA renders different banners /
// error messages keyed off them and the values are documented in
// docs/SETTINGS.md.
const (
	// RepoReasonNotConfigured: AppSettings.RepoRoot is empty. Handlers
	// reply 409 repo_root_not_configured so the SPA can show a "Pick
	// a workspace" banner with a link to the Settings page.
	RepoReasonNotConfigured = "repo_root_not_configured"
	// RepoReasonOpenFailed: AppSettings.RepoRoot is set but
	// repo.OpenRoot rejected it (missing dir, not a directory, symlink
	// loop). Handlers reply 500 with the OpenRoot error so the
	// operator can fix the path or filesystem.
	RepoReasonOpenFailed = "repo_root_open_failed"
)

// staticRepoProvider returns a fixed *repo.Root regardless of context.
// Used by tests that want to pin a tmpdir or by handlers that want to
// preserve the legacy "rep argument to NewHandler" behaviour for code
// paths that haven't migrated to AppSettings yet.
type staticRepoProvider struct {
	root *repo.Root
}

// NewStaticRepoProvider wraps r so it satisfies RepoProvider. nil r
// is allowed and means "always not configured" (mirrors the
// pre-AppSettings behaviour where rep == nil disabled /repo routes).
func NewStaticRepoProvider(r *repo.Root) RepoProvider {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.NewStaticRepoProvider")
	return &staticRepoProvider{root: r}
}

func (p *staticRepoProvider) Repo(_ context.Context) (*repo.Root, string, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.staticRepoProvider.Repo")
	if p.root == nil {
		return nil, RepoReasonNotConfigured, nil
	}
	return p.root, "", nil
}

// settingsRepoProvider re-opens repo.Root whenever AppSettings.RepoRoot
// changes. Caches the last successful (path, *repo.Root) pair so steady
// state hits don't re-stat the workspace. The cache is keyed on the
// raw RepoRoot string from the DB; a PATCH /settings that changes the
// path invalidates the cache on the next call automatically because
// the lookup compares against the stored row.
//
// Failures are intentionally NOT cached so an operator who fixes a
// missing dir doesn't have to bounce the process.
type settingsRepoProvider struct {
	store *store.Store

	mu     sync.Mutex
	cached *repo.Root
	source string
}

// NewSettingsRepoProvider returns a provider backed by the AppSettings
// row in s. Use this in production wiring; tests should use
// NewStaticRepoProvider unless they specifically exercise the
// settings hot-reload path.
func NewSettingsRepoProvider(s *store.Store) RepoProvider {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.NewSettingsRepoProvider")
	return &settingsRepoProvider{store: s}
}

func (p *settingsRepoProvider) Repo(ctx context.Context) (*repo.Root, string, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.settingsRepoProvider.Repo")
	if p == nil || p.store == nil {
		return nil, RepoReasonNotConfigured, nil
	}
	cfg, err := p.store.GetSettings(ctx)
	if err != nil {
		return nil, "", err
	}
	if cfg.RepoRoot == "" {
		p.invalidate()
		return nil, RepoReasonNotConfigured, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cached != nil && p.source == cfg.RepoRoot {
		return p.cached, "", nil
	}
	root, openErr := repo.OpenRoot(cfg.RepoRoot)
	if openErr != nil {
		p.cached = nil
		p.source = ""
		return nil, RepoReasonOpenFailed, openErr
	}
	p.cached = root
	p.source = cfg.RepoRoot
	return root, "", nil
}

func (p *settingsRepoProvider) invalidate() {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.settingsRepoProvider.invalidate")
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cached = nil
	p.source = ""
}
