package main

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/Hamix/internal/taskapiconfig"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func maybeRunGitReconcileOnStartup(ctx context.Context, taskStore *store.Store) {
	mode := taskapiconfig.GitReconcileOnStartupMode()
	if mode == "" {
		return
	}
	slog.Info("git startup reconcile enabled", "cmd", cmdName, "operation", "taskapi.git_startup_reconcile", "mode", mode)
	taskStore.ReconcileGitRepositoriesOnStartup(ctx, nil)
}
