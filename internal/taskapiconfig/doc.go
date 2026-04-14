// Package taskapiconfig holds taskapi-only startup parsing for flags and environment
// variables that are not already handled inside pkgs/tasks/handler. See docs/RUNTIME-ENV.md
// for the full env table; cmd/taskapi/run.go wires the server after envload.Load.
package taskapiconfig
