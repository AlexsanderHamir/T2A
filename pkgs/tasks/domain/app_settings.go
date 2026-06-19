package domain

import (
	"time"

	"gorm.io/datatypes"
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
//   - AgentPaused: operator-facing soft pause exposed in the SPA
//     header. The supervisor honors it by going idle with reason
//     "paused_by_operator". Default false; there is no separate
//     "disabled" master switch — the worker is always configured to
//     run, and pause is the operator's stop-the-dequeue knob.
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
//   - OptimisticMutationsEnabled: when true, the SPA uses optimistic
//     mutations for PATCH, DELETE, checklist, requeue, and subtask
//     create. Stored for API compatibility; always true for new rows
//     and no longer exposed in Settings (not user-configurable).
//   - SSEReplayEnabled: retained for API/DB compatibility. Lossless
//     SSE replay is always active in the `/events` handler; this column
//     is migrated to true on read for older databases.
type AppSettings struct {
	ID                         uint   `gorm:"primaryKey;autoIncrement:false;check:chk_app_settings_singleton,id = 1"`
	AgentPaused                bool   `gorm:"not null;default:false"`
	Runner                     string `gorm:"not null;default:'cursor'"`
	RepoRoot                   string `gorm:"not null;default:''"`
	CursorBin                  string `gorm:"not null;default:''"`
	CursorModel                string `gorm:"not null;default:''"`
	MaxRunDurationSeconds      int    `gorm:"not null;default:0;check:chk_app_settings_max_run_duration_seconds,max_run_duration_seconds >= 0"`
	StreamIdleStuckSeconds     int    `gorm:"not null;default:60;check:chk_app_settings_stream_idle_stuck_seconds,stream_idle_stuck_seconds >= 0"`
	AgentPickupDelaySeconds    int    `gorm:"not null;default:5;check:chk_app_settings_agent_pickup_delay_seconds,agent_pickup_delay_seconds >= 0"`
	DisplayTimezone            string `gorm:"not null;default:''"`
	OptimisticMutationsEnabled bool   `gorm:"not null;default:true"`
	SSEReplayEnabled           bool   `gorm:"not null;default:true"`
	// RunnerConfigs stores per-runner config blobs keyed by runner ID.
	// Example: {"cursor":{"binary_path":"...","default_model":"opus"}}.
	// Dual-written alongside the legacy CursorBin/CursorModel columns
	// during the migration to pluggable runners.
	RunnerConfigs datatypes.JSON `gorm:"column:runner_configs;type:jsonb;not null;default:'{}'"`
	// VerifyMaxRetries is the corrective execute retries after verify failure.
	VerifyMaxRetries int `gorm:"not null;default:2;check:chk_app_settings_verify_max_retries,verify_max_retries >= 0"`
	// VerifyRunnerName empty means use the execute runner id.
	VerifyRunnerName string `gorm:"not null;default:''"`
	// VerifyRunnerModel empty means use the verify runner's default model.
	VerifyRunnerModel string `gorm:"not null;default:''"`
	// VerifyCommandTimeoutSeconds caps each optional criterion shell check during verify.
	VerifyCommandTimeoutSeconds int       `gorm:"not null;default:120;check:chk_app_settings_verify_command_timeout_seconds,verify_command_timeout_seconds > 0"`
	UpdatedAt                   time.Time `gorm:"not null"`
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

// DefaultStreamIdleStuckSeconds is the stdout-silence threshold before
// the worker kills a hung cursor-agent run and attempts evidence recovery.
const DefaultStreamIdleStuckSeconds = 60

// DefaultVerifyMaxRetries is the seed value for VerifyMaxRetries on first boot.
const DefaultVerifyMaxRetries = 2

// DefaultVerifyCommandTimeoutSeconds is the per-command wall-clock cap during verify.
const DefaultVerifyCommandTimeoutSeconds = 120

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
		ID:                          AppSettingsRowID,
		AgentPaused:                 false,
		Runner:                      DefaultRunner,
		RepoRoot:                    "",
		CursorBin:                   "",
		MaxRunDurationSeconds:       0,
		StreamIdleStuckSeconds:      DefaultStreamIdleStuckSeconds,
		AgentPickupDelaySeconds:     DefaultAgentPickupDelaySeconds,
		DisplayTimezone:             DefaultDisplayTimezone,
		OptimisticMutationsEnabled:  true,
		SSEReplayEnabled:            true,
		VerifyMaxRetries:            DefaultVerifyMaxRetries,
		VerifyRunnerName:            "",
		VerifyRunnerModel:           "",
		VerifyCommandTimeoutSeconds: DefaultVerifyCommandTimeoutSeconds,
	}
}

// TableName pins the table name so Postgres migrations match between
// dialects. Skip-listed in cmd/funclogmeasure/analyze.go for the same
// reason as TaskChecklistItem.TableName.
func (AppSettings) TableName() string { return "app_settings" }
