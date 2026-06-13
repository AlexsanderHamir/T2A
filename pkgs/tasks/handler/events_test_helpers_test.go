package handler

import (
	"context"
	"strconv"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// appendApprovalRequestedEvent appends approval_requested and returns its seq.
// Use after POST /tasks with checklist_items so event numbering stays stable.
func appendApprovalRequestedEvent(t *testing.T, st *store.Store, ctx context.Context, taskID string) int64 {
	t.Helper()
	if err := st.AppendTaskEvent(ctx, taskID, domain.EventApprovalRequested, domain.ActorAgent, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	seq, err := st.LastEventSeq(ctx, taskID)
	if err != nil {
		t.Fatal(err)
	}
	return seq
}

func formatEventSeq(seq int64) string {
	return strconv.FormatInt(seq, 10)
}
