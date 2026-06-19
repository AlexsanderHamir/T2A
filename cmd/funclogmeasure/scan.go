package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

const defaultToolImportPath = "github.com/AlexsanderHamir/T2A/cmd/funclogmeasure"

type analyzeOpts struct {
	tests       bool
	includeTool bool
}

func isNPMWebNodeModulesGo(path string) bool {
	p := filepath.ToSlash(path)
	if strings.Contains(p, "/web/node_modules/") {
		return true
	}
	return strings.HasPrefix(p, "web/node_modules/")
}

func isGeneratedGo(src []byte) bool {
	s := string(src)
	if len(s) > 8192 {
		s = s[:8192]
	}
	return strings.Contains(s, "Code generated") || strings.Contains(s, "DO NOT EDIT")
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

	cache := newPkgSatisfyCache()
	for _, pkg := range pkgs {
		if pkg.PkgPath == defaultToolImportPath && !opts.includeTool {
			continue
		}
		if pkg.TypesInfo == nil || len(pkg.Syntax) != len(pkg.CompiledGoFiles) {
			continue
		}
		analyzePackageSatisfaction(pkg, cache)
	}

	var rep report
	for _, pkg := range pkgs {
		if err := accumulateViolationsFromPackage(pkg, fset, opts, cache, &rep); err != nil {
			return nil, err
		}
	}
	return &rep, nil
}

func accumulateViolationsFromPackage(pkg *packages.Package, fset *token.FileSet, opts analyzeOpts, cache *pkgSatisfyCache, rep *report) error {
	for _, e := range pkg.Errors {
		slog.Warn("package analysis issue", "pkg", pkg.PkgPath, "err", e)
	}

	if pkg.PkgPath == defaultToolImportPath && !opts.includeTool {
		return nil
	}
	if pkg.TypesInfo == nil {
		slog.Warn("skipping package without types info", "pkg", pkg.PkgPath)
		return nil
	}
	if len(pkg.Syntax) != len(pkg.CompiledGoFiles) {
		slog.Warn("syntax/compiled file count mismatch", "pkg", pkg.PkgPath,
			"syntax", len(pkg.Syntax), "compiled", len(pkg.CompiledGoFiles))
		return nil
	}

	info := pkg.TypesInfo
	for i, f := range pkg.Syntax {
		path := pkg.CompiledGoFiles[i]
		if isNPMWebNodeModulesGo(path) {
			continue
		}
		if !opts.tests && strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
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
			rep.FuncsConsidered++

			layer := classifyFuncLayer(fd, pkg, info, cache, true)
			if layer == layerUnsatisfied {
				rep.FuncsMissingTrace++
				pos := fset.Position(fd.Type.Pos())
				rep.Violations = append(rep.Violations, violation{
					Pkg:      pkg.PkgPath,
					File:     path,
					Line:     pos.Line,
					FuncName: name,
				})
				continue
			}
			rep.recordSatisfied(layer)
		}
	}
	return nil
}
