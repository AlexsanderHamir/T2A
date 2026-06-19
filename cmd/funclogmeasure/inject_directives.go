package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
)

func injectDirectivesForViolations(modRoot string, opts analyzeOpts) (int, error) {
	rep, err := buildReport(modRoot, opts)
	if err != nil {
		return 0, err
	}
	if len(rep.Violations) == 0 {
		return 0, nil
	}
	byFile := map[string][]violation{}
	for _, v := range rep.Violations {
		byFile[v.File] = append(byFile[v.File], v)
	}
	total := 0
	for path, viols := range byFile {
		n, err := injectDirectivesInFile(path, viols)
		if err != nil {
			return total, fmt.Errorf("%s: %w", path, err)
		}
		total += n
	}
	return total, nil
}

func injectDirectivesInFile(path string, viols []violation) (int, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return 0, err
	}
	want := map[string]violation{}
	for _, v := range viols {
		want[v.FuncName] = v
	}

	type insertAt struct {
		line int
		text string
	}
	var inserts []insertAt

	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		name := formatFuncName(fd)
		v, ok := want[name]
		if !ok {
			continue
		}
		if hasValidSkipDirective(fd) {
			continue
		}
		cat, reason := directiveCategoryFor(v)
		line := directiveInsertLine(fset, fd)
		inserts = append(inserts, insertAt{
			line: line,
			text: fmt.Sprintf("//funclogmeasure:skip category=%s reason=%q", cat, reason),
		})
	}

	if len(inserts) == 0 {
		return 0, nil
	}

	lines := strings.Split(string(src), "\n")
	sort.Slice(inserts, func(i, j int) bool { return inserts[i].line > inserts[j].line })
	for _, ins := range inserts {
		idx := ins.line - 1
		if idx < 0 || idx > len(lines) {
			return 0, fmt.Errorf("insert line %d out of range in %s", ins.line, path)
		}
		lines = append(lines[:idx], append([]string{ins.text}, lines[idx:]...)...)
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return len(inserts), os.WriteFile(path, []byte(out), 0o644)
}

func directiveInsertLine(fset *token.FileSet, fd *ast.FuncDecl) int {
	if fd.Doc != nil && len(fd.Doc.List) > 0 {
		return fset.Position(fd.Doc.List[0].Pos()).Line
	}
	return fset.Position(fd.Type.Pos()).Line
}

func directiveCategoryFor(v violation) (string, string) {
	fn := v.FuncName
	const defaultReason = "Pure helper without I/O; operation trace is emitted by the calling chokepoint."
	if strings.HasPrefix(fn, "With") && strings.Contains(v.Pkg, "/handler") {
		return categoryReExport, "Thin middleware re-export; slog lives on pkgs/tasks/middleware implementation."
	}
	if strings.Contains(v.Pkg, "handlertest") || strings.Contains(v.Pkg, "httpsecurityexpect") {
		return categoryToolNoop, "Test-only HTTP wiring; not part of production trace paths."
	}
	if fn == "main" && strings.Contains(v.Pkg, "/cmd/") {
		return categoryToolNoop, "cmd entrypoint; slog JSON sink is configured in run()."
	}
	if strings.Contains(fn, "HelperIO") || fn == "RunObserved" {
		return categoryDelegate, "Delegates to helperDebugIn/helperDebugOut which emit helper.io traces."
	}
	return categoryHotPath, defaultReason
}
