package handler

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
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
	WorktreeID            *string                          `json:"worktree_id,omitempty"`
	BranchID              *string                          `json:"branch_id,omitempty"`
	WorktreeBranchID      *string                          `json:"worktree_branch_id,omitempty"`
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

type taskTemplateInstantiateItemJSON struct {
	TemplateID string `json:"template_id"`
	Count      *int   `json:"count,omitempty"`
}

type taskTemplateInstantiateJSON struct {
	TemplateIDs []string                          `json:"template_ids,omitempty"`
	Count       *int                              `json:"count,omitempty"`
	Items       []taskTemplateInstantiateItemJSON `json:"items,omitempty"`
}

type taskTemplateInstantiateItem struct {
	TemplateID string
	Count      int
}

type taskTemplateInstantiateErrorJSON struct {
	TemplateID string `json:"template_id"`
	Error      string `json:"error"`
}

type taskTemplateInstantiateResponseJSON struct {
	Tasks  []domain.Task                      `json:"tasks"`
	Errors []taskTemplateInstantiateErrorJSON `json:"errors"`
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func resolveInstantiateCount(raw *int) (int, error) {
	if raw == nil {
		return 1, nil
	}
	if *raw < 1 || *raw > maxTemplateInstantiateCountPerItem {
		return 0, fmt.Errorf("%w: count must be integer 1..%d", domain.ErrInvalidInput, maxTemplateInstantiateCountPerItem)
	}
	return *raw, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func normalizeInstantiateItems(body taskTemplateInstantiateJSON) ([]taskTemplateInstantiateItem, error) {
	if len(body.Items) > 0 {
		items := make([]taskTemplateInstantiateItem, 0, len(body.Items))
		seen := make(map[string]struct{}, len(body.Items))
		total := 0
		for _, row := range body.Items {
			templateID := strings.TrimSpace(row.TemplateID)
			if templateID == "" {
				return nil, fmt.Errorf("%w: template id required", domain.ErrInvalidInput)
			}
			if _, dup := seen[templateID]; dup {
				return nil, fmt.Errorf("%w: duplicate template_id %q in items", domain.ErrInvalidInput, templateID)
			}
			seen[templateID] = struct{}{}
			count, err := resolveInstantiateCount(row.Count)
			if err != nil {
				return nil, err
			}
			total += count
			if total > maxTemplateInstantiateTotalCreates {
				return nil, fmt.Errorf("%w: total creates must not exceed %d", domain.ErrInvalidInput, maxTemplateInstantiateTotalCreates)
			}
			items = append(items, taskTemplateInstantiateItem{
				TemplateID: templateID,
				Count:      count,
			})
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("%w: items required", domain.ErrInvalidInput)
		}
		return items, nil
	}

	if len(body.TemplateIDs) == 0 {
		return nil, fmt.Errorf("%w: template_ids or items required", domain.ErrInvalidInput)
	}
	defaultCount, err := resolveInstantiateCount(body.Count)
	if err != nil {
		return nil, err
	}
	total := defaultCount * len(body.TemplateIDs)
	if total > maxTemplateInstantiateTotalCreates {
		return nil, fmt.Errorf("%w: total creates must not exceed %d", domain.ErrInvalidInput, maxTemplateInstantiateTotalCreates)
	}
	items := make([]taskTemplateInstantiateItem, 0, len(body.TemplateIDs))
	for _, templateID := range body.TemplateIDs {
		templateID = strings.TrimSpace(templateID)
		if templateID == "" {
			return nil, fmt.Errorf("%w: template id required", domain.ErrInvalidInput)
		}
		items = append(items, taskTemplateInstantiateItem{
			TemplateID: templateID,
			Count:      defaultCount,
		})
	}
	return items, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func composePayloadToRaw(payload taskComposePayloadJSON) (json.RawMessage, error) {
	return json.Marshal(payload)
}
