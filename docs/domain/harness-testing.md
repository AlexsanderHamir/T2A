# Harness testing

How to write and run tests under `pkgs/agents/harness/`. Behavioral contracts live in [harness.md](harness.md); this doc covers **test tiers**, fakes, and verification commands.

## Three tiers

| Tier | Location | Dependencies | Purpose |
| --- | --- | --- | --- |
| **Pure** | `internal/orchestration/`, `internal/reports/`, `internal/prompt/`, `invariant_test.go`, `meta_test.go`, `recovery_test.go` | None (stdlib + domain types) | FSM decisions, prompt golden text, meta normalization |
| **Contract** | Root `*_test.go`, most `internal/*` integration tests | `harness.Store` via [`storefake`](../../pkgs/agents/harness/storefake/), `runnerfake`, `notifierfake`, `metricsfake` | Cycle loop, verify retry, resume, cancel — call `harness.Run` directly, not `worker.Worker` |
| **Wrapper** | `internal/git/*_test.go` | Real `git` binary in `t.TempDir()` + `storefake` for store I/O | Git wrapper is the system under test |

**Rule:** No `*_test.go` under `pkgs/agents/harness/` may import `internal/tasktestdb` directly. Use `storefake.New(t)` so SQLite stays behind `contract.Store`. `import_guard_test.go` enforces this for the root package.

## Direct harness.Run recipe (contract tier)

1. `sf := storefake.New(t)` (or `newHarnessWithFakes` in `testhelpers_test.go` for `harness_test` package).
2. Create a task (`StatusReady`), add checklist items as needed.
3. Transition to `StatusRunning` before `Run` — the harness expects an admitted task.
4. `harness.New(sf, runner, opts)` then `Run(ctx, task)` (sync or goroutine + `<-done`).
5. Assert on store rows, notifier recordings, or metrics fakes — not on worker queue pickup.

Do **not** spin up `worker.Worker.Run` in harness tests. Worker admission (git bindings, queue) is tested in `pkgs/agents/worker/` and agentreconcile E2E tests.

## Verify phase integration files

After the verify-phase split, each file maps to edge cases in [harness.md § Verify retry edge cases](harness.md):

| File | Edge cases |
| --- | --- |
| `verify_phase_full_reexecute_test.go` | EC-02–EC-06 style full re-execute paths |
| `verify_phase_infra_retry_test.go` | EC-01, EC-09 (carry passes, verify-only retry) |
| `verify_phase_terminal_test.go` | EC-07 tamper, EC-08 budget exhausted |
| `verify_phase_progress_test.go` | Progress events under verify `phase_seq` |
| `verify_phase_separate_runner_test.go` | `Options.VerifyRunner` routing |
| `verify_phase_integrity_test.go` | Repo cleanliness, report-dir GC |
| `verify_phase_helpers_test.go` | Shared runners and report writers (no tests) |

## Local verification

```powershell
# Same as CI harness group (includes coverage gate)
.\scripts\check-go.ps1 -TestsOnly -Group harness -Verbose

# Full Go bar
.\scripts\check.ps1 -GoOnly
```

See [testing.md](testing.md) for all groups and the verification ladder.

## Store interface

Production code depends on `harness.Store` (`contract.Store`). `*store.Store` and `storefake.Fake` both satisfy it. When adding a store call from harness code, extend the interface in `internal/contract/store.go` and update `store_assert_test.go`.
