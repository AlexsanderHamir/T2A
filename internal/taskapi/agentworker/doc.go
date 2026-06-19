// Package agentworker owns the in-process agent worker supervisor:
// bounded ready-task queue wiring lives in cmd/taskapi; this package
// reads app_settings, probes/builds runners, spawns worker.Worker
// incarnations, hot-reloads on material changes, and drains on shutdown.
package agentworker
