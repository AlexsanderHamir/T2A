package worker

import (
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness"
)

// Public re-exports so cmd/taskapi and tests keep importing worker without
// churn. See docs/adr/ADR-0005-extract-agent-harness.md.

type (
	Options             = harness.Options
	RunMetrics          = harness.RunMetrics
	CycleChangeNotifier = harness.CycleChangeNotifier
	ProgressNotifier    = harness.ProgressNotifier
)

const (
	CancelledByOperatorReason   = harness.CancelledByOperatorReason
	DefaultShutdownAbortTimeout = harness.DefaultShutdownAbortTimeout
	PanicReason                 = harness.PanicReason
	DefaultReportDirSubdir      = harness.DefaultReportDirSubdir
	ShutdownReason              = harness.ShutdownReason
)
