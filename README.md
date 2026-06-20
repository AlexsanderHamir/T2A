# T2A

The control plane for AI-driven software development.

Think of T2A as the equivalent of [n8n](https://n8n.io) for engineering work. While n8n orchestrates business workflows across SaaS applications, T2A orchestrates the dependencies between engineering tasks.

Human involvement happens where it creates the most value: defining goals, reviewing outcomes, and making key decisions. Everything else can be coordinated and executed automatically.

---

## Get started

**Requirements:** Go 1.25+, Node 20+ (npm/npx included), and `DATABASE_URL` in a repo-root `.env`.

1. Create a repo-root `.env` with `DATABASE_URL` set
2. Apply the schema: `go run ./cmd/dbcheck -migrate`
3. Start the API and web UI:

```bash
./scripts/dev.sh        # Unix — chmod +x once if needed
.\scripts\dev.ps1       # Windows
```

API at `http://127.0.0.1:8080` · Web at `http://localhost:5173`. Ctrl+C stops both.

4. Verify your setup from the repo root:

```bash
./scripts/check.sh      # Unix — ./scripts/check.sh --help for flags
.\scripts\check.ps1     # Windows — .\scripts\check.ps1 -Help for flags
```

Setting up to contribute? See [CONTRIBUTING.md](CONTRIBUTING.md).

## Important / Limitations

### Important

1. Every task runs an **execute** agent and a **verify** agent. Read [docs/execute-and-verify.md](docs/execute-and-verify.md) before defining tasks and done criteria.
2. The worker runs **one task at a time**; you can queue many tasks, but they execute sequentially.

### Limitations

1. Do not edit, commit, or checkout files in the workspace repo while a task is in the **verify** phase — the worker treats any git change during verify as tampering and terminates the cycle as `verify_tampered` (no retry).
2. Point **Workspace repository** at a **dedicated git worktree** so you can keep working in your main checkout; see [docs/execute-and-verify.md](docs/execute-and-verify.md#dedicated-worktree-recommended).

## Docs

- [docs/guide.md](docs/guide.md): how documentation fits together — start here to learn the project
- [CONTRIBUTING.md](CONTRIBUTING.md): setup, PR checklist, where to find docs
- [AGENTS.md](AGENTS.md): scoped paths and code lookups when editing the repo

## License

[MIT](LICENSE).