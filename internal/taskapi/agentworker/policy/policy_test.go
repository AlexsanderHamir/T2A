package policy_test

import (
	"errors"
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/taskapi/agentworker/policy"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func TestDecideIdle(t *testing.T) {
	t.Parallel()
	okRepo := func(string) error { return nil }
	badRepo := func(string) error { return errors.New("not a directory") }

	tests := []struct {
		name   string
		cfg    store.AppSettings
		check  policy.RepoRootChecker
		idle   bool
		reason string
	}{
		{
			name:   "paused",
			cfg:    store.AppSettings{AgentPaused: true, RepoRoot: "/repo"},
			check:  okRepo,
			idle:   true,
			reason: "paused_by_operator",
		},
		{
			name:   "empty repo",
			cfg:    store.AppSettings{RepoRoot: ""},
			check:  okRepo,
			idle:   true,
			reason: "repo_root_not_configured",
		},
		{
			name:   "invalid repo",
			cfg:    store.AppSettings{RepoRoot: "/bad"},
			check:  badRepo,
			idle:   true,
			reason: "repo_root_invalid",
		},
		{
			name:   "configured",
			cfg:    store.AppSettings{RepoRoot: "/repo", Runner: "cursor"},
			check:  okRepo,
			idle:   false,
			reason: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			idle, reason := policy.DecideIdle(tt.cfg, tt.check)
			if idle != tt.idle || reason != tt.reason {
				t.Fatalf("DecideIdle() = (%v, %q), want (%v, %q)", idle, reason, tt.idle, tt.reason)
			}
		})
	}
}

func TestDecideSchedulingIdleHint(t *testing.T) {
	t.Parallel()
	if got := policy.DecideSchedulingIdleHint(true, 3); got != policy.SchedulingIdleHintReason {
		t.Fatalf("got %q", got)
	}
	if got := policy.DecideSchedulingIdleHint(false, 3); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := policy.DecideSchedulingIdleHint(true, 0); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestInstanceMatchesSettings(t *testing.T) {
	t.Parallel()
	base := store.AppSettings{
		Runner:                "cursor",
		CursorBin:             "/bin/cursor",
		CursorModel:           "gpt",
		RepoRoot:              "/repo",
		MaxRunDurationSeconds: 600,
		VerifyRunnerName:      "cursor",
		VerifyRunnerModel:     "gpt",
	}
	inst := &policy.InstanceSnapshot{
		Settings:        base,
		RunnerVersion:   "1.0",
		HasVerifyRunner: false,
	}
	if !policy.InstanceMatchesSettings(inst, base, "1.0") {
		t.Fatal("expected match for identical settings")
	}
	changed := base
	changed.RepoRoot = "/other"
	if policy.InstanceMatchesSettings(inst, changed, "1.0") {
		t.Fatal("expected mismatch on repo root")
	}
	if policy.InstanceMatchesSettings(inst, base, "2.0") {
		t.Fatal("expected mismatch on runner version")
	}
}

func TestVerifyRunnerStatus(t *testing.T) {
	t.Parallel()
	cfg := store.AppSettings{Runner: "cursor", VerifyRunnerName: "cursor"}
	if got := policy.VerifyRunnerStatus(false, cfg); got != "reuse_execute_runner" {
		t.Fatalf("got %q", got)
	}
	if got := policy.VerifyRunnerStatus(true, cfg); got != "ok" {
		t.Fatalf("got %q", got)
	}
}
