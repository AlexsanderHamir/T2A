# T2A — product context

This document states who T2A is for, what outcomes we optimize for, and how we decide scope. It complements `docs/DESIGN.md` (contracts and behavior) and should be updated when strategy shifts—not on every PR.

## Status

Living. Direction: a dedicated place for task delegation that treats terminals and CLIs as first-class, not the code editor as the only cockpit.

## Vision

Modern AI CLIs (Cursor, Claude, OpenAI, and similar) already match or rival in-editor AI for many workflows. It no longer follows that engineers should live inside an editor to delegate and track work. T2A exists to make **task delegation** a first-class concern: durable tasks, a real audit trail, observable behavior, and **patterns** you can repeat—so humans, scripts, and agent runners share one system instead of ad-hoc chat, tickets, or hidden state.

We do not think today’s default shapes of delegation are good enough. T2A is a bet that **strong observability** (what changed, who acted, when) plus **explicit contracts** (API, events, persistence) beat informal handoffs.

## Primary user

Engineers and small teams who delegate work to agents and to each other from **terminals, runners, and APIs** as often as from a UI. They want one store and history that CLIs, automation, and an optional browser can all use without re-implementing state or guessing what happened.

## Job to be done

When I delegate work to agents and people from the tools I already use (including CLIs), I want a single observable system of record with clear patterns, so coordination does not depend on staying in an editor or losing context in chat threads.

## Positioning (internal)

For engineers who delegate via CLI, scripts, and agents as much as via UI  
Who are stuck with fragmented state, weak observability, or “delegation = editor session”  
T2A is a task system with REST, append-only audit, and live hints (`GET /events`)  
So delegation is durable, inspectable, and syncable without blind polling  
Unlike chat-in-the-editor or generic PM tools as the default coordination layer  
We optimize for observability, contracts, and repeatable patterns at the integration layer (`docs/DESIGN.md`)

## North star (value)

Delegation is trustworthy: anyone (human or tool) can see task state, history, and who did what, without the editor as a gate.

Proxy signals we can observe without a product analytics stack:

- Time from “something changed” to “client shows current row” stays low (SSE + refetch pattern).
- Audit answers “what happened and who acted” without spelunking logs or replaying a chat.
- New behavior lands as a vertical slice (domain → store → handler → optional web) with tests, so patterns stay consistent in code, not only in docs.

## Priorities (outcome framing)

| Horizon | Intent |
|--------|--------|
| Now | Core loop: correct writes, reliable audit, SSE invalidation, docs that match behavior—the foundation observability and CLIs rely on. |
| Next | Smoother path for CLI and runner authors: onboarding, env and `REPO_ROOT` clarity, fewer footguns when optional pieces are off. |
| Later | Bets that need evidence: multi-replica notify, stronger auth, migration story beyond `AutoMigrate`—only after real delegation pain is documented. |

Features belong in specs and PRs; this table is direction, not a commitment calendar.

## What we are not optimizing for (today)

- Replacing Jira, Linear, or full PM suites as the product definition.
- Multi-tenant SaaS auth and billing inside `taskapi`.
- Guaranteed cross-replica SSE without external infra (see `docs/DESIGN.md` Limitations).

Saying no to those keeps scope honest; see Out of scope in `DESIGN.md` for the technical list.

## How we decide the next change

1. Is this solving a problem we have observed (runbook pain, missing contract, confusion in issues), or one we imagined?
2. Does it strengthen trustworthy delegation and observability, or only add surface area?
3. What is the smallest slice that tests the hypothesis, and what would we measure or observe to know it worked?
4. What do we defer if we say yes?

If those answers are weak, prefer documentation, tests, or operability over new features.

## Related docs

| Doc | Role |
|-----|------|
| [DESIGN.md](./DESIGN.md) | HTTP, SSE, persistence, limits, extensibility |
| [WEB.md](./WEB.md) | Optional SPA behavior |
| [README.md](../README.md) | Commands and setup |
| [AGENTS.md](../AGENTS.md) | Repo map for contributors and agents |
