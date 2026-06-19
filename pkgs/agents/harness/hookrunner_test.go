package harness_test

import (
	"context"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
)

// hookRunner wraps a runnerfake so tests can run a side effect when Run
// lands on a given phase.
type hookRunner struct {
	*runnerfake.Runner
	preRun func(req runner.Request)
}

func (h *hookRunner) Run(ctx context.Context, req runner.Request) (runner.Result, error) {
	if h.preRun != nil {
		h.preRun(req)
	}
	if req.OnProgress != nil {
		req.OnProgress(runner.ProgressEvent{Kind: "stream", Subtype: "tool_use", Message: "verify probe"})
	}
	return h.Runner.Run(ctx, req)
}
