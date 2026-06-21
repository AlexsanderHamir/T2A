package repo

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const (
	browseRootInstall = "install"
	browseRootHome    = "home"
	dockerHomeMount   = "/host-home"
	dockerInstallPath = "/app"
)

// BrowseRoot is a top-level directory the workspace picker may start from.
type BrowseRoot struct {
	ID                string `json:"id"`
	Path              string `json:"path"`
	Label             string `json:"label"`
	Available         bool   `json:"available"`
	UnavailableReason string `json:"unavailable_reason,omitempty"`
}

// BrowseEnvironment is where taskapi runs (native host vs Docker container).
type BrowseEnvironment string

const (
	BrowseEnvNative BrowseEnvironment = "native"
	BrowseEnvDocker BrowseEnvironment = "docker"
)

// DetectBrowseEnvironment reports docker when /.dockerenv exists.
func DetectBrowseEnvironment() BrowseEnvironment {
	slog.Debug("trace", "operation", "repo.DetectBrowseEnvironment")
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return BrowseEnvDocker
	}
	return BrowseEnvNative
}

// ResolveBrowseRoots returns allowed picker roots for the current process environment.
// When HAMIX_BROWSE_ROOTS is set (comma-separated absolute paths), it replaces the defaults.
func ResolveBrowseRoots(startDir string) ([]BrowseRoot, BrowseEnvironment, error) {
	slog.Debug("trace", "operation", "repo.ResolveBrowseRoots")
	env := DetectBrowseEnvironment()
	if override := strings.TrimSpace(os.Getenv("HAMIX_BROWSE_ROOTS")); override != "" {
		roots, err := parseBrowseRootPaths(override)
		if err != nil {
			return nil, env, err
		}
		return roots, env, nil
	}
	var roots []BrowseRoot
	install, installErr := resolveInstallBrowseRoot(startDir, env)
	if installErr == nil {
		roots = append(roots, install)
	}
	home, homeErr := resolveHomeBrowseRoot(env)
	if homeErr == nil {
		roots = append(roots, home)
	}
	if len(roots) == 0 {
		return nil, env, fmt.Errorf("no browse roots available")
	}
	return roots, env, nil
}

//funclogmeasure:skip category=hot-path reason="Browse sub-step; operation trace is emitted by ResolveBrowseRoots."
func parseBrowseRootPaths(raw string) ([]BrowseRoot, error) {
	parts := strings.Split(raw, ",")
	out := make([]BrowseRoot, 0, len(parts))
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("browse root %q: %w", p, err)
		}
		root := BrowseRoot{
			ID:    fmt.Sprintf("custom-%d", i),
			Path:  abs,
			Label: browseRootLabel(abs),
		}
		markBrowseRootAvailable(&root)
		out = append(out, root)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("HAMIX_BROWSE_ROOTS is empty after parsing")
	}
	return out, nil
}

//funclogmeasure:skip category=hot-path reason="Browse sub-step; operation trace is emitted by ResolveBrowseRoots."
func resolveInstallBrowseRoot(startDir string, env BrowseEnvironment) (BrowseRoot, error) {
	if env == BrowseEnvDocker {
		root := BrowseRoot{
			ID:    browseRootInstall,
			Path:  dockerInstallPath,
			Label: "Hamix checkout",
		}
		markBrowseRootAvailable(&root)
		return root, nil
	}
	install, err := FindInstallRoot(startDir)
	if err != nil {
		return BrowseRoot{}, err
	}
	root := BrowseRoot{
		ID:    browseRootInstall,
		Path:  install,
		Label: "Hamix checkout",
	}
	markBrowseRootAvailable(&root)
	return root, nil
}

//funclogmeasure:skip category=hot-path reason="Browse sub-step; operation trace is emitted by ResolveBrowseRoots."
func resolveHomeBrowseRoot(env BrowseEnvironment) (BrowseRoot, error) {
	path := ""
	if env == BrowseEnvDocker {
		path = dockerHomeMount
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return BrowseRoot{}, err
		}
		path = home
	}
	root := BrowseRoot{
		ID:    browseRootHome,
		Path:  path,
		Label: "Home",
	}
	markBrowseRootAvailable(&root)
	return root, nil
}

//funclogmeasure:skip category=hot-path reason="Browse sub-step; operation trace is emitted by ResolveBrowseRoots."
func markBrowseRootAvailable(root *BrowseRoot) {
	fi, err := os.Stat(root.Path)
	if err != nil {
		root.Available = false
		root.UnavailableReason = "directory is not accessible"
		return
	}
	if !fi.IsDir() {
		root.Available = false
		root.UnavailableReason = "path is not a directory"
		return
	}
	root.Available = true
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func browseRootLabel(abs string) string {
	base := filepath.Base(filepath.Clean(abs))
	if base == "" || base == string(filepath.Separator) {
		return abs
	}
	return base
}
