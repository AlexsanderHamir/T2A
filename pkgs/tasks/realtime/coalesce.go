package realtime

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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
