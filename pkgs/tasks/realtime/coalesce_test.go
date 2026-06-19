package realtime

import "testing"

func TestCoalesceKey(t *testing.T) {
	tests := []struct {
		name string
		ev   Event
		want string
	}{
		{
			name: "hint task updated",
			ev:   Event{Type: TaskUpdated, ID: "t1"},
			want: "task_updated:t1",
		},
		{
			name: "enriched skips coalesce",
			ev:   Event{Type: TaskUpdated, ID: "t1", Data: map[string]any{"id": "t1"}},
			want: "",
		},
		{
			name: "cycle never coalesces",
			ev:   Event{Type: TaskCycleChanged, ID: "t1", CycleID: "c1"},
			want: "",
		},
		{
			name: "progress never coalesces",
			ev:   Event{Type: AgentRunProgress, ID: "t1", CycleID: "c1"},
			want: "",
		},
		{
			name: "settings hint",
			ev:   Event{Type: SettingsChanged, ID: "settings"},
			want: "settings_changed:settings",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CoalesceKey(tt.ev); got != tt.want {
				t.Fatalf("CoalesceKey() = %q, want %q", got, tt.want)
			}
		})
	}
}
