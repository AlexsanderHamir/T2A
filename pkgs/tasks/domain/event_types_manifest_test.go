package domain

import "testing"

// productionEventTypes is the canonical audit EventType set mirrored by
// web TASK_EVENT_TYPES. Update in lockstep with enums.go and
// web/src/types/contractManifest.test.ts.
func productionEventTypes() []EventType {
	return []EventType{
		EventTaskCreated,
		EventStatusChanged,
		EventPriorityChanged,
		EventPromptAppended,
		EventContextAdded,
		EventConstraintAdded,
		EventSuccessCriterionAdded,
		EventNonGoalAdded,
		EventPlanAdded,
		EventChecklistItemAdded,
		EventChecklistItemToggled,
		EventChecklistItemUpdated,
		EventChecklistItemRemoved,
		EventMessageAdded,
		EventArtifactAdded,
		EventApprovalRequested,
		EventApprovalGranted,
		EventTaskCompleted,
		EventOnTaskDone,
		EventTaskFailed,
		EventTaskRetryRequested,
		EventCycleStarted,
		EventCycleCompleted,
		EventCycleFailed,
		EventPhaseStarted,
		EventPhaseCompleted,
		EventPhaseFailed,
		EventPhaseSkipped,
		EventSyncPing,
	}
}

func TestProductionEventTypesManifest(t *testing.T) {
	t.Parallel()

	types := productionEventTypes()
	if len(types) != 29 {
		t.Fatalf("productionEventTypes len = %d, want 29 (update manifest when adding EventType)", len(types))
	}

	seen := make(map[EventType]struct{}, len(types))
	for _, et := range types {
		if et == "" {
			t.Fatal("empty EventType in manifest")
		}
		if _, dup := seen[et]; dup {
			t.Fatalf("duplicate EventType in manifest: %q", et)
		}
		seen[et] = struct{}{}
	}

	for _, removed := range []string{"subtask_added", "task_type"} {
		if _, ok := seen[EventType(removed)]; ok {
			t.Fatalf("removed EventType %q must not appear in production manifest", removed)
		}
	}
}
