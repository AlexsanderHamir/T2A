package devsim

import (
	"context"
	"log/slog"
	"math/rand/v2"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/google/uuid"
)

const devsimTaskIDPrefix = "t2a-devsim-"

// RunLifecycleOnce either creates a prefixed dev task or deletes one (no children), then calls publish.
func RunLifecycleOnce(ctx context.Context, st *store.Store, publish func(ChangeKind, string)) {
	if st == nil || publish == nil {
		return
	}
	if rand.IntN(2) == 0 {
		tryCreateDevsimTask(ctx, st, publish)
		return
	}
	tryDeleteDevsimTask(ctx, st, publish)
}

func tryCreateDevsimTask(ctx context.Context, st *store.Store, publish func(ChangeKind, string)) {
	id := devsimTaskIDPrefix + uuid.NewString()
	t, err := st.Create(ctx, store.CreateTaskInput{
		ID:            id,
		Title:         "Dev sim task",
		InitialPrompt: "<p>Synthetic task for UI / SSE exercise.</p>",
		Status:        domain.StatusReady,
		Priority:      domain.PriorityMedium,
	}, domain.ActorAgent)
	if err != nil {
		slog.Debug("sse dev lifecycle create skipped", "cmd", logCmd, "operation", "devsim.lifecycle_create", "err", err)
		return
	}
	publish(ChangeCreated, t.ID)
}

func tryDeleteDevsimTask(ctx context.Context, st *store.Store, publish func(ChangeKind, string)) {
	tasks, err := st.ListDevsimTasks(ctx, devsimTaskIDPrefix+"%")
	if err != nil {
		slog.Debug("sse dev lifecycle list skipped", "cmd", logCmd, "operation", "devsim.lifecycle_list", "err", err)
		return
	}
	if len(tasks) == 0 {
		return
	}
	for _, i := range rand.Perm(len(tasks)) {
		id := tasks[i].ID
		if err := st.Delete(ctx, id); err != nil {
			continue
		}
		publish(ChangeDeleted, id)
		return
	}
}
