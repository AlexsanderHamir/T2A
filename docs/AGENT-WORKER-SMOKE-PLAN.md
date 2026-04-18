# Agent worker — real-cursor smoke plan

> **Where this fits.** V1 of the agent worker shipped in
> [`AGENT-WORKER-PLAN.md`](./AGENT-WORKER-PLAN.md) (Stages 1–6). The
> contract lives in [`AGENT-WORKER.md`](./AGENT-WORKER.md). Every
> existing test uses the **fake runner** (`runnerfake`) — no test
> proves the wired-up system can actually drive a real Cursor CLI
> invocation to completion. This document is the **per-stage
> execution playbook** for the smallest test that closes that gap:
> a real `cursor-agent` is invoked, performs a real task, and we
> assert on the side effect. Same shape as
> [`AGENT-WORKER-PLAN.md`](./AGENT-WORKER-PLAN.md).

The single sentence that drives every decision below:

> **The agent should actually be used, and the task should be
> done.**

Everything else (HTTP wiring, SSE, metrics) is verified by V1's
existing fake-runner tests; what is unverified is the only thing this
plan addresses — that handing a real prompt to a real Cursor binary
through the worker results in a real, verifiable change on disk.

## Rules of engagement

1. **One stage per commit.** Each stage leaves the repo buildable,
   tested, and shippable — including with the new test silently
   skipped when Cursor CLI is not present (the default in CI).
2. **Verification gate per stage.** Stage is not "done" until its
   checklist is green AND `./scripts/check.ps1` passes locally with
   the smoke test skipped, AND (for Stages 2 + 3) the smoke test
   passes locally with `T2A_TEST_REAL_CURSOR=1` and `cursor-agent` on
   PATH.
3. **Commit + push at end of stage.** Conventional commit message,
   one logical concern, push to `main`.
4. **STOP and ask permission between stages.** No silent rollover.
5. **TDD where it makes sense.** Stage 1 ships the harness and a
   fake-runner test that proves the harness is correct **before**
   Stages 2–3 hand it a real binary. The fake-runner test is what
   keeps CI honest about the harness itself.
6. **Default-skip in CI.** The real-cursor stages are gated by both a
   Go build tag (`cursor_real`) **and** an env var
   (`T2A_TEST_REAL_CURSOR=1`). CI never depends on Cursor CLI being
   installed; operators run the smoke locally before merging changes
   to `pkgs/agents/runner/cursor` or `pkgs/agents/worker`.

## Reference points

- **Adapter under test:**
  [`pkgs/agents/runner/cursor/cursor.go`](../pkgs/agents/runner/cursor/cursor.go).
  Defaults: `cursor-agent --print --output-format json`, prompt on
  stdin, working directory from `runner.Request.WorkingDir`. Env
  allowlist + redaction documented in
  [`AGENT-WORKER.md`](./AGENT-WORKER.md) "Cursor adapter".
- **Worker contract:**
  [`AGENT-WORKER.md`](./AGENT-WORKER.md) "Lifecycle of one task" — the
  full HTTP-to-`TerminateCycle` sequence the Stage 3 test exercises.
- **Closest analog in repo:** the Stage 6 e2e at
  [`pkgs/tasks/agentreconcile/agentworker_e2e_test.go`](../pkgs/tasks/agentreconcile/agentworker_e2e_test.go).
  Stage 3 below mirrors it but starts at HTTP and swaps `runnerfake`
  for the real `cursor.Adapter`.
- **Engineering bar:**
  `.cursor/rules/BACKEND_AUTOMATION/backend-engineering-bar.mdc`,
  `.cursor/rules/BACKEND_AUTOMATION/go-testing-recipes.mdc`.

## Why the prompt must be deterministic

A real Cursor invocation is non-deterministic by construction — the
LLM picks how to phrase its reasoning, when to call tools, which
files to read first. The smoke test cannot assert on Cursor's
**process** (output text, tool sequence, latency); it can only assert
on the **outcome** the operator demanded. So the prompt must:

