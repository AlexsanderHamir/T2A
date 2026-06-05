# ADR-0004: Verdicts on the database

**Date:** 2026-06-05
**Status:** Accepted
**Deciders:** Backend / agents-worker maintainers
**Supersedes:** none — extends ADR-0003 ("Verify component upgrade").

## Context

ADR-0003 made the verify pass adversarial, retry-efficient, and
observable, but it did not change the **shape** of the agent ↔ worker
communication. The execute agent writes `criteria-report.json` and the
verify agent writes `verify-report.json`; the worker parses both,
makes a decision, and the files are GC'd at cycle terminate. Two real
gaps remain:

1. **Customer-repo pollution.** Until PR1 of this plan, both files
   lived under `app_settings.repo_root/.t2a/<cycleID>/`. Operators saw
   `.t2a` directories in their working tree and asked "what is this?".
   In an AI-infra deployment shape that pollution is a non-starter.
2. **Evidence is non-durable.** The decision survives in
   `task_checklist_completions` (terminal pass-rows only) and in the
   `verification_failed:<id>,<id>,…` `terminate_reason`, but the
   verifier's *reasoning* and the agent's *self-claimed evidence* live
   only in the JSON files, which are deleted at cycle terminate. Support
   has no way to answer "why did attempt 2 reject criterion X?" hours
   later. Cross-cycle analytics (prompt tuning, false-positive review)
   are impossible.

The problem is small in absolute terms — a few KB per cycle — but the
positioning gap is large. AI-infra customers expect verdict rationale
to be queryable by URL, not by ssh-into-the-box-and-grep.

The staff-engineering call: smallest move that closes both gaps today
and does not foreclose moving the agent off shared filesystem in the
future. Two PRs, each independently valuable.

## Decision

### PR-1 — Files leave `RepoRoot`

Report files now live in a worker-managed scratch directory
(`T2A_WORKER_REPORT_DIR`, default `<os.TempDir()>/t2a-worker`). The
agent prompt receives the absolute path; the worker reads/parses then
GC's the cycle subdirectory at terminate. The pre/post integrity-check
allowlist is now empty — *any* change inside `RepoRoot` during the
verify pass is tampering, since the report file no longer needs to be
exempted.

### PR-2 — Mirror to two normalized tables

Two new tables — `task_cycle_criteria_reports` and
`task_cycle_verify_reports` — one row per criterion per attempt,
keyed `(cycle_id, attempt_seq, criterion_id)`. The worker bulk-upserts
each parsed report at the verify-phase boundary using
`ON CONFLICT … DO UPDATE` so a transient store error during persist is
safe to retry. The handler exposes both arrays via
`GET /tasks/{id}/cycles/{cycleId}/verdicts`; the SPA renders verifier
reasoning per criterion in the cycle row.

Cascade semantics deliberately split:

- `cycle_id` cascades on delete (verdicts disappear with their cycle).
- `criterion_id` is `ON DELETE NO ACTION` (verdict history survives a
  checklist edit; the SPA renders the criterion id verbatim if the FK
  is stale).

The wire format (the JSON files) is unchanged. The mirror is additive.

## Why option B (worker-mediated DB persistence)

Three options were considered:

- **A — HTTP-from-agent direct to taskapi.** Removes the shared-FS
  assumption permanently, lets the agent write verdicts directly to
  the API. Costs: requires an authenticated, idempotent ingest
  endpoint; needs a retry/backpressure policy on the agent side; and
  forces a contract between the agent CLI and `taskapi` that we'd have
  to version. **Premature** at the current single-host deployment
  shape — the worker already runs next to the agent and already parses
  the files. Adopting A now without the forcing function (multiple
  hosts) would introduce its own gaps (auth, retry semantics) we'd
  have to design before we have evidence we'll ever need them.

- **C — Stream-extracted verdicts from the runner's stream-json.**
  Couples the worker to runner-internal output formats permanently.
  Cursor's `--output-format stream-json` is not a contract; it has
  changed across CLI versions. Negative leverage: every Cursor-side
  change risks breaking the verdict pipeline.

