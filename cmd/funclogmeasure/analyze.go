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

// skipSlogRequirement marks pkg+func pairs that intentionally omit log/slog
// despite funclogmeasure -enforce. Each entry MUST carry a per-block
// rationale comment explaining why logging here would be redundant or
// harmful, so a future contributor adding a new entry has a concrete
// model to follow.
//
// There are exactly four legitimate categories for entries in this map.
// New entries that don't fit one of these categories should be questioned:
//
//  1. Tool-required no-ops — pure helpers / attribute builders / context
//     wiring whose only relationship to slog was a `_ = slog.Default()
//     .Enabled(...)` stub added solely to satisfy this analyzer. The
//     surrounding caller already logs the decision (e.g. taskCreateInputFields
//     runs only inside debugHTTPRequest's gate; cmd/taskapi/main runs
//     before the slog JSON sink is installed). See Sessions 25 and 27 in
//     .agent/backend-improvement-agent.log.
//
//  2. Hot-path optimizations — per-row / per-frame / per-scrape helpers
//     where one slog.Debug per invocation would flood the trace volume
//     for marginal observability value. The canonical trace lives on a
//     chokepoint one layer down (e.g. scanStringEnum / valueStringEnum
//     for the per-type Scan/Value methods on enum types; the access-log
//     middleware for context-id reads). See Session 26.
//
//  3. Delegate-already-logs orchestration — public helpers whose body is
//     a one-line call to a private helper that already emits the trace
//     line (e.g. HelperIOIn → helperDebugIn → slog.Log). A per-function
//     log here would multi-count every observed invocation. See
//     Session 27.
//
//  4. Re-export thin wrappers — package-boundary aliases that exist
//     only to avoid leaking the dependency on the real implementation
//     (e.g. handler.WithRecovery wraps middleware.WithRecovery; slog
//     lives on the real one). The wrapper carries no decision logic.
//
// Entries are grouped below by category; each block begins with a short
// comment naming the category and the per-package rationale. See
// docs/architecture.md for the broader trace-line contract.
var skipSlogRequirement = map[string]struct{}{
	"github.com/AlexsanderHamir/T2A/internal/version\tString":                    {},
	"github.com/AlexsanderHamir/T2A/internal/version\tPrometheusBuildInfoLabels": {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents\t*MemoryQueue.BufferDepth":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents\t*MemoryQueue.BufferCap":         {},
	"github.com/AlexsanderHamir/T2A/pkgs/repo\tisMentionDelimiter":               {},
	// Header-only helper on every response; JSON paths log via setJSONHeaders / setAPISecurityHeaders.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/apijson\tApplySecurityHeaders": {},
	// Header-only sibling of ApplySecurityHeaders for ETag/304-revalidating
	// GETs. Same rationale as the parent entry: this is a single block of
	// h.Set() calls, no decision logic to trace; per-response logging
	// happens at the caller (writeJSONWithETag in handler/handler_http_json.go).
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/apijson\tApplyRevalidatableHeaders": {},
	// Pure SHA-256 hash function and tolerant header comparison. Both are
	// called on every conditional GET; per-call slog would flood the trace
	// volume and the decisions are logged once at the writeJSONWithETag
	// chokepoint that calls them.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/apijson\tComputeETag":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/apijson\tIfNoneMatchMatches": {},
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

	// pkgs/agents/harness: error/Stringer interface methods called on
	// every wrapped error (Error, Unwrap) and a tiny int-to-string
	// helper used inside summariseTamperedPaths. Logging here would
	// fire on every error.Error() call (formatter, comparison,
	// log.Fatal stack walk), which is hot-path category 2. The
	// surrounding integrity-check entry point (checkVerifyIntegrity)
	// already logs decisions and tampering verdicts at a stable
	// operation key.
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\t*TamperedError.Error": {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*execErr.Error":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*execErr.Unwrap":         {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\titoa":                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tItoa":                    {},

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
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskChecklistItem.TableName":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskChecklistCompletion.TableName":  {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskCycle.TableName":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskCyclePhase.TableName":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskCycleStreamEvent.TableName":     {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskCycleCriteriaReport.TableName":  {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskCycleVerifyReport.TableName":    {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskChecklistItemCommand.TableName": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskCycleCommandRun.TableName":      {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskCycleCommit.TableName":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tAppSettings.TableName":              {},
	// pkgs/tasks/domain: pure predicates / constructors with no I/O. Every
	// caller (store.StartPhase, store.CompletePhase, store.GetAppSettings)
	// already logs the surrounding decision with the relevant context.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tValidPhaseTransition":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tValidInterruptResumeTransition": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTerminalCycleStatus":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTerminalPhaseStatus":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tDefaultAppSettings":             {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tGateCriteriaAllDone":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\t*PendingRetry.Validate":         {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\t*PendingRetry.Clone":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\t*PendingRetry.Equal":            {},

	// pkgs/tasks/calltrace: thin orchestration / pure-context helpers that
	// either delegate to helperDebugIn/helperDebugOut (which DO log) or
	// only mutate / read context. RunObserved emits the helper.io trace
	// pair through its delegates so a per-function log here would
	// triple-count every observed helper invocation; HelperIOIn /
	// HelperIOOut are one-line public wrappers over the same delegates;
	// Push / Path / WithRequestRoot are pure context-stack manipulation
	// embedded into other trace lines (call_path field on every
	// http.access / http.io / helper.io frame).
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace\tRunObserved":     {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace\tHelperIOIn":      {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace\tHelperIOOut":     {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace\tPush":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace\tPath":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace\tWithRequestRoot": {},
	// pkgs/tasks/logctx: pure context-read helper for the request id;
	// embedded by the access-log middleware into the http.access trace.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx\tRequestIDFromContext": {},
	// pkgs/tasks/store/internal/settings: pure five-pointer-nil predicate
	// for the no-op short-circuit on PATCH /settings; the surrounding
	// handler emits the trace line with the decision context.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/settings\tPatch.IsEmpty": {},

	// Session 38 — Category (2) pure transforms / hot-path helpers and one
	// Category (3)-style argv builder: recent commits added these without
	// slog; logging at each site would duplicate traces already emitted by
	// the caller (OpenPostgres, phase event marshal, Adapter.Run, ListModels,
	// handler.Handler.create).
	//
	// pkgs/tasks/postgres: DSN string normalization only; connection open path
	// logs outcomes.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/postgres\tensureQueryExecModeSimpleProtocol": {},
	// pkgs/tasks/store/internal/cycles: JSON copy + rune clamp for phase event
	// payloads; store write paths already trace via kernel / mirror helpers.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/cycles\tphaseDetailsForEventPayload":   {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/cycles\ttruncatePhaseEventDetailValue": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/cycles\ttruncateStringRunes":           {},
	// pkgs/agents/runner/cursor: argv assembly each Run; Adapter.Run logs first.
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\t*Adapter.argvFor": {},
	// clipSummaryRunes: stderrFirstLineHint logs before calling clipSummaryRunes.
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tclipSummaryRunes": {},
	// Deterministic shaping of Result.Details; Run / worker surfaces failures.
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\ttitleForFailureKind":   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tclassifyCursorFailure": {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tmergeDetailsJSON":      {},
	// parseListModelsOutput: ListModels logs at entry before parsing stdout.
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tparseListModelsOutput": {},
	// resolveTaskRunnerModel: handler.Handler.create logs the request trace first.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tresolveTaskRunnerModel": {},

	// Session 39 — Stage 3 system health aggregator (GET /system/health).
	// internal/systemhealth.Read is the canonical chokepoint trace for one
	// snapshot scrape; everything below is either a Category (1) pure
	// helper / constructor, a Category (2) per-MetricFamily dispatcher
	// invoked once per scrape per family (logging here would emit ~30+
	// lines per /system/health hit and bury the actual scrape outcome),
	// or a Category (3) thin wrapper whose body is a one-line call back
	// into Read (the snapshot trace already fires there). See
	// internal/systemhealth/snapshot.go for the chokepoint.
	"github.com/AlexsanderHamir/T2A/internal/systemhealth\tnewZeroSnapshot":       {},
	"github.com/AlexsanderHamir/T2A/internal/systemhealth\treadBuildFromVersion":  {},
	"github.com/AlexsanderHamir/T2A/internal/systemhealth\tapplyFamily":           {},
	"github.com/AlexsanderHamir/T2A/internal/systemhealth\tapplyHTTPRequests":     {},
	"github.com/AlexsanderHamir/T2A/internal/systemhealth\tclassifyStatus":        {},
	"github.com/AlexsanderHamir/T2A/internal/systemhealth\tapplyHTTPDuration":     {},
	"github.com/AlexsanderHamir/T2A/internal/systemhealth\tmergeHistograms":       {},
	"github.com/AlexsanderHamir/T2A/internal/systemhealth\tpercentileFromBuckets": {},
	"github.com/AlexsanderHamir/T2A/internal/systemhealth\tapplyAgentRuns":        {},
	"github.com/AlexsanderHamir/T2A/internal/systemhealth\tapplyUptime":           {},
	"github.com/AlexsanderHamir/T2A/internal/systemhealth\tgaugeSum":              {},
	"github.com/AlexsanderHamir/T2A/internal/systemhealth\tcounterSum":            {},
	"github.com/AlexsanderHamir/T2A/internal/systemhealth\tlabelValue":            {},
	"github.com/AlexsanderHamir/T2A/internal/systemhealth\tSnapshot.String":       {},
	// Handler thin wrappers: defaultSystemHealthSnapshotter returns a
	// closure over systemhealth.Read; *Handler.snapshotSystemHealth is a
	// one-line dispatch to that closure or the test override. Both are
	// invoked from systemHealth (which DOES log the operation).
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tdefaultSystemHealthSnapshotter": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\t*Handler.snapshotSystemHealth":  {},

	// Session 40 — MVP lean-down follow-up. These helpers are either pure
	// transformations, heap.Interface callbacks, metric counter wrappers, or
	// SSE/RUM write helpers whose callers already emit the trace line with
	// request/event context. Adding slog to each would turn hot-path utility
	// code into noisy trace spam without improving operator diagnosis.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/cycles\tFailureSurfaceMessage":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/cycles\tstandardizedMessageFromDetails":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/cycles\tfailureKindFromDetails":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/cycles\thumanizeFailureKind":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/cycles\ttruncateReasonRunes":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/stats\tresolveRecentFailureReason":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/stats\tobservabilityReasonFromPhaseAndCycle":  {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store\t*Store.schedulePickupWake":                            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store\t*Store.cancelPickupWake":                              {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents\twakeHeap.Len":                                              {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents\twakeHeap.Less":                                             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents\twakeHeap.Swap":                                             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents\t*wakeHeap.Push":                                            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents\t*wakeHeap.Pop":                                             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents\t*PickupWakeScheduler.Schedule":                             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents\t*PickupWakeScheduler.Cancel":                               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents\t*PickupWakeScheduler.Stop":                                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents\t*PickupWakeScheduler.resetTimerLocked":                     {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents\t*PickupWakeScheduler.fire":                                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents\t*PickupWakeScheduler.tryNotify":                            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tsplitNDJSON":                                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\temitProgressFromLine":                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tprogressFromLine":                            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\ttextContent":                                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tfirstNonEmpty":                               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\ttoolProgressMessage":                         {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tscanStdoutLines":                             {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\tRUMEventsAcceptedCounter":                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\tRUMEventsDroppedCounter":                         {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\tRecordRUMAccepted":                               {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\tRecordRUMDropped":                                {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\tRecordRUMMutationStarted":                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\tRecordRUMMutationOptimisticApplied":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\tRecordRUMMutationSettled":                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\tRecordRUMMutationRolledBack":                     {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\tRecordRUMSSEReconnected":                         {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\tRecordRUMSSEResyncReceived":                      {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware\tRecordRUMWebVital":                               {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tparseCycleFailuresQuery":                            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tfoldRUMEvent":                                       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tvalidDurationSeconds":                               {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\trumStatusBucket":                                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\trecentFailuresToJSON":                               {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tDefaultSSEHubOptions":                               {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tNewSSEHubWith":                                      {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\t*SSEHub.snapshotSinceLocked":                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\t*SSEHub.appendRingLocked":                           {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/realtime\tCoalesceKey":                                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\t*SSEHub.evictSubscriber":                            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\t*SSEHub.LastEventID":                                {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tparseLastEventIDHeader":                             {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\twriteBufferedEvent":                                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\twriteResyncFrame":                                   {},
	"github.com/AlexsanderHamir/T2A/cmd/taskapi\tresolveTaskAPILogDir":                                      {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\t*runProgressSSEAdapter.shouldDrop":        {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker/policy\tDecideSchedulingIdleHint":          {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker/policy\tDecideIdle":                        {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker/policy\tInstanceMatchesSettings":           {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker/policy\tVerifyRunnerStatus":                {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\tinstanceSnapshot":                         {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\t*Supervisor.loadApplySettingsSnapshot":    {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\tbaseEffectiveSettings":                    {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\t*Supervisor.finishApplySettings":          {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\t*Supervisor.clearCurrentInstance":         {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\t*Supervisor.handleApplySettingsIdle":      {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\t*Supervisor.probeExecuteRunner":           {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\tverifyRunnerStatusForInstance":            {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\t*Supervisor.handleApplySettingsUnchanged": {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\t*Supervisor.HasRunningInstance":           {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\t*Supervisor.RunningInstanceIdentity":      {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\t*Supervisor.RunningInstanceRepoRoot":      {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\t*Supervisor.RunningInstanceRunnerVersion": {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\t*Supervisor.SetProbeForTest":              {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\t*Supervisor.SetProbeBudgetForTest":        {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\t*Supervisor.BuildVerifyRunnerForTest":     {},
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker\t*Supervisor.ProbeSchedulingHintForTest":   {},

	// Session 41 — Projects / project-context vertical slice. Same categories
	// as existing skips: GORM TableName constant returns (reflection-time only;
	// SQL trace is GORM/store), pure constructors and validation/patch helpers
	// invoked only from store or handler paths that already emit the operation
	// trace, adapterkit string/env/exec utilities (Run/Probe log at boundaries),
	// cursor stream-json field extractors (emitProgressFromLine / Adapter paths
	// log), worker context assembly (cycle run logs), and handler JSON/query
	// parsers (HTTP handlers log first).
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tProjectContextItem.TableName":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tProjectContextEdge.TableName":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskContextSnapshot.TableName":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tTaskDependency.TableName":                     {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tDefaultProject":                               {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects\tvalidateContextEdgeFields":   {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects\tvalidateContextEdgePatch":    {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects\tvalidateContextRelation":     {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects\tvalidateEdgeNodesExist":      {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects\tapplyContextEdgePatch":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects\tnormalizeEdgeNodeFilter":     {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects\tvalidateProjectPatch":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects\tvalidateDefaultProjectPatch": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects\tapplyProjectPatch":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects\tvalidateContextPatch":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects\tapplyContextPatch":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects\tvalidateContextKind":         {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects\ttrimOptional":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects\tmapNotFound":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects\tmapWriteError":               {},

	// Flat task hierarchy: pure validation/patch helpers and SQL predicate
	// builders; store CRUD and handler paths emit the operation trace.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\t*TaskGate.GateBlocksWorker":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tValidateTaskTag":                              {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tValidateTaskTags":                             {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tValidateTaskMilestone":                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tNormalizeTaskTags":                            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/ready\tapplyDequeuableTaskPredicates":  {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\thydrateDependsOn":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tensureTaskExists":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tvalidateParentIsRootTask":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tisUniqueViolation":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tapplyListFilter":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tApplyTaskGateAction":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tapplyTagsPatch":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tapplyMilestonePatch":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tapplyGatePatch":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tapplyDependsOnPatch":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tlistDependenciesInTx":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tnormalizeCreateTaskModelFields": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tgateFieldPatchToStore":                       {},

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.selectedProjectContext": {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\trenderProjectContext":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\testimateTokens":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tpromptWithProjectContext":        {},
	// Criteria verification guardrail: pure helpers and thin wrappers; pipeline
	// entry points (runVerificationPipeline, runVerifyChecks, runLLMVerifyAgent,
	// applyVerifiedCompletions, completeChecklistLegacy) emit the canonical trace.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tValidVerifierKind":                              {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist\tisTaskCycleRunningInTx":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist\tvalidateEvidencePayload":      {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist\tNormalizeVerifyCommandInputs": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist\tcommandsForItemInTx":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist\tloadItemForCommandEdit":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist\tValidateCriteriaMutable":      {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist\tvalidateCriteriaMutable":      {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist\tcriterionLockedByCompletion":  {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tValidCommitStatus":                              {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tCommitStatusRank":                               {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/commits\tdedupeCommitsBySHA":             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tassignCommitAdmissionStatuses":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\treasonRemediation":                            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tclassifyParentFailure":                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\trouteResumeEntry":                             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tcontinuationSufficient":                       {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tformatCommitsByStatusForResume":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tcomposeContinuationPrompt":                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tlastExecutePhase":                             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tphaseSummary":                                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tisExecuteGateReason":                          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tbundleToCheckpoint":                           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tappendExecuteHarnessFeedback":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tparseCriteriaReportPartial":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tparentFailureReason":                          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\trunnerFeedbackFromPhase":                      {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tscopeFilesFromExecutePhase":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tgitStatusPorcelain":                           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\trunnerDetailsExcerpt":                         {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tresolveRetryParentCycleInTx":      {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store\tNormalizeVerifyCommands":                         {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\ttruncateBytes":                                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tcriteriaReportPath":                           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tverifyReportPath":                             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\treportCycleDir":                               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tensureReportCycleDir":                         {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tcleanupReportDir":                             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tscrubCycleArtifacts":                          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\treadJSONFile":                                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tparseCriteriaReport":                          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tparseVerifyReport":                            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tinjectCriteria":                               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tappendVerifyFeedback":                         {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.invokeRunnerWithTask":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tgitDiff":                                      {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tencodeCriteriaSnapshot":                       {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tcommandEvidenceDir":                           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tcommandArtifactBase":                          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\ttruncateCommandOutput":                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tpreviewCommandOutput":                         {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tshellCommand":                                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tdefaultShellExec":                             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tformatCommandEvidenceSection":                 {},
	// pkgs/agents/worker: thin queue-consumer wiring; harness owns cycle trace.
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker\tNewWorker":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker\t*Worker.CancelCurrentRun": {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker\t*Worker.clock":            {},
	// pkgs/agents/harness: constructor and SSE/cancel plumbing without I/O.
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tNew":                                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.setCurrentRunCancel":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.consumeOperatorCancel":     {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.publish":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.publishProgress":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tCombineStreams":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tClipRunes":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tRedactedTail":             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tTrimLeadingPartialRune":   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tBuildEnv":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tIsDeniedEnvKey":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tLiveHomePaths":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tDefaultExec":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tDefaultStreamExec":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tScanStdoutLines":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tNormalizePipeReadError":   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tIsClosedPipeReadError":    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tDefaultProbeFunc":         {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tRunProbe":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tResolveBinaryPath":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tFirstNonEmptyLine":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tTrimForLog":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tDefaultRedactionPolicy":   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tRedact":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit\tredactEnvAssignments":     {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tenvPolicy":                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tredactionPolicy":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tfirstRawMessage":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\ttoolCallDetails":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tpreferredToolCallKeys":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\ttoolCallBodyDetails":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\ttoolNameFromCallKey":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tfunctionArguments":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\trawString":                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\ttoolInputSummary":             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\treadFileSummary":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tsearchFilesSummary":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tripgrepSummary":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\teditSummary":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tpathActionSummary":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tshellSummary":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tgenericInputSummary":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tstringField":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tinputField":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tpathLabel":                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tscopeLabel":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tlineRange":                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tnumericField":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tintString":                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tshellCommandLabel":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor\tclipProgressSummary":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tparseProjectContextPath":             {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tparseProjectContextEdgePath":         {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tparseProjectListParams":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tparseProjectContextListParams":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tparseBoundedLimit":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tfirstQueryValue":                     {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tprojectPatchJSON.isEmpty":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tprojectContextPatchJSON.isEmpty":     {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tprojectContextEdgePatchJSON.isEmpty": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tprojectFieldPatchToStore":            {},

	// Epic scheduling (ADR-0008): pure dependency predicates, JSON wire parsers,
	// and thin store facades whose callers already trace (notifyUnblockedDependents,
	// checklist backfill, ListDependencyEdges).
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tValidDependencySatisfies":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tNormalizeDependencySatisfies":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tListDependencies":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tDependencyEdgeIDs":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tEdgeSatisfied":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tlistDependencyEdgesInTx": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store\tBackfillCriteriaSatisfiedAt":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store\t*Store.NotifyUnblockedDependents":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store\t*Store.HasIncompleteSubtasks":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tparseDependsOnWire":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tparseCreateChecklistItems":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\t*dependsOnWire.UnmarshalJSON":         {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\t*dependsOnPatchWire.UnmarshalJSON":    {},

	// Prompt automations (ADR-0013): GORM TableName, pure domain validation,
	// store validators/resolvers, and handler wire parsers — same categories as
	// project-context and dependency slices; CRUD handlers trace first.
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tAutomation.TableName":                            {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tValidateAutomationFields":                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tNormalizeAutomationState":                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain\tValidateAutomationSelections":                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/automations\tValidateSelectionIDs":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/automations\tResolveForTask":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/automations\tassertTitleAvailable":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/automations\tmapNotFound":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/automations\tmapWriteError":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks\tapplyAutomationSelectionsOnCreate": {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tparseAutomationListParams":                      {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tautomationPatchJSON.isEmpty":                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tparseAutomationSelectionsWire":                  {},

	// Cycle commit tracking (ADR-0014): git subprocess helpers, JSON/prompt
	// formatters, and handler DTO mappers. runCycleLoop logs git snapshot and
	// commit_ingest_err; getTaskCycleVerdicts logs the HTTP trace; captureExecuteGitSnapshot
	// logs at entry. Per-git-call slog would flood execute ingest.
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tapplyOperatorCancelToRunResult":    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\trecordPassedCriterionVerdicts":     {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tunionPreviouslyPassedVerdicts":     {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.runCycleLoop":             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.repoRootForGit":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.priorCycleBaseSHA":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tgitCycleBaseFromPhaseDetails":      {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tgitSnapshotToMap":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tmergeRunnerDetailsWithGit":         {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tparseCommitReports":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tgitRevListRange":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tmatchReportedSHAInAncestry":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tgitCommitDetails":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tgitBranchContaining":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tgitWorkingTreeDirty":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tresolvePhaseCommits":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tbuildReportedBranchMap":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tgitCommitExists":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tbuildInheritedCommitEntries":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tretryModeFromCycleMeta":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tmergeCycleMetaBytes":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.Run":                      {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.runFreshCycle":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.loadVerifyCheckpointData": {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.resolveFreshRetryAnchor":  {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tgitResetHardClean":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.ingestExecuteCommits":     {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tformatGitContextForPrompt":         {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tformatKnownCommitsForResume":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler\tcycleGitContextFromCommits":         {},

	// pkgs/agents/harness/internal/* (ADR-0017): pure helpers, formatters, and thin
	// service wiring. Cycle entry points (runCycleLoop, RunPipeline, Run) emit canonical trace.
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports\tReportCycleDir":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports\tCriteriaReportPath":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports\tVerifyReportPath":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports\tEnsureReportCycleDir":             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports\tScrubCycleArtifacts":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports\tCleanupReportDir":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports\treadJSONFile":                     {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports\tParseCriteriaReportPartial":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports\tParseCriteriaReport":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports\tParseVerifyReport":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports\tvalidateSchemaVersion":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports\tvalidateCriteriaReportSchema":     {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports\tvalidateVerifyReportSchema":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tNewService":                           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tNewExecRepo":                          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tDefaultRepo":                          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tRun":                                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*Service.repo":                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*Service.Repo":                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*Service.ReportDir":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*Service.SetReportDir":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*Service.revListRange":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*Service.commitDetails":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*Service.branchContaining":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*Service.resolvePhaseCommits":         {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*Service.commitExists":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*Service.BuildInheritedCommitEntries": {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*Service.PriorCycleBaseSHA":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*Service.parseCommitReports":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*Service.resolveFreshRetryAnchor":     {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\t*Service.ResolveFreshRetryAnchor":     {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tMatchReportedSHAInAncestry":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tAssignCommitAdmissionStatuses":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tassignCommitAdmissionStatuses":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tResolvePhaseCommitsFromReports":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tphaseContextFromSnapshot":             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tCycleBaseFromPhaseDetails":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tsnapshotToMap":                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tMergeRunnerDetailsWithGit":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tWorkingTreeDirty":                     {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tStatusPorcelain":                      {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tScopeFilesFromPhaseDetails":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tFormatGitContextForPrompt":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tFormatKnownCommitsForResume":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tReasonRemediation":                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tIsExecuteGateReason":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\treadCriteriaReportJSON":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tResetHardClean":                       {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git\tresetHardClean":                       {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt\tInjectCriteria":                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt\tAppendVerifyFeedback":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt\tAppendExecuteHarnessFeedback":      {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt\tInjectAutomations":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt\tComposeContinuation":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt\tFormatCommitsByStatusForResume":    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt\tBuildProjectContextSection":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt\tEstimateTokens":                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt\tWrapWithProjectContext":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt\tAppendOperatorRetryResumeNotice":   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt\tAppendResumeNotice":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt\tAppendGitCommitPolicy":             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt\tFormatKnownCommitsForResume":       {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt\tFormatVerifyDiffSection":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/resume\tNewService":                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/resume\t*Service.gitRepo":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/resume\t*Service.loadVerifyCheckpointData": {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/resume\tbundleToCheckpoint":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/resume\tclassifyParentFailure":             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/resume\trouteResumeEntry":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/resume\tparentFailureReason":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/resume\tcontinuationSufficient":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/resume\tlastExecutePhase":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/resume\tphaseSummary":                      {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/resume\trunnerFeedbackFromPhase":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/resume\trunnerDetailsExcerpt":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\tNewService":                        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\t*Service.SetReportDir":             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\t*Service.SetWorkingDir":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\t*Service.SetVerifyRunner":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\t*Service.publish":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\t*Service.recordVerdict":            {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\t*Service.observeDuration":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\t*Service.checkIntegrity":           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\t*Service.captureIntegritySnapshot": {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\t*Service.loadEligibleCommits":      {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\tcommandEvidenceDir":                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\tcommandArtifactBase":               {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\ttruncateCommandOutput":             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\tpreviewCommandOutput":              {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\tshellCommand":                      {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\tdefaultShellExec":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\tFormatCommandEvidenceSection":      {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\tgitDiff":                           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify\tDiffSection":                       {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/orchestration\tDecideVerifyRetry":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/orchestration\tVerifyDisabled":             {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.gitSvc":                                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.gitResetForFreshRetry":                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.checkVerifyIntegrity":                     {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tcaptureExecuteGitSnapshot":                         {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tcaptureIntegritySnapshot":                          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.resumeSvc":                                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tharnessVerdictsFromResume":                         {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.verifySvc":                                {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.loadVerificationSnapshot":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.completeChecklistLegacy":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.applyVerifiedCompletions":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.runVerificationPipeline":                  {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tformatVerificationFailedReason":                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tverifyDiffSection":                                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.persistCriteriaReports":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.reconstructCheckpoint":                    {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.loadCheckpointFromParent":                 {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.loadContinuationBundle":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.seedCrossCycleExecuteFromParent":          {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.mirrorParentCriteriaForVerifyOnly":        {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\t*Harness.failTaskAfterRetryPrep":                   {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tchecklistItemsForPrompt":                           {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tverifiedCriterionIDs":                              {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tcontinuationInputFromBundle":                       {},
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness\tcycleIDOrEmpty":                                    {},
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
