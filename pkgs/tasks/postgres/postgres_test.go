package postgres

import (
	"errors"
	"strings"
	"testing"
)

func TestOpen_rejectsEmptyDSN(t *testing.T) {
	_, err := Open("", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errEmptyDSN) {
		t.Fatalf("expected wrapped errEmptyDSN, got %v", err)
	}
}

func TestOpen_rejectsWhitespaceOnlyDSN(t *testing.T) {
	_, err := Open("   \t  ", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEnsureQueryExecModeSimpleProtocol(t *testing.T) {
	t.Parallel()
	if got := ensureQueryExecModeSimpleProtocol(""); got != "" {
		t.Fatalf("empty: got %q", got)
	}
	uri := ensureQueryExecModeSimpleProtocol("postgres://u:p@h/db")
	if !strings.Contains(uri, "default_query_exec_mode=simple_protocol") {
		t.Fatalf("uri: got %q", uri)
	}
	if !strings.Contains(uri, "?") {
		t.Fatalf("uri should use ? for first param: %q", uri)
	}
	uri2 := ensureQueryExecModeSimpleProtocol(uri)
	if uri2 != uri {
		t.Fatalf("idempotent: got %q want %q", uri2, uri)
	}
	uri3 := ensureQueryExecModeSimpleProtocol("postgres://u:p@h/db?sslmode=disable")
	if !strings.Contains(uri3, "&default_query_exec_mode=simple_protocol") {
		t.Fatalf("uri with query: got %q", uri3)
	}
	kv := ensureQueryExecModeSimpleProtocol("host=localhost dbname=t")
	if !strings.HasSuffix(strings.TrimSpace(kv), "default_query_exec_mode=simple_protocol") {
		t.Fatalf("keyword dsn: got %q", kv)
	}
}
