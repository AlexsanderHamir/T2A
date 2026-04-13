package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

const defaultToolImportPath = "github.com/AlexsanderHamir/T2A/cmd/funclogmeasure"

// skipSlogRequirement marks pkg+func pairs that intentionally omit log/slog.
// Keep this tiny: pure helpers on hot paths (e.g. health JSON) or inner-loop
// predicates where Debug each call would flood logs. See docs/OBSERVABILITY.md.
var skipSlogRequirement = map[string]struct{}{
	"github.com/AlexsanderHamir/T2A/internal/version\tString":      {},
	"github.com/AlexsanderHamir/T2A/pkgs/repo\tisMentionDelimiter": {},
	// Header-only helper on every response; JSON paths log via setJSONHeaders / setAPISecurityHeaders.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tapplyAPISecurityHeaders": {},
	// Thin wrapper over internal/version.String (already excluded); health and JSON embed version without duplicating logs here.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tServerVersion": {},
	// Prometheus metrics wrapper: per-chunk Write / Flush must not allocate log attrs on hot paths.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\t*metricsHTTPResponseWriter.WriteHeader": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\t*metricsHTTPResponseWriter.Write":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\t*metricsHTTPResponseWriter.Flush":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\t*metricsHTTPResponseWriter.statusCode":  {},
}

func shouldSkipSlogRequirement(pkgPath, funcName string) bool {
	_, ok := skipSlogRequirement[pkgPath+"\t"+funcName]
	return ok
}

type analyzeOpts struct {
	tests       bool
	includeTool bool
}

func buildReport(modRoot string, opts analyzeOpts) (*report, error) {
	fset := token.NewFileSet()
	cfg := &packages.Config{
		Fset: fset,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedModule,
		Dir:   modRoot,
		Tests: opts.tests,
		Env:   os.Environ(),
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("packages.Load: %w", err)
	}

	var rep report
	for _, pkg := range pkgs {
		for _, e := range pkg.Errors {
			slog.Warn("package analysis issue", "pkg", pkg.PkgPath, "err", e)
		}

		if pkg.PkgPath == defaultToolImportPath && !opts.includeTool {
			continue
		}
		if pkg.TypesInfo == nil {
			slog.Warn("skipping package without types info", "pkg", pkg.PkgPath)
			continue
		}
		if len(pkg.Syntax) != len(pkg.CompiledGoFiles) {
			slog.Warn("syntax/compiled file count mismatch", "pkg", pkg.PkgPath,
				"syntax", len(pkg.Syntax), "compiled", len(pkg.CompiledGoFiles))
			continue
		}

		info := pkg.TypesInfo
		for i, f := range pkg.Syntax {
			path := pkg.CompiledGoFiles[i]
			if !opts.tests && strings.HasSuffix(path, "_test.go") {
				continue
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", path, err)
			}
			if isGeneratedGo(src) {
				continue
			}

			rep.FilesScanned++
			for _, decl := range f.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				name := formatFuncName(fd)
				if shouldSkipSlogRequirement(pkg.PkgPath, name) {
					continue
				}
				rep.FuncsConsidered++
				if funcDeclBodyHasSlogCall(fd.Body, info) {
					rep.FuncsWithSlog++
				} else {
					rep.FuncsMissingSlog++
					pos := fset.Position(fd.Pos())
					rep.Violations = append(rep.Violations, violation{
						Pkg:      pkg.PkgPath,
						File:     path,
						Line:     pos.Line,
						FuncName: name,
					})
				}
			}
		}
	}

	return &rep, nil
}

func funcDeclBodyHasSlogCall(body *ast.BlockStmt, info *types.Info) bool {
	if body == nil {
		return true
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		// Nested func literals are not named FuncDecls; do not count their calls for the outer func.
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if callUsesSlog(info, call) {
			found = true
			return false
		}
		return true
	})
	return found
}

func callUsesSlog(info *types.Info, call *ast.CallExpr) bool {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		if sel, ok := info.Selections[fun]; ok {
			return objectFromSlogPkg(sel.Obj())
		}
		// Package-qualified calls (e.g. slog.Info) sometimes only record Uses on the method id.
		if obj, ok := info.Uses[fun.Sel]; ok {
			return objectFromSlogPkg(obj)
		}
	case *ast.Ident:
		if obj, ok := info.Uses[fun]; ok {
			return objectFromSlogPkg(obj)
		}
	}
	return false
}

func objectFromSlogPkg(obj types.Object) bool {
	if obj == nil {
		return false
	}
	pkg := obj.Pkg()
	if pkg == nil {
		return false
	}
	return pkg.Path() == "log/slog"
}

func formatFuncName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) != 1 {
		return fd.Name.Name
	}
	recv := formatRecvType(fd.Recv.List[0].Type)
	if recv == "" {
		return fd.Name.Name
	}
	return recv + "." + fd.Name.Name
}

func formatRecvType(ty ast.Expr) string {
	switch t := ty.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return "*" + id.Name
		}
		return ""
	default:
		return ""
	}
}

func isGeneratedGo(src []byte) bool {
	s := string(src)
	if len(s) > 8192 {
		s = s[:8192]
	}
	return strings.Contains(s, "Code generated") || strings.Contains(s, "DO NOT EDIT")
}

func printTextReport(rep *report, maxPrint int, modRoot string) {
	var pct float64
	if rep.FuncsConsidered > 0 {
		pct = 100.0 * float64(rep.FuncsWithSlog) / float64(rep.FuncsConsidered)
	}
	fmt.Fprintf(os.Stdout, "funclogmeasure: files=%d funcs=%d with_slog=%d missing_slog=%d (%.1f%% have slog)\n",
		rep.FilesScanned, rep.FuncsConsidered, rep.FuncsWithSlog, rep.FuncsMissingSlog, pct)
	if rep.FuncsMissingSlog == 0 {
		fmt.Fprintln(os.Stdout, "All considered functions contain at least one type-resolved log/slog call.")
		return
	}
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Functions with no type-resolved log/slog call in the body (nested func literals do not count):")
	n := 0
	for _, v := range rep.Violations {
		rel, _ := filepath.Rel(modRoot, v.File)
		if rel == "" {
			rel = v.File
		}
		fmt.Fprintf(os.Stdout, "%s:%d\t%s\t%s\n", rel, v.Line, v.Pkg, v.FuncName)
		n++
		if maxPrint > 0 && n >= maxPrint {
			rest := len(rep.Violations) - n
			if rest > 0 {
				fmt.Fprintf(os.Stdout, "... and %d more (increase -max or use -json)\n", rest)
			}
			break
		}
	}
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Counts include package-level slog functions, slog.Logger methods, and dot-imported slog names (type-checked via go/types).")
}
