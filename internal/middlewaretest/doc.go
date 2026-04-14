// Package middlewaretest holds black-box tests for pkgs/tasks/middleware using only its
// exported API. They live under internal/ so the production middleware tree stays smaller
// and easier to scan (implementation + whitebox tests only).
package middlewaretest
