package main

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/Hamix/internal/taskapi/agentworker"
	"github.com/AlexsanderHamir/Hamix/internal/taskapiconfig"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// run_agentworker.go wires the bounded ready-task queue, reconcile loop,
// and agent worker supervisor boot. Supervisor lifecycle lives in
// internal/taskapi/agentworker.
func startReadyTaskAgents(ctx context.Context, taskStore *store.Store, hub *handler.SSEHub) (context.CancelFunc, *agents.MemoryQueue, *agentworker.Supervisor, error) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.startReadyTaskAgents")
	qcap := taskapiconfig.UserTaskAgentQueueCap()
	agentQueue := agents.NewMemoryQueue(qcap)
	taskStore.SetReadyTaskNotifier(agentQueue)
	pickupWake := agents.NewPickupWakeScheduler(taskStore, agentQueue)
	taskStore.SetPickupWake(pickupWake)
	if err := pickupWake.Hydrate(ctx); err != nil {
		return nil, nil, nil, err
	}
	iv := agents.ReconcileTickInterval
	slog.Info("ready task agent queue", "cmd", cmdName, "operation", "taskapi.agent_queue", "cap", qcap)
	slog.Info("ready task agent reconcile", "cmd", cmdName, "operation", "taskapi.agent_reconcile",
		"tick_interval", iv.String())

	reconcileCtx, reconcileCancel := context.WithCancel(ctx)
	go agents.RunReconcileLoop(reconcileCtx, taskStore, agentQueue, iv, nil)

	sup := agentworker.New(ctx, taskStore, agentQueue, hub)
	if err := sup.Start(ctx); err != nil {
		pickupWake.Stop()
		reconcileCancel()
		return nil, nil, nil, err
	}
	stopAgents := func() {
		pickupWake.Stop()
		reconcileCancel()
	}
	return stopAgents, agentQueue, sup, nil
}