- **B — Worker-mediated DB persistence (this ADR).** Worker already
  parses both files for the verify decision. Adding two upserts at the
  same boundary is small, reversible, and lands the durable record
  immediately. When the forcing function for A appears (N agents per
  host, or N hosts), the worker's `UpsertCriteriaReports` /
  `UpsertVerifyReports` becomes the single seam to swap for an HTTP
  ingest adapter — option A becomes a one-PR adapter change, not a
  re-architecture. **Picked.**

The schema is normalized rather than blob — per-criterion query is
the read pattern that matters (SPA per-criterion timeline, support
"why did X fail?", prompt tuning per criterion across cycles). A
single JSON-blob column would have to be rehydrated at every read.

## Consequences

### Positive

- Customer working trees stay clean (PR-1).
- Verdict evidence is durable and queryable by URL (PR-2). Support
  can pull verifier reasoning hours after a cycle terminates without
  ssh.
- The verify-phase retry timeline is renderable: every attempt's
  verdict survives, including the rejected attempt that triggered the
  retry. Previously only the in-memory `previouslyPassed` lock had
  visibility into retry-vs-fresh-pass.
- Disk pressure is bounded: PR-1 GCs the per-cycle scratch dir at
  terminate, so report file disk usage is O(running cycles) rather
  than O(all cycles ever).
- Idempotent persist: a transient store error during the verify
  phase is safe to retry without producing duplicate or shifted
  rows. The composite unique index does the work.

### Negative / Trade-offs

- Two new tables to migrate. AutoMigrate handles it; pre-existing
  cycles return zero rows (the handler returns empty arrays, not
  404). No data migration is performed.
- Worker now does two extra writes per verify phase. Bulk insert keeps
  it to one round trip per parsed report; the additional latency at
  current scale is well below the verify-phase end-to-end duration.
- `verifier_kind` is now denormalized (also lives on
  `task_checklist_completions.verified_by`). The SPA reads from both
  surfaces — the verdict table for per-attempt evidence, the
  completion table for the terminal pass-row. If we ever need to
  rename a `VerifierKind` value, both tables (and the SPA chips) must
  move in lockstep. Acceptable at five values.
- The agent ↔ worker file contract still exists. PR-2 does not remove
  it; option A is what removes it.

## Re-evaluation triggers

Reopen this design if any of:

1. **N workers per host or N hosts.** Forces a move to option A
   (HTTP-from-agent) so the agent doesn't need to know which host's
   filesystem to write to. The worker's two upsert seams become the
   adapter swap.
2. **Sustained verify-report size > 100 KB on average.** Today's
   16 KB per-field cap is fine; if reasoning routinely runs to
   chapters, consider streaming or compressing the columns.
3. **Verdict-write throughput > ~1000 cycles/sec on a single Postgres.**
   At that scale we have a much better problem and should profile
   before redesigning.
4. **Customer support flow needs partial-progress visibility before
   the verify phase ends.** Today reports are produced once at end of
   phase; if support needs partial verdicts mid-phase, consider
   streaming verdict events alongside `agent_run_progress`.

## Out of scope (deferred, explicitly)

- Multi-tenant row-level security / data residency tagging on the
  verdict tables. No tenant model exists yet; designing without
  constraints is wasted work.
- Retention / archival policies. At projected scale (tens of thousands
  of rows, KB-scale per row) this is years away from being a problem.
- One-shot cleanup of legacy `<repo>/.t2a/` directories from operator
  repos. PR-1 stops creating them; an operator's `rm -rf .t2a/` is a
  one-time hygiene step we don't need to ship.
- Real-time verdict streaming via SSE. Reports are produced once at
  end of phase; nothing to stream.

## Alternatives considered

| Alternative | Reason rejected |
|---|---|
| A — HTTP-from-agent direct to `taskapi` | Premature without a forcing function (single host today). Worker-mediated path keeps the same DB outcome and leaves A as a one-PR adapter swap when justified. |
| C — Stream-extracted verdicts from runner stream-json | Couples worker to runner-internal output. Cursor stream-json is not a contract; bad leverage. |
| JSON-blob column variant | Loses per-criterion query patterns we will use for support, prompt tuning, and cross-cycle analytics. |
