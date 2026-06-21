# T2A

Control plane for coding agents. Coordinates Cursor CLI, Claude Code, Codex, and other agentic systems.

![T2A task board — structured tasks with status, priority, and acceptance criteria](assets/product_example.png)

#### Why T2A Exists

To address the following problems and more:

1. Chat-based interfaces introduce too much variability into software engineering.
2. Developers already coordinate multiple AI agents manually. T2A makes that workflow explicit and repeatable.
3. Existing project management tools were designed for human teams, not AI-driven software engineering.

---

## Initial Features

**Task Templates** —> Define a task once, instantiate it many times.

**Execute & Verify** —> One agent executes the task, another independently verifies the acceptance criteria.

**Execution History** —> Every execution is recorded with commits, verification results, and an audit trail.

**Acceptance Criteria** —> Define what "done" means with checklists and optional verification commands.

**Runner Adapters** —> Run T2A with different agentic systems.

---

## Get started

**Requirements:** Go 1.25+, Node 20+, and `DATABASE_URL` in a repo-root `.env`.

1. Create a `.env` file and set `DATABASE_URL`.
2. Apply the schema: `go run ./cmd/dbcheck -migrate`
3. Start the API and web UI:

```bash
./scripts/dev.sh        # Unix — chmod +x once if needed
.\scripts\dev.ps1       # Windows
```

API at `http://127.0.0.1:8080` · Web at `http://localhost:5173`. Ctrl+C stops both.

4. Verify your setup: `./scripts/check.sh` or `.\scripts\check.ps1` (add `--install` / `-Install` on first run)

Contributing? See [CONTRIBUTING.md](CONTRIBUTING.md). Agent and workspace settings are in the web UI at `/settings` — see [docs/configuration.md](docs/configuration.md).

## Before you run tasks

Read [docs/execute-and-verify.md](docs/execute-and-verify.md) before creating tasks or writing done criteria.

- Every task runs an **execute** agent and a **verify** agent.
- The worker runs **one task at a time** — you can queue many, but they run sequentially.
- Do not edit, commit, or checkout files in the workspace repo during **verify**. Git changes there end the cycle as `verify_tampered` (no retry).
- Point **Workspace repository** at a **dedicated git worktree** so you can keep working in your main checkout — [details](docs/execute-and-verify.md#dedicated-worktree-recommended).

## Docs

- [docs/guide.md](docs/guide.md) — how documentation fits together
- [CONTRIBUTING.md](CONTRIBUTING.md) — setup and PR checklist
- [AGENTS.md](AGENTS.md) — code paths when editing the repo

## License

[MIT](LICENSE)
