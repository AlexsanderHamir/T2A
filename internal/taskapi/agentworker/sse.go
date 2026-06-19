package agentworker

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
)

const (
	agentRunProgressMinInterval     = 750 * time.Millisecond
	agentRunProgressThrottleEntries = 512
)

type cycleChangeSSEAdapter struct {
	hub *handler.SSEHub
}

func newCycleChangeSSEAdapter(hub *handler.SSEHub) *cycleChangeSSEAdapter {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.newCycleChangeSSEAdapter")
	return &cycleChangeSSEAdapter{hub: hub}
}

func (a *cycleChangeSSEAdapter) PublishCycleChange(taskID, cycleID string) {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.cycleChangeSSEAdapter.PublishCycleChange",
		"task_id", taskID, "cycle_id", cycleID)
	if a == nil || a.hub == nil || taskID == "" {
		return
	}
	a.hub.Publish(handler.TaskChangeEvent{
		Type:    handler.TaskCycleChanged,
		ID:      taskID,
		CycleID: cycleID,
	})
}

type runProgressSSEAdapter struct {
	hub         *handler.SSEHub
	minInterval time.Duration

	mu       sync.Mutex
	lastSent map[string]time.Time
}

func newRunProgressSSEAdapter(hub *handler.SSEHub, minInterval time.Duration) *runProgressSSEAdapter {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.newRunProgressSSEAdapter")
	return &runProgressSSEAdapter{
		hub:         hub,
		minInterval: minInterval,
		lastSent:    make(map[string]time.Time),
	}
}

func (a *runProgressSSEAdapter) PublishRunProgress(taskID, cycleID string, phaseSeq int64, ev runner.ProgressEvent) {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.runProgressSSEAdapter.PublishRunProgress",
		"task_id", taskID, "cycle_id", cycleID, "phase_seq", phaseSeq,
		"kind", ev.Kind, "subtype", ev.Subtype)
	if a == nil || a.hub == nil || taskID == "" || cycleID == "" || phaseSeq <= 0 || ev.Kind == "" {
		return
	}
	if a.shouldDrop(taskID, cycleID, phaseSeq) {
		return
	}
	a.hub.Publish(handler.TaskChangeEvent{
		Type:     handler.AgentRunProgress,
		ID:       taskID,
		CycleID:  cycleID,
		PhaseSeq: phaseSeq,
		Progress: &handler.AgentRunProgressPayload{
			Kind:    ev.Kind,
			Subtype: ev.Subtype,
			Message: ev.Message,
			Tool:    ev.Tool,
		},
	})
}

func (a *runProgressSSEAdapter) shouldDrop(taskID, cycleID string, phaseSeq int64) bool {
	if a.minInterval <= 0 {
		return false
	}
	key := fmt.Sprintf("%s:%s:%d", taskID, cycleID, phaseSeq)
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	last, ok := a.lastSent[key]
	if ok && now.Sub(last) < a.minInterval {
		return true
	}
	a.lastSent[key] = now
	if len(a.lastSent) > agentRunProgressThrottleEntries {
		for old := range a.lastSent {
			if old != key {
				delete(a.lastSent, old)
				break
			}
		}
	}
	return false
}
