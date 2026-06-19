// Package realtime defines SSE wire types, coalesce policy, and the
// Publisher port for in-process fanout. Transport (HTTP GET /events,
// ring buffer, eviction) lives in pkgs/tasks/handler; this package is
// the stable import surface for worker, supervisor, and harness adapters.
package realtime
