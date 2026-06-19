package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

const (
	layerDirectSlog     = "direct_slog"
	layerTraceDelegate  = "trace_delegate"
	layerAutoExempt     = "auto_exempt"
	layerDirective      = "directive"
	layerUnsatisfied    = "unsatisfied"
)

type pkgSatisfyCache struct {
	layers map[string]map[string]string
}

func newPkgSatisfyCache() *pkgSatisfyCache {
	return &pkgSatisfyCache{layers: make(map[string]map[string]string)}
}

func (c *pkgSatisfyCache) layer(pkgPath, funcName string) (string, bool) {
	m, ok := c.layers[pkgPath]
	if !ok {
		return "", false
	}
	l, ok := m[funcName]
	return l, ok
}

func (c *pkgSatisfyCache) set(pkgPath, funcName, layer string) {
	if c.layers[pkgPath] == nil {
		c.layers[pkgPath] = make(map[string]string)
	}
	c.layers[pkgPath][funcName] = layer
}

func analyzePackageSatisfaction(pkg *packages.Package, cache *pkgSatisfyCache) map[string]string {
	out := make(map[string]string)
	if pkg.TypesInfo == nil {
		return out
	}
	info := pkg.TypesInfo

	var funcs []*ast.FuncDecl
	for _, f := range pkg.Syntax {
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			funcs = append(funcs, fd)
		}
	}

	for {
		changed := false
		for _, fd := range funcs {
			name := formatFuncName(fd)
			prev, had := cache.layer(pkg.PkgPath, name)
			layer := classifyFuncLayer(fd, pkg, info, cache, true)
			if !had || prev != layer {
				changed = true
			}
			out[name] = layer
			cache.set(pkg.PkgPath, name, layer)
		}
		if !changed {
			break
		}
	}
	return out
}

func classifyFuncLayer(fd *ast.FuncDecl, pkg *packages.Package, info *types.Info, cache *pkgSatisfyCache, allowWrapper bool) string {
	if hasValidSkipDirective(fd) {
		return layerDirective
	}
	if rule := autoExemptRule(fd, pkg.PkgPath, info); rule != "" {
		return layerAutoExempt
	}
	if funcDeclBodyHasSlogCall(fd.Body, info) {
		return layerDirectSlog
	}
	if bodyUsesTracePrimitive(fd.Body, info) {
		return layerTraceDelegate
	}
	if allowWrapper && bodyIsThinSamePackageDelegate(fd.Body, info, pkg, fd, cache) {
		return layerTraceDelegate
	}
	return layerUnsatisfied
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
