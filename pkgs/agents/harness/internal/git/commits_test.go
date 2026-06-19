package git

import (
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestAssignCommitAdmissionStatuses(t *testing.T) {
	t.Parallel()
	entries := []struct {
		status domain.CommitStatus
		sha    string
	}{
		{domain.CommitInherited, "aaa"},
		{"", "bbb"},
	}
	out := make([]store.CycleCommitEntry, len(entries))
	for i, e := range entries {
		out[i] = store.CycleCommitEntry{SHA: e.sha, Status: e.status}
	}
	assignCommitAdmissionStatuses(out, "")
	if out[0].Status != domain.CommitEligible {
		t.Fatalf("inherited promoted: %+v", out[0])
	}
	if out[1].Status != domain.CommitEligible {
		t.Fatalf("default eligible: %+v", out[1])
	}
	assignCommitAdmissionStatuses(out, ExecuteUncommittedWorkReason)
	if out[0].Status != domain.CommitObserved || out[0].GateReason != ExecuteUncommittedWorkReason {
		t.Fatalf("observed on gate fail: %+v", out[0])
	}
}
