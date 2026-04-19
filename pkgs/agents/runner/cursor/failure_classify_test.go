package cursor

import "testing"

func TestClassifyCursorFailure_usageLimit(t *testing.T) {
	t.Parallel()
	kind, msg := classifyCursorFailure("Error: You've hit your usage limit. Switch models.")
	if kind != FailureKindCursorUsageLimit {
		t.Fatalf("kind=%q want %q", kind, FailureKindCursorUsageLimit)
	}
	if msg == "" {
		t.Fatal("expected standardized message")
	}
}

func TestClassifyCursorFailure_spendLimitPhrase(t *testing.T) {
	t.Parallel()
	kind, _ := classifyCursorFailure("set a Spend Limit to continue with this model")
	if kind != FailureKindCursorUsageLimit {
		t.Fatalf("kind=%q want %q", kind, FailureKindCursorUsageLimit)
	}
}

func TestClassifyCursorFailure_unknown(t *testing.T) {
	t.Parallel()
	kind, msg := classifyCursorFailure("compile failed")
	if kind != "" || msg != "" {
		t.Fatalf("expected empty, got kind=%q msg=%q", kind, msg)
	}
}
