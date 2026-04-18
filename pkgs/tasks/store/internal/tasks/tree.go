package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

// ListFlat returns tasks ordered by id ASC with limit/offset over all
// rows; no tree shape is built. limit is clamped to [1, 200] (default
// 50) and offset to [0, +inf).
func ListFlat(ctx context.Context, db *gorm.DB, limit, offset int) ([]domain.Task, error) {
	defer kernel.DeferLatency(kernel.OpListFlat)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.ListFlat")
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
	err := db.WithContext(ctx).
		Order("id ASC").
		Limit(limit).
		Offset(offset).
		Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return out, nil
}

// ListRootForest pages root tasks (parent_id IS NULL) ordered by id
// ASC and attaches each full descendant subtree. limit+1 rows are
// fetched so hasMore can be reported without a separate count.
func ListRootForest(ctx context.Context, db *gorm.DB, limit, offset int) (nodes []Node, hasMore bool, err error) {
	defer kernel.DeferLatency(kernel.OpListRootForest)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.ListRootForest")
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
	err = db.WithContext(ctx).
		Where("parent_id IS NULL").
		Order("id ASC").
		Limit(limit + 1).
		Offset(offset).
		Find(&roots).Error
	if err != nil {
		return nil, false, fmt.Errorf("list root tasks: %w", err)
	}
	hasMore = len(roots) > limit
	if hasMore {
		roots = roots[:limit]
	}
	nodes, err = rootsToForest(ctx, db, roots)
	if err != nil {
		return nil, false, err
	}
	return nodes, hasMore, nil
}

// ListRootForestAfter is the keyset variant of ListRootForest: it
// returns root tasks with id strictly greater than afterID (same
// ordering and tree shape).
func ListRootForestAfter(ctx context.Context, db *gorm.DB, limit int, afterID string) (nodes []Node, hasMore bool, err error) {
	defer kernel.DeferLatency(kernel.OpListRootForestAfter)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.ListRootForestAfter")
	afterID = strings.TrimSpace(afterID)
	if afterID == "" {
		return nil, false, fmt.Errorf("%w: after_id", domain.ErrInvalidInput)
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var roots []domain.Task
	err = db.WithContext(ctx).
		Where("parent_id IS NULL AND id > ?", afterID).
		Order("id ASC").
		Limit(limit + 1).
		Find(&roots).Error
	if err != nil {
		return nil, false, fmt.Errorf("list root tasks after id: %w", err)
	}
	hasMore = len(roots) > limit
	if hasMore {
		roots = roots[:limit]
	}
	nodes, err = rootsToForest(ctx, db, roots)
	if err != nil {
		return nil, false, err
	}
	return nodes, hasMore, nil
}

// GetTree returns one task and every descendant nested under it.
func GetTree(ctx context.Context, db *gorm.DB, id string) (Node, error) {
	defer kernel.DeferLatency(kernel.OpGetTaskTree)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.GetTree")
	id = strings.TrimSpace(id)
	if id == "" {
		return Node{}, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var root domain.Task
	err := db.WithContext(ctx).Where("id = ?", id).First(&root).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return Node{}, domain.ErrNotFound
		}
		return Node{}, fmt.Errorf("get task: %w", err)
	}
	all, err := loadTasksForForest(ctx, db, []domain.Task{root})
	if err != nil {
		return Node{}, err
	}
	nodes, err := buildForest([]domain.Task{root}, all)
	if err != nil {
		return Node{}, err
	}
	if len(nodes) != 1 {
		return Node{}, fmt.Errorf("get task tree: %w", domain.ErrNotFound)
	}
	return nodes[0], nil
}

func rootsToForest(ctx context.Context, db *gorm.DB, roots []domain.Task) ([]Node, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.rootsToForest")
	if len(roots) == 0 {
		return []Node{}, nil
	}
	all, err := loadTasksForForest(ctx, db, roots)
	if err != nil {
		return nil, err
	}
	return buildForest(roots, all)
}

func loadTasksForForest(ctx context.Context, db *gorm.DB, seeds []domain.Task) (map[string]domain.Task, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.loadTasksForForest")
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
		if err := db.WithContext(ctx).Where("parent_id IN ?", queue).Find(&batch).Error; err != nil {
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

func buildForest(roots []domain.Task, byID map[string]domain.Task) ([]Node, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.buildForest")
	childrenOf := make(map[string][]domain.Task)
	for _, t := range byID {
		if t.ParentID == nil || *t.ParentID == "" {
			continue
		}
		p := *t.ParentID
		childrenOf[p] = append(childrenOf[p], t)
	}
	for p, slice := range childrenOf {
		for i := 0; i < len(slice); i++ {
			for j := i + 1; j < len(slice); j++ {
				if slice[j].ID < slice[i].ID {
					slice[i], slice[j] = slice[j], slice[i]
				}
			}
		}
		childrenOf[p] = slice
	}
	out := make([]Node, 0, len(roots))
	for _, r := range roots {
		n, err := buildNode(r, childrenOf, 0)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func buildNode(t domain.Task, childrenOf map[string][]domain.Task, depth int) (Node, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.buildNode")
	if depth > MaxTreeDepth {
		return Node{}, fmt.Errorf("%w: task tree exceeds maximum depth", domain.ErrInvalidInput)
	}
	kids := childrenOf[t.ID]
	ch := make([]Node, 0, len(kids))
	for _, c := range kids {
		n, err := buildNode(c, childrenOf, depth+1)
		if err != nil {
			return Node{}, err
		}
		ch = append(ch, n)
	}
	return Node{Task: t, Children: ch}, nil
}
