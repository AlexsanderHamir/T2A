package orchestration

import "testing"

func TestDecideVerifyRetry_pass(t *testing.T) {
	t.Parallel()
	e := DecideVerifyRetry(0, 3, VerifyResultPass)
	if e.RetryLoop || e.TerminalFailure || e.Tampered {
		t.Fatalf("unexpected effects: %+v", e)
	}
}

func TestDecideVerifyRetry_tampered(t *testing.T) {
	t.Parallel()
	e := DecideVerifyRetry(0, 3, VerifyResultFailTampered)
	if !e.TerminalFailure || !e.Tampered || e.RetryLoop {
		t.Fatalf("unexpected effects: %+v", e)
	}
}

func TestDecideVerifyRetry_retryable_within_budget(t *testing.T) {
	t.Parallel()
	e := DecideVerifyRetry(0, 3, VerifyResultFailRetryable)
	if !e.RetryLoop || e.TerminalFailure {
		t.Fatalf("unexpected effects: %+v", e)
	}
}

func TestDecideVerifyRetry_retryable_exhausted(t *testing.T) {
	t.Parallel()
	e := DecideVerifyRetry(3, 3, VerifyResultFailRetryable)
	if !e.TerminalFailure || e.RetryLoop {
		t.Fatalf("unexpected effects: %+v", e)
	}
}

func TestDecideVerifyRetryWithValidity_skipsExecuteWhenValid(t *testing.T) {
	t.Parallel()
	e := DecideVerifyRetryWithValidity(0, 3, VerifyResultFailRetryable, true)
	if !e.RetryLoop || !e.SkipNextExecute || e.TerminalFailure {
		t.Fatalf("unexpected effects: %+v", e)
	}
}

func TestDecideVerifyRetryWithValidity_fullReexecuteWhenInvalid(t *testing.T) {
	t.Parallel()
	e := DecideVerifyRetryWithValidity(0, 3, VerifyResultFailRetryable, false)
	if !e.RetryLoop || e.SkipNextExecute || e.TerminalFailure {
		t.Fatalf("unexpected effects: %+v", e)
	}
}

func TestDecideVerifyRetry_last_retry_slot(t *testing.T) {
	t.Parallel()
	e := DecideVerifyRetry(2, 3, VerifyResultFailRetryable)
	if !e.RetryLoop {
		t.Fatalf("want retry at attempt 2 with max 3, got %+v", e)
	}
}
