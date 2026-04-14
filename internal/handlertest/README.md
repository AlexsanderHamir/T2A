# `internal/handlertest`

Black-box HTTP tests for [`pkgs/tasks/handler`](../../pkgs/tasks/handler): only the exported `handler` API, `httptest`, and `net/http`. [`server.go`](./server.go) duplicates the former `newTaskTestServer*` helpers so production code does not carry test-only exports. Baseline security header assertions live in [`internal/httpsecurityexpect`](../httpsecurityexpect/) so `handler` tests can share them **without** an import cycle (`handler` → `handlertest` → `handler`).

**Whitebox tests** (unexported symbols, `decodeJSON`, path helpers) stay next to the code under `pkgs/tasks/handler`. See [`docs/HANDLER-SCALE.md`](../../docs/HANDLER-SCALE.md).
