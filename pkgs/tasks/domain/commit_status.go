package domain

// CommitStatus is the worker-indexed lifecycle state of one git commit row
// (ADR-0016). Observation persists even when execute admission gates fail.
type CommitStatus string

const (
	// CommitEligible passed all execute gates for the indexing cycle.
	CommitEligible CommitStatus = "eligible"
	// CommitObserved is in cycle ancestry after runner exit but gates blocked admission.
	CommitObserved CommitStatus = "observed"
	// CommitInherited was copied from a prior attempt on resume zero-new-commit ingest.
	CommitInherited CommitStatus = "inherited"
	// CommitSuperseded was indexed but no longer appears in cycle ancestry.
	CommitSuperseded CommitStatus = "superseded"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ValidCommitStatus reports whether s is a known commit status value.
func ValidCommitStatus(s CommitStatus) bool {
	switch s {
	case CommitEligible, CommitObserved, CommitInherited, CommitSuperseded:
		return true
	default:
		return false
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// CommitStatusRank orders statuses for task-wide dedupe (higher wins).
func CommitStatusRank(s CommitStatus) int {
	switch s {
	case CommitEligible:
		return 4
	case CommitInherited:
		return 3
	case CommitObserved:
		return 2
	case CommitSuperseded:
		return 1
	default:
		return 0
	}
}

// ExecuteCriteriaReportAttemptSeq is the attempt_seq used when mirroring
// criteria-report.json at execute phase end. Verify attempts use 1..N;
// this sentinel avoids colliding with the verify retry budget.
const ExecuteCriteriaReportAttemptSeq int64 = 1_000_000
