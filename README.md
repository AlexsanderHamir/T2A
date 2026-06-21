<p align="center">
  <img src="assets/Hamix_wordmark.png" alt="Hamix" width="420" />
</p>

Control plane for coding agents. Coordinates Cursor CLI, Claude Code, Codex, and other agentic systems.

![T2A task board — structured tasks with status, priority, and acceptance criteria](assets/product_example.png)

## Why T2A Exists

To address the following problems and more:

1. Chat-based interfaces introduce too much variability into software engineering.
2. Developers already coordinate multiple AI agents manually. T2A makes that workflow explicit and repeatable.
3. Existing project management tools were designed for human teams, not AI-driven software engineering.

## Initial Features

**Task Templates** —> Define a task once, instantiate it many times.

**Execute & Verify** —> One agent executes the task, another independently verifies the acceptance criteria.

**Execution History** —> Every execution is recorded with commits, verification results, and an audit trail.

**Acceptance Criteria** —> Define what "done" means with checklists and optional verification commands.

**Runner Adapters** —> Run T2A with different agentic systems.

## Get started

**Requirements:** Go 1.25+ | Node 20+

1. Create a `.env` file and set `DATABASE_URL`.
2. Apply the schema: `go run ./cmd/dbcheck -migrate`
3. Start the API and web UI:

```bash
./scripts/dev.sh        # Unix — chmod +x once if needed
.\scripts\dev.ps1       # Windows

# Result 

API at `http://127.0.0.1:8080`
Web at `http://localhost:5173`
Ctrl+C stops both.
```

4. Optional (when changing code): Run the same checks as CI before opening a PR.

```bash
./scripts/check.sh
.\scripts\check.ps1

# Result

T2A check (Go)

[1/5] gofmt                  ok 6s
[2/5] go vet                 ok 8s
[3/5] scheduling boundary    ok 0s
[4/5] go test                ok 19s  (65 packages)
[5/5] funclogmeasure         ok 2s
check OK  5/5 passed  35s

T2A check (web)

[1/4] web test               ok 22s
[2/4] web lint               ok 5s  (4 warnings)
[3/4] web standards          ok 1s
[4/4] web build              ok 6s
check OK  4/4 passed  33s

```

## Before You Run Tasks

The current implementation has a few important limitations:

* Every task is executed by one AI agent and independently verified by another.
* Tasks run sequentially. You can queue multiple tasks, but only one runs at a time.
* Use a dedicated Git worktree as the workspace so you can continue working in your main checkout.
* Do not edit the workspace while verification is running. Any Git changes will invalidate the verification attempt.

See [docs/execute-and-verify.md](docs/execute-and-verify.md) for details.

## Docs

- [docs/guide.md](docs/guide.md) — how documentation fits together
- [CONTRIBUTING.md](CONTRIBUTING.md) — setup and PR checklist
- [AGENTS.md](AGENTS.md) — code paths when editing the repo

## License

[MIT](LICENSE)
