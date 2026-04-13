package repo

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func pathEscapesRoot(rel string) bool {
	slog.Debug("trace", "operation", "repo.pathEscapesRoot")
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func canonicalizePathForContainment(path string) (string, error) {
	slog.Debug("trace", "operation", "repo.canonicalizePathForContainment")
	if target, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(target), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	cur := filepath.Clean(path)
	var suffix []string
	for {
		fi, statErr := os.Stat(cur)
		if statErr == nil {
			if !fi.IsDir() {
				return "", fmt.Errorf("path parent is not a directory")
			}
			baseCanonical, evalErr := filepath.EvalSymlinks(cur)
			if evalErr != nil {
				return "", evalErr
			}
			canonical := filepath.Clean(baseCanonical)
			for i := len(suffix) - 1; i >= 0; i-- {
				canonical = filepath.Join(canonical, suffix[i])
			}
			return filepath.Clean(canonical), nil
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		next := filepath.Dir(cur)
		if next == cur {
			return "", statErr
		}
		suffix = append(suffix, filepath.Base(cur))
		cur = next
	}
}
