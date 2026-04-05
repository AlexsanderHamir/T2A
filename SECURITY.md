# Security

## Reporting a vulnerability

Do **not** open a public issue for an undisclosed security problem.

- Prefer opening a **[private security advisory](https://github.com/AlexsanderHamir/T2A/security/advisories/new)** on this repository, or
- Contact the maintainers via GitHub (e.g. repository owner) with enough detail to reproduce.

Include affected component (`taskapi`, `web/`, etc.), steps to reproduce, and suspected impact. This is a small project: there is no formal SLA, but reports are taken seriously.

## Notes

- `taskapi` serves **plain HTTP**; use TLS at your reverse proxy or gateway in production.
- Never paste **secrets** (for example `DATABASE_URL`, tokens) into public issues, discussions, or chat logs.