- **Demand a single, mechanical filesystem mutation** that is
  expressible as a one-line shell or editor command, e.g. "create a
  file at exactly `<absolute path>` whose contents are exactly the
  literal string `OK\n` and nothing else".
- **Forbid tangential work** (no commentary files, no formatting
  passes) so the assertion can be tight.
- **Be self-checking from disk alone** — the test asserts on
  `os.ReadFile(target)`, never on Cursor's stdout. Cursor is allowed
  to be verbose, lie, hallucinate, or refuse politely; what matters
  is whether the side effect landed.

If a future Cursor model declines such prompts, the plan's response
is to refine the prompt **once** and pin the working version in the
test fixture. We are not chasing a model — we are proving the wiring.

## Edge cases the plan must handle

These are the failure modes that would make the smoke test either
flaky or actively misleading. Each is addressed by the stage in
parentheses:

1. **Cursor CLI not installed.** CI must build and the test must be
   silently skipped. (Stage 2/3 — build tag + env gate; standard
   `t.Skip` with a clear message when the gate is off.)
2. **Stale workspace from a previous run.** Each test gets a fresh
   `t.TempDir()` per Go's testing API; teardown is automatic. The
   working directory passed to `runner.Request.WorkingDir` is that
   tempdir. (Stage 1 — harness owns the tempdir lifecycle.)
3. **Cursor takes too long.** The test must fail loudly (not hang)
   if the run exceeds a budget. We use a per-stage cap (Stage 2: 60s
   on the runner directly; Stage 3: `T2A_AGENT_WORKER_RUN_TIMEOUT=90s`
   bound on the worker side plus a polling deadline in the test).
4. **Cursor partially mutates the workspace.** The assertion is
   **post-condition exact match** on the target file's contents (and,
   where appropriate, a check that no other file was created in the
   tempdir). A partial mutation fails the test with a diff of
   "expected vs actual" so the operator can debug.
5. **Cursor exits non-zero but the file is correct.** This is still
   a failure of the contract (the worker would mark the cycle
   `failed`), so the test asserts both the runner result status AND
   the filesystem post-condition. Both must match the expected happy
   path for a green run.
6. **Cursor needs auth that the operator has not configured.** The
   test surfaces this clearly: if `cursor.Probe` succeeds at startup
   but `Run` returns an auth-related error, the test fails with the
   redacted runner output and a hint about
   `T2A_AGENT_WORKER_CURSOR_BIN` / Cursor login state. We do not try
   to manage auth from inside the test.
7. **Operator runs the smoke from a path with secrets in env.** The
   adapter's existing env allowlist + redaction (documented in
   [`AGENT-WORKER.md`](./AGENT-WORKER.md) "Security model") still
   applies. The test does **not** dump `RawOutput` on success and
   only logs a redacted tail on failure.
8. **Concurrent smoke runs in the same temp parent.** `t.TempDir()`
   gives each run a unique directory. The test does not share
   working directories across parallel `t.Parallel()` invocations.

## Stages

> Each stage is a single commit. Stage 0 (this commit) is the plan
> itself; commits land in order Stage 0 → 1 → 2 → 3 → 4 with a STOP
> after each.

### Stage 0 — Plan (this commit)

**Scope:**

- [x] Draft this document.
- [x] Add it to `docs/README.md` as an index entry under the
      "Testing / runbooks" group (Stage 4 finishes the README
      cross-link; Stage 0 only ships the plan file).

**Exit criteria:**

- This file exists at `docs/AGENT-WORKER-SMOKE-PLAN.md`.
- `./scripts/check.ps1` green (no Go change in Stage 0).

**Commit:** `docs: plan real-cursor agent smoke test`

---

### Stage 1 — Smoke harness + fake-runner self-test

**Scope:**

