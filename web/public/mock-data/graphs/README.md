# Graph UI mock datasets

This folder stores static graph fixtures used to stress-test graph rendering and virtualization in the web UI.

## Generate datasets

From `web/`:

- `npm run mock:graph:100k`
- `npm run mock:graph:200k`
- `npm run mock:graph:generate` (defaults to 200k)

## Enable graph page mock mode (env)

Quickest flow (auto-generate + auto-set env):

- `npm run mock:graph:use:200k`

Or custom size:

- `npm run mock:graph:use -- --size=350k`
- `npm run mock:graph:use -- --size=1m`
- `npm run mock:graph:use -- --node-count=275000 --branching-factor=5`

This command:

- generates the target JSON under `public/mock-data/graphs/`
- updates `web/.env.local` with `VITE_TASK_GRAPH_MOCK_URL=<generated-file-url>`

When this env var is set, `TaskGraphPage` loads the graph from this static mock URL instead of calling `/tasks/:id`.

Restart Vite after changing env values.

Custom size/output:

- CLI args:
  - `node ./scripts/generate-graph-mock.mjs --node-count=350000 --branching-factor=5 --output=public/mock-data/graphs/task-graph-350k.json`
- Env vars (optional):
  - `GRAPH_MOCK_NODE_COUNT`
  - `GRAPH_MOCK_BRANCHING_FACTOR`
  - `GRAPH_MOCK_OUTPUT`

## Why this location

`web/public/mock-data/graphs/` is served directly by Vite/dev server and production static hosting, which makes fixtures easy to fetch during visualization experiments without adding API/backend dependencies.
