# Contributing to T2A

GitHub entry point for pull requests. The full developer guide is **[docs/contributing.md](docs/contributing.md)** — read that for vertical slices, contract sync, tests, and troubleshooting.

> **Important** — This file is a stub. Do not duplicate its content here; update [docs/contributing.md](docs/contributing.md) instead.

## Security

For **undisclosed vulnerabilities**, use [SECURITY.md](SECURITY.md) (private advisory on GitHub, not a public issue).

## Verify before PR

From the repo root (matches CI — see [AGENTS.md](AGENTS.md#commands-to-run-before-you-finish)):

```bash
(cd web && npm ci)   # first time or after lockfile changes
./scripts/check.sh     # Windows: .\scripts\check.ps1
```

Go-only quick path: `CHECK_SKIP_WEB=1 ./scripts/check.sh`.

## See also

- [docs/contributing.md](docs/contributing.md) — developer guide (checklist, features, troubleshooting)
- [docs/guide.md](docs/guide.md) — documentation map
- [AGENTS.md](AGENTS.md) — scoped paths and commands when editing
