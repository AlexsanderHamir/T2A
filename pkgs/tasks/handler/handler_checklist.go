package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

type checklistItemCreateJSON struct {
	Text           string                     `json:"text"`
	VerifyCommands []store.VerifyCommandInput `json:"verify_commands,omitempty"`
}

type patchChecklistItemBody struct {
	Text           *string                     `json:"text,omitempty"`
	VerifyCommands *[]store.VerifyCommandInput `json:"verify_commands,omitempty"`
	Done           *bool                       `json:"done,omitempty"`
	Evidence       *string                     `json:"evidence,omitempty"`
	VerifiedBy     *string                     `json:"verified_by,omitempty"`
}

type checklistListResponse struct {
	Items []store.ChecklistItemView `json:"items"`
}

func (h *Handler) getChecklist(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.getChecklist")
	const op = "tasks.checklist.list"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "task_id", id)
	items, err := h.store.ListChecklistForSubject(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSONWithETag(w, r, op, http.StatusOK, checklistListResponse{Items: items})
}

func (h *Handler) postChecklistItem(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.postChecklistItem")
	const op = "tasks.checklist.create"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body checklistItemCreateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "task_id", id, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	by := actorFromRequest(r)
	debugHTTPRequest(r, op, "task_id", id, "actor", string(by),
		"text_len", len(body.Text), "text_preview", truncateRunes(body.Text, maxHTTPLogTextRunes))
	if running, err := h.store.IsTaskCycleRunning(r.Context(), id); err != nil {
		writeStoreError(w, r, op, err)
		return
	} else if running {
		writeStoreError(w, r, op, fmt.Errorf("%w: cycle running; cannot add criteria", domain.ErrConflict))
		return
	}
	it, err := h.store.AddChecklistItem(r.Context(), id, body.Text, body.VerifyCommands, by)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if err := h.notifyTaskUpdatedEnriched(r.Context(), id); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusCreated, it)
}

func (h *Handler) patchChecklistItem(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.patchChecklistItem")
	const op = "tasks.checklist.patch"
	r = calltrace.WithRequestRoot(r, op)
	taskID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	itemID, err := parseTaskPathItemID(r.PathValue("itemId"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body patchChecklistItemBody
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "task_id", taskID, "item_id", itemID, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	setCount := 0
	if body.Text != nil {
		setCount++
	}
	if body.VerifyCommands != nil {
		setCount++
	}
	if body.Done != nil {
		setCount++
	}
	if setCount != 1 {
		writeStoreError(w, r, op, fmt.Errorf("%w: send exactly one of text, verify_commands, or done", domain.ErrInvalidInput))
		return
	}
	by := actorFromRequest(r)
	if body.Text != nil {
		if running, err := h.store.IsTaskCycleRunning(r.Context(), taskID); err != nil {
			writeStoreError(w, r, op, err)
			return
		} else if running {
			writeStoreError(w, r, op, fmt.Errorf("%w: cycle running; cannot edit criteria", domain.ErrConflict))
			return
		}
	} else if body.VerifyCommands != nil {
		if running, err := h.store.IsTaskCycleRunning(r.Context(), taskID); err != nil {
			writeStoreError(w, r, op, err)
			return
		} else if running {
			writeStoreError(w, r, op, fmt.Errorf("%w: cycle running; cannot edit criteria", domain.ErrConflict))
			return
		}
	}
	if body.Text != nil {
		t := strings.TrimSpace(*body.Text)
		debugHTTPRequest(r, op, "task_id", taskID, "item_id", itemID, "text_len", len(t), "text_preview", truncateRunes(t, maxHTTPLogTextRunes), "actor", string(by))
		if t == "" {
			writeStoreError(w, r, op, fmt.Errorf("%w: text required", domain.ErrInvalidInput))
			return
		}
		if err := h.store.UpdateChecklistItemText(r.Context(), taskID, itemID, t, by); err != nil {
			writeStoreError(w, r, op, err)
			return
		}
	} else if body.VerifyCommands != nil {
		if err := h.store.ReplaceChecklistVerifyCommands(r.Context(), taskID, itemID, *body.VerifyCommands, by); err != nil {
			writeStoreError(w, r, op, err)
			return
		}
	} else {
		debugHTTPRequest(r, op, "task_id", taskID, "item_id", itemID, "done", *body.Done, "actor", string(by))
		if *body.Done {
			if by != domain.ActorAgent {
				writeStoreError(w, r, op, fmt.Errorf("%w: only the agent may mark checklist items done", domain.ErrInvalidInput))
				return
			}
			evidence := ""
			if body.Evidence != nil {
				evidence = strings.TrimSpace(*body.Evidence)
			}
			if evidence == "" {
				writeStoreError(w, r, op, fmt.Errorf("%w: evidence required when marking done", domain.ErrInvalidInput))
				return
			}
			verifier := domain.VerifierAgentSelf
			if body.VerifiedBy != nil {
				verifier = domain.VerifierKind(strings.TrimSpace(*body.VerifiedBy))
			}
			if !domain.ValidVerifierKind(verifier) || verifier == domain.VerifierLegacy {
				writeStoreError(w, r, op, fmt.Errorf("%w: invalid verified_by", domain.ErrInvalidInput))
				return
			}
			if err := h.store.SetChecklistItemDoneWithEvidence(r.Context(), taskID, itemID, evidence, verifier, "", "", by); err != nil {
				writeStoreError(w, r, op, err)
				return
			}
		} else if err := h.store.SetChecklistItemDone(r.Context(), taskID, itemID, false, by); err != nil {
			writeStoreError(w, r, op, err)
			return
		}
	}
	if err := h.notifyTaskUpdatedEnriched(r.Context(), taskID); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	items, err := h.store.ListChecklistForSubject(r.Context(), taskID)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, checklistListResponse{Items: items})
}

func (h *Handler) deleteChecklistItem(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.deleteChecklistItem")
	const op = "tasks.checklist.delete"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	itemID, err := parseTaskPathItemID(r.PathValue("itemId"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "task_id", id, "item_id", itemID)
	if running, err := h.store.IsTaskCycleRunning(r.Context(), id); err != nil {
		writeStoreError(w, r, op, err)
		return
	} else if running {
		writeStoreError(w, r, op, fmt.Errorf("%w: cycle running; cannot delete criteria", domain.ErrConflict))
		return
	}
	by := actorFromRequest(r)
	if err := h.store.DeleteChecklistItem(r.Context(), id, itemID, by); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if err := h.notifyTaskUpdatedEnriched(r.Context(), id); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPOut(r.Context(), op, http.StatusNoContent, "task_id", id, "item_id", itemID, "response_empty", true)
	w.WriteHeader(http.StatusNoContent)
}
