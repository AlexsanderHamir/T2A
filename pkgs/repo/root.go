package repo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

const (
	// Empty q lists the first N files (walk order) for @-mention browse; non-empty q caps matches for performance.
	maxSearchResultsBrowse = 250
	maxSearchResultsFilter = 100
	maxFileReadBytes       = 32 << 20 // 32 MiB upper bound for line counting
)

// Root is a validated absolute directory used for repo-relative paths.
type Root struct {
	abs string
}

// OpenRoot returns a Root for dir, or ErrInvalidInput if missing or not a directory.
func OpenRoot(dir string) (*Root, error) {
	slog.Debug("trace", "operation", "repo.OpenRoot")
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("%w: REPO_ROOT is empty", domain.ErrInvalidInput)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("%w: repo root: %v", domain.ErrInvalidInput, err)
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("%w: repo root: %v", domain.ErrInvalidInput, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%w: repo root is not a directory", domain.ErrInvalidInput)
	}
	return &Root{abs: abs}, nil
}

// Abs returns the absolute root path.
func (r *Root) Abs() string {
	slog.Debug("trace", "operation", "repo.Root.Abs")
	return r.abs
}

// Ready verifies the root directory still exists and is a directory (for HTTP readiness when REPO_ROOT is configured).
func (r *Root) Ready() error {
	slog.Debug("trace", "operation", "repo.Root.Ready")
	if r == nil {
		return errors.New("repo: nil root")
	}
	fi, err := os.Stat(r.abs)
	if err != nil {
		return fmt.Errorf("repo root: %w", err)
	}
	if !fi.IsDir() {
		return errors.New("repo root is not a directory")
	}
	return nil
}

// Resolve returns an absolute path for a repo-relative path (slashes or OS separators).
func (r *Root) Resolve(rel string) (string, error) {
	slog.Debug("trace", "operation", "repo.Root.Resolve")
	rel = strings.TrimSpace(rel)
	rel = filepath.ToSlash(rel)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.Contains(rel, "..") {
		return "", fmt.Errorf("%w: invalid path", domain.ErrInvalidInput)
	}
	joined := filepath.Join(r.abs, filepath.FromSlash(rel))
	clean := filepath.Clean(joined)
	rootClean := filepath.Clean(r.abs)
	relOut, err := filepath.Rel(rootClean, clean)
	if err != nil || strings.HasPrefix(relOut, "..") {
		return "", fmt.Errorf("%w: path escapes repo root", domain.ErrInvalidInput)
	}
	return clean, nil
}

// Search returns repo-relative paths matching query (substring, case-insensitive).
// Empty query lists up to maxSearchResultsBrowse files (walk order); non-empty query up to maxSearchResultsFilter matches.
func (r *Root) Search(query string) ([]string, error) {
	slog.Debug("trace", "operation", "repo.Root.Search")
	q := strings.ToLower(strings.TrimSpace(query))
	limit := maxSearchResultsFilter
	if q == "" {
		limit = maxSearchResultsBrowse
	}
	var out []string
	err := filepath.WalkDir(r.abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			switch name {
			case ".git", "node_modules", "vendor":
				return filepath.SkipDir
			// Build / cache trees — skip for @-mention browse speed (large workspaces, OneDrive, etc.)
			case "dist", "build", "out", "target", "coverage", ".next", ".nuxt", ".turbo",
				"__pycache__", ".pytest_cache", ".venv", "venv", ".mypy_cache", ".tox":
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(r.abs, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if q == "" || strings.Contains(strings.ToLower(rel), q) {
			out = append(out, rel)
			if len(out) >= limit {
				return fs.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// LineCount returns the number of lines in a file (newline-separated).
func LineCount(absPath string) (int, error) {
	slog.Debug("trace", "operation", "repo.LineCount")
	fi, err := os.Stat(absPath)
	if err != nil {
		return 0, err
	}
	if fi.IsDir() {
		return 0, fmt.Errorf("is a directory")
	}
	if fi.Size() > maxFileReadBytes {
		return 0, fmt.Errorf("%w: file too large", domain.ErrInvalidInput)
	}
	f, err := os.Open(absPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	lr := io.LimitReader(f, maxFileReadBytes+1)
	buf := make([]byte, 32*1024)
	var n int
	var total int64
	var last byte
	hasData := false
	for {
		readN, readErr := lr.Read(buf)
		if readN > 0 {
			chunk := buf[:readN]
			total += int64(readN)
			n += bytes.Count(chunk, []byte{'\n'})
			last = chunk[readN-1]
			hasData = true
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return 0, readErr
		}
	}
	if total > maxFileReadBytes {
		return 0, fmt.Errorf("%w: file too large", domain.ErrInvalidInput)
	}
	if !hasData {
		return 0, nil
	}
	if last != '\n' {
		n++
	}
	return n, nil
}

// ValidateRange returns nil if start..end are valid 1-based inclusive line numbers for the file.
func ValidateRange(absPath string, start, end int) error {
	slog.Debug("trace", "operation", "repo.ValidateRange")
	n, err := LineCount(absPath)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
	}
	return validateRangeWithLineCount(start, end, n)
}

func validateRangeWithLineCount(start, end, n int) error {
	if start < 1 || end < 1 {
		return fmt.Errorf("%w: line numbers must be >= 1", domain.ErrInvalidInput)
	}
	if start > end {
		return fmt.Errorf("%w: start line must be <= end line", domain.ErrInvalidInput)
	}
	if end > n {
		return fmt.Errorf("%w: line range 1-%d is past end of file (%d lines)", domain.ErrInvalidInput, end, n)
	}
	return nil
}

// ValidatePromptMentions checks every parsed mention against the repo root.
func (r *Root) ValidatePromptMentions(prompt string) error {
	slog.Debug("trace", "operation", "repo.Root.ValidatePromptMentions")
	resolvedPaths := make(map[string]string)
	seenFiles := make(map[string]struct{})
	lineCounts := make(map[string]int)
	for _, m := range ParseFileMentions(prompt) {
		abs, ok := resolvedPaths[m.Path]
		if !ok {
			var err error
			abs, err = r.Resolve(m.Path)
			if err != nil {
				return fmt.Errorf("%w: mention @%s: %v", domain.ErrInvalidInput, m.Path, err)
			}
			resolvedPaths[m.Path] = abs
		}
		if _, ok := seenFiles[abs]; !ok {
			fi, err := os.Stat(abs)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("%w: mention @%s: file does not exist", domain.ErrInvalidInput, m.Path)
				}
				return fmt.Errorf("%w: mention @%s: %v", domain.ErrInvalidInput, m.Path, err)
			}
			if fi.IsDir() {
				return fmt.Errorf("%w: mention @%s: path is a directory, not a file", domain.ErrInvalidInput, m.Path)
			}
			seenFiles[abs] = struct{}{}
		}
		if m.HasRange {
			n, ok := lineCounts[abs]
			if !ok {
				lineCount, lineErr := LineCount(abs)
				if lineErr != nil {
					return fmt.Errorf("%w: mention @%s(%d-%d): %w", domain.ErrInvalidInput, m.Path, m.StartLine, m.EndLine, lineErr)
				}
				n = lineCount
				lineCounts[abs] = n
			}
			if err := validateRangeWithLineCount(m.StartLine, m.EndLine, n); err != nil {
				return fmt.Errorf("%w: mention @%s(%d-%d): %v", domain.ErrInvalidInput, m.Path, m.StartLine, m.EndLine, err)
			}
		}
	}
	return nil
}