- [ ] New package `pkgs/agents/agentsmoke` with two files:
  - `agentsmoke.go` — exports `Fixture`:
    - `NewFixture(t *testing.T) *Fixture` — creates a `t.TempDir()`,
      computes `TargetPath` (e.g. `<tempdir>/agent-smoke-output.txt`),
      and pins the canonical `Prompt` and `ExpectedContents`
      constants used by every stage.
    - `(*Fixture).WorkingDir() string`, `(*Fixture).Prompt() string`,
      `(*Fixture).TargetPath() string`,
      `(*Fixture).ExpectedContents() string`.
    - `(*Fixture).AssertSucceeded(t *testing.T)` — reads
      `TargetPath`, fails with a side-by-side diff if contents do not
      match `ExpectedContents`; also asserts that no **other** file
      was created in the tempdir (catches "Cursor wrote helpful
      explanations everywhere").
    - `(*Fixture).AssertNotMutated(t *testing.T)` — used by
      negative tests (Stage 3 failure-path) to assert the runner did
      not touch disk.
  - `agentsmoke_test.go` — exercises the fixture against
    `runnerfake.Runner` whose script writes the expected file via a
    `WithCallback` hook (or via the harness directly so the fake
    doesn't need filesystem access). The test must:
    - Pass when the harness writes the expected contents.
    - Fail (via `t.Run` + a recovered fail) when the harness writes
      wrong contents or extra files.
- [ ] **No real Cursor invocation in Stage 1** — the harness is
      the unit under test here.
- [ ] Doc note in `pkgs/agents/agentsmoke/doc.go` describing the
      package as "shared fixtures for Stages 2–3 of
      `docs/AGENT-WORKER-SMOKE-PLAN.md`".

**Exit criteria:**

- `go test -race ./pkgs/agents/agentsmoke/... -count=1` green.
- `./scripts/check.ps1` green (web unaffected).
- `funclogmeasure -enforce` green for the new package.
- The harness has zero hard dependency on Cursor — Stage 2 wires
  Cursor in, not the harness itself.

**Commit:** `test(agentsmoke): smoke fixture and fake-runner self-test`

---

### Stage 2 — Runner-layer real-cursor smoke (build-tagged + env-gated)

**Scope:**

- [ ] New file `pkgs/agents/runner/cursor/cursor_real_smoke_test.go`
      with `//go:build cursor_real` at the top.
- [ ] Single test `TestCursorAdapter_RealBinary_writesExpectedFile`:
  - `t.Skip` if `os.Getenv("T2A_TEST_REAL_CURSOR") != "1"`.
  - Resolve `cursor-agent` (or override via
    `T2A_AGENT_WORKER_CURSOR_BIN`) and run `cursor.Probe` first; on
    probe error, fail with the same operator-friendly message the
    worker logs on startup.
  - Build a `cursor.Adapter` with default Options + the resolved
    binary path.
  - Build an `agentsmoke.Fixture` (Stage 1).
  - Call `adapter.Run(ctx, runner.Request{...})` with:
    - `TaskID` = a fresh UUID.
    - `Phase = domain.PhaseExecute`.
    - `Prompt = fixture.Prompt()`.
    - `WorkingDir = fixture.WorkingDir()`.
    - `Timeout = 60 * time.Second`.
  - Assertions, in order:
    1. `err == nil`.
    2. `result.Status == domain.PhaseStatusSucceeded`.
    3. `fixture.AssertSucceeded(t)` — the file is on disk with the
       expected bytes.
- [ ] Add a one-paragraph runbook section in
      `pkgs/agents/runner/cursor/cursor_real_smoke_test.go` package
      doc comment explaining how to run:
      `go test -tags=cursor_real -run TestCursorAdapter_RealBinary_writesExpectedFile ./pkgs/agents/runner/cursor/...`
      with `T2A_TEST_REAL_CURSOR=1` and `cursor-agent` on PATH.
- [ ] Append a row to the test inventory table in Stage 4's docs
      sweep (do not edit `AGENT-WORKER.md` in Stage 2; defer to
      Stage 4).

**Exit criteria:**

- `go test ./pkgs/agents/runner/cursor/... -count=1` (no tags) green
  — confirms the new file is correctly tagged-out by default.
- `T2A_TEST_REAL_CURSOR=1 go test -tags=cursor_real -run TestCursorAdapter_RealBinary -race ./pkgs/agents/runner/cursor/... -count=1`
  green when run **locally** with `cursor-agent` on PATH.
- `./scripts/check.ps1` green (smoke skipped in default mode).
- Test wall-clock under 90s on a developer laptop.

**Commit:** `test(cursor): runner-layer real-cursor smoke (opt-in)`

---

### Stage 3 — Full HTTP → worker → real cursor smoke

**Scope:**

- [ ] New file
      `pkgs/tasks/agentreconcile/agent_real_cursor_e2e_test.go` with
      `//go:build cursor_real` at the top. Sibling to the existing
      Stage 6 fake-runner e2e to keep all "real flow" tests in one
      directory.
- [ ] Single test
      `TestAgentE2E_RealCursor_taskFromHTTPReachesDoneWithFileWritten`:
  - `t.Skip` unless `T2A_TEST_REAL_CURSOR=1`.
  - `cursor.Probe` first; skip with a clear message if the binary is
    missing or unusable.
  - Build the **same** stack `cmd/taskapi/run_helpers.go` builds:
    real SQLite store, real `MemoryQueue`, `RunReconcileLoop`, real
    `cursor.Adapter`, real `cycleChangeSSEAdapter` over an
    `httptest.Server`-mounted handler, real worker, real Prometheus
    adapter on a `prometheus.NewPedanticRegistry`. The wiring lives
    in a helper added to the test file (or, if Stage A of the
    broader e2e plan has shipped, reuses that harness).
  - Open an SSE subscription to `GET /events` against the test
    server.
  - `POST /tasks` with `status=ready, initial_prompt=fixture.Prompt(),
    title="real cursor smoke"`. The request's working directory is
    set via `T2A_AGENT_WORKER_WORKING_DIR=fixture.WorkingDir()` on
    the test harness (the worker forwards it to the runner per
    `AGENT-WORKER.md` "Environment variables").
  - Poll the store with `pollTimeout=120s` (Cursor is slower than
    the fake) until `task.status == done`. Fail loudly on
    `task.status == failed` with the cycle's reason and the
    truncated, **redacted** `RawOutput` from the execute phase.
  - Assertions:
    1. Final task `status == done`.
    2. Exactly one cycle, `status = succeeded`, `meta_json.runner ==
       "cursor-cli"`, non-empty `meta_json.runner_version`.
    3. Exactly two phases: `diagnose=skipped`, `execute=succeeded`.
    4. `fixture.AssertSucceeded(t)` — the file is on disk.
    5. SSE stream observed at least one `task_cycle_changed` for the
       new task (sanity check; Stage 6 e2e covers exact counts with
       the fake).
    6. Pedantic registry shows
       `t2a_agent_runs_total{runner="cursor-cli",terminal_status="succeeded"} == 1`
       and one histogram observation.
- [ ] Same opt-in runbook hint in the test file's package doc.

**Exit criteria:**

- `go test ./pkgs/tasks/agentreconcile/... -count=1` (no tags) green.
- `T2A_TEST_REAL_CURSOR=1 go test -tags=cursor_real -run TestAgentE2E_RealCursor -race ./pkgs/tasks/agentreconcile/... -count=1`
  green locally.
- `./scripts/check.ps1` green.
- Test wall-clock under 180s on a developer laptop.

**Commit:** `test(agentreconcile): real-cursor end-to-end smoke from HTTP to done`

---

### Stage 4 — Docs + runbook

**Scope:**

- [ ] Append a **"Smoke run"** subsection under
      [`AGENT-WORKER.md`](./AGENT-WORKER.md) "Operator quick-look"
      explaining:
  - When to run (any change to `pkgs/agents/runner/cursor` or the
    `pkgs/agents/worker` happy path).
  - How to run both stages:
    - Runner-only:
      `T2A_TEST_REAL_CURSOR=1 go test -tags=cursor_real -run TestCursorAdapter_RealBinary ./pkgs/agents/runner/cursor/...`
    - Full flow:
      `T2A_TEST_REAL_CURSOR=1 go test -tags=cursor_real -run TestAgentE2E_RealCursor ./pkgs/tasks/agentreconcile/...`
  - Prerequisites: `cursor-agent` on PATH and Cursor logged in.
  - What to do when the smoke fails (read the redacted RawOutput tail
    the test prints, check Cursor login, check
    `T2A_AGENT_WORKER_WORKING_DIR`).
- [ ] Update `docs/README.md` to link to
      `docs/AGENT-WORKER-SMOKE-PLAN.md` under the testing/runbooks
      section.
- [ ] Tick all stage checkboxes above; update the status table.
- [ ] Append a **"Smoke shipped"** note to `### Notes / followups`
      below.

**Exit criteria:**

- Full `./scripts/check.ps1` green.
- `funclogmeasure -enforce` green.
- All checkboxes in this file checked, status table updated.

**Commit:** `docs(agent-worker): runbook for real-cursor smoke + plan close-out`

---

## Common verification

| Before commit (per stage) | Command |
|---|---|
| Stages 0, 4 (docs-only) | `$env:CHECK_SKIP_WEB='1' ; .\scripts\check.ps1` |
| Stages 1, 2, 3 (Go) | `go vet ./... ; go test ./... -count=1 ; go run ./cmd/funclogmeasure -enforce` |
| Concurrency-touching (3) | also `go test -race ./pkgs/agents/... ./pkgs/tasks/agentreconcile/...` |
| Stage 2/3 with real Cursor (operator-only) | `T2A_TEST_REAL_CURSOR=1 go test -tags=cursor_real -race ./...` |
| Full pass before Stage 4 close-out | `.\scripts\check.ps1` |

`gofmt -w` on touched `*.go` files always.

## What's deliberately deferred (not scope)

- **Multiple prompts / a prompt suite.** One canonical prompt is
  enough to prove the wiring. A library of prompts is V2 of this
  plan once we have a real reason (e.g. "we keep regressing on
  multi-file edits").
- **Long-running soak / load tests.** Out of scope; covered by a
  separate perf plan if needed.
- **Cross-runner conformance tests.** Only Cursor exists today; new
  adapters add their own smoke variants by reusing
  `pkgs/agents/agentsmoke`.
- **Sandboxing the smoke's working directory beyond `t.TempDir()`.**
  Cursor's whole job is to modify files; we trust `t.TempDir()` to
  contain blast radius. Per-cycle isolation lives in
  `AGENT-WORKER-PLAN.md` "Notes / followups" and is V2 of
  `AGENTIC-LAYER-PLAN.md`.
- **Wiring the smoke into CI.** CI does not have Cursor CLI; the
  smoke is an operator-run gate before merging changes that touch
  the cursor adapter or the worker happy path. Auto-running it
  would require a CI image with Cursor + a service account with
  Cursor login state, which is V3 of `AGENTIC-LAYER-PLAN.md`.

## Notes / followups

(Populated as stages discover incidental work — keep this section as
the catch-all so individual stages stay scoped.)

## Status

| Stage | State | Commit |
|---|---|---|
| 0 — Plan | done | _backfilled in this commit_ |
| 1 — Smoke harness + fake-runner self-test | pending | — |
| 2 — Runner-layer real-cursor smoke | pending | — |
| 3 — Full HTTP → worker → real cursor smoke | pending | — |
| 4 — Docs + runbook | pending | — |
