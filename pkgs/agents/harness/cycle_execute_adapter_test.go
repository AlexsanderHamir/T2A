package harness

import (
	"errors"
	"fmt"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/orchestration"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
)

func TestMapRunnerOutcome(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err  error
		want orchestration.ExecuteRunnerOutcome
	}{
		{nil, orchestration.ExecuteRunnerOutcomeOK},
		{runner.ErrTimeout, orchestration.ExecuteRunnerOutcomeTimeout},
		{runner.ErrNonZeroExit, orchestration.ExecuteRunnerOutcomeNonZeroExit},
		{runner.ErrInvalidOutput, orchestration.ExecuteRunnerOutcomeInvalidOutput},
		{fmt.Errorf("wrap: %w", runner.ErrTimeout), orchestration.ExecuteRunnerOutcomeTimeout},
		{errors.New("other"), orchestration.ExecuteRunnerOutcomeError},
	}
	for _, tt := range tests {
		name := "nil"
		if tt.err != nil {
			name = tt.err.Error()
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := mapRunnerOutcome(tt.err); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestMapRunnerOutcome_matchesDecideReasons(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err    error
		reason orchestration.TerminationReason
	}{
		{runner.ErrTimeout, orchestration.ReasonRunnerTimeout},
		{runner.ErrNonZeroExit, orchestration.ReasonRunnerNonZeroExit},
		{runner.ErrInvalidOutput, orchestration.ReasonRunnerInvalidOutput},
		{errors.New("x"), orchestration.ReasonRunnerError},
	}
	for _, tt := range tests {
		t.Run(string(tt.reason), func(t *testing.T) {
			t.Parallel()
			effects := orchestration.DecideExecutePostRun(orchestration.ExecutePostRunInput{
				RunnerOutcome: mapRunnerOutcome(tt.err),
			})
			if effects.Reason != tt.reason {
				t.Fatalf("got %q want %q", effects.Reason, tt.reason)
			}
		})
	}
}
