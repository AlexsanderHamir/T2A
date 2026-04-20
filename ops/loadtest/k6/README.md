# k6 scripts

These scripts use the `xk6-sse` extension because stock k6 does not speak SSE natively. Build a k6 binary with the extension once:

```bash
xk6 build --with github.com/grafana/xk6-sse@latest
# → ./k6 (custom binary in the current dir)
```

Run from the repo root:

```bash
TASKAPI_ORIGIN=http://127.0.0.1:8080 ./k6 run ops/loadtest/k6/scenario_a_fanout.js
```

Scripts assume `TASKAPI_ORIGIN` is unauthenticated (dev config). For staging, add a `API_TOKEN` env var and an `Authorization` header in the default `params`.
