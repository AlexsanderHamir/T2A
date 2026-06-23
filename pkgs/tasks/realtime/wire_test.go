package realtime

import "testing"

func productionChangeTypes() []ChangeType {
	return []ChangeType{
		TaskCreated,
		TaskUpdated,
		TaskDeleted,
		TaskGateChanged,
		TaskDependencyChanged,
		TaskCycleChanged,
		AgentRunProgress,
		ProjectCreated,
		ProjectUpdated,
		ProjectDeleted,
		ProjectContextChanged,
		SettingsChanged,
		AgentRunCancelled,
		Resync,
	}
}

func TestProductionChangeTypesManifest(t *testing.T) {
	t.Parallel()

	types := productionChangeTypes()
	if len(types) != 14 {
		t.Fatalf("productionChangeTypes len = %d, want 14 (update manifest when adding ChangeType)", len(types))
	}

	seen := make(map[ChangeType]struct{}, len(types))
	for _, ct := range types {
		if ct == "" {
			t.Fatal("empty ChangeType in manifest")
		}
		if _, dup := seen[ct]; dup {
			t.Fatalf("duplicate ChangeType in manifest: %q", ct)
		}
		seen[ct] = struct{}{}
	}
}
