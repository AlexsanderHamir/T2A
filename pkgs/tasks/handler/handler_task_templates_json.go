package handler

import (
	"encoding/json"
	"fmt"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

type taskComposePayloadJSON struct {
	Title                 string                           `json:"title"`
	InitialPrompt         string                           `json:"initial_prompt"`
	Status                domain.Status                    `json:"status"`
	Priority              domain.Priority                  `json:"priority"`
	ProjectID             *string                          `json:"project_id"`
	ProjectContextItemIDs []string                         `json:"project_context_item_ids"`
	Runner                *string                          `json:"runner"`
	CursorModel           *string                          `json:"cursor_model"`
	PickupNotBefore       *string                          `json:"pickup_not_before,omitempty"`
	Tags                  []string                         `json:"tags,omitempty"`
	Milestone             *string                          `json:"milestone,omitempty"`
	DependsOn             dependsOnWire                    `json:"depends_on,omitempty"`
	ChecklistItems        []store.CreateChecklistItemInput `json:"checklist_items"`
}

type taskTemplateSaveJSON struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Payload json.RawMessage `json:"payload"`
}

type taskTemplatePatchJSON struct {
	Name    *string         `json:"name"`
	Payload json.RawMessage `json:"payload"`
}

type taskTemplateInstantiateJSON struct {
	TemplateIDs []string `json:"template_ids"`
}

type taskTemplateInstantiateErrorJSON struct {
	TemplateID string `json:"template_id"`
	Error      string `json:"error"`
}

type taskTemplateInstantiateResponseJSON struct {
	Tasks  []domain.Task                    `json:"tasks"`
	Errors []taskTemplateInstantiateErrorJSON `json:"errors"`
}

func decodeComposePayload(raw json.RawMessage) (taskComposePayloadJSON, error) {
	var payload taskComposePayloadJSON
	if len(raw) == 0 {
		return payload, fmt.Errorf("%w: payload required", domain.ErrInvalidInput)
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return payload, fmt.Errorf("%w: invalid payload: %v", domain.ErrInvalidInput, err)
	}
	return payload, nil
}

func composePayloadToRaw(payload taskComposePayloadJSON) (json.RawMessage, error) {
	return json.Marshal(payload)
}
