// Package storefake provides a contract.Store test double backed by in-memory
// SQLite so harness contract tests exercise real store semantics without a
// worker loop or external database.
package storefake

import (
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/internal/contract"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// Fake satisfies harness.Store using the same SQLite-backed store as integration
// tests, isolated per test via t.TempDir-backed DB.
type Fake struct {
	*store.Store
}

// New returns a Fake that implements contract.Store (alias harness.Store).
//
//funclogmeasure:skip category=tool-required-noop reason="Harness test fake only; store I/O traces live on production harness.Run chokepoints."
func New(t *testing.T) *Fake {
	t.Helper()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	return &Fake{Store: st}
}

var _ contract.Store = (*Fake)(nil)
