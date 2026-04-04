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
	f, path, err := openTaskAPILogFile(base)
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
	if !strings.Contains(string(raw), `"msg":"probe"`) {
		t.Fatalf("expected JSON log line with probe, got %q", string(raw))
	}
}

func TestOpenTaskAPILogFile_prefersFlagOverEnv(t *testing.T) {
	flagDir := t.TempDir()
	envDir := t.TempDir()
	t.Setenv("T2A_LOG_DIR", envDir)
	f, path, err := openTaskAPILogFile(flagDir)
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
