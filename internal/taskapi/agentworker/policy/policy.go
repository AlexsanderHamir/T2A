package policy

import "github.com/AlexsanderHamir/T2A/pkgs/tasks/store"

// SchedulingIdleHintReason is the diagnostic idle reason emitted when
// the worker is fully configured and could run, but the ready queue
// is empty only because every ready task is deferred via
// pickup_not_before > now. Not returned by DecideIdle — see
// docs/domain/agent-supervisor.md.
const SchedulingIdleHintReason = "awaiting_scheduled_task"

// RepoRootChecker validates AppSettings.RepoRoot before the worker runs.
type RepoRootChecker func(dir string) error

// DecideSchedulingIdleHint reports the diagnostic hint when the queue
// is empty but scheduled tasks exist. Errors from probes degrade to "".
func DecideSchedulingIdleHint(queueEmpty bool, scheduledCount int64) string {
	if queueEmpty && scheduledCount > 0 {
		return SchedulingIdleHintReason
	}
	return ""
}

// DecideIdle reports whether the worker should stay idle given settings.
// checkRepo validates RepoRoot when non-empty; failures yield
// repo_root_invalid.
func DecideIdle(cfg store.AppSettings, checkRepo RepoRootChecker) (idle bool, reason string) {
	if cfg.AgentPaused {
		return true, "paused_by_operator"
	}
	if cfg.RepoRoot == "" {
		return true, "repo_root_not_configured"
	}
	if checkRepo != nil {
		if err := checkRepo(cfg.RepoRoot); err != nil {
			return true, "repo_root_invalid"
		}
	}
	return false, ""
}

// InstanceSnapshot captures the running worker state needed for
// material-change comparison without importing supervisor types.
type InstanceSnapshot struct {
	Settings        store.AppSettings
	RunnerVersion   string
	HasVerifyRunner bool
}

// InstanceMatchesSettings reports whether the running worker already
// matches desired settings and probed runner version.
func InstanceMatchesSettings(inst *InstanceSnapshot, cfg store.AppSettings, version string) bool {
	if inst == nil {
		return false
	}
	if inst.Settings.Runner != cfg.Runner {
		return false
	}
	if inst.Settings.CursorBin != cfg.CursorBin {
		return false
	}
	if inst.Settings.CursorModel != cfg.CursorModel {
		return false
	}
	if inst.Settings.RepoRoot != cfg.RepoRoot {
		return false
	}
	if inst.Settings.MaxRunDurationSeconds != cfg.MaxRunDurationSeconds {
		return false
	}
	if inst.Settings.StreamIdleStuckSeconds != cfg.StreamIdleStuckSeconds {
		return false
	}
	if inst.Settings.VerifyRunnerName != cfg.VerifyRunnerName {
		return false
	}
	if inst.Settings.VerifyRunnerModel != cfg.VerifyRunnerModel {
		return false
	}
	if inst.Settings.AgentPaused != cfg.AgentPaused {
		return false
	}
	if inst.RunnerVersion != "" && inst.RunnerVersion != version {
		return false
	}
	return true
}

// VerifyRunnerStatus returns the effective-config verify_runner_status
// label for an unchanged reload.
func VerifyRunnerStatus(hasVerifyRunner bool, cfg store.AppSettings) string {
	if hasVerifyRunner {
		return "ok"
	}
	if cfg.VerifyRunnerName == cfg.Runner && cfg.VerifyRunnerName != "" {
		return "reuse_execute_runner"
	}
	return ""
}
