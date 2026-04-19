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
	"github.com/AlexsanderHamir/T2A/internal/version\tString":                    {},
	"github.com/AlexsanderHamir/T2A/internal/version\tPrometheusBuildInfoLabels": {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents\t*MemoryQueue.BufferDepth":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents\t*MemoryQueue.BufferCap":         {},
	"github.com/AlexsanderHamir/T2A/pkgs/repo\tisMentionDelimiter":               {},
	// Header-only helper on every response; JSON paths log via setJSONHeaders / setAPISecurityHeaders.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/apijson\tApplySecurityHeaders": {},
	// Thin wrapper over internal/version.String (already excluded); health and JSON embed version without duplicating logs here.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tServerVersion": {},
	// Prometheus metrics wrapper: per-chunk Write / Flush must not allocate log attrs on hot paths.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\t*metricsHTTPResponseWriter.WriteHeader": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\t*metricsHTTPResponseWriter.Write":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\t*metricsHTTPResponseWriter.Flush":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\t*metricsHTTPResponseWriter.statusCode":  {},
	// Test/metrics accessor; RecordSSESubscriberGauge already traces.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\tSSESubscribersGauge": {},
	// Thin re-exports to pkgs/tasks/middleware (slog lives on the real implementations).
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tWithRecovery":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tWithHTTPMetrics":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tWithAccessLog":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tWithRateLimit":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tWithAPIAuth":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tWithRequestTimeout":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tWithMaxRequestBody":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tWithIdempotency":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tRateLimitPerMinuteConfigured":  {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tAPIAuthEnabled":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tMaxRequestBodyBytesConfigured": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tRequestTimeout":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tIdempotencyTTL":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tIdempotencyCacheLimits":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tclearIdempotencyStateForTest":  {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tHasValidBearerToken":           {},
	// Test-only httptest wiring for black-box handler tests (no production logging).
	"github.com/AlexsanderHamir/T2A/internal/handlertest\tNewServer":                    {},
	"github.com/AlexsanderHamir/T2A/internal/handlertest\tNewServerWithStore":           {},
	"github.com/AlexsanderHamir/T2A/internal/handlertest\tNewServerWithRepo":            {},
	"github.com/AlexsanderHamir/T2A/internal/httpsecurityexpect\tAssertBaselineHeaders": {},
	// Prometheus Collector hooks; no per-scrape slog (scrapes can be frequent).
	"github.com/AlexsanderHamir/T2A/internal/taskapi\t*sqlDBStatsCollector.Describe": {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi\t*sqlDBStatsCollector.Collect":  {},
	// Store Prometheus latency helper; per-call slog would flood and duplicate SQL traces.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel\tDeferLatency": {},

	// funclogmeasure: stub exemptions for pure helpers that have no observable
	// behavior worth logging. Each was previously gated only by the no-op
	// `_ = slog.Default().Enabled(context.Background(), ...)` line, which
	// satisfied the analyzer at the cost of one extra function-table read
	// per call and a misleading "this function logs" claim. Skip-listing
	// here is the documented escape hatch the rule reserves for trivial
	// pure helpers; the calling functions still log so a request trace is
	// never lost. See Session 24 of .agent/backend-improvement-agent.log
	// for the audit trail and rationale.
	//
	// cmd/taskapi/main.go: main() is already a thin wrapper around run();
	// run() is the slog setup point, so logging in main() before the JSON
	// sink is configured would emit on stderr before the file exists
	// (see the in-file comment).
	"github.com/AlexsanderHamir/T2A/cmd/taskapi\tmain": {},
	// pkgs/tasks/handler/httplog_io.go: pure attribute-builder helpers
	// for the http.io trace line. The actual slog.Log call lives on the
	// calling function (logHTTPRequest / logHTTPResponse); these helpers
	// only flatten request/response state into []any so the per-call cost
	// is one slice append per field, not one slog formatter pass.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\ttruncateRunes":         {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\ttaskCreateInputFields": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\ttaskPatchInputFields":  {},
	// pkgs/tasks/apijson/truncate.go: UTF-8-safe truncation used by the
	// http.io trace line preview fields above; same rationale as the
	// helpers in handler/httplog_io.go (pure transformation, no I/O).
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/apijson\tTruncateUTF8ByBytes": {},
	// pkgs/tasks/logctx/log_seq.go: ContextWithLogSeq attaches a counter
	// pointer to the request context; logSeqFromContext reads it back.
	// Both are called once per request from middleware that already logs
	// the http.access line. WrapSlogHandlerWithLogSequence is a one-shot
	// wiring helper called at startup from cmd/taskapi/run.go (which
	// logs the wiring step itself).
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx\tContextWithLogSeq":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx\tlogSeqFromContext":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx\tWrapSlogHandlerWithLogSequence": {},

	// pkgs/tasks/domain: per-row hot path. Every database/sql Scan and Value
	// call goes through one of the typed Scan/Value pairs below; logging on
	// each per-type wrapper would emit two trace lines per row (the wrapper
	// + the underlying scanStringEnum / valueStringEnum). The two generic
	// helpers carry a single slog.Debug each and ARE the canonical trace
	// line for these per-row mutations. See Session 26 in
	// .agent/backend-improvement-agent.log.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\t*Status.Scan":      {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tStatus.Value":      {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\t*Priority.Scan":    {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tPriority.Value":    {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\t*TaskType.Scan":    {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskType.Value":    {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\t*EventType.Scan":   {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tEventType.Value":   {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\t*Actor.Scan":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tActor.Value":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\t*Phase.Scan":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tPhase.Value":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\t*CycleStatus.Scan": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tCycleStatus.Value": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\t*PhaseStatus.Scan": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tPhaseStatus.Value": {},
	// pkgs/tasks/domain: GORM TableName methods are pure constant-string
	// returns invoked at reflection time by GORM (no per-row hit, but also
	// no decision logic); their callers (gorm itself) own the SQL trace.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskChecklistItem.TableName":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskChecklistCompletion.TableName": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskCycle.TableName":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskCyclePhase.TableName":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tAppSettings.TableName":             {},
	// pkgs/tasks/domain: pure predicates / constructors with no I/O. Every
	// caller (store.StartPhase, store.CompletePhase, store.GetAppSettings)
	// already logs the surrounding decision with the relevant context.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tValidPhaseTransition": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTerminalCycleStatus":  {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTerminalPhaseStatus":  {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tDefaultAppSettings":   {},
}

func shouldSkipSlogRequirement(pkgPath, funcName string) bool {
	_, ok := skipSlogRequirement[pkgPath+"\t"+funcName]
	return ok
}

type analyzeOpts struct {
	tests       bool
	includeTool bool
}

// isNPMWebNodeModulesGo reports paths under web/node_modules (npm may ship
// auxiliary Go packages such as flatted). Those files are not T2A product code
// and must not affect funclogmeasure -enforce.
func isNPMWebNodeModulesGo(path string) bool {
	p := filepath.ToSlash(path)
	if strings.Contains(p, "/web/node_modules/") {
		return true
	}
	return strings.HasPrefix(p, "web/node_modules/")
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
		if err := accumulateViolationsFromPackage(pkg, fset, opts, &rep); err != nil {
			return nil, err
		}
	}

	return &rep, nil
}

func accumulateViolationsFromPackage(pkg *packages.Package, fset *token.FileSet, opts analyzeOpts, rep *report) error {
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
	return nil
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
