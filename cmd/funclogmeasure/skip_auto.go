package main

import (
	"go/ast"
	"go/types"
	"strings"
)

func autoExemptRule(fd *ast.FuncDecl, pkgPath string, info *types.Info) string {
	if fd == nil || fd.Name == nil {
		return ""
	}
	name := fd.Name.Name
	if fd.Recv != nil && len(fd.Recv.List) == 1 {
		if rule := autoExemptMethod(name, fd, info); rule != "" {
			return rule
		}
	}
	if name == "main" && strings.Contains(pkgPath, "/cmd/") && bodyCallsIdent(fd.Body, "run") {
		return "cmd.main"
	}
	return ""
}

func autoExemptMethod(name string, fd *ast.FuncDecl, info *types.Info) string {
	sig := info.Defs[fd.Name]
	fn, ok := sig.(*types.Func)
	if !ok {
		return ""
	}
	recvType := receiverType(fn)
	if recvType == nil {
		return ""
	}

	switch name {
	case "Error":
		if sig, ok := fn.Type().(*types.Signature); ok && sig.Params().Len() == 0 && sig.Results().Len() == 1 &&
			sig.Results().At(0).Type().String() == "string" {
			return "error.Error"
		}
	case "Unwrap":
		if methodReturnsError(fn) {
			return "error.Unwrap"
		}
	case "String":
		if sig, ok := fn.Type().(*types.Signature); ok && sig.Params().Len() == 0 && sig.Results().Len() == 1 &&
			sig.Results().At(0).Type().String() == "string" {
			return "fmt.Stringer"
		}
	case "Scan":
		if methodHasFirstParam(fn, "[]uint8") || methodHasFirstParam(fn, "[]byte") {
			return "sql.Scanner"
		}
	case "Value":
		if sig, ok := fn.Type().(*types.Signature); ok && sig.Results().Len() == 2 {
			return "driver.Valuer"
		}
	case "TableName":
		if bodyReturnsStringLiteral(fd.Body) {
			return "gorm.TableName"
		}
	case "Len", "Less", "Swap", "Push", "Pop":
		return "heap.Interface"
	case "Describe", "Collect":
		return "prometheus.Collector"
	}
	return ""
}

func receiverType(fn *types.Func) types.Type {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return nil
	}
	return sig.Recv().Type()
}

func methodReturnsError(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Results().Len() != 1 {
		return false
	}
	return sig.Results().At(0).Type() == types.Universe.Lookup("error").Type()
}

func methodHasFirstParam(fn *types.Func, want string) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Params().Len() != 1 {
		return false
	}
	return sig.Params().At(0).Type().String() == want
}

func bodyReturnsStringLiteral(body *ast.BlockStmt) bool {
	if body == nil || len(body.List) != 1 {
		return false
	}
	ret, ok := body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	_, ok = ret.Results[0].(*ast.BasicLit)
	return ok
}

func bodyCallsIdent(body *ast.BlockStmt, name string) bool {
	if body == nil {
		return false
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if fun.Name == name {
				found = true
				return false
			}
		}
		return true
	})
	return found
}
