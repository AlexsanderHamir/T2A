# ADR-0013: Prompt automations library and harness injection

**Date:** 2026-06-15  
**Status:** Superseded (2026-06-20) — prompt automations removed; see commit removing `/automations`, `automation_selections`, and harness injection.  
**Deciders:** T2A maintainers

## Context

Operators repeat the same behavioral instructions across tasks. `initial_prompt` is free-form only; there is no global reusable library, no structured Yes/No/Omit semantics, and no runtime injection path comparable to checklist criteria or project context.

## Decision

Introduce a **global automations library** (`automations` table) and per-task **`automation_selections`** JSON on `tasks`. The SPA manages the library on `/automations` and exposes Yes / Omit / No toggles during task create. **Omit** is represented by absence from the array; only `yes` and `no` are persisted.

At agent run time, `pkgs/agents/harness` resolves library rows by ID and injects a `## Agent behaviors` section into the composed execute prompt:

- **Yes:** `- [YES] {title}: {description}`
- **No:** `- [NO] {title}: Do NOT {description}`

Descriptions are validated at library write time to avoid leading with "Do not" (double-negation in the No line). Archived or missing library rows referenced by a task are skipped with a structured warn log; the cycle continues.

`GET /v1/bootstrap` includes the automation catalog so the create modal avoids an extra round trip.

## Consequences

### Positive

- Deterministic behavior toggles without polluting rich-text prompts
- Editable task bindings via PATCH without re-parsing HTML
- Stable audit via existing composed `prompt_hash` metadata

### Negative / Trade-offs

- Library text is resolved at compose time (not snapshotted on the task) — editing a library row affects future runs
- Task detail edit UI for automations deferred; PATCH field exists for follow-up

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| Bake toggles into `initial_prompt` | Loses structured Yes/No/Omit, harder to PATCH and audit |
| Per-project automation libraries | Extra scope; global catalog matches first use case |
| Snapshot description on task | Stale copy when library improves; compose-time resolve is simpler |
