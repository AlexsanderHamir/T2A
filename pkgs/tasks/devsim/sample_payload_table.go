package devsim

import (
	"encoding/json"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// samplePayloadByType returns deterministic JSON payloads for the dev ticker.
// default branch in samplePayload handles unknown types.
var samplePayloadByType = map[domain.EventType]func() ([]byte, error){
	domain.EventStatusChanged: func() ([]byte, error) {
		return json.Marshal(map[string]string{"from": "ready", "to": "running"})
	},
	domain.EventPriorityChanged: func() ([]byte, error) {
		return json.Marshal(map[string]string{"from": "medium", "to": "high"})
	},
	domain.EventPromptAppended: func() ([]byte, error) {
		return json.Marshal(map[string]string{"from": "<p>a</p>", "to": "<p>a</p><p>b</p>"})
	},
	domain.EventMessageAdded: func() ([]byte, error) {
		return json.Marshal(map[string]string{"from": "Title A", "to": "Title B"})
	},
	domain.EventContextAdded: func() ([]byte, error) {
		return json.Marshal(map[string]string{"summary": "Repo layout", "detail": "Tasks live under pkgs/tasks."})
	},
	domain.EventConstraintAdded: func() ([]byte, error) {
		return json.Marshal(map[string]string{"text": "Must keep default go test ./... green."})
	},
	domain.EventSuccessCriterionAdded: func() ([]byte, error) {
		return json.Marshal(map[string]string{"text": "UI timeline renders without console errors."})
	},
	domain.EventNonGoalAdded: func() ([]byte, error) {
		return json.Marshal(map[string]string{"text": "No production deploy in this iteration."})
	},
	domain.EventPlanAdded: func() ([]byte, error) {
		return json.Marshal(map[string]any{
			"title": "Dev sim plan",
			"steps": []string{"Sketch", "Implement", "Verify"},
		})
	},
	domain.EventSubtaskAdded: func() ([]byte, error) {
		return json.Marshal(map[string]string{
			"child_task_id": "00000000-0000-0000-0000-000000000099",
			"title":         "Child (synthetic id)",
		})
	},
	domain.EventSubtaskRemoved: func() ([]byte, error) {
		return json.Marshal(map[string]string{
			"child_task_id": "00000000-0000-0000-0000-000000000099",
			"title":         "Removed child (synthetic)",
		})
	},
	domain.EventChecklistItemAdded: func() ([]byte, error) {
		return json.Marshal(map[string]string{"item_id": "cli-dev-1", "text": "Run go test ./..."})
	},
	domain.EventChecklistItemToggled: func() ([]byte, error) {
		return json.Marshal(map[string]string{"item_id": "cli-dev-1", "done": "true"})
	},
	domain.EventChecklistItemUpdated: func() ([]byte, error) {
		return json.Marshal(map[string]string{"item_id": "cli-dev-1", "text": "Run go test ./... (updated)"})
	},
	domain.EventChecklistItemRemoved: func() ([]byte, error) {
		return json.Marshal(map[string]string{"item_id": "cli-dev-1", "text": "Removed criterion (synthetic)"})
	},
	domain.EventChecklistInheritChanged: func() ([]byte, error) {
		return json.Marshal(map[string]bool{"from": false, "to": true})
	},
	domain.EventArtifactAdded: func() ([]byte, error) {
		return json.Marshal(map[string]string{"name": "notes.md", "uri": "file:///tmp/t2a-devsim"})
	},
	domain.EventApprovalRequested: func() ([]byte, error) {
		return json.Marshal(map[string]string{"reason": "Checkpoint ready", "checkpoint": "plan_review"})
	},
	domain.EventApprovalGranted: func() ([]byte, error) {
		return json.Marshal(map[string]string{"grantor": "lead", "note": "LGTM (synthetic)"})
	},
	domain.EventTaskCompleted: func() ([]byte, error) {
		return json.Marshal(map[string]string{"summary": "Synthetic completion."})
	},
	domain.EventTaskFailed: func() ([]byte, error) {
		return json.Marshal(map[string]string{"error": "Simulated failure", "retryable": "true"})
	},
	domain.EventSyncPing: func() ([]byte, error) {
		return json.Marshal(map[string]string{"source": "devsim"})
	},
}
