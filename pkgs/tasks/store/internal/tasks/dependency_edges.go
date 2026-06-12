package tasks

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func normalizeDependencyEdges(taskID string, edges []domain.DependencyEdge) ([]domain.DependencyEdge, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.normalizeDependencyEdges")
	out := make([]domain.DependencyEdge, 0, len(edges))
	seen := make(map[string]struct{})
	for _, e := range edges {
		id := strings.TrimSpace(e.TaskID)
		if id == "" {
			continue
		}
		if id == taskID {
			return nil, fmt.Errorf("%w: task cannot depend on itself", domain.ErrInvalidInput)
		}
		satisfies := domain.NormalizeDependencySatisfies(e.Satisfies)
		if !domain.ValidDependencySatisfies(satisfies) {
			return nil, fmt.Errorf("%w: invalid dependency satisfies %q", domain.ErrInvalidInput, e.Satisfies)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, domain.DependencyEdge{TaskID: id, Satisfies: satisfies})
	}
	return out, nil
}

// DependencyEdgeIDs returns predecessor task ids in edge order.
func DependencyEdgeIDs(edges []domain.DependencyEdge) []string {
	ids := make([]string, 0, len(edges))
	for _, e := range edges {
		ids = append(ids, e.TaskID)
	}
	return ids
}

// EdgeSatisfied reports whether predecessor meets the edge predicate.
func EdgeSatisfied(predecessor *domain.Task, satisfies domain.DependencySatisfies) bool {
	if predecessor == nil {
		return false
	}
	switch domain.NormalizeDependencySatisfies(satisfies) {
	case domain.DependencySatisfiesCriteriaComplete:
		return predecessor.CriteriaSatisfiedAt != nil
	default:
		return predecessor.Status == domain.StatusDone
	}
}
