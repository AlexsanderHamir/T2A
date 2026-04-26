# ADR-0001: Project Context In Taskapi

**Date:** 2026-04-26
**Status:** Accepted
**Deciders:** T2A maintainers

## Context

T2A currently models durable work as tasks, execution cycles, cycle phases, and
append-only task events. That works for one-off work, but some initiatives span
many tasks and months. If all continuity lives inside individual task prompts or
chat transcripts, agents lose the shared memory needed to keep making progress.

The product needs a first-class project context layer: a place where humans and
agents can maintain durable facts, decisions, constraints, and handoff notes
across many tasks.

## Decision

Projects will be implemented inside the existing `taskapi` boundary, using the
same Go/GORM/Postgres store, REST contracts, SSE hints, and SPA architecture as
tasks.

A project is not a task parent. Tasks keep their existing subtask tree through
`parent_id`, while project membership is represented separately through
`project_id`.

Project context is canonical at the project level. When an agent runs a
project-linked task, the worker persists a cycle-scoped snapshot of the exact
project context bundle passed to the runner before invoking it.

The first version will not introduce embeddings, a vector database, hidden
memory, or a separate memory service.

## Consequences

### Positive

- Long-running work gets a durable, inspectable context surface.
- Existing task, cycle, event, and worker patterns continue to apply.
- Projectless tasks remain valid and unchanged.
- Historical runs stay reproducible because each run records the context it
  actually received.
- The implementation avoids introducing a second persistence technology before
  the product loop proves it needs one.

### Negative / Trade-offs

- Relational context selection is less powerful than semantic retrieval.
- Prompt bundles must be bounded manually by store and worker logic.
- Project context and task-local snapshots introduce another data boundary that
  UI and docs must explain clearly.
- `pkgs/tasks` grows to include a broader work-management domain, so file
  structure and docs must stay disciplined.

## Alternatives Considered

| Alternative | Reason Rejected |
| --- | --- |
| Use parent tasks as projects | Subtasks already mean task decomposition. Overloading `parent_id` would make project membership and task ownership ambiguous. |
| Add a vector database immediately | It adds operational complexity before T2A has a validated project-memory workflow. |
| Store project context only in task prompts | Prompts are execution-local and hard to audit across months of work. They do not provide a shared, editable memory surface. |
| Create a separate project service | The current product is a single-process control plane. A second service would violate the current simplicity and deployment constraints. |
