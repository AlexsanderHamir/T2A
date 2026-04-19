package handler

import (
	"encoding/json"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

type taskCreateJSON struct {
	ID               string          `json:"id"`
	DraftID          string          `json:"draft_id"`
	Title            string          `json:"title"`
	InitialPrompt    string          `json:"initial_prompt"`
	Status           domain.Status   `json:"status"`
	Priority         domain.Priority `json:"priority"`
	TaskType         domain.TaskType `json:"task_type"`
	ParentID         *string         `json:"parent_id"`
	ChecklistInherit *bool           `json:"checklist_inherit"`
	Runner           *string         `json:"runner"`
	CursorModel      *string         `json:"cursor_model"`
}

type taskEvaluateJSON struct {
	ID               string                                  `json:"id"`
	Title            string                                  `json:"title"`
	InitialPrompt    string                                  `json:"initial_prompt"`
	Status           domain.Status                           `json:"status"`
	Priority         domain.Priority                         `json:"priority"`
	TaskType         domain.TaskType                         `json:"task_type"`
	ParentID         *string                                 `json:"parent_id"`
	ChecklistInherit *bool                                   `json:"checklist_inherit"`
	ChecklistItems   []store.EvaluateDraftChecklistItemInput `json:"checklist_items"`
}

type taskDraftSaveJSON struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Payload json.RawMessage `json:"payload"`
}

type taskPatchJSON struct {
	Title            *string          `json:"title"`
	InitialPrompt    *string          `json:"initial_prompt"`
	Status           *domain.Status   `json:"status"`
	Priority         *domain.Priority `json:"priority"`
	TaskType         *domain.TaskType `json:"task_type"`
	ParentID         patchParentField `json:"parent_id"`
	ChecklistInherit *bool            `json:"checklist_inherit"`
}

type listResponse struct {
	Tasks   []store.TaskNode `json:"tasks"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
	HasMore bool             `json:"has_more"`
}

type taskStatsResponse struct {
	Total      int64                     `json:"total"`
	Ready      int64                     `json:"ready"`
	Critical   int64                     `json:"critical"`
	ByStatus   map[domain.Status]int64   `json:"by_status"`
	ByPriority map[domain.Priority]int64 `json:"by_priority"`
	ByScope    map[string]int64          `json:"by_scope"`
}

type taskEventLine struct {
	Seq            int64                        `json:"seq"`
	At             time.Time                    `json:"at"`
	Type           domain.EventType             `json:"type"`
	By             domain.Actor                 `json:"by"`
	Data           json.RawMessage              `json:"data"`
	UserResponse   *string                      `json:"user_response,omitempty"`
	UserResponseAt *time.Time                   `json:"user_response_at,omitempty"`
	ResponseThread []domain.ResponseThreadEntry `json:"response_thread,omitempty"`
}

type taskEventsResponse struct {
	TaskID          string          `json:"task_id"`
	Events          []taskEventLine `json:"events"`
	Limit           *int            `json:"limit,omitempty"`
	Total           *int64          `json:"total,omitempty"`
	RangeStart      *int64          `json:"range_start,omitempty"`
	RangeEnd        *int64          `json:"range_end,omitempty"`
	HasMoreNewer    bool            `json:"has_more_newer"`
	HasMoreOlder    bool            `json:"has_more_older"`
	ApprovalPending bool            `json:"approval_pending"`
}

type taskEventDetailResponse struct {
	TaskID         string                       `json:"task_id"`
	Seq            int64                        `json:"seq"`
	At             time.Time                    `json:"at"`
	Type           domain.EventType             `json:"type"`
	By             domain.Actor                 `json:"by"`
	Data           json.RawMessage              `json:"data"`
	UserResponse   *string                      `json:"user_response,omitempty"`
	UserResponseAt *time.Time                   `json:"user_response_at,omitempty"`
	ResponseThread []domain.ResponseThreadEntry `json:"response_thread,omitempty"`
}

type taskEventUserResponseJSON struct {
	UserResponse string `json:"user_response"`
}
