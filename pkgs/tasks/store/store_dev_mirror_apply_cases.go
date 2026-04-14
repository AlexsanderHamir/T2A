package store

import (
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

func devMirrorStatusChanged(tx *gorm.DB, taskID string, t *domain.Task, data []byte) (map[string]any, error) {
	m, err := pairFromJSON(data)
	if err != nil {
		return nil, err
	}
	up := map[string]any{}
	st := domain.Status(m["to"])
	if validStatus(st) && st != t.Status {
		if st == domain.StatusDone {
			if err := validateCanMarkDoneTx(tx, taskID); err != nil {
				return nil, err
			}
		}
		up["status"] = string(st)
	}
	return up, nil
}

func devMirrorPriorityChanged(t *domain.Task, data []byte) (map[string]any, error) {
	m, err := pairFromJSON(data)
	if err != nil {
		return nil, err
	}
	up := map[string]any{}
	pr := domain.Priority(m["to"])
	if validPriority(pr) && pr != t.Priority {
		up["priority"] = string(pr)
	}
	return up, nil
}

func devMirrorPromptOrTitle(t *domain.Task, data []byte, field string) (map[string]any, error) {
	m, err := pairFromJSON(data)
	if err != nil {
		return nil, err
	}
	up := map[string]any{}
	if field == "prompt" {
		to := m["to"]
		if to != "" && to != t.InitialPrompt {
			up["initial_prompt"] = to
		}
		return up, nil
	}
	to := strings.TrimSpace(m["to"])
	if to != "" && to != t.Title {
		up["title"] = to
	}
	return up, nil
}

func devMirrorTaskCompleted(tx *gorm.DB, taskID string, t *domain.Task) (map[string]any, error) {
	if err := validateCanMarkDoneTx(tx, taskID); err != nil {
		return nil, err
	}
	up := map[string]any{}
	if t.Status != domain.StatusDone {
		up["status"] = string(domain.StatusDone)
	}
	return up, nil
}

func devMirrorTaskFailed(t *domain.Task) map[string]any {
	up := map[string]any{}
	if t.Status != domain.StatusFailed {
		up["status"] = string(domain.StatusFailed)
	}
	return up
}
