package main

import (
	"testing"
	"time"
)

func TestResolveSSETestTickerInterval(t *testing.T) {
	t.Run("defaults to 3s when unset", func(t *testing.T) {
		t.Setenv(sseTestIntervalEnv, "")
		if got := resolveSSETestTickerInterval(); got != sseTestDefaultInterval {
			t.Fatalf("got %v want %v", got, sseTestDefaultInterval)
		}
	})
	t.Run("zero disables ticker", func(t *testing.T) {
		t.Setenv(sseTestIntervalEnv, "0")
		if got := resolveSSETestTickerInterval(); got != 0 {
			t.Fatalf("got %v want 0", got)
		}
	})
	t.Run("custom duration", func(t *testing.T) {
		t.Setenv(sseTestIntervalEnv, "7s")
		if got := resolveSSETestTickerInterval(); got != 7*time.Second {
			t.Fatalf("got %v", got)
		}
	})
}
