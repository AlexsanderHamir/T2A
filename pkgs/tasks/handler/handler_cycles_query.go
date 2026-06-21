package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// maxCycleListLimitParamBytes mirrors maxTaskEventSeqParamBytes — keep
// list-paging limit query strings short.
const maxCycleListLimitParamBytes = 32

// defaultCycleListLimit and maxCycleListLimit are the documented bounds for
// GET /tasks/{id}/cycles ?limit=. They follow the same 50/200 conventions
// used by GET /tasks and GET /tasks/{id}/events.
const (
	defaultCycleListLimit   = 50
	maxCycleListLimit       = 200
	defaultCycleStreamLimit = 100
	maxCycleStreamLimit     = 500
)

func parseCyclePathPair(r *http.Request) (string, string, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseCyclePathPair")
	taskID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		return "", "", err
	}
	cycleID, err := parseTaskPathCycleID(r.PathValue("cycleId"))
	if err != nil {
		return "", "", err
	}
	return taskID, cycleID, nil
}

// assertCycleBelongsToTask preflights write routes so a cycleId from a
// different task surfaces as 404 instead of mutating the wrong row. The
// store does not enforce this implicitly because cycleId is unique on its
// own, so the handler must check.
func assertCycleBelongsToTask(ctx context.Context, s *store.Store, taskID, cycleID string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.assertCycleBelongsToTask")
	c, err := s.GetCycle(ctx, cycleID)
	if err != nil {
		return err
	}
	if c.TaskID != taskID {
		return domain.ErrNotFound
	}
	return nil
}

// parseCycleListBeforeAttemptSeq parses the optional ?before_attempt_seq=
// keyset cursor for GET /tasks/{id}/cycles. Mirrors the validation used
// by ?before_seq= on /tasks/{id}/events: 32-byte abuse guard, must be a
// strictly positive int64. Returns 0 (no cursor / first page) when the
// param is absent or empty after trim.
func parseCycleListBeforeAttemptSeq(ctx context.Context, q url.Values) (before int64, err error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseCycleListBeforeAttemptSeq")
	ctx = calltrace.Push(ctx, "parseCycleListBeforeAttemptSeq")
	calltrace.HelperIOIn(ctx, "parseCycleListBeforeAttemptSeq", "before_q", q.Get("before_attempt_seq"))
	defer func() {
		calltrace.HelperIOOut(ctx, "parseCycleListBeforeAttemptSeq", "before_attempt_seq", before, "err", err)
	}()
	v := strings.TrimSpace(q.Get("before_attempt_seq"))
	if v == "" {
		return 0, nil
	}
	if len(v) > maxCycleListLimitParamBytes {
		return 0, fmt.Errorf("%w: before_attempt_seq too long", domain.ErrInvalidInput)
	}
	n, e := strconv.ParseInt(v, 10, 64)
	if e != nil || n < 1 {
		return 0, fmt.Errorf("%w: before_attempt_seq must be a positive integer", domain.ErrInvalidInput)
	}
	return n, nil
}

// parseCycleListLimit is the GET /tasks/{id}/cycles equivalent of
// parseTaskEventsLimit. Same 0..200 cap and 32-byte abuse guard.
func parseCycleListLimit(ctx context.Context, q url.Values) (int, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseCycleListLimit")
	ctx = calltrace.Push(ctx, "parseCycleListLimit")
	calltrace.HelperIOIn(ctx, "parseCycleListLimit", "limit_q", q.Get("limit"))
	var (
		limit = defaultCycleListLimit
		err   error
	)
	defer func() { calltrace.HelperIOOut(ctx, "parseCycleListLimit", "limit", limit, "err", err) }()
	v := strings.TrimSpace(q.Get("limit"))
	if v == "" {
		return limit, nil
	}
	if len(v) > maxCycleListLimitParamBytes {
		err = fmt.Errorf("%w: limit too long", domain.ErrInvalidInput)
		return 0, err
	}
	n, e := strconv.Atoi(v)
	if e != nil || n < 0 || n > maxCycleListLimit {
		err = fmt.Errorf("%w: limit must be integer 0..200", domain.ErrInvalidInput)
		return 0, err
	}
	if n == 0 {
		return defaultCycleListLimit, nil
	}
	limit = n
	return limit, nil
}

func parseCycleStreamAfterSeq(ctx context.Context, q url.Values) (after int64, err error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseCycleStreamAfterSeq")
	ctx = calltrace.Push(ctx, "parseCycleStreamAfterSeq")
	calltrace.HelperIOIn(ctx, "parseCycleStreamAfterSeq", "after_q", q.Get("after_seq"))
	defer func() { calltrace.HelperIOOut(ctx, "parseCycleStreamAfterSeq", "after_seq", after, "err", err) }()
	v := strings.TrimSpace(q.Get("after_seq"))
	if v == "" {
		return 0, nil
	}
	if len(v) > maxCycleListLimitParamBytes {
		return 0, fmt.Errorf("%w: after_seq too long", domain.ErrInvalidInput)
	}
	n, e := strconv.ParseInt(v, 10, 64)
	if e != nil || n < 1 {
		return 0, fmt.Errorf("%w: after_seq must be a positive integer", domain.ErrInvalidInput)
	}
	return n, nil
}

func parseCycleStreamLimit(ctx context.Context, q url.Values) (int, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseCycleStreamLimit")
	ctx = calltrace.Push(ctx, "parseCycleStreamLimit")
	calltrace.HelperIOIn(ctx, "parseCycleStreamLimit", "limit_q", q.Get("limit"))
	var (
		limit = defaultCycleStreamLimit
		err   error
	)
	defer func() { calltrace.HelperIOOut(ctx, "parseCycleStreamLimit", "limit", limit, "err", err) }()
	v := strings.TrimSpace(q.Get("limit"))
	if v == "" {
		return limit, nil
	}
	if len(v) > maxCycleListLimitParamBytes {
		err = fmt.Errorf("%w: limit too long", domain.ErrInvalidInput)
		return 0, err
	}
	n, e := strconv.Atoi(v)
	if e != nil || n < 0 || n > maxCycleStreamLimit {
		err = fmt.Errorf("%w: limit must be integer 0..500", domain.ErrInvalidInput)
		return 0, err
	}
	if n == 0 {
		return defaultCycleStreamLimit, nil
	}
	limit = n
	return limit, nil
}
