# Web testing

Contributor playbook for Vitest projects, MSW handlers, and the CI web matrix.

| | |
| --- | --- |
| **Applies to** | Adding or fixing tests under `web/` |
| **Audience** | Frontend contributors and agents |
| **Prerequisite** | [docs/web.md](../web.md), [CONTRIBUTING.md](../../CONTRIBUTING.md) |

## CI groups

CI runs ten parallel `web` jobs via `scripts/check-web.sh --group=<name>`:

| Group | Runs |
| --- | --- |
| `lint` | check-brand, eslint, check:standards |
| `build` | `tsc --noEmit`, vite build |
| `test-unit` | Vitest `unit` project |
| `test-components` | Vitest `components` project |
| `test-app` | Vitest `app` project |
| `test-task-pages` | Vitest `task-pages` project |
| `test-task-create` | Vitest `task-create` project |
| `test-settings` | Vitest `settings` project |
| `test-projects` | Vitest `projects` project |
| `test-worktrees` | Vitest `worktrees` project |

Each `test-*` group maps 1:1 to a Vitest project name (`--project=<name>`).

Locally:

```powershell
.\scripts\check-web.ps1 -Group test-unit
.\scripts\check-web.ps1 -Group test-app
```

Unix: `./scripts/check-web.sh --group=test-unit`

Scoped iteration inside `web/`:

```bash
npm test -- --project=unit
npm test -- --project=app src/app/AppRouting.test.tsx
```

## Vitest projects

Defined in [`web/vitest.workspace.ts`](../../web/vitest.workspace.ts):

| Project | Owns |
| --- | --- |
| `unit` | `*.test.ts` — parsers, pure helpers, `renderHook` |
| `components` | `*.test.tsx` — single components, hooks (`renderHook`), no `<App />`, no full pages |
| `app` | App shell, routing, bootstrap, 404, route announcer |
| `task-pages` | Task detail/cycle/event/home/templates/drafts pages |
| `task-create` | Create-task modal flows (not hooks — those are `components`) |
| `settings` | Settings page |
| `projects` | Project list and detail pages |
| `worktrees` | Worktrees page |

## Testing boundary

Web tests prove UI and HTTP contracts given fixture data. They do **not** simulate infrastructure the SPA cannot run in jsdom.

| In scope | Out of scope |
| --- | --- |
| Component renders given mocked JSON | Worker picked up a task |
| User action produces correct HTTP request/body | Cursor or Claude CLI executed |
| Loading, error, and retry UI | Postgres or live database state |
| URL query params affect page filters | Live SSE from a real agent run |

Hook tests under `src/tasks/create/hooks/` belong in the `components` project, not `task-create`.

## Five rules

1. **Network via MSW.** New tests use `server.use(...)` with handlers in [`web/src/test/handlers/`](../../web/src/test/handlers/). Do not add `vi.spyOn(globalThis, "fetch")`.
2. **No bare async.** No `setTimeout` and no `new Promise(() => {})` in tests — use `vi.useFakeTimers()` or `createDeferred()` from [`web/src/test/deferred.ts`](../../web/src/test/deferred.ts).
3. **One unit per file.** No `<App />` outside `app`; no full page render outside the matching full-app project.
4. **File size ≤ 500 lines** (see `CODE_STANDARDS.mdc`).
5. **Mind the budget.** Tests over ~2s or files over ~30s should be split or simplified.

## MSW pattern

```ts
import { server } from "@/test/server";
import { appDefaultHandlers, renderApp, setupAppTest } from "@/test/integration/appHarness";
import { tasksListEmpty } from "@/test/handlers/tasks";

beforeEach(() => {
  setupAppTest();
  server.use(...appDefaultHandlers(), tasksListEmpty());
});
```

Baseline handlers: [`bootstrap.ts`](../../web/src/test/handlers/bootstrap.ts) (404 bootstrap), [`tasks.ts`](../../web/src/test/handlers/tasks.ts), [`repo.ts`](../../web/src/test/handlers/repo.ts).

## See also

- [docs/domain/testing.md](testing.md) — Go verification ladder
- [docs/web.md](../web.md) — SPA architecture
