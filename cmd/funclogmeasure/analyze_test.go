package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
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

func TestMiniMod_typeResolvedSlog(t *testing.T) {
	dir := filepath.Join("testdata", "minimod")
	rep, err := buildReport(dir, analyzeOpts{includeTool: true})
	if err != nil {
		t.Fatal(err)
	}
	var noLog *violation
	for i := range rep.Violations {
		v := &rep.Violations[i]
		if v.Pkg == "minimod/bad" && v.FuncName == "NoLog" {
			noLog = v
			break
		}
	}
	if noLog == nil {
		t.Fatalf("expected minimod/bad.NoLog violation, got %#v", rep.Violations)
	}
	if rep.Satisfaction.DirectSlog < 4 {
		t.Fatalf("expected direct_slog from good/dotpkg packages, got %d", rep.Satisfaction.DirectSlog)
	}
}

func TestSatisfy_runObservedDelegate(t *testing.T) {
	rep := mustBuildMinimodPkg(t, "minimod/delegate")
	if layer := rep.layerFor("ViaRunObserved"); layer != layerTraceDelegate {
		t.Fatalf("ViaRunObserved layer: got %q want %q", layer, layerTraceDelegate)
	}
}

func TestSatisfy_samePackageWrapper(t *testing.T) {
	rep := mustBuildMinimodPkg(t, "minimod/delegate")
	if layer := rep.layerFor("ThinWrapper"); layer != layerTraceDelegate {
		t.Fatalf("ThinWrapper layer: got %q want %q", layer, layerTraceDelegate)
	}
}

func TestAutoExempt_errorString(t *testing.T) {
	rep := mustBuildMinimodPkg(t, "minimod/exempt")
	for _, fn := range []string{"errVal.Error", "stringerVal.String"} {
		if layer := rep.layerFor(fn); layer != layerAutoExempt {
			t.Fatalf("%s layer: got %q want %q", fn, layer, layerAutoExempt)
		}
	}
}

func TestAutoExempt_scanValueTableName(t *testing.T) {
	rep := mustBuildMinimodPkg(t, "minimod/exempt")
	for _, fn := range []string{"*enum.Scan", "enum.Value", "model.TableName"} {
		if layer := rep.layerFor(fn); layer != layerAutoExempt {
			t.Fatalf("%s layer: got %q want %q", fn, layer, layerAutoExempt)
		}
	}
}

func TestDirective_validAndInvalid(t *testing.T) {
	rep := mustBuildMinimodPkg(t, "minimod/directive")
	if layer := rep.layerFor("Skipped"); layer != layerDirective {
		t.Fatalf("Skipped layer: got %q want %q", layer, layerDirective)
	}
	if layer := rep.layerFor("BadReason"); layer != layerUnsatisfied {
		t.Fatalf("BadReason should fail validation, got %q", layer)
	}
	if layer := rep.layerFor("MissingDirective"); layer != layerUnsatisfied {
		t.Fatalf("MissingDirective should violate, got %q", layer)
	}
}

func TestDirective_missingReasonFails(t *testing.T) {
	fd := parseFuncDecl(t, `package p
//funclogmeasure:skip category=hot-path reason="short"
func Bad() {}`)
	if hasValidSkipDirective(fd) {
		t.Fatal("expected invalid directive with short reason")
	}
	_, err := parseSkipDirective(fd)
	if err == nil {
		t.Fatal("expected parse error for short reason")
	}
}

func TestDirective_validParse(t *testing.T) {
	fd := parseFuncDecl(t, `package p
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func Ok() {}`)
	d, err := parseSkipDirective(fd)
	if err != nil || d == nil {
		t.Fatalf("parseSkipDirective: err=%v d=%v", err, d)
	}
	if d.Category != categoryHotPath {
		t.Fatalf("category: got %q", d.Category)
	}
}

type pkgLayerReport struct {
	layers map[string]string
}

func (r pkgLayerReport) layerFor(name string) string {
	return r.layers[name]
}

func mustBuildMinimodPkg(t *testing.T, pkgPath string) pkgLayerReport {
	t.Helper()
	dir := filepath.Join("testdata", "minimod")
	rep, err := buildReport(dir, analyzeOpts{includeTool: true})
	if err != nil {
		t.Fatal(err)
	}
	cache := newPkgSatisfyCache()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedCompiledGoFiles,
		Dir:  dir,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatal(err)
	}
	out := pkgLayerReport{layers: map[string]string{}}
	for _, pkg := range pkgs {
		if pkg.PkgPath != pkgPath {
			continue
		}
		layers := analyzePackageSatisfaction(pkg, cache)
		return pkgLayerReport{layers: layers}
	}
	t.Fatalf("package %q not found in minimod (funcs_considered=%d violations=%d)", pkgPath, rep.FuncsConsidered, len(rep.Violations))
	return out
}

func parseFuncDecl(t *testing.T, src string) *ast.FuncDecl {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok {
			return fd
		}
	}
	t.Fatal("no FuncDecl")
	return nil
}
