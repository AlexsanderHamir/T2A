# Cycle commit tracking

How the worker indexes git commits per task from agent claims, and feeds verify, resume, and the commits API.

| | |
| --- | --- |
| **Applies to** | Agent harness execute/verify phases, `task_cycle_commits`, commits API |
| **Audience** | Contributors touching `pkgs/agents/harness`, cycle store, or cycle detail UI |
| **Prerequisite** | [execute-agent.md](./execute-agent.md) — execute prompt and criteria self-report |
| **Decision record** | [ADR-0014](../adr/ADR-0014-cycle-commit-tracking.md), [ADR-0032](../adr/ADR-0032-agent-claimed-commit-index.md) (supersedes [ADR-0016](../adr/ADR-0016-observe-vs-admit-commits.md)) |

## Overview

When `app_settings.repo_root` points at a git worktree, the execute agent declares commits in `criteria-report.json` under `commits[]`. After a successful runner exit, the worker validates each SHA (`git cat-file`, `git log -1`) and upserts rows into `task_cycle_commits`. **Execute never fails on commit hygiene** — only runner errors, cancel, or git/store I/O errors block the cycle.

Verify reads **all commits indexed for the task** via `ListCommitsForTask(task_id)`.

> **Note** — Non-git working directories skip snapshot, ingest, and commit indexing entirely (`git.skipped` in phase details).

## Wire contract

### criteria-report.json (execute)

```json
{
  "schema_version": 1,
  "criteria": [{ "id": "...", "claimed_done": true, "evidence": "..." }],
  "commits": [{ "sha": "<full-or-abbrev>", "branch": "optional" }]
}
```

- List commits **created in this execute visit** (incremental is fine — the DB accumulates).
- **Additive-only:** create new commits only; never amend, rebase, squash, or delete history.

### Ingest (worker)

1. Read `commits[]` via `ParseCriteriaReportCommits`.
2. Per SHA: `cat-file -e`, `git log -1`, optional in-range warn vs `cycle_base_sha..HEAD`.
3. Upsert on `(cycle_id, sha)` — append-only; never delete or supersede rows.
4. Empty `commits[]` → no new rows; execute continues to verify.

### Verify prompt

`ListCommitsForTask(task_id)` → `FormatGitContextForPrompt` — full task ledger across cycles/attempts.

### HTTP

- `GET /tasks/{id}/commits` — task-wide deduped by SHA (earliest `committed_at` wins).
- `GET /tasks/{id}/cycles/{cycleId}/verdicts` — per-cycle commit rows (no `status` / `gate_reason`).

## See also

- [execute-agent.md](./execute-agent.md)
- [verify-agent.md](./verify-agent.md)
- [data-model.md](../data-model.md) — `task_cycle_commits` schema
