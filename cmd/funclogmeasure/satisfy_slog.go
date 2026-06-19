package main

import (
	"go/ast"
	"go/types"
)

func funcDeclBodyHasSlogCall(body *ast.BlockStmt, info *types.Info) bool {
	if body == nil {
		return true
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
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
