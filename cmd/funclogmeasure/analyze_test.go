package main

import (
	"path/filepath"
	"testing"
)

func TestShouldSkipSlogRequirement_versionString(t *testing.T) {
	if !shouldSkipSlogRequirement("github.com/AlexsanderHamir/T2A/internal/version", "String") {
		t.Fatal("expected internal/version.String to be excluded from funclogmeasure slog requirement")
	}
	if shouldSkipSlogRequirement("github.com/AlexsanderHamir/T2A/internal/version", "Other") {
		t.Fatal("unexpected skip")
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
