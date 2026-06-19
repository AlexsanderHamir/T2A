package orchestration

// ExecuteRunnerOutcome classifies runner.Run result at the harness I/O boundary.
type ExecuteRunnerOutcome int

const (
	ExecuteRunnerOutcomeOK ExecuteRunnerOutcome = iota
	ExecuteRunnerOutcomeTimeout
	ExecuteRunnerOutcomeNonZeroExit
	ExecuteRunnerOutcomeInvalidOutput
	ExecuteRunnerOutcomeError
)

// ExecuteCommitIngestSummary carries post-execute commit observe/admit facts.
type ExecuteCommitIngestSummary struct {
	GitSnapshotSkipped bool
	IngestAttempted    bool
	IngestErr          bool
	FailReason         string
}

// ExecutePostRunInput is the pure input to DecideExecutePostRun after runner
// and optional commit ingest complete.
type ExecutePostRunInput struct {
	RunnerOutcome     ExecuteRunnerOutcome
	OperatorCancelled bool
	ContextCancelled  bool
	EvidenceRecovery  bool
	CommitIngest      ExecuteCommitIngestSummary
}
