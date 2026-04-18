// Package agentsmoke provides shared test fixtures for Stages 2 and 3
// of docs/AGENT-WORKER-SMOKE-PLAN.md. It exposes a Fixture that owns
// a per-test tempdir, a canonical Cursor prompt requesting a single
// deterministic filesystem mutation, and post-condition assertions
// over the resulting workspace.
//
// The package has zero hard dependency on the cursor adapter: Stage 2
// hands a Fixture to the real cursor.Adapter directly, Stage 3 hands
// the same Fixture (transitively, via T2A_AGENT_WORKER_WORKING_DIR)
// to the full HTTP -> worker -> cursor stack. Stage 1 (this commit)
// only ships the harness and a fake-runner self-test that proves the
// assertion logic recognises both the happy path and the common
// failure modes (missing target, wrong contents, unexpected sibling
// files, untouched workdir).
//
// See docs/AGENT-WORKER-SMOKE-PLAN.md "Why the prompt must be
// deterministic" for the rationale behind the prompt shape and the
// "outcome, not process" assertion model.
package agentsmoke
