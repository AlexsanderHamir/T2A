package repo

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

const maxBrowseDirEntries = 200

var browseSkipDirNames = map[string]struct{}{
	"node_modules": {},
}

// BrowseDirEntry is one subdirectory in a picker listing.
type BrowseDirEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	HasChildren bool   `json:"has_children"`
	IsGitRepo   bool   `json:"is_git_repo"`
}

// BrowseDirListing is the response for listing one directory level.
type BrowseDirListing struct {
	Path       string           `json:"path"`
	ParentPath string           `json:"parent_path,omitempty"`
	IsGitRepo  bool             `json:"is_git_repo,omitempty"`
	Entries    []BrowseDirEntry `json:"entries"`
}

// ListBrowseDirsUnrestricted lists immediate subdirectories of absPath without
// root containment enforcement. Used by the register-repo bootstrap flow when
// HAMIX_BROWSE_ROOTS is not configured so operators can locate any git repository
// on the filesystem before registration.
// When absPath is empty, returns an empty listing (caller navigates from a typed path).
func ListBrowseDirsUnrestricted(absPath string) (BrowseDirListing, error) {
	slog.Debug("trace", "operation", "repo.ListBrowseDirsUnrestricted")
	absPath = strings.TrimSpace(absPath)
	if absPath == "" {
		return BrowseDirListing{Entries: []BrowseDirEntry{}}, nil
	}
	clean, err := filepath.Abs(absPath)
	if err != nil {
		return BrowseDirListing{}, fmt.Errorf("%w: invalid path", domain.ErrInvalidInput)
	}
	clean = filepath.Clean(clean)
	if err := ensureDirExists(clean); err != nil {
		return BrowseDirListing{}, err
	}
	entries, err := readBrowseSubdirs(clean)
	if err != nil {
		return BrowseDirListing{}, err
	}
	parent := filepath.Dir(clean)
	if parent == clean {
		parent = ""
	}
	return BrowseDirListing{
		Path:       clean,
		ParentPath: parent,
		IsGitRepo:  isGitWorktree(clean),
		Entries:    entries,
	}, nil
}

// ListBrowseDirs lists immediate subdirectories of absPath when absPath is under roots.
// When absPath is empty, returns one synthetic entry per available root (navigation start).
func ListBrowseDirs(roots []BrowseRoot, absPath string) (BrowseDirListing, error) {
	slog.Debug("trace", "operation", "repo.ListBrowseDirs")
	absPath = strings.TrimSpace(absPath)
	if absPath == "" {
		return listBrowseRootEntries(roots)
	}
	clean, root, err := resolvePathUnderBrowseRoots(roots, absPath)
	if err != nil {
		return BrowseDirListing{}, err
	}
	entries, err := readBrowseSubdirs(clean)
	if err != nil {
		return BrowseDirListing{}, err
	}
	parent := parentBrowsePath(root, clean)
	return BrowseDirListing{
		Path:       clean,
		ParentPath: parent,
		IsGitRepo:  isGitWorktree(clean),
		Entries:    entries,
	}, nil
}

//funclogmeasure:skip category=hot-path reason="Browse sub-step; operation trace is emitted by ListBrowseDirs."
func listBrowseRootEntries(roots []BrowseRoot) (BrowseDirListing, error) {
	entries := make([]BrowseDirEntry, 0, len(roots))
	for _, r := range roots {
		if !r.Available {
			continue
		}
		hasChildren, _ := dirHasBrowsableChildren(r.Path)
		entries = append(entries, BrowseDirEntry{
			Name:        r.Label,
			Path:        r.Path,
			HasChildren: hasChildren,
			IsGitRepo:   isGitWorktree(r.Path),
		})
	}
	sortBrowseEntries(entries)
	return BrowseDirListing{Entries: entries}, nil
}

//funclogmeasure:skip category=hot-path reason="Browse sub-step; operation trace is emitted by ListBrowseDirs."
func resolvePathUnderBrowseRoots(roots []BrowseRoot, absPath string) (string, *BrowseRoot, error) {
	abs, err := filepath.Abs(absPath)
	if err != nil {
		return "", nil, fmt.Errorf("%w: invalid path", domain.ErrInvalidInput)
	}
	clean := filepath.Clean(abs)
	for i := range roots {
		r := &roots[i]
		if !r.Available {
			continue
		}
		rootClean := filepath.Clean(r.Path)
		rel, err := filepath.Rel(rootClean, clean)
		if err != nil || pathEscapesRoot(rel) {
			continue
		}
		if err := ensureDirExists(clean); err != nil {
			return "", nil, err
		}
		if err := ensureUnderRootCanon(rootClean, clean); err != nil {
			return "", nil, err
		}
		return clean, r, nil
	}
	return "", nil, fmt.Errorf("%w: path is outside allowed browse roots", domain.ErrInvalidInput)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ensureDirExists(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: directory does not exist", domain.ErrInvalidInput)
		}
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%w: path is not a directory", domain.ErrInvalidInput)
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ensureUnderRootCanon(rootClean, target string) error {
	targetCanon, err := canonicalizePathForContainment(target)
	if err != nil {
		return fmt.Errorf("%w: path canonicalization failed: %v", domain.ErrInvalidInput, err)
	}
	rootCanon, err := canonicalizePathForContainment(rootClean)
	if err != nil {
		return fmt.Errorf("%w: root canonicalization failed: %v", domain.ErrInvalidInput, err)
	}
	rel, err := filepath.Rel(rootCanon, targetCanon)
	if err != nil || pathEscapesRoot(rel) {
		return fmt.Errorf("%w: path escapes browse root via symlink", domain.ErrInvalidInput)
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func parentBrowsePath(root *BrowseRoot, clean string) string {
	if root == nil {
		return ""
	}
	rootClean := filepath.Clean(root.Path)
	if clean == rootClean {
		return ""
	}
	parent := filepath.Dir(clean)
	if parent == clean {
		return ""
	}
	rel, err := filepath.Rel(rootClean, parent)
	if err != nil || pathEscapesRoot(rel) {
		return rootClean
	}
	return parent
}

//funclogmeasure:skip category=hot-path reason="Browse sub-step; operation trace is emitted by ListBrowseDirs."
func readBrowseSubdirs(dir string) ([]BrowseDirEntry, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]BrowseDirEntry, 0, len(ents))
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if _, skip := browseSkipDirNames[name]; skip {
			continue
		}
		child := filepath.Join(dir, name)
		hasChildren, _ := dirHasBrowsableChildren(child)
		out = append(out, BrowseDirEntry{
			Name:        name,
			Path:        child,
			HasChildren: hasChildren,
			IsGitRepo:   isGitWorktree(child),
		})
		if len(out) >= maxBrowseDirEntries {
			break
		}
	}
	sortBrowseEntries(out)
	return out, nil
}

//funclogmeasure:skip category=hot-path reason="Browse sub-step; operation trace is emitted by ListBrowseDirs."
func dirHasBrowsableChildren(dir string) (bool, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		if _, skip := browseSkipDirNames[e.Name()]; skip {
			continue
		}
		return true, nil
	}
	return false, nil
}

//funclogmeasure:skip category=hot-path reason="Browse sub-step; operation trace is emitted by ListBrowseDirs."
func isGitWorktree(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func sortBrowseEntries(entries []BrowseDirEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}
