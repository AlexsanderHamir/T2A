package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// jsonObjectMessageEmpty is the canonical "{}" RawMessage emitted by the
// response chokepoint when a JSON-object column is missing or corrupt.
var jsonObjectMessageEmpty = json.RawMessage(`{}`)

// normalizeJSONObjectForResponse mirrors the store-side normalizeJSONObject
// chokepoint on the response side. The store enforces the "always a JSON
// object" invariant on writes (see pkgs/tasks/store/store_cycles.go), but
// legacy rows from before that chokepoint — or any out-of-band write path
// (raw SQL, migrations, future bug) — can still carry nil / empty /
// whitespace / "null" / scalars / arrays / malformed bytes in meta_json,
// details_json, or data_json. Per docs/api.md these columns must
// surface as a JSON object on every response, never as a JSON null or
// scalar (the SPA crashes on `Object.entries(null)`). Rather than 500
// for legacy data, the response builder defensively coerces anything
// non-object to "{}" so the client invariant always holds.
func normalizeJSONObjectForResponse(raw []byte) json.RawMessage {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.normalizeJSONObjectForResponse")
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return jsonObjectMessageEmpty
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return jsonObjectMessageEmpty
	}
	return json.RawMessage(trimmed)
}

// taskCycleResponseFromDomain copies a TaskCycle GORM row into the JSON
// response shape. Meta is normalized to "{}" if the column came back as
// nil / empty / whitespace / null / a scalar / an array / malformed JSON,
// matching the docs/api.md "always a JSON object" invariant.
func taskCycleResponseFromDomain(c *domain.TaskCycle) taskCycleResponse {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.taskCycleResponseFromDomain")
	meta := normalizeJSONObjectForResponse(c.MetaJSON)
	return taskCycleResponse{
		ID:            c.ID,
		TaskID:        c.TaskID,
		AttemptSeq:    c.AttemptSeq,
		Status:        c.Status,
		StartedAt:     c.StartedAt,
		EndedAt:       c.EndedAt,
		TriggeredBy:   c.TriggeredBy,
		ParentCycleID: c.ParentCycleID,
		Meta:          meta,
		CycleMeta:     projectCycleMeta(meta),
	}
}

// projectCycleMeta extracts the typed runner / model / prompt-hash
// fields from a normalized meta object (always a valid JSON object,
// per normalizeJSONObjectForResponse). Unknown keys are ignored;
// missing keys decode to "" — that empty string is meaningful and
// MUST be preserved end-to-end (see cycleMetaProjection doc). Errors
// from json.Unmarshal are not possible in practice (input is a
// valid object) but we defensively log + return the zero projection
// so a corrupt row never breaks the cycles endpoint.
func projectCycleMeta(meta json.RawMessage) cycleMetaProjection {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.projectCycleMeta")
	var out cycleMetaProjection
	if len(meta) == 0 {
		return out
	}
	if err := json.Unmarshal(meta, &out); err != nil {
		slog.Warn("cycle meta projection failed",
			"cmd", calltrace.LogCmd,
			"operation", "handler.projectCycleMeta.err",
			"err", err)
		return cycleMetaProjection{}
	}
	return out
}

// taskCyclePhaseResponseFromDomain copies a TaskCyclePhase row into the
// JSON response shape. Details is normalized via the same chokepoint as
// Meta — see taskCycleResponseFromDomain.
func taskCyclePhaseResponseFromDomain(p *domain.TaskCyclePhase) taskCyclePhaseResponse {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.taskCyclePhaseResponseFromDomain")
	details := normalizeJSONObjectForResponse(p.DetailsJSON)
	return taskCyclePhaseResponse{
		ID:        p.ID,
		CycleID:   p.CycleID,
		Phase:     p.Phase,
		PhaseSeq:  p.PhaseSeq,
		Status:    p.Status,
		StartedAt: p.StartedAt,
		EndedAt:   p.EndedAt,
		Summary:   p.Summary,
		Details:   details,
		EventSeq:  p.EventSeq,
	}
}

