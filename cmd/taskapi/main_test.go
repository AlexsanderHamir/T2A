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

func TestResolveListenHost(t *testing.T) {
	t.Run("defaults to localhost when flag and env are empty", func(t *testing.T) {
		t.Setenv("T2A_LISTEN_HOST", "")
		if got := resolveListenHost(""); got != "127.0.0.1" {
			t.Fatalf("got %q want 127.0.0.1", got)
		}
	})
	t.Run("uses env when flag is empty", func(t *testing.T) {
		t.Setenv("T2A_LISTEN_HOST", "0.0.0.0")
		if got := resolveListenHost(""); got != "0.0.0.0" {
			t.Fatalf("got %q want 0.0.0.0", got)
		}
	})
	t.Run("flag overrides env", func(t *testing.T) {
		t.Setenv("T2A_LISTEN_HOST", "0.0.0.0")
		if got := resolveListenHost("127.0.0.1"); got != "127.0.0.1" {
			t.Fatalf("got %q want 127.0.0.1", got)
		}
	})
}
