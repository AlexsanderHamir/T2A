package store

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// ListFlat returns tasks ordered by id with limit/offset over all rows (no tree).
func (s *Store) ListFlat(ctx context.Context, limit, offset int) ([]domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListFlat")
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	var out []domain.Task
	err := s.db.WithContext(ctx).
		Order("id ASC").
		Limit(limit).
		Offset(offset).
		Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return out, nil
}

// List is an alias for ListFlat (all tasks, id ASC, limit/offset). Prefer ListFlat in new code.
func (s *Store) List(ctx context.Context, limit, offset int) ([]domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.List")
	return s.ListFlat(ctx, limit, offset)
}

// ListRootForest pages root tasks (parent_id IS NULL) and attaches each full descendant subtree.
func (s *Store) ListRootForest(ctx context.Context, limit, offset int) ([]TaskNode, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListRootForest")
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	var roots []domain.Task
	err := s.db.WithContext(ctx).
		Where("parent_id IS NULL").
		Order("id ASC").
		Limit(limit).
		Offset(offset).
		Find(&roots).Error
	if err != nil {
		return nil, fmt.Errorf("list root tasks: %w", err)
	}
	if len(roots) == 0 {
		return []TaskNode{}, nil
	}
	all, err := s.loadTasksForForest(ctx, roots)
	if err != nil {
		return nil, err
	}
	return buildForest(roots, all), nil
}

// GetTaskTree returns one task and every descendant nested under it.
func (s *Store) GetTaskTree(ctx context.Context, id string) (TaskNode, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.GetTaskTree")
	id = strings.TrimSpace(id)
	if id == "" {
		return TaskNode{}, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var root domain.Task
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&root).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return TaskNode{}, domain.ErrNotFound
		}
		return TaskNode{}, fmt.Errorf("get task: %w", err)
	}
	all, err := s.loadTasksForForest(ctx, []domain.Task{root})
	if err != nil {
		return TaskNode{}, err
	}
	nodes := buildForest([]domain.Task{root}, all)
	if len(nodes) != 1 {
		return TaskNode{}, fmt.Errorf("get task tree: %w", domain.ErrNotFound)
	}
	return nodes[0], nil
}

func (s *Store) loadTasksForForest(ctx context.Context, seeds []domain.Task) (map[string]domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.loadTasksForForest")
	all := make(map[string]domain.Task)
	for _, t := range seeds {
		all[t.ID] = t
	}
	queue := make([]string, 0, len(seeds))
	for _, t := range seeds {
		queue = append(queue, t.ID)
	}
	for len(queue) > 0 {
		var batch []domain.Task
		if err := s.db.WithContext(ctx).Where("parent_id IN ?", queue).Find(&batch).Error; err != nil {
			return nil, fmt.Errorf("list child tasks: %w", err)
		}
		queue = queue[:0]
		for _, t := range batch {
			if _, ok := all[t.ID]; ok {
				continue
			}
			all[t.ID] = t
			queue = append(queue, t.ID)
		}
	}
	return all, nil
}

func buildForest(roots []domain.Task, byID map[string]domain.Task) []TaskNode {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.buildForest")
	childrenOf := make(map[string][]domain.Task)
	for _, t := range byID {
		if t.ParentID == nil || *t.ParentID == "" {
			continue
		}
		p := *t.ParentID
		childrenOf[p] = append(childrenOf[p], t)
	}
	for p, slice := range childrenOf {
		// sort by id ascending (insertion order from map is random)
		for i := 0; i < len(slice); i++ {
			for j := i + 1; j < len(slice); j++ {
				if slice[j].ID < slice[i].ID {
					slice[i], slice[j] = slice[j], slice[i]
				}
			}
		}
		childrenOf[p] = slice
	}
	out := make([]TaskNode, 0, len(roots))
	for _, r := range roots {
		out = append(out, buildNode(r, childrenOf))
	}
	return out
}

func buildNode(t domain.Task, childrenOf map[string][]domain.Task) TaskNode {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.buildNode")
	kids := childrenOf[t.ID]
	ch := make([]TaskNode, 0, len(kids))
	for _, c := range kids {
		ch = append(ch, buildNode(c, childrenOf))
	}
	return TaskNode{Task: t, Children: ch}
}
