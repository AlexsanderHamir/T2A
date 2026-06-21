# Naming registry

Authoritative product and operator identifiers for Hamix. New code and docs must follow this table.

| Surface | Value | Notes |
|---------|-------|-------|
| Product name (prose, UI) | **Hamix** | Sentence case in UI copy |
| Go module / imports | `github.com/AlexsanderHamir/Hamix` | Matches GitHub repo name |
| Env vars | `HAMIX_*` | See [configuration.md](./configuration.md) |
| Prometheus namespace | `hamix` | e.g. `hamix_agent_runs_total` |
| Worker scratch dir | `hamix-worker` | Under `$TMPDIR` or `HAMIX_WORKER_REPORT_DIR` |
| Temp probe prefix | `.hamix-worker-probe-*` | Supervisor writability probe |
| npm package | `hamix-web` | `web/package.json` |
| localStorage keys | `hamix:*`, `hamix_ui_test_mode` | Client-only persistence |
| Stream event source (type union) | `"hamix"` | Reserved alongside `"cursor"` |
| Script temp files | `hamix-check.*` | Check script scratch logs |
| Nav wordmark | `web/src/components/layout/HamixWordmark.tsx` | CSS gradient text (not raster) |
| Marketing wordmark asset | `assets/Hamix_wordmark.png` | README and external use only |

**Unchanged (not brand):** `taskapi`, `dbcheck`, `funclogmeasure`, `pkgs/tasks`, SSE route `/events`, Postgres table names.

Historical ADRs under [adr/](./adr/) may retain old identifiers where they document decisions at the time they were written. CI allowlists that directory via [scripts/check-brand-allowlist.txt](../scripts/check-brand-allowlist.txt).
