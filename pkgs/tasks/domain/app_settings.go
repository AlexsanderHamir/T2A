package domain

import (
	"time"
)

// AppSettings is the singleton row (id=1) holding all UI-configurable
// app-level settings. There is intentionally only one row: every PATCH
// upserts onto id=1 and every GET reads id=1, optionally creating it
// with defaults on first read.
//
// This row replaces the historical T2A_AGENT_WORKER_* env vars and the
// REPO_ROOT env var. Env vars are no longer read at runtime — the row
// is the only source of truth and is "saved until changed".
//
// Field semantics:
//   - WorkerEnabled: master switch for the in-process agent worker.
//     Default true.
//   - AgentPaused: operator-facing soft pause. Distinct from
//     WorkerEnabled in intent, even though both keep the worker idle:
//     WorkerEnabled is the "configured to run at all" flag (defaults
//     to true; flipping it off is a deliberate teardown), AgentPaused
//     is the "stop dequeuing for now, I'll resume in a minute"
//     flag (defaults to false; the SPA exposes a toggle in the
//     header). The supervisor honors either by going idle with a
//     distinct reason ("disabled_by_settings" vs "paused_by_operator")
//     so the observability page can tell them apart.
//   - Runner: id of the runner registered in pkgs/agents/runner/registry
//     (today only "cursor"). Default "cursor".
//   - RepoRoot: absolute or process-relative path used for both the
//     agent worker WorkingDir and the global repo file picker / @-mention
//     autocomplete. Empty means "not configured": worker stays idle and
//     repo endpoints respond 409 repo_root_not_configured.
//   - CursorBin: cursor binary path. Empty means "auto-detect from PATH"
//     (the supervisor probes `cursor --version` at boot).
//   - CursorModel: optional `cursor-agent --model` value. Empty means omit
//     the flag so Cursor picks its default model for the account.
//   - MaxRunDurationSeconds: per-run wall-clock cap in seconds. 0 means
//     "no limit" — the worker does not wrap runner.Run with a timeout.
//   - AgentPickupDelaySeconds: new ready tasks get pickup_not_before (see tasks
//     model) deferred by this many seconds so the worker does not dequeue them
//     immediately (smoother UX right after create). Default 5. Set to 0 to
//     disable the delay.
//   - DisplayTimezone: IANA timezone identifier (e.g. "America/New_York")
//     used by the SPA to render every operator-facing timestamp
//     (scheduled pickup time, "last updated", etc.). Validated server-side
//     via time.LoadLocation on PATCH; stored as the canonical name returned
//     by the lookup. Default "" — empty string is the "auto-detect" sentinel
//     that tells the SPA to fall back to the operator's browser timezone
//     (Intl.DateTimeFormat().resolvedOptions().timeZone). Setting this to
//     any non-empty IANA zone (including "UTC") is a deliberate override
//     that wins over auto-detect. The wire format for every timestamp
//     stays RFC3339 UTC — this column governs PRESENTATION only.
//   - OptimisticMutationsEnabled: feature flag introduced in
//     .cursor/plans/production_realtime_smoothness_b17202b6.plan.md
//     Phase 1. When true, the SPA uses the onMutate/onError optimistic
//     code path for PATCH, DELETE, checklist, requeue, and subtask
//     create; when false it falls back to the legacy await-then-render
//     path. Default false on first ship; flip to true after one full
//     SLO window of green metrics in staging. Hook code branches on
//     this flag without any server involvement — the flag is purely a
//     kill-switch for a client behavior.
//   - SSEReplayEnabled: feature flag introduced by the same plan for
//     Phase 2 (lossless SSE). When true, the `/events` handler honors
//     Last-Event-ID replay and emits resync directives from the ring
//     buffer. When false, the handler behaves as it did before the
//     ring-buffer work (live fanout only; reconnect = cold start).
//     Default false on first ship. The client-side EventSource resume
//     header is harmless when the server ignores it, so flipping the
//     flag server-side is fully additive.
type AppSettings struct {
	ID                         uint      `gorm:"primaryKey;autoIncrement:false;check:chk_app_settings_singleton,id = 1"`
	WorkerEnabled              bool      `gorm:"not null;default:true"`
	AgentPaused                bool      `gorm:"not null;default:false"`
	Runner                     string    `gorm:"not null;default:'cursor'"`
	RepoRoot                   string    `gorm:"not null;default:''"`
	CursorBin                  string    `gorm:"not null;default:''"`
	CursorModel                string    `gorm:"not null;default:''"`
	MaxRunDurationSeconds      int       `gorm:"not null;default:0;check:chk_app_settings_max_run_duration_seconds,max_run_duration_seconds >= 0"`
	AgentPickupDelaySeconds    int       `gorm:"not null;default:5;check:chk_app_settings_agent_pickup_delay_seconds,agent_pickup_delay_seconds >= 0"`
	DisplayTimezone            string    `gorm:"not null;default:''"`
	OptimisticMutationsEnabled bool      `gorm:"not null;default:false"`
	SSEReplayEnabled           bool      `gorm:"not null;default:false"`
	UpdatedAt                  time.Time `gorm:"not null"`
}

// AppSettingsRowID is the singleton primary key. Every read/write of
// app_settings uses this id; alternative ids are not allowed (the CHECK
// constraint above enforces it at the DB level).
const AppSettingsRowID uint = 1

// DefaultRunner is the seed value for AppSettings.Runner on first boot.
// Mirrors the only registered runner today (pkgs/agents/runner/cursor).
const DefaultRunner = "cursor"

// DefaultAgentPickupDelaySeconds is the seed value for AgentPickupDelaySeconds
// on first boot (seconds before the worker may dequeue a newly created ready task).
const DefaultAgentPickupDelaySeconds = 5

// DefaultDisplayTimezone is the seed value for DisplayTimezone on first
// boot. Empty string is the "auto-detect" sentinel: the SPA reads it as
// "no explicit operator choice yet" and falls back to the browser's own
// IANA zone (Intl.DateTimeFormat().resolvedOptions().timeZone), so a
// freshly-installed T2A renders timestamps in the operator's local time
// without anyone touching the SettingsPage. Setting the column to any
// non-empty zone (including literal "UTC") via PATCH /settings is a
// deliberate override that pins every operator to that zone, regardless
// of where their browser is.
const DefaultDisplayTimezone = ""

// DefaultAppSettings returns the hard-coded first-boot defaults. Used
// by the store's Get path when the row doesn't exist yet, so callers
// always observe a fully populated value. Skip-listed in
// cmd/funclogmeasure/analyze.go: pure struct constructor; the calling
// store.GetAppSettings already logs the seed-on-first-read decision.
func DefaultAppSettings() AppSettings {
	return AppSettings{
		ID:                         AppSettingsRowID,
		WorkerEnabled:              true,
		AgentPaused:                false,
		Runner:                     DefaultRunner,
		RepoRoot:                   "",
		CursorBin:                  "",
		MaxRunDurationSeconds:      0,
		AgentPickupDelaySeconds:    DefaultAgentPickupDelaySeconds,
		DisplayTimezone:            DefaultDisplayTimezone,
		OptimisticMutationsEnabled: false,
		SSEReplayEnabled:           false,
	}
}

// TableName pins the table name so Postgres migrations match between
// dialects. Skip-listed in cmd/funclogmeasure/analyze.go for the same
// reason as TaskChecklistItem.TableName.
func (AppSettings) TableName() string { return "app_settings" }
