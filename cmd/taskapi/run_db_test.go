package main

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/postgres"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	prev := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = prev })
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestEmitSchemaDriftAlerts_pendingWritesStderr(t *testing.T) {
	report := postgres.SchemaDriftReport{
		Status:       postgres.SchemaDriftPending,
		CodeRevision: 2,
		DBRevision:   1,
	}
	out := captureStderr(t, func() { emitSchemaDriftAlerts(report) })
	if out == "" {
		t.Fatal("expected stderr banner")
	}
	if !bytes.Contains([]byte(out), []byte("SCHEMA MIGRATION REQUIRED")) {
		t.Fatalf("stderr %q", out)
	}
}

func TestEmitSchemaDriftAlerts_okSilent(t *testing.T) {
	report := postgres.SchemaDriftReport{Status: postgres.SchemaDriftOK}
	out := captureStderr(t, func() { emitSchemaDriftAlerts(report) })
	if out != "" {
		t.Fatalf("unexpected stderr: %q", out)
	}
}

func TestEmitSchemaDriftAlerts_downgradeSilent(t *testing.T) {
	report := postgres.SchemaDriftReport{Status: postgres.SchemaDriftDowngrade}
	out := captureStderr(t, func() { emitSchemaDriftAlerts(report) })
	if out != "" {
		t.Fatalf("downgrade uses slog only, got stderr: %q", out)
	}
}
