package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/apijson"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
)

const (
	maxHTTPLogQueryBytes       = 1024
	maxHTTPLogJSONPreviewBytes = 16384
	maxHTTPLogTitleRunes       = 160
	maxHTTPLogPromptRunes      = 400
	maxHTTPLogTextRunes        = 240
)

// debugHTTPRequest logs structured request context (method, path, query, headers safe for logs).
// Skips work when Debug is disabled for ctx.
func debugHTTPRequest(r *http.Request, op string, extra ...any) {
	if r == nil || !slog.Default().Enabled(r.Context(), slog.LevelDebug) {
		return
	}
	q := r.URL.RawQuery
	if len(q) > maxHTTPLogQueryBytes {
		q = apijson.TruncateUTF8ByBytes(q, maxHTTPLogQueryBytes)
	}
	args := []any{
		"cmd", calltrace.LogCmd,
		"obs_category", "http_io",
		"operation", op,
		"call_path", calltrace.Path(r.Context()),
		"phase", "in",
		"method", r.Method,
		"path", r.URL.Path,
		"route_pattern", r.Pattern,
		"query", q,
		"content_length", r.ContentLength,
		"x_actor", strings.TrimSpace(r.Header.Get("X-Actor")),
	}
	args = append(args, extra...)
	slog.Log(r.Context(), slog.LevelDebug, "http.io", args...)
}

// debugHTTPOut logs a non-JSON outcome (e.g. 204) at Debug.
func debugHTTPOut(ctx context.Context, op string, httpStatus int, extra ...any) {
	if ctx == nil || !slog.Default().Enabled(ctx, slog.LevelDebug) {
		return
	}
	args := []any{
		"cmd", calltrace.LogCmd,
		"obs_category", "http_io",
		"operation", op,
		"call_path", calltrace.Path(ctx),
		"phase", "out",
		"http_status", httpStatus,
	}
	args = append(args, extra...)
	slog.Log(ctx, slog.LevelDebug, "http.io", args...)
}

// truncateRunes is a pure helper called only from taskCreateInputFields /
// taskPatchInputFields, which themselves only run inside the
// debugHTTPRequest gate. Skip-listed in cmd/funclogmeasure/analyze.go
// rather than logging per-call (would emit per-trace-line per truncation).
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= maxRunes {
			b.WriteString("…")
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// taskCreateInputFields builds the body_* slog attribute slice for the
// debugHTTPRequest http.io trace. Pure transformation; the trace itself
// logs and is gated by Enabled() upstream so this helper never runs when
// debug is off. Skip-listed in cmd/funclogmeasure/analyze.go.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func taskCreateInputFields(body *taskCreateJSON, actor string) []any {
	if body == nil {
		return nil
	}
	out := []any{
		"body_task_id", strings.TrimSpace(body.ID),
		"body_draft_id", strings.TrimSpace(body.DraftID),
		"body_status", string(body.Status),
		"body_priority", string(body.Priority),
		"body_title_len", len(body.Title),
		"body_title_preview", truncateRunes(body.Title, maxHTTPLogTitleRunes),
		"body_initial_prompt_len", len(body.InitialPrompt),
		"body_initial_prompt_preview", truncateRunes(body.InitialPrompt, maxHTTPLogPromptRunes),
		"body_project_id_set", body.ProjectID != nil,
		"actor", actor,
	}
	if body.ProjectID != nil {
		out = append(out, "body_project_id", strings.TrimSpace(*body.ProjectID))
	}
	if body.PickupNotBefore != nil {
		out = append(out, "body_pickup_not_before", strings.TrimSpace(*body.PickupNotBefore), "body_pickup_not_before_set", true)
	} else {
		out = append(out, "body_pickup_not_before_set", false)
	}
	out = append(out, "body_checklist_items_count", len(body.ChecklistItems))
	return out
}

// taskPatchInputFields is the PATCH /tasks/{id} mirror of
// taskCreateInputFields above; same pure-helper rationale and skip-list
// entry.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func taskPatchInputFields(body *taskPatchJSON) []any {
	if body == nil {
		return nil
	}
	out := []any{}
	if body.Title != nil {
		out = append(out, "patch_title", true, "patch_title_len", len(*body.Title), "patch_title_preview", truncateRunes(*body.Title, maxHTTPLogTitleRunes))
	}
	if body.InitialPrompt != nil {
		out = append(out, "patch_initial_prompt", true, "patch_initial_prompt_len", len(*body.InitialPrompt),
			"patch_initial_prompt_preview", truncateRunes(*body.InitialPrompt, maxHTTPLogPromptRunes))
	}
	if body.Status != nil {
		out = append(out, "patch_status", string(*body.Status))
	}
	if body.Priority != nil {
		out = append(out, "patch_priority", string(*body.Priority))
	}
	if body.ProjectID.Defined {
		if body.ProjectID.Clear {
			out = append(out, "patch_project_id", "clear")
		} else {
			out = append(out, "patch_project_id", body.ProjectID.SetID)
		}
	}
	if body.PickupNotBefore.Defined {
		if body.PickupNotBefore.Clear {
			out = append(out, "patch_pickup_not_before", "clear")
		} else {
			out = append(out, "patch_pickup_not_before", body.PickupNotBefore.Set.UTC().Format(time.RFC3339))
		}
	}
	if body.CursorModel != nil {
		out = append(out, "patch_cursor_model", true, "patch_cursor_model_len", len(strings.TrimSpace(*body.CursorModel)))
	}
	return out
}
