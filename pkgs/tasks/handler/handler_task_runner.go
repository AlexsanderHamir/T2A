package handler

import (
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/registry"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// resolveTaskRunnerModel merges optional JSON fields with app settings and
// validates the runner id against the registry.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func resolveTaskRunnerModel(body *taskCreateJSON, settings domain.AppSettings) (runner, cursorModel string, err error) {
	return resolveRunnerModelFields(body.Runner, body.CursorModel, settings)
}

func resolveRunnerModelFields(runnerPtr, cursorModelPtr *string, settings domain.AppSettings) (runner, cursorModel string, err error) {
	if runnerPtr != nil && strings.TrimSpace(*runnerPtr) != "" {
		runner = strings.TrimSpace(*runnerPtr)
	} else {
		runner = strings.TrimSpace(settings.Runner)
	}
	if runner == "" {
		runner = domain.DefaultRunner
	}
	if _, lerr := registry.Lookup(runner); lerr != nil {
		return "", "", fmt.Errorf("%w: runner", domain.ErrInvalidInput)
	}
	if cursorModelPtr != nil {
		cursorModel = strings.TrimSpace(*cursorModelPtr)
	} else {
		cursorModel = strings.TrimSpace(settings.CursorModel)
	}
	return runner, cursorModel, nil
}
