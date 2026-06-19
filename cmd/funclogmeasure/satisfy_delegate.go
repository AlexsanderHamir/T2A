package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

const calltracePkg = "github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"

var tracePrimitiveNames = map[string]struct{}{
	"RunObserved": {},
	"HelperIOIn":  {},
	"HelperIOOut": {},
}

func bodyUsesTracePrimitive(body *ast.BlockStmt, info *types.Info) bool {
	if body == nil {
		return false
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
		if callIsTracePrimitive(info, call) {
			found = true
			return false
		}
		return true
	})
	return found
}

func callIsTracePrimitive(info *types.Info, call *ast.CallExpr) bool {
	obj := calleeObject(info, call)
	if obj == nil {
		return false
	}
	pkg := obj.Pkg()
	if pkg == nil || pkg.Path() != calltracePkg {
		return false
	}
	_, ok := tracePrimitiveNames[obj.Name()]
	return ok
}

func bodyIsThinSamePackageDelegate(body *ast.BlockStmt, info *types.Info, pkg *packages.Package, self *ast.FuncDecl, cache *pkgSatisfyCache) bool {
	if body == nil || len(body.List) == 0 {
		return false
	}
	callee := thinDelegateCallee(body)
	if callee == nil {
		return false
	}
	obj := calleeObject(info, callee)
	if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != pkg.PkgPath {
		return false
	}
	if obj == info.ObjectOf(self.Name) {
		return false
	}
	fnObj, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	calleeName := formatTypesFuncName(fnObj)
	selfName := formatFuncName(self)
	if calleeName == selfName {
		return false
	}
	layer, known := cache.layer(pkg.PkgPath, calleeName)
	if !known {
		return false
	}
	return layer == layerDirectSlog || layer == layerTraceDelegate
}

func thinDelegateCallee(body *ast.BlockStmt) *ast.CallExpr {
	if len(body.List) != 1 {
		return nil
	}
	switch s := body.List[0].(type) {
	case *ast.ExprStmt:
		call, ok := s.X.(*ast.CallExpr)
		if !ok {
			return nil
		}
		return call
	case *ast.ReturnStmt:
		if len(s.Results) != 1 {
			return nil
		}
		call, ok := s.Results[0].(*ast.CallExpr)
		if !ok {
			return nil
		}
		return call
	default:
		return nil
	}
}

func calleeObject(info *types.Info, call *ast.CallExpr) types.Object {
	if call == nil {
		return nil
	}
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		if sel, ok := info.Selections[fun]; ok {
			return sel.Obj()
		}
		if obj, ok := info.Uses[fun.Sel]; ok {
			return obj
		}
	case *ast.Ident:
		if obj, ok := info.Uses[fun]; ok {
			return obj
		}
	}
	return nil
}

func formatTypesFuncName(fn *types.Func) string {
	sig := fn.Type().(*types.Signature)
	if recv := sig.Recv(); recv != nil {
		return formatTypesType(recv.Type()) + "." + fn.Name()
	}
	return fn.Name()
}

func formatTypesType(t types.Type) string {
	switch tt := t.(type) {
	case *types.Pointer:
		if named, ok := tt.Elem().(*types.Named); ok {
			return "*" + named.Obj().Name()
		}
		return "*" + tt.Elem().String()
	case *types.Named:
		return tt.Obj().Name()
	default:
		return t.String()
	}
}
