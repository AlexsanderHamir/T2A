package harness

import (
	"encoding/json"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func effectiveModelFromCycleMeta(r runner.Runner, task *domain.Task, cycle *domain.TaskCycle) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.effectiveModelFromCycleMeta",
		"task_id", task.ID, "cycle_id", cycleIDOrEmpty(cycle))
	if cycle != nil && len(cycle.MetaJSON) > 0 {
		var meta map[string]any
		if json.Unmarshal(cycle.MetaJSON, &meta) == nil {
			if v, ok := meta["cursor_model_effective"].(string); ok && v != "" {
				return v
			}
		}
	}
	req := runner.Request{
		TaskID:      task.ID,
		Prompt:      task.InitialPrompt,
		CursorModel: task.CursorModel,
	}
	if ml, ok := r.(runner.MetricsLabeler); ok {
		return ml.MetricsLabels(req)["model"]
	}
	return r.EffectiveModel(req)
}
