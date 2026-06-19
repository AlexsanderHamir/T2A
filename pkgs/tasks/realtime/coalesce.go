package realtime

// CoalesceKey returns a canonical "type:id" dedup key for hint-only
// frames inside the hub coalesce window. Empty string means do not coalesce.
func CoalesceKey(ev Event) string {
	if ev.Type == TaskCycleChanged || ev.Type == AgentRunProgress {
		return ""
	}
	if ev.Data != nil {
		return ""
	}
	return string(ev.Type) + ":" + ev.ID
}
