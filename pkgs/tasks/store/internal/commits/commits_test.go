package commits

import (
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func TestDedupeCommitsBySHA_keepsFirstOccurrence(t *testing.T) {
	t.Parallel()
	when := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	got := dedupeCommitsBySHA([]domain.TaskCycleCommit{
		{SHA: "aaa", Message: "first", CommittedAt: when, Seq: 1},
		{SHA: "bbb", Message: "other", CommittedAt: when, Seq: 2},
		{SHA: "aaa", Message: "later", CommittedAt: when.Add(2 * time.Minute), Seq: 3},
	})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].SHA != "aaa" || got[0].Message != "first" {
		t.Fatalf("first row = %+v", got[0])
	}
}
