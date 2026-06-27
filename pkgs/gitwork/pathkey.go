package gitwork

import (
	"path/filepath"
	"runtime"
	"strings"
)

// PathKey normalizes filesystem paths for Hamix ↔ git comparisons.
// Git paths use forward slashes; DB rows may use OS-native separators on Windows.
// Shared by reconcile (store/reconcile_git.go), inventory (store_git_inventory.go),
// and worktree probe — keep compare semantics identical across those call sites.
//
//funclogmeasure:skip category=hot-path reason="Pure path compare helper without I/O; operation trace is emitted by reconcile/inventory chokepoints."
func PathKey(path string) string {
	key := strings.TrimSpace(path)
	key = strings.ReplaceAll(key, `\`, `/`)
	key = filepath.ToSlash(filepath.Clean(key))
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return key
}

// PathKeyEqual reports whether two paths refer to the same filesystem location
// after PathKey normalization.
//
//funclogmeasure:skip category=hot-path reason="Pure path compare helper without I/O; operation trace is emitted by reconcile/inventory chokepoints."
func PathKeyEqual(a, b string) bool {
	return PathKey(a) == PathKey(b)
}
