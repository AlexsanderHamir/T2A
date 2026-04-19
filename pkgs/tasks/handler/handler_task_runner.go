package handler

import (
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/registry"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// resolveTaskRunnerModel merges optional JSON fields with app settings and
// validates the runner id against the registry.
func resolveTaskRunnerModel(body *taskCreateJSON, settings domain.AppSettings) (runner, cursorModel string, err error) {
	if body.Runner != nil && strings.TrimSpace(*body.Runner) != "" {
		runner = strings.TrimSpace(*body.Runner)
	} else {
		runner = strings.TrimSpace(settings.Runner)
	}
	if runner == "" {
		runner = domain.DefaultRunner
	}
	if _, lerr := registry.Lookup(runner); lerr != nil {
		return "", "", fmt.Errorf("%w: runner", domain.ErrInvalidInput)
	}
	if body.CursorModel != nil {
		cursorModel = strings.TrimSpace(*body.CursorModel)
	} else {
		cursorModel = strings.TrimSpace(settings.CursorModel)
	}
	return runner, cursorModel, nil
}
