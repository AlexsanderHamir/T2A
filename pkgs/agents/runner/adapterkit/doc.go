// Package adapterkit provides reusable building blocks for CLI-backed runner
// adapters.
//
// The package intentionally knows nothing about Cursor, task phases, models,
// or runner-specific output protocols. It owns the mechanics that every CLI
// runner needs: bounded command execution, environment allow/deny policies,
// baseline redaction, UTF-8-safe diagnostics, and simple probe helpers.
package adapterkit
