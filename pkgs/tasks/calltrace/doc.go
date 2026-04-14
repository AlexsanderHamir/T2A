// Package calltrace holds the per-request call stack used for structured logs (call_path,
// helper.io) without depending on pkgs/tasks/handler. HTTP handlers attach the route operation
// via WithRequestRoot; nested helpers use Push. Path is passed into access middleware and JSON
// error helpers (see internal/taskapi and pkgs/tasks/middleware).
package calltrace
