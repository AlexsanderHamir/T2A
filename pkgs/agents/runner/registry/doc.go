// Package registry exposes the catalogue of agent runner adapters
// available to the in-process agent worker. The supervisor consults
// the registry at boot and on every settings reload to (a) populate
// the runner choice surface in the SPA settings page, (b) build a
// runner.Runner for the configured id, and (c) probe a binary path
// before persisting it so the operator gets fail-fast feedback.
//
// Today the registry returns only the "cursor" descriptor wrapping
// pkgs/agents/runner/cursor. New runners (e.g. a future "claude-cli"
// or "plan-only" mode) plug in here without changing supervisor or
// handler code: register a Descriptor with a Build/Probe pair and the
// SPA picks them up automatically via GET /settings.
package registry
