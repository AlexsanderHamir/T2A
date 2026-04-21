# Scheduling, agents, and replicas

This document expands on risks called out briefly in [SCHEDULING.md](../SCHEDULING.md) and [AGENT-QUEUE.md](../AGENT-QUEUE.md). It is **not** a commitment to implement any item below.

## Multi-replica / horizontal `taskapi`

The in-process **`MemoryQueue`** and **`PickupWakeScheduler`** are **single-process**. They are not shared across machines or even across multiple `taskapi` processes attached to the same database.

**Deployment assumption today:** one long-lived `taskapi` instance per database that runs the in-process agent queue (or equivalent: only one instance may run the consumer that drains ready work).

**If you scale out HTTP horizontally:**

- Multiple instances may still **serve REST** safely, but **only one** should run the ready-task consumer + pickup wake, **or**
- You need **distributed coordination** so exactly one node claims a task at a time: database row locks (`SELECT … FOR UPDATE SKIP LOCKED`), advisory locks, a lease column, or an external job broker (Redis, SQS, etc.).

The **reconcile loop** helps recover missed work but does **not** replace cross-replica mutual exclusion.

## Clock skew (app vs database)

`PickupWakeScheduler` uses the **process clock** (`time.Now`) for timers and eligibility checks. SQL eligibility in `ListQueueCandidates` uses the database’s **`now()`** at query time.

**Mitigation in production:** keep **NTP** on application hosts and the database (standard).

**If skew causes visible issues:** optional enhancements include sampling **`SELECT NOW()`** once per wake batch (heavier) or relying on a single source of truth for “current time” in the application layer. The fixed **reconcile** tick still bounds worst-case delay.

## Neutral package for scheduling interfaces

Today, **`store.PickupWake`** is defined in **`pkgs/tasks/store`** and implemented in **`pkgs/agents`**. That avoids import cycles.

**If** the dependency graph grows (multiple implementations, multiple consumers) or **if** a cycle appears, **extract** a small neutral package (e.g. `pkgs/taskschedule` or similar) holding only interfaces and wire implementations from `cmd/taskapi`. Document any migration in this folder.
