// Package kernel holds the low-level building blocks shared by every store
// subpackage under pkgs/tasks/store/internal/: the Prometheus latency
// histogram (DeferLatency + Op* constants), the audit-log primitives
// (NextEventSeq + AppendEvent + EventPairJSON), the domain enum
// validators (ValidStatus / ValidPhase / ...), and the generic
// transactional helpers (LoadTask).
//
// The package is owned exclusively by the public store facade tree. The
// Go internal/ rule keeps it from being imported by anything outside
// pkgs/tasks/store/..., which preserves the invariant that every API
// entrypoint goes through the public store package and records latency
// through the same registered histogram.
package kernel

// LogCmd is the cmd label every kernel slog call uses, mirroring the
// historical "taskapi" tag from the original pkgs/tasks/store package.
// Sub-packages that wrap kernel helpers should set their own cmd label
// when they wish to differentiate; the kernel itself is considered part
// of the same operational surface.
const LogCmd = "taskapi"
