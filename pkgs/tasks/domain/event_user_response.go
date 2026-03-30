package domain

// EventTypeAcceptsUserResponse reports whether clients may PATCH user_response on this audit row.
// Keep aligned with web `eventTypeNeedsUserInput` (taskEventNeedsUser.ts).
func EventTypeAcceptsUserResponse(t EventType) bool {
	switch t {
	case EventApprovalRequested, EventTaskFailed:
		return true
	default:
		return false
	}
}
