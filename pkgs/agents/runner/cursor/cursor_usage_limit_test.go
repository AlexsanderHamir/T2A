package cursor_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
)

func TestRun_usageLimit_nonZeroExitStructuredDetails(t *testing.T) {
	t.Parallel()

	stderr := []byte("x: You've hit your usage limit. Switch to another model.\n")
	var c captured
	a := newAdapter(fakeExec(&c, []byte(""), stderr, 1, nil, false))

	res, err := a.Run(context.Background(), defaultRequest())
	if !errors.Is(err, runner.ErrNonZeroExit) {
		t.Fatalf("err: %v", err)
	}
	if res.Summary != "Cursor usage limit reached" {
		t.Fatalf("summary=%q want usage-limit title", res.Summary)
	}
	if !strings.Contains(string(res.Details), `"failure_kind":"cursor_usage_limit"`) {
		t.Fatalf("details should include failure_kind: %s", res.Details)
	}
}
