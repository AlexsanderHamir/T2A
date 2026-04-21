package cycles

import "testing"

func TestFailureSurfaceMessage_precedence(t *testing.T) {
	t.Parallel()
	details := map[string]any{
		"failure_kind":         "cursor_usage_limit",
		"standardized_message": "Full usage message.",
	}
	got := FailureSurfaceMessage(true, "runner_non_zero_exit", "Short title", details)
	if got != "Full usage message." {
		t.Fatalf("got %q want standardized_message first", got)
	}
}

func TestFailureSurfaceMessage_noPhase(t *testing.T) {
	t.Parallel()
	if got := FailureSurfaceMessage(false, "runner_non_zero_exit", "", nil); got != "" {
		t.Fatalf("got %q want empty", got)
	}
}

func TestFailureSurfaceMessage_fallsBackToCycleReason(t *testing.T) {
	t.Parallel()
	got := FailureSurfaceMessage(true, "runner_non_zero_exit", "", map[string]any{})
	if got != "runner_non_zero_exit" {
		t.Fatalf("got %q", got)
	}
}
