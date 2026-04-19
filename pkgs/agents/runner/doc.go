// Package runner is the contract every agent invocation must satisfy.
//
// One Runner.Run call corresponds to exactly one execution-cycle phase
// (diagnose / execute / verify / persist as defined in moat.md and
// docs/EXECUTION-CYCLES.md). The worker in pkgs/agents/worker (contract:
// docs/AGENT-WORKER.md) drives the lifecycle (start cycle, start phase,
// patch phase) and asks a Runner to actually do the per-phase work.
//
// # Multi-runner roadmap
//
// V1 ships a single concrete adapter for Cursor's CLI (see
// pkgs/agents/runner/cursor). Additional runners (Codex CLI, Claude
// Code CLI, in-process tool callers, etc.) land as new packages under
// pkgs/agents/runner/ without touching the worker. The Runner
// interface is therefore deliberately minimal: anything that can be
// expressed as "given a Request, eventually return a Result or a
// typed error" can be a Runner.
//
// # Wire format
//
// Request and Result use snake_case JSON tags. The shape is part of the
// contract and is pinned by runner_test.go. Adapters that serialise either
// type onto a wire (file, pipe, RPC envelope) MUST round-trip cleanly through
// these structs so behaviour stays adapter-agnostic at the worker layer.
//
// # Secret redaction
//
// Every adapter is responsible for redacting secrets from Result.RawOutput
// and Result.Details BEFORE handing the Result back. Redaction belongs in the
// adapter because only the adapter knows which env vars and which CLI flags
// it passed to the underlying tool. The runner package itself only enforces
// byte caps (see NewResult); it does not look at content.
//
// The repo-wide rule from docs/OBSERVABILITY.md applies: never persist or
// log passwords, tokens, full Authorization headers, or DATABASE_URL.
//
// # Determinism for tests
//
// runnerfake.Runner is the canonical fake used by every later test in the
// V1 worker plan. It is keyed on (TaskID, Phase) so tests can script the
// outcome of each phase without depending on a real CLI.
package runner
