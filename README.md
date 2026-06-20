# T2A

The control plane for AI-driven software development.

Think of T2A as the equivalent of [n8n](https://n8n.io) for engineering work. While n8n orchestrates business workflows across SaaS applications, T2A orchestrates the dependencies between engineering tasks.

Human involvement happens where it creates the most value: defining goals, reviewing outcomes, and making key decisions. Everything else can be coordinated and executed automatically.

---

## Get started

Requirements: Go 1.25+, Postgres, and a repo-root `.env` with `DATABASE_URL` (copy from [.env.example](.env.example)).

Start the API and web UI together:

```bash
./scripts/dev.sh        # Unix — chmod +x once if needed
.\scripts\dev.ps1       # Windows
```

This builds `taskapi`, runs it on `http://127.0.0.1:8080`, and starts Vite on `http://localhost:5173`. Ctrl+C stops both.

Run pieces individually:

```bash
go run ./cmd/dbcheck -migrate   # apply schema
go run ./cmd/taskapi            # REST /tasks + SSE /events
```

Verify everything before pushing:

```bash
./scripts/check.sh      # or .\scripts\check.ps1
```

## Important

**Creating tasks with AI agents.** Every task runs an **execute** agent (implements work) and a **verify** agent (judges your checklist). Read [docs/execute-and-verify.md](docs/execute-and-verify.md) before defining tasks and done criteria. As of today, the worker runs **one task at a time**; you can queue many tasks, but they execute sequentially.

## Docs

- **[execute-and-verify.md](docs/execute-and-verify.md)**: execute vs verify agents, criteria reports, and how to write good checklist items
- [docs/architecture.md](docs/architecture.md): how `taskapi`, the store, the worker, and SSE fit together
- [docs/domain/](docs/domain/): subsystem deep dives (harness, execute agent, verify agent, …)
- [docs/api.md](docs/api.md): REST + SSE surface
- [docs/web.md](docs/web.md): web UI architecture
- [AGENTS.md](AGENTS.md): repo map and conventions
- [CONTRIBUTING.md](CONTRIBUTING.md): PR checklist

## License

[MIT](LICENSE).