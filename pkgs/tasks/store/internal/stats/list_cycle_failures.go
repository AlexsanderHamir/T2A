package stats

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// Cycle failure list sort modes for GET /tasks/cycle-failures.
const (
	CycleFailureSortAtDesc       = "at_desc"
	CycleFailureSortAtAsc        = "at_asc"
	CycleFailureSortReasonAsc    = "reason_asc"
	CycleFailureSortReasonDesc   = "reason_desc"
	defaultCycleFailureListLimit = 50
	maxCycleFailureListLimit     = 200
	// reasonSortFetchCap bounds how many newest cycle_failed rows we load
	// when sorting by reason (enrichment + sort happen in memory).
	reasonSortFetchCap = 2000
)

// ListCycleFailuresInput is the paginated / sorted query for the
// dedicated cycle failures view.
type ListCycleFailuresInput struct {
	Limit  int
	Offset int
	Sort   string
}

// ListCycleFailuresResult is returned by ListCycleFailures.
type ListCycleFailuresResult struct {
	Total int64
	// Failures is the current page (may be shorter than Limit after
	// decode skips malformed rows).
	Failures []RecentFailure
	// ReasonSortTruncated is true when sort is reason_* and the database
	// has more cycle_failed rows than reasonSortFetchCap, so only the
	// newest cap rows participated in the sort.
	ReasonSortTruncated bool
}

// ListCycleFailures returns cycle_failed mirror rows with the same
// enrichment as /tasks/stats recent_failures. Time-based sorts use SQL
// pagination; reason sorts load up to reasonSortFetchCap newest rows,
// enrich, sort in memory, then slice for offset/limit.
func ListCycleFailures(ctx context.Context, db *gorm.DB, in ListCycleFailuresInput) (ListCycleFailuresResult, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.stats.ListCycleFailures",
		"limit", in.Limit, "offset", in.Offset, "sort", in.Sort)
	limit := in.Limit
	if limit <= 0 {
		limit = defaultCycleFailureListLimit
	}
	if limit > maxCycleFailureListLimit {
		limit = maxCycleFailureListLimit
	}
	offset := in.Offset
	if offset < 0 {
		offset = 0
	}
	sortKey := strings.TrimSpace(in.Sort)
	if sortKey == "" {
		sortKey = CycleFailureSortAtDesc
	}
	switch sortKey {
	case CycleFailureSortAtDesc, CycleFailureSortAtAsc, CycleFailureSortReasonAsc, CycleFailureSortReasonDesc:
	default:
		return ListCycleFailuresResult{}, fmt.Errorf("%w: invalid sort", domain.ErrInvalidInput)
	}

	var total int64
	if err := db.WithContext(ctx).Model(&domain.TaskEvent{}).
		Where("type = ?", string(domain.EventCycleFailed)).
		Count(&total).Error; err != nil {
		return ListCycleFailuresResult{}, fmt.Errorf("count cycle failures: %w", err)
	}

	switch sortKey {
	case CycleFailureSortAtDesc, CycleFailureSortAtAsc:
		var rows []cycleFailedRow
		q := db.WithContext(ctx).Model(&domain.TaskEvent{}).
			Select("task_id, seq, at, data_json").
			Where("type = ?", string(domain.EventCycleFailed))
		if sortKey == CycleFailureSortAtDesc {
			q = q.Order("at DESC, seq DESC")
		} else {
			q = q.Order("at ASC, seq ASC")
		}
		if err := q.Limit(limit).Offset(offset).Scan(&rows).Error; err != nil {
			return ListCycleFailuresResult{}, fmt.Errorf("list cycle failures: %w", err)
		}
		failures := decodeCycleFailedRows(rows)
		enrichRecentFailuresFromPhaseEvents(ctx, db, failures)
		return ListCycleFailuresResult{Total: total, Failures: failures}, nil

	case CycleFailureSortReasonAsc, CycleFailureSortReasonDesc:
		var rows []cycleFailedRow
		if err := db.WithContext(ctx).Model(&domain.TaskEvent{}).
			Select("task_id, seq, at, data_json").
			Where("type = ?", string(domain.EventCycleFailed)).
			Order("at DESC, seq DESC").
			Limit(reasonSortFetchCap).
			Scan(&rows).Error; err != nil {
			return ListCycleFailuresResult{}, fmt.Errorf("list cycle failures for reason sort: %w", err)
		}
		failures := decodeCycleFailedRows(rows)
		enrichRecentFailuresFromPhaseEvents(ctx, db, failures)
		if sortKey == CycleFailureSortReasonAsc {
			sort.SliceStable(failures, func(i, j int) bool {
				return strings.ToLower(failures[i].Reason) < strings.ToLower(failures[j].Reason)
			})
		} else {
			sort.SliceStable(failures, func(i, j int) bool {
				return strings.ToLower(failures[i].Reason) > strings.ToLower(failures[j].Reason)
			})
		}
		truncated := total > int64(reasonSortFetchCap)
		if offset >= len(failures) {
			return ListCycleFailuresResult{
				Total:               total,
				Failures:            nil,
				ReasonSortTruncated: truncated,
			}, nil
		}
		end := offset + limit
		if end > len(failures) {
			end = len(failures)
		}
		page := failures[offset:end]
		return ListCycleFailuresResult{
			Total:               total,
			Failures:            page,
			ReasonSortTruncated: truncated,
		}, nil

	default:
		return ListCycleFailuresResult{}, fmt.Errorf("%w: invalid sort", domain.ErrInvalidInput)
	}
}
