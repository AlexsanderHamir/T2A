package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/apijson"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
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
func taskCreateInputFields(body *taskCreateJSON, actor string) []any {
	if body == nil {
		return nil
	}
	inherit := false
	if body.ChecklistInherit != nil {
		inherit = *body.ChecklistInherit
	}
	out := []any{
		"body_task_id", strings.TrimSpace(body.ID),
		"body_draft_id", strings.TrimSpace(body.DraftID),
		"body_status", string(body.Status),
		"body_priority", string(body.Priority),
		"body_task_type", string(body.TaskType),
		"body_title_len", len(body.Title),
		"body_title_preview", truncateRunes(body.Title, maxHTTPLogTitleRunes),
		"body_initial_prompt_len", len(body.InitialPrompt),
		"body_initial_prompt_preview", truncateRunes(body.InitialPrompt, maxHTTPLogPromptRunes),
		"body_parent_id_set", body.ParentID != nil,
		"body_checklist_inherit", inherit,
		"actor", actor,
	}
	if body.ParentID != nil {
		out = append(out, "body_parent_id", strings.TrimSpace(*body.ParentID))
	}
	return out
}

// taskPatchInputFields is the PATCH /tasks/{id} mirror of
// taskCreateInputFields above; same pure-helper rationale and skip-list
// entry.
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
	if body.TaskType != nil {
		out = append(out, "patch_task_type", string(*body.TaskType))
	}
	if body.ChecklistInherit != nil {
		out = append(out, "patch_checklist_inherit", *body.ChecklistInherit)
	}
	if body.ParentID.Defined {
		if body.ParentID.Clear {
			out = append(out, "patch_parent_id", "clear")
		} else {
			out = append(out, "patch_parent_id", body.ParentID.SetID)
		}
	}
	return out
}
