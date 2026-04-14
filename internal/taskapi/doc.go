// Package taskapi wires the instrumented task HTTP handler for cmd/taskapi: the standard
// middleware stack around pkgs/tasks/handler.NewHandler. Individual With* implementations and
// route handlers remain in pkgs/tasks/handler. Env-driven startup flags parsed only in cmd/taskapi
// live in internal/taskapiconfig (see docs/RUNTIME-ENV.md).
package taskapi
