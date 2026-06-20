package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
)

type taskCommitEntry struct {
	CycleID     string `json:"cycle_id"`
	AttemptSeq  int64  `json:"attempt_seq"`
	Seq         int64  `json:"seq"`
	Repo        string `json:"repo"`
	Worktree    string `json:"worktree"`
	Branch      string `json:"branch"`
	SHA         string `json:"sha"`
	CommittedAt string `json:"committed_at"`
	Message     string `json:"message"`
}

type taskCommitsResponse struct {
	TaskID  string            `json:"task_id"`
	Commits []taskCommitEntry `json:"commits"`
}

// getTaskCommits handles GET /tasks/{id}/commits.
func (h *Handler) getTaskCommits(w http.ResponseWriter, r *http.Request) {
	const op = "tasks.commits.list"
	r = calltrace.WithRequestRoot(r, op)
	taskID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "task_id", taskID)
	if _, err := h.store.Get(r.Context(), taskID); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	rows, err := h.store.ListCommitsForTask(r.Context(), taskID)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	cycles, err := h.store.ListCyclesForTaskBefore(r.Context(), taskID, 0, 500)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	attemptByCycle := make(map[string]int64, len(cycles))
	for i := range cycles {
		attemptByCycle[cycles[i].ID] = cycles[i].AttemptSeq
	}
	resp := taskCommitsResponse{
		TaskID:  taskID,
		Commits: make([]taskCommitEntry, 0, len(rows)),
	}
	for i := range rows {
		row := &rows[i]
		resp.Commits = append(resp.Commits, taskCommitEntry{
			CycleID:     row.CycleID,
			AttemptSeq:  attemptByCycle[row.CycleID],
			Seq:         row.Seq,
			Repo:        row.Repo,
			Worktree:    row.Worktree,
			Branch:      row.Branch,
			SHA:         row.SHA,
			CommittedAt: row.CommittedAt.UTC().Format(time.RFC3339),
			Message:     row.Message,
		})
	}
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.getTaskCommits",
		"task_id", taskID, "commit_count", len(resp.Commits))
	writeJSON(w, r, op, http.StatusOK, resp)
}
