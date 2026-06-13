# ADR-0011: Remove Task Type and DMAP

**Date:** 2026-06-13
**Status:** Accepted
**Deciders:** T2A maintainers

## Context

Tasks carried a `task_type` enum (`general`, `bug_fix`, `feature`, `refactor`, `docs`, and a UI-only `dmap` variant) from early product exploration. The field was optional metadata: it did not drive worker admission, harness behavior, or evaluation scoring. Operators still had to pick a type in create/edit flows, which added clutter to the essentials row without improving outcomes.

The DMAP create path layered commit-limit, domain, and description fields on top of `task_type === "dmap"`, synthesizing a prompt at submit time. That workflow duplicated what the rich prompt editor already supports and was rarely used compared to standard task creation.

## Decision

1. **Remove `task_type` end-to-end.** Delete the domain enum, GORM field, Postgres/SQLite column (`migrateRemoveTaskType`), store create/patch inputs, handler JSON, and eval draft input.
2. **Remove DMAP UI and draft helpers.** Delete `TaskTypeSelect`, the create-modal DMAP section, `dmapPrompt` / `toApiTaskType`, and related autosave signature fields. The create modal essentials row is title + priority only.
3. **Keep test scenarios.** `TestScenariosTrigger` and the scenario catalog remain; scenarios no longer carry a `taskType` field.
4. **Breaking JSON change.** `task_type` is absent from `GET/POST/PATCH /tasks` and draft evaluation requests. Unknown keys in inbound JSON are ignored by Go unmarshaling; legacy draft blobs may still contain `task_type` / `dmap_config` — the web draft parser tolerates but does not persist them.

## Consequences

### Positive

- Simpler create/edit surfaces and fewer form fields to validate.
- Smaller API contract and store validation surface.
- One less enum to keep in sync across Go, TypeScript, docs, and tests.

### Negative / Trade-offs

- Breaking change for any external client that relied on `task_type` in task JSON.
- Historical eval `input_json` and old drafts may still mention `task_type`; no blob migration is performed.
- Operators who used DMAP must enter structured prompts directly in the rich editor.

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| Keep `task_type` as read-only metadata | Still requires schema, docs, and UI maintenance for unused data |
| Keep DMAP, drop other types | DMAP was the only special-case branch; removing the enum removes the need for the branch |
| Hide task type behind advanced options | Field still persisted and documented without product benefit |
