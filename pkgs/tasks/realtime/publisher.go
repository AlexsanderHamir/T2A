package realtime

// Publisher fans out change events to connected SSE clients. Implemented
// by handler.SSEHub in production; tests may use a recording fake.
type Publisher interface {
	Publish(ev Event)
}
