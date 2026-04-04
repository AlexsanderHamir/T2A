package main

import (
	"log/slog"
	"testing"
)

func TestResolveTaskAPILogLevel_defaultsToInfo(t *testing.T) {
	t.Setenv("T2A_LOG_LEVEL", "")
	got, err := resolveTaskAPILogLevel("")
	if err != nil {
		t.Fatal(err)
	}
	if got != slog.LevelInfo {
		t.Fatalf("default: got %v want %v", got, slog.LevelInfo)
	}
}

func TestResolveTaskAPILogLevel_envWhenFlagEmpty(t *testing.T) {
	t.Setenv("T2A_LOG_LEVEL", "info")
	got, err := resolveTaskAPILogLevel("")
	if err != nil {
		t.Fatal(err)
	}
	if got != slog.LevelInfo {
		t.Fatalf("got %v want info", got)
	}
}

func TestResolveTaskAPILogLevel_flagOverridesEnv(t *testing.T) {
	t.Setenv("T2A_LOG_LEVEL", "info")
	got, err := resolveTaskAPILogLevel("error")
	if err != nil {
		t.Fatal(err)
	}
	if got != slog.LevelError {
		t.Fatalf("got %v want error", got)
	}
}

func TestResolveTaskAPILogLevel_caseInsensitiveAliases(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"Info", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
	} {
		got, err := resolveTaskAPILogLevel(tt.in)
		if err != nil {
			t.Fatalf("%q: %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("%q: got %v want %v", tt.in, got, tt.want)
		}
	}
}

func TestResolveTaskAPILogLevel_invalid(t *testing.T) {
	t.Setenv("T2A_LOG_LEVEL", "")
	_, err := resolveTaskAPILogLevel("verbose")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveTaskAPILogLevel_invalidEnv(t *testing.T) {
	t.Setenv("T2A_LOG_LEVEL", "nope")
	_, err := resolveTaskAPILogLevel("")
	if err == nil {
		t.Fatal("expected error")
	}
}
