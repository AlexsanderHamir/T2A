package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

type createTaskComposeOpts struct {
	ID                      string
	DraftID                 string
	Gate                    *domain.TaskGate
	StripDependsOn          bool
	OmitPastPickupNotBefore bool
	InstantiateFromTemplate bool
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func taskCreateJSONToCompose(body taskCreateJSON) taskComposePayloadJSON {
	return taskComposePayloadJSON{
		Title:                 body.Title,
		InitialPrompt:         body.InitialPrompt,
		Status:                body.Status,
		Priority:              body.Priority,
		ProjectID:             body.ProjectID,
		ProjectContextItemIDs: body.ProjectContextItemIDs,
		Runner:                body.Runner,
		CursorModel:           body.CursorModel,
		PickupNotBefore:       body.PickupNotBefore,
		Tags:                  body.Tags,
		Milestone:             body.Milestone,
		DependsOn:             body.DependsOn,
		ChecklistItems:        body.ChecklistItems,
		WorktreeID:            body.WorktreeID,
		BranchID:              body.BranchID,
		WorktreeBranchID:      body.WorktreeBranchID,
	}
}

func (h *Handler) createTaskFromComposeJSON(
	ctx context.Context,
	r *http.Request,
	op string,
	payload taskComposePayloadJSON,
	opts createTaskComposeOpts,
	by domain.Actor,
) (*domain.Task, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.createTaskFromComposeJSON")
	settings, err := h.store.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.validateComposePayload(r.Context(), payload, settings); err != nil {
		return nil, err
	}
	runner, cursorModel, err := resolveRunnerModelFields(payload.Runner, payload.CursorModel, settings)
	if err != nil {
		return nil, err
	}
	pickupNotBefore, err := resolvePickupNotBeforeForCreate(payload.PickupNotBefore, payload.Status, settings)
	if err != nil {
		return nil, err
	}
	if opts.OmitPastPickupNotBefore {
		pickupNotBefore = omitPastPickupNotBefore(pickupNotBefore)
	}
	var dependsOn []domain.DependencyEdge
	if !opts.StripDependsOn {
		dependsOn, err = parseDependsOnWire(payload.DependsOn)
		if err != nil {
			return nil, err
		}
	}
	checklistItems, err := parseCreateChecklistItems(payload.ChecklistItems)
	if err != nil {
		return nil, err
	}
	draftID := opts.DraftID
	if opts.InstantiateFromTemplate {
		draftID = ""
	}
	t, err := h.store.Create(ctx, store.CreateTaskInput{
		ID:                    opts.ID,
		DraftID:               draftID,
		Title:                 payload.Title,
		InitialPrompt:         payload.InitialPrompt,
		Status:                payload.Status,
		Priority:              payload.Priority,
		ProjectID:             payload.ProjectID,
		ProjectContextItemIDs: payload.ProjectContextItemIDs,
		Runner:                runner,
		CursorModel:           cursorModel,
		PickupNotBefore:       pickupNotBefore,
		Tags:                  payload.Tags,
		Milestone:             payload.Milestone,
		Gate:                  opts.Gate,
		DependsOn:             dependsOn,
		ChecklistItems:        checklistItems,
		WorktreeID:            payload.WorktreeID,
		BranchID:              payload.BranchID,
		WorktreeBranchID:      payload.WorktreeBranchID,
	}, by)
	if err != nil {
		return nil, err
	}
	return h.finalizeCreatedTask(ctx, t)
}

func (h *Handler) finalizeCreatedTask(ctx context.Context, t *domain.Task) (*domain.Task, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.finalizeCreatedTask")
	h.notifyTaskChanged(TaskCreated, t.ID, t)
	if t.Gate != nil {
		h.notifyChange(TaskGateChanged, t.ID)
	}
	if len(t.DependsOn) > 0 {
		h.notifyChange(TaskDependencyChanged, t.ID)
	}
	taskapiDomainTasksCreatedTotal.Inc()
	return t, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Handler) validateComposePayload(ctx context.Context, payload taskComposePayloadJSON, settings domain.AppSettings) error {
	if err := h.validatePromptMentionsIfRepo(ctx, payload.WorktreeID, payload.InitialPrompt); err != nil {
		return err
	}
	if err := h.validateTaskGitBinding(ctx, payload.ProjectID, payload.WorktreeID, payload.BranchID, payload.WorktreeBranchID); err != nil {
		return err
	}
	if _, _, err := resolveRunnerModelFields(payload.Runner, payload.CursorModel, settings); err != nil {
		return err
	}
	if _, err := resolvePickupNotBeforeForCreate(payload.PickupNotBefore, payload.Status, settings); err != nil {
		return err
	}
	if strings.TrimSpace(payload.Title) == "" {
		return fmt.Errorf("%w: title required", domain.ErrInvalidInput)
	}
	if payload.Priority == "" {
		return fmt.Errorf("%w: priority required", domain.ErrInvalidInput)
	}
	if _, err := parseCreateChecklistItems(payload.ChecklistItems); err != nil {
		return err
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func omitPastPickupNotBefore(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	if t.Before(time.Now().UTC()) {
		return nil
	}
	return t
}
