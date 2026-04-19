# Proposals

Forward-looking design docs for features that have **not yet shipped**.

This folder is for the **what** and the **why** of a proposed feature — the
problem statement, the constraint it removes, the design tradeoffs, and the
implementation boundary (in-scope / out-of-scope). Once a proposal is
accepted and execution begins, the contract should move into a top-level
`docs/<FEATURE>.md` doc, and the per-stage execution playbook (if any) lives
alongside it as `docs/<FEATURE>-PLAN.md`. After the feature ships and the
execution playbook is complete, the `*-PLAN.md` is **deleted** and the
contract doc stays.

## What goes here

- **Proposals** for new product capabilities (one file per proposal).
- **Design RFCs** for cross-cutting changes (data model, security model,
  agent loop changes) that need discussion before implementation.

## What does NOT go here

- Contracts for already-shipped behavior — those live in `docs/` directly
  (e.g. `docs/AGENT-WORKER.md`, `docs/EXECUTION-CYCLES.md`).
- Per-stage execution playbooks for in-flight work — those live in
  `docs/<FEATURE>-PLAN.md` next to the contract doc.
- Long-term version roadmaps — those live in `docs/<AREA>-LAYER-PLAN.md`
  (e.g. `docs/AGENTIC-LAYER-PLAN.md`).

## Suggested file shape

One Markdown file per proposal, named after the feature
(`VERIFY-PHASE.md`, `MULTI-RUNNER-ROUTING.md`, …). Keep the structure tight:

1. **Feature name**
2. **Problem** (single root constraint being removed)
3. **Why this is the next feature** (working-backwards rationale)
4. **System impact** (loop / coordination / reliability / decision quality)
5. **Implementation boundary** (in-scope / out-of-scope)

Once a proposal is accepted, link it from `docs/README.md` so contributors
can find it without grepping.
