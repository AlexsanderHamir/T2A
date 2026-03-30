package domain

import "time"

// ResponseThreadEntry is one message in the user↔agent thread on an event
// (stored as JSON in task_events.response_thread_json).
type ResponseThreadEntry struct {
	At   time.Time `json:"at"`
	By   Actor     `json:"by"`
	Body string    `json:"body"`
}
