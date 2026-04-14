package store

import (
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

func devMirrorRowUpdates(tx *gorm.DB, taskID string, t *domain.Task, typ domain.EventType, data []byte) (map[string]any, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.devMirrorRowUpdates")
	switch typ {
	case domain.EventStatusChanged:
		return devMirrorStatusChanged(tx, taskID, t, data)
	case domain.EventPriorityChanged:
		return devMirrorPriorityChanged(t, data)
	case domain.EventPromptAppended:
		return devMirrorPromptOrTitle(t, data, "prompt")
	case domain.EventMessageAdded:
		return devMirrorPromptOrTitle(t, data, "title")
	case domain.EventTaskCompleted:
		return devMirrorTaskCompleted(tx, taskID, t)
	case domain.EventTaskFailed:
		return devMirrorTaskFailed(t), nil
	default:
		return nil, nil
	}
}
