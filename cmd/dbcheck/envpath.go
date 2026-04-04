package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

func findRepoRoot(startDir string) (string, error) {
	slog.Debug("trace", "cmd", cmdName, "operation", "dbcheck.findRepoRoot")
	dir := startDir
	for {
		mod := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(mod); err == nil {
			return dir, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stat %s: %w", mod, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found (run from inside this repository)")
		}
		dir = parent
	}
}

func resolveDotenvPath(workingDir, flagPath string) (string, error) {
	slog.Debug("trace", "cmd", cmdName, "operation", "dbcheck.resolveDotenvPath")
	if flagPath != "" {
		return filepath.Clean(flagPath), nil
	}
	root, err := findRepoRoot(workingDir)
	if err != nil {
		return "", fmt.Errorf("find repo root from %s: %w", workingDir, err)
	}
	return filepath.Join(root, ".env"), nil
}
