// Package registry exposes the catalogue of agent runner adapters
// available to the in-process agent worker. Adapters register
// themselves via Register (typically called from an init() in their
// package); the cmd/taskapi binary imports registry/all to trigger
// all registrations.
//
// The supervisor consults the registry at boot and on every settings
// reload to (a) populate the runner choice surface in the SPA
// settings page, (b) build a runner.Runner for the configured id,
// and (c) probe a binary path before persisting it so the operator
// gets fail-fast feedback.
//
// Adding a new runner adapter requires:
//  1. Implement runner.Runner + any desired capability interfaces.
//  2. Create a register.go with init() calling registry.Register.
//  3. Add a blank import line to registry/all/all.go.
package registry
