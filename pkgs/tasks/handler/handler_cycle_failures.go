package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func (h *Handler) cycleFailures(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.cycleFailures")
	const op = "tasks.cycle_failures"
	r = calltrace.WithRequestRoot(r, op)
	limit, offset, sort, err := parseCycleFailuresQuery(r.URL.Query())
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "limit", limit, "offset", offset, "sort", sort)
	out, err := h.store.ListCycleFailures(r.Context(), store.ListCycleFailuresInput{
		Limit:  limit,
		Offset: offset,
		Sort:   sort,
	})
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, cycleFailuresResponse{
		Total:               out.Total,
		Limit:               limit,
		Offset:              offset,
		Sort:                sort,
		ReasonSortTruncated: out.ReasonSortTruncated,
		Failures:            recentFailuresToJSON(out.Failures),
	})
}

func parseCycleFailuresQuery(q url.Values) (limit, offset int, sort string, err error) {
	limit = 50
	offset = 0
	sort = store.CycleFailureSortAtDesc
	if v := q.Get("limit"); v != "" {
		if len(v) > maxListIntQueryParamBytes {
			return 0, 0, "", fmt.Errorf("%w: limit value too long", domain.ErrInvalidInput)
		}
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 || n > 200 {
			return 0, 0, "", fmt.Errorf("%w: limit must be integer 1..200", domain.ErrInvalidInput)
		}
		limit = n
	}
	if v := q.Get("offset"); v != "" {
		if len(v) > maxListIntQueryParamBytes {
			return 0, 0, "", fmt.Errorf("%w: offset value too long", domain.ErrInvalidInput)
		}
		n, e := strconv.Atoi(v)
		if e != nil || n < 0 {
			return 0, 0, "", fmt.Errorf("%w: offset must be non-negative integer", domain.ErrInvalidInput)
		}
		offset = n
	}
	if v := strings.TrimSpace(q.Get("sort")); v != "" {
		sort = v
	}
	return limit, offset, sort, nil
}
