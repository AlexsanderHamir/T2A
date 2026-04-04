package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenTaskAPILogFile_createsFileUnderDir(t *testing.T) {
	t.Setenv("T2A_LOG_DIR", "")
	base := t.TempDir()
	f, path, err := openTaskAPILogFile(base, slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, base) {
		t.Fatalf("path %q not under %q", path, base)
	}
	baseName := filepath.Base(path)
	if !strings.HasPrefix(baseName, "taskapi-") || !strings.HasSuffix(baseName, ".jsonl") {
		t.Fatalf("unexpected file name: %s", baseName)
	}
	h := slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo}))
	h.Info("probe", "ok", true)
	if err := f.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "taskapi.openTaskAPILogFile") {
		t.Fatalf("expected open trace in jsonl, got %q", s)
	}
	if !strings.Contains(s, `"msg":"probe"`) {
		t.Fatalf("expected JSON log line with probe, got %q", s)
	}
}

func TestOpenTaskAPILogFile_skipsBootstrapDebugWhenMinInfo(t *testing.T) {
	t.Setenv("T2A_LOG_DIR", "")
	base := t.TempDir()
	f, path, err := openTaskAPILogFile(base, slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "taskapi.openTaskAPILogFile") {
		t.Fatalf("bootstrap debug should be suppressed at info level, got %q", string(raw))
	}
}

func TestOpenTaskAPILogFile_prefersFlagOverEnv(t *testing.T) {
	flagDir := t.TempDir()
	envDir := t.TempDir()
	t.Setenv("T2A_LOG_DIR", envDir)
	f, path, err := openTaskAPILogFile(flagDir, slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, flagDir) {
		t.Fatalf("want log under flag dir %q, got %q", flagDir, path)
	}
}
