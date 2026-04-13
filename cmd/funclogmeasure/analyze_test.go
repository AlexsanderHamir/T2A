package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIsNPMWebNodeModulesGo(t *testing.T) {
	sep := string(filepath.Separator)
	for _, tt := range []struct {
		path string
		want bool
	}{
		{filepath.Join("x", "web", "node_modules", "p", "f.go"), true},
		{strings.ReplaceAll("x/web/node_modules/p/f.go", "/", sep), true},
		{filepath.Join("pkgs", "tasks", "handler", "x.go"), false},
		{filepath.Join("web", "src", "not_node_modules", "x.go"), false},
		{filepath.Join("myweb", "node_modules", "x.go"), false},
	} {
		if got := isNPMWebNodeModulesGo(tt.path); got != tt.want {
			t.Fatalf("isNPMWebNodeModulesGo(%q): got %v want %v", tt.path, got, tt.want)
		}
	}
}

func TestShouldSkipSlogRequirement_versionString(t *testing.T) {
	if !shouldSkipSlogRequirement("github.com/AlexsanderHamir/T2A/internal/version", "String") {
		t.Fatal("expected internal/version.String to be excluded from funclogmeasure slog requirement")
	}
	if shouldSkipSlogRequirement("github.com/AlexsanderHamir/T2A/internal/version", "Other") {
		t.Fatal("unexpected skip")
	}
}

func TestShouldSkipSlogRequirement_repoIsMentionDelimiter(t *testing.T) {
	if !shouldSkipSlogRequirement("github.com/AlexsanderHamir/T2A/pkgs/repo", "isMentionDelimiter") {
		t.Fatal("expected pkgs/repo.isMentionDelimiter to be excluded (inner loop of ParseFileMentions)")
	}
}

func TestShouldSkipSlogRequirement_handlerHotHelpers(t *testing.T) {
	for _, tt := range []struct {
		pkg, fn string
	}{
		{"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler", "applyAPISecurityHeaders"},
		{"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler", "ServerVersion"},
		{"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler", "*metricsHTTPResponseWriter.WriteHeader"},
		{"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler", "*metricsHTTPResponseWriter.Write"},
		{"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler", "*metricsHTTPResponseWriter.Flush"},
		{"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler", "*metricsHTTPResponseWriter.statusCode"},
	} {
		if !shouldSkipSlogRequirement(tt.pkg, tt.fn) {
			t.Fatalf("expected skip %s.%s", tt.pkg, tt.fn)
		}
	}
}

func TestMiniMod_typeResolvedSlog(t *testing.T) {
	dir := filepath.Join("testdata", "minimod")
	rep, err := buildReport(dir, analyzeOpts{includeTool: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep.FilesScanned != 3 {
		t.Fatalf("files_scanned: got %d want 3", rep.FilesScanned)
	}
	if rep.FuncsConsidered != 5 {
		t.Fatalf("funcs_considered: got %d want 5", rep.FuncsConsidered)
	}
	if rep.FuncsWithSlog != 4 {
		t.Fatalf("funcs_with_slog: got %d want 4", rep.FuncsWithSlog)
	}
	if rep.FuncsMissingSlog != 1 || len(rep.Violations) != 1 {
		t.Fatalf("missing: got %d violations %#v", rep.FuncsMissingSlog, rep.Violations)
	}
	v := rep.Violations[0]
	if v.Pkg != "minimod/bad" || v.FuncName != "NoLog" {
		t.Fatalf("unexpected violation: %+v", v)
	}
}
