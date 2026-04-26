# Project context

Project context is T2A's continuity primitive for work that spans many tasks,
cycles, and weeks of agent execution.

## Mental model

Think of a project like a process and tasks like threads:

- the project is the shared memory space for a long-running body of work;
- tasks and subtasks can read from that shared memory, but they do not own it;
- a task run receives an immutable snapshot of the project context that was
  passed to that run;
- task-local notes do not become canonical project memory unless they are
  explicitly promoted or appended.

This keeps the system explainable. A project can evolve over time, while each
task execution remains reviewable against the exact context it used.

## Why this exists

Not every task is a one-off. Some initiatives span months, and tying all
working memory to individual chats or tasks makes progress harder over time.
Project context gives agents and humans one durable, inspectable memory surface
without turning every prompt into a growing transcript.

The first version is intentionally relational and explicit:

- project metadata describes the long-running initiative;
- curated context items hold facts, decisions, constraints, and handoff notes;
- tasks can optionally belong to a project while preserving their existing
  parent/subtask tree;
- task cycles snapshot the context bundle they used before invoking a runner.

## Ownership rules

Projects and tasks answer different questions:

- `project_id` answers "which long-running body of work does this task draw
  shared context from?"
- `parent_id` answers "which task owns this subtask in the task breakdown?"

A task may have both. A project is not a task parent, and subtasks are not
project children.

Canonical context lives at the project level. A task or cycle may hold a copy
of context for audit and reproducibility, but edits to that copy do not mutate
project memory.

## Initial implementation boundary

In scope:

- first-class projects;
- project context items;
- optional task membership in a project;
- cycle-scoped context snapshots for agent runs;
- REST, SSE hints, and SPA views for the same data;
- bounded prompt injection in the existing in-process worker.

Out of scope for the first version:

- embeddings or vector search;
- hidden autonomous memory pruning;
- background summarization daemons;
- permissions, sharing, billing, or tenancy;
- automatic migration of existing tasks into synthetic projects.

The guiding rule is: context should be durable, visible, and editable before it
becomes clever.
