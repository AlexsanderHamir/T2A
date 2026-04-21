package agents

import (
	"container/heap"
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// PickupWakeScheduler implements store.PickupWake: a min-heap of
// (pickup_not_before, task_id) with one timer for the earliest deadline.
// On fire it loads the task and enqueues when ShouldNotifyReadyNow holds.
type PickupWakeScheduler struct {
	st *store.Store
	q  *MemoryQueue

	mu      sync.Mutex
	byID    map[string]*wakeItem
	heap    wakeHeap
	timer   *time.Timer
	stopped bool
}

type wakeItem struct {
	taskID    string
	notBefore time.Time
	index     int
}

type wakeHeap []*wakeItem

func (h wakeHeap) Len() int { return len(h) }

func (h wakeHeap) Less(i, j int) bool {
	a, b := h[i].notBefore, h[j].notBefore
	if !a.Equal(b) {
		return a.Before(b)
	}
	return h[i].taskID < h[j].taskID
}

func (h wakeHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *wakeHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*wakeItem)
	item.index = n
	*h = append(*h, item)
}

func (h *wakeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[0 : n-1]
	return item
}

// NewPickupWakeScheduler returns a scheduler backed by st and q. The
// caller must register it with (*store.Store).SetPickupWake and call
// Hydrate once at startup.
func NewPickupWakeScheduler(st *store.Store, q *MemoryQueue) *PickupWakeScheduler {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.NewPickupWakeScheduler")
	return &PickupWakeScheduler{
		st:   st,
		q:    q,
		byID: make(map[string]*wakeItem),
	}
}

// Hydrate schedules wake timers for every ready task with pickup_not_before
// in the future (bounded list). Safe to call once after SetPickupWake.
func (w *PickupWakeScheduler) Hydrate(ctx context.Context) error {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.PickupWakeScheduler.Hydrate")
	if w == nil || w.st == nil {
		return nil
	}
	rows, err := w.st.ListDeferredReadyPickupTasks(ctx, 10_000)
	if err != nil {
		return err
	}
	for i := range rows {
		r := rows[i]
		w.Schedule(ctx, r.ID, r.PickupNotBefore)
	}
	return nil
}

// Schedule implements store.PickupWake.
func (w *PickupWakeScheduler) Schedule(ctx context.Context, taskID string, notBefore time.Time) {
	if w == nil || taskID == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped {
		return
	}
	if it, ok := w.byID[taskID]; ok {
		heap.Remove(&w.heap, it.index)
		delete(w.byID, taskID)
	}
	nb := notBefore.UTC()
	item := &wakeItem{taskID: taskID, notBefore: nb}
	heap.Push(&w.heap, item)
	w.byID[taskID] = item
	w.resetTimerLocked()
}

// Cancel implements store.PickupWake.
func (w *PickupWakeScheduler) Cancel(taskID string) {
	if w == nil || taskID == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped {
		return
	}
	it, ok := w.byID[taskID]
	if !ok {
		return
	}
	heap.Remove(&w.heap, it.index)
	delete(w.byID, taskID)
	w.resetTimerLocked()
}

// Stop implements store.PickupWake.
func (w *PickupWakeScheduler) Stop() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stopped = true
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	w.byID = nil
	w.heap = nil
}

func (w *PickupWakeScheduler) resetTimerLocked() {
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	if len(w.heap) == 0 {
		return
	}
	next := w.heap[0]
	d := time.Until(next.notBefore)
	if d < 0 {
		d = 0
	}
	w.timer = time.AfterFunc(d, w.fire)
}

func (w *PickupWakeScheduler) fire() {
	w.mu.Lock()
	if w.stopped {
		w.mu.Unlock()
		return
	}
	now := time.Now().UTC()
	for len(w.heap) > 0 {
		peek := w.heap[0]
		if peek.notBefore.After(now) {
			break
		}
		item := heap.Pop(&w.heap).(*wakeItem)
		delete(w.byID, item.taskID)
		tid := item.taskID
		w.mu.Unlock()
		w.tryNotify(tid, now)
		w.mu.Lock()
		if w.stopped {
			w.mu.Unlock()
			return
		}
		now = time.Now().UTC()
	}
	w.resetTimerLocked()
	w.mu.Unlock()
}

func (w *PickupWakeScheduler) tryNotify(taskID string, now time.Time) {
	if w.st == nil || w.q == nil {
		return
	}
	ctx := context.Background()
	t, err := w.st.Get(ctx, taskID)
	if err != nil || t == nil || t.Status != domain.StatusReady {
		return
	}
	if !store.ShouldNotifyReadyNow(t.PickupNotBefore, now) {
		return
	}
	_ = w.q.NotifyReadyTask(ctx, *t)
}
