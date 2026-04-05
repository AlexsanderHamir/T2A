package handler

import (
	"encoding/json"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

type taskCreateJSON struct {
	ID               string          `json:"id"`
	Title            string          `json:"title"`
	InitialPrompt    string          `json:"initial_prompt"`
	Status           domain.Status   `json:"status"`
	Priority         domain.Priority `json:"priority"`
	ParentID         *string         `json:"parent_id"`
	ChecklistInherit *bool           `json:"checklist_inherit"`
}

type taskPatchJSON struct {
	Title            *string          `json:"title"`
	InitialPrompt    *string          `json:"initial_prompt"`
	Status           *domain.Status   `json:"status"`
	Priority         *domain.Priority `json:"priority"`
	ParentID         patchParentField `json:"parent_id"`
	ChecklistInherit *bool            `json:"checklist_inherit"`
}

type listResponse struct {
	Tasks   []store.TaskNode `json:"tasks"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
	HasMore bool             `json:"has_more"`
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
