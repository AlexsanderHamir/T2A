package domain

import "testing"

func TestRunCorrelationIDFromDetailsJSON(t *testing.T) {
	t.Parallel()
	if got := RunCorrelationIDFromDetailsJSON(nil); got != "" {
		t.Fatalf("nil = %q", got)
	}
	if got := RunCorrelationIDFromDetailsJSON([]byte(`{}`)); got != "" {
		t.Fatalf("empty = %q", got)
	}
	if got := RunCorrelationIDFromDetailsJSON([]byte(`{"run_correlation_id":"abc"}`)); got != "abc" {
		t.Fatalf("got %q", got)
	}
	if got := RunCorrelationIDFromDetailsJSON([]byte(`not json`)); got != "" {
		t.Fatalf("bad json = %q", got)
	}
}

func TestSessionIDFromDetailsJSON(t *testing.T) {
	t.Parallel()
	if got := SessionIDFromDetailsJSON(nil); got != "" {
		t.Fatalf("nil = %q", got)
	}
	if got := SessionIDFromDetailsJSON([]byte(`{"session_id":"  sess-1  "}`)); got != "sess-1" {
		t.Fatalf("got %q", got)
	}
}