// taskCycleDetailFromDomain assembles the GET /tasks/{id}/cycles/{cycleId}
// envelope: cycle fields inlined plus phases in execution order.
func taskCycleDetailFromDomain(c *domain.TaskCycle, phases []domain.TaskCyclePhase) taskCycleDetailResponse {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.taskCycleDetailFromDomain")
	meta := normalizeJSONObjectForResponse(c.MetaJSON)
	out := taskCycleDetailResponse{
		ID:            c.ID,
		TaskID:        c.TaskID,
		AttemptSeq:    c.AttemptSeq,
		Status:        c.Status,
		StartedAt:     c.StartedAt,
		EndedAt:       c.EndedAt,
		TriggeredBy:   c.TriggeredBy,
		ParentCycleID: c.ParentCycleID,
		Meta:          meta,
		CycleMeta:     projectCycleMeta(meta),
		Phases:        make([]taskCyclePhaseResponse, 0, len(phases)),
	}
	for i := range phases {
		out.Phases = append(out.Phases, taskCyclePhaseResponseFromDomain(&phases[i]))
	}
	return out
}

func cycleCriteriaReportFromDomain(r *domain.TaskCycleCriteriaReport) cycleCriteriaReportEntry {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.cycleCriteriaReportFromDomain")
	return cycleCriteriaReportEntry{
		ID:          r.ID,
		CycleID:     r.CycleID,
		AttemptSeq:  r.AttemptSeq,
		CriterionID: r.CriterionID,
		ClaimedDone: r.ClaimedDone,
		Evidence:    r.Evidence,
		WrittenAt:   r.WrittenAt,
	}
}

func cycleVerifyReportFromDomain(r *domain.TaskCycleVerifyReport) cycleVerifyReportEntry {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.cycleVerifyReportFromDomain")
	return cycleVerifyReportEntry{
		ID:           r.ID,
		CycleID:      r.CycleID,
		AttemptSeq:   r.AttemptSeq,
		CriterionID:  r.CriterionID,
		Verified:     r.Verified,
		VerifierKind: r.VerifierKind,
		Reasoning:    r.Reasoning,
		WrittenAt:    r.WrittenAt,
	}
}

func cycleCommandRunFromDomain(r *domain.TaskCycleCommandRun) cycleCommandRunEntry {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.cycleCommandRunFromDomain")
	return cycleCommandRunEntry{
		ID:          r.ID,
		CycleID:     r.CycleID,
		AttemptSeq:  r.AttemptSeq,
		CriterionID: r.CriterionID,
		CommandSeq:  r.CommandSeq,
		ExitCode:    r.ExitCode,
		MetaPath:    r.MetaPath,
		WrittenAt:   r.WrittenAt,
	}
}

func cycleCommitFromDomain(r *domain.TaskCycleCommit) cycleCommitEntry {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.cycleCommitFromDomain")
	return cycleCommitEntry{
		Seq:         r.Seq,
		Repo:        r.Repo,
		Worktree:    r.Worktree,
		Branch:      r.Branch,
		SHA:         r.SHA,
		CommittedAt: r.CommittedAt,
		Message:     r.Message,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func cycleGitContextFromCommits(rows []domain.TaskCycleCommit) *cycleGitContextResponse {
	if len(rows) == 0 {
		return nil
	}
	first := rows[0]
	branch := first.Branch
	if last := rows[len(rows)-1]; last.Branch != "" {
		branch = last.Branch
	}
	return &cycleGitContextResponse{
		Repo:     first.Repo,
		Worktree: first.Worktree,
		Branch:   branch,
	}
}

func taskCycleStreamEventResponseFromDomain(ev *domain.TaskCycleStreamEvent) taskCycleStreamEventResponse {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.taskCycleStreamEventResponseFromDomain")
	return taskCycleStreamEventResponse{
		ID:        ev.ID,
		TaskID:    ev.TaskID,
		CycleID:   ev.CycleID,
		PhaseSeq:  ev.PhaseSeq,
		StreamSeq: ev.StreamSeq,
		At:        ev.At,
		Source:    ev.Source,
		Kind:      ev.Kind,
		Subtype:   ev.Subtype,
		Message:   ev.Message,
		Tool:      ev.Tool,
		Payload:   normalizeJSONObjectForResponse(ev.PayloadJSON),
	}
}
