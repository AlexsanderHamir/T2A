// Package agentsmoke provides shared test fixtures for the operator-run
// real-cursor smoke (see docs/AGENT-WORKER.md "Smoke run"). It exposes
// a Fixture that owns a per-test tempdir, a canonical Cursor prompt
// requesting a single deterministic filesystem mutation, and
// post-condition assertions over the resulting workspace.
//
// The package has zero hard dependency on the cursor adapter: the
// runner-layer smoke
// (pkgs/agents/runner/cursor/cursor_real_smoke_test.go) hands a
// Fixture to the real cursor.Adapter directly, and the full-stack
// smoke (pkgs/tasks/agentreconcile/agent_real_cursor_e2e_test.go)
// hands the same Fixture (transitively, via T2A_AGENT_WORKER_WORKING_DIR)
// to the full HTTP -> worker -> cursor stack. The harness ships with
// a fake-runner self-test that proves the assertion logic recognises
// both the happy path and the common failure modes (missing target,
// wrong contents, unexpected sibling files, untouched workdir).
//
// A real Cursor invocation is non-deterministic by construction, so
// the prompt is shaped to demand a single mechanical filesystem
// mutation expressible as one shell/editor command, forbid tangential
// work, and be self-checking from disk alone. The test asserts on
// os.ReadFile(target), never on Cursor's stdout — Cursor is allowed
// to be verbose, hallucinate, or refuse politely; what matters is
// whether the side effect landed.
package agentsmoke
